package cron

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	rqlite "github.com/rqlite/gorqlite"

	"github.com/lynnyq/bdopsflow/scheduler/internal/service"
	"github.com/lynnyq/bdopsflow/scheduler/pkg/database"
)

// mockDB 实现 database.DB 接口，用于测试中模拟数据库行为
type mockDB struct {
	queryOneErr      error
	queryOneParamErr error
	writeOneParamErr error
	writeParamErr    error
	queryResult      rqlite.QueryResult
	writeResult      rqlite.WriteResult
	writeResults     []rqlite.WriteResult
}

func (m *mockDB) QueryOne(sqlStatement string) (rqlite.QueryResult, error) {
	if m.queryOneErr != nil {
		return rqlite.QueryResult{}, m.queryOneErr
	}
	return m.queryResult, nil
}

func (m *mockDB) QueryOneParameterized(statement rqlite.ParameterizedStatement) (rqlite.QueryResult, error) {
	if m.queryOneParamErr != nil {
		return rqlite.QueryResult{}, m.queryOneParamErr
	}
	return m.queryResult, nil
}

func (m *mockDB) WriteOneParameterized(statement rqlite.ParameterizedStatement) (rqlite.WriteResult, error) {
	if m.writeOneParamErr != nil {
		return rqlite.WriteResult{}, m.writeOneParamErr
	}
	return m.writeResult, nil
}

func (m *mockDB) WriteParameterized(sqlStatements []rqlite.ParameterizedStatement) ([]rqlite.WriteResult, error) {
	if m.writeParamErr != nil {
		return nil, m.writeParamErr
	}
	return m.writeResults, nil
}

// newTestRedisClient 创建测试用 Redis 客户端，使用 DB 15 隔离测试数据
func newTestRedisClient(t *testing.T) *redis.Client {
	t.Helper()
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379", DB: 15})
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		t.Skipf("Redis 不可用，跳过测试: %v", err)
	}
	t.Cleanup(func() {
		client.FlushDB(context.Background())
		client.Close()
	})
	return client
}

// TestStart_WithRedis_LoadsPausedState 测试 Start 从 Redis 加载暂停状态
func TestStart_WithRedis_LoadsPausedState(t *testing.T) {
	client := newTestRedisClient(t)
	ctx := context.Background()

	// 场景1: Redis 中 paused="1"，Start 后应设置本地 paused=true
	t.Run("Redis暂停状态_应同步本地", func(t *testing.T) {
		client.Set(ctx, "scheduler:paused", "1", 0)

		cs := NewCronScheduler(nil, client)
		if err := cs.Start(); err != nil {
			t.Fatalf("Start 返回错误: %v", err)
		}

		if !cs.paused {
			t.Error("期望 paused=true，实际 false")
		}
	})

	// 场景2: Redis 中 paused="0"，Start 后本地 paused 应保持 false
	t.Run("Redis非暂停状态_本地保持非暂停", func(t *testing.T) {
		client.Set(ctx, "scheduler:paused", "0", 0)

		cs := NewCronScheduler(nil, client)
		if err := cs.Start(); err != nil {
			t.Fatalf("Start 返回错误: %v", err)
		}

		if cs.paused {
			t.Error("期望 paused=false，实际 true")
		}
	})

	// 场景3: Redis 中无 paused key，Start 后本地 paused 应保持 false
	t.Run("Redis无暂停key_本地保持非暂停", func(t *testing.T) {
		client.Del(ctx, "scheduler:paused")

		cs := NewCronScheduler(nil, client)
		if err := cs.Start(); err != nil {
			t.Fatalf("Start 返回错误: %v", err)
		}

		if cs.paused {
			t.Error("期望 paused=false，实际 true")
		}
	})
}

// TestSyncPausedFromRedis_V2 测试从 Redis 同步暂停状态到本地
func TestSyncPausedFromRedis_V2(t *testing.T) {
	client := newTestRedisClient(t)
	ctx := context.Background()

	t.Run("本地非暂停_Redis暂停_应更新为暂停", func(t *testing.T) {
		client.Set(ctx, "scheduler:paused", "1", 0)

		cs := NewCronScheduler(nil, client)
		cs.paused = false

		cs.syncPausedFromRedis()

		if !cs.paused {
			t.Error("期望 paused=true，实际 false")
		}
	})

	t.Run("本地暂停_Redis非暂停_应更新为非暂停", func(t *testing.T) {
		client.Set(ctx, "scheduler:paused", "0", 0)

		cs := NewCronScheduler(nil, client)
		cs.paused = true

		cs.syncPausedFromRedis()

		if cs.paused {
			t.Error("期望 paused=false，实际 true")
		}
	})

	t.Run("Redis无key_应保持本地状态不变", func(t *testing.T) {
		client.Del(ctx, "scheduler:paused")

		cs := NewCronScheduler(nil, client)
		cs.paused = false

		cs.syncPausedFromRedis()

		if cs.paused {
			t.Error("期望 paused=false（保持不变），实际 true")
		}
	})

	t.Run("同步后应更新lastRedisSync时间戳", func(t *testing.T) {
		client.Set(ctx, "scheduler:paused", "0", 0)

		cs := NewCronScheduler(nil, client)
		cs.lastRedisSync = time.Now().Add(-1 * time.Hour)

		beforeSync := cs.lastRedisSync
		cs.syncPausedFromRedis()

		if !cs.lastRedisSync.After(beforeSync) {
			t.Error("期望 lastRedisSync 被更新为更晚的时间")
		}
	})
}

// TestIsPaused_TriggersRedisSync 测试 IsPaused 在超过同步间隔时触发 Redis 同步
func TestIsPaused_TriggersRedisSync(t *testing.T) {
	client := newTestRedisClient(t)
	ctx := context.Background()

	t.Run("超过同步间隔_应从Redis同步暂停状态", func(t *testing.T) {
		client.Set(ctx, "scheduler:paused", "1", 0)

		cs := NewCronScheduler(nil, client)
		cs.redisSyncInterval = 1 * time.Millisecond
		// 设置一个过去的同步时间，确保超过间隔
		cs.lastRedisSync = time.Now().Add(-1 * time.Second)

		if !cs.IsPaused() {
			t.Error("期望 IsPaused 返回 true（从 Redis 同步），实际 false")
		}
	})

	t.Run("未超过同步间隔_应返回本地状态", func(t *testing.T) {
		client.Set(ctx, "scheduler:paused", "1", 0)

		cs := NewCronScheduler(nil, client)
		cs.redisSyncInterval = 1 * time.Hour
		cs.paused = false
		cs.lastRedisSync = time.Now()

		if cs.IsPaused() {
			t.Error("期望 IsPaused 返回 false（使用本地状态），实际 true")
		}
	})
}

// TestLoadAndRegisterTasks_V2 测试 loadAndRegisterTasks 的各种路径
func TestLoadAndRegisterTasks_V2(t *testing.T) {
	t.Run("svc为nil_应跳过加载", func(t *testing.T) {
		cs := NewCronScheduler(nil, nil)
		// 不应 panic
		cs.loadAndRegisterTasks()
	})

	t.Run("非主节点_应跳过加载", func(t *testing.T) {
		db := &mockDB{}
		svc := service.NewSchedulerService(db, nil)
		svc.SetLeader(false)

		cs := NewCronScheduler(svc, nil)
		cs.loadAndRegisterTasks()

		cs.mu.RLock()
		entries := len(cs.taskEntries)
		cs.mu.RUnlock()
		if entries != 0 {
			t.Errorf("期望 0 个任务注册，实际 %d", entries)
		}
	})

	t.Run("主节点_ScanPendingTasks返回错误_应跳过加载", func(t *testing.T) {
		db := &mockDB{
			queryOneErr: errors.New("database connection error"),
		}
		svc := service.NewSchedulerService(db, nil)
		svc.SetLeader(true)

		cs := NewCronScheduler(svc, nil)
		cs.loadAndRegisterTasks()

		cs.mu.RLock()
		entries := len(cs.taskEntries)
		cs.mu.RUnlock()
		if entries != 0 {
			t.Errorf("期望 0 个任务注册（查询出错），实际 %d", entries)
		}
	})

	t.Run("主节点_无待处理任务_应跳过加载", func(t *testing.T) {
		db := &mockDB{
			queryResult: rqlite.QueryResult{}, // 空结果
		}
		svc := service.NewSchedulerService(db, nil)
		svc.SetLeader(true)

		cs := NewCronScheduler(svc, nil)
		cs.loadAndRegisterTasks()

		cs.mu.RLock()
		entries := len(cs.taskEntries)
		cs.mu.RUnlock()
		if entries != 0 {
			t.Errorf("期望 0 个任务注册（无任务），实际 %d", entries)
		}
	})
}

// TestLoadAndRegisterTasks_PublicWrapper 测试公开的 LoadAndRegisterTasks 包装方法
func TestLoadAndRegisterTasks_PublicWrapper(t *testing.T) {
	t.Run("nil_svc_应安全返回", func(t *testing.T) {
		cs := NewCronScheduler(nil, nil)
		// 不应 panic
		cs.LoadAndRegisterTasks()
	})

	t.Run("非主节点_应安全返回", func(t *testing.T) {
		db := &mockDB{}
		svc := service.NewSchedulerService(db, nil)
		svc.SetLeader(false)

		cs := NewCronScheduler(svc, nil)
		cs.LoadAndRegisterTasks()
	})
}

// TestRecoverRunningTasks_V2 测试 recoverRunningTasks 的各种路径
func TestRecoverRunningTasks_V2(t *testing.T) {
	t.Run("svc为nil_应跳过恢复", func(t *testing.T) {
		cs := NewCronScheduler(nil, nil)
		// 不应 panic
		cs.recoverRunningTasks()
	})

	t.Run("非主节点_应跳过恢复", func(t *testing.T) {
		db := &mockDB{}
		svc := service.NewSchedulerService(db, nil)
		svc.SetLeader(false)

		cs := NewCronScheduler(svc, nil)
		cs.recoverRunningTasks()
	})

	t.Run("主节点_恢复任务查询出错_应安全返回", func(t *testing.T) {
		db := &mockDB{
			queryOneParamErr: errors.New("database error"),
		}
		svc := service.NewSchedulerService(db, nil)
		svc.SetLeader(true)

		cs := NewCronScheduler(svc, nil)
		// 不应 panic，错误仅记录日志
		cs.recoverRunningTasks()
	})

	t.Run("主节点_无运行中任务_应安全返回", func(t *testing.T) {
		db := &mockDB{
			queryResult: rqlite.QueryResult{}, // 空结果
		}
		svc := service.NewSchedulerService(db, nil)
		svc.SetLeader(true)

		cs := NewCronScheduler(svc, nil)
		cs.recoverRunningTasks()
	})
}

// TestExecuteTask_V2 测试 executeTask 的各种路径
func TestExecuteTask_V2(t *testing.T) {
	t.Run("svc为nil_应跳过执行", func(t *testing.T) {
		cs := NewCronScheduler(nil, nil)
		// 不应 panic
		cs.executeTask(1)
	})

	t.Run("非主节点_应跳过执行", func(t *testing.T) {
		db := &mockDB{}
		svc := service.NewSchedulerService(db, nil)
		svc.SetLeader(false)

		cs := NewCronScheduler(svc, nil)
		cs.isLeader = false
		cs.executeTask(1)
	})

	t.Run("主节点_调度器已暂停_应跳过执行", func(t *testing.T) {
		db := &mockDB{}
		svc := service.NewSchedulerService(db, nil)
		svc.SetLeader(true)

		cs := NewCronScheduler(svc, nil)
		cs.isLeader = true
		cs.paused = true

		cs.executeTask(1)
	})

	t.Run("主节点_获取任务失败_应跳过执行", func(t *testing.T) {
		// GetTaskByID 调用 QueryOneParameterized，返回空结果时
		// qr.Next() 为 false，返回 "task not found" 错误
		db := &mockDB{
			queryResult: rqlite.QueryResult{}, // 空结果 → task not found
		}
		svc := service.NewSchedulerService(db, nil)
		svc.SetLeader(true)

		cs := NewCronScheduler(svc, nil)
		cs.isLeader = true
		cs.paused = false

		// 不应 panic，GetTaskByID 返回错误后仅记录日志
		cs.executeTask(999)
	})
}

// TestAcquireTaskLock_V2 测试 acquireTaskLock 的 Redis 路径
func TestAcquireTaskLock_V2(t *testing.T) {
	t.Run("Redis可用_首次获取锁成功", func(t *testing.T) {
		client := newTestRedisClient(t)
		cs := NewCronScheduler(nil, client)

		ctx := context.Background()
		acquired, err := cs.acquireTaskLock(ctx, 1001, 30*time.Second)
		if err != nil {
			t.Fatalf("获取锁失败: %v", err)
		}
		if !acquired {
			t.Error("期望首次获取锁成功")
		}
	})

	t.Run("Redis可用_同节点重新获取成功", func(t *testing.T) {
		client := newTestRedisClient(t)
		cs := NewCronScheduler(nil, client)

		ctx := context.Background()
		// 首次获取
		acquired1, _ := cs.acquireTaskLock(ctx, 1002, 30*time.Second)
		if !acquired1 {
			t.Fatal("首次获取锁失败")
		}
		// 同节点重新获取（刷新）
		acquired2, err := cs.acquireTaskLock(ctx, 1002, 30*time.Second)
		if err != nil {
			t.Fatalf("重新获取锁失败: %v", err)
		}
		if !acquired2 {
			t.Error("期望同节点重新获取锁成功")
		}
	})

	t.Run("Redis可用_不同节点获取失败", func(t *testing.T) {
		client := newTestRedisClient(t)
		cs1 := NewCronScheduler(nil, client)

		ctx := context.Background()
		// 节点1 获取锁
		acquired1, _ := cs1.acquireTaskLock(ctx, 1003, 30*time.Second)
		if !acquired1 {
			t.Fatal("节点1 获取锁失败")
		}

		// 节点2 尝试获取同一任务锁
		cs2 := &CronScheduler{redis: client, nodeID: "another-node-12345"}
		acquired2, err := cs2.acquireTaskLock(ctx, 1003, 30*time.Second)
		if err != nil {
			t.Fatalf("节点2 获取锁不应返回错误: %v", err)
		}
		if acquired2 {
			t.Error("期望不同节点获取锁失败")
		}
	})
}

// TestReleaseTaskLock_V2 测试 releaseTaskLock 的 Redis 路径
func TestReleaseTaskLock_V2(t *testing.T) {
	t.Run("Redis可用_释放他人锁_应记录警告", func(t *testing.T) {
		client := newTestRedisClient(t)
		ctx := context.Background()

		// 节点1 获取锁
		cs1 := NewCronScheduler(nil, client)
		acquired, _ := cs1.acquireTaskLock(ctx, 2001, 30*time.Second)
		if !acquired {
			t.Fatal("节点1 获取锁失败")
		}

		// 节点2 尝试释放（非所有者）
		cs2 := &CronScheduler{redis: client, nodeID: "another-node-release-test"}
		cs2.releaseTaskLock(ctx, 2001)

		// 锁应仍然存在（非所有者无法释放）
		// 验证: 节点1 仍可重新获取（因为锁还在）
		acquiredAgain, _ := cs1.acquireTaskLock(ctx, 2001, 30*time.Second)
		if !acquiredAgain {
			t.Error("期望节点1 仍能刷新锁（非所有者释放无效）")
		}
	})

	t.Run("Redis可用_释放不存在的锁_应安全返回", func(t *testing.T) {
		client := newTestRedisClient(t)
		cs := NewCronScheduler(nil, client)

		ctx := context.Background()
		// 释放一个不存在的锁，不应 panic
		cs.releaseTaskLock(ctx, 9999)
	})
}

// TestRenewTaskLock_V2 测试 renewTaskLock 的 Redis 路径
func TestRenewTaskLock_V2(t *testing.T) {
	t.Run("Redis可用_续期自己持有的锁_成功", func(t *testing.T) {
		client := newTestRedisClient(t)
		cs := NewCronScheduler(nil, client)

		ctx := context.Background()
		// 先获取锁
		acquired, _ := cs.acquireTaskLock(ctx, 3001, 30*time.Second)
		if !acquired {
			t.Fatal("获取锁失败")
		}

		// 续期
		err := cs.renewTaskLock(ctx, 3001, 30*time.Second)
		if err != nil {
			t.Errorf("续期锁失败: %v", err)
		}
	})

	t.Run("Redis可用_续期他人持有的锁_应返回错误", func(t *testing.T) {
		client := newTestRedisClient(t)
		ctx := context.Background()

		// 节点1 获取锁
		cs1 := NewCronScheduler(nil, client)
		cs1.acquireTaskLock(ctx, 3002, 30*time.Second)

		// 节点2 尝试续期
		cs2 := &CronScheduler{redis: client, nodeID: "another-node-renew-test"}
		err := cs2.renewTaskLock(ctx, 3002, 30*time.Second)
		if err == nil {
			t.Error("期望续期他人锁返回错误")
		}
		if !strings.Contains(err.Error(), "not owned") {
			t.Errorf("期望错误包含 'not owned'，实际: %v", err)
		}
	})

	t.Run("Redis可用_续期不存在的锁_应返回错误", func(t *testing.T) {
		client := newTestRedisClient(t)
		cs := NewCronScheduler(nil, client)

		ctx := context.Background()
		err := cs.renewTaskLock(ctx, 9999, 30*time.Second)
		if err == nil {
			t.Error("期望续期不存在的锁返回错误")
		}
	})
}

// TestStartLockRenewer_V2 测试 startLockRenewer 的退出路径
func TestStartLockRenewer_V2(t *testing.T) {
	t.Run("通过stopCh退出", func(t *testing.T) {
		cs := NewCronScheduler(nil, nil)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		stopCh := make(chan struct{})
		done := make(chan struct{})

		go func() {
			cs.startLockRenewer(ctx, 4001, 30*time.Second, stopCh)
			close(done)
		}()

		// 关闭 stop 通道，触发退出
		close(stopCh)

		select {
		case <-done:
			// 成功退出
		case <-time.After(2 * time.Second):
			t.Error("startLockRenewer 未在 stopCh 关闭后退出")
		}
	})

	t.Run("通过context取消退出", func(t *testing.T) {
		cs := NewCronScheduler(nil, nil)

		ctx, cancel := context.WithCancel(context.Background())
		stopCh := make(chan struct{})
		done := make(chan struct{})

		go func() {
			cs.startLockRenewer(ctx, 4002, 30*time.Second, stopCh)
			close(done)
		}()

		// 取消 context，触发退出
		cancel()

		select {
		case <-done:
			// 成功退出
		case <-time.After(2 * time.Second):
			t.Error("startLockRenewer 未在 context 取消后退出")
		}
	})

	t.Run("无Redis时定时器触发_应安全返回", func(t *testing.T) {
		// 无 Redis 时 renewTaskLock 返回 nil，定时器触发后不退出
		// 但通过 stopCh 退出
		cs := NewCronScheduler(nil, nil)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		stopCh := make(chan struct{})
		done := make(chan struct{})

		// 使用极短的续期间隔让定时器快速触发
		go func() {
			cs.startLockRenewer(ctx, 4003, 3*time.Millisecond, stopCh)
			close(done)
		}()

		// 等待定时器至少触发一次
		time.Sleep(20 * time.Millisecond)

		// 通过 stopCh 退出
		close(stopCh)

		select {
		case <-done:
			// 成功退出
		case <-time.After(2 * time.Second):
			t.Error("startLockRenewer 未在 stopCh 关闭后退出")
		}
	})
}

// TestOnBecomeLeader_WithService 测试有 service 时 OnBecomeLeader 的行为
func TestOnBecomeLeader_WithService(t *testing.T) {
	t.Run("主节点_非主节点_应启动后台任务", func(t *testing.T) {
		db := &mockDB{
			queryResult: rqlite.QueryResult{}, // 空 → ScanPendingTasks 返回空
		}
		svc := service.NewSchedulerService(db, nil)
		svc.SetLeader(true)

		cs := NewCronScheduler(svc, nil)
		defer cs.Stop()

		cs.OnBecomeLeader()

		if !cs.isLeader {
			t.Error("期望 isLeader=true")
		}
		if !cs.started {
			t.Error("期望 started=true")
		}

		// 等待后台 goroutine 完成
		time.Sleep(200 * time.Millisecond)
	})

	t.Run("已经是主节点_重复调用应为无操作", func(t *testing.T) {
		db := &mockDB{}
		svc := service.NewSchedulerService(db, nil)
		svc.SetLeader(true)

		cs := NewCronScheduler(svc, nil)
		defer cs.Stop()

		cs.OnBecomeLeader()
		time.Sleep(100 * time.Millisecond)

		// 再次调用应为无操作
		cs.OnBecomeLeader()

		if !cs.isLeader {
			t.Error("期望 isLeader 保持 true")
		}
	})
}

// TestOnLoseLeader_WithRegisteredTasks 测试有已注册任务时失去主节点地位
func TestOnLoseLeader_WithRegisteredTasks(t *testing.T) {
	cs := NewCronScheduler(nil, nil)
	defer cs.Stop()

	if err := cs.Start(); err != nil {
		t.Fatalf("Start 失败: %v", err)
	}

	// 先成为主节点
	cs.OnBecomeLeader()

	// 注册多个任务
	cs.RegisterTask(1, "0 * * * * *")
	cs.RegisterTask(2, "30 * * * * *")
	cs.RegisterTask(3, "0 */5 * * * *")

	cs.mu.RLock()
	countBefore := len(cs.taskEntries)
	cs.mu.RUnlock()
	if countBefore != 3 {
		t.Fatalf("期望 3 个已注册任务，实际 %d", countBefore)
	}

	// 失去主节点
	cs.OnLoseLeader()

	cs.mu.RLock()
	countAfter := len(cs.taskEntries)
	isLeader := cs.isLeader
	cs.mu.RUnlock()

	if isLeader {
		t.Error("期望 isLeader=false")
	}
	if countAfter != 0 {
		t.Errorf("期望 0 个任务（已清空），实际 %d", countAfter)
	}
}

// TestRegisterTask_InvalidThenValid5Field 测试无效表达式后尝试5位格式
func TestRegisterTask_InvalidThenValid5Field(t *testing.T) {
	cs := NewCronScheduler(nil, nil)
	if err := cs.Start(); err != nil {
		t.Fatalf("Start 失败: %v", err)
	}
	defer cs.Stop()

	t.Run("纯无效表达式_不应注册", func(t *testing.T) {
		// 这个表达式既不是6位有效cron，也不是5位标准cron
		cs.RegisterTask(500, "not a cron at all !!!")

		cs.mu.RLock()
		_, exists := cs.taskEntries[500]
		cs.mu.RUnlock()
		if exists {
			t.Error("期望无效表达式不注册任务")
		}
	})

	t.Run("6位有效表达式_应注册成功", func(t *testing.T) {
		cs.RegisterTask(501, "0 */2 * * * *")

		cs.mu.RLock()
		_, exists := cs.taskEntries[501]
		cs.mu.RUnlock()
		if !exists {
			t.Error("期望6位有效表达式注册成功")
		}
	})
}

// 确保编译时检查 database.DB 接口实现
var _ database.DB = (*mockDB)(nil)
