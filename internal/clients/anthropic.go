package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
	
	"podcast-analyzer/internal/config"
	"podcast-analyzer/internal/logger"
	
	"github.com/sirupsen/logrus"
)

// AnthropicClientInterface defines the interface for Anthropic API client
type AnthropicClientInterface interface {
	CallClaude(ctx context.Context, agentName, prompt, systemPrompt string, useWebSearch bool) (string, error)
}

// AnthropicClient handles communication with the Anthropic API
type AnthropicClient struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
	logger     *logrus.Logger
}

// AnthropicRequest represents a request to the Anthropic API
type AnthropicRequest struct {
	Model       string                 `json:"model"`
	MaxTokens   int                    `json:"max_tokens"`
	Temperature float64                `json:"temperature"`
	Messages    []AnthropicMessage     `json:"messages"`
	System      string                 `json:"system,omitempty"`
	Tools       []AnthropicTool        `json:"tools,omitempty"`
}

// AnthropicMessage represents a message in the conversation
type AnthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// AnthropicTool represents a tool that can be used by Claude
type AnthropicTool struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

// AnthropicResponse represents a response from the Anthropic API
type AnthropicResponse struct {
	ID      string              `json:"id"`
	Type    string              `json:"type"`
	Role    string              `json:"role"`
	Content []AnthropicContent  `json:"content"`
	Model   string              `json:"model"`
	Usage   AnthropicUsage      `json:"usage"`
}

// AnthropicContent represents content in the response
type AnthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// AnthropicUsage represents token usage information
type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// AnthropicError represents an error response from the API
type AnthropicError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func (e *AnthropicError) Error() string {
	return fmt.Sprintf("anthropic API error (%s): %s", e.Type, e.Message)
}

// NewAnthropicClient creates a new Anthropic API client
func NewAnthropicClient(cfg *config.Config) *AnthropicClient {
	return &AnthropicClient{
		apiKey:  cfg.AnthropicAPIKey,
		model:   cfg.ClaudeModel,
		baseURL: "https://api.anthropic.com/v1/messages",
		httpClient: &http.Client{
			Timeout: 120 * time.Second, // 2 minute timeout for AI calls
		},
		logger: logger.Log,
	}
}

// CallClaude makes a request to the Claude API
func (c *AnthropicClient) CallClaude(ctx context.Context, agentName, prompt, systemPrompt string, useWebSearch bool) (string, error) {
	start := time.Now()
	
	// Prepare the request
	request := c.buildAnthropicRequest(prompt, systemPrompt, useWebSearch)
	
	// Log the API call
	correlationID := getCorrelationIDFromContext(ctx)
	c.logger.WithFields(map[string]interface{}{
		"agent":          agentName,
		"correlation_id": correlationID,
		"model":          c.model,
		"prompt_length":  len(prompt),
		"has_system":     systemPrompt != "",
		"use_web_search": useWebSearch,
	}).Info("Making Anthropic API call")
	
	// Marshal the request
	requestBody, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}
	
	// Create HTTP request
	httpReq, err := c.prepareHTTPRequest(ctx, requestBody, useWebSearch)
	if err != nil {
		return "", err
	}
	
	// Make the request with retry logic
	response, err := c.makeRequestWithRetry(ctx, httpReq, agentName, 3)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	
	// Parse the response
	responseText, anthropicResp, err := c.parseAnthropicResponse(response)
	if err != nil {
		return "", err
	}
	
	// Log successful response
	duration := time.Since(start)
	c.logger.WithFields(map[string]interface{}{
		"agent":           agentName,
		"correlation_id":  correlationID,
		"duration_ms":     duration.Milliseconds(),
		"response_length": len(responseText),
		"input_tokens":    anthropicResp.Usage.InputTokens,
		"output_tokens":   anthropicResp.Usage.OutputTokens,
	}).Info("Anthropic API response received")
	
	return responseText, nil
}

// makeRequestWithRetry makes an HTTP request with retry logic for retryable errors
func (c *AnthropicClient) makeRequestWithRetry(ctx context.Context, req *http.Request, agentName string, maxRetries int) (*http.Response, error) {
	var lastErr error
	
	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Clone request for retry attempts
		var requestBody []byte
		if req.Body != nil {
			bodyBytes, err := io.ReadAll(req.Body)
			if err != nil {
				return nil, fmt.Errorf("failed to read request body for retry: %w", err)
			}
			requestBody = bodyBytes
			req.Body = io.NopCloser(bytes.NewReader(requestBody))
		}
		
		// Make the request
		response, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("HTTP request failed: %w", err)
			
			// Don't retry on context cancellation or timeout
			if ctx.Err() != nil {
				return nil, lastErr
			}
			
			// Wait before retry
			if attempt < maxRetries {
				waitTime := time.Duration(1<<uint(attempt)) * time.Second // Exponential backoff
				c.logger.WithFields(map[string]interface{}{
					"agent":         agentName,
					"attempt":       attempt + 1,
					"max_attempts":  maxRetries + 1,
					"wait_seconds":  waitTime.Seconds(),
				}).Warn("Request failed, retrying")
				
				select {
				case <-time.After(waitTime):
					continue
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			continue
		}
		
		// Check for retryable HTTP status codes
		if response.StatusCode >= 500 || response.StatusCode == http.StatusTooManyRequests {
			response.Body.Close()
			
			if attempt < maxRetries {
				waitTime := time.Duration(1<<uint(attempt)) * time.Second
				if response.StatusCode == http.StatusTooManyRequests {
					// Use Retry-After header if available
					if retryHeader := response.Header.Get("Retry-After"); retryHeader != "" {
						if seconds, parseErr := strconv.Atoi(retryHeader); parseErr == nil {
							waitTime = time.Duration(seconds) * time.Second
						}
					}
				}
				
				c.logger.WithFields(map[string]interface{}{
					"agent":        agentName,
					"status_code":  response.StatusCode,
					"attempt":      attempt + 1,
					"max_attempts": maxRetries + 1,
					"wait_seconds": waitTime.Seconds(),
				}).Warn("Received retryable status code, retrying")
				
				select {
				case <-time.After(waitTime):
					// Reset request body for next attempt
					if requestBody != nil {
						req.Body = io.NopCloser(bytes.NewReader(requestBody))
					}
					continue
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			
			lastErr = fmt.Errorf("server error after retries (status %d)", response.StatusCode)
			continue
		}
		
		// Success or non-retryable error
		return response, nil
	}
	
	return nil, lastErr
}

// buildAnthropicRequest constructs the request payload for the Anthropic API
func (c *AnthropicClient) buildAnthropicRequest(prompt, systemPrompt string, useWebSearch bool) AnthropicRequest {
	request := AnthropicRequest{
		Model:       c.model,
		MaxTokens:   4000,
		Temperature: 0.1,
		Messages: []AnthropicMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}
	
	// Add system prompt if provided
	if systemPrompt != "" {
		request.System = systemPrompt
	}
	
	// Add web search tool if needed (for fact-checking)
	if useWebSearch {
		request.Tools = []AnthropicTool{
			{
				Type: "web_search",
				Name: "web_search",
			},
		}
	}
	
	return request
}

// prepareHTTPRequest creates and configures the HTTP request
func (c *AnthropicClient) prepareHTTPRequest(ctx context.Context, requestBody []byte, useWebSearch bool) (*http.Request, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	
	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	
	// Add web search beta header if needed
	if useWebSearch {
		httpReq.Header.Set("anthropic-beta", "web-search-2025-03-05")
	}
	
	return httpReq, nil
}

// parseAnthropicResponse parses the successful response from Anthropic API
func (c *AnthropicClient) parseAnthropicResponse(response *http.Response) (string, *AnthropicResponse, error) {
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read response body: %w", err)
	}
	
	// Handle error responses
	if response.StatusCode != http.StatusOK {
		var apiErr AnthropicError
		if json.Unmarshal(responseBody, &apiErr) == nil {
			if response.StatusCode == http.StatusTooManyRequests {
				// Extract retry-after header if present
				retryAfter := 60 // default to 60 seconds
				if retryHeader := response.Header.Get("Retry-After"); retryHeader != "" {
					if parsed, parseErr := strconv.Atoi(retryHeader); parseErr == nil {
						retryAfter = parsed
					}
				}
				return "", nil, fmt.Errorf("rate limit exceeded (retry after %ds): %w", retryAfter, &apiErr)
			}
			return "", nil, fmt.Errorf("API error (status %d): %w", response.StatusCode, &apiErr)
		}
		return "", nil, fmt.Errorf("unknown API error (status %d)", response.StatusCode)
	}
	
	// Parse the successful response
	var anthropicResp AnthropicResponse
	if err := json.Unmarshal(responseBody, &anthropicResp); err != nil {
		return "", nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	// Extract text from response
	if len(anthropicResp.Content) == 0 {
		return "", nil, fmt.Errorf("empty response content")
	}
	
	responseText := anthropicResp.Content[0].Text
	if responseText == "" {
		return "", nil, fmt.Errorf("empty response text")
	}
	
	return responseText, &anthropicResp, nil
}

// getCorrelationIDFromContext extracts correlation ID from context
func getCorrelationIDFromContext(ctx context.Context) string {
	if id := ctx.Value("correlation_id"); id != nil {
		if correlationID, ok := id.(string); ok {
			return correlationID
		}
	}
	return ""
}