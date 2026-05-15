package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"github.com/lynnyq/bdopsflow/scheduler/internal/service"
)

type LogHandler struct {
	svc *service.SchedulerService
}

func NewLogHandler(svc *service.SchedulerService) *LogHandler {
	return &LogHandler{svc: svc}
}

func (h *LogHandler) List(c *gin.Context) {
	ctx := c.Request.Context()

	defer func() {
		if r := recover(); r != nil {
			slog.Error("LogHandler.List: panic recovered", "panic", r)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
	}()

	slog.Debug("LogHandler.List: handling request", "query", c.Request.URL.RawQuery)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	filter := make(map[string]string)
	filter["id"] = c.Query("id")
	filter["execution_id"] = c.Query("execution_id")
	filter["executor_name"] = c.Query("executor_name")
	filter["task_name"] = c.Query("task_name")
	filter["status"] = c.Query("status")
	filter["start_time_from"] = c.Query("start_time_from")
	filter["start_time_to"] = c.Query("start_time_to")
	filter["end_time_from"] = c.Query("end_time_from")
	filter["end_time_to"] = c.Query("end_time_to")
	filter["duration_min"] = c.Query("duration_min")
	filter["duration_max"] = c.Query("duration_max")

	executions, total, err := h.svc.GetAllExecutions(ctx, filter, page, pageSize)
	if err != nil {
		slog.Error("LogHandler.List: failed to get executions", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var response []interface{}
	for _, exec := range executions {
		resp := toTaskExecutionResponse(&exec.TaskExecution)
		if exec.TaskName != "" {
			resp.TaskName = &exec.TaskName
		}
		if exec.TaskType != "" {
			resp.TaskType = &exec.TaskType
		}
		if exec.ExecutorName != "" {
			resp.ExecutorName = &exec.ExecutorName
		}
		response = append(response, resp)
	}

	slog.Debug("LogHandler.List: returning response", "count", len(response), "total", total)

	c.JSON(http.StatusOK, gin.H{
		"data":      response,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (h *LogHandler) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		slog.Warn("LogHandler.Delete: invalid id", "id_str", idStr, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if id <= 0 {
		slog.Warn("LogHandler.Delete: id must be positive", "id", id)
		c.JSON(http.StatusBadRequest, gin.H{"error": "id must be positive"})
		return
	}

	ctx := c.Request.Context()
	err = h.svc.DeleteExecution(ctx, id)
	if err != nil {
		slog.Error("LogHandler.Delete: failed to delete execution", "id", id, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	slog.Info("LogHandler.Delete: execution deleted", "id", id)
	c.JSON(http.StatusOK, gin.H{"message": "deleted successfully"})
}

func (h *LogHandler) BatchDelete(c *gin.Context) {
	ctx := c.Request.Context()

	defer func() {
		if r := recover(); r != nil {
			slog.Error("LogHandler.BatchDelete: panic recovered", "panic", r)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
	}()

	var req struct {
		IDs []int64 `json:"ids"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("LogHandler.BatchDelete: invalid request body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(req.IDs) == 0 {
		slog.Warn("LogHandler.BatchDelete: no ids provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "no ids provided"})
		return
	}

	err := h.svc.BatchDeleteExecutions(ctx, req.IDs)
	if err != nil {
		slog.Error("LogHandler.BatchDelete: failed to batch delete executions", "count", len(req.IDs), "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	slog.Info("LogHandler.BatchDelete: executions deleted", "count", len(req.IDs))
	c.JSON(http.StatusOK, gin.H{"message": "deleted successfully"})
}

type TaskExecutionWithNames struct {
	model.TaskExecution
	TaskName     string `db:"task_name" json:"task_name"`
	TaskType     string `db:"task_type" json:"task_type"`
	ExecutorName string `db:"executor_name" json:"executor_name"`
}

func (h *LogHandler) GetStats(c *gin.Context) {
	ctx := c.Request.Context()

	defer func() {
		if r := recover(); r != nil {
			slog.Error("LogHandler.GetStats: panic recovered", "panic", r)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
	}()

	slog.Debug("LogHandler.GetStats: handling request")

	// 构建筛选条件
	filter := make(map[string]string)
	filter["id"] = c.Query("id")
	filter["execution_id"] = c.Query("execution_id")
	filter["executor_name"] = c.Query("executor_name")
	filter["task_name"] = c.Query("task_name")
	filter["status"] = c.Query("status")
	filter["start_time_from"] = c.Query("start_time_from")
	filter["start_time_to"] = c.Query("start_time_to")
	filter["end_time_from"] = c.Query("end_time_from")
	filter["end_time_to"] = c.Query("end_time_to")
	filter["duration_min"] = c.Query("duration_min")
	filter["duration_max"] = c.Query("duration_max")

	stats, err := h.svc.GetExecutionStats(ctx, filter)
	if err != nil {
		slog.Error("LogHandler.GetStats: failed to get stats", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	slog.Debug("LogHandler.GetStats: returning stats", "stats", stats)

	c.JSON(http.StatusOK, stats)
}
