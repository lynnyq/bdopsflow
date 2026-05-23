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
