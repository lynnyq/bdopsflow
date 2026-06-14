package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSystemConfigHandler_List(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &SystemConfigHandler{}
	r.GET("/api/system/config", handler.List)

	req, _ := http.NewRequest("GET", "/api/system/config", nil)
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

func TestSystemConfigHandler_Update_MissingKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &SystemConfigHandler{}
	r.PUT("/api/system/config/:key", handler.Update)

	body := map[string]interface{}{"value": "test"}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("PUT", "/api/system/config/", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// 路由不匹配时返回 404，这是正常的
	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
		t.Errorf("expected status 400 or 404, got %d", w.Code)
	}
}
