package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/velocitypay/velocitypay/internal/metrics"
)

// PrometheusMetrics records HTTP request counts and durations.
func PrometheusMetrics(m *metrics.Metrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath() // uses the route pattern, not the actual path
		if path == "" {
			path = "unknown"
		}

		c.Next()

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())

		m.HTTPRequestsTotal.WithLabelValues(c.Request.Method, path, status).Inc()
		m.HTTPRequestDuration.WithLabelValues(c.Request.Method, path).Observe(duration)
	}
}
