package logger

import (
	"os"
	"runtime"

	"github.com/sirupsen/logrus"
)

var Log *logrus.Logger

func init() {
	Log = logrus.New()
	Log.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
	})
	Log.SetOutput(os.Stdout)
}

// SetLevel sets the logging level
func SetLevel(level string) {
	switch level {
	case "DEBUG":
		Log.SetLevel(logrus.DebugLevel)
	case "INFO":
		Log.SetLevel(logrus.InfoLevel)
	case "WARN":
		Log.SetLevel(logrus.WarnLevel)
	case "ERROR":
		Log.SetLevel(logrus.ErrorLevel)
	default:
		Log.SetLevel(logrus.InfoLevel)
	}
}

// WithCorrelationID creates a logger with correlation ID
func WithCorrelationID(correlationID string) *logrus.Entry {
	return Log.WithField("correlation_id", correlationID)
}

// GetStackTrace captures the current stack trace
func GetStackTrace(skip int) string {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}

// LogErrorWithStack logs an error with stack trace
func LogErrorWithStack(err error, fields map[string]interface{}) {
	if fields == nil {
		fields = make(map[string]interface{})
	}
	fields["stack_trace"] = GetStackTrace(2) // Skip 2 levels: this function and the caller
	Log.WithFields(fields).WithError(err).Error("Error occurred")
}

// LogErrorWithStackAndCorrelation logs an error with stack trace and correlation ID
func LogErrorWithStackAndCorrelation(err error, correlationID string, fields map[string]interface{}) {
	if fields == nil {
		fields = make(map[string]interface{})
	}
	fields["correlation_id"] = correlationID
	fields["stack_trace"] = GetStackTrace(2) // Skip 2 levels: this function and the caller
	Log.WithFields(fields).WithError(err).Error("Error occurred")
}