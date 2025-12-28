package services

import (
	"context"
	"encoding/json"
	"podcast-analyzer/internal/agents"
	"podcast-analyzer/internal/logger"

	"github.com/google/uuid"
)

// runAnalysisAgents runs the AI analysis agents in sequence
func (s *AnalysisService) runAnalysisAgents(ctx context.Context, content string, jobID uuid.UUID, correlationID string) (*AnalysisResults, error) {
	log := logger.WithCorrelationID(correlationID)
	log.WithFields(map[string]interface{}{
		"job_id":         jobID,
		"content_length": len(content),
		"word_count":     len([]rune(content)) / 6, // rough estimate
	}).Info("Starting AI agent analysis")
	
	// Set correlation ID in context for agent tracing
	ctx = context.WithValue(ctx, "correlation_id", correlationID)
	
	// 1. Run Summarizer Agent
	summary, err := s.runSummarizerAgent(ctx, content, jobID, correlationID)
	if err != nil {
		return nil, err
	}
	
	// 2. Run Takeaway Extractor Agent (with summary context)
	takeaways, err := s.runTakeawayExtractorAgent(ctx, content, summary, jobID, correlationID)
	if err != nil {
		return nil, err
	}
	
	// 3. Run Fact Checker Agent
	factCheckResults, err := s.runFactCheckerAgent(ctx, content, jobID, correlationID)
	if err != nil {
		return nil, err
	}
	
	// Transform results to expected API format
	return s.transformAnalysisResults(summary, takeaways, factCheckResults, jobID, correlationID)
}

// runSummarizerAgent processes content through the summarizer agent
func (s *AnalysisService) runSummarizerAgent(ctx context.Context, content string, jobID uuid.UUID, correlationID string) (string, error) {
	log := logger.WithCorrelationID(correlationID)
	summarizerAgent := agents.NewSummarizerAgent(s.config)
	
	log.WithField("job_id", jobID).Info("Agent started: summarizer")
	summarizerResult, err := summarizerAgent.Process(ctx, content)
	if err != nil {
		log.WithFields(map[string]interface{}{
			"job_id": jobID,
			"agent":  "summarizer",
			"error":  err.Error(),
		}).Error("Summarizer agent failed")
		return "", err
	}
	
	summary := summarizerResult.Summary
	log.WithFields(map[string]interface{}{
		"job_id":        jobID,
		"agent":         "summarizer",
		"summary_chars": len(summary),
	}).Info("Agent completed: summarizer")
	
	return summary, nil
}

// runTakeawayExtractorAgent processes content through the takeaway extractor agent
func (s *AnalysisService) runTakeawayExtractorAgent(ctx context.Context, content, summary string, jobID uuid.UUID, correlationID string) ([]string, error) {
	log := logger.WithCorrelationID(correlationID)
	takeawayAgent := agents.NewTakeawayExtractorAgent(s.config)
	
	log.WithField("job_id", jobID).Info("Agent started: takeaway_extractor")
	takeawayResult, err := takeawayAgent.ProcessWithOptions(ctx, content, agents.ProcessingOptions{
		Summary: summary,
	})
	if err != nil {
		log.WithFields(map[string]interface{}{
			"job_id": jobID,
			"agent":  "takeaway_extractor",
			"error":  err.Error(),
		}).Error("Takeaway extractor agent failed, continuing without takeaways")
		// Return empty takeaways instead of error to continue processing
		return []string{}, nil
	}
	
	takeaways := takeawayResult.Takeaways
	log.WithFields(map[string]interface{}{
		"job_id":          jobID,
		"agent":           "takeaway_extractor",
		"takeaways_count": len(takeaways),
	}).Info("Agent completed: takeaway_extractor")
	
	return takeaways, nil
}

// runFactCheckerAgent processes content through the fact checker agent
func (s *AnalysisService) runFactCheckerAgent(ctx context.Context, content string, jobID uuid.UUID, correlationID string) ([]agents.FactCheck, error) {
	log := logger.WithCorrelationID(correlationID)
	factCheckerAgent := agents.NewFactCheckerAgent(s.config)
	
	log.WithField("job_id", jobID).Info("Agent started: fact_checker")
	factCheckResult, err := factCheckerAgent.Process(ctx, content)
	if err != nil {
		log.WithFields(map[string]interface{}{
			"job_id": jobID,
			"agent":  "fact_checker",
			"error":  err.Error(),
		}).Error("Fact checker agent failed, continuing without fact checks")
		// Return empty fact checks instead of error to continue processing
		return []agents.FactCheck{}, nil
	}
	
	factCheckResults := factCheckResult.FactChecks
	
	// Count verdicts for logging
	verdictCounts := make(map[string]int)
	for _, fc := range factCheckResults {
		verdictCounts[fc.Verdict]++
	}
	
	log.WithFields(map[string]interface{}{
		"job_id":                   jobID,
		"agent":                    "fact_checker",
		"claims_verified":          len(factCheckResults),
		"claims_true":              verdictCounts["true"],
		"claims_false":             verdictCounts["false"],
		"claims_partially_true":    verdictCounts["partially_true"],
		"claims_unverifiable":      verdictCounts["unverifiable"],
	}).Info("Agent completed: fact_checker")
	
	return factCheckResults, nil
}

// transformAnalysisResults converts agent outputs to the expected API response format
func (s *AnalysisService) transformAnalysisResults(summary string, takeaways []string, factCheckResults []agents.FactCheck, jobID uuid.UUID, correlationID string) (*AnalysisResults, error) {
	log := logger.WithCorrelationID(correlationID)
	
	// Convert takeaways to the expected format
	takeawaysJSON, err := json.Marshal(takeaways)
	if err != nil {
		log.WithFields(map[string]interface{}{
			"job_id": jobID,
			"error":  err.Error(),
		}).Error("Failed to marshal takeaways")
		return nil, err
	}
	
	var takeawaysMap map[string]interface{}
	if err := json.Unmarshal(takeawaysJSON, &takeawaysMap); err != nil {
		// Fallback to simple format
		takeawaysMap = map[string]interface{}{
			"takeaways": takeaways,
		}
	} else {
		takeawaysMap = map[string]interface{}{
			"takeaways": takeaways,
		}
	}
	
	// Convert fact checks to the expected format
	factChecksConverted := make([]FactCheckResult, len(factCheckResults))
	for i, fc := range factCheckResults {
		sourcesMap := map[string]interface{}{
			"sources": fc.Sources,
		}
		
		factChecksConverted[i] = FactCheckResult{
			Claim:      fc.Claim,
			Verdict:    fc.Verdict,
			Confidence: fc.Confidence,
			Evidence:   fc.Evidence,
			Sources:    sourcesMap,
		}
	}
	
	results := &AnalysisResults{
		Summary:    summary,
		Takeaways:  takeawaysMap,
		FactChecks: factChecksConverted,
	}
	
	log.WithFields(map[string]interface{}{
		"job_id":            jobID,
		"summary_length":    len(summary),
		"takeaways_count":   len(takeaways),
		"fact_checks_count": len(factCheckResults),
	}).Info("All AI agents completed successfully")
	
	return results, nil
}