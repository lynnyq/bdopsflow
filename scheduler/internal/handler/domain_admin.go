package handler

import (
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/service"
)

// DomainAdminHandler 领域管理 Handler
type DomainAdminHandler struct {
	svc *service.DomainAdminService
}

// NewDomainAdminHandler 创建领域管理 Handler
func NewDomainAdminHandler(svc *service.DomainAdminService) *DomainAdminHandler {
	return &DomainAdminHandler{svc: svc}
}

// ListDomains 获取领域列表
func (h *DomainAdminHandler) ListDomains(c *gin.Context) {
	ctx := c.Request.Context()
	defer func() {
		if r := recover(); r != nil {
			slog.Error("DomainAdminHandler.ListDomains: panic recovered", "panic", r)
			InternalServerError(c, "internal server error")
		}
	}()

	slog.Debug("DomainAdminHandler.ListDomains: handling request")

	bdopsflow_domains, err := h.svc.ListDomains(ctx)
	if err != nil {
		slog.Error("DomainAdminHandler.ListDomains: failed to list bdopsflow_domains", "error", err)
		FailFromError(c, err)
		return
	}

	Success(c, gin.H{"items": bdopsflow_domains})
}

// GetDomain 获取领域详情
func (h *DomainAdminHandler) GetDomain(c *gin.Context) {
	ctx := c.Request.Context()
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		slog.Warn("DomainAdminHandler.GetDomain: invalid id", "id_str", idStr, "error", err)
		BadRequest(c, "invalid id")
		return
	}

	if id <= 0 {
		slog.Warn("DomainAdminHandler.GetDomain: id must be positive", "id", id)
		BadRequest(c, "id must be positive")
		return
	}

	slog.Debug("DomainAdminHandler.GetDomain: handling request", "domain_id", id)

	domain, err := h.svc.GetDomain(ctx, id)
	if err != nil {
		slog.Error("DomainAdminHandler.GetDomain: failed to get domain", "domain_id", id, "error", err)
		FailFromError(c, err)
		return
	}

	if domain == nil {
		NotFound(c, "domain not found")
		return
	}

	Success(c, domain)
}

// CreateDomain 创建领域
func (h *DomainAdminHandler) CreateDomain(c *gin.Context) {
	ctx := c.Request.Context()

	var req struct {
		Name        string `json:"name" binding:"required,min=2,max=100"`
		Description string `json:"description" binding:"max=500"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("DomainAdminHandler.CreateDomain: invalid request", "error", err)
		BadRequest(c, err.Error())
		return
	}

	slog.Debug("DomainAdminHandler.CreateDomain: handling request", "name", req.Name)

	domain, err := h.svc.CreateDomain(ctx, req.Name, req.Description)
	if err != nil {
		slog.Error("DomainAdminHandler.CreateDomain: failed to create domain", "name", req.Name, "error", err)
		FailFromError(c, err)
		return
	}

	Created(c, domain)
}

// UpdateDomain 更新领域
func (h *DomainAdminHandler) UpdateDomain(c *gin.Context) {
	ctx := c.Request.Context()
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		slog.Warn("DomainAdminHandler.UpdateDomain: invalid id", "id_str", idStr, "error", err)
		BadRequest(c, "invalid id")
		return
	}

	if id <= 0 {
		slog.Warn("DomainAdminHandler.UpdateDomain: id must be positive", "id", id)
		BadRequest(c, "id must be positive")
		return
	}

	var req struct {
		Name        string `json:"name" binding:"required,min=2,max=100"`
		Description string `json:"description" binding:"max=500"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("DomainAdminHandler.UpdateDomain: invalid request", "error", err)
		BadRequest(c, err.Error())
		return
	}

	slog.Debug("DomainAdminHandler.UpdateDomain: handling request", "domain_id", id)

	domain, err := h.svc.UpdateDomain(ctx, id, req.Name, req.Description)
	if err != nil {
		slog.Error("DomainAdminHandler.UpdateDomain: failed to update domain", "domain_id", id, "error", err)
		FailFromError(c, err)
		return
	}

	if domain == nil {
		NotFound(c, "domain not found")
		return
	}

	Success(c, domain)
}

// DeleteDomain 删除领域
func (h *DomainAdminHandler) DeleteDomain(c *gin.Context) {
	ctx := c.Request.Context()
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		slog.Warn("DomainAdminHandler.DeleteDomain: invalid id", "id_str", idStr, "error", err)
		BadRequest(c, "invalid id")
		return
	}

	if id <= 0 {
		slog.Warn("DomainAdminHandler.DeleteDomain: id must be positive", "id", id)
		BadRequest(c, "id must be positive")
		return
	}

	slog.Debug("DomainAdminHandler.DeleteDomain: handling request", "domain_id", id)

	err = h.svc.DeleteDomain(ctx, id)
	if err != nil {
		if err == service.ErrDomainHasResources {
			BadRequest(c, err.Error())
			return
		}
		if err == service.ErrDomainNotFound {
			NotFound(c, err.Error())
			return
		}
		slog.Error("DomainAdminHandler.DeleteDomain: failed to delete domain", "domain_id", id, "error", err)
		FailFromError(c, err)
		return
	}

	Success(c, nil)
}
