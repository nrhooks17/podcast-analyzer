package handlers

import (
	"net/http"
	"backend-golang/internal/services"
	"backend-golang/pkg/logger"
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
func (h *TranscriptHandler) UploadTranscript(w http.ResponseWriter, r *http.Request) {
	// Only handle POST and multipart uploads
	if r.Method != http.MethodPost {
		if matched, _ := matchPath(r.URL.Path, "/api/transcripts/"); matched {
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
			return
		}
		// If not a transcript upload path, skip
		return
	}

	correlationID := getCorrelationID(r)
	
	// Log request start
	logger.Log.WithFields(map[string]interface{}{
		"correlation_id": correlationID,
		"client_ip":      getClientIP(r),
		"user_agent":     r.UserAgent(),
	}).Info("Upload transcript request received")

	// Parse multipart form
	err := r.ParseMultipartForm(32 << 20) // 32 MB max memory
	if err != nil {
		errMsg := "Failed to parse multipart form"
		logger.Log.WithFields(map[string]interface{}{
			"correlation_id": correlationID,
			"error":          err.Error(),
		}).Error("Form parsing failed")
		
		writeErrorWithCorrelation(w, http.StatusBadRequest, "FORM_PARSE_ERROR", errMsg, correlationID)
		return
	}

	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		errMsg := "No file uploaded or invalid file"
		logger.Log.WithFields(map[string]interface{}{
			"correlation_id": correlationID,
			"error":          err.Error(),
		}).Error("File upload validation failed")
		
		writeErrorWithCorrelation(w, http.StatusBadRequest, "FILE_VALIDATION_ERROR", errMsg, correlationID)
		return
	}
	defer file.Close()

	logger.Log.WithFields(map[string]interface{}{
		"correlation_id": correlationID,
		"filename":       fileHeader.Filename,
		"file_size":      fileHeader.Size,
	}).Info("Processing uploaded file")

	req := &services.UploadTranscriptRequest{
		File: fileHeader,
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
			"filename":    fileHeader.Filename,
			"file_size":   fileHeader.Size,
			"operation":   "upload_transcript",
		})

		writeJSON(w, statusCode, map[string]interface{}{
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

	writeJSON(w, http.StatusOK, response)
}

// GetTranscripts returns paginated list of transcripts
func (h *TranscriptHandler) GetTranscripts(w http.ResponseWriter, r *http.Request) {
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

	transcripts, total, err := h.transcriptService.GetTranscripts(page, perPage)
	if err != nil {
		logger.LogErrorWithStack(err, map[string]interface{}{
			"operation": "get_transcripts",
			"page":      page,
			"per_page":  perPage,
		})
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to retrieve transcripts")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"transcripts": transcripts,
		"total":       total,
		"page":        page,
		"per_page":    perPage,
	})
}

// GetTranscript returns a single transcript
func (h *TranscriptHandler) GetTranscript(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	// Extract ID from path like /api/transcripts/123
	idStr, err := extractIDFromPath(r.URL.Path, "/api/transcripts/")
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PATH", "Invalid transcript path")
		return
	}

	id, err := parseUUIDParam(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_UUID", "Invalid transcript ID format")
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

		writeError(w, statusCode, errorCode, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, transcript)
}

// DeleteTranscript deletes a transcript
func (h *TranscriptHandler) DeleteTranscript(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	correlationID := getCorrelationID(r)
	
	// Extract ID from path
	idStr, err := extractIDFromPath(r.URL.Path, "/api/transcripts/")
	if err != nil {
		writeErrorWithCorrelation(w, http.StatusBadRequest, "INVALID_PATH", "Invalid transcript path", correlationID)
		return
	}

	id, err := parseUUIDParam(idStr)
	if err != nil {
		writeErrorWithCorrelation(w, http.StatusBadRequest, "INVALID_UUID", "Invalid transcript ID format", correlationID)
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

		writeErrorWithCorrelation(w, statusCode, errorCode, err.Error(), correlationID)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Transcript deleted successfully",
	})
}