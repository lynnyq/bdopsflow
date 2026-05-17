package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"github.com/lynnyq/bdopsflow/scheduler/internal/service"
)

type ExecutorHandler struct {
	svc *service.SchedulerService
}

func NewExecutorHandler(svc *service.SchedulerService) *ExecutorHandler {
	return &ExecutorHandler{svc: svc}
}

type ExecutorDTO struct {
	ID             int64  `json:"id"`
	ExecutorID     string `json:"executor_id"`
	Name           string `json:"name"`
	Address        string `json:"address"`
	Status         string `json:"status"`
	LastHeartbeat  string `json:"last_heartbeat"`
	Capacity       int64  `json:"capacity"`
	CurrentLoad    int64  `json:"current_load"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

func executorToDTO(exec *model.Executor) *ExecutorDTO {
	dto := &ExecutorDTO{
		ID:          exec.ID,
		ExecutorID:  exec.ExecutorID,
		Name:        exec.Name,
		Address:     exec.Address,
		Status:      exec.Status,
		Capacity:    exec.Capacity,
		CurrentLoad: exec.CurrentLoad,
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

func (h *ExecutorHandler) List(c *gin.Context) {
	ctx := c.Request.Context()

	defer func() {
		if r := recover(); r != nil {
			slog.Error("ExecutorHandler.List: panic recovered", "panic", r)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
	}()

	slog.Debug("ExecutorHandler.List: handling request")

	bdopsflow_executors, err := h.svc.ListExecutors(ctx)
	if err != nil {
		slog.Error("ExecutorHandler.List: failed to list bdopsflow_executors", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var dtos []*ExecutorDTO
	for _, exec := range bdopsflow_executors {
		dtos = append(dtos, executorToDTO(exec))
	}

	c.JSON(http.StatusOK, dtos)
}

func (h *ExecutorHandler) Get(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		slog.Warn("ExecutorHandler.Get: invalid id", "id_str", idStr, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if id <= 0 {
		slog.Warn("ExecutorHandler.Get: id must be positive", "id", id)
		c.JSON(http.StatusBadRequest, gin.H{"error": "id must be positive"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

func (h *ExecutorHandler) Delete(c *gin.Context) {
	executorID := c.Param("id")
	if executorID == "" {
		slog.Warn("ExecutorHandler.Delete: executor_id is required")
		c.JSON(http.StatusBadRequest, gin.H{"error": "executor_id is required"})
		return
	}

	defer func() {
		if r := recover(); r != nil {
			slog.Error("ExecutorHandler.Delete: panic recovered", "panic", r)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
	}()

	if err := h.svc.DeleteExecutor(c.Request.Context(), executorID); err != nil {
		slog.Error("ExecutorHandler.Delete: failed to delete executor", "executor_id", executorID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	slog.Info("ExecutorHandler.Delete: executor deleted", "executor_id", executorID)
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *ExecutorHandler) Online(c *gin.Context) {
	executorID := c.Param("id")
	if executorID == "" {
		slog.Warn("ExecutorHandler.Online: executor_id is required")
		c.JSON(http.StatusBadRequest, gin.H{"error": "executor_id is required"})
		return
	}

	defer func() {
		if r := recover(); r != nil {
			slog.Error("ExecutorHandler.Online: panic recovered", "panic", r)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
	}()

	if err := h.svc.SetExecutorStatus(c.Request.Context(), executorID, "online"); err != nil {
		slog.Error("ExecutorHandler.Online: failed to set executor online", "executor_id", executorID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	slog.Info("ExecutorHandler.Online: executor set online", "executor_id", executorID)
	c.JSON(http.StatusOK, gin.H{"message": "online"})
}

func (h *ExecutorHandler) Offline(c *gin.Context) {
	executorID := c.Param("id")
	if executorID == "" {
		slog.Warn("ExecutorHandler.Offline: executor_id is required")
		c.JSON(http.StatusBadRequest, gin.H{"error": "executor_id is required"})
		return
	}

	defer func() {
		if r := recover(); r != nil {
			slog.Error("ExecutorHandler.Offline: panic recovered", "panic", r)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
	}()

	if err := h.svc.SetExecutorStatus(c.Request.Context(), executorID, "offline"); err != nil {
		slog.Error("ExecutorHandler.Offline: failed to set executor offline", "executor_id", executorID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	slog.Info("ExecutorHandler.Offline: executor set offline", "executor_id", executorID)
	c.JSON(http.StatusOK, gin.H{"message": "offline"})
}

// UpdateCapacity 更新执行器容量
func (h *ExecutorHandler) UpdateCapacity(c *gin.Context) {
	executorID := c.Param("id")
	if executorID == "" {
		slog.Warn("ExecutorHandler.UpdateCapacity: executor_id is required")
		c.JSON(http.StatusBadRequest, gin.H{"error": "executor_id is required"})
		return
	}

	var req struct {
		Capacity int64 `json:"capacity" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("ExecutorHandler.UpdateCapacity: invalid request", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: capacity must be a positive integer"})
		return
	}

	defer func() {
		if r := recover(); r != nil {
			slog.Error("ExecutorHandler.UpdateCapacity: panic recovered", "panic", r)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
	}()

	if err := h.svc.UpdateExecutorCapacity(c.Request.Context(), executorID, req.Capacity); err != nil {
		slog.Error("ExecutorHandler.UpdateCapacity: failed to update executor capacity", "executor_id", executorID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	slog.Info("ExecutorHandler.UpdateCapacity: executor capacity updated", "executor_id", executorID, "capacity", req.Capacity)
	c.JSON(http.StatusOK, gin.H{"message": "capacity updated", "capacity": req.Capacity})
}
