package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRouterForMiddleware() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

func TestCORSMiddleware(t *testing.T) {
	tests := []struct {
		name            string
		allowedOrigins  []string
		requestOrigin   string
		method          string
		expectedStatus  int
		expectOriginSet bool
		expectedOrigin  string
	}{
		{
			name:            "allowed origin",
			allowedOrigins:  []string{"https://example.com", "https://test.com"},
			requestOrigin:   "https://example.com",
			method:          "GET",
			expectedStatus:  http.StatusOK,
			expectOriginSet: true,
			expectedOrigin:  "https://example.com",
		},
		{
			name:            "wildcard origin",
			allowedOrigins:  []string{"*"},
			requestOrigin:   "https://any-origin.com",
			method:          "GET",
			expectedStatus:  http.StatusOK,
			expectOriginSet: true,
			expectedOrigin:  "https://any-origin.com",
		},
		{
			name:            "disallowed origin",
			allowedOrigins:  []string{"https://example.com"},
			requestOrigin:   "https://malicious.com",
			method:          "GET",
			expectedStatus:  http.StatusOK,
			expectOriginSet: false,
		},
		{
			name:            "options request",
			allowedOrigins:  []string{"https://example.com"},
			requestOrigin:   "https://example.com",
			method:          "OPTIONS",
			expectedStatus:  http.StatusNoContent,
			expectOriginSet: true,
			expectedOrigin:  "https://example.com",
		},
		{
			name:            "no origin header",
			allowedOrigins:  []string{"https://example.com"},
			requestOrigin:   "",
			method:          "GET",
			expectedStatus:  http.StatusOK,
			expectOriginSet: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := setupTestRouterForMiddleware()
			router.Use(CORSMiddleware(tt.allowedOrigins))
			
			// Add a test endpoint
			router.GET("/test", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "test"})
			})

			req := httptest.NewRequest(tt.method, "/test", nil)
			if tt.requestOrigin != "" {
				req.Header.Set("Origin", tt.requestOrigin)
			}

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			assert.Equal(t, tt.expectedStatus, recorder.Code)

			// Check CORS headers
			if tt.expectOriginSet {
				assert.Equal(t, tt.expectedOrigin, recorder.Header().Get("Access-Control-Allow-Origin"))
			} else {
				assert.Empty(t, recorder.Header().Get("Access-Control-Allow-Origin"))
			}

			// Check other CORS headers are always set
			assert.Equal(t, "GET, POST, PUT, DELETE, OPTIONS", recorder.Header().Get("Access-Control-Allow-Methods"))
			assert.Equal(t, "Accept, Authorization, Content-Type, X-CSRF-Token, X-Correlation-ID, X-Request-ID", recorder.Header().Get("Access-Control-Allow-Headers"))
			assert.Equal(t, "Link", recorder.Header().Get("Access-Control-Expose-Headers"))
			assert.Equal(t, "true", recorder.Header().Get("Access-Control-Allow-Credentials"))
			assert.Equal(t, "300", recorder.Header().Get("Access-Control-Max-Age"))
		})
	}
}

func TestCORSMiddleware_OriginMatching(t *testing.T) {
	router := setupTestRouterForMiddleware()
	allowedOrigins := []string{"https://example.com", "https://test.com"}
	router.Use(CORSMiddleware(allowedOrigins))
	
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "test"})
	})

	tests := []struct {
		name          string
		origin        string
		expectAllowed bool
	}{
		{"exact match first", "https://example.com", true},
		{"exact match second", "https://test.com", true},
		{"subdomain not allowed", "https://sub.example.com", false},
		{"different protocol", "http://example.com", false},
		{"partial match", "https://example.com.evil.com", false},
		{"case sensitive", "https://Example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("Origin", tt.origin)

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			if tt.expectAllowed {
				assert.Equal(t, tt.origin, recorder.Header().Get("Access-Control-Allow-Origin"))
			} else {
				assert.Empty(t, recorder.Header().Get("Access-Control-Allow-Origin"))
			}
		})
	}
}

func TestRequestIDMiddleware(t *testing.T) {
	router := setupTestRouterForMiddleware()
	router.Use(RequestIDMiddleware())
	
	var capturedCorrelationID string
	router.GET("/test", func(c *gin.Context) {
		// Capture the correlation ID from context
		if id, exists := c.Get("correlation_id"); exists {
			capturedCorrelationID = id.(string)
		}
		c.JSON(http.StatusOK, gin.H{"message": "test"})
	})

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
			router.ServeHTTP(recorder, req)

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
	router := setupTestRouterForMiddleware()
	router.Use(RequestIDMiddleware())
	
	var capturedCorrelationID string
	router.GET("/test", func(c *gin.Context) {
		if id, exists := c.Get("correlation_id"); exists {
			capturedCorrelationID = id.(string)
		}
		c.JSON(http.StatusOK, gin.H{"message": "test"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.NotEmpty(t, capturedCorrelationID)
	
	// Check UUID format (8-4-4-4-12 characters)
	assert.Regexp(t, `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`, capturedCorrelationID)
}

func TestLoggingMiddleware(t *testing.T) {
	router := setupTestRouterForMiddleware()
	router.Use(LoggingMiddleware())
	
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "test response"})
	})

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
			method:         "POST", // Note: our test handler only handles GET
			expectedStatus: http.StatusNotFound, // 404 because no POST route is defined
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.correlationHeader != "" {
				req.Header.Set("X-Correlation-ID", tt.correlationHeader)
			}

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			assert.Equal(t, tt.expectedStatus, recorder.Code)
			
			// Logging middleware should not interfere with the response
			// The actual logging is tested through the formatter function
		})
	}
}

func TestMiddlewareChaining(t *testing.T) {
	router := setupTestRouterForMiddleware()
	
	// Chain all middleware together
	allowedOrigins := []string{"https://example.com"}
	router.Use(CORSMiddleware(allowedOrigins))
	router.Use(RequestIDMiddleware())
	router.Use(LoggingMiddleware())
	
	var capturedCorrelationID string
	router.GET("/test", func(c *gin.Context) {
		if id, exists := c.Get("correlation_id"); exists {
			capturedCorrelationID = id.(string)
		}
		c.JSON(http.StatusOK, gin.H{"message": "test"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)
	
	// Check CORS headers
	assert.Equal(t, "https://example.com", recorder.Header().Get("Access-Control-Allow-Origin"))
	
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