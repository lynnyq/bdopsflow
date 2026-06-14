package datasource

import (
	"log/slog"
	"sync"
	"time"
)

// CircuitState 熔断器状态
type CircuitState int

const (
	CircuitStateClosed   CircuitState = iota // 关闭状态（正常）
	CircuitStateOpen                         // 开启状态（熔断）
	CircuitStateHalfOpen                     // 半开状态（试探）
)

func (s CircuitState) String() string {
	switch s {
	case CircuitStateClosed:
		return "closed"
	case CircuitStateOpen:
		return "open"
	case CircuitStateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreaker 熔断器
type CircuitBreaker struct {
	mu sync.RWMutex

	// 状态
	state CircuitState

	// 失败计数
	failureCount int

	// 配置
	failureThreshold int           // 失败阈值，超过后进入open状态
	recoveryTimeout  time.Duration // 从open到half-open的等待时间
	successThreshold int           // half-open状态下成功次数阈值，达到后恢复closed

	// 时间戳
	lastFailureTime time.Time
	lastStateChange time.Time

	// 数据源ID（用于日志）
	datasourceID int64
}

// NewCircuitBreaker 创建熔断器
func NewCircuitBreaker(datasourceID int64) *CircuitBreaker {
	return &CircuitBreaker{
		state:            CircuitStateClosed,
		failureThreshold: 5,                    // 连续5次失败后熔断
		recoveryTimeout:  30 * time.Second,     // 30秒后尝试恢复
		successThreshold: 2,                    // 连续2次成功后恢复
		lastStateChange:  time.Now(),
		datasourceID:     datasourceID,
	}
}

// AllowRequest 检查是否允许请求通过
func (cb *CircuitBreaker) AllowRequest() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitStateClosed:
		return true
	case CircuitStateOpen:
		// 检查是否到了恢复时间
		if time.Since(cb.lastStateChange) > cb.recoveryTimeout {
			cb.state = CircuitStateHalfOpen
			cb.lastStateChange = time.Now()
			cb.failureCount = 0
			slog.Info("circuit breaker transitioned to half-open",
				"datasource_id", cb.datasourceID,
				"recovery_timeout", cb.recoveryTimeout)
			return true
		}
		return false
	case CircuitStateHalfOpen:
		// 半开状态允许试探性请求
		return true
	default:
		return false
	}
}

// RecordSuccess 记录成功
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitStateHalfOpen:
		cb.failureCount++
		if cb.failureCount >= cb.successThreshold {
			cb.state = CircuitStateClosed
			cb.failureCount = 0
			cb.lastStateChange = time.Now()
			slog.Info("circuit breaker recovered, transitioned to closed",
				"datasource_id", cb.datasourceID,
				"success_count", cb.failureCount)
		}
	case CircuitStateClosed:
		// 重置失败计数
		cb.failureCount = 0
	}
}

// RecordFailure 记录失败
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureCount++
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case CircuitStateClosed:
		if cb.failureCount >= cb.failureThreshold {
			cb.state = CircuitStateOpen
			cb.lastStateChange = time.Now()
			slog.Warn("circuit breaker opened due to failures",
				"datasource_id", cb.datasourceID,
				"failure_count", cb.failureCount,
				"threshold", cb.failureThreshold,
				"recovery_timeout", cb.recoveryTimeout)
		}
	case CircuitStateHalfOpen:
		// 半开状态下失败，立即回到open状态
		cb.state = CircuitStateOpen
		cb.failureCount = 0
		cb.lastStateChange = time.Now()
		slog.Warn("circuit breaker reopened due to failure in half-open state",
			"datasource_id", cb.datasourceID,
			"recovery_timeout", cb.recoveryTimeout)
	}
}

// GetState 获取当前状态
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetFailureCount 获取失败次数
func (cb *CircuitBreaker) GetFailureCount() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.failureCount
}

// Reset 重置熔断器
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = CircuitStateClosed
	cb.failureCount = 0
	cb.lastStateChange = time.Now()
	slog.Info("circuit breaker reset",
		"datasource_id", cb.datasourceID)
}
