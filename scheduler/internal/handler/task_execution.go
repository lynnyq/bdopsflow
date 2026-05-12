package handler

import (
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
		c.JSON(400, gin.H{"error": "invalid id"})
		return
	}

	ctx := c.Request.Context()
	executions, err := h.svc.GetTaskExecutions(ctx, id)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, executions)
}
