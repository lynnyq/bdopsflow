package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestAuthHandler_Login_InvalidJSON 测试 Login 传入非法 JSON
func TestAuthHandler_Login_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &AuthHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("not json")))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Login(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for invalid JSON, got %d", CodeBadRequest, resp.Code)
	}
}

// TestAuthHandler_Login_EmptyBody 测试 Login 传入空 body
func TestAuthHandler_Login_EmptyBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &AuthHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("{}")))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Login(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for empty body, got %d", CodeBadRequest, resp.Code)
	}
}

// TestAuthHandler_Register_InvalidJSON 测试 Register 传入非法 JSON
func TestAuthHandler_Register_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &AuthHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("not json")))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Register(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for invalid JSON, got %d", CodeBadRequest, resp.Code)
	}
}

// TestAuthHandler_Register_EmptyBody 测试 Register 传入空 body
func TestAuthHandler_Register_EmptyBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &AuthHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("{}")))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Register(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for empty body, got %d", CodeBadRequest, resp.Code)
	}
}

// TestAuthHandler_Register_MissingPassword 测试 Register 缺少密码
func TestAuthHandler_Register_MissingPassword(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &AuthHandler{}

	body := map[string]interface{}{
		"username": "testuser",
	}
	bodyBytes, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(bodyBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Register(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for missing password, got %d", CodeBadRequest, resp.Code)
	}
}

// TestAuthHandler_Register_MissingUsername 测试 Register 缺少用户名
func TestAuthHandler_Register_MissingUsername(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &AuthHandler{}

	body := map[string]interface{}{
		"password": "password123",
	}
	bodyBytes, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(bodyBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	h.Register(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for missing username, got %d", CodeBadRequest, resp.Code)
	}
}

// TestAuthHandler_GetCurrentUser_NoAuth 测试 GetCurrentUser 未授权
func TestAuthHandler_GetCurrentUser_NoAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &AuthHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	h.GetCurrentUser(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeUnauthorized {
		t.Errorf("expected code %d for no auth, got %d", CodeUnauthorized, resp.Code)
	}
}

// TestAuthHandler_SwitchDomain_InvalidJSON 测试 SwitchDomain 传入非法 JSON
func TestAuthHandler_SwitchDomain_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &AuthHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("not json")))
	c.Request.Header.Set("Content-Type", "application/json")

	h.SwitchDomain(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for invalid JSON, got %d", CodeBadRequest, resp.Code)
	}
}

// TestAuthHandler_SwitchDomain_ZeroDomainID 测试 SwitchDomain domain_id 为 0
func TestAuthHandler_SwitchDomain_ZeroDomainID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &AuthHandler{}

	body := map[string]interface{}{
		"domain_id": 0,
	}
	bodyBytes, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(bodyBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	h.SwitchDomain(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for zero domain_id, got %d", CodeBadRequest, resp.Code)
	}
}

// TestAuthHandler_SSOLogin_Disabled 测试 SSO 未启用时 SSOLogin
func TestAuthHandler_SSOLogin_Disabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &AuthHandler{ssoEnabled: false}

	body := map[string]interface{}{
		"username": "testuser",
		"password": "password",
	}
	bodyBytes, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(bodyBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	h.SSOLogin(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d when SSO disabled, got %d", CodeBadRequest, resp.Code)
	}
}

// TestAuthHandler_SSOLogin_InvalidJSON 测试 SSOLogin 传入非法 JSON
func TestAuthHandler_SSOLogin_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &AuthHandler{ssoEnabled: true}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("not json")))
	c.Request.Header.Set("Content-Type", "application/json")

	h.SSOLogin(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for invalid JSON, got %d", CodeBadRequest, resp.Code)
	}
}

// TestAuthHandler_SSOLogin_EmptyBody 测试 SSOLogin 传入空 body
func TestAuthHandler_SSOLogin_EmptyBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &AuthHandler{ssoEnabled: true}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("{}")))
	c.Request.Header.Set("Content-Type", "application/json")

	h.SSOLogin(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for empty body, got %d", CodeBadRequest, resp.Code)
	}
}

// TestAuthHandler_RefreshToken_InvalidJSON 测试 RefreshToken 传入非法 JSON
func TestAuthHandler_RefreshToken_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &AuthHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("not json")))
	c.Request.Header.Set("Content-Type", "application/json")

	h.RefreshToken(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for invalid JSON, got %d", CodeBadRequest, resp.Code)
	}
}

// TestNewAPITokenHandler 测试 NewAPITokenHandler 构造函数
func TestNewAPITokenHandler(t *testing.T) {
	h := NewAPITokenHandler(nil, nil)
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
	if h.apiTokenSvc != nil {
		t.Errorf("expected nil apiTokenSvc, got %v", h.apiTokenSvc)
	}
	if h.auditSvc != nil {
		t.Errorf("expected nil auditSvc, got %v", h.auditSvc)
	}
}

// TestAPITokenHandler_Generate_ZeroUserID 测试 Generate 当 user_id 为 0
func TestAPITokenHandler_Generate_ZeroUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &APITokenHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	c.Set("user_id", int64(0))

	h.Generate(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for zero user_id, got %d", CodeBadRequest, resp.Code)
	}
}

// TestAPITokenHandler_Generate_NegativeUserID 测试 Generate 当 user_id 为负数
func TestAPITokenHandler_Generate_NegativeUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &APITokenHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	c.Set("user_id", int64(-1))

	h.Generate(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for negative user_id, got %d", CodeBadRequest, resp.Code)
	}
}

// TestAPITokenHandler_GetInfo_ZeroUserID 测试 GetInfo 当 user_id 为 0
func TestAPITokenHandler_GetInfo_ZeroUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &APITokenHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Set("user_id", int64(0))

	h.GetInfo(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for zero user_id, got %d", CodeBadRequest, resp.Code)
	}
}

// TestAPITokenHandler_Reveal_ZeroUserID 测试 Reveal 当 user_id 为 0
func TestAPITokenHandler_Reveal_ZeroUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &APITokenHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	c.Set("user_id", int64(0))

	h.Reveal(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for zero user_id, got %d", CodeBadRequest, resp.Code)
	}
}

// TestAPITokenHandler_Revoke_ZeroUserID 测试 Revoke 当 user_id 为 0
func TestAPITokenHandler_Revoke_ZeroUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &APITokenHandler{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	c.Set("user_id", int64(0))

	h.Revoke(c)

	var resp Response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for zero user_id, got %d", CodeBadRequest, resp.Code)
	}
}
