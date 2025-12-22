package services

import (
	"encoding/json"
	"fmt"
	"backend-golang/internal/config"
	"backend-golang/internal/models"
	"backend-golang/pkg/logger"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AnalysisServiceInterface defines the interface for analysis service operations
type AnalysisServiceInterface interface {
	CreateAnalysisJob(req *AnalysisJobRequest, correlationID string) (*AnalysisJobResponse, error)
	GetJobStatus(jobID uuid.UUID, correlationID string) (*JobStatusResponse, error)
	ListAnalysisResults(page, perPage int) ([]*AnalysisResultsResponse, int64, error)
	GetAnalysisResults(analysisID uuid.UUID, correlationID string) (*AnalysisResultsResponse, error)
	UpdateJobStatus(jobID uuid.UUID, status string, errorMessage string) error
}

// KafkaServiceInterface defines the interface for Kafka operations
type KafkaServiceInterface interface {
	PublishAnalysisJob(message interface{}) error
	Close() error
}

type AnalysisService struct {
	db           *gorm.DB
	config       *config.Config
	kafkaService KafkaServiceInterface
}

func NewAnalysisService(db *gorm.DB, cfg *config.Config, kafkaService KafkaServiceInterface) *AnalysisService {
	return &AnalysisService{
		db:           db,
		config:       cfg,
		kafkaService: kafkaService,
	}
}

// AnalysisJobRequest represents the request to start analysis
type AnalysisJobRequest struct {
	TranscriptID uuid.UUID `json:"transcript_id" binding:"required"`
}

// AnalysisJobResponse represents the job creation response
type AnalysisJobResponse struct {
	JobID        uuid.UUID `json:"job_id"`
	TranscriptID uuid.UUID `json:"transcript_id"`
	Status       string    `json:"status"`
	Message      string    `json:"message"`
}

// JobStatusResponse represents the job status polling response
type JobStatusResponse struct {
	JobID        uuid.UUID  `json:"job_id"`
	TranscriptID uuid.UUID  `json:"transcript_id"`
	Status       string     `json:"status"` // pending, processing, completed, failed
	CreatedAt    time.Time  `json:"created_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	ErrorMessage *string    `json:"error_message,omitempty"`
}

// AnalysisResultsResponse represents complete analysis results
type AnalysisResultsResponse struct {
	ID                 uuid.UUID                `json:"id"`
	JobID              uuid.UUID                `json:"job_id"`
	TranscriptID       uuid.UUID                `json:"transcript_id"`
	Status             string                   `json:"status"`
	Summary            *string                  `json:"summary,omitempty"`
	Takeaways          []string                 `json:"takeaways,omitempty"`
	FactChecks         []FactCheckResultResponse `json:"fact_checks"`
	CreatedAt          time.Time                `json:"created_at"`
	CompletedAt        *time.Time               `json:"completed_at,omitempty"`
	TranscriptFilename *string                  `json:"transcript_filename,omitempty"`
	TranscriptTitle    *string                  `json:"transcript_title,omitempty"`
}

// FactCheckResultResponse represents individual fact-check results
type FactCheckResultResponse struct {
	ID         uuid.UUID `json:"id"`
	Claim      string    `json:"claim"`
	Verdict    string    `json:"verdict"`
	Confidence float64   `json:"confidence"`
	Evidence   *string   `json:"evidence,omitempty"`
	Sources    []string  `json:"sources,omitempty"`
	CheckedAt  time.Time `json:"checked_at"`
}

// KafkaMessage represents the message sent to Kafka
type KafkaMessage struct {
	JobID        uuid.UUID `json:"job_id"`
	TranscriptID uuid.UUID `json:"transcript_id"`
	CreatedAt    time.Time `json:"created_at"`
}

// CreateAnalysisJob creates a new analysis job
func (s *AnalysisService) CreateAnalysisJob(req *AnalysisJobRequest, correlationID string) (*AnalysisJobResponse, error) {
	log := logger.WithCorrelationID(correlationID)

	// Verify transcript exists
	var transcript models.Transcript
	if err := s.db.Where("id = ?", req.TranscriptID).First(&transcript).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			log.WithField("transcript_id", req.TranscriptID).Error("Transcript not found for analysis")
			return nil, fmt.Errorf("transcript %s not found", req.TranscriptID)
		}
		logger.LogErrorWithStackAndCorrelation(err, correlationID, map[string]interface{}{
			"transcript_id": req.TranscriptID,
			"operation":     "find_transcript_for_analysis",
		})
		return nil, fmt.Errorf("failed to find transcript: %w", err)
	}

	// Create analysis record
	analysis := &models.AnalysisResult{
		TranscriptID: req.TranscriptID,
		JobID:        uuid.New(),
		Status:       "pending",
	}

	if err := s.db.Create(analysis).Error; err != nil {
		logger.LogErrorWithStackAndCorrelation(err, correlationID, map[string]interface{}{
			"transcript_id": req.TranscriptID,
			"job_id":        analysis.JobID,
			"operation":     "create_analysis_job",
		})
		return nil, fmt.Errorf("failed to create analysis job: %w", err)
	}

	// Send message to Kafka
	message := KafkaMessage{
		JobID:        analysis.JobID,
		TranscriptID: analysis.TranscriptID,
		CreatedAt:    analysis.CreatedAt,
	}

	if err := s.kafkaService.PublishAnalysisJob(message); err != nil {
		logger.LogErrorWithStackAndCorrelation(err, correlationID, map[string]interface{}{
			"job_id":        analysis.JobID,
			"transcript_id": req.TranscriptID,
			"operation":     "publish_analysis_job_kafka",
		})
		// Update status to failed
		s.db.Model(analysis).Update("status", "failed")
		return nil, fmt.Errorf("failed to queue analysis job: %w", err)
	}

	log.WithFields(map[string]interface{}{
		"job_id":        analysis.JobID,
		"transcript_id": req.TranscriptID,
		"analysis_id":   analysis.ID,
	}).Info("Analysis job created")

	return &AnalysisJobResponse{
		JobID:        analysis.JobID,
		TranscriptID: req.TranscriptID,
		Status:       "pending",
		Message:      "Analysis job created and queued for processing",
	}, nil
}

// GetJobStatus returns the status of an analysis job
func (s *AnalysisService) GetJobStatus(jobID uuid.UUID, correlationID string) (*JobStatusResponse, error) {
	log := logger.WithCorrelationID(correlationID)

	var analysis models.AnalysisResult
	if err := s.db.Where("job_id = ?", jobID).First(&analysis).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			log.WithField("job_id", jobID).Error("Analysis job not found")
			return nil, fmt.Errorf("analysis job %s not found", jobID)
		}
		logger.LogErrorWithStackAndCorrelation(err, correlationID, map[string]interface{}{
			"job_id":    jobID,
			"operation": "get_job_status",
		})
		return nil, fmt.Errorf("failed to get job status: %w", err)
	}

	log.WithFields(map[string]interface{}{
		"job_id":      jobID,
		"status":      analysis.Status,
		"analysis_id": analysis.ID,
	}).Info("Retrieved job status")

	return &JobStatusResponse{
		JobID:        analysis.JobID,
		TranscriptID: analysis.TranscriptID,
		Status:       analysis.Status,
		CreatedAt:    analysis.CreatedAt,
		CompletedAt:  analysis.CompletedAt,
		ErrorMessage: analysis.ErrorMessage,
	}, nil
}

// GetAnalysisResults returns complete analysis results
func (s *AnalysisService) GetAnalysisResults(analysisID uuid.UUID, correlationID string) (*AnalysisResultsResponse, error) {
	log := logger.WithCorrelationID(correlationID)

	// Join with transcript to get filename and metadata
	var analysis models.AnalysisResult
	var transcript models.Transcript
	
	if err := s.db.Where("id = ?", analysisID).First(&analysis).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			log.WithField("analysis_id", analysisID).Error("Analysis not found")
			return nil, fmt.Errorf("analysis %s not found", analysisID)
		}
		logger.LogErrorWithStackAndCorrelation(err, correlationID, map[string]interface{}{
			"analysis_id": analysisID,
			"operation":   "get_analysis",
		})
		return nil, fmt.Errorf("failed to get analysis: %w", err)
	}

	if err := s.db.Where("id = ?", analysis.TranscriptID).First(&transcript).Error; err != nil {
		logger.LogErrorWithStackAndCorrelation(err, correlationID, map[string]interface{}{
			"transcript_id": analysis.TranscriptID,
			"analysis_id":   analysisID,
			"operation":     "get_transcript_for_analysis",
		})
		return nil, fmt.Errorf("failed to get transcript: %w", err)
	}

	// Load fact checks
	var factChecks []models.FactCheck
	if err := s.db.Where("analysis_id = ?", analysisID).Find(&factChecks).Error; err != nil {
		logger.LogErrorWithStackAndCorrelation(err, correlationID, map[string]interface{}{
			"analysis_id": analysisID,
			"operation":   "load_fact_checks",
		})
		return nil, fmt.Errorf("failed to load fact checks: %w", err)
	}

	// Convert fact checks to response format
	factCheckResponses := make([]FactCheckResultResponse, len(factChecks))
	for i, fc := range factChecks {
		var sources []string
		if fc.Sources != nil {
			json.Unmarshal(fc.Sources, &sources)
		}
		
		factCheckResponses[i] = FactCheckResultResponse{
			ID:         fc.ID,
			Claim:      fc.Claim,
			Verdict:    fc.Verdict,
			Confidence: fc.Confidence,
			Evidence:   fc.Evidence,
			Sources:    sources,
			CheckedAt:  fc.CheckedAt,
		}
	}

	// Convert takeaways from JSON
	var takeaways []string
	if analysis.Takeaways != nil {
		json.Unmarshal(analysis.Takeaways, &takeaways)
	}

	// Extract title from transcript metadata if available
	var transcriptTitle *string
	if transcript.TranscriptMetadata != nil {
		var metadata map[string]interface{}
		if err := json.Unmarshal(transcript.TranscriptMetadata, &metadata); err == nil {
			if title, ok := metadata["title"].(string); ok {
				transcriptTitle = &title
			}
		}
	}

	log.WithFields(map[string]interface{}{
		"analysis_id":        analysisID,
		"status":             analysis.Status,
		"fact_checks_count":  len(factChecks),
	}).Info("Retrieved analysis results")

	return &AnalysisResultsResponse{
		ID:                 analysis.ID,
		JobID:              analysis.JobID,
		TranscriptID:       analysis.TranscriptID,
		Status:             analysis.Status,
		Summary:            analysis.Summary,
		Takeaways:          takeaways,
		FactChecks:         factCheckResponses,
		CreatedAt:          analysis.CreatedAt,
		CompletedAt:        analysis.CompletedAt,
		TranscriptFilename: &transcript.Filename,
		TranscriptTitle:    transcriptTitle,
	}, nil
}

// ListAnalysisResults returns paginated list of analysis results
func (s *AnalysisService) ListAnalysisResults(page, perPage int) ([]*AnalysisResultsResponse, int64, error) {
	var results []struct {
		models.AnalysisResult
		TranscriptFilename string `json:"transcript_filename"`
	}
	var total int64

	offset := (page - 1) * perPage

	// Count total
	if err := s.db.Model(&models.AnalysisResult{}).Count(&total).Error; err != nil {
		logger.LogErrorWithStack(err, map[string]interface{}{
			"operation": "count_analysis_results",
			"page":      page,
			"per_page":  perPage,
		})
		return nil, 0, fmt.Errorf("failed to count analysis results: %w", err)
	}

	// Get results with transcript filename
	if err := s.db.
		Table("analysis_results").
		Select("analysis_results.*, transcripts.filename as transcript_filename").
		Joins("JOIN transcripts ON analysis_results.transcript_id = transcripts.id").
		Order("analysis_results.created_at DESC").
		Offset(offset).
		Limit(perPage).
		Scan(&results).Error; err != nil {
		logger.LogErrorWithStack(err, map[string]interface{}{
			"operation": "get_analysis_results_list",
			"page":      page,
			"per_page":  perPage,
			"offset":    offset,
		})
		return nil, 0, fmt.Errorf("failed to get analysis results: %w", err)
	}

	// Convert to response format
	responses := make([]*AnalysisResultsResponse, len(results))
	for i, result := range results {
		// Load fact checks for this analysis
		var factChecks []models.FactCheck
		s.db.Where("analysis_id = ?", result.ID).Find(&factChecks)

		factCheckResponses := make([]FactCheckResultResponse, len(factChecks))
		for j, fc := range factChecks {
			var sources []string
			if fc.Sources != nil {
				json.Unmarshal(fc.Sources, &sources)
			}
			
			factCheckResponses[j] = FactCheckResultResponse{
				ID:         fc.ID,
				Claim:      fc.Claim,
				Verdict:    fc.Verdict,
				Confidence: fc.Confidence,
				Evidence:   fc.Evidence,
				Sources:    sources,
				CheckedAt:  fc.CheckedAt,
			}
		}

		// Convert takeaways from JSON
		var takeaways []string
		if result.Takeaways != nil {
			json.Unmarshal(result.Takeaways, &takeaways)
		}

		responses[i] = &AnalysisResultsResponse{
			ID:                 result.ID,
			JobID:              result.JobID,
			TranscriptID:       result.TranscriptID,
			Status:             result.Status,
			Summary:            result.Summary,
			Takeaways:          takeaways,
			FactChecks:         factCheckResponses,
			CreatedAt:          result.CreatedAt,
			CompletedAt:        result.CompletedAt,
			TranscriptFilename: &result.TranscriptFilename,
		}
	}

	return responses, total, nil
}

// UpdateJobStatus updates the status of an analysis job (matches Python def update_job_status)
func (s *AnalysisService) UpdateJobStatus(jobID uuid.UUID, status string, errorMessage string) error {
	var analysis models.AnalysisResult
	if err := s.db.Where("job_id = ?", jobID).First(&analysis).Error; err != nil {
		logger.LogErrorWithStack(err, map[string]interface{}{
			"job_id":    jobID,
			"operation": "find_job_for_status_update",
		})
		return err
	}

	analysis.Status = status
	if errorMessage != "" {
		analysis.ErrorMessage = &errorMessage
	}
	if status == "completed" || status == "failed" {
		now := time.Now()
		analysis.CompletedAt = &now
	}

	if err := s.db.Save(&analysis).Error; err != nil {
		logger.LogErrorWithStack(err, map[string]interface{}{
			"job_id":      jobID,
			"analysis_id": analysis.ID,
			"status":      status,
			"operation":   "save_job_status_update",
		})
		return err
	}

	logger.Log.WithFields(map[string]interface{}{
		"job_id": jobID,
		"status": status,
		"analysis_id": analysis.ID,
	}).Info("Updated job status")

	return nil
}