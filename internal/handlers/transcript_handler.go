package handlers

import (
	"fmt"
	"net/http"
	"podcast-analyzer/internal/models"
	"podcast-analyzer/internal/services"
	"podcast-analyzer/internal/logger"
	"podcast-analyzer/internal/utils"

	"github.com/google/uuid"
)

// TranscriptServiceInterface defines the interface for transcript service
type TranscriptServiceInterface interface {
	UploadTranscript(req *services.UploadTranscriptRequest, correlationID string) (*services.UploadTranscriptResponse, error)
	GetTranscripts(page, perPage int) ([]*models.Transcript, int64, error)
	GetTranscript(id uuid.UUID) (*models.Transcript, error)
	DeleteTranscript(id uuid.UUID, correlationID string) error
}

type TranscriptHandler struct {
	transcriptService TranscriptServiceInterface
}

func NewTranscriptHandler(transcriptService TranscriptServiceInterface) *TranscriptHandler {
	return &TranscriptHandler{
		transcriptService: transcriptService,
	}
}

// validateUploadRequest validates the upload request and extracts file
func (h *TranscriptHandler) validateUploadRequest(r *http.Request, correlationID string) (*services.UploadTranscriptRequest, error) {
	// Parse multipart form
	err := r.ParseMultipartForm(32 << 20) // 32 MB max memory
	if err != nil {
		logger.Log.WithFields(map[string]interface{}{
			"correlation_id": correlationID,
			"error":          err.Error(),
		}).Error("Form parsing failed")
		return nil, fmt.Errorf("failed to parse multipart form: %w", err)
	}

	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		logger.Log.WithFields(map[string]interface{}{
			"correlation_id": correlationID,
			"error":          err.Error(),
		}).Error("File upload validation failed")
		return nil, fmt.Errorf("no file uploaded or invalid file: %w", err)
	}
	defer file.Close()

	return &services.UploadTranscriptRequest{
		File: fileHeader,
	}, nil
}

// handleServiceError determines error type and status code for service errors
func (h *TranscriptHandler) handleServiceError(err error) (int, string) {
	if utils.Contains(err.Error(), "duplicate") {
		return http.StatusConflict, "DUPLICATE_TRANSCRIPT"
	}
	return http.StatusBadRequest, "FILE_VALIDATION_ERROR"
}

// logUploadRequest logs the start of an upload request
func (h *TranscriptHandler) logUploadRequest(r *http.Request, correlationID string) {
	logger.Log.WithFields(map[string]interface{}{
		"correlation_id": correlationID,
		"client_ip":      utils.GetClientIP(r),
		"user_agent":     r.UserAgent(),
	}).Info("Upload transcript request received")
}

// logUploadSuccess logs successful upload completion
func (h *TranscriptHandler) logUploadSuccess(response *services.UploadTranscriptResponse, correlationID string) {
	logger.Log.WithFields(map[string]interface{}{
		"correlation_id":  correlationID,
		"transcript_id":   response.TranscriptID,
		"filename":        response.Filename,
		"word_count":      response.WordCount,
	}).Info("Upload completed successfully")
}

// UploadTranscript handles file upload
func (h *TranscriptHandler) UploadTranscript(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	utils.SetCORSHeaders(w)
	
	// Only handle POST and multipart uploads
	if r.Method != http.MethodPost {
		if matched, _ := utils.MatchPath(r.URL.Path, "/api/transcripts/"); matched {
			utils.WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
			return
		}
		// If not a transcript upload path, skip
		return
	}

	correlationID := utils.GetCorrelationID(r)
	h.logUploadRequest(r, correlationID)

	// Validate upload request
	req, err := h.validateUploadRequest(r, correlationID)
	if err != nil {
		utils.WriteErrorWithCorrelation(w, http.StatusBadRequest, "FORM_PARSE_ERROR", err.Error(), correlationID)
		return
	}

	logger.Log.WithFields(map[string]interface{}{
		"correlation_id": correlationID,
		"filename":       req.File.Filename,
		"file_size":      req.File.Size,
	}).Info("Processing uploaded file")

	// Process upload through service
	response, err := h.transcriptService.UploadTranscript(req, correlationID)
	if err != nil {
		statusCode, errorCode := h.handleServiceError(err)

		logger.LogErrorWithStackAndCorrelation(err, correlationID, map[string]interface{}{
			"error_code":  errorCode,
			"status_code": statusCode,
			"filename":    req.File.Filename,
			"file_size":   req.File.Size,
			"operation":   "upload_transcript",
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

	h.logUploadSuccess(response, correlationID)
	utils.WriteJSON(w, http.StatusOK, response)
}

// GetTranscripts returns paginated list of transcripts
func (h *TranscriptHandler) GetTranscripts(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	utils.SetCORSHeaders(w)
	
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

	transcripts, total, err := h.transcriptService.GetTranscripts(page, perPage)
	if err != nil {
		logger.LogErrorWithStack(err, map[string]interface{}{
			"operation": "get_transcripts",
			"page":      page,
			"per_page":  perPage,
		})
		utils.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve transcripts")
		return
	}

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"transcripts": transcripts,
		"total":       total,
		"page":        page,
		"per_page":    perPage,
	})
}

// GetTranscript returns a single transcript
func (h *TranscriptHandler) GetTranscript(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	utils.SetCORSHeaders(w)
	
	if r.Method != http.MethodGet {
		utils.WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	// Extract ID from path like /api/transcripts/123
	idStr, err := utils.ExtractIDFromPath(r.URL.Path, "/api/transcripts/")
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "INVALID_PATH", "Invalid transcript path")
		return
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, "INVALID_UUID", "Invalid transcript ID format")
		return
	}

	transcript, err := h.transcriptService.GetTranscript(id)
	if err != nil {
		statusCode := http.StatusNotFound
		errorCode := "TRANSCRIPT_NOT_FOUND"

		if !utils.Contains(err.Error(), "not found") {
			statusCode = http.StatusInternalServerError
			errorCode = "INTERNAL_ERROR"
		}

		logger.LogErrorWithStack(err, map[string]interface{}{
			"transcript_id": id,
			"error_code":    errorCode,
			"status_code":   statusCode,
			"operation":     "get_transcript",
		})

		utils.WriteError(w, statusCode, errorCode, err.Error())
		return
	}

	utils.WriteJSON(w, http.StatusOK, transcript)
}

// DeleteTranscript deletes a transcript
func (h *TranscriptHandler) DeleteTranscript(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers
	utils.SetCORSHeaders(w)
	
	if r.Method != http.MethodDelete {
		utils.WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	correlationID := utils.GetCorrelationID(r)
	
	// Extract ID from path
	idStr, err := utils.ExtractIDFromPath(r.URL.Path, "/api/transcripts/")
	if err != nil {
		utils.WriteErrorWithCorrelation(w, http.StatusBadRequest, "INVALID_PATH", "Invalid transcript path", correlationID)
		return
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		utils.WriteErrorWithCorrelation(w, http.StatusBadRequest, "INVALID_UUID", "Invalid transcript ID format", correlationID)
		return
	}

	if err := h.transcriptService.DeleteTranscript(id, correlationID); err != nil {
		statusCode := http.StatusNotFound
		errorCode := "TRANSCRIPT_NOT_FOUND"

		if !utils.Contains(err.Error(), "not found") {
			statusCode = http.StatusInternalServerError
			errorCode = "INTERNAL_ERROR"
		}

		logger.LogErrorWithStackAndCorrelation(err, correlationID, map[string]interface{}{
			"transcript_id": id,
			"error_code":    errorCode,
			"status_code":   statusCode,
			"operation":     "delete_transcript",
		})

		utils.WriteErrorWithCorrelation(w, statusCode, errorCode, err.Error(), correlationID)
		return
	}

	utils.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Transcript deleted successfully",
	})
}