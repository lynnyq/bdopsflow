package handler

import (
	"log/slog"

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
	slog.Debug("AuditLogHandler.List: entering", "module", "handler_audit_log")

	var filter model.AuditLogFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		BadRequest(c, err.Error())
		return
	}

	logs, total, err := h.service.List(c.Request.Context(), filter)
	if err != nil {
		slog.Error("AuditLogHandler.List: failed to query audit logs", "module", "handler_audit_log", "error", err)
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

	slog.Debug("AuditLogHandler.CleanExpired: entering", "module", "handler_audit_log", "retention_days", req.RetentionDays)

	deleted, err := h.service.CleanExpired(c.Request.Context(), req.RetentionDays)
	if err != nil {
		slog.Error("AuditLogHandler.CleanExpired: failed to clean expired audit logs", "module", "handler_audit_log", "retention_days", req.RetentionDays, "error", err)
		InternalServerError(c, "清理审计日志失败")
		return
	}

	slog.Info("AuditLogHandler.CleanExpired: expired audit logs cleaned successfully", "module", "handler_audit_log", "deleted_count", deleted)
	SuccessWithMessage(c, "清理完成", gin.H{
		"deleted_count":  deleted,
		"retention_days": req.RetentionDays,
	})
}

func (h *AuditLogHandler) GetRetentionDays(c *gin.Context) {
	slog.Debug("AuditLogHandler.GetRetentionDays: entering", "module", "handler_audit_log")
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

	slog.Debug("AuditLogHandler.UpdateRetentionDays: entering", "module", "handler_audit_log", "retention_days", req.RetentionDays)
	slog.Info("AuditLogHandler.UpdateRetentionDays: retention days updated successfully", "module", "handler_audit_log", "retention_days", req.RetentionDays)
	Success(c, gin.H{
		"retention_days": req.RetentionDays,
	})
}

func (h *AuditLogHandler) GetStats(c *gin.Context) {
	slog.Debug("AuditLogHandler.GetStats: entering", "module", "handler_audit_log")

	filter := model.AuditLogFilter{
		Page:     1,
		PageSize: 1,
	}

	_, total, err := h.service.List(c.Request.Context(), filter)
	if err != nil {
		slog.Error("AuditLogHandler.GetStats: failed to query audit log stats", "module", "handler_audit_log", "error", err)
		InternalServerError(c, "查询审计日志统计失败")
		return
	}

	Success(c, gin.H{
		"total": total,
	})
}


