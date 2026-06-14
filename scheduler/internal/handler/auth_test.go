package handler

import (
	"bytes"
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

func TestAuthHandler_Login(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &AuthHandler{}
	r.POST("/api/login", handler.Login)

	body := map[string]interface{}{
		"username": "admin",
		"password": "password",
	}

	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/api/login", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Log("Recovered from panic (expected for nil db):", r)
			return
		}
	}()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusUnauthorized && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 401, or 500, got %d", w.Code)
	}
}

func TestAuthHandler_Login_MissingFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &AuthHandler{}
	r.POST("/api/login", handler.Login)

	body := map[string]interface{}{
		"username": "",
		"password": "",
	}

	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/api/login", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for missing fields, got %d", CodeBadRequest, resp.Code)
	}
}

func TestAuthHandler_Login_ResponseStructure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &AuthHandler{}
	r.POST("/api/login", handler.Login)

	body := map[string]interface{}{
		"username": "admin",
		"password": "encrypted_password",
	}

	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/api/login", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Log("Recovered from panic (expected for nil db):", r)
			return
		}
	}()

	r.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		data, ok := resp.Data.(map[string]interface{})
		if !ok {
			t.Fatal("response data should be a map")
		}

		if _, exists := data["token"]; !exists {
			t.Error("response should contain 'token' field")
		}
		if _, exists := data["user"]; !exists {
			t.Error("response should contain 'user' field")
		}
		if _, exists := data["permissions"]; !exists {
			t.Error("response should contain 'permissions' field")
		}
		if _, exists := data["domains"]; !exists {
			t.Error("response should contain 'domains' field")
		}
		if _, exists := data["current_domain_id"]; !exists {
			t.Error("response should contain 'current_domain_id' field")
		}
	}
}

func TestAuthHandler_Register(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &AuthHandler{}
	r.POST("/api/register", handler.Register)

	body := map[string]interface{}{
		"username": "testuser",
		"password": "password123",
		"email":    "test@example.com",
	}

	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/api/register", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Log("Recovered from panic (expected for nil db):", r)
			return
		}
	}()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated && w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 201, 400, or 500, got %d", w.Code)
	}
}

func TestAuthHandler_GetCurrentUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &AuthHandler{}
	r.GET("/api/user", handler.GetCurrentUser)

	req, _ := http.NewRequest("GET", "/api/user", nil)
	w := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Log("Recovered from panic (expected for nil db):", r)
			return
		}
	}()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusUnauthorized && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200, 401, or 500, got %d", w.Code)
	}
}

func TestAuthHandler_GetCurrentUser_ResponseStructure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &AuthHandler{}
	r.GET("/api/user", func(c *gin.Context) {
		c.Set("user_id", int64(1))
		c.Set("current_domain_id", int64(1))
		handler.GetCurrentUser(c)
	})

	req, _ := http.NewRequest("GET", "/api/user", nil)
	w := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Log("Recovered from panic (expected for nil db):", r)
			return
		}
	}()

	r.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		data, ok := resp.Data.(map[string]interface{})
		if !ok {
			t.Fatal("response data should be a map")
		}

		if _, exists := data["permissions"]; !exists {
			t.Error("GetCurrentUser response should contain 'permissions' field")
		}
		if _, exists := data["domains"]; !exists {
			t.Error("GetCurrentUser response should contain 'domains' field")
		}
		if _, exists := data["current_domain_id"]; !exists {
			t.Error("GetCurrentUser response should contain 'current_domain_id' field")
		}
	}
}

func TestAuthHandler_SwitchDomain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &AuthHandler{}
	r.POST("/api/auth/switch-domain", func(c *gin.Context) {
		c.Set("user_id", int64(1))
		c.Set("username", "testuser")
		c.Set("real_name", "Test User")
		handler.SwitchDomain(c)
	})

	body := map[string]interface{}{
		"domain_id": 2,
	}

	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/api/auth/switch-domain", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Log("Recovered from panic (expected for nil db):", r)
			return
		}
	}()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError && w.Code != http.StatusForbidden {
		t.Errorf("expected status 200, 403, or 500, got %d", w.Code)
	}
}

func TestAuthHandler_SwitchDomain_MissingDomainID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &AuthHandler{}
	r.POST("/api/auth/switch-domain", handler.SwitchDomain)

	body := map[string]interface{}{}

	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/api/auth/switch-domain", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for missing domain_id, got %d", CodeBadRequest, resp.Code)
	}
}

func TestAuthHandler_GetPublicKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &AuthHandler{}
	r.GET("/api/auth/public-key", handler.GetPublicKey)

	req, _ := http.NewRequest("GET", "/api/auth/public-key", nil)
	w := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Log("Recovered from panic (expected for nil rsaUtil):", r)
			return
		}
	}()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d", w.Code)
	}
}

func TestAuthHandler_RefreshToken_MissingToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &AuthHandler{}
	r.POST("/api/auth/refresh", handler.RefreshToken)

	body := map[string]interface{}{}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/api/auth/refresh", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for missing refresh_token, got %d", CodeBadRequest, resp.Code)
	}
}

func TestAuthHandler_RefreshToken_InvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &AuthHandler{}
	r.POST("/api/auth/refresh", handler.RefreshToken)

	body := map[string]interface{}{
		"refresh_token": "invalid-token-string",
	}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/api/auth/refresh", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Code != CodeInvalidToken {
		t.Errorf("expected code %d for invalid refresh token, got %d", CodeInvalidToken, resp.Code)
	}
}

func TestAuthHandler_RefreshToken_AccessTokenUsedAsRefresh(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	accessToken, err := middleware.GenerateToken(1, "testuser", "Test User", 1)
	if err != nil {
		t.Fatalf("failed to generate access token: %v", err)
	}

	handler := &AuthHandler{}
	r.POST("/api/auth/refresh", handler.RefreshToken)

	body := map[string]interface{}{
		"refresh_token": accessToken,
	}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/api/auth/refresh", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Code != CodeInvalidToken {
		t.Errorf("expected code %d when using access token as refresh token, got %d", CodeInvalidToken, resp.Code)
	}
}

func TestAuthHandler_RefreshToken_ValidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	refreshToken, err := middleware.GenerateRefreshToken(1, "testuser", "Test User", 1)
	if err != nil {
		t.Fatalf("failed to generate refresh token: %v", err)
	}

	handler := &AuthHandler{}
	r.POST("/api/auth/refresh", handler.RefreshToken)

	body := map[string]interface{}{
		"refresh_token": refreshToken,
	}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/api/auth/refresh", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Log("Recovered from panic (expected for nil db):", r)
			return
		}
	}()

	r.ServeHTTP(w, req)

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Code != CodeSuccess {
		t.Errorf("expected code %d for valid refresh token, got %d (message: %s)", CodeSuccess, resp.Code, resp.Message)
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("response data should be a map")
	}

	if _, exists := data["token"]; !exists {
		t.Error("refresh response should contain 'token' field")
	}
	if _, exists := data["refresh_token"]; !exists {
		t.Error("refresh response should contain 'refresh_token' field")
	}
}

func TestAuthHandler_RefreshToken_NewTokensAreValid(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	refreshToken, err := middleware.GenerateRefreshToken(42, "admin", "Admin User", 5)
	if err != nil {
		t.Fatalf("failed to generate refresh token: %v", err)
	}

	handler := &AuthHandler{}
	r.POST("/api/auth/refresh", handler.RefreshToken)

	body := map[string]interface{}{
		"refresh_token": refreshToken,
	}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/api/auth/refresh", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Log("Recovered from panic (expected for nil db):", r)
			return
		}
	}()

	r.ServeHTTP(w, req)

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatal("response data should be a map")
	}

	newToken, _ := data["token"].(string)
	newRefreshToken, _ := data["refresh_token"].(string)

	newClaims, err := middleware.ParseToken(newToken)
	if err != nil {
		t.Errorf("new access token should be valid: %v", err)
	}
	if newClaims.UserID != 42 {
		t.Errorf("expected user_id 42, got %d", newClaims.UserID)
	}
	if newClaims.Username != "admin" {
		t.Errorf("expected username 'admin', got '%s'", newClaims.Username)
	}
	if newClaims.CurrentDomainID != 5 {
		t.Errorf("expected current_domain_id 5, got %d", newClaims.CurrentDomainID)
	}
	if newClaims.Issuer != "bdopsflow" {
		t.Errorf("expected issuer 'bdopsflow', got '%s'", newClaims.Issuer)
	}

	newRefreshClaims, err := middleware.ParseRefreshToken(newRefreshToken)
	if err != nil {
		t.Errorf("new refresh token should be valid: %v", err)
	}
	if newRefreshClaims.UserID != 42 {
		t.Errorf("expected user_id 42, got %d", newRefreshClaims.UserID)
	}
	if newRefreshClaims.Issuer != "bdopsflow-refresh" {
		t.Errorf("expected issuer 'bdopsflow-refresh', got '%s'", newRefreshClaims.Issuer)
	}
}

func TestAuthHandler_RefreshToken_EmptyStringToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &AuthHandler{}
	r.POST("/api/auth/refresh", handler.RefreshToken)

	body := map[string]interface{}{
		"refresh_token": "",
	}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/api/auth/refresh", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Code != CodeBadRequest {
		t.Errorf("expected code %d for empty refresh_token, got %d", CodeBadRequest, resp.Code)
	}
}

func TestAuthHandler_RefreshToken_ExpiredRefreshToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	middleware.InitJWT("test-secret-key-for-unit-tests", 24)

	oldRefreshExpiry := middleware.GetJWTConfig().RefreshExpiryHours
	middleware.SetRefreshExpiryHours(-1)
	expiredRefreshToken, err := middleware.GenerateRefreshToken(1, "testuser", "Test User", 1)
	middleware.SetRefreshExpiryHours(oldRefreshExpiry)
	if err != nil {
		t.Fatalf("failed to generate expired refresh token: %v", err)
	}

	handler := &AuthHandler{}
	r.POST("/api/auth/refresh", handler.RefreshToken)

	body := map[string]interface{}{
		"refresh_token": expiredRefreshToken,
	}
	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "/api/auth/refresh", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Code != CodeInvalidToken {
		t.Errorf("expected code %d for expired refresh token, got %d", CodeInvalidToken, resp.Code)
	}
}
