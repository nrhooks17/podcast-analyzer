package clients

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"podcast-analyzer/internal/config"

	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
)

func setupTestSerperClient() (*SerperClient, *test.Hook) {
	cfg := &config.Config{
		SerperAPIKey: "test-serper-key",
	}
	
	logger, hook := test.NewNullLogger()
	client := NewSerperClient(cfg)
	client.logger = logger
	
	return client, hook
}

func TestNewSerperClient(t *testing.T) {
	cfg := &config.Config{
		SerperAPIKey: "test-serper-key",
	}

	client := NewSerperClient(cfg)

	assert.NotNil(t, client)
	assert.Equal(t, "test-serper-key", client.apiKey)
	assert.Equal(t, "https://google.serper.dev/search", client.baseURL)
	assert.NotNil(t, client.httpClient)
}

func TestSerperError_Error(t *testing.T) {
	err := &SerperError{
		Type:    "rate_limit_exceeded",
		Message: "API rate limit exceeded",
	}

	result := err.Error()
	expected := "serper API error (rate_limit_exceeded): API rate limit exceeded"
	assert.Equal(t, expected, result)
}

func TestSerperClient_Search_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/search", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "test-serper-key", r.Header.Get("X-API-KEY"))

		// Verify request body
		body, _ := io.ReadAll(r.Body)
		var request SerperRequest
		json.Unmarshal(body, &request)
		assert.Equal(t, "test query", request.Query)
		assert.Equal(t, 5, request.Num)

		// Return successful response
		response := SerperResponse{
			Organic: []SerperResult{
				{
					Title:   "Test Result 1",
					Link:    "https://example.com/1",
					Snippet: "This is the first test result snippet",
				},
				{
					Title:   "Test Result 2",
					Link:    "https://example.com/2",
					Snippet: "This is the second test result snippet",
				},
			},
			AnswerBox: &SerperAnswerBox{
				Answer:  "Test answer from answer box",
				Title:   "Answer Box Title",
				Link:    "https://example.com/answer",
				Snippet: "Answer box snippet",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client, _ := setupTestSerperClient()
	client.baseURL = server.URL + "/search"

	ctx := context.WithValue(context.Background(), "correlation_id", "test-correlation-123")
	result, err := client.Search(ctx, "test-agent", "test query", 5)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Organic, 2)
	assert.Equal(t, "Test Result 1", result.Organic[0].Title)
	assert.Equal(t, "https://example.com/1", result.Organic[0].Link)
	assert.NotNil(t, result.AnswerBox)
	assert.Equal(t, "Test answer from answer box", result.AnswerBox.Answer)
}

func TestSerperClient_Search_NoAPIKey(t *testing.T) {
	client := &SerperClient{
		apiKey: "",
	}

	ctx := context.Background()
	result, err := client.Search(ctx, "test-agent", "test query", 5)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "Serper API key not configured")
}

func TestSerperClient_Search_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		apiErr := SerperError{
			Type:    "invalid_request",
			Message: "Invalid search query",
		}
		json.NewEncoder(w).Encode(apiErr)
	}))
	defer server.Close()

	client, _ := setupTestSerperClient()
	client.baseURL = server.URL + "/search"

	ctx := context.Background()
	result, err := client.Search(ctx, "test-agent", "test query", 5)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "API error (status 400)")
	assert.Contains(t, err.Error(), "Invalid search query")
}

func TestSerperClient_Search_UnknownAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("invalid json response"))
	}))
	defer server.Close()

	client, _ := setupTestSerperClient()
	client.baseURL = server.URL + "/search"

	ctx := context.Background()
	result, err := client.Search(ctx, "test-agent", "test query", 5)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "unknown API error (status 500)")
}

func TestSerperClient_Search_InvalidResponseJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	client, _ := setupTestSerperClient()
	client.baseURL = server.URL + "/search"

	ctx := context.Background()
	result, err := client.Search(ctx, "test-agent", "test query", 5)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to parse response")
}

func TestSerperClient_SearchForClaim_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := SerperResponse{
			Organic: []SerperResult{
				{
					Title:   "Moon Landing Facts",
					Link:    "https://nasa.gov/moon-landing",
					Snippet: "The Apollo 11 mission landed on the moon on July 20, 1969",
				},
			},
			AnswerBox: &SerperAnswerBox{
				Answer:  "July 20, 1969",
				Title:   "When did humans first land on the moon?",
				Link:    "https://nasa.gov/apollo11",
				Snippet: "Neil Armstrong and Buzz Aldrin became the first humans to land on the moon",
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client, _ := setupTestSerperClient()
	client.baseURL = server.URL + "/search"

	ctx := context.Background()
	claim := "The moon landing happened in 1969"
	result, err := client.SearchForClaim(ctx, "test-agent", claim)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, claim, result.OriginalClaim)
	assert.Equal(t, "The moon landing happened in 1969", result.SearchQuery)
	assert.Len(t, result.Snippets, 2) // Answer box + organic result
	assert.Equal(t, "When did humans first land on the moon?", result.Snippets[0].Title)
	assert.Contains(t, result.Sources, "https://nasa.gov/apollo11")
	assert.Contains(t, result.Sources, "https://nasa.gov/moon-landing")
}

func TestSerperClient_extractSearchContext(t *testing.T) {
	client, _ := setupTestSerperClient()

	tests := []struct {
		name           string
		response       *SerperResponse
		expectedSnippets int
		expectedSources  int
	}{
		{
			name: "with answer box and organic results",
			response: &SerperResponse{
				Organic: []SerperResult{
					{
						Title:   "Organic Result",
						Link:    "https://example.com",
						Snippet: "Organic snippet",
					},
				},
				AnswerBox: &SerperAnswerBox{
					Answer:  "Answer box response",
					Title:   "Answer Title",
					Link:    "https://answerbox.com",
					Snippet: "Answer snippet",
				},
			},
			expectedSnippets: 2,
			expectedSources:  2,
		},
		{
			name: "with knowledge graph",
			response: &SerperResponse{
				Organic: []SerperResult{
					{
						Title:   "Organic Result",
						Link:    "https://example.com",
						Snippet: "Organic snippet",
					},
				},
				KnowledgeGraph: &SerperKnowledgeGraph{
					Title:       "Knowledge Title",
					Description: "Knowledge description",
					Website:     "https://knowledge.com",
				},
			},
			expectedSnippets: 2,
			expectedSources:  2,
		},
		{
			name: "organic results only",
			response: &SerperResponse{
				Organic: []SerperResult{
					{
						Title:   "Result 1",
						Link:    "https://example1.com",
						Snippet: "Snippet 1",
					},
					{
						Title:   "Result 2",
						Link:    "https://example2.com",
						Snippet: "Snippet 2",
					},
				},
			},
			expectedSnippets: 2,
			expectedSources:  2,
		},
		{
			name: "empty response",
			response: &SerperResponse{
				Organic: []SerperResult{},
			},
			expectedSnippets: 0,
			expectedSources:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.extractSearchContext(tt.response)

			assert.NotNil(t, result)
			assert.Len(t, result.Snippets, tt.expectedSnippets)
			assert.Len(t, result.Sources, tt.expectedSources)
			assert.Equal(t, len(tt.response.Organic), result.TotalResults)
		})
	}
}

func TestSerperClient_extractSearchContext_AnswerBoxWithoutSnippet(t *testing.T) {
	client, _ := setupTestSerperClient()

	response := &SerperResponse{
		AnswerBox: &SerperAnswerBox{
			Answer: "Direct answer without snippet",
			Title:  "Answer Title",
			Link:   "https://answerbox.com",
		},
	}

	result := client.extractSearchContext(response)

	assert.Len(t, result.Snippets, 1)
	assert.Equal(t, "Direct answer without snippet", result.Snippets[0].Snippet)
}

func TestSerperClient_optimizeClaimQuery(t *testing.T) {
	client, _ := setupTestSerperClient()

	tests := []struct {
		name     string
		claim    string
		expected string
	}{
		{
			name:     "basic claim",
			claim:    "The earth is round",
			expected: "The earth is round",
		},
		{
			name:     "claim with quotes",
			claim:    `"The moon landing" happened in 1969`,
			expected: "The moon landing happened in 1969",
		},
		{
			name:     "claim with extra whitespace",
			claim:    "  The earth is round  ",
			expected: "The earth is round",
		},
		{
			name:     "long claim gets truncated",
			claim:    "This is a very long claim with many words that should be truncated to ten words maximum for better search results",
			expected: "This is a very long claim with many words that",
		},
		{
			name:     "exactly ten words",
			claim:    "This claim has exactly ten words for the search query",
			expected: "This claim has exactly ten words for the search query",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.optimizeClaimQuery(tt.claim)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSerperClient_FormatSearchResultsForAnalysis(t *testing.T) {
	client, _ := setupTestSerperClient()

	tests := []struct {
		name        string
		context     *SearchContext
		expectedLen int
		contains    []string
	}{
		{
			name: "multiple results",
			context: &SearchContext{
				Snippets: []SearchSnippet{
					{
						Title:   "Result 1",
						Snippet: "First snippet",
						URL:     "https://example1.com",
					},
					{
						Title:   "Result 2",
						Snippet: "Second snippet",
						URL:     "https://example2.com",
					},
					{
						Title:   "Result 3",
						Snippet: "Third snippet",
						URL:     "https://example3.com",
					},
					{
						Title:   "Result 4",
						Snippet: "Fourth snippet",
						URL:     "https://example4.com",
					},
				},
			},
			contains: []string{"Result 1:", "Result 2:", "Result 3:", "First snippet", "Second snippet", "Third snippet"},
		},
		{
			name: "single result",
			context: &SearchContext{
				Snippets: []SearchSnippet{
					{
						Title:   "Single Result",
						Snippet: "Only snippet",
						URL:     "https://example.com",
					},
				},
			},
			contains: []string{"Result 1:", "Single Result", "Only snippet", "https://example.com"},
		},
		{
			name:     "empty results",
			context:  &SearchContext{Snippets: []SearchSnippet{}},
			contains: []string{"No search results found."},
		},
		{
			name: "result without URL",
			context: &SearchContext{
				Snippets: []SearchSnippet{
					{
						Title:   "Result Without URL",
						Snippet: "Snippet without URL",
						URL:     "",
					},
				},
			},
			contains: []string{"Result 1:", "Result Without URL", "Snippet without URL"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.FormatSearchResultsForAnalysis(tt.context)

			assert.NotEmpty(t, result)
			for _, expectedContent := range tt.contains {
				assert.Contains(t, result, expectedContent)
			}

			// Verify that we don't exceed 3 results
			resultCount := strings.Count(result, "Result ")
			if len(tt.context.Snippets) > 3 {
				assert.LessOrEqual(t, resultCount, 6) // Max 3 results * 2 occurrences each (title + format)
			} else {
				assert.LessOrEqual(t, resultCount, len(tt.context.Snippets)*2) // Account for titles containing "Result "
			}
		})
	}
}

func TestSerperClient_FormatSearchResultsForAnalysis_LimitsToThreeResults(t *testing.T) {
	client, _ := setupTestSerperClient()

	// Create 5 snippets
	context := &SearchContext{
		Snippets: []SearchSnippet{
			{Title: "Result 1", Snippet: "Snippet 1", URL: "https://example1.com"},
			{Title: "Result 2", Snippet: "Snippet 2", URL: "https://example2.com"},
			{Title: "Result 3", Snippet: "Snippet 3", URL: "https://example3.com"},
			{Title: "Result 4", Snippet: "Snippet 4", URL: "https://example4.com"},
			{Title: "Result 5", Snippet: "Snippet 5", URL: "https://example5.com"},
		},
	}

	result := client.FormatSearchResultsForAnalysis(context)

	// Should only contain first 3 results
	assert.Contains(t, result, "Result 1:")
	assert.Contains(t, result, "Result 2:")
	assert.Contains(t, result, "Result 3:")
	assert.NotContains(t, result, "Result 4:")
	assert.NotContains(t, result, "Result 5:")

	// Count occurrences to verify exactly 3 results (6 total "Result " strings due to titles)
	resultCount := strings.Count(result, "Result ")
	assert.Equal(t, 6, resultCount) // 3 results * 2 occurrences each
}