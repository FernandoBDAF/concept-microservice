package config

import (
	"testing"
)

func TestVersion(t *testing.T) {
	// Test valid version
	version, err := NewVersion("1.2.3")
	if err != nil {
		t.Fatalf("Failed to create version: %v", err)
	}

	if version.Major != 1 || version.Minor != 2 || version.Patch != 3 {
		t.Errorf("Expected version 1.2.3, got %d.%d.%d",
			version.Major, version.Minor, version.Patch)
	}

	// Test string representation
	if version.String() != "1.2.3" {
		t.Errorf("Expected '1.2.3', got '%s'", version.String())
	}

	// Test invalid version format
	_, err = NewVersion("1.2")
	if err == nil {
		t.Error("Expected error for invalid version format")
	}

	// Test invalid version numbers
	_, err = NewVersion("1.a.3")
	if err == nil {
		t.Error("Expected error for invalid version numbers")
	}
}

func TestVersion_Compare(t *testing.T) {
	tests := []struct {
		name     string
		v1       string
		v2       string
		expected int
	}{
		{"equal", "1.2.3", "1.2.3", 0},
		{"less major", "1.2.3", "2.2.3", -1},
		{"greater major", "2.2.3", "1.2.3", 1},
		{"less minor", "1.1.3", "1.2.3", -1},
		{"greater minor", "1.2.3", "1.1.3", 1},
		{"less patch", "1.2.2", "1.2.3", -1},
		{"greater patch", "1.2.3", "1.2.2", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v1, err := NewVersion(tt.v1)
			if err != nil {
				t.Fatalf("Failed to create version %s: %v", tt.v1, err)
			}

			v2, err := NewVersion(tt.v2)
			if err != nil {
				t.Fatalf("Failed to create version %s: %v", tt.v2, err)
			}

			result := v1.Compare(v2)
			if result != tt.expected {
				t.Errorf("Compare(%s, %s) = %d, want %d",
					tt.v1, tt.v2, result, tt.expected)
			}
		})
	}
}

func TestVersion_IsCompatible(t *testing.T) {
	tests := []struct {
		name     string
		v1       string
		v2       string
		expected bool
	}{
		{"same version", "1.2.3", "1.2.3", true},
		{"same major", "1.2.3", "1.3.0", true},
		{"different major", "1.2.3", "2.0.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v1, err := NewVersion(tt.v1)
			if err != nil {
				t.Fatalf("Failed to create version %s: %v", tt.v1, err)
			}

			v2, err := NewVersion(tt.v2)
			if err != nil {
				t.Fatalf("Failed to create version %s: %v", tt.v2, err)
			}

			result := v1.IsCompatible(v2)
			if result != tt.expected {
				t.Errorf("IsCompatible(%s, %s) = %v, want %v",
					tt.v1, tt.v2, result, tt.expected)
			}
		})
	}
}

func TestVersionManager(t *testing.T) {
	// Create version manager
	vm, err := NewVersionManager("1.0.0")
	if err != nil {
		t.Fatalf("Failed to create version manager: %v", err)
	}

	// Test current version
	if vm.GetCurrentVersion() != "1.0.0" {
		t.Errorf("Expected current version '1.0.0', got '%s'",
			vm.GetCurrentVersion())
	}

	// Create test config
	config := &Config{
		Version: "0.9.0",
		Settings: map[string]interface{}{
			"old_setting": "value",
		},
	}

	// Register migration
	err = vm.RegisterMigration("1.0.0", func(cfg *Config) error {
		// Rename old_setting to new_setting
		if value, exists := cfg.Settings["old_setting"]; exists {
			cfg.Settings["new_setting"] = value
			delete(cfg.Settings, "old_setting")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to register migration: %v", err)
	}

	// Test migration
	err = vm.Migrate(config)
	if err != nil {
		t.Fatalf("Failed to migrate config: %v", err)
	}

	// Verify migration
	if config.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got '%s'", config.Version)
	}

	if _, exists := config.Settings["old_setting"]; exists {
		t.Error("Expected old_setting to be removed")
	}

	if value, exists := config.Settings["new_setting"]; !exists || value != "value" {
		t.Error("Expected new_setting to be set to 'value'")
	}

	// Test invalid migration version
	err = vm.RegisterMigration("invalid", func(cfg *Config) error {
		return nil
	})
	if err == nil {
		t.Error("Expected error for invalid migration version")
	}

	// Test newer config version
	config.Version = "2.0.0"
	err = vm.Migrate(config)
	if err == nil {
		t.Error("Expected error for newer config version")
	}
}
