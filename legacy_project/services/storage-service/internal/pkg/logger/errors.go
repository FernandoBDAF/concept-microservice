package logger

import (
	"fmt"
)

// Error types
var (
	ErrLoggerNotInitialized = NewLogError("logger not initialized")
	ErrInvalidConfig        = NewLogError("invalid logger configuration")
	ErrLogRotationFailed    = NewLogError("log rotation failed")
	ErrLogCleanupFailed     = NewLogError("log cleanup failed")
	ErrLogWriteFailed       = NewLogError("log write failed")
)

// LogError represents a logging error
type LogError struct {
	message string
	cause   error
}

// NewLogError creates a new logging error
func NewLogError(message string) *LogError {
	return &LogError{
		message: message,
	}
}

// WrapError wraps an existing error with a message
func WrapError(err error, message string) *LogError {
	return &LogError{
		message: message,
		cause:   err,
	}
}

// Error returns the error message
func (e *LogError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", e.message, e.cause)
	}
	return e.message
}

// Unwrap returns the cause of the error
func (e *LogError) Unwrap() error {
	return e.cause
}

// Is checks if the error is of a specific type
func (e *LogError) Is(target error) bool {
	t, ok := target.(*LogError)
	if !ok {
		return false
	}
	return e.message == t.message
}

// WithCause adds a cause to the error
func (e *LogError) WithCause(cause error) *LogError {
	e.cause = cause
	return e
}

// IsLoggingError checks if an error is a logging error
func IsLoggingError(err error) bool {
	_, ok := err.(*LogError)
	return ok
}

// GetLoggingError returns the logging error if the error is a logging error
func GetLoggingError(err error) *LogError {
	if e, ok := err.(*LogError); ok {
		return e
	}
	return nil
}
