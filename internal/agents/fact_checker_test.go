package agents

import (
	"context"
	"errors"
	"testing"

	"podcast-analyzer/internal/clients"
	"podcast-analyzer/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockSerperClient for testing
type MockSerperClient struct {
	mock.Mock
}

func (m *MockSerperClient) SearchForClaim(ctx context.Context, agentName, claim string) (*clients.SearchContext, error) {
	args := m.Called(ctx, agentName, claim)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*clients.SearchContext), args.Error(1)
}

func (m *MockSerperClient) FormatSearchResultsForAnalysis(context *clients.SearchContext) string {
	args := m.Called(context)
	return args.String(0)
}

func TestNewFactCheckerAgent(t *testing.T) {
	cfg := &config.Config{
		AnthropicAPIKey: "test-key",
		SerperAPIKey:    "test-serper-key",
	}

	agent := NewFactCheckerAgent(cfg)

	assert.NotNil(t, agent)
	assert.Equal(t, "fact_checker", agent.Name())
	assert.NotNil(t, agent.anthropicClient)
	assert.NotNil(t, agent.serperClient)
}

func TestFactCheckerAgent_Process_Success(t *testing.T) {
	mockAnthropicClient := &MockAnthropicClient{}
	mockSerperClient := &MockSerperClient{}
	agent := &FactCheckerAgent{
		BaseAgent:       NewBaseAgent("fact_checker"),
		anthropicClient: mockAnthropicClient,
		serperClient:    mockSerperClient,
	}

	ctx := context.Background()
	content := "The podcast mentioned that the moon landing happened in 1969."

	// Mock claim extraction
	claimsResponse := "1. The moon landing happened in 1969"
	mockAnthropicClient.On("CallClaude", 
		mock.Anything, 
		"fact_checker", 
		mock.AnythingOfType("string"), 
		mock.AnythingOfType("string"), 
		false,
	).Return(claimsResponse, nil).Once()

	// Mock search
	searchContext := &clients.SearchContext{
		Sources: []string{"https://nasa.gov/moon-landing"},
		Snippets: []clients.SearchSnippet{
			{
				Title:   "NASA Moon Landing",
				Snippet: "NASA moon landing information",
				URL:     "https://nasa.gov/moon-landing",
			},
		},
	}
	mockSerperClient.On("SearchForClaim", 
		mock.Anything, 
		"fact_checker", 
		"The moon landing happened in 1969",
	).Return(searchContext, nil)

	mockSerperClient.On("FormatSearchResultsForAnalysis",
		searchContext,
	).Return("Result 1:\nTitle: NASA Moon Landing\nSnippet: NASA moon landing information\nSource: https://nasa.gov/moon-landing")

	// Mock verification
	verificationResponse := "VERDICT: true\nCONFIDENCE: 0.95\nEVIDENCE: Historical records confirm this\nSOURCES: https://nasa.gov/moon-landing"
	mockAnthropicClient.On("CallClaude", 
		mock.Anything, 
		"fact_checker", 
		mock.AnythingOfType("string"), 
		mock.AnythingOfType("string"), 
		false,
	).Return(verificationResponse, nil).Once()

	result, err := agent.Process(ctx, content)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.FactChecks, 1)
	assert.Equal(t, "true", result.FactChecks[0].Verdict)
	assert.Equal(t, 0.95, result.FactChecks[0].Confidence)
	mockAnthropicClient.AssertExpectations(t)
	mockSerperClient.AssertExpectations(t)
}

func TestFactCheckerAgent_Process_NoClaims(t *testing.T) {
	mockAnthropicClient := &MockAnthropicClient{}
	mockSerperClient := &MockSerperClient{}
	agent := &FactCheckerAgent{
		BaseAgent:       NewBaseAgent("fact_checker"),
		anthropicClient: mockAnthropicClient,
		serperClient:    mockSerperClient,
	}

	ctx := context.Background()
	content := "This is just opinion content without factual claims."

	// Mock claim extraction returning empty response that won't be parsed as claims
	claimsResponse := "" // Empty response should result in no claims
	mockAnthropicClient.On("CallClaude", 
		mock.Anything, 
		mock.Anything, 
		mock.Anything, 
		mock.Anything, 
		false,
	).Return(claimsResponse, nil)

	result, err := agent.Process(ctx, content)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.FactChecks)
	mockAnthropicClient.AssertExpectations(t)
}

func TestFactCheckerAgent_extractClaims_Success(t *testing.T) {
	mockClient := &MockAnthropicClient{}
	agent := &FactCheckerAgent{
		BaseAgent:       NewBaseAgent("fact_checker"),
		anthropicClient: mockClient,
	}

	ctx := context.Background()
	content := "Test content"
	response := "1. First factual claim here\n2. Second factual claim here"

	mockClient.On("CallClaude", 
		mock.Anything, 
		mock.Anything, 
		mock.Anything, 
		mock.Anything, 
		false,
	).Return(response, nil)

	claims, err := agent.extractClaims(ctx, content)

	assert.NoError(t, err)
	assert.Len(t, claims, 2)
	assert.Equal(t, "First factual claim here", claims[0])
	assert.Equal(t, "Second factual claim here", claims[1])
	mockClient.AssertExpectations(t)
}


func TestFactCheckerAgent_parseClaims(t *testing.T) {
	agent := &FactCheckerAgent{
		BaseAgent: NewBaseAgent("fact_checker"),
	}

	tests := []struct {
		name     string
		response string
		expected []string
	}{
		{
			name:     "numbered claims",
			response: "1. First factual claim here\n2. Second factual claim here\n3. Third factual claim here",
			expected: []string{"First factual claim here", "Second factual claim here", "Third factual claim here"},
		},
		{
			name:     "bullet points",
			response: "• First factual claim statement\n• Second factual claim statement",
			expected: []string{"First factual claim statement", "Second factual claim statement"},
		},
		{
			name:     "mixed format with headers",
			response: "Factual claims:\n\n1. Climate change is definitely real\n2. The earth is definitely round\n\nEnd of claims.",
			expected: []string{"Climate change is definitely real", "The earth is definitely round"},
		},
		{
			name:     "no claims found",
			response: "No claims found.",
			expected: nil,
		},
		{
			name:     "empty response",
			response: "",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := agent.parseClaims(tt.response)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFactCheckerAgent_verifyClaim_Success(t *testing.T) {
	mockSerperClient := &MockSerperClient{}
	mockAnthropicClient := &MockAnthropicClient{}
	agent := &FactCheckerAgent{
		BaseAgent:       NewBaseAgent("fact_checker"),
		serperClient:    mockSerperClient,
		anthropicClient: mockAnthropicClient,
	}

	ctx := context.Background()
	claim := "The earth is round"

	// Mock search
	searchContext := &clients.SearchContext{
		Sources: []string{"https://nasa.gov/earth-shape"},
		Snippets: []clients.SearchSnippet{
			{
				Title:   "Earth Shape Evidence",
				Snippet: "Scientific evidence about earth's shape",
				URL:     "https://nasa.gov/earth-shape",
			},
		},
	}
	mockSerperClient.On("SearchForClaim", 
		mock.Anything, 
		"fact_checker", 
		claim,
	).Return(searchContext, nil)

	mockSerperClient.On("FormatSearchResultsForAnalysis",
		searchContext,
	).Return("Result 1:\nTitle: NASA Earth Shape\nSnippet: Scientific evidence confirms Earth is round\nSource: https://nasa.gov/earth-shape")

	// Mock analysis
	verificationResponse := "VERDICT: true\nCONFIDENCE: 0.99\nEVIDENCE: Scientific consensus confirms SOURCES: https://nasa.gov/earth-shape"
	mockAnthropicClient.On("CallClaude", 
		mock.Anything, 
		"fact_checker", 
		mock.AnythingOfType("string"), 
		mock.AnythingOfType("string"), 
		false,
	).Return(verificationResponse, nil)

	factCheck, err := agent.verifyClaim(ctx, claim)

	assert.NoError(t, err)
	assert.Equal(t, claim, factCheck.Claim)
	assert.Equal(t, "true", factCheck.Verdict)
	assert.Equal(t, 0.99, factCheck.Confidence)
	assert.Contains(t, factCheck.Evidence, "Scientific consensus")
	mockSerperClient.AssertExpectations(t)
	mockAnthropicClient.AssertExpectations(t)
}

func TestFactCheckerAgent_verifyClaim_SearchError(t *testing.T) {
	mockSerperClient := &MockSerperClient{}
	agent := &FactCheckerAgent{
		BaseAgent:    NewBaseAgent("fact_checker"),
		serperClient: mockSerperClient,
	}

	ctx := context.Background()
	claim := "Test claim"
	expectedError := errors.New("search service unavailable")

	mockSerperClient.On("SearchForClaim", 
		mock.Anything, 
		mock.Anything, 
		claim,
	).Return(nil, expectedError)

	factCheck, err := agent.verifyClaim(ctx, claim)

	assert.Error(t, err)
	assert.Equal(t, FactCheck{}, factCheck)
	assert.Contains(t, err.Error(), "web search failed")
	mockSerperClient.AssertExpectations(t)
}

func TestFactCheckerAgent_extractVerdict(t *testing.T) {
	agent := &FactCheckerAgent{
		BaseAgent: NewBaseAgent("fact_checker"),
	}

	tests := []struct {
		name     string
		response string
		expected string
	}{
		{
			name:     "true verdict",
			response: "VERDICT: true\nOther content",
			expected: "true",
		},
		{
			name:     "false verdict",
			response: "VERDICT: false\nOther content",
			expected: "false",
		},
		{
			name:     "partially true verdict",
			response: "VERDICT: partially_true\nOther content",
			expected: "partially_true",
		},
		{
			name:     "unverifiable verdict",
			response: "VERDICT: unverifiable\nOther content",
			expected: "unverifiable",
		},
		{
			name:     "invalid verdict",
			response: "VERDICT: maybe\nOther content",
			expected: "unverifiable",
		},
		{
			name:     "no verdict found",
			response: "No verdict in this response",
			expected: "unverifiable",
		},
		{
			name:     "case insensitive",
			response: "verdict: TRUE\nOther content",
			expected: "true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := agent.extractVerdict(tt.response)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFactCheckerAgent_extractConfidence(t *testing.T) {
	agent := &FactCheckerAgent{
		BaseAgent: NewBaseAgent("fact_checker"),
	}

	tests := []struct {
		name     string
		response string
		expected float64
	}{
		{
			name:     "valid confidence",
			response: "CONFIDENCE: 0.85\nOther content",
			expected: 0.85,
		},
		{
			name:     "confidence at upper bound",
			response: "CONFIDENCE: 1.0\nOther content", 
			expected: 1.0,
		},
		{
			name:     "confidence at lower bound",
			response: "CONFIDENCE: 0.0\nOther content",
			expected: 0.0,
		},
		{
			name:     "confidence above upper bound",
			response: "CONFIDENCE: 1.5\nOther content",
			expected: 1.0,
		},
		{
			name:     "confidence below lower bound",
			response: "CONFIDENCE: -0.2\nOther content",
			expected: 0.5,
		},
		{
			name:     "invalid confidence",
			response: "CONFIDENCE: invalid\nOther content",
			expected: 0.5,
		},
		{
			name:     "no confidence found",
			response: "No confidence in response",
			expected: 0.5,
		},
		{
			name:     "case insensitive",
			response: "confidence: 0.75\nOther content",
			expected: 0.75,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := agent.extractConfidence(tt.response)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFactCheckerAgent_extractEvidence(t *testing.T) {
	agent := &FactCheckerAgent{
		BaseAgent: NewBaseAgent("fact_checker"),
	}

	tests := []struct {
		name     string
		response string
		expected string
	}{
		{
			name:     "evidence found",
			response: "EVIDENCE: Multiple scientific studies confirm this claim SOURCES: http://example.com",
			expected: "Multiple scientific studies confirm this claim",
		},
		{
			name:     "evidence with extra whitespace",
			response: "EVIDENCE:   Trimmed evidence text   SOURCES: sources",
			expected: "Trimmed evidence text",
		},
		{
			name:     "no evidence found",
			response: "VERDICT: true\nCONFIDENCE: 0.8",
			expected: "No evidence provided",
		},
		{
			name:     "case insensitive",
			response: "evidence: Case insensitive evidence SOURCES: sources",
			expected: "Case insensitive evidence",
		},
		{
			name:     "evidence without sources at end",
			response: "EVIDENCE: Evidence text only",
			expected: "Evidence text only",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := agent.extractEvidence(tt.response)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFactCheckerAgent_extractSources(t *testing.T) {
	agent := &FactCheckerAgent{
		BaseAgent: NewBaseAgent("fact_checker"),
	}

	availableSources := []string{
		"https://nasa.gov/article1",
		"https://wikipedia.org/page1", 
		"https://scientificjournal.com/study",
	}

	tests := []struct {
		name     string
		response string
		expected []string
	}{
		{
			name:     "valid sources found",
			response: "SOURCES: https://nasa.gov/article1, https://wikipedia.org/page1",
			expected: []string{"https://nasa.gov/article1", "https://wikipedia.org/page1"},
		},
		{
			name:     "single source",
			response: "SOURCES: https://nasa.gov/article1",
			expected: []string{"https://nasa.gov/article1"},
		},
		{
			name:     "invalid source not in available list",
			response: "SOURCES: https://fakesource.com/article",
			expected: availableSources[:2], // Fallback to first 2 available
		},
		{
			name:     "no sources found",
			response: "VERDICT: true\nCONFIDENCE: 0.8",
			expected: availableSources[:2], // Fallback to first 2 available
		},
		{
			name:     "empty sources",
			response: "SOURCES: []",
			expected: availableSources[:2], // Fallback to first 2 available
		},
		{
			name:     "mixed valid and invalid sources",
			response: "SOURCES: https://nasa.gov/article1, https://fakesource.com/bad, https://wikipedia.org/page1",
			expected: []string{"https://nasa.gov/article1", "https://wikipedia.org/page1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := agent.extractSources(tt.response, availableSources)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFactCheckerAgent_parseVerificationResult(t *testing.T) {
	agent := &FactCheckerAgent{
		BaseAgent: NewBaseAgent("fact_checker"),
	}

	claim := "Test claim"
	response := "VERDICT: true\nCONFIDENCE: 0.85\nEVIDENCE: Strong evidence supports this SOURCES: https://nasa.gov/article1"
	availableSources := []string{"https://nasa.gov/article1", "https://other.com/page"}

	result := agent.parseVerificationResult(claim, response, availableSources)

	assert.Equal(t, claim, result.Claim)
	assert.Equal(t, "true", result.Verdict)
	assert.Equal(t, 0.85, result.Confidence)
	assert.Equal(t, "Strong evidence supports this", result.Evidence)
	assert.Equal(t, []string{"https://nasa.gov/article1"}, result.Sources)
}

func TestFactCheckerAgent_countVerdicts(t *testing.T) {
	agent := &FactCheckerAgent{
		BaseAgent: NewBaseAgent("fact_checker"),
	}

	factChecks := []FactCheck{
		{Verdict: "true"},
		{Verdict: "true"},
		{Verdict: "false"},
		{Verdict: "partially_true"},
		{Verdict: "unverifiable"},
		{Verdict: "true"},
	}

	result := agent.countVerdicts(factChecks)

	expected := map[string]int{
		"true":            3,
		"false":           1,
		"partially_true":  1,
		"unverifiable":    1,
	}

	assert.Equal(t, expected, result)
}