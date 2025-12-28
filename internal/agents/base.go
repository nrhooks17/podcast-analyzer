package agents

import (
	"context"
	"strings"
	"time"
	
	"podcast-analyzer/internal/logger"
	"github.com/sirupsen/logrus"
)

// BaseAgent provides common functionality for all AI agents
type BaseAgent struct {
	name   string
	logger *logrus.Logger
}

// NewBaseAgent creates a new base agent
func NewBaseAgent(name string) *BaseAgent {
	return &BaseAgent{
		name:   name,
		logger: logger.Log,
	}
}

// Name returns the agent's name
func (b *BaseAgent) Name() string {
	return b.name
}

// LogStart logs the beginning of agent processing
func (b *BaseAgent) LogStart(ctx context.Context, contentLength int) {
	correlationID := getCorrelationID(ctx)
	b.logger.WithFields(map[string]interface{}{
		"agent":          b.name,
		"correlation_id": correlationID,
		"content_length": contentLength,
		"word_count":     estimateWordCount(contentLength),
	}).Info("Agent processing started")
}

// LogSuccess logs successful completion of agent processing
func (b *BaseAgent) LogSuccess(ctx context.Context, result *Result, duration time.Duration) {
	correlationID := getCorrelationID(ctx)
	fields := map[string]interface{}{
		"agent":            b.name,
		"correlation_id":   correlationID,
		"duration_ms":      duration.Milliseconds(),
		"duration_seconds": duration.Seconds(),
	}
	
	// Add result-specific metrics
	if result.Summary != "" {
		fields["summary_length"] = len(result.Summary)
		fields["summary_chars"] = len(result.Summary)
	}
	
	if len(result.Takeaways) > 0 {
		fields["takeaways_count"] = len(result.Takeaways)
	}
	
	if len(result.FactChecks) > 0 {
		fields["fact_checks_count"] = len(result.FactChecks)
		
		// Count verdicts
		verdictCounts := make(map[string]int)
		for _, fc := range result.FactChecks {
			verdictCounts[fc.Verdict]++
		}
		
		if verdictCounts["true"] > 0 {
			fields["fact_checks_true"] = verdictCounts["true"]
		}
		if verdictCounts["false"] > 0 {
			fields["fact_checks_false"] = verdictCounts["false"]
		}
		if verdictCounts["partially_true"] > 0 {
			fields["fact_checks_partial"] = verdictCounts["partially_true"]
		}
		if verdictCounts["unverifiable"] > 0 {
			fields["fact_checks_unverifiable"] = verdictCounts["unverifiable"]
		}
	}
	
	b.logger.WithFields(fields).Info("Agent processing completed successfully")
}

// LogError logs agent processing errors
func (b *BaseAgent) LogError(ctx context.Context, err error, duration time.Duration) {
	correlationID := getCorrelationID(ctx)
	logger.LogErrorWithStackAndCorrelation(err, correlationID, map[string]interface{}{
		"agent":            b.name,
		"duration_ms":      duration.Milliseconds(),
		"duration_seconds": duration.Seconds(),
		"operation":        "agent_processing",
	})
}

// LogAPICall logs details about external API calls
func (b *BaseAgent) LogAPICall(ctx context.Context, service string, promptLength int, hasSystem bool) {
	correlationID := getCorrelationID(ctx)
	b.logger.WithFields(map[string]interface{}{
		"agent":          b.name,
		"correlation_id": correlationID,
		"service":        service,
		"prompt_length":  promptLength,
		"has_system":     hasSystem,
	}).Info("Making API call")
}

// LogAPIResponse logs details about API responses
func (b *BaseAgent) LogAPIResponse(ctx context.Context, service string, responseLength int, duration time.Duration) {
	correlationID := getCorrelationID(ctx)
	b.logger.WithFields(map[string]interface{}{
		"agent":           b.name,
		"correlation_id":  correlationID,
		"service":         service,
		"response_length": responseLength,
		"duration_ms":     duration.Milliseconds(),
	}).Info("API response received")
}

// ValidateContent performs basic validation on input content
func (b *BaseAgent) ValidateContent(content string) error {
	content = strings.TrimSpace(content)
	if content == "" {
		return NewAgentError(b.name, "cannot process empty content", nil)
	}
	
	// Check for reasonable content length (not too short, not too long)
	if len(content) < 50 {
		return NewAgentError(b.name, "content too short for meaningful analysis", nil)
	}
	
	if len(content) > 1000000 { // 1MB limit
		return NewAgentError(b.name, "content too long for processing", nil)
	}
	
	return nil
}

// TruncateContent truncates content to a maximum length for API calls
func (b *BaseAgent) TruncateContent(content string, maxLength int) string {
	if len(content) <= maxLength {
		return content
	}
	
	truncated := content[:maxLength]
	
	// Try to end at a word boundary
	if lastSpace := strings.LastIndex(truncated, " "); lastSpace > maxLength-100 {
		truncated = truncated[:lastSpace]
	}
	
	return truncated + "\n[...content truncated...]"
}

// TruncateForLog truncates text for logging to avoid overly long log messages
func (b *BaseAgent) TruncateForLog(text string, maxLength int) string {
	if len(text) <= maxLength {
		return text
	}
	return text[:maxLength] + "..."
}

// Helper functions

// getCorrelationID extracts correlation ID from context
func getCorrelationID(ctx context.Context) string {
	if id := ctx.Value("correlation_id"); id != nil {
		if correlationID, ok := id.(string); ok {
			return correlationID
		}
	}
	return ""
}

// estimateWordCount provides a rough word count estimate from character count
func estimateWordCount(charCount int) int {
	// Rough estimate: average English word is ~5 characters + space
	return charCount / 6
}

// IsUpperCase checks if a byte represents an uppercase letter
func (b *BaseAgent) IsUpperCase(char byte) bool {
	return char >= 'A' && char <= 'Z'
}