package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"backend-golang/internal/config"
	"backend-golang/internal/models"
	"backend-golang/pkg/logger"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TranscriptServiceInterface defines the interface for transcript service operations
type TranscriptServiceInterface interface {
	UploadTranscript(req *UploadTranscriptRequest, correlationID string) (*UploadTranscriptResponse, error)
	GetTranscripts(page, perPage int) ([]*models.Transcript, int64, error)
	GetTranscript(id uuid.UUID) (*models.Transcript, error)
	DeleteTranscript(id uuid.UUID, correlationID string) error
	ReadTranscriptContent(transcript *models.Transcript) (string, error)
}

type TranscriptService struct {
	db     *gorm.DB
	config *config.Config
}

func NewTranscriptService(db *gorm.DB, cfg *config.Config) *TranscriptService {
	return &TranscriptService{
		db:     db,
		config: cfg,
	}
}

// UploadTranscriptRequest represents the upload request
type UploadTranscriptRequest struct {
	File *multipart.FileHeader
}

// UploadTranscriptResponse represents the upload response
type UploadTranscriptResponse struct {
	TranscriptID uuid.UUID `json:"transcript_id"`
	Filename     string    `json:"filename"`
	WordCount    int       `json:"word_count"`
	Message      string    `json:"message"`
}

// UploadTranscript handles file upload and validation
func (s *TranscriptService) UploadTranscript(req *UploadTranscriptRequest, correlationID string) (*UploadTranscriptResponse, error) {
	log := logger.WithCorrelationID(correlationID)

	// Validate file extension
	ext := strings.ToLower(filepath.Ext(req.File.Filename))
	isValidExt := false
	for _, allowedExt := range s.config.AllowedExts {
		if ext == allowedExt {
			isValidExt = true
			break
		}
	}
	if !isValidExt {
		return nil, fmt.Errorf("invalid file extension: %s. Allowed: %v", ext, s.config.AllowedExts)
	}

	// Validate file size
	if req.File.Size > s.config.MaxFileSize {
		return nil, fmt.Errorf("file too large: %d bytes. Maximum: %d bytes", req.File.Size, s.config.MaxFileSize)
	}

	// Open and read file
	file, err := req.File.Open()
	if err != nil {
		logger.LogErrorWithStackAndCorrelation(err, correlationID, map[string]interface{}{
			"filename":  req.File.Filename,
			"operation": "open_upload_file",
		})
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		logger.LogErrorWithStackAndCorrelation(err, correlationID, map[string]interface{}{
			"filename":  req.File.Filename,
			"operation": "read_file_content",
		})
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Validate UTF-8 encoding
	if !isValidUTF8(content) {
		return nil, fmt.Errorf("file must be UTF-8 encoded")
	}

	// Calculate content hash
	hash := sha256.Sum256(content)
	contentHash := hex.EncodeToString(hash[:])

	// Check for duplicates
	var existingTranscript models.Transcript
	if err := s.db.Where("content_hash = ?", contentHash).First(&existingTranscript).Error; err == nil {
		log.WithField("existing_id", existingTranscript.ID).Info("Duplicate transcript detected")
		return nil, fmt.Errorf("duplicate transcript already exists with ID: %s", existingTranscript.ID)
	}

	// Parse content and calculate word count
	wordCount, metadata, err := s.parseTranscriptContent(content, ext)
	if err != nil {
		logger.LogErrorWithStackAndCorrelation(err, correlationID, map[string]interface{}{
			"filename":   req.File.Filename,
			"extension":  ext,
			"operation":  "parse_transcript_content",
		})
		return nil, fmt.Errorf("failed to parse transcript: %w", err)
	}

	// Create transcript record
	transcript := &models.Transcript{
		ID:                 uuid.New(),
		Filename:           req.File.Filename,
		ContentHash:        contentHash,
		WordCount:          wordCount,
		TranscriptMetadata: metadata,
		UploadedAt:         time.Now(),
	}

	// Save file to storage
	filePath, err := s.saveFile(transcript.ID, content)
	if err != nil {
		logger.LogErrorWithStackAndCorrelation(err, correlationID, map[string]interface{}{
			"transcript_id": transcript.ID,
			"filename":      req.File.Filename,
			"operation":     "save_file",
		})
		return nil, fmt.Errorf("failed to save file: %w", err)
	}
	transcript.FilePath = filePath

	// Save to database
	if err := s.db.Create(transcript).Error; err != nil {
		// Clean up file if database save fails
		_ = os.Remove(filePath)
		logger.LogErrorWithStackAndCorrelation(err, correlationID, map[string]interface{}{
			"transcript_id": transcript.ID,
			"filename":      req.File.Filename,
			"operation":     "save_transcript_to_database",
		})
		return nil, fmt.Errorf("failed to save transcript to database: %w", err)
	}

	log.WithFields(map[string]interface{}{
		"transcript_id": transcript.ID,
		"filename":      transcript.Filename,
		"word_count":    transcript.WordCount,
		"file_size":     req.File.Size,
	}).Info("Transcript uploaded successfully")

	return &UploadTranscriptResponse{
		TranscriptID: transcript.ID,
		Filename:     transcript.Filename,
		WordCount:    transcript.WordCount,
		Message:      "Transcript uploaded successfully",
	}, nil
}

// GetTranscripts returns paginated list of transcripts
func (s *TranscriptService) GetTranscripts(page, perPage int) ([]*models.Transcript, int64, error) {
	var transcripts []*models.Transcript
	var total int64

	offset := (page - 1) * perPage

	// Count total
	if err := s.db.Model(&models.Transcript{}).Count(&total).Error; err != nil {
		logger.LogErrorWithStack(err, map[string]interface{}{
			"operation": "count_transcripts",
			"page":      page,
			"per_page":  perPage,
		})
		return nil, 0, fmt.Errorf("failed to count transcripts: %w", err)
	}

	// Get paginated results
	if err := s.db.Offset(offset).Limit(perPage).Order("uploaded_at DESC").Find(&transcripts).Error; err != nil {
		logger.LogErrorWithStack(err, map[string]interface{}{
			"operation": "get_transcripts_list",
			"page":      page,
			"per_page":  perPage,
			"offset":    offset,
		})
		return nil, 0, fmt.Errorf("failed to get transcripts: %w", err)
	}

	return transcripts, total, nil
}

// GetTranscript returns a single transcript by ID
func (s *TranscriptService) GetTranscript(id uuid.UUID) (*models.Transcript, error) {
	var transcript models.Transcript
	if err := s.db.Where("id = ?", id).First(&transcript).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("transcript not found")
		}
		logger.LogErrorWithStack(err, map[string]interface{}{
			"transcript_id": id,
			"operation":     "get_transcript_by_id",
		})
		return nil, fmt.Errorf("failed to get transcript: %w", err)
	}
	return &transcript, nil
}

// DeleteTranscript deletes a transcript and its file
func (s *TranscriptService) DeleteTranscript(id uuid.UUID, correlationID string) error {
	log := logger.WithCorrelationID(correlationID)

	var transcript models.Transcript
	if err := s.db.Where("id = ?", id).First(&transcript).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("transcript not found")
		}
		logger.LogErrorWithStackAndCorrelation(err, correlationID, map[string]interface{}{
			"transcript_id": id,
			"operation":     "find_transcript_for_delete",
		})
		return fmt.Errorf("failed to find transcript: %w", err)
	}

	// Delete file
	if err := os.Remove(transcript.FilePath); err != nil && !os.IsNotExist(err) {
		log.WithError(err).Warn("Failed to delete transcript file")
	}

	// Delete from database (cascade deletes analyses and fact checks)
	if err := s.db.Delete(&transcript).Error; err != nil {
		logger.LogErrorWithStackAndCorrelation(err, correlationID, map[string]interface{}{
			"transcript_id": id,
			"operation":     "delete_transcript_from_database",
		})
		return fmt.Errorf("failed to delete transcript from database: %w", err)
	}

	log.WithField("transcript_id", id).Info("Transcript deleted successfully")
	return nil
}

// Helper functions

func (s *TranscriptService) saveFile(transcriptID uuid.UUID, content []byte) (string, error) {
	// Ensure storage directory exists
	if err := os.MkdirAll(s.config.StoragePath, 0755); err != nil {
		logger.LogErrorWithStack(err, map[string]interface{}{
			"storage_path": s.config.StoragePath,
			"operation":    "create_storage_directory",
		})
		return "", fmt.Errorf("failed to create storage directory: %w", err)
	}

	filename := transcriptID.String() + ".txt"
	filePath := filepath.Join(s.config.StoragePath, filename)

	if err := os.WriteFile(filePath, content, 0644); err != nil {
		logger.LogErrorWithStack(err, map[string]interface{}{
			"file_path":     filePath,
			"transcript_id": transcriptID,
			"operation":     "write_file",
		})
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return filePath, nil
}

func (s *TranscriptService) parseTranscriptContent(content []byte, ext string) (int, []byte, error) {
	var wordCount int
	var metadata map[string]interface{}

	if ext == ".json" {
		var jsonData map[string]interface{}
		if err := json.Unmarshal(content, &jsonData); err != nil {
			logger.LogErrorWithStack(err, map[string]interface{}{
				"operation": "unmarshal_json_transcript",
			})
			return 0, nil, fmt.Errorf("invalid JSON format: %w", err)
		}

		// Extract metadata
		metadata = make(map[string]interface{})
		for key, value := range jsonData {
			if key != "transcript" {
				metadata[key] = value
			}
		}

		// Count words in transcript array or text field
		if transcript, ok := jsonData["transcript"]; ok {
			if transcriptArray, ok := transcript.([]interface{}); ok {
				// Array format: [{"text": "...", "speaker": "...", "timestamp": "..."}, ...]
				for _, item := range transcriptArray {
					if itemMap, ok := item.(map[string]interface{}); ok {
						if text, ok := itemMap["text"].(string); ok {
							wordCount += countWords(text)
						}
					}
				}
			} else if transcriptText, ok := transcript.(string); ok {
				// String format
				wordCount = countWords(transcriptText)
			}
		}
	} else {
		// Plain text format
		wordCount = countWords(string(content))
	}

	metadataBytes, _ := json.Marshal(metadata)
	return wordCount, metadataBytes, nil
}

func countWords(text string) int {
	words := strings.Fields(strings.TrimSpace(text))
	return len(words)
}

// ReadTranscriptContent reads the content of a transcript file (matches Python async def read_transcript_content)
func (s *TranscriptService) ReadTranscriptContent(transcript *models.Transcript) (string, error) {
	if _, err := os.Stat(transcript.FilePath); os.IsNotExist(err) {
		logger.Log.WithFields(map[string]interface{}{
			"transcript_id": transcript.ID,
			"file_path": transcript.FilePath,
		}).Error("Transcript file not found")
		return "", fmt.Errorf("transcript file not found: %s", transcript.FilePath)
	}

	content, err := os.ReadFile(transcript.FilePath)
	if err != nil {
		logger.LogErrorWithStack(err, map[string]interface{}{
			"transcript_id": transcript.ID,
			"file_path":     transcript.FilePath,
			"operation":     "read_transcript_file",
		})
		return "", fmt.Errorf("failed to read transcript file: %w", err)
	}

	logger.Log.WithFields(map[string]interface{}{
		"transcript_id": transcript.ID,
		"content_length": len(content),
	}).Info("Read transcript content")

	return string(content), nil
}

func isValidUTF8(data []byte) bool {
	return strings.ToValidUTF8(string(data), "") == string(data)
}