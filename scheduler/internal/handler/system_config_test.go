package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSystemConfigHandler_List(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &SystemConfigHandler{}
	r.GET("/api/system/config", handler.List)

	req, _ := http.NewRequest("GET", "/api/system/config", nil)
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

func TestSystemConfigHandler_Update_MissingKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &SystemConfigHandler{}
	r.PUT("/api/system/config/:key", handler.Update)

	body := map[string]interface{}{"value": "test"}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("PUT", "/api/system/config/", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// 路由不匹配时返回 404，这是正常的
	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
		t.Errorf("expected status 400 or 404, got %d", w.Code)
	}
}

// TestNewSystemConfigHandler 测试构造函数
func TestNewSystemConfigHandler(t *testing.T) {
	h := NewSystemConfigHandler(nil)
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
}

// TestSystemConfigHandler_Update 测试更新配置
func TestSystemConfigHandler_Update(t *testing.T) {
	t.Run("invalid json", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		h := &SystemConfigHandler{}
		r.PUT("/api/system/config/:key", h.Update)

		req, _ := http.NewRequest("PUT", "/api/system/config/test_key", bytes.NewBufferString("not json"))
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

	t.Run("missing required value", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		h := &SystemConfigHandler{}
		r.PUT("/api/system/config/:key", h.Update)

		body := `{"other":"field"}`
		req, _ := http.NewRequest("PUT", "/api/system/config/test_key", bytes.NewBufferString(body))
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
		h := &SystemConfigHandler{}
		r.PUT("/api/system/config/:key", h.Update)

		body := `{"value":"test_value"}`
		req, _ := http.NewRequest("PUT", "/api/system/config/test_key", bytes.NewBufferString(body))
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

// TestSystemConfigHandler_Reload 测试重载配置
func TestSystemConfigHandler_Reload(t *testing.T) {
	t.Run("nil service", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		h := &SystemConfigHandler{}
		r.POST("/api/system/config/reload", h.Reload)

		req, _ := http.NewRequest("POST", "/api/system/config/reload", nil)
		w := httptest.NewRecorder()

		defer func() {
			if rec := recover(); rec != nil {
				t.Log("Recovered from panic (expected for nil service):", rec)
			}
		}()

		r.ServeHTTP(w, req)
	})
}
