package agents

import (
	"context"
)

// Agent defines the interface that all AI agents must implement
type Agent interface {
	// Process analyzes the given content and returns results
	Process(ctx context.Context, content string) (Result, error)
	
	// Name returns the agent's name for logging and identification
	Name() string
}

// Result represents the output from an agent's processing
type Result struct {
	// Summary contains generated summary text (for SummarizerAgent)
	Summary string `json:"summary,omitempty"`
	
	// Takeaways contains extracted key insights (for TakeawayExtractorAgent)
	Takeaways []string `json:"takeaways,omitempty"`
	
	// FactChecks contains verification results (for FactCheckerAgent)
	FactChecks []FactCheck `json:"fact_checks,omitempty"`
}

// FactCheck represents a single fact verification result
type FactCheck struct {
	Claim      string   `json:"claim"`
	Verdict    string   `json:"verdict"`    // "true", "false", "partially_true", "unverifiable"
	Confidence float64  `json:"confidence"` // 0.0-1.0
	Evidence   string   `json:"evidence"`
	Sources    []string `json:"sources"`
}

// ProcessingOptions contains optional parameters for agent processing
type ProcessingOptions struct {
	// Summary provides context for takeaway extraction
	Summary string
	
	// MaxResults limits the number of results returned
	MaxResults int
}