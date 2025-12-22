package middleware

import (
	"backend-golang/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// LoggingMiddleware logs HTTP requests with structured logging
func LoggingMiddleware() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		correlationID := param.Request.Header.Get("X-Correlation-ID")
		if correlationID == "" {
			correlationID = uuid.New().String()
		}

		logger.Log.WithFields(map[string]interface{}{
			"correlation_id": correlationID,
			"method":         param.Method,
			"path":           param.Path,
			"status":         param.StatusCode,
			"latency_ms":     param.Latency.Milliseconds(),
			"client_ip":      param.ClientIP,
			"user_agent":     param.Request.UserAgent(),
			"response_size":  param.BodySize,
		}).Info("HTTP request processed")

		return ""
	})
}

// RequestIDMiddleware adds correlation ID to request context
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		correlationID := c.GetHeader("X-Correlation-ID")
		if correlationID == "" {
			correlationID = uuid.New().String()
			c.Header("X-Correlation-ID", correlationID)
		}
		c.Set("correlation_id", correlationID)
		c.Next()
	}
}