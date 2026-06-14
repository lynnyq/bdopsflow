package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lynnyq/bdopsflow/scheduler/internal/metrics"
)

// MetricsCollector HTTP 请求指标收集中间件
func MetricsCollector() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		// 处理请求
		c.Next()

		// 记录指标
		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())

		// HTTP 请求总数
		metrics.HTTPRequestsTotal.WithLabelValues(c.Request.Method, path, status).Inc()

		// HTTP 请求耗时
		metrics.HTTPRequestDurationSeconds.WithLabelValues(c.Request.Method, path).Observe(duration)

		// HTTP 请求大小
		requestSize := float64(c.Request.ContentLength)
		metrics.HTTPRequestSizeBytes.WithLabelValues(c.Request.Method, path).Observe(requestSize)

		// HTTP 响应大小
		responseSize := float64(c.Writer.Size())
		metrics.HTTPResponseSizeBytes.WithLabelValues(c.Request.Method, path).Observe(responseSize)
	}
}
