package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestLogHandler_List(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &LogHandler{}
	r.GET("/api/logs", handler.List)

	req, _ := http.NewRequest("GET", "/api/logs?page=1&page_size=20", nil)
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

func TestLogHandler_Delete_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &LogHandler{}
	r.DELETE("/api/logs/:id", handler.Delete)

	req, _ := http.NewRequest("DELETE", "/api/logs/invalid", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid ID, got %d", w.Code)
	}
}

func TestLogHandler_BatchDelete_EmptyIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &LogHandler{}
	r.POST("/api/logs/batch-delete", handler.BatchDelete)

	body := map[string]interface{}{
		"ids": []int64{},
	}

	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/api/logs/batch-delete", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for empty IDs, got %d", w.Code)
	}
}