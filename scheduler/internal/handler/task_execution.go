package handler

import (
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/service"
)

type TaskExecutionHandler struct {
	svc *service.SchedulerService
}

func NewTaskExecutionHandler(svc *service.SchedulerService) *TaskExecutionHandler {
	return &TaskExecutionHandler{svc: svc}
}

func (h *TaskExecutionHandler) ListByTask(c *gin.Context) {
	idStr := c.Param("task_id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		slog.Warn("TaskExecutionHandler.ListByTask: invalid id", "id_str", idStr, "error", err)
		BadRequest(c, "invalid id")
		return
	}

	if id <= 0 {
		slog.Warn("TaskExecutionHandler.ListByTask: id must be positive", "id", id)
		BadRequest(c, "id must be positive")
		return
	}

	ctx := c.Request.Context()
	executions, err := h.svc.GetTaskExecutions(ctx, id)
	if err != nil {
		slog.Error("TaskExecutionHandler.ListByTask: failed to get executions", "task_id", id, "error", err)
		FailFromError(c, err)
		return
	}

	var response []*TaskExecutionResponse
	for _, exec := range executions {
		response = append(response, toTaskExecutionResponse(exec))
	}

	Success(c, response)
}
