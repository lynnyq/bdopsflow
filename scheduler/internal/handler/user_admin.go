package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"github.com/lynnyq/bdopsflow/scheduler/internal/service"
)

// UserAdminHandler 用户管理 Handler
type UserAdminHandler struct {
	svc *service.UserAdminService
}

// NewUserAdminHandler 创建用户管理 Handler
func NewUserAdminHandler(svc *service.UserAdminService) *UserAdminHandler {
	return &UserAdminHandler{svc: svc}
}

// ListUsers 获取用户列表
func (h *UserAdminHandler) ListUsers(c *gin.Context) {
	ctx := c.Request.Context()
	defer func() {
		if r := recover(); r != nil {
			slog.Error("UserAdminHandler.ListUsers: panic recovered", "panic", r)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
	}()

	slog.Debug("UserAdminHandler.ListUsers: handling request")

	users, err := h.svc.ListUsers(ctx)
	if err != nil {
		slog.Error("UserAdminHandler.ListUsers: failed to list users", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": users})
}

// GetUser 获取用户详情
func (h *UserAdminHandler) GetUser(c *gin.Context) {
	ctx := c.Request.Context()
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		slog.Warn("UserAdminHandler.GetUser: invalid id", "id_str", idStr, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if id <= 0 {
		slog.Warn("UserAdminHandler.GetUser: id must be positive", "id", id)
		c.JSON(http.StatusBadRequest, gin.H{"error": "id must be positive"})
		return
	}

	slog.Debug("UserAdminHandler.GetUser: handling request", "user_id", id)

	user, err := h.svc.GetUserByID(ctx, id)
	if err != nil {
		slog.Error("UserAdminHandler.GetUser: failed to get user", "user_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	// 获取用户角色
	roles, err := h.svc.GetUserRoles(ctx, id)
	if err != nil {
		slog.Error("UserAdminHandler.GetUser: failed to get user roles", "user_id", id, "error", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"user":  user,
		"roles": roles,
	})
}

// CreateUser 创建用户
func (h *UserAdminHandler) CreateUser(c *gin.Context) {
	ctx := c.Request.Context()

	var req struct {
		Username string `json:"username" binding:"required,min=3,max=50,alphanum"`
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required,min=6,max=100"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("UserAdminHandler.CreateUser: invalid request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 获取当前用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	createdBy, ok := userID.(int64)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user"})
		return
	}

	slog.Debug("UserAdminHandler.CreateUser: handling request", "username", req.Username)

	user, err := h.svc.CreateUser(ctx, req.Username, req.Email, req.Password, createdBy)
	if err != nil {
		slog.Error("UserAdminHandler.CreateUser: failed to create user", "username", req.Username, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, user)
}

// UpdateUser 更新用户
func (h *UserAdminHandler) UpdateUser(c *gin.Context) {
	ctx := c.Request.Context()

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	adminID, ok := userID.(int64)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		slog.Warn("UserAdminHandler.UpdateUser: invalid id", "id_str", idStr, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if id <= 0 {
		slog.Warn("UserAdminHandler.UpdateUser: id must be positive", "id", id)
		c.JSON(http.StatusBadRequest, gin.H{"error": "id must be positive"})
		return
	}

	var req struct {
		Username string `json:"username" binding:"required,min=3,max=50,alphanum"`
		Email    string `json:"email" binding:"required,email"`
		Role     string `json:"role" binding:"required,oneof=system_admin domain_admin user"`
		IsActive bool   `json:"is_active"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("UserAdminHandler.UpdateUser: invalid request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	slog.Debug("UserAdminHandler.UpdateUser: handling request", "user_id", id, "username", req.Username)

	user, err := h.svc.UpdateUserWithDomainCheck(ctx, adminID, id, req.Username, req.Email, req.Role, req.IsActive)
	if err != nil {
		if err == service.ErrPermissionDenied {
			slog.Warn("UserAdminHandler.UpdateUser: permission denied", "admin_id", adminID, "target_id", id)
			c.JSON(http.StatusForbidden, gin.H{"error": "permission denied"})
			return
		}
		if err == service.ErrUserNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		slog.Error("UserAdminHandler.UpdateUser: failed to update user", "user_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, user)
}

// DeleteUser 删除用户
func (h *UserAdminHandler) DeleteUser(c *gin.Context) {
	ctx := c.Request.Context()
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		slog.Warn("UserAdminHandler.DeleteUser: invalid id", "id_str", idStr, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if id <= 0 {
		slog.Warn("UserAdminHandler.DeleteUser: id must be positive", "id", id)
		c.JSON(http.StatusBadRequest, gin.H{"error": "id must be positive"})
		return
	}

	slog.Debug("UserAdminHandler.DeleteUser: handling request", "user_id", id)

	err = h.svc.DeleteUser(ctx, id)
	if err != nil {
		slog.Error("UserAdminHandler.DeleteUser: failed to delete user", "user_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// GetUserRoles 获取用户角色详情
func (h *UserAdminHandler) GetUserRoles(c *gin.Context) {
	ctx := c.Request.Context()
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		slog.Warn("UserAdminHandler.GetUserRoles: invalid id", "id_str", idStr, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if id <= 0 {
		slog.Warn("UserAdminHandler.GetUserRoles: id must be positive", "id", id)
		c.JSON(http.StatusBadRequest, gin.H{"error": "id must be positive"})
		return
	}

	slog.Debug("UserAdminHandler.GetUserRoles: handling request", "user_id", id)

	roles, err := h.svc.GetUserRoles(ctx, id)
	if err != nil {
		slog.Error("UserAdminHandler.GetUserRoles: failed to get user roles", "user_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": roles})
}

// AssignUserRoles 分配用户角色
func (h *UserAdminHandler) AssignUserRoles(c *gin.Context) {
	ctx := c.Request.Context()
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		slog.Warn("UserAdminHandler.AssignUserRoles: invalid id", "id_str", idStr, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if id <= 0 {
		slog.Warn("UserAdminHandler.AssignUserRoles: id must be positive", "id", id)
		c.JSON(http.StatusBadRequest, gin.H{"error": "id must be positive"})
		return
	}

	var req model.UserRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("UserAdminHandler.AssignUserRoles: invalid request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	slog.Debug("UserAdminHandler.AssignUserRoles: handling request", "user_id", id)

	err = h.svc.AssignUserRoles(ctx, id, req.RoleIDs, req.DomainIDs)
	if err != nil {
		slog.Error("UserAdminHandler.AssignUserRoles: failed to assign roles", "user_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "roles assigned successfully"})
}

// AssignUserDomains 分配用户领域
func (h *UserAdminHandler) AssignUserDomains(c *gin.Context) {
	ctx := c.Request.Context()
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		slog.Warn("UserAdminHandler.AssignUserDomains: invalid id", "id_str", idStr, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if id <= 0 {
		slog.Warn("UserAdminHandler.AssignUserDomains: id must be positive", "id", id)
		c.JSON(http.StatusBadRequest, gin.H{"error": "id must be positive"})
		return
	}

	var req struct {
		DomainIDs []int64 `json:"domain_ids" binding:"required,min=1"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("UserAdminHandler.AssignUserDomains: invalid request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	slog.Debug("UserAdminHandler.AssignUserDomains: handling request", "user_id", id, "domain_ids", req.DomainIDs)

	err = h.svc.AssignUserDomains(ctx, id, req.DomainIDs)
	if err != nil {
		slog.Error("UserAdminHandler.AssignUserDomains: failed to assign domains", "user_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "domains assigned successfully"})
}

// GetCurrentUser 获取当前用户信息
func (h *UserAdminHandler) GetCurrentUser(c *gin.Context) {
	ctx := c.Request.Context()

	userID, exists := c.Get("user_id")
	if !exists {
		slog.Warn("UserAdminHandler.GetCurrentUser: user_id not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	id, ok := userID.(int64)
	if !ok {
		slog.Warn("UserAdminHandler.GetCurrentUser: invalid user_id type")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user"})
		return
	}

	slog.Debug("UserAdminHandler.GetCurrentUser: handling request", "user_id", id)

	user, err := h.svc.GetCurrentUserInfo(ctx, id)
	if err != nil {
		slog.Error("UserAdminHandler.GetCurrentUser: failed to get user", "user_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	c.JSON(http.StatusOK, user)
}

// UpdateCurrentUser 更新当前用户信息（只能修改邮箱）
func (h *UserAdminHandler) UpdateCurrentUser(c *gin.Context) {
	ctx := c.Request.Context()

	userID, exists := c.Get("user_id")
	if !exists {
		slog.Warn("UserAdminHandler.UpdateCurrentUser: user_id not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	id, ok := userID.(int64)
	if !ok {
		slog.Warn("UserAdminHandler.UpdateCurrentUser: invalid user_id type")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user"})
		return
	}

	var req model.UpdateCurrentUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("UserAdminHandler.UpdateCurrentUser: invalid request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	slog.Debug("UserAdminHandler.UpdateCurrentUser: handling request", "user_id", id)

	user, err := h.svc.UpdateCurrentUser(ctx, id, req.Email)
	if err != nil {
		slog.Error("UserAdminHandler.UpdateCurrentUser: failed to update user", "user_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, user)
}

// ChangePassword 修改当前用户密码
func (h *UserAdminHandler) ChangePassword(c *gin.Context) {
	ctx := c.Request.Context()

	userID, exists := c.Get("user_id")
	if !exists {
		slog.Warn("UserAdminHandler.ChangePassword: user_id not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	id, ok := userID.(int64)
	if !ok {
		slog.Warn("UserAdminHandler.ChangePassword: invalid user_id type")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user"})
		return
	}

	var req model.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("UserAdminHandler.ChangePassword: invalid request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	slog.Debug("UserAdminHandler.ChangePassword: handling request", "user_id", id)

	err := h.svc.ChangePassword(ctx, id, req.OldPassword, req.NewPassword)
	if err != nil {
		if err == service.ErrWrongPassword {
			slog.Warn("UserAdminHandler.ChangePassword: wrong old password", "user_id", id)
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err == service.ErrPasswordTooShort {
			slog.Warn("UserAdminHandler.ChangePassword: password too short", "user_id", id)
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		slog.Error("UserAdminHandler.ChangePassword: failed to change password", "user_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "password changed successfully"})
}

// ResetUserPassword 重置用户密码（管理员用）
func (h *UserAdminHandler) ResetUserPassword(c *gin.Context) {
	ctx := c.Request.Context()

	currentUserID, exists := c.Get("user_id")
	if !exists {
		slog.Warn("UserAdminHandler.ResetUserPassword: user_id not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	currID, ok := currentUserID.(int64)
	if !ok {
		slog.Warn("UserAdminHandler.ResetUserPassword: invalid current user_id type")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user"})
		return
	}

	idStr := c.Param("id")
	targetUserID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		slog.Warn("UserAdminHandler.ResetUserPassword: invalid target user id", "id_str", idStr, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if targetUserID <= 0 {
		slog.Warn("UserAdminHandler.ResetUserPassword: id must be positive", "id", targetUserID)
		c.JSON(http.StatusBadRequest, gin.H{"error": "id must be positive"})
		return
	}

	var req model.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("UserAdminHandler.ResetUserPassword: invalid request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	slog.Debug("UserAdminHandler.ResetUserPassword: handling request", "current_user_id", currID, "target_user_id", targetUserID)

	err = h.svc.ResetUserPasswordWithDomainCheck(ctx, currID, targetUserID, req.NewPassword)
	if err != nil {
		if err == service.ErrPermissionDenied {
			slog.Warn("UserAdminHandler.ResetUserPassword: permission denied", "current_user_id", currID, "target_user_id", targetUserID)
			c.JSON(http.StatusForbidden, gin.H{"error": "permission denied: you can only manage users in your domain"})
			return
		}
		if err == service.ErrUserNotFound {
			slog.Warn("UserAdminHandler.ResetUserPassword: target user not found", "target_user_id", targetUserID)
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		slog.Error("UserAdminHandler.ResetUserPassword: failed to reset password", "target_user_id", targetUserID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "password reset successfully"})
}
