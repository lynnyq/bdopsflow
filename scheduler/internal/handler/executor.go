package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/service"
	rqlite "github.com/rqlite/gorqlite"
)

type ExecutorHandler struct {
	svc *service.SchedulerService
}

func NewExecutorHandler(svc *service.SchedulerService) *ExecutorHandler {
	return &ExecutorHandler{svc: svc}
}

func (h *ExecutorHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	executors, err := h.svc.ListExecutors(ctx)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, executors)
}

func (h *ExecutorHandler) Get(c *gin.Context) {
	c.JSON(200, gin.H{"message": "ok"})
}

func (h *ExecutorHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")

	query := `DELETE FROM executors WHERE id = ?`
	result, err := h.svc.DB.WriteOneParameterized(rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{idStr},
	})
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	if result.Err != nil {
		c.JSON(500, gin.H{"error": result.Err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "deleted"})
}