package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestWebhookHandler_Test_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	h := &WebhookHandler{}
	r.POST("/api/webhooks/:id/test", h.Test)

	req, _ := http.NewRequest("POST", "/api/webhooks/invalid/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200, got %d", w.Code)
	}

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	if code, ok := body["code"].(float64); !ok || int(code) != 400 {
		t.Errorf("expected body.code 400 for invalid ID, got %v", body["code"])
	}
}

func TestWebhookHandler_Delete_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	h := &WebhookHandler{}
	r.DELETE("/api/webhooks/:id", h.Delete)

	req, _ := http.NewRequest("DELETE", "/api/webhooks/invalid", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200, got %d", w.Code)
	}

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	if code, ok := body["code"].(float64); !ok || int(code) != 400 {
		t.Errorf("expected body.code 400 for invalid ID, got %v", body["code"])
	}
}

func TestWebhookHandler_Create_MissingRequired(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	h := &WebhookHandler{}
	r.POST("/api/webhooks", h.Create)

	body := map[string]interface{}{
		"description": "test",
	}

	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/api/webhooks", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if code, ok := resp["code"].(float64); !ok || int(code) != 400 {
		t.Errorf("expected body.code 400 for missing required fields, got %v", resp["code"])
	}
}

func TestWebhookHandler_List_MissingDomainID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	h := &WebhookHandler{}
	r.GET("/api/webhooks", h.List)

	req, _ := http.NewRequest("GET", "/api/webhooks", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if code, ok := resp["code"].(float64); !ok || int(code) != 0 {
		t.Errorf("expected body.code 0 for optional domain_id, got %v", resp["code"])
	}
}

// TestNewWebhookHandler 测试构造函数
func TestNewWebhookHandler(t *testing.T) {
	h := NewWebhookHandler(nil)
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
}

// TestWebhookHandler_Create 测试创建 webhook 的各种场景
func TestWebhookHandler_Create(t *testing.T) {
	t.Run("invalid json", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		h := &WebhookHandler{}
		r.POST("/api/webhooks", h.Create)

		req, _ := http.NewRequest("POST", "/api/webhooks", bytes.NewBufferString("not json"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Code != CodeBadRequest {
			t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
		}
	})

	t.Run("missing required name", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		h := &WebhookHandler{}
		r.POST("/api/webhooks", h.Create)

		body := `{"url":"http://example.com","domain_id":1}`
		req, _ := http.NewRequest("POST", "/api/webhooks", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Code != CodeBadRequest {
			t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
		}
	})

	t.Run("missing required url", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		h := &WebhookHandler{}
		r.POST("/api/webhooks", h.Create)

		body := `{"name":"test","domain_id":1}`
		req, _ := http.NewRequest("POST", "/api/webhooks", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Code != CodeBadRequest {
			t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
		}
	})

	t.Run("missing required domain_id", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		h := &WebhookHandler{}
		r.POST("/api/webhooks", h.Create)

		body := `{"name":"test","url":"http://example.com"}`
		req, _ := http.NewRequest("POST", "/api/webhooks", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Code != CodeBadRequest {
			t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
		}
	})

	t.Run("valid request but nil service", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		h := &WebhookHandler{}
		r.POST("/api/webhooks", h.Create)

		body := `{"name":"test","url":"http://example.com","domain_id":1}`
		req, _ := http.NewRequest("POST", "/api/webhooks", bytes.NewBufferString(body))
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

// TestWebhookHandler_List 测试列出 webhook
func TestWebhookHandler_List(t *testing.T) {
	t.Run("invalid domain_id", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		h := &WebhookHandler{}
		r.GET("/api/webhooks", h.List)

		req, _ := http.NewRequest("GET", "/api/webhooks?domain_id=abc", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Code != CodeBadRequest {
			t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
		}
	})

	t.Run("nil service returns empty array", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		h := &WebhookHandler{} // webhookSvc is nil
		r.GET("/api/webhooks", h.List)

		req, _ := http.NewRequest("GET", "/api/webhooks?domain_id=1", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Code != CodeSuccess {
			t.Errorf("code = %d, want %d", resp.Code, CodeSuccess)
		}
	})

	t.Run("no domain_id param with nil service", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		h := &WebhookHandler{}
		r.GET("/api/webhooks", h.List)

		req, _ := http.NewRequest("GET", "/api/webhooks", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Code != CodeSuccess {
			t.Errorf("code = %d, want %d", resp.Code, CodeSuccess)
		}
	})
}

// TestWebhookHandler_Update 测试更新 webhook
func TestWebhookHandler_Update(t *testing.T) {
	t.Run("invalid id", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		h := &WebhookHandler{}
		r.PUT("/api/webhooks/:id", h.Update)

		body := `{"name":"test","url":"http://example.com","domain_id":1}`
		req, _ := http.NewRequest("PUT", "/api/webhooks/abc", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Code != CodeBadRequest {
			t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		h := &WebhookHandler{}
		r.PUT("/api/webhooks/:id", h.Update)

		req, _ := http.NewRequest("PUT", "/api/webhooks/1", bytes.NewBufferString("not json"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Code != CodeBadRequest {
			t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
		}
	})

	t.Run("missing required name", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		h := &WebhookHandler{}
		r.PUT("/api/webhooks/:id", h.Update)

		body := `{"url":"http://example.com","domain_id":1}`
		req, _ := http.NewRequest("PUT", "/api/webhooks/1", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Code != CodeBadRequest {
			t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
		}
	})

	t.Run("missing required url", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		h := &WebhookHandler{}
		r.PUT("/api/webhooks/:id", h.Update)

		body := `{"name":"test","domain_id":1}`
		req, _ := http.NewRequest("PUT", "/api/webhooks/1", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Code != CodeBadRequest {
			t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
		}
	})

	t.Run("missing required domain_id", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		h := &WebhookHandler{}
		r.PUT("/api/webhooks/:id", h.Update)

		body := `{"name":"test","url":"http://example.com"}`
		req, _ := http.NewRequest("PUT", "/api/webhooks/1", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Code != CodeBadRequest {
			t.Errorf("code = %d, want %d", resp.Code, CodeBadRequest)
		}
	})

	t.Run("valid request but nil service", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		h := &WebhookHandler{}
		r.PUT("/api/webhooks/:id", h.Update)

		body := `{"name":"test","url":"http://example.com","domain_id":1}`
		req, _ := http.NewRequest("PUT", "/api/webhooks/1", bytes.NewBufferString(body))
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

// TestWebhookHandler_Delete_NilService 测试删除 webhook（nil service）
func TestWebhookHandler_Delete_NilService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &WebhookHandler{}
	r.DELETE("/api/webhooks/:id", h.Delete)

	req, _ := http.NewRequest("DELETE", "/api/webhooks/1", nil)
	w := httptest.NewRecorder()

	defer func() {
		if rec := recover(); rec != nil {
			t.Log("Recovered from panic (expected for nil service):", rec)
		}
	}()

	r.ServeHTTP(w, req)
}

// TestWebhookHandler_Test_NilService 测试测试 webhook（nil service）
func TestWebhookHandler_Test_NilService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &WebhookHandler{}
	r.POST("/api/webhooks/:id/test", h.Test)

	req, _ := http.NewRequest("POST", "/api/webhooks/1/test", nil)
	w := httptest.NewRecorder()

	defer func() {
		if rec := recover(); rec != nil {
			t.Log("Recovered from panic (expected for nil service):", rec)
		}
	}()

	r.ServeHTTP(w, req)
}
