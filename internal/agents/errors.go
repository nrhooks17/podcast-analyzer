package agents

import (
	"errors"
	"fmt"
	"net/http"
)

// AgentError represents a general agent processing error
type AgentError struct {
	Agent   string
	Message string
	Cause   error
}

func (e *AgentError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("agent %s: %s: %v", e.Agent, e.Message, e.Cause)
	}
	return fmt.Sprintf("agent %s: %s", e.Agent, e.Message)
}

func (e *AgentError) Unwrap() error {
	return e.Cause
}

// NewAgentError creates a new agent error
func NewAgentError(agent, message string, cause error) *AgentError {
	return &AgentError{
		Agent:   agent,
		Message: message,
		Cause:   cause,
	}
}

// RateLimitError indicates an API rate limit was exceeded
type RateLimitError struct {
	Agent      string
	RetryAfter int // seconds to wait before retrying
	Cause      error
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limit exceeded for agent %s (retry after %ds): %v", e.Agent, e.RetryAfter, e.Cause)
}

func (e *RateLimitError) Unwrap() error {
	return e.Cause
}

// NewRateLimitError creates a new rate limit error
func NewRateLimitError(agent string, retryAfter int, cause error) *RateLimitError {
	return &RateLimitError{
		Agent:      agent,
		RetryAfter: retryAfter,
		Cause:      cause,
	}
}

// APIError represents an error from external API calls
type APIError struct {
	Agent      string
	StatusCode int
	Message    string
	Cause      error
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error for agent %s (status %d): %s", e.Agent, e.StatusCode, e.Message)
}

func (e *APIError) Unwrap() error {
	return e.Cause
}

// NewAPIError creates a new API error
func NewAPIError(agent string, statusCode int, message string, cause error) *APIError {
	return &APIError{
		Agent:      agent,
		StatusCode: statusCode,
		Message:    message,
		Cause:      cause,
	}
}

// IsRateLimitError checks if an error is a rate limit error
func IsRateLimitError(err error) bool {
	var rateLimitErr *RateLimitError
	return errors.As(err, &rateLimitErr)
}

// IsRetryableError checks if an error indicates a retryable condition
func IsRetryableError(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		// Retry on server errors and rate limits
		return apiErr.StatusCode >= 500 || apiErr.StatusCode == http.StatusTooManyRequests
	}
	
	// Also retry on rate limit errors
	return IsRateLimitError(err)
}