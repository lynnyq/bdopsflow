package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// === NewCertificateHandler 测试 ===

func TestNewCertificateHandler(t *testing.T) {
	h := NewCertificateHandler(nil, nil)
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
	if h.certSvc != nil {
		t.Errorf("expected nil certSvc, got %v", h.certSvc)
	}
	if h.permSvc != nil {
		t.Errorf("expected nil permSvc, got %v", h.permSvc)
	}
}

// === CertificateHandler.List 未授权测试 ===

func TestCertificateHandler_List_NoAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &CertificateHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	h.List(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeUnauthorized {
		t.Errorf("expected code %d for no auth, got %d", CodeUnauthorized, resp.Code)
	}
}

// === CertificateHandler.Create 测试 ===

func TestCertificateHandler_Create_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &CertificateHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("not json")))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Create(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for invalid JSON, got %d", CodeBadRequest, resp.Code)
	}
}

func TestCertificateHandler_Create_MissingRequiredFields(t *testing.T) {
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
				"ca_cert": "test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			h := &CertificateHandler{}

			bodyBytes, _ := json.Marshal(tt.body)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(bodyBytes))
			c.Request.Header.Set("Content-Type", "application/json")

			h.Create(c)

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

func TestCertificateHandler_Create_NoAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &CertificateHandler{}

	body := map[string]interface{}{
		"name": "test-cert",
	}
	bodyBytes, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(bodyBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Create(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeUnauthorized {
		t.Errorf("expected code %d for no auth, got %d", CodeUnauthorized, resp.Code)
	}
}

// === CertificateHandler.Get ID 参数校验测试 ===

func TestCertificateHandler_Get_InvalidID(t *testing.T) {
	// 注意：certificate.go 的 Get/Update/Delete 只检查 parse 错误，
	// 不检查 id <= 0，所以 "zero" 和 "negative" 会通过 parse 进入 svc 调用。
	tests := []struct {
		name  string
		idVal string
	}{
		{"non-numeric", "abc"},
		{"empty", ""},
		{"float", "1.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			h := &CertificateHandler{}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
			c.Params = gin.Params{{Key: "id", Value: tt.idVal}}

			h.Get(c)

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

// === CertificateHandler.Update 测试 ===

func TestCertificateHandler_Update_InvalidID(t *testing.T) {
	// 注意：certificate.go 的 Update 只检查 parse 错误，不检查 id <= 0
	tests := []struct {
		name  string
		idVal string
	}{
		{"non-numeric", "abc"},
		{"empty", ""},
		{"float", "1.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			h := &CertificateHandler{}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPut, "/", nil)
			c.Params = gin.Params{{Key: "id", Value: tt.idVal}}

			h.Update(c)

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

func TestCertificateHandler_Update_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &CertificateHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/", bytes.NewReader([]byte("not json")))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = gin.Params{{Key: "id", Value: "1"}}

	h.Update(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for invalid JSON, got %d", CodeBadRequest, resp.Code)
	}
}

// === CertificateHandler.Delete ID 参数校验测试 ===

func TestCertificateHandler_Delete_InvalidID(t *testing.T) {
	// 注意：certificate.go 的 Delete 只检查 parse 错误，不检查 id <= 0
	tests := []struct {
		name  string
		idVal string
	}{
		{"non-numeric", "abc"},
		{"empty", ""},
		{"float", "1.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			h := &CertificateHandler{}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodDelete, "/", nil)
			c.Params = gin.Params{{Key: "id", Value: tt.idVal}}

			h.Delete(c)

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
