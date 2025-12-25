package handlers

import (
	"net/http"
	"strings"
	"backend-golang/internal/services"
	"backend-golang/pkg/logger"

	"github.com/google/uuid"
)

type AnalysisHandler struct {
	analysisService services.AnalysisServiceInterface
}

func NewAnalysisHandler(analysisService services.AnalysisServiceInterface) *AnalysisHandler {
	return &AnalysisHandler{
		analysisService: analysisService,
	}
}

// StartAnalysis starts an analysis job
func (h *AnalysisHandler) StartAnalysis(w http.ResponseWriter, r *http.Request) {
	correlationID := getCorrelationID(r)
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}
	
	// Extract transcript ID from path like /api/analyze/123
	transcriptIDParam, err := extractIDFromPath(r.URL.Path, "/api/analyze/")
	if err != nil {
		writeErrorWithCorrelation(w, http.StatusBadRequest, "INVALID_PATH", "Invalid analysis path", correlationID)
		return
	}
	
	logger.Log.WithFields(map[string]interface{}{
		"correlation_id": correlationID,
		"transcript_id":  transcriptIDParam,
		"client_ip":      getClientIP(r),
	}).Info("Analysis job request received")
	
	transcriptID, err := parseUUIDParam(transcriptIDParam)
	if err != nil {
		errMsg := "Invalid transcript ID format"
		logger.Log.WithFields(map[string]interface{}{
			"correlation_id":    correlationID,
			"transcript_id_raw": transcriptIDParam,
			"error":             err.Error(),
		}).Error("Invalid transcript ID format")
		
		writeJSON(w,http.StatusBadRequest, map[string]interface{}{
			"error": map[string]interface{}{
				"code":           "INVALID_UUID",
				"message":        errMsg,
				"correlation_id": correlationID,
			},
		})
		return
	}

	req := &services.AnalysisJobRequest{
		TranscriptID: transcriptID,
	}

	logger.Log.WithFields(map[string]interface{}{
		"correlation_id": correlationID,
		"transcript_id":  transcriptID,
	}).Info("Creating analysis job")

	response, err := h.analysisService.CreateAnalysisJob(req, correlationID)
	if err != nil {
		statusCode := http.StatusBadRequest
		errorCode := "ANALYSIS_CREATION_ERROR"

		if contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
			errorCode = "TRANSCRIPT_NOT_FOUND"
		}

		logger.LogErrorWithStackAndCorrelation(err, correlationID, map[string]interface{}{
			"transcript_id": transcriptID,
			"error_code":    errorCode,
			"status_code":   statusCode,
			"operation":     "analysis_job_creation",
		})

		writeJSON(w,statusCode, map[string]interface{}{
			"error": map[string]interface{}{
				"code":           errorCode,
				"message":        err.Error(),
				"correlation_id": correlationID,
			},
		})
		return
	}

	logger.Log.WithFields(map[string]interface{}{
		"correlation_id": correlationID,
		"job_id":         response.JobID,
		"transcript_id":  response.TranscriptID,
		"status":         response.Status,
	}).Info("Analysis job created successfully")

	writeJSON(w, http.StatusOK, response)
}

// GetJobStatus returns job status
func (h *AnalysisHandler) GetJobStatus(w http.ResponseWriter, r *http.Request) {
	correlationID := getCorrelationID(r)
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}
	
	// Extract job ID from path like /api/jobs/123/status
	jobIDParam, err := extractIDFromPath(r.URL.Path, "/api/jobs/")
	if err != nil {
		writeErrorWithCorrelation(w, http.StatusBadRequest, "INVALID_PATH", "Invalid job path", correlationID)
		return
	}
	// Remove /status suffix if present
	jobIDParam = strings.TrimSuffix(jobIDParam, "/status")
	
	jobID, err := uuid.Parse(jobIDParam)
	if err != nil {
		writeJSON(w,http.StatusBadRequest, map[string]interface{}{
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

		if !contains(err.Error(), "not found") {
			statusCode = http.StatusInternalServerError
			errorCode = "INTERNAL_ERROR"
		}

		logger.LogErrorWithStackAndCorrelation(err, correlationID, map[string]interface{}{
			"job_id":     jobID,
			"error_code": errorCode,
			"status_code": statusCode,
			"operation":   "get_job_status",
		})

		writeJSON(w,statusCode, map[string]interface{}{
			"error": map[string]interface{}{
				"code":           errorCode,
				"message":        err.Error(),
				"correlation_id": correlationID,
			},
		})
		return
	}

	writeJSON(w, http.StatusOK, response)
}

// GetAnalysisResults returns complete analysis results
func (h *AnalysisHandler) GetAnalysisResults(w http.ResponseWriter, r *http.Request) {
	correlationID := getCorrelationID(r)
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}
	
	// Extract analysis ID from path like /api/results/123
	analysisIDParam, err := extractIDFromPath(r.URL.Path, "/api/results/")
	if err != nil {
		writeErrorWithCorrelation(w, http.StatusBadRequest, "INVALID_PATH", "Invalid analysis path", correlationID)
		return
	}
	
	analysisID, err := uuid.Parse(analysisIDParam)
	if err != nil {
		writeJSON(w,http.StatusBadRequest, map[string]interface{}{
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

		if !contains(err.Error(), "not found") {
			statusCode = http.StatusInternalServerError
			errorCode = "INTERNAL_ERROR"
		}

		logger.LogErrorWithStackAndCorrelation(err, correlationID, map[string]interface{}{
			"analysis_id": analysisID,
			"error_code":  errorCode,
			"status_code": statusCode,
			"operation":   "get_analysis_results",
		})

		writeJSON(w,statusCode, map[string]interface{}{
			"error": map[string]interface{}{
				"code":           errorCode,
				"message":        err.Error(),
				"correlation_id": correlationID,
			},
		})
		return
	}

	writeJSON(w, http.StatusOK, response)
}

// ListAnalysisResults returns paginated list of analysis results
func (h *AnalysisHandler) ListAnalysisResults(w http.ResponseWriter, r *http.Request) {
	// Handle both /api/results/ and /api/results
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	page := getQueryParamInt(r, "page", 1)
	perPage := getQueryParamInt(r, "per_page", 20)

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
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve analysis results")
		return
	}

	// Set CORS header directly (this was the original fix for CORS issue)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"results":  results,
		"total":    total,
		"page":     page,
		"per_page": perPage,
	})
}