package utils

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestGetCorrelationID(t *testing.T) {
	tests := []struct {
		name     string
		headers  map[string]string
		hasID    bool
	}{
		{
			name: "with X-Correlation-ID header",
			headers: map[string]string{
				"X-Correlation-ID": "test-correlation-123",
			},
			hasID: true,
		},
		{
			name: "with X-Request-ID header",
			headers: map[string]string{
				"X-Request-ID": "test-request-456",
			},
			hasID: true,
		},
		{
			name: "with both headers - prefer correlation ID",
			headers: map[string]string{
				"X-Correlation-ID": "correlation-123",
				"X-Request-ID":     "request-456",
			},
			hasID: true,
		},
		{
			name:    "without any headers - generate new",
			headers: map[string]string{},
			hasID:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			result := GetCorrelationID(req)

			assert.NotEmpty(t, result)
			
			if tt.hasID {
				if correlationID := tt.headers["X-Correlation-ID"]; correlationID != "" {
					assert.Equal(t, correlationID, result)
				} else if requestID := tt.headers["X-Request-ID"]; requestID != "" {
					assert.Equal(t, requestID, result)
				}
			} else {
				// Should generate a valid UUID
				_, err := uuid.Parse(result)
				assert.NoError(t, err)
			}
		})
	}
}

func TestSetCORSHeaders(t *testing.T) {
	recorder := httptest.NewRecorder()
	
	SetCORSHeaders(recorder)

	expectedHeaders := map[string]string{
		"Access-Control-Allow-Origin":      "*",
		"Access-Control-Allow-Methods":     "GET, POST, PUT, DELETE, OPTIONS",
		"Access-Control-Allow-Headers":     "Accept, Authorization, Content-Type, X-CSRF-Token, X-Correlation-ID, X-Request-ID",
		"Access-Control-Allow-Credentials": "false",
	}

	for header, expectedValue := range expectedHeaders {
		assert.Equal(t, expectedValue, recorder.Header().Get(header))
	}
}

func TestWriteJSON(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		data       interface{}
		expectJSON bool
	}{
		{
			name:   "success response",
			status: http.StatusOK,
			data: map[string]interface{}{
				"message": "success",
				"data":    "test data",
			},
			expectJSON: true,
		},
		{
			name:   "array response",
			status: http.StatusOK,
			data:   []string{"item1", "item2", "item3"},
			expectJSON: true,
		},
		{
			name:   "string response",
			status: http.StatusCreated,
			data:   "simple string response",
			expectJSON: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()

			err := WriteJSON(recorder, tt.status, tt.data)

			assert.NoError(t, err)
			assert.Equal(t, tt.status, recorder.Code)
			assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
			
			// Verify CORS headers are set
			assert.Equal(t, "*", recorder.Header().Get("Access-Control-Allow-Origin"))

			if tt.expectJSON {
				var decoded interface{}
				err := json.Unmarshal(recorder.Body.Bytes(), &decoded)
				assert.NoError(t, err)
			}
		})
	}
}

func TestWriteError(t *testing.T) {
	recorder := httptest.NewRecorder()
	
	WriteError(recorder, http.StatusBadRequest, "INVALID_INPUT", "The input provided is invalid")

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))

	var response map[string]interface{}
	err := json.Unmarshal(recorder.Body.Bytes(), &response)
	assert.NoError(t, err)

	errorData, exists := response["error"]
	assert.True(t, exists)

	errorMap := errorData.(map[string]interface{})
	assert.Equal(t, "INVALID_INPUT", errorMap["code"])
	assert.Equal(t, "The input provided is invalid", errorMap["message"])
}

func TestWriteErrorWithCorrelation(t *testing.T) {
	recorder := httptest.NewRecorder()
	correlationID := "test-correlation-123"
	
	WriteErrorWithCorrelation(recorder, http.StatusInternalServerError, "SERVER_ERROR", "Internal server error occurred", correlationID)

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))

	var response map[string]interface{}
	err := json.Unmarshal(recorder.Body.Bytes(), &response)
	assert.NoError(t, err)

	errorData, exists := response["error"]
	assert.True(t, exists)

	errorMap := errorData.(map[string]interface{})
	assert.Equal(t, "SERVER_ERROR", errorMap["code"])
	assert.Equal(t, "Internal server error occurred", errorMap["message"])
	assert.Equal(t, correlationID, errorMap["correlation_id"])
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name        string
		headers     map[string]string
		remoteAddr  string
		expectedIP  string
	}{
		{
			name: "X-Forwarded-For header single IP",
			headers: map[string]string{
				"X-Forwarded-For": "192.168.1.100",
			},
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "192.168.1.100",
		},
		{
			name: "X-Forwarded-For header multiple IPs",
			headers: map[string]string{
				"X-Forwarded-For": "192.168.1.100, 10.0.0.1, 172.16.0.1",
			},
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "192.168.1.100",
		},
		{
			name: "X-Forwarded-For header with spaces",
			headers: map[string]string{
				"X-Forwarded-For": "  192.168.1.100  ",
			},
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "192.168.1.100",
		},
		{
			name: "X-Real-IP header",
			headers: map[string]string{
				"X-Real-IP": "203.0.113.45",
			},
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "203.0.113.45",
		},
		{
			name: "both X-Forwarded-For and X-Real-IP - prefer X-Forwarded-For",
			headers: map[string]string{
				"X-Forwarded-For": "192.168.1.100",
				"X-Real-IP":       "203.0.113.45",
			},
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "192.168.1.100",
		},
		{
			name:       "no headers - use RemoteAddr",
			headers:    map[string]string{},
			remoteAddr: "198.51.100.25:54321",
			expectedIP: "198.51.100.25",
		},
		{
			name:       "no headers - RemoteAddr without port",
			headers:    map[string]string{},
			remoteAddr: "198.51.100.25",
			expectedIP: "198.51.100.25",
		},
		{
			name: "empty X-Forwarded-For - fall back to X-Real-IP",
			headers: map[string]string{
				"X-Forwarded-For": "",
				"X-Real-IP":       "203.0.113.45",
			},
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "203.0.113.45",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = tt.remoteAddr
			
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			result := GetClientIP(req)
			assert.Equal(t, tt.expectedIP, result)
		})
	}
}

func TestGetQueryParam(t *testing.T) {
	tests := []struct {
		name         string
		queryString  string
		key          string
		defaultValue string
		expected     string
	}{
		{
			name:         "existing parameter",
			queryString:  "?page=2&limit=10&sort=name",
			key:          "page",
			defaultValue: "1",
			expected:     "2",
		},
		{
			name:         "missing parameter - use default",
			queryString:  "?page=2&limit=10",
			key:          "sort",
			defaultValue: "id",
			expected:     "id",
		},
		{
			name:         "empty parameter value",
			queryString:  "?page=&limit=10",
			key:          "page",
			defaultValue: "1",
			expected:     "1",
		},
		{
			name:         "no query string",
			queryString:  "",
			key:          "page",
			defaultValue: "1",
			expected:     "1",
		},
		{
			name:         "parameter with special characters",
			queryString:  "?search=hello%20world&category=tech",
			key:          "search",
			defaultValue: "",
			expected:     "hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test"+tt.queryString, nil)

			result := GetQueryParam(req, tt.key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetQueryParamInt(t *testing.T) {
	tests := []struct {
		name         string
		queryString  string
		key          string
		defaultValue int
		expected     int
	}{
		{
			name:         "valid integer parameter",
			queryString:  "?page=5&limit=20",
			key:          "page",
			defaultValue: 1,
			expected:     5,
		},
		{
			name:         "missing parameter - use default",
			queryString:  "?page=5",
			key:          "limit",
			defaultValue: 10,
			expected:     10,
		},
		{
			name:         "invalid integer - use default",
			queryString:  "?page=abc&limit=20",
			key:          "page",
			defaultValue: 1,
			expected:     1,
		},
		{
			name:         "empty parameter - use default",
			queryString:  "?page=&limit=20",
			key:          "page",
			defaultValue: 1,
			expected:     1,
		},
		{
			name:         "zero value",
			queryString:  "?page=0&limit=20",
			key:          "page",
			defaultValue: 1,
			expected:     0,
		},
		{
			name:         "negative value",
			queryString:  "?offset=-5&limit=20",
			key:          "offset",
			defaultValue: 0,
			expected:     -5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test"+tt.queryString, nil)

			result := GetQueryParamInt(req, tt.key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWriteJSON_Integration(t *testing.T) {
	recorder := httptest.NewRecorder()

	testData := map[string]interface{}{
		"id":      "123",
		"name":    "Test Item",
		"active":  true,
		"count":   42,
		"tags":    []string{"tag1", "tag2"},
		"metadata": map[string]interface{}{
			"created": "2023-01-01T00:00:00Z",
			"version": "1.0",
		},
	}

	err := WriteJSON(recorder, http.StatusCreated, testData)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, recorder.Code)
	assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))

	// Verify CORS headers
	assert.Equal(t, "*", recorder.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET, POST, PUT, DELETE, OPTIONS", recorder.Header().Get("Access-Control-Allow-Methods"))

	// Verify JSON content
	var decoded map[string]interface{}
	err = json.Unmarshal(recorder.Body.Bytes(), &decoded)
	assert.NoError(t, err)
	assert.Equal(t, "123", decoded["id"])
	assert.Equal(t, "Test Item", decoded["name"])
	assert.Equal(t, true, decoded["active"])
	assert.Equal(t, float64(42), decoded["count"]) // JSON numbers decode as float64
}