package services

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"time"
	"podcast-analyzer/internal/models"
	"podcast-analyzer/internal/logger"

	"github.com/google/uuid"
)

// setupJobPanicRecovery sets up panic recovery for analysis jobs
func (s *AnalysisService) setupJobPanicRecovery(jobID uuid.UUID, correlationID string) func() {
	return func() {
		if r := recover(); r != nil {
			// Get stack trace
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			
			logger.Log.WithFields(map[string]interface{}{
				"panic":          r,
				"stack_trace":    string(buf[:n]),
				"job_id":         jobID,
				"correlation_id": correlationID,
			}).Error("Analysis job panicked")
			
			s.UpdateJobStatus(jobID, "failed", fmt.Sprintf("Job panicked: %v", r))
		}
	}
}

// getTranscriptForJob retrieves the transcript for analysis
func (s *AnalysisService) getTranscriptForJob(transcriptID uuid.UUID, jobID uuid.UUID, correlationID string) (*models.Transcript, string, error) {
	var transcript models.Transcript
	if err := s.db.Where("id = ?", transcriptID).First(&transcript).Error; err != nil {
		errorMsg := fmt.Sprintf("Transcript not found: %s", transcriptID.String())
		logger.LogErrorWithStackAndCorrelation(err, correlationID, map[string]interface{}{
			"transcript_id": transcriptID,
			"operation":     "get_transcript",
		})
		s.UpdateJobStatus(jobID, "failed", errorMsg)
		return nil, "", fmt.Errorf("%s: %w", errorMsg, err)
	}

	// Read transcript content
	transcriptService := NewTranscriptService(s.db, s.config)
	content, err := transcriptService.ReadTranscriptContent(&transcript)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to read transcript content from %s", transcript.FilePath)
		logger.LogErrorWithStackAndCorrelation(err, correlationID, map[string]interface{}{
			"file_path": transcript.FilePath,
			"operation": "read_transcript_content",
		})
		s.UpdateJobStatus(jobID, "failed", errorMsg)
		return nil, "", fmt.Errorf("%s: %w", errorMsg, err)
	}

	return &transcript, content, nil
}

// saveAnalysisResults saves the analysis results to the database
func (s *AnalysisService) saveAnalysisResults(jobID uuid.UUID, results *AnalysisResults, correlationID string) (*models.AnalysisResult, error) {
	// Convert takeaways to JSON for database storage
	takeawaysJSON, err := json.Marshal(results.Takeaways)
	if err != nil {
		errorMsg := "Failed to serialize takeaways"
		logger.LogErrorWithStackAndCorrelation(err, correlationID, map[string]interface{}{
			"operation": "serialize_takeaways",
		})
		s.UpdateJobStatus(jobID, "failed", errorMsg)
		return nil, err
	}

	// Update existing analysis record
	var analysis models.AnalysisResult
	if err := s.db.Where("job_id = ?", jobID).First(&analysis).Error; err != nil {
		errorMsg := "Failed to find analysis record to update"
		logger.LogErrorWithStackAndCorrelation(err, correlationID, map[string]interface{}{
			"job_id":    jobID,
			"operation": "find_analysis_record",
		})
		s.UpdateJobStatus(jobID, "failed", errorMsg)
		return nil, err
	}

	analysis.Summary = &results.Summary
	analysis.Takeaways = takeawaysJSON
	now := time.Now()
	analysis.CompletedAt = &now

	if err := s.db.Save(&analysis).Error; err != nil {
		errorMsg := "Failed to save analysis results"
		logger.LogErrorWithStackAndCorrelation(err, correlationID, map[string]interface{}{
			"analysis_id": analysis.ID,
			"operation":   "save_analysis_results",
		})
		s.UpdateJobStatus(jobID, "failed", errorMsg)
		return nil, err
	}

	return &analysis, nil
}

// saveFactChecks saves fact check results to the database
func (s *AnalysisService) saveFactChecks(analysisID uuid.UUID, factChecks []FactCheckResult, correlationID string) {
	if len(factChecks) == 0 {
		return
	}

	for _, fc := range factChecks {
		// Convert sources to JSON
		sourcesJSON, _ := json.Marshal(fc.Sources)
		
		factCheck := &models.FactCheck{
			ID:         uuid.New(),
			AnalysisID: analysisID,
			Claim:      fc.Claim,
			Verdict:    fc.Verdict,
			Confidence: fc.Confidence,
			Evidence:   &fc.Evidence,
			Sources:    sourcesJSON,
			CheckedAt:  time.Now(),
		}
		if err := s.db.Create(factCheck).Error; err != nil {
			logger.LogErrorWithStackAndCorrelation(err, correlationID, map[string]interface{}{
				"analysis_id": analysisID,
				"claim":       fc.Claim,
				"operation":   "save_fact_check",
			})
			// Continue with other fact checks
		}
	}
}

// processAnalysisJob processes an analysis job in the background
func (s *AnalysisService) processAnalysisJob(ctx context.Context, jobID uuid.UUID, transcriptID uuid.UUID, correlationID string) (retErr error) {
	// Setup panic recovery for this job
	defer s.setupJobPanicRecovery(jobID, correlationID)

	log := logger.WithCorrelationID(correlationID)
	log.WithFields(map[string]interface{}{
		"job_id":        jobID,
		"transcript_id": transcriptID,
	}).Info("Processing analysis job")

	// Update job status to processing
	if err := s.UpdateJobStatus(jobID, "processing", ""); err != nil {
		logger.LogErrorWithStackAndCorrelation(err, correlationID, map[string]interface{}{
			"job_id":    jobID,
			"operation": "update_job_status_processing",
		})
		return fmt.Errorf("failed to update job status to processing: %w", err)
	}

	// Get transcript and content
	transcript, content, err := s.getTranscriptForJob(transcriptID, jobID, correlationID)
	if err != nil {
		return err
	}
	_ = transcript // transcript available for future use (metadata, file path, etc.)

	log.WithFields(map[string]interface{}{
		"job_id":         jobID,
		"content_length": len(content),
	}).Info("Analysis starting")

	// Process with AI agents
	startTime := time.Now()
	results, err := s.runAnalysisAgents(ctx, content, jobID, correlationID)
	duration := time.Since(startTime)
	
	if err != nil {
		errorMsg := fmt.Sprintf("Analysis processing failed after %v", duration)
		logger.LogErrorWithStackAndCorrelation(err, correlationID, map[string]interface{}{
			"job_id":    jobID,
			"duration":  duration,
			"operation": "run_analysis_agents",
		})
		s.UpdateJobStatus(jobID, "failed", errorMsg)
		return fmt.Errorf("%s: %w", errorMsg, err)
	}

	log.WithFields(map[string]interface{}{
		"job_id":   jobID,
		"duration": duration,
	}).Info("AI analysis completed")

	// Save analysis results
	analysis, err := s.saveAnalysisResults(jobID, results, correlationID)
	if err != nil {
		return err
	}

	// Save fact checks
	s.saveFactChecks(analysis.ID, results.FactChecks, correlationID)

	// Mark job as completed
	if err := s.UpdateJobStatus(jobID, "completed", ""); err != nil {
		logger.LogErrorWithStackAndCorrelation(err, correlationID, map[string]interface{}{
			"job_id":    jobID,
			"operation": "update_job_status_completed",
		})
		return err
	}

	log.WithField("job_id", jobID).Info("Analysis complete. Results saved to database.")
	return nil
}