package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/lynnyq/bdopsflow/scheduler/internal/service"
)

type JWTConfig struct {
	Secret             []byte
	ExpiryHours        int
	RefreshSecret      []byte
	RefreshExpiryHours int
}

var jwtConfig JWTConfig

func InitJWT(secret string, expiryHours int, refreshExpiryHours ...int) {
	refreshHours := 168
	if len(refreshExpiryHours) > 0 && refreshExpiryHours[0] > 0 {
		refreshHours = refreshExpiryHours[0]
	}
	jwtConfig = JWTConfig{
		Secret:             []byte(secret),
		ExpiryHours:        expiryHours,
		RefreshSecret:      []byte(secret + "_refresh"),
		RefreshExpiryHours: refreshHours,
	}
	if jwtConfig.ExpiryHours <= 0 {
		jwtConfig.ExpiryHours = 2
	}
}

func GetJWTConfig() *JWTConfig {
	return &jwtConfig
}

func SetRefreshExpiryHours(hours int) {
	jwtConfig.RefreshExpiryHours = hours
}

type Claims struct {
	UserID          int64  `json:"user_id"`
	Username        string `json:"username"`
	RealName        string `json:"real_name"`
	CurrentDomainID int64  `json:"current_domain_id"`
	jwt.RegisteredClaims
}

func GenerateToken(userID int64, username, realName string, currentDomainID int64) (string, error) {
	claims := &Claims{
		UserID:          userID,
		Username:        username,
		RealName:        realName,
		CurrentDomainID: currentDomainID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(jwtConfig.ExpiryHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "bdopsflow",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtConfig.Secret)
}

func ParseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return jwtConfig.Secret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, jwt.ErrSignatureInvalid
}

func GenerateRefreshToken(userID int64, username, realName string, currentDomainID int64) (string, error) {
	claims := &Claims{
		UserID:          userID,
		Username:        username,
		RealName:        realName,
		CurrentDomainID: currentDomainID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(jwtConfig.RefreshExpiryHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "bdopsflow-refresh",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtConfig.RefreshSecret)
}

func ParseRefreshToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return jwtConfig.RefreshSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, jwt.ErrSignatureInvalid
}

func JWTAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var tokenString string

		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && parts[0] == "Bearer" {
				tokenString = parts[1]
				slog.Debug("token extracted from Authorization header", "module", "middleware_auth", "path", c.Request.URL.Path)
			}
		}

		if tokenString == "" {
			tokenString = c.Query("token")
			if tokenString != "" {
				slog.Debug("token extracted from query parameter", "module", "middleware_auth", "path", c.Request.URL.Path)
			}
		}

		if tokenString == "" {
			slog.Warn("no token found in request", "module", "middleware_auth", "path", c.Request.URL.Path)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization required"})
			c.Abort()
			return
		}

		claims, err := ParseToken(tokenString)
		if err != nil {
			slog.Warn("token is invalid or expired", "module", "middleware_auth", "path", c.Request.URL.Path, "error", err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("real_name", claims.RealName)
		c.Set("current_domain_id", claims.CurrentDomainID)

		c.Next()
	}
}

type PermissionChecker interface {
	IsSystemAdmin(ctx context.Context, userID int64) (bool, error)
	HasPermission(ctx context.Context, userID int64, resource, action string, domainID int64) (bool, error)
}

type RoleInjector interface {
	GetUserRoleCodes(ctx context.Context, userID int64) ([]string, error)
}

func InjectUserRole(roleSvc RoleInjector) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.Next()
			return
		}

		uid, ok := userID.(int64)
		if !ok || uid <= 0 {
			c.Next()
			return
		}

		roleCodes, err := roleSvc.GetUserRoleCodes(c.Request.Context(), uid)
		if err != nil {
			slog.Warn("failed to get user role codes", "module", "middleware_auth", "user_id", uid, "error", err)
			c.Next()
			return
		}

		if len(roleCodes) == 0 {
			slog.Warn("user has no roles, possibly deleted from database", "module", "middleware_auth", "user_id", uid)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			c.Abort()
			return
		}

		role := "user"
		for _, code := range roleCodes {
			if code == "system_admin" {
				role = "system_admin"
				break
			}
			if code == "domain_admin" && role != "system_admin" {
				role = "domain_admin"
			}
		}

		c.Set("role", role)
		slog.Debug("user role injected", "module", "middleware_auth", "user_id", uid, "role", role)
		c.Next()
	}
}

func RequireSystemAdmin(permSvc PermissionChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
			c.Abort()
			return
		}

		slog.Debug("checking system admin permission", "module", "middleware_auth", "user_id", userID.(int64))

		isAdmin, err := permSvc.IsSystemAdmin(c.Request.Context(), userID.(int64))
		if err != nil {
			slog.Error("failed to check system admin permission", "module", "middleware_auth", "user_id", userID.(int64), "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check permissions"})
			c.Abort()
			return
		}

		if !isAdmin {
			slog.Warn("user is not system admin", "module", "middleware_auth", "user_id", userID.(int64))
			c.JSON(http.StatusForbidden, gin.H{"error": "System admin access required"})
			c.Abort()
			return
		}

		c.Next()
	}
}

func RequirePermission(permSvc PermissionChecker, resource, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
			c.Abort()
			return
		}

		domainID, _ := c.Get("current_domain_id")
		var dID int64
		if v, ok := domainID.(int64); ok {
			dID = v
		}

		slog.Debug("checking permission", "module", "middleware_auth", "user_id", userID.(int64), "resource", resource, "action", action, "domain_id", dID)

		ok, err := permSvc.HasPermission(c.Request.Context(), userID.(int64), resource, action, dID)
		if err != nil {
			slog.Error("failed to check permission", "module", "middleware_auth", "user_id", userID.(int64), "resource", resource, "action", action, "domain_id", dID, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check permissions"})
			c.Abort()
			return
		}
		if !ok {
			slog.Warn("permission denied", "module", "middleware_auth", "user_id", userID.(int64), "resource", resource, "action", action, "domain_id", dID)
			c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func RequireInstancePermission(instancePermSvc *service.InstancePermissionService, resourceType string, getID func(*gin.Context) int64, permissionType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
			c.Abort()
			return
		}

		instanceID := getID(c)
		if instanceID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid resource ID"})
			c.Abort()
			return
		}

		slog.Debug("checking instance permission", "module", "middleware_auth", "user_id", userID.(int64), "resource_type", resourceType, "instance_id", instanceID, "permission_type", permissionType)

		var ok bool
		var err error
		if resourceType == "datasource" {
			ok, err = instancePermSvc.HasDatasourcePermission(c.Request.Context(), userID.(int64), instanceID, permissionType)
		} else if resourceType == "webhook" {
			ok, err = instancePermSvc.HasWebhookPermission(c.Request.Context(), userID.(int64), instanceID, permissionType)
		}

		if err != nil {
			slog.Error("failed to check instance permission", "module", "middleware_auth", "user_id", userID.(int64), "resource_type", resourceType, "instance_id", instanceID, "permission_type", permissionType, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check permissions"})
			c.Abort()
			return
		}
		if !ok {
			slog.Warn("instance permission denied", "module", "middleware_auth", "user_id", userID.(int64), "resource_type", resourceType, "instance_id", instanceID, "permission_type", permissionType)
			c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
			c.Abort()
			return
		}
		c.Next()
	}
}

type DatasourcePermissionChecker interface {
	HasDatasourcePermission(ctx context.Context, userID int64, datasourceID int64, permissionType string) (bool, error)
}

func DatasourcePermissionMiddleware(instancePermSvc DatasourcePermissionChecker, permissionType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
			c.Abort()
			return
		}

		var datasourceID int64
		idStr := c.Param("id")
		if idStr != "" {
			datasourceID = parseInt64(idStr)
		}
		if datasourceID == 0 {
			datasourceID = parseDatasourceIDFromBody(c)
		}
		if datasourceID == 0 {
			if c.Request.Method == http.MethodPost || c.Request.Method == http.MethodPut || c.Request.Method == http.MethodPatch {
				c.JSON(http.StatusBadRequest, gin.H{"error": "datasource_id is required"})
				c.Abort()
				return
			}
			c.Next()
			return
		}

		slog.Debug("checking datasource permission", "module", "middleware_auth", "user_id", userID.(int64), "datasource_id", datasourceID, "permission_type", permissionType)

		ok, err := instancePermSvc.HasDatasourcePermission(c.Request.Context(), userID.(int64), datasourceID, permissionType)
		if err != nil {
			slog.Error("failed to check datasource permission", "module", "middleware_auth", "user_id", userID.(int64), "datasource_id", datasourceID, "permission_type", permissionType, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check permissions"})
			c.Abort()
			return
		}
		if !ok {
			slog.Warn("datasource permission denied", "module", "middleware_auth", "user_id", userID.(int64), "datasource_id", datasourceID, "permission_type", permissionType)
			c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func parseDatasourceIDFromBody(c *gin.Context) int64 {
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return 0
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	var body struct {
		DatasourceID int64 `json:"datasource_id"`
	}
	if json.Unmarshal(bodyBytes, &body) == nil {
		return body.DatasourceID
	}
	return 0
}

func parseInt64(s string) int64 {
	var n int64
	fmt.Sscanf(s, "%d", &n)
	return n
}
