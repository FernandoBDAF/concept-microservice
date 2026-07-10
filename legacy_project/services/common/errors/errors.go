package errors

import (
	"fmt"
)

// Error represents a custom error
type Error struct {
	Message string
	Code    string
	Err     error
}

// New creates a new error
func New(message string) error {
	return &Error{
		Message: message,
	}
}

// Wrap wraps an error with a message
func Wrap(err error, message string) error {
	if err == nil {
		return nil
	}
	return &Error{
		Message: message,
		Err:     err,
	}
}

// Error implements the error interface
func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// Unwrap returns the wrapped error
func (e *Error) Unwrap() error {
	return e.Err
}

// IsNotFound checks if the error is a not found error
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	if e, ok := err.(*Error); ok {
		return e.Code == "not_found"
	}
	return false
}

// IsValidation checks if the error is a validation error
func IsValidation(err error) bool {
	if err == nil {
		return false
	}
	if e, ok := err.(*Error); ok {
		return e.Code == "validation"
	}
	return false
}
