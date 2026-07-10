package logging

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// MetricsConfig defines the configuration for performance metrics logging
type MetricsConfig struct {
	// Enabled determines if metrics logging is enabled
	Enabled bool
	// Interval is the interval between metrics collection
	Interval time.Duration
	// IncludeMemory determines if memory metrics should be included
	IncludeMemory bool
	// IncludeGoroutines determines if goroutine metrics should be included
	IncludeGoroutines bool
	// IncludeGC determines if garbage collection metrics should be included
	IncludeGC bool
}

// DefaultMetricsConfig returns a default metrics configuration
func DefaultMetricsConfig() *MetricsConfig {
	return &MetricsConfig{
		Enabled:           true,
		Interval:          time.Minute,
		IncludeMemory:     true,
		IncludeGoroutines: true,
		IncludeGC:         true,
	}
}

// MetricsLogger is a logger that records metrics
type MetricsLogger struct {
	logger *Logger
}

// NewMetricsLogger creates a new metrics logger
func NewMetricsLogger(logger *Logger) *MetricsLogger {
	return &MetricsLogger{
		logger: logger,
	}
}

// RecordMetric records a metric
func (l *MetricsLogger) RecordMetric(name string, value float64, labels map[string]string) {
	fields := make([]zap.Field, 0, len(labels)+2)
	fields = append(fields, zap.String("metric_name", name))
	fields = append(fields, zap.Float64("value", value))
	for k, v := range labels {
		fields = append(fields, zap.String(k, v))
	}
	l.logger.Info("Metric recorded", fields...)
}

// RecordLatency records a latency metric
func (l *MetricsLogger) RecordLatency(operation string, duration time.Duration, labels map[string]string) {
	fields := make([]zap.Field, 0, len(labels)+2)
	fields = append(fields, zap.String("operation", operation))
	fields = append(fields, zap.Duration("duration", duration))
	for k, v := range labels {
		fields = append(fields, zap.String(k, v))
	}
	l.logger.Info("Latency recorded", fields...)
}

// RecordError records an error metric
func (l *MetricsLogger) RecordError(operation string, err error, labels map[string]string) {
	fields := make([]zap.Field, 0, len(labels)+2)
	fields = append(fields, zap.String("operation", operation))
	fields = append(fields, zap.Error(err))
	for k, v := range labels {
		fields = append(fields, zap.String(k, v))
	}
	l.logger.Error("Error recorded", fields...)
}

// RecordCounter records a counter metric
func (l *MetricsLogger) RecordCounter(name string, value int64, labels map[string]string) {
	fields := make([]zap.Field, 0, len(labels)+2)
	fields = append(fields, zap.String("counter_name", name))
	fields = append(fields, zap.Int64("value", value))
	for k, v := range labels {
		fields = append(fields, zap.String(k, v))
	}
	l.logger.Info("Counter recorded", fields...)
}

// WithContext returns a new metrics logger with the given context
func (l *MetricsLogger) WithContext(ctx context.Context) *MetricsLogger {
	return &MetricsLogger{
		logger: l.logger.WithContext(ctx),
	}
}

// WithFields returns a new metrics logger with the given fields
func (l *MetricsLogger) WithFields(fields map[string]interface{}) *MetricsLogger {
	return &MetricsLogger{
		logger: l.logger.WithFields(fields),
	}
}

// Debug logs a debug message
func (l *MetricsLogger) Debug(msg string, fields ...zap.Field) {
	l.logger.Debug(msg, fields...)
}

// Info logs an info message
func (l *MetricsLogger) Info(msg string, fields ...zap.Field) {
	l.logger.Info(msg, fields...)
}

// Warn logs a warning message
func (l *MetricsLogger) Warn(msg string, fields ...zap.Field) {
	l.logger.Warn(msg, fields...)
}

// Error logs an error message
func (l *MetricsLogger) Error(msg string, fields ...zap.Field) {
	l.logger.Error(msg, fields...)
}

// Fatal logs a fatal message
func (l *MetricsLogger) Fatal(msg string, fields ...zap.Field) {
	l.logger.Fatal(msg, fields...)
}

// With creates a child logger with the given fields
func (l *MetricsLogger) With(fields ...zap.Field) *MetricsLogger {
	return &MetricsLogger{
		logger: l.logger.With(fields...),
	}
}

// Sync flushes any buffered log entries
func (l *MetricsLogger) Sync() error {
	return l.logger.Sync()
}
