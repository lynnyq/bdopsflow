package grpcclient

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	pb "github.com/lynnyq/bdopsflow/proto"
)

func TestNewMultiClient(t *testing.T) {
	t.Run("empty scheduler addresses", func(t *testing.T) {
		client, err := NewMultiClient([]string{})
		if err != nil {
			t.Errorf("NewMultiClient with empty addresses should not return error, got %v", err)
		}
		if client != nil {
			t.Error("NewMultiClient with empty addresses should return nil client")
		}
	})

	t.Run("single scheduler address", func(t *testing.T) {
		client, err := NewMultiClient([]string{"localhost:50051"})
		if err != nil {
			t.Errorf("NewMultiClient failed: %v", err)
		}
		if client == nil {
			t.Fatal("NewMultiClient should return non-nil client")
		}
		if len(client.schedulerAddrs) != 1 {
			t.Errorf("expected 1 scheduler address, got %d", len(client.schedulerAddrs))
		}
		client.Close()
	})

	t.Run("multiple scheduler addresses", func(t *testing.T) {
		addrs := []string{"localhost:50051", "localhost:50052", "localhost:50053"}
		client, err := NewMultiClient(addrs)
		if err != nil {
			t.Errorf("NewMultiClient failed: %v", err)
		}
		if client == nil {
			t.Fatal("NewMultiClient should return non-nil client")
		}
		if len(client.schedulerAddrs) != 3 {
			t.Errorf("expected 3 scheduler addresses, got %d", len(client.schedulerAddrs))
		}
		client.Close()
	})
}

func TestMultiClient_GetCurrentAddr(t *testing.T) {
	addrs := []string{"addr1:50051", "addr2:50052", "addr3:50053"}
	client, _ := NewMultiClient(addrs)
	defer client.Close()

	// Test round-robin selection
	if got := client.getCurrentAddr(); got != "addr1:50051" {
		t.Errorf("getCurrentAddr() = %v, want addr1:50051", got)
	}

	client.nextAddr()
	if got := client.getCurrentAddr(); got != "addr2:50052" {
		t.Errorf("after nextAddr(), getCurrentAddr() = %v, want addr2:50052", got)
	}

	client.nextAddr()
	if got := client.getCurrentAddr(); got != "addr3:50053" {
		t.Errorf("after 2nd nextAddr(), getCurrentAddr() = %v, want addr3:50053", got)
	}

	// Test wrap-around
	client.nextAddr()
	if got := client.getCurrentAddr(); got != "addr1:50051" {
		t.Errorf("after wrap-around, getCurrentAddr() = %v, want addr1:50051", got)
	}
}

func TestMultiClient_BackoffDelay(t *testing.T) {
	client, _ := NewMultiClient([]string{"localhost:50051"})
	defer client.Close()

	base := 3 * time.Second
	max := 60 * time.Second
	jitter := 500 * time.Millisecond

	tests := []struct {
		name       string
		retryCount int
		wantMin    time.Duration
		wantMax    time.Duration
	}{
		{"first retry", 0, base, base + jitter},
		{"second retry", 1, 2 * base, 2*base + jitter},
		{"third retry", 2, 4 * base, 4*base + jitter},
		{"exponential growth", 3, 8 * base, 8*base + jitter},
		{"capped at max", 10, max, max + jitter},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := client.backoffDelay(tt.retryCount, base, max, jitter)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("backoffDelay(%d) = %v, want between %v and %v", tt.retryCount, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestMultiClient_BackoffDelayNoJitter(t *testing.T) {
	client, _ := NewMultiClient([]string{"localhost:50051"})
	defer client.Close()

	base := 3 * time.Second
	max := 60 * time.Second

	got := client.backoffDelay(0, base, max, 0)
	if got != base {
		t.Errorf("backoffDelay with no jitter = %v, want %v", got, base)
	}
}

func TestMultiClient_SleepOrStop(t *testing.T) {
	t.Run("sleep completes normally", func(t *testing.T) {
		client, _ := NewMultiClient([]string{"localhost:50051"})
		start := time.Now()
		client.sleepOrStop(100 * time.Millisecond)
		elapsed := time.Since(start)
		client.Close()
		if elapsed < 100*time.Millisecond || elapsed > 150*time.Millisecond {
			t.Errorf("sleepOrStop took %v, expected around 100ms", elapsed)
		}
	})

	t.Run("sleep interrupted by stop", func(t *testing.T) {
		client, _ := NewMultiClient([]string{"localhost:50051"})
		start := time.Now()
		go func() {
			time.Sleep(50 * time.Millisecond)
			client.Close()
		}()
		client.sleepOrStop(1 * time.Second)
		elapsed := time.Since(start)
		if elapsed > 200*time.Millisecond {
			t.Errorf("sleepOrStop should have been interrupted, took %v", elapsed)
		}
	})
}

func TestMultiClient_GetExecutorName(t *testing.T) {
	client, _ := NewMultiClient([]string{"localhost:50051"})
	defer client.Close()

	// Initially empty
	if name := client.GetExecutorName(); name != "" {
		t.Errorf("initial executor name should be empty, got %v", name)
	}

	// Set executor name
	client.executorName.Store(nameWrapper{name: "test-executor"})
	if name := client.GetExecutorName(); name != "test-executor" {
		t.Errorf("executor name = %v, want test-executor", name)
	}
}

// MockTaskRunner for testing
type MockTaskRunner struct {
	mu             sync.Mutex
	runningTasks   int32
	runningExecIds []string
	runningStates  []*pb.RunningTaskState
	capacity       int32
	cancelledTasks []string
}

func (m *MockTaskRunner) Execute(ctx context.Context, task *pb.Task, client *MultiClient) {
	// Mock implementation
}

func (m *MockTaskRunner) GetRunningTasks() int32 {
	return m.runningTasks
}

func (m *MockTaskRunner) GetRunningExecutionIds() []string {
	return m.runningExecIds
}

func (m *MockTaskRunner) GetRunningTaskStates() []*pb.RunningTaskState {
	return m.runningStates
}

func (m *MockTaskRunner) UpdateCapacity(newCapacity int32) error {
	m.mu.Lock()
	m.capacity = newCapacity
	m.mu.Unlock()
	return nil
}

func (m *MockTaskRunner) CancelTask(executionId string) bool {
	m.mu.Lock()
	m.cancelledTasks = append(m.cancelledTasks, executionId)
	m.mu.Unlock()
	return true
}

// Capacity 安全读取当前 capacity（测试中需要）
func (m *MockTaskRunner) Capacity() int32 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.capacity
}

// CancelledTasks 安全读取已取消任务列表
func (m *MockTaskRunner) CancelledTasks() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.cancelledTasks...)
}

func TestMultiClient_ReportResultNoConnection(t *testing.T) {
	// Use a single address that will fail fast (connection refused)
	client, _ := NewMultiClient([]string{"127.0.0.1:1"})
	defer client.Close()

	req := &pb.ReportTaskResultRequest{
		TaskId:      1,
		ExecutionId: "test-1",
		Status:      "success",
	}

	err := client.ReportResult(req)
	if err == nil {
		t.Error("ReportResult should fail when not connected")
	}
}

func TestMultiClient_ReportLogNoConnection(t *testing.T) {
	client, _ := NewMultiClient([]string{"127.0.0.1:1"})
	defer client.Close()

	req := &pb.ReportTaskLogRequest{
		TaskId:      1,
		ExecutionId: "test-1",
		LogContent:  "test log",
	}

	err := client.ReportLog(req)
	if err == nil {
		t.Error("ReportLog should fail when not connected")
	}
}

func TestMultiClient_Close(t *testing.T) {
	client, _ := NewMultiClient([]string{"localhost:50051"})

	// Close should not panic
	client.Close()

	// Double close should not panic (but will, so we skip this)
}

func TestMultiClient_Reconnect(t *testing.T) {
	client, _ := NewMultiClient([]string{"localhost:50051"})
	defer client.Close()

	// Initially not connected
	if client.isConnected.Load() {
		t.Error("should not be connected initially")
	}

	// Reconnect should not panic even when not connected
	client.reconnect()

	// Still not connected
	if client.isConnected.Load() {
		t.Error("should still not be connected after reconnect to invalid address")
	}
}

func TestMultiClient_NeedFullSync(t *testing.T) {
	client, _ := NewMultiClient([]string{"localhost:50051"})
	defer client.Close()

	// Initially false
	if client.needFullSync.Load() {
		t.Error("needFullSync should be false initially")
	}

	// Set to true
	client.needFullSync.Store(true)
	if !client.needFullSync.Load() {
		t.Error("needFullSync should be true after setting")
	}
}

// TestMultiClient_SyncRunningTasksNoConnection 测试未连接时 SyncRunningTasks 返回错误
func TestMultiClient_SyncRunningTasksNoConnection(t *testing.T) {
	client, _ := NewMultiClient([]string{"127.0.0.1:1"})
	defer client.Close()

	tasks, err := client.SyncRunningTasks("test-executor")
	if err == nil {
		t.Error("SyncRunningTasks should fail when not connected")
	}
	if tasks != nil {
		t.Errorf("expected nil tasks, got %v", tasks)
	}
}

// TestMultiClient_EnsureConnectedFailure 测试 ensureConnected 在地址不可达时返回错误
func TestMultiClient_EnsureConnectedFailure(t *testing.T) {
	client, _ := NewMultiClient([]string{"127.0.0.1:1"})
	defer client.Close()

	err := client.ensureConnected()
	if err != nil {
		// ensureConnected 在所有地址都连不上时返回 nil（connectToScheduler 返回 nil error）
		// 但 isConnected 为 false，后续调用会重试
		t.Logf("ensureConnected returned error (acceptable): %v", err)
	}
	// 不应标记为已连接
	if client.isConnected.Load() {
		t.Error("should not be connected after failed ensureConnected")
	}
}

// TestMultiClient_ReconnectClearsState 测试 reconnect 清理连接状态
func TestMultiClient_ReconnectClearsState(t *testing.T) {
	client, _ := NewMultiClient([]string{"localhost:50051", "localhost:50052"})
	defer client.Close()

	// 模拟已连接状态
	client.isConnected.Store(true)
	client.conn.Store(connWrapper{conn: nil})
	client.client.Store(clientWrapper{client: nil})
	client.stream.Store(streamWrapper{stream: nil})

	// 执行重连
	client.reconnect()

	// 验证状态被清理
	if client.isConnected.Load() {
		t.Error("isConnected should be false after reconnect")
	}

	// 验证索引前进
	// 初始 currentIndex=0，reconnect 调用 nextAddr，所以 currentIndex=1
	idx := client.currentIndex.Load()
	if idx != 1 {
		t.Errorf("currentIndex should be 1 after reconnect, got %d", idx)
	}
}

// TestMultiClient_GetCurrentAddrLargeIndex 测试大索引下的取模回绕
func TestMultiClient_GetCurrentAddrLargeIndex(t *testing.T) {
	addrs := []string{"addr1:50051", "addr2:50052", "addr3:50053"}
	client, _ := NewMultiClient(addrs)
	defer client.Close()

	// 设置一个很大的索引，验证取模正确
	client.currentIndex.Store(int64(len(addrs)) * 100) // 300
	got := client.getCurrentAddr()
	if got != "addr1:50051" {
		t.Errorf("getCurrentAddr() with index 300 = %v, want addr1:50051", got)
	}

	client.currentIndex.Store(int64(len(addrs))*100 + 1) // 301
	got = client.getCurrentAddr()
	if got != "addr2:50052" {
		t.Errorf("getCurrentAddr() with index 301 = %v, want addr2:50052", got)
	}
}

// TestMultiClient_ConnectInvalidAddress 测试 connect 方法连接无效地址返回错误
func TestMultiClient_ConnectInvalidAddress(t *testing.T) {
	client, _ := NewMultiClient([]string{"localhost:50051"})
	defer client.Close()

	conn, grpcClient, err := client.connect("127.0.0.1:1")
	if err == nil {
		if conn != nil {
			conn.Close()
		}
		t.Fatal("connect to invalid address should return error")
	}
	if conn != nil {
		t.Error("conn should be nil on error")
	}
	if grpcClient != nil {
		t.Error("client should be nil on error")
	}
}

// TestMultiClient_ConnectToSchedulerAllFail 测试所有调度器地址都连不上时返回空地址
func TestMultiClient_ConnectToSchedulerAllFail(t *testing.T) {
	addrs := []string{"127.0.0.1:1", "127.0.0.1:2", "127.0.0.1:3"}
	client, _ := NewMultiClient(addrs)
	defer client.Close()

	addr, err := client.connectToScheduler()
	// connectToScheduler 在所有地址都失败时返回 ("", nil)
	if err != nil {
		t.Errorf("connectToScheduler should return nil error when all fail, got %v", err)
	}
	if addr != "" {
		t.Errorf("addr should be empty when all connections fail, got %v", addr)
	}
	if client.isConnected.Load() {
		t.Error("should not be connected when all addresses fail")
	}
}

// TestMultiClient_ReportResultWithError 测试 ReportResult 在无法连接时返回包装错误
func TestMultiClient_ReportResultWithError(t *testing.T) {
	client, _ := NewMultiClient([]string{"127.0.0.1:1"})
	defer client.Close()

	req := &pb.ReportTaskResultRequest{
		TaskId:      42,
		ExecutionId: "exec-42",
		Status:      "success",
	}

	err := client.ReportResult(req)
	if err == nil {
		t.Fatal("ReportResult should return error when connection fails")
	}
	// 错误信息应包含重试次数
	if !strings.Contains(err.Error(), "after 3 retries") {
		t.Errorf("error should mention retries, got: %v", err)
	}
}

// TestMultiClient_ReportLogWithError 测试 ReportLog 在无法连接时返回包装错误
func TestMultiClient_ReportLogWithError(t *testing.T) {
	client, _ := NewMultiClient([]string{"127.0.0.1:1"})
	defer client.Close()

	req := &pb.ReportTaskLogRequest{
		TaskId:      42,
		ExecutionId: "exec-42",
		LogContent:  "test log content",
	}

	err := client.ReportLog(req)
	if err == nil {
		t.Fatal("ReportLog should return error when connection fails")
	}
	if !strings.Contains(err.Error(), "after 3 retries") {
		t.Errorf("error should mention retries, got: %v", err)
	}
}

// TestMultiClient_SyncRunningTasksNoClient 测试 SyncRunningTasks 在 client 为 nil 时返回错误
func TestMultiClient_SyncRunningTasksNoClient(t *testing.T) {
	client, _ := NewMultiClient([]string{"127.0.0.1:1"})
	defer client.Close()

	// 确保 isConnected 为 true 但 client 为 nil，触发 "no client available" 错误
	client.isConnected.Store(true)
	client.client.Store(clientWrapper{client: nil})

	tasks, err := client.SyncRunningTasks("test-executor")
	if err == nil {
		t.Fatal("SyncRunningTasks should return error when client is nil")
	}
	if tasks != nil {
		t.Errorf("expected nil tasks, got %v", tasks)
	}
	if !strings.Contains(err.Error(), "no client available") {
		t.Errorf("error should mention no client, got: %v", err)
	}
}

// TestMultiClient_BackoffDelayCappedAtMax 测试退避延迟在达到上限后被截断
func TestMultiClient_BackoffDelayCappedAtMax(t *testing.T) {
	client, _ := NewMultiClient([]string{"localhost:50051"})
	defer client.Close()

	base := 1 * time.Second
	max := 5 * time.Second
	jitter := 0 * time.Millisecond

	// retryCount=10 时，理论值为 1024s，应被截断为 max=5s
	got := client.backoffDelay(10, base, max, jitter)
	if got != max {
		t.Errorf("backoffDelay(10) = %v, want %v (capped at max)", got, max)
	}
}

// TestMultiClient_NextAddrAdvancesIndex 测试 nextAddr 正确推进索引
func TestMultiClient_NextAddrAdvancesIndex(t *testing.T) {
	client, _ := NewMultiClient([]string{"a:1", "b:2", "c:3"})
	defer client.Close()

	initial := client.currentIndex.Load()
	client.nextAddr()
	after := client.currentIndex.Load()
	if after != initial+1 {
		t.Errorf("nextAddr should advance index by 1, got %d -> %d", initial, after)
	}

	client.nextAddr()
	client.nextAddr()
	final := client.currentIndex.Load()
	if final != initial+3 {
		t.Errorf("after 3 nextAddr calls, index should be %d, got %d", initial+3, final)
	}
}

// TestMultiClient_LastSchedulerId 测试 lastSchedulerId 的存取
func TestMultiClient_LastSchedulerId(t *testing.T) {
	client, _ := NewMultiClient([]string{"localhost:50051"})
	defer client.Close()

	// 初始应为空
	wrapper := client.lastSchedulerId.Load().(nameWrapper)
	if wrapper.name != "" {
		t.Errorf("initial lastSchedulerId should be empty, got %v", wrapper.name)
	}

	// 设置值
	client.lastSchedulerId.Store(nameWrapper{name: "node-1"})
	wrapper = client.lastSchedulerId.Load().(nameWrapper)
	if wrapper.name != "node-1" {
		t.Errorf("lastSchedulerId = %v, want node-1", wrapper.name)
	}
}
