package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"backend-golang/internal/config"
	"backend-golang/internal/models"
	"backend-golang/internal/services"
	"backend-golang/pkg/kafka"
	"backend-golang/pkg/logger"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// AnalysisJobMessage represents the Kafka message for analysis jobs
type AnalysisJobMessage struct {
	JobID        string `json:"job_id"`
	TranscriptID string `json:"transcript_id"`
}

type AnalysisWorker struct {
	db               *gorm.DB
	cfg              *config.Config
	kafkaService     *kafka.Service
	transcriptSvc    *services.TranscriptService
	analysisSvc      *services.AnalysisService
	running          bool
}

func NewAnalysisWorker(db *gorm.DB, cfg *config.Config, kafkaService *kafka.Service) *AnalysisWorker {
	return &AnalysisWorker{
		db:            db,
		cfg:           cfg,
		kafkaService:  kafkaService,
		transcriptSvc: services.NewTranscriptService(db, cfg),
		analysisSvc:   services.NewAnalysisService(db, cfg, kafkaService),
		running:       false,
	}
}

func (w *AnalysisWorker) processAnalysisJob(ctx context.Context, message AnalysisJobMessage) (retErr error) {
	// Setup panic recovery for this job
	defer func() {
		if r := recover(); r != nil {
			// Get stack trace
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			
			logger.Log.WithFields(map[string]interface{}{
				"panic":      r,
				"stack_trace": string(buf[:n]),
			}).Error("Worker panic in job processing")
			
			retErr = fmt.Errorf("worker panicked: %v", r)
		}
	}()

	logger.Log.WithField("message", message).Info("Processing job message")
	
	jobID, err := uuid.Parse(message.JobID)
	if err != nil {
		logger.LogErrorWithStack(err, map[string]interface{}{
			"job_id_raw": message.JobID,
			"operation":  "parse_job_id",
		})
		return fmt.Errorf("invalid job ID format %s: %w", message.JobID, err)
	}

	transcriptID, err := uuid.Parse(message.TranscriptID)
	if err != nil {
		logger.LogErrorWithStack(err, map[string]interface{}{
			"transcript_id_raw": message.TranscriptID,
			"operation":         "parse_transcript_id",
		})
		return fmt.Errorf("invalid transcript ID format %s: %w", message.TranscriptID, err)
	}

	logger.Log.WithFields(map[string]interface{}{
		"job_id":        jobID,
		"transcript_id": transcriptID,
	}).Info("Worker picked up job")

	// Update job status to processing
	logger.Log.WithField("job_id", jobID).Info("Updating job status to processing")
	err = w.analysisSvc.UpdateJobStatus(jobID, "processing", "")
	if err != nil {
		logger.LogErrorWithStack(err, map[string]interface{}{
			"job_id":    jobID,
			"operation": "update_job_status_processing",
		})
		return fmt.Errorf("failed to update job status to processing: %w", err)
	}
	logger.Log.WithField("job_id", jobID).Info("Job status updated to processing")

	// Get transcript
	logger.Log.WithField("transcript_id", transcriptID).Info("Fetching transcript")
	transcript, err := w.transcriptSvc.GetTranscript(transcriptID)
	if err != nil {
		errorMsg := fmt.Sprintf("Transcript not found: %s", transcriptID.String())
		logger.LogErrorWithStack(err, map[string]interface{}{
			"transcript_id": transcriptID,
			"operation":     "get_transcript",
		})
		w.analysisSvc.UpdateJobStatus(jobID, "failed", errorMsg)
		return fmt.Errorf("%s: %w", errorMsg, err)
	}
	logger.Log.WithFields(map[string]interface{}{
		"filename":   transcript.Filename,
		"word_count": transcript.WordCount,
	}).Info("Transcript fetched")

	// Read transcript content
	logger.Log.WithField("file_path", transcript.FilePath).Info("Reading transcript content")
	content, err := w.transcriptSvc.ReadTranscriptContent(transcript)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to read transcript content from %s", transcript.FilePath)
		logger.LogErrorWithStack(err, map[string]interface{}{
			"file_path": transcript.FilePath,
			"operation": "read_transcript_content",
		})
		w.analysisSvc.UpdateJobStatus(jobID, "failed", errorMsg)
		return fmt.Errorf("%s: %w", errorMsg, err)
	}
	logger.Log.WithField("content_length", len(content)).Info("Transcript content read")

	logger.Log.WithFields(map[string]interface{}{
		"job_id":        jobID,
		"content_length": len(content),
	}).Info("Analysis starting")

	// Process with AI agents (this would call the actual AI processing)
	logger.Log.WithField("job_id", jobID).Info("Starting AI analysis agents")
	startTime := time.Now()
	results, err := w.runAnalysisAgents(ctx, content, jobID)
	duration := time.Since(startTime)
	
	if err != nil {
		errorMsg := fmt.Sprintf("Analysis processing failed after %v", duration)
		logger.LogErrorWithStack(err, map[string]interface{}{
			"job_id":    jobID,
			"duration":  duration,
			"operation": "run_analysis_agents",
		})
		w.analysisSvc.UpdateJobStatus(jobID, "failed", errorMsg)
		return fmt.Errorf("%s: %w", errorMsg, err)
	}
	logger.Log.WithFields(map[string]interface{}{
		"job_id":   jobID,
		"duration": duration,
	}).Info("AI analysis completed")

	// Convert takeaways to JSON for database storage
	takeawaysJSON, err := json.Marshal(results.Takeaways)
	if err != nil {
		errorMsg := "Failed to serialize takeaways"
		logger.LogErrorWithStack(err, map[string]interface{}{
			"operation": "serialize_takeaways",
		})
		w.analysisSvc.UpdateJobStatus(jobID, "failed", errorMsg)
		return err
	}

	// Update existing analysis record instead of creating new one
	var analysis models.AnalysisResult
	if err := w.db.Where("job_id = ?", jobID).First(&analysis).Error; err != nil {
		errorMsg := "Failed to find analysis record to update"
		logger.LogErrorWithStack(err, map[string]interface{}{
			"job_id":    jobID,
			"operation": "find_analysis_record",
		})
		w.analysisSvc.UpdateJobStatus(jobID, "failed", errorMsg)
		return err
	}

	analysis.Summary = &results.Summary
	analysis.Takeaways = takeawaysJSON
	now := time.Now()
	analysis.CompletedAt = &now

	if err := w.db.Save(&analysis).Error; err != nil {
		errorMsg := "Failed to save analysis results"
		logger.LogErrorWithStack(err, map[string]interface{}{
			"analysis_id": analysis.ID,
			"operation":   "save_analysis_results",
		})
		w.analysisSvc.UpdateJobStatus(jobID, "failed", errorMsg)
		return err
	}

	// Save fact checks if any
	if len(results.FactChecks) > 0 {
		for _, fc := range results.FactChecks {
			// Convert sources to JSON
			sourcesJSON, _ := json.Marshal(fc.Sources)
			
			factCheck := &models.FactCheck{
				ID:         uuid.New(),
				AnalysisID: analysis.ID,
				Claim:      fc.Claim,
				Verdict:    fc.Verdict,
				Confidence: fc.Confidence,
				Evidence:   &fc.Evidence,
				Sources:    sourcesJSON,
				CheckedAt:  time.Now(),
			}
			if err := w.db.Create(factCheck).Error; err != nil {
				logger.LogErrorWithStack(err, map[string]interface{}{
					"analysis_id": analysis.ID,
					"claim":       fc.Claim,
					"operation":   "save_fact_check",
				})
				// Continue with other fact checks
			}
		}
	}

	// Mark job as completed
	err = w.analysisSvc.UpdateJobStatus(jobID, "completed", "")
	if err != nil {
		logger.LogErrorWithStack(err, map[string]interface{}{
			"job_id":    jobID,
			"operation": "update_job_status_completed",
		})
		return err
	}

	logger.Log.WithField("job_id", jobID).Info("Analysis complete. Results saved to database.")
	return nil
}

type AnalysisResults struct {
	Summary    string                 `json:"summary"`
	Takeaways  map[string]interface{} `json:"takeaways"`
	FactChecks []FactCheckResult      `json:"fact_checks"`
}

type FactCheckResult struct {
	Claim      string                 `json:"claim"`
	Verdict    string                 `json:"verdict"`
	Confidence float64                `json:"confidence"`
	Evidence   string                 `json:"evidence"`
	Sources    map[string]interface{} `json:"sources"`
}

func (w *AnalysisWorker) runAnalysisAgents(ctx context.Context, content string, jobID uuid.UUID) (*AnalysisResults, error) {
	// This is a placeholder for the actual AI agent processing
	// In the full implementation, this would call the equivalent of:
	// - SummarizerAgent
	// - TakeawayExtractorAgent  
	// - FactCheckerAgent
	
	logger.Log.WithField("job_id", jobID).Info("Running analysis agents (placeholder implementation)")
	
	// Simulate processing time
	time.Sleep(2 * time.Second)
	
	results := &AnalysisResults{
		Summary: "This is a placeholder summary generated by the Go worker. The actual implementation would use AI agents to analyze the transcript content.",
		Takeaways: map[string]interface{}{
			"takeaways": []string{
				"Placeholder takeaway 1",
				"Placeholder takeaway 2", 
				"Placeholder takeaway 3",
			},
		},
		FactChecks: []FactCheckResult{
			{
				Claim:      "Example factual claim from transcript",
				Verdict:    "unverifiable",
				Confidence: 0.8,
				Evidence:   "Placeholder evidence",
				Sources:    map[string]interface{}{"sources": []string{}},
			},
		},
	}
	
	return results, nil
}

func (w *AnalysisWorker) Run(ctx context.Context) error {
	logger.Log.Info("Starting analysis worker")
	
	w.running = true
	
	// Setup Kafka consumer
	consumer, err := w.kafkaService.CreateConsumer("analysis-workers")
	if err != nil {
		return err
	}
	defer consumer.Close()
	
	logger.Log.Info("Worker ready to process analysis jobs")
	
	for w.running {
		select {
		case <-ctx.Done():
			logger.Log.Info("Context cancelled, stopping worker")
			return ctx.Err()
		default:
			// Read message from Kafka
			message, err := consumer.ReadMessage(ctx)
			if err != nil {
				logger.LogErrorWithStack(err, map[string]interface{}{
					"operation": "kafka_read_message",
				})
				continue
			}
			
			// Parse message
			var jobMessage AnalysisJobMessage
			if err := json.Unmarshal(message.Value, &jobMessage); err != nil {
				logger.LogErrorWithStack(err, map[string]interface{}{
					"message_value": string(message.Value),
					"operation":     "parse_job_message",
				})
				continue
			}
			
			// Process the job
			if err := w.processAnalysisJob(ctx, jobMessage); err != nil {
				logger.LogErrorWithStack(err, map[string]interface{}{
					"job_message": jobMessage,
					"operation":   "process_analysis_job",
				})
			}
		}
	}
	
	return nil
}

func (w *AnalysisWorker) Stop() {
	logger.Log.Info("Stopping analysis worker")
	w.running = false
}

func main() {
	// Setup panic recovery
	defer func() {
		if r := recover(); r != nil {
			// Get stack trace
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			
			logger.Log.WithFields(map[string]interface{}{
				"panic":       r,
				"stack_trace": string(buf[:n]),
			}).Fatal("Worker application panicked")
		}
	}()

	logger.Log.Info("Starting Podcast Analyzer Analysis Worker")
	
	// Load configuration
	logger.Log.Info("Loading worker configuration")
	cfg, err := config.Load()
	if err != nil {
		logger.LogErrorWithStack(err, map[string]interface{}{
			"operation": "config_load",
		})
		logger.Log.WithError(err).Fatal("Failed to load worker configuration")
	}
	logger.Log.WithField("log_level", cfg.LogLevel).Info("Worker configuration loaded")

	// Set log level
	logger.SetLevel(cfg.LogLevel)
	
	// Connect to database
	logger.Log.WithField("database_url", maskDatabaseURL(cfg.DatabaseURL)).Info("Worker connecting to database")
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{})
	if err != nil {
		logger.LogErrorWithStack(err, map[string]interface{}{
			"operation":    "database_connect",
			"database_url": maskDatabaseURL(cfg.DatabaseURL),
		})
		logger.Log.WithError(err).Fatal("Failed to connect to database")
	}
	
	// Test database connection
	sqlDB, err := db.DB()
	if err != nil {
		logger.LogErrorWithStack(err, map[string]interface{}{
			"operation": "database_get_sql_instance",
		})
		logger.Log.WithError(err).Fatal("Failed to get database SQL instance")
	}
	
	if err := sqlDB.Ping(); err != nil {
		logger.LogErrorWithStack(err, map[string]interface{}{
			"operation": "database_ping",
		})
		logger.Log.WithError(err).Fatal("Failed to ping database")
	}
	logger.Log.Info("Worker database connected and pingable")

	// Initialize Kafka service
	logger.Log.WithFields(map[string]interface{}{
		"kafka_servers": cfg.KafkaBootstrapServers,
		"topic":         cfg.KafkaTopicAnalysis,
	}).Info("Worker initializing Kafka service")
	kafkaConfig := kafka.Config{
		BootstrapServers: cfg.KafkaBootstrapServers,
		Topic:           cfg.KafkaTopicAnalysis,
	}
	kafkaService := kafka.NewService(kafkaConfig)
	defer func() {
		logger.Log.Info("Closing worker Kafka service")
		if err := kafkaService.Close(); err != nil {
			logger.LogErrorWithStack(err, map[string]interface{}{
				"operation": "kafka_close",
			})
			logger.Log.WithError(err).Warn("Failed to close worker Kafka service")
		}
	}()
	logger.Log.Info("Worker Kafka service initialized")

	// Create worker
	logger.Log.Info("Creating analysis worker")
	worker := NewAnalysisWorker(db, cfg, kafkaService)
	logger.Log.Info("Analysis worker created")

	// Setup signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	go func() {
		<-sigChan
		logger.Log.Info("Worker shutdown signal received")
		worker.Stop()
		cancel()
	}()

	// Start worker
	logger.Log.Info("Starting analysis worker - waiting for jobs")
	
	if err := worker.Run(ctx); err != nil && err != context.Canceled {
		logger.LogErrorWithStack(err, map[string]interface{}{
			"operation": "worker_run",
		})
		logger.Log.WithError(err).Fatal("Worker failed")
	}
	
	logger.Log.Info("Analysis worker stopped gracefully")
}

// maskDatabaseURL masks sensitive information in database URL for logging
func maskDatabaseURL(dbURL string) string {
	if len(dbURL) > 20 {
		return dbURL[:10] + "***masked***" + dbURL[len(dbURL)-10:]
	}
	return "***masked***"
}