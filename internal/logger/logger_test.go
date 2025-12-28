package logger

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestSetLevel_Debug(t *testing.T) {
	originalLevel := Log.Level
	defer Log.SetLevel(originalLevel) // Restore original level

	SetLevel("DEBUG")
	assert.Equal(t, logrus.DebugLevel, Log.Level)
}

func TestSetLevel_Info(t *testing.T) {
	originalLevel := Log.Level
	defer Log.SetLevel(originalLevel)

	SetLevel("INFO")
	assert.Equal(t, logrus.InfoLevel, Log.Level)
}

func TestSetLevel_Warn(t *testing.T) {
	originalLevel := Log.Level
	defer Log.SetLevel(originalLevel)

	SetLevel("WARN")
	assert.Equal(t, logrus.WarnLevel, Log.Level)
}

func TestSetLevel_Error(t *testing.T) {
	originalLevel := Log.Level
	defer Log.SetLevel(originalLevel)

	SetLevel("ERROR")
	assert.Equal(t, logrus.ErrorLevel, Log.Level)
}

func TestSetLevel_Invalid_DefaultsToInfo(t *testing.T) {
	originalLevel := Log.Level
	defer Log.SetLevel(originalLevel)

	SetLevel("INVALID")
	assert.Equal(t, logrus.InfoLevel, Log.Level)
}

func TestSetLevel_Empty_DefaultsToInfo(t *testing.T) {
	originalLevel := Log.Level
	defer Log.SetLevel(originalLevel)

	SetLevel("")
	assert.Equal(t, logrus.InfoLevel, Log.Level)
}

func TestSetLevel_Lowercase(t *testing.T) {
	originalLevel := Log.Level
	defer Log.SetLevel(originalLevel)

	SetLevel("debug")
	// Should default to INFO since case-sensitive matching
	assert.Equal(t, logrus.InfoLevel, Log.Level)
}

func TestWithCorrelationID(t *testing.T) {
	correlationID := "test-correlation-123"
	
	entry := WithCorrelationID(correlationID)
	
	assert.NotNil(t, entry)
	assert.Equal(t, correlationID, entry.Data["correlation_id"])
}

func TestWithCorrelationID_EmptyString(t *testing.T) {
	entry := WithCorrelationID("")
	
	assert.NotNil(t, entry)
	assert.Equal(t, "", entry.Data["correlation_id"])
}

func TestGetStackTrace(t *testing.T) {
	stackTrace := GetStackTrace(0)
	
	assert.NotEmpty(t, stackTrace)
	// Stack trace should contain this test function name
	assert.Contains(t, stackTrace, "TestGetStackTrace")
	// Should contain the file name
	assert.Contains(t, stackTrace, "logger_test.go")
}

func TestGetStackTrace_WithSkip(t *testing.T) {
	// Helper function to test skip parameter
	getStackFromHelper := func() string {
		return GetStackTrace(1) // Skip 1 level (this helper function)
	}
	
	stackTrace := getStackFromHelper()
	
	assert.NotEmpty(t, stackTrace)
	// Should show the test function, not the helper
	assert.Contains(t, stackTrace, "TestGetStackTrace_WithSkip")
}

func TestLogErrorWithStack(t *testing.T) {
	// Capture log output
	var buffer bytes.Buffer
	originalOutput := Log.Out
	Log.SetOutput(&buffer)
	defer Log.SetOutput(originalOutput)
	
	// Set level to ensure error is logged
	originalLevel := Log.Level
	Log.SetLevel(logrus.ErrorLevel)
	defer Log.SetLevel(originalLevel)

	testError := errors.New("test error message")
	testFields := map[string]interface{}{
		"test_field": "test_value",
		"count":      42,
	}

	LogErrorWithStack(testError, testFields)

	// Parse the logged JSON
	logOutput := buffer.String()
	assert.NotEmpty(t, logOutput)

	var logEntry map[string]interface{}
	err := json.Unmarshal([]byte(logOutput), &logEntry)
	assert.NoError(t, err)

	// Verify error message
	assert.Equal(t, "test error message", logEntry["error"])
	
	// Verify custom fields
	assert.Equal(t, "test_value", logEntry["test_field"])
	assert.Equal(t, float64(42), logEntry["count"]) // JSON numbers are float64
	
	// Verify stack trace is present
	stackTrace, exists := logEntry["stack_trace"]
	assert.True(t, exists)
	assert.NotEmpty(t, stackTrace)
	
	// Verify log level
	assert.Equal(t, "error", logEntry["level"])
	
	// Verify message
	assert.Equal(t, "Error occurred", logEntry["msg"])
}

func TestLogErrorWithStack_NilFields(t *testing.T) {
	var buffer bytes.Buffer
	originalOutput := Log.Out
	Log.SetOutput(&buffer)
	defer Log.SetOutput(originalOutput)
	
	originalLevel := Log.Level
	Log.SetLevel(logrus.ErrorLevel)
	defer Log.SetLevel(originalLevel)

	testError := errors.New("test error with nil fields")

	LogErrorWithStack(testError, nil)

	logOutput := buffer.String()
	assert.NotEmpty(t, logOutput)

	var logEntry map[string]interface{}
	err := json.Unmarshal([]byte(logOutput), &logEntry)
	assert.NoError(t, err)

	assert.Equal(t, "test error with nil fields", logEntry["error"])
	assert.NotEmpty(t, logEntry["stack_trace"])
}

func TestLogErrorWithStackAndCorrelation(t *testing.T) {
	var buffer bytes.Buffer
	originalOutput := Log.Out
	Log.SetOutput(&buffer)
	defer Log.SetOutput(originalOutput)
	
	originalLevel := Log.Level
	Log.SetLevel(logrus.ErrorLevel)
	defer Log.SetLevel(originalLevel)

	testError := errors.New("test error with correlation")
	correlationID := "test-correlation-456"
	testFields := map[string]interface{}{
		"service": "test-service",
		"user_id": "user123",
	}

	LogErrorWithStackAndCorrelation(testError, correlationID, testFields)

	logOutput := buffer.String()
	assert.NotEmpty(t, logOutput)

	var logEntry map[string]interface{}
	err := json.Unmarshal([]byte(logOutput), &logEntry)
	assert.NoError(t, err)

	// Verify error message
	assert.Equal(t, "test error with correlation", logEntry["error"])
	
	// Verify correlation ID
	assert.Equal(t, correlationID, logEntry["correlation_id"])
	
	// Verify custom fields
	assert.Equal(t, "test-service", logEntry["service"])
	assert.Equal(t, "user123", logEntry["user_id"])
	
	// Verify stack trace
	stackTrace, exists := logEntry["stack_trace"]
	assert.True(t, exists)
	assert.NotEmpty(t, stackTrace)
	
	// Verify log level and message
	assert.Equal(t, "error", logEntry["level"])
	assert.Equal(t, "Error occurred", logEntry["msg"])
}

func TestLogErrorWithStackAndCorrelation_NilFields(t *testing.T) {
	var buffer bytes.Buffer
	originalOutput := Log.Out
	Log.SetOutput(&buffer)
	defer Log.SetOutput(originalOutput)
	
	originalLevel := Log.Level
	Log.SetLevel(logrus.ErrorLevel)
	defer Log.SetLevel(originalLevel)

	testError := errors.New("test error with correlation no fields")
	correlationID := "test-correlation-789"

	LogErrorWithStackAndCorrelation(testError, correlationID, nil)

	logOutput := buffer.String()
	assert.NotEmpty(t, logOutput)

	var logEntry map[string]interface{}
	err := json.Unmarshal([]byte(logOutput), &logEntry)
	assert.NoError(t, err)

	assert.Equal(t, "test error with correlation no fields", logEntry["error"])
	assert.Equal(t, correlationID, logEntry["correlation_id"])
	assert.NotEmpty(t, logEntry["stack_trace"])
}

func TestLogger_GlobalInstance(t *testing.T) {
	// Test that the global Log instance is properly initialized
	assert.NotNil(t, Log)
	assert.IsType(t, &logrus.Logger{}, Log)
	
	// Test formatter is JSONFormatter
	_, isJSONFormatter := Log.Formatter.(*logrus.JSONFormatter)
	assert.True(t, isJSONFormatter)
}

func TestLogger_JSONFormat(t *testing.T) {
	var buffer bytes.Buffer
	originalOutput := Log.Out
	Log.SetOutput(&buffer)
	defer Log.SetOutput(originalOutput)
	
	originalLevel := Log.Level
	Log.SetLevel(logrus.InfoLevel)
	defer Log.SetLevel(originalLevel)

	Log.Info("test message")

	logOutput := buffer.String()
	assert.NotEmpty(t, logOutput)

	// Should be valid JSON
	var logEntry map[string]interface{}
	err := json.Unmarshal([]byte(logOutput), &logEntry)
	assert.NoError(t, err)

	assert.Equal(t, "info", logEntry["level"])
	assert.Equal(t, "test message", logEntry["msg"])
	
	// Should have timestamp in expected format
	timestamp, exists := logEntry["time"]
	assert.True(t, exists)
	assert.NotEmpty(t, timestamp)
}

func TestLogger_CorrelationIDIntegration(t *testing.T) {
	var buffer bytes.Buffer
	originalOutput := Log.Out
	Log.SetOutput(&buffer)
	defer Log.SetOutput(originalOutput)
	
	originalLevel := Log.Level
	Log.SetLevel(logrus.InfoLevel)
	defer Log.SetLevel(originalLevel)

	correlationID := "integration-test-123"
	entry := WithCorrelationID(correlationID)
	entry.Info("test message with correlation")

	logOutput := buffer.String()
	assert.NotEmpty(t, logOutput)

	var logEntry map[string]interface{}
	err := json.Unmarshal([]byte(logOutput), &logEntry)
	assert.NoError(t, err)

	assert.Equal(t, "info", logEntry["level"])
	assert.Equal(t, "test message with correlation", logEntry["msg"])
	assert.Equal(t, correlationID, logEntry["correlation_id"])
}

func TestLogger_MultipleFields(t *testing.T) {
	var buffer bytes.Buffer
	originalOutput := Log.Out
	Log.SetOutput(&buffer)
	defer Log.SetOutput(originalOutput)
	
	originalLevel := Log.Level
	Log.SetLevel(logrus.InfoLevel)
	defer Log.SetLevel(originalLevel)

	Log.WithFields(logrus.Fields{
		"field1": "value1",
		"field2": 123,
		"field3": true,
	}).Info("test with multiple fields")

	logOutput := buffer.String()
	assert.NotEmpty(t, logOutput)

	var logEntry map[string]interface{}
	err := json.Unmarshal([]byte(logOutput), &logEntry)
	assert.NoError(t, err)

	assert.Equal(t, "value1", logEntry["field1"])
	assert.Equal(t, float64(123), logEntry["field2"])
	assert.Equal(t, true, logEntry["field3"])
}

func TestStackTrace_ContainsExpectedInformation(t *testing.T) {
	stackTrace := GetStackTrace(0)
	
	// Should contain function name
	assert.Contains(t, stackTrace, "TestStackTrace_ContainsExpectedInformation")
	
	// Should contain file name
	assert.Contains(t, stackTrace, "logger_test.go")
	
	// Should contain go runtime information
	assert.Contains(t, stackTrace, "goroutine")
}

func TestSetLevel_AllValidLevels(t *testing.T) {
	originalLevel := Log.Level
	defer Log.SetLevel(originalLevel)

	testCases := []struct {
		input    string
		expected logrus.Level
	}{
		{"DEBUG", logrus.DebugLevel},
		{"INFO", logrus.InfoLevel},
		{"WARN", logrus.WarnLevel},
		{"ERROR", logrus.ErrorLevel},
		{"TRACE", logrus.InfoLevel},    // Invalid, should default to INFO
		{"", logrus.InfoLevel},         // Empty, should default to INFO
		{"debug", logrus.InfoLevel},    // Lowercase, should default to INFO
		{"UnKnOwN", logrus.InfoLevel},  // Mixed case, should default to INFO
	}

	for _, tc := range testCases {
		t.Run("level_"+tc.input, func(t *testing.T) {
			SetLevel(tc.input)
			assert.Equal(t, tc.expected, Log.Level)
		})
	}
}