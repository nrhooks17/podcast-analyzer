package handlers

import (
	"net/http"
	"backend-golang/internal/services"
	"backend-golang/pkg/logger"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type TranscriptHandler struct {
	transcriptService services.TranscriptServiceInterface
}

func NewTranscriptHandler(transcriptService services.TranscriptServiceInterface) *TranscriptHandler {
	return &TranscriptHandler{
		transcriptService: transcriptService,
	}
}

// UploadTranscript handles file upload
func (h *TranscriptHandler) UploadTranscript(c *gin.Context) {
	correlationID := getCorrelationID(c)
	
	// Log request start
	logger.Log.WithFields(map[string]interface{}{
		"correlation_id": correlationID,
		"client_ip":      c.ClientIP(),
		"user_agent":     c.GetHeader("User-Agent"),
	}).Info("Upload transcript request received")

	file, err := c.FormFile("file")
	if err != nil {
		errMsg := "No file uploaded or invalid file"
		logger.Log.WithFields(map[string]interface{}{
			"correlation_id": correlationID,
			"error":          err.Error(),
		}).Error("File upload validation failed")
		
		c.JSON(http.StatusBadRequest, gin.H{
			"error": map[string]interface{}{
				"code":    "FILE_VALIDATION_ERROR",
				"message": errMsg,
			},
		})
		return
	}

	logger.Log.WithFields(map[string]interface{}{
		"correlation_id": correlationID,
		"filename":       file.Filename,
		"file_size":      file.Size,
	}).Info("Processing uploaded file")

	req := &services.UploadTranscriptRequest{
		File: file,
	}

	response, err := h.transcriptService.UploadTranscript(req, correlationID)
	if err != nil {
		statusCode := http.StatusBadRequest
		errorCode := "FILE_VALIDATION_ERROR"

		if contains(err.Error(), "duplicate") {
			statusCode = http.StatusConflict
			errorCode = "DUPLICATE_TRANSCRIPT"
		}

		logger.LogErrorWithStackAndCorrelation(err, correlationID, map[string]interface{}{
			"error_code":  errorCode,
			"status_code": statusCode,
			"filename":    file.Filename,
			"file_size":   file.Size,
			"operation":   "upload_transcript",
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
		"correlation_id":  correlationID,
		"transcript_id":   response.TranscriptID,
		"filename":        response.Filename,
		"word_count":      response.WordCount,
	}).Info("Upload completed successfully")

	c.JSON(http.StatusOK, response)
}

// GetTranscripts returns paginated list of transcripts
func (h *TranscriptHandler) GetTranscripts(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	transcripts, total, err := h.transcriptService.GetTranscripts(page, perPage)
	if err != nil {
		logger.LogErrorWithStack(err, map[string]interface{}{
			"operation": "get_transcripts",
			"page":      page,
			"per_page":  perPage,
		})
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": map[string]interface{}{
				"code":    "INTERNAL_ERROR",
				"message": "Failed to retrieve transcripts",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"transcripts": transcripts,
		"total":       total,
		"page":        page,
		"per_page":    perPage,
	})
}

// GetTranscript returns a single transcript
func (h *TranscriptHandler) GetTranscript(c *gin.Context) {
	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": map[string]interface{}{
				"code":    "INVALID_UUID",
				"message": "Invalid transcript ID format",
			},
		})
		return
	}

	transcript, err := h.transcriptService.GetTranscript(id)
	if err != nil {
		statusCode := http.StatusNotFound
		errorCode := "TRANSCRIPT_NOT_FOUND"

		if !contains(err.Error(), "not found") {
			statusCode = http.StatusInternalServerError
			errorCode = "INTERNAL_ERROR"
		}

		logger.LogErrorWithStack(err, map[string]interface{}{
			"transcript_id": id,
			"error_code":    errorCode,
			"status_code":   statusCode,
			"operation":     "get_transcript",
		})

		c.JSON(statusCode, gin.H{
			"error": map[string]interface{}{
				"code":    errorCode,
				"message": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, transcript)
}

// DeleteTranscript deletes a transcript
func (h *TranscriptHandler) DeleteTranscript(c *gin.Context) {
	correlationID := getCorrelationID(c)
	idParam := c.Param("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": map[string]interface{}{
				"code":    "INVALID_UUID",
				"message": "Invalid transcript ID format",
			},
		})
		return
	}

	if err := h.transcriptService.DeleteTranscript(id, correlationID); err != nil {
		statusCode := http.StatusNotFound
		errorCode := "TRANSCRIPT_NOT_FOUND"

		if !contains(err.Error(), "not found") {
			statusCode = http.StatusInternalServerError
			errorCode = "INTERNAL_ERROR"
		}

		logger.LogErrorWithStackAndCorrelation(err, correlationID, map[string]interface{}{
			"transcript_id": id,
			"error_code":    errorCode,
			"status_code":   statusCode,
			"operation":     "delete_transcript",
		})

		c.JSON(statusCode, gin.H{
			"error": map[string]interface{}{
				"code":    errorCode,
				"message": err.Error(),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Transcript deleted successfully",
	})
}