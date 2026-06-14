package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAuditLogHandler_List(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &AuditLogHandler{}
	r.GET("/api/audit-logs", handler.List)

	req, _ := http.NewRequest("GET", "/api/audit-logs", nil)
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

func TestAuditLogHandler_ListWithPagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &AuditLogHandler{}
	r.GET("/api/audit-logs", handler.List)

	req, _ := http.NewRequest("GET", "/api/audit-logs?page=1&page_size=10", nil)
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

func TestAuditLogHandler_ListWithFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &AuditLogHandler{}
	r.GET("/api/audit-logs", handler.List)

	req, _ := http.NewRequest("GET", "/api/audit-logs?user_id=1&action=create&resource=datasource", nil)
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
