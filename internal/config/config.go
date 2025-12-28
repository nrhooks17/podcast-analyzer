package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the application
type Config struct {
	// Database configuration
	DatabaseURL string

	// Anthropic API configuration
	AnthropicAPIKey string

	// Serper API configuration for web search
	SerperAPIKey string


	// File storage configuration
	StoragePath   string
	MaxFileSize   int64
	AllowedExts   []string

	// Server configuration
	ServerPort string
	LogLevel   string

	// CORS configuration
	CORSOrigins []string

	// AI model configuration
	ClaudeModel       string
	SummaryMaxChars   int
	SummaryMaxWords   int
	SummaryMinWords   int
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	// Try to load .env file, but don't fail if it doesn't exist
	_ = godotenv.Load()

	cfg := &Config{
		DatabaseURL:           getEnvWithDefault("DATABASE_URL", "postgresql://postgres:postgres@localhost:5432/podcast_analyzer"),
		AnthropicAPIKey:       os.Getenv("ANTHROPIC_API_KEY"),
		SerperAPIKey:          os.Getenv("SERPER_API_KEY"),
		StoragePath:           getEnvWithDefault("STORAGE_PATH", "/app/storage/transcripts"),
		MaxFileSize:           10 * 1024 * 1024, // 10MB
		AllowedExts:           []string{".txt", ".json"},
		ServerPort:            getEnvWithDefault("SERVER_PORT", "8000"), // Different port from Python backend
		LogLevel:              getEnvWithDefault("LOG_LEVEL", "INFO"),
		ClaudeModel:           "claude-sonnet-4-20250514",
		SummaryMaxChars:       150,  // For social media posts
		SummaryMaxWords:       300,
		SummaryMinWords:       200,
	}

	// Parse CORS origins
	corsOriginsStr := getEnvWithDefault("CORS_ORIGINS", "http://localhost:3000")
	cfg.CORSOrigins = strings.Split(corsOriginsStr, ",")
	for i := range cfg.CORSOrigins {
		cfg.CORSOrigins[i] = strings.TrimSpace(cfg.CORSOrigins[i])
	}

	// Validate required configuration
	if cfg.AnthropicAPIKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY is required")
	}

	return cfg, nil
}

func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

