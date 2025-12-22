package handlers

import (
	"net/http"
	"backend-golang/internal/services"
	"backend-golang/pkg/logger"
	"strconv"

	"github.com/gin-gonic/gin"
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
func (h *AnalysisHandler) StartAnalysis(c *gin.Context) {
	correlationID := getCorrelationID(c)
	transcriptIDParam := c.Param("transcript_id")
	
	logger.Log.WithFields(map[string]interface{}{
		"correlation_id": correlationID,
		"transcript_id":  transcriptIDParam,
		"client_ip":      c.ClientIP(),
	}).Info("Analysis job request received")
	
	transcriptID, err := uuid.Parse(transcriptIDParam)
	if err != nil {
		errMsg := "Invalid transcript ID format"
		logger.Log.WithFields(map[string]interface{}{
			"correlation_id":    correlationID,
			"transcript_id_raw": transcriptIDParam,
			"error":             err.Error(),
		}).Error("Invalid transcript ID format")
		
		c.JSON(http.StatusBadRequest, gin.H{
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

		c.JSON(statusCode, gin.H{
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

	c.JSON(http.StatusOK, response)
}

// GetJobStatus returns job status
func (h *AnalysisHandler) GetJobStatus(c *gin.Context) {
	correlationID := getCorrelationID(c)
	jobIDParam := c.Param("job_id")
	
	jobID, err := uuid.Parse(jobIDParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
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

		c.JSON(statusCode, gin.H{
			"error": map[string]interface{}{
				"code":           errorCode,
				"message":        err.Error(),
				"correlation_id": correlationID,
			},
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetAnalysisResults returns complete analysis results
func (h *AnalysisHandler) GetAnalysisResults(c *gin.Context) {
	correlationID := getCorrelationID(c)
	analysisIDParam := c.Param("analysis_id")
	
	analysisID, err := uuid.Parse(analysisIDParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
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

		c.JSON(statusCode, gin.H{
			"error": map[string]interface{}{
				"code":           errorCode,
				"message":        err.Error(),
				"correlation_id": correlationID,
			},
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// ListAnalysisResults returns paginated list of analysis results
func (h *AnalysisHandler) ListAnalysisResults(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))

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
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": map[string]interface{}{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to retrieve analysis results",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"results":  results,
		"total":    total,
		"page":     page,
		"per_page": perPage,
	})
}