package config

import (
	"os"
	"testing"
	"time"
)

func BenchmarkLoadConfig(b *testing.B) {
	// Create a temporary config file
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
	tmpFile := createBenchmarkTempFile(b, jsonContent, ".json")
	defer os.Remove(tmpFile.Name())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := LoadConfig(tmpFile.Name())
		if err != nil {
			b.Fatalf("Failed to load config: %v", err)
		}
	}
}

func BenchmarkConfigValidation(b *testing.B) {
	// Create a temporary config file
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
	tmpFile := createBenchmarkTempFile(b, jsonContent, ".json")
	defer os.Remove(tmpFile.Name())

	// Load config
	cfg, err := LoadConfig(tmpFile.Name())
	if err != nil {
		b.Fatalf("Failed to load config: %v", err)
	}

	// Define validation rules
	rules := ValidationRules{
		"environment": []ValidationRule{Required()},
		"version":     []ValidationRule{Required()},
		"database": []ValidationRule{
			Required(),
			func(value interface{}) error {
				if db, ok := value.(map[string]interface{}); ok {
					if _, exists := db["host"]; !exists {
						return nil
					}
					if port, exists := db["port"]; exists {
						if p, ok := port.(float64); !ok || p < 1 || p > 65535 {
							return nil
						}
					}
				}
				return nil
			},
		},
		"api": []ValidationRule{
			Required(),
			func(value interface{}) error {
				if api, ok := value.(map[string]interface{}); ok {
					if port, exists := api["port"]; exists {
						if p, ok := port.(float64); !ok || p < 1 || p > 65535 {
							return nil
						}
					}
				}
				return nil
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := cfg.Validate(rules); err != nil {
			b.Fatalf("Validation failed: %v", err)
		}
	}
}

func BenchmarkEncryption(b *testing.B) {
	// Create encryptor
	key := "12345678901234567890123456789012"
	encryptor, err := NewEncryptor(key)
	if err != nil {
		b.Fatalf("Failed to create encryptor: %v", err)
	}

	// Create test config
	config := &Config{
		Environment: "test",
		Version:     "1.0.0",
		Settings: map[string]interface{}{
			"password": "secret-password",
			"api_key":  "secret-api-key",
			"port":     8080,
		},
	}

	// Define sensitive keys
	sensitiveKeys := []string{"password", "api_key"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := encryptor.EncryptConfig(config, sensitiveKeys); err != nil {
			b.Fatalf("Failed to encrypt config: %v", err)
		}
		if err := encryptor.DecryptConfig(config, sensitiveKeys); err != nil {
			b.Fatalf("Failed to decrypt config: %v", err)
		}
	}
}

func BenchmarkVersioning(b *testing.B) {
	// Create version manager
	vm, err := NewVersionManager("2.0.0")
	if err != nil {
		b.Fatalf("Failed to create version manager: %v", err)
	}

	// Register migration
	err = vm.RegisterMigration("2.0.0", func(cfg *Config) error {
		if db, ok := cfg.Settings["database"].(map[string]interface{}); ok {
			if host, exists := db["host"]; exists {
				db["address"] = host
				delete(db, "host")
			}
		}
		return nil
	})
	if err != nil {
		b.Fatalf("Failed to register migration: %v", err)
	}

	// Create test config
	config := &Config{
		Version: "1.0.0",
		Settings: map[string]interface{}{
			"database": map[string]interface{}{
				"host": "localhost",
				"port": 5432,
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := vm.Migrate(config); err != nil {
			b.Fatalf("Failed to migrate config: %v", err)
		}
		// Reset config for next iteration
		config.Version = "1.0.0"
		config.Settings["database"] = map[string]interface{}{
			"host": "localhost",
			"port": 5432,
		}
	}
}

func BenchmarkHotReloading(b *testing.B) {
	// Create a temporary config file
	jsonContent := `{
		"environment": "test",
		"version": "1.0.0",
		"settings": {
			"port": 8080
		}
	}`
	tmpFile := createBenchmarkTempFile(b, jsonContent, ".json")
	defer os.Remove(tmpFile.Name())

	// Load initial config
	config, err := LoadConfig(tmpFile.Name())
	if err != nil {
		b.Fatalf("Failed to load config: %v", err)
	}

	// Create reloader
	reloader, err := NewReloader(config, tmpFile.Name())
	if err != nil {
		b.Fatalf("Failed to create reloader: %v", err)
	}

	// Start reloader
	if err := reloader.Start(); err != nil {
		b.Fatalf("Failed to start reloader: %v", err)
	}
	defer reloader.Stop()

	// Update config file
	newContent := `{
		"environment": "test",
		"version": "1.0.0",
		"settings": {
			"port": 9090
		}
	}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := os.WriteFile(tmpFile.Name(), []byte(newContent), 0644); err != nil {
			b.Fatalf("Failed to write new config: %v", err)
		}
		// Wait for reload
		time.Sleep(100 * time.Millisecond)
	}
}

func createBenchmarkTempFile(t testing.TB, content string, ext string) *os.File {
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
