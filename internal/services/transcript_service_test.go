package services

import (
	"podcast-analyzer/internal/config"
	"podcast-analyzer/internal/models"
	"bytes"
	"mime/multipart"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	
	// Create tables manually to avoid PostgreSQL-specific syntax
	err = db.Exec(`
		CREATE TABLE transcripts (
			id TEXT PRIMARY KEY,
			filename TEXT NOT NULL,
			file_path TEXT NOT NULL,
			content_hash TEXT NOT NULL UNIQUE,
			word_count INTEGER NOT NULL,
			uploaded_at DATETIME,
			transcript_metadata TEXT
		)
	`).Error
	require.NoError(t, err)
	
	err = db.Exec(`
		CREATE TABLE analysis_results (
			id TEXT PRIMARY KEY,
			transcript_id TEXT NOT NULL,
			job_id TEXT NOT NULL UNIQUE,
			status TEXT NOT NULL DEFAULT 'pending',
			summary TEXT,
			takeaways TEXT,
			created_at DATETIME,
			completed_at DATETIME,
			error_message TEXT
		)
	`).Error
	require.NoError(t, err)
	
	err = db.Exec(`
		CREATE TABLE fact_checks (
			id TEXT PRIMARY KEY,
			analysis_id TEXT NOT NULL,
			claim TEXT NOT NULL,
			verdict TEXT NOT NULL,
			confidence REAL NOT NULL,
			evidence TEXT,
			sources TEXT,
			checked_at DATETIME
		)
	`).Error
	require.NoError(t, err)
	
	return db
}

func setupTestConfig(t *testing.T) *config.Config {
	tempDir := t.TempDir()
	return &config.Config{
		StoragePath:   tempDir,
		MaxFileSize:   10 * 1024 * 1024, // 10MB
		AllowedExts:   []string{".txt", ".json"},
		DatabaseURL:   "sqlite://:memory:",
		ServerPort:    "8000",
		LogLevel:      "DEBUG",
		AnthropicAPIKey: "test-key",
		SerperAPIKey:    "test-key",
	}
}

func createTestFileHeader(t *testing.T, filename, content string) *multipart.FileHeader {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	
	part, err := writer.CreateFormFile("file", filename)
	require.NoError(t, err)
	
	_, err = part.Write([]byte(content))
	require.NoError(t, err)
	
	err = writer.Close()
	require.NoError(t, err)
	
	// Parse the form to get the file header
	req := multipart.NewReader(body, writer.Boundary())
	form, err := req.ReadForm(1024)
	require.NoError(t, err)
	
	return form.File["file"][0]
}

func TestTranscriptService_UploadTranscript(t *testing.T) {
	db := setupTestDB(t)
	cfg := setupTestConfig(t)
	service := NewTranscriptService(db, cfg)

	tests := []struct {
		name        string
		filename    string
		content     string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid text file",
			filename:    "test.txt",
			content:     "[00:00:00] Host: Welcome to the show.\n[00:00:15] Guest: Thanks for having me.",
			expectError: false,
		},
		{
			name:        "valid json file",
			filename:    "test.json",
			content:     `{"transcript": [{"text": "Hello world", "speaker": "Host", "timestamp": "00:00:00"}]}`,
			expectError: false,
		},
		{
			name:        "invalid extension",
			filename:    "test.pdf",
			content:     "test content",
			expectError: true,
			errorMsg:    "invalid file extension",
		},
		{
			name:        "invalid json",
			filename:    "test.json",
			content:     `{"invalid": json}`,
			expectError: true,
			errorMsg:    "invalid JSON format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fileHeader := createTestFileHeader(t, tt.filename, tt.content)
			req := &UploadTranscriptRequest{File: fileHeader}
			
			resp, err := service.UploadTranscript(req, "test-correlation-id")
			
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, tt.filename, resp.Filename)
				assert.True(t, resp.WordCount > 0)
				assert.NotEqual(t, uuid.Nil, resp.TranscriptID)
				assert.Equal(t, "Transcript uploaded successfully", resp.Message)
			}
		})
	}
}

func TestTranscriptService_UploadTranscript_DuplicateDetection(t *testing.T) {
	db := setupTestDB(t)
	cfg := setupTestConfig(t)
	service := NewTranscriptService(db, cfg)

	content := "This is test content for duplicate detection."
	fileHeader := createTestFileHeader(t, "test.txt", content)
	req := &UploadTranscriptRequest{File: fileHeader}

	// First upload should succeed
	resp1, err := service.UploadTranscript(req, "test-correlation-id")
	assert.NoError(t, err)
	assert.NotNil(t, resp1)

	// Second upload with same content should fail
	fileHeader2 := createTestFileHeader(t, "test2.txt", content)
	req2 := &UploadTranscriptRequest{File: fileHeader2}
	
	resp2, err := service.UploadTranscript(req2, "test-correlation-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate transcript already exists")
	assert.Nil(t, resp2)
}

func TestTranscriptService_UploadTranscript_FileTooLarge(t *testing.T) {
	db := setupTestDB(t)
	cfg := setupTestConfig(t)
	cfg.MaxFileSize = 100 // 100 bytes
	service := NewTranscriptService(db, cfg)

	largeContent := string(make([]byte, 200)) // 200 bytes
	fileHeader := createTestFileHeader(t, "large.txt", largeContent)
	req := &UploadTranscriptRequest{File: fileHeader}

	resp, err := service.UploadTranscript(req, "test-correlation-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "file too large")
	assert.Nil(t, resp)
}

func TestTranscriptService_GetTranscripts(t *testing.T) {
	db := setupTestDB(t)
	cfg := setupTestConfig(t)
	service := NewTranscriptService(db, cfg)

	// Create test transcripts with known UUIDs
	now := time.Now()
	id1 := uuid.New()
	id2 := uuid.New() 
	id3 := uuid.New()
	
	transcriptData := []map[string]interface{}{
		{
			"id":          id1.String(),
			"filename":    "test1.txt",
			"file_path":   "/tmp/test1.txt",
			"content_hash": "hash1",
			"word_count":  100,
			"uploaded_at": now.Add(-2 * time.Hour),
		},
		{
			"id":          id2.String(),
			"filename":    "test2.txt", 
			"file_path":   "/tmp/test2.txt",
			"content_hash": "hash2",
			"word_count":  200,
			"uploaded_at": now.Add(-1 * time.Hour),
		},
		{
			"id":          id3.String(),
			"filename":    "test3.txt",
			"file_path":   "/tmp/test3.txt", 
			"content_hash": "hash3",
			"word_count":  300,
			"uploaded_at": now,
		},
	}

	for _, data := range transcriptData {
		err := db.Table("transcripts").Create(data).Error
		require.NoError(t, err)
	}

	// Test pagination
	page1Transcripts, total, err := service.GetTranscripts(1, 2)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, page1Transcripts, 2)

	// Should be ordered by uploaded_at DESC (newest first)
	assert.Equal(t, id3, page1Transcripts[0].ID)  // newest (test3)
	assert.Equal(t, id2, page1Transcripts[1].ID)  // middle (test2)

	page2Transcripts, total2, err := service.GetTranscripts(2, 2)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), total2)
	assert.Len(t, page2Transcripts, 1)
	assert.Equal(t, id1, page2Transcripts[0].ID)   // oldest (test1)
}

func TestTranscriptService_GetTranscript(t *testing.T) {
	db := setupTestDB(t)
	cfg := setupTestConfig(t)
	service := NewTranscriptService(db, cfg)

	// Create test transcript
	testTranscript := &models.Transcript{
		ID:          uuid.New(),
		Filename:    "test.txt",
		ContentHash: "testhash",
		WordCount:   150,
		UploadedAt:  time.Now(),
	}
	err := db.Create(testTranscript).Error
	require.NoError(t, err)

	// Test getting existing transcript
	transcript, err := service.GetTranscript(testTranscript.ID)
	assert.NoError(t, err)
	assert.NotNil(t, transcript)
	assert.Equal(t, testTranscript.ID, transcript.ID)
	assert.Equal(t, testTranscript.Filename, transcript.Filename)
	assert.Equal(t, testTranscript.WordCount, transcript.WordCount)

	// Test getting non-existent transcript
	nonExistentID := uuid.New()
	transcript, err = service.GetTranscript(nonExistentID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "transcript not found")
	assert.Nil(t, transcript)
}

func TestTranscriptService_DeleteTranscript(t *testing.T) {
	db := setupTestDB(t)
	cfg := setupTestConfig(t)
	service := NewTranscriptService(db, cfg)

	// Create test file
	testContent := "Test transcript content for deletion"
	tempFile := filepath.Join(cfg.StoragePath, "test-file.txt")
	err := os.WriteFile(tempFile, []byte(testContent), 0644)
	require.NoError(t, err)

	// Create test transcript
	testTranscript := &models.Transcript{
		ID:          uuid.New(),
		Filename:    "test.txt",
		ContentHash: "testhash",
		WordCount:   150,
		FilePath:    tempFile,
		UploadedAt:  time.Now(),
	}
	err = db.Create(testTranscript).Error
	require.NoError(t, err)

	// Verify file exists
	assert.FileExists(t, tempFile)

	// Delete transcript
	err = service.DeleteTranscript(testTranscript.ID, "test-correlation-id")
	assert.NoError(t, err)

	// Verify transcript is deleted from database
	var count int64
	err = db.Model(&models.Transcript{}).Where("id = ?", testTranscript.ID).Count(&count).Error
	assert.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// Verify file is deleted
	assert.NoFileExists(t, tempFile)

	// Test deleting non-existent transcript
	nonExistentID := uuid.New()
	err = service.DeleteTranscript(nonExistentID, "test-correlation-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "transcript not found")
}

func TestParseTranscriptContent(t *testing.T) {
	cfg := setupTestConfig(t)
	service := &TranscriptService{config: cfg}

	tests := []struct {
		name            string
		content         string
		ext             string
		expectedWords   int
		expectError     bool
	}{
		{
			name:          "plain text",
			content:       "Hello world this is a test",
			ext:           ".txt",
			expectedWords: 6,
			expectError:   false,
		},
		{
			name:          "json with transcript array",
			content:       `{"transcript": [{"text": "Hello world", "speaker": "Host"}, {"text": "How are you", "speaker": "Guest"}]}`,
			ext:           ".json",
			expectedWords: 5,
			expectError:   false,
		},
		{
			name:          "json with transcript string",
			content:       `{"transcript": "Hello world test string"}`,
			ext:           ".json",
			expectedWords: 4,
			expectError:   false,
		},
		{
			name:        "invalid json",
			content:     `{"invalid": json}`,
			ext:         ".json",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wordCount, metadata, err := service.parseTranscriptContent([]byte(tt.content), tt.ext)
			
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedWords, wordCount)
				assert.NotNil(t, metadata)
			}
		})
	}
}

func TestCountWords(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"", 0},
		{"hello", 1},
		{"hello world", 2},
		{"  hello   world  ", 2},
		{"hello\nworld\ttest", 3},
		{"hello, world! how are you?", 5},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := countWords(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsValidUTF8(t *testing.T) {
	tests := []struct {
		input    []byte
		expected bool
	}{
		{[]byte("hello world"), true},
		{[]byte("héllo wörld"), true},
		{[]byte("rocket"), true},
		{[]byte{0xff, 0xfe}, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			result := isValidUTF8(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}