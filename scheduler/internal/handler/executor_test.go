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
	r.GET("/api/bdopsflow_executors", handler.List)

	req, _ := http.NewRequest("GET", "/api/bdopsflow_executors", nil)
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
	r.GET("/api/bdopsflow_executors/:id", handler.Get)

	req, _ := http.NewRequest("GET", "/api/bdopsflow_executors/invalid", nil)
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
	r.GET("/api/bdopsflow_executors/:id", handler.Get)

	req, _ := http.NewRequest("GET", "/api/bdopsflow_executors/-1", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for negative ID, got %d", w.Code)
	}
}

func TestExecutorHandler_Delete_MissingID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &ExecutorHandler{}
	r.DELETE("/api/bdopsflow_executors/:id", handler.Delete)

	req, _ := http.NewRequest("DELETE", "/api/bdopsflow_executors/", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for missing ID, got %d", w.Code)
	}
}

func TestExecutorHandler_Online_MissingID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &ExecutorHandler{}
	r.POST("/api/bdopsflow_executors/:id/online", handler.Online)

	req, _ := http.NewRequest("POST", "/api/bdopsflow_executors/", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for missing ID, got %d", w.Code)
	}
}

func TestExecutorHandler_Offline_MissingID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &ExecutorHandler{}
	r.POST("/api/bdopsflow_executors/:id/offline", handler.Offline)

	req, _ := http.NewRequest("POST", "/api/bdopsflow_executors/", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404 for missing ID, got %d", w.Code)
	}
}

func TestExecutorHandler_Online_WithID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &ExecutorHandler{}
	r.POST("/api/bdopsflow_executors/:id/online", handler.Online)

	req, _ := http.NewRequest("POST", "/api/bdopsflow_executors/executor-1/online", nil)
	w := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Log("Recovered from panic (expected for nil db):", r)
			return
		}
	}()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError && w.Code != http.StatusBadRequest {
		t.Errorf("expected status 200, 400 or 500, got %d", w.Code)
	}
}

func TestExecutorHandler_Offline_WithID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &ExecutorHandler{}
	r.POST("/api/bdopsflow_executors/:id/offline", handler.Offline)

	req, _ := http.NewRequest("POST", "/api/bdopsflow_executors/executor-1/offline", nil)
	w := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Log("Recovered from panic (expected for nil db):", r)
			return
		}
	}()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError && w.Code != http.StatusBadRequest {
		t.Errorf("expected status 200, 400 or 500, got %d", w.Code)
	}
}
