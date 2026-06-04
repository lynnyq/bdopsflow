package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	pb "github.com/lynnyq/bdopsflow/proto"
	"github.com/lynnyq/bdopsflow/scheduler/internal/webhook"
	"github.com/lynnyq/bdopsflow/scheduler/pkg/database"
	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
	rqlite "github.com/rqlite/gorqlite"
)

func CalculateNextExecutionTime(cronExpr string, isEnabled bool) string {
	if cronExpr == "" || !isEnabled {
		return ""
	}

	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(cronExpr)
	if err != nil {
		schedule, err = cron.ParseStandard(cronExpr)
		if err != nil {
			return ""
		}
	}

	nextTime := schedule.Next(time.Now())
	if nextTime.IsZero() {
		return ""
	}
	return nextTime.Format(time.RFC3339)
}

type TaskDispatcher func(executorName string, task *pb.Task) error

type ExecutorConnectivityChecker interface {
	IsExecutorConnected(executorName string) bool
}

type LeaderAddrResolver interface {
	GetLeaderHTTPAddr(ctx context.Context) (string, error)
}

type CancelNotifier interface {
	AddCancelExecutionId(executorName, executionId string)
}

type SchedulerService struct {
	DB                   database.DB
	redis                *redis.Client
	dispatcher           TaskDispatcher
	cronScheduler        interface {
		RegisterTask(taskID int64, cronExpr string)
		UnregisterTask(taskID int64)
		Pause()
		Resume()
		IsPaused() bool
		GetUptime() time.Duration
		LoadAndRegisterTasks()
	}
	webhookSvc            *WebhookService
	stopCleanupCh         chan struct{}
	ExecutorDomainService *ExecutorDomainService
	isLeader              bool
	leaderMu              sync.RWMutex
	connectivityChecker   ExecutorConnectivityChecker
	leaderAddrResolver    LeaderAddrResolver
	httpClient            *http.Client
	cancelNotifier        CancelNotifier
}

func NewSchedulerService(db database.DB, redis *redis.Client) *SchedulerService {
	return &SchedulerService{
		DB:           db,
		redis:        redis,
		stopCleanupCh: make(chan struct{}),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (s *SchedulerService) SetCronScheduler(cs interface {
	RegisterTask(taskID int64, cronExpr string)
	UnregisterTask(taskID int64)
	Pause()
	Resume()
	IsPaused() bool
	GetUptime() time.Duration
	LoadAndRegisterTasks()
}) {
	s.cronScheduler = cs
}

func (s *SchedulerService) SetTaskDispatcher(dispatcher TaskDispatcher) {
	s.dispatcher = dispatcher
}

func (s *SchedulerService) SetWebhookService(webhookSvc *WebhookService) {
	s.webhookSvc = webhookSvc
}

func (s *SchedulerService) SetConnectivityChecker(checker ExecutorConnectivityChecker) {
	s.connectivityChecker = checker
}

func (s *SchedulerService) SetLeaderAddrResolver(resolver LeaderAddrResolver) {
	s.leaderAddrResolver = resolver
}

func (s *SchedulerService) SetCancelNotifier(notifier CancelNotifier) {
	s.cancelNotifier = notifier
}

func (s *SchedulerService) ForwardToLeader(ctx context.Context, method, path string, body io.Reader) ([]byte, int, error) {
	if s.leaderAddrResolver == nil {
		return nil, http.StatusServiceUnavailable, fmt.Errorf("leader address resolver not configured")
	}

	leaderAddr, err := s.leaderAddrResolver.GetLeaderHTTPAddr(ctx)
	if err != nil {
		return nil, http.StatusServiceUnavailable, fmt.Errorf("failed to resolve leader address: %w", err)
	}
	if leaderAddr == "" {
		return nil, http.StatusServiceUnavailable, fmt.Errorf("leader address is empty")
	}

	targetURL := fmt.Sprintf("http://%s%s", leaderAddr, path)
	slog.Info("forwarding request to leader", "method", method, "url", targetURL)

	req, err := http.NewRequestWithContext(ctx, method, targetURL, body)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to create forwarded request: %w", err)
	}

	if contentType, ok := ctx.Value("content_type").(string); ok {
		req.Header.Set("Content-Type", contentType)
	} else {
		req.Header.Set("Content-Type", "application/json")
	}

	if authHeader, ok := ctx.Value("authorization").(string); ok && authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	req.Header.Set("X-Forwarded-By", "bdopsflow-scheduler")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, http.StatusBadGateway, fmt.Errorf("failed to forward request to leader: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to read leader response: %w", err)
	}

	return respBody, resp.StatusCode, nil
}

func (s *SchedulerService) IsLeader() bool {
	s.leaderMu.RLock()
	defer s.leaderMu.RUnlock()
	return s.isLeader
}

func (s *SchedulerService) SetLeader(leader bool) {
	s.leaderMu.Lock()
	defer s.leaderMu.Unlock()
	s.isLeader = leader
}

func (s *SchedulerService) SendWebhookNotification(ctx context.Context, taskID int64, executionID, status, output, errorMsg string, durationMs int64) {
	if s.webhookSvc == nil {
		return
	}

	task, err := s.GetTaskByID(ctx, taskID)
	if err != nil {
		return
	}

	if task.WebhookID == nil {
		return
	}

	wh, err := s.webhookSvc.GetByID(ctx, *task.WebhookID)
	if err != nil {
		return
	}

	if !wh.IsEnabled {
		return
	}

	event := "success"
	if status == "failed" {
		event = "failed"
	} else if status == "skipped" {
		event = "skipped"
	}

	var events []string
	if task.WebhookEvents != "" {
		json.Unmarshal([]byte(task.WebhookEvents), &events)
	}
	if len(events) > 0 {
		matched := false
		for _, e := range events {
			if e == event || e == "*" {
				matched = true
				break
			}
		}
		if !matched {
			return
		}
	}

	payload := map[string]interface{}{
		"event":       event,
		"timestamp":   time.Now().Unix(),
		"delivery_id": uuid.New().String(),
		"task": map[string]interface{}{
			"id":   taskID,
			"name": task.Name,
			"type": task.Type,
		},
		"execution": map[string]interface{}{
			"id":          executionID,
			"status":      status,
			"output":      output,
			"error":       errorMsg,
			"duration_ms": durationMs,
		},
	}

	config := webhook.WebhookConfig{
		URL:     wh.URL,
		Method:  wh.Method,
		Headers: make(map[string]string),
		Events:  events,
	}

	if wh.Headers != "" {
		json.Unmarshal([]byte(wh.Headers), &config.Headers)
	}

	if err := s.webhookSvc.SendWithSignature(ctx, config, payload, wh.Secret); err != nil {
		slog.Error("failed to send webhook notification", "task_id", taskID, "execution_id", executionID, "error", err)
	} else {
		slog.Info("webhook notification sent", "task_id", taskID, "execution_id", executionID, "event", event)
	}
}

func (s *SchedulerService) executeQuery(query string, args []interface{}) (rqlite.QueryResult, error) {
	if len(args) > 0 {
		stmt := rqlite.ParameterizedStatement{
			Query:     query,
			Arguments: args,
		}
		return s.DB.QueryOneParameterized(stmt)
	}
	return s.DB.QueryOne(query)
}

func (s *SchedulerService) getLastExecutionStatus(ctx context.Context, taskID int64) string {
	query := `SELECT status FROM bdopsflow_task_executions WHERE task_id = ? ORDER BY created_at DESC LIMIT 1`
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{taskID},
	}
	qr, err := s.DB.QueryOneParameterized(stmt)
	if err != nil || qr.Err != nil {
		return ""
	}
	if !qr.Next() {
		return ""
	}
	row, err := qr.Slice()
	if err != nil {
		return ""
	}
	return rowString(row[0])
}

func (s *SchedulerService) GetDomainName(ctx context.Context, domainID int64) string {
	query := `SELECT name FROM bdopsflow_domains WHERE id = ?`
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{domainID},
	}
	qr, err := s.DB.QueryOneParameterized(stmt)
	if err != nil {
		return fmt.Sprintf("领域 %d", domainID)
	}
	if qr.Err != nil {
		return fmt.Sprintf("领域 %d", domainID)
	}

	if !qr.Next() {
		return fmt.Sprintf("领域 %d", domainID)
	}

	row, err := qr.Slice()
	if err != nil {
		return fmt.Sprintf("领域 %d", domainID)
	}

	name := rowString(row[0])
	if name == "" {
		return fmt.Sprintf("领域 %d", domainID)
	}

	return name
}
