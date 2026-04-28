package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kerbos/k8sinsight/internal/infra/metrics"
)

// Metrics Prometheus HTTP 指标中间件
func Metrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		c.Next()

		status := strconv.Itoa(c.Writer.Status())
		elapsed := time.Since(start).Seconds()

		metrics.HTTPRequestsTotal.WithLabelValues(c.Request.Method, path, status).Inc()
		metrics.HTTPRequestDuration.WithLabelValues(c.Request.Method, path).Observe(elapsed)
	}
}
