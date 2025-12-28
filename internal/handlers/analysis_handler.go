package handlers

import (
	"podcast-analyzer/internal/services"
	"podcast-analyzer/internal/logger"
	"podcast-analyzer/internal/utils"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// AnalysisServiceInterface defines the interface for analysis service
type AnalysisServiceInterface interface {
	CreateAnalysisJob(req *services.AnalysisJobRequest, correlationID string) (*services.AnalysisJobResponse, error)
	GetJobStatus(jobID uuid.UUID, correlationID string) (*services.JobStatusResponse, error)
	ListAnalysisResults(page, perPage int) ([]*services.AnalysisResultsResponse, int64, error)
	GetAnalysisResults(analysisID uuid.UUID, correlationID string) (*services.AnalysisResultsResponse, error)
}

type AnalysisHandler struct {
	analysisService AnalysisServiceInterface
}

func NewAnalysisHandler(analysisService AnalysisServiceInterface) *AnalysisHandler {
	return &AnalysisHandler{
		analysisService: analysisService,
	}
}

// validateAnalysisRequest validates the analysis request and extracts transcript ID
func (h *AnalysisHandler) validateAnalysisRequest(r *http.Request, correlationID string) (uuid.UUID, error) {
	// Extract transcript ID from path like /api/analyze/123
	transcriptIDParam, err := utils.ExtractIDFromPath(r.URL.Path, "/api/analyze/")
	if err != nil {
		return uuid.Nil, err
	}

	transcriptID, err := uuid.Parse(transcriptIDParam)
	if err != nil {
		logger.Log.WithFields(map[string]interface{}{
			"correlation_id":    correlationID,
			"transcript_id_raw": transcriptIDParam,
			"error":             err.Error(),
		}).Error("Invalid transcript ID format")
		return uuid.Nil, err
	}

	return transcriptID, nil
}

// handleAnalysisServiceError determines error type and status code for analysis service errors
func (h *AnalysisHandler) handleAnalysisServiceError(err error) (int, string) {
	if utils.Contains(err.Error(), "not found") {
		return http.StatusNotFound, "TRANSCRIPT_NOT_FOUND"
	}
	return http.StatusBadRequest, "ANALYSIS_CREATION_ERROR"
}

// logAnalysisRequest logs the start of an analysis request
func (h *AnalysisHandler) logAnalysisRequest(r *http.Request, transcriptIDParam, correlationID string) {
	logger.Log.WithFields(map[string]interface{}{
		"correlation_id": correlationID,
		"transcript_id":  transcriptIDParam,
		"client_ip":      utils.GetClientIP(r),
	}).Info("Analysis job request received")
}

// logAnalysisSuccess logs successful analysis job creation
func (h *AnalysisHandler) logAnalysisSuccess(response *services.AnalysisJobResponse, correlationID string) {
	logger.Log.WithFields(map[string]interface{}{
		"correlation_id": correlationID,
		"job_id":         response.JobID,
		"transcript_id":  response.TranscriptID,
		"status":         response.Status,
	}).Info("Analysis job created successfully")
}

// StartAnalysis starts an analysis job
func (h *AnalysisHandler) StartAnalysis(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	utils.SetCORSHeaders(w)

	correlationID := utils.GetCorrelationID(r)

	if r.Method == http.MethodOptions {
		// Handle preflight request
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodPost {
		utils.WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	// Extract transcript ID from path
	transcriptIDParam, _ := utils.ExtractIDFromPath(r.URL.Path, "/api/analyze/")
	h.logAnalysisRequest(r, transcriptIDParam, correlationID)

	// Validate analysis request
	transcriptID, err := h.validateAnalysisRequest(r, correlationID)
	if err != nil {
		if transcriptIDParam == "" {
			utils.WriteErrorWithCorrelation(w, http.StatusBadRequest, "INVALID_PATH", "Invalid analysis path", correlationID)
		} else {
			utils.WriteJSON(w, http.StatusBadRequest, map[string]interface{}{
				"error": map[string]interface{}{
					"code":           "INVALID_UUID",
					"message":        "Invalid UUID format",
					"correlation_id": correlationID,
				},
			})
		}
		return
	}

	req := &services.AnalysisJobRequest{
		TranscriptID: transcriptID,
	}

	logger.Log.WithFields(map[string]interface{}{
		"correlation_id": correlationID,
		"transcript_id":  transcriptID,
	}).Info("Creating analysis job")

	// Process analysis job through service
	response, err := h.analysisService.CreateAnalysisJob(req, correlationID)
	if err != nil {
		statusCode, errorCode := h.handleAnalysisServiceError(err)

		logger.LogErrorWithStackAndCorrelation(err, correlationID, map[string]interface{}{
			"transcript_id": transcriptID,
			"error_code":    errorCode,
			"status_code":   statusCode,
			"operation":     "analysis_job_creation",
		})

		utils.WriteJSON(w, statusCode, map[string]interface{}{
			"error": map[string]interface{}{
				"code":           errorCode,
				"message":        err.Error(),
				"correlation_id": correlationID,
			},
		})
		return
	}

	h.logAnalysisSuccess(response, correlationID)
	utils.WriteJSON(w, http.StatusOK, response)
}

// GetJobStatus returns job status
func (h *AnalysisHandler) GetJobStatus(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	utils.SetCORSHeaders(w)

	correlationID := utils.GetCorrelationID(r)
	
	if r.Method == http.MethodOptions {
		// Handle preflight request
		w.WriteHeader(http.StatusNoContent)
		return
	}
	
	if r.Method != http.MethodGet {
		utils.WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	// Extract job ID from path like /api/jobs/123/status
	jobIDParam, err := utils.ExtractIDFromPath(r.URL.Path, "/api/jobs/")
	if err != nil {
		utils.WriteErrorWithCorrelation(w, http.StatusBadRequest, "INVALID_PATH", "Invalid job path", correlationID)
		return
	}
	// Remove /status suffix if present
	jobIDParam = strings.TrimSuffix(jobIDParam, "/status")

	jobID, err := uuid.Parse(jobIDParam)
	if err != nil {
		utils.WriteJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error": map[string]interface{}{
				"code":    "INVALID_UUID",
				"message": "Invalid job ID format",
			},
		})
		return
	}

	response, err := h.analysisService.GetJobStatus(jobID, correlationID)
	if err != nil {
		statusCode := http.StatusNotFound
		errorCode := "JOB_NOT_FOUND"

		if !utils.Contains(err.Error(), "not found") {
			statusCode = http.StatusInternalServerError
			errorCode = "INTERNAL_ERROR"
		}

		logger.LogErrorWithStackAndCorrelation(err, correlationID, map[string]interface{}{
			"job_id":      jobID,
			"error_code":  errorCode,
			"status_code": statusCode,
			"operation":   "get_job_status",
		})

		utils.WriteJSON(w, statusCode, map[string]interface{}{
			"error": map[string]interface{}{
				"code":           errorCode,
				"message":        err.Error(),
				"correlation_id": correlationID,
			},
		})
		return
	}

	utils.WriteJSON(w, http.StatusOK, response)
}

// GetAnalysisResults returns complete analysis results
func (h *AnalysisHandler) GetAnalysisResults(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	utils.SetCORSHeaders(w)

	correlationID := utils.GetCorrelationID(r)
	if r.Method != http.MethodGet {
		utils.WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	// Extract analysis ID from path like /api/results/123
	analysisIDParam, err := utils.ExtractIDFromPath(r.URL.Path, "/api/results/")
	if err != nil {
		utils.WriteErrorWithCorrelation(w, http.StatusBadRequest, "INVALID_PATH", "Invalid analysis path", correlationID)
		return
	}

	analysisID, err := uuid.Parse(analysisIDParam)
	if err != nil {
		utils.WriteJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error": map[string]interface{}{
				"code":    "INVALID_UUID",
				"message": "Invalid analysis ID format",
			},
		})
		return
	}

	response, err := h.analysisService.GetAnalysisResults(analysisID, correlationID)
	if err != nil {
		statusCode := http.StatusNotFound
		errorCode := "ANALYSIS_NOT_FOUND"

		if !utils.Contains(err.Error(), "not found") {
			statusCode = http.StatusInternalServerError
			errorCode = "INTERNAL_ERROR"
		}

		logger.LogErrorWithStackAndCorrelation(err, correlationID, map[string]interface{}{
			"analysis_id": analysisID,
			"error_code":  errorCode,
			"status_code": statusCode,
			"operation":   "get_analysis_results",
		})

		utils.WriteJSON(w, statusCode, map[string]interface{}{
			"error": map[string]interface{}{
				"code":           errorCode,
				"message":        err.Error(),
				"correlation_id": correlationID,
			},
		})
		return
	}

	utils.WriteJSON(w, http.StatusOK, response)
}

// ListAnalysisResults returns paginated list of analysis results
func (h *AnalysisHandler) ListAnalysisResults(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	utils.SetCORSHeaders(w)

	// Handle both /api/results/ and /api/results
	if r.Method != http.MethodGet {
		utils.WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	page := utils.GetQueryParamInt(r, "page", 1)
	perPage := utils.GetQueryParamInt(r, "per_page", 20)

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	results, total, err := h.analysisService.ListAnalysisResults(page, perPage)
	if err != nil {
		logger.LogErrorWithStack(err, map[string]interface{}{
			"operation": "list_analysis_results",
			"page":      page,
			"per_page":  perPage,
		})
		utils.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve analysis results")
		return
	}

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"results":  results,
		"total":    total,
		"page":     page,
		"per_page": perPage,
	})
}

