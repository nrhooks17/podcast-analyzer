package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Helper function to set up environment variables for tests
func setTestEnv(envVars map[string]string) func() {
	originalEnv := make(map[string]string)
	
	// Store original values and set test values
	for key, value := range envVars {
		if original := os.Getenv(key); original != "" {
			originalEnv[key] = original
		}
		os.Setenv(key, value)
	}
	
	// Return cleanup function
	return func() {
		for key := range envVars {
			if original, exists := originalEnv[key]; exists {
				os.Setenv(key, original)
			} else {
				os.Unsetenv(key)
			}
		}
	}
}

func TestLoad_Success_WithDefaults(t *testing.T) {
	// Set up minimum required environment
	cleanup := setTestEnv(map[string]string{
		"ANTHROPIC_API_KEY": "test-api-key",
	})
	defer cleanup()

	cfg, err := Load()

	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	
	// Test required fields
	assert.Equal(t, "test-api-key", cfg.AnthropicAPIKey)
	
	// Test defaults
	assert.Equal(t, "postgresql://postgres:postgres@localhost:5432/podcast_analyzer", cfg.DatabaseURL)
	assert.Equal(t, "", cfg.SerperAPIKey) // Not set, should be empty
	assert.Equal(t, "/app/storage/transcripts", cfg.StoragePath)
	assert.Equal(t, int64(10*1024*1024), cfg.MaxFileSize)
	assert.Equal(t, []string{".txt", ".json"}, cfg.AllowedExts)
	assert.Equal(t, "8000", cfg.ServerPort)
	assert.Equal(t, "INFO", cfg.LogLevel)
	assert.Equal(t, "claude-sonnet-4-20250514", cfg.ClaudeModel)
	assert.Equal(t, 150, cfg.SummaryMaxChars)
	assert.Equal(t, 300, cfg.SummaryMaxWords)
	assert.Equal(t, 200, cfg.SummaryMinWords)
	
	// Test CORS origins default
	assert.Equal(t, []string{"http://localhost:3000"}, cfg.CORSOrigins)
}

func TestLoad_Success_WithCustomValues(t *testing.T) {
	cleanup := setTestEnv(map[string]string{
		"ANTHROPIC_API_KEY": "custom-api-key",
		"SERPER_API_KEY":    "custom-serper-key",
		"DATABASE_URL":      "postgresql://custom:custom@custom:5432/custom_db",
		"STORAGE_PATH":      "/custom/storage/path",
		"SERVER_PORT":       "9000",
		"LOG_LEVEL":         "DEBUG",
		"CORS_ORIGINS":      "http://localhost:3000,http://example.com,https://app.example.com",
	})
	defer cleanup()

	cfg, err := Load()

	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	
	// Test custom values
	assert.Equal(t, "custom-api-key", cfg.AnthropicAPIKey)
	assert.Equal(t, "custom-serper-key", cfg.SerperAPIKey)
	assert.Equal(t, "postgresql://custom:custom@custom:5432/custom_db", cfg.DatabaseURL)
	assert.Equal(t, "/custom/storage/path", cfg.StoragePath)
	assert.Equal(t, "9000", cfg.ServerPort)
	assert.Equal(t, "DEBUG", cfg.LogLevel)
	
	// Test CORS origins parsing
	expectedOrigins := []string{
		"http://localhost:3000",
		"http://example.com", 
		"https://app.example.com",
	}
	assert.Equal(t, expectedOrigins, cfg.CORSOrigins)
}

func TestLoad_Failure_MissingAnthropicAPIKey(t *testing.T) {
	// Clear any existing ANTHROPIC_API_KEY
	cleanup := setTestEnv(map[string]string{})
	defer cleanup()
	
	originalKey := os.Getenv("ANTHROPIC_API_KEY")
	os.Unsetenv("ANTHROPIC_API_KEY")
	defer func() {
		if originalKey != "" {
			os.Setenv("ANTHROPIC_API_KEY", originalKey)
		}
	}()

	cfg, err := Load()

	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "ANTHROPIC_API_KEY is required")
}

func TestLoad_CORSOrigins_SingleOrigin(t *testing.T) {
	cleanup := setTestEnv(map[string]string{
		"ANTHROPIC_API_KEY": "test-key",
		"CORS_ORIGINS":      "https://production.example.com",
	})
	defer cleanup()

	cfg, err := Load()

	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, []string{"https://production.example.com"}, cfg.CORSOrigins)
}

func TestLoad_CORSOrigins_MultipleOriginsWithSpaces(t *testing.T) {
	cleanup := setTestEnv(map[string]string{
		"ANTHROPIC_API_KEY": "test-key",
		"CORS_ORIGINS":      " http://localhost:3000 , https://staging.example.com , https://production.example.com ",
	})
	defer cleanup()

	cfg, err := Load()

	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	
	expectedOrigins := []string{
		"http://localhost:3000",
		"https://staging.example.com",
		"https://production.example.com",
	}
	assert.Equal(t, expectedOrigins, cfg.CORSOrigins)
}

func TestLoad_CORSOrigins_EmptyString(t *testing.T) {
	cleanup := setTestEnv(map[string]string{
		"ANTHROPIC_API_KEY": "test-key",
		"CORS_ORIGINS":      "",
	})
	defer cleanup()

	cfg, err := Load()

	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	// Should use default when empty
	assert.Equal(t, []string{"http://localhost:3000"}, cfg.CORSOrigins)
}

func TestLoad_AllEnvironmentVariables(t *testing.T) {
	cleanup := setTestEnv(map[string]string{
		"ANTHROPIC_API_KEY": "test-anthropic-key",
		"SERPER_API_KEY":    "test-serper-key",
		"DATABASE_URL":      "postgresql://testuser:testpass@testhost:5432/testdb",
		"STORAGE_PATH":      "/test/storage",
		"SERVER_PORT":       "8080",
		"LOG_LEVEL":         "ERROR",
		"CORS_ORIGINS":      "http://test1.com,http://test2.com",
	})
	defer cleanup()

	cfg, err := Load()

	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	
	// Verify all environment variables are correctly loaded
	assert.Equal(t, "test-anthropic-key", cfg.AnthropicAPIKey)
	assert.Equal(t, "test-serper-key", cfg.SerperAPIKey)
	assert.Equal(t, "postgresql://testuser:testpass@testhost:5432/testdb", cfg.DatabaseURL)
	assert.Equal(t, "/test/storage", cfg.StoragePath)
	assert.Equal(t, "8080", cfg.ServerPort)
	assert.Equal(t, "ERROR", cfg.LogLevel)
	assert.Equal(t, []string{"http://test1.com", "http://test2.com"}, cfg.CORSOrigins)
	
	// Verify hardcoded values remain unchanged
	assert.Equal(t, int64(10*1024*1024), cfg.MaxFileSize)
	assert.Equal(t, []string{".txt", ".json"}, cfg.AllowedExts)
	assert.Equal(t, "claude-sonnet-4-20250514", cfg.ClaudeModel)
	assert.Equal(t, 150, cfg.SummaryMaxChars)
	assert.Equal(t, 300, cfg.SummaryMaxWords)
	assert.Equal(t, 200, cfg.SummaryMinWords)
}

func TestGetEnvWithDefault_ExistingValue(t *testing.T) {
	cleanup := setTestEnv(map[string]string{
		"TEST_KEY": "test_value",
	})
	defer cleanup()

	result := getEnvWithDefault("TEST_KEY", "default_value")
	assert.Equal(t, "test_value", result)
}

func TestGetEnvWithDefault_MissingValue(t *testing.T) {
	// Ensure the key is not set
	os.Unsetenv("MISSING_TEST_KEY")

	result := getEnvWithDefault("MISSING_TEST_KEY", "default_value")
	assert.Equal(t, "default_value", result)
}

func TestGetEnvWithDefault_EmptyValue(t *testing.T) {
	cleanup := setTestEnv(map[string]string{
		"EMPTY_TEST_KEY": "",
	})
	defer cleanup()

	result := getEnvWithDefault("EMPTY_TEST_KEY", "default_value")
	assert.Equal(t, "default_value", result)
}

func TestConfig_StructFields(t *testing.T) {
	cfg := &Config{
		DatabaseURL:      "test_db_url",
		AnthropicAPIKey:  "test_anthropic_key",
		SerperAPIKey:     "test_serper_key",
		StoragePath:      "/test/path",
		MaxFileSize:      1024,
		AllowedExts:      []string{".test"},
		ServerPort:       "9000",
		LogLevel:         "TEST",
		CORSOrigins:      []string{"http://test.com"},
		ClaudeModel:      "test-model",
		SummaryMaxChars:  100,
		SummaryMaxWords:  200,
		SummaryMinWords:  50,
	}

	// Test that all fields can be set and accessed
	assert.Equal(t, "test_db_url", cfg.DatabaseURL)
	assert.Equal(t, "test_anthropic_key", cfg.AnthropicAPIKey)
	assert.Equal(t, "test_serper_key", cfg.SerperAPIKey)
	assert.Equal(t, "/test/path", cfg.StoragePath)
	assert.Equal(t, int64(1024), cfg.MaxFileSize)
	assert.Equal(t, []string{".test"}, cfg.AllowedExts)
	assert.Equal(t, "9000", cfg.ServerPort)
	assert.Equal(t, "TEST", cfg.LogLevel)
	assert.Equal(t, []string{"http://test.com"}, cfg.CORSOrigins)
	assert.Equal(t, "test-model", cfg.ClaudeModel)
	assert.Equal(t, 100, cfg.SummaryMaxChars)
	assert.Equal(t, 200, cfg.SummaryMaxWords)
	assert.Equal(t, 50, cfg.SummaryMinWords)
}

func TestLoad_HardcodedValues(t *testing.T) {
	cleanup := setTestEnv(map[string]string{
		"ANTHROPIC_API_KEY": "test-key",
	})
	defer cleanup()

	cfg, err := Load()

	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	
	// Test that hardcoded values are set correctly and can't be overridden by environment
	assert.Equal(t, int64(10*1024*1024), cfg.MaxFileSize) // 10MB
	assert.Equal(t, []string{".txt", ".json"}, cfg.AllowedExts)
	assert.Equal(t, "claude-sonnet-4-20250514", cfg.ClaudeModel)
	assert.Equal(t, 150, cfg.SummaryMaxChars)
	assert.Equal(t, 300, cfg.SummaryMaxWords)
	assert.Equal(t, 200, cfg.SummaryMinWords)
}

func TestLoad_PartialEnvironment(t *testing.T) {
	// Test with only some environment variables set
	cleanup := setTestEnv(map[string]string{
		"ANTHROPIC_API_KEY": "test-key",
		"SERVER_PORT":       "9999",
		"LOG_LEVEL":         "WARN",
	})
	defer cleanup()

	cfg, err := Load()

	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	
	// Set values should use environment
	assert.Equal(t, "test-key", cfg.AnthropicAPIKey)
	assert.Equal(t, "9999", cfg.ServerPort)
	assert.Equal(t, "WARN", cfg.LogLevel)
	
	// Unset values should use defaults
	assert.Equal(t, "postgresql://postgres:postgres@localhost:5432/podcast_analyzer", cfg.DatabaseURL)
	assert.Equal(t, "", cfg.SerperAPIKey)
	assert.Equal(t, "/app/storage/transcripts", cfg.StoragePath)
	assert.Equal(t, []string{"http://localhost:3000"}, cfg.CORSOrigins)
}