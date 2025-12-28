package utils

import (
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		str      string
		substr   string
		expected bool
	}{
		{
			name:     "exact match lowercase",
			str:      "hello world",
			substr:   "world",
			expected: true,
		},
		{
			name:     "exact match uppercase",
			str:      "HELLO WORLD",
			substr:   "WORLD",
			expected: true,
		},
		{
			name:     "case insensitive match",
			str:      "Hello World",
			substr:   "hello",
			expected: true,
		},
		{
			name:     "case insensitive match reverse",
			str:      "hello world",
			substr:   "HELLO",
			expected: true,
		},
		{
			name:     "mixed case both strings",
			str:      "HeLLo WoRLd",
			substr:   "WoRld",
			expected: true,
		},
		{
			name:     "substring not found",
			str:      "hello world",
			substr:   "foo",
			expected: false,
		},
		{
			name:     "empty substring",
			str:      "hello world",
			substr:   "",
			expected: true,
		},
		{
			name:     "empty string",
			str:      "",
			substr:   "hello",
			expected: false,
		},
		{
			name:     "both empty",
			str:      "",
			substr:   "",
			expected: true,
		},
		{
			name:     "substring longer than string",
			str:      "hi",
			substr:   "hello",
			expected: false,
		},
		{
			name:     "special characters",
			str:      "hello@world.com",
			substr:   "@world",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Contains(tt.str, tt.substr)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractIDFromPath(t *testing.T) {
	tests := []struct {
		name        string
		urlPath     string
		prefix      string
		expectedID  string
		expectError bool
	}{
		{
			name:        "valid ID extraction",
			urlPath:     "/api/transcripts/123",
			prefix:      "/api/transcripts/",
			expectedID:  "123",
			expectError: false,
		},
		{
			name:        "UUID extraction",
			urlPath:     "/api/transcripts/550e8400-e29b-41d4-a716-446655440000",
			prefix:      "/api/transcripts/",
			expectedID:  "550e8400-e29b-41d4-a716-446655440000",
			expectError: false,
		},
		{
			name:        "path with trailing slash",
			urlPath:     "/api/transcripts/123/",
			prefix:      "/api/transcripts/",
			expectedID:  "123",
			expectError: false,
		},
		{
			name:        "path without prefix match",
			urlPath:     "/api/users/123",
			prefix:      "/api/transcripts/",
			expectedID:  "",
			expectError: true,
		},
		{
			name:        "path with exact prefix match only",
			urlPath:     "/api/transcripts/",
			prefix:      "/api/transcripts/",
			expectedID:  "",
			expectError: true,
		},
		{
			name:        "empty path",
			urlPath:     "",
			prefix:      "/api/transcripts/",
			expectedID:  "",
			expectError: true,
		},
		{
			name:        "complex ID with dashes",
			urlPath:     "/api/analysis/complex-id-123",
			prefix:      "/api/analysis/",
			expectedID:  "complex-id-123",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExtractIDFromPath(tt.urlPath, tt.prefix)

			if tt.expectError {
				assert.Error(t, err)
				assert.Empty(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedID, result)
			}
		})
	}
}

func TestMatchPath(t *testing.T) {
	tests := []struct {
		name           string
		requestPath    string
		pattern        string
		expectedMatch  bool
		expectedParam  string
	}{
		{
			name:           "exact match",
			requestPath:    "/api/transcripts/",
			pattern:        "/api/transcripts/",
			expectedMatch:  true,
			expectedParam:  "",
		},
		{
			name:           "match with ID",
			requestPath:    "/api/transcripts/123",
			pattern:        "/api/transcripts/",
			expectedMatch:  true,
			expectedParam:  "123",
		},
		{
			name:           "match with UUID",
			requestPath:    "/api/transcripts/550e8400-e29b-41d4-a716-446655440000",
			pattern:        "/api/transcripts/",
			expectedMatch:  true,
			expectedParam:  "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			name:           "match with trailing slash",
			requestPath:    "/api/transcripts/123/",
			pattern:        "/api/transcripts/",
			expectedMatch:  true,
			expectedParam:  "123",
		},
		{
			name:           "no match - different pattern",
			requestPath:    "/api/users/123",
			pattern:        "/api/transcripts/",
			expectedMatch:  false,
			expectedParam:  "",
		},
		{
			name:           "no match - additional path segments",
			requestPath:    "/api/transcripts/123/results",
			pattern:        "/api/transcripts/",
			expectedMatch:  false,
			expectedParam:  "",
		},
		{
			name:           "match without trailing slash in pattern",
			requestPath:    "/api/status",
			pattern:        "/api/status",
			expectedMatch:  true,
			expectedParam:  "",
		},
		{
			name:           "partial prefix not matching",
			requestPath:    "/api/transcript",
			pattern:        "/api/transcripts/",
			expectedMatch:  false,
			expectedParam:  "",
		},
		{
			name:           "root path match",
			requestPath:    "/",
			pattern:        "/",
			expectedMatch:  true,
			expectedParam:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match, param := MatchPath(tt.requestPath, tt.pattern)

			assert.Equal(t, tt.expectedMatch, match)
			assert.Equal(t, tt.expectedParam, param)
		})
	}
}

func TestValidateHTTPMethod(t *testing.T) {
	tests := []struct {
		name           string
		requestMethod  string
		expectedMethod string
		expectError    bool
	}{
		{
			name:           "GET method matches",
			requestMethod:  "GET",
			expectedMethod: "GET",
			expectError:    false,
		},
		{
			name:           "POST method matches",
			requestMethod:  "POST",
			expectedMethod: "POST",
			expectError:    false,
		},
		{
			name:           "method mismatch",
			requestMethod:  "GET",
			expectedMethod: "POST",
			expectError:    true,
		},
		{
			name:           "case sensitive mismatch",
			requestMethod:  "get",
			expectedMethod: "GET",
			expectError:    true,
		},
		{
			name:           "PUT method matches",
			requestMethod:  "PUT",
			expectedMethod: "PUT",
			expectError:    false,
		},
		{
			name:           "DELETE method matches",
			requestMethod:  "DELETE",
			expectedMethod: "DELETE",
			expectError:    false,
		},
		{
			name:           "OPTIONS method matches",
			requestMethod:  "OPTIONS",
			expectedMethod: "OPTIONS",
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.requestMethod, "/test", nil)

			err := ValidateHTTPMethod(req, tt.expectedMethod)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "method not allowed")
				assert.Contains(t, err.Error(), tt.expectedMethod)
				assert.Contains(t, err.Error(), tt.requestMethod)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateAndParseUUID(t *testing.T) {
	validUUID := "550e8400-e29b-41d4-a716-446655440000"
	
	tests := []struct {
		name        string
		idStr       string
		fieldName   string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid UUID",
			idStr:       validUUID,
			fieldName:   "transcript_id",
			expectError: false,
		},
		{
			name:        "empty string",
			idStr:       "",
			fieldName:   "user_id",
			expectError: true,
			errorMsg:    "user_id cannot be empty",
		},
		{
			name:        "invalid UUID format",
			idStr:       "not-a-uuid",
			fieldName:   "job_id",
			expectError: true,
			errorMsg:    "invalid job_id format",
		},
		{
			name:        "UUID without hyphens",
			idStr:       "550e8400e29b41d4a716446655440000",
			fieldName:   "transcript_id",
			expectError: true,
			errorMsg:    "invalid transcript_id format",
		},
		{
			name:        "UUID with wrong length",
			idStr:       "550e8400-e29b-41d4-a716",
			fieldName:   "analysis_id",
			expectError: true,
			errorMsg:    "invalid analysis_id format",
		},
		{
			name:        "UUID with invalid characters",
			idStr:       "550e8400-e29b-41d4-a716-44665544000g",
			fieldName:   "result_id",
			expectError: true,
			errorMsg:    "invalid result_id format",
		},
		{
			name:        "nil UUID representation",
			idStr:       "00000000-0000-0000-0000-000000000000",
			fieldName:   "session_id",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ValidateAndParseUUID(tt.idStr, tt.fieldName)

			if tt.expectError {
				assert.Error(t, err)
				assert.Equal(t, uuid.Nil, result)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
				// Allow nil UUID as valid - it's a legitimate UUID value
				if tt.idStr == validUUID {
					expected, _ := uuid.Parse(validUUID)
					assert.Equal(t, expected, result)
				}
			}
		})
	}
}

func TestValidateAndParseUUID_SpecificValidCases(t *testing.T) {
	testUUIDs := []string{
		"550e8400-e29b-41d4-a716-446655440000",
		"6ba7b810-9dad-11d1-80b4-00c04fd430c8",
		"6ba7b811-9dad-11d1-80b4-00c04fd430c8",
		"00000000-0000-0000-0000-000000000000", // nil UUID
	}

	for _, uuidStr := range testUUIDs {
		t.Run("valid_uuid_"+uuidStr, func(t *testing.T) {
			result, err := ValidateAndParseUUID(uuidStr, "test_id")

			assert.NoError(t, err)
			
			expected, _ := uuid.Parse(uuidStr)
			assert.Equal(t, expected, result)
			assert.Equal(t, uuidStr, result.String())
		})
	}
}

func TestExtractIDFromPath_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		urlPath     string
		prefix      string
		expectError bool
		description string
	}{
		{
			name:        "prefix longer than path",
			urlPath:     "/api",
			prefix:      "/api/transcripts/",
			expectError: true,
			description: "Should fail when prefix is longer than the URL path",
		},
		{
			name:        "prefix exactly matches path",
			urlPath:     "/api/transcripts/",
			prefix:      "/api/transcripts/",
			expectError: true,
			description: "Should fail when path exactly matches prefix with no ID",
		},
		{
			name:        "multiple slashes in ID",
			urlPath:     "/api/transcripts/123/456",
			prefix:      "/api/transcripts/",
			expectError: false,
			description: "Should extract compound ID with slashes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ExtractIDFromPath(tt.urlPath, tt.prefix)

			if tt.expectError {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
			}
		})
	}
}

func TestMatchPath_EdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		requestPath   string
		pattern       string
		expectedMatch bool
		description   string
	}{
		{
			name:          "empty pattern",
			requestPath:   "/api/test",
			pattern:       "",
			expectedMatch: true,
			description:   "Empty pattern should match any path",
		},
		{
			name:          "empty request path",
			requestPath:   "",
			pattern:       "/api",
			expectedMatch: false,
			description:   "Empty request path should not match non-empty pattern",
		},
		{
			name:          "pattern is substring but not prefix",
			requestPath:   "prefix/api/test",
			pattern:       "/api/test",
			expectedMatch: false,
			description:   "Pattern that appears in middle should not match",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match, _ := MatchPath(tt.requestPath, tt.pattern)
			assert.Equal(t, tt.expectedMatch, match, tt.description)
		})
	}
}