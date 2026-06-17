package handler

import (
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/service"
)

// extractUserID extracts the user_id from gin context.
func extractUserID(c *gin.Context) int64 {
	userID, _ := c.Get("user_id")
	if v, ok := userID.(int64); ok {
		return v
	}
	return 0
}

// checkOwnership verifies the current user owns the resource or is a system admin.
// Returns true if ownership is valid. Sends Forbidden/Unauthorized and returns false otherwise.
func checkOwnership(c *gin.Context, permSvc *service.PermissionService, createdBy int64) bool {
	userID := extractUserID(c)
	if userID == 0 {
		Unauthorized(c, "用户未登录")
		return false
	}
	if createdBy == userID {
		return true
	}
	isAdmin, err := permSvc.IsSystemAdmin(c.Request.Context(), userID)
	if err != nil {
		slog.Warn("failed to check system admin status", "user_id", userID, "error", err)
		Forbidden(c, "无权访问该资源")
		return false
	}
	if isAdmin {
		return true
	}
	Forbidden(c, "无权访问该资源")
	return false
}

// parseIDParam parses an ID parameter from the URL.
func parseIDParam(c *gin.Context, paramName string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(paramName), 10, 64)
	if err != nil {
		BadRequest(c, "无效的ID参数")
		return 0, false
	}
	return id, true
}
