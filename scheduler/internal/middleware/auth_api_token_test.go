package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// mockAPITokenValidator APITokenValidator 的 mock 实现
type mockAPITokenValidator struct {
	validateFunc    func(ctx context.Context, tokenString string) (int64, error)
	getUserInfoFunc func(ctx context.Context, userID int64) (string, string, int64, error)
}

func (m *mockAPITokenValidator) ValidateToken(ctx context.Context, tokenString string) (int64, error) {
	if m.validateFunc != nil {
		return m.validateFunc(ctx, tokenString)
	}
	return 0, nil
}

func (m *mockAPITokenValidator) GetTokenUserInfo(ctx context.Context, userID int64) (string, string, int64, error) {
	if m.getUserInfoFunc != nil {
		return m.getUserInfoFunc(ctx, userID)
	}
	return "", "", 0, nil
}

// setupRouterWithAPIToken 创建带 JWTAuthMiddlewareWithAPIToken 中间件的测试路由
// handler 会将上下文中的 user_id、username、real_name、current_domain_id、auth_type 写入响应
func setupRouterWithAPIToken(validator APITokenValidator) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(JWTAuthMiddlewareWithAPIToken(validator))
	router.GET("/test", func(c *gin.Context) {
		userID, _ := c.Get("user_id")
		username, _ := c.Get("username")
		realName, _ := c.Get("real_name")
		domainID, _ := c.Get("current_domain_id")
		authType, _ := c.Get("auth_type")

		c.JSON(http.StatusOK, gin.H{
			"user_id":           userID,
			"username":          username,
			"real_name":         realName,
			"current_domain_id": domainID,
			"auth_type":         authType,
		})
	})
	return router
}

func TestJWTAuthMiddlewareWithAPIToken_ValidJWT(t *testing.T) {
	validator := &mockAPITokenValidator{}
	router := setupRouterWithAPIToken(validator)

	token, err := GenerateToken(42, "testuser", "Test User", 10)
	if err != nil {
		t.Fatalf("生成 token 失败: %v", err)
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，实际 %d", w.Code)
	}

	body := w.Body.String()
	expectedPairs := []struct {
		key   string
		value string
	}{
		{`"user_id"`, `42`},
		{`"username"`, `"testuser"`},
		{`"real_name"`, `"Test User"`},
		{`"current_domain_id"`, `10`},
		{`"auth_type"`, `"jwt"`},
	}
	for _, p := range expectedPairs {
		expected := fmt.Sprintf(`%s:%s`, p.key, p.value)
		if !containsSubstring(body, expected) {
			t.Errorf("响应体中未找到 %s，响应: %s", expected, body)
		}
	}
}

func TestJWTAuthMiddlewareWithAPIToken_InvalidToken_NoPrefix(t *testing.T) {
	validator := &mockAPITokenValidator{
		validateFunc: func(ctx context.Context, tokenString string) (int64, error) {
			return 0, errors.New("invalid token")
		},
	}
	router := setupRouterWithAPIToken(validator)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer some-random-token")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("期望状态码 401，实际 %d", w.Code)
	}
}

func TestJWTAuthMiddlewareWithAPIToken_APIToken_Valid(t *testing.T) {
	validator := &mockAPITokenValidator{
		validateFunc: func(ctx context.Context, tokenString string) (int64, error) {
			if tokenString == "bdf_valid_token" {
				return 100, nil
			}
			return 0, errors.New("invalid api token")
		},
		getUserInfoFunc: func(ctx context.Context, userID int64) (string, string, int64, error) {
			if userID == 100 {
				return "apiuser", "API User", 5, nil
			}
			return "", "", 0, errors.New("user not found")
		},
	}
	router := setupRouterWithAPIToken(validator)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer bdf_valid_token")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，实际 %d，响应: %s", w.Code, w.Body.String())
	}

	body := w.Body.String()
	expectedPairs := []struct {
		key   string
		value string
	}{
		{`"user_id"`, `100`},
		{`"username"`, `"apiuser"`},
		{`"real_name"`, `"API User"`},
		{`"current_domain_id"`, `5`},
		{`"auth_type"`, `"api_token"`},
	}
	for _, p := range expectedPairs {
		expected := fmt.Sprintf(`%s:%s`, p.key, p.value)
		if !containsSubstring(body, expected) {
			t.Errorf("响应体中未找到 %s，响应: %s", expected, body)
		}
	}
}

func TestJWTAuthMiddlewareWithAPIToken_APIToken_Invalid(t *testing.T) {
	validator := &mockAPITokenValidator{
		validateFunc: func(ctx context.Context, tokenString string) (int64, error) {
			return 0, errors.New("invalid api token")
		},
	}
	router := setupRouterWithAPIToken(validator)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer bdf_invalid_token")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("期望状态码 401，实际 %d", w.Code)
	}
}

func TestJWTAuthMiddlewareWithAPIToken_APIToken_GetUserInfoFails(t *testing.T) {
	validator := &mockAPITokenValidator{
		validateFunc: func(ctx context.Context, tokenString string) (int64, error) {
			return 100, nil
		},
		getUserInfoFunc: func(ctx context.Context, userID int64) (string, string, int64, error) {
			return "", "", 0, errors.New("database error")
		},
	}
	router := setupRouterWithAPIToken(validator)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer bdf_token_user_info_fail")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("期望状态码 401，实际 %d", w.Code)
	}
}

func TestJWTAuthMiddlewareWithAPIToken_NoToken(t *testing.T) {
	validator := &mockAPITokenValidator{}
	router := setupRouterWithAPIToken(validator)

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("期望状态码 401，实际 %d", w.Code)
	}
}

func TestJWTAuthMiddlewareWithAPIToken_NilValidator(t *testing.T) {
	router := setupRouterWithAPIToken(nil)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer bdf_some_token")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("期望状态码 401，实际 %d", w.Code)
	}
}

func TestJWTAuthMiddlewareWithAPIToken_QueryToken(t *testing.T) {
	validator := &mockAPITokenValidator{}
	router := setupRouterWithAPIToken(validator)

	token, err := GenerateToken(7, "queryuser", "Query User", 3)
	if err != nil {
		t.Fatalf("生成 token 失败: %v", err)
	}

	req := httptest.NewRequest("GET", "/test?token="+token, nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，实际 %d", w.Code)
	}

	body := w.Body.String()
	expectedPairs := []struct {
		key   string
		value string
	}{
		{`"user_id"`, `7`},
		{`"username"`, `"queryuser"`},
		{`"real_name"`, `"Query User"`},
		{`"current_domain_id"`, `3`},
		{`"auth_type"`, `"jwt"`},
	}
	for _, p := range expectedPairs {
		expected := fmt.Sprintf(`%s:%s`, p.key, p.value)
		if !containsSubstring(body, expected) {
			t.Errorf("响应体中未找到 %s，响应: %s", expected, body)
		}
	}
}

// containsSubstring 检查字符串 s 是否包含 substr
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || searchSubstring(s, substr))
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
