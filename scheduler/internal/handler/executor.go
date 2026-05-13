package handler

import (
	"log/slog"
	"net/http"
	"strconv"

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

	defer func() {
		if r := recover(); r != nil {
			slog.Error("ExecutorHandler.List: panic recovered", "panic", r)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
	}()

	slog.Debug("ExecutorHandler.List: handling request")

	executors, err := h.svc.ListExecutors(ctx)
	if err != nil {
		slog.Error("ExecutorHandler.List: failed to list executors", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, executors)
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
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		slog.Warn("ExecutorHandler.Delete: invalid id", "id_str", idStr, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if id <= 0 {
		slog.Warn("ExecutorHandler.Delete: id must be positive", "id", id)
		c.JSON(http.StatusBadRequest, gin.H{"error": "id must be positive"})
		return
	}

	defer func() {
		if r := recover(); r != nil {
			slog.Error("ExecutorHandler.Delete: panic recovered", "panic", r)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
	}()

	query := `DELETE FROM executors WHERE id = ?`
	result, err := h.svc.DB.WriteOneParameterized(rqlite.ParameterizedStatement{
		Query:     query,
		Arguments: []interface{}{id},
	})
	if err != nil {
		slog.Error("ExecutorHandler.Delete: failed to delete executor", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if result.Err != nil {
		slog.Error("ExecutorHandler.Delete: delete executor error", "id", id, "error", result.Err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Err.Error()})
		return
	}

	slog.Info("ExecutorHandler.Delete: executor deleted", "id", id)
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}
