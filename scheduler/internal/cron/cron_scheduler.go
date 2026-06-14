package cron

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"

	"github.com/lynnyq/bdopsflow/scheduler/internal/metrics"
	"github.com/lynnyq/bdopsflow/scheduler/internal/service"
)

type CronScheduler struct {
	cron              *cron.Cron
	svc               *service.SchedulerService
	redis             *redis.Client
	taskEntries       map[int64]cron.EntryID
	mu                sync.RWMutex
	paused            bool
	isLeader          bool
	started           bool
	startTime         time.Time
	lastRedisSync     time.Time
	redisSyncInterval time.Duration
	nodeID            string // 节点唯一标识，用于分布式锁所有权验证
}

func NewCronScheduler(svc *service.SchedulerService, redis *redis.Client) *CronScheduler {
	// 生成节点唯一标识
	nodeID := generateNodeID()

	return &CronScheduler{
		cron:              cron.New(cron.WithSeconds()),
		svc:               svc,
		redis:             redis,
		taskEntries:       make(map[int64]cron.EntryID),
		startTime:         time.Now(),
		redisSyncInterval: 5 * time.Second,
		nodeID:            nodeID,
	}
}

// generateNodeID 生成节点唯一标识
func generateNodeID() string {
	hostname, _ := os.Hostname()
	return fmt.Sprintf("%s-%d", hostname, os.Getpid())
}

func (cs *CronScheduler) Start() error {
	// 不立即启动cron，等成为主节点后再启动
	slog.Info("cron scheduler initialized, waiting to become leader")

	// 从 Redis 加载暂停状态
	if cs.redis != nil {
		ctx := context.Background()
		paused, err := cs.redis.Get(ctx, "scheduler:paused").Bool()
		if err == nil && paused {
			cs.paused = true
			slog.Info("cron scheduler initialized in paused state from redis")
		}
	}

	return nil
}

// OnBecomeLeader 当成为主节点时调用
func (cs *CronScheduler) OnBecomeLeader() {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.isLeader {
		return
	}

	cs.isLeader = true
	slog.Info("node became leader, starting cron scheduler")

	// 启动cron
	if !cs.started {
		cs.cron.Start()
		cs.started = true
		slog.Info("cron scheduler started", "mode", "6-field (with seconds)", "distributed_lock", "enabled")
	}

	// 加载和注册所有任务
	go cs.loadAndRegisterTasks()

	// 恢复正在执行的任务
	go cs.recoverRunningTasks()
}

func (cs *CronScheduler) recoverRunningTasks() {
	if cs.svc == nil {
		slog.Warn("SchedulerService is nil, cannot recover running tasks")
		return
	}

	if !cs.svc.IsLeader() {
		slog.Warn("lost leadership before recovering tasks, aborting")
		return
	}

	ctx := context.Background()
	if err := cs.svc.RecoverRunningTasksOnBecomeLeader(ctx); err != nil {
		slog.Error("failed to recover running tasks", "error", err)
	}
}

// OnLoseLeader 当失去主节点地位时调用
func (cs *CronScheduler) OnLoseLeader() {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if !cs.isLeader {
		return
	}

	cs.isLeader = false
	slog.Info("node lost leadership, stopping cron scheduler")

	for taskID, entryID := range cs.taskEntries {
		cs.cron.Remove(entryID)
		slog.Info("unregistered cron task on leadership loss", "task_id", taskID)
	}
	cs.taskEntries = make(map[int64]cron.EntryID)
}

// Pause 暂停调度器
func (cs *CronScheduler) Pause() {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.paused = true
	cs.lastRedisSync = time.Now()
	slog.Info("cron scheduler paused")

	if cs.redis != nil {
		ctx := context.Background()
		cs.redis.Set(ctx, "scheduler:paused", "1", 0)
	}
}

// Resume 恢复调度器
func (cs *CronScheduler) Resume() {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.paused = false
	cs.lastRedisSync = time.Now()
	slog.Info("cron scheduler resumed")

	if cs.redis != nil {
		ctx := context.Background()
		cs.redis.Set(ctx, "scheduler:paused", "0", 0)
	}
}

// IsPaused 获取暂停状态（定期从 Redis 同步，确保多节点一致性）
func (cs *CronScheduler) IsPaused() bool {
	cs.mu.RLock()
	lastSync := cs.lastRedisSync
	cs.mu.RUnlock()

	if cs.redis != nil && time.Since(lastSync) > cs.redisSyncInterval {
		cs.syncPausedFromRedis()
	}

	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.paused
}

// syncPausedFromRedis 从 Redis 同步暂停状态到本地
func (cs *CronScheduler) syncPausedFromRedis() {
	ctx := context.Background()
	redisPaused, err := cs.redis.Get(ctx, "scheduler:paused").Bool()
	if err != nil {
		return
	}

	cs.mu.Lock()
	defer cs.mu.Unlock()
	if cs.paused != redisPaused {
		slog.Info("synced scheduler paused state from redis", "local", cs.paused, "redis", redisPaused)
		cs.paused = redisPaused
	}
	cs.lastRedisSync = time.Now()
}

// GetUptime 获取运行时长
func (cs *CronScheduler) GetUptime() time.Duration {
	return time.Since(cs.startTime)
}

func (cs *CronScheduler) Stop() {
	cs.cron.Stop()
}

// loadAndRegisterTasks 从数据库加载并注册所有任务
func (cs *CronScheduler) LoadAndRegisterTasks() {
	cs.loadAndRegisterTasks()
}

func (cs *CronScheduler) loadAndRegisterTasks() {
	if cs.svc == nil {
		slog.Debug("Scheduler service is nil, skipping task loading")
		return
	}

	if !cs.svc.IsLeader() {
		slog.Warn("lost leadership before loading tasks, aborting")
		return
	}

	ctx := context.Background()
	bdopsflow_tasks, err := cs.svc.ScanPendingTasks(ctx)
	if err != nil {
		slog.Error("load bdopsflow_tasks failed", "error", err)
		return
	}

	if len(bdopsflow_tasks) == 0 {
		slog.Debug("no bdopsflow_tasks found to load")
		return
	}

	slog.Info("loading bdopsflow_tasks from database", "count", len(bdopsflow_tasks))

	for _, task := range bdopsflow_tasks {
		if !cs.svc.IsLeader() {
			slog.Warn("lost leadership during task loading, aborting", "loaded", len(bdopsflow_tasks))
			return
		}
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

	// 检查是否是主节点
	cs.mu.RLock()
	isLeader := cs.isLeader
	cs.mu.RUnlock()

	if !isLeader {
		slog.Debug("not leader, skipping task execution", "task_id", taskID)
		return
	}

	// 检查是否暂停（从 Redis 同步最新状态）
	if cs.redis != nil {
		cs.syncPausedFromRedis()
	}
	if cs.IsPaused() {
		slog.Debug("scheduler is paused, skipping task execution", "task_id", taskID)
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
	if lockTTL > 7200*time.Second {
		lockTTL = 7200 * time.Second
	}
	if task.TimeoutSeconds == 0 {
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

	// 启动锁续期协程（防止长任务执行期间锁过期）
	stopRenewer := make(chan struct{})
	go cs.startLockRenewer(ctx, taskID, lockTTL, stopRenewer)
	defer close(stopRenewer)

	slog.Info("cron task triggering", "task_id", taskID, "task_name", task.Name)

	executionID, err := cs.svc.TriggerTask(ctx, taskID)
	if err == nil {
		slog.Info("cron task triggered successfully",
			"task_id", taskID,
			"execution_id", executionID,
		)
		metrics.CronTriggers.WithLabelValues("success").Inc()
		metrics.TasksTriggered.WithLabelValues("cron").Inc()
		return
	}

	if strings.Contains(err.Error(), "already running") || strings.Contains(err.Error(), "skipped") {
		slog.Warn("cron task skipped: previous execution still running",
			"task_id", taskID,
			"error", err,
		)
		metrics.CronTriggers.WithLabelValues("skipped").Inc()
		return
	}

	slog.Error("cron trigger task failed",
		"task_id", taskID,
		"error", err,
	)
	metrics.CronTriggers.WithLabelValues("failed").Inc()
}

// acquireTaskLock 尝试获取任务执行锁（带所有权验证）
func (cs *CronScheduler) acquireTaskLock(ctx context.Context, taskID int64, lockTTL time.Duration) (bool, error) {
	if cs.redis == nil {
		return true, nil
	}

	lockKey := fmt.Sprintf("cron:lock:task:%d", taskID)
	// 使用 Lua 脚本确保原子性：只有当锁不存在或已过期时才能获取
	luaScript := `
		if redis.call("EXISTS", KEYS[1]) == 0 then
			redis.call("HSET", KEYS[1], "owner", ARGV[1], "expire", ARGV[2])
			return 1
		elseif redis.call("HGET", KEYS[1], "owner") == ARGV[1] then
			redis.call("HSET", KEYS[1], "expire", ARGV[2])
			return 1
		else
			return 0
		end
	`

	expireTime := time.Now().Add(lockTTL).Unix()
	result, err := cs.redis.Eval(ctx, luaScript, []string{lockKey}, cs.nodeID, expireTime).Int()
	if err != nil {
		slog.Warn("failed to acquire lock", "task_id", taskID, "error", err)
		return false, err
	}

	return result == 1, nil
}

// releaseTaskLock 释放任务执行锁（验证所有权）
func (cs *CronScheduler) releaseTaskLock(ctx context.Context, taskID int64) {
	if cs.redis == nil {
		return
	}

	lockKey := fmt.Sprintf("cron:lock:task:%d", taskID)
	// 使用 Lua 脚本确保只有锁的所有者才能释放
	luaScript := `
		if redis.call("HGET", KEYS[1], "owner") == ARGV[1] then
			return redis.call("DEL", KEYS[1])
		else
			return 0
		end
	`

	result, err := cs.redis.Eval(ctx, luaScript, []string{lockKey}, cs.nodeID).Int()
	if err != nil {
		slog.Warn("failed to release lock", "task_id", taskID, "error", err)
		return
	}

	if result == 0 {
		slog.Warn("lock already released or owned by another node", "task_id", taskID, "node_id", cs.nodeID)
	}
}

// renewTaskLock 续期任务锁（防止长任务锁过期）
func (cs *CronScheduler) renewTaskLock(ctx context.Context, taskID int64, lockTTL time.Duration) error {
	if cs.redis == nil {
		return nil
	}

	lockKey := fmt.Sprintf("cron:lock:task:%d", taskID)
	// 使用 Lua 脚本确保只有锁的所有者才能续期
	luaScript := `
		if redis.call("HGET", KEYS[1], "owner") == ARGV[1] then
			redis.call("HSET", KEYS[1], "expire", ARGV[2])
			return 1
		else
			return 0
		end
	`

	expireTime := time.Now().Add(lockTTL).Unix()
	result, err := cs.redis.Eval(ctx, luaScript, []string{lockKey}, cs.nodeID, expireTime).Int()
	if err != nil {
		return err
	}

	if result == 0 {
		return fmt.Errorf("lock not owned by this node")
	}

	return nil
}

// startLockRenewer 启动锁续期协程
func (cs *CronScheduler) startLockRenewer(ctx context.Context, taskID int64, lockTTL time.Duration, stopCh <-chan struct{}) {
	// 每 lockTTL/3 续期一次，确保锁不会过期
	renewInterval := lockTTL / 3
	ticker := time.NewTicker(renewInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := cs.renewTaskLock(ctx, taskID, lockTTL); err != nil {
				slog.Warn("failed to renew lock, task may be taken by another node",
					"task_id", taskID,
					"error", err,
				)
				return
			}
			slog.Debug("lock renewed successfully", "task_id", taskID)
		case <-stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}
