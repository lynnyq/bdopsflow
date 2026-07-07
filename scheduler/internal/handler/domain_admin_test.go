package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// === NewDomainAdminHandler 测试 ===

func TestNewDomainAdminHandler(t *testing.T) {
	h := NewDomainAdminHandler(nil)
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
	if h.svc != nil {
		t.Errorf("expected nil svc, got %v", h.svc)
	}
}

// === DomainAdminHandler.GetDomain ID 参数校验测试 ===

func TestDomainAdminHandler_GetDomain_InvalidID(t *testing.T) {
	tests := []struct {
		name  string
		idVal string
	}{
		{"non-numeric", "abc"},
		{"empty", ""},
		{"float", "1.5"},
		{"zero", "0"},
		{"negative", "-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			h := &DomainAdminHandler{}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
			c.Params = gin.Params{{Key: "id", Value: tt.idVal}}

			h.GetDomain(c)

			var resp Response
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to parse response: %v", err)
			}
			if resp.Code != CodeBadRequest {
				t.Errorf("expected code %d, got %d", CodeBadRequest, resp.Code)
			}
		})
	}
}

// === DomainAdminHandler.UpdateDomain 测试 ===

func TestDomainAdminHandler_UpdateDomain_InvalidID(t *testing.T) {
	tests := []struct {
		name  string
		idVal string
	}{
		{"non-numeric", "abc"},
		{"empty", ""},
		{"zero", "0"},
		{"negative", "-1"},
		{"float", "1.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			h := &DomainAdminHandler{}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPut, "/", nil)
			c.Params = gin.Params{{Key: "id", Value: tt.idVal}}

			h.UpdateDomain(c)

			var resp Response
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to parse response: %v", err)
			}
			if resp.Code != CodeBadRequest {
				t.Errorf("expected code %d, got %d", CodeBadRequest, resp.Code)
			}
		})
	}
}

func TestDomainAdminHandler_UpdateDomain_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &DomainAdminHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/", bytes.NewReader([]byte("not json")))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "1"}}

	h.UpdateDomain(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for invalid JSON, got %d", CodeBadRequest, resp.Code)
	}
}

func TestDomainAdminHandler_UpdateDomain_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name string
		body interface{}
	}{
		{
			name: "empty body",
			body: map[string]interface{}{},
		},
		{
			name: "missing name",
			body: map[string]interface{}{
				"description": "test",
			},
		},
		{
			name: "name too short",
			body: map[string]interface{}{
				"name": "a",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			h := &DomainAdminHandler{}

			bodyBytes, _ := json.Marshal(tt.body)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(bodyBytes))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Params = gin.Params{{Key: "id", Value: "1"}}

			h.UpdateDomain(c)

			var resp Response
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to parse response: %v", err)
			}
			if resp.Code != CodeBadRequest {
				t.Errorf("expected code %d, got %d", CodeBadRequest, resp.Code)
			}
		})
	}
}

// === DomainAdminHandler.DeleteDomain ID 参数校验测试 ===

func TestDomainAdminHandler_DeleteDomain_InvalidID(t *testing.T) {
	tests := []struct {
		name  string
		idVal string
	}{
		{"non-numeric", "abc"},
		{"empty", ""},
		{"zero", "0"},
		{"negative", "-1"},
		{"float", "1.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			h := &DomainAdminHandler{}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodDelete, "/", nil)
			c.Params = gin.Params{{Key: "id", Value: tt.idVal}}

			h.DeleteDomain(c)

			var resp Response
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to parse response: %v", err)
			}
			if resp.Code != CodeBadRequest {
				t.Errorf("expected code %d, got %d", CodeBadRequest, resp.Code)
			}
		})
	}
}

// === DomainAdminHandler.CreateDomain 测试 ===

func TestDomainAdminHandler_CreateDomain_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &DomainAdminHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("not json")))
	c.Request.Header.Set("Content-Type", "application/json")

	h.CreateDomain(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for invalid JSON, got %d", CodeBadRequest, resp.Code)
	}
}

func TestDomainAdminHandler_CreateDomain_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name string
		body interface{}
	}{
		{
			name: "empty body",
			body: map[string]interface{}{},
		},
		{
			name: "missing name",
			body: map[string]interface{}{
				"description": "test",
			},
		},
		{
			name: "name too short",
			body: map[string]interface{}{
				"name": "a",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			h := &DomainAdminHandler{}

			bodyBytes, _ := json.Marshal(tt.body)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(bodyBytes))
			c.Request.Header.Set("Content-Type", "application/json")

			h.CreateDomain(c)

			var resp Response
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("failed to parse response: %v", err)
			}
			if resp.Code != CodeBadRequest {
				t.Errorf("expected code %d, got %d", CodeBadRequest, resp.Code)
			}
		})
	}
}

// === DomainAdminHandler.ListDomains panic 恢复测试 ===

func TestDomainAdminHandler_ListDomains_NilService_PanicRecovery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &DomainAdminHandler{} // svc 为 nil

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Set("user_id", int64(1)) // 设置 user_id 触发 svc 调用

	h.ListDomains(c)

	// handler 内部有 defer recover，应返回 500 而非 panic
	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeInternalError {
		t.Errorf("expected code %d for nil service panic recovery, got %d", CodeInternalError, resp.Code)
	}
}
