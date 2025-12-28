package agents

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
	
	"podcast-analyzer/internal/clients"
	"podcast-analyzer/internal/config"
)

// SummarizerAgent generates concise summaries of podcast transcripts
type SummarizerAgent struct {
	*BaseAgent
	anthropicClient clients.AnthropicClientInterface
	maxChars        int
}

// NewSummarizerAgent creates a new summarizer agent
func NewSummarizerAgent(cfg *config.Config) *SummarizerAgent {
	return &SummarizerAgent{
		BaseAgent:       NewBaseAgent("summarizer"),
		anthropicClient: clients.NewAnthropicClient(cfg),
		maxChars:        cfg.SummaryMaxChars,
	}
}

// Process generates a summary of the podcast transcript
func (s *SummarizerAgent) Process(ctx context.Context, content string) (Result, error) {
	start := time.Now()
	defer func() {
		s.LogAPICall(ctx, "anthropic", len(content), true)
	}()
	
	// Log start of processing
	s.LogStart(ctx, len(content))
	
	// Validate content
	if err := s.ValidateContent(content); err != nil {
		s.LogError(ctx, err, time.Since(start))
		return Result{}, err
	}
	
	// Build prompts
	systemPrompt := s.buildSystemPrompt()
	userPrompt := s.buildUserPrompt(content)
	
	// Call Claude API
	rawSummary, err := s.anthropicClient.CallClaude(ctx, s.Name(), userPrompt, systemPrompt, false)
	if err != nil {
		s.LogError(ctx, err, time.Since(start))
		return Result{}, NewAgentError(s.Name(), "failed to generate summary", err)
	}
	
	// Clean and validate the summary
	summary := s.cleanSummary(rawSummary)
	if err := s.validateSummary(summary); err != nil {
		s.LogError(ctx, err, time.Since(start))
		return Result{}, err
	}
	
	result := Result{Summary: summary}
	
	// Log success
	s.LogSuccess(ctx, &result, time.Since(start))
	
	return result, nil
}

// buildSystemPrompt creates the system prompt for Claude
func (s *SummarizerAgent) buildSystemPrompt() string {
	return fmt.Sprintf(`You are an expert at creating concise, professional summaries of podcast content for business audiences.

Your task is to create a summary that:
- Is a maximum of %d characters
- Captures the main topics and themes discussed
- Focuses on factual content rather than opinions
- Does not include filler words or transcription artifacts

The summary should be useful for someone who wants to post a tweet on X or update their status on Facebook.`, s.maxChars)
}

// buildUserPrompt creates the user prompt with the transcript content
func (s *SummarizerAgent) buildUserPrompt(content string) string {
	// Truncate very long transcripts for the prompt
	maxTranscriptLength := 15000 // Reasonable limit for Claude context
	if len(content) > maxTranscriptLength {
		content = s.TruncateContent(content, maxTranscriptLength)
	}
	
	return fmt.Sprintf(`Please create a professional summary of the following podcast transcript.

The summary should be a maximum of %d characters and should include:
- Main topics and themes discussed
- Overall context and purpose of the discussion

TRANSCRIPT:
%s

SUMMARY:`, s.maxChars, content)
}

// cleanSummary cleans and formats the generated summary
func (s *SummarizerAgent) cleanSummary(rawSummary string) string {
	// Remove any leading/trailing whitespace
	summary := strings.TrimSpace(rawSummary)
	
	// Remove common prefixes that might be added by Claude
	prefixes := []string{
		"Summary:",
		"SUMMARY:",
		"Podcast Summary:",
		"This podcast discusses",
		"In this podcast",
		"The podcast covers",
	}
	
	for _, prefix := range prefixes {
		if strings.HasPrefix(summary, prefix) {
			summary = strings.TrimSpace(summary[len(prefix):])
			break
		}
	}
	
	// Ensure it starts with a capital letter
	if len(summary) > 0 && !s.IsUpperCase(summary[0]) {
		summary = strings.ToUpper(string(summary[0])) + summary[1:]
	}
	
	// Remove extra whitespace and normalize spacing
	summary = regexp.MustCompile(`\s+`).ReplaceAllString(summary, " ")
	
	// Ensure it ends with proper punctuation
	if len(summary) > 0 && !strings.HasSuffix(summary, ".") && !strings.HasSuffix(summary, "!") && !strings.HasSuffix(summary, "?") {
		summary += "."
	}
	
	return summary
}

// validateSummary validates the generated summary
func (s *SummarizerAgent) validateSummary(summary string) error {
	if summary == "" {
		return NewAgentError(s.Name(), "generated summary is empty", nil)
	}
	
	if len(summary) > s.maxChars {
		// Log warning but don't fail - truncate if necessary
		s.logger.WithFields(map[string]interface{}{
			"agent":        s.Name(),
			"summary_length": len(summary),
			"max_chars":    s.maxChars,
		}).Warn("Summary exceeds maximum character limit, truncating")
		
		// Truncate to max chars, trying to end at word boundary
		if len(summary) > s.maxChars {
			truncated := summary[:s.maxChars]
			if lastSpace := strings.LastIndex(truncated, " "); lastSpace > s.maxChars-20 {
				truncated = truncated[:lastSpace]
			}
			summary = truncated + "..."
		}
	}
	
	// Check minimum length (very short summaries are probably not useful)
	if len(summary) < 20 {
		return NewAgentError(s.Name(), "summary too short to be meaningful", nil)
	}
	
	return nil
}