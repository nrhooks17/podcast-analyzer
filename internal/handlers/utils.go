package handlers

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// getCorrelationID gets or generates a correlation ID for request tracing
func getCorrelationID(c *gin.Context) string {
	if id := c.GetHeader("X-Correlation-ID"); id != "" {
		return id
	}
	if id := c.GetHeader("X-Request-ID"); id != "" {
		return id
	}
	return uuid.New().String()
}

// contains checks if a string contains a substring (case-insensitive)
func contains(str, substr string) bool {
	return strings.Contains(strings.ToLower(str), strings.ToLower(substr))
}