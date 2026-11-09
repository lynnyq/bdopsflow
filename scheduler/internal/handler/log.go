package handler

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
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

func (h *LogHandler) List(c *gin.Context) {
	ctx := c.Request.Context()

	defer func() {
		if r := recover(); r != nil {
			slog.Error("LogHandler.List: panic recovered", "panic", r)
			InternalServerError(c, "internal server error")
		}
	}()

	slog.Debug("LogHandler.List: handling request", "query", c.Request.URL.RawQuery)

	domainID, _ := c.Get("current_domain_id")
	userRole, _ := c.Get("role")
	userID, _ := c.Get("user_id")

	var dID int64
	var role string
	var uid int64
	if v, ok := domainID.(int64); ok {
		dID = v
	}
	if v, ok := userRole.(string); ok {
		role = v
	}
	if v, ok := userID.(int64); ok {
		uid = v
	}

	var page, pageSize int
	if p, err := strconv.Atoi(c.DefaultQuery("page", "1")); err == nil && p > 0 {
		page = p
	} else {
		page = 1
	}
	if ps, err := strconv.Atoi(c.DefaultQuery("page_size", "20")); err == nil && ps > 0 {
		pageSize = ps
		if pageSize > 100 {
			pageSize = 100
		}
	} else {
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

	executions, total, err := h.svc.GetAllExecutions(ctx, dID, uid, role, filter, page, pageSize)
	if err != nil {
		slog.Error("LogHandler.List: failed to get executions", "error", err)
		InternalServerError(c, err.Error())
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

	Success(c, gin.H{
		"items":     response,
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
		BadRequest(c, "invalid id")
		return
	}

	if id <= 0 {
		slog.Warn("LogHandler.Delete: id must be positive", "id", id)
		BadRequest(c, "id must be positive")
		return
	}

	ctx := c.Request.Context()

	domainID, _ := c.Get("current_domain_id")
	userRole, _ := c.Get("role")
	userID, _ := c.Get("user_id")

	var dID int64
	var role string
	var uid int64
	if v, ok := domainID.(int64); ok {
		dID = v
	}
	if v, ok := userRole.(string); ok {
		role = v
	}
	if v, ok := userID.(int64); ok {
		uid = v
	}

	err = h.svc.DeleteExecutionWithDomainCheck(ctx, id, dID, uid, role)
	if err != nil {
		slog.Error("LogHandler.Delete: failed to delete execution", "id", id, "error", err)
		InternalServerError(c, err.Error())
		return
	}

	slog.Info("LogHandler.Delete: execution deleted", "id", id)
	SuccessWithMessage(c, "deleted successfully", nil)
}

func (h *LogHandler) BatchDelete(c *gin.Context) {
	ctx := c.Request.Context()

	defer func() {
		if r := recover(); r != nil {
			slog.Error("LogHandler.BatchDelete: panic recovered", "panic", r)
			InternalServerError(c, "internal server error")
		}
	}()

	domainID, _ := c.Get("current_domain_id")
	userRole, _ := c.Get("role")
	userID, _ := c.Get("user_id")

	var dID int64
	var role string
	var uid int64
	if v, ok := domainID.(int64); ok {
		dID = v
	}
	if v, ok := userRole.(string); ok {
		role = v
	}
	if v, ok := userID.(int64); ok {
		uid = v
	}

	var req struct {
		IDs []int64 `json:"ids"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("LogHandler.BatchDelete: invalid request body", "error", err)
		BadRequest(c, err.Error())
		return
	}

	if len(req.IDs) == 0 {
		slog.Warn("LogHandler.BatchDelete: no ids provided")
		BadRequest(c, "no ids provided")
		return
	}

	err := h.svc.BatchDeleteExecutionsWithDomainCheck(ctx, req.IDs, dID, uid, role)
	if err != nil {
		slog.Error("LogHandler.BatchDelete: failed to batch delete executions", "count", len(req.IDs), "error", err)
		InternalServerError(c, err.Error())
		return
	}

	slog.Info("LogHandler.BatchDelete: executions deleted", "count", len(req.IDs))
	SuccessWithMessage(c, "deleted successfully", nil)
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
			InternalServerError(c, "internal server error")
		}
	}()

	slog.Debug("LogHandler.GetStats: handling request")

	domainID, _ := c.Get("current_domain_id")
	userRole, _ := c.Get("role")
	userID, _ := c.Get("user_id")

	var dID int64
	var role string
	var uid int64
	if v, ok := domainID.(int64); ok {
		dID = v
	}
	if v, ok := userRole.(string); ok {
		role = v
	}
	if v, ok := userID.(int64); ok {
		uid = v
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

	stats, err := h.svc.GetExecutionStats(ctx, dID, uid, role, filter)
	if err != nil {
		slog.Error("LogHandler.GetStats: failed to get stats", "error", err)
		InternalServerError(c, err.Error())
		return
	}

	slog.Debug("LogHandler.GetStats: returning stats", "stats", stats)

	Success(c, stats)
}

// CleanExecutions 手动清理过期执行记录，级联删除关联的过程日志。
// 请求体可选字段：retention_days（显式覆盖系统配置）、time_from、time_to、task_name、executor_name。
// 至少要能算出一个 > 0 的 retention_days，否则返回 400。
func (h *LogHandler) CleanExecutions(c *gin.Context) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("LogHandler.CleanExecutions: panic recovered", "panic", r)
			InternalServerError(c, "internal server error")
		}
	}()

	type cleanRequest struct {
		RetentionDays *int   `json:"retention_days"`
		TimeFrom      string `json:"time_from"`
		TimeTo        string `json:"time_to"`
		TaskName      string `json:"task_name"`
		ExecutorName  string `json:"executor_name"`
	}

	var req cleanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 无 body 视为全默认，合法
		if err.Error() != "EOF" {
			slog.Warn("LogHandler.CleanExecutions: invalid request body", "error", err)
		}
	}

	// 计算最终 retention_days
	var retentionDays int
	if req.RetentionDays != nil && *req.RetentionDays > 0 {
		retentionDays = *req.RetentionDays
	} else {
		retentionDays = h.svc.GetExecutionRetentionDays()
	}

	if retentionDays <= 0 {
		BadRequest(c, "retention_days 必须 > 0 （系统配置为 0 表示自动清理已禁用，手动清理请指定具体天数）")
		return
	}

	// 时间格式校验
	timeFrom := strings.TrimSpace(req.TimeFrom)
	timeTo := strings.TrimSpace(req.TimeTo)
	if timeFrom != "" {
		if _, err := time.Parse("2006-01-02", timeFrom); err != nil {
			BadRequest(c, "time_from 格式错误，应为 YYYY-MM-DD")
			return
		}
	}
	if timeTo != "" {
		if _, err := time.Parse("2006-01-02", timeTo); err != nil {
			BadRequest(c, "time_to 格式错误，应为 YYYY-MM-DD")
			return
		}
	}
	if timeFrom != "" && timeTo != "" && timeFrom > timeTo {
		BadRequest(c, "time_from 不能晚于 time_to")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()

	domainID, _ := c.Get("current_domain_id")
	userRole, _ := c.Get("role")
	userID, _ := c.Get("user_id")

	var dID int64
	var role string
	var uid int64
	if v, ok := domainID.(int64); ok {
		dID = v
	}
	if v, ok := userRole.(string); ok {
		role = v
	}
	if v, ok := userID.(int64); ok {
		uid = v
	}

	_ = dID
	_ = role
	_ = uid

	filter := service.ExecutionCleanupFilter{
		RetentionDays: retentionDays,
		TimeFrom:      timeFrom,
		TimeTo:        timeTo,
		TaskName:      req.TaskName,
		ExecutorName:  req.ExecutorName,
	}

	slog.Info("LogHandler.CleanExecutions: start",
		"retention_days", retentionDays,
		"domain_id", dID, "user_id", uid, "role", role)

	deleted, err := h.svc.CleanExpiredExecutions(ctx, filter)
	if err != nil {
		slog.Error("LogHandler.CleanExecutions: failed",
			"error", err, "deleted_so_far", deleted)
		InternalServerError(c, err.Error())
		return
	}

	slog.Info("LogHandler.CleanExecutions: done",
		"deleted_count", deleted, "retention_days", retentionDays,
		"task_name", req.TaskName, "executor_name", req.ExecutorName)

	Success(c, gin.H{
		"deleted_count":  deleted,
		"retention_days": retentionDays,
	})
}

// CleanTaskLogs 手动清理过期过程日志，不触碰执行记录。
// 请求体可选字段：retention_days（显式覆盖系统配置）、time_from、time_to、task_name、executor_name。
func (h *LogHandler) CleanTaskLogs(c *gin.Context) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("LogHandler.CleanTaskLogs: panic recovered", "panic", r)
			InternalServerError(c, "internal server error")
		}
	}()

	type cleanRequest struct {
		RetentionDays *int   `json:"retention_days"`
		TimeFrom      string `json:"time_from"`
		TimeTo        string `json:"time_to"`
		TaskName      string `json:"task_name"`
		ExecutorName  string `json:"executor_name"`
	}

	var req cleanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		if err.Error() != "EOF" {
			slog.Warn("LogHandler.CleanTaskLogs: invalid request body", "error", err)
		}
	}

	var retentionDays int
	if req.RetentionDays != nil && *req.RetentionDays > 0 {
		retentionDays = *req.RetentionDays
	} else {
		retentionDays = h.svc.GetTaskLogRetentionDays()
	}

	if retentionDays <= 0 {
		BadRequest(c, "retention_days 必须 > 0 （系统配置为 0 表示自动清理已禁用，手动清理请指定具体天数）")
		return
	}

	timeFrom := strings.TrimSpace(req.TimeFrom)
	timeTo := strings.TrimSpace(req.TimeTo)
	if timeFrom != "" {
		if _, err := time.Parse("2006-01-02", timeFrom); err != nil {
			BadRequest(c, "time_from 格式错误，应为 YYYY-MM-DD")
			return
		}
	}
	if timeTo != "" {
		if _, err := time.Parse("2006-01-02", timeTo); err != nil {
			BadRequest(c, "time_to 格式错误，应为 YYYY-MM-DD")
			return
		}
	}
	if timeFrom != "" && timeTo != "" && timeFrom > timeTo {
		BadRequest(c, "time_from 不能晚于 time_to")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()

	filter := service.TaskLogCleanupFilter{
		RetentionDays: retentionDays,
		TimeFrom:      timeFrom,
		TimeTo:        timeTo,
		TaskName:      req.TaskName,
		ExecutorName:  req.ExecutorName,
	}

	slog.Info("LogHandler.CleanTaskLogs: start",
		"retention_days", retentionDays)

	deleted, err := h.svc.CleanExpiredTaskLogs(ctx, filter)
	if err != nil {
		slog.Error("LogHandler.CleanTaskLogs: failed",
			"error", err, "deleted_so_far", deleted)
		InternalServerError(c, err.Error())
		return
	}

	slog.Info("LogHandler.CleanTaskLogs: done",
		"deleted_count", deleted, "retention_days", retentionDays,
		"task_name", req.TaskName, "executor_name", req.ExecutorName)

	Success(c, gin.H{
		"deleted_count":  deleted,
		"retention_days": retentionDays,
	})
}
