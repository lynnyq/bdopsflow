package datasource

import (
	"testing"
	"time"
)

func TestCircuitBreaker_InitialState(t *testing.T) {
	cb := NewCircuitBreaker(1)

	if cb.GetState() != CircuitStateClosed {
		t.Errorf("expected initial state to be Closed, got %v", cb.GetState())
	}

	if cb.GetFailureCount() != 0 {
		t.Errorf("expected initial failure count to be 0, got %d", cb.GetFailureCount())
	}
}

func TestCircuitBreaker_AllowRequest_ClosedState(t *testing.T) {
	cb := NewCircuitBreaker(1)

	if !cb.AllowRequest() {
		t.Error("expected AllowRequest to return true in Closed state")
	}
}

func TestCircuitBreaker_TransitionToOpen(t *testing.T) {
	cb := NewCircuitBreaker(1)
	cb.failureThreshold = 3 // 设置为3次失败后熔断

	// 连续失败3次
	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}

	if cb.GetState() != CircuitStateOpen {
		t.Errorf("expected state to be Open after %d failures, got %v", cb.failureThreshold, cb.GetState())
	}

	if cb.GetFailureCount() != 3 {
		t.Errorf("expected failure count to be 3, got %d", cb.GetFailureCount())
	}
}

func TestCircuitBreaker_RejectRequest_OpenState(t *testing.T) {
	cb := NewCircuitBreaker(1)
	cb.failureThreshold = 2
	cb.recoveryTimeout = 1 * time.Second

	// 触发熔断
	for i := 0; i < 2; i++ {
		cb.RecordFailure()
	}

	// 在Open状态下应该拒绝请求
	if cb.AllowRequest() {
		t.Error("expected AllowRequest to return false in Open state")
	}
}

func TestCircuitBreaker_TransitionToHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(1)
	cb.failureThreshold = 2
	cb.recoveryTimeout = 100 * time.Millisecond

	// 触发熔断
	for i := 0; i < 2; i++ {
		cb.RecordFailure()
	}

	if cb.GetState() != CircuitStateOpen {
		t.Errorf("expected state to be Open, got %v", cb.GetState())
	}

	// 等待恢复超时
	time.Sleep(150 * time.Millisecond)

	// 应该自动转换到HalfOpen状态
	if !cb.AllowRequest() {
		t.Error("expected AllowRequest to return true after recovery timeout")
	}

	if cb.GetState() != CircuitStateHalfOpen {
		t.Errorf("expected state to be HalfOpen, got %v", cb.GetState())
	}
}

func TestCircuitBreaker_RecoveryFromHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(1)
	cb.failureThreshold = 2
	cb.recoveryTimeout = 100 * time.Millisecond
	cb.successThreshold = 2

	// 触发熔断
	for i := 0; i < 2; i++ {
		cb.RecordFailure()
	}

	// 等待恢复超时
	time.Sleep(150 * time.Millisecond)

	// 触发AllowRequest以转换到HalfOpen
	cb.AllowRequest()

	if cb.GetState() != CircuitStateHalfOpen {
		t.Errorf("expected state to be HalfOpen, got %v", cb.GetState())
	}

	// 连续成功2次
	for i := 0; i < 2; i++ {
		cb.RecordSuccess()
	}

	if cb.GetState() != CircuitStateClosed {
		t.Errorf("expected state to be Closed after recovery, got %v", cb.GetState())
	}
}

func TestCircuitBreaker_ReopenFromHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(1)
	cb.failureThreshold = 2
	cb.recoveryTimeout = 100 * time.Millisecond
	cb.successThreshold = 2

	// 触发熔断
	for i := 0; i < 2; i++ {
		cb.RecordFailure()
	}

	// 等待恢复超时
	time.Sleep(150 * time.Millisecond)

	// 触发AllowRequest以转换到HalfOpen
	cb.AllowRequest()

	if cb.GetState() != CircuitStateHalfOpen {
		t.Errorf("expected state to be HalfOpen, got %v", cb.GetState())
	}

	// 在HalfOpen状态下失败，应该重新打开
	cb.RecordFailure()

	if cb.GetState() != CircuitStateOpen {
		t.Errorf("expected state to be Open after failure in HalfOpen, got %v", cb.GetState())
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	cb := NewCircuitBreaker(1)
	cb.failureThreshold = 2

	// 触发熔断
	for i := 0; i < 2; i++ {
		cb.RecordFailure()
	}

	if cb.GetState() != CircuitStateOpen {
		t.Errorf("expected state to be Open, got %v", cb.GetState())
	}

	// 重置熔断器
	cb.Reset()

	if cb.GetState() != CircuitStateClosed {
		t.Errorf("expected state to be Closed after reset, got %v", cb.GetState())
	}

	if cb.GetFailureCount() != 0 {
		t.Errorf("expected failure count to be 0 after reset, got %d", cb.GetFailureCount())
	}
}

func TestCircuitBreaker_SuccessResetsFailureCount(t *testing.T) {
	cb := NewCircuitBreaker(1)
	cb.failureThreshold = 3

	// 失败2次
	cb.RecordFailure()
	cb.RecordFailure()

	if cb.GetFailureCount() != 2 {
		t.Errorf("expected failure count to be 2, got %d", cb.GetFailureCount())
	}

	// 成功一次应该重置失败计数
	cb.RecordSuccess()

	if cb.GetFailureCount() != 0 {
		t.Errorf("expected failure count to be 0 after success, got %d", cb.GetFailureCount())
	}
}

func TestCircuitBreaker_StateString(t *testing.T) {
	tests := []struct {
		state    CircuitState
		expected string
	}{
		{CircuitStateClosed, "closed"},
		{CircuitStateOpen, "open"},
		{CircuitStateHalfOpen, "half-open"},
	}

	for _, tt := range tests {
		if tt.state.String() != tt.expected {
			t.Errorf("expected state string to be %s, got %s", tt.expected, tt.state.String())
		}
	}
}
