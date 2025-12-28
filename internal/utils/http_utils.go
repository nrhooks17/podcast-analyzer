package utils

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// getCorrelationID gets or generates a correlation ID for request tracing
func GetCorrelationID(r *http.Request) string {
	if id := r.Header.Get("X-Correlation-ID"); id != "" {
		return id
	}
	if id := r.Header.Get("X-Request-ID"); id != "" {
		return id
	}
	return uuid.New().String()
}

// SetCORSHeaders sets CORS headers on the response writer
func SetCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-CSRF-Token, X-Correlation-ID, X-Request-ID")
	w.Header().Set("Access-Control-Allow-Credentials", "false")
}

// writeJSON writes a JSON response with proper headers
func WriteJSON(w http.ResponseWriter, status int, data interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	SetCORSHeaders(w)
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(data)
}

// writeError writes a standardized error response
func WriteError(w http.ResponseWriter, status int, code, message string) {
	WriteJSON(w, status, map[string]interface{}{
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	})
}

// writeErrorWithCorrelation writes a standardized error response with correlation ID
func WriteErrorWithCorrelation(w http.ResponseWriter, status int, code, message, correlationID string) {
	WriteJSON(w, status, map[string]interface{}{
		"error": map[string]interface{}{
			"code":           code,
			"message":        message,
			"correlation_id": correlationID,
		},
	})
}

// getClientIP extracts the real client IP address
func GetClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the chain
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if ip != "" {
				return ip
			}
		}
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// getQueryParam gets a query parameter with a default value
func GetQueryParam(r *http.Request, key, defaultValue string) string {
	if value := r.URL.Query().Get(key); value != "" {
		return value
	}
	return defaultValue
}

// getQueryParamInt gets an integer query parameter with a default value
func GetQueryParamInt(r *http.Request, key string, defaultValue int) int {
	if value := r.URL.Query().Get(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}