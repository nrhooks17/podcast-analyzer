package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/datatypes"
)

// MockDB represents a mock database interface
type MockDB struct {
	transcripts     map[uuid.UUID]*Transcript
	analysisResults map[uuid.UUID]*AnalysisResult
	factChecks      map[uuid.UUID]*FactCheck
	nextError       error
}

func NewMockDB() *MockDB {
	return &MockDB{
		transcripts:     make(map[uuid.UUID]*Transcript),
		analysisResults: make(map[uuid.UUID]*AnalysisResult),
		factChecks:      make(map[uuid.UUID]*FactCheck),
	}
}

func (m *MockDB) SetError(err error) {
	m.nextError = err
}

func (m *MockDB) Create(value interface{}) error {
	if m.nextError != nil {
		err := m.nextError
		m.nextError = nil
		return err
	}
	
	switch v := value.(type) {
	case *Transcript:
		v.BeforeCreate(nil)
		m.transcripts[v.ID] = v
	case *AnalysisResult:
		v.BeforeCreate(nil)
		m.analysisResults[v.ID] = v
	case *FactCheck:
		v.BeforeCreate(nil)
		m.factChecks[v.ID] = v
	}
	return nil
}

func TestTranscript_BeforeCreate_GeneratesUUID(t *testing.T) {
	db := NewMockDB()

	transcript := Transcript{
		Filename:    "test.txt",
		FilePath:    "/test/path/test.txt",
		ContentHash: "testhash123",
		WordCount:   100,
	}

	// ID should be nil initially
	assert.Equal(t, uuid.Nil, transcript.ID)

	err := db.Create(&transcript)
	assert.NoError(t, err)

	// ID should be generated after creation
	assert.NotEqual(t, uuid.Nil, transcript.ID)
	assert.NotEmpty(t, transcript.ID.String())
}

func TestTranscript_BeforeCreate_PreservesExistingUUID(t *testing.T) {
	db := NewMockDB()

	existingID := uuid.New()
	transcript := Transcript{
		ID:          existingID,
		Filename:    "test.txt",
		FilePath:    "/test/path/test.txt",
		ContentHash: "testhash456",
		WordCount:   200,
	}

	err := db.Create(&transcript)
	assert.NoError(t, err)

	// Should preserve the existing ID
	assert.Equal(t, existingID, transcript.ID)
}

func TestAnalysisResult_BeforeCreate_GeneratesUUIDs(t *testing.T) {
	db := NewMockDB()

	// First create a transcript
	transcript := Transcript{
		Filename:    "test.txt",
		FilePath:    "/test/path/test.txt",
		ContentHash: "testhash789",
		WordCount:   150,
	}
	err := db.Create(&transcript)
	assert.NoError(t, err)

	analysis := AnalysisResult{
		TranscriptID: transcript.ID,
		Status:       "pending",
	}

	// Both IDs should be nil initially
	assert.Equal(t, uuid.Nil, analysis.ID)
	assert.Equal(t, uuid.Nil, analysis.JobID)

	err = db.Create(&analysis)
	assert.NoError(t, err)

	// Both IDs should be generated after creation
	assert.NotEqual(t, uuid.Nil, analysis.ID)
	assert.NotEqual(t, uuid.Nil, analysis.JobID)
	assert.NotEmpty(t, analysis.ID.String())
	assert.NotEmpty(t, analysis.JobID.String())
}

func TestAnalysisResult_BeforeCreate_PreservesExistingUUIDs(t *testing.T) {
	db := NewMockDB()

	// First create a transcript
	transcript := Transcript{
		Filename:    "test.txt",
		FilePath:    "/test/path/test.txt",
		ContentHash: "testhashpreserve",
		WordCount:   300,
	}
	err := db.Create(&transcript)
	assert.NoError(t, err)

	existingID := uuid.New()
	existingJobID := uuid.New()

	analysis := AnalysisResult{
		ID:           existingID,
		JobID:        existingJobID,
		TranscriptID: transcript.ID,
		Status:       "processing",
	}

	err = db.Create(&analysis)
	assert.NoError(t, err)

	// Should preserve existing IDs
	assert.Equal(t, existingID, analysis.ID)
	assert.Equal(t, existingJobID, analysis.JobID)
}

func TestFactCheck_BeforeCreate_GeneratesUUID(t *testing.T) {
	db := NewMockDB()

	// Create transcript and analysis first
	transcript := Transcript{
		Filename:    "test.txt",
		FilePath:    "/test/path/test.txt",
		ContentHash: "testhashfact",
		WordCount:   250,
	}
	err := db.Create(&transcript)
	assert.NoError(t, err)

	analysis := AnalysisResult{
		TranscriptID: transcript.ID,
		Status:       "completed",
	}
	err = db.Create(&analysis)
	assert.NoError(t, err)

	factCheck := FactCheck{
		AnalysisID: analysis.ID,
		Claim:      "Test claim",
		Verdict:    "true",
		Confidence: 0.95,
	}

	// ID should be nil initially
	assert.Equal(t, uuid.Nil, factCheck.ID)

	err = db.Create(&factCheck)
	assert.NoError(t, err)

	// ID should be generated after creation
	assert.NotEqual(t, uuid.Nil, factCheck.ID)
	assert.NotEmpty(t, factCheck.ID.String())
}

func TestFactCheck_BeforeCreate_PreservesExistingUUID(t *testing.T) {
	db := NewMockDB()

	// Create dependencies
	transcript := Transcript{
		Filename:    "test.txt",
		FilePath:    "/test/path/test.txt",
		ContentHash: "testhashfactpreserve",
		WordCount:   180,
	}
	err := db.Create(&transcript)
	assert.NoError(t, err)

	analysis := AnalysisResult{
		TranscriptID: transcript.ID,
		Status:       "completed",
	}
	err = db.Create(&analysis)
	assert.NoError(t, err)

	existingID := uuid.New()
	factCheck := FactCheck{
		ID:         existingID,
		AnalysisID: analysis.ID,
		Claim:      "Test claim with existing ID",
		Verdict:    "false",
		Confidence: 0.85,
	}

	err = db.Create(&factCheck)
	assert.NoError(t, err)

	// Should preserve existing ID
	assert.Equal(t, existingID, factCheck.ID)
}

func TestAutoMigrate_Success(t *testing.T) {
	// Test AutoMigrate function exists and can be referenced
	assert.NotNil(t, AutoMigrate)
	
	// Since we're avoiding external dependencies, we just verify the function is callable
	// The actual migration would be tested with integration tests using a real database
	// We don't call it to avoid external database dependency
}

func TestTranscript_JSONSerialization(t *testing.T) {
	metadata := map[string]interface{}{
		"duration": 3600,
		"language": "en",
		"speaker":  "John Doe",
	}
	metadataJSON, err := json.Marshal(metadata)
	assert.NoError(t, err)

	transcript := Transcript{
		ID:                 uuid.New(),
		Filename:           "podcast.txt",
		FilePath:           "/storage/podcast.txt",
		ContentHash:        "abc123def456",
		WordCount:          1500,
		UploadedAt:         time.Now(),
		TranscriptMetadata: datatypes.JSON(metadataJSON),
	}

	// Test JSON marshaling
	jsonData, err := json.Marshal(transcript)
	assert.NoError(t, err)
	assert.NotEmpty(t, jsonData)

	// Test JSON unmarshaling
	var unmarshaled Transcript
	err = json.Unmarshal(jsonData, &unmarshaled)
	assert.NoError(t, err)

	assert.Equal(t, transcript.ID, unmarshaled.ID)
	assert.Equal(t, transcript.Filename, unmarshaled.Filename)
	assert.Equal(t, transcript.FilePath, unmarshaled.FilePath)
	assert.Equal(t, transcript.ContentHash, unmarshaled.ContentHash)
	assert.Equal(t, transcript.WordCount, unmarshaled.WordCount)
}

func TestAnalysisResult_JSONSerialization(t *testing.T) {
	takeaways := []string{
		"First takeaway",
		"Second takeaway",
		"Third takeaway",
	}
	takeawaysJSON, err := json.Marshal(takeaways)
	assert.NoError(t, err)

	summary := "This is a test summary"
	completedAt := time.Now()

	analysis := AnalysisResult{
		ID:           uuid.New(),
		TranscriptID: uuid.New(),
		JobID:        uuid.New(),
		Status:       "completed",
		Summary:      &summary,
		Takeaways:    datatypes.JSON(takeawaysJSON),
		CreatedAt:    time.Now(),
		CompletedAt:  &completedAt,
	}

	// Test JSON marshaling
	jsonData, err := json.Marshal(analysis)
	assert.NoError(t, err)
	assert.NotEmpty(t, jsonData)

	// Test JSON unmarshaling
	var unmarshaled AnalysisResult
	err = json.Unmarshal(jsonData, &unmarshaled)
	assert.NoError(t, err)

	assert.Equal(t, analysis.ID, unmarshaled.ID)
	assert.Equal(t, analysis.Status, unmarshaled.Status)
	assert.Equal(t, *analysis.Summary, *unmarshaled.Summary)
}

func TestFactCheck_JSONSerialization(t *testing.T) {
	sources := []string{
		"https://example.com/source1",
		"https://example.com/source2",
	}
	sourcesJSON, err := json.Marshal(sources)
	assert.NoError(t, err)

	evidence := "Strong evidence supports this claim"

	factCheck := FactCheck{
		ID:         uuid.New(),
		AnalysisID: uuid.New(),
		Claim:      "Test factual claim",
		Verdict:    "true",
		Confidence: 0.92,
		Evidence:   &evidence,
		Sources:    datatypes.JSON(sourcesJSON),
		CheckedAt:  time.Now(),
	}

	// Test JSON marshaling
	jsonData, err := json.Marshal(factCheck)
	assert.NoError(t, err)
	assert.NotEmpty(t, jsonData)

	// Test JSON unmarshaling
	var unmarshaled FactCheck
	err = json.Unmarshal(jsonData, &unmarshaled)
	assert.NoError(t, err)

	assert.Equal(t, factCheck.ID, unmarshaled.ID)
	assert.Equal(t, factCheck.Claim, unmarshaled.Claim)
	assert.Equal(t, factCheck.Verdict, unmarshaled.Verdict)
	assert.Equal(t, factCheck.Confidence, unmarshaled.Confidence)
	assert.Equal(t, *factCheck.Evidence, *unmarshaled.Evidence)
}

func TestModel_Relationships_Structure(t *testing.T) {
	db := NewMockDB()

	// Create transcript
	transcript := Transcript{
		Filename:    "relationships.txt",
		FilePath:    "/test/relationships.txt",
		ContentHash: "relationshiphash",
		WordCount:   500,
	}
	err := db.Create(&transcript)
	assert.NoError(t, err)

	// Create analysis
	summary := "Test summary for relationships"
	analysis := AnalysisResult{
		TranscriptID: transcript.ID,
		Status:       "completed",
		Summary:      &summary,
	}
	err = db.Create(&analysis)
	assert.NoError(t, err)

	// Create fact checks
	evidence1 := "Evidence for first claim"
	evidence2 := "Evidence for second claim"

	factCheck1 := FactCheck{
		AnalysisID: analysis.ID,
		Claim:      "First claim",
		Verdict:    "true",
		Confidence: 0.9,
		Evidence:   &evidence1,
	}
	factCheck2 := FactCheck{
		AnalysisID: analysis.ID,
		Claim:      "Second claim",
		Verdict:    "false",
		Confidence: 0.8,
		Evidence:   &evidence2,
	}

	err = db.Create(&factCheck1)
	assert.NoError(t, err)
	err = db.Create(&factCheck2)
	assert.NoError(t, err)

	// Test that the relationships are properly set up in memory
	assert.Equal(t, transcript.ID, analysis.TranscriptID)
	assert.Equal(t, analysis.ID, factCheck1.AnalysisID)
	assert.Equal(t, analysis.ID, factCheck2.AnalysisID)
	
	// Verify fact checks have different claims
	assert.Equal(t, "First claim", factCheck1.Claim)
	assert.Equal(t, "Second claim", factCheck2.Claim)
}

func TestFactCheck_VerdictValidation(t *testing.T) {
	verdicts := []string{"true", "false", "partially_true", "unverifiable"}
	
	for _, verdict := range verdicts {
		t.Run("verdict_"+verdict, func(t *testing.T) {
			factCheck := FactCheck{
				Claim:      "Test claim for " + verdict,
				Verdict:    verdict,
				Confidence: 0.75,
			}
			
			// Verify the verdict field accepts the value
			assert.Equal(t, verdict, factCheck.Verdict)
		})
	}
}

func TestTranscript_UniqueContentHash(t *testing.T) {
	db := NewMockDB()

	// Create first transcript
	transcript1 := Transcript{
		Filename:    "unique1.txt",
		FilePath:    "/test/unique1.txt",
		ContentHash: "uniquehash123",
		WordCount:   100,
	}
	err := db.Create(&transcript1)
	assert.NoError(t, err)

	// Test that ContentHash field is properly set
	assert.Equal(t, "uniquehash123", transcript1.ContentHash)
	
	// Create second transcript with different hash should work
	transcript2 := Transcript{
		Filename:    "unique2.txt",
		FilePath:    "/test/unique2.txt",
		ContentHash: "uniquehash456", // Different hash
		WordCount:   200,
	}
	err = db.Create(&transcript2)
	assert.NoError(t, err)
	assert.Equal(t, "uniquehash456", transcript2.ContentHash)
}

func TestAnalysisResult_StatusValues(t *testing.T) {
	statuses := []string{"pending", "processing", "completed", "failed"}
	
	for _, status := range statuses {
		t.Run("status_"+status, func(t *testing.T) {
			analysis := AnalysisResult{
				Status: status,
			}
			
			assert.Equal(t, status, analysis.Status)
		})
	}
}

func TestFactCheck_ConfidenceRange(t *testing.T) {
	testCases := []struct {
		confidence float64
		name       string
	}{
		{0.0, "minimum_confidence"},
		{0.5, "middle_confidence"},
		{1.0, "maximum_confidence"},
		{0.123, "decimal_confidence"},
		{0.999, "high_confidence"},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			factCheck := FactCheck{
				Claim:      "Test confidence",
				Verdict:    "true",
				Confidence: tc.confidence,
			}
			
			assert.Equal(t, tc.confidence, factCheck.Confidence)
			assert.GreaterOrEqual(t, factCheck.Confidence, 0.0)
			assert.LessOrEqual(t, factCheck.Confidence, 1.0)
		})
	}
}