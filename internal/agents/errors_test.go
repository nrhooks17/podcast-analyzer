package agents

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAgentError_Error(t *testing.T) {
	tests := []struct {
		name     string
		agent    string
		message  string
		cause    error
		expected string
	}{
		{
			name:     "error with cause",
			agent:    "summarizer",
			message:  "failed to process",
			cause:    errors.New("connection timeout"),
			expected: "agent summarizer: failed to process: connection timeout",
		},
		{
			name:     "error without cause",
			agent:    "fact_checker",
			message:  "invalid response format",
			cause:    nil,
			expected: "agent fact_checker: invalid response format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &AgentError{
				Agent:   tt.agent,
				Message: tt.message,
				Cause:   tt.cause,
			}

			result := err.Error()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAgentError_Unwrap(t *testing.T) {
	cause := errors.New("original error")
	err := &AgentError{
		Agent:   "test-agent",
		Message: "wrapped error",
		Cause:   cause,
	}

	result := err.Unwrap()
	assert.Equal(t, cause, result)
}

func TestAgentError_Unwrap_NoCause(t *testing.T) {
	err := &AgentError{
		Agent:   "test-agent",
		Message: "error without cause",
		Cause:   nil,
	}

	result := err.Unwrap()
	assert.Nil(t, result)
}

func TestNewAgentError(t *testing.T) {
	cause := errors.New("root cause")

	err := NewAgentError("summarizer", "processing failed", cause)

	assert.NotNil(t, err)
	assert.Equal(t, "summarizer", err.Agent)
	assert.Equal(t, "processing failed", err.Message)
	assert.Equal(t, cause, err.Cause)
}

func TestRateLimitError_Error(t *testing.T) {
	tests := []struct {
		name       string
		agent      string
		retryAfter int
		cause      error
		expected   string
	}{
		{
			name:       "rate limit with cause",
			agent:      "fact_checker",
			retryAfter: 60,
			cause:      errors.New("429 Too Many Requests"),
			expected:   "rate limit exceeded for agent fact_checker (retry after 60s): 429 Too Many Requests",
		},
		{
			name:       "rate limit without cause",
			agent:      "summarizer",
			retryAfter: 30,
			cause:      nil,
			expected:   "rate limit exceeded for agent summarizer (retry after 30s): <nil>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &RateLimitError{
				Agent:      tt.agent,
				RetryAfter: tt.retryAfter,
				Cause:      tt.cause,
			}

			result := err.Error()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRateLimitError_Unwrap(t *testing.T) {
	cause := errors.New("HTTP 429")
	err := &RateLimitError{
		Agent:      "test-agent",
		RetryAfter: 60,
		Cause:      cause,
	}

	result := err.Unwrap()
	assert.Equal(t, cause, result)
}

func TestNewRateLimitError(t *testing.T) {
	cause := errors.New("rate limit exceeded")

	err := NewRateLimitError("fact_checker", 120, cause)

	assert.NotNil(t, err)
	assert.Equal(t, "fact_checker", err.Agent)
	assert.Equal(t, 120, err.RetryAfter)
	assert.Equal(t, cause, err.Cause)
}

func TestAPIError_Error(t *testing.T) {
	tests := []struct {
		name       string
		agent      string
		statusCode int
		message    string
		cause      error
		expected   string
	}{
		{
			name:       "API error with cause",
			agent:      "summarizer",
			statusCode: 500,
			message:    "Internal server error",
			cause:      errors.New("server unavailable"),
			expected:   "API error for agent summarizer (status 500): Internal server error",
		},
		{
			name:       "API error without cause",
			agent:      "takeaway_extractor",
			statusCode: 400,
			message:    "Bad request",
			cause:      nil,
			expected:   "API error for agent takeaway_extractor (status 400): Bad request",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &APIError{
				Agent:      tt.agent,
				StatusCode: tt.statusCode,
				Message:    tt.message,
				Cause:      tt.cause,
			}

			result := err.Error()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAPIError_Unwrap(t *testing.T) {
	cause := errors.New("network error")
	err := &APIError{
		Agent:      "test-agent",
		StatusCode: 503,
		Message:    "Service unavailable",
		Cause:      cause,
	}

	result := err.Unwrap()
	assert.Equal(t, cause, result)
}

func TestNewAPIError(t *testing.T) {
	cause := errors.New("connection refused")

	err := NewAPIError("summarizer", 503, "Service unavailable", cause)

	assert.NotNil(t, err)
	assert.Equal(t, "summarizer", err.Agent)
	assert.Equal(t, 503, err.StatusCode)
	assert.Equal(t, "Service unavailable", err.Message)
	assert.Equal(t, cause, err.Cause)
}

func TestIsRateLimitError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "rate limit error",
			err:      &RateLimitError{Agent: "test", RetryAfter: 60},
			expected: true,
		},
		{
			name:     "wrapped rate limit error",
			err:      NewAgentError("test", "wrapped", &RateLimitError{Agent: "test", RetryAfter: 60}),
			expected: true,
		},
		{
			name:     "different error type",
			err:      &AgentError{Agent: "test", Message: "not rate limit"},
			expected: false,
		},
		{
			name:     "standard error",
			err:      errors.New("standard error"),
			expected: false,
		},
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRateLimitError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "rate limit error - retryable",
			err:      &RateLimitError{Agent: "test", RetryAfter: 60},
			expected: true,
		},
		{
			name:     "API error 500 - retryable",
			err:      &APIError{Agent: "test", StatusCode: http.StatusInternalServerError},
			expected: true,
		},
		{
			name:     "API error 502 - retryable",
			err:      &APIError{Agent: "test", StatusCode: http.StatusBadGateway},
			expected: true,
		},
		{
			name:     "API error 503 - retryable",
			err:      &APIError{Agent: "test", StatusCode: http.StatusServiceUnavailable},
			expected: true,
		},
		{
			name:     "API error 504 - retryable",
			err:      &APIError{Agent: "test", StatusCode: http.StatusGatewayTimeout},
			expected: true,
		},
		{
			name:     "API error 429 - retryable",
			err:      &APIError{Agent: "test", StatusCode: http.StatusTooManyRequests},
			expected: true,
		},
		{
			name:     "API error 400 - not retryable",
			err:      &APIError{Agent: "test", StatusCode: http.StatusBadRequest},
			expected: false,
		},
		{
			name:     "API error 401 - not retryable",
			err:      &APIError{Agent: "test", StatusCode: http.StatusUnauthorized},
			expected: false,
		},
		{
			name:     "API error 404 - not retryable",
			err:      &APIError{Agent: "test", StatusCode: http.StatusNotFound},
			expected: false,
		},
		{
			name:     "agent error - not retryable",
			err:      &AgentError{Agent: "test", Message: "validation failed"},
			expected: false,
		},
		{
			name:     "standard error - not retryable",
			err:      errors.New("standard error"),
			expected: false,
		},
		{
			name:     "nil error - not retryable",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryableError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

