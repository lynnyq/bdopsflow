package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
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
		ID          int64
		TaskID      int64
		ExecutionID string
		StartTime   string
	}

	for qr.Next() {
		row, err := qr.Slice()
		if err != nil {
			continue
		}
		stuck := struct {
			ID          int64
			TaskID      int64
			ExecutionID string
			StartTime   string
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

func (s *SchedulerService) StopCleanupRoutine() {
	close(s.stopCleanupCh)
}

func (s *SchedulerService) cleanupStaleTaskLocks() {
	ctx := context.Background()

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
	lockTTLSeconds := int64(300)
	maxInterval := lockTTLSeconds
	requiredFailCount := int64(3)

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

func (s *SchedulerService) renewTaskLock(ctx context.Context, executionID string) error {
	lockKey := fmt.Sprintf("task:lock:%s", executionID)
	exists, err := s.redis.Exists(ctx, lockKey).Result()
	if err != nil {
		return err
	}

	lockTTL := 300

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
