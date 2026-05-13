package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestWebhookHandler_Test_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &WebhookHandler{}
	r.POST("/api/webhooks/:id/test", handler.Test)

	req, _ := http.NewRequest("POST", "/api/webhooks/invalid/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid ID, got %d", w.Code)
	}
}

func TestWebhookHandler_Test_NegativeID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &WebhookHandler{}
	r.POST("/api/webhooks/:id/test", handler.Test)

	req, _ := http.NewRequest("POST", "/api/webhooks/-1/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for negative ID, got %d", w.Code)
	}
}

func TestWebhookHandler_Delete_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &WebhookHandler{}
	r.DELETE("/api/webhooks/:id", handler.Delete)

	req, _ := http.NewRequest("DELETE", "/api/webhooks/invalid", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid ID, got %d", w.Code)
	}
}

func TestWebhookHandler_Delete_NegativeID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &WebhookHandler{}
	r.DELETE("/api/webhooks/:id", handler.Delete)

	req, _ := http.NewRequest("DELETE", "/api/webhooks/-1", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for negative ID, got %d", w.Code)
	}
}

func TestWebhookHandler_Create_MissingURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &WebhookHandler{}
	r.POST("/api/webhooks", handler.Create)

	body := map[string]interface{}{
		"events": []string{"task_completed"},
	}

	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/api/webhooks", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for missing URL, got %d", w.Code)
	}
}

func TestWebhookHandler_Create_EmptyURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &WebhookHandler{}
	r.POST("/api/webhooks", handler.Create)

	body := map[string]interface{}{
		"url":    "   ",
		"events": []string{"task_completed"},
	}

	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/api/webhooks", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for empty URL, got %d", w.Code)
	}
}