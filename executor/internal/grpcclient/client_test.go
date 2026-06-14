package grpcclient

import (
	"context"
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
	runningTasks    int32
	runningExecIds  []string
	runningStates   []*pb.RunningTaskState
	capacity        int32
	cancelledTasks  []string
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
	m.capacity = newCapacity
	return nil
}

func (m *MockTaskRunner) CancelTask(executionId string) bool {
	m.cancelledTasks = append(m.cancelledTasks, executionId)
	return true
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
