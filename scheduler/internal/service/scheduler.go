package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

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

type TaskDispatcher func(executorID string, task *pb.Task) error

type SchedulerService struct {
	DB        rqlite.Connection
	redis     *redis.Client
	dispatcher TaskDispatcher
	cronScheduler interface {
		RegisterTask(taskID int64, cronExpr string)
		UnregisterTask(taskID int64)
	}
	webhookSvc *webhook.Service
	stopCleanupCh chan struct{}
}

func NewSchedulerService(db rqlite.Connection, redis *redis.Client) *SchedulerService {
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
		}
	}
}

func (s *SchedulerService) cleanupDeadTasks() {
	ctx := context.Background()

	// 查找所有 running 状态超过 10 分钟的任务
	query := `
		SELECT id, task_id, execution_id, start_time, created_at
		FROM task_executions
		WHERE status = 'running'
		AND (start_time IS NOT NULL AND start_time != '')
		AND created_at < datetime('now', '-10 minutes')
	`

	qr, err := s.DB.QueryOne(query)
	if err != nil {
		slog.Error("cleanup: query stuck tasks failed", "error", err)
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

	slog.Warn("found stuck tasks", "count", len(stuckExecutions))

	for _, exec := range stuckExecutions {
		// 检查锁是否存在
		lockKey := fmt.Sprintf("task:lock:%s", exec.ExecutionID)
		lockExists, err := s.redis.Exists(ctx, lockKey).Result()
		if err != nil {
			slog.Error("cleanup: check lock existence failed", "error", err, "execution_id", exec.ExecutionID)
			continue
		}

		if lockExists == 0 {
			// 锁不存在，说明任务已经卡死，强制更新状态
			slog.Warn("cleanup: task is stuck, force updating status to failed",
				"task_id", exec.TaskID,
				"execution_id", exec.ExecutionID,
				"start_time", exec.StartTime,
			)

			// 更新执行状态为失败
			err := s.UpdateExecutionResult(ctx, exec.ExecutionID, "failed", "", "task execution timeout or crashed, no lock found")
			if err != nil {
				slog.Error("cleanup: failed to update execution result", "error", err, "execution_id", exec.ExecutionID)
			}

			// 更新任务状态
			err = s.UpdateTaskStatusByID(ctx, exec.TaskID, "failed")
			if err != nil {
				slog.Error("cleanup: failed to update task status", "error", err, "task_id", exec.TaskID)
			}
		}
	}
}

func (s *SchedulerService) cleanupOfflineExecutors() {
	query := `
		UPDATE executors 
		SET status = 'offline', updated_at = datetime('now')
		WHERE status = 'online' 
		AND last_heartbeat < datetime('now', '-60 seconds')
	`

	result, err := s.DB.WriteOne(query)
	if err != nil {
		slog.Error("cleanup: update offline executors failed", "error", err)
		return
	}
	if result.Err != nil {
		slog.Error("cleanup: update offline executors returned error", "error", result.Err)
		return
	}

	if result.RowsAffected > 0 {
		slog.Warn("marked executors as offline", "count", result.RowsAffected)
	}
}

// StopCleanupRoutine 停止定时清理例程
func (s *SchedulerService) StopCleanupRoutine() {
	close(s.stopCleanupCh)
}

func (s *SchedulerService) SetCronScheduler(cs interface {
	RegisterTask(taskID int64, cronExpr string)
	UnregisterTask(taskID int64)
}) {
	s.cronScheduler = cs
}

func (s *SchedulerService) SetTaskDispatcher(dispatcher TaskDispatcher) {
	s.dispatcher = dispatcher
}

func (s *SchedulerService) SetWebhookService(webhookSvc *webhook.Service) {
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

	if task.WebhookConfig == "" {
		slog.Info("no webhook configured for task", "task_id", taskID)
		return
	}

	var config map[string]interface{}
	if err := json.Unmarshal([]byte(task.WebhookConfig), &config); err != nil {
		slog.Error("failed to parse webhook config", "task_id", taskID, "error", err)
		return
	}

	event := "success"
	if status == "failed" {
		event = "failed"
	}

	payload := map[string]interface{}{
		"event":        event,
		"timestamp":    time.Now().Unix(),
		"task_id":      taskID,
		"execution_id": executionID,
		"status":       status,
		"output":       output,
		"error":        errorMsg,
		"duration_ms":  durationMs,
	}

	if task.Name != "" || task.Type != "" {
		metadata := make(map[string]interface{})
		if task.Name != "" {
			metadata["task_name"] = task.Name
		}
		if task.Type != "" {
			metadata["task_type"] = task.Type
		}
		payload["metadata"] = metadata
	}

	if err := s.webhookSvc.SendFromMap(ctx, config, payload); err != nil {
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
	query := `SELECT status FROM task_executions WHERE task_id = ? ORDER BY created_at DESC LIMIT 1`
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
		       retry_count, retry_interval, is_enabled, status, domain_id, webhook_config,
		       assigned_executor_id, created_by, created_at, updated_at
		FROM tasks WHERE id = ?
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

func (s *SchedulerService) ListTasks(ctx context.Context) ([]*model.Task, error) {
	query := `
		SELECT id, workflow_id, name, type, config, cron_expression, timeout_seconds,
		       retry_count, retry_interval, is_enabled, status, domain_id, webhook_config,
		       assigned_executor_id, created_by, created_at, updated_at
		FROM tasks ORDER BY created_at DESC
	`

	qr, err := s.DB.QueryOne(query)
	if err != nil {
		return nil, err
	}

	if qr.Err != nil {
		return nil, qr.Err
	}

	var tasks []*model.Task
	for qr.Next() {
		task := &model.Task{}
		if err := scanTaskResult(&qr, task); err != nil {
			return nil, err
		}
		task.NextExecutionTime = CalculateNextExecutionTime(task.CronExpression, task.IsEnabled)
		task.LastExecutionStatus = s.getLastExecutionStatus(ctx, task.ID)
		tasks = append(tasks, task)
	}

	return tasks, nil
}

func (s *SchedulerService) UpdateTask(ctx context.Context, id int64, task *model.Task) error {
	query := `
		UPDATE tasks SET name = ?, type = ?, config = ?, cron_expression = ?,
		               timeout_seconds = ?, retry_count = ?, retry_interval = ?,
		               is_enabled = ?, webhook_config = ?, assigned_executor_id = ?, updated_at = ?
		WHERE id = ?
	`

	isEnabled := int64(0)
	if task.IsEnabled {
		isEnabled = 1
	}

	stmt := rqlite.ParameterizedStatement{
		Query: query,
		Arguments: []interface{}{
			task.Name, task.Type, task.Config, task.CronExpression,
			int64(task.TimeoutSeconds), int64(task.RetryCount), int64(task.RetryInterval),
			isEnabled, task.WebhookConfig, task.AssignedExecutorID, time.Now().Format("2006-01-02 15:04:05"), id,
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
	query := `DELETE FROM tasks WHERE id = ?`

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
		SELECT execution_id FROM task_executions 
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
		nowUTC := time.Now().UTC().Format("2006-01-02 15:04:05")
		skippedReason := fmt.Sprintf("skipped: previous execution (id: %s) is still running", runningExecID)
		
		insertQuery := `
			INSERT INTO task_executions (task_id, execution_id, executor_id, status, output, error, start_time, retry_times, created_at)
			VALUES (?, ?, '', 'skipped', '', ?, ?, 0, ?)
		`
		insertStmt := rqlite.ParameterizedStatement{
			Query:     insertQuery,
			Arguments: []interface{}{taskID, skippedExecutionID, skippedReason, nowUTC, nowUTC},
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

	nowUTC := time.Now().UTC().Format("2006-01-02 15:04:05")
	insertQuery := `
		INSERT INTO task_executions (task_id, execution_id, executor_id, status, start_time, retry_times, created_at)
		VALUES (?, ?, '', 'running', ?, 0, ?)
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     insertQuery,
		Arguments: []interface{}{taskID, executionID, nowUTC, nowUTC},
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
	if task.AssignedExecutorID != "" {
		// 使用指定的执行器
		executor, err = s.GetExecutorByID(ctx, task.AssignedExecutorID)
		if err != nil {
			slog.Error("specified executor not found",
				"task_id", taskID,
				"assigned_executor_id", task.AssignedExecutorID,
				"error", err,
			)
			s.UpdateExecutionResult(ctx, executionID, "failed", "", fmt.Sprintf("specified executor %s not found: %v", task.AssignedExecutorID, err))
			s.UpdateTaskStatusByID(ctx, taskID, "failed")
			s.redis.Del(ctx, lockKey)
			return executionID, fmt.Errorf("specified executor %s not found: %w", task.AssignedExecutorID, err)
		}

		// 检查执行器是否在线且有容量
		if executor.Status != "online" {
			slog.Error("specified executor is not online",
				"task_id", taskID,
				"assigned_executor_id", task.AssignedExecutorID,
				"executor_status", executor.Status,
			)
			s.UpdateExecutionResult(ctx, executionID, "failed", "", fmt.Sprintf("specified executor %s is not online (status: %s)", task.AssignedExecutorID, executor.Status))
			s.UpdateTaskStatusByID(ctx, taskID, "failed")
			s.redis.Del(ctx, lockKey)
			return executionID, fmt.Errorf("specified executor %s is not online", task.AssignedExecutorID)
		}

		if executor.CurrentLoad >= executor.Capacity {
			slog.Error("specified executor has no capacity",
				"task_id", taskID,
				"assigned_executor_id", task.AssignedExecutorID,
				"current_load", executor.CurrentLoad,
				"capacity", executor.Capacity,
			)
			s.UpdateExecutionResult(ctx, executionID, "failed", "", fmt.Sprintf("specified executor %s has no capacity (load: %d/%d)", task.AssignedExecutorID, executor.CurrentLoad, executor.Capacity))
			s.UpdateTaskStatusByID(ctx, taskID, "failed")
			s.redis.Del(ctx, lockKey)
			return executionID, fmt.Errorf("specified executor %s has no capacity", task.AssignedExecutorID)
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
			s.redis.Del(ctx, lockKey)
			return executionID, fmt.Errorf("no available executor: %w", err)
		}
		slog.Info("using load-balanced executor",
			"task_id", taskID,
			"execution_id", executionID,
			"executor_id", executor.ExecutorID,
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
		// 清理锁
		s.redis.Del(ctx, lockKey)
		return executionID, fmt.Errorf("dispatcher not configured")
	}

	// 在分派任务前，先更新 task_executions 表设置 executor_id
	updateExecutorQuery := `UPDATE task_executions SET executor_id = ? WHERE execution_id = ?`
	updateExecutorStmt := rqlite.ParameterizedStatement{
		Query:     updateExecutorQuery,
		Arguments: []interface{}{executor.ExecutorID, executionID},
	}
	_, err = s.DB.WriteOneParameterized(updateExecutorStmt)
	if err != nil {
		slog.Warn("failed to update executor_id in task_executions", "error", err, "execution_id", executionID)
		// 继续执行，不影响任务调度
	}

	if err := s.dispatcher(executor.ExecutorID, grpcTask); err != nil {
		slog.Error("dispatch task failed", "task_id", taskID, "executor", executor.ExecutorID, "error", err)
		s.UpdateExecutionResult(ctx, executionID, "failed", "", fmt.Sprintf("dispatch failed: %v", err))
		s.UpdateTaskStatusByID(ctx, taskID, "failed")
		// 清理锁
		s.redis.Del(ctx, lockKey)
		return executionID, fmt.Errorf("dispatch failed: %w", err)
	}

	slog.Info("task dispatched",
		"task_id", taskID,
		"execution_id", executionID,
		"executor", executor.ExecutorID,
	)

	return executionID, nil
}

func (s *SchedulerService) UpdateTaskStatusByID(ctx context.Context, taskID int64, status string) error {
	// 如果任务有cron表达式，我们不应该修改任务状态，让它继续保持pending以便下次cron触发
	task, err := s.GetTaskByID(ctx, taskID)
	if err == nil && task.CronExpression != "" {
		// 对于定时任务，只更新updated_at，不改变status
		query := `UPDATE tasks SET updated_at = ? WHERE id = ?`
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
	query := `UPDATE tasks SET status = ?, updated_at = ? WHERE id = ?`
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
		       retry_count, retry_interval, is_enabled, status, domain_id, webhook_config,
		       assigned_executor_id, created_by, created_at, updated_at
		FROM tasks
		WHERE is_enabled = 1 AND cron_expression != ''
	`

	qr, err := s.DB.QueryOne(query)
	if err != nil {
		return nil, err
	}

	if qr.Err != nil {
		return nil, qr.Err
	}

	var tasks []*model.Task
	for qr.Next() {
		task := &model.Task{}
		if err := scanTaskResult(&qr, task); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}

func (s *SchedulerService) SelectAvailableExecutor(ctx context.Context) (*model.Executor, error) {
	query := `
		SELECT id, executor_id, name, address, status, last_heartbeat, capacity, current_load, created_at, updated_at
		FROM executors
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

func (s *SchedulerService) RegisterExecutor(ctx context.Context, executorID, name, address string, capacity int32) error {
	query := `
		INSERT INTO executors (executor_id, name, address, status, capacity, current_load, last_heartbeat, created_at, updated_at)
		VALUES (?, ?, ?, 'online', ?, 0, ?, ?, ?)
		ON CONFLICT(executor_id) DO UPDATE SET
			name = excluded.name, status = 'online', capacity = excluded.capacity,
			last_heartbeat = excluded.last_heartbeat, updated_at = excluded.updated_at
	`

	now := time.Now().Format("2006-01-02 15:04:05")
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{executorID, name, address, capacity, now, now, now},
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

func (s *SchedulerService) UpdateExecutorHeartbeat(ctx context.Context, executorID string, currentLoad int32) error {
	query := `
		UPDATE executors SET current_load = ?, last_heartbeat = ?, status = 'online', updated_at = ?
		WHERE executor_id = ?
	`

	now := time.Now().Format("2006-01-02 15:04:05")
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{currentLoad, now, now, executorID},
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

// GetExecutorByID 根据 executor_id 获取执行器信息
func (s *SchedulerService) GetExecutorByID(ctx context.Context, executorID string) (*model.Executor, error) {
	query := `
		SELECT id, executor_id, name, address, status, last_heartbeat, capacity, current_load, created_at, updated_at
		FROM executors WHERE executor_id = ?
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{executorID},
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

func (s *SchedulerService) ListExecutors(ctx context.Context) ([]*model.Executor, error) {
	query := `
		SELECT id, executor_id, name, address, status, last_heartbeat, capacity, current_load, created_at, updated_at
		FROM executors ORDER BY created_at DESC
	`

	qr, err := s.DB.QueryOne(query)
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
			return nil, err
		}
		executors = append(executors, exec)
	}

	return executors, nil
}

func (s *SchedulerService) UpdateExecutionResult(ctx context.Context, executionID, status, output, errorMsg string) error {
	nowUTC := time.Now().UTC().Format("2006-01-02 15:04:05")
	query := `
		UPDATE task_executions
		SET status = ?, output = ?, error = ?,
		    end_time = CASE WHEN ? IN ('success', 'failed', 'timeout') THEN ? ELSE end_time END,
		    start_time = CASE WHEN start_time IS NULL OR start_time = '' THEN ? ELSE start_time END
		WHERE execution_id = ?
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{status, output, errorMsg, status, nowUTC, nowUTC, executionID},
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
	getTaskIDQuery := `SELECT task_id FROM task_executions WHERE execution_id = ?`
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

func (s *SchedulerService) GetTaskExecutions(ctx context.Context, taskID int64) ([]*model.TaskExecution, error) {
	query := `
		SELECT id, task_id, execution_id, executor_id, status, start_time, end_time,
		       output, error, retry_times, created_at
		FROM task_executions
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

// GetExecutorInfoByID 根据执行器ID获取执行器信息
func (s *SchedulerService) GetExecutorInfoByID(ctx context.Context, executorID string) (*model.Executor, error) {
	query := `
		SELECT id, executor_id, name, address, status, last_heartbeat, capacity, current_load, created_at, updated_at
		FROM executors WHERE executor_id = ?
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{executorID},
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

// TaskExecutionWithNames 包含任务名和执行器名的执行记录
type TaskExecutionWithNames struct {
	model.TaskExecution
	TaskName     string
	TaskType     string
	ExecutorName string
}

// GetAllExecutions 获取所有执行记录，支持筛选和分页
func (s *SchedulerService) GetAllExecutions(ctx context.Context, filter map[string]string, page int, pageSize int) ([]*TaskExecutionWithNames, int, error) {
	// 构建 WHERE 条件
	whereClause := " WHERE 1=1"
	var args []interface{}

	// 应用筛选条件
	if filter["executor_name"] != "" {
		whereClause += " AND e.name LIKE ?"
		args = append(args, "%"+filter["executor_name"]+"%")
	}
	if filter["task_name"] != "" {
		whereClause += " AND t.name LIKE ?"
		args = append(args, "%"+filter["task_name"]+"%")
	}
	if filter["task_type"] != "" {
		whereClause += " AND t.type = ?"
		args = append(args, filter["task_type"])
	}
	if filter["status"] != "" {
		whereClause += " AND te.status = ?"
		args = append(args, filter["status"])
	}

	// 统一使用 JOIN，简化逻辑
	joinClause := `
		FROM task_executions te
		LEFT JOIN tasks t ON te.task_id = t.id
		LEFT JOIN executors e ON te.executor_id = e.executor_id
	`

	// 1. 先获取总数
	countQuery := "SELECT COUNT(*) " + joinClause + whereClause
	var countQr rqlite.QueryResult
	var err error

	slog.Debug("GetAllExecutions: counting records", "filter", filter, "query", countQuery)

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
		exec.ExecutorID = rowString(row[3])
		exec.Status = rowString(row[4])

		// 处理 start_time
		if t, ok := row[5].(time.Time); ok {
			exec.StartTime = rqlite.NullTime{Time: t.UTC(), Valid: true}
		} else if s, ok := row[5].(string); ok && s != "" {
			parsed, err := time.Parse("2006-01-02 15:04:05", s)
			if err == nil {
				exec.StartTime = rqlite.NullTime{Time: parsed.UTC(), Valid: true}
			}
		}

		// 处理 end_time
		if t, ok := row[6].(time.Time); ok {
			exec.EndTime = rqlite.NullTime{Time: t.UTC(), Valid: true}
		} else if s, ok := row[6].(string); ok && s != "" {
			parsed, err := time.Parse("2006-01-02 15:04:05", s)
			if err == nil {
				exec.EndTime = rqlite.NullTime{Time: parsed.UTC(), Valid: true}
			}
		}

		exec.Output = rowString(row[7])
		exec.Error = rowString(row[8])
		exec.RetryTimes = int32(rowInt64(row[9]))

		// 处理 created_at
		if t, ok := row[10].(time.Time); ok {
			exec.CreatedAt = t.UTC()
		} else if s, ok := row[10].(string); ok && s != "" {
			parsed, err := time.Parse("2006-01-02 15:04:05", s)
			if err == nil {
				exec.CreatedAt = parsed.UTC()
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
	query := "SELECT execution_id FROM task_executions WHERE id = ?"
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
	deleteLogsQuery := "DELETE FROM task_logs WHERE execution_id = ?"
	deleteLogsStmt := rqlite.ParameterizedStatement{
		Query:     deleteLogsQuery,
		Arguments: []interface{}{executionID},
	}
	_, err = s.DB.WriteOneParameterized(deleteLogsStmt)
	if err != nil {
		slog.Warn("failed to delete related logs", "error", err)
	}

	// 删除执行记录
	deleteExecQuery := "DELETE FROM task_executions WHERE id = ?"
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
	query := "SELECT execution_id FROM task_executions WHERE id IN (" + strings.Join(placeholders, ",") + ")"
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

		deleteLogsQuery := "DELETE FROM task_logs WHERE execution_id IN (" + strings.Join(logPlaceholders, ",") + ")"
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
	deleteExecQuery := "DELETE FROM task_executions WHERE id IN (" + strings.Join(placeholders, ",") + ")"
	deleteExecStmt := rqlite.ParameterizedStatement{
		Query:     deleteExecQuery,
		Arguments: args,
	}
	_, err = s.DB.WriteOneParameterized(deleteExecStmt)
	return err
}

func (s *SchedulerService) GetTaskLogs(ctx context.Context, executionID string) ([]*model.TaskLog, error) {
	query := `
		SELECT id, execution_id, task_id, executor_id, node_id, log_level, message, log_time
		FROM task_logs WHERE execution_id = ?
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

func (s *SchedulerService) AddTaskLog(ctx context.Context, executionID string, taskID int64, nodeID string, logLevel string, message string) error {
	// 首先获取执行记录中的 executor_id
	var executorID string
	execQuery := `SELECT executor_id FROM task_executions WHERE execution_id = ? LIMIT 1`
	execStmt := rqlite.ParameterizedStatement{
		Query:     execQuery,
		Arguments: []interface{}{executionID},
	}
	execQr, err := s.DB.QueryOneParameterized(execStmt)
	if err == nil && execQr.Err == nil && execQr.Next() {
		row, _ := execQr.Slice()
		executorID = rowString(row[0])
	}

	// 尝试插入带 executor_id 的新表结构
	query := `
		INSERT INTO task_logs (execution_id, task_id, executor_id, node_id, log_level, message, log_time)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	nowUTC := time.Now().UTC().Format("2006-01-02 15:04:05")
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{executionID, taskID, executorID, nodeID, logLevel, message, nowUTC},
	}
	result, err := s.DB.WriteOneParameterized(stmt)
	
	// 如果失败，回退到旧表结构
	if err != nil || result.Err != nil {
		slog.Debug("Falling back to old insert format for task_logs")
		fallbackQuery := `
			INSERT INTO task_logs (execution_id, task_id, node_id, log_level, message, log_time)
			VALUES (?, ?, ?, ?, ?, ?)
		`
		fallbackStmt := rqlite.ParameterizedStatement{
			Query:     fallbackQuery,
			Arguments: []interface{}{executionID, taskID, nodeID, logLevel, message, nowUTC},
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
			UPDATE task_executions 
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
		FROM workflows WHERE id = ?
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

func (s *SchedulerService) ListWorkflows(ctx context.Context) ([]*model.Workflow, error) {
	query := `
		SELECT id, name, description, domain_id, dag_config, cron_expression,
		       is_enabled, created_by, created_at, updated_at
		FROM workflows ORDER BY created_at DESC
	`

	qr, err := s.DB.QueryOne(query)
	if err != nil {
		return nil, err
	}
	if qr.Err != nil {
		return nil, qr.Err
	}

	var workflows []*model.Workflow
	for qr.Next() {
		wf := &model.Workflow{}
		if err := scanWorkflowResult(&qr, wf); err != nil {
			return nil, err
		}
		workflows = append(workflows, wf)
	}

	return workflows, nil
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
		UPDATE workflows SET name = ?, description = ?, dag_config = ?, cron_expression = ?,
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
	query := `DELETE FROM workflows WHERE id = ?`
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
		INSERT INTO workflow_executions (workflow_id, execution_id, status, node_states, created_at)
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
		FROM workflow_executions WHERE id = ?
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
		FROM workflow_executions WHERE execution_id = ?
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
		FROM workflow_executions WHERE workflow_id = ?
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
		UPDATE workflow_executions
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
	query := `UPDATE workflow_executions SET node_states = ? WHERE execution_id = ?`

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
		nodeStatesJSON, _ := json.Marshal(nodeStates)
		s.UpdateWorkflowExecutionNodeStates(ctx, executionID, string(nodeStatesJSON))
		s.AddTaskLog(ctx, executionID, 0, nodeID, "info", fmt.Sprintf("Node %s started", node.Name))

		time.Sleep(1 * time.Second)

		nodeStates[nodeID] = "success"
		nodeStatesJSON, _ = json.Marshal(nodeStates)
		s.UpdateWorkflowExecutionNodeStates(ctx, executionID, string(nodeStatesJSON))
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
	task.WebhookConfig = rowString(row[12])
	task.AssignedExecutorID = rowString(row[13])
	task.CreatedBy = rowInt64(row[14])
	if t, ok := row[15].(time.Time); ok {
		task.CreatedAt = t
	}
	if t, ok := row[16].(time.Time); ok {
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
	exec.ExecutorID = rowString(row[1])
	exec.Name = rowString(row[2])
	exec.Address = rowString(row[3])
	exec.Status = rowString(row[4])
	if t, ok := row[5].(time.Time); ok {
		exec.LastHeartbeat = rqlite.NullTime{Time: t, Valid: true}
	}
	exec.Capacity = rowInt64(row[6])
	exec.CurrentLoad = rowInt64(row[7])
	if t, ok := row[8].(time.Time); ok {
		exec.CreatedAt = t
	}
	if t, ok := row[9].(time.Time); ok {
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
	exec.ExecutorID = rowString(row[3])
	exec.Status = rowString(row[4])
	
	// 处理 start_time
	if t, ok := row[5].(time.Time); ok {
		exec.StartTime = rqlite.NullTime{Time: t.UTC(), Valid: true}
	} else if s, ok := row[5].(string); ok && s != "" {
		parsed, err := time.Parse("2006-01-02 15:04:05", s)
		if err == nil {
			exec.StartTime = rqlite.NullTime{Time: parsed.UTC(), Valid: true}
		}
	}
	
	// 处理 end_time
	if t, ok := row[6].(time.Time); ok {
		exec.EndTime = rqlite.NullTime{Time: t.UTC(), Valid: true}
	} else if s, ok := row[6].(string); ok && s != "" {
		parsed, err := time.Parse("2006-01-02 15:04:05", s)
		if err == nil {
			exec.EndTime = rqlite.NullTime{Time: parsed.UTC(), Valid: true}
		}
	}
	
	exec.Output = rowString(row[7])
	exec.Error = rowString(row[8])
	exec.RetryTimes = int32(rowInt64(row[9]))
	
	// 处理 created_at
	if t, ok := row[10].(time.Time); ok {
		exec.CreatedAt = t.UTC()
	} else if s, ok := row[10].(string); ok && s != "" {
		parsed, err := time.Parse("2006-01-02 15:04:05", s)
		if err == nil {
			exec.CreatedAt = parsed.UTC()
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
		we.StartTime = rqlite.NullTime{Time: t.UTC(), Valid: true}
	} else if s, ok := row[4].(string); ok && s != "" {
		parsed, err := time.Parse("2006-01-02 15:04:05", s)
		if err == nil {
			we.StartTime = rqlite.NullTime{Time: parsed.UTC(), Valid: true}
		}
	}
	
	// 处理 end_time
	if t, ok := row[5].(time.Time); ok {
		we.EndTime = rqlite.NullTime{Time: t.UTC(), Valid: true}
	} else if s, ok := row[5].(string); ok && s != "" {
		parsed, err := time.Parse("2006-01-02 15:04:05", s)
		if err == nil {
			we.EndTime = rqlite.NullTime{Time: parsed.UTC(), Valid: true}
		}
	}
	
	we.NodeStates = rowString(row[6])
	
	// 处理 created_at
	if t, ok := row[7].(time.Time); ok {
		we.CreatedAt = t.UTC()
	} else if s, ok := row[7].(string); ok && s != "" {
		parsed, err := time.Parse("2006-01-02 15:04:05", s)
		if err == nil {
			we.CreatedAt = parsed.UTC()
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
	tl.ExecutorID = rowString(row[3])
	tl.NodeID = rowString(row[4])
	tl.LogLevel = rowString(row[5])
	tl.Message = rowString(row[6])
	
	// 处理 log_time
	if t, ok := row[7].(time.Time); ok {
		tl.LogTime = t.UTC()
	} else if s, ok := row[7].(string); ok && s != "" {
		parsed, err := time.Parse("2006-01-02 15:04:05", s)
		if err == nil {
			tl.LogTime = parsed.UTC()
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
	case float64:
		return int64(val)
	case string:
		var n int64
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
func (s *SchedulerService) GetExecutionStats(ctx context.Context, filter map[string]string) (map[string]int, error) {
	// 构建 WHERE 条件
	whereClause := " WHERE 1=1"
	var args []interface{}

	// 应用筛选条件
	if filter["executor_name"] != "" {
		whereClause += " AND e.name LIKE ?"
		args = append(args, "%"+filter["executor_name"]+"%")
	}
	if filter["task_name"] != "" {
		whereClause += " AND t.name LIKE ?"
		args = append(args, "%"+filter["task_name"]+"%")
	}
	if filter["task_type"] != "" {
		whereClause += " AND t.type = ?"
		args = append(args, filter["task_type"])
	}
	if filter["status"] != "" {
		whereClause += " AND te.status = ?"
		args = append(args, filter["status"])
	}

	// 统一使用 JOIN
	joinClause := `
		FROM task_executions te
		LEFT JOIN tasks t ON te.task_id = t.id
		LEFT JOIN executors e ON te.executor_id = e.executor_id
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
