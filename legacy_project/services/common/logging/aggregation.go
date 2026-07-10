package logging

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// AggregationConfig defines the configuration for log aggregation
type AggregationConfig struct {
	// Enabled determines if log aggregation is enabled
	Enabled bool
	// BufferSize is the size of the log buffer
	BufferSize int
	// FlushInterval is the interval between buffer flushes
	FlushInterval time.Duration
	// MaxBatchSize is the maximum number of logs to send in a batch
	MaxBatchSize int
}

// DefaultAggregationConfig returns a default aggregation configuration
func DefaultAggregationConfig() *AggregationConfig {
	return &AggregationConfig{
		Enabled:       true,
		BufferSize:    1000,
		FlushInterval: time.Second * 5,
		MaxBatchSize:  100,
	}
}

// AggregationLogger is a logger that aggregates logs before sending them
type AggregationLogger struct {
	logger *Logger
	config *AggregationConfig
	mu     sync.Mutex
	buffer []*LogEntry
}

// NewAggregationLogger creates a new aggregation logger
func NewAggregationLogger(logger *Logger, config *AggregationConfig) *AggregationLogger {
	if config == nil {
		config = DefaultAggregationConfig()
	}

	aggLogger := &AggregationLogger{
		logger: logger,
		config: config,
		buffer: make([]*LogEntry, 0, config.BufferSize),
	}

	if config.Enabled {
		go aggLogger.flusher()
	}

	return aggLogger
}

// flusher periodically flushes the log buffer
func (l *AggregationLogger) flusher() {
	ticker := time.NewTicker(l.config.FlushInterval)
	defer ticker.Stop()

	for range ticker.C {
		l.Flush()
	}
}

// Flush sends all buffered logs
func (l *AggregationLogger) Flush() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.buffer) == 0 {
		return
	}

	// Send logs in batches
	for i := 0; i < len(l.buffer); i += l.config.MaxBatchSize {
		end := i + l.config.MaxBatchSize
		if end > len(l.buffer) {
			end = len(l.buffer)
		}

		batch := l.buffer[i:end]
		l.sendBatch(batch)
	}

	// Clear the buffer
	l.buffer = l.buffer[:0]
}

// sendBatch sends a batch of logs
func (l *AggregationLogger) sendBatch(batch []*LogEntry) {
	// Convert batch to fields
	fields := make([]zap.Field, 0, len(batch)*2)
	for _, entry := range batch {
		fields = append(fields, zap.String("message", entry.Message))
		fields = append(fields, zap.Any("fields", entry.Fields))
	}

	// Log the batch
	l.logger.Info("Log batch", fields...)
}

// WithContext returns a new aggregation logger with the given context
func (l *AggregationLogger) WithContext(ctx context.Context) *AggregationLogger {
	return &AggregationLogger{
		logger: l.logger.WithContext(ctx),
		config: l.config,
		buffer: make([]*LogEntry, 0, l.config.BufferSize),
	}
}

// WithFields returns a new aggregation logger with the given fields
func (l *AggregationLogger) WithFields(fields map[string]interface{}) *AggregationLogger {
	return &AggregationLogger{
		logger: l.logger.WithFields(fields),
		config: l.config,
		buffer: make([]*LogEntry, 0, l.config.BufferSize),
	}
}

// Debug logs a debug message
func (l *AggregationLogger) Debug(msg string, fields ...zap.Field) {
	l.log(DebugLevel, msg, fields...)
}

// Info logs an info message
func (l *AggregationLogger) Info(msg string, fields ...zap.Field) {
	l.log(InfoLevel, msg, fields...)
}

// Warn logs a warning message
func (l *AggregationLogger) Warn(msg string, fields ...zap.Field) {
	l.log(WarnLevel, msg, fields...)
}

// Error logs an error message
func (l *AggregationLogger) Error(msg string, fields ...zap.Field) {
	l.log(ErrorLevel, msg, fields...)
}

// Fatal logs a fatal message
func (l *AggregationLogger) Fatal(msg string, fields ...zap.Field) {
	l.log(FatalLevel, msg, fields...)
}

// With creates a child logger with the given fields
func (l *AggregationLogger) With(fields ...zap.Field) *AggregationLogger {
	return &AggregationLogger{
		logger: l.logger.With(fields...),
		config: l.config,
		buffer: make([]*LogEntry, 0, l.config.BufferSize),
	}
}

// Sync flushes any buffered log entries
func (l *AggregationLogger) Sync() error {
	l.Flush()
	return l.logger.Sync()
}

// log adds a log entry to the buffer
func (l *AggregationLogger) log(level LogLevel, msg string, fields ...zap.Field) {
	entry := &LogEntry{
		Level:     level,
		Message:   msg,
		Timestamp: time.Now(),
		Fields:    make(map[string]interface{}),
	}

	// Convert zap fields to map
	for _, field := range fields {
		entry.Fields[field.Key] = field.Interface
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Add to buffer
	l.buffer = append(l.buffer, entry)

	// Flush if buffer is full
	if len(l.buffer) >= l.config.BufferSize {
		l.Flush()
	}
}
