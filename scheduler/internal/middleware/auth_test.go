package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	InitJWT("test-secret-key-for-unit-tests", 24)
}

type mockPermissionChecker struct {
	isSystemAdminFn func(ctx context.Context, userID int64) (bool, error)
	hasPermissionFn func(ctx context.Context, userID int64, resource, action string, domainID int64) (bool, error)
}

func (m *mockPermissionChecker) IsSystemAdmin(ctx context.Context, userID int64) (bool, error) {
	return m.isSystemAdminFn(ctx, userID)
}

func (m *mockPermissionChecker) HasPermission(ctx context.Context, userID int64, resource, action string, domainID int64) (bool, error) {
	return m.hasPermissionFn(ctx, userID, resource, action, domainID)
}

func TestGenerateToken(t *testing.T) {
	token, err := GenerateToken(1, "testuser", "Test User", 1)
	if err != nil {
		t.Errorf("failed to generate token: %v", err)
	}

	if token == "" {
		t.Error("expected non-empty token")
	}
}

func TestParseToken(t *testing.T) {
	token, err := GenerateToken(1, "testuser", "Test User", 1)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	claims, err := ParseToken(token)
	if err != nil {
		t.Errorf("failed to parse token: %v", err)
	}

	if claims.Username != "testuser" {
		t.Errorf("expected username 'testuser', got '%s'", claims.Username)
	}

	if claims.RealName != "Test User" {
		t.Errorf("expected real_name 'Test User', got '%s'", claims.RealName)
	}

	if claims.CurrentDomainID != 1 {
		t.Errorf("expected current_domain_id 1, got %d", claims.CurrentDomainID)
	}
}

func TestParseInvalidToken(t *testing.T) {
	_, err := ParseToken("invalid-token")
	if err == nil {
		t.Error("expected error for invalid token")
	}
}

func TestJWTAuthMiddleware_NoHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(JWTAuthMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestJWTAuthMiddleware_InvalidFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(JWTAuthMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "InvalidFormat token123")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestJWTAuthMiddleware_ValidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(JWTAuthMiddleware())

	var capturedUserID int64
	var capturedUsername string
	var capturedRealName string
	var capturedCurrentDomainID int64

	router.GET("/test", func(c *gin.Context) {
		userID, _ := c.Get("user_id")
		username, _ := c.Get("username")
		realName, _ := c.Get("real_name")
		currentDomainID, _ := c.Get("current_domain_id")

		capturedUserID = userID.(int64)
		capturedUsername = username.(string)
		capturedRealName = realName.(string)
		capturedCurrentDomainID = currentDomainID.(int64)

		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	token, err := GenerateToken(42, "testuser", "Test User", 10)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if capturedUserID != 42 {
		t.Errorf("expected user_id 42, got %d", capturedUserID)
	}

	if capturedUsername != "testuser" {
		t.Errorf("expected username 'testuser', got '%s'", capturedUsername)
	}

	if capturedRealName != "Test User" {
		t.Errorf("expected real_name 'Test User', got '%s'", capturedRealName)
	}

	if capturedCurrentDomainID != 10 {
		t.Errorf("expected current_domain_id 10, got %d", capturedCurrentDomainID)
	}
}

func TestRequireSystemAdmin_NoUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	permSvc := &mockPermissionChecker{
		isSystemAdminFn: func(ctx context.Context, userID int64) (bool, error) {
			t.Error("IsSystemAdmin should not be called without user_id")
			return false, nil
		},
	}

	router := gin.New()
	router.Use(RequireSystemAdmin(permSvc))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestRequireSystemAdmin_IsAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	permSvc := &mockPermissionChecker{
		isSystemAdminFn: func(ctx context.Context, userID int64) (bool, error) {
			return true, nil
		},
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", int64(1))
		c.Next()
	})
	router.Use(RequireSystemAdmin(permSvc))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestRequireSystemAdmin_NotAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	permSvc := &mockPermissionChecker{
		isSystemAdminFn: func(ctx context.Context, userID int64) (bool, error) {
			return false, nil
		},
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", int64(2))
		c.Next()
	})
	router.Use(RequireSystemAdmin(permSvc))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}
}

func TestRequireSystemAdmin_CheckError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	permSvc := &mockPermissionChecker{
		isSystemAdminFn: func(ctx context.Context, userID int64) (bool, error) {
			return false, context.DeadlineExceeded
		},
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", int64(1))
		c.Next()
	})
	router.Use(RequireSystemAdmin(permSvc))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

func TestRequirePermission_NoUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	permSvc := &mockPermissionChecker{
		hasPermissionFn: func(ctx context.Context, userID int64, resource, action string, domainID int64) (bool, error) {
			t.Error("HasPermission should not be called without user_id")
			return false, nil
		},
	}

	router := gin.New()
	router.Use(RequirePermission(permSvc, "task", "create"))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestRequirePermission_HasPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var capturedUserID int64
	var capturedResource string
	var capturedAction string
	var capturedDomainID int64

	permSvc := &mockPermissionChecker{
		hasPermissionFn: func(ctx context.Context, userID int64, resource, action string, domainID int64) (bool, error) {
			capturedUserID = userID
			capturedResource = resource
			capturedAction = action
			capturedDomainID = domainID
			return true, nil
		},
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", int64(1))
		c.Set("current_domain_id", int64(5))
		c.Next()
	})
	router.Use(RequirePermission(permSvc, "task", "create"))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if capturedUserID != 1 {
		t.Errorf("expected userID 1, got %d", capturedUserID)
	}
	if capturedResource != "task" {
		t.Errorf("expected resource 'task', got '%s'", capturedResource)
	}
	if capturedAction != "create" {
		t.Errorf("expected action 'create', got '%s'", capturedAction)
	}
	if capturedDomainID != 5 {
		t.Errorf("expected domainID 5, got %d", capturedDomainID)
	}
}

func TestRequirePermission_NoPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	permSvc := &mockPermissionChecker{
		hasPermissionFn: func(ctx context.Context, userID int64, resource, action string, domainID int64) (bool, error) {
			return false, nil
		},
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", int64(1))
		c.Set("current_domain_id", int64(1))
		c.Next()
	})
	router.Use(RequirePermission(permSvc, "task", "delete"))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}
}

func TestRequirePermission_CheckError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	permSvc := &mockPermissionChecker{
		hasPermissionFn: func(ctx context.Context, userID int64, resource, action string, domainID int64) (bool, error) {
			return false, context.DeadlineExceeded
		},
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", int64(1))
		c.Set("current_domain_id", int64(1))
		c.Next()
	})
	router.Use(RequirePermission(permSvc, "task", "create"))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

func TestRequirePermission_NoDomainID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var capturedDomainID int64

	permSvc := &mockPermissionChecker{
		hasPermissionFn: func(ctx context.Context, userID int64, resource, action string, domainID int64) (bool, error) {
			capturedDomainID = domainID
			return true, nil
		},
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", int64(1))
		c.Next()
	})
	router.Use(RequirePermission(permSvc, "task", "create"))
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if capturedDomainID != 0 {
		t.Errorf("expected domainID 0 when not set, got %d", capturedDomainID)
	}
}

func TestRequirePermission_AbortPreventsNextHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	nextCalled := false
	permSvc := &mockPermissionChecker{
		hasPermissionFn: func(ctx context.Context, userID int64, resource, action string, domainID int64) (bool, error) {
			return false, nil
		},
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", int64(1))
		c.Set("current_domain_id", int64(1))
		c.Next()
	})
	router.Use(RequirePermission(permSvc, "task", "create"))
	router.GET("/test", func(c *gin.Context) {
		nextCalled = true
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if nextCalled {
		t.Error("next handler should not be called after abort")
	}
}

func TestGenerateRefreshToken(t *testing.T) {
	token, err := GenerateRefreshToken(1, "testuser", "Test User", 1)
	if err != nil {
		t.Errorf("failed to generate refresh token: %v", err)
	}
	if token == "" {
		t.Error("expected non-empty refresh token")
	}
}

func TestParseRefreshToken(t *testing.T) {
	token, err := GenerateRefreshToken(1, "testuser", "Test User", 1)
	if err != nil {
		t.Fatalf("failed to generate refresh token: %v", err)
	}

	claims, err := ParseRefreshToken(token)
	if err != nil {
		t.Errorf("failed to parse refresh token: %v", err)
	}

	if claims.UserID != 1 {
		t.Errorf("expected user_id 1, got %d", claims.UserID)
	}
	if claims.Username != "testuser" {
		t.Errorf("expected username 'testuser', got '%s'", claims.Username)
	}
	if claims.RealName != "Test User" {
		t.Errorf("expected real_name 'Test User', got '%s'", claims.RealName)
	}
	if claims.CurrentDomainID != 1 {
		t.Errorf("expected current_domain_id 1, got %d", claims.CurrentDomainID)
	}
	if claims.Issuer != "bdopsflow-refresh" {
		t.Errorf("expected issuer 'bdopsflow-refresh', got '%s'", claims.Issuer)
	}
}

func TestParseRefreshToken_InvalidToken(t *testing.T) {
	_, err := ParseRefreshToken("invalid-refresh-token")
	if err == nil {
		t.Error("expected error for invalid refresh token")
	}
}

func TestParseRefreshToken_ExpiredToken(t *testing.T) {
	oldExpiry := jwtConfig.RefreshExpiryHours
	jwtConfig.RefreshExpiryHours = -1
	defer func() { jwtConfig.RefreshExpiryHours = oldExpiry }()

	token, err := GenerateRefreshToken(1, "testuser", "Test User", 1)
	if err != nil {
		t.Fatalf("failed to generate expired refresh token: %v", err)
	}

	_, err = ParseRefreshToken(token)
	if err == nil {
		t.Error("expected error for expired refresh token")
	}
}

func TestParseRefreshToken_WrongSecret(t *testing.T) {
	token, err := GenerateToken(1, "testuser", "Test User", 1)
	if err != nil {
		t.Fatalf("failed to generate access token: %v", err)
	}

	_, err = ParseRefreshToken(token)
	if err == nil {
		t.Error("expected error when parsing access token as refresh token")
	}
}

func TestRefreshToken_DifferentSecretFromAccessToken(t *testing.T) {
	accessToken, err := GenerateToken(1, "testuser", "Test User", 1)
	if err != nil {
		t.Fatalf("failed to generate access token: %v", err)
	}

	refreshToken, err := GenerateRefreshToken(1, "testuser", "Test User", 1)
	if err != nil {
		t.Fatalf("failed to generate refresh token: %v", err)
	}

	_, err = ParseToken(refreshToken)
	if err == nil {
		t.Error("access token parser should reject refresh token")
	}

	_, err = ParseRefreshToken(accessToken)
	if err == nil {
		t.Error("refresh token parser should reject access token")
	}
}

func TestRefreshToken_IssuerIsDifferent(t *testing.T) {
	accessToken, _ := GenerateToken(1, "testuser", "Test User", 1)
	refreshToken, _ := GenerateRefreshToken(1, "testuser", "Test User", 1)

	accessClaims, _ := ParseToken(accessToken)
	refreshClaims, _ := ParseRefreshToken(refreshToken)

	if accessClaims.Issuer == refreshClaims.Issuer {
		t.Errorf("access token and refresh token should have different issuers, got same: '%s'", accessClaims.Issuer)
	}

	if accessClaims.Issuer != "bdopsflow" {
		t.Errorf("expected access token issuer 'bdopsflow', got '%s'", accessClaims.Issuer)
	}
	if refreshClaims.Issuer != "bdopsflow-refresh" {
		t.Errorf("expected refresh token issuer 'bdopsflow-refresh', got '%s'", refreshClaims.Issuer)
	}
}

func TestRefreshToken_ContainsCorrectClaims(t *testing.T) {
	token, err := GenerateRefreshToken(42, "admin", "Admin User", 5)
	if err != nil {
		t.Fatalf("failed to generate refresh token: %v", err)
	}

	claims, err := ParseRefreshToken(token)
	if err != nil {
		t.Fatalf("failed to parse refresh token: %v", err)
	}

	if claims.UserID != 42 {
		t.Errorf("expected user_id 42, got %d", claims.UserID)
	}
	if claims.Username != "admin" {
		t.Errorf("expected username 'admin', got '%s'", claims.Username)
	}
	if claims.RealName != "Admin User" {
		t.Errorf("expected real_name 'Admin User', got '%s'", claims.RealName)
	}
	if claims.CurrentDomainID != 5 {
		t.Errorf("expected current_domain_id 5, got %d", claims.CurrentDomainID)
	}
}

func TestRefreshToken_ZeroDomainID(t *testing.T) {
	token, err := GenerateRefreshToken(1, "testuser", "Test User", 0)
	if err != nil {
		t.Fatalf("failed to generate refresh token: %v", err)
	}

	claims, err := ParseRefreshToken(token)
	if err != nil {
		t.Fatalf("failed to parse refresh token: %v", err)
	}

	if claims.CurrentDomainID != 0 {
		t.Errorf("expected current_domain_id 0, got %d", claims.CurrentDomainID)
	}
}

func TestInitJWT_DefaultExpiry(t *testing.T) {
	InitJWT("test-secret", 0)
	if jwtConfig.ExpiryHours != 2 {
		t.Errorf("expected default expiry 2, got %d", jwtConfig.ExpiryHours)
	}
	if jwtConfig.RefreshExpiryHours != 168 {
		t.Errorf("expected default refresh expiry 168, got %d", jwtConfig.RefreshExpiryHours)
	}
	if string(jwtConfig.RefreshSecret) != "test-secret_refresh" {
		t.Errorf("expected refresh secret 'test-secret_refresh', got '%s'", string(jwtConfig.RefreshSecret))
	}

	InitJWT("test-secret-key-for-unit-tests", 24)
}

func TestInitJWT_CustomRefreshExpiry(t *testing.T) {
	InitJWT("test-secret", 24, 72)
	if jwtConfig.ExpiryHours != 24 {
		t.Errorf("expected expiry 24, got %d", jwtConfig.ExpiryHours)
	}
	if jwtConfig.RefreshExpiryHours != 72 {
		t.Errorf("expected refresh expiry 72, got %d", jwtConfig.RefreshExpiryHours)
	}

	InitJWT("test-secret-key-for-unit-tests", 24)
}

func TestInitJWT_ZeroRefreshExpiryFallsBackToDefault(t *testing.T) {
	InitJWT("test-secret", 24, 0)
	if jwtConfig.RefreshExpiryHours != 168 {
		t.Errorf("expected default refresh expiry 168 when 0 provided, got %d", jwtConfig.RefreshExpiryHours)
	}

	InitJWT("test-secret-key-for-unit-tests", 24)
}
