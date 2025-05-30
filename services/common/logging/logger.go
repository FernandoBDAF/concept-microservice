package logging

import (
	"context"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LogLevel represents the severity level of a log entry
type LogLevel int

const (
	// DebugLevel represents debug level logs
	DebugLevel LogLevel = iota
	// InfoLevel represents info level logs
	InfoLevel
	// WarnLevel represents warning level logs
	WarnLevel
	// ErrorLevel represents error level logs
	ErrorLevel
	// FatalLevel represents fatal level logs
	FatalLevel
)

// String returns the string representation of the log level
func (l LogLevel) String() string {
	switch l {
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	case FatalLevel:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// LogEntry represents a single log entry with all its metadata
type LogEntry struct {
	Timestamp time.Time
	Level     LogLevel
	Message   string
	Fields    map[string]interface{}
	Context   context.Context
}

// Logger is a wrapper around zap.Logger
type Logger struct {
	*zap.Logger
}

// New creates a new logger
func New() (*Logger, error) {
	config := zap.NewProductionConfig()
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	logger, err := config.Build()
	if err != nil {
		return nil, err
	}

	return &Logger{logger}, nil
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, fields ...zap.Field) {
	l.Logger.Debug(msg, fields...)
}

// Info logs an info message
func (l *Logger) Info(msg string, fields ...zap.Field) {
	l.Logger.Info(msg, fields...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, fields ...zap.Field) {
	l.Logger.Warn(msg, fields...)
}

// Error logs an error message
func (l *Logger) Error(msg string, fields ...zap.Field) {
	l.Logger.Error(msg, fields...)
}

// Fatal logs a fatal message and then calls os.Exit(1)
func (l *Logger) Fatal(msg string, fields ...zap.Field) {
	l.Logger.Fatal(msg, fields...)
}

// With creates a child logger with the given fields
func (l *Logger) With(fields ...zap.Field) *Logger {
	return &Logger{l.Logger.With(fields...)}
}

// Sync flushes any buffered log entries
func (l *Logger) Sync() error {
	return l.Logger.Sync()
}

// WithContext returns a new logger with the given context
func (l *Logger) WithContext(ctx context.Context) *Logger {
	return l
}

// WithFields returns a new logger with the given fields
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	zapFields := make([]zap.Field, 0, len(fields))
	for k, v := range fields {
		zapFields = append(zapFields, zap.Any(k, v))
	}
	return &Logger{l.Logger.With(zapFields...)}
}

// Logger defines the interface for logging operations
type LoggerInterface interface {
	// Debug logs a message at debug level
	Debug(ctx context.Context, msg string, fields ...map[string]interface{})
	// Info logs a message at info level
	Info(ctx context.Context, msg string, fields ...map[string]interface{})
	// Warn logs a message at warning level
	Warn(ctx context.Context, msg string, fields ...map[string]interface{})
	// Error logs a message at error level
	Error(ctx context.Context, msg string, fields ...map[string]interface{})
	// Fatal logs a message at fatal level and then calls os.Exit(1)
	Fatal(ctx context.Context, msg string, fields ...map[string]interface{})
	// WithFields returns a new logger with the given fields added to all future log entries
	WithFields(fields map[string]interface{}) LoggerInterface
	// WithContext returns a new logger with the given context
	WithContext(ctx context.Context) LoggerInterface
}
