package handler

import (
	"encoding/json"
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

func TestExecutorHandler_Get_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &ExecutorHandler{}
	r.GET("/api/bdopsflow_executors/:id", handler.Get)

	req, _ := http.NewRequest("GET", "/api/bdopsflow_executors/invalid", nil)
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

func TestExecutorHandler_Get_NegativeID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &ExecutorHandler{}
	r.GET("/api/bdopsflow_executors/:id", handler.Get)

	req, _ := http.NewRequest("GET", "/api/bdopsflow_executors/-1", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200, got %d", w.Code)
	}

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	if code, ok := body["code"].(float64); !ok || int(code) != 400 {
		t.Errorf("expected body.code 400 for negative ID, got %v", body["code"])
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
		t.Errorf("expected HTTP 404 for missing ID, got %d", w.Code)
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
		t.Errorf("expected HTTP 404 for missing ID, got %d", w.Code)
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
		t.Errorf("expected HTTP 404 for missing ID, got %d", w.Code)
	}
}

func TestExecutorHandler_Online_WithID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &ExecutorHandler{}
	r.POST("/api/bdopsflow_executors/:id/online", handler.Online)

	req, _ := http.NewRequest("POST", "/api/bdopsflow_executors/executor-1/online", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200, got %d", w.Code)
	}

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	if code, ok := body["code"].(float64); !ok || int(code) != 400 {
		t.Errorf("expected body.code 400 for nil service, got %v", body["code"])
	}
}

func TestExecutorHandler_Offline_WithID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &ExecutorHandler{}
	r.POST("/api/bdopsflow_executors/:id/offline", handler.Offline)

	req, _ := http.NewRequest("POST", "/api/bdopsflow_executors/executor-1/offline", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200, got %d", w.Code)
	}

	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	if code, ok := body["code"].(float64); !ok || int(code) != 400 {
		t.Errorf("expected body.code 400 for nil service, got %v", body["code"])
	}
}
