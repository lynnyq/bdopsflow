package handler

import (
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"github.com/lynnyq/bdopsflow/scheduler/internal/service"
)

func formatValidationError(err error) string {
	if err == nil {
		return "请求参数错误"
	}

	errStr := err.Error()

	fieldMap := map[string]string{
		"Username": "用户名",
		"Email":    "邮箱",
		"Password": "密码",
		"Role":     "角色",
		"DomainID": "领域",
	}

	for eng, chn := range fieldMap {
		errStr = strings.ReplaceAll(errStr, eng, chn)
	}

	errStr = strings.ReplaceAll(errStr, "Key: '", "")
	errStr = strings.ReplaceAll(errStr, "' Error:", "：")
	errStr = strings.ReplaceAll(errStr, "Field validation for", "")
	errStr = strings.ReplaceAll(errStr, "failed on the", "校验失败：")
	errStr = strings.ReplaceAll(errStr, "tag", "")

	errStr = strings.ReplaceAll(errStr, "required", "不能为空")
	errStr = strings.ReplaceAll(errStr, "min", "最小长度为")
	errStr = strings.ReplaceAll(errStr, "max", "最大长度为")
	errStr = strings.ReplaceAll(errStr, "email", "邮箱格式不正确")
	errStr = strings.ReplaceAll(errStr, "alphanum", "只能包含字母和数字")
	errStr = strings.ReplaceAll(errStr, "oneof", "可选值为")

	errStr = strings.TrimSpace(errStr)
	errStr = regexp.MustCompile(`\s+`).ReplaceAllString(errStr, " ")

	return errStr
}

func getUserFriendlyError(err error, operation string) (string, int) {
	if err == nil {
		return "操作失败，请稍后重试", http.StatusInternalServerError
	}

	errStr := err.Error()

	switch {
	case strings.Contains(errStr, "UNIQUE constraint failed"):
		if strings.Contains(errStr, "username") {
			return "用户名已存在", http.StatusBadRequest
		}
		if strings.Contains(errStr, "email") {
			return "邮箱已被使用", http.StatusBadRequest
		}
		return "数据已存在，请检查后重试", http.StatusBadRequest

	case strings.Contains(errStr, "FOREIGN KEY constraint failed"):
		return "关联数据不存在，请检查输入", http.StatusBadRequest

	case strings.Contains(errStr, "NOT NULL constraint failed"):
		return "缺少必填字段", http.StatusBadRequest

	default:
		slog.Error("UserAdminHandler: "+operation+" failed", "error", err)
		return "操作失败，请稍后重试", http.StatusInternalServerError
	}
}

type UserAdminHandler struct {
	svc *service.UserAdminService
}

func NewUserAdminHandler(svc *service.UserAdminService) *UserAdminHandler {
	return &UserAdminHandler{svc: svc}
}

func (h *UserAdminHandler) ListUsers(c *gin.Context) {
	ctx := c.Request.Context()
	defer func() {
		if r := recover(); r != nil {
			slog.Error("UserAdminHandler.ListUsers: panic recovered", "panic", r)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "服务异常，请稍后重试"})
		}
	}()

	users, err := h.svc.ListUsers(ctx)
	if err != nil {
		slog.Error("UserAdminHandler.ListUsers: failed to list users", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "加载用户列表失败，请稍后重试"})
		return
	}

	for i := range users {
		if users[i] != nil {
			users[i].Password = ""
		}
	}

	c.JSON(http.StatusOK, gin.H{"items": users})
}

func (h *UserAdminHandler) GetUser(c *gin.Context) {
	ctx := c.Request.Context()
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的用户ID"})
		return
	}

	user, err := h.svc.GetUserByID(ctx, id)
	if err != nil {
		slog.Error("UserAdminHandler.GetUser: failed to get user", "user_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取用户信息失败，请稍后重试"})
		return
	}

	if user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	user.Password = ""

	roles, err := h.svc.GetUserRoles(ctx, id)
	if err != nil {
		slog.Error("UserAdminHandler.GetUser: failed to get user roles", "user_id", id, "error", err)
		roles = nil
	}

	c.JSON(http.StatusOK, gin.H{
		"user":  user,
		"roles": roles,
	})
}

func (h *UserAdminHandler) CreateUser(c *gin.Context) {
	ctx := c.Request.Context()
	defer func() {
		if r := recover(); r != nil {
			slog.Error("UserAdminHandler.CreateUser: panic recovered", "panic", r)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "服务异常，请稍后重试"})
		}
	}()

	var req struct {
		Username string `json:"username" binding:"required,min=3,max=50,alphanum"`
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required,min=6,max=100"`
		Role     string `json:"role" binding:"required,oneof=system_admin domain_admin user"`
		DomainID *int64 `json:"domain_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("UserAdminHandler.CreateUser: invalid request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": formatValidationError(err)})
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权，请重新登录"})
		return
	}

	createdBy, ok := userID.(int64)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户信息无效，请重新登录"})
		return
	}

	user, err := h.svc.CreateUser(ctx, req.Username, req.Email, req.Password, req.Role, req.DomainID, createdBy)
	if err != nil {
		errMsg, statusCode := getUserFriendlyError(err, "CreateUser")
		c.JSON(statusCode, gin.H{"error": errMsg})
		return
	}

	if user != nil {
		user.Password = ""
	}

	c.JSON(http.StatusCreated, user)
}

func (h *UserAdminHandler) UpdateUser(c *gin.Context) {
	ctx := c.Request.Context()
	defer func() {
		if r := recover(); r != nil {
			slog.Error("UserAdminHandler.UpdateUser: panic recovered", "panic", r)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "服务异常，请稍后重试"})
		}
	}()

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权，请重新登录"})
		return
	}

	adminID, ok := userID.(int64)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户信息无效，请重新登录"})
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的用户ID"})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": formatValidationError(err)})
		return
	}

	user, err := h.svc.UpdateUserWithDomainCheck(ctx, adminID, id, req.Username, req.Email, req.Role, req.IsActive)
	if err != nil {
		if err == service.ErrPermissionDenied {
			c.JSON(http.StatusForbidden, gin.H{"error": "权限不足，无法修改此用户"})
			return
		}
		if err == service.ErrUserNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
			return
		}
		errMsg, statusCode := getUserFriendlyError(err, "UpdateUser")
		c.JSON(statusCode, gin.H{"error": errMsg})
		return
	}

	if user != nil {
		user.Password = ""
	}

	c.JSON(http.StatusOK, user)
}

func (h *UserAdminHandler) DeleteUser(c *gin.Context) {
	ctx := c.Request.Context()
	defer func() {
		if r := recover(); r != nil {
			slog.Error("UserAdminHandler.DeleteUser: panic recovered", "panic", r)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "服务异常，请稍后重试"})
		}
	}()

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的用户ID"})
		return
	}

	if err := h.svc.DeleteUser(ctx, id); err != nil {
		errMsg, statusCode := getUserFriendlyError(err, "DeleteUser")
		c.JSON(statusCode, gin.H{"error": errMsg})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

func (h *UserAdminHandler) GetUserRoles(c *gin.Context) {
	ctx := c.Request.Context()
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的用户ID"})
		return
	}

	roles, err := h.svc.GetUserRoles(ctx, id)
	if err != nil {
		slog.Error("UserAdminHandler.GetUserRoles: failed to get user roles", "user_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取用户角色失败，请稍后重试"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": roles})
}

func (h *UserAdminHandler) AssignUserRoles(c *gin.Context) {
	ctx := c.Request.Context()
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的用户ID"})
		return
	}

	var req model.UserRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("UserAdminHandler.AssignUserRoles: invalid request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": formatValidationError(err)})
		return
	}

	if err := h.svc.AssignUserRoles(ctx, id, req.RoleIDs, req.DomainIDs); err != nil {
		errMsg, statusCode := getUserFriendlyError(err, "AssignUserRoles")
		c.JSON(statusCode, gin.H{"error": errMsg})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "角色分配成功"})
}

func (h *UserAdminHandler) AssignUserDomains(c *gin.Context) {
	ctx := c.Request.Context()
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的用户ID"})
		return
	}

	var req struct {
		DomainIDs []int64 `json:"domain_ids" binding:"required,min=1"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("UserAdminHandler.AssignUserDomains: invalid request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": formatValidationError(err)})
		return
	}

	if err := h.svc.AssignUserDomains(ctx, id, req.DomainIDs); err != nil {
		errMsg, statusCode := getUserFriendlyError(err, "AssignUserDomains")
		c.JSON(statusCode, gin.H{"error": errMsg})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "领域分配成功"})
}

func (h *UserAdminHandler) GetCurrentUser(c *gin.Context) {
	ctx := c.Request.Context()

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权，请重新登录"})
		return
	}

	id, ok := userID.(int64)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户信息无效，请重新登录"})
		return
	}

	user, err := h.svc.GetCurrentUserInfo(ctx, id)
	if err != nil {
		slog.Error("UserAdminHandler.GetCurrentUser: failed to get user", "user_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取用户信息失败，请稍后重试"})
		return
	}

	if user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	user.Password = ""

	c.JSON(http.StatusOK, user)
}

func (h *UserAdminHandler) UpdateCurrentUser(c *gin.Context) {
	ctx := c.Request.Context()

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权，请重新登录"})
		return
	}

	id, ok := userID.(int64)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户信息无效，请重新登录"})
		return
	}

	var req model.UpdateCurrentUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("UserAdminHandler.UpdateCurrentUser: invalid request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": formatValidationError(err)})
		return
	}

	user, err := h.svc.UpdateCurrentUser(ctx, id, req.Email)
	if err != nil {
		errMsg, statusCode := getUserFriendlyError(err, "UpdateCurrentUser")
		c.JSON(statusCode, gin.H{"error": errMsg})
		return
	}

	if user != nil {
		user.Password = ""
	}

	c.JSON(http.StatusOK, user)
}

func (h *UserAdminHandler) ChangePassword(c *gin.Context) {
	ctx := c.Request.Context()

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权，请重新登录"})
		return
	}

	id, ok := userID.(int64)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户信息无效，请重新登录"})
		return
	}

	var req model.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("UserAdminHandler.ChangePassword: invalid request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": formatValidationError(err)})
		return
	}

	if err := h.svc.ChangePassword(ctx, id, req.OldPassword, req.NewPassword); err != nil {
		if err == service.ErrWrongPassword {
			c.JSON(http.StatusBadRequest, gin.H{"error": "原密码错误"})
			return
		}
		if err == service.ErrPasswordTooShort {
			c.JSON(http.StatusBadRequest, gin.H{"error": "新密码长度不足"})
			return
		}
		errMsg, statusCode := getUserFriendlyError(err, "ChangePassword")
		c.JSON(statusCode, gin.H{"error": errMsg})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "密码修改成功"})
}

func (h *UserAdminHandler) ResetUserPassword(c *gin.Context) {
	ctx := c.Request.Context()
	defer func() {
		if r := recover(); r != nil {
			slog.Error("UserAdminHandler.ResetUserPassword: panic recovered", "panic", r)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "服务异常，请稍后重试"})
		}
	}()

	currentUserID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权，请重新登录"})
		return
	}

	currID, ok := currentUserID.(int64)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户信息无效，请重新登录"})
		return
	}

	idStr := c.Param("id")
	targetUserID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || targetUserID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的用户ID"})
		return
	}

	var req model.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("UserAdminHandler.ResetUserPassword: invalid request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": formatValidationError(err)})
		return
	}

	if err := h.svc.ResetUserPasswordWithDomainCheck(ctx, currID, targetUserID, req.NewPassword); err != nil {
		if err == service.ErrPermissionDenied {
			c.JSON(http.StatusForbidden, gin.H{"error": "权限不足，只能管理本领域用户"})
			return
		}
		if err == service.ErrUserNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "目标用户不存在"})
			return
		}
		errMsg, statusCode := getUserFriendlyError(err, "ResetUserPassword")
		c.JSON(statusCode, gin.H{"error": errMsg})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "密码重置成功"})
}
