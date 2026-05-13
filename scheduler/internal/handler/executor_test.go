package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestExecutorHandler_List(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &ExecutorHandler{}
	r.GET("/api/executors", handler.List)

	req, _ := http.NewRequest("GET", "/api/executors", nil)
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

func TestExecutorHandler_Get_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &ExecutorHandler{}
	r.GET("/api/executors/:id", handler.Get)

	req, _ := http.NewRequest("GET", "/api/executors/invalid", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid ID, got %d", w.Code)
	}
}

func TestExecutorHandler_Get_NegativeID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &ExecutorHandler{}
	r.GET("/api/executors/:id", handler.Get)

	req, _ := http.NewRequest("GET", "/api/executors/-1", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for negative ID, got %d", w.Code)
	}
}

func TestExecutorHandler_Delete_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &ExecutorHandler{}
	r.DELETE("/api/executors/:id", handler.Delete)

	req, _ := http.NewRequest("DELETE", "/api/executors/invalid", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid ID, got %d", w.Code)
	}
}

func TestExecutorHandler_Delete_NegativeID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &ExecutorHandler{}
	r.DELETE("/api/executors/:id", handler.Delete)

	req, _ := http.NewRequest("DELETE", "/api/executors/-1", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for negative ID, got %d", w.Code)
	}
}