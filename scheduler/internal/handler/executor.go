package handler

import (
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"github.com/lynnyq/bdopsflow/scheduler/internal/service"
)

const heartbeatTimeout = 30

type ExecutorHandler struct {
	svc *service.SchedulerService
}

func NewExecutorHandler(svc *service.SchedulerService) *ExecutorHandler {
	return &ExecutorHandler{svc: svc}
}

func parseName(nameStr string) (string, error) {
	if nameStr == "" {
		return "", fmt.Errorf("name cannot be empty")
	}
	return nameStr, nil
}

func parseParam(s string, handler func(int64)) (bool, error) {
	if s == "" {
		return false, fmt.Errorf("parameter is required")
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return false, err
	}
	if v <= 0 {
		return false, fmt.Errorf("parameter must be positive")
	}
	handler(v)
	return true, nil
}

type ExecutorDTO struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	Address        string `json:"address"`
	Status         string `json:"status"`
	LastHeartbeat  string `json:"last_heartbeat"`
	Capacity       int64  `json:"capacity"`
	CurrentLoad    int64  `json:"current_load"`
	IsGlobal       bool   `json:"is_global"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

func executorToDTO(exec *model.Executor) *ExecutorDTO {
	actualStatus := exec.Status
	if exec.Status == "online" {
		if !exec.LastHeartbeat.Valid || exec.LastHeartbeat.Time.IsZero() {
			actualStatus = "offline"
		} else {
			localTime := service.ConvertToLocalTime(exec.LastHeartbeat.Time)
			if time.Since(localTime) > time.Duration(heartbeatTimeout)*time.Second {
				actualStatus = "offline"
			}
		}
	}

	dto := &ExecutorDTO{
		ID:          exec.ID,
		Name:        exec.Name,
		Address:     exec.Address,
		Status:      actualStatus,
		Capacity:    exec.Capacity,
		CurrentLoad: exec.CurrentLoad,
		IsGlobal:    exec.IsGlobal,
	}

	if exec.LastHeartbeat.Valid {
		dto.LastHeartbeat = service.FormatTimeInLocal(exec.LastHeartbeat.Time)
	}

	if !exec.CreatedAt.IsZero() {
		dto.CreatedAt = exec.CreatedAt.Format(TimeResponseFormat)
	}

	if !exec.UpdatedAt.IsZero() {
		dto.UpdatedAt = exec.UpdatedAt.Format(TimeResponseFormat)
	}

	return dto
}

func (h *ExecutorHandler) List(c *gin.Context) {
	ctx := c.Request.Context()

	defer func() {
		if r := recover(); r != nil {
			slog.Error("ExecutorHandler.List: panic recovered", "panic", r)
			InternalServerError(c, "internal server error")
		}
	}()

	slog.Debug("ExecutorHandler.List: handling request")

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	bdopsflow_executors, total, err := h.svc.ListExecutors(ctx, page, pageSize)
	if err != nil {
		slog.Error("ExecutorHandler.List: failed to list bdopsflow_executors", "error", err)
		FailFromError(c, err)
		return
	}

	var dtos []*ExecutorDTO
	for _, exec := range bdopsflow_executors {
		dtos = append(dtos, executorToDTO(exec))
	}

	Success(c, gin.H{"items": dtos, "total": total})
}

func (h *ExecutorHandler) Get(c *gin.Context) {
	nameStr := c.Param("name")

	_, err := parseName(nameStr)
	if err != nil {
		slog.Warn("ExecutorHandler.Get: invalid name", "name_str", nameStr, "error", err)
		BadRequest(c, "invalid name")
		return
	}

	SuccessWithMessage(c, "ok", nil)
}

func (h *ExecutorHandler) Delete(c *gin.Context) {
	nameStr := c.Param("name")
	name, err := parseName(nameStr)
	if err != nil {
		slog.Warn("ExecutorHandler.Delete: invalid name", "name_str", nameStr, "error", err)
		BadRequest(c, "invalid name")
		return
	}

	defer func() {
		if r := recover(); r != nil {
			slog.Error("ExecutorHandler.Delete: panic recovered", "panic", r)
			InternalServerError(c, "internal server error")
		}
	}()

	if err := h.svc.DeleteExecutorByName(c.Request.Context(), name); err != nil {
		slog.Error("ExecutorHandler.Delete: failed to delete executor", "name", name, "error", err)
		FailFromError(c, err)
		return
	}

	slog.Info("ExecutorHandler.Delete: executor deleted", "name", name)
	SuccessWithMessage(c, "deleted", nil)
}

func (h *ExecutorHandler) Online(c *gin.Context) {
	nameStr := c.Param("name")
	name, err := parseName(nameStr)
	if err != nil {
		slog.Warn("ExecutorHandler.Online: invalid name", "name_str", nameStr, "error", err)
		BadRequest(c, "invalid name")
		return
	}

	defer func() {
		if r := recover(); r != nil {
			slog.Error("ExecutorHandler.Online: panic recovered", "panic", r)
			InternalServerError(c, "internal server error")
		}
	}()

	if err := h.svc.SetExecutorStatusByName(c.Request.Context(), name, "online"); err != nil {
		slog.Error("ExecutorHandler.Online: failed to set executor online", "name", name, "error", err)
		FailFromError(c, err)
		return
	}

	slog.Info("ExecutorHandler.Online: executor set online", "name", name)
	SuccessWithMessage(c, "online", nil)
}

func (h *ExecutorHandler) Offline(c *gin.Context) {
	nameStr := c.Param("name")
	name, err := parseName(nameStr)
	if err != nil {
		slog.Warn("ExecutorHandler.Offline: invalid name", "name_str", nameStr, "error", err)
		BadRequest(c, "invalid name")
		return
	}

	defer func() {
		if r := recover(); r != nil {
			slog.Error("ExecutorHandler.Offline: panic recovered", "panic", r)
			InternalServerError(c, "internal server error")
		}
	}()

	if err := h.svc.SetExecutorStatusByName(c.Request.Context(), name, "offline"); err != nil {
		slog.Error("ExecutorHandler.Offline: failed to set executor offline", "name", name, "error", err)
		FailFromError(c, err)
		return
	}

	slog.Info("ExecutorHandler.Offline: executor set offline", "name", name)
	SuccessWithMessage(c, "offline", nil)
}

// UpdateCapacity 更新执行器容量
func (h *ExecutorHandler) UpdateCapacity(c *gin.Context) {
	nameStr := c.Param("name")
	name, err := parseName(nameStr)
	if err != nil {
		slog.Warn("ExecutorHandler.UpdateCapacity: invalid name", "name_str", nameStr, "error", err)
		BadRequest(c, "invalid name")
		return
	}

	var req struct {
		Capacity int64 `json:"capacity" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("ExecutorHandler.UpdateCapacity: invalid request", "error", err)
		BadRequest(c, "invalid request: capacity must be a positive integer")
		return
	}

	defer func() {
		if r := recover(); r != nil {
			slog.Error("ExecutorHandler.UpdateCapacity: panic recovered", "panic", r)
			InternalServerError(c, "internal server error")
		}
	}()

	if err := h.svc.UpdateExecutorCapacityByName(c.Request.Context(), name, req.Capacity); err != nil {
		slog.Error("ExecutorHandler.UpdateCapacity: failed to update executor capacity", "name", name, "error", err)
		FailFromError(c, err)
		return
	}

	slog.Info("ExecutorHandler.UpdateCapacity: executor capacity updated", "name", name, "capacity", req.Capacity)
	SuccessWithMessage(c, "capacity updated", gin.H{"capacity": req.Capacity})
}
