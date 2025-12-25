package middleware

import (
	"context"
	"net/http"
	"time"
	"backend-golang/pkg/logger"

	"github.com/google/uuid"
)

// LoggingMiddleware logs HTTP requests with structured logging
func LoggingMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			
			// Get or generate correlation ID
			correlationID := r.Header.Get("X-Correlation-ID")
			if correlationID == "" {
				correlationID = uuid.New().String()
			}

			// Wrap ResponseWriter to capture response data
			lrw := &loggingResponseWriter{
				ResponseWriter: w,
				statusCode:     200, // Default to 200
			}

			// Process request
			next.ServeHTTP(lrw, r)

			// Log request completion
			duration := time.Since(start)
			logger.Log.WithFields(map[string]interface{}{
				"correlation_id": correlationID,
				"method":         r.Method,
				"path":           r.URL.Path,
				"status":         lrw.statusCode,
				"latency_ms":     duration.Milliseconds(),
				"client_ip":      getClientIP(r),
				"user_agent":     r.UserAgent(),
				"response_size":  lrw.bytesWritten,
			}).Info("HTTP request processed")
		})
	}
}

// RequestIDMiddleware adds correlation ID to request context and response header
func RequestIDMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			correlationID := r.Header.Get("X-Correlation-ID")
			if correlationID == "" {
				correlationID = uuid.New().String()
				w.Header().Set("X-Correlation-ID", correlationID)
			}
			
			// Add correlation ID to request context
			ctx := context.WithValue(r.Context(), "correlation_id", correlationID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RecoveryMiddleware provides panic recovery equivalent to gin.Recovery()
func RecoveryMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					logger.Log.WithFields(map[string]interface{}{
						"panic":          err,
						"method":         r.Method,
						"path":           r.URL.Path,
						"client_ip":      getClientIP(r),
						"correlation_id": r.Header.Get("X-Correlation-ID"),
					}).Error("HTTP handler panicked")

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(`{"error":{"code":"INTERNAL_ERROR","message":"Internal server error"}}`))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// loggingResponseWriter wraps http.ResponseWriter to capture response data
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

func (lrw *loggingResponseWriter) Write(b []byte) (int, error) {
	n, err := lrw.ResponseWriter.Write(b)
	lrw.bytesWritten += n
	return n, err
}

// getClientIP extracts the real client IP address
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// Fall back to RemoteAddr
	return r.RemoteAddr
}