package handlers

import (
	"backend-golang/internal/models"
	"backend-golang/internal/services"
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TranscriptServiceInterface for testing
type TranscriptServiceInterface interface {
	UploadTranscript(req *services.UploadTranscriptRequest, correlationID string) (*services.UploadTranscriptResponse, error)
	GetTranscripts(page, perPage int) ([]*models.Transcript, int64, error)
	GetTranscript(id uuid.UUID) (*models.Transcript, error)
	DeleteTranscript(id uuid.UUID, correlationID string) error
}

// MockTranscriptService for testing
type MockTranscriptService struct {
	mock.Mock
}

func (m *MockTranscriptService) UploadTranscript(req *services.UploadTranscriptRequest, correlationID string) (*services.UploadTranscriptResponse, error) {
	args := m.Called(req, correlationID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.UploadTranscriptResponse), args.Error(1)
}

func (m *MockTranscriptService) GetTranscripts(page, perPage int) ([]*models.Transcript, int64, error) {
	args := m.Called(page, perPage)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*models.Transcript), args.Get(1).(int64), args.Error(2)
}

func (m *MockTranscriptService) GetTranscript(id uuid.UUID) (*models.Transcript, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Transcript), args.Error(1)
}

func (m *MockTranscriptService) DeleteTranscript(id uuid.UUID, correlationID string) error {
	args := m.Called(id, correlationID)
	return args.Error(0)
}

func (m *MockTranscriptService) ReadTranscriptContent(transcript *models.Transcript) (string, error) {
	args := m.Called(transcript)
	if args.Get(0) == nil {
		return "", args.Error(1)
	}
	return args.Get(0).(string), args.Error(1)
}



func createTestFileUpload(t *testing.T, fieldName, filename, content string) (*bytes.Buffer, string) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile(fieldName, filename)
	require.NoError(t, err)

	_, err = part.Write([]byte(content))
	require.NoError(t, err)

	err = writer.Close()
	require.NoError(t, err)

	return body, writer.FormDataContentType()
}

func TestTranscriptHandler_UploadTranscript(t *testing.T) {
	mockService := &MockTranscriptService{}
	handler := NewTranscriptHandler(mockService)

	tests := []struct {
		name           string
		setupMock      func()
		filename       string
		content        string
		expectedStatus int
		expectedError  string
	}{
		{
			name: "successful upload",
			setupMock: func() {
				mockService.On("UploadTranscript", mock.AnythingOfType("*services.UploadTranscriptRequest"), mock.AnythingOfType("string")).Return(
					&services.UploadTranscriptResponse{
						TranscriptID: uuid.New(),
						Filename:     "test.txt",
						WordCount:    100,
						Message:      "Transcript uploaded successfully",
					}, nil)
			},
			filename:       "test.txt",
			content:        "This is a test transcript content",
			expectedStatus: http.StatusOK,
		},
		{
			name: "service error",
			setupMock: func() {
				mockService.On("UploadTranscript", mock.AnythingOfType("*services.UploadTranscriptRequest"), mock.AnythingOfType("string")).Return(
					nil, fmt.Errorf("invalid file extension"))
			},
			filename:       "test.pdf",
			content:        "This is invalid content",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid file extension",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mock
			mockService.ExpectedCalls = nil
			mockService.Calls = nil
			tt.setupMock()

			// Create multipart form request
			body, contentType := createTestFileUpload(t, "file", tt.filename, tt.content)
			req := httptest.NewRequest(http.MethodPost, "/api/transcripts/", body)
			req.Header.Set("Content-Type", contentType)
			req.Header.Set("X-Correlation-ID", "test-correlation-id")

			recorder := httptest.NewRecorder()
			handler.UploadTranscript(recorder, req)

			assert.Equal(t, tt.expectedStatus, recorder.Code)

			if tt.expectedError != "" {
				var response map[string]interface{}
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				errorObj := response["error"].(map[string]interface{})
			assert.Contains(t, errorObj["message"].(string), tt.expectedError)
			} else {
				var response services.UploadTranscriptResponse
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, tt.filename, response.Filename)
				assert.Equal(t, "Transcript uploaded successfully", response.Message)
			}

			mockService.AssertExpectations(t)
		})
	}
}

func TestTranscriptHandler_UploadTranscript_NoFile(t *testing.T) {
	mockService := &MockTranscriptService{}
	handler := NewTranscriptHandler(mockService)

	// Create request without file
	req := httptest.NewRequest(http.MethodPost, "/api/transcripts/", nil)
	req.Header.Set("X-Correlation-ID", "test-correlation-id")

	recorder := httptest.NewRecorder()
	handler.UploadTranscript(recorder, req)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)

	var response map[string]interface{}
	err := json.Unmarshal(recorder.Body.Bytes(), &response)
	require.NoError(t, err)
	errorObj := response["error"].(map[string]interface{})
	assert.Contains(t, errorObj["message"].(string), "Failed to parse multipart form")
}

func TestTranscriptHandler_GetTranscripts(t *testing.T) {
	mockService := &MockTranscriptService{}
	handler := NewTranscriptHandler(mockService)

	testTranscripts := []*models.Transcript{
		{
			ID:        uuid.New(),
			Filename:  "test1.txt",
			WordCount: 100,
		},
		{
			ID:        uuid.New(),
			Filename:  "test2.txt",
			WordCount: 200,
		},
	}

	mockService.On("GetTranscripts", 1, 10).Return(testTranscripts, int64(2), nil)

	req := httptest.NewRequest(http.MethodGet, "/api/transcripts?page=1&per_page=10", nil)
	recorder := httptest.NewRecorder()
	handler.GetTranscripts(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)

	var response map[string]interface{}
	err := json.Unmarshal(recorder.Body.Bytes(), &response)
	require.NoError(t, err)

	transcripts := response["transcripts"].([]interface{})
	assert.Len(t, transcripts, 2)
	assert.Equal(t, float64(2), response["total"])
	assert.Equal(t, float64(1), response["page"])
	assert.Equal(t, float64(10), response["per_page"])

	mockService.AssertExpectations(t)
}

func TestTranscriptHandler_GetTranscripts_InvalidPagination(t *testing.T) {
	mockService := &MockTranscriptService{}
	handler := NewTranscriptHandler(mockService)

	// Mock service should return empty results for all these tests
	mockService.On("GetTranscripts", mock.AnythingOfType("int"), mock.AnythingOfType("int")).Return([]*models.Transcript{}, int64(0), nil)

	tests := []struct {
		name         string
		query        string
		expectedPage int
		expectedPer  int
	}{
		{
			name:         "invalid page defaults to 1",
			query:        "page=invalid&per_page=10",
			expectedPage: 1,
			expectedPer:  10,
		},
		{
			name:         "invalid per_page defaults to 20",
			query:        "page=1&per_page=invalid",
			expectedPage: 1,
			expectedPer:  20,
		},
		{
			name:         "page 0 corrected to 1",
			query:        "page=0&per_page=10",
			expectedPage: 1,
			expectedPer:  10,
		},
		{
			name:         "per_page 0 corrected to 20",
			query:        "page=1&per_page=0",
			expectedPage: 1,
			expectedPer:  20,
		},
		{
			name:         "per_page too large corrected to 20",
			query:        "page=1&per_page=101",
			expectedPage: 1,
			expectedPer:  20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/transcripts?"+tt.query, nil)
			recorder := httptest.NewRecorder()
			handler.GetTranscripts(recorder, req)

			// Should succeed with corrected parameters
			assert.Equal(t, http.StatusOK, recorder.Code)

			var response map[string]interface{}
			err := json.Unmarshal(recorder.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Equal(t, float64(tt.expectedPage), response["page"])
			assert.Equal(t, float64(tt.expectedPer), response["per_page"])
		})
	}

	mockService.AssertExpectations(t)
}

func TestTranscriptHandler_GetTranscript(t *testing.T) {
	mockService := &MockTranscriptService{}
	handler := NewTranscriptHandler(mockService)

	testID := uuid.New()
	testTranscript := &models.Transcript{
		ID:        testID,
		Filename:  "test.txt",
		WordCount: 150,
	}

	tests := []struct {
		name           string
		id             string
		setupMock      func()
		expectedStatus int
		expectedError  string
	}{
		{
			name: "successful get",
			id:   testID.String(),
			setupMock: func() {
				mockService.On("GetTranscript", testID).Return(testTranscript, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "transcript not found",
			id:   testID.String(),
			setupMock: func() {
				mockService.On("GetTranscript", testID).Return(nil, fmt.Errorf("transcript not found"))
			},
			expectedStatus: http.StatusNotFound,
			expectedError:  "transcript not found",
		},
		{
			name:           "invalid UUID",
			id:             "invalid-uuid",
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

			req := httptest.NewRequest(http.MethodGet, "/api/transcripts/"+tt.id, nil)
			recorder := httptest.NewRecorder()
			handler.GetTranscript(recorder, req)

			assert.Equal(t, tt.expectedStatus, recorder.Code)

			if tt.expectedError != "" {
				var response map[string]interface{}
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				errorObj := response["error"].(map[string]interface{})
			assert.Contains(t, errorObj["message"].(string), tt.expectedError)
			} else {
				var response models.Transcript
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Equal(t, testTranscript.ID, response.ID)
				assert.Equal(t, testTranscript.Filename, response.Filename)
			}

			mockService.AssertExpectations(t)
		})
	}
}

func TestTranscriptHandler_DeleteTranscript(t *testing.T) {
	mockService := &MockTranscriptService{}
	handler := NewTranscriptHandler(mockService)

	testID := uuid.New()

	tests := []struct {
		name           string
		id             string
		setupMock      func()
		expectedStatus int
		expectedError  string
	}{
		{
			name: "successful delete",
			id:   testID.String(),
			setupMock: func() {
				mockService.On("DeleteTranscript", testID, mock.AnythingOfType("string")).Return(nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "transcript not found",
			id:   testID.String(),
			setupMock: func() {
				mockService.On("DeleteTranscript", testID, mock.AnythingOfType("string")).Return(fmt.Errorf("transcript not found"))
			},
			expectedStatus: http.StatusNotFound,
			expectedError:  "transcript not found",
		},
		{
			name:           "invalid UUID",
			id:             "invalid-uuid",
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

			req := httptest.NewRequest(http.MethodDelete, "/api/transcripts/"+tt.id, nil)
			req.Header.Set("X-Correlation-ID", "test-correlation-id")
			recorder := httptest.NewRecorder()
			handler.DeleteTranscript(recorder, req)

			assert.Equal(t, tt.expectedStatus, recorder.Code)

			var response map[string]interface{}
			err := json.Unmarshal(recorder.Body.Bytes(), &response)
			require.NoError(t, err)

			if tt.expectedError != "" {
				errorObj := response["error"].(map[string]interface{})
			assert.Contains(t, errorObj["message"].(string), tt.expectedError)
			} else {
				assert.Equal(t, "Transcript deleted successfully", response["message"])
			}

			mockService.AssertExpectations(t)
		})
	}
}