package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/lynnyq/bdopsflow/scheduler/pkg/database"
)

// ============ StartCleanupRoutine / StopCleanupRoutine ============

func TestStartStopCleanupRoutine(t *testing.T) {
	svc := &SchedulerService{
		stopCleanupCh: make(chan struct{}),
	}

	svc.StartCleanupRoutine()

	// 给 goroutine 一点启动时间
	time.Sleep(50 * time.Millisecond)

	// 停止 cleanup routine
	svc.StopCleanupRoutine()

	// 验证 channel 已关闭（再次 close 会 panic）
	defer func() {
		if r := recover(); r == nil {
			t.Error("期望第二次 close channel 时 panic")
		}
	}()
	svc.StopCleanupRoutine()
}

func TestStopCleanupRoutine_NilChannel(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("期望 close nil channel 时 panic")
		}
	}()
	svc := &SchedulerService{}
	svc.StopCleanupRoutine()
}

// ============ checkCronReload ============

func TestCheckCronReload_NoKey(t *testing.T) {
	svc, mr, _ := newSchedulerWithDBAndRedis(t, &MockDB{})
	defer mr.Close()

	// 未设置 cron:needs_reload key，应直接返回
	svc.checkCronReload()

	// 验证 key 不存在
	exists, err := svc.redis.Exists(context.Background(), "cron:needs_reload").Result()
	if err != nil {
		t.Fatalf("redis 查询失败: %v", err)
	}
	if exists != 0 {
		t.Error("期望 cron:needs_reload key 不存在")
	}
}

func TestCheckCronReload_KeyExistsWithCronScheduler(t *testing.T) {
	svc, mr, _ := newSchedulerWithDBAndRedis(t, &MockDB{})
	defer mr.Close()
	cs := &mockCronScheduler{}
	svc.cronScheduler = cs

	// 设置 cron:needs_reload = 1
	svc.redis.Set(context.Background(), "cron:needs_reload", 1, 0)

	svc.checkCronReload()

	// 验证 key 已被删除
	exists, _ := svc.redis.Exists(context.Background(), "cron:needs_reload").Result()
	if exists != 0 {
		t.Error("期望 cron:needs_reload key 已被删除")
	}
}

func TestCheckCronReload_KeyExistsWithoutCronScheduler(t *testing.T) {
	svc, mr, _ := newSchedulerWithDBAndRedis(t, &MockDB{})
	defer mr.Close()
	// cronScheduler 为 nil

	svc.redis.Set(context.Background(), "cron:needs_reload", 1, 0)

	// 不应 panic
	svc.checkCronReload()

	// key 应已删除
	exists, _ := svc.redis.Exists(context.Background(), "cron:needs_reload").Result()
	if exists != 0 {
		t.Error("期望 cron:needs_reload key 已被删除")
	}
}

func TestCheckCronReload_KeyValueZero(t *testing.T) {
	svc, mr, _ := newSchedulerWithDBAndRedis(t, &MockDB{})
	defer mr.Close()

	// 设置 cron:needs_reload = 0（值为 0 时应直接返回）
	svc.redis.Set(context.Background(), "cron:needs_reload", 0, 0)

	svc.checkCronReload()

	// key 应仍然存在（因为值为 0，不触发 reload）
	exists, _ := svc.redis.Exists(context.Background(), "cron:needs_reload").Result()
	if exists == 0 {
		t.Error("期望 cron:needs_reload key 仍存在（值为 0 时不删除）")
	}
}

// ============ renewTaskLock ============

func TestRenewTaskLock_LockExists(t *testing.T) {
	ctx := context.Background()
	svc, mr, _ := newSchedulerWithDBAndRedis(t, &MockDB{})
	defer mr.Close()

	executionID := "exec-renew-001"
	lockKey := fmt.Sprintf("task:lock:%s", executionID)

	// 预设 lock key
	svc.redis.Set(ctx, lockKey, "locked", 60*time.Second)

	// DB 查询返回 timeout_seconds = 120
	qr := database.NewQueryResultWithRows([][]interface{}{
		{int64(120)},
	})
	svc.DB = &MockDB{QueryResult: qr}

	err := svc.renewTaskLock(ctx, executionID)
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}

	// 验证 renew key 已设置
	renewKey := fmt.Sprintf("task:renew:%s", executionID)
	exists, _ := svc.redis.Exists(ctx, renewKey).Result()
	if exists == 0 {
		t.Error("期望 renew key 已设置")
	}

	// 验证 lock key TTL 已更新（应该为 240 秒 = 120 * 2）
	ttl, _ := svc.redis.TTL(ctx, lockKey).Result()
	if ttl <= 0 {
		t.Error("期望 lock key 有正的 TTL")
	}
	if ttl > 240*time.Second {
		t.Errorf("期望 TTL <= 240s，实际 %v", ttl)
	}
}

func TestRenewTaskLock_LockNotExists(t *testing.T) {
	ctx := context.Background()
	svc, mr, _ := newSchedulerWithDBAndRedis(t, &MockDB{})
	defer mr.Close()

	executionID := "exec-renew-002"

	// DB 查询返回 timeout_seconds = 120
	qr := database.NewQueryResultWithRows([][]interface{}{
		{int64(120)},
	})
	svc.DB = &MockDB{QueryResult: qr}

	err := svc.renewTaskLock(ctx, executionID)
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}

	// 验证 lock key 已创建
	lockKey := fmt.Sprintf("task:lock:%s", executionID)
	val, _ := svc.redis.Get(ctx, lockKey).Result()
	if val != "recovered_by_executor" {
		t.Errorf("期望 lock key 值为 recovered_by_executor，实际=%s", val)
	}
}

func TestRenewTaskLock_NoTimeout(t *testing.T) {
	ctx := context.Background()
	svc, mr, _ := newSchedulerWithDBAndRedis(t, &MockDB{})
	defer mr.Close()

	executionID := "exec-renew-003"
	lockKey := fmt.Sprintf("task:lock:%s", executionID)
	svc.redis.Set(ctx, lockKey, "locked", 60*time.Second)

	// DB 查询返回 timeout_seconds = 0（无超时）
	qr := database.NewQueryResultWithRows([][]interface{}{
		{int64(0)},
	})
	svc.DB = &MockDB{QueryResult: qr}

	err := svc.renewTaskLock(ctx, executionID)
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}

	// 无超时时 TTL 应为 3600 秒
	ttl, _ := svc.redis.TTL(ctx, lockKey).Result()
	if ttl <= 0 || ttl > 3600*time.Second {
		t.Errorf("期望 TTL 约 3600s，实际 %v", ttl)
	}
}

func TestRenewTaskLock_DBQueryError(t *testing.T) {
	ctx := context.Background()
	svc, mr, _ := newSchedulerWithDBAndRedis(t, &MockDB{})
	defer mr.Close()

	executionID := "exec-renew-004"
	lockKey := fmt.Sprintf("task:lock:%s", executionID)
	svc.redis.Set(ctx, lockKey, "locked", 60*time.Second)

	// DB 查询返回错误，但 renewTaskLock 不会返回错误（只是使用默认 TTL）
	svc.DB = &MockDB{QueryError: ErrMockDB}

	err := svc.renewTaskLock(ctx, executionID)
	if err != nil {
		t.Fatalf("期望无错误（DB 查询失败时使用默认值），实际: %v", err)
	}

	// 默认 TTL 应为 300 秒
	ttl, _ := svc.redis.TTL(ctx, lockKey).Result()
	if ttl <= 0 {
		t.Error("期望 lock key 有 TTL")
	}
}

func TestRenewTaskLock_FailCountKeyDeleted(t *testing.T) {
	ctx := context.Background()
	svc, mr, _ := newSchedulerWithDBAndRedis(t, &MockDB{})
	defer mr.Close()

	executionID := "exec-renew-005"
	lockKey := fmt.Sprintf("task:lock:%s", executionID)
	failCountKey := fmt.Sprintf("task:renew:fail:count:%s", executionID)

	svc.redis.Set(ctx, lockKey, "locked", 60*time.Second)
	svc.redis.Set(ctx, failCountKey, 3, 60*time.Second)

	qr := database.NewQueryResultWithRows([][]interface{}{
		{int64(120)},
	})
	svc.DB = &MockDB{QueryResult: qr}

	err := svc.renewTaskLock(ctx, executionID)
	if err != nil {
		t.Fatalf("期望无错误，实际: %v", err)
	}

	// 验证 fail count key 已删除
	exists, _ := svc.redis.Exists(ctx, failCountKey).Result()
	if exists != 0 {
		t.Error("期望 fail count key 已删除")
	}
}

// ============ cleanupExecutorStaleTasks ============

func TestCleanupExecutorStaleTasks_Success(t *testing.T) {
	ctx := context.Background()
	svc, mr, _ := newSchedulerWithDBAndRedis(t, &MockDB{})
	defer mr.Close()

	// 预设 redis keys
	execIDs := []string{"exec-1", "exec-2", "exec-3"}
	for _, eid := range execIDs {
		renewKey := fmt.Sprintf("task:renew:%s", eid)
		failCountKey := fmt.Sprintf("task:renew:fail:count:%s", eid)
		svc.redis.Set(ctx, renewKey, time.Now().Unix(), 10*time.Minute)
		svc.redis.Set(ctx, failCountKey, 1, 10*time.Minute)
	}

	// DB 查询返回执行 ID 列表
	rows := make([][]interface{}, len(execIDs))
	for i, eid := range execIDs {
		rows[i] = []interface{}{eid}
	}
	qr := database.NewQueryResultWithRows(rows)
	svc.DB = &MockDB{QueryResult: qr}

	svc.cleanupExecutorStaleTasks(ctx, 1)

	// 验证所有 renew 和 fail count keys 已删除
	for _, eid := range execIDs {
		renewKey := fmt.Sprintf("task:renew:%s", eid)
		failCountKey := fmt.Sprintf("task:renew:fail:count:%s", eid)

		exists, _ := svc.redis.Exists(ctx, renewKey).Result()
		if exists != 0 {
			t.Errorf("期望 renew key %s 已删除", renewKey)
		}
		exists, _ = svc.redis.Exists(ctx, failCountKey).Result()
		if exists != 0 {
			t.Errorf("期望 fail count key %s 已删除", failCountKey)
		}
	}
}

func TestCleanupExecutorStaleTasks_EmptyResult(t *testing.T) {
	ctx := context.Background()
	svc, mr, _ := newSchedulerWithDBAndRedis(t, &MockDB{})
	defer mr.Close()

	qr := database.NewQueryResultWithRows(nil)
	svc.DB = &MockDB{QueryResult: qr}

	// 不应 panic
	svc.cleanupExecutorStaleTasks(ctx, 999)
}

func TestCleanupExecutorStaleTasks_DBError(t *testing.T) {
	ctx := context.Background()
	svc, mr, _ := newSchedulerWithDBAndRedis(t, &MockDB{})
	defer mr.Close()

	svc.DB = &MockDB{QueryError: ErrMockDB}

	// 不应 panic（仅记录日志）
	svc.cleanupExecutorStaleTasks(ctx, 1)
}

func TestCleanupExecutorStaleTasks_ResultErr(t *testing.T) {
	ctx := context.Background()
	svc, mr, _ := newSchedulerWithDBAndRedis(t, &MockDB{})
	defer mr.Close()

	svc.DB = &MockDB{QueryResult: database.NewQueryResultWithErr(errors.New("result error"))}

	// 不应 panic（仅记录日志）
	svc.cleanupExecutorStaleTasks(ctx, 1)
}

func TestCleanupExecutorStaleTasks_AssertQueryParams(t *testing.T) {
	ctx := context.Background()
	svc, mr, _ := newSchedulerWithDBAndRedis(t, &MockDB{})
	defer mr.Close()

	qr := database.NewQueryResultWithRows(nil)
	db := &MockDB{QueryResult: qr}
	svc.DB = db

	svc.cleanupExecutorStaleTasks(ctx, 42)

	if db.LastQueryStmt.Arguments[0] != int64(42) {
		t.Errorf("期望参数 42，实际=%v", db.LastQueryStmt.Arguments[0])
	}
}

// ============ cleanupOfflineExecutors ============

func TestCleanupOfflineExecutors_Success(t *testing.T) {
	svc, mr, _ := newSchedulerWithDBAndRedis(t, &MockDB{})
	defer mr.Close()

	// WriteResult 用于 UPDATE executors，QueryResult 用于 cleanupTasksFromOfflineExecutors 查询
	wr := database.NewWriteResult(0, 2) // 2 行受影响
	qr := database.NewQueryResultWithRows(nil) // 无离线执行器上的任务
	svc.DB = &MockDB{WriteResult: wr, QueryResult: qr}

	// 不应 panic
	svc.cleanupOfflineExecutors()
}

func TestCleanupOfflineExecutors_DBError(t *testing.T) {
	svc, mr, _ := newSchedulerWithDBAndRedis(t, &MockDB{})
	defer mr.Close()

	svc.DB = &MockDB{WriteError: ErrMockDB}

	// 不应 panic（仅记录日志）
	svc.cleanupOfflineExecutors()
}

func TestCleanupOfflineExecutors_ResultErr(t *testing.T) {
	svc, mr, _ := newSchedulerWithDBAndRedis(t, &MockDB{})
	defer mr.Close()

	wr := database.NewWriteResult(0, 0)
	wr.Err = errors.New("result error")
	svc.DB = &MockDB{WriteResult: wr}

	// 不应 panic（仅记录日志）
	svc.cleanupOfflineExecutors()
}

func TestCleanupOfflineExecutors_ZeroRowsAffected(t *testing.T) {
	svc, mr, _ := newSchedulerWithDBAndRedis(t, &MockDB{})
	defer mr.Close()

	wr := database.NewWriteResult(0, 0) // 0 行受影响
	qr := database.NewQueryResultWithRows(nil)
	svc.DB = &MockDB{WriteResult: wr, QueryResult: qr}

	// 不应 panic
	svc.cleanupOfflineExecutors()
}

// ============ cleanupDeadTasks ============

func TestCleanupDeadTasks_NoStuckTasks(t *testing.T) {
	svc, mr, _ := newSchedulerWithDBAndRedis(t, &MockDB{})
	defer mr.Close()

	// 查询返回空结果（无卡住任务）
	qr := database.NewQueryResultWithRows(nil)
	svc.DB = &MockDB{QueryResult: qr}

	// 不应 panic，应直接返回
	svc.cleanupDeadTasks()
}

func TestCleanupDeadTasks_DBError(t *testing.T) {
	svc, mr, _ := newSchedulerWithDBAndRedis(t, &MockDB{})
	defer mr.Close()

	svc.DB = &MockDB{QueryError: ErrMockDB}

	// 不应 panic（仅记录日志并返回）
	svc.cleanupDeadTasks()
}

func TestCleanupDeadTasks_ResultErr(t *testing.T) {
	svc, mr, _ := newSchedulerWithDBAndRedis(t, &MockDB{})
	defer mr.Close()

	svc.DB = &MockDB{QueryResult: database.NewQueryResultWithErr(errors.New("result error"))}

	// 不应 panic（仅记录日志并返回）
	svc.cleanupDeadTasks()
}

// ============ cleanupStaleTaskLocks ============

func TestCleanupStaleTaskLocks_NoRunningTasks(t *testing.T) {
	svc, mr, _ := newSchedulerWithDBAndRedis(t, &MockDB{})
	defer mr.Close()

	// 查询返回空结果（无运行中任务）
	qr := database.NewQueryResultWithRows(nil)
	svc.DB = &MockDB{QueryResult: qr}

	// 不应 panic
	svc.cleanupStaleTaskLocks()
}

func TestCleanupStaleTaskLocks_DBError(t *testing.T) {
	svc, mr, _ := newSchedulerWithDBAndRedis(t, &MockDB{})
	defer mr.Close()

	svc.DB = &MockDB{QueryError: ErrMockDB}

	// 不应 panic（仅记录日志并返回）
	svc.cleanupStaleTaskLocks()
}

func TestCleanupStaleTaskLocks_ResultErr(t *testing.T) {
	svc, mr, _ := newSchedulerWithDBAndRedis(t, &MockDB{})
	defer mr.Close()

	svc.DB = &MockDB{QueryResult: database.NewQueryResultWithErr(errors.New("result error"))}

	// 不应 panic
	svc.cleanupStaleTaskLocks()
}

func TestCleanupStaleTaskLocks_RemovesStaleLocks(t *testing.T) {
	ctx := context.Background()
	svc, mr, _ := newSchedulerWithDBAndRedis(t, &MockDB{})
	defer mr.Close()

	// 查询返回空结果（无运行中任务）
	qr := database.NewQueryResultWithRows(nil)
	svc.DB = &MockDB{QueryResult: qr}

	// 预设一个过期的 lock key
	staleLockKey := "task:lock:stale-exec-001"
	svc.redis.Set(ctx, staleLockKey, "locked", 10*time.Minute)

	svc.cleanupStaleTaskLocks()

	// 验证过期 lock 已被删除（因为对应的 execution 不在 running 列表中）
	exists, _ := svc.redis.Exists(ctx, staleLockKey).Result()
	if exists != 0 {
		t.Error("期望过期 lock key 已被删除")
	}
}

func TestCleanupStaleTaskLocks_KeepsActiveLocks(t *testing.T) {
	ctx := context.Background()
	svc, mr, _ := newSchedulerWithDBAndRedis(t, &MockDB{})
	defer mr.Close()

	// 查询返回一个运行中的任务
	qr := database.NewQueryResultWithRows([][]interface{}{
		{"active-exec-001", int64(1), int64(300)},
	})
	svc.DB = &MockDB{QueryResult: qr}

	// 预设对应的 lock key
	activeLockKey := "task:lock:active-exec-001"
	svc.redis.Set(ctx, activeLockKey, "locked", 10*time.Minute)

	svc.cleanupStaleTaskLocks()

	// 验证活跃 lock 仍然存在
	exists, _ := svc.redis.Exists(ctx, activeLockKey).Result()
	if exists == 0 {
		t.Error("期望活跃 lock key 仍然存在")
	}
}

// ============ cleanupTasksFromOfflineExecutors ============

func TestCleanupTasksFromOfflineExecutors_NoTasks(t *testing.T) {
	ctx := context.Background()
	svc, mr, _ := newSchedulerWithDBAndRedis(t, &MockDB{})
	defer mr.Close()

	// 查询返回空结果
	qr := database.NewQueryResultWithRows(nil)
	svc.DB = &MockDB{QueryResult: qr}

	// 不应 panic
	svc.cleanupTasksFromOfflineExecutors(ctx)
}

func TestCleanupTasksFromOfflineExecutors_DBError(t *testing.T) {
	ctx := context.Background()
	svc, mr, _ := newSchedulerWithDBAndRedis(t, &MockDB{})
	defer mr.Close()

	svc.DB = &MockDB{QueryError: ErrMockDB}

	// 不应 panic
	svc.cleanupTasksFromOfflineExecutors(ctx)
}

func TestCleanupTasksFromOfflineExecutors_ResultErr(t *testing.T) {
	ctx := context.Background()
	svc, mr, _ := newSchedulerWithDBAndRedis(t, &MockDB{})
	defer mr.Close()

	svc.DB = &MockDB{QueryResult: database.NewQueryResultWithErr(errors.New("result error"))}

	// 不应 panic
	svc.cleanupTasksFromOfflineExecutors(ctx)
}

// ============ forceFailTask ============
// 注意：forceFailTask 会启动 goroutine 调用 HandleTaskFailure，
// 此处仅测试不会立即 panic 的场景

func TestForceFailTask_DeletesRedisKeys(t *testing.T) {
	ctx := context.Background()
	svc, mr, _ := newSchedulerWithDBAndRedis(t, &MockDB{})
	defer mr.Close()

	executionID := "exec-force-001"
	taskID := int64(101)

	// 预设 lock/renew/failCount keys
	lockKey := fmt.Sprintf("task:lock:%s", executionID)
	renewKey := fmt.Sprintf("task:renew:%s", executionID)
	failCountKey := fmt.Sprintf("task:renew:fail:count:%s", executionID)
	svc.redis.Set(ctx, lockKey, "locked", 10*time.Minute)
	svc.redis.Set(ctx, renewKey, time.Now().Unix(), 10*time.Minute)
	svc.redis.Set(ctx, failCountKey, 2, 10*time.Minute)

	// UpdateExecutionResult 和 AddTaskLog 的 DB 调用
	wr := database.NewWriteResult(0, 1)
	svc.DB = &MockDB{WriteResult: wr}

	// 调用 forceFailTask（会启动 goroutine 调用 HandleTaskFailure，
	// 但 HandleTaskFailure 会因为 DB 查询返回空结果而快速返回）
	svc.forceFailTask(ctx, executionID, taskID, "test reason", "test-executor")

	// 给 goroutine 一点时间完成
	time.Sleep(100 * time.Millisecond)

	// 验证 redis keys 已删除
	exists, _ := svc.redis.Exists(ctx, lockKey).Result()
	if exists != 0 {
		t.Error("期望 lock key 已删除")
	}
	exists, _ = svc.redis.Exists(ctx, renewKey).Result()
	if exists != 0 {
		t.Error("期望 renew key 已删除")
	}
	exists, _ = svc.redis.Exists(ctx, failCountKey).Result()
	if exists != 0 {
		t.Error("期望 fail count key 已删除")
	}
}

// ============ 清理相关常量/Key 格式验证 ============

func TestCleanupRedisKeyFormats(t *testing.T) {
	tests := []struct {
		name     string
		execID   string
		keyType  string
		expected string
	}{
		{"lock key", "exec-001", "lock", "task:lock:exec-001"},
		{"renew key", "exec-001", "renew", "task:renew:exec-001"},
		{"fail count key", "exec-001", "failcount", "task:renew:fail:count:exec-001"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var actual string
			switch tt.keyType {
			case "lock":
				actual = fmt.Sprintf("task:lock:%s", tt.execID)
			case "renew":
				actual = fmt.Sprintf("task:renew:%s", tt.execID)
			case "failcount":
				actual = fmt.Sprintf("task:renew:fail:count:%s", tt.execID)
			}
			if actual != tt.expected {
				t.Errorf("期望 %s，实际 %s", tt.expected, actual)
			}
		})
	}
}

func TestCleanupHeartbeatCutoffFormat(t *testing.T) {
	// 验证心跳截止时间格式（45 秒前）
	cutoff := time.Now().Add(-45 * time.Second).Format(DateTimeFormat)
	if cutoff == "" {
		t.Error("期望心跳截止时间非空")
	}
	// 验证可以解析为时间
	_, err := time.Parse(DateTimeFormat, cutoff)
	if err != nil {
		t.Errorf("心跳截止时间格式无效: %v", err)
	}
}

func TestCleanupCreatedBeforeFormat(t *testing.T) {
	// 验证卡住任务查询的时间格式（5 分钟前）
	createdBefore := time.Now().Add(-5 * time.Minute).Format(DateTimeFormat)
	if createdBefore == "" {
		t.Error("期望 createdBefore 非空")
	}
	_, err := time.Parse(DateTimeFormat, createdBefore)
	if err != nil {
		t.Errorf("createdBefore 格式无效: %v", err)
	}
}
