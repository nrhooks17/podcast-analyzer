package handlers

import (
	"backend-golang/internal/services"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)


// AnalysisServiceInterface for testing
type AnalysisServiceInterface interface {
	CreateAnalysisJob(req *services.AnalysisJobRequest, correlationID string) (*services.AnalysisJobResponse, error)
	GetJobStatus(jobID uuid.UUID, correlationID string) (*services.JobStatusResponse, error)
	ListAnalysisResults(page, perPage int) ([]*services.AnalysisResultsResponse, int64, error)
	GetAnalysisResults(analysisID uuid.UUID, correlationID string) (*services.AnalysisResultsResponse, error)
}

// MockAnalysisService for testing
type MockAnalysisService struct {
	mock.Mock
}

func (m *MockAnalysisService) CreateAnalysisJob(req *services.AnalysisJobRequest, correlationID string) (*services.AnalysisJobResponse, error) {
	args := m.Called(req, correlationID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.AnalysisJobResponse), args.Error(1)
}

func (m *MockAnalysisService) GetJobStatus(jobID uuid.UUID, correlationID string) (*services.JobStatusResponse, error) {
	args := m.Called(jobID, correlationID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.JobStatusResponse), args.Error(1)
}

func (m *MockAnalysisService) ListAnalysisResults(page, perPage int) ([]*services.AnalysisResultsResponse, int64, error) {
	args := m.Called(page, perPage)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*services.AnalysisResultsResponse), args.Get(1).(int64), args.Error(2)
}

func (m *MockAnalysisService) GetAnalysisResults(analysisID uuid.UUID, correlationID string) (*services.AnalysisResultsResponse, error) {
	args := m.Called(analysisID, correlationID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.AnalysisResultsResponse), args.Error(1)
}

func (m *MockAnalysisService) UpdateJobStatus(jobID uuid.UUID, status string, errorMessage string) error {
	args := m.Called(jobID, status, errorMessage)
	return args.Error(0)
}

func TestAnalysisHandler_StartAnalysis(t *testing.T) {
	mockService := &MockAnalysisService{}
	handler := NewAnalysisHandler(mockService)

	testTranscriptID := uuid.New()

	tests := []struct {
		name           string
		transcriptID   string
		setupMock      func()
		expectedStatus int
		expectedError  string
	}{
		{
			name:         "successful analysis start",
			transcriptID: testTranscriptID.String(),
			setupMock: func() {
				mockService.On("CreateAnalysisJob", mock.MatchedBy(func(req *services.AnalysisJobRequest) bool {
					return req.TranscriptID == testTranscriptID
				}), mock.AnythingOfType("string")).Return(
					&services.AnalysisJobResponse{
						JobID:        uuid.New(),
						TranscriptID: testTranscriptID,
						Status:       "pending",
						Message:      "Analysis job created and queued for processing",
					}, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:         "transcript not found",
			transcriptID: testTranscriptID.String(),
			setupMock: func() {
				mockService.On("CreateAnalysisJob", mock.AnythingOfType("*services.AnalysisJobRequest"), mock.AnythingOfType("string")).Return(
					nil, fmt.Errorf("transcript not found"))
			},
			expectedStatus: http.StatusNotFound,
			expectedError:  "transcript not found",
		},
		{
			name:           "invalid UUID",
			transcriptID:   "invalid-uuid",
			setupMock:      func() {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid transcript ID format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mock
			mockService.ExpectedCalls = nil
			mockService.Calls = nil
			tt.setupMock()

			req := httptest.NewRequest(http.MethodPost, "/api/analyze/"+tt.transcriptID, nil)
			req.Header.Set("X-Correlation-ID", "test-correlation-id")
			recorder := httptest.NewRecorder()
			handler.StartAnalysis(recorder, req)

			assert.Equal(t, tt.expectedStatus, recorder.Code)

			var response map[string]interface{}
			err := json.Unmarshal(recorder.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.expectedError != "" {
				errorObj := response["error"].(map[string]interface{})
				assert.Contains(t, errorObj["message"].(string), tt.expectedError)
			} else {
				assert.Equal(t, "pending", response["status"])
				assert.Equal(t, "Analysis job created and queued for processing", response["message"])
				assert.NotNil(t, response["job_id"])
			}

			mockService.AssertExpectations(t)
		})
	}
}

func TestAnalysisHandler_GetJobStatus(t *testing.T) {
	mockService := &MockAnalysisService{}
	handler := NewAnalysisHandler(mockService)

	testJobID := uuid.New()
	testTranscriptID := uuid.New()

	tests := []struct {
		name           string
		jobID          string
		setupMock      func()
		expectedStatus int
		expectedError  string
	}{
		{
			name:  "successful status check",
			jobID: testJobID.String(),
			setupMock: func() {
				mockService.On("GetJobStatus", testJobID, mock.AnythingOfType("string")).Return(
					&services.JobStatusResponse{
						JobID:        testJobID,
						TranscriptID: testTranscriptID,
						Status:       "in_progress",
						CreatedAt:    time.Now(),
					}, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:  "job not found",
			jobID: testJobID.String(),
			setupMock: func() {
				mockService.On("GetJobStatus", testJobID, mock.AnythingOfType("string")).Return(
					nil, fmt.Errorf("analysis job not found"))
			},
			expectedStatus: http.StatusNotFound,
			expectedError:  "analysis job not found",
		},
		{
			name:           "invalid UUID",
			jobID:          "invalid-uuid",
			setupMock:      func() {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid job ID format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mock
			mockService.ExpectedCalls = nil
			mockService.Calls = nil
			tt.setupMock()

			req := httptest.NewRequest(http.MethodGet, "/api/jobs/"+tt.jobID+"/status", nil)
			req.Header.Set("X-Correlation-ID", "test-correlation-id")
			recorder := httptest.NewRecorder()
			handler.GetJobStatus(recorder, req)

			assert.Equal(t, tt.expectedStatus, recorder.Code)

			var response map[string]interface{}
			err := json.Unmarshal(recorder.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.expectedError != "" {
				errorObj := response["error"].(map[string]interface{})
				assert.Contains(t, errorObj["message"].(string), tt.expectedError)
			} else {
				assert.Equal(t, "in_progress", response["status"])
				// No progress field in current implementation
				assert.NotNil(t, response["job_id"])
				assert.NotNil(t, response["transcript_id"])
			}

			mockService.AssertExpectations(t)
		})
	}
}

func TestAnalysisHandler_ListAnalysisResults(t *testing.T) {
	mockService := &MockAnalysisService{}
	handler := NewAnalysisHandler(mockService)

	summary1 := "Test summary 1"
	summary2 := "Test summary 2"
	testResults := []*services.AnalysisResultsResponse{
		{
			ID:           uuid.New(),
			TranscriptID: uuid.New(),
			Status:       "completed",
			Summary:      &summary1,
			CreatedAt:    time.Now(),
		},
		{
			ID:           uuid.New(),
			TranscriptID: uuid.New(),
			Status:       "completed",
			Summary:      &summary2,
			CreatedAt:    time.Now(),
		},
	}

	tests := []struct {
		name           string
		query          string
		setupMock      func()
		expectedStatus int
		expectedError  string
	}{
		{
			name:  "successful list",
			query: "page=1&per_page=10",
			setupMock: func() {
				mockService.On("ListAnalysisResults", 1, 10).Return(
					testResults, int64(2), nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:  "invalid page gets default",
			query: "page=invalid&per_page=10",
			setupMock: func() {
				mockService.On("ListAnalysisResults", 1, 10).Return(
					testResults, int64(2), nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:  "invalid per_page gets default",
			query: "page=1&per_page=invalid",
			setupMock: func() {
				mockService.On("ListAnalysisResults", 1, 20).Return(
					testResults, int64(2), nil)
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mock
			mockService.ExpectedCalls = nil
			mockService.Calls = nil
			tt.setupMock()

			req := httptest.NewRequest(http.MethodGet, "/api/results?"+tt.query, nil)
			req.Header.Set("X-Correlation-ID", "test-correlation-id")
			recorder := httptest.NewRecorder()
			handler.ListAnalysisResults(recorder, req)

			assert.Equal(t, tt.expectedStatus, recorder.Code)

			var response map[string]interface{}
			err := json.Unmarshal(recorder.Body.Bytes(), &response)
			require.NoError(t, err)

			results := response["results"].([]interface{})
			assert.Len(t, results, 2)
			assert.Equal(t, float64(2), response["total"])

			mockService.AssertExpectations(t)
		})
	}
}

func TestAnalysisHandler_GetAnalysisResults(t *testing.T) {
	mockService := &MockAnalysisService{}
	handler := NewAnalysisHandler(mockService)

	testAnalysisID := uuid.New()
	summary := "Test summary"
	testResult := &services.AnalysisResultsResponse{
		ID:           testAnalysisID,
		TranscriptID: uuid.New(),
		JobID:        uuid.New(),
		Status:       "completed",
		Summary:      &summary,
		Takeaways:    []string{"Takeaway 1", "Takeaway 2"},
		CreatedAt:    time.Now(),
		FactChecks: []services.FactCheckResultResponse{
			{
				ID:         uuid.New(),
				Claim:      "Test claim",
				Verdict:    "Verified",
				Confidence: 0.9,
				CheckedAt:  time.Now(),
			},
		},
	}

	tests := []struct {
		name           string
		analysisID     string
		setupMock      func()
		expectedStatus int
		expectedError  string
	}{
		{
			name:       "successful get results",
			analysisID: testAnalysisID.String(),
			setupMock: func() {
				mockService.On("GetAnalysisResults", testAnalysisID, mock.AnythingOfType("string")).Return(
					testResult, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:       "results not found",
			analysisID: testAnalysisID.String(),
			setupMock: func() {
				mockService.On("GetAnalysisResults", testAnalysisID, mock.AnythingOfType("string")).Return(
					nil, fmt.Errorf("analysis result not found"))
			},
			expectedStatus: http.StatusNotFound,
			expectedError:  "analysis result not found",
		},
		{
			name:           "invalid UUID",
			analysisID:     "invalid-uuid",
			setupMock:      func() {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid analysis ID format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mock
			mockService.ExpectedCalls = nil
			mockService.Calls = nil
			tt.setupMock()

			req := httptest.NewRequest(http.MethodGet, "/api/results/"+tt.analysisID, nil)
			req.Header.Set("X-Correlation-ID", "test-correlation-id")
			recorder := httptest.NewRecorder()
			handler.GetAnalysisResults(recorder, req)

			assert.Equal(t, tt.expectedStatus, recorder.Code)

			var response map[string]interface{}
			err := json.Unmarshal(recorder.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.expectedError != "" {
				errorObj := response["error"].(map[string]interface{})
				assert.Contains(t, errorObj["message"].(string), tt.expectedError)
			} else {
				assert.Equal(t, "completed", response["status"])
				assert.Equal(t, "Test summary", response["summary"])
				assert.NotNil(t, response["takeaways"])
				factChecks := response["fact_checks"].([]interface{})
				assert.Len(t, factChecks, 1)
			}

			mockService.AssertExpectations(t)
		})
	}
}