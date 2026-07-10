package utils

import (
	"fmt"
	"strings"

	"github.com/FBDAF/microservices/services/common/errors/types"
)

// Wrap wraps an error with additional context
func Wrap(err error, code types.ErrorCode, message string) types.Error {
	if err == nil {
		return nil
	}

	// If the error is already our custom error type, just add the new message
	if customErr, ok := err.(types.Error); ok {
		return customErr.WithCause(fmt.Errorf(message))
	}

	// Create a new error with the provided code and message
	return types.New(code, message).WithCause(err)
}

// Wrapf wraps an error with additional context using format string
func Wrapf(err error, code types.ErrorCode, format string, args ...interface{}) types.Error {
	return Wrap(err, code, fmt.Sprintf(format, args...))
}

// Unwrap returns the underlying error
func Unwrap(err error) error {
	if customErr, ok := err.(types.Error); ok {
		return customErr.Cause()
	}
	return err
}

// Is checks if the error is of a specific type
func Is(err error, code types.ErrorCode) bool {
	if customErr, ok := err.(types.Error); ok {
		return customErr.Code() == code
	}
	return false
}

// GetStackTrace returns a formatted stack trace for the error
func GetStackTrace(err error) string {
	var stackTrace strings.Builder
	current := err

	for current != nil {
		stackTrace.WriteString(fmt.Sprintf("%v\n", current))
		current = Unwrap(current)
	}

	return stackTrace.String()
}

// WithSeverity wraps an error with a specific severity level
func WithSeverity(err error, severity types.ErrorSeverity) types.Error {
	if err == nil {
		return nil
	}

	if customErr, ok := err.(types.Error); ok {
		return customErr.WithSeverity(severity)
	}

	return types.New(types.ErrCodeInternal, err.Error()).WithSeverity(severity)
}

// WithTraceID wraps an error with a trace ID
func WithTraceID(err error, traceID string) types.Error {
	if err == nil {
		return nil
	}

	if customErr, ok := err.(types.Error); ok {
		return customErr.WithTraceID(traceID)
	}

	return types.New(types.ErrCodeInternal, err.Error()).WithTraceID(traceID)
}
