package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	pb "github.com/lynnyq/bdopsflow/proto"
	"github.com/lynnyq/bdopsflow/scheduler/internal/dag"
	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"github.com/lynnyq/bdopsflow/scheduler/internal/webhook"
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
			slog.Debug("failed to parse cron expression", "cron", cronExpr, "error", err)
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

type SchedulerService struct {
	DB        *rqlite.Connection
	redis     *redis.Client
	dispatcher TaskDispatcher
	cronScheduler interface {
		RegisterTask(taskID int64, cronExpr string)
		UnregisterTask(taskID int64)
		Pause()
		Resume()
		IsPaused() bool
		GetUptime() time.Duration
	}
	webhookSvc *WebhookService
	stopCleanupCh chan struct{}
	ExecutorDomainService *ExecutorDomainService
}

func NewSchedulerService(db *rqlite.Connection, redis *redis.Client) *SchedulerService {
	return &SchedulerService{
		DB:    db,
		redis: redis,
		stopCleanupCh: make(chan struct{}),
	}
}

// StartCleanupRoutine 启动定时清理卡住任务的例程
func (s *SchedulerService) StartCleanupRoutine() {
	go s.cleanupStuckTasks()
}

func (s *SchedulerService) cleanupStuckTasks() {
	ticker := time.NewTicker(60 * time.Second) // 每分钟检查一次
	defer ticker.Stop()

	slog.Info("stuck task cleanup routine started")

	for {
		select {
		case <-s.stopCleanupCh:
			slog.Info("stuck task cleanup routine stopped")
			return
		case <-ticker.C:
			s.cleanupDeadTasks()
			s.cleanupOfflineExecutors()
			s.cleanupStaleTaskLocks()
		}
	}
}

func (s *SchedulerService) cleanupDeadTasks() {
	ctx := context.Background()

	query := `
		SELECT id, task_id, execution_id, start_time, created_at
		FROM bdopsflow_task_executions
		WHERE status = 'running'
		AND (start_time IS NOT NULL AND start_time != '')
		AND created_at < datetime('now', '-5 minutes')
	`

	qr, err := s.DB.QueryOne(query)
	if err != nil {
		slog.Error("cleanup: query stuck bdopsflow_tasks failed", "error", err)
		return
	}
	if qr.Err != nil {
		slog.Error("cleanup: query returned error", "error", qr.Err)
		return
	}

	var stuckExecutions []struct {
		ID           int64
		TaskID       int64
		ExecutionID  string
		StartTime    string
	}

	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			continue
		}
		stuck := struct {
			ID           int64
			TaskID       int64
			ExecutionID  string
			StartTime    string
		}{
			ID:          rowInt64(row[0]),
			TaskID:      rowInt64(row[1]),
			ExecutionID: rowString(row[2]),
			StartTime:   rowString(row[3]),
		}
		stuckExecutions = append(stuckExecutions, stuck)
	}

	if len(stuckExecutions) == 0 {
		return
	}

	slog.Warn("found stuck bdopsflow_tasks", "count", len(stuckExecutions))

	for _, exec := range stuckExecutions {
		lockKey := fmt.Sprintf("task:lock:%s", exec.ExecutionID)
		lockTTL, err := s.redis.TTL(ctx, lockKey).Result()
		if err != nil {
			slog.Warn("cleanup: get lock TTL failed, treating as stuck", "error", err, "execution_id", exec.ExecutionID)
			lockTTL = -1
		}

		lockExists, err := s.redis.Exists(ctx, lockKey).Result()
		if err != nil {
			slog.Error("cleanup: check lock existence failed", "error", err, "execution_id", exec.ExecutionID)
			continue
		}

		if lockExists == 0 || lockTTL < 0 {
			slog.Warn("cleanup: task is stuck, force updating status to failed",
				"task_id", exec.TaskID,
				"execution_id", exec.ExecutionID,
				"start_time", exec.StartTime,
				"lock_exists", lockExists,
				"lock_ttl_seconds", lockTTL,
			)

			err := s.UpdateExecutionResult(ctx, exec.ExecutionID, "failed", "", "task execution timeout or executor crashed")
			if err != nil {
				slog.Error("cleanup: failed to update execution result", "error", err, "execution_id", exec.ExecutionID)
			}

			if lockExists > 0 {
				s.redis.Del(ctx, lockKey)
				slog.Info("cleanup: removed stale lock", "execution_id", exec.ExecutionID)
			}

			go func() {
				if err := s.HandleTaskFailure(context.Background(), exec.TaskID, exec.ExecutionID, "", "task execution timeout or executor crashed"); err != nil {
					slog.Error("cleanupDeadTasks: HandleTaskFailure failed",
						"execution_id", exec.ExecutionID,
						"task_id", exec.TaskID,
						"error", err,
					)
				}
			}()
		} else {
			slog.Warn("cleanup: task has valid lock, skipping",
				"task_id", exec.TaskID,
				"execution_id", exec.ExecutionID,
				"lock_ttl_seconds", lockTTL,
			)
		}
	}
}

func (s *SchedulerService) cleanupOfflineExecutors() {
	ctx := context.Background()

	result, err := s.DB.WriteOne(`
		UPDATE bdopsflow_executors 
		SET status = 'offline', updated_at = datetime('now')
		WHERE status = 'online' 
		AND last_heartbeat < datetime('now', '-30 seconds')
	`)
	if err != nil {
		slog.Error("cleanup: update offline bdopsflow_executors failed", "error", err)
		return
	}
	if result.Err != nil {
		slog.Error("cleanup: update offline bdopsflow_executors returned error", "error", result.Err)
		return
	}

	if result.RowsAffected > 0 {
		slog.Warn("marked bdopsflow_executors as offline", "count", result.RowsAffected)
		s.cleanupTasksFromOfflineExecutors(ctx)
	}
}

func (s *SchedulerService) cleanupTasksFromOfflineExecutors(ctx context.Context) {
	query := `
		SELECT te.id, te.task_id, te.execution_id, te.start_time
		FROM bdopsflow_task_executions te
		JOIN bdopsflow_executors e ON te.executor_id = e.id
		WHERE te.status = 'running'
		  AND (e.status = 'offline' OR e.last_heartbeat < datetime('now', '-30 seconds'))
	`

	qr, err := s.DB.QueryOne(query)
	if err != nil {
		slog.Error("cleanup: query bdopsflow_tasks from offline bdopsflow_executors failed", "error", err)
		return
	}
	if qr.Err != nil {
		slog.Error("cleanup: query bdopsflow_tasks from offline bdopsflow_executors returned error", "error", qr.Err)
		return
	}

	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			continue
		}

		taskID := rowInt64(row[1])
		executionID := rowString(row[2])

		slog.Warn("cleanup: found task on offline executor, marking as failed",
			"execution_id", executionID,
			"task_id", taskID,
		)

		err = s.UpdateExecutionResult(ctx, executionID, "failed", "", "executor went offline during task execution")
		if err != nil {
			slog.Error("cleanup: failed to update execution result", "error", err, "execution_id", executionID)
		}

		lockKey := fmt.Sprintf("task:lock:%s", executionID)
		s.redis.Del(ctx, lockKey)

		go func() {
			if err := s.HandleTaskFailure(context.Background(), taskID, executionID, "", "executor went offline during task execution"); err != nil {
				slog.Error("cleanupTasksFromOfflineExecutors: HandleTaskFailure failed",
					"execution_id", executionID,
					"task_id", taskID,
					"error", err,
				)
			}
		}()
	}
}

// StopCleanupRoutine 停止定时清理例程
func (s *SchedulerService) StopCleanupRoutine() {
	close(s.stopCleanupCh)
}

func (s *SchedulerService) SetCronScheduler(cs interface {
	RegisterTask(taskID int64, cronExpr string)
	UnregisterTask(taskID int64)
	Pause()
	Resume()
	IsPaused() bool
	GetUptime() time.Duration
}) {
	s.cronScheduler = cs
}

func (s *SchedulerService) SetTaskDispatcher(dispatcher TaskDispatcher) {
	s.dispatcher = dispatcher
}

func (s *SchedulerService) SetWebhookService(webhookSvc *WebhookService) {
	s.webhookSvc = webhookSvc
}

func (s *SchedulerService) SendWebhookNotification(ctx context.Context, taskID int64, executionID, status, output, errorMsg string, durationMs int64) {
	slog.Info("SendWebhookNotification called", "task_id", taskID, "execution_id", executionID, "status", status)

	if s.webhookSvc == nil {
		slog.Info("webhook service not configured, skipping notification")
		return
	}

	task, err := s.GetTaskByID(ctx, taskID)
	if err != nil {
		slog.Error("failed to get task for webhook notification", "task_id", taskID, "error", err)
		return
	}

	if task.WebhookID == nil {
		slog.Info("no webhook configured for task", "task_id", taskID)
		return
	}

	wh, err := s.webhookSvc.GetByID(ctx, *task.WebhookID)
	if err != nil {
		slog.Error("failed to get webhook by id", "task_id", taskID, "webhook_id", *task.WebhookID, "error", err)
		return
	}

	if !wh.IsEnabled {
		slog.Info("webhook is disabled, skipping notification", "task_id", taskID, "webhook_id", wh.ID)
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
			slog.Info("event not in webhook_events, skipping", "task_id", taskID, "event", event, "events", events)
			return
		}
	}

	payload := map[string]interface{}{
		"event":        event,
		"timestamp":    time.Now().Unix(),
		"delivery_id":  uuid.New().String(),
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

func (s *SchedulerService) CreateTask(ctx context.Context, query string, args ...interface{}) (*model.Task, error) {
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: args,
	}
	result, err := s.DB.WriteOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if result.Err != nil {
		return nil, result.Err
	}

	id := result.LastInsertID
	task, err := s.GetTaskByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if s.cronScheduler != nil && task.IsEnabled && task.CronExpression != "" {
		s.cronScheduler.RegisterTask(task.ID, task.CronExpression)
	}

	return task, nil
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

func (s *SchedulerService) GetTaskByID(ctx context.Context, id int64) (*model.Task, error) {
	query := `
		SELECT id, workflow_id, name, type, config, cron_expression, timeout_seconds,
		       retry_count, retry_interval, is_enabled, status, domain_id, webhook_id, webhook_events,
		       assigned_executor_id, created_by, created_at, updated_at
		FROM bdopsflow_tasks WHERE id = ?
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{id},
	}
	qr, err := s.DB.QueryOneParameterized(stmt)
	if err != nil {
		return nil, err
	}

	if qr.Err != nil {
		return nil, qr.Err
	}

	if !qr.Next() {
		return nil, fmt.Errorf("task not found")
	}

	task := &model.Task{}
	err = scanTaskResult(&qr, task)
	if err != nil {
		return nil, err
	}

	task.NextExecutionTime = CalculateNextExecutionTime(task.CronExpression, task.IsEnabled)
	task.LastExecutionStatus = s.getLastExecutionStatus(ctx, task.ID)

	return task, nil
}

func (s *SchedulerService) ListTasks(ctx context.Context, domainID int64, role string) ([]*model.Task, error) {
	var query string
	var args []interface{}

	isSystemAdmin := role == "system_admin" || role == "admin"

	if isSystemAdmin {
		query = `
			SELECT id, workflow_id, name, type, config, cron_expression, timeout_seconds,
			       retry_count, retry_interval, is_enabled, status, domain_id, webhook_id, webhook_events,
			       assigned_executor_id, created_by, created_at, updated_at
			FROM bdopsflow_tasks ORDER BY created_at DESC
		`
	} else {
		query = `
			SELECT id, workflow_id, name, type, config, cron_expression, timeout_seconds,
			       retry_count, retry_interval, is_enabled, status, domain_id, webhook_id, webhook_events,
			       assigned_executor_id, created_by, created_at, updated_at
			FROM bdopsflow_tasks WHERE domain_id = ? ORDER BY created_at DESC
		`
		args = append(args, domainID)
	}

	var qr rqlite.QueryResult
	var err error
	if len(args) > 0 {
		stmt := rqlite.ParameterizedStatement{
			Query:     query,
			Arguments: args,
		}
		qr, err = s.DB.QueryOneParameterized(stmt)
	} else {
		qr, err = s.DB.QueryOne(query)
	}

	if err != nil {
		return nil, err
	}

	if qr.Err != nil {
		return nil, qr.Err
	}

	var bdopsflow_tasks []*model.Task
	for qr.Next() {
		task := &model.Task{}
		if err := scanTaskResult(&qr, task); err != nil {
			return nil, err
		}
		task.NextExecutionTime = CalculateNextExecutionTime(task.CronExpression, task.IsEnabled)
		task.LastExecutionStatus = s.getLastExecutionStatus(ctx, task.ID)
		bdopsflow_tasks = append(bdopsflow_tasks, task)
	}

	return bdopsflow_tasks, nil
}

func (s *SchedulerService) UpdateTask(ctx context.Context, id int64, task *model.Task) error {
	query := `
		UPDATE bdopsflow_tasks SET name = ?, type = ?, config = ?, cron_expression = ?,
		               timeout_seconds = ?, retry_count = ?, retry_interval = ?,
		               is_enabled = ?, webhook_id = ?, webhook_events = ?, assigned_executor_id = ?, updated_at = ?
		WHERE id = ?
	`

	isEnabled := int64(0)
	if task.IsEnabled {
		isEnabled = 1
	}

	var assignedExecutorID interface{}
	if task.AssignedExecutorID > 0 {
		assignedExecutorID = task.AssignedExecutorID
	} else {
		assignedExecutorID = nil
	}

	var webhookID interface{}
	if task.WebhookID != nil {
		webhookID = *task.WebhookID
	} else {
		webhookID = nil
	}

	stmt := rqlite.ParameterizedStatement{
		Query: query,
		Arguments: []interface{}{
			task.Name, task.Type, task.Config, task.CronExpression,
			int64(task.TimeoutSeconds), int64(task.RetryCount), int64(task.RetryInterval),
			isEnabled, webhookID, task.WebhookEvents, assignedExecutorID, time.Now().Format("2006-01-02 15:04:05"), id,
		},
	}

	result, err := s.DB.WriteOneParameterized(stmt)
	if err != nil {
		return err
	}

	if result.Err != nil {
		return result.Err
	}

	// 重新从数据库获取最新的任务信息
	updatedTask, err := s.GetTaskByID(ctx, id)
	if err != nil {
		slog.Error("UpdateTask: failed to get updated task", "id", id, "error", err)
		return err
	}

	// 使用最新的数据库信息来更新 Cron 调度器
	if s.cronScheduler != nil {
		if updatedTask.IsEnabled && updatedTask.CronExpression != "" {
			s.cronScheduler.RegisterTask(id, updatedTask.CronExpression)
			slog.Info("UpdateTask: task registered to cron", "id", id, "cron", updatedTask.CronExpression)
		} else {
			s.cronScheduler.UnregisterTask(id)
			slog.Info("UpdateTask: task unregistered from cron", "id", id)
		}
	}

	return nil
}

func (s *SchedulerService) DeleteTask(ctx context.Context, id int64) error {
	query := `DELETE FROM bdopsflow_tasks WHERE id = ?`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{id},
	}

	result, err := s.DB.WriteOneParameterized(stmt)
	if err != nil {
		return err
	}

	if result.Err != nil {
		return result.Err
	}

	if s.cronScheduler != nil {
		s.cronScheduler.UnregisterTask(id)
	}

	return nil
}

func (s *SchedulerService) TriggerTask(ctx context.Context, taskID int64) (string, error) {
	// 检查任务是否已经在运行（数据库级别）
	checkRunningQuery := `
		SELECT execution_id FROM bdopsflow_task_executions 
		WHERE task_id = ? AND status = 'running'
		ORDER BY created_at DESC
		LIMIT 1
	`
	checkStmt := rqlite.ParameterizedStatement{
		Query:     checkRunningQuery,
		Arguments: []interface{}{taskID},
	}
	checkQr, err := s.DB.QueryOneParameterized(checkStmt)
	if err == nil && checkQr.Err == nil && checkQr.Next() {
		row, _ := checkQr.Slice()
		runningExecID := rowString(row[0])
		
		// 任务正在运行，记录一个 skipped 的执行
		skippedExecutionID := fmt.Sprintf("exec-%d-%d", taskID, time.Now().UnixNano())
		now := time.Now().Format("2006-01-02 15:04:05")
		skippedReason := fmt.Sprintf("skipped: previous execution (id: %s) is still running", runningExecID)
		
		var executorID interface{} = nil
		insertQuery := `
			INSERT INTO bdopsflow_task_executions (task_id, execution_id, executor_id, status, output, error, start_time, retry_times, created_at)
			VALUES (?, ?, ?, 'skipped', '', ?, ?, 0, ?)
		`
		insertStmt := rqlite.ParameterizedStatement{
			Query:     insertQuery,
			Arguments: []interface{}{taskID, skippedExecutionID, executorID, skippedReason, now, now},
		}
		
		// 忽略插入错误，继续执行
		if _, dbErr := s.DB.WriteOneParameterized(insertStmt); dbErr != nil {
			slog.Warn("failed to record skipped execution", "task_id", taskID, "error", dbErr)
		} else {
			slog.Warn("task skipped: previous execution still running",
				"task_id", taskID,
				"skipped_execution_id", skippedExecutionID,
				"running_execution_id", runningExecID,
			)

			s.SendWebhookNotification(ctx, taskID, skippedExecutionID, "skipped", "", skippedReason, 0)
		}
		
		return "", fmt.Errorf("task %d is already running (execution_id: %s), skipped", taskID, runningExecID)
	}

	task, err := s.GetTaskByID(ctx, taskID)
	if err != nil {
		return "", fmt.Errorf("get task failed: %w", err)
	}

	executionID := fmt.Sprintf("exec-%d-%d", taskID, time.Now().UnixNano())

	// 使用 execution_id 作为锁的标识
	lockKey := fmt.Sprintf("task:lock:%s", executionID)
	lockValue := fmt.Sprintf("%d", time.Now().UnixNano())
	
	// 尝试获取分布式锁，锁超时时间设置为任务超时时间的2倍，最小60秒，最大3600秒
	lockTTL := time.Duration(task.TimeoutSeconds) * 2 * time.Second
	if lockTTL < 60*time.Second {
		lockTTL = 60 * time.Second
	}
	if lockTTL > 3600*time.Second {
		lockTTL = 3600 * time.Second
	}
	if task.TimeoutSeconds == 0 {
		// 如果任务没有超时限制，锁超时设为10分钟
		lockTTL = 600 * time.Second
	}
	
	lockSet, err := s.redis.SetNX(ctx, lockKey, lockValue, lockTTL).Result()
	if err != nil {
		slog.Warn("acquire lock failed, continuing anyway", "error", err)
	} else if !lockSet {
		return "", fmt.Errorf("task %d is already being executed (lock conflict)", taskID)
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	var executorID interface{} = nil
	insertQuery := `
		INSERT INTO bdopsflow_task_executions (task_id, execution_id, executor_id, status, start_time, retry_times, created_at)
		VALUES (?, ?, ?, 'running', ?, 0, ?)
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     insertQuery,
		Arguments: []interface{}{taskID, executionID, executorID, now, now},
	}
	_, err = s.DB.WriteOneParameterized(stmt)
	if err != nil {
		// 插入失败，清理锁
		if lockSet {
			s.redis.Del(ctx, lockKey)
		}
		return "", fmt.Errorf("create execution record failed: %w", err)
	}

	slog.Info("task execution started",
		"task_id", taskID,
		"execution_id", executionID,
		"type", task.Type,
		"name", task.Name,
		"lock_ttl", lockTTL,
	)

	// 选择执行器：如果任务指定了执行器，则使用指定的执行器；否则使用负载均衡
	var executor *model.Executor
	if task.AssignedExecutorID > 0 {
		// 使用指定的执行器
		executor, err = s.GetExecutorByID(ctx, task.AssignedExecutorID)
		if err != nil {
			slog.Error("specified executor not found",
				"task_id", taskID,
				"assigned_executor_id", task.AssignedExecutorID,
				"error", err,
			)
			s.UpdateExecutionResult(ctx, executionID, "failed", "", fmt.Sprintf("specified executor %d not found: %v", task.AssignedExecutorID, err))
			s.UpdateTaskStatusByID(ctx, taskID, "failed")
			s.SendWebhookNotification(ctx, taskID, executionID, "failed", "", fmt.Sprintf("specified executor %d not found: %v", task.AssignedExecutorID, err), 0)
			s.redis.Del(ctx, lockKey)
			return executionID, fmt.Errorf("specified executor %d not found: %w", task.AssignedExecutorID, err)
		}

		// 检查执行器是否在线且有容量
		if executor.Status != "online" {
			slog.Error("specified executor is not online",
				"task_id", taskID,
				"assigned_executor_id", task.AssignedExecutorID,
				"executor_status", executor.Status,
			)
			s.UpdateExecutionResult(ctx, executionID, "failed", "", fmt.Sprintf("specified executor %d is not online (status: %s)", task.AssignedExecutorID, executor.Status))
			s.UpdateTaskStatusByID(ctx, taskID, "failed")
			s.SendWebhookNotification(ctx, taskID, executionID, "failed", "", fmt.Sprintf("specified executor %d is not online (status: %s)", task.AssignedExecutorID, executor.Status), 0)
			s.redis.Del(ctx, lockKey)
			return executionID, fmt.Errorf("specified executor %d is not online", task.AssignedExecutorID)
		}

		if executor.CurrentLoad >= executor.Capacity {
			slog.Error("specified executor has no capacity",
				"task_id", taskID,
				"assigned_executor_id", task.AssignedExecutorID,
				"current_load", executor.CurrentLoad,
				"capacity", executor.Capacity,
			)
			s.UpdateExecutionResult(ctx, executionID, "failed", "", fmt.Sprintf("specified executor %d has no capacity (load: %d/%d)", task.AssignedExecutorID, executor.CurrentLoad, executor.Capacity))
			s.UpdateTaskStatusByID(ctx, taskID, "failed")
			s.SendWebhookNotification(ctx, taskID, executionID, "failed", "", fmt.Sprintf("specified executor %d has no capacity (load: %d/%d)", task.AssignedExecutorID, executor.CurrentLoad, executor.Capacity), 0)
			s.redis.Del(ctx, lockKey)
			return executionID, fmt.Errorf("specified executor %d has no capacity", task.AssignedExecutorID)
		}

		slog.Info("using specified executor",
			"task_id", taskID,
			"execution_id", executionID,
			"assigned_executor_id", task.AssignedExecutorID,
			"executor_name", executor.Name,
		)
	} else {
		// 使用默认负载均衡算法
		executor, err = s.SelectAvailableExecutor(ctx)
		if err != nil {
			slog.Error("no available executor", "task_id", taskID, "error", err)
			s.UpdateExecutionResult(ctx, executionID, "failed", "", fmt.Sprintf("no available executor: %v", err))
			s.UpdateTaskStatusByID(ctx, taskID, "failed")
			s.SendWebhookNotification(ctx, taskID, executionID, "failed", "", fmt.Sprintf("no available executor: %v", err), 0)
			s.redis.Del(ctx, lockKey)
			return executionID, fmt.Errorf("no available executor: %w", err)
		}
		slog.Info("using load-balanced executor",
			"task_id", taskID,
			"execution_id", executionID,
			"executor_id", executor.ID,
			"executor_name", executor.Name,
		)
	}

	grpcTask := &pb.Task{
		TaskId:         taskID,
		ExecutionId:    executionID,
		Type:           task.Type,
		Config:         task.Config,
		TimeoutSeconds: task.TimeoutSeconds,
		RetryCount:     task.RetryCount,
		RetryInterval:  task.RetryInterval,
	}

	if s.dispatcher == nil {
		slog.Warn("no task dispatcher set", "task_id", taskID)
		s.UpdateExecutionResult(ctx, executionID, "failed", "", "dispatcher not configured")
		s.UpdateTaskStatusByID(ctx, taskID, "failed")
		s.SendWebhookNotification(ctx, taskID, executionID, "failed", "", "dispatcher not configured", 0)
		s.redis.Del(ctx, lockKey)
		return executionID, fmt.Errorf("dispatcher not configured")
	}

	// 在分派任务前，先更新 bdopsflow_task_executions 表设置 executor_id
	updateExecutorQuery := `UPDATE bdopsflow_task_executions SET executor_id = ? WHERE execution_id = ?`
	updateExecutorStmt := rqlite.ParameterizedStatement{
		Query:     updateExecutorQuery,
		Arguments: []interface{}{executor.ID, executionID},
	}
	_, err = s.DB.WriteOneParameterized(updateExecutorStmt)
	if err != nil {
		slog.Warn("failed to update executor_id in bdopsflow_task_executions", "error", err, "execution_id", executionID)
		// 继续执行，不影响任务调度
	}

	if err := s.dispatcher(executor.Name, grpcTask); err != nil {
		slog.Error("dispatch task failed", "task_id", taskID, "executor", executor.Name, "error", err)
		s.UpdateExecutionResult(ctx, executionID, "failed", "", fmt.Sprintf("dispatch failed: %v", err))
		s.UpdateTaskStatusByID(ctx, taskID, "failed")
		s.SendWebhookNotification(ctx, taskID, executionID, "failed", "", fmt.Sprintf("dispatch failed: %v", err), 0)
		s.redis.Del(ctx, lockKey)
		return executionID, fmt.Errorf("dispatch failed: %w", err)
	}

	slog.Info("task dispatched",
		"task_id", taskID,
		"execution_id", executionID,
		"executor", executor.ID,
	)

	renewKey := fmt.Sprintf("task:renew:%s", executionID)
	s.redis.Set(ctx, renewKey, time.Now().Unix(), time.Duration(lockTTL)*time.Second)

	return executionID, nil
}

func (s *SchedulerService) UpdateTaskStatusByID(ctx context.Context, taskID int64, status string) error {
	// 如果任务有cron表达式，我们不应该修改任务状态，让它继续保持pending以便下次cron触发
	task, err := s.GetTaskByID(ctx, taskID)
	if err == nil && task.CronExpression != "" {
		// 对于定时任务，只更新updated_at，不改变status
		query := `UPDATE bdopsflow_tasks SET updated_at = ? WHERE id = ?`
		stmt := rqlite.ParameterizedStatement{
			Query:     query,
			Arguments: []interface{}{time.Now().Format("2006-01-02 15:04:05"), taskID},
		}
		result, err := s.DB.WriteOneParameterized(stmt)
		if err != nil {
			return err
		}
		return result.Err
	}
	
	// 对于非定时任务，正常更新状态
	query := `UPDATE bdopsflow_tasks SET status = ?, updated_at = ? WHERE id = ?`
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{status, time.Now().Format("2006-01-02 15:04:05"), taskID},
	}

	result, err := s.DB.WriteOneParameterized(stmt)
	if err != nil {
		return err
	}

	if result.Err != nil {
		return result.Err
	}

	return nil
}

func (s *SchedulerService) ScanPendingTasks(ctx context.Context) ([]*model.Task, error) {
	query := `
		SELECT id, workflow_id, name, type, config, cron_expression, timeout_seconds,
		       retry_count, retry_interval, is_enabled, status, domain_id, webhook_id, webhook_events,
		       assigned_executor_id, created_by, created_at, updated_at
		FROM bdopsflow_tasks
		WHERE is_enabled = 1 AND cron_expression != ''
	`

	qr, err := s.DB.QueryOne(query)
	if err != nil {
		return nil, err
	}

	if qr.Err != nil {
		return nil, qr.Err
	}

	var bdopsflow_tasks []*model.Task
	for qr.Next() {
		task := &model.Task{}
		if err := scanTaskResult(&qr, task); err != nil {
			return nil, err
		}
		bdopsflow_tasks = append(bdopsflow_tasks, task)
	}

	return bdopsflow_tasks, nil
}

func (s *SchedulerService) SelectAvailableExecutor(ctx context.Context) (*model.Executor, error) {
	query := `
		SELECT id, name, address, status, last_heartbeat, capacity, current_load, created_at, updated_at
		FROM bdopsflow_executors
		WHERE status = 'online' AND current_load < capacity
		  AND last_heartbeat > datetime('now', '-30 seconds')
		ORDER BY current_load ASC, RANDOM()
		LIMIT 1
	`

	qr, err := s.DB.QueryOne(query)
	if err != nil {
		return nil, err
	}

	if qr.Err != nil {
		return nil, qr.Err
	}

	if !qr.Next() {
		return nil, fmt.Errorf("no available executor")
	}

	exec := &model.Executor{}
	if err := scanExecutorResult(&qr, exec); err != nil {
		return nil, err
	}

	return exec, nil
}

func (s *SchedulerService) RegisterExecutor(ctx context.Context, name, address string, capacity int32) (string, error) {
	now := time.Now().Format("2006-01-02 15:04:05")

	existingExecutor, err := s.GetExecutorByName(ctx, name)
	if err == nil && existingExecutor != nil {
		updateQuery := `
			UPDATE bdopsflow_executors 
			SET address = ?, capacity = ?, status = 'online', last_heartbeat = ?, updated_at = ?
			WHERE name = ?
		`
		stmt := rqlite.ParameterizedStatement{
			Query:     updateQuery,
			Arguments: []interface{}{address, capacity, now, now, name},
		}
		result, err := s.DB.WriteOneParameterized(stmt)
		if err != nil {
			return "", err
		}
		if result.Err != nil {
			return "", result.Err
		}

		slog.Info("RegisterExecutor: updated existing executor",
			"name", name,
			"executor_id", existingExecutor.ID,
			"address", address,
			"capacity", capacity,
		)
		return name, nil
	}

	insertQuery := `
		INSERT INTO bdopsflow_executors (name, address, status, capacity, current_load, is_global, last_heartbeat, created_at, updated_at)
		VALUES (?, ?, 'online', ?, 0, 0, ?, ?, ?)
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     insertQuery,
		Arguments: []interface{}{name, address, capacity, now, now, now},
	}

	result, err := s.DB.WriteOneParameterized(stmt)
	if err != nil {
		return "", err
	}

	if result.Err != nil {
		return "", result.Err
	}

	executorDBID := result.LastInsertID

	if executorDBID > 0 && s.ExecutorDomainService != nil {
		_ = s.ExecutorDomainService.AssignExecutorToDefaultDomain(ctx, name, 1)
	}

	slog.Info("RegisterExecutor: created new executor",
		"name", name,
		"executor_id", executorDBID,
		"address", address,
		"capacity", capacity,
	)

	return name, nil
}

func (s *SchedulerService) DeleteExecutor(ctx context.Context, id int64) error {
	query := `DELETE FROM bdopsflow_executors WHERE id = ?`
	result, err := s.DB.WriteOneParameterized(rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{id},
	})
	if err != nil {
		return err
	}

	if result.Err != nil {
		return result.Err
	}

	return nil
}

func (s *SchedulerService) DeleteExecutorByName(ctx context.Context, name string) error {
	query := `DELETE FROM bdopsflow_executors WHERE name = ?`
	result, err := s.DB.WriteOneParameterized(rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{name},
	})
	if err != nil {
		return err
	}

	if result.Err != nil {
		return result.Err
	}

	return nil
}

func (s *SchedulerService) SetExecutorStatusByName(ctx context.Context, name string, status string) error {
	query := `UPDATE bdopsflow_executors SET status = ?, updated_at = ? WHERE name = ?`
	now := time.Now().Format("2006-01-02 15:04:05")
	result, err := s.DB.WriteOneParameterized(rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{status, now, name},
	})
	if err != nil {
		return err
	}

	if result.Err != nil {
		return result.Err
	}

	return nil
}

// UpdateExecutorCapacityByName 更新执行器的容量（通过名称）
func (s *SchedulerService) UpdateExecutorCapacityByName(ctx context.Context, name string, capacity int64) error {
	if capacity <= 0 {
		return fmt.Errorf("capacity must be positive")
	}

	query := `UPDATE bdopsflow_executors SET capacity = ?, updated_at = ? WHERE name = ?`
	now := time.Now().Format("2006-01-02 15:04:05")
	result, err := s.DB.WriteOneParameterized(rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{capacity, now, name},
	})
	if err != nil {
		return err
	}
	if result.Err != nil {
		return result.Err
	}

	key := fmt.Sprintf("executor:target_capacity:%s", name)
	if err := s.redis.Set(ctx, key, capacity, 0).Err(); err != nil {
		slog.Warn("failed to store target capacity in redis", "error", err)
	}

	slog.Info("updated executor capacity",
		"executor_name", name,
		"new_capacity", capacity)
	return nil
}

func (s *SchedulerService) SetExecutorStatus(ctx context.Context, id int64, status string) error {
	query := `UPDATE bdopsflow_executors SET status = ?, updated_at = ? WHERE id = ?`
	now := time.Now().Format("2006-01-02 15:04:05")
	result, err := s.DB.WriteOneParameterized(rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{status, now, id},
	})
	if err != nil {
		return err
	}

	if result.Err != nil {
		return result.Err
	}

	return nil
}

// UpdateExecutorCapacity 更新执行器的容量
func (s *SchedulerService) UpdateExecutorCapacity(ctx context.Context, id int64, capacity int64) error {
	if capacity <= 0 {
		return fmt.Errorf("capacity must be positive")
	}

	query := `UPDATE bdopsflow_executors SET capacity = ?, updated_at = ? WHERE id = ?`
	now := time.Now().Format("2006-01-02 15:04:05")
	result, err := s.DB.WriteOneParameterized(rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{capacity, now, id},
	})
	if err != nil {
		return err
	}
	if result.Err != nil {
		return result.Err
	}

	key := fmt.Sprintf("executor:target_capacity:%d", id)
	if err := s.redis.Set(ctx, key, capacity, 0).Err(); err != nil {
		slog.Warn("failed to store target capacity in redis", "error", err)
	}

	slog.Info("updated executor capacity",
		"executor_id", id,
		"new_capacity", capacity)
	return nil
}

// GetExecutorTargetCapacity 获取执行器的目标容量
func (s *SchedulerService) GetExecutorTargetCapacity(ctx context.Context, name string) (int32, error) {
	key := fmt.Sprintf("executor:target_capacity:%s", name)
	val, err := s.redis.Get(ctx, key).Int64()
	if err != nil {
		exec, err := s.GetExecutorByName(ctx, name)
		if err != nil {
			return 0, err
		}
		return int32(exec.Capacity), nil
	}
	return int32(val), nil
}

func (s *SchedulerService) UpdateExecutorHeartbeat(ctx context.Context, name string, currentLoad int32) error {
	return s.UpdateExecutorHeartbeatWithRunningTasks(ctx, name, currentLoad, nil)
}

func (s *SchedulerService) UpdateExecutorHeartbeatWithRunningTasks(ctx context.Context, name string, currentLoad int32, runningExecutionIds []string) error {
	query := `
		UPDATE bdopsflow_executors SET current_load = ?, last_heartbeat = ?, updated_at = ?
		WHERE name = ? AND status = 'online'
	`

	now := time.Now()
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{currentLoad, now, now, name},
	}

	result, err := s.DB.WriteOneParameterized(stmt)
	if err != nil {
		return err
	}

	if result.Err != nil {
		return result.Err
	}

	for _, execID := range runningExecutionIds {
		if err := s.renewTaskLock(ctx, execID); err != nil {
			slog.Warn("failed to renew task lock", "execution_id", execID, "error", err)
		}
	}

	return nil
}

func (s *SchedulerService) renewTaskLock(ctx context.Context, executionID string) error {
	lockKey := fmt.Sprintf("task:lock:%s", executionID)
	exists, err := s.redis.Exists(ctx, lockKey).Result()
	if err != nil {
		return err
	}

	lockTTL := 300

	if exists == 0 {
		// 锁不存在，可能是新调度器接管，或者任务锁过期了
		// 执行器正在运行此任务，我们应该重新创建锁
		slog.Warn("lock not found, recreating for executor reported running task", "execution_id", executionID)
		if err := s.redis.Set(ctx, lockKey, "recovered_by_executor", time.Duration(lockTTL)*time.Second).Err(); err != nil {
			return err
		}
	} else {
		// 锁存在，延长过期时间
		if err := s.redis.Expire(ctx, lockKey, time.Duration(lockTTL)*time.Second).Err(); err != nil {
			return err
		}
	}

	// 无论锁是新建还是延长，我们都更新 renew 时间戳
	renewKey := fmt.Sprintf("task:renew:%s", executionID)
	s.redis.Set(ctx, renewKey, time.Now().Unix(), time.Duration(lockTTL)*time.Second)

	// 任务已经被执行器确认了，清除失败计数
	failCountKey := fmt.Sprintf("task:renew:fail:count:%s", executionID)
	s.redis.Del(ctx, failCountKey)

	slog.Debug("task lock renewed", "execution_id", executionID, "lock_ttl_seconds", lockTTL)
	return nil
}

func (s *SchedulerService) cleanupStaleTaskLocks() {
	ctx := context.Background()

	// 第一部分：清理数据库中显示为 running 但状态异常的任务
	query := `
		SELECT execution_id, task_id
		FROM bdopsflow_task_executions
		WHERE status = 'running'
	`

	qr, err := s.DB.QueryOne(query)
	if err != nil {
		slog.Error("cleanup: query running bdopsflow_tasks failed", "error", err)
		return
	}
	if qr.Err != nil {
		slog.Error("cleanup: query returned error", "error", qr.Err)
		return
	}

	now := time.Now().Unix()
	lockTTLSeconds := int64(300) // 与 renewTaskLock 中的 TTL 一致（秒）
	maxInterval := lockTTLSeconds // 给足够的缓冲时间，超过 TTL 才标记为失败
	requiredFailCount := int64(3) // 连续失败 3 次才标记为失败

	runningExecutionIDs := make(map[string]bool)

	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			continue
		}

		executionID := rowString(row[0])
		taskID := rowInt64(row[1])
		
		runningExecutionIDs[executionID] = true

		renewKey := fmt.Sprintf("task:renew:%s", executionID)
		failCountKey := fmt.Sprintf("task:renew:fail:count:%s", executionID)

		lastRenewStr, err := s.redis.Get(ctx, renewKey).Result()
		if err != nil {
			failCountStr, _ := s.redis.Get(ctx, failCountKey).Result()
			var failCount int64 = 0
			if failCountStr != "" {
				fmt.Sscanf(failCountStr, "%d", &failCount)
			}
			failCount++

			s.redis.Set(ctx, failCountKey, failCount, 0)

			if failCount >= requiredFailCount {
				slog.Warn("cleanup: consecutive renewal failures reached threshold, marking task as failed",
					"execution_id", executionID,
					"task_id", taskID,
					"fail_count", failCount,
				)
				s.forceFailTask(ctx, executionID, taskID, fmt.Sprintf("executor heartbeat timeout after %d consecutive failures", failCount))
			} else {
				slog.Warn("cleanup: no renewal record found, incrementing fail count",
					"execution_id", executionID,
					"task_id", taskID,
					"fail_count", failCount,
				)
			}
			continue
		}

		var lastRenew int64
		fmt.Sscanf(lastRenewStr, "%d", &lastRenew)

		interval := now - lastRenew
		if interval > maxInterval {
			failCountStr, _ := s.redis.Get(ctx, failCountKey).Result()
			var failCount int64 = 0
			if failCountStr != "" {
				fmt.Sscanf(failCountStr, "%d", &failCount)
			}
			failCount++

			s.redis.Set(ctx, failCountKey, failCount, 0)

			if failCount >= requiredFailCount {
				slog.Warn("cleanup: consecutive renewal failures reached threshold, marking task as failed",
					"execution_id", executionID,
					"task_id", taskID,
					"fail_count", failCount,
					"last_renew_seconds_ago", interval,
				)
				s.forceFailTask(ctx, executionID, taskID, fmt.Sprintf("task execution timeout, no heartbeat for %d seconds after %d consecutive failures", interval, failCount))
			} else {
				slog.Warn("cleanup: task lock renewal timeout, incrementing fail count",
					"execution_id", executionID,
					"task_id", taskID,
					"fail_count", failCount,
					"last_renew_seconds_ago", interval,
				)
			}
		} else {
			if failCountStr, _ := s.redis.Get(ctx, failCountKey).Result(); failCountStr != "" {
				s.redis.Del(ctx, failCountKey)
				slog.Debug("cleanup: renewal recovered, reset fail count",
					"execution_id", executionID,
					"task_id", taskID,
				)
			}
		}
	}

	// 第二部分：清理 Redis 中残留的但数据库中已不是 running 状态的任务锁
	// 首先获取所有 task:lock: 前缀的键
	lockPattern := "task:lock:*"
	var cursor uint64 = 0
	cleanedLocks := 0

	for {
		var keys []string
		var err error
		keys, cursor, err = s.redis.Scan(ctx, cursor, lockPattern, 100).Result()
		if err != nil {
			slog.Error("cleanup: scan task locks failed", "error", err)
			break
		}

		for _, key := range keys {
			// 提取 execution_id
			// key 格式: task:lock:exec-xxx-xxx
			executionID := ""
			parts := strings.SplitN(key, ":", 3)
			if len(parts) >= 3 {
				executionID = parts[2]
			}

			// 检查这个 execution_id 是否还在 running 状态
			if !runningExecutionIDs[executionID] {
				// 这个锁对应的任务已经不是 running 状态了，清理掉
				slog.Info("cleanup: removing stale task lock for non-running execution", 
					"execution_id", executionID, 
					"key", key)
				
				// 删除锁和相关的键
				s.redis.Del(ctx, key)
				
				// 也清理相关的 renew 和 fail count 键
				renewKey := fmt.Sprintf("task:renew:%s", executionID)
				failCountKey := fmt.Sprintf("task:renew:fail:count:%s", executionID)
				s.redis.Del(ctx, renewKey, failCountKey)
				
				cleanedLocks++
			}
		}

		if cursor == 0 {
			break
		}
	}

	if cleanedLocks > 0 {
		slog.Info("cleanup: removed stale task locks", "count", cleanedLocks)
	}
}

func (s *SchedulerService) forceFailTask(ctx context.Context, executionID string, taskID int64, reason string) {
	slog.Warn("force failing task",
		"execution_id", executionID,
		"task_id", taskID,
		"reason", reason,
	)

	err := s.UpdateExecutionResult(ctx, executionID, "failed", "", reason)
	if err != nil {
		slog.Error("cleanup: failed to update execution result", "error", err, "execution_id", executionID)
	}

	lockKey := fmt.Sprintf("task:lock:%s", executionID)
	s.redis.Del(ctx, lockKey)

	renewKey := fmt.Sprintf("task:renew:%s", executionID)
	s.redis.Del(ctx, renewKey)

	go func() {
		if err := s.HandleTaskFailure(context.Background(), taskID, executionID, "", reason); err != nil {
			slog.Error("forceFailTask: HandleTaskFailure failed",
				"execution_id", executionID,
				"task_id", taskID,
				"error", err,
			)
		}
	}()
}

// GetExecutorByID 根据数据库 id 获取执行器信息
func (s *SchedulerService) GetExecutorByID(ctx context.Context, id int64) (*model.Executor, error) {
	query := `
		SELECT id, name, address, status, last_heartbeat, capacity, current_load, is_global, created_at, updated_at
		FROM bdopsflow_executors WHERE id = ?
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{id},
	}
	qr, err := s.DB.QueryOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	if !qr.Next() {
		return nil, fmt.Errorf("executor not found")
	}

	exec := &model.Executor{}
	if err := scanExecutorResult(&qr, exec); err != nil {
		return nil, err
	}

	return exec, nil
}

// GetExecutorByName 根据 name 获取执行器信息
func (s *SchedulerService) GetExecutorByName(ctx context.Context, name string) (*model.Executor, error) {
	query := `
		SELECT id, name, address, status, last_heartbeat, capacity, current_load, is_global, created_at, updated_at
		FROM bdopsflow_executors WHERE name = ?
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{name},
	}
	qr, err := s.DB.QueryOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	if !qr.Next() {
		return nil, fmt.Errorf("executor not found")
	}

	exec := &model.Executor{}
	if err := scanExecutorResult(&qr, exec); err != nil {
		return nil, err
	}

	return exec, nil
}

func (s *SchedulerService) HandleTaskFailure(ctx context.Context, taskID int64, failedExecutionID, output, errorMsg string) error {
	task, err := s.GetTaskByID(ctx, taskID)
	if err != nil {
		slog.Error("HandleTaskFailure: failed to get task", "task_id", taskID, "error", err)
		s.UpdateTaskStatusByID(ctx, taskID, "failed")
		s.SendWebhookNotification(ctx, taskID, failedExecutionID, "failed", output, errorMsg, 0)
		return err
	}

	executions, err := s.GetTaskExecutions(ctx, taskID)
	if err != nil {
		slog.Error("HandleTaskFailure: failed to get task executions", "task_id", taskID, "error", err)
		s.UpdateTaskStatusByID(ctx, taskID, "failed")
		s.SendWebhookNotification(ctx, taskID, failedExecutionID, "failed", output, errorMsg, 0)
		return err
	}

	maxRetries := int(task.RetryCount)
	if maxRetries <= 0 {
		maxRetries = 0
	}

	currentRetryTimes := 0
	for _, exec := range executions {
		if exec.ExecutionID == failedExecutionID {
			currentRetryTimes = int(exec.RetryTimes)
			break
		}
	}

	if currentRetryTimes < maxRetries {
		retryTimes := currentRetryTimes + 1
		slog.Info("HandleTaskFailure: scheduling retry",
			"task_id", taskID,
			"failed_execution_id", failedExecutionID,
			"retry_times", retryTimes,
			"max_retries", maxRetries,
			"retry_interval", task.RetryInterval,
		)

		go func() {
			select {
			case <-time.After(time.Duration(task.RetryInterval) * time.Second):
				slog.Info("HandleTaskFailure: executing retry",
					"task_id", taskID,
					"retry_times", retryTimes,
				)

				newExecutionID, err := s.RetryTask(ctx, taskID, retryTimes)
				if err != nil {
					slog.Error("HandleTaskFailure: retry failed",
						"task_id", taskID,
						"retry_times", retryTimes,
						"error", err,
					)
					s.UpdateTaskStatusByID(ctx, taskID, "failed")
					s.SendWebhookNotification(ctx, taskID, failedExecutionID, "failed", output, fmt.Sprintf("retry %d failed: %v", retryTimes, err), 0)
				} else {
					slog.Info("HandleTaskFailure: retry scheduled successfully",
						"task_id", taskID,
						"retry_times", retryTimes,
						"new_execution_id", newExecutionID,
					)
				}
			}
		}()
	} else {
		slog.Info("HandleTaskFailure: max retries reached, marking as failed",
			"task_id", taskID,
			"failed_execution_id", failedExecutionID,
			"retry_times", currentRetryTimes,
		)
		s.UpdateTaskStatusByID(ctx, taskID, "failed")
		s.SendWebhookNotification(ctx, taskID, failedExecutionID, "failed", output, errorMsg, 0)
	}

	return nil
}

func (s *SchedulerService) RetryTask(ctx context.Context, taskID int64, retryTimes int) (string, error) {
	task, err := s.GetTaskByID(ctx, taskID)
	if err != nil {
		return "", fmt.Errorf("get task failed: %w", err)
	}

	executionID := fmt.Sprintf("exec-%d-%d-retry-%d", taskID, time.Now().UnixNano(), retryTimes)

	lockKey := fmt.Sprintf("task:lock:%s", executionID)
	lockValue := fmt.Sprintf("%d", time.Now().UnixNano())
	lockTTL := time.Duration(task.TimeoutSeconds) * 2 * time.Second
	if lockTTL < 60*time.Second {
		lockTTL = 60 * time.Second
	}
	if lockTTL > 3600*time.Second {
		lockTTL = 3600 * time.Second
	}
	if task.TimeoutSeconds == 0 {
		lockTTL = 600 * time.Second
	}

	lockSet, err := s.redis.SetNX(ctx, lockKey, lockValue, lockTTL).Result()
	if err != nil {
		slog.Warn("acquire lock failed, continuing anyway", "error", err)
	} else if !lockSet {
		return "", fmt.Errorf("task %d is already being executed (lock conflict)", taskID)
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	var executorID interface{} = nil
	insertQuery := `
		INSERT INTO bdopsflow_task_executions (task_id, execution_id, executor_id, status, start_time, retry_times, created_at)
		VALUES (?, ?, ?, 'running', ?, ?, ?)
	`
	stmt := rqlite.ParameterizedStatement{
		Query:     insertQuery,
		Arguments: []interface{}{taskID, executionID, executorID, now, retryTimes, now},
	}
	_, err = s.DB.WriteOneParameterized(stmt)
	if err != nil {
		if lockSet {
			s.redis.Del(ctx, lockKey)
		}
		return "", fmt.Errorf("create retry execution record failed: %w", err)
	}

	slog.Info("task retry execution started",
		"task_id", taskID,
		"execution_id", executionID,
		"retry_times", retryTimes,
	)

	var executor *model.Executor
	if task.AssignedExecutorID > 0 {
		executor, err = s.GetExecutorByID(ctx, task.AssignedExecutorID)
		if err != nil || executor.Status != "online" || executor.CurrentLoad >= executor.Capacity {
			s.UpdateExecutionResult(ctx, executionID, "failed", "", "specified executor unavailable for retry")
			s.UpdateTaskStatusByID(ctx, taskID, "failed")
			s.SendWebhookNotification(ctx, taskID, executionID, "failed", "", "retry failed: specified executor unavailable", 0)
			s.redis.Del(ctx, lockKey)
			return executionID, fmt.Errorf("specified executor unavailable for retry")
		}
	} else {
		executor, err = s.SelectAvailableExecutor(ctx)
		if err != nil {
			s.UpdateExecutionResult(ctx, executionID, "failed", "", fmt.Sprintf("no available executor: %v", err))
			s.UpdateTaskStatusByID(ctx, taskID, "failed")
			s.SendWebhookNotification(ctx, taskID, executionID, "failed", "", fmt.Sprintf("retry failed: no available executor: %v", err), 0)
			s.redis.Del(ctx, lockKey)
			return executionID, fmt.Errorf("no available executor: %w", err)
		}
	}

	grpcTask := &pb.Task{
		TaskId:         taskID,
		ExecutionId:    executionID,
		Type:           task.Type,
		Config:         task.Config,
		TimeoutSeconds: task.TimeoutSeconds,
		RetryCount:     task.RetryCount,
		RetryInterval:  task.RetryInterval,
	}

	if s.dispatcher == nil {
		s.UpdateExecutionResult(ctx, executionID, "failed", "", "dispatcher not configured")
		s.UpdateTaskStatusByID(ctx, taskID, "failed")
		s.SendWebhookNotification(ctx, taskID, executionID, "failed", "", "retry failed: dispatcher not configured", 0)
		s.redis.Del(ctx, lockKey)
		return executionID, fmt.Errorf("dispatcher not configured")
	}

	updateExecutorQuery := `UPDATE bdopsflow_task_executions SET executor_id = ? WHERE execution_id = ?`
	updateExecutorStmt := rqlite.ParameterizedStatement{
		Query:     updateExecutorQuery,
		Arguments: []interface{}{executor.ID, executionID},
	}
	_, dbErr := s.DB.WriteOneParameterized(updateExecutorStmt)
	if dbErr != nil {
		slog.Warn("failed to update executor_id in bdopsflow_task_executions", "error", dbErr, "execution_id", executionID)
	}

	if err := s.dispatcher(executor.Name, grpcTask); err != nil {
		s.UpdateExecutionResult(ctx, executionID, "failed", "", fmt.Sprintf("dispatch failed: %v", err))
		s.UpdateTaskStatusByID(ctx, taskID, "failed")
		s.SendWebhookNotification(ctx, taskID, executionID, "failed", "", fmt.Sprintf("retry failed: dispatch failed: %v", err), 0)
		s.redis.Del(ctx, lockKey)
		return executionID, fmt.Errorf("dispatch failed: %w", err)
	}

	renewKey := fmt.Sprintf("task:renew:%s", executionID)
	s.redis.Set(ctx, renewKey, time.Now().Unix(), time.Duration(lockTTL)*time.Second)

	slog.Info("task retry dispatched",
		"task_id", taskID,
		"execution_id", executionID,
		"retry_times", retryTimes,
		"executor", executor.ID,
	)

	return executionID, nil
}

func (s *SchedulerService) ListExecutors(ctx context.Context) ([]*model.Executor, error) {
	query := `
		SELECT id, name, address, status, last_heartbeat, capacity, current_load, created_at, updated_at
		FROM bdopsflow_executors ORDER BY created_at DESC
	`

	qr, err := s.DB.QueryOne(query)
	if err != nil {
		return nil, err
	}

	if qr.Err != nil {
		return nil, qr.Err
	}

	var bdopsflow_executors []*model.Executor
	for qr.Next() {
		exec := &model.Executor{}
		if err := scanExecutorResult(&qr, exec); err != nil {
			return nil, err
		}
		bdopsflow_executors = append(bdopsflow_executors, exec)
	}

	return bdopsflow_executors, nil
}

func (s *SchedulerService) UpdateExecutionResult(ctx context.Context, executionID, status, output, errorMsg string) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	query := `
		UPDATE bdopsflow_task_executions
		SET status = ?, output = ?, error = ?,
		    end_time = CASE WHEN ? IN ('success', 'failed', 'timeout') THEN ? ELSE end_time END,
		    start_time = CASE WHEN start_time IS NULL OR start_time = '' THEN ? ELSE start_time END
		WHERE execution_id = ?
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{status, output, errorMsg, status, now, now, executionID},
	}

	result, err := s.DB.WriteOneParameterized(stmt)
	if err != nil {
		return err
	}

	if result.Err != nil {
		return result.Err
	}

	// 任务完成后，尝试清理锁
	// 先获取 task_id
	getTaskIDQuery := `SELECT task_id FROM bdopsflow_task_executions WHERE execution_id = ?`
	getTaskIDStmt := rqlite.ParameterizedStatement{
		Query:     getTaskIDQuery,
		Arguments: []interface{}{executionID},
	}
	taskIDQr, err := s.DB.QueryOneParameterized(getTaskIDStmt)
	if err == nil && taskIDQr.Err == nil && taskIDQr.Next() {
		row, _ := taskIDQr.Slice()
		taskID := rowInt64(row[0])
		// 清理锁
		lockKey := fmt.Sprintf("task:lock:%s", executionID)
		s.redis.Del(ctx, lockKey)
		slog.Debug("cleaned up task lock", "task_id", taskID, "execution_id", executionID)
	}

	slog.Info("task execution finished",
		"execution_id", executionID,
		"status", status,
	)

	return nil
}

func (s *SchedulerService) UpdateTaskProgress(ctx context.Context, executionID string, progress int32, progressMsg string) error {
	query := `
		UPDATE bdopsflow_task_executions
		SET progress = ?, progress_msg = ?, updated_at = ?
		WHERE execution_id = ?
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{progress, progressMsg, time.Now(), executionID},
	}

	result, err := s.DB.WriteOneParameterized(stmt)
	if err != nil {
		return err
	}
	if result.Err != nil {
		return result.Err
	}

	slog.Debug("task progress updated",
		"execution_id", executionID,
		"progress", progress,
		"message", progressMsg,
	)

	return nil
}

func (s *SchedulerService) RecoverRunningTasksOnBecomeLeader(ctx context.Context) error {
	slog.Info("recovering running tasks on becoming leader")

	query := `
		SELECT execution_id, task_id, executor_id, status, start_time, progress, progress_msg
		FROM bdopsflow_task_executions
		WHERE status = 'running'
	`

	qr, err := s.DB.QueryOne(query)
	if err != nil {
		return err
	}
	if qr.Err != nil {
		return qr.Err
	}

	recoveredCount := 0
	failedCount := 0
	validatedCount := 0

	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			continue
		}

		executionID := rowString(row[0])
		taskID := rowInt64(row[1])
		executorID := rowInt64(row[2])
		startTimeStr := rowString(row[4])
		progress := int32(rowInt(row[5])) // 使用 rowInt 并转换为 int32
		progressMsg := rowString(row[6])

		slog.Debug("recovering running task",
			"execution_id", executionID,
			"task_id", taskID,
			"executor_id", executorID,
			"progress", progress,
		)

		// 检查执行器是否还在线且心跳正常
		executor, err := s.GetExecutorByID(ctx, executorID)
		executorOnline := err == nil && executor.Status == "online"

		// 检查任务锁是否还存在
		lockKey := fmt.Sprintf("task:lock:%s", executionID)
		lockExists, _ := s.redis.Exists(ctx, lockKey).Result()

		// 检查任务是否已经超时
		taskTimeout := false
		if startTimeStr != "" {
			if startTime, err := time.Parse("2006-01-02 15:04:05", startTimeStr); err == nil {
				// 如果任务超过2小时，认为它可能已经卡死
				if time.Since(startTime) > 2*time.Hour {
					taskTimeout = true
				}
			}
		}

		// 如果执行器离线、锁不存在，或者任务超时，标记任务失败
		if !executorOnline || lockExists == 0 || taskTimeout {
			slog.Warn("task recovery: marking task as failed",
				"execution_id", executionID,
				"task_id", taskID,
				"executor_id", executorID,
				"executor_online", executorOnline,
				"lock_exists", lockExists,
				"task_timeout", taskTimeout,
			)
			
			var reason string
			if !executorOnline {
				reason = "scheduler failover: executor is offline"
			} else if lockExists == 0 {
				reason = "scheduler failover: task lock not found"
			} else {
				reason = "scheduler failover: task execution timeout"
			}
			
			s.forceFailTask(ctx, executionID, taskID, reason)
			failedCount++
			continue
		}

		// 任务看起来还在正常运行，更新任务锁和相关状态
		lockTTL := 300 // 5分钟
		if err := s.redis.Set(ctx, lockKey, "leader_recovered", time.Duration(lockTTL)*time.Second).Err(); err != nil {
			slog.Warn("failed to set task lock during recovery", "execution_id", executionID, "error", err)
		}

		// 更新 renew 时间戳
		renewKey := fmt.Sprintf("task:renew:%s", executionID)
		if err := s.redis.Set(ctx, renewKey, time.Now().Unix(), time.Duration(lockTTL)*time.Second).Err(); err != nil {
			slog.Warn("failed to set task renew timestamp during recovery", "execution_id", executionID, "error", err)
		}

		// 清理连续失败计数器
		failCountKey := fmt.Sprintf("task:renew:fail:count:%s", executionID)
		s.redis.Del(ctx, failCountKey)

		// 记录恢复事件（使用去重机制）
		s.addRecoveryLogSafe(ctx, executionID, taskID, "info", 
			fmt.Sprintf("Task recovered by new leader, progress: %d%%, message: %s", progress, progressMsg))

		recoveredCount++
		validatedCount++
	}

	slog.Info("finished recovering running tasks",
		"recovered_count", recoveredCount,
		"failed_count", failedCount,
		"validated_count", validatedCount,
	)
	return nil
}

func (s *SchedulerService) GetTaskExecutions(ctx context.Context, taskID int64) ([]*model.TaskExecution, error) {
	query := `
		SELECT id, task_id, execution_id, executor_id, status, start_time, end_time,
		       output, error, retry_times, created_at, progress, progress_msg, updated_at
		FROM bdopsflow_task_executions
		WHERE task_id = ?
		ORDER BY created_at DESC
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{taskID},
	}
	qr, err := s.DB.QueryOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	var executions []*model.TaskExecution
	for qr.Next() {
		exec := &model.TaskExecution{}
		if err := scanExecutionResult(&qr, exec); err != nil {
			return nil, err
		}
		executions = append(executions, exec)
	}

	return executions, nil
}

// GetTaskInfoByID 根据任务ID获取任务信息
func (s *SchedulerService) GetTaskInfoByID(ctx context.Context, taskID int64) (*model.Task, error) {
	return s.GetTaskByID(ctx, taskID)
}

// GetExecutorInfoByID 根据执行器数据库ID获取执行器信息
func (s *SchedulerService) GetExecutorInfoByID(ctx context.Context, id int64) (*model.Executor, error) {
	return s.GetExecutorByID(ctx, id)
}

// TaskExecutionWithNames 包含任务名和执行器名的执行记录
type TaskExecutionWithNames struct {
	model.TaskExecution
	TaskName     string
	TaskType     string
	ExecutorName string
}

// GetAllExecutions 获取所有执行记录，支持筛选和分页
func (s *SchedulerService) GetAllExecutions(ctx context.Context, domainID int64, role string, filter map[string]string, page int, pageSize int) ([]*TaskExecutionWithNames, int, error) {
	// 构建 WHERE 条件
	whereClause := " WHERE 1=1"
	var args []interface{}
	
	// 应用领域隔离
	isSystemAdmin := role == "system_admin" || role == "admin"
	if !isSystemAdmin {
		whereClause += " AND t.domain_id = ?"
		args = append(args, domainID)
	}

	// 应用筛选条件
	if filter["id"] != "" {
		if id, err := strconv.ParseInt(filter["id"], 10, 64); err == nil {
			whereClause += " AND te.id = ?"
			args = append(args, id)
		}
	}
	if filter["execution_id"] != "" {
		whereClause += " AND te.execution_id LIKE ?"
		args = append(args, "%"+filter["execution_id"]+"%")
	}
	if filter["executor_name"] != "" {
		whereClause += " AND e.name LIKE ?"
		args = append(args, "%"+filter["executor_name"]+"%")
	}
	if filter["task_name"] != "" {
		whereClause += " AND t.name LIKE ?"
		args = append(args, "%"+filter["task_name"]+"%")
	}
	if filter["status"] != "" {
		whereClause += " AND te.status = ?"
		args = append(args, filter["status"])
	}
	if filter["start_time_from"] != "" {
		whereClause += " AND te.start_time >= ?"
		args = append(args, filter["start_time_from"])
	}
	if filter["start_time_to"] != "" {
		whereClause += " AND te.start_time <= ?"
		args = append(args, filter["start_time_to"])
	}
	if filter["end_time_from"] != "" {
		whereClause += " AND te.end_time >= ?"
		args = append(args, filter["end_time_from"])
	}
	if filter["end_time_to"] != "" {
		whereClause += " AND te.end_time <= ?"
		args = append(args, filter["end_time_to"])
	}
	if filter["duration_min"] != "" || filter["duration_max"] != "" {
		whereClause += " AND te.end_time IS NOT NULL"
		if filter["duration_min"] != "" {
			if duration, err := strconv.ParseFloat(filter["duration_min"], 64); err == nil {
				durationSecs := int64(duration)
				whereClause += " AND (STRFTIME('%s', te.end_time) - STRFTIME('%s', te.start_time)) >= ?"
				args = append(args, durationSecs)
			}
		}
		if filter["duration_max"] != "" {
			if duration, err := strconv.ParseFloat(filter["duration_max"], 64); err == nil {
				durationSecs := int64(duration)
				whereClause += " AND (STRFTIME('%s', te.end_time) - STRFTIME('%s', te.start_time)) <= ?"
				args = append(args, durationSecs)
			}
		}
	}

	// 统一使用 JOIN，简化逻辑
	joinClause := `
		FROM bdopsflow_task_executions te
		LEFT JOIN bdopsflow_tasks t ON te.task_id = t.id
		LEFT JOIN bdopsflow_executors e ON te.executor_id = e.id
	`

	// 1. 先获取总数
	countQuery := "SELECT COUNT(*) " + joinClause + whereClause
	var countQr rqlite.QueryResult
	var err error

	if len(args) > 0 {
		countStmt := rqlite.ParameterizedStatement{
			Query:     countQuery,
			Arguments: args,
		}
		countQr, err = s.DB.QueryOneParameterized(countStmt)
	} else {
		countQr, err = s.DB.QueryOne(countQuery)
	}

	if err != nil {
		slog.Error("GetAllExecutions: count query failed", "error", err)
		return nil, 0, err
	}
	if countQr.Err != nil {
		slog.Error("GetAllExecutions: count query returned error", "error", countQr.Err)
		return nil, 0, countQr.Err
	}

	var total int
	if countQr.Next() {
		row, err := countQr.Slice()
		if err == nil {
			total = int(rowInt64(row[0]))
		}
	}

	// 2. 获取分页数据，同时 JOIN 出 task_name, task_type, executor_name
	offset := (page - 1) * pageSize
	dataQuery := `
		SELECT te.id, te.task_id, te.execution_id, te.executor_id, te.status, te.start_time, te.end_time,
		       te.output, te.error, te.retry_times, te.created_at,
		       t.name, t.type, e.name
	` + joinClause + whereClause + " ORDER BY te.created_at DESC LIMIT ? OFFSET ?"

	// 复制筛选参数给数据查询
	dataArgs := make([]interface{}, len(args))
	copy(dataArgs, args)
	dataArgs = append(dataArgs, pageSize, offset)

	slog.Debug("GetAllExecutions: fetching records", "page", page, "pageSize", pageSize)

	var dataQr rqlite.QueryResult
	if len(dataArgs) > 0 {
		dataStmt := rqlite.ParameterizedStatement{
			Query:     dataQuery,
			Arguments: dataArgs,
		}
		dataQr, err = s.DB.QueryOneParameterized(dataStmt)
	} else {
		dataQr, err = s.DB.QueryOne(dataQuery)
	}

	if err != nil {
		slog.Error("GetAllExecutions: data query failed", "error", err)
		return nil, 0, err
	}
	if dataQr.Err != nil {
		slog.Error("GetAllExecutions: data query returned error", "error", dataQr.Err)
		return nil, 0, dataQr.Err
	}

	var executions []*TaskExecutionWithNames
	for dataQr.Next() {
		exec := &TaskExecutionWithNames{}
		row, err := dataQr.Slice()
		if err != nil {
			slog.Error("GetAllExecutions: slice failed", "error", err)
			return nil, 0, err
		}

		// 处理基本字段
		exec.ID = rowInt64(row[0])
		exec.TaskID = rowInt64(row[1])
		exec.ExecutionID = rowString(row[2])
		exec.ExecutorID = rowInt64(row[3])
		exec.Status = rowString(row[4])

		// 处理 start_time
		if t, ok := row[5].(time.Time); ok {
			exec.StartTime = rqlite.NullTime{Time: t, Valid: true}
		} else if s, ok := row[5].(string); ok && s != "" {
			parsed, err := time.Parse("2006-01-02 15:04:05", s)
			if err == nil {
				exec.StartTime = rqlite.NullTime{Time: parsed, Valid: true}
			}
		}

		// 处理 end_time
		if t, ok := row[6].(time.Time); ok {
			exec.EndTime = rqlite.NullTime{Time: t, Valid: true}
		} else if s, ok := row[6].(string); ok && s != "" {
			parsed, err := time.Parse("2006-01-02 15:04:05", s)
			if err == nil {
				exec.EndTime = rqlite.NullTime{Time: parsed, Valid: true}
			}
		}

		exec.Output = rowString(row[7])
		exec.Error = rowString(row[8])
		exec.RetryTimes = int32(rowInt64(row[9]))

		// 处理 created_at
		if t, ok := row[10].(time.Time); ok {
			exec.CreatedAt = t
		} else if s, ok := row[10].(string); ok && s != "" {
			parsed, err := time.Parse("2006-01-02 15:04:05", s)
			if err == nil {
				exec.CreatedAt = parsed
			}
		}

		// 处理额外的 JOIN 字段
		exec.TaskName = rowString(row[11])
		exec.TaskType = rowString(row[12])
		exec.ExecutorName = rowString(row[13])

		executions = append(executions, exec)
	}

	slog.Debug("GetAllExecutions: completed", "total", total, "returned", len(executions))
	return executions, total, nil
}

// DeleteExecution 删除指定执行记录及其相关日志
func (s *SchedulerService) DeleteExecution(ctx context.Context, id int64) error {
	// 先获取执行记录，以便获取 execution_id 来删除日志
	query := "SELECT execution_id FROM bdopsflow_task_executions WHERE id = ?"
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{id},
	}
	qr, err := s.DB.QueryOneParameterized(stmt)
	if err != nil {
		return err
	}
	if qr.Err != nil {
		return qr.Err
	}

	var executionID string
	if qr.Next() {
		row, err := qr.Slice()
		if err == nil {
			executionID = rowString(row[0])
		}
	} else {
		return fmt.Errorf("execution not found")
	}

	// 删除相关日志
	deleteLogsQuery := "DELETE FROM bdopsflow_task_logs WHERE execution_id = ?"
	deleteLogsStmt := rqlite.ParameterizedStatement{
		Query:     deleteLogsQuery,
		Arguments: []interface{}{executionID},
	}
	_, err = s.DB.WriteOneParameterized(deleteLogsStmt)
	if err != nil {
		slog.Warn("failed to delete related logs", "error", err)
	}

	// 删除执行记录
	deleteExecQuery := "DELETE FROM bdopsflow_task_executions WHERE id = ?"
	deleteExecStmt := rqlite.ParameterizedStatement{
		Query:     deleteExecQuery,
		Arguments: []interface{}{id},
	}
	_, err = s.DB.WriteOneParameterized(deleteExecStmt)
	return err
}

// BatchDeleteExecutions 批量删除执行记录及其相关日志
func (s *SchedulerService) BatchDeleteExecutions(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}

	// 构建查询参数占位符
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	// 先获取所有 execution_ids
	query := "SELECT execution_id FROM bdopsflow_task_executions WHERE id IN (" + strings.Join(placeholders, ",") + ")"
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: args,
	}
	qr, err := s.DB.QueryOneParameterized(stmt)
	if err != nil {
		return err
	}
	if qr.Err != nil {
		return qr.Err
	}

	var executionIDs []string
	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			continue
		}
		eid := rowString(row[0])
		executionIDs = append(executionIDs, eid)
	}

	// 删除相关日志
	if len(executionIDs) > 0 {
		logPlaceholders := make([]string, len(executionIDs))
		logArgs := make([]interface{}, len(executionIDs))
		for i, eid := range executionIDs {
			logPlaceholders[i] = "?"
			logArgs[i] = eid
		}

		deleteLogsQuery := "DELETE FROM bdopsflow_task_logs WHERE execution_id IN (" + strings.Join(logPlaceholders, ",") + ")"
		deleteLogsStmt := rqlite.ParameterizedStatement{
			Query:     deleteLogsQuery,
			Arguments: logArgs,
		}
		_, err = s.DB.WriteOneParameterized(deleteLogsStmt)
		if err != nil {
			slog.Warn("failed to delete related logs", "error", err)
		}
	}

	// 删除执行记录
	deleteExecQuery := "DELETE FROM bdopsflow_task_executions WHERE id IN (" + strings.Join(placeholders, ",") + ")"
	deleteExecStmt := rqlite.ParameterizedStatement{
		Query:     deleteExecQuery,
		Arguments: args,
	}
	_, err = s.DB.WriteOneParameterized(deleteExecStmt)
	return err
}

// DeleteExecutionWithDomainCheck 删除指定执行记录，先验证领域权限
func (s *SchedulerService) DeleteExecutionWithDomainCheck(ctx context.Context, id int64, domainID int64, role string) error {
	isSystemAdmin := role == "system_admin" || role == "admin"
	
	// 先验证领域权限
	checkQuery := `
		SELECT te.execution_id 
		FROM bdopsflow_task_executions te
		LEFT JOIN bdopsflow_tasks t ON te.task_id = t.id
		WHERE te.id = ?
	`
	checkArgs := []interface{}{id}
	
	if !isSystemAdmin {
		checkQuery += " AND t.domain_id = ?"
		checkArgs = append(checkArgs, domainID)
	}
	
	checkStmt := rqlite.ParameterizedStatement{
		Query:     checkQuery,
		Arguments: checkArgs,
	}
	
	qr, err := s.DB.QueryOneParameterized(checkStmt)
	if err != nil {
		return err
	}
	if qr.Err != nil {
		return qr.Err
	}
	
	var executionID string
	if qr.Next() {
		row, err := qr.Slice()
		if err == nil {
			executionID = rowString(row[0])
		}
	} else {
		return fmt.Errorf("execution not found or permission denied")
	}
	
	// 删除相关日志
	if executionID != "" {
		deleteLogsQuery := "DELETE FROM bdopsflow_task_logs WHERE execution_id = ?"
		deleteLogsStmt := rqlite.ParameterizedStatement{
			Query:     deleteLogsQuery,
			Arguments: []interface{}{executionID},
		}
		_, _ = s.DB.WriteOneParameterized(deleteLogsStmt)
	}
	
	// 删除执行记录
	deleteExecQuery := "DELETE FROM bdopsflow_task_executions WHERE id = ?"
	deleteExecStmt := rqlite.ParameterizedStatement{
		Query:     deleteExecQuery,
		Arguments: []interface{}{id},
	}
	_, err = s.DB.WriteOneParameterized(deleteExecStmt)
	return err
}

// BatchDeleteExecutionsWithDomainCheck 批量删除执行记录，先验证领域权限
func (s *SchedulerService) BatchDeleteExecutionsWithDomainCheck(ctx context.Context, ids []int64, domainID int64, role string) error {
	if len(ids) == 0 {
		return nil
	}
	
	isSystemAdmin := role == "system_admin" || role == "admin"
	
	// 构建查询参数占位符
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	
	// 先验证领域权限并获取 execution_ids
	checkQuery := `
		SELECT te.execution_id 
		FROM bdopsflow_task_executions te
		LEFT JOIN bdopsflow_tasks t ON te.task_id = t.id
		WHERE te.id IN (` + strings.Join(placeholders, ",") + `)
	`
	
	if !isSystemAdmin {
		checkQuery += " AND t.domain_id = ?"
		args = append(args, domainID)
	}
	
	checkStmt := rqlite.ParameterizedStatement{
		Query:     checkQuery,
		Arguments: args,
	}
	
	qr, err := s.DB.QueryOneParameterized(checkStmt)
	if err != nil {
		return err
	}
	if qr.Err != nil {
		return qr.Err
	}
	
	var executionIDs []string
	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			continue
		}
		eid := rowString(row[0])
		executionIDs = append(executionIDs, eid)
	}
	
	// 删除相关日志
	if len(executionIDs) > 0 {
		logPlaceholders := make([]string, len(executionIDs))
		logArgs := make([]interface{}, len(executionIDs))
		for i, eid := range executionIDs {
			logPlaceholders[i] = "?"
			logArgs[i] = eid
		}
		
		deleteLogsQuery := "DELETE FROM bdopsflow_task_logs WHERE execution_id IN (" + strings.Join(logPlaceholders, ",") + ")"
		deleteLogsStmt := rqlite.ParameterizedStatement{
			Query:     deleteLogsQuery,
			Arguments: logArgs,
		}
		_, _ = s.DB.WriteOneParameterized(deleteLogsStmt)
	}
	
	// 删除执行记录
	deleteArgs := make([]interface{}, len(ids))
	for i, id := range ids {
		deleteArgs[i] = id
	}
	
	deleteExecQuery := "DELETE FROM bdopsflow_task_executions WHERE id IN (" + strings.Join(placeholders, ",") + ")"
	deleteExecStmt := rqlite.ParameterizedStatement{
		Query:     deleteExecQuery,
		Arguments: deleteArgs,
	}
	
	if !isSystemAdmin {
		// 再次应用领域权限
		deleteExecQuery = `
			DELETE FROM bdopsflow_task_executions 
			WHERE id IN (` + strings.Join(placeholders, ",") + `)
			AND task_id IN (SELECT id FROM bdopsflow_tasks WHERE domain_id = ?)
		`
		deleteArgs = append(deleteArgs, domainID)
	}
	
	deleteExecStmt = rqlite.ParameterizedStatement{
		Query:     deleteExecQuery,
		Arguments: deleteArgs,
	}
	
	_, err = s.DB.WriteOneParameterized(deleteExecStmt)
	return err
}

func (s *SchedulerService) GetTaskLogs(ctx context.Context, executionID string) ([]*model.TaskLog, error) {
	query := `
		SELECT id, execution_id, task_id, executor_id, node_id, log_level, message, log_time
		FROM bdopsflow_task_logs WHERE execution_id = ?
		ORDER BY log_time ASC
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{executionID},
	}
	qr, err := s.DB.QueryOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	var logs []*model.TaskLog
	for qr.Next() {
		tl := &model.TaskLog{}
		if err := scanTaskLogResult(&qr, tl); err != nil {
			return nil, err
		}
		logs = append(logs, tl)
	}

	return logs, nil
}

// addRecoveryLogSafe 专门用于添加恢复日志，自带去重机制
func (s *SchedulerService) addRecoveryLogSafe(ctx context.Context, executionID string, taskID int64, logLevel string, message string) error {
	// 使用 Redis 来检查是否已经记录过类似的恢复日志
	dedupKey := fmt.Sprintf("task:log:dedup:%s:recovery", executionID)
	exists, err := s.redis.Exists(ctx, dedupKey).Result()
	if err == nil && exists > 0 {
		// 已经记录过恢复日志，跳过
		slog.Debug("Skipping duplicate recovery log", "execution_id", executionID)
		return nil
	}
	
	// 设置去重标记，有效期1小时
	s.redis.Set(ctx, dedupKey, "1", time.Hour)
	
	return s.AddTaskLog(ctx, executionID, taskID, "", logLevel, message)
}

func (s *SchedulerService) AddTaskLog(ctx context.Context, executionID string, taskID int64, nodeID string, logLevel string, message string) error {
	// 首先获取执行记录中的 executor_id
	var executorID interface{} = nil
	execQuery := `SELECT executor_id FROM bdopsflow_task_executions WHERE execution_id = ? LIMIT 1`
	execStmt := rqlite.ParameterizedStatement{
		Query:     execQuery,
		Arguments: []interface{}{executionID},
	}
	execQr, err := s.DB.QueryOneParameterized(execStmt)
	if err == nil && execQr.Err == nil && execQr.Next() {
		row, _ := execQr.Slice()
		rawID := rowInt64(row[0])
		if rawID > 0 {
			executorID = rawID
		}
	}

	// 实现简单的去重机制：避免短时间内相同的日志重复记录
	dedupEnabled := true
	if dedupEnabled && s.redis != nil {
		// 生成日志的唯一标识
		logHash := fmt.Sprintf("%x", []byte(fmt.Sprintf("%s-%s-%s-%s", executionID, nodeID, logLevel, message)))
		dedupKey := fmt.Sprintf("task:log:dedup:%s", logHash)
		
		// 检查是否已经记录过
		exists, _ := s.redis.Exists(ctx, dedupKey).Result()
		if exists > 0 {
			// 已经记录过相同的日志，跳过
			slog.Debug("Skipping duplicate task log", 
				"execution_id", executionID, 
				"log_level", logLevel)
			return nil
		}
		
		// 设置去重标记，有效期30秒
		s.redis.Set(ctx, dedupKey, "1", 30*time.Second)
	}

	// 尝试插入带 executor_id 的新表结构
	query := `
		INSERT INTO bdopsflow_task_logs (execution_id, task_id, executor_id, node_id, log_level, message, log_time)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	now := time.Now().Format("2006-01-02 15:04:05")
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{executionID, taskID, executorID, nodeID, logLevel, message, now},
	}
	result, err := s.DB.WriteOneParameterized(stmt)
	
	// 如果失败，回退到旧表结构
	if err != nil || result.Err != nil {
		slog.Debug("Falling back to old insert format for bdopsflow_task_logs")
		fallbackQuery := `
			INSERT INTO bdopsflow_task_logs (execution_id, task_id, node_id, log_level, message, log_time)
			VALUES (?, ?, ?, ?, ?, ?)
		`
		fallbackStmt := rqlite.ParameterizedStatement{
			Query:     fallbackQuery,
			Arguments: []interface{}{executionID, taskID, nodeID, logLevel, message, now},
		}
		result, err = s.DB.WriteOneParameterized(fallbackStmt)
		if err != nil {
			return err
		}
		if result.Err != nil {
			return result.Err
		}
	}

	// 如果是 stdout 或 stderr 日志，也更新对应的 execution 字段
	if logLevel == "stdout" || logLevel == "stderr" {
		updateQuery := `
			UPDATE bdopsflow_task_executions 
			SET `
		if logLevel == "stdout" {
			updateQuery += `output = COALESCE(output, '') || ?`
		} else {
			updateQuery += `error = COALESCE(error, '') || ?`
		}
		updateQuery += ` WHERE execution_id = ?`

		updateStmt := rqlite.ParameterizedStatement{
			Query:     updateQuery,
			Arguments: []interface{}{message, executionID},
		}
		result, err := s.DB.WriteOneParameterized(updateStmt)
		if err != nil {
			// 日志记录已成功，更新字段失败不影响
			slog.Warn("failed to update execution output/error", "error", err, "execution_id", executionID)
		} else if result.Err != nil {
			slog.Warn("failed to update execution output/error", "error", result.Err, "execution_id", executionID)
		}
	}

	return nil
}

// Workflow 相关
func (s *SchedulerService) GetWorkflow(ctx context.Context, id int64) (*model.Workflow, error) {
	query := `
		SELECT id, name, description, domain_id, dag_config, cron_expression,
		       is_enabled, created_by, created_at, updated_at
		FROM bdopsflow_workflows WHERE id = ?
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{id},
	}
	qr, err := s.DB.QueryOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	if !qr.Next() {
		return nil, fmt.Errorf("workflow not found")
	}

	wf := &model.Workflow{}
	if err := scanWorkflowResult(&qr, wf); err != nil {
		return nil, err
	}

	return wf, nil
}

func (s *SchedulerService) ListWorkflows(ctx context.Context, domainID int64, role string) ([]*model.Workflow, error) {
	var query string
	var args []interface{}

	isSystemAdmin := role == "system_admin" || role == "admin"

	if isSystemAdmin {
		query = `
			SELECT id, name, description, domain_id, dag_config, cron_expression,
			       is_enabled, created_by, created_at, updated_at
			FROM bdopsflow_workflows ORDER BY created_at DESC
		`
	} else {
		query = `
			SELECT id, name, description, domain_id, dag_config, cron_expression,
			       is_enabled, created_by, created_at, updated_at
			FROM bdopsflow_workflows WHERE domain_id = ? ORDER BY created_at DESC
		`
		args = append(args, domainID)
	}

	var qr rqlite.QueryResult
	var err error
	if len(args) > 0 {
		stmt := rqlite.ParameterizedStatement{
			Query:     query,
			Arguments: args,
		}
		qr, err = s.DB.QueryOneParameterized(stmt)
	} else {
		qr, err = s.DB.QueryOne(query)
	}

	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	var bdopsflow_workflows []*model.Workflow
	for qr.Next() {
		wf := &model.Workflow{}
		if err := scanWorkflowResult(&qr, wf); err != nil {
			return nil, err
		}
		bdopsflow_workflows = append(bdopsflow_workflows, wf)
	}

	return bdopsflow_workflows, nil
}

func (s *SchedulerService) CreateWorkflow(ctx context.Context, query string, args ...interface{}) (*model.Workflow, error) {
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: args,
	}
	result, err := s.DB.WriteOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if result.Err != nil {
		return nil, result.Err
	}

	id := result.LastInsertID
	return s.GetWorkflow(ctx, id)
}

func (s *SchedulerService) UpdateWorkflow(ctx context.Context, id int64, wf *model.Workflow) error {
	query := `
		UPDATE bdopsflow_workflows SET name = ?, description = ?, dag_config = ?, cron_expression = ?,
		                    is_enabled = ?, updated_at = ?
		WHERE id = ?
	`

	stmt := rqlite.ParameterizedStatement{
		Query: query,
		Arguments: []interface{}{
			wf.Name, wf.Description, wf.DAGConfig,
			wf.CronExpression, wf.IsEnabled, time.Now().Format("2006-01-02 15:04:05"), id,
		},
	}

	result, err := s.DB.WriteOneParameterized(stmt)
	if err != nil {
		return err
	}
	if result.Err != nil {
		return result.Err
	}

	return nil
}

func (s *SchedulerService) DeleteWorkflow(ctx context.Context, id int64) error {
	query := `DELETE FROM bdopsflow_workflows WHERE id = ?`
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{id},
	}
	result, err := s.DB.WriteOneParameterized(stmt)
	if err != nil {
		return err
	}
	if result.Err != nil {
		return result.Err
	}
	return nil
}

func (s *SchedulerService) CreateWorkflowExecution(ctx context.Context, workflowID int64) (*model.WorkflowExecution, error) {
	executionID := fmt.Sprintf("wf-%d-%d", workflowID, time.Now().UnixNano())
	nodeStates := "{}"

	query := `
		INSERT INTO bdopsflow_workflow_executions (workflow_id, execution_id, status, node_states, created_at)
		VALUES (?, ?, 'pending', ?, ?)
	`

	now := time.Now().Format("2006-01-02 15:04:05")
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{workflowID, executionID, nodeStates, now},
	}
	result, err := s.DB.WriteOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if result.Err != nil {
		return nil, result.Err
	}

	id := result.LastInsertID
	return s.GetWorkflowExecution(ctx, id)
}

func (s *SchedulerService) GetWorkflowExecution(ctx context.Context, id int64) (*model.WorkflowExecution, error) {
	query := `
		SELECT id, workflow_id, execution_id, status, start_time, end_time, node_states, created_at
		FROM bdopsflow_workflow_executions WHERE id = ?
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{id},
	}
	qr, err := s.DB.QueryOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	if !qr.Next() {
		return nil, fmt.Errorf("workflow execution not found")
	}

	we := &model.WorkflowExecution{}
	if err := scanWorkflowExecutionResult(&qr, we); err != nil {
		return nil, err
	}

	return we, nil
}

func (s *SchedulerService) GetWorkflowExecutionByExecutionID(ctx context.Context, executionID string) (*model.WorkflowExecution, error) {
	query := `
		SELECT id, workflow_id, execution_id, status, start_time, end_time, node_states, created_at
		FROM bdopsflow_workflow_executions WHERE execution_id = ?
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{executionID},
	}
	qr, err := s.DB.QueryOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	if !qr.Next() {
		return nil, fmt.Errorf("workflow execution not found")
	}

	we := &model.WorkflowExecution{}
	if err := scanWorkflowExecutionResult(&qr, we); err != nil {
		return nil, err
	}

	return we, nil
}

func (s *SchedulerService) ListWorkflowExecutions(ctx context.Context, workflowID int64) ([]*model.WorkflowExecution, error) {
	query := `
		SELECT id, workflow_id, execution_id, status, start_time, end_time, node_states, created_at
		FROM bdopsflow_workflow_executions WHERE workflow_id = ?
		ORDER BY created_at DESC
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{workflowID},
	}
	qr, err := s.DB.QueryOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	var executions []*model.WorkflowExecution
	for qr.Next() {
		we := &model.WorkflowExecution{}
		if err := scanWorkflowExecutionResult(&qr, we); err != nil {
			return nil, err
		}
		executions = append(executions, we)
	}

	return executions, nil
}

func (s *SchedulerService) UpdateWorkflowExecutionStatus(ctx context.Context, executionID string, status string) error {
	query := `
		UPDATE bdopsflow_workflow_executions
		SET status = ?, 
		    start_time = CASE WHEN start_time IS NULL OR start_time = '' THEN ? ELSE start_time END,
		    end_time = CASE WHEN ? IN ('success', 'failed') THEN ? ELSE end_time END
		WHERE execution_id = ?
	`

	now := time.Now().Format("2006-01-02 15:04:05")
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{status, now, status, now, executionID},
	}
	result, err := s.DB.WriteOneParameterized(stmt)
	if err != nil {
		return err
	}
	if result.Err != nil {
		return result.Err
	}

	return nil
}

func (s *SchedulerService) UpdateWorkflowExecutionNodeStates(ctx context.Context, executionID string, nodeStates string) error {
	query := `UPDATE bdopsflow_workflow_executions SET node_states = ? WHERE execution_id = ?`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{nodeStates, executionID},
	}
	result, err := s.DB.WriteOneParameterized(stmt)
	if err != nil {
		return err
	}
	if result.Err != nil {
		return result.Err
	}

	return nil
}

func (s *SchedulerService) GetTaskLogsByExecutionID(ctx context.Context, executionID string) ([]*model.TaskLog, error) {
	return s.GetTaskLogs(ctx, executionID)
}

func (s *SchedulerService) TriggerWorkflow(ctx context.Context, workflowID int64) (*model.WorkflowExecution, error) {
	wf, err := s.GetWorkflow(ctx, workflowID)
	if err != nil {
		return nil, err
	}

	we, err := s.CreateWorkflowExecution(ctx, workflowID)
	if err != nil {
		return nil, err
	}

	dagConfig, err := dag.ParseDAGConfig(wf.DAGConfig)
	if err != nil {
		return nil, fmt.Errorf("parse dag config failed: %w", err)
	}

	validator := dag.NewDAGValidator(*dagConfig)
	topoOrder, err := validator.TopologicalSort()
	if err != nil {
		return nil, fmt.Errorf("topological sort failed: %w", err)
	}

	nodeStates := make(map[string]string)
	for _, node := range dagConfig.Nodes {
		nodeStates[node.ID] = "pending"
	}
	nodeStatesJSON, _ := json.Marshal(nodeStates)
	s.UpdateWorkflowExecutionNodeStates(ctx, we.ExecutionID, string(nodeStatesJSON))
	s.UpdateWorkflowExecutionStatus(ctx, we.ExecutionID, "running")

	go s.runWorkflowAsync(ctx, we.ExecutionID, workflowID, dagConfig, topoOrder)

	return we, nil
}

func (s *SchedulerService) runWorkflowAsync(ctx context.Context, executionID string, workflowID int64, dagConfig *dag.DAGConfig, topoOrder []string) {
	s.AddTaskLog(ctx, executionID, 0, "", "info", "Workflow execution started")

	nodeStates := make(map[string]string)
	for _, node := range dagConfig.Nodes {
		nodeStates[node.ID] = "pending"
	}

	for _, nodeID := range topoOrder {
		var node *dag.DAGNode
		for i := range dagConfig.Nodes {
			if dagConfig.Nodes[i].ID == nodeID {
				node = &dagConfig.Nodes[i]
				break
			}
		}
		if node == nil {
			continue
		}

		nodeStates[nodeID] = "running"
		nodeStatesJSON, err := json.Marshal(nodeStates)
		if err != nil {
			slog.Error("failed to marshal node states", "error", err, "node_id", nodeID)
		} else {
			s.UpdateWorkflowExecutionNodeStates(ctx, executionID, string(nodeStatesJSON))
		}
		s.AddTaskLog(ctx, executionID, 0, nodeID, "info", fmt.Sprintf("Node %s started", node.Name))

		time.Sleep(1 * time.Second)

		nodeStates[nodeID] = "success"
		nodeStatesJSON, err = json.Marshal(nodeStates)
		if err != nil {
			slog.Error("failed to marshal node states", "error", err, "node_id", nodeID)
		} else {
			s.UpdateWorkflowExecutionNodeStates(ctx, executionID, string(nodeStatesJSON))
		}
		s.AddTaskLog(ctx, executionID, 0, nodeID, "info", fmt.Sprintf("Node %s completed", node.Name))
	}

	s.UpdateWorkflowExecutionStatus(ctx, executionID, "success")
	s.AddTaskLog(ctx, executionID, 0, "", "info", "Workflow execution completed successfully")
}

func scanTaskResult(qr *rqlite.QueryResult, task *model.Task) error {
	row, err := qr.Slice()
	if err != nil {
		return err
	}

	task.ID = rowInt64(row[0])
	if v := rowInt64(row[1]); v > 0 {
		task.WorkflowID = &v
	}
	task.Name = rowString(row[2])
	task.Type = rowString(row[3])
	task.Config = rowString(row[4])
	task.CronExpression = rowString(row[5])
	task.TimeoutSeconds = int32(rowInt64(row[6]))
	task.RetryCount = int32(rowInt64(row[7]))
	task.RetryInterval = int32(rowInt64(row[8]))
	task.IsEnabled = rowBool(row[9])
	task.Status = rowString(row[10])
	task.DomainID = rowInt64(row[11])
	if !isEmpty(row[12]) {
		webhookID := rowInt64(row[12])
		task.WebhookID = &webhookID
	}
	task.WebhookEvents = rowString(row[13])
	task.AssignedExecutorID = rowInt64(row[14])
	task.CreatedBy = rowInt64(row[15])
	if t, ok := row[16].(time.Time); ok {
		task.CreatedAt = t
	}
	if t, ok := row[17].(time.Time); ok {
		task.UpdatedAt = t
	}
	return nil
}

func scanWorkflowResult(qr *rqlite.QueryResult, wf *model.Workflow) error {
	row, err := qr.Slice()
	if err != nil {
		return err
	}
	
	wf.ID = rowInt64(row[0])
	wf.Name = rowString(row[1])
	wf.Description = rowString(row[2])
	wf.DomainID = rowInt64(row[3])
	wf.DAGConfig = rowString(row[4])
	wf.CronExpression = rowString(row[5])
	wf.IsEnabled = rowBool(row[6])
	
	if v := rowInt64(row[7]); v > 0 {
		wf.CreatedBy = &v
	}
	
	if t, ok := row[8].(time.Time); ok {
		wf.CreatedAt = t
	}
	if t, ok := row[9].(time.Time); ok {
		wf.UpdatedAt = t
	}
	return nil
}

func scanExecutorResult(qr *rqlite.QueryResult, exec *model.Executor) error {
	row, err := qr.Slice()
	if err != nil {
		return err
	}
	exec.ID = rowInt64(row[0])
	exec.Name = rowString(row[1])
	exec.Address = rowString(row[2])
	exec.Status = rowString(row[3])
	if t, ok := row[4].(time.Time); ok {
		exec.LastHeartbeat = rqlite.NullTime{Time: t, Valid: true}
	}
	exec.Capacity = rowInt64(row[5])
	exec.CurrentLoad = rowInt64(row[6])
	if t, ok := row[7].(time.Time); ok {
		exec.CreatedAt = t
	}
	if t, ok := row[8].(time.Time); ok {
		exec.UpdatedAt = t
	}
	return nil
}

func scanExecutionResult(qr *rqlite.QueryResult, exec *model.TaskExecution) error {
	row, err := qr.Slice()
	if err != nil {
		return err
	}

	exec.ID = rowInt64(row[0])
	exec.TaskID = rowInt64(row[1])
	exec.ExecutionID = rowString(row[2])
	exec.ExecutorID = rowInt64(row[3])
	exec.Status = rowString(row[4])

	// 处理 start_time
	if t, ok := row[5].(time.Time); ok {
		exec.StartTime = rqlite.NullTime{Time: t, Valid: true}
	} else if s, ok := row[5].(string); ok && s != "" {
		parsed, err := time.Parse("2006-01-02 15:04:05", s)
		if err == nil {
			exec.StartTime = rqlite.NullTime{Time: parsed, Valid: true}
		}
	}

	// 处理 end_time
	if t, ok := row[6].(time.Time); ok {
		exec.EndTime = rqlite.NullTime{Time: t, Valid: true}
	} else if s, ok := row[6].(string); ok && s != "" {
		parsed, err := time.Parse("2006-01-02 15:04:05", s)
		if err == nil {
			exec.EndTime = rqlite.NullTime{Time: parsed, Valid: true}
		}
	}

	exec.Output = rowString(row[7])
	exec.Error = rowString(row[8])
	exec.RetryTimes = int32(rowInt64(row[9]))

	// 处理 created_at
	if t, ok := row[10].(time.Time); ok {
		exec.CreatedAt = t
	} else if s, ok := row[10].(string); ok && s != "" {
		parsed, err := time.Parse("2006-01-02 15:04:05", s)
		if err == nil {
			exec.CreatedAt = parsed
		}
	}

	// 处理新增的字段
	if len(row) > 11 {
		exec.Progress = int32(rowInt64(row[11]))
	}
	if len(row) > 12 {
		exec.ProgressMsg = rowString(row[12])
	}
	if len(row) > 13 {
		if t, ok := row[13].(time.Time); ok {
			exec.UpdatedAt = t
		} else if s, ok := row[13].(string); ok && s != "" {
			parsed, err := time.Parse("2006-01-02 15:04:05", s)
			if err == nil {
				exec.UpdatedAt = parsed
			}
		}
	}

	return nil
}

func scanWorkflowExecutionResult(qr *rqlite.QueryResult, we *model.WorkflowExecution) error {
	row, err := qr.Slice()
	if err != nil {
		return err
	}
	we.ID = rowInt64(row[0])
	we.WorkflowID = rowInt64(row[1])
	we.ExecutionID = rowString(row[2])
	we.Status = rowString(row[3])
	
	// 处理 start_time
	if t, ok := row[4].(time.Time); ok {
		we.StartTime = rqlite.NullTime{Time: t, Valid: true}
	} else if s, ok := row[4].(string); ok && s != "" {
		parsed, err := time.Parse("2006-01-02 15:04:05", s)
		if err == nil {
			we.StartTime = rqlite.NullTime{Time: parsed, Valid: true}
		}
	}
	
	// 处理 end_time
	if t, ok := row[5].(time.Time); ok {
		we.EndTime = rqlite.NullTime{Time: t, Valid: true}
	} else if s, ok := row[5].(string); ok && s != "" {
		parsed, err := time.Parse("2006-01-02 15:04:05", s)
		if err == nil {
			we.EndTime = rqlite.NullTime{Time: parsed, Valid: true}
		}
	}
	
	we.NodeStates = rowString(row[6])
	
	// 处理 created_at
	if t, ok := row[7].(time.Time); ok {
		we.CreatedAt = t
	} else if s, ok := row[7].(string); ok && s != "" {
		parsed, err := time.Parse("2006-01-02 15:04:05", s)
		if err == nil {
			we.CreatedAt = parsed
		}
	}
	
	return nil
}

func scanTaskLogResult(qr *rqlite.QueryResult, tl *model.TaskLog) error {
	row, err := qr.Slice()
	if err != nil {
		return err
	}
	tl.ID = rowInt64(row[0])
	tl.ExecutionID = rowString(row[1])
	tl.TaskID = rowInt64(row[2])
	tl.ExecutorID = rowInt64(row[3])
	tl.NodeID = rowString(row[4])
	tl.LogLevel = rowString(row[5])
	tl.Message = rowString(row[6])

	// 处理 log_time
	if t, ok := row[7].(time.Time); ok {
		tl.LogTime = t
	} else if s, ok := row[7].(string); ok && s != "" {
		parsed, err := time.Parse("2006-01-02 15:04:05", s)
		if err == nil {
			tl.LogTime = parsed
		}
	}

	return nil
}

func rowInt64(v interface{}) int64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case int64:
		return val
	case int:
		return int64(val)
	case float64:
		return int64(val)
	case string:
		var n int64
		fmt.Sscanf(val, "%d", &n)
		return n
	}
	return 0
}

func rowInt(v interface{}) int {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case float64:
		return int(val)
	case string:
		var n int
		fmt.Sscanf(val, "%d", &n)
		return n
	}
	return 0
}

func rowString(v interface{}) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

func rowBool(v interface{}) bool {
	if v == nil {
		return false
	}
	switch val := v.(type) {
	case bool:
		return val
	case int64:
		return val != 0
	case float64:
		return val != 0
	}
	return false
}

// GetExecutionStats 获取执行记录统计信息
func (s *SchedulerService) GetExecutionStats(ctx context.Context, domainID int64, role string, filter map[string]string) (map[string]int, error) {
	// 构建 WHERE 条件
	whereClause := " WHERE 1=1"
	var args []interface{}
	
	// 应用领域隔离
	isSystemAdmin := role == "system_admin" || role == "admin"
	if !isSystemAdmin {
		whereClause += " AND t.domain_id = ?"
		args = append(args, domainID)
	}

	// 应用筛选条件
	if filter["id"] != "" {
		if id, err := strconv.ParseInt(filter["id"], 10, 64); err == nil {
			whereClause += " AND te.id = ?"
			args = append(args, id)
		}
	}
	if filter["execution_id"] != "" {
		whereClause += " AND te.execution_id LIKE ?"
		args = append(args, "%"+filter["execution_id"]+"%")
	}
	if filter["executor_name"] != "" {
		whereClause += " AND e.name LIKE ?"
		args = append(args, "%"+filter["executor_name"]+"%")
	}
	if filter["task_name"] != "" {
		whereClause += " AND t.name LIKE ?"
		args = append(args, "%"+filter["task_name"]+"%")
	}
	if filter["status"] != "" {
		whereClause += " AND te.status = ?"
		args = append(args, filter["status"])
	}
	if filter["start_time_from"] != "" {
		whereClause += " AND te.start_time >= ?"
		args = append(args, filter["start_time_from"])
	}
	if filter["start_time_to"] != "" {
		whereClause += " AND te.start_time <= ?"
		args = append(args, filter["start_time_to"])
	}
	if filter["end_time_from"] != "" {
		whereClause += " AND te.end_time >= ?"
		args = append(args, filter["end_time_from"])
	}
	if filter["end_time_to"] != "" {
		whereClause += " AND te.end_time <= ?"
		args = append(args, filter["end_time_to"])
	}
	if filter["duration_min"] != "" || filter["duration_max"] != "" {
		whereClause += " AND te.end_time IS NOT NULL"
		if filter["duration_min"] != "" {
			if duration, err := strconv.ParseFloat(filter["duration_min"], 64); err == nil {
				durationSecs := int64(duration)
				whereClause += " AND (STRFTIME('%s', te.end_time) - STRFTIME('%s', te.start_time)) >= ?"
				args = append(args, durationSecs)
			}
		}
		if filter["duration_max"] != "" {
			if duration, err := strconv.ParseFloat(filter["duration_max"], 64); err == nil {
				durationSecs := int64(duration)
				whereClause += " AND (STRFTIME('%s', te.end_time) - STRFTIME('%s', te.start_time)) <= ?"
				args = append(args, durationSecs)
			}
		}
	}

	// 统一使用 JOIN
	joinClause := `
		FROM bdopsflow_task_executions te
		LEFT JOIN bdopsflow_tasks t ON te.task_id = t.id
		LEFT JOIN bdopsflow_executors e ON te.executor_id = e.id
	`

	// 统计各个状态的数量
	query := `
		SELECT te.status, COUNT(*) as count
	` + joinClause + whereClause + " GROUP BY te.status"

	var qr rqlite.QueryResult
	var err error

	if len(args) > 0 {
		stmt := rqlite.ParameterizedStatement{
			Query:     query,
			Arguments: args,
		}
		qr, err = s.DB.QueryOneParameterized(stmt)
	} else {
		qr, err = s.DB.QueryOne(query)
	}

	if err != nil {
		slog.Error("GetExecutionStats: query failed", "error", err)
		return nil, err
	}
	if qr.Err != nil {
		slog.Error("GetExecutionStats: query returned error", "error", qr.Err)
		return nil, qr.Err
	}

	stats := make(map[string]int)
	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			continue
		}
		status := rowString(row[0])
		count := int(rowInt64(row[1]))
		stats[status] = count
	}

	return stats, nil
}

// DashboardStats 仪表盘统计数据
type DashboardStats struct {
	Tasks struct {
		Total         int64 `json:"total"`
		Enabled       int64 `json:"enabled"`
		Cron          int64 `json:"cron"`
		Running       int64 `json:"running"`
		Success       int64 `json:"success"`
		Failed        int64 `json:"failed"`
		AvgDuration   int64 `json:"avg_duration"` // 平均执行时长（秒）
	} `json:"tasks"`
	Workflows struct {
		Total   int64 `json:"total"`
		Enabled int64 `json:"enabled"`
	} `json:"workflows"`
	Executors struct {
		Total  int64 `json:"total"`
		Active int64 `json:"active"`
	} `json:"executors"`
	Scheduler struct {
		Paused bool   `json:"paused"`
		Uptime int64  `json:"uptime"` // 运行时长（秒）
	} `json:"scheduler"`
}

// TrendData 趋势数据
type TrendData struct {
	Date    string `json:"date"`
	Total   int64  `json:"total"`
	Success int64  `json:"success"`
	Failed  int64  `json:"failed"`
}

// GetDashboardStats 获取仪表盘统计数据
func (s *SchedulerService) GetDashboardStats(ctx context.Context, domainID int64, role string) (*DashboardStats, error) {
	stats := &DashboardStats{}
	isSystemAdmin := role == "system_admin" || role == "admin"
	
	// 任务统计
	var taskQuery string
	var args []interface{}
	if isSystemAdmin {
		taskQuery = `
			SELECT 
				COUNT(*) as total,
				SUM(CASE WHEN is_enabled = 1 THEN 1 ELSE 0 END) as enabled,
				SUM(CASE WHEN cron_expression IS NOT NULL AND cron_expression != '' THEN 1 ELSE 0 END) as cron
			FROM bdopsflow_tasks
		`
	} else {
		taskQuery = `
			SELECT 
				COUNT(*) as total,
				SUM(CASE WHEN is_enabled = 1 THEN 1 ELSE 0 END) as enabled,
				SUM(CASE WHEN cron_expression IS NOT NULL AND cron_expression != '' THEN 1 ELSE 0 END) as cron
			FROM bdopsflow_tasks WHERE domain_id = ?
		`
		args = append(args, domainID)
	}
	qr, err := s.executeQuery(taskQuery, args)
	if err != nil {
		return nil, err
	}
	if qr.Next() {
		row, _ := qr.Slice()
		stats.Tasks.Total = rowInt64(row[0])
		stats.Tasks.Enabled = rowInt64(row[1])
		stats.Tasks.Cron = rowInt64(row[2])
	}
	
	// 运行中的任务
	var runningQuery string
	args = []interface{}{}
	if isSystemAdmin {
		runningQuery = `SELECT COUNT(*) FROM bdopsflow_task_executions WHERE status = 'running'`
	} else {
		runningQuery = `
			SELECT COUNT(*) 
			FROM bdopsflow_task_executions te 
			JOIN bdopsflow_tasks t ON te.task_id = t.id
			WHERE te.status = 'running' AND t.domain_id = ?
		`
		args = append(args, domainID)
	}
	qr, err = s.executeQuery(runningQuery, args)
	if err == nil && qr.Next() {
		row, _ := qr.Slice()
		stats.Tasks.Running = rowInt64(row[0])
	}
	
	// 最近执行的任务统计
	var recentExecQuery string
	args = []interface{}{}
	if isSystemAdmin {
		recentExecQuery = `
			SELECT 
				SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as success,
				SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed,
				AVG(CASE WHEN end_time IS NOT NULL AND start_time IS NOT NULL 
					THEN julianday(end_time) - julianday(start_time) ELSE 0 END) * 86400 as avg_duration
			FROM bdopsflow_task_executions
			WHERE created_at > datetime('now', '-7 days')
		`
	} else {
		recentExecQuery = `
			SELECT 
				SUM(CASE WHEN te.status = 'success' THEN 1 ELSE 0 END) as success,
				SUM(CASE WHEN te.status = 'failed' THEN 1 ELSE 0 END) as failed,
				AVG(CASE WHEN te.end_time IS NOT NULL AND te.start_time IS NOT NULL 
					THEN julianday(te.end_time) - julianday(te.start_time) ELSE 0 END) * 86400 as avg_duration
			FROM bdopsflow_task_executions te
			JOIN bdopsflow_tasks t ON te.task_id = t.id
			WHERE te.created_at > datetime('now', '-7 days') AND t.domain_id = ?
		`
		args = append(args, domainID)
	}
	qr, err = s.executeQuery(recentExecQuery, args)
	if err == nil && qr.Next() {
		row, _ := qr.Slice()
		stats.Tasks.Success = rowInt64(row[0])
		stats.Tasks.Failed = rowInt64(row[1])
		stats.Tasks.AvgDuration = int64(rowFloat64(row[2]))
	}
	
	// 工作流统计
	var wfQuery string
	args = []interface{}{}
	if isSystemAdmin {
		wfQuery = `
			SELECT 
				COUNT(*) as total,
				SUM(CASE WHEN is_enabled = 1 THEN 1 ELSE 0 END) as enabled
			FROM bdopsflow_workflows
		`
	} else {
		wfQuery = `
			SELECT 
				COUNT(*) as total,
				SUM(CASE WHEN is_enabled = 1 THEN 1 ELSE 0 END) as enabled
			FROM bdopsflow_workflows WHERE domain_id = ?
		`
		args = append(args, domainID)
	}
	qr, err = s.executeQuery(wfQuery, args)
	if err == nil && qr.Next() {
		row, _ := qr.Slice()
		stats.Workflows.Total = rowInt64(row[0])
		stats.Workflows.Enabled = rowInt64(row[1])
	}
	
	// 执行器统计
	var execQuery string
	args = []interface{}{}
	if isSystemAdmin {
		execQuery = `
			SELECT 
				COUNT(*) as total,
				SUM(CASE WHEN status = 'online' THEN 1 ELSE 0 END) as online
			FROM bdopsflow_executors
		`
	} else {
		execQuery = `
			SELECT 
				COUNT(DISTINCT e.id) as total,
				SUM(CASE WHEN e.status = 'online' THEN 1 ELSE 0 END) as online
			FROM bdopsflow_executors e
			JOIN bdopsflow_domain_executors de ON e.id = de.executor_id
			WHERE de.domain_id = ?
		`
		args = append(args, domainID)
	}
	qr, err = s.executeQuery(execQuery, args)
	if err == nil && qr.Next() {
		row, _ := qr.Slice()
		stats.Executors.Total = rowInt64(row[0])
		stats.Executors.Active = rowInt64(row[1])
	}
	
	// 调度器状态
	if s.cronScheduler != nil {
		stats.Scheduler.Paused = s.cronScheduler.IsPaused()
		stats.Scheduler.Uptime = int64(s.cronScheduler.GetUptime().Seconds())
	}
	
	return stats, nil
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

// GetTrendData 获取最近7天的趋势数据
func (s *SchedulerService) GetTrendData(ctx context.Context, domainID int64, role string) ([]*TrendData, error) {
	var trends []*TrendData
	isSystemAdmin := role == "system_admin" || role == "admin"
	
	var query string
	var args []interface{}
	if isSystemAdmin {
		query = `
			SELECT 
				date(created_at) as exec_date,
				COUNT(*) as total,
				SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as success,
				SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed
			FROM bdopsflow_task_executions
			WHERE created_at > datetime('now', '-7 days')
			GROUP BY date(created_at)
			ORDER BY exec_date DESC
		`
	} else {
		query = `
			SELECT 
				date(te.created_at) as exec_date,
				COUNT(*) as total,
				SUM(CASE WHEN te.status = 'success' THEN 1 ELSE 0 END) as success,
				SUM(CASE WHEN te.status = 'failed' THEN 1 ELSE 0 END) as failed
			FROM bdopsflow_task_executions te
			JOIN bdopsflow_tasks t ON te.task_id = t.id
			WHERE te.created_at > datetime('now', '-7 days') AND t.domain_id = ?
			GROUP BY date(te.created_at)
			ORDER BY exec_date DESC
		`
		args = append(args, domainID)
	}
	
	qr, err := s.executeQuery(query, args)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}
	
	for qr.Next() {
		row, _ := qr.Slice()
		trend := &TrendData{
			Date:    rowString(row[0]),
			Total:   rowInt64(row[1]),
			Success: rowInt64(row[2]),
			Failed:  rowInt64(row[3]),
		}
		trends = append(trends, trend)
	}
	
	return trends, nil
}

// PauseScheduler 暂停调度器
func (s *SchedulerService) PauseScheduler() {
	if s.cronScheduler != nil {
		s.cronScheduler.Pause()
	}
}

// ResumeScheduler 恢复调度器
func (s *SchedulerService) ResumeScheduler() {
	if s.cronScheduler != nil {
		s.cronScheduler.Resume()
	}
}

// IsSchedulerPaused 获取调度器暂停状态
func (s *SchedulerService) IsSchedulerPaused() bool {
	if s.cronScheduler != nil {
		return s.cronScheduler.IsPaused()
	}
	return false
}

// HealthCheckResult 健康检查结果
type HealthCheckResult struct {
	Status    string                   `json:"status"`
	Timestamp string                   `json:"timestamp"`
	Components map[string]ComponentCheck `json:"components"`
}

// ComponentCheck 组件检查结果
type ComponentCheck struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Latency string `json:"latency,omitempty"`
}

// requiredTables 必需的表列表
var requiredTables = []string{
	"bdopsflow_domains",
	"bdopsflow_users",
	"bdopsflow_workflows",
	"bdopsflow_tasks",
	"bdopsflow_task_executions",
	"bdopsflow_executors",
	"bdopsflow_workflow_executions",
	"bdopsflow_task_logs",
	"bdopsflow_roles",
	"bdopsflow_permissions",
	"bdopsflow_role_permissions",
	"bdopsflow_user_roles",
	"bdopsflow_domain_executors",
}

// HealthCheck 执行系统健康检查
func (s *SchedulerService) HealthCheck(ctx context.Context) *HealthCheckResult {
	result := &HealthCheckResult{
		Timestamp:  time.Now().Format("2006-01-02 15:04:05"),
		Components: make(map[string]ComponentCheck),
	}

	allHealthy := true

	// 1. 检查 rqlite 连接
	rqliteCheck := s.checkRQLite()
	result.Components["rqlite"] = rqliteCheck
	if rqliteCheck.Status != "healthy" {
		allHealthy = false
	}

	// 2. 检查 rqlite 表结构
	tableCheck := s.checkTables()
	result.Components["rqlite_tables"] = tableCheck
	if tableCheck.Status != "healthy" {
		allHealthy = false
	}

	// 3. 检查 Redis 连接
	redisCheck := s.checkRedis()
	result.Components["redis"] = redisCheck
	if redisCheck.Status != "healthy" {
		allHealthy = false
	}

	// 4. 检查调度器状态
	schedulerCheck := s.checkScheduler()
	result.Components["scheduler"] = schedulerCheck
	if schedulerCheck.Status != "healthy" {
		allHealthy = false
	}

	if allHealthy {
		result.Status = "healthy"
	} else {
		result.Status = "unhealthy"
	}

	return result
}

// checkRQLite 检查 rqlite 连接
func (s *SchedulerService) checkRQLite() ComponentCheck {
	start := time.Now()
	
	query := "SELECT 1"
	qr, err := s.DB.QueryOne(query)
	latency := time.Since(start)
	
	if err != nil {
		return ComponentCheck{
			Status:  "unhealthy",
			Message: fmt.Sprintf("连接失败: %v", err),
			Latency: latency.String(),
		}
	}
	if qr.Err != nil {
		return ComponentCheck{
			Status:  "unhealthy",
			Message: fmt.Sprintf("查询失败: %v", qr.Err),
			Latency: latency.String(),
		}
	}
	
	return ComponentCheck{
		Status:  "healthy",
		Message: "连接正常",
		Latency: latency.String(),
	}
}

// checkTables 检查必需的表是否存在
func (s *SchedulerService) checkTables() ComponentCheck {
	missingTables := []string{}
	
	for _, tableName := range requiredTables {
		query := fmt.Sprintf("SELECT name FROM sqlite_master WHERE type='table' AND name='%s'", tableName)
		qr, err := s.DB.QueryOne(query)
		if err != nil {
			missingTables = append(missingTables, tableName)
			continue
		}
		if qr.Err != nil || !qr.Next() {
			missingTables = append(missingTables, tableName)
			continue
		}
	}
	
	if len(missingTables) > 0 {
		return ComponentCheck{
			Status:  "unhealthy",
			Message: fmt.Sprintf("缺少表: %v", missingTables),
		}
	}
	
	return ComponentCheck{
		Status:  "healthy",
		Message: fmt.Sprintf("所有 %d 个表正常", len(requiredTables)),
	}
}

// checkRedis 检查 Redis 连接
func (s *SchedulerService) checkRedis() ComponentCheck {
	start := time.Now()
	
	err := s.redis.Ping(context.Background()).Err()
	latency := time.Since(start)
	
	if err != nil {
		return ComponentCheck{
			Status:  "unhealthy",
			Message: fmt.Sprintf("连接失败: %v", err),
			Latency: latency.String(),
		}
	}
	
	return ComponentCheck{
		Status:  "healthy",
		Message: "连接正常",
		Latency: latency.String(),
	}
}

// checkScheduler 检查调度器状态
func (s *SchedulerService) checkScheduler() ComponentCheck {
	paused := s.IsSchedulerPaused()
	
	if s.cronScheduler == nil {
		return ComponentCheck{
			Status:  "unhealthy",
			Message: "调度器未初始化",
		}
	}
	
	if paused {
		return ComponentCheck{
			Status:  "unhealthy",
			Message: "已暂停",
		}
	}
	
	return ComponentCheck{
		Status:  "healthy",
		Message: "运行中",
	}
}

func rowFloat64(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case int64:
		return float64(val)
	case string:
		var n float64
		fmt.Sscanf(val, "%f", &n)
		return n
	}
	return 0
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

func (s *SchedulerService) ListExecutorsByDomain(ctx context.Context, domainID int64) ([]*model.Executor, error) {
	query := `
		SELECT e.id, e.name, e.address, e.status, e.last_heartbeat, e.capacity, e.current_load, e.created_at, e.updated_at
		FROM bdopsflow_executors e
		LEFT JOIN bdopsflow_domain_executors de ON e.id = de.executor_id
		WHERE e.status = 'online' AND (de.domain_id = ? OR e.is_global = 1)
		ORDER BY e.current_load ASC
	`
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{domainID},
	}
	qr, err := s.DB.QueryOneParameterized(stmt)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	var executors []*model.Executor
	for qr.Next() {
		exec := &model.Executor{}
		if err := scanExecutorResult(&qr, exec); err != nil {
			continue
		}
		executors = append(executors, exec)
	}

	if executors == nil {
		executors = []*model.Executor{}
	}

	return executors, nil
}
