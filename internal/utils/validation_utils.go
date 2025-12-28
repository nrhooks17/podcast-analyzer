package utils

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// contains checks if a string contains a substring (case-insensitive)
func Contains(str, substr string) bool {
	return strings.Contains(strings.ToLower(str), strings.ToLower(substr))
}

// extractIDFromPath extracts an ID from the URL path
// e.g., "/api/transcripts/123" with prefix "/api/transcripts/" returns "123"
func ExtractIDFromPath(urlPath, prefix string) (string, error) {
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

// matchPath checks if a request path matches a pattern and extracts parameters
// e.g., matchPath("/api/transcripts/123", "/api/transcripts/") returns true, "123"
func MatchPath(requestPath, pattern string) (bool, string) {
	// Empty pattern matches any path
	if pattern == "" {
		return true, requestPath
	}
	
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

// validateHTTPMethod validates that request method matches expected method
func ValidateHTTPMethod(r *http.Request, expectedMethod string) error {
	if r.Method != expectedMethod {
		return fmt.Errorf("method not allowed: expected %s, got %s", expectedMethod, r.Method)
	}
	return nil
}

// validateAndParseUUID validates and parses UUID from string
func ValidateAndParseUUID(idStr string, fieldName string) (uuid.UUID, error) {
	if idStr == "" {
		return uuid.Nil, fmt.Errorf("%s cannot be empty", fieldName)
	}
	
	// Ensure UUID has proper format with hyphens
	if len(idStr) != 36 || strings.Count(idStr, "-") != 4 {
		return uuid.Nil, fmt.Errorf("invalid %s format", fieldName)
	}
	
	id, err := uuid.Parse(idStr)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid %s format: %w", fieldName, err)
	}
	
	return id, nil
}