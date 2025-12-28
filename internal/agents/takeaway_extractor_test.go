package agents

import (
	"context"
	"strings"
	"testing"

	"podcast-analyzer/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNewTakeawayExtractorAgent(t *testing.T) {
	cfg := &config.Config{
		AnthropicAPIKey: "test-key",
	}

	agent := NewTakeawayExtractorAgent(cfg)

	assert.NotNil(t, agent)
	assert.Equal(t, "takeaway_extractor", agent.Name())
	assert.NotNil(t, agent.anthropicClient)
}

func TestTakeawayExtractorAgent_Process_Success(t *testing.T) {
	mockClient := &MockAnthropicClient{}
	agent := &TakeawayExtractorAgent{
		BaseAgent:       NewBaseAgent("takeaway_extractor"),
		anthropicClient: mockClient,
	}

	ctx := context.Background()
	content := strings.Repeat("This is a long enough podcast content for testing purposes. ", 10) // More than 50 chars
	expectedResponse := "1. First takeaway point here with enough words\n2. Second takeaway point here with enough words\n3. Third takeaway point here with enough words"

	mockClient.On("CallClaude", 
		ctx, 
		"takeaway_extractor", 
		mock.AnythingOfType("string"), 
		mock.AnythingOfType("string"), 
		false,
	).Return(expectedResponse, nil)

	result, err := agent.Process(ctx, content)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Takeaways, 3)
	assert.Contains(t, result.Takeaways[0], "First takeaway point")
	assert.Empty(t, result.Summary)
	assert.Empty(t, result.FactChecks)
	mockClient.AssertExpectations(t)
}

func TestTakeawayExtractorAgent_ProcessWithOptions_Success(t *testing.T) {
	mockClient := &MockAnthropicClient{}
	agent := &TakeawayExtractorAgent{
		BaseAgent:       NewBaseAgent("takeaway_extractor"),
		anthropicClient: mockClient,
	}

	ctx := context.Background()
	content := strings.Repeat("This is a long enough podcast content for testing purposes. ", 10) // More than 50 chars
	summary := "This is the summary context"
	opts := ProcessingOptions{Summary: summary}
	expectedResponse := "• Key insight one with enough words here\n• Key insight two with enough words here"

	mockClient.On("CallClaude", 
		mock.Anything, 
		mock.Anything, 
		mock.AnythingOfType("string"), 
		mock.AnythingOfType("string"), 
		false,
	).Return(expectedResponse, nil)

	result, err := agent.ProcessWithOptions(ctx, content, opts)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Takeaways, 2)
	assert.Contains(t, result.Takeaways[0], "Key insight one")
	mockClient.AssertExpectations(t)
}


func TestTakeawayExtractorAgent_removeListMarkers(t *testing.T) {
	agent := &TakeawayExtractorAgent{
		BaseAgent: NewBaseAgent("takeaway_extractor"),
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "numbered list",
			input:    "1. This is a takeaway",
			expected: "This is a takeaway",
		},
		{
			name:     "numbered list with parenthesis",
			input:    "2) Another takeaway",
			expected: "Another takeaway",
		},
		{
			name:     "bullet point with dash",
			input:    "- Bullet point takeaway",
			expected: "Bullet point takeaway",
		},
		{
			name:     "bullet point with bullet",
			input:    "• Unicode bullet takeaway",
			expected: "Unicode bullet takeaway",
		},
		{
			name:     "bullet point with asterisk",
			input:    "* Asterisk takeaway",
			expected: "Asterisk takeaway",
		},
		{
			name:     "no markers",
			input:    "Plain text without markers",
			expected: "Plain text without markers",
		},
		{
			name:     "multiple spaces after marker",
			input:    "3.   Extra spaces after number",
			expected: "Extra spaces after number",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := agent.removeListMarkers(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTakeawayExtractorAgent_shouldSkipLine(t *testing.T) {
	agent := &TakeawayExtractorAgent{
		BaseAgent: NewBaseAgent("takeaway_extractor"),
	}

	tests := []struct {
		name     string
		line     string
		expected bool
	}{
		{
			name:     "valid takeaway",
			line:     "This is a substantial takeaway point",
			expected: false,
		},
		{
			name:     "too short",
			line:     "Short",
			expected: true,
		},
		{
			name:     "contains skip phrase - key takeaways",
			line:     "Key takeaways from the discussion",
			expected: true,
		},
		{
			name:     "contains skip phrase - takeaways:",
			line:     "Takeaways: here are the points",
			expected: true,
		},
		{
			name:     "contains skip phrase - summary:",
			line:     "Summary: this is a summary",
			expected: true,
		},
		{
			name:     "contains skip phrase - in conclusion",
			line:     "In conclusion, we learned a lot",
			expected: true,
		},
		{
			name:     "contains skip phrase - to summarize",
			line:     "To summarize the main points",
			expected: true,
		},
		{
			name:     "case insensitive skip phrase",
			line:     "KEY TAKEAWAYS from today",
			expected: true,
		},
		{
			name:     "empty line",
			line:     "",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := agent.shouldSkipLine(tt.line)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTakeawayExtractorAgent_processTakeawayLine(t *testing.T) {
	agent := &TakeawayExtractorAgent{
		BaseAgent: NewBaseAgent("takeaway_extractor"),
	}

	tests := []struct {
		name     string
		line     string
		expected string
	}{
		{
			name:     "valid numbered takeaway",
			line:     "1. This is a good takeaway point that should be processed",
			expected: "This is a good takeaway point that should be processed.",
		},
		{
			name:     "takeaway with bullet",
			line:     "• Another excellent insight worth keeping in the list",
			expected: "Another excellent insight worth keeping in the list.",
		},
		{
			name:     "empty line",
			line:     "",
			expected: "",
		},
		{
			name:     "line to skip",
			line:     "Key takeaways:",
			expected: "",
		},
		{
			name:     "too short line",
			line:     "- No",
			expected: "",
		},
		{
			name:     "line with extra whitespace",
			line:     "  2.   Takeaway with extra spaces   ",
			expected: "Takeaway with extra spaces.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := agent.processTakeawayLine(tt.line)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTakeawayExtractorAgent_parseTakeaways(t *testing.T) {
	agent := &TakeawayExtractorAgent{
		BaseAgent: NewBaseAgent("takeaway_extractor"),
	}

	tests := []struct {
		name     string
		response string
		expected []string
	}{
		{
			name: "numbered list",
			response: "1. First important takeaway here\n2. Second important takeaway here\n3. Third important takeaway here",
			expected: []string{
				"First important takeaway here.",
				"Second important takeaway here.",
				"Third important takeaway here.",
			},
		},
		{
			name: "bullet points",
			response: "• First important point here\n• Second important point here\n• Third important point here",
			expected: []string{
				"First important point here.",
				"Second important point here.",
				"Third important point here.",
			},
		},
		{
			name: "mixed format with headers to skip",
			response: "Key takeaways:\n\n1. Important insight one\n2. Important insight two\n\nSummary:\nThat's all",
			expected: []string{
				"Important insight one.",
				"Important insight two.",
			},
		},
		{
			name:     "empty response",
			response: "",
			expected: nil, // parseTakeaways returns nil for empty input, not empty slice
		},
		{
			name: "response with too many takeaways",
			response: "1. First important takeaway here\n2. Second important takeaway here\n3. Third important takeaway here\n4. Fourth important takeaway here\n5. Fifth important takeaway here\n6. Sixth important takeaway here\n7. Seventh important takeaway here\n8. Eighth important takeaway here\n9. Ninth important takeaway here\n10. Tenth important takeaway here\n11. Eleventh important takeaway here\n12. Twelfth important takeaway here",
			expected: []string{
				"First important takeaway here.", "Second important takeaway here.", "Third important takeaway here.", "Fourth important takeaway here.", "Fifth important takeaway here.", 
				"Sixth important takeaway here.", "Seventh important takeaway here.", "Eighth important takeaway here.", "Ninth important takeaway here.", "Tenth important takeaway here.",
			}, // Should be truncated to 10
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := agent.parseTakeaways(tt.response)
			assert.Equal(t, tt.expected, result)
			assert.LessOrEqual(t, len(result), 10) // Should never exceed 10
		})
	}
}

func TestTakeawayExtractorAgent_cleanTakeaway(t *testing.T) {
	agent := &TakeawayExtractorAgent{
		BaseAgent: NewBaseAgent("takeaway_extractor"),
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "clean takeaway",
			input:    "This is already clean",
			expected: "This is already clean.",
		},
		{
			name:     "takeaway without period",
			input:    "This needs a period",
			expected: "This needs a period.",
		},
		{
			name:     "takeaway with extra whitespace",
			input:    "  Trim this whitespace  ",
			expected: "Trim this whitespace.",
		},
		{
			name:     "takeaway starting lowercase",
			input:    "lowercase start needs fixing",
			expected: "Lowercase start needs fixing.",
		},
		{
			name:     "takeaway with period already",
			input:    "This already has a period.",
			expected: "This already has a period.",
		},
		{
			name:     "takeaway with other punctuation",
			input:    "Question mark at end?",
			expected: "Question mark at end?",
		},
		{
			name:     "takeaway with exclamation",
			input:    "Exclamation at end!",
			expected: "Exclamation at end!",
		},
		{
			name:     "empty takeaway",
			input:    "",
			expected: "",
		},
		{
			name:     "whitespace only",
			input:    "   \t\n   ",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := agent.cleanTakeaway(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTakeawayExtractorAgent_buildSystemPrompt(t *testing.T) {
	agent := &TakeawayExtractorAgent{
		BaseAgent: NewBaseAgent("takeaway_extractor"),
	}

	prompt := agent.buildSystemPrompt()

	assert.Contains(t, prompt, "actionable takeaways")
	assert.Contains(t, prompt, "key insights")
	assert.Contains(t, prompt, "numbered list")
	assert.Contains(t, prompt, "substantive content")
}

func TestTakeawayExtractorAgent_buildUserPrompt(t *testing.T) {
	agent := &TakeawayExtractorAgent{
		BaseAgent: NewBaseAgent("takeaway_extractor"),
	}

	tests := []struct {
		name            string
		content         string
		summary         string
		expectedContent string
	}{
		{
			name:            "with summary",
			content:         "Test content",
			summary:         "Test summary",
			expectedContent: "Test summary",
		},
		{
			name:            "without summary",
			content:         "Test content", 
			summary:         "",
			expectedContent: "Test content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := agent.buildUserPrompt(tt.content, tt.summary)
			assert.Contains(t, prompt, "extract the key takeaways")
			assert.Contains(t, prompt, tt.expectedContent)
		})
	}
}