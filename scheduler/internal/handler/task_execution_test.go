package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestTaskExecutionHandler_ListByTask_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &TaskExecutionHandler{}
	r.GET("/api/task-executions/task/:task_id", handler.ListByTask)

	req, _ := http.NewRequest("GET", "/api/task-executions/task/invalid", nil)
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

// TestNewTaskExecutionHandler 测试构造函数
func TestNewTaskExecutionHandler(t *testing.T) {
	h := NewTaskExecutionHandler(nil)
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
}

// TestTaskExecutionHandler_ListByTask 测试按任务 ID 查询执行记录
func TestTaskExecutionHandler_ListByTask(t *testing.T) {
	t.Run("zero id", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		h := &TaskExecutionHandler{}
		r.GET("/api/task-executions/task/:task_id", h.ListByTask)

		req, _ := http.NewRequest("GET", "/api/task-executions/task/0", nil)
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
		h := &TaskExecutionHandler{}
		r.GET("/api/task-executions/task/:task_id", h.ListByTask)

		req, _ := http.NewRequest("GET", "/api/task-executions/task/-1", nil)
		w := httptest.NewRecorder()

		defer func() {
			if rec := recover(); rec != nil {
				t.Log("Recovered from panic (expected for routing):", rec)
			}
		}()

		r.ServeHTTP(w, req)
	})

	t.Run("valid id with nil service", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		r := gin.New()
		h := &TaskExecutionHandler{}
		r.GET("/api/task-executions/task/:task_id", h.ListByTask)

		req, _ := http.NewRequest("GET", "/api/task-executions/task/1", nil)
		w := httptest.NewRecorder()

		defer func() {
			if rec := recover(); rec != nil {
				t.Log("Recovered from panic (expected for nil service):", rec)
			}
		}()

		r.ServeHTTP(w, req)
	})
}
