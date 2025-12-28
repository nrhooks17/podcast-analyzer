package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os/signal"
	"syscall"
	"time"
	"podcast-analyzer/internal/config"
	"podcast-analyzer/internal/handlers"
	"podcast-analyzer/internal/middleware"
	"podcast-analyzer/internal/models"
	"podcast-analyzer/internal/services"
	"podcast-analyzer/internal/logger"
	"podcast-analyzer/internal/utils"

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
	cfg := loadConfiguration()
	
	// Initialize database
	db := initializeDatabase(cfg)
	
	// Initialize services
	transcriptService, analysisService := initializeServices(db, cfg)

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
	server := setupServer(cfg, router)
	
	// Start server with graceful shutdown
	runWithGracefulShutdown(server, cfg)
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

// writeError writes a standardized error response
func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	})
}

// healthHandler handles the health check endpoint
func healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	response := map[string]interface{}{
		"status":  "healthy",
		"service": "podcast-analyzer-go",
		"version": "1.0.0",
	}
	json.NewEncoder(w).Encode(response)
}

// transcriptsHandler handles /api/transcripts endpoint routing
func transcriptsHandler(transcriptHandler *handlers.TranscriptHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			transcriptHandler.GetTranscripts(w, r)
		} else if r.Method == http.MethodPost {
			transcriptHandler.UploadTranscript(w, r)
		} else if r.Method == http.MethodOptions {
			// Handle preflight request
			utils.SetCORSHeaders(w)
			w.WriteHeader(http.StatusNoContent)
		} else {
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		}
	}
}

// transcriptsWithIDHandler handles /api/transcripts/ endpoint routing
func transcriptsWithIDHandler(transcriptHandler *handlers.TranscriptHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			transcriptHandler.UploadTranscript(w, r)
		} else if r.Method == http.MethodGet {
			transcriptHandler.GetTranscript(w, r)
		} else if r.Method == http.MethodDelete {
			transcriptHandler.DeleteTranscript(w, r)
		} else if r.Method == http.MethodOptions {
			// Handle preflight request
			utils.SetCORSHeaders(w)
			w.WriteHeader(http.StatusNoContent)
		} else {
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		}
	}
}

// analysisResultsHandler handles /api/results endpoint routing
func analysisResultsHandler(analysisHandler *handlers.AnalysisHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodPost {
			analysisHandler.ListAnalysisResults(w, r)
		} else if r.Method == http.MethodOptions {
			// Handle preflight request
			utils.SetCORSHeaders(w)
			w.WriteHeader(http.StatusNoContent)
		} else {
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		}
	}
}

// analysisResultsWithIDHandler handles /api/results/ endpoint routing
func analysisResultsWithIDHandler(analysisHandler *handlers.AnalysisHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			analysisHandler.GetAnalysisResults(w, r)
		} else {
			writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		}
	}
}

func setupRouter(cfg *config.Config, transcriptHandler *handlers.TranscriptHandler, analysisHandler *handlers.AnalysisHandler) http.Handler {
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", healthHandler)

	// Register handlers with proper routing
	mux.HandleFunc("/api/transcripts", transcriptsHandler(transcriptHandler))
	mux.HandleFunc("/api/transcripts/", transcriptsWithIDHandler(transcriptHandler))
	mux.HandleFunc("/api/analyze/", analysisHandler.StartAnalysis)
	mux.HandleFunc("/api/jobs/", analysisHandler.GetJobStatus)
	mux.HandleFunc("/api/results", analysisResultsHandler(analysisHandler))
	mux.HandleFunc("/api/results/", analysisResultsWithIDHandler(analysisHandler))

	// Chain middleware - CORS is handled directly in utils.SetCORSHeaders
	handler := middleware.RequestIDMiddleware()(mux)
	handler = middleware.LoggingMiddleware()(handler)
	handler = middleware.RecoveryMiddleware()(handler)

	return handler
}

// loadConfiguration loads and validates the application configuration
func loadConfiguration() *config.Config {
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
	
	return cfg
}

// initializeDatabase connects to the database, tests connection, and runs migrations
func initializeDatabase(cfg *config.Config) *gorm.DB {
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
	
	return db
}

// initializeServices creates and returns the application services
func initializeServices(db *gorm.DB, cfg *config.Config) (*services.TranscriptService, *services.AnalysisService) {
	logger.Log.Info("Initializing services")
	transcriptService := services.NewTranscriptService(db, cfg)
	analysisService := services.NewAnalysisService(db, cfg)
	logger.Log.Info("Services initialized")
	
	return transcriptService, analysisService
}

// setupServer creates an HTTP server with proper timeouts
func setupServer(cfg *config.Config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
}

// runWithGracefulShutdown starts the server and handles graceful shutdown
func runWithGracefulShutdown(server *http.Server, cfg *config.Config) {
	// Setup graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start server in a goroutine
	go func() {
		logger.Log.WithFields(map[string]interface{}{
			"port":       cfg.ServerPort,
			"health_url": "http://localhost:" + cfg.ServerPort + "/health",
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