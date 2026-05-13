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
	taskEntries  map[int64]cron.EntryID // 任务ID到cron entry的映射
	mu           sync.RWMutex
}

func NewCronScheduler(svc *service.SchedulerService, redis *redis.Client) *CronScheduler {
	return &CronScheduler{
		cron:        cron.New(cron.WithSeconds()),
		svc:         svc,
		redis:       redis,
		taskEntries: make(map[int64]cron.EntryID),
	}
}

func (cs *CronScheduler) Start() error {
	cs.cron.Start()
	slog.Info("cron scheduler started", "mode", "6-field (with seconds)", "distributed_lock", "enabled")
	
	// 启动后立即加载和注册所有任务
	go cs.loadAndRegisterTasks()
	
	return nil
}

func (cs *CronScheduler) Stop() {
	cs.cron.Stop()
}

// loadAndRegisterTasks 从数据库加载并注册所有任务
func (cs *CronScheduler) loadAndRegisterTasks() {
	if cs.svc == nil {
		slog.Debug("Scheduler service is nil, skipping task loading")
		return
	}

	ctx := context.Background()
	tasks, err := cs.svc.ScanPendingTasks(ctx)
	if err != nil {
		slog.Error("load tasks failed", "error", err)
		return
	}

	if len(tasks) == 0 {
		slog.Debug("no tasks found to load")
		return
	}

	slog.Info("loading tasks from database", "count", len(tasks))

	for _, task := range tasks {
		if task.CronExpression != "" && task.IsEnabled {
			cs.RegisterTask(task.ID, task.CronExpression)
		}
	}
}

// RegisterTask 注册一个新的定时任务
func (cs *CronScheduler) RegisterTask(taskID int64, cronExpr string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// 如果任务已存在，先移除旧的
	if entryID, exists := cs.taskEntries[taskID]; exists {
		cs.cron.Remove(entryID)
		delete(cs.taskEntries, taskID)
	}

	// 使用 AddFunc 注册任务
	entryID, err := cs.cron.AddFunc(cronExpr, func() {
		cs.executeTask(taskID)
	})

	if err != nil {
		slog.Error("register task failed", "task_id", taskID, "cron", cronExpr, "error", err)
		return
	}

	cs.taskEntries[taskID] = entryID
	slog.Info("task registered", "task_id", taskID, "cron", cronExpr, "entry_id", entryID)
}

// UnregisterTask 取消注册任务
func (cs *CronScheduler) UnregisterTask(taskID int64) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if entryID, exists := cs.taskEntries[taskID]; exists {
		cs.cron.Remove(entryID)
		delete(cs.taskEntries, taskID)
		slog.Info("task unregistered", "task_id", taskID)
	}
}

// executeTask 执行单个任务
func (cs *CronScheduler) executeTask(taskID int64) {
	if cs.svc == nil {
		slog.Debug("Scheduler service is nil, skipping task execution", "task_id", taskID)
		return
	}

	ctx := context.Background()
	
	// 获取分布式锁，避免多实例重复执行
	acquired, err := cs.acquireTaskLock(ctx, taskID)
	if err != nil || !acquired {
		if err != nil {
			slog.Warn("acquire task lock failed", "task_id", taskID, "error", err)
		}
		return
	}
	defer cs.releaseTaskLock(ctx, taskID)

	slog.Info("cron task triggering", "task_id", taskID)

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
}

// acquireTaskLock 尝试获取任务执行锁
func (cs *CronScheduler) acquireTaskLock(ctx context.Context, taskID int64) (bool, error) {
	if cs.redis == nil {
		return true, nil
	}

	lockKey := fmt.Sprintf("cron:lock:task:%d", taskID)
	ok, err := cs.redis.SetNX(ctx, lockKey, "locked", 30*time.Second).Result()
	if err != nil {
		slog.Warn("failed to acquire lock", "task_id", taskID, "error", err)
		return false, err
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
	}
}
