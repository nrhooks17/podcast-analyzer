package main

import (
	"context"
	"net/http"
	"os/signal"
	"syscall"
	"time"
	"backend-golang/internal/config"
	"backend-golang/internal/handlers"
	"backend-golang/internal/middleware"
	"backend-golang/internal/models"
	"backend-golang/internal/services"
	"backend-golang/pkg/kafka"
	"backend-golang/pkg/logger"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	// Setup panic recovery
	defer func() {
		if r := recover(); r != nil {
			logger.Log.WithFields(map[string]interface{}{
				"panic":       r,
				"stack_trace": logger.GetStackTrace(0),
			}).Fatal("Application panicked")
		}
	}()

	logger.Log.Info("Starting Podcast Analyzer Go Backend Server")
	
	// Load configuration
	logger.Log.Info("Loading configuration")
	cfg, err := config.Load()
	if err != nil {
		logger.LogErrorWithStack(err, map[string]interface{}{
			"operation": "config_load",
		})
		logger.Log.WithError(err).Fatal("Failed to load configuration")
	}
	logger.Log.WithField("log_level", cfg.LogLevel).Info("Configuration loaded successfully")

	// Set log level
	logger.SetLevel(cfg.LogLevel)

	// Connect to database
	logger.Log.WithField("database_url", maskDatabaseURL(cfg.DatabaseURL)).Info("Connecting to database")
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
	logger.Log.Info("Database connected and pingable")

	// Auto-migrate database schema
	logger.Log.Info("Running database migrations")
	if err := models.AutoMigrate(db); err != nil {
		logger.LogErrorWithStack(err, map[string]interface{}{
			"operation": "database_migrate",
		})
		logger.Log.WithError(err).Fatal("Failed to migrate database")
	}
	logger.Log.Info("Database migrations completed")

	// Initialize Kafka service
	logger.Log.WithField("kafka_servers", cfg.KafkaBootstrapServers).Info("Initializing Kafka service")
	kafkaConfig := kafka.Config{
		BootstrapServers: cfg.KafkaBootstrapServers,
		Topic:           cfg.KafkaTopicAnalysis,
	}
	kafkaService := kafka.NewService(kafkaConfig)
	defer func() {
		logger.Log.Info("Closing Kafka service")
		if err := kafkaService.Close(); err != nil {
			logger.LogErrorWithStack(err, map[string]interface{}{
				"operation": "kafka_close",
			})
			logger.Log.WithError(err).Warn("Failed to close Kafka service")
		}
	}()
	logger.Log.WithField("topic", cfg.KafkaTopicAnalysis).Info("Kafka service initialized")

	// Initialize services
	logger.Log.Info("Initializing services")
	transcriptService := services.NewTranscriptService(db, cfg)
	analysisService := services.NewAnalysisService(db, cfg, kafkaService)
	logger.Log.Info("Services initialized")

	// Initialize handlers
	logger.Log.Info("Initializing handlers")
	transcriptHandler := handlers.NewTranscriptHandler(transcriptService)
	analysisHandler := handlers.NewAnalysisHandler(analysisService)
	logger.Log.Info("Handlers initialized")

	// Setup router
	logger.Log.Info("Setting up router")
	router := setupRouter(cfg, transcriptHandler, analysisHandler)
	logger.Log.Info("Router configured")

	// Create HTTP server
	server := &http.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Setup graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start server in a goroutine
	go func() {
		logger.Log.WithFields(map[string]interface{}{
			"port":        cfg.ServerPort,
			"health_url":  "http://localhost:" + cfg.ServerPort + "/health",
		}).Info("Starting Go backend server")
		
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.LogErrorWithStack(err, map[string]interface{}{
				"operation": "server_listen",
				"port":      cfg.ServerPort,
			})
			logger.Log.WithError(err).Fatal("Failed to start server")
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	<-ctx.Done()
	stop()
	logger.Log.Info("Shutdown signal received, starting graceful shutdown")

	// Give outstanding requests 30 seconds to complete
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Log.WithError(err).Fatal("Server forced to shutdown")
	}

	logger.Log.Info("Server gracefully stopped")
}

// maskDatabaseURL masks sensitive information in database URL for logging
func maskDatabaseURL(dbURL string) string {
	// Simple masking - replace password with asterisks
	// More sophisticated parsing could be added if needed
	if len(dbURL) > 20 {
		return dbURL[:10] + "***masked***" + dbURL[len(dbURL)-10:]
	}
	return "***masked***"
}

func setupRouter(cfg *config.Config, transcriptHandler *handlers.TranscriptHandler, analysisHandler *handlers.AnalysisHandler) *gin.Engine {
	// Set Gin mode
	if cfg.LogLevel == "DEBUG" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Add middleware
	router.Use(middleware.CORSMiddleware(cfg.CORSOrigins))
	router.Use(middleware.RequestIDMiddleware())
	router.Use(middleware.LoggingMiddleware())
	router.Use(gin.Recovery())

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"service": "podcast-analyzer-go",
			"version": "1.0.0",
		})
	})

	// API routes
	api := router.Group("/api")
	{
		// Transcript routes
		transcripts := api.Group("/transcripts")
		{
			transcripts.POST("/", transcriptHandler.UploadTranscript)
			transcripts.GET("/", transcriptHandler.GetTranscripts)
			transcripts.GET("/:id", transcriptHandler.GetTranscript)
			transcripts.DELETE("/:id", transcriptHandler.DeleteTranscript)
		}

		// Analysis routes
		api.POST("/analyze/:transcript_id", analysisHandler.StartAnalysis)
		
		jobs := api.Group("/jobs")
		{
			jobs.GET("/:job_id/status", analysisHandler.GetJobStatus)
		}

		results := api.Group("/results")
		{
			results.GET("/", analysisHandler.ListAnalysisResults)
			results.GET("/:analysis_id", analysisHandler.GetAnalysisResults)
		}
	}

	return router
}