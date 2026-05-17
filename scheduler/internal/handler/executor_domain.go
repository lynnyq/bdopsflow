package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"github.com/lynnyq/bdopsflow/scheduler/internal/service"
)

type ExecutorDomainHandler struct {
	svc           *service.ExecutorDomainService
	permissionSvc *service.PermissionService
	userAdminSvc  *service.UserAdminService
}

func NewExecutorDomainHandler(svc *service.ExecutorDomainService, permissionSvc *service.PermissionService, userAdminSvc *service.UserAdminService) *ExecutorDomainHandler {
	return &ExecutorDomainHandler{
		svc:           svc,
		permissionSvc: permissionSvc,
		userAdminSvc:  userAdminSvc,
	}
}

type ExecutorWithDomainsDTO struct {
	ID             int64           `json:"id"`
	Name           string          `json:"name"`
	Address        string          `json:"address"`
	Status         string          `json:"status"`
	LastHeartbeat  string          `json:"last_heartbeat"`
	Capacity       int64           `json:"capacity"`
	CurrentLoad    int64           `json:"current_load"`
	IsGlobal       bool            `json:"is_global"`
	CreatedAt      string          `json:"created_at"`
	UpdatedAt      string          `json:"updated_at"`
	Domains        []*model.Domain `json:"domains"`
}

func executorWithDomainsToDTO(exec *model.ExecutorWithDomains) *ExecutorWithDomainsDTO {
	dto := &ExecutorWithDomainsDTO{
		ID:          exec.ID,
		Name:        exec.Name,
		Address:     exec.Address,
		Status:      exec.Status,
		Capacity:    exec.Capacity,
		CurrentLoad: exec.CurrentLoad,
		IsGlobal:    exec.IsGlobal,
		Domains:     exec.Domains,
	}

	if exec.LastHeartbeat.Valid {
		dto.LastHeartbeat = exec.LastHeartbeat.Time.Format("2006-01-02 15:04:05")
	}

	if !exec.CreatedAt.IsZero() {
		dto.CreatedAt = exec.CreatedAt.Format("2006-01-02 15:04:05")
	}

	if !exec.UpdatedAt.IsZero() {
		dto.UpdatedAt = exec.UpdatedAt.Format("2006-01-02 15:04:05")
	}

	return dto
}

func (h *ExecutorDomainHandler) GetExecutorDomains(c *gin.Context) {
	ctx := c.Request.Context()
	name := c.Param("name")

	if name == "" {
		slog.Warn("ExecutorDomainHandler.GetExecutorDomains: name is required", "name", name)
		c.JSON(http.StatusBadRequest, gin.H{"error": "executor name is required"})
		return
	}

	slog.Debug("ExecutorDomainHandler.GetExecutorDomains: handling request", "executor_name", name)

	executor, err := h.svc.GetExecutorByName(ctx, name)
	if err != nil {
		slog.Error("ExecutorDomainHandler.GetExecutorDomains: executor not found", "executor_name", name, "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "executor not found"})
		return
	}

	domains, err := h.svc.GetExecutorDomains(ctx, executor.ID)
	if err != nil {
		slog.Error("ExecutorDomainHandler.GetExecutorDomains: failed to get executor domains", "executor_name", name, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": domains})
}

func (h *ExecutorDomainHandler) AssignDomains(c *gin.Context) {
	ctx := c.Request.Context()
	name := c.Param("name")

	if name == "" {
		slog.Warn("ExecutorDomainHandler.AssignDomains: name is required", "name", name)
		c.JSON(http.StatusBadRequest, gin.H{"error": "executor name is required"})
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

	slog.Debug("ExecutorDomainHandler.AssignDomains: handling request", "executor_name", name, "domain_ids", req.DomainIDs)

	executor, err := h.svc.GetExecutorByName(ctx, name)
	if err != nil {
		slog.Error("ExecutorDomainHandler.AssignDomains: executor not found", "executor_name", name, "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "executor not found"})
		return
	}

	err = h.svc.AssignExecutorToDomains(ctx, executor.ID, req.DomainIDs, assignedBy)
	if err != nil {
		slog.Error("ExecutorDomainHandler.AssignDomains: failed to assign domains", "executor_name", name, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "domains assigned successfully"})
}

func (h *ExecutorDomainHandler) RemoveDomain(c *gin.Context) {
	ctx := c.Request.Context()
	name := c.Param("name")

	if name == "" {
		slog.Warn("ExecutorDomainHandler.RemoveDomain: name is required", "name", name)
		c.JSON(http.StatusBadRequest, gin.H{"error": "executor name is required"})
		return
	}

	domainIDStr := c.Param("domainId")
	domainID, err := parseInt64Param(domainIDStr)
	if err != nil {
		slog.Warn("ExecutorDomainHandler.RemoveDomain: invalid domain id", "domain_id_str", domainIDStr, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid domain id"})
		return
	}

	slog.Debug("ExecutorDomainHandler.RemoveDomain: handling request", "executor_name", name, "domain_id", domainID)

	executor, err := h.svc.GetExecutorByName(ctx, name)
	if err != nil {
		slog.Error("ExecutorDomainHandler.RemoveDomain: executor not found", "executor_name", name, "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "executor not found"})
		return
	}

	err = h.svc.RemoveExecutorFromDomain(ctx, executor.ID, domainID)
	if err != nil {
		slog.Error("ExecutorDomainHandler.RemoveDomain: failed to remove domain", "executor_name", name, "domain_id", domainID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "domain removed successfully"})
}

func (h *ExecutorDomainHandler) GetExecutorsWithDomains(c *gin.Context) {
	ctx := c.Request.Context()

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	uid, ok := userID.(int64)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user"})
		return
	}

	user, err := h.userAdminSvc.GetUserByID(ctx, uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user"})
		return
	}

	executors, err := h.svc.GetExecutorsByUserRole(ctx, user.Role, user.DomainID)
	if err != nil {
		slog.Error("ExecutorDomainHandler.GetExecutorsWithDomains: failed to get executors", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var dtos []*ExecutorWithDomainsDTO
	for _, exec := range executors {
		dtos = append(dtos, executorWithDomainsToDTO(exec))
	}

	c.JSON(http.StatusOK, gin.H{"items": dtos})
}

func (h *ExecutorDomainHandler) GetAssignedTasks(c *gin.Context) {
	ctx := c.Request.Context()
	name := c.Param("name")

	if name == "" {
		slog.Warn("ExecutorDomainHandler.GetAssignedTasks: name is required", "name", name)
		c.JSON(http.StatusBadRequest, gin.H{"error": "executor name is required"})
		return
	}

	slog.Debug("ExecutorDomainHandler.GetAssignedTasks: handling request", "executor_name", name)

	executor, err := h.svc.GetExecutorByName(ctx, name)
	if err != nil {
		slog.Error("ExecutorDomainHandler.GetAssignedTasks: executor not found", "executor_name", name, "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "executor not found"})
		return
	}

	taskCount, err := h.svc.GetAssignedTasksForExecutor(ctx, executor.ID)
	if err != nil {
		slog.Error("ExecutorDomainHandler.GetAssignedTasks: failed to get tasks", "executor_name", name, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	taskNames, err := h.svc.GetAssignedTaskNamesForExecutor(ctx, executor.ID)
	if err != nil {
		slog.Error("ExecutorDomainHandler.GetAssignedTasks: failed to get task names", "executor_name", name, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"task_count": taskCount, "task_names": taskNames})
}

func (h *ExecutorDomainHandler) CanDeleteExecutor(c *gin.Context) {
	ctx := c.Request.Context()
	name := c.Param("name")

	if name == "" {
		slog.Warn("ExecutorDomainHandler.CanDeleteExecutor: name is required", "name", name)
		c.JSON(http.StatusBadRequest, gin.H{"error": "executor name is required"})
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	uid, ok := userID.(int64)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user"})
		return
	}

	user, err := h.userAdminSvc.GetUserByID(ctx, uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get user"})
		return
	}

	executor, err := h.svc.GetExecutorByName(ctx, name)
	if err != nil {
		slog.Error("ExecutorDomainHandler.CanDeleteExecutor: executor not found", "executor_name", name, "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "executor not found"})
		return
	}

	taskCount, err := h.svc.GetAssignedTasksForExecutor(ctx, executor.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var canDelete bool
	var reason string

	if user.Role == "system_admin" || user.Role == "admin" {
		canDelete = true
	} else {
		canDelete, err = h.svc.CanDomainAdminDeleteExecutor(ctx, executor.ID, user.DomainID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if !canDelete {
			reason = "executor is assigned to multiple domains, only system admin can delete"
		}
	}

	hasTasks := taskCount > 0

	c.JSON(http.StatusOK, gin.H{
		"can_delete": canDelete,
		"reason":     reason,
		"has_tasks":  hasTasks,
		"task_count": taskCount,
	})
}

func parseInt64Param(s string) (int64, error) {
	var result int64
	_, err := parseParam(s, func(v int64) { result = v })
	return result, err
}
