package handler

import (
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

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid ID, got %d", w.Code)
	}
}