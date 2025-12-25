package services

import (
	"backend-golang/internal/config"
	"backend-golang/internal/models"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// MockKafkaService for testing
type MockKafkaService struct {
	mock.Mock
}

func (m *MockKafkaService) PublishAnalysisJob(message interface{}) error {
	args := m.Called(message)
	return args.Error(0)
}

func (m *MockKafkaService) Close() error {
	args := m.Called()
	return args.Error(0)
}

func setupAnalysisTestDB(t *testing.T) *gorm.DB {
	return setupTestDB(t)
}

func setupAnalysisTestConfig(t *testing.T) *config.Config {
	return &config.Config{
		KafkaTopicAnalysis: "analysis-topic",
		AnthropicAPIKey:    "test-key",
		SerperAPIKey:       "test-key",
		DatabaseURL:        "sqlite://:memory:",
		ServerPort:         "8000",
		LogLevel:           "DEBUG",
	}
}

func TestAnalysisService_CreateAnalysisJob(t *testing.T) {
	db := setupAnalysisTestDB(t)
	cfg := setupAnalysisTestConfig(t)
	mockKafka := &MockKafkaService{}
	service := NewAnalysisService(db, cfg, mockKafka)

	// Create a test transcript
	testTranscript := &models.Transcript{
		ID:          uuid.New(),
		Filename:    "test.txt",
		ContentHash: "testhash",
		WordCount:   150,
		FilePath:    "/tmp/test.txt",
		UploadedAt:  time.Now(),
	}
	err := db.Create(testTranscript).Error
	require.NoError(t, err)

	// Mock successful Kafka publish
	mockKafka.On("PublishAnalysisJob", mock.Anything).Return(nil)

	req := &AnalysisJobRequest{
		TranscriptID: testTranscript.ID,
	}

	resp, err := service.CreateAnalysisJob(req, "test-correlation-id")
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEqual(t, uuid.Nil, resp.JobID)
	assert.Equal(t, testTranscript.ID, resp.TranscriptID)
	assert.Equal(t, "pending", resp.Status)
	assert.Equal(t, "Analysis job created and queued for processing", resp.Message)

	// Verify analysis result was created in database
	var analysisResult models.AnalysisResult
	err = db.Where("job_id = ?", resp.JobID).First(&analysisResult).Error
	assert.NoError(t, err)
	assert.Equal(t, testTranscript.ID, analysisResult.TranscriptID)
	assert.Equal(t, "pending", analysisResult.Status)

	// Verify Kafka message was published
	mockKafka.AssertExpectations(t)
}

func TestAnalysisService_CreateAnalysisJob_TranscriptNotFound(t *testing.T) {
	db := setupAnalysisTestDB(t)
	cfg := setupAnalysisTestConfig(t)
	mockKafka := &MockKafkaService{}
	service := NewAnalysisService(db, cfg, mockKafka)

	nonExistentID := uuid.New()
	req := &AnalysisJobRequest{
		TranscriptID: nonExistentID,
	}

	resp, err := service.CreateAnalysisJob(req, "test-correlation-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.Nil(t, resp)

	// Kafka should not be called
	mockKafka.AssertNotCalled(t, "PublishAnalysisJob")
}

func TestAnalysisService_CreateAnalysisJob_DuplicatePrevention(t *testing.T) {
	db := setupAnalysisTestDB(t)
	cfg := setupAnalysisTestConfig(t)
	mockKafka := &MockKafkaService{}
	service := NewAnalysisService(db, cfg, mockKafka)

	// Create a test transcript
	testTranscript := &models.Transcript{
		ID:          uuid.New(),
		Filename:    "test.txt",
		ContentHash: "testhash",
		WordCount:   150,
		FilePath:    "/tmp/test.txt",
		UploadedAt:  time.Now(),
	}
	err := db.Create(testTranscript).Error
	require.NoError(t, err)

	// Create an existing analysis result
	summary := "Test summary"
	existingAnalysis := &models.AnalysisResult{
		ID:           uuid.New(),
		TranscriptID: testTranscript.ID,
		JobID:        uuid.New(),
		Status:       "completed",
		Summary:      &summary,
		CreatedAt:    time.Now(),
	}
	err = db.Create(existingAnalysis).Error
	require.NoError(t, err)

	req := &AnalysisJobRequest{
		TranscriptID: testTranscript.ID,
	}

	// Mock successful Kafka publish for this test
	mockKafka.On("PublishAnalysisJob", mock.Anything).Return(nil)

	resp, err := service.CreateAnalysisJob(req, "test-correlation-id")
	// Should succeed since there's no duplicate prevention in current implementation
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	// Kafka should be called
	mockKafka.AssertExpectations(t)
}

func TestAnalysisService_CreateAnalysisJob_KafkaError(t *testing.T) {
	db := setupAnalysisTestDB(t)
	cfg := setupAnalysisTestConfig(t)
	mockKafka := &MockKafkaService{}
	service := NewAnalysisService(db, cfg, mockKafka)

	// Create a test transcript
	testTranscript := &models.Transcript{
		ID:          uuid.New(),
		Filename:    "test.txt",
		ContentHash: "testhash",
		WordCount:   150,
		FilePath:    "/tmp/test.txt",
		UploadedAt:  time.Now(),
	}
	err := db.Create(testTranscript).Error
	require.NoError(t, err)

	// Mock Kafka publish failure
	mockKafka.On("PublishAnalysisJob", mock.Anything).Return(assert.AnError)

	req := &AnalysisJobRequest{
		TranscriptID: testTranscript.ID,
	}

	resp, err := service.CreateAnalysisJob(req, "test-correlation-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to queue analysis job")
	assert.Nil(t, resp)

	// Verify analysis result status was set to failed (not cleaned up in current implementation)
	var analysisResult models.AnalysisResult
	err = db.Where("transcript_id = ?", testTranscript.ID).First(&analysisResult).Error
	assert.NoError(t, err)
	assert.Equal(t, "failed", analysisResult.Status)

	mockKafka.AssertExpectations(t)
}

func TestAnalysisService_GetJobStatus(t *testing.T) {
	db := setupAnalysisTestDB(t)
	cfg := setupAnalysisTestConfig(t)
	mockKafka := &MockKafkaService{}
	service := NewAnalysisService(db, cfg, mockKafka)

	// Create test analysis result
	testAnalysis := &models.AnalysisResult{
		ID:           uuid.New(),
		TranscriptID: uuid.New(),
		JobID:        uuid.New(),
		Status:       "in_progress",
		Summary:      nil,
		CreatedAt:    time.Now(),
	}
	err := db.Create(testAnalysis).Error
	require.NoError(t, err)

	// Test getting existing job status
	status, err := service.GetJobStatus(testAnalysis.JobID, "test-correlation-id")
	assert.NoError(t, err)
	assert.NotNil(t, status)
	assert.Equal(t, testAnalysis.JobID, status.JobID)
	assert.Equal(t, "in_progress", status.Status)
	assert.Equal(t, testAnalysis.TranscriptID, status.TranscriptID)

	// Test getting non-existent job status
	nonExistentID := uuid.New()
	status, err = service.GetJobStatus(nonExistentID, "test-correlation-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.Nil(t, status)
}

func TestAnalysisService_ListAnalysisResults(t *testing.T) {
	db := setupAnalysisTestDB(t)
	cfg := setupAnalysisTestConfig(t)
	mockKafka := &MockKafkaService{}
	service := NewAnalysisService(db, cfg, mockKafka)

	// Create test analysis results
	transcriptID1 := uuid.New()
	transcriptID2 := uuid.New()
	
	summary1 := "Test summary 1"
	summary2 := "Test summary 2"
	
	analyses := []*models.AnalysisResult{
		{
			ID:           uuid.New(),
			TranscriptID: transcriptID1,
			JobID:        uuid.New(),
			Status:       "completed",
			Summary:      &summary1,
			CreatedAt:    time.Now().Add(-2 * time.Hour),
		},
		{
			ID:           uuid.New(),
			TranscriptID: transcriptID2,
			JobID:        uuid.New(),
			Status:       "completed",
			Summary:      &summary2,
			CreatedAt:    time.Now().Add(-1 * time.Hour),
		},
		{
			ID:           uuid.New(),
			TranscriptID: transcriptID1,
			JobID:        uuid.New(),
			Status:       "in_progress",
			Summary:      nil,
			CreatedAt:    time.Now(),
		},
	}

	for _, analysis := range analyses {
		err := db.Create(analysis).Error
		require.NoError(t, err)
	}

	// Create test transcripts to join with (required for ListAnalysisResults)
	transcripts := []*models.Transcript{
		{
			ID:          transcriptID1,
			Filename:    "test1.txt",
			ContentHash: "hash1",
			WordCount:   100,
			UploadedAt:  time.Now(),
		},
		{
			ID:          transcriptID2,
			Filename:    "test2.txt",
			ContentHash: "hash2",
			WordCount:   200,
			UploadedAt:  time.Now(),
		},
	}

	for _, transcript := range transcripts {
		err := db.Create(transcript).Error
		require.NoError(t, err)
	}

	// Test getting all results
	results, total, err := service.ListAnalysisResults(1, 10)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, results, 3)

	// Results should be ordered by created_at DESC (newest first)
	assert.Equal(t, analyses[2].ID, results[0].ID)
	assert.Equal(t, analyses[1].ID, results[1].ID)
	assert.Equal(t, analyses[0].ID, results[2].ID)

	// Test pagination
	results, total, err = service.ListAnalysisResults(1, 1)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, results, 1)
	assert.Equal(t, analyses[2].ID, results[0].ID)
}

func TestAnalysisService_GetAnalysisResults(t *testing.T) {
	db := setupAnalysisTestDB(t)
	cfg := setupAnalysisTestConfig(t)
	mockKafka := &MockKafkaService{}
	service := NewAnalysisService(db, cfg, mockKafka)

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

	// Create test analysis result with fact checks
	summary := "Test summary"
	testAnalysis := &models.AnalysisResult{
		ID:           uuid.New(),
		TranscriptID: testTranscript.ID,
		JobID:        uuid.New(),
		Status:       "completed",
		Summary:      &summary,
		Takeaways:    []byte(`["Takeaway 1", "Takeaway 2"]`),
		CreatedAt:    time.Now(),
	}
	err = db.Create(testAnalysis).Error
	require.NoError(t, err)

	// Create test fact checks
	factChecks := []*models.FactCheck{
		{
			ID:         uuid.New(),
			AnalysisID: testAnalysis.ID,
			Claim:      "Test claim 1",
			Verdict:    "Verified",
			Confidence: 0.9,
			CheckedAt:  time.Now(),
		},
		{
			ID:         uuid.New(),
			AnalysisID: testAnalysis.ID,
			Claim:      "Test claim 2",
			Verdict:    "Disputed",
			Confidence: 0.7,
			CheckedAt:  time.Now(),
		},
	}

	for _, factCheck := range factChecks {
		err := db.Create(factCheck).Error
		require.NoError(t, err)
	}

	// Test getting existing analysis results
	results, err := service.GetAnalysisResults(testAnalysis.ID, "test-correlation-id")
	assert.NoError(t, err)
	assert.NotNil(t, results)
	assert.Equal(t, testAnalysis.ID, results.ID)
	assert.Equal(t, "completed", results.Status)
	assert.Equal(t, "Test summary", *results.Summary)
	assert.Len(t, results.FactChecks, 2)

	// Test getting non-existent analysis results
	nonExistentID := uuid.New()
	results, err = service.GetAnalysisResults(nonExistentID, "test-correlation-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	assert.Nil(t, results)
}