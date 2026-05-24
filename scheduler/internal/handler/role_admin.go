package handler

import (
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"github.com/lynnyq/bdopsflow/scheduler/internal/service"
)

// RoleAdminHandler 角色管理 Handler
type RoleAdminHandler struct {
	svc *service.RoleAdminService
}

// NewRoleAdminHandler 创建角色管理 Handler
func NewRoleAdminHandler(svc *service.RoleAdminService) *RoleAdminHandler {
	return &RoleAdminHandler{svc: svc}
}

// ListRoles 获取角色列表
func (h *RoleAdminHandler) ListRoles(c *gin.Context) {
	ctx := c.Request.Context()
	defer func() {
		if r := recover(); r != nil {
			slog.Error("RoleAdminHandler.ListRoles: panic recovered", "panic", r)
			Fail(c, CodeInternalError, "internal server error")
		}
	}()

	slog.Debug("RoleAdminHandler.ListRoles: handling request")

	bdopsflow_roles, err := h.svc.ListRoles(ctx)
	if err != nil {
		slog.Error("RoleAdminHandler.ListRoles: failed to list bdopsflow_roles", "error", err)
		FailFromError(c, err)
		return
	}

	Success(c, gin.H{"items": bdopsflow_roles})
}

// GetRole 获取角色详情
func (h *RoleAdminHandler) GetRole(c *gin.Context) {
	ctx := c.Request.Context()
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		slog.Warn("RoleAdminHandler.GetRole: invalid id", "id_str", idStr, "error", err)
		BadRequest(c, "invalid id")
		return
	}

	if id <= 0 {
		slog.Warn("RoleAdminHandler.GetRole: id must be positive", "id", id)
		BadRequest(c, "id must be positive")
		return
	}

	slog.Debug("RoleAdminHandler.GetRole: handling request", "role_id", id)

	role, err := h.svc.GetRole(ctx, id)
	if err != nil {
		slog.Error("RoleAdminHandler.GetRole: failed to get role", "role_id", id, "error", err)
		FailFromError(c, err)
		return
	}

	if role == nil {
		NotFound(c, "role not found")
		return
	}

	// 获取角色权限
	bdopsflow_permissions, err := h.svc.GetRolePermissions(ctx, id)
	if err != nil {
		slog.Error("RoleAdminHandler.GetRole: failed to get role bdopsflow_permissions", "role_id", id, "error", err)
	}

	Success(c, gin.H{
		"role":        role,
		"bdopsflow_permissions": bdopsflow_permissions,
	})
}

// CreateRole 创建角色
func (h *RoleAdminHandler) CreateRole(c *gin.Context) {
	ctx := c.Request.Context()

	var req model.RoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("RoleAdminHandler.CreateRole: invalid request", "error", err)
		BadRequest(c, err.Error())
		return
	}

	slog.Debug("RoleAdminHandler.CreateRole: handling request", "name", req.Name)

	var domainID *int64
	if req.DomainID != nil && *req.DomainID > 0 {
		domainID = req.DomainID
	}

	role, err := h.svc.CreateRole(ctx, req.Name, req.Code, req.Description, domainID)
	if err != nil {
		slog.Error("RoleAdminHandler.CreateRole: failed to create role", "name", req.Name, "error", err)
		FailFromError(c, err)
		return
	}

	c.Set("audit_resource_id", strconv.FormatInt(role.ID, 10))
	c.Set("audit_resource_name", role.Name)
	Created(c, role)
}

// UpdateRole 更新角色
func (h *RoleAdminHandler) UpdateRole(c *gin.Context) {
	ctx := c.Request.Context()
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		slog.Warn("RoleAdminHandler.UpdateRole: invalid id", "id_str", idStr, "error", err)
		BadRequest(c, "invalid id")
		return
	}

	if id <= 0 {
		slog.Warn("RoleAdminHandler.UpdateRole: id must be positive", "id", id)
		BadRequest(c, "id must be positive")
		return
	}

	var req struct {
		Name        string `json:"name" binding:"required,min=2,max=100"`
		Description string `json:"description" binding:"max=500"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("RoleAdminHandler.UpdateRole: invalid request", "error", err)
		BadRequest(c, err.Error())
		return
	}

	slog.Debug("RoleAdminHandler.UpdateRole: handling request", "role_id", id)

	role, err := h.svc.UpdateRole(ctx, id, req.Name, req.Description)
	if err != nil {
		if err == service.ErrSystemRoleCannotModify {
			BadRequest(c, err.Error())
			return
		}
		slog.Error("RoleAdminHandler.UpdateRole: failed to update role", "role_id", id, "error", err)
		FailFromError(c, err)
		return
	}

	if role == nil {
		NotFound(c, "role not found")
		return
	}

	c.Set("audit_resource_id", strconv.FormatInt(id, 10))
	c.Set("audit_resource_name", role.Name)
	Success(c, role)
}

// DeleteRole 删除角色
func (h *RoleAdminHandler) DeleteRole(c *gin.Context) {
	ctx := c.Request.Context()
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		slog.Warn("RoleAdminHandler.DeleteRole: invalid id", "id_str", idStr, "error", err)
		BadRequest(c, "invalid id")
		return
	}

	if id <= 0 {
		slog.Warn("RoleAdminHandler.DeleteRole: id must be positive", "id", id)
		BadRequest(c, "id must be positive")
		return
	}

	slog.Debug("RoleAdminHandler.DeleteRole: handling request", "role_id", id)

	// 先获取角色信息用于审计日志
	role, _ := h.svc.GetRole(ctx, id)

	err = h.svc.DeleteRole(ctx, id)
	if err != nil {
		if err == service.ErrSystemRoleCannotDelete {
			BadRequest(c, err.Error())
			return
		}
		if err == service.ErrRoleNotFound {
			NotFound(c, err.Error())
			return
		}
		slog.Error("RoleAdminHandler.DeleteRole: failed to delete role", "role_id", id, "error", err)
		FailFromError(c, err)
		return
	}

	c.Set("audit_resource_id", strconv.FormatInt(id, 10))
	if role != nil {
		c.Set("audit_resource_name", role.Name)
	}
	Success(c, nil)
}

// GetRolePermissions 获取角色权限
func (h *RoleAdminHandler) GetRolePermissions(c *gin.Context) {
	ctx := c.Request.Context()
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		slog.Warn("RoleAdminHandler.GetRolePermissions: invalid id", "id_str", idStr, "error", err)
		BadRequest(c, "invalid id")
		return
	}

	if id <= 0 {
		slog.Warn("RoleAdminHandler.GetRolePermissions: id must be positive", "id", id)
		BadRequest(c, "id must be positive")
		return
	}

	slog.Debug("RoleAdminHandler.GetRolePermissions: handling request", "role_id", id)

	bdopsflow_permissions, err := h.svc.GetRolePermissions(ctx, id)
	if err != nil {
		slog.Error("RoleAdminHandler.GetRolePermissions: failed to get role bdopsflow_permissions", "role_id", id, "error", err)
		FailFromError(c, err)
		return
	}

	Success(c, gin.H{"items": bdopsflow_permissions})
}

// AssignPermissions 分配权限给角色
func (h *RoleAdminHandler) AssignPermissions(c *gin.Context) {
	ctx := c.Request.Context()
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		slog.Warn("RoleAdminHandler.AssignPermissions: invalid id", "id_str", idStr, "error", err)
		BadRequest(c, "invalid id")
		return
	}

	if id <= 0 {
		slog.Warn("RoleAdminHandler.AssignPermissions: id must be positive", "id", id)
		BadRequest(c, "id must be positive")
		return
	}

	var req model.RolePermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("RoleAdminHandler.AssignPermissions: invalid request", "error", err)
		BadRequest(c, err.Error())
		return
	}

	slog.Debug("RoleAdminHandler.AssignPermissions: handling request", "role_id", id, "permission_count", len(req.PermissionIDs))

	err = h.svc.AssignPermissionsToRole(ctx, id, req.PermissionIDs)
	if err != nil {
		if err == service.ErrSystemRoleCannotModify {
			BadRequest(c, err.Error())
			return
		}
		if err == service.ErrRoleNotFound {
			NotFound(c, err.Error())
			return
		}
		slog.Error("RoleAdminHandler.AssignPermissions: failed to assign bdopsflow_permissions", "role_id", id, "error", err)
		FailFromError(c, err)
		return
	}

	SuccessWithMessage(c, "bdopsflow_permissions assigned successfully", nil)
}

// GetAllPermissions 获取所有权限
func (h *RoleAdminHandler) GetAllPermissions(c *gin.Context) {
	ctx := c.Request.Context()

	slog.Debug("RoleAdminHandler.GetAllPermissions: handling request")

	bdopsflow_permissions, err := h.svc.GetAllPermissions(ctx)
	if err != nil {
		slog.Error("RoleAdminHandler.GetAllPermissions: failed to get all bdopsflow_permissions", "error", err)
		FailFromError(c, err)
		return
	}

	groups := model.BuildPermissionGroups(bdopsflow_permissions)

	Success(c, gin.H{
		"items":  bdopsflow_permissions,
		"groups": groups,
	})
}
