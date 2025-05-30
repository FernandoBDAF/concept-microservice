package domain

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

// ModelTester provides testing utilities for models
type ModelTester struct {
	t *testing.T
}

// NewModelTester creates a new ModelTester
func NewModelTester(t *testing.T) *ModelTester {
	return &ModelTester{t: t}
}

// TestValidation tests model validation
func (mt *ModelTester) TestValidation(model Validator, expectedError error) {
	err := model.Validate()
	if expectedError == nil {
		if err != nil {
			mt.t.Errorf("expected no error, got %v", err)
		}
		return
	}
	if err == nil {
		mt.t.Errorf("expected error %v, got nil", expectedError)
		return
	}
	if err.Error() != expectedError.Error() {
		mt.t.Errorf("expected error %v, got %v", expectedError, err)
	}
}

// TestSerialization tests model serialization
func (mt *ModelTester) TestSerialization(model Serializer) {
	// Test JSON serialization
	jsonData, err := model.ToJSON()
	if err != nil {
		mt.t.Errorf("failed to serialize to JSON: %v", err)
		return
	}

	// Create a new instance of the same type
	newModel := reflect.New(reflect.TypeOf(model).Elem()).Interface().(Serializer)
	if err := newModel.FromJSON(jsonData); err != nil {
		mt.t.Errorf("failed to deserialize from JSON: %v", err)
		return
	}

	// Compare the original and deserialized models
	if !reflect.DeepEqual(model, newModel) {
		mt.t.Errorf("deserialized model does not match original")
	}

	// Test XML serialization
	xmlData, err := model.ToXML()
	if err != nil {
		mt.t.Errorf("failed to serialize to XML: %v", err)
		return
	}

	// Create another new instance
	newModel = reflect.New(reflect.TypeOf(model).Elem()).Interface().(Serializer)
	if err := newModel.FromXML(xmlData); err != nil {
		mt.t.Errorf("failed to deserialize from XML: %v", err)
		return
	}

	// Compare again
	if !reflect.DeepEqual(model, newModel) {
		mt.t.Errorf("deserialized model does not match original")
	}
}

// TestConversion tests model conversion
func (mt *ModelTester) TestConversion(model Converter, target interface{}) {
	if err := model.ConvertTo(target); err != nil {
		mt.t.Errorf("failed to convert model: %v", err)
		return
	}

	newModel := reflect.New(reflect.TypeOf(model).Elem()).Interface().(Converter)
	if err := newModel.ConvertFrom(target); err != nil {
		mt.t.Errorf("failed to convert back: %v", err)
		return
	}

	if !reflect.DeepEqual(model, newModel) {
		mt.t.Errorf("converted model does not match original")
	}
}

// TestVersioning tests model versioning
func (mt *ModelTester) TestVersioning(model VersionedModel, targetVersion Version) {
	registry := NewMigrationRegistry()
	migrated, err := registry.Migrate(model, targetVersion)
	if err != nil {
		mt.t.Errorf("failed to migrate model: %v", err)
		return
	}

	migratedModel, ok := migrated.(VersionedModel)
	if !ok {
		mt.t.Errorf("migrated model does not implement VersionedModel")
		return
	}

	if migratedModel.GetVersion() != targetVersion {
		mt.t.Errorf("migrated model version %v does not match target version %v",
			migratedModel.GetVersion(), targetVersion)
	}
}

// TestDocumentation tests model documentation
func (mt *ModelTester) TestDocumentation(model interface{}, generator *DocumentationGenerator) {
	modelType := reflect.TypeOf(model)
	doc := generator.GenerateMarkdown(modelType)
	if doc == "" {
		mt.t.Errorf("failed to generate documentation for model")
	}
}

// TestEquality tests model equality
func (mt *ModelTester) TestEquality(model1, model2 interface{}) {
	if !reflect.DeepEqual(model1, model2) {
		mt.t.Errorf("models are not equal")
	}
}

// TestCloning tests model cloning
func (mt *ModelTester) TestCloning(model interface{}) {
	// Create a deep copy using JSON serialization
	jsonData, err := json.Marshal(model)
	if err != nil {
		mt.t.Errorf("failed to serialize model: %v", err)
		return
	}

	clone := reflect.New(reflect.TypeOf(model).Elem()).Interface()
	if err := json.Unmarshal(jsonData, clone); err != nil {
		mt.t.Errorf("failed to deserialize model: %v", err)
		return
	}

	if !reflect.DeepEqual(model, clone) {
		mt.t.Errorf("cloned model does not match original")
	}
}

// TestValidationRules tests validation rules
func (mt *ModelTester) TestValidationRules(rules ValidationRules, value interface{}, expectedError error) {
	var err error
	switch v := value.(type) {
	case string:
		err = ValidateString(v, rules)
	case time.Time:
		err = ValidateTime(v, rules)
	default:
		mt.t.Errorf("unsupported value type: %T", value)
		return
	}

	if expectedError == nil {
		if err != nil {
			mt.t.Errorf("expected no error, got %v", err)
		}
		return
	}
	if err == nil {
		mt.t.Errorf("expected error %v, got nil", expectedError)
		return
	}
	if err.Error() != expectedError.Error() {
		mt.t.Errorf("expected error %v, got %v", expectedError, err)
	}
}
