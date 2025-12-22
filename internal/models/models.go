package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Transcript represents a uploaded transcript file
type Transcript struct {
	ID               uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Filename         string         `gorm:"size:255;not null" json:"filename"`
	FilePath         string         `gorm:"size:500;not null" json:"file_path"`
	ContentHash      string         `gorm:"size:64;not null;unique" json:"content_hash"`
	WordCount        int            `gorm:"not null" json:"word_count"`
	UploadedAt       time.Time      `gorm:"default:CURRENT_TIMESTAMP" json:"uploaded_at"`
	TranscriptMetadata datatypes.JSON `gorm:"type:jsonb" json:"transcript_metadata,omitempty"`
	
	// Relationships
	Analyses []AnalysisResult `gorm:"foreignKey:TranscriptID" json:"analyses,omitempty"`
}

// AnalysisResult represents the results of AI analysis
type AnalysisResult struct {
	ID           uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	TranscriptID uuid.UUID      `gorm:"type:uuid;not null;index" json:"transcript_id"`
	JobID        uuid.UUID      `gorm:"type:uuid;not null;unique;index" json:"job_id"`
	Status       string         `gorm:"size:20;not null;default:'pending';index" json:"status"` // pending, processing, completed, failed
	Summary      *string        `gorm:"type:text" json:"summary,omitempty"`
	Takeaways    datatypes.JSON `gorm:"type:jsonb" json:"takeaways,omitempty"` // Array of key takeaways
	CreatedAt    time.Time      `gorm:"default:CURRENT_TIMESTAMP;index" json:"created_at"`
	CompletedAt  *time.Time     `json:"completed_at,omitempty"`
	ErrorMessage *string        `gorm:"type:text" json:"error_message,omitempty"`

	// Relationships
	Transcript Transcript  `gorm:"foreignKey:TranscriptID" json:"transcript,omitempty"`
	FactChecks []FactCheck `gorm:"foreignKey:AnalysisID;constraint:OnDelete:CASCADE" json:"fact_checks,omitempty"`
}

// FactCheck represents individual fact-check results
type FactCheck struct {
	ID         uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	AnalysisID uuid.UUID      `gorm:"type:uuid;not null;index" json:"analysis_id"`
	Claim      string         `gorm:"type:text;not null" json:"claim"`
	Verdict    string         `gorm:"size:20;not null" json:"verdict"` // true, false, partially_true, unverifiable
	Confidence float64        `gorm:"not null;check:confidence >= 0 AND confidence <= 1" json:"confidence"`
	Evidence   *string        `gorm:"type:text" json:"evidence,omitempty"`
	Sources    datatypes.JSON `gorm:"type:jsonb" json:"sources,omitempty"`
	CheckedAt  time.Time      `gorm:"default:CURRENT_TIMESTAMP" json:"checked_at"`

	// Relationships
	Analysis AnalysisResult `gorm:"foreignKey:AnalysisID" json:"analysis,omitempty"`
}

// BeforeCreate will set a UUID rather than numeric ID
func (t *Transcript) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}

func (a *AnalysisResult) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	if a.JobID == uuid.Nil {
		a.JobID = uuid.New()
	}
	return nil
}

func (f *FactCheck) BeforeCreate(tx *gorm.DB) error {
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}
	return nil
}

// AutoMigrate creates or updates database tables
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&Transcript{}, &AnalysisResult{}, &FactCheck{})
}