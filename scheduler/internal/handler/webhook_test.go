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

	h := &WebhookHandler{}
	r.POST("/api/webhooks/:id/test", h.Test)

	req, _ := http.NewRequest("POST", "/api/webhooks/invalid/test", nil)
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

func TestWebhookHandler_Delete_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	h := &WebhookHandler{}
	r.DELETE("/api/webhooks/:id", h.Delete)

	req, _ := http.NewRequest("DELETE", "/api/webhooks/invalid", nil)
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

func TestWebhookHandler_Create_MissingRequired(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	h := &WebhookHandler{}
	r.POST("/api/webhooks", h.Create)

	body := map[string]interface{}{
		"description": "test",
	}

	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/api/webhooks", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if code, ok := resp["code"].(float64); !ok || int(code) != 400 {
		t.Errorf("expected body.code 400 for missing required fields, got %v", resp["code"])
	}
}

func TestWebhookHandler_List_MissingDomainID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	h := &WebhookHandler{}
	r.GET("/api/webhooks", h.List)

	req, _ := http.NewRequest("GET", "/api/webhooks", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if code, ok := resp["code"].(float64); !ok || int(code) != 0 {
		t.Errorf("expected body.code 0 for optional domain_id, got %v", resp["code"])
	}
}
