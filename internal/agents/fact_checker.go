package agents

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	
	"podcast-analyzer/internal/clients"
	"podcast-analyzer/internal/config"
)

// FactCheckerAgent extracts and verifies factual claims from podcast transcripts
type FactCheckerAgent struct {
	*BaseAgent
	anthropicClient clients.AnthropicClientInterface
	serperClient    clients.SerperClientInterface
}

// NewFactCheckerAgent creates a new fact checker agent
func NewFactCheckerAgent(cfg *config.Config) *FactCheckerAgent {
	return &FactCheckerAgent{
		BaseAgent:       NewBaseAgent("fact_checker"),
		anthropicClient: clients.NewAnthropicClient(cfg),
		serperClient:    clients.NewSerperClient(cfg),
	}
}

// Process extracts and verifies factual claims from the transcript
func (f *FactCheckerAgent) Process(ctx context.Context, content string) (Result, error) {
	start := time.Now()
	
	// Log start of processing
	f.LogStart(ctx, len(content))
	
	// Validate content
	if err := f.ValidateContent(content); err != nil {
		f.LogError(ctx, err, time.Since(start))
		return Result{}, err
	}
	
	// Step 1: Extract factual claims from transcript
	claims, err := f.extractClaims(ctx, content)
	if err != nil {
		f.LogError(ctx, err, time.Since(start))
		return Result{}, NewAgentError(f.Name(), "failed to extract claims", err)
	}
	
	if len(claims) == 0 {
		f.logger.WithFields(map[string]interface{}{
			"agent": f.Name(),
			"correlation_id": getCorrelationID(ctx),
		}).Info("No factual claims found in transcript")
		
		result := Result{FactChecks: []FactCheck{}}
		f.LogSuccess(ctx, &result, time.Since(start))
		return result, nil
	}
	
	f.logger.WithFields(map[string]interface{}{
		"agent":        f.Name(),
		"correlation_id": getCorrelationID(ctx),
		"claims_count": len(claims),
	}).Info("Extracted factual claims from transcript")
	
	// Step 2: Verify each claim with rate limiting
	factChecks := make([]FactCheck, 0, len(claims))
	
	for i, claim := range claims {
		correlationID := getCorrelationID(ctx)
		f.logger.WithFields(map[string]interface{}{
			"agent":          f.Name(),
			"correlation_id": correlationID,
			"claim_num":      i + 1,
			"total_claims":   len(claims),
			"claim":          f.TruncateForLog(claim, 100),
		}).Info("Checking claim")
		
		factCheck, err := f.verifyClaim(ctx, claim)
		if err != nil {
			f.logger.WithFields(map[string]interface{}{
				"agent":          f.Name(),
				"correlation_id": correlationID,
				"claim_num":      i + 1,
				"claim":          claim,
				"error":          err.Error(),
			}).Error("Failed to verify claim, marking as unverifiable")
			
			// Continue with other claims instead of failing completely
			factCheck = FactCheck{
				Claim:      claim,
				Verdict:    "unverifiable",
				Confidence: 0.0,
				Evidence:   fmt.Sprintf("Verification failed: %s", err.Error()),
				Sources:    []string{},
			}
		}
		
		factChecks = append(factChecks, factCheck)
		
		// Log claim result
		f.logger.WithFields(map[string]interface{}{
			"agent":          f.Name(),
			"correlation_id": correlationID,
			"claim_num":      i + 1,
			"verdict":        factCheck.Verdict,
			"confidence":     factCheck.Confidence,
			"evidence":       f.TruncateForLog(factCheck.Evidence, 100),
		}).Info("Claim verification result")
		
		// Add delay between claims to avoid hitting rate limits
		if i < len(claims)-1 { // Don't delay after the last claim
			select {
			case <-time.After(3 * time.Second):
				// Continue to next claim
			case <-ctx.Done():
				return Result{}, ctx.Err()
			}
		}
	}
	
	// Log summary
	verdictCounts := f.countVerdicts(factChecks)
	f.logger.WithFields(map[string]interface{}{
		"agent":                        f.Name(),
		"correlation_id":               getCorrelationID(ctx),
		"total_claims":                 len(factChecks),
		"claims_true":                  verdictCounts["true"],
		"claims_false":                 verdictCounts["false"],
		"claims_partially_true":        verdictCounts["partially_true"],
		"claims_unverifiable":          verdictCounts["unverifiable"],
	}).Info("Fact checking completed")
	
	result := Result{FactChecks: factChecks}
	f.LogSuccess(ctx, &result, time.Since(start))
	
	return result, nil
}

// extractClaims extracts factual claims from the transcript that can be verified
func (f *FactCheckerAgent) extractClaims(ctx context.Context, content string) ([]string, error) {
	// Truncate very long transcripts
	maxTranscriptLength := 10000
	if len(content) > maxTranscriptLength {
		content = f.TruncateContent(content, maxTranscriptLength)
	}
	
	systemPrompt := `You are an expert at identifying specific, verifiable factual claims in text. Focus on concrete statements that make specific assertions about real-world facts, events, dates, numbers, or entities that can be checked against reliable sources.`
	
	userPrompt := fmt.Sprintf(`Analyze the following podcast transcript and extract factual claims that can be verified.

Look for statements that:
- Make specific factual assertions about events, dates, numbers, or statistics
- Reference real people, companies, organizations, or places
- Mention scientific findings, research results, or studies
- Claim specific achievements, milestones, or historical events
- Make predictions with specific timelines or targets

Ignore:
- Opinions, beliefs, or personal views
- General statements without specific details
- Hypothetical scenarios
- Common knowledge facts
- Vague or ambiguous statements

TRANSCRIPT:
%s

Extract 2-3 specific factual claims that can be verified. Format as a simple numbered list:

1. [First specific factual claim]
2. [Second specific factual claim]
etc.

FACTUAL CLAIMS:`, content)
	
	f.LogAPICall(ctx, "anthropic", len(userPrompt), true)
	
	response, err := f.anthropicClient.CallClaude(ctx, f.Name(), userPrompt, systemPrompt, false)
	if err != nil {
		return nil, err
	}
	
	claims := f.parseClaims(response)
	return claims, nil
}

// parseClaims parses claims from Claude's response
func (f *FactCheckerAgent) parseClaims(rawResponse string) []string {
	var claims []string
	lines := strings.Split(strings.TrimSpace(rawResponse), "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		// Remove list markers using regex
		patterns := []*regexp.Regexp{
			regexp.MustCompile(`^\d+\.\s*`),     // 1. 
			regexp.MustCompile(`^\d+\)\s*`),     // 1) 
			regexp.MustCompile(`^-\s*`),         // - 
			regexp.MustCompile(`^•\s*`),         // • 
			regexp.MustCompile(`^\*\s*`),        // * 
		}
		
		cleanedLine := line
		for _, pattern := range patterns {
			cleanedLine = pattern.ReplaceAllString(cleanedLine, "")
		}
		
		// Skip if too short
		if len(strings.Fields(cleanedLine)) < 4 {
			continue
		}
		
		claims = append(claims, cleanedLine)
	}
	
	// Limit to 3 claims to reduce token usage and processing time
	if len(claims) > 3 {
		claims = claims[:3]
	}
	
	return claims
}

// verifyClaim verifies a single factual claim using Serper web search and Claude analysis
func (f *FactCheckerAgent) verifyClaim(ctx context.Context, claim string) (FactCheck, error) {
	// Step 1: Use Serper to search for the claim
	f.LogAPICall(ctx, "serper", len(claim), false)
	searchContext, err := f.serperClient.SearchForClaim(ctx, f.Name(), claim)
	if err != nil {
		return FactCheck{}, NewAgentError(f.Name(), "web search failed", err)
	}
	
	if len(searchContext.Snippets) == 0 {
		f.logger.WithFields(map[string]interface{}{
			"agent": f.Name(),
			"correlation_id": getCorrelationID(ctx),
			"claim": claim,
		}).Warn("No search results found for claim")
		
		return FactCheck{
			Claim:      claim,
			Verdict:    "unverifiable",
			Confidence: 0.0,
			Evidence:   "No search results found",
			Sources:    []string{},
		}, nil
	}
	
	// Step 2: Use Claude to analyze the search results
	f.LogAPICall(ctx, "anthropic", len(claim), true)
	analysisResult, err := f.analyzeSearchResults(ctx, claim, searchContext)
	if err != nil {
		return FactCheck{}, NewAgentError(f.Name(), "analysis failed", err)
	}
	
	return analysisResult, nil
}

// analyzeSearchResults uses Claude to analyze search results and determine claim validity
func (f *FactCheckerAgent) analyzeSearchResults(ctx context.Context, claim string, searchContext *clients.SearchContext) (FactCheck, error) {
	// Format search results for Claude
	formattedResults := f.serperClient.FormatSearchResultsForAnalysis(searchContext)
	
	systemPrompt := `You are a professional fact-checker analyzing web search results. Evaluate claims objectively based on source quality and evidence strength. Be precise and concise in your assessment.`
	
	userPrompt := fmt.Sprintf(`Analyze the following search results to verify this claim:

CLAIM: %s

SEARCH RESULTS:
%s

Based on these search results, provide your assessment:

VERDICT: [true/false/partially_true/unverifiable]
CONFIDENCE: [0.0-1.0]
EVIDENCE: [Brief explanation in 1-2 sentences max]
SOURCES: [List the most relevant source URLs from the search results]

Guidelines:
- true: Claim is fully supported by reliable sources
- false: Claim is contradicted by reliable sources  
- partially_true: Claim has some truth but lacks important context/nuance
- unverifiable: Insufficient or unreliable sources to make determination

Be concise and focus on the most relevant evidence.`, claim, formattedResults)
	
	response, err := f.anthropicClient.CallClaude(ctx, f.Name(), userPrompt, systemPrompt, false)
	if err != nil {
		return FactCheck{}, err
	}
	
	return f.parseVerificationResult(claim, response, searchContext.Sources), nil
}

// parseVerificationResult parses the verification result from Claude's response
func (f *FactCheckerAgent) parseVerificationResult(claim, response string, availableSources []string) FactCheck {
	verdict := f.extractVerdict(response)
	confidence := f.extractConfidence(response)
	evidence := f.extractEvidence(response)
	sources := f.extractSources(response, availableSources)
	
	return FactCheck{
		Claim:      claim,
		Verdict:    verdict,
		Confidence: confidence,
		Evidence:   evidence,
		Sources:    sources,
	}
}

// extractVerdict parses and validates the verdict from the response
func (f *FactCheckerAgent) extractVerdict(response string) string {
	verdictRegex := regexp.MustCompile(`(?i)VERDICT:\s*(\w+)`)
	verdictMatch := verdictRegex.FindStringSubmatch(response)
	verdict := "unverifiable"
	if len(verdictMatch) > 1 {
		verdict = strings.ToLower(verdictMatch[1])
	}
	
	// Ensure valid verdict
	validVerdicts := map[string]bool{
		"true": true, "false": true, "partially_true": true, "unverifiable": true,
	}
	if !validVerdicts[verdict] {
		verdict = "unverifiable"
	}
	
	return verdict
}

// extractConfidence parses and validates the confidence value from the response
func (f *FactCheckerAgent) extractConfidence(response string) float64 {
	confidenceRegex := regexp.MustCompile(`(?i)CONFIDENCE:\s*([\d.]+)`)
	confidenceMatch := confidenceRegex.FindStringSubmatch(response)
	confidence := 0.5 // default
	if len(confidenceMatch) > 1 {
		if parsed, err := strconv.ParseFloat(confidenceMatch[1], 64); err == nil {
			confidence = parsed
			// Clamp to valid range
			if confidence < 0.0 {
				confidence = 0.0
			} else if confidence > 1.0 {
				confidence = 1.0
			}
		}
	}
	return confidence
}

// extractEvidence parses the evidence text from the response
func (f *FactCheckerAgent) extractEvidence(response string) string {
	evidenceRegex := regexp.MustCompile(`(?i)EVIDENCE:\s*(.+?)SOURCES:`)
	evidenceMatch := evidenceRegex.FindStringSubmatch(response)
	evidence := "No evidence provided"
	if len(evidenceMatch) > 1 {
		evidence = strings.TrimSpace(evidenceMatch[1])
	} else {
		// Try without SOURCES: at the end
		evidenceRegex := regexp.MustCompile(`(?i)EVIDENCE:\s*(.+)$`)
		evidenceMatch := evidenceRegex.FindStringSubmatch(response)
		if len(evidenceMatch) > 1 {
			evidence = strings.TrimSpace(evidenceMatch[1])
		}
	}
	return evidence
}

// extractSources parses and validates source URLs from the response
func (f *FactCheckerAgent) extractSources(response string, availableSources []string) []string {
	sourcesRegex := regexp.MustCompile(`(?i)SOURCES:\s*(.+?)$`)
	sourcesMatch := sourcesRegex.FindStringSubmatch(response)
	var sources []string
	
	if len(sourcesMatch) > 1 {
		sourcesText := strings.TrimSpace(sourcesMatch[1])
		if sourcesText != "" && sourcesText != "[]" {
			// Extract URLs using regex
			urlRegex := regexp.MustCompile(`https?://[^\s\],]+`)
			foundURLs := urlRegex.FindAllString(sourcesText, -1)
			
			// Validate against available sources
			for _, url := range foundURLs {
				for _, availableURL := range availableSources {
					if url == availableURL {
						sources = append(sources, url)
						break
					}
				}
			}
		}
	}
	
	// If no sources found but we have available sources, use first 2 as fallback
	if len(sources) == 0 && len(availableSources) > 0 {
		maxSources := 2
		if len(availableSources) < maxSources {
			maxSources = len(availableSources)
		}
		sources = availableSources[:maxSources]
	}
	
	return sources
}

// countVerdicts counts the number of each verdict type
func (f *FactCheckerAgent) countVerdicts(factChecks []FactCheck) map[string]int {
	counts := map[string]int{
		"true":            0,
		"false":           0,
		"partially_true":  0,
		"unverifiable":    0,
	}
	
	for _, fc := range factChecks {
		if _, exists := counts[fc.Verdict]; exists {
			counts[fc.Verdict]++
		}
	}
	
	return counts
}