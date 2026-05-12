package handler

import (
	"context"
	"log/slog"
	"strconv"
	"time"

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

// TaskExecutionListResponse 用于 API 返回的任务执行记录列表
type TaskExecutionListResponse struct {
	ID           int64   `json:"id"`
	TaskID       int64   `json:"task_id"`
	ExecutionID  string  `json:"execution_id"`
	ExecutorID   string  `json:"executor_id"`
	ExecutorName *string `json:"executor_name"`
	TaskName     *string `json:"task_name"`
	TaskType     *string `json:"task_type"`
	Status       string  `json:"status"`
	StartTime    *string `json:"start_time"`
	EndTime      *string `json:"end_time"`
	Output       string  `json:"output"`
	Error        string  `json:"error"`
	RetryTimes   int32   `json:"retry_times"`
	CreatedAt    string  `json:"created_at"`
}

// PaginatedResponse 分页响应结构
type PaginatedResponse struct {
	Data     []*TaskExecutionListResponse `json:"data"`
	Total    int                          `json:"total"`
	Page     int                          `json:"page"`
	PageSize int                          `json:"page_size"`
}

func (h *LogHandler) toTaskExecutionListResponse(ctx context.Context, exec *model.TaskExecution) *TaskExecutionListResponse {
	resp := &TaskExecutionListResponse{
		ID:          exec.ID,
		TaskID:       exec.TaskID,
		ExecutionID: exec.ExecutionID,
		ExecutorID:  exec.ExecutorID,
		Status:      exec.Status,
		Output:      exec.Output,
		Error:       exec.Error,
		RetryTimes:   exec.RetryTimes,
		CreatedAt:    exec.CreatedAt.Format(time.RFC3339),
	}

	// 获取任务信息
	task, err := h.svc.GetTaskInfoByID(ctx, exec.TaskID)
	if err == nil {
		resp.TaskName = &task.Name
		resp.TaskType = &task.Type
	} else {
		slog.Warn("toTaskExecutionListResponse: failed to get task info", "task_id", exec.TaskID, "error", err)
	}

	// 获取执行器信息
	if exec.ExecutorID != "" {
		executor, err := h.svc.GetExecutorInfoByID(ctx, exec.ExecutorID)
		if err == nil {
			resp.ExecutorName = &executor.Name
		} else {
			slog.Warn("toTaskExecutionListResponse: failed to get executor info", "executor_id", exec.ExecutorID, "error", err)
		}
	}

	if exec.StartTime.Valid {
		t := exec.StartTime.Time.Format(time.RFC3339)
		resp.StartTime = &t
	}

	if exec.EndTime.Valid {
		t := exec.EndTime.Time.Format(time.RFC3339)
		resp.EndTime = &t
	}

	return resp
}

func (h *LogHandler) List(c *gin.Context) {
	ctx := c.Request.Context()

	slog.Debug("LogHandler.List: handling request", "query", c.Request.URL.RawQuery)

	defer func() {
		if r := recover(); r != nil {
			slog.Error("LogHandler.List: recovered from panic", "panic", r)
			c.JSON(500, gin.H{"error": "internal server error"})
		}
	}()

	// 解析分页参数
	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("page_size", "20")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}
	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil || pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// 构建筛选条件
	filter := make(map[string]string)
	filter["executor_name"] = c.Query("executor_name")
	filter["task_name"] = c.Query("task_name")
	filter["task_type"] = c.Query("task_type")
	filter["status"] = c.Query("status")

	executions, total, err := h.svc.GetAllExecutions(ctx, filter, page, pageSize)
	if err != nil {
		slog.Error("LogHandler.List: failed to get executions", "error", err)
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	var response []*TaskExecutionListResponse
	for _, exec := range executions {
		resp := &TaskExecutionListResponse{
			ID:          exec.ID,
			TaskID:      exec.TaskID,
			ExecutionID: exec.ExecutionID,
			ExecutorID:  exec.ExecutorID,
			Status:      exec.Status,
			Output:      exec.Output,
			Error:       exec.Error,
			RetryTimes:  exec.RetryTimes,
			CreatedAt:   exec.CreatedAt.Format(time.RFC3339),
		}

		// 直接使用 JOIN 出来的字段
		if exec.TaskName != "" {
			resp.TaskName = &exec.TaskName
		}
		if exec.TaskType != "" {
			resp.TaskType = &exec.TaskType
		}
		if exec.ExecutorName != "" {
			resp.ExecutorName = &exec.ExecutorName
		} else if exec.ExecutorID != "" {
			// 如果没有 executor_name 但有 executor_id，使用 executor_id 作为显示
			displayExecutorID := exec.ExecutorID
			resp.ExecutorName = &displayExecutorID
		}

		if exec.StartTime.Valid {
			t := exec.StartTime.Time.Format(time.RFC3339)
			resp.StartTime = &t
		}
		if exec.EndTime.Valid {
			t := exec.EndTime.Time.Format(time.RFC3339)
			resp.EndTime = &t
		}

		response = append(response, resp)
	}

	slog.Debug("LogHandler.List: returning response", "count", len(response), "total", total)

	c.JSON(200, PaginatedResponse{
		Data:     response,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

// Delete 删除单个执行记录
func (h *LogHandler) Delete(c *gin.Context) {
	ctx := c.Request.Context()

	slog.Debug("LogHandler.Delete: handling request", "id", c.Param("id"))

	defer func() {
		if r := recover(); r != nil {
			slog.Error("LogHandler.Delete: recovered from panic", "panic", r)
			c.JSON(500, gin.H{"error": "internal server error"})
		}
	}()

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid id"})
		return
	}

	err = h.svc.DeleteExecution(ctx, id)
	if err != nil {
		slog.Error("LogHandler.Delete: failed to delete execution", "id", id, "error", err)
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "deleted successfully"})
}

// BatchDelete 批量删除执行记录
func (h *LogHandler) BatchDelete(c *gin.Context) {
	ctx := c.Request.Context()

	slog.Debug("LogHandler.BatchDelete: handling request")

	defer func() {
		if r := recover(); r != nil {
			slog.Error("LogHandler.BatchDelete: recovered from panic", "panic", r)
			c.JSON(500, gin.H{"error": "internal server error"})
		}
	}()

	var req struct {
		IDs []int64 `json:"ids"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if len(req.IDs) == 0 {
		c.JSON(400, gin.H{"error": "no ids provided"})
		return
	}

	err := h.svc.BatchDeleteExecutions(ctx, req.IDs)
	if err != nil {
		slog.Error("LogHandler.BatchDelete: failed to batch delete executions", "count", len(req.IDs), "error", err)
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "deleted successfully"})
}
