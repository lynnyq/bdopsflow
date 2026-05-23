package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/lynnyq/bdopsflow/scheduler/internal/service"
)

var jwtSecret = []byte("bdopsflow-secret-key")

type Claims struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	RealName string `json:"real_name"`
	Role     string `json:"role"`
	DomainID int64  `json:"domain_id"`
	jwt.RegisteredClaims
}

func GenerateToken(userID int64, username, realName, role string, domainID int64) (string, error) {
	claims := &Claims{
		UserID:   userID,
		Username: username,
		RealName: realName,
		Role:     role,
		DomainID: domainID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

func ParseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
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
		
		// 首先尝试从 Authorization header 中获取 token
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && parts[0] == "Bearer" {
				tokenString = parts[1]
			}
		}
		
		// 如果 header 中没有 token，尝试从查询参数中获取
		if tokenString == "" {
			tokenString = c.Query("token")
		}
		
		if tokenString == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization required"})
			c.Abort()
			return
		}

		claims, err := ParseToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("real_name", claims.RealName)
		c.Set("role", claims.Role)
		c.Set("domain_id", claims.DomainID)

		c.Next()
	}
}

func RBACMiddleware(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{"error": "Role not found"})
			c.Abort()
			return
		}

		userRole := role.(string)
		
		// 如果检查 'admin'，同时允许 'system_admin'
		// 如果检查 'system_admin'，同时允许 'admin'
		for _, allowed := range allowedRoles {
			if userRole == allowed {
				c.Next()
				return
			}
			if (allowed == "admin" && userRole == "system_admin") || 
			   (allowed == "system_admin" && userRole == "admin") {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
		c.Abort()
	}
}

func RequireSystemAdmin(permissionService *service.PermissionService) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if exists {
			userRole := role.(string)
			if userRole == "system_admin" || userRole == "admin" {
				c.Next()
				return
			}
		}

		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
			c.Abort()
			return
		}

		isAdmin, err := permissionService.IsSystemAdmin(c.Request.Context(), userID.(int64))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check permissions"})
			c.Abort()
			return
		}

		if !isAdmin {
			c.JSON(http.StatusForbidden, gin.H{"error": "System admin access required"})
			c.Abort()
			return
		}

		c.Next()
	}
}

func RequireAdminOrDomainAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
			c.Abort()
			return
		}

		userRole := role.(string)
		if userRole == "system_admin" || userRole == "domain_admin" || userRole == "admin" {
			c.Next()
			return
		}

		c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
		c.Abort()
	}
}
