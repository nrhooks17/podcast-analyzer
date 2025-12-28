package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestIDMiddleware(t *testing.T) {
	var capturedCorrelationID string
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture the correlation ID from context
		if id := r.Context().Value("correlation_id"); id != nil {
			capturedCorrelationID = id.(string)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "test"})
	})

	handler := RequestIDMiddleware()(testHandler)

	tests := []struct {
		name                 string
		headerValue          string
		expectHeaderInResp   bool
		expectNewIDGenerated bool
	}{
		{
			name:               "with existing correlation ID",
			headerValue:        "existing-correlation-id-123",
			expectHeaderInResp: false, // Should not add header if already present
		},
		{
			name:                 "without correlation ID header",
			headerValue:          "",
			expectHeaderInResp:   true, // Should generate and add new header
			expectNewIDGenerated: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capturedCorrelationID = "" // Reset
			
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.headerValue != "" {
				req.Header.Set("X-Correlation-ID", tt.headerValue)
			}

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)

			assert.Equal(t, http.StatusOK, recorder.Code)

			if tt.headerValue != "" {
				// Should use the provided correlation ID
				assert.Equal(t, tt.headerValue, capturedCorrelationID)
			} else {
				// Should generate a new correlation ID
				assert.NotEmpty(t, capturedCorrelationID)
				assert.Len(t, capturedCorrelationID, 36) // UUID format
			}

			if tt.expectHeaderInResp {
				responseHeaderID := recorder.Header().Get("X-Correlation-ID")
				assert.NotEmpty(t, responseHeaderID)
				assert.Equal(t, capturedCorrelationID, responseHeaderID)
			}
		})
	}
}

func TestRequestIDMiddleware_UUIDFormat(t *testing.T) {
	var capturedCorrelationID string
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id := r.Context().Value("correlation_id"); id != nil {
			capturedCorrelationID = id.(string)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "test"})
	})

	handler := RequestIDMiddleware()(testHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.NotEmpty(t, capturedCorrelationID)
	
	// Check UUID format (8-4-4-4-12 characters)
	assert.Regexp(t, `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`, capturedCorrelationID)
}

func TestLoggingMiddleware(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "test response"})
	})

	handler := LoggingMiddleware()(testHandler)

	tests := []struct {
		name               string
		path               string
		method             string
		correlationHeader  string
		expectedStatus     int
	}{
		{
			name:              "GET request with correlation ID",
			path:              "/test",
			method:            "GET",
			correlationHeader: "test-correlation-123",
			expectedStatus:    http.StatusOK,
		},
		{
			name:           "POST request without correlation ID",
			path:           "/test",
			method:         "POST",
			expectedStatus: http.StatusOK, // Handler accepts all methods
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.correlationHeader != "" {
				req.Header.Set("X-Correlation-ID", tt.correlationHeader)
			}

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, req)

			assert.Equal(t, tt.expectedStatus, recorder.Code)
			
			// Logging middleware should not interfere with the response
			// The actual logging is tested through the formatter function
		})
	}
}

func TestMiddlewareChaining(t *testing.T) {
	var capturedCorrelationID string
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id := r.Context().Value("correlation_id"); id != nil {
			capturedCorrelationID = id.(string)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "test"})
	})
	
	// Chain middleware together (CORS is handled in utils.SetCORSHeaders)
	handler := RequestIDMiddleware()(testHandler)
	handler = LoggingMiddleware()(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)
	
	// Check correlation ID was generated and set
	assert.NotEmpty(t, capturedCorrelationID)
	responseCorrelationID := recorder.Header().Get("X-Correlation-ID")
	assert.Equal(t, capturedCorrelationID, responseCorrelationID)
	
	// Check response body
	var response map[string]interface{}
	err := json.Unmarshal(recorder.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "test", response["message"])
}