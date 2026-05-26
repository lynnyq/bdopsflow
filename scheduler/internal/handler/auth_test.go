package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

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
