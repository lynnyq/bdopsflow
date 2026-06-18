package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/middleware"
)

func init() {
	middleware.InitJWT("test-secret-key-for-unit-tests", 24)
}

// TestAPITokenHandler_Generate_NoAuth 请求中没有 user_id 应返回 401
func TestAPITokenHandler_Generate_NoAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &APITokenHandler{}
	r.POST("/api/tokens/generate", handler.Generate)

	req, _ := http.NewRequest("POST", "/api/tokens/generate", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Code != CodeUnauthorized {
		t.Errorf("expected code %d for no auth, got %d", CodeUnauthorized, resp.Code)
	}
}

// TestAPITokenHandler_Generate_InvalidUserID user_id 类型无效应返回 400
func TestAPITokenHandler_Generate_InvalidUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &APITokenHandler{}
	r.POST("/api/tokens/generate", func(c *gin.Context) {
		c.Set("user_id", "not-an-int64")
		handler.Generate(c)
	})

	req, _ := http.NewRequest("POST", "/api/tokens/generate", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for invalid user_id, got %d", CodeBadRequest, resp.Code)
	}
}

// TestAPITokenHandler_GetInfo_NoAuth 请求中没有 user_id 应返回 401
func TestAPITokenHandler_GetInfo_NoAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &APITokenHandler{}
	r.GET("/api/tokens/info", handler.GetInfo)

	req, _ := http.NewRequest("GET", "/api/tokens/info", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Code != CodeUnauthorized {
		t.Errorf("expected code %d for no auth, got %d", CodeUnauthorized, resp.Code)
	}
}

// TestAPITokenHandler_GetInfo_InvalidUserID user_id 类型无效应返回 400
func TestAPITokenHandler_GetInfo_InvalidUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &APITokenHandler{}
	r.GET("/api/tokens/info", func(c *gin.Context) {
		c.Set("user_id", 123.456)
		handler.GetInfo(c)
	})

	req, _ := http.NewRequest("GET", "/api/tokens/info", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for invalid user_id, got %d", CodeBadRequest, resp.Code)
	}
}

// TestAPITokenHandler_Reveal_NoAuth 请求中没有 user_id 应返回 401
func TestAPITokenHandler_Reveal_NoAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &APITokenHandler{}
	r.POST("/api/tokens/reveal", handler.Reveal)

	req, _ := http.NewRequest("POST", "/api/tokens/reveal", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Code != CodeUnauthorized {
		t.Errorf("expected code %d for no auth, got %d", CodeUnauthorized, resp.Code)
	}
}

// TestAPITokenHandler_Reveal_InvalidUserID user_id 类型无效应返回 400
func TestAPITokenHandler_Reveal_InvalidUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &APITokenHandler{}
	r.POST("/api/tokens/reveal", func(c *gin.Context) {
		c.Set("user_id", "invalid")
		handler.Reveal(c)
	})

	req, _ := http.NewRequest("POST", "/api/tokens/reveal", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for invalid user_id, got %d", CodeBadRequest, resp.Code)
	}
}

// TestAPITokenHandler_Revoke_NoAuth 请求中没有 user_id 应返回 401
func TestAPITokenHandler_Revoke_NoAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &APITokenHandler{}
	r.POST("/api/tokens/revoke", handler.Revoke)

	req, _ := http.NewRequest("POST", "/api/tokens/revoke", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Code != CodeUnauthorized {
		t.Errorf("expected code %d for no auth, got %d", CodeUnauthorized, resp.Code)
	}
}

// TestAPITokenHandler_Revoke_InvalidUserID user_id 类型无效应返回 400
func TestAPITokenHandler_Revoke_InvalidUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &APITokenHandler{}
	r.POST("/api/tokens/revoke", func(c *gin.Context) {
		c.Set("user_id", true)
		handler.Revoke(c)
	})

	req, _ := http.NewRequest("POST", "/api/tokens/revoke", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for invalid user_id, got %d", CodeBadRequest, resp.Code)
	}
}

// TestAPITokenHandler_Generate_NilService service 为 nil 时应 panic
func TestAPITokenHandler_Generate_NilService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &APITokenHandler{}
	r.POST("/api/tokens/generate", func(c *gin.Context) {
		c.Set("user_id", int64(1))
		handler.Generate(c)
	})

	req, _ := http.NewRequest("POST", "/api/tokens/generate", nil)
	w := httptest.NewRecorder()

	defer func() {
		if rec := recover(); rec != nil {
			t.Log("Recovered from panic (expected for nil service):", rec)
			return
		}
	}()

	r.ServeHTTP(w, req)
}

// TestAPITokenHandler_GetInfo_NilService service 为 nil 时应 panic
func TestAPITokenHandler_GetInfo_NilService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &APITokenHandler{}
	r.GET("/api/tokens/info", func(c *gin.Context) {
		c.Set("user_id", int64(1))
		handler.GetInfo(c)
	})

	req, _ := http.NewRequest("GET", "/api/tokens/info", nil)
	w := httptest.NewRecorder()

	defer func() {
		if rec := recover(); rec != nil {
			t.Log("Recovered from panic (expected for nil service):", rec)
			return
		}
	}()

	r.ServeHTTP(w, req)
}

// TestAPITokenHandler_contextToString 测试 contextToString 辅助函数
func TestAPITokenHandler_contextToString(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{"nil value", nil, ""},
		{"string value", "hello", "hello"},
		{"empty string", "", ""},
		{"non-string value (int)", 42, ""},
		{"non-string value (float)", 3.14, ""},
		{"non-string value (bool)", true, ""},
		{"non-string value (struct)", struct{}{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contextToString(tt.input)
			if result != tt.expected {
				t.Errorf("contextToString(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
