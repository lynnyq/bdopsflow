package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestDashboardHandler_GetStats(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &DashboardHandler{}
	r.GET("/api/dashboard/stats", handler.GetStats)

	req, _ := http.NewRequest("GET", "/api/dashboard/stats", nil)
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

func TestDashboardHandler_GetTrends(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &DashboardHandler{}
	r.GET("/api/dashboard/trends", handler.GetTrends)

	req, _ := http.NewRequest("GET", "/api/dashboard/trends", nil)
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

func TestDashboardHandler_GetSchedulerStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &DashboardHandler{}
	r.GET("/api/dashboard/scheduler/status", handler.GetSchedulerStatus)

	req, _ := http.NewRequest("GET", "/api/dashboard/scheduler/status", nil)
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

func TestDashboardHandler_PauseScheduler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &DashboardHandler{}
	r.POST("/api/dashboard/scheduler/pause", handler.PauseScheduler)

	req, _ := http.NewRequest("POST", "/api/dashboard/scheduler/pause", nil)
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

func TestDashboardHandler_ResumeScheduler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &DashboardHandler{}
	r.POST("/api/dashboard/scheduler/resume", handler.ResumeScheduler)

	req, _ := http.NewRequest("POST", "/api/dashboard/scheduler/resume", nil)
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
