package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// === NewPermissionHandler 测试 ===

func TestNewPermissionHandler(t *testing.T) {
	h := NewPermissionHandler(nil)
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
	if h.svc != nil {
		t.Errorf("expected nil svc, got %v", h.svc)
	}
}

// === PermissionHandler.GetAllPermissions panic 恢复测试 ===

func TestPermissionHandler_GetAllPermissions_NilService_PanicRecovery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &PermissionHandler{} // svc 为 nil

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	h.GetAllPermissions(c)

	// handler 内部有 defer recover，应返回 500 而非 panic
	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeInternalError {
		t.Errorf("expected code %d for nil service panic recovery, got %d", CodeInternalError, resp.Code)
	}
}
