package handler

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/service"
)

type DashboardHandler struct {
	svc *service.SchedulerService
}

func NewDashboardHandler(svc *service.SchedulerService) *DashboardHandler {
	return &DashboardHandler{svc: svc}
}

func (h *DashboardHandler) GetStats(c *gin.Context) {
	ctx := c.Request.Context()

	defer func() {
		if r := recover(); r != nil {
			slog.Error("DashboardHandler.GetStats: panic recovered", "panic", r)
			InternalServerError(c, "internal server error")
		}
	}()

	slog.Debug("DashboardHandler.GetStats: handling request")

	domainID, _ := c.Get("domain_id")
	userRole, _ := c.Get("role")

	var dID int64
	var role string
	if v, ok := domainID.(int64); ok {
		dID = v
	}
	if v, ok := userRole.(string); ok {
		role = v
	}

	stats, err := h.svc.GetDashboardStats(ctx, dID, role)
	if err != nil {
		slog.Error("DashboardHandler.GetStats: failed to get stats", "error", err)
		InternalServerError(c, err.Error())
		return
	}

	Success(c, stats)
}

func (h *DashboardHandler) GetTrends(c *gin.Context) {
	ctx := c.Request.Context()

	defer func() {
		if r := recover(); r != nil {
			slog.Error("DashboardHandler.GetTrends: panic recovered", "panic", r)
			InternalServerError(c, "internal server error")
		}
	}()

	slog.Debug("DashboardHandler.GetTrends: handling request")

	domainID, _ := c.Get("domain_id")
	userRole, _ := c.Get("role")

	var dID int64
	var role string
	if v, ok := domainID.(int64); ok {
		dID = v
	}
	if v, ok := userRole.(string); ok {
		role = v
	}

	trends, err := h.svc.GetTrendData(ctx, dID, role)
	if err != nil {
		slog.Error("DashboardHandler.GetTrends: failed to get trends", "error", err)
		InternalServerError(c, err.Error())
		return
	}

	Success(c, gin.H{"items": trends})
}

func (h *DashboardHandler) PauseScheduler(c *gin.Context) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("DashboardHandler.PauseScheduler: panic recovered", "panic", r)
			InternalServerError(c, "internal server error")
		}
	}()

	slog.Debug("DashboardHandler.PauseScheduler: handling request")

	h.svc.PauseScheduler()
	slog.Info("DashboardHandler.PauseScheduler: scheduler paused")

	SuccessWithMessage(c, "scheduler paused", nil)
}

func (h *DashboardHandler) ResumeScheduler(c *gin.Context) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("DashboardHandler.ResumeScheduler: panic recovered", "panic", r)
			InternalServerError(c, "internal server error")
		}
	}()

	slog.Debug("DashboardHandler.ResumeScheduler: handling request")

	h.svc.ResumeScheduler()
	slog.Info("DashboardHandler.ResumeScheduler: scheduler resumed")

	SuccessWithMessage(c, "scheduler resumed", nil)
}

func (h *DashboardHandler) GetSchedulerStatus(c *gin.Context) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("DashboardHandler.GetSchedulerStatus: panic recovered", "panic", r)
			InternalServerError(c, "internal server error")
		}
	}()

	slog.Debug("DashboardHandler.GetSchedulerStatus: handling request")

	paused := h.svc.IsSchedulerPaused()

	Success(c, gin.H{"paused": paused})
}

func (h *DashboardHandler) HealthCheck(c *gin.Context) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("DashboardHandler.HealthCheck: panic recovered", "panic", r)
			InternalServerError(c, "internal server error")
		}
	}()

	slog.Debug("DashboardHandler.HealthCheck: handling request")

	result := h.svc.HealthCheck(c.Request.Context())

	Success(c, result)
}
