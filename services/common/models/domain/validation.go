package domain

import (
	"fmt"
	"regexp"
	"time"
)

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// Validator interface defines methods for model validation
type Validator interface {
	Validate() error
}

// ValidationRules contains common validation rules
type ValidationRules struct {
	Required    bool
	MinLength   int
	MaxLength   int
	Pattern     string
	MinValue    float64
	MaxValue    float64
	CustomRules []func(interface{}) error
}

// ValidateString validates a string against the given rules
func ValidateString(value string, rules ValidationRules) error {
	if rules.Required && value == "" {
		return &ValidationError{
			Field:   "string",
			Message: "value is required",
		}
	}

	if rules.MinLength > 0 && len(value) < rules.MinLength {
		return &ValidationError{
			Field:   "string",
			Message: fmt.Sprintf("length must be at least %d", rules.MinLength),
		}
	}

	if rules.MaxLength > 0 && len(value) > rules.MaxLength {
		return &ValidationError{
			Field:   "string",
			Message: fmt.Sprintf("length must be at most %d", rules.MaxLength),
		}
	}

	if rules.Pattern != "" {
		matched, err := regexp.MatchString(rules.Pattern, value)
		if err != nil {
			return &ValidationError{
				Field:   "string",
				Message: fmt.Sprintf("invalid pattern: %v", err),
			}
		}
		if !matched {
			return &ValidationError{
				Field:   "string",
				Message: "value does not match required pattern",
			}
		}
	}

	for _, rule := range rules.CustomRules {
		if err := rule(value); err != nil {
			return &ValidationError{
				Field:   "string",
				Message: err.Error(),
			}
		}
	}

	return nil
}

// ValidateTime validates a time value against the given rules
func ValidateTime(value time.Time, rules ValidationRules) error {
	if rules.Required && value.IsZero() {
		return &ValidationError{
			Field:   "time",
			Message: "value is required",
		}
	}

	for _, rule := range rules.CustomRules {
		if err := rule(value); err != nil {
			return &ValidationError{
				Field:   "time",
				Message: err.Error(),
			}
		}
	}

	return nil
}

// Common validation patterns
var (
	EmailPattern    = `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
	URLPattern      = `^(https?:\/\/)?([\da-z\.-]+)\.([a-z\.]{2,6})([\/\w \.-]*)*\/?$`
	PhonePattern    = `^\+?[1-9]\d{1,14}$`
	UsernamePattern = `^[a-zA-Z0-9_-]{3,16}$`
)

// Common validation rules
var (
	EmailRules = ValidationRules{
		Required:  true,
		MinLength: 3,
		MaxLength: 254,
		Pattern:   EmailPattern,
	}

	URLRules = ValidationRules{
		Required:  true,
		MinLength: 3,
		MaxLength: 2048,
		Pattern:   URLPattern,
	}

	PhoneRules = ValidationRules{
		Required:  true,
		MinLength: 10,
		MaxLength: 15,
		Pattern:   PhonePattern,
	}

	UsernameRules = ValidationRules{
		Required:  true,
		MinLength: 3,
		MaxLength: 16,
		Pattern:   UsernamePattern,
	}
)
