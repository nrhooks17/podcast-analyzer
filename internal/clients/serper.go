package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	
	"podcast-analyzer/internal/config"
	"podcast-analyzer/internal/logger"
	
	"github.com/sirupsen/logrus"
)

// SerperClientInterface defines the interface for Serper API client
type SerperClientInterface interface {
	SearchForClaim(ctx context.Context, agentName, claim string) (*SearchContext, error)
	FormatSearchResultsForAnalysis(context *SearchContext) string
}

// SerperClient handles communication with the Serper API for web search
type SerperClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	logger     *logrus.Logger
}

// SerperRequest represents a request to the Serper API
type SerperRequest struct {
	Query string `json:"q"`
	Num   int    `json:"num"`
}

// SerperResponse represents a response from the Serper API
type SerperResponse struct {
	Organic       []SerperResult    `json:"organic"`
	AnswerBox     *SerperAnswerBox  `json:"answerBox,omitempty"`
	KnowledgeGraph *SerperKnowledgeGraph `json:"knowledgeGraph,omitempty"`
}

// SerperResult represents a single search result
type SerperResult struct {
	Title   string `json:"title"`
	Link    string `json:"link"`
	Snippet string `json:"snippet"`
}

// SerperAnswerBox represents an answer box result
type SerperAnswerBox struct {
	Answer  string `json:"answer"`
	Title   string `json:"title"`
	Link    string `json:"link"`
	Snippet string `json:"snippet"`
}

// SerperKnowledgeGraph represents a knowledge graph result
type SerperKnowledgeGraph struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Website     string `json:"website"`
}

// SearchContext represents formatted search context for fact verification
type SearchContext struct {
	OriginalClaim string                 `json:"original_claim"`
	SearchQuery   string                 `json:"search_query"`
	Snippets      []SearchSnippet        `json:"snippets"`
	Sources       []string               `json:"sources"`
	TotalResults  int                    `json:"total_results"`
}

// SearchSnippet represents a formatted search result snippet
type SearchSnippet struct {
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
	URL     string `json:"url"`
}

// SerperError represents an error response from the Serper API
type SerperError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

func (e *SerperError) Error() string {
	return fmt.Sprintf("serper API error (%s): %s", e.Type, e.Message)
}

// NewSerperClient creates a new Serper API client
func NewSerperClient(cfg *config.Config) *SerperClient {
	return &SerperClient{
		apiKey:  cfg.SerperAPIKey,
		baseURL: "https://google.serper.dev/search",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger.Log,
	}
}

// Search performs a web search using Serper API
func (c *SerperClient) Search(ctx context.Context, agentName, query string, numResults int) (*SerperResponse, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("Serper API key not configured")
	}
	
	start := time.Now()
	correlationID := getCorrelationIDFromContext(ctx)
	
	c.logger.WithFields(map[string]interface{}{
		"agent":          agentName,
		"correlation_id": correlationID,
		"query":          query,
		"num_results":    numResults,
	}).Info("Performing Serper web search")
	
	// Prepare the request
	request := SerperRequest{
		Query: query,
		Num:   numResults,
	}
	
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	
	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	
	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-API-KEY", c.apiKey)
	
	// Make the request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()
	
	// Read the response
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	
	// Handle error responses
	if resp.StatusCode != http.StatusOK {
		var apiErr SerperError
		if json.Unmarshal(responseBody, &apiErr) == nil {
			return nil, fmt.Errorf("API error (status %d): %w", resp.StatusCode, &apiErr)
		}
		return nil, fmt.Errorf("unknown API error (status %d)", resp.StatusCode)
	}
	
	// Parse the successful response
	var serperResp SerperResponse
	if err := json.Unmarshal(responseBody, &serperResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	// Log successful response
	duration := time.Since(start)
	c.logger.WithFields(map[string]interface{}{
		"agent":          agentName,
		"correlation_id": correlationID,
		"duration_ms":    duration.Milliseconds(),
		"results_count":  len(serperResp.Organic),
		"has_answer_box": serperResp.AnswerBox != nil,
		"has_knowledge_graph": serperResp.KnowledgeGraph != nil,
	}).Info("Serper search completed")
	
	return &serperResp, nil
}

// SearchForClaim performs a targeted search for a specific factual claim
func (c *SerperClient) SearchForClaim(ctx context.Context, agentName, claim string) (*SearchContext, error) {
	// Optimize the claim for better search results
	searchQuery := c.optimizeClaimQuery(claim)
	
	// Perform the search
	searchResults, err := c.Search(ctx, agentName, searchQuery, 5)
	if err != nil {
		return nil, err
	}
	
	// Format the results for fact verification
	context := c.extractSearchContext(searchResults)
	context.OriginalClaim = claim
	context.SearchQuery = searchQuery
	
	return context, nil
}

// extractSearchContext extracts relevant context from Serper search results
func (c *SerperClient) extractSearchContext(results *SerperResponse) *SearchContext {
	context := &SearchContext{
		Snippets:     []SearchSnippet{},
		Sources:      []string{},
		TotalResults: len(results.Organic),
	}
	
	// Add answer box if available (highest priority)
	if results.AnswerBox != nil {
		snippet := results.AnswerBox.Snippet
		if snippet == "" {
			snippet = results.AnswerBox.Answer
		}
		
		if snippet != "" {
			context.Snippets = append(context.Snippets, SearchSnippet{
				Title:   results.AnswerBox.Title,
				Snippet: snippet,
				URL:     results.AnswerBox.Link,
			})
			
			if results.AnswerBox.Link != "" {
				context.Sources = append(context.Sources, results.AnswerBox.Link)
			}
		}
	}
	
	// Add knowledge graph if available
	if results.KnowledgeGraph != nil && results.KnowledgeGraph.Description != "" {
		title := fmt.Sprintf("Knowledge Graph: %s", results.KnowledgeGraph.Title)
		context.Snippets = append(context.Snippets, SearchSnippet{
			Title:   title,
			Snippet: results.KnowledgeGraph.Description,
			URL:     results.KnowledgeGraph.Website,
		})
		
		if results.KnowledgeGraph.Website != "" {
			context.Sources = append(context.Sources, results.KnowledgeGraph.Website)
		}
	}
	
	// Add organic search results
	for _, result := range results.Organic {
		if result.Snippet != "" {
			context.Snippets = append(context.Snippets, SearchSnippet{
				Title:   result.Title,
				Snippet: result.Snippet,
				URL:     result.Link,
			})
		}
		
		if result.Link != "" {
			context.Sources = append(context.Sources, result.Link)
		}
	}
	
	return context
}

// optimizeClaimQuery optimizes a factual claim for web search
func (c *SerperClient) optimizeClaimQuery(claim string) string {
	// Clean up the claim
	query := strings.TrimSpace(claim)
	
	// Remove quotation marks that might be too restrictive
	query = strings.ReplaceAll(query, "\"", "")
	
	// Limit query length for better results (Serper works better with shorter queries)
	words := strings.Fields(query)
	if len(words) > 10 {
		query = strings.Join(words[:10], " ")
	}
	
	return query
}

// FormatSearchResultsForAnalysis formats search results into readable text for Claude analysis
func (c *SerperClient) FormatSearchResultsForAnalysis(context *SearchContext) string {
	if len(context.Snippets) == 0 {
		return "No search results found."
	}
	
	var results []string
	
	// Limit to top 3 results to avoid overwhelming Claude
	maxResults := 3
	if len(context.Snippets) < maxResults {
		maxResults = len(context.Snippets)
	}
	
	for i, snippet := range context.Snippets[:maxResults] {
		result := fmt.Sprintf("Result %d:\nTitle: %s\nSnippet: %s", i+1, snippet.Title, snippet.Snippet)
		if snippet.URL != "" {
			result += fmt.Sprintf("\nSource: %s", snippet.URL)
		}
		results = append(results, result)
	}
	
	return strings.Join(results, "\n\n")
}