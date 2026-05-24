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
	r.GET("/api/bdopsflow_workflows/:id", handler.Get)

	req, _ := http.NewRequest("GET", "/api/bdopsflow_workflows/invalid", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200, got %d", w.Code)
	}
	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code 400 for invalid ID, got %d", resp.Code)
	}
}

func TestWorkflowHandler_Get_NegativeID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &WorkflowHandler{}
	r.GET("/api/bdopsflow_workflows/:id", handler.Get)

	req, _ := http.NewRequest("GET", "/api/bdopsflow_workflows/-1", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200, got %d", w.Code)
	}
	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code 400 for negative ID, got %d", resp.Code)
	}
}

func TestWorkflowHandler_Create_MissingName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &WorkflowHandler{}
	r.POST("/api/bdopsflow_workflows", handler.Create)

	body := map[string]interface{}{
		"description": "test",
	}

	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/api/bdopsflow_workflows", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200, got %d", w.Code)
	}
	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code 400 for missing name, got %d", resp.Code)
	}
}

func TestWorkflowHandler_Create_EmptyName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &WorkflowHandler{}
	r.POST("/api/bdopsflow_workflows", handler.Create)

	body := map[string]interface{}{
		"name":        "   ",
		"description": "test",
	}

	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/api/bdopsflow_workflows", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200, got %d", w.Code)
	}
	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code 400 for empty name, got %d", resp.Code)
	}
}

func TestWorkflowHandler_TriggerWorkflow_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &WorkflowHandler{}
	r.POST("/api/bdopsflow_workflows/:id/trigger", handler.TriggerWorkflow)

	req, _ := http.NewRequest("POST", "/api/bdopsflow_workflows/invalid/trigger", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200, got %d", w.Code)
	}
	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code 400 for invalid ID, got %d", resp.Code)
	}
}

func TestWorkflowHandler_TriggerWorkflow_NegativeID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &WorkflowHandler{}
	r.POST("/api/bdopsflow_workflows/:id/trigger", handler.TriggerWorkflow)

	req, _ := http.NewRequest("POST", "/api/bdopsflow_workflows/-1/trigger", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200, got %d", w.Code)
	}
	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code 400 for negative ID, got %d", resp.Code)
	}
}

func TestWorkflowHandler_Delete_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &WorkflowHandler{}
	r.DELETE("/api/bdopsflow_workflows/:id", handler.Delete)

	req, _ := http.NewRequest("DELETE", "/api/bdopsflow_workflows/invalid", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200, got %d", w.Code)
	}
	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code 400 for invalid ID, got %d", resp.Code)
	}
}

func TestWorkflowHandler_Delete_NegativeID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &WorkflowHandler{}
	r.DELETE("/api/bdopsflow_workflows/:id", handler.Delete)

	req, _ := http.NewRequest("DELETE", "/api/bdopsflow_workflows/-1", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200, got %d", w.Code)
	}
	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code 400 for negative ID, got %d", resp.Code)
	}
}

func TestWorkflowHandler_Update_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &WorkflowHandler{}
	r.PUT("/api/bdopsflow_workflows/:id", handler.Update)

	body := map[string]interface{}{
		"name": "updated",
	}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("PUT", "/api/bdopsflow_workflows/invalid", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200, got %d", w.Code)
	}
	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code 400 for invalid ID, got %d", resp.Code)
	}
}
