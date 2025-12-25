package handlers

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// getCorrelationID gets or generates a correlation ID for request tracing
func getCorrelationID(r *http.Request) string {
	if id := r.Header.Get("X-Correlation-ID"); id != "" {
		return id
	}
	if id := r.Header.Get("X-Request-ID"); id != "" {
		return id
	}
	return uuid.New().String()
}

// contains checks if a string contains a substring (case-insensitive)
func contains(str, substr string) bool {
	return strings.Contains(strings.ToLower(str), strings.ToLower(substr))
}

// writeJSON writes a JSON response with proper headers
func writeJSON(w http.ResponseWriter, status int, data interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(data)
}

// writeError writes a standardized error response
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]interface{}{
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	})
}

// writeErrorWithCorrelation writes a standardized error response with correlation ID
func writeErrorWithCorrelation(w http.ResponseWriter, status int, code, message, correlationID string) {
	writeJSON(w, status, map[string]interface{}{
		"error": map[string]interface{}{
			"code":           code,
			"message":        message,
			"correlation_id": correlationID,
		},
	})
}

// extractIDFromPath extracts an ID from the URL path
// e.g., "/api/transcripts/123" with prefix "/api/transcripts/" returns "123"
func extractIDFromPath(urlPath, prefix string) (string, error) {
	if !strings.HasPrefix(urlPath, prefix) {
		return "", fmt.Errorf("path does not match expected prefix")
	}
	id := strings.TrimPrefix(urlPath, prefix)
	id = strings.TrimSuffix(id, "/")
	if id == "" {
		return "", fmt.Errorf("no ID found in path")
	}
	return id, nil
}

// parseUUIDParam parses a UUID from a path parameter
func parseUUIDParam(idStr string) (uuid.UUID, error) {
	return uuid.Parse(idStr)
}

// getClientIP extracts the real client IP address
func getClientIP(r *http.Request) string {
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
func getQueryParam(r *http.Request, key, defaultValue string) string {
	if value := r.URL.Query().Get(key); value != "" {
		return value
	}
	return defaultValue
}

// getQueryParamInt gets an integer query parameter with a default value
func getQueryParamInt(r *http.Request, key string, defaultValue int) int {
	if value := r.URL.Query().Get(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// matchPath checks if a request path matches a pattern and extracts parameters
// e.g., matchPath("/api/transcripts/123", "/api/transcripts/") returns true, "123"
func matchPath(requestPath, pattern string) (bool, string) {
	if !strings.HasPrefix(requestPath, pattern) {
		return false, ""
	}
	
	remainder := strings.TrimPrefix(requestPath, pattern)
	remainder = strings.TrimSuffix(remainder, "/")
	
	// For exact match (no ID)
	if remainder == "" {
		return true, ""
	}
	
	// For patterns with ID, ensure no additional path segments
	if !strings.Contains(remainder, "/") {
		return true, remainder
	}
	
	return false, ""
}