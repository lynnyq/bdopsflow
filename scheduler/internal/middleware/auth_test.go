package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGenerateToken(t *testing.T) {
	token, err := GenerateToken(1, "testuser", "admin", 1)
	if err != nil {
		t.Errorf("failed to generate token: %v", err)
	}

	if token == "" {
		t.Error("expected non-empty token")
	}
}

func TestParseToken(t *testing.T) {
	token, err := GenerateToken(1, "testuser", "admin", 1)
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

	if claims.Role != "admin" {
		t.Errorf("expected role 'admin', got '%s'", claims.Role)
	}

	if claims.DomainID != 1 {
		t.Errorf("expected domain_id 1, got %d", claims.DomainID)
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
	var capturedRole string
	var capturedDomainID int64

	router.GET("/test", func(c *gin.Context) {
		userID, _ := c.Get("user_id")
		username, _ := c.Get("username")
		role, _ := c.Get("role")
		domainID, _ := c.Get("domain_id")

		capturedUserID = userID.(int64)
		capturedUsername = username.(string)
		capturedRole = role.(string)
		capturedDomainID = domainID.(int64)

		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	token, err := GenerateToken(42, "testuser", "admin", 10)
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

	if capturedRole != "admin" {
		t.Errorf("expected role 'admin', got '%s'", capturedRole)
	}

	if capturedDomainID != 10 {
		t.Errorf("expected domain_id 10, got %d", capturedDomainID)
	}
}

func TestRBACMiddleware_AllowedRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})

	router.Use(RBACMiddleware("admin", "operator"))
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

func TestRBACMiddleware_DeniedRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.Use(func(c *gin.Context) {
		c.Set("role", "viewer")
		c.Next()
	})

	router.Use(RBACMiddleware("admin", "operator"))
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

func TestRBACMiddleware_NoRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	router.Use(RBACMiddleware("admin"))
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
