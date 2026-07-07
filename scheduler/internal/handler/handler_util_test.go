package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	rqlite "github.com/rqlite/gorqlite"
)

// TestExtractUserID 测试从 gin.Context 中提取 user_id
func TestExtractUserID(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(c *gin.Context)
		expected int64
	}{
		{
			name:     "user_id not set",
			setup:    func(c *gin.Context) {},
			expected: 0,
		},
		{
			name: "user_id is int64",
			setup: func(c *gin.Context) {
				c.Set("user_id", int64(123))
			},
			expected: 123,
		},
		{
			name: "user_id is zero",
			setup: func(c *gin.Context) {
				c.Set("user_id", int64(0))
			},
			expected: 0,
		},
		{
			name: "user_id is negative int64",
			setup: func(c *gin.Context) {
				c.Set("user_id", int64(-1))
			},
			expected: -1,
		},
		{
			name: "user_id is wrong type string",
			setup: func(c *gin.Context) {
				c.Set("user_id", "abc")
			},
			expected: 0,
		},
		{
			name: "user_id is wrong type int",
			setup: func(c *gin.Context) {
				c.Set("user_id", 123)
			},
			expected: 0,
		},
		{
			name: "user_id is wrong type nil",
			setup: func(c *gin.Context) {
				c.Set("user_id", nil)
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
			tt.setup(c)

			got := extractUserID(c)
			if got != tt.expected {
				t.Errorf("extractUserID() = %d, want %d", got, tt.expected)
			}
		})
	}
}

// TestParseIDParam 测试 URL 路径参数 ID 解析
func TestParseIDParam(t *testing.T) {
	tests := []struct {
		name        string
		paramName   string
		paramValue  string
		wantID      int64
		wantOK      bool
		wantCode    int
	}{
		{
			name:       "valid positive id",
			paramName:  "id",
			paramValue: "123",
			wantID:     123,
			wantOK:     true,
		},
		{
			name:       "valid zero id",
			paramName:  "id",
			paramValue: "0",
			wantID:     0,
			wantOK:     true,
		},
		{
			name:       "invalid non-numeric",
			paramName:  "id",
			paramValue: "abc",
			wantID:     0,
			wantOK:     false,
		},
		{
			name:       "empty value",
			paramName:  "id",
			paramValue: "",
			wantID:     0,
			wantOK:     false,
		},
		{
			name:       "valid large id",
			paramName:  "id",
			paramValue: "9223372036854775807",
			wantID:     9223372036854775807,
			wantOK:     true,
		},
		{
			name:       "custom param name",
			paramName:  "test_id",
			paramValue: "456",
			wantID:     456,
			wantOK:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
			c.Params = gin.Params{{Key: tt.paramName, Value: tt.paramValue}}

			gotID, gotOK := parseIDParam(c, tt.paramName)
			if gotID != tt.wantID {
				t.Errorf("parseIDParam() id = %d, want %d", gotID, tt.wantID)
			}
			if gotOK != tt.wantOK {
				t.Errorf("parseIDParam() ok = %v, want %v", gotOK, tt.wantOK)
			}

			// 失败时应该返回 BadRequest
			if !tt.wantOK {
				var resp Response
				if err := decodeJSON(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if resp.Code != CodeBadRequest {
					t.Errorf("expected code %d, got %d", CodeBadRequest, resp.Code)
				}
			}
		})
	}
}

// TestInt64Ptr 测试 int64 指针转换
func TestInt64Ptr(t *testing.T) {
	tests := []struct {
		name  string
		value int64
	}{
		{"zero", 0},
		{"positive", 123},
		{"negative", -456},
		{"max value", 9223372036854775807},
		{"min value", -9223372036854775808},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ptr := int64Ptr(tt.value)
			if ptr == nil {
				t.Fatal("expected non-nil pointer")
			}
			if *ptr != tt.value {
				t.Errorf("int64Ptr(%d) = %d, want %d", tt.value, *ptr, tt.value)
			}
		})
	}
}

// TestFormatTimePtr 测试 time.Time 指针格式化
func TestFormatTimePtr(t *testing.T) {
	tests := []struct {
		name string
		t    *time.Time
		want string
	}{
		{
			name: "nil time",
			t:    nil,
			want: "",
		},
		{
			name: "zero time",
			t:    &time.Time{},
			want: "0001-01-01T00:00:00Z",
		},
		{
			name: "valid time",
			t:    func() *time.Time { t := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC); return &t }(),
			want: "2024-01-01T12:00:00Z",
		},
		{
			name: "time with nanoseconds",
			t:    func() *time.Time { t := time.Date(2024, 6, 15, 10, 30, 45, 123456789, time.UTC); return &t }(),
			want: "2024-06-15T10:30:45.123456789Z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatTimePtr(tt.t)
			if got != tt.want {
				t.Errorf("formatTimePtr() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestSafeString_AdditionalCases 补充测试 safeString 的额外边界情况
func TestSafeString_AdditionalCases(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want string
	}{
		{"tab whitespace", "\thello\t", "hello"},
		{"newline whitespace", "\nhello\n", "hello"},
		{"mixed whitespace", " \t\n hello \t\n ", "hello"},
		{"only whitespace", "   \t\n  ", ""},
		{"leading whitespace", "  hello", "hello"},
		{"trailing whitespace", "hello  ", "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := safeString(tt.s)
			if got != tt.want {
				t.Errorf("safeString(%q) = %q, want %q", tt.s, got, tt.want)
			}
		})
	}
}

// TestSafeTimePtr 测试安全时间指针转换
func TestSafeTimePtr(t *testing.T) {
	t.Run("zero time returns nil", func(t *testing.T) {
		result := safeTimePtr(time.Time{})
		if result != nil {
			t.Errorf("expected nil for zero time, got %v", result)
		}
	})

	t.Run("valid time returns pointer", func(t *testing.T) {
		now := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
		result := safeTimePtr(now)
		if result == nil {
			t.Fatal("expected non-nil for valid time")
		}
		expected := now.Format(TimeResponseFormat)
		if *result != expected {
			t.Errorf("safeTimePtr() = %q, want %q", *result, expected)
		}
	})

	t.Run("non-zero time returns correct format", func(t *testing.T) {
		tt := time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC)
		result := safeTimePtr(tt)
		if result == nil {
			t.Fatal("expected non-nil for valid time")
		}
		expected := "2023-12-31T23:59:59Z"
		if *result != expected {
			t.Errorf("safeTimePtr() = %q, want %q", *result, expected)
		}
	})
}

// TestIsPoolBusyError 测试连接池繁忙错误判断
func TestIsPoolBusyError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"pool busy error", errors.New("pool fully occupied"), true},
		{"pool busy with details", errors.New("datasource pool fully occupied for query"), true},
		{"other error", errors.New("connection refused"), false},
		{"empty error", errors.New(""), false},
		{"similar but not exact", errors.New("pool is full"), false},
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

// TestIsExecutorOnline_AdditionalCases 补充测试 isExecutorOnline 的额外状态
func TestIsExecutorOnline_AdditionalCases(t *testing.T) {
	now := time.Now()
	recentTime := now
	staleTime := now.Add(-2 * time.Minute) // 超过30秒超时

	tests := []struct {
		name string
		exec *model.Executor
		want bool
	}{
		{
			name: "online with recent heartbeat",
			exec: &model.Executor{
				Status:        "online",
				LastHeartbeat: rqlite.NullTime{Valid: true, Time: recentTime},
			},
			want: true,
		},
		{
			name: "online with stale heartbeat",
			exec: &model.Executor{
				Status:        "online",
				LastHeartbeat: rqlite.NullTime{Valid: true, Time: staleTime},
			},
			want: false,
		},
		{
			name: "empty status",
			exec: &model.Executor{
				Status:        "",
				LastHeartbeat: rqlite.NullTime{Valid: true, Time: recentTime},
			},
			want: false,
		},
		{
			name: "other status",
			exec: &model.Executor{
				Status:        "maintenance",
				LastHeartbeat: rqlite.NullTime{Valid: true, Time: recentTime},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isExecutorOnline(tt.exec)
			if got != tt.want {
				t.Errorf("isExecutorOnline() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestNewAuthHandler 测试 AuthHandler 构造函数
func TestNewAuthHandler(t *testing.T) {
	t.Run("with positive sso timeout", func(t *testing.T) {
		h := NewAuthHandler(nil, nil, nil, true, "http://sso.example.com", nil, 30)
		if h == nil {
			t.Fatal("expected non-nil handler")
		}
		if !h.ssoEnabled {
			t.Error("expected ssoEnabled to be true")
		}
		if h.ssoUrl != "http://sso.example.com" {
			t.Errorf("expected ssoUrl to be 'http://sso.example.com', got %q", h.ssoUrl)
		}
		if h.ssoTimeout != 30*time.Second {
			t.Errorf("expected ssoTimeout to be 30s, got %v", h.ssoTimeout)
		}
	})

	t.Run("with zero sso timeout uses default", func(t *testing.T) {
		h := NewAuthHandler(nil, nil, nil, false, "", nil, 0)
		if h == nil {
			t.Fatal("expected non-nil handler")
		}
		if h.ssoEnabled {
			t.Error("expected ssoEnabled to be false")
		}
		if h.ssoTimeout != 10*time.Second {
			t.Errorf("expected default ssoTimeout 10s, got %v", h.ssoTimeout)
		}
	})

	t.Run("with negative sso timeout uses default", func(t *testing.T) {
		h := NewAuthHandler(nil, nil, nil, false, "", nil, -5)
		if h.ssoTimeout != 10*time.Second {
			t.Errorf("expected default ssoTimeout 10s for negative input, got %v", h.ssoTimeout)
		}
	})
}

// TestNewApiTestHandler 测试 ApiTestHandler 构造函数
func TestNewApiTestHandler(t *testing.T) {
	h := NewApiTestHandler(nil, nil, nil, nil, nil, nil)
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
}

// decodeJSON 是辅助函数，用于解码响应体
func decodeJSON(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
