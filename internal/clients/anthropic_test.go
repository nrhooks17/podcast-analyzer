package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"podcast-analyzer/internal/config"

	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockHTTPClient for testing HTTP interactions
type MockHTTPClient struct {
	mock.Mock
}

func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*http.Response), args.Error(1)
}

func setupTestAnthropicClient() (*AnthropicClient, *test.Hook) {
	cfg := &config.Config{
		AnthropicAPIKey: "test-api-key",
		ClaudeModel:     "claude-3-sonnet-20240229",
	}
	
	logger, hook := test.NewNullLogger()
	client := NewAnthropicClient(cfg)
	client.logger = logger
	
	return client, hook
}

func TestNewAnthropicClient(t *testing.T) {
	cfg := &config.Config{
		AnthropicAPIKey: "test-api-key",
		ClaudeModel:     "claude-3-sonnet-20240229",
	}

	client := NewAnthropicClient(cfg)

	assert.NotNil(t, client)
	assert.Equal(t, "test-api-key", client.apiKey)
	assert.Equal(t, "claude-3-sonnet-20240229", client.model)
	assert.Equal(t, "https://api.anthropic.com/v1/messages", client.baseURL)
	assert.NotNil(t, client.httpClient)
	assert.Equal(t, 120*time.Second, client.httpClient.Timeout)
}

func TestAnthropicError_Error(t *testing.T) {
	err := &AnthropicError{
		Type:    "invalid_request_error",
		Message: "Missing required field",
	}

	result := err.Error()
	expected := "anthropic API error (invalid_request_error): Missing required field"
	assert.Equal(t, expected, result)
}

func TestAnthropicClient_CallClaude_Success(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/messages", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "test-api-key", r.Header.Get("x-api-key"))
		assert.Equal(t, "2023-06-01", r.Header.Get("anthropic-version"))

		// Return successful response
		response := AnthropicResponse{
			ID:   "msg_123",
			Type: "message",
			Role: "assistant",
			Content: []AnthropicContent{
				{
					Type: "text",
					Text: "This is a test response from Claude",
				},
			},
			Model: "claude-3-sonnet-20240229",
			Usage: AnthropicUsage{
				InputTokens:  50,
				OutputTokens: 25,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client, _ := setupTestAnthropicClient()
	client.baseURL = server.URL + "/v1/messages"

	ctx := context.WithValue(context.Background(), "correlation_id", "test-correlation-123")
	result, err := client.CallClaude(ctx, "test-agent", "Test prompt", "Test system prompt", false)

	assert.NoError(t, err)
	assert.Equal(t, "This is a test response from Claude", result)
}

func TestAnthropicClient_CallClaude_WithWebSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify web search headers
		assert.Equal(t, "web-search-2025-03-05", r.Header.Get("anthropic-beta"))

		// Verify request body includes tools
		body, _ := io.ReadAll(r.Body)
		var request AnthropicRequest
		json.Unmarshal(body, &request)
		assert.Len(t, request.Tools, 1)
		assert.Equal(t, "web_search", request.Tools[0].Type)

		response := AnthropicResponse{
			Content: []AnthropicContent{{Type: "text", Text: "Response with web search"}},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client, _ := setupTestAnthropicClient()
	client.baseURL = server.URL + "/v1/messages"

	ctx := context.Background()
	result, err := client.CallClaude(ctx, "test-agent", "Test prompt", "", true)

	assert.NoError(t, err)
	assert.Equal(t, "Response with web search", result)
}

func TestAnthropicClient_CallClaude_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		apiErr := AnthropicError{
			Type:    "invalid_request_error",
			Message: "Invalid prompt format",
		}
		json.NewEncoder(w).Encode(apiErr)
	}))
	defer server.Close()

	client, _ := setupTestAnthropicClient()
	client.baseURL = server.URL + "/v1/messages"

	ctx := context.Background()
	result, err := client.CallClaude(ctx, "test-agent", "Test prompt", "", false)

	assert.Error(t, err)
	assert.Empty(t, result)
	assert.Contains(t, err.Error(), "API error (status 400)")
	assert.Contains(t, err.Error(), "Invalid prompt format")
}

func TestAnthropicClient_CallClaude_RateLimitError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
		apiErr := AnthropicError{
			Type:    "rate_limit_error",
			Message: "Rate limit exceeded",
		}
		json.NewEncoder(w).Encode(apiErr)
	}))
	defer server.Close()

	client, _ := setupTestAnthropicClient()
	client.baseURL = server.URL + "/v1/messages"

	ctx := context.Background()
	result, err := client.CallClaude(ctx, "test-agent", "Test prompt", "", false)

	assert.Error(t, err)
	assert.Empty(t, result)
	assert.Contains(t, err.Error(), "server error after retries")
}

func TestAnthropicClient_makeRequestWithRetry_Success(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			// Fail first attempt
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		// Succeed on retry
		response := AnthropicResponse{
			Content: []AnthropicContent{{Type: "text", Text: "Success after retry"}},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client, _ := setupTestAnthropicClient()
	
	ctx := context.Background()
	req, _ := http.NewRequestWithContext(ctx, "POST", server.URL, bytes.NewBufferString("test"))
	
	resp, err := client.makeRequestWithRetry(ctx, req, "test-agent", 2)
	
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 2, callCount)
	resp.Body.Close()
}

func TestAnthropicClient_makeRequestWithRetry_ExceedsMaxRetries(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client, _ := setupTestAnthropicClient()
	
	ctx := context.Background()
	req, _ := http.NewRequestWithContext(ctx, "POST", server.URL, bytes.NewBufferString("test"))
	
	resp, err := client.makeRequestWithRetry(ctx, req, "test-agent", 2)
	
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "server error after retries")
	assert.Equal(t, 3, callCount) // Initial attempt + 2 retries
}

func TestAnthropicClient_makeRequestWithRetry_ContextCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond) // Simulate slow response
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client, _ := setupTestAnthropicClient()
	
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	
	req, _ := http.NewRequestWithContext(ctx, "POST", server.URL, bytes.NewBufferString("test"))
	
	resp, err := client.makeRequestWithRetry(ctx, req, "test-agent", 2)
	
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestAnthropicClient_buildAnthropicRequest(t *testing.T) {
	client, _ := setupTestAnthropicClient()

	tests := []struct {
		name          string
		prompt        string
		systemPrompt  string
		useWebSearch  bool
		expectedTools int
		hasSystem     bool
	}{
		{
			name:          "basic request",
			prompt:        "Test prompt",
			systemPrompt:  "",
			useWebSearch:  false,
			expectedTools: 0,
			hasSystem:     false,
		},
		{
			name:          "request with system prompt",
			prompt:        "Test prompt",
			systemPrompt:  "Test system prompt",
			useWebSearch:  false,
			expectedTools: 0,
			hasSystem:     true,
		},
		{
			name:          "request with web search",
			prompt:        "Test prompt",
			systemPrompt:  "",
			useWebSearch:  true,
			expectedTools: 1,
			hasSystem:     false,
		},
		{
			name:          "request with system prompt and web search",
			prompt:        "Test prompt",
			systemPrompt:  "Test system prompt",
			useWebSearch:  true,
			expectedTools: 1,
			hasSystem:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.buildAnthropicRequest(tt.prompt, tt.systemPrompt, tt.useWebSearch)

			assert.Equal(t, "claude-3-sonnet-20240229", result.Model)
			assert.Equal(t, 4000, result.MaxTokens)
			assert.Equal(t, 0.1, result.Temperature)
			assert.Len(t, result.Messages, 1)
			assert.Equal(t, "user", result.Messages[0].Role)
			assert.Equal(t, tt.prompt, result.Messages[0].Content)
			assert.Len(t, result.Tools, tt.expectedTools)
			
			if tt.hasSystem {
				assert.Equal(t, tt.systemPrompt, result.System)
			} else {
				assert.Empty(t, result.System)
			}
			
			if tt.useWebSearch {
				assert.Equal(t, "web_search", result.Tools[0].Type)
				assert.Equal(t, "web_search", result.Tools[0].Name)
			}
		})
	}
}

func TestAnthropicClient_prepareHTTPRequest(t *testing.T) {
	client, _ := setupTestAnthropicClient()
	
	tests := []struct {
		name         string
		useWebSearch bool
		expectBeta   bool
	}{
		{
			name:         "without web search",
			useWebSearch: false,
			expectBeta:   false,
		},
		{
			name:         "with web search",
			useWebSearch: true,
			expectBeta:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			requestBody := []byte(`{"test": "data"}`)

			req, err := client.prepareHTTPRequest(ctx, requestBody, tt.useWebSearch)

			assert.NoError(t, err)
			assert.NotNil(t, req)
			assert.Equal(t, "POST", req.Method)
			assert.Equal(t, client.baseURL, req.URL.String())
			assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
			assert.Equal(t, "test-api-key", req.Header.Get("x-api-key"))
			assert.Equal(t, "2023-06-01", req.Header.Get("anthropic-version"))
			
			if tt.expectBeta {
				assert.Equal(t, "web-search-2025-03-05", req.Header.Get("anthropic-beta"))
			} else {
				assert.Empty(t, req.Header.Get("anthropic-beta"))
			}
		})
	}
}

func TestAnthropicClient_parseAnthropicResponse_Success(t *testing.T) {
	client, _ := setupTestAnthropicClient()

	response := AnthropicResponse{
		ID:   "msg_123",
		Type: "message",
		Role: "assistant",
		Content: []AnthropicContent{
			{
				Type: "text",
				Text: "Test response text",
			},
		},
		Model: "claude-3-sonnet-20240229",
		Usage: AnthropicUsage{
			InputTokens:  100,
			OutputTokens: 50,
		},
	}

	responseBody, _ := json.Marshal(response)
	httpResp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(string(responseBody))),
	}

	responseText, anthropicResp, err := client.parseAnthropicResponse(httpResp)

	assert.NoError(t, err)
	assert.Equal(t, "Test response text", responseText)
	assert.NotNil(t, anthropicResp)
	assert.Equal(t, "msg_123", anthropicResp.ID)
	assert.Equal(t, 100, anthropicResp.Usage.InputTokens)
	assert.Equal(t, 50, anthropicResp.Usage.OutputTokens)
}

func TestAnthropicClient_parseAnthropicResponse_EmptyContent(t *testing.T) {
	client, _ := setupTestAnthropicClient()

	response := AnthropicResponse{
		Content: []AnthropicContent{}, // Empty content
	}

	responseBody, _ := json.Marshal(response)
	httpResp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(string(responseBody))),
	}

	responseText, anthropicResp, err := client.parseAnthropicResponse(httpResp)

	assert.Error(t, err)
	assert.Empty(t, responseText)
	assert.Nil(t, anthropicResp)
	assert.Contains(t, err.Error(), "empty response content")
}

func TestAnthropicClient_parseAnthropicResponse_EmptyText(t *testing.T) {
	client, _ := setupTestAnthropicClient()

	response := AnthropicResponse{
		Content: []AnthropicContent{
			{
				Type: "text",
				Text: "", // Empty text
			},
		},
	}

	responseBody, _ := json.Marshal(response)
	httpResp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(string(responseBody))),
	}

	responseText, anthropicResp, err := client.parseAnthropicResponse(httpResp)

	assert.Error(t, err)
	assert.Empty(t, responseText)
	assert.Nil(t, anthropicResp)
	assert.Contains(t, err.Error(), "empty response text")
}

func TestAnthropicClient_parseAnthropicResponse_InvalidJSON(t *testing.T) {
	client, _ := setupTestAnthropicClient()

	httpResp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader("invalid json")),
	}

	responseText, anthropicResp, err := client.parseAnthropicResponse(httpResp)

	assert.Error(t, err)
	assert.Empty(t, responseText)
	assert.Nil(t, anthropicResp)
	assert.Contains(t, err.Error(), "failed to parse response")
}

func TestGetCorrelationIDFromContext(t *testing.T) {
	tests := []struct {
		name     string
		ctx      context.Context
		expected string
	}{
		{
			name:     "context with correlation ID",
			ctx:      context.WithValue(context.Background(), "correlation_id", "test-id-123"),
			expected: "test-id-123",
		},
		{
			name:     "context without correlation ID",
			ctx:      context.Background(),
			expected: "",
		},
		{
			name:     "context with wrong type",
			ctx:      context.WithValue(context.Background(), "correlation_id", 12345),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getCorrelationIDFromContext(tt.ctx)
			assert.Equal(t, tt.expected, result)
		})
	}
}