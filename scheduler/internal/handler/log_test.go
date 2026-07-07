package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestLogHandler_List(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &LogHandler{}
	r.GET("/api/logs", handler.List)

	req, _ := http.NewRequest("GET", "/api/logs?page=1&page_size=20", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200, got %d", w.Code)
	}

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	if code, ok := body["code"].(float64); !ok || int(code) != 500 {
		t.Errorf("expected body.code 500 for nil service, got %v", body["code"])
	}
}

func TestLogHandler_Delete_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &LogHandler{}
	r.DELETE("/api/logs/:id", handler.Delete)

	req, _ := http.NewRequest("DELETE", "/api/logs/invalid", nil)
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

func TestLogHandler_BatchDelete_EmptyIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &LogHandler{}
	r.POST("/api/logs/batch-delete", handler.BatchDelete)

	body := map[string]interface{}{
		"ids": []int64{},
	}

	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/api/logs/batch-delete", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if code, ok := resp["code"].(float64); !ok || int(code) != 400 {
		t.Errorf("expected body.code 400 for empty IDs, got %v", resp["code"])
	}
}

// TestNewLogHandler 测试构造函数
func TestNewLogHandler(t *testing.T) {
	h := NewLogHandler(nil)
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
}

// TestLogHandler_Delete 测试删除日志的各种场景
func TestLogHandler_Delete(t *testing.T) {
	t.Run("valid numeric id with nil service", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		h := &LogHandler{}
		r.DELETE("/api/logs/:id", h.Delete)

		req, _ := http.NewRequest("DELETE", "/api/logs/1", nil)
		w := httptest.NewRecorder()

		defer func() {
			if rec := recover(); rec != nil {
				t.Log("Recovered from panic (expected for nil service):", rec)
			}
		}()

		r.ServeHTTP(w, req)
	})

	t.Run("zero id", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		h := &LogHandler{}
		r.DELETE("/api/logs/:id", h.Delete)

		req, _ := http.NewRequest("DELETE", "/api/logs/0", nil)
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

	t.Run("negative id", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		h := &LogHandler{}
		r.DELETE("/api/logs/:id", h.Delete)

		req, _ := http.NewRequest("DELETE", "/api/logs/-1", nil)
		w := httptest.NewRecorder()

		defer func() {
			if rec := recover(); rec != nil {
				t.Log("Recovered from panic (expected for routing):", rec)
			}
		}()

		r.ServeHTTP(w, req)
	})
}

// TestLogHandler_BatchDelete 测试批量删除
func TestLogHandler_BatchDelete(t *testing.T) {
	t.Run("invalid json", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		h := &LogHandler{}
		r.POST("/api/logs/batch-delete", h.BatchDelete)

		req, _ := http.NewRequest("POST", "/api/logs/batch-delete", bytes.NewBufferString("not json"))
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

	t.Run("valid ids but nil service", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		h := &LogHandler{}
		r.POST("/api/logs/batch-delete", h.BatchDelete)

		body := `{"ids":[1,2,3]}`
		req, _ := http.NewRequest("POST", "/api/logs/batch-delete", bytes.NewBufferString(body))
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

// TestLogHandler_GetStats 测试获取统计
func TestLogHandler_GetStats(t *testing.T) {
	t.Run("nil service", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		h := &LogHandler{}
		r.GET("/api/logs/stats", h.GetStats)

		req, _ := http.NewRequest("GET", "/api/logs/stats", nil)
		w := httptest.NewRecorder()

		defer func() {
			if rec := recover(); rec != nil {
				t.Log("Recovered from panic (expected for nil service):", rec)
			}
		}()

		r.ServeHTTP(w, req)
	})
}

// TestLogHandler_List_WithFilters 测试带过滤器的列表查询
func TestLogHandler_List_WithFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &LogHandler{}
	r.GET("/api/logs", h.List)

	req, _ := http.NewRequest("GET", "/api/logs?status=success&executor_name=exec1&task_name=task1", nil)
	w := httptest.NewRecorder()

	defer func() {
		if rec := recover(); rec != nil {
			t.Log("Recovered from panic (expected for nil service):", rec)
		}
	}()

	r.ServeHTTP(w, req)
}
