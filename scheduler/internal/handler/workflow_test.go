package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestWorkflowHandler_Get_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &WorkflowHandler{}
	r.GET("/api/workflows/:id", handler.Get)

	req, _ := http.NewRequest("GET", "/api/workflows/invalid", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid ID, got %d", w.Code)
	}
}

func TestWorkflowHandler_Get_NegativeID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &WorkflowHandler{}
	r.GET("/api/workflows/:id", handler.Get)

	req, _ := http.NewRequest("GET", "/api/workflows/-1", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for negative ID, got %d", w.Code)
	}
}

func TestWorkflowHandler_Create_MissingName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &WorkflowHandler{}
	r.POST("/api/workflows", handler.Create)

	body := map[string]interface{}{
		"description": "test",
	}

	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/api/workflows", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for missing name, got %d", w.Code)
	}
}

func TestWorkflowHandler_Create_EmptyName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &WorkflowHandler{}
	r.POST("/api/workflows", handler.Create)

	body := map[string]interface{}{
		"name":        "   ",
		"description": "test",
	}

	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/api/workflows", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for empty name, got %d", w.Code)
	}
}

func TestWorkflowHandler_TriggerWorkflow_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &WorkflowHandler{}
	r.POST("/api/workflows/:id/trigger", handler.TriggerWorkflow)

	req, _ := http.NewRequest("POST", "/api/workflows/invalid/trigger", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid ID, got %d", w.Code)
	}
}

func TestWorkflowHandler_TriggerWorkflow_NegativeID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &WorkflowHandler{}
	r.POST("/api/workflows/:id/trigger", handler.TriggerWorkflow)

	req, _ := http.NewRequest("POST", "/api/workflows/-1/trigger", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for negative ID, got %d", w.Code)
	}
}

func TestWorkflowHandler_Delete_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &WorkflowHandler{}
	r.DELETE("/api/workflows/:id", handler.Delete)

	req, _ := http.NewRequest("DELETE", "/api/workflows/invalid", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid ID, got %d", w.Code)
	}
}

func TestWorkflowHandler_Delete_NegativeID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &WorkflowHandler{}
	r.DELETE("/api/workflows/:id", handler.Delete)

	req, _ := http.NewRequest("DELETE", "/api/workflows/-1", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for negative ID, got %d", w.Code)
	}
}

func TestWorkflowHandler_Update_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &WorkflowHandler{}
	r.PUT("/api/workflows/:id", handler.Update)

	body := map[string]interface{}{
		"name": "updated",
	}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("PUT", "/api/workflows/invalid", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid ID, got %d", w.Code)
	}
}