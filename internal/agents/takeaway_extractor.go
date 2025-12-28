package agents

import (
	"context"
	"regexp"
	"strings"
	"time"
	
	"podcast-analyzer/internal/clients"
	"podcast-analyzer/internal/config"
)

// TakeawayExtractorAgent extracts key takeaways and insights from podcast transcripts
type TakeawayExtractorAgent struct {
	*BaseAgent
	anthropicClient clients.AnthropicClientInterface
}

// NewTakeawayExtractorAgent creates a new takeaway extractor agent
func NewTakeawayExtractorAgent(cfg *config.Config) *TakeawayExtractorAgent {
	return &TakeawayExtractorAgent{
		BaseAgent:       NewBaseAgent("takeaway_extractor"),
		anthropicClient: clients.NewAnthropicClient(cfg),
	}
}

// Process extracts key takeaways from the podcast transcript
func (t *TakeawayExtractorAgent) Process(ctx context.Context, content string) (Result, error) {
	return t.ProcessWithOptions(ctx, content, ProcessingOptions{})
}

// ProcessWithOptions extracts key takeaways with optional summary context
func (t *TakeawayExtractorAgent) ProcessWithOptions(ctx context.Context, content string, opts ProcessingOptions) (Result, error) {
	start := time.Now()
	defer func() {
		t.LogAPICall(ctx, "anthropic", len(content), true)
	}()
	
	// Log start of processing
	t.LogStart(ctx, len(content))
	
	// Validate content
	if err := t.ValidateContent(content); err != nil {
		t.LogError(ctx, err, time.Since(start))
		return Result{}, err
	}
	
	// Build prompts
	systemPrompt := t.buildSystemPrompt()
	userPrompt := t.buildUserPrompt(content, opts.Summary)
	
	// Call Claude API
	rawResponse, err := t.anthropicClient.CallClaude(ctx, t.Name(), userPrompt, systemPrompt, false)
	if err != nil {
		t.LogError(ctx, err, time.Since(start))
		return Result{}, NewAgentError(t.Name(), "failed to extract takeaways", err)
	}
	
	// Parse and validate the takeaways
	takeaways := t.parseTakeaways(rawResponse)
	if len(takeaways) == 0 {
		err := NewAgentError(t.Name(), "no takeaways extracted from transcript", nil)
		t.LogError(ctx, err, time.Since(start))
		return Result{}, err
	}
	
	result := Result{Takeaways: takeaways}
	
	// Log success with takeaway details
	t.logTakeaways(ctx, takeaways)
	t.LogSuccess(ctx, &result, time.Since(start))
	
	return result, nil
}

// buildSystemPrompt creates the system prompt for Claude
func (t *TakeawayExtractorAgent) buildSystemPrompt() string {
	return `You are an expert at identifying key insights and actionable takeaways from podcast discussions.

Your task is to extract the most important, valuable, and memorable points that:
- Represent key insights or learnings shared during the discussion
- Are actionable or applicable to the audience
- Capture important facts, statistics, or expert opinions
- Highlight notable quotes or profound statements
- Include practical advice or recommendations mentioned
- Cover significant predictions or future outlook discussed

Focus on substantive content that would be valuable for someone to remember or act upon. Avoid:
- Basic introductory statements
- Small talk or casual conversation
- Obvious or common knowledge points
- Repetitive information

Return your response as a simple numbered list, with each takeaway as a complete, clear sentence.`
}

// buildUserPrompt creates the user prompt with transcript and optional summary
func (t *TakeawayExtractorAgent) buildUserPrompt(content, summary string) string {
	// Truncate very long transcripts for the prompt
	maxTranscriptLength := 12000 // Reasonable limit for Claude context
	if len(content) > maxTranscriptLength {
		content = t.TruncateContent(content, maxTranscriptLength)
	}
	
	prompt := `Analyze the following podcast transcript and extract the key takeaways and insights.

Focus on identifying:
- Important facts, statistics, or expert insights
- Actionable advice or recommendations
- Significant predictions or future outlook
- Notable quotes or profound statements
- Key lessons learned or wisdom shared
- Practical tips mentioned

`
	
	// Add summary context if available
	if summary != "" {
		prompt += "CONTEXT SUMMARY:\n" + summary + "\n\n"
	}
	
	prompt += "TRANSCRIPT:\n" + content + "\n\n"
	prompt += `Please extract 4-8 key takeaways from this podcast. Format your response as a simple numbered list:

1. [First key takeaway]
2. [Second key takeaway]
3. [Third key takeaway]
etc.

KEY TAKEAWAYS:`
	
	return prompt
}

// parseTakeaways parses takeaways from Claude's response
func (t *TakeawayExtractorAgent) parseTakeaways(rawResponse string) []string {
	var takeaways []string
	
	// Split response into lines
	lines := strings.Split(strings.TrimSpace(rawResponse), "\n")
	
	for _, line := range lines {
		processedLine := t.processTakeawayLine(line)
		if processedLine != "" {
			takeaways = append(takeaways, processedLine)
		}
	}
	
	// Limit to reasonable number of takeaways
	if len(takeaways) > 10 {
		t.logger.WithFields(map[string]interface{}{
			"agent":            t.Name(),
			"original_count":   len(takeaways),
			"truncated_count":  10,
		}).Warn("Truncated takeaways list to maximum count")
		takeaways = takeaways[:10]
	}
	
	return takeaways
}

// removeListMarkers removes numbered and bulleted list markers from a line
func (t *TakeawayExtractorAgent) removeListMarkers(line string) string {
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
	
	return cleanedLine
}

// shouldSkipLine determines if a line should be filtered out as a non-takeaway
func (t *TakeawayExtractorAgent) shouldSkipLine(line string) bool {
	// Skip if line is too short
	words := strings.Fields(line)
	if len(words) < 3 {
		return true
	}
	
	// Skip common non-takeaway phrases
	skipPhrases := []string{
		"key takeaways",
		"takeaways:",
		"summary:",
		"in conclusion",
		"to summarize",
	}
	
	lowerLine := strings.ToLower(line)
	for _, phrase := range skipPhrases {
		if strings.Contains(lowerLine, phrase) {
			return true
		}
	}
	
	return false
}

// processTakeawayLine applies list marker removal, validation, and cleaning to a line
func (t *TakeawayExtractorAgent) processTakeawayLine(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}
	
	// Remove list markers
	cleanedLine := t.removeListMarkers(line)
	
	// Check if line should be skipped
	if t.shouldSkipLine(cleanedLine) {
		return ""
	}
	
	// Clean up the takeaway
	cleanedLine = t.cleanTakeaway(cleanedLine)
	
	return cleanedLine
}

// cleanTakeaway cleans and formats a single takeaway
func (t *TakeawayExtractorAgent) cleanTakeaway(takeaway string) string {
	// Trim whitespace
	cleaned := strings.TrimSpace(takeaway)
	
	// Ensure it ends with proper punctuation
	if len(cleaned) > 0 && !strings.HasSuffix(cleaned, ".") && !strings.HasSuffix(cleaned, "!") && !strings.HasSuffix(cleaned, "?") {
		cleaned += "."
	}
	
	// Ensure it starts with capital letter
	if len(cleaned) > 0 && !t.IsUpperCase(cleaned[0]) {
		cleaned = strings.ToUpper(string(cleaned[0])) + cleaned[1:]
	}
	
	return cleaned
}

// logTakeaways logs individual takeaways for visibility
func (t *TakeawayExtractorAgent) logTakeaways(ctx context.Context, takeaways []string) {
	correlationID := getCorrelationID(ctx)
	
	for i, takeaway := range takeaways {
		t.logger.WithFields(map[string]interface{}{
			"agent":          t.Name(),
			"correlation_id": correlationID,
			"takeaway_num":   i + 1,
			"takeaway":       t.TruncateForLog(takeaway, 150),
		}).Info("Extracted takeaway")
	}
}