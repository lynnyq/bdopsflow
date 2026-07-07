package cron

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestGenerateNodeID 验证 generateNodeID 返回非空字符串
func TestGenerateNodeID(t *testing.T) {
	nodeID := generateNodeID()
	if nodeID == "" {
		t.Fatal("expected non-empty node ID")
	}
}

// TestGenerateNodeID_ContainsHostnameAndPID 验证 nodeID 包含 hostname 和 pid
func TestGenerateNodeID_ContainsHostnameAndPID(t *testing.T) {
	nodeID := generateNodeID()
	if !strings.Contains(nodeID, "-") {
		t.Errorf("expected node ID to contain '-' separator, got %q", nodeID)
	}
	parts := strings.SplitN(nodeID, "-", -1)
	if len(parts) < 2 {
		t.Fatalf("expected at least 2 parts in node ID, got %d: %q", len(parts), nodeID)
	}
	// 最后一部分应该是 PID（数字）
	lastPart := parts[len(parts)-1]
	if lastPart == "" {
		t.Errorf("expected last part to be PID, got empty string")
	}
}

// TestGenerateNodeID_Deterministic 验证同一进程多次调用返回相同结果
func TestGenerateNodeID_Deterministic(t *testing.T) {
	id1 := generateNodeID()
	id2 := generateNodeID()
	if id1 != id2 {
		t.Errorf("expected same node ID within same process, got %q and %q", id1, id2)
	}
}

// TestNewCronScheduler_NilDeps 验证使用 nil 依赖创建调度器的初始状态
func TestNewCronScheduler_NilDeps(t *testing.T) {
	cs := NewCronScheduler(nil, nil)
	if cs == nil {
		t.Fatal("expected non-nil CronScheduler")
	}
	if cs.cron == nil {
		t.Error("expected cron to be initialized")
	}
	if cs.svc != nil {
		t.Error("expected svc to be nil")
	}
	if cs.redis != nil {
		t.Error("expected redis to be nil")
	}
	if cs.taskEntries == nil {
		t.Error("expected taskEntries to be initialized")
	}
	if len(cs.taskEntries) != 0 {
		t.Errorf("expected empty taskEntries, got %d", len(cs.taskEntries))
	}
	if cs.paused {
		t.Error("expected paused to be false initially")
	}
	if cs.isLeader {
		t.Error("expected isLeader to be false initially")
	}
	if cs.started {
		t.Error("expected started to be false initially")
	}
	if cs.nodeID == "" {
		t.Error("expected nodeID to be non-empty")
	}
	if cs.redisSyncInterval != 5*time.Second {
		t.Errorf("expected redisSyncInterval to be 5s, got %v", cs.redisSyncInterval)
	}
}

// TestCronScheduler_PauseResume_NilRedis 验证无 Redis 时 Pause/Resume 仍能更新本地状态
func TestCronScheduler_PauseResume_NilRedis(t *testing.T) {
	cs := NewCronScheduler(nil, nil)

	if cs.IsPaused() {
		t.Error("expected not paused initially")
	}

	cs.Pause()
	if !cs.IsPaused() {
		t.Error("expected paused after Pause()")
	}

	cs.Resume()
	if cs.IsPaused() {
		t.Error("expected not paused after Resume()")
	}
}

// TestCronScheduler_IsPaused_NilRedis 验证无 Redis 时 IsPaused 返回本地状态
func TestCronScheduler_IsPaused_NilRedis(t *testing.T) {
	cs := NewCronScheduler(nil, nil)

	// 无 Redis 时，IsPaused 不应尝试同步，直接返回本地状态
	if cs.IsPaused() {
		t.Error("expected false initially")
	}

	cs.mu.Lock()
	cs.paused = true
	cs.mu.Unlock()

	if !cs.IsPaused() {
		t.Error("expected true after setting paused")
	}

	cs.mu.Lock()
	cs.paused = false
	cs.mu.Unlock()

	if cs.IsPaused() {
		t.Error("expected false after clearing paused")
	}
}

// TestCronScheduler_Start_NilRedis 验证无 Redis 时 Start 不 panic
func TestCronScheduler_Start_NilRedis(t *testing.T) {
	cs := NewCronScheduler(nil, nil)
	err := cs.Start()
	if err != nil {
		t.Errorf("expected no error from Start(), got %v", err)
	}
}

// TestCronScheduler_OnLoseLeader_NotLeader 验证非主节点调用 OnLoseLeader 是无操作
func TestCronScheduler_OnLoseLeader_NotLeader(t *testing.T) {
	cs := NewCronScheduler(nil, nil)
	// 未成为主节点时调用 OnLoseLeader 应为无操作
	cs.OnLoseLeader()
	if cs.isLeader {
		t.Error("expected isLeader to remain false")
	}
}

// TestCronScheduler_OnBecomeLeader_NilSvc 验证 nil svc 时 OnBecomeLeader 不 panic
func TestCronScheduler_OnBecomeLeader_NilSvc(t *testing.T) {
	cs := NewCronScheduler(nil, nil)
	defer cs.Stop()

	cs.OnBecomeLeader()

	if !cs.isLeader {
		t.Error("expected isLeader to be true after OnBecomeLeader")
	}
	if !cs.started {
		t.Error("expected started to be true after OnBecomeLeader")
	}

	// 等待后台 goroutine 完成（它们会因 nil svc 而立即返回）
	time.Sleep(100 * time.Millisecond)

	// 再次调用应为无操作
	cs.OnBecomeLeader()
}

// TestCronScheduler_OnLoseLeader_AfterBecomeLeader 验证成为主节点后失去主节点地位
func TestCronScheduler_OnLoseLeader_AfterBecomeLeader(t *testing.T) {
	cs := NewCronScheduler(nil, nil)
	defer cs.Stop()

	cs.OnBecomeLeader()
	if !cs.isLeader {
		t.Fatal("expected isLeader to be true")
	}

	cs.OnLoseLeader()
	if cs.isLeader {
		t.Error("expected isLeader to be false after OnLoseLeader")
	}

	// taskEntries 应被清空
	cs.mu.RLock()
	entries := len(cs.taskEntries)
	cs.mu.RUnlock()
	if entries != 0 {
		t.Errorf("expected 0 task entries after OnLoseLeader, got %d", entries)
	}
}

// TestCronScheduler_RenewTaskLock_NilRedis 验证 nil Redis 时 renewTaskLock 返回 nil
func TestCronScheduler_RenewTaskLock_NilRedis(t *testing.T) {
	cs := NewCronScheduler(nil, nil)
	err := cs.renewTaskLock(context.Background(), 1, 30*time.Second)
	if err != nil {
		t.Errorf("expected nil error with nil redis, got %v", err)
	}
}

// TestCronScheduler_GetUptime_Positive 验证运行时长为正值
func TestCronScheduler_GetUptime_Positive(t *testing.T) {
	cs := NewCronScheduler(nil, nil)
	time.Sleep(50 * time.Millisecond)
	uptime := cs.GetUptime()
	if uptime <= 0 {
		t.Errorf("expected positive uptime, got %v", uptime)
	}
	if uptime < 50*time.Millisecond {
		t.Errorf("expected uptime >= 50ms, got %v", uptime)
	}
}

// TestCronScheduler_RegisterTask_NilSvc 验证 nil svc 时注册任务仍能成功
func TestCronScheduler_RegisterTask_NilSvc(t *testing.T) {
	cs := NewCronScheduler(nil, nil)
	if err := cs.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer cs.Stop()

	taskID := int64(100)
	cronExpr := "0 */5 * * * *"
	cs.RegisterTask(taskID, cronExpr)

	cs.mu.RLock()
	_, exists := cs.taskEntries[taskID]
	cs.mu.RUnlock()
	if !exists {
		t.Error("expected task to be registered")
	}
}

// TestCronScheduler_UnregisterTask_AfterRegister 验证注册后取消注册
func TestCronScheduler_UnregisterTask_AfterRegister(t *testing.T) {
	cs := NewCronScheduler(nil, nil)
	if err := cs.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer cs.Stop()

	taskID := int64(200)
	cs.RegisterTask(taskID, "0 * * * * *")

	cs.mu.RLock()
	existsBefore := cs.taskEntries[taskID] != 0
	cs.mu.RUnlock()
	if !existsBefore {
		t.Fatal("expected task to be registered")
	}

	cs.UnregisterTask(taskID)

	cs.mu.RLock()
	_, existsAfter := cs.taskEntries[taskID]
	cs.mu.RUnlock()
	if existsAfter {
		t.Error("expected task to be unregistered")
	}
}
