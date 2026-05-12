package cron

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"

	"github.com/lynnyq/bdopsflow/scheduler/internal/service"
)

type CronScheduler struct {
	cron         *cron.Cron
	svc          *service.SchedulerService
	redis        *redis.Client
	lastExecuted sync.Map // 记录每个任务的最后执行时间
}

func NewCronScheduler(svc *service.SchedulerService, redis *redis.Client) *CronScheduler {
	return &CronScheduler{
		cron:         cron.New(cron.WithSeconds()),
		svc:          svc,
		redis:        redis,
		lastExecuted: sync.Map{},
	}
}

func (cs *CronScheduler) Start() error {
	cs.cron.AddFunc("@every 10s", cs.scanAndTriggerTasks) // 增加扫描频率到10秒一次
	cs.cron.Start()
	slog.Info("cron scheduler started", "mode", "6-field (with seconds)", "scan_interval", "10s", "distributed_lock", "enabled")
	return nil
}

func (cs *CronScheduler) Stop() {
	cs.cron.Stop()
}

// acquireTaskLock 尝试获取任务执行锁，防止多个实例同时执行同一个任务
func (cs *CronScheduler) acquireTaskLock(ctx context.Context, taskID int64) (bool, error) {
	if cs.redis == nil {
		slog.Warn("redis client not available, skipping distributed lock", "task_id", taskID)
		return true, nil
	}

	lockKey := fmt.Sprintf("cron:lock:task:%d", taskID)
	// 设置锁过期时间为2分钟，避免死锁
	ok, err := cs.redis.SetNX(ctx, lockKey, "locked", 2*time.Minute).Result()
	if err != nil {
		slog.Warn("failed to acquire lock", "task_id", taskID, "error", err)
		return false, err
	}

	if ok {
		slog.Debug("acquired task lock", "task_id", taskID, "lock_key", lockKey)
	} else {
		slog.Debug("task already locked by another instance", "task_id", taskID)
	}

	return ok, nil
}

// releaseTaskLock 释放任务执行锁
func (cs *CronScheduler) releaseTaskLock(ctx context.Context, taskID int64) {
	if cs.redis == nil {
		return
	}

	lockKey := fmt.Sprintf("cron:lock:task:%d", taskID)
	err := cs.redis.Del(ctx, lockKey).Err()
	if err != nil {
		slog.Warn("failed to release lock", "task_id", taskID, "error", err)
	} else {
		slog.Debug("released task lock", "task_id", taskID)
	}
}

func (cs *CronScheduler) scanAndTriggerTasks() {
	ctx := context.Background()
	tasks, err := cs.svc.ScanPendingTasks(ctx)
	if err != nil {
		slog.Error("scan pending tasks failed", "error", err)
		return
	}

	now := time.Now()
	slog.Debug("scanning for cron tasks", "count", len(tasks), "time", now)

	for _, task := range tasks {
		if task.CronExpression != "" {
			parser := cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
			schedule, err := parser.Parse(task.CronExpression)
			if err != nil {
				slog.Warn("invalid cron expression", "task_id", task.ID, "cron", task.CronExpression, "error", err)
				continue
			}

			// 获取上次执行时间
			var lastExecTime time.Time
			if val, ok := cs.lastExecuted.Load(task.ID); ok {
				lastExecTime = val.(time.Time)
			}

			// 计算下一个应该触发的时间
			var triggerTime time.Time
			if lastExecTime.IsZero() {
				// 第一次执行，从任务创建时间开始算
				triggerTime = schedule.Next(task.CreatedAt)
			} else {
				triggerTime = schedule.Next(lastExecTime)
			}

			// 判断是否需要触发：如果下一个触发时间已经到了或者在30秒内，并且还没有执行过
			if !triggerTime.IsZero() && (now.After(triggerTime) || now.Sub(triggerTime) < 30*time.Second) {
				// 检查是否在最近1分钟内已经执行过（避免重复触发）
				if now.Sub(lastExecTime) < 1*time.Minute {
					slog.Debug("task already executed recently, skipping",
						"task_id", task.ID,
						"name", task.Name,
						"last_executed", lastExecTime,
					)
					continue
				}

				// 尝试获取分布式锁
				acquired, err := cs.acquireTaskLock(ctx, task.ID)
				if err != nil || !acquired {
					continue
				}

				slog.Info("cron task triggering",
					"task_id", task.ID,
					"name", task.Name,
					"cron", task.CronExpression,
					"trigger_time", triggerTime,
				)
				
				// 更新最后执行时间
				cs.lastExecuted.Store(task.ID, now)
				
				go func(taskID int64) {
					defer cs.releaseTaskLock(ctx, taskID) // 确保锁被释放
					
					executionID, err := cs.svc.TriggerTask(ctx, taskID)
					if err != nil {
						slog.Error("cron trigger task failed",
							"task_id", taskID,
							"execution_id", executionID,
							"error", err,
						)
					} else {
						slog.Info("cron task triggered successfully",
							"task_id", taskID,
							"execution_id", executionID,
						)
					}
				}(task.ID)
			}
		}
	}
}