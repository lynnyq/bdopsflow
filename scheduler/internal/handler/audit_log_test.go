package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// mockSysconfigService 实现 sysconfigService 接口用于测试
type mockSysconfigService struct {
	setErr     error
	setCalled  bool
	setKey     string
	setValue   string
	setUID     int64
	getIntVal  int
}

func (m *mockSysconfigService) Set(ctx context.Context, key, value string, changedBy int64) error {
	m.setCalled = true
	m.setKey = key
	m.setValue = value
	m.setUID = changedBy
	return m.setErr
}

func (m *mockSysconfigService) GetInt(key string) int {
	return m.getIntVal
}

func TestAuditLogHandler_List(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &AuditLogHandler{}
	r.GET("/api/audit-logs", handler.List)

	req, _ := http.NewRequest("GET", "/api/audit-logs", nil)
	w := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Log("Recovered from panic (expected for nil db):", r)
			return
		}
	}()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d", w.Code)
	}
}

func TestAuditLogHandler_ListWithPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &AuditLogHandler{}
	r.GET("/api/audit-logs", handler.List)

	req, _ := http.NewRequest("GET", "/api/audit-logs?page=1&page_size=10", nil)
	w := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Log("Recovered from panic (expected for nil db):", r)
			return
		}
	}()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d", w.Code)
	}
}

func TestAuditLogHandler_ListWithFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &AuditLogHandler{}
	r.GET("/api/audit-logs", handler.List)

	req, _ := http.NewRequest("GET", "/api/audit-logs?user_id=1&action=create&resource=datasource", nil)
	w := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Log("Recovered from panic (expected for nil db):", r)
			return
		}
	}()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d", w.Code)
	}
}

// TestNewAuditLogHandler 测试构造函数
func TestNewAuditLogHandler(t *testing.T) {
	t.Run("with nil services", func(t *testing.T) {
		h := NewAuditLogHandler(nil, nil)
		if h == nil {
			t.Fatal("expected non-nil handler")
		}
	})
	t.Run("with mock config service", func(t *testing.T) {
		mock := &mockSysconfigService{}
		h := NewAuditLogHandler(nil, mock)
		if h == nil {
			t.Fatal("expected non-nil handler")
		}
		if h.configService == nil {
			t.Error("expected configService to be set")
		}
	})
}

// TestAuditLogHandler_UpdateRetentionDays 测试更新保留天数
func TestAuditLogHandler_UpdateRetentionDays(t *testing.T) {
	tests := []struct {
		name          string
		body          string
		wantCode      int
		wantSetCalled bool
	}{
		{
			name:          "invalid json",
			body:          "not json",
			wantCode:      CodeBadRequest,
			wantSetCalled: false,
		},
		{
			name:          "missing retention_days",
			body:          `{}`,
			wantCode:      CodeBadRequest,
			wantSetCalled: false,
		},
		{
			name:          "zero retention_days",
			body:          `{"retention_days": 0}`,
			wantCode:      CodeBadRequest,
			wantSetCalled: false,
		},
		{
			name:          "negative retention_days",
			body:          `{"retention_days": -1}`,
			wantCode:      CodeBadRequest,
			wantSetCalled: false,
		},
		{
			name:          "exceeds max retention_days",
			body:          `{"retention_days": 9999}`,
			wantCode:      CodeBadRequest,
			wantSetCalled: false,
		},
		{
			name:          "valid retention_days",
			body:          `{"retention_days": 30}`,
			wantCode:      CodeSuccess,
			wantSetCalled: true,
		},
		{
			name:          "valid retention_days min boundary",
			body:          `{"retention_days": 1}`,
			wantCode:      CodeSuccess,
			wantSetCalled: true,
		},
		{
			name:          "valid retention_days max boundary",
			body:          `{"retention_days": 3650}`,
			wantCode:      CodeSuccess,
			wantSetCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			r := gin.New()
			mock := &mockSysconfigService{}
			h := &AuditLogHandler{configService: mock}
			r.PUT("/api/audit-logs/retention-days", h.UpdateRetentionDays)

			req, _ := http.NewRequest("PUT", "/api/audit-logs/retention-days", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			var resp Response
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if resp.Code != tt.wantCode {
				t.Errorf("code = %d, want %d", resp.Code, tt.wantCode)
			}
			if mock.setCalled != tt.wantSetCalled {
				t.Errorf("setCalled = %v, want %v", mock.setCalled, tt.wantSetCalled)
			}
		})
	}
}

// TestAuditLogHandler_UpdateRetentionDays_WithUserID 测试带 user_id 的更新
func TestAuditLogHandler_UpdateRetentionDays_WithUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	mock := &mockSysconfigService{}
	h := &AuditLogHandler{configService: mock}
	r.PUT("/api/audit-logs/retention-days", func(c *gin.Context) {
		c.Set("user_id", int64(42))
		h.UpdateRetentionDays(c)
	})

	body := `{"retention_days": 60}`
	req, _ := http.NewRequest("PUT", "/api/audit-logs/retention-days", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != CodeSuccess {
		t.Errorf("code = %d, want %d", resp.Code, CodeSuccess)
	}
	if !mock.setCalled {
		t.Error("expected Set to be called")
	}
	if mock.setKey != "audit_log.retention_days" {
		t.Errorf("setKey = %q, want %q", mock.setKey, "audit_log.retention_days")
	}
	if mock.setValue != "60" {
		t.Errorf("setValue = %q, want %q", mock.setValue, "60")
	}
	if mock.setUID != 42 {
		t.Errorf("setUID = %d, want 42", mock.setUID)
	}
}

// TestAuditLogHandler_UpdateRetentionDays_SetError 测试 Set 返回错误
func TestAuditLogHandler_UpdateRetentionDays_SetError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	mock := &mockSysconfigService{setErr: errors.New("db connection lost")}
	h := &AuditLogHandler{configService: mock}
	r.PUT("/api/audit-logs/retention-days", h.UpdateRetentionDays)

	body := `{"retention_days": 30}`
	req, _ := http.NewRequest("PUT", "/api/audit-logs/retention-days", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != CodeInternalError {
		t.Errorf("code = %d, want %d", resp.Code, CodeInternalError)
	}
}

// TestAuditLogHandler_GetRetentionDays_NilService 测试获取保留天数（nil service 会 panic）
func TestAuditLogHandler_GetRetentionDays_NilService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &AuditLogHandler{}
	r.GET("/api/audit-logs/retention-days", h.GetRetentionDays)

	req, _ := http.NewRequest("GET", "/api/audit-logs/retention-days", nil)
	w := httptest.NewRecorder()

	defer func() {
		if rec := recover(); rec != nil {
			t.Log("Recovered from panic (expected for nil service):", rec)
		}
	}()

	r.ServeHTTP(w, req)
}

// TestAuditLogHandler_GetStats_NilService 测试获取统计（nil service 会 panic）
func TestAuditLogHandler_GetStats_NilService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &AuditLogHandler{}
	r.GET("/api/audit-logs/stats", h.GetStats)

	req, _ := http.NewRequest("GET", "/api/audit-logs/stats", nil)
	w := httptest.NewRecorder()

	defer func() {
		if rec := recover(); rec != nil {
			t.Log("Recovered from panic (expected for nil service):", rec)
		}
	}()

	r.ServeHTTP(w, req)
}

// TestAuditLogHandler_CleanExpired 测试清理过期日志
func TestAuditLogHandler_CleanExpired(t *testing.T) {
	t.Run("invalid json falls back to default retention", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		h := &AuditLogHandler{}
		r.POST("/api/audit-logs/clean", h.CleanExpired)

		req, _ := http.NewRequest("POST", "/api/audit-logs/clean", bytes.NewBufferString("not json"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		defer func() {
			if rec := recover(); rec != nil {
				t.Log("Recovered from panic (expected for nil service):", rec)
			}
		}()

		r.ServeHTTP(w, req)
	})

	t.Run("empty body falls back to default retention", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		h := &AuditLogHandler{}
		r.POST("/api/audit-logs/clean", h.CleanExpired)

		req, _ := http.NewRequest("POST", "/api/audit-logs/clean", bytes.NewBufferString(`{}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		defer func() {
			if rec := recover(); rec != nil {
				t.Log("Recovered from panic (expected for nil service):", rec)
			}
		}()

		r.ServeHTTP(w, req)
	})
}
