package config

import (
	"fmt"
	"strings"
)

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationRule defines a validation rule function
type ValidationRule func(value interface{}) error

// ValidationRules maps field names to their validation rules
type ValidationRules map[string][]ValidationRule

// Required validates that a field is not empty
func Required() ValidationRule {
	return func(value interface{}) error {
		if value == nil {
			return &ValidationError{
				Message: "field is required",
			}
		}

		switch v := value.(type) {
		case string:
			if strings.TrimSpace(v) == "" {
				return &ValidationError{
					Message: "field cannot be empty",
				}
			}
		case []interface{}:
			if len(v) == 0 {
				return &ValidationError{
					Message: "field cannot be empty",
				}
			}
		case map[string]interface{}:
			if len(v) == 0 {
				return &ValidationError{
					Message: "field cannot be empty",
				}
			}
		}

		return nil
	}
}

// MinLength validates that a string field has a minimum length
func MinLength(length int) ValidationRule {
	return func(value interface{}) error {
		str, ok := value.(string)
		if !ok {
			return &ValidationError{
				Message: "field must be a string",
			}
		}

		if len(str) < length {
			return &ValidationError{
				Message: fmt.Sprintf("field must be at least %d characters long", length),
			}
		}

		return nil
	}
}

// MaxLength validates that a string field has a maximum length
func MaxLength(length int) ValidationRule {
	return func(value interface{}) error {
		str, ok := value.(string)
		if !ok {
			return &ValidationError{
				Message: "field must be a string",
			}
		}

		if len(str) > length {
			return &ValidationError{
				Message: fmt.Sprintf("field must be at most %d characters long", length),
			}
		}

		return nil
	}
}

// MinValue validates that a numeric field has a minimum value
func MinValue(min float64) ValidationRule {
	return func(value interface{}) error {
		var num float64

		switch v := value.(type) {
		case float64:
			num = v
		case int:
			num = float64(v)
		default:
			return &ValidationError{
				Message: "field must be a number",
			}
		}

		if num < min {
			return &ValidationError{
				Message: fmt.Sprintf("field must be at least %v", min),
			}
		}

		return nil
	}
}

// MaxValue validates that a numeric field has a maximum value
func MaxValue(max float64) ValidationRule {
	return func(value interface{}) error {
		var num float64

		switch v := value.(type) {
		case float64:
			num = v
		case int:
			num = float64(v)
		default:
			return &ValidationError{
				Message: "field must be a number",
			}
		}

		if num > max {
			return &ValidationError{
				Message: fmt.Sprintf("field must be at most %v", max),
			}
		}

		return nil
	}
}

// Validate validates the configuration against the provided rules
func (c *Config) Validate(rules ValidationRules) error {
	var errors []error

	// Validate environment
	if envRules, ok := rules["environment"]; ok {
		for _, rule := range envRules {
			if err := rule(c.Environment); err != nil {
				if ve, ok := err.(*ValidationError); ok {
					ve.Field = "environment"
				}
				errors = append(errors, err)
			}
		}
	}

	// Validate version
	if versionRules, ok := rules["version"]; ok {
		for _, rule := range versionRules {
			if err := rule(c.Version); err != nil {
				if ve, ok := err.(*ValidationError); ok {
					ve.Field = "version"
				}
				errors = append(errors, err)
			}
		}
	}

	// Validate settings
	for field, value := range c.Settings {
		if fieldRules, ok := rules[field]; ok {
			for _, rule := range fieldRules {
				if err := rule(value); err != nil {
					if ve, ok := err.(*ValidationError); ok {
						ve.Field = field
					}
					errors = append(errors, err)
				}
			}
		}
	}

	if len(errors) > 0 {
		return &ValidationError{
			Message: fmt.Sprintf("validation failed: %v", errors),
		}
	}

	return nil
}
