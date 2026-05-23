package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/model"
	"github.com/lynnyq/bdopsflow/scheduler/internal/service"
)

type AuditLogHandler struct {
	service *service.AuditLogService
}

func NewAuditLogHandler(service *service.AuditLogService) *AuditLogHandler {
	return &AuditLogHandler{service: service}
}

func (h *AuditLogHandler) List(c *gin.Context) {
	var filter model.AuditLogFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		BadRequest(c, err.Error())
		return
	}

	logs, total, err := h.service.List(c.Request.Context(), filter)
	if err != nil {
		InternalServerError(c, "查询审计日志失败")
		return
	}

	Success(c, gin.H{
		"items": logs,
		"total": total,
		"page":  filter.Page,
		"page_size": filter.PageSize,
	})
}

func (h *AuditLogHandler) CleanExpired(c *gin.Context) {
	var req struct {
		RetentionDays int `json:"retention_days"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		req.RetentionDays = 0
	}

	if req.RetentionDays <= 0 {
		req.RetentionDays = h.service.GetRetentionDays()
	}

	deleted, err := h.service.CleanExpired(c.Request.Context(), req.RetentionDays)
	if err != nil {
		InternalServerError(c, "清理审计日志失败")
		return
	}

	SuccessWithMessage(c, "清理完成", gin.H{
		"deleted_count":  deleted,
		"retention_days": req.RetentionDays,
	})
}

func (h *AuditLogHandler) GetRetentionDays(c *gin.Context) {
	days := h.service.GetRetentionDays()
	Success(c, gin.H{
		"retention_days": days,
	})
}

func (h *AuditLogHandler) UpdateRetentionDays(c *gin.Context) {
	var req struct {
		RetentionDays int `json:"retention_days" binding:"required,min=1,max=3650"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, err.Error())
		return
	}

	Success(c, gin.H{
		"retention_days": req.RetentionDays,
	})
}

func (h *AuditLogHandler) GetStats(c *gin.Context) {
	filter := model.AuditLogFilter{
		Page:     1,
		PageSize: 1,
	}

	_, total, err := h.service.List(c.Request.Context(), filter)
	if err != nil {
		InternalServerError(c, "查询审计日志统计失败")
		return
	}

	Success(c, gin.H{
		"total": total,
	})
}

func getIntParam(c *gin.Context, key string, defaultValue int) int {
	val := c.Query(key)
	if val == "" {
		return defaultValue
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return defaultValue
	}
	return n
}
