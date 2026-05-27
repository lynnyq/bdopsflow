package service

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	rqlite "github.com/rqlite/gorqlite"
)

func (s *SchedulerService) StartCleanupRoutine() {
	go s.cleanupStuckTasks()
}

func (s *SchedulerService) cleanupStuckTasks() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	slog.Info("stuck task cleanup routine started")

	for {
		select {
		case <-s.stopCleanupCh:
			slog.Info("stuck task cleanup routine stopped")
			return
		case <-ticker.C:
			if !s.IsLeader() {
				continue
			}
			s.checkCronReload()
			s.cleanupDeadTasks()
			s.cleanupOfflineExecutors()
			s.cleanupStaleTaskLocks()
		}
	}
}

func (s *SchedulerService) cleanupDeadTasks() {
	ctx, cancel := cleanupCtx()
	defer cancel()

	createdBefore := time.Now().Add(-5 * time.Minute).Format(DateTimeFormat)
	query := `
		SELECT te.id, te.task_id, te.execution_id, te.start_time, te.created_at,
		       COALESCE(t.timeout_seconds, 300) AS timeout_seconds,
		       te.executor_id,
		       e.status AS executor_status
		FROM bdopsflow_task_executions te
		JOIN bdopsflow_tasks t ON te.task_id = t.id
		LEFT JOIN bdopsflow_executors e ON te.executor_id = e.id
		WHERE te.status = 'running'
		AND (te.start_time IS NOT NULL AND te.start_time != '')
		AND te.created_at < ?
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{createdBefore},
	}
	qr, err := s.DB.QueryOneParameterized(stmt)
	if err != nil {
		slog.Error("cleanup: query stuck bdopsflow_tasks failed", "error", err)
		return
	}
	if qr.Err != nil {
		slog.Error("cleanup: query returned error", "error", qr.Err)
		return
	}

	type stuckExec struct {
		ID             int64
		TaskID         int64
		ExecutionID    string
		StartTime      string
		TimeoutSeconds int64
		ExecutorID     int64
		NoTimeout      bool
		ExecutorStatus string
	}

	var stuckExecutions []stuckExec

	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			continue
		}
		rawTimeout := rowInt64(row[5])
		stuck := stuckExec{
			ID:             rowInt64(row[0]),
			TaskID:         rowInt64(row[1]),
			ExecutionID:    rowString(row[2]),
			StartTime:      rowString(row[3]),
			TimeoutSeconds: rawTimeout,
			ExecutorID:     rowInt64(row[6]),
			NoTimeout:      rawTimeout <= 0,
			ExecutorStatus: rowString(row[7]),
		}
		if stuck.TimeoutSeconds <= 0 {
			stuck.TimeoutSeconds = 300
		}
		stuckExecutions = append(stuckExecutions, stuck)
	}

	if len(stuckExecutions) == 0 {
		return
	}

	slog.Warn("found stuck bdopsflow_tasks", "count", len(stuckExecutions))

	renewKeys := make([]string, 0, len(stuckExecutions))
	for _, exec := range stuckExecutions {
		renewKeys = append(renewKeys, fmt.Sprintf("task:renew:%s", exec.ExecutionID))
	}

	renewValues := make(map[string]string)
	if len(renewKeys) > 0 {
		pipe := s.redis.Pipeline()
		cmds := make([]*redis.StringCmd, len(renewKeys))
		for i, key := range renewKeys {
			cmds[i] = pipe.Get(ctx, key)
		}
		_, _ = pipe.Exec(ctx)
		for i, cmd := range cmds {
			if val, err := cmd.Result(); err == nil {
				renewValues[renewKeys[i]] = val
			}
		}
	}

	executorIDs := make(map[int64]bool)
	for _, exec := range stuckExecutions {
		if exec.ExecutorID > 0 && exec.ExecutorStatus != "online" {
			executorIDs[exec.ExecutorID] = true
		}
	}

	onlineExecutorIDs := make(map[int64]bool)
	if len(executorIDs) > 0 {
		ids := make([]string, 0, len(executorIDs))
		for id := range executorIDs {
			ids = append(ids, strconv.FormatInt(id, 10))
		}
		onlineQuery := fmt.Sprintf(
			"SELECT id FROM bdopsflow_executors WHERE id IN (%s) AND status = 'online'",
			strings.Join(ids, ","),
		)
		if oqr, oerr := s.DB.QueryOne(onlineQuery); oerr == nil && oqr.Err == nil {
			for oqr.Next() {
				row, _ := oqr.Slice()
				onlineExecutorIDs[rowInt64(row[0])] = true
			}
		}
	}

	for _, exec := range stuckExecutions {
		renewKey := fmt.Sprintf("task:renew:%s", exec.ExecutionID)
		lastRenewStr, hasRenewal := renewValues[renewKey]
		noRenewal := !hasRenewal || lastRenewStr == ""

		var lastRenewSecondsAgo int64 = 9999
		if !noRenewal {
			var lastRenew int64
			fmt.Sscanf(lastRenewStr, "%d", &lastRenew)
			lastRenewSecondsAgo = time.Now().Unix() - lastRenew
		}

		executorOnline := true
		if exec.ExecutorID > 0 {
			if onlineExecutorIDs[exec.ExecutorID] {
				executorOnline = true
			} else if exec.ExecutorStatus == "online" {
				executorOnline = true
			} else {
				executorOnline = false
			}
		}

		executorReachable := true
		if executorOnline && exec.ExecutorID > 0 {
			executor, execErr := s.GetExecutorByID(ctx, exec.ExecutorID)
			if execErr != nil {
				executorReachable = false
			} else {
				executorReachable = s.pingExecutor(ctx, executor)
				if !executorReachable {
					slog.Warn("cleanup: executor is online in DB but unreachable via ping",
						"execution_id", exec.ExecutionID,
						"task_id", exec.TaskID,
						"executor_id", exec.ExecutorID,
						"executor_name", executor.Name,
						"executor_address", executor.Address,
					)
				}
			}
		}

		taskTimeout := false
		if !exec.NoTimeout && exec.StartTime != "" {
			if startTime, parseErr := parseTimeInLocalTimezone(exec.StartTime); parseErr == nil {
				if time.Since(startTime) > time.Duration(exec.TimeoutSeconds)*time.Second {
					taskTimeout = true
				}
			}
		}

		lockKey := fmt.Sprintf("task:lock:%s", exec.ExecutionID)
		lockExists, _ := s.redis.Exists(ctx, lockKey).Result()

		renewalExpired := !exec.NoTimeout && lastRenewSecondsAgo > exec.TimeoutSeconds
		shouldFail := !executorOnline || !executorReachable || noRenewal || renewalExpired || taskTimeout

		if shouldFail {
			var reason string
			if !executorOnline {
				reason = "executor is offline"
			} else if !executorReachable {
				reason = "executor is unreachable (ping failed)"
			} else if noRenewal {
				reason = "no renewal record found"
			} else if renewalExpired {
				reason = fmt.Sprintf("renewal expired (%d seconds ago, timeout %d)", lastRenewSecondsAgo, exec.TimeoutSeconds)
			} else {
				reason = "task execution timeout"
			}

			slog.Warn("cleanup: task is stuck, force updating status to failed",
				"task_id", exec.TaskID,
				"execution_id", exec.ExecutionID,
				"start_time", exec.StartTime,
				"lock_exists", lockExists,
				"no_renewal", noRenewal,
				"executor_online", executorOnline,
				"task_timeout", taskTimeout,
				"last_renew_seconds_ago", lastRenewSecondsAgo,
				"timeout_seconds", exec.TimeoutSeconds,
				"reason", reason,
			)

			err := s.UpdateExecutionResult(ctx, exec.ExecutionID, "failed", "", fmt.Sprintf("task stuck: %s", reason))
			if err != nil {
				slog.Error("cleanup: failed to update execution result", "error", err, "execution_id", exec.ExecutionID)
			}

			failCountKey := fmt.Sprintf("task:renew:fail:count:%s", exec.ExecutionID)
			s.redis.Del(ctx, lockKey, renewKey, failCountKey)

			go func(taskID int64, executionID string, failReason string) {
				if err := s.HandleTaskFailure(context.Background(), taskID, executionID, "", failReason); err != nil {
					slog.Error("cleanupDeadTasks: HandleTaskFailure failed",
						"execution_id", executionID,
						"task_id", taskID,
						"error", err,
					)
				}
			}(exec.TaskID, exec.ExecutionID, fmt.Sprintf("task stuck: %s", reason))
		} else {
			slog.Warn("cleanup: task has recent renewal, skipping",
				"task_id", exec.TaskID,
				"execution_id", exec.ExecutionID,
				"last_renew_seconds_ago", lastRenewSecondsAgo,
			)
		}
	}
}

func (s *SchedulerService) cleanupOfflineExecutors() {
	ctx, cancel := cleanupCtx()
	defer cancel()

	now := time.Now().Format(DateTimeFormat)
	heartbeatCutoff := time.Now().Add(-45 * time.Second).Format(DateTimeFormat)
	result, err := s.DB.WriteOneParameterized(rqlite.ParameterizedStatement{
		Query: `
		UPDATE bdopsflow_executors
		SET status = 'offline', updated_at = ?
		WHERE status = 'online'
		AND last_heartbeat < ?
	`,
		Arguments: []interface{}{now, heartbeatCutoff},
	})
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
	}

	s.cleanupTasksFromOfflineExecutors(ctx)
}

func (s *SchedulerService) cleanupTasksFromOfflineExecutors(ctx context.Context) {
	heartbeatCutoff := time.Now().Add(-45 * time.Second).Format(DateTimeFormat)
	query := `
		SELECT te.id, te.task_id, te.execution_id, te.start_time
		FROM bdopsflow_task_executions te
		LEFT JOIN bdopsflow_executors e ON te.executor_id = e.id
		WHERE te.status = 'running'
		  AND (
		    e.status = 'offline'
		    OR e.last_heartbeat < ?
		    OR te.executor_id = 0
		    OR te.executor_id IS NULL
		  )
	`

	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{heartbeatCutoff},
	}
	qr, err := s.DB.QueryOneParameterized(stmt)
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
		renewKey := fmt.Sprintf("task:renew:%s", executionID)
		failCountKey := fmt.Sprintf("task:renew:fail:count:%s", executionID)
		s.redis.Del(ctx, lockKey, renewKey, failCountKey)

		go func(tid int64, eid string) {
			if err := s.HandleTaskFailure(context.Background(), tid, eid, "", "executor went offline during task execution"); err != nil {
				slog.Error("cleanupTasksFromOfflineExecutors: HandleTaskFailure failed",
					"execution_id", eid,
					"task_id", tid,
					"error", err,
				)
			}
		}(taskID, executionID)
	}
}

func (s *SchedulerService) StopCleanupRoutine() {
	close(s.stopCleanupCh)
}

func (s *SchedulerService) cleanupStaleTaskLocks() {
	ctx, cancel := cleanupCtx()
	defer cancel()

	query := `
		SELECT te.execution_id, te.task_id, COALESCE(t.timeout_seconds, 300) AS timeout_seconds
		FROM bdopsflow_task_executions te
		JOIN bdopsflow_tasks t ON te.task_id = t.id
		WHERE te.status = 'running'
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

	runningExecutionIDs := make(map[string]bool)

	renewKeys := make([]string, 0)
	failCountKeys := make([]string, 0)
	timeoutMap := make(map[string]int64)
	noTimeoutMap := make(map[string]bool)

	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			continue
		}

		executionID := rowString(row[0])
		_ = rowInt64(row[1])
		timeoutSeconds := rowInt64(row[2])
		noTimeout := timeoutSeconds <= 0
		if timeoutSeconds <= 0 {
			timeoutSeconds = 300
		}

		runningExecutionIDs[executionID] = true
		renewKeys = append(renewKeys, fmt.Sprintf("task:renew:%s", executionID))
		failCountKeys = append(failCountKeys, fmt.Sprintf("task:renew:fail:count:%s", executionID))
		timeoutMap[executionID] = timeoutSeconds
		noTimeoutMap[executionID] = noTimeout
	}

	if len(renewKeys) > 0 {
		pipe := s.redis.Pipeline()
		renewCmds := make([]*redis.StringCmd, len(renewKeys))
		failCmds := make([]*redis.StringCmd, len(failCountKeys))
		for i, key := range renewKeys {
			renewCmds[i] = pipe.Get(ctx, key)
		}
		for i, key := range failCountKeys {
			failCmds[i] = pipe.Get(ctx, key)
		}
		_, _ = pipe.Exec(ctx)

		delKeys := make([]string, 0)
		for i, cmd := range renewCmds {
			executionID := ""
			parts := strings.SplitN(renewKeys[i], ":", 3)
			if len(parts) >= 3 {
				executionID = parts[2]
			}

			lastRenewStr, err := cmd.Result()
			if err != nil {
				delKeys = append(delKeys, failCountKeys[i])
				continue
			}

			var lastRenew int64
			fmt.Sscanf(lastRenewStr, "%d", &lastRenew)

			timeoutSeconds := timeoutMap[executionID]
			noTimeout := noTimeoutMap[executionID]
			interval := time.Now().Unix() - lastRenew
			if noTimeout || interval <= timeoutSeconds {
				if failStr, _ := failCmds[i].Result(); failStr != "" {
					delKeys = append(delKeys, failCountKeys[i])
					slog.Debug("cleanup: renewal recovered, reset fail count",
						"execution_id", executionID,
					)
				}
			}
		}

		if len(delKeys) > 0 {
			s.redis.Del(ctx, delKeys...)
		}
	}

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
			executionID := ""
			parts := strings.SplitN(key, ":", 3)
			if len(parts) >= 3 {
				executionID = parts[2]
			}

			if !runningExecutionIDs[executionID] {
				slog.Info("cleanup: removing stale task lock for non-running execution",
					"execution_id", executionID,
					"key", key)

				s.redis.Del(ctx, key)

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
	renewKey := fmt.Sprintf("task:renew:%s", executionID)
	failCountKey := fmt.Sprintf("task:renew:fail:count:%s", executionID)
	s.redis.Del(ctx, lockKey, renewKey, failCountKey)

	go func(tid int64, eid string, failReason string) {
		if err := s.HandleTaskFailure(context.Background(), tid, eid, "", failReason); err != nil {
			slog.Error("forceFailTask: HandleTaskFailure failed",
				"execution_id", eid,
				"task_id", tid,
				"error", err,
			)
		}
	}(taskID, executionID, reason)
}

func (s *SchedulerService) renewTaskLock(ctx context.Context, executionID string) error {
	lockKey := fmt.Sprintf("task:lock:%s", executionID)
	exists, err := s.redis.Exists(ctx, lockKey).Result()
	if err != nil {
		return err
	}

	lockTTL := 300
	noTimeout := false

	execQuery := `SELECT t.timeout_seconds FROM bdopsflow_task_executions te JOIN bdopsflow_tasks t ON te.task_id = t.id WHERE te.execution_id = ?`
	execStmt := rqlite.ParameterizedStatement{
		Query:     execQuery,
		Arguments: []interface{}{executionID},
	}
	if qr, qerr := s.DB.QueryOneParameterized(execStmt); qerr == nil && qr.Err == nil && qr.Next() {
		row, _ := qr.Slice()
		taskTimeout := rowInt64(row[0])
		if taskTimeout > 0 {
			lockTTL = int(taskTimeout) * 2
		} else {
			noTimeout = true
		}
	}
	if noTimeout {
		lockTTL = 3600
	}
	if lockTTL < 60 {
		lockTTL = 60
	}
	if lockTTL > 7200 {
		lockTTL = 7200
	}

	if exists == 0 {
		slog.Warn("lock not found, recreating for executor reported running task", "execution_id", executionID)
		if err := s.redis.Set(ctx, lockKey, "recovered_by_executor", time.Duration(lockTTL)*time.Second).Err(); err != nil {
			return err
		}
	} else {
		if err := s.redis.Expire(ctx, lockKey, time.Duration(lockTTL)*time.Second).Err(); err != nil {
			return err
		}
	}

	renewKey := fmt.Sprintf("task:renew:%s", executionID)
	s.redis.Set(ctx, renewKey, time.Now().Unix(), time.Duration(lockTTL)*time.Second)

	failCountKey := fmt.Sprintf("task:renew:fail:count:%s", executionID)
	s.redis.Del(ctx, failCountKey)

	slog.Debug("task lock renewed", "execution_id", executionID, "lock_ttl_seconds", lockTTL)
	return nil
}

func (s *SchedulerService) checkCronReload() {
	ctx := context.Background()
	val, err := s.redis.Get(ctx, "cron:needs_reload").Int64()
	if err != nil || val == 0 {
		return
	}

	s.redis.Del(ctx, "cron:needs_reload")

	if s.cronScheduler == nil {
		return
	}

	slog.Info("detected cron reload flag, reloading all cron tasks")
	go func() {
		if s.cronScheduler != nil {
			s.cronScheduler.LoadAndRegisterTasks()
		}
	}()
}

// cleanupExecutorStaleTasks 清理指定执行器上所有正在运行任务的 renew 记录
// 当执行器重启时调用，因为执行器重启后不会再 renew 旧任务
func (s *SchedulerService) cleanupExecutorStaleTasks(ctx context.Context, executorID int64) {
	query := `
		SELECT te.execution_id
		FROM bdopsflow_task_executions te
		WHERE te.status = 'running'
		  AND te.executor_id = ?
	`
	stmt := rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{executorID},
	}
	qr, err := s.DB.QueryOneParameterized(stmt)
	if err != nil {
		slog.Error("cleanupExecutorStaleTasks: query failed",
			"executor_id", executorID,
			"error", err,
		)
		return
	}
	if qr.Err != nil {
		slog.Error("cleanupExecutorStaleTasks: query returned error",
			"executor_id", executorID,
			"error", qr.Err,
		)
		return
	}

	var executionIDs []string
	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			continue
		}
		executionID := rowString(row[0])
		if executionID != "" {
			executionIDs = append(executionIDs, executionID)
		}
	}

	if len(executionIDs) == 0 {
		return
	}

	slog.Info("cleanupExecutorStaleTasks: cleaning up stale task renewals",
		"executor_id", executorID,
		"task_count", len(executionIDs),
	)

	// 删除所有任务的 renew 记录和 fail count 记录
	var redisKeys []string
	for _, execID := range executionIDs {
		redisKeys = append(redisKeys, fmt.Sprintf("task:renew:%s", execID))
		redisKeys = append(redisKeys, fmt.Sprintf("task:renew:fail:count:%s", execID))
	}

	if len(redisKeys) > 0 {
		if err := s.redis.Del(ctx, redisKeys...).Err(); err != nil {
			slog.Error("cleanupExecutorStaleTasks: failed to delete redis keys",
				"executor_id", executorID,
				"error", err,
			)
		}
	}
}
