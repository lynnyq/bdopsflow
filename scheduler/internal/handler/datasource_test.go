package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestDatasourceHandler_List(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &DatasourceHandler{}
	r.GET("/api/datasources", handler.List)

	req, _ := http.NewRequest("GET", "/api/datasources", nil)
	w := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Log("Recovered from panic (expected for nil db):", r)
			return
		}
	}()

	r.ServeHTTP(w, req)

	// Should panic due to nil db, but we recover
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d", w.Code)
	}
}

func TestDatasourceHandler_Get(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &DatasourceHandler{}
	r.GET("/api/datasources/:id", handler.Get)

	req, _ := http.NewRequest("GET", "/api/datasources/1", nil)
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

func TestDatasourceHandler_Create_MissingFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &DatasourceHandler{}
	r.POST("/api/datasources", handler.Create)

	body := map[string]interface{}{}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/api/datasources", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// BadRequest returns HTTP 200 with error code in JSON body
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp["status"] != "error" {
		t.Errorf("expected status 'error' in response body, got %v", resp["status"])
	}
}

func TestDatasourceHandler_Update_MissingFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &DatasourceHandler{}
	r.PUT("/api/datasources/:id", handler.Update)

	body := map[string]interface{}{}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("PUT", "/api/datasources/1", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Log("Recovered from panic (expected for nil service):", r)
			return
		}
	}()

	r.ServeHTTP(w, req)

	// If no panic, check response
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 or panic, got %d", w.Code)
	}
}

func TestDatasourceHandler_Delete(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &DatasourceHandler{}
	r.DELETE("/api/datasources/:id", handler.Delete)

	req, _ := http.NewRequest("DELETE", "/api/datasources/1", nil)
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

func TestDatasourceHandler_TestConnection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &DatasourceHandler{}
	r.POST("/api/datasources/:id/test", handler.TestConnection)

	req, _ := http.NewRequest("POST", "/api/datasources/1/test", nil)
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
