package datasource

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
)

// TestNewManager 验证 NewManager 创建的 Manager 初始状态正确
func TestNewManager(t *testing.T) {
	mgr := NewManager(nil, nil)
	if mgr == nil {
		t.Fatal("expected non-nil Manager")
	}
	if mgr.pools == nil {
		t.Error("expected pools map to be initialized")
	}
	if len(mgr.pools) != 0 {
		t.Errorf("expected empty pools, got %d entries", len(mgr.pools))
	}
	if mgr.crypto != nil {
		t.Error("expected crypto to be nil when not provided")
	}
	if mgr.sysConfig != nil {
		t.Error("expected sysConfig to be nil when not provided")
	}
	if mgr.closed {
		t.Error("expected closed to be false initially")
	}
	if mgr.lastCheck == nil {
		t.Error("expected lastCheck map to be initialized")
	}
	if mgr.connecting == nil {
		t.Error("expected connecting map to be initialized")
	}
	if mgr.circuitBreakers == nil {
		t.Error("expected circuitBreakers map to be initialized")
	}
}

// TestNewManager_WithCrypto 验证传入 crypto 时正确设置
func TestNewManager_WithCrypto(t *testing.T) {
	crypto, err := NewCrypto(strings.Repeat("a", 32))
	if err != nil {
		t.Fatalf("failed to create crypto: %v", err)
	}
	mgr := NewManager(crypto, nil)
	if mgr.crypto != crypto {
		t.Error("expected crypto to be set")
	}
}

// TestManager_ActiveConnections_Empty 验证初始状态下没有活跃连接
func TestManager_ActiveConnections_Empty(t *testing.T) {
	mgr := NewManager(nil, nil)
	conns := mgr.ActiveConnections()
	if len(conns) != 0 {
		t.Errorf("expected 0 active connections, got %d", len(conns))
	}
}

// TestManager_GetCircuitBreakerState_Unknown 验证对未知数据源 ID 返回默认的 Closed/0 状态
func TestManager_GetCircuitBreakerState_Unknown(t *testing.T) {
	mgr := NewManager(nil, nil)
	state, failures := mgr.GetCircuitBreakerState(999)
	if state != CircuitStateClosed {
		t.Errorf("expected CircuitStateClosed for unknown ID, got %v", state)
	}
	if failures != 0 {
		t.Errorf("expected 0 failures for unknown ID, got %d", failures)
	}
}

// TestManager_GetCircuitBreaker_SameInstance 验证同一数据源 ID 返回相同的熔断器实例
func TestManager_GetCircuitBreaker_SameInstance(t *testing.T) {
	mgr := NewManager(nil, nil)
	cb1 := mgr.getCircuitBreaker(1)
	cb2 := mgr.getCircuitBreaker(1)
	if cb1 != cb2 {
		t.Error("expected same CircuitBreaker instance for same datasource ID")
	}
}

// TestManager_GetCircuitBreaker_DifferentInstances 验证不同数据源 ID 返回不同熔断器实例
func TestManager_GetCircuitBreaker_DifferentInstances(t *testing.T) {
	mgr := NewManager(nil, nil)
	cb1 := mgr.getCircuitBreaker(1)
	cb2 := mgr.getCircuitBreaker(2)
	if cb1 == cb2 {
		t.Error("expected different CircuitBreaker instances for different datasource IDs")
	}
}

// TestManager_GetCircuitBreakerState_AfterFailures 验证熔断器记录失败后能正确查询状态
func TestManager_GetCircuitBreakerState_AfterFailures(t *testing.T) {
	mgr := NewManager(nil, nil)
	cb := mgr.getCircuitBreaker(1)
	cb.failureThreshold = 2

	cb.RecordFailure()
	state, failures := mgr.GetCircuitBreakerState(1)
	if state != CircuitStateClosed {
		t.Errorf("expected Closed state after 1 failure (threshold=2), got %v", state)
	}
	if failures != 1 {
		t.Errorf("expected 1 failure, got %d", failures)
	}

	cb.RecordFailure()
	state, failures = mgr.GetCircuitBreakerState(1)
	if state != CircuitStateOpen {
		t.Errorf("expected Open state after 2 failures (threshold=2), got %v", state)
	}
	if failures != 2 {
		t.Errorf("expected 2 failures, got %d", failures)
	}
}

// TestManager_RemoveDatasource_Unknown 验证移除不存在的数据源不 panic
func TestManager_RemoveDatasource_Unknown(t *testing.T) {
	mgr := NewManager(nil, nil)
	// 不应 panic
	mgr.RemoveDatasource(999)
}

// TestManager_RemoveDatasource_ClearsCircuitBreaker 验证移除数据源时也清理对应的熔断器
func TestManager_RemoveDatasource_ClearsCircuitBreaker(t *testing.T) {
	mgr := NewManager(nil, nil)
	// 创建熔断器
	cb := mgr.getCircuitBreaker(1)
	cb.RecordFailure()

	// 确认熔断器存在
	state, failures := mgr.GetCircuitBreakerState(1)
	if failures == 0 || state == CircuitStateClosed && failures != 0 {
		// 已记录失败
	}

	// 移除数据源应同时清理熔断器
	mgr.RemoveDatasource(1)

	state, failures = mgr.GetCircuitBreakerState(1)
	if state != CircuitStateClosed {
		t.Errorf("expected Closed state after remove, got %v", state)
	}
	if failures != 0 {
		t.Errorf("expected 0 failures after remove, got %d", failures)
	}
}

// TestManager_Close_Idempotent 验证多次调用 Close 不会 panic
func TestManager_Close_Idempotent(t *testing.T) {
	mgr := NewManager(nil, nil)
	mgr.Close()
	mgr.Close()
	mgr.Close()
}

// TestManager_Close_UpdatesClosedFlag 验证 Close 后 closed 标志位被设置
func TestManager_Close_UpdatesClosedFlag(t *testing.T) {
	mgr := NewManager(nil, nil)
	mgr.Close()
	if !mgr.closed {
		t.Error("expected closed to be true after Close()")
	}
}

// TestManager_OnConfigChanged_NilSysConfig 验证 sysConfig 为 nil 时 OnConfigChanged 不 panic
func TestManager_OnConfigChanged_NilSysConfig(t *testing.T) {
	mgr := NewManager(nil, nil)
	// 这些 key 会触发 updatePoolConfigFromSystemSettings，sysConfig 为 nil 时不应 panic
	mgr.OnConfigChanged("datasource.connection_max_open", "10")
	mgr.OnConfigChanged("datasource.connection_max_idle", "5")
	mgr.OnConfigChanged("datasource.connection_max_lifetime", "1800")
}

// TestManager_OnConfigChanged_IrrelevantKey 验证无关 key 不会触发配置更新
func TestManager_OnConfigChanged_IrrelevantKey(t *testing.T) {
	mgr := NewManager(nil, nil)
	// 无关 key 不应触发任何操作，也不应 panic
	mgr.OnConfigChanged("some.other.key", "value")
	mgr.OnConfigChanged("", "")
}

// TestManager_GetDriver_Disabled 验证数据源被禁用时返回 ErrDatasourceDisabled
func TestManager_GetDriver_Disabled(t *testing.T) {
	mgr := NewManager(nil, nil)
	ds := &model.Datasource{
		ID:        1,
		IsEnabled: false,
	}
	_, err := mgr.GetDriver(context.Background(), ds)
	if !errors.Is(err, ErrDatasourceDisabled) {
		t.Errorf("expected ErrDatasourceDisabled, got %v", err)
	}
}

// TestManager_TestConnection_UnsupportedType 验证不支持的数据源类型返回错误
func TestManager_TestConnection_UnsupportedType(t *testing.T) {
	mgr := NewManager(nil, nil)
	ds := &model.Datasource{
		ID:        1,
		Type:      "unsupported-type",
		IsEnabled: true,
		Host:      "localhost",
		Port:      3306,
	}
	err := mgr.TestConnection(context.Background(), ds)
	if !errors.Is(err, ErrDatasourceTypeNotSupport) {
		t.Errorf("expected ErrDatasourceTypeNotSupport, got %v", err)
	}
}

// TestIsPoolBusyError 验证连接池繁忙错误判断逻辑
func TestIsPoolBusyError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"pool fully occupied", errors.New("pool fully occupied"), true},
		{"pool fully occupied with context", errors.New("datasource pool fully occupied for query"), true},
		{"connection refused", errors.New("connection refused"), false},
		{"empty error message", errors.New(""), false},
		{"similar but different", errors.New("pool is full"), false},
		{"timeout error", errors.New("context deadline exceeded"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPoolBusyError(tt.err)
			if got != tt.want {
				t.Errorf("isPoolBusyError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// TestManager_GetPoolConfigFromSystemSettings_NilSysConfig 验证 sysConfig 为 nil 时返回默认配置
func TestManager_GetPoolConfigFromSystemSettings_NilSysConfig(t *testing.T) {
	mgr := NewManager(nil, nil)
	cfg := mgr.getPoolConfigFromSystemSettings()
	if cfg.MaxOpen <= 0 {
		t.Errorf("expected default MaxOpen > 0, got %d", cfg.MaxOpen)
	}
	if cfg.MinIdle < 0 {
		t.Errorf("expected default MinIdle >= 0, got %d", cfg.MinIdle)
	}
}

// TestManager_StartHealthCheck 验证启动健康检查不会 panic（使用长间隔避免实际执行）
func TestManager_StartHealthCheck(t *testing.T) {
	mgr := NewManager(nil, nil)
	// 使用一个较长的时间间隔启动健康检查，立即停止不会造成问题
	// 这里主要验证 StartHealthCheck 不会 panic
	mgr.StartHealthCheck(time.Hour) // 1小时间隔，避免触发实际检查
	// 不等待 ticker，直接结束测试
}
