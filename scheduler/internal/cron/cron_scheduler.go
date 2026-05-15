package cron

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
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
		cron:        cron.New(cron.WithSeconds()), // 使用本地时间
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

	var entryID cron.EntryID
	var err error
	
	// 先尝试直接添加（可能是6位）
	entryID, err = cs.cron.AddFunc(cronExpr, func() {
		cs.executeTask(taskID)
	})
	
	// 如果失败，尝试解析为标准的5位表达式并加上秒位0
	if err != nil {
		// 先检查是否是标准5位格式
		_, parseErr := cron.ParseStandard(cronExpr)
		if parseErr == nil {
			// 如果是标准格式，尝试添加前缀 "0 " 变成6位
			entryID, err = cs.cron.AddFunc("0 "+cronExpr, func() {
				cs.executeTask(taskID)
			})
		}
	}

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

	// 重新获取任务并检查是否仍然启用
	task, err := cs.svc.GetTaskByID(ctx, taskID)
	if err != nil {
		slog.Warn("get task failed before cron execution", "task_id", taskID, "error", err)
		return
	}

	if !task.IsEnabled {
		slog.Debug("task is disabled, skipping execution", "task_id", taskID)
		// 清理任务调度
		cs.UnregisterTask(taskID)
		return
	}

	if task.CronExpression == "" {
		slog.Debug("task has no cron expression, skipping execution", "task_id", taskID)
		cs.UnregisterTask(taskID)
		return
	}

	// 获取分布式锁，避免多实例重复执行
	// 锁超时时间与任务超时时间相关，最小60秒，最大3600秒
	lockTTL := time.Duration(task.TimeoutSeconds) * 2
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

	acquired, err := cs.acquireTaskLock(ctx, taskID, lockTTL)
	if err != nil || !acquired {
		if err != nil {
			slog.Warn("acquire task lock failed", "task_id", taskID, "error", err)
		}
		return
	}
	defer cs.releaseTaskLock(ctx, taskID)

	slog.Info("cron task triggering", "task_id", taskID, "task_name", task.Name)

	// 尝试触发任务，如果正在运行则等待重试
	maxRetries := 10 // 最多重试10次
	retryInterval := 10 * time.Second // 每10秒重试一次
	
	for attempt := 1; attempt <= maxRetries; attempt++ {
		executionID, err := cs.svc.TriggerTask(ctx, taskID)
		if err == nil {
			// 成功触发
			slog.Info("cron task triggered successfully",
				"task_id", taskID,
				"execution_id", executionID,
			)
			return
		}

		// 检查是否是"正在运行"错误
		if strings.Contains(err.Error(), "already running") || strings.Contains(err.Error(), "skipped") {
			slog.Warn("task is still running, waiting for retry",
				"task_id", taskID,
				"attempt", attempt,
				"max_retries", maxRetries,
				"retry_interval", retryInterval,
				"error", err,
			)
			
			// 等待后重试
			time.Sleep(retryInterval)
			continue
		}

		// 其他错误，不再重试
		slog.Error("cron trigger task failed",
			"task_id", taskID,
			"execution_id", executionID,
			"error", err,
		)
		return
	}

	// 所有重试都失败
	slog.Error("cron trigger task failed after all retries",
		"task_id", taskID,
		"max_retries", maxRetries,
		"total_wait_time", maxRetries*int(retryInterval),
	)
}

// acquireTaskLock 尝试获取任务执行锁
func (cs *CronScheduler) acquireTaskLock(ctx context.Context, taskID int64, lockTTL time.Duration) (bool, error) {
	if cs.redis == nil {
		return true, nil
	}

	lockKey := fmt.Sprintf("cron:lock:task:%d", taskID)
	ok, err := cs.redis.SetNX(ctx, lockKey, "locked", lockTTL).Result()
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
