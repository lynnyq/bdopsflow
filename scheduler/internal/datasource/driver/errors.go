package driver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"
)

// 错误分类常量
const (
	ErrCategoryConnection    = "connection"
	ErrCategoryAuthentication = "authentication"
	ErrCategoryQuery         = "query"
	ErrCategoryTimeout       = "timeout"
	ErrCategoryPermission    = "permission"
	ErrCategoryResource      = "resource"
	ErrCategoryUnknown       = "unknown"
)

// DatasourceError 统一的数据源错误类型
type DatasourceError struct {
	// 原始错误
	Err error
	// 错误分类
	Category string
	// 数据源类型
	DatasourceType string
	// 是否可重试
	Retryable bool
	// 建议的重试延迟
	RetryAfter time.Duration
}

func (e *DatasourceError) Error() string {
	if e.DatasourceType != "" {
		return fmt.Sprintf("[%s] %s error: %v", e.DatasourceType, e.Category, e.Err)
	}
	return fmt.Sprintf("%s error: %v", e.Category, e.Err)
}

func (e *DatasourceError) Unwrap() error {
	return e.Err
}

// IsRetryable 判断错误是否可重试
func (e *DatasourceError) IsRetryable() bool {
	return e.Retryable
}

// ClassifyError 根据错误信息分类错误
func ClassifyError(err error, dsType string) *DatasourceError {
	if err == nil {
		return nil
	}

	errMsg := strings.ToLower(err.Error())
	dsErr := &DatasourceError{
		Err:            err,
		DatasourceType: dsType,
	}

	switch {
	// 连接错误
	case strings.Contains(errMsg, "broken pipe"),
		strings.Contains(errMsg, "connection reset"),
		strings.Contains(errMsg, "connection refused"),
		strings.Contains(errMsg, "network is unreachable"),
		strings.Contains(errMsg, "no such host"),
		strings.Contains(errMsg, "dial tcp"),
		strings.Contains(errMsg, "eof"):
		dsErr.Category = ErrCategoryConnection
		dsErr.Retryable = true
		dsErr.RetryAfter = 2 * time.Second

	// 超时错误
	case strings.Contains(errMsg, "timeout"),
		strings.Contains(errMsg, "i/o timeout"),
		strings.Contains(errMsg, "deadline exceeded"):
		dsErr.Category = ErrCategoryTimeout
		dsErr.Retryable = true
		dsErr.RetryAfter = 5 * time.Second

	// 认证错误
	case strings.Contains(errMsg, "authentication"),
		strings.Contains(errMsg, "access denied"),
		strings.Contains(errMsg, "unauthorized"),
		strings.Contains(errMsg, "invalid credentials"),
		strings.Contains(errMsg, "wrong password"):
		dsErr.Category = ErrCategoryAuthentication
		dsErr.Retryable = false

	// 权限错误
	case strings.Contains(errMsg, "permission denied"),
		strings.Contains(errMsg, "forbidden"),
		strings.Contains(errMsg, "not allowed"):
		dsErr.Category = ErrCategoryPermission
		dsErr.Retryable = false

	// 资源错误（如连接池耗尽）
	case strings.Contains(errMsg, "too many connections"),
		strings.Contains(errMsg, "connection pool"),
		strings.Contains(errMsg, "resource temporarily unavailable"):
		dsErr.Category = ErrCategoryResource
		dsErr.Retryable = true
		dsErr.RetryAfter = 3 * time.Second

	// 传输层错误（Thrift 等）
	case strings.Contains(errMsg, "ttransport"),
		strings.Contains(errMsg, "transport error"):
		dsErr.Category = ErrCategoryConnection
		dsErr.Retryable = true
		dsErr.RetryAfter = 2 * time.Second

	default:
		dsErr.Category = ErrCategoryQuery
		dsErr.Retryable = false
	}

	return dsErr
}

// RetryConfig 重试配置
type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Multiplier  float64
}

// DefaultRetryConfig 默认重试配置
var DefaultRetryConfig = RetryConfig{
	MaxAttempts: 3,
	BaseDelay:   1 * time.Second,
	MaxDelay:    30 * time.Second,
	Multiplier:  2.0,
}

// RetryableFunc 可重试的函数类型
type RetryableFunc func(ctx context.Context) (*QueryResult, error)

// WithRetry 带重试的执行包装器
func WithRetry(ctx context.Context, cfg RetryConfig, fn RetryableFunc, dsType string) (*QueryResult, error) {
	var lastErr error

	for attempt := 0; attempt < cfg.MaxAttempts; attempt++ {
		if attempt > 0 {
			delay := calculateDelay(attempt, cfg)
			slog.Info("retrying query",
				"datasource_type", dsType,
				"attempt", attempt+1,
				"max_attempts", cfg.MaxAttempts,
				"delay", delay,
				"last_error", lastErr,
			)

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		result, err := fn(ctx)
		if err == nil {
			if attempt > 0 {
				slog.Info("query succeeded after retry",
					"datasource_type", dsType,
					"attempt", attempt+1,
				)
			}
			return result, nil
		}

		lastErr = err

		// 分类错误，判断是否可重试
		dsErr := ClassifyError(err, dsType)
		if !dsErr.IsRetryable() {
			return nil, dsErr
		}

		// 如果错误指定了重试延迟，使用它
		if dsErr.RetryAfter > 0 {
			cfg.BaseDelay = dsErr.RetryAfter
		}
	}

	return nil, fmt.Errorf("query failed after %d attempts: %w", cfg.MaxAttempts, lastErr)
}

// calculateDelay 计算指数退避延迟
func calculateDelay(attempt int, cfg RetryConfig) time.Duration {
	delay := float64(cfg.BaseDelay) * math.Pow(cfg.Multiplier, float64(attempt-1))
	if delay > float64(cfg.MaxDelay) {
		delay = float64(cfg.MaxDelay)
	}
	return time.Duration(delay)
}

// IsConnectionError 判断是否为连接错误
func IsConnectionError(err error) bool {
	if err == nil {
		return false
	}
	var dsErr *DatasourceError
	if errors.As(err, &dsErr) {
		return dsErr.Category == ErrCategoryConnection || dsErr.Category == ErrCategoryTimeout
	}
	return isConnectionError(err)
}

// IsRetryableError 判断错误是否可重试
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}
	var dsErr *DatasourceError
	if errors.As(err, &dsErr) {
		return dsErr.IsRetryable()
	}
	return isConnectionError(err)
}
