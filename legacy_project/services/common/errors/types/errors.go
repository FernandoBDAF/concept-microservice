package types

import (
	"fmt"
	"time"
)

// ErrorCode represents a standardized error code
type ErrorCode string

// ErrorSeverity represents the severity level of an error
type ErrorSeverity string

const (
	// Error severities
	SeverityLow    ErrorSeverity = "LOW"
	SeverityMedium ErrorSeverity = "MEDIUM"
	SeverityHigh   ErrorSeverity = "HIGH"
	SeverityFatal  ErrorSeverity = "FATAL"

	// Common error codes
	ErrCodeInternal     ErrorCode = "INTERNAL_ERROR"
	ErrCodeValidation   ErrorCode = "VALIDATION_ERROR"
	ErrCodeNotFound     ErrorCode = "NOT_FOUND"
	ErrCodeUnauthorized ErrorCode = "UNAUTHORIZED"
	ErrCodeForbidden    ErrorCode = "FORBIDDEN"
	ErrCodeTimeout      ErrorCode = "TIMEOUT"
	ErrCodeConflict     ErrorCode = "CONFLICT"
)

// Error represents the base error interface
type Error interface {
	error
	Code() ErrorCode
	Severity() ErrorSeverity
	Timestamp() time.Time
	TraceID() string
	WithTraceID(traceID string) Error
	WithSeverity(severity ErrorSeverity) Error
	WithCause(cause error) Error
	Cause() error
}

// BaseError implements the Error interface
type BaseError struct {
	code      ErrorCode
	message   string
	severity  ErrorSeverity
	timestamp time.Time
	traceID   string
	cause     error
}

// New creates a new BaseError
func New(code ErrorCode, message string) *BaseError {
	return &BaseError{
		code:      code,
		message:   message,
		severity:  SeverityMedium,
		timestamp: time.Now(),
	}
}

// Error implements the error interface
func (e *BaseError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", e.message, e.cause)
	}
	return e.message
}

// Code returns the error code
func (e *BaseError) Code() ErrorCode {
	return e.code
}

// Severity returns the error severity
func (e *BaseError) Severity() ErrorSeverity {
	return e.severity
}

// Timestamp returns the error timestamp
func (e *BaseError) Timestamp() time.Time {
	return e.timestamp
}

// TraceID returns the error trace ID
func (e *BaseError) TraceID() string {
	return e.traceID
}

// WithTraceID sets the trace ID and returns the error
func (e *BaseError) WithTraceID(traceID string) Error {
	e.traceID = traceID
	return e
}

// WithSeverity sets the severity and returns the error
func (e *BaseError) WithSeverity(severity ErrorSeverity) Error {
	e.severity = severity
	return e
}

// WithCause sets the underlying cause and returns the error
func (e *BaseError) WithCause(cause error) Error {
	e.cause = cause
	return e
}

// Cause returns the underlying cause of the error
func (e *BaseError) Cause() error {
	return e.cause
}
