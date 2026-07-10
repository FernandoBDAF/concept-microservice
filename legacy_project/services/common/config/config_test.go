package config

import (
	"os"
	"testing"
)

func TestConfig_LoadConfig(t *testing.T) {
	// Create a temporary JSON config file
	jsonContent := `{
		"environment": "test",
		"version": "1.0.0",
		"settings": {
			"database": {
				"host": "localhost",
				"port": 5432
			},
			"api": {
				"port": 8080,
				"timeout": 30
			}
		}
	}`
	tmpFile := createTempFile(t, jsonContent, ".json")
	defer os.Remove(tmpFile.Name())

	// Test loading JSON config
	config, err := LoadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify basic fields
	if config.Environment != "test" {
		t.Errorf("Expected environment 'test', got '%s'", config.Environment)
	}
	if config.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got '%s'", config.Version)
	}

	// Verify settings
	dbHost := config.GetStringMap("database")["host"]
	if dbHost != "localhost" {
		t.Errorf("Expected database host 'localhost', got '%v'", dbHost)
	}

	dbPort := config.GetStringMap("database")["port"]
	if dbPort != float64(5432) {
		t.Errorf("Expected database port 5432, got '%v'", dbPort)
	}

	apiPort := config.GetStringMap("api")["port"]
	if apiPort != float64(8080) {
		t.Errorf("Expected API port 8080, got '%v'", apiPort)
	}
}

func TestConfig_EnvironmentVariables(t *testing.T) {
	// Set environment variables
	os.Setenv("APP_ENV", "prod")
	os.Setenv("APP_VERSION", "2.0.0")
	os.Setenv("APP_SETTING_DATABASE_HOST", "db.example.com")
	defer func() {
		os.Unsetenv("APP_ENV")
		os.Unsetenv("APP_VERSION")
		os.Unsetenv("APP_SETTING_DATABASE_HOST")
	}()

	// Create a temporary config file
	jsonContent := `{
		"environment": "test",
		"version": "1.0.0",
		"settings": {
			"database": {
				"host": "localhost",
				"port": 5432
			}
		}
	}`
	tmpFile := createTempFile(t, jsonContent, ".json")
	defer os.Remove(tmpFile.Name())

	// Load config
	config, err := LoadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify environment variable overrides
	if config.Environment != "prod" {
		t.Errorf("Expected environment 'prod', got '%s'", config.Environment)
	}
	if config.Version != "2.0.0" {
		t.Errorf("Expected version '2.0.0', got '%s'", config.Version)
	}

	dbHost := config.GetStringMap("database")["host"]
	if dbHost != "db.example.com" {
		t.Errorf("Expected database host 'db.example.com', got '%v'", dbHost)
	}
}

func TestConfig_Validation(t *testing.T) {
	// Create a temporary config file
	jsonContent := `{
		"environment": "test",
		"version": "1.0.0",
		"settings": {
			"name": "test-app",
			"port": 8080,
			"timeout": 30
		}
	}`
	tmpFile := createTempFile(t, jsonContent, ".json")
	defer os.Remove(tmpFile.Name())

	// Load config
	config, err := LoadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Define validation rules
	rules := ValidationRules{
		"environment": {Required()},
		"version":     {Required()},
		"name":        {Required(), MinLength(3), MaxLength(50)},
		"port":        {Required(), MinValue(1), MaxValue(65535)},
		"timeout":     {Required(), MinValue(1), MaxValue(300)},
	}

	// Test validation
	err = config.Validate(rules)
	if err != nil {
		t.Errorf("Validation failed: %v", err)
	}

	// Test validation failure
	config.Settings["port"] = 70000
	err = config.Validate(rules)
	if err == nil {
		t.Error("Expected validation to fail for port > 65535")
	}
}

func createTempFile(t *testing.T, content string, ext string) *os.File {
	tmpFile, err := os.CreateTemp("", "config-*"+ext)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}

	if err := tmpFile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	return tmpFile
}
