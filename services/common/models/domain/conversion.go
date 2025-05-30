package domain

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// ConversionError represents a conversion error
type ConversionError struct {
	Message string
	Err     error
}

func (e *ConversionError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

// Converter interface defines methods for model conversion
type Converter interface {
	ConvertTo(target interface{}) error
	ConvertFrom(source interface{}) error
}

// BaseConverter provides common conversion functionality
type BaseConverter struct {
	Model interface{}
}

// ConvertTo converts the model to the target type
func (c *BaseConverter) ConvertTo(target interface{}) error {
	// First try direct conversion
	if err := c.directConvert(target); err == nil {
		return nil
	}

	// If direct conversion fails, try JSON conversion
	return c.jsonConvert(target)
}

// ConvertFrom converts from the source type to the model
func (c *BaseConverter) ConvertFrom(source interface{}) error {
	// First try direct conversion
	if err := c.directConvert(c.Model); err == nil {
		return nil
	}

	// If direct conversion fails, try JSON conversion
	return c.jsonConvert(c.Model)
}

// directConvert attempts to convert directly using reflection
func (c *BaseConverter) directConvert(target interface{}) error {
	sourceValue := reflect.ValueOf(c.Model)
	targetValue := reflect.ValueOf(target)

	if targetValue.Kind() != reflect.Ptr {
		return &ConversionError{
			Message: "target must be a pointer",
		}
	}

	targetValue = targetValue.Elem()
	if !targetValue.CanSet() {
		return &ConversionError{
			Message: "target is not settable",
		}
	}

	if sourceValue.Type() == targetValue.Type() {
		targetValue.Set(sourceValue)
		return nil
	}

	return &ConversionError{
		Message: "direct conversion not possible",
	}
}

// jsonConvert converts using JSON as an intermediate format
func (c *BaseConverter) jsonConvert(target interface{}) error {
	// Convert source to JSON
	jsonData, err := json.Marshal(c.Model)
	if err != nil {
		return &ConversionError{
			Message: "failed to marshal source to JSON",
			Err:     err,
		}
	}

	// Convert JSON to target
	if err := json.Unmarshal(jsonData, target); err != nil {
		return &ConversionError{
			Message: "failed to unmarshal JSON to target",
			Err:     err,
		}
	}

	return nil
}

// ConversionRegistry stores conversion functions for different model types
type ConversionRegistry struct {
	converters map[reflect.Type]map[reflect.Type]func(interface{}) (interface{}, error)
}

// NewConversionRegistry creates a new ConversionRegistry
func NewConversionRegistry() *ConversionRegistry {
	return &ConversionRegistry{
		converters: make(map[reflect.Type]map[reflect.Type]func(interface{}) (interface{}, error)),
	}
}

// RegisterConverter registers a conversion function for a specific source and target type
func (r *ConversionRegistry) RegisterConverter(sourceType, targetType reflect.Type, converter func(interface{}) (interface{}, error)) {
	if _, exists := r.converters[sourceType]; !exists {
		r.converters[sourceType] = make(map[reflect.Type]func(interface{}) (interface{}, error))
	}
	r.converters[sourceType][targetType] = converter
}

// Convert converts a model to the target type using registered converters
func (r *ConversionRegistry) Convert(source interface{}, target interface{}) error {
	sourceType := reflect.TypeOf(source)
	targetType := reflect.TypeOf(target)

	if targetType.Kind() != reflect.Ptr {
		return &ConversionError{
			Message: "target must be a pointer",
		}
	}

	targetType = targetType.Elem()
	converters, exists := r.converters[sourceType]
	if !exists {
		return &ConversionError{
			Message: fmt.Sprintf("no converters found for source type %v", sourceType),
		}
	}

	converter, exists := converters[targetType]
	if !exists {
		return &ConversionError{
			Message: fmt.Sprintf("no converter found from %v to %v", sourceType, targetType),
		}
	}

	result, err := converter(source)
	if err != nil {
		return &ConversionError{
			Message: "conversion failed",
			Err:     err,
		}
	}

	reflect.ValueOf(target).Elem().Set(reflect.ValueOf(result))
	return nil
}
