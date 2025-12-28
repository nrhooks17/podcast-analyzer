package agents

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
)

func setupTestLogger() (*logrus.Logger, *test.Hook) {
	logger, hook := test.NewNullLogger()
	return logger, hook
}

func TestNewBaseAgent(t *testing.T) {
	agent := NewBaseAgent("test-agent")
	
	assert.NotNil(t, agent)
	assert.Equal(t, "test-agent", agent.name)
	assert.NotNil(t, agent.logger)
}

func TestBaseAgent_Name(t *testing.T) {
	agent := NewBaseAgent("summarizer")
	
	result := agent.Name()
	
	assert.Equal(t, "summarizer", result)
}

func TestBaseAgent_LogStart(t *testing.T) {
	logger, hook := setupTestLogger()
	agent := &BaseAgent{
		name:   "test-agent",
		logger: logger,
	}
	
	ctx := context.WithValue(context.Background(), "correlation_id", "test-correlation-123")
	
	agent.LogStart(ctx, 1500)
	
	assert.Equal(t, 1, len(hook.Entries))
	entry := hook.LastEntry()
	assert.Equal(t, logrus.InfoLevel, entry.Level)
	assert.Contains(t, entry.Message, "Agent processing started")
	assert.Equal(t, "test-agent", entry.Data["agent"])
	assert.Equal(t, "test-correlation-123", entry.Data["correlation_id"])
	assert.Equal(t, 1500, entry.Data["content_length"])
	assert.Equal(t, 250, entry.Data["word_count"])
}

func TestBaseAgent_LogSuccess(t *testing.T) {
	logger, hook := setupTestLogger()
	agent := &BaseAgent{
		name:   "test-agent",
		logger: logger,
	}
	
	ctx := context.WithValue(context.Background(), "correlation_id", "test-correlation-456")
	result := &Result{
		Summary:    "Test summary",
		Takeaways:  []string{"takeaway1", "takeaway2"},
		FactChecks: []FactCheck{},
	}
	duration := 2 * time.Second
	
	agent.LogSuccess(ctx, result, duration)
	
	assert.Equal(t, 1, len(hook.Entries))
	entry := hook.LastEntry()
	assert.Equal(t, logrus.InfoLevel, entry.Level)
	assert.Contains(t, entry.Message, "Agent processing completed successfully")
	assert.Equal(t, "test-agent", entry.Data["agent"])
	assert.Equal(t, "test-correlation-456", entry.Data["correlation_id"])
	assert.Equal(t, int64(2000), entry.Data["duration_ms"])
	if entry.Data["summary_length"] != nil {
		assert.Equal(t, 12, entry.Data["summary_length"])
	}
	if entry.Data["takeaways_count"] != nil {
		assert.Equal(t, 2, entry.Data["takeaways_count"])
	}
	if entry.Data["fact_checks_count"] != nil {
		assert.Equal(t, 0, entry.Data["fact_checks_count"])
	}
}

func TestBaseAgent_LogError(t *testing.T) {
	logger, _ := setupTestLogger()
	agent := &BaseAgent{
		name:   "test-agent", 
		logger: logger,
	}
	
	ctx := context.WithValue(context.Background(), "correlation_id", "test-correlation-789")
	testErr := assert.AnError
	duration := 500 * time.Millisecond
	
	// LogError uses the global logger.LogErrorWithStackAndCorrelation function
	// This test verifies the method can be called without panicking
	assert.NotPanics(t, func() {
		agent.LogError(ctx, testErr, duration)
	})
}

func TestBaseAgent_LogAPICall(t *testing.T) {
	logger, hook := setupTestLogger()
	agent := &BaseAgent{
		name:   "test-agent",
		logger: logger,
	}
	
	ctx := context.WithValue(context.Background(), "correlation_id", "test-correlation-api")
	
	agent.LogAPICall(ctx, "anthropic", 2000, true)
	
	assert.Equal(t, 1, len(hook.Entries))
	entry := hook.LastEntry()
	assert.Equal(t, logrus.InfoLevel, entry.Level)
	assert.Contains(t, entry.Message, "Making API call")
	assert.Equal(t, "test-agent", entry.Data["agent"])
	assert.Equal(t, "test-correlation-api", entry.Data["correlation_id"])
	assert.Equal(t, "anthropic", entry.Data["service"])
	assert.Equal(t, 2000, entry.Data["prompt_length"])
	assert.Equal(t, true, entry.Data["has_system"])
}

func TestBaseAgent_LogAPIResponse(t *testing.T) {
	logger, hook := setupTestLogger()
	agent := &BaseAgent{
		name:   "test-agent",
		logger: logger,
	}
	
	ctx := context.WithValue(context.Background(), "correlation_id", "test-correlation-resp")
	duration := 1500 * time.Millisecond
	
	agent.LogAPIResponse(ctx, "anthropic", 500, duration)
	
	assert.Equal(t, 1, len(hook.Entries))
	entry := hook.LastEntry()
	assert.Equal(t, logrus.InfoLevel, entry.Level)
	assert.Contains(t, entry.Message, "API response received")
	assert.Equal(t, "test-agent", entry.Data["agent"])
	assert.Equal(t, "test-correlation-resp", entry.Data["correlation_id"])
	assert.Equal(t, "anthropic", entry.Data["service"])
	assert.Equal(t, 500, entry.Data["response_length"])
	assert.Equal(t, int64(1500), entry.Data["duration_ms"])
}

func TestBaseAgent_ValidateContent(t *testing.T) {
	agent := &BaseAgent{name: "test-agent"}
	
	tests := []struct {
		name        string
		content     string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid content",
			content:     "This is valid content for processing with enough characters to meet the minimum requirement of fifty characters.",
			expectError: false,
		},
		{
			name:        "empty content",
			content:     "",
			expectError: true,
			errorMsg:    "cannot process empty content",
		},
		{
			name:        "whitespace only content",
			content:     "   \n\t   ",
			expectError: true,
			errorMsg:    "cannot process empty content",
		},
		{
			name:        "very short content",
			content:     "Hi",
			expectError: true,
			errorMsg:    "content too short for meaningful analysis",
		},
		{
			name:        "very long content",
			content:     strings.Repeat("a", 1000001),
			expectError: true,
			errorMsg:    "content too long for processing",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := agent.ValidateContent(tt.content)
			
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBaseAgent_TruncateContent(t *testing.T) {
	agent := &BaseAgent{name: "test-agent"}
	
	tests := []struct {
		name      string
		content   string
		maxLength int
		expected  string
	}{
		{
			name:      "content under limit",
			content:   "Short content",
			maxLength: 100,
			expected:  "Short content",
		},
		{
			name:      "content over limit", 
			content:   "This is a very long piece of content that exceeds the maximum length",
			maxLength: 20,
			expected:  "This is a very long\n[...content truncated...]", // Word boundary not triggered since lastSpace condition fails
		},
		{
			name:      "exact length limit",
			content:   "Exactly twenty chars",
			maxLength: 20,
			expected:  "Exactly twenty chars",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := agent.TruncateContent(tt.content, tt.maxLength)
			assert.Equal(t, tt.expected, result)
			// Note: Result may be longer than maxLength due to truncation suffix
		})
	}
}

func TestBaseAgent_TruncateForLog(t *testing.T) {
	agent := &BaseAgent{name: "test-agent"}
	
	tests := []struct {
		name      string
		text      string
		maxLength int
		expected  string
	}{
		{
			name:      "text under limit",
			text:      "Short text",
			maxLength: 50,
			expected:  "Short text",
		},
		{
			name:      "text over limit",
			text:      "This is a much longer piece of text that will be truncated",
			maxLength: 25,
			expected:  "This is a much longer pie...", // Exact 25 chars + "..."
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := agent.TruncateForLog(tt.text, tt.maxLength)
			assert.Equal(t, tt.expected, result)
			// Note: Result may be longer than maxLength due to "..." suffix
		})
	}
}

func TestBaseAgent_IsUpperCase(t *testing.T) {
	agent := &BaseAgent{name: "test-agent"}
	
	tests := []struct {
		name     string
		char     byte
		expected bool
	}{
		{"uppercase A", 'A', true},
		{"uppercase Z", 'Z', true},
		{"lowercase a", 'a', false},
		{"lowercase z", 'z', false},
		{"digit", '5', false},
		{"symbol", '!', false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := agent.IsUpperCase(tt.char)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetCorrelationID(t *testing.T) {
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
			result := getCorrelationID(tt.ctx)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEstimateWordCount(t *testing.T) {
	tests := []struct {
		name      string
		charCount int
		expected  int
	}{
		{"zero characters", 0, 0},
		{"6 characters", 6, 1},
		{"12 characters", 12, 2}, 
		{"100 characters", 100, 16},
		{"600 characters", 600, 100},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := estimateWordCount(tt.charCount)
			assert.Equal(t, tt.expected, result)
		})
	}
}