// ================================
// internal/api/middleware/metrics.middleware.go - Request metrics collection
// ================================

package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/platformbuilds/mirador-core/internal/metrics"
)

// MetricsMiddleware collects HTTP request metrics for MIRADOR-CORE monitoring
func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Process request
		c.Next()

		// Record metrics
		duration := time.Since(start).Seconds()
		statusCode := strconv.Itoa(c.Writer.Status())
		tenantID := c.GetString("tenant_id")
		if tenantID == "" {
			tenantID = "unknown"
		}

		// Update Prometheus metrics
		metrics.HTTPRequestsTotal.WithLabelValues(
			c.Request.Method,
			c.FullPath(),
			statusCode,
			tenantID,
		).Inc()

		metrics.HTTPRequestDuration.WithLabelValues(
			c.Request.Method,
			c.FullPath(),
			tenantID,
		).Observe(duration)
	}
}
