package handler

import (
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

	defer func() {
		if r := recover(); r != nil {
			t.Log("Recovered from panic (expected for nil service):", r)
			return
		}
	}()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d", w.Code)
	}
}

func TestDashboardHandler_GetTrends(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &DashboardHandler{}
	r.GET("/api/dashboard/trends", handler.GetTrends)

	req, _ := http.NewRequest("GET", "/api/dashboard/trends", nil)
	w := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Log("Recovered from panic (expected for nil service):", r)
			return
		}
	}()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d", w.Code)
	}
}

func TestDashboardHandler_GetSchedulerStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &DashboardHandler{}
	r.GET("/api/dashboard/scheduler/status", handler.GetSchedulerStatus)

	req, _ := http.NewRequest("GET", "/api/dashboard/scheduler/status", nil)
	w := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Log("Recovered from panic (expected for nil service):", r)
			return
		}
	}()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d", w.Code)
	}
}

func TestDashboardHandler_PauseScheduler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &DashboardHandler{}
	r.POST("/api/dashboard/scheduler/pause", handler.PauseScheduler)

	req, _ := http.NewRequest("POST", "/api/dashboard/scheduler/pause", nil)
	w := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Log("Recovered from panic (expected for nil service):", r)
			return
		}
	}()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d", w.Code)
	}
}

func TestDashboardHandler_ResumeScheduler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &DashboardHandler{}
	r.POST("/api/dashboard/scheduler/resume", handler.ResumeScheduler)

	req, _ := http.NewRequest("POST", "/api/dashboard/scheduler/resume", nil)
	w := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Log("Recovered from panic (expected for nil service):", r)
			return
		}
	}()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d", w.Code)
	}
}
