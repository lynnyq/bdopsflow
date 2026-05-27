package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/health"
	"github.com/lynnyq/bdopsflow/scheduler/internal/metrics"
)

type HealthHandler struct {
	checker   *health.HealthChecker
	collector *metrics.MetricsCollector
	version   string
	startTime time.Time
}

func NewHealthHandler(checker *health.HealthChecker, collector *metrics.MetricsCollector, version string) *HealthHandler {
	return &HealthHandler{
		checker:   checker,
		collector: collector,
		version:   version,
		startTime: time.Now(),
	}
}

func (hh *HealthHandler) Liveness(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"version": hh.version,
		"uptime":  time.Since(hh.startTime).String(),
	})
}

func (hh *HealthHandler) Readiness(c *gin.Context) {
	report := hh.checker.Check(c.Request.Context())
	
	statusCode := http.StatusOK
	if report.Status == health.StatusFailing {
		statusCode = http.StatusServiceUnavailable
	}
	
	c.JSON(statusCode, report)
}

func (hh *HealthHandler) Metrics(c *gin.Context) {
	if hh.collector == nil {
		c.JSON(http.StatusOK, gin.H{
			"status": "metrics_not_available",
		})
		return
	}

	snapshot := hh.collector.GetSnapshot()
	
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"metrics": snapshot,
		"version": hh.version,
		"uptime":  time.Since(hh.startTime).String(),
	})
}
