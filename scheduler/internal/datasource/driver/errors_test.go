package driver

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		dsType       string
		wantCategory string
		wantRetry    bool
	}{
		{
			name:         "connection refused",
			err:          errors.New("connection refused"),
			dsType:       "mysql",
			wantCategory: ErrCategoryConnection,
			wantRetry:    true,
		},
		{
			name:         "timeout",
			err:          errors.New("i/o timeout"),
			dsType:       "mysql",
			wantCategory: ErrCategoryTimeout,
			wantRetry:    true,
		},
		{
			name:         "authentication failed",
			err:          errors.New("access denied for user"),
			dsType:       "mysql",
			wantCategory: ErrCategoryAuthentication,
			wantRetry:    false,
		},
		{
			name:         "permission denied",
			err:          errors.New("permission denied"),
			dsType:       "mysql",
			wantCategory: ErrCategoryPermission,
			wantRetry:    false,
		},
		{
			name:         "too many connections",
			err:          errors.New("too many connections"),
			dsType:       "mysql",
			wantCategory: ErrCategoryResource,
			wantRetry:    true,
		},
		{
			name:         "transport error",
			err:          errors.New("TTransport error"),
			dsType:       "hive",
			wantCategory: ErrCategoryConnection,
			wantRetry:    true,
		},
		{
			name:         "unknown error",
			err:          errors.New("some unknown error"),
			dsType:       "mysql",
			wantCategory: ErrCategoryQuery,
			wantRetry:    false,
		},
		{
			name:         "nil error",
			err:          nil,
			dsType:       "mysql",
			wantCategory: "",
			wantRetry:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dsErr := ClassifyError(tt.err, tt.dsType)
			if tt.err == nil {
				if dsErr != nil {
					t.Errorf("expected nil, got %v", dsErr)
				}
				return
			}
			if dsErr.Category != tt.wantCategory {
				t.Errorf("got category %q, want %q", dsErr.Category, tt.wantCategory)
			}
			if dsErr.Retryable != tt.wantRetry {
				t.Errorf("got retryable %v, want %v", dsErr.Retryable, tt.wantRetry)
			}
		})
	}
}

func TestWithRetry(t *testing.T) {
	tests := []struct {
		name        string
		maxAttempts int
		failCount   int
		wantErr     bool
	}{
		{
			name:        "success on first attempt",
			maxAttempts: 3,
			failCount:   0,
			wantErr:     false,
		},
		{
			name:        "success after retry",
			maxAttempts: 3,
			failCount:   2,
			wantErr:     false,
		},
		{
			name:        "all attempts failed",
			maxAttempts: 3,
			failCount:   5,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attempts := 0
			cfg := RetryConfig{
				MaxAttempts: tt.maxAttempts,
				BaseDelay:   1 * time.Millisecond, // 快速测试
				MaxDelay:    10 * time.Millisecond,
				Multiplier:  2.0,
			}

			fn := func(ctx context.Context) (*QueryResult, error) {
				attempts++
				if attempts <= tt.failCount {
					return nil, errors.New("connection refused")
				}
				return &QueryResult{}, nil
			}

			result, err := WithRetry(context.Background(), cfg, fn, "mysql")
			if (err != nil) != tt.wantErr {
				t.Errorf("got error %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && result == nil {
				t.Error("expected non-nil result")
			}
		})
	}
}

func TestWithRetry_NonRetryableError(t *testing.T) {
	attempts := 0
	cfg := RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   1 * time.Millisecond,
		MaxDelay:    10 * time.Millisecond,
		Multiplier:  2.0,
	}

	fn := func(ctx context.Context) (*QueryResult, error) {
		attempts++
		return nil, errors.New("access denied") // 不可重试的错误
	}

	_, err := WithRetry(context.Background(), cfg, fn, "mysql")
	if err == nil {
		t.Error("expected error")
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt for non-retryable error, got %d", attempts)
	}
}

func TestCalculateDelay(t *testing.T) {
	cfg := RetryConfig{
		BaseDelay:  1 * time.Second,
		MaxDelay:   10 * time.Second,
		Multiplier: 2.0,
	}

	tests := []struct {
		attempt     int
		wantSeconds float64
	}{
		{1, 1.0},  // 1s
		{2, 2.0},  // 2s
		{3, 4.0},  // 4s
		{4, 8.0},  // 8s
		{5, 10.0}, // 10s (capped)
	}

	for _, tt := range tests {
		delay := calculateDelay(tt.attempt, cfg)
		if delay.Seconds() != tt.wantSeconds {
			t.Errorf("attempt %d: got %.1fs, want %.1fs", tt.attempt, delay.Seconds(), tt.wantSeconds)
		}
	}
}

func TestIsConnectionError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "connection error",
			err:  errors.New("connection refused"),
			want: true,
		},
		{
			name: "timeout error",
			err:  errors.New("i/o timeout"),
			want: true,
		},
		{
			name: "non-connection error",
			err:  errors.New("syntax error"),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsConnectionError(tt.err); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDatasourceError_Error(t *testing.T) {
	err := &DatasourceError{
		Err:            errors.New("connection refused"),
		Category:       ErrCategoryConnection,
		DatasourceType: "mysql",
		Retryable:      true,
	}

	errStr := err.Error()
	if errStr == "" {
		t.Error("expected non-empty error string")
	}
}
