package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"
)

// StructuredLogger implements the Logger interface with structured logging
type StructuredLogger struct {
	mu     sync.RWMutex
	fields map[string]interface{}
	output *json.Encoder
}

// NewStructuredLogger creates a new structured logger
func NewStructuredLogger() *StructuredLogger {
	return &StructuredLogger{
		fields: make(map[string]interface{}),
		output: json.NewEncoder(os.Stdout),
	}
}

// log writes a log entry with the given level and message
func (l *StructuredLogger) log(ctx context.Context, level LogLevel, msg string, fields ...map[string]interface{}) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	// Create base log entry
	entry := map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"level":     level.String(),
		"message":   msg,
	}

	// Add context fields
	ctxFields := GetFields(ctx)
	for k, v := range ctxFields {
		entry[k] = v
	}

	// Add logger fields
	for k, v := range l.fields {
		entry[k] = v
	}

	// Add additional fields
	for _, f := range fields {
		for k, v := range f {
			entry[k] = v
		}
	}

	// Encode and write the log entry
	if err := l.output.Encode(entry); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to encode log entry: %v\n", err)
	}
}

// Debug implements Logger.Debug
func (l *StructuredLogger) Debug(msg string, fields ...zap.Field) {
	l.log(context.Background(), DebugLevel, msg, convertZapFields(fields))
}

// Info implements Logger.Info
func (l *StructuredLogger) Info(msg string, fields ...zap.Field) {
	l.log(context.Background(), InfoLevel, msg, convertZapFields(fields))
}

// Warn implements Logger.Warn
func (l *StructuredLogger) Warn(msg string, fields ...zap.Field) {
	l.log(context.Background(), WarnLevel, msg, convertZapFields(fields))
}

// Error implements Logger.Error
func (l *StructuredLogger) Error(msg string, fields ...zap.Field) {
	l.log(context.Background(), ErrorLevel, msg, convertZapFields(fields))
}

// Fatal implements Logger.Fatal
func (l *StructuredLogger) Fatal(msg string, fields ...zap.Field) {
	l.log(context.Background(), FatalLevel, msg, convertZapFields(fields))
	os.Exit(1)
}

// WithFields implements Logger.WithFields
func (l *StructuredLogger) WithFields(fields map[string]interface{}) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()

	newLogger := &StructuredLogger{
		fields: make(map[string]interface{}),
		output: l.output,
	}

	// Copy existing fields
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}

	// Add new fields
	for k, v := range fields {
		newLogger.fields[k] = v
	}

	return &Logger{}
}

// WithContext implements Logger.WithContext
func (l *StructuredLogger) WithContext(ctx context.Context) *Logger {
	return &Logger{}
}

// With creates a child logger with the given fields
func (l *StructuredLogger) With(fields ...zap.Field) *Logger {
	return &Logger{}
}

// Sync flushes any buffered log entries
func (l *StructuredLogger) Sync() error {
	return nil
}

// convertZapFields converts zap.Field to map[string]interface{}
func convertZapFields(fields []zap.Field) map[string]interface{} {
	result := make(map[string]interface{}, len(fields))
	for _, field := range fields {
		result[field.Key] = field.Interface
	}
	return result
}
