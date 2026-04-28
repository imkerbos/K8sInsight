package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// RequestLogger 结构化请求日志
func RequestLogger(logger *zap.Logger) gin.HandlerFunc {
	log := logger.Named("http")
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		fields := []zap.Field{
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.Int("status", status),
			zap.Duration("latency", latency),
			zap.String("clientIP", c.ClientIP()),
		}
		if id, ok := c.Get(RequestIDKey); ok {
			fields = append(fields, zap.String("requestId", id.(string)))
		}

		if status >= 500 {
			log.Error("请求处理失败", fields...)
		} else if latency > 500*time.Millisecond {
			log.Warn("慢请求", fields...)
		} else {
			log.Info("请求完成", fields...)
		}
	}
}
