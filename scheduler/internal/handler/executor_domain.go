package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/service"
)

// ExecutorDomainHandler 执行器领域分配 Handler
type ExecutorDomainHandler struct {
	svc *service.ExecutorDomainService
}

// NewExecutorDomainHandler 创建执行器领域分配 Handler
func NewExecutorDomainHandler(svc *service.ExecutorDomainService) *ExecutorDomainHandler {
	return &ExecutorDomainHandler{svc: svc}
}

// GetExecutorDomains 获取执行器所属的领域
func (h *ExecutorDomainHandler) GetExecutorDomains(c *gin.Context) {
	ctx := c.Request.Context()
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		slog.Warn("ExecutorDomainHandler.GetExecutorDomains: invalid id", "id_str", idStr, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if id <= 0 {
		slog.Warn("ExecutorDomainHandler.GetExecutorDomains: id must be positive", "id", id)
		c.JSON(http.StatusBadRequest, gin.H{"error": "id must be positive"})
		return
	}

	slog.Debug("ExecutorDomainHandler.GetExecutorDomains: handling request", "executor_id", id)

	domains, err := h.svc.GetExecutorDomains(ctx, id)
	if err != nil {
		slog.Error("ExecutorDomainHandler.GetExecutorDomains: failed to get executor domains", "executor_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": domains})
}

// AssignDomains 分配执行器到领域
func (h *ExecutorDomainHandler) AssignDomains(c *gin.Context) {
	ctx := c.Request.Context()
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		slog.Warn("ExecutorDomainHandler.AssignDomains: invalid id", "id_str", idStr, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if id <= 0 {
		slog.Warn("ExecutorDomainHandler.AssignDomains: id must be positive", "id", id)
		c.JSON(http.StatusBadRequest, gin.H{"error": "id must be positive"})
		return
	}

	var req struct {
		DomainIDs []int64 `json:"domain_ids" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("ExecutorDomainHandler.AssignDomains: invalid request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	// 获取当前用户ID
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	assignedBy, ok := userID.(int64)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user"})
		return
	}

	slog.Debug("ExecutorDomainHandler.AssignDomains: handling request", "executor_id", id, "domain_ids", req.DomainIDs)

	err = h.svc.AssignExecutorToDomains(ctx, id, req.DomainIDs, assignedBy)
	if err != nil {
		slog.Error("ExecutorDomainHandler.AssignDomains: failed to assign domains", "executor_id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "domains assigned successfully"})
}

// RemoveDomain 从领域移除执行器
func (h *ExecutorDomainHandler) RemoveDomain(c *gin.Context) {
	ctx := c.Request.Context()
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		slog.Warn("ExecutorDomainHandler.RemoveDomain: invalid id", "id_str", idStr, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if id <= 0 {
		slog.Warn("ExecutorDomainHandler.RemoveDomain: id must be positive", "id", id)
		c.JSON(http.StatusBadRequest, gin.H{"error": "id must be positive"})
		return
	}

	domainIDStr := c.Param("domainId")
	domainID, err := strconv.ParseInt(domainIDStr, 10, 64)
	if err != nil {
		slog.Warn("ExecutorDomainHandler.RemoveDomain: invalid domain id", "domain_id_str", domainIDStr, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid domain id"})
		return
	}

	if domainID <= 0 {
		slog.Warn("ExecutorDomainHandler.RemoveDomain: domain id must be positive", "domain_id", domainID)
		c.JSON(http.StatusBadRequest, gin.H{"error": "domain id must be positive"})
		return
	}

	slog.Debug("ExecutorDomainHandler.RemoveDomain: handling request", "executor_id", id, "domain_id", domainID)

	err = h.svc.RemoveExecutorFromDomain(ctx, id, domainID)
	if err != nil {
		slog.Error("ExecutorDomainHandler.RemoveDomain: failed to remove domain", "executor_id", id, "domain_id", domainID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "domain removed successfully"})
}
