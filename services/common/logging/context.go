package logging

import (
	"context"
)

type contextKey string

const (
	// LoggerContextKey is the key used to store the logger in the context
	LoggerContextKey contextKey = "logger"
	// FieldsContextKey is the key used to store additional fields in the context
	FieldsContextKey contextKey = "log_fields"
)

// WithLogger adds a logger to the context
func WithLogger(ctx context.Context, logger Logger) context.Context {
	return context.WithValue(ctx, LoggerContextKey, logger)
}

// FromContext retrieves the logger from the context
func FromContext(ctx context.Context) (Logger, bool) {
	logger, ok := ctx.Value(LoggerContextKey).(Logger)
	return logger, ok
}

// WithFields adds fields to the context that will be included in all log entries
func WithFields(ctx context.Context, fields map[string]interface{}) context.Context {
	existingFields, _ := ctx.Value(FieldsContextKey).(map[string]interface{})
	if existingFields == nil {
		existingFields = make(map[string]interface{})
	}

	// Merge new fields with existing fields
	for k, v := range fields {
		existingFields[k] = v
	}

	return context.WithValue(ctx, FieldsContextKey, existingFields)
}

// GetFields retrieves the fields from the context
func GetFields(ctx context.Context) map[string]interface{} {
	fields, _ := ctx.Value(FieldsContextKey).(map[string]interface{})
	if fields == nil {
		fields = make(map[string]interface{})
	}
	return fields
}
