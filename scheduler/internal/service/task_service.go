package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	pb "github.com/lynnyq/bdopsflow/proto"
	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	rqlite "github.com/rqlite/gorqlite"
)

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

	if s.cronScheduler != nil && s.IsLeader() && task.IsEnabled && task.CronExpression != "" {
		s.cronScheduler.RegisterTask(task.ID, task.CronExpression)
	} else if s.cronScheduler != nil && !s.IsLeader() && s.redis != nil {
		s.redis.Set(ctx, "cron:needs_reload", time.Now().Unix(), 5*time.Minute)
	}

	return task, nil
}

func (s *SchedulerService) GetTaskByID(ctx context.Context, id int64) (*model.Task, error) {
	query := `
		SELECT id, name, type, config, cron_expression, timeout_seconds,
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

func (s *SchedulerService) ListTasks(ctx context.Context, domainID int64, role string, page, pageSize int) ([]*model.Task, int, error) {
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	if page <= 0 {
		page = 1
	}

	isSystemAdmin := role == "system_admin" || role == "admin"

	var whereClause string
	var args []interface{}

	if isSystemAdmin {
		whereClause = ""
	} else {
		whereClause = " WHERE domain_id = ?"
		args = append(args, domainID)
	}

	countQuery := "SELECT COUNT(*) FROM bdopsflow_tasks" + whereClause
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
		return nil, 0, err
	}
	if countQr.Err != nil {
		return nil, 0, countQr.Err
	}

	var total int
	if countQr.Next() {
		row, _ := countQr.Slice()
		total = int(rowInt64(row[0]))
	}

	offset := (page - 1) * pageSize
	dataQuery := `
		SELECT t.id, t.name, t.type, t.config, t.cron_expression, t.timeout_seconds,
		       t.retry_count, t.retry_interval, t.is_enabled, t.status, t.domain_id, t.webhook_id, t.webhook_events,
		       t.assigned_executor_id, t.created_by, u.real_name as created_by_name, t.created_at, t.updated_at
		FROM bdopsflow_tasks t
		LEFT JOIN bdopsflow_users u ON t.created_by = u.id` + whereClause + " ORDER BY t.created_at DESC LIMIT ? OFFSET ?"

	dataArgs := make([]interface{}, len(args))
	copy(dataArgs, args)
	dataArgs = append(dataArgs, pageSize, offset)

	var qr rqlite.QueryResult
	if len(dataArgs) > 0 {
		stmt := rqlite.ParameterizedStatement{
			Query:     dataQuery,
			Arguments: dataArgs,
		}
		qr, err = s.DB.QueryOneParameterized(stmt)
	} else {
		qr, err = s.DB.QueryOne(dataQuery)
	}

	if err != nil {
		return nil, 0, err
	}
	if qr.Err != nil {
		return nil, 0, qr.Err
	}

	var bdopsflow_tasks []*model.Task
	for qr.Next() {
		task := &model.Task{}
		if err := scanTaskResult(&qr, task); err != nil {
			return nil, 0, err
		}
		task.NextExecutionTime = CalculateNextExecutionTime(task.CronExpression, task.IsEnabled)
		task.LastExecutionStatus = s.getLastExecutionStatus(ctx, task.ID)
		bdopsflow_tasks = append(bdopsflow_tasks, task)
	}

	return bdopsflow_tasks, total, nil
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
			isEnabled, webhookID, task.WebhookEvents, assignedExecutorID, time.Now().Format(DateTimeFormat), id,
		},
	}

	result, err := s.DB.WriteOneParameterized(stmt)
	if err != nil {
		return err
	}

	if result.Err != nil {
		return result.Err
	}

	updatedTask, err := s.GetTaskByID(ctx, id)
	if err != nil {
		slog.Error("UpdateTask: failed to get updated task", "id", id, "error", err)
		return err
	}

	if s.cronScheduler != nil && s.IsLeader() {
		if updatedTask.IsEnabled && updatedTask.CronExpression != "" {
			s.cronScheduler.RegisterTask(id, updatedTask.CronExpression)
			slog.Info("UpdateTask: task registered to cron", "id", id, "cron", updatedTask.CronExpression)
		} else {
			s.cronScheduler.UnregisterTask(id)
			slog.Info("UpdateTask: task unregistered from cron", "id", id)
		}
	} else if s.cronScheduler != nil && !s.IsLeader() && s.redis != nil {
		s.redis.Set(ctx, "cron:needs_reload", time.Now().Unix(), 5*time.Minute)
		slog.Info("UpdateTask: set cron reload flag for leader", "id", id)
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

	if s.cronScheduler != nil && s.IsLeader() {
		s.cronScheduler.UnregisterTask(id)
	} else if s.cronScheduler != nil && !s.IsLeader() && s.redis != nil {
		s.redis.Set(ctx, "cron:needs_reload", time.Now().Unix(), 5*time.Minute)
	}

	return nil
}

func (s *SchedulerService) TriggerTask(ctx context.Context, taskID int64) (string, error) {
	if !s.IsLeader() {
		return "", fmt.Errorf("this node is not the leader, cannot trigger task")
	}

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

		renewKey := fmt.Sprintf("task:renew:%s", runningExecID)
		lastRenewStr, renewErr := s.redis.Get(ctx, renewKey).Result()
		if renewErr != nil || lastRenewStr == "" {
			slog.Warn("found running task with no renewal, force failing stale execution",
				"task_id", taskID,
				"execution_id", runningExecID,
			)
			s.UpdateExecutionResult(ctx, runningExecID, "failed", "", "stale execution cleaned up on new trigger")
			s.AddTaskLog(ctx, runningExecID, taskID, "", "error", "stale execution cleaned up on new trigger")
			s.HandleTaskFailure(ctx, taskID, runningExecID, "", "stale execution cleaned up on new trigger")
			lockKey := fmt.Sprintf("task:lock:%s", runningExecID)
			failCountKey := fmt.Sprintf("task:renew:fail:count:%s", runningExecID)
			s.redis.Del(ctx, lockKey, renewKey, failCountKey)
		} else {
			var lastRenew int64
			fmt.Sscanf(lastRenewStr, "%d", &lastRenew)
			lastRenewSecondsAgo := time.Now().Unix() - lastRenew

			task, taskErr := s.GetTaskByID(ctx, taskID)
			timeoutSeconds := int64(300)
			noTimeout := false
			if taskErr == nil && task.TimeoutSeconds > 0 {
				timeoutSeconds = int64(task.TimeoutSeconds)
			} else if taskErr == nil && task.TimeoutSeconds <= 0 {
				noTimeout = true
			}

			if !noTimeout && lastRenewSecondsAgo > timeoutSeconds {
				slog.Warn("found running task with expired renewal, force failing stale execution",
					"task_id", taskID,
					"execution_id", runningExecID,
					"last_renew_seconds_ago", lastRenewSecondsAgo,
					"timeout_seconds", timeoutSeconds,
				)
				s.UpdateExecutionResult(ctx, runningExecID, "failed", "", "stale execution cleaned up on new trigger")
				s.AddTaskLog(ctx, runningExecID, taskID, "", "error", "stale execution cleaned up on new trigger")
				s.HandleTaskFailure(ctx, taskID, runningExecID, "", "stale execution cleaned up on new trigger")
				lockKey := fmt.Sprintf("task:lock:%s", runningExecID)
				failCountKey := fmt.Sprintf("task:renew:fail:count:%s", runningExecID)
				s.redis.Del(ctx, lockKey, renewKey, failCountKey)
			} else {
				skippedExecutionID := fmt.Sprintf("exec-%d-%d", taskID, time.Now().UnixNano())
				now := time.Now().Format(DateTimeFormat)
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

				if _, dbErr := s.DB.WriteOneParameterized(insertStmt); dbErr != nil {
					slog.Warn("failed to record skipped execution", "task_id", taskID, "error", dbErr)
				} else {
					slog.Warn("task skipped: previous execution still running",
						"task_id", taskID,
						"skipped_execution_id", skippedExecutionID,
						"running_execution_id", runningExecID,
					)

					s.AddTaskLog(ctx, skippedExecutionID, taskID, "", "warn", skippedReason)
					s.SendWebhookNotification(ctx, taskID, skippedExecutionID, "skipped", "", skippedReason, 0)
				}

				return "", fmt.Errorf("task %d is already running (execution_id: %s), skipped", taskID, runningExecID)
			}
		}
	}

	task, err := s.GetTaskByID(ctx, taskID)
	if err != nil {
		return "", fmt.Errorf("get task failed: %w", err)
	}

	executionID := fmt.Sprintf("exec-%d-%d", taskID, time.Now().UnixNano())

	lockKey := fmt.Sprintf("task:lock:%s", executionID)
	lockValue := fmt.Sprintf("%d", time.Now().UnixNano())

	lockTTL := time.Duration(task.TimeoutSeconds) * 2 * time.Second
	if lockTTL < 60*time.Second {
		lockTTL = 60 * time.Second
	}
	if lockTTL > 7200*time.Second {
		lockTTL = 7200 * time.Second
	}
	if task.TimeoutSeconds == 0 {
		lockTTL = 3600 * time.Second
	}

	lockSet, err := s.redis.SetNX(ctx, lockKey, lockValue, lockTTL).Result()
	if err != nil {
		slog.Warn("acquire lock failed, continuing anyway", "error", err)
	} else if !lockSet {
		return "", fmt.Errorf("task %d is already being executed (lock conflict)", taskID)
	}

	now := time.Now().Format(DateTimeFormat)
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

	var executor *model.Executor
	if task.AssignedExecutorID > 0 {
		executor, err = s.GetExecutorByID(ctx, task.AssignedExecutorID)
		if err != nil {
			errMsg := fmt.Sprintf("specified executor %d not found: %v", task.AssignedExecutorID, err)
			slog.Error("specified executor not found",
				"task_id", taskID,
				"assigned_executor_id", task.AssignedExecutorID,
				"error", err,
			)
			s.UpdateExecutionResult(ctx, executionID, "failed", "", errMsg)
			s.UpdateTaskStatusByID(ctx, taskID, "failed")
			s.AddTaskLog(ctx, executionID, taskID, "", "error", errMsg)
			s.SendWebhookNotification(ctx, taskID, executionID, "failed", "", errMsg, 0)
			s.redis.Del(ctx, lockKey)
			return executionID, fmt.Errorf("specified executor %d not found: %w", task.AssignedExecutorID, err)
		}

		if executor.Status != "online" {
			errMsg := fmt.Sprintf("specified executor %d is not online (status: %s)", task.AssignedExecutorID, executor.Status)
			slog.Error("specified executor is not online",
				"task_id", taskID,
				"assigned_executor_id", task.AssignedExecutorID,
				"executor_status", executor.Status,
			)
			s.UpdateExecutionResult(ctx, executionID, "failed", "", errMsg)
			s.UpdateTaskStatusByID(ctx, taskID, "failed")
			s.AddTaskLog(ctx, executionID, taskID, "", "error", errMsg)
			s.SendWebhookNotification(ctx, taskID, executionID, "failed", "", errMsg, 0)
			s.redis.Del(ctx, lockKey)
			return executionID, fmt.Errorf("specified executor %d is not online", task.AssignedExecutorID)
		}

		if executor.CurrentLoad >= executor.Capacity {
			errMsg := fmt.Sprintf("specified executor %d has no capacity (load: %d/%d)", task.AssignedExecutorID, executor.CurrentLoad, executor.Capacity)
			slog.Error("specified executor has no capacity",
				"task_id", taskID,
				"assigned_executor_id", task.AssignedExecutorID,
				"current_load", executor.CurrentLoad,
				"capacity", executor.Capacity,
			)
			s.UpdateExecutionResult(ctx, executionID, "failed", "", errMsg)
			s.UpdateTaskStatusByID(ctx, taskID, "failed")
			s.AddTaskLog(ctx, executionID, taskID, "", "error", errMsg)
			s.SendWebhookNotification(ctx, taskID, executionID, "failed", "", errMsg, 0)
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
		executor, err = s.SelectAvailableExecutor(ctx, task.DomainID)
		if err != nil {
			errMsg := fmt.Sprintf("no available executor: %v", err)
			slog.Error("no available executor", "task_id", taskID, "domain_id", task.DomainID, "error", err)
			s.UpdateExecutionResult(ctx, executionID, "failed", "", errMsg)
			s.UpdateTaskStatusByID(ctx, taskID, "failed")
			s.AddTaskLog(ctx, executionID, taskID, "", "error", errMsg)
			s.SendWebhookNotification(ctx, taskID, executionID, "failed", "", errMsg, 0)
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
		errMsg := "dispatcher not configured"
		s.UpdateExecutionResult(ctx, executionID, "failed", "", errMsg)
		s.UpdateTaskStatusByID(ctx, taskID, "failed")
		s.AddTaskLog(ctx, executionID, taskID, "", "error", errMsg)
		s.SendWebhookNotification(ctx, taskID, executionID, "failed", "", errMsg, 0)
		s.redis.Del(ctx, lockKey)
		return executionID, fmt.Errorf("dispatcher not configured")
	}

	updateExecutorQuery := `UPDATE bdopsflow_task_executions SET executor_id = ? WHERE execution_id = ?`
	updateExecutorStmt := rqlite.ParameterizedStatement{
		Query:     updateExecutorQuery,
		Arguments: []interface{}{executor.ID, executionID},
	}
	_, err = s.DB.WriteOneParameterized(updateExecutorStmt)
	if err != nil {
		slog.Warn("failed to update executor_id in bdopsflow_task_executions", "error", err, "execution_id", executionID)
	}

	if err := s.dispatcher(executor.Name, grpcTask); err != nil {
		errMsg := fmt.Sprintf("dispatch failed: %v", err)
		slog.Error("dispatch task failed", "task_id", taskID, "executor", executor.Name, "error", err)
		s.UpdateExecutionResult(ctx, executionID, "failed", "", errMsg)
		s.UpdateTaskStatusByID(ctx, taskID, "failed")
		s.AddTaskLog(ctx, executionID, taskID, "", "error", errMsg)
		s.SendWebhookNotification(ctx, taskID, executionID, "failed", "", errMsg, 0)
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
	if lockTTL > 7200*time.Second {
		lockTTL = 7200 * time.Second
	}
	if task.TimeoutSeconds == 0 {
		lockTTL = 3600 * time.Second
	}

	lockSet, err := s.redis.SetNX(ctx, lockKey, lockValue, lockTTL).Result()
	if err != nil {
		slog.Warn("acquire lock failed, continuing anyway", "error", err)
	} else if !lockSet {
		return "", fmt.Errorf("task %d is already being executed (lock conflict)", taskID)
	}

	now := time.Now().Format(DateTimeFormat)
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
			errMsg := "specified executor unavailable for retry"
			s.UpdateExecutionResult(ctx, executionID, "failed", "", errMsg)
			s.UpdateTaskStatusByID(ctx, taskID, "failed")
			s.AddTaskLog(ctx, executionID, taskID, "", "error", errMsg)
			s.SendWebhookNotification(ctx, taskID, executionID, "failed", "", "retry failed: specified executor unavailable", 0)
			s.redis.Del(ctx, lockKey)
			return executionID, fmt.Errorf("specified executor unavailable for retry")
		}
	} else {
		executor, err = s.SelectAvailableExecutor(ctx, task.DomainID)
		if err != nil {
			errMsg := fmt.Sprintf("no available executor: %v", err)
			s.UpdateExecutionResult(ctx, executionID, "failed", "", errMsg)
			s.UpdateTaskStatusByID(ctx, taskID, "failed")
			s.AddTaskLog(ctx, executionID, taskID, "", "error", errMsg)
			s.SendWebhookNotification(ctx, taskID, executionID, "failed", "", fmt.Sprintf("retry failed: %s", errMsg), 0)
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
		errMsg := "dispatcher not configured"
		s.UpdateExecutionResult(ctx, executionID, "failed", "", errMsg)
		s.UpdateTaskStatusByID(ctx, taskID, "failed")
		s.AddTaskLog(ctx, executionID, taskID, "", "error", errMsg)
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
		errMsg := fmt.Sprintf("dispatch failed: %v", err)
		s.UpdateExecutionResult(ctx, executionID, "failed", "", errMsg)
		s.UpdateTaskStatusByID(ctx, taskID, "failed")
		s.AddTaskLog(ctx, executionID, taskID, "", "error", errMsg)
		s.SendWebhookNotification(ctx, taskID, executionID, "failed", "", fmt.Sprintf("retry failed: %s", errMsg), 0)
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

func (s *SchedulerService) HandleTaskFailure(ctx context.Context, taskID int64, failedExecutionID, output, errorMsg string) error {
	task, err := s.GetTaskByID(ctx, taskID)
	if err != nil {
		slog.Error("HandleTaskFailure: failed to get task", "task_id", taskID, "error", err)
		s.UpdateTaskStatusByID(ctx, taskID, "failed")
		s.SendWebhookNotification(ctx, taskID, failedExecutionID, "failed", output, errorMsg, 0)
		return err
	}

	maxRetries := int(task.RetryCount)
	if maxRetries <= 0 {
		slog.Info("HandleTaskFailure: no retries configured, marking as failed",
			"task_id", taskID,
			"failed_execution_id", failedExecutionID,
		)
		s.UpdateTaskStatusByID(ctx, taskID, "failed")
		s.SendWebhookNotification(ctx, taskID, failedExecutionID, "failed", output, errorMsg, 0)
		return nil
	}

	retryLockKey := fmt.Sprintf("task:retry:lock:%d", taskID)
	retryLockSet, retryLockErr := s.redis.SetNX(ctx, retryLockKey, "locked", 30*time.Minute).Result()
	if retryLockErr != nil {
		slog.Warn("HandleTaskFailure: failed to acquire retry lock, proceeding anyway",
			"task_id", taskID, "error", retryLockErr)
		retryLockSet = true
	}
	if !retryLockSet {
		slog.Warn("HandleTaskFailure: retry already in progress for task, skipping",
			"task_id", taskID,
			"failed_execution_id", failedExecutionID,
		)
		return nil
	}

	currentRetryTimes, err := s.getRetryTimesForExecution(ctx, taskID, failedExecutionID)
	if err != nil {
		slog.Error("HandleTaskFailure: failed to get retry times",
			"task_id", taskID,
			"failed_execution_id", failedExecutionID,
			"error", err,
		)
		s.redis.Del(ctx, retryLockKey)
		s.UpdateTaskStatusByID(ctx, taskID, "failed")
		s.SendWebhookNotification(ctx, taskID, failedExecutionID, "failed", output, errorMsg, 0)
		return err
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
			defer s.redis.Del(context.Background(), retryLockKey)

			time.Sleep(time.Duration(task.RetryInterval) * time.Second)
			if !s.IsLeader() {
				slog.Warn("HandleTaskFailure: lost leadership before retry, aborting",
					"task_id", taskID,
					"retry_times", retryTimes,
				)
				s.UpdateTaskStatusByID(context.Background(), taskID, "failed")
				s.SendWebhookNotification(context.Background(), taskID, failedExecutionID, "failed", output, fmt.Sprintf("retry %d aborted: lost leadership", retryTimes), 0)
				return
			}

			slog.Info("HandleTaskFailure: executing retry",
				"task_id", taskID,
				"retry_times", retryTimes,
			)

			newExecutionID, err := s.RetryTask(context.Background(), taskID, retryTimes)
			if err != nil {
				slog.Error("HandleTaskFailure: retry failed",
					"task_id", taskID,
					"retry_times", retryTimes,
					"error", err,
				)
				s.UpdateTaskStatusByID(context.Background(), taskID, "failed")
				s.SendWebhookNotification(context.Background(), taskID, failedExecutionID, "failed", output, fmt.Sprintf("retry %d failed: %v", retryTimes, err), 0)
			} else {
				slog.Info("HandleTaskFailure: retry scheduled successfully",
					"task_id", taskID,
					"retry_times", retryTimes,
					"new_execution_id", newExecutionID,
				)
			}
		}()
	} else {
		slog.Info("HandleTaskFailure: max retries reached, marking as failed",
			"task_id", taskID,
			"failed_execution_id", failedExecutionID,
			"retry_times", currentRetryTimes,
			"max_retries", maxRetries,
		)
		s.redis.Del(ctx, retryLockKey)
		s.UpdateTaskStatusByID(ctx, taskID, "failed")
		s.SendWebhookNotification(ctx, taskID, failedExecutionID, "failed", output, errorMsg, 0)
	}

	return nil
}

func (s *SchedulerService) getRetryTimesForExecution(ctx context.Context, taskID int64, executionID string) (int, error) {
	query := `
		SELECT retry_times FROM bdopsflow_task_executions
		WHERE execution_id = ?
	`
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{executionID},
	}
	qr, err := s.DB.QueryOneParameterized(stmt)
	if err != nil {
		return 0, err
	}
	if qr.Err != nil {
		return 0, qr.Err
	}

	if !qr.Next() {
		return 0, fmt.Errorf("execution %s not found", executionID)
	}

	row, err := qr.Slice()
	if err != nil {
		return 0, err
	}

	return int(rowInt64(row[0])), nil
}

func (s *SchedulerService) UpdateTaskStatusByID(ctx context.Context, taskID int64, status string) error {
	task, err := s.GetTaskByID(ctx, taskID)
	if err == nil && task.CronExpression != "" {
		query := `UPDATE bdopsflow_tasks SET updated_at = ? WHERE id = ?`
		stmt := rqlite.ParameterizedStatement{
			Query:     query,
			Arguments: []interface{}{time.Now().Format(DateTimeFormat), taskID},
		}
		result, err := s.DB.WriteOneParameterized(stmt)
		if err != nil {
			return err
		}
		return result.Err
	}

	query := `UPDATE bdopsflow_tasks SET status = ?, updated_at = ? WHERE id = ?`
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{status, time.Now().Format(DateTimeFormat), taskID},
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
		SELECT id, name, type, config, cron_expression, timeout_seconds,
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

func (s *SchedulerService) GetTaskInfoByID(ctx context.Context, taskID int64) (*model.Task, error) {
	return s.GetTaskByID(ctx, taskID)
}
