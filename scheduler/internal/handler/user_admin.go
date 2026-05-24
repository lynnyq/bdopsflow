package handler

import (
	"context"
	"fmt"
	"log/slog"
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
		"RealName": "姓名",
		"Phone":    "手机号",
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
	errStr = strings.ReplaceAll(errStr, "regexp", "格式不正确")
	errStr = strings.ReplaceAll(errStr, "oneof", "可选值为")

	errStr = strings.TrimSpace(errStr)
	errStr = regexp.MustCompile(`\s+`).ReplaceAllString(errStr, " ")

	return errStr
}

func getUserFriendlyError(err error, operation string) (string, int) {
	if err == nil {
		return "操作失败，请稍后重试", CodeInternalError
	}

	errStr := err.Error()

	switch {
	case strings.Contains(errStr, "UNIQUE constraint failed"):
		if strings.Contains(errStr, "username") {
			return "用户名已存在", CodeBadRequest
		}
		if strings.Contains(errStr, "email") {
			return "邮箱已被使用", CodeBadRequest
		}
		return "数据已存在，请检查后重试", CodeBadRequest

	case strings.Contains(errStr, "FOREIGN KEY constraint failed"):
		return "关联数据不存在，请检查输入", CodeBadRequest

	case strings.Contains(errStr, "NOT NULL constraint failed"):
		return "缺少必填字段", CodeBadRequest

	default:
		slog.Error("UserAdminHandler: "+operation+" failed", "error", err)
		return "操作失败，请稍后重试", CodeInternalError
	}
}

type UserAdminHandler struct {
	svc *service.UserAdminService
}

func NewUserAdminHandler(svc *service.UserAdminService) *UserAdminHandler {
	return &UserAdminHandler{svc: svc}
}

func (h *UserAdminHandler) validateRoleCode(ctx context.Context, roleCode string) error {
	validRoles := map[string]bool{
		"system_admin": true,
		"domain_admin": true,
		"user":         true,
	}

	if validRoles[roleCode] {
		return nil
	}

	roles, err := h.svc.GetAllRoles(ctx)
	if err != nil {
		return fmt.Errorf("验证角色失败")
	}

	for _, role := range roles {
		if role.Code == roleCode {
			return nil
		}
	}

	return fmt.Errorf("无效的角色代码: %s", roleCode)
}

func (h *UserAdminHandler) ListUsers(c *gin.Context) {
	ctx := c.Request.Context()
	defer func() {
		if r := recover(); r != nil {
			slog.Error("UserAdminHandler.ListUsers: panic recovered", "panic", r)
			Fail(c, CodeInternalError, "服务异常，请稍后重试")
		}
	}()

	users, err := h.svc.ListUsers(ctx)
	if err != nil {
		slog.Error("UserAdminHandler.ListUsers: failed to list users", "error", err)
		Fail(c, CodeInternalError, "加载用户列表失败，请稍后重试")
		return
	}

	for i := range users {
		if users[i] != nil {
			users[i].Password = ""
		}
	}

	Success(c, gin.H{"items": users})
}

func (h *UserAdminHandler) GetUser(c *gin.Context) {
	ctx := c.Request.Context()
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		BadRequest(c, "无效的用户ID")
		return
	}

	user, err := h.svc.GetUserByID(ctx, id)
	if err != nil {
		slog.Error("UserAdminHandler.GetUser: failed to get user", "user_id", id, "error", err)
		Fail(c, CodeInternalError, "获取用户信息失败，请稍后重试")
		return
	}

	if user == nil {
		NotFound(c, "用户不存在")
		return
	}

	user.Password = ""

	roles, err := h.svc.GetUserRoles(ctx, id)
	if err != nil {
		slog.Error("UserAdminHandler.GetUser: failed to get user roles", "user_id", id, "error", err)
		roles = nil
	}

	Success(c, gin.H{
		"user":  user,
		"roles": roles,
	})
}

func (h *UserAdminHandler) CreateUser(c *gin.Context) {
	ctx := c.Request.Context()
	defer func() {
		if r := recover(); r != nil {
			slog.Error("UserAdminHandler.CreateUser: panic recovered", "panic", r)
			Fail(c, CodeInternalError, "服务异常，请稍后重试")
		}
	}()

	var req struct {
		Username string `json:"username" binding:"required,min=3,max=50,regexp=^[a-zA-Z0-9_ ]+$"`
		RealName string `json:"real_name" binding:"max=50"`
		Phone    string `json:"phone" binding:"max=20"`
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required,min=1,max=512"`
		Role     string `json:"role" binding:"required,min=1,max=100"`
		DomainID *int64 `json:"domain_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("UserAdminHandler.CreateUser: invalid request", "error", err)
		BadRequest(c, formatValidationError(err))
		return
	}

	if err := h.validateRoleCode(ctx, req.Role); err != nil {
		BadRequest(c, err.Error())
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		Unauthorized(c, "未授权，请重新登录")
		return
	}

	createdBy, ok := userID.(int64)
	if !ok {
		Unauthorized(c, "用户信息无效，请重新登录")
		return
	}

	user, err := h.svc.CreateUser(ctx, req.Username, req.RealName, req.Phone, req.Email, req.Password, req.Role, req.DomainID, createdBy)
	if err != nil {
		if err == service.ErrPasswordTooShort {
			BadRequest(c, "密码长度至少为6位")
			return
		}
		if err == service.ErrPasswordTooLong {
			BadRequest(c, "密码长度不能超过30位")
			return
		}
		if err == service.ErrPasswordWeak {
			BadRequest(c, "密码必须包含字母和数字")
			return
		}
		errMsg, statusCode := getUserFriendlyError(err, "CreateUser")
		Fail(c, statusCode, errMsg)
		return
	}

	if user != nil {
		user.Password = ""
	}

	c.Set("audit_resource_id", strconv.FormatInt(user.ID, 10))
	c.Set("audit_resource_name", req.Username)
	Created(c, user)
}

func (h *UserAdminHandler) UpdateUser(c *gin.Context) {
	ctx := c.Request.Context()
	defer func() {
		if r := recover(); r != nil {
			slog.Error("UserAdminHandler.UpdateUser: panic recovered", "panic", r)
			Fail(c, CodeInternalError, "服务异常，请稍后重试")
		}
	}()

	userID, exists := c.Get("user_id")
	if !exists {
		Unauthorized(c, "未授权，请重新登录")
		return
	}

	adminID, ok := userID.(int64)
	if !ok {
		Unauthorized(c, "用户信息无效，请重新登录")
		return
	}

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		BadRequest(c, "无效的用户ID")
		return
	}

	var req struct {
		Username string `json:"username" binding:"required,min=3,max=50,regexp=^[a-zA-Z0-9_ ]+$"`
		RealName string `json:"real_name" binding:"max=50"`
		Phone    string `json:"phone" binding:"max=20"`
		Email    string `json:"email" binding:"required,email"`
		Role     string `json:"role" binding:"required,min=1,max=100"`
		IsActive bool   `json:"is_active"`
		DomainID *int64 `json:"domain_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("UserAdminHandler.UpdateUser: invalid request", "error", err)
		BadRequest(c, formatValidationError(err))
		return
	}

	if err := h.validateRoleCode(ctx, req.Role); err != nil {
		BadRequest(c, err.Error())
		return
	}

	user, err := h.svc.UpdateUserWithDomainCheck(ctx, adminID, id, req.Username, req.RealName, req.Phone, req.Email, req.Role, req.IsActive, req.DomainID)
	if err != nil {
		if err == service.ErrPermissionDenied {
			Forbidden(c, "权限不足，无法修改此用户")
			return
		}
		if err == service.ErrUserNotFound {
			NotFound(c, "用户不存在")
			return
		}
		errMsg, statusCode := getUserFriendlyError(err, "UpdateUser")
		Fail(c, statusCode, errMsg)
		return
	}

	if user != nil {
		user.Password = ""
	}

	c.Set("audit_resource_id", strconv.FormatInt(id, 10))
	c.Set("audit_resource_name", req.Username)
	Success(c, user)
}

func (h *UserAdminHandler) DeleteUser(c *gin.Context) {
	ctx := c.Request.Context()
	defer func() {
		if r := recover(); r != nil {
			slog.Error("UserAdminHandler.DeleteUser: panic recovered", "panic", r)
			Fail(c, CodeInternalError, "服务异常，请稍后重试")
		}
	}()

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		BadRequest(c, "无效的用户ID")
		return
	}

	// 先获取用户信息用于审计日志
	user, _ := h.svc.GetUserByID(ctx, id)

	if err := h.svc.DeleteUser(ctx, id); err != nil {
		errMsg, statusCode := getUserFriendlyError(err, "DeleteUser")
		Fail(c, statusCode, errMsg)
		return
	}

	c.Set("audit_resource_id", strconv.FormatInt(id, 10))
	if user != nil {
		c.Set("audit_resource_name", user.Username)
	}
	Success(c, nil)
}

func (h *UserAdminHandler) GetUserRoles(c *gin.Context) {
	ctx := c.Request.Context()
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		BadRequest(c, "无效的用户ID")
		return
	}

	roles, err := h.svc.GetUserRoles(ctx, id)
	if err != nil {
		slog.Error("UserAdminHandler.GetUserRoles: failed to get user roles", "user_id", id, "error", err)
		Fail(c, CodeInternalError, "获取用户角色失败，请稍后重试")
		return
	}

	Success(c, gin.H{"items": roles})
}

func (h *UserAdminHandler) AssignUserRoles(c *gin.Context) {
	ctx := c.Request.Context()
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		BadRequest(c, "无效的用户ID")
		return
	}

	var req model.UserRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("UserAdminHandler.AssignUserRoles: invalid request", "error", err)
		BadRequest(c, formatValidationError(err))
		return
	}

	if err := h.svc.AssignUserRoles(ctx, id, req.RoleIDs, req.DomainIDs); err != nil {
		errMsg, statusCode := getUserFriendlyError(err, "AssignUserRoles")
		Fail(c, statusCode, errMsg)
		return
	}

	c.Set("audit_resource_id", strconv.FormatInt(id, 10))
	c.Set("audit_action", "assign")
	SuccessWithMessage(c, "角色分配成功", nil)
}

func (h *UserAdminHandler) AssignUserDomains(c *gin.Context) {
	ctx := c.Request.Context()
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		BadRequest(c, "无效的用户ID")
		return
	}

	var req struct {
		DomainIDs []int64 `json:"domain_ids" binding:"required,min=1"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("UserAdminHandler.AssignUserDomains: invalid request", "error", err)
		BadRequest(c, formatValidationError(err))
		return
	}

	if err := h.svc.AssignUserDomains(ctx, id, req.DomainIDs); err != nil {
		errMsg, statusCode := getUserFriendlyError(err, "AssignUserDomains")
		Fail(c, statusCode, errMsg)
		return
	}

	SuccessWithMessage(c, "领域分配成功", nil)
}

func (h *UserAdminHandler) ListUsersByDomain(c *gin.Context) {
	ctx := c.Request.Context()
	role, _ := c.Get("role")
	domainID, _ := c.Get("domain_id")

	var users []*model.User
	var err error

	if role == "system_admin" || role == "admin" {
		if d := c.Query("domain_id"); d != "" {
			did, parseErr := strconv.ParseInt(d, 10, 64)
			if parseErr == nil && did > 0 {
				users, err = h.svc.GetUsersByDomain(ctx, did)
			} else {
				users, err = h.svc.ListUsers(ctx)
			}
		} else {
			users, err = h.svc.ListUsers(ctx)
		}
	} else {
		users, err = h.svc.GetUsersByDomain(ctx, domainID.(int64))
	}

	if err != nil {
		slog.Error("UserAdminHandler.ListUsersByDomain: failed to list users", "error", err)
		Fail(c, CodeInternalError, "加载用户列表失败，请稍后重试")
		return
	}

	for i := range users {
		if users[i] != nil {
			users[i].Password = ""
		}
	}

	Success(c, gin.H{"items": users})
}

func (h *UserAdminHandler) GetCurrentUser(c *gin.Context) {
	ctx := c.Request.Context()

	userID, exists := c.Get("user_id")
	if !exists {
		Unauthorized(c, "未授权，请重新登录")
		return
	}

	id, ok := userID.(int64)
	if !ok {
		Unauthorized(c, "用户信息无效，请重新登录")
		return
	}

	user, err := h.svc.GetCurrentUserInfo(ctx, id)
	if err != nil {
		slog.Error("UserAdminHandler.GetCurrentUser: failed to get user", "user_id", id, "error", err)
		Fail(c, CodeInternalError, "获取用户信息失败，请稍后重试")
		return
	}

	if user == nil {
		NotFound(c, "用户不存在")
		return
	}

	user.Password = ""

	Success(c, user)
}

func (h *UserAdminHandler) UpdateCurrentUser(c *gin.Context) {
	ctx := c.Request.Context()

	userID, exists := c.Get("user_id")
	if !exists {
		Unauthorized(c, "未授权，请重新登录")
		return
	}

	id, ok := userID.(int64)
	if !ok {
		Unauthorized(c, "用户信息无效，请重新登录")
		return
	}

	var req model.UpdateCurrentUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("UserAdminHandler.UpdateCurrentUser: invalid request", "error", err)
		BadRequest(c, formatValidationError(err))
		return
	}

	user, err := h.svc.UpdateCurrentUser(ctx, id, req.RealName, req.Phone, req.Email)
	if err != nil {
		errMsg, statusCode := getUserFriendlyError(err, "UpdateCurrentUser")
		Fail(c, statusCode, errMsg)
		return
	}

	if user != nil {
		user.Password = ""
	}

	Success(c, user)
}

func (h *UserAdminHandler) ChangePassword(c *gin.Context) {
	ctx := c.Request.Context()

	userID, exists := c.Get("user_id")
	if !exists {
		Unauthorized(c, "未授权，请重新登录")
		return
	}

	id, ok := userID.(int64)
	if !ok {
		Unauthorized(c, "用户信息无效，请重新登录")
		return
	}

	var req model.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("UserAdminHandler.ChangePassword: invalid request", "error", err)
		BadRequest(c, formatValidationError(err))
		return
	}

	if err := h.svc.ChangePassword(ctx, id, req.OldPassword, req.NewPassword); err != nil {
		if err == service.ErrWrongPassword {
			BadRequest(c, "原密码错误")
			return
		}
		if err == service.ErrPasswordTooShort {
			BadRequest(c, "密码长度至少为6位")
			return
		}
		if err == service.ErrPasswordTooLong {
			BadRequest(c, "密码长度不能超过30位")
			return
		}
		if err == service.ErrPasswordWeak {
			BadRequest(c, "密码必须包含字母和数字")
			return
		}
		errMsg, statusCode := getUserFriendlyError(err, "ChangePassword")
		Fail(c, statusCode, errMsg)
		return
	}

	SuccessWithMessage(c, "密码修改成功", nil)
}

func (h *UserAdminHandler) ResetUserPassword(c *gin.Context) {
	ctx := c.Request.Context()
	defer func() {
		if r := recover(); r != nil {
			slog.Error("UserAdminHandler.ResetUserPassword: panic recovered", "panic", r)
			Fail(c, CodeInternalError, "服务异常，请稍后重试")
		}
	}()

	currentUserID, exists := c.Get("user_id")
	if !exists {
		Unauthorized(c, "未授权，请重新登录")
		return
	}

	currID, ok := currentUserID.(int64)
	if !ok {
		Unauthorized(c, "用户信息无效，请重新登录")
		return
	}

	idStr := c.Param("id")
	targetUserID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || targetUserID <= 0 {
		BadRequest(c, "无效的用户ID")
		return
	}

	var req model.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("UserAdminHandler.ResetUserPassword: invalid request", "error", err)
		BadRequest(c, formatValidationError(err))
		return
	}

	if err := h.svc.ResetUserPasswordWithDomainCheck(ctx, currID, targetUserID, req.NewPassword); err != nil {
		if err == service.ErrPermissionDenied {
			Forbidden(c, "权限不足，只能管理本领域用户")
			return
		}
		if err == service.ErrUserNotFound {
			NotFound(c, "目标用户不存在")
			return
		}
		if err == service.ErrPasswordTooShort {
			BadRequest(c, "密码长度至少为6位")
			return
		}
		if err == service.ErrPasswordTooLong {
			BadRequest(c, "密码长度不能超过30位")
			return
		}
		if err == service.ErrPasswordWeak {
			BadRequest(c, "密码必须包含字母和数字")
			return
		}
		errMsg, statusCode := getUserFriendlyError(err, "ResetUserPassword")
		Fail(c, statusCode, errMsg)
		return
	}

	SuccessWithMessage(c, "密码重置成功", nil)
	c.Set("audit_resource_id", strconv.FormatInt(targetUserID, 10))
	c.Set("audit_action", "reset_password")
}
