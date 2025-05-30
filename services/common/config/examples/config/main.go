package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/FBDAF/microservices/services/common/config"
)

func main() {
	// Create a temporary config file
	configContent := `{
		"environment": "development",
		"version": "1.0.0",
		"settings": {
			"database": {
				"host": "localhost",
				"port": 5432,
				"password": "secret-password"
			},
			"api": {
				"port": 8080,
				"timeout": 30
			}
		}
	}`
	tmpFile := createTempFile(configContent, ".json")
	defer os.Remove(tmpFile.Name())

	// Load configuration
	cfg, err := config.LoadConfig(tmpFile.Name())
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create encryptor
	key := "12345678901234567890123456789012" // 32-byte key
	encryptor, err := config.NewEncryptor(key)
	if err != nil {
		log.Fatalf("Failed to create encryptor: %v", err)
	}

	// Encrypt sensitive values
	sensitiveKeys := []string{"database.password"}
	if err := encryptor.EncryptConfig(cfg, sensitiveKeys); err != nil {
		log.Fatalf("Failed to encrypt config: %v", err)
	}

	// Create version manager
	vm, err := config.NewVersionManager("2.0.0")
	if err != nil {
		log.Fatalf("Failed to create version manager: %v", err)
	}

	// Register migration
	err = vm.RegisterMigration("2.0.0", func(cfg *config.Config) error {
		// Rename database.host to database.address
		if db, ok := cfg.Settings["database"].(map[string]interface{}); ok {
			if host, exists := db["host"]; exists {
				db["address"] = host
				delete(db, "host")
			}
		}
		return nil
	})
	if err != nil {
		log.Fatalf("Failed to register migration: %v", err)
	}

	// Create backup before migration
	backupPath, err := config.MigrationBackup(tmpFile.Name())
	if err != nil {
		log.Fatalf("Failed to create backup: %v", err)
	}

	// Migrate configuration
	if err := vm.Migrate(cfg); err != nil {
		log.Fatalf("Failed to migrate config: %v", err)
	}

	// Generate migration report
	report := config.GenerateMigrationReport(cfg, "1.0.0", backupPath)
	if err := config.SaveMigrationReport(report, tmpFile.Name()); err != nil {
		log.Fatalf("Failed to save migration report: %v", err)
	}

	// Create reloader
	reloader, err := config.NewReloader(cfg, tmpFile.Name())
	if err != nil {
		log.Fatalf("Failed to create reloader: %v", err)
	}

	// Register reload callback
	reloader.OnReload(func(newCfg *config.Config) {
		fmt.Printf("Configuration reloaded: port=%d\n", newCfg.GetInt("api.port"))
	})

	// Start watching for changes
	if err := reloader.Start(); err != nil {
		log.Fatalf("Failed to start reloader: %v", err)
	}
	defer reloader.Stop()

	// Define validation rules
	rules := config.ValidationRules{
		"environment": []config.ValidationRule{config.Required()},
		"version":     []config.ValidationRule{config.Required()},
		"database": []config.ValidationRule{
			config.Required(),
			func(value interface{}) error {
				if db, ok := value.(map[string]interface{}); ok {
					if _, exists := db["address"]; !exists {
						return fmt.Errorf("database.address is required")
					}
					if port, exists := db["port"]; exists {
						if p, ok := port.(float64); !ok || p < 1 || p > 65535 {
							return fmt.Errorf("database.port must be between 1 and 65535")
						}
					}
				}
				return nil
			},
		},
		"api": []config.ValidationRule{
			config.Required(),
			func(value interface{}) error {
				if api, ok := value.(map[string]interface{}); ok {
					if port, exists := api["port"]; exists {
						if p, ok := port.(float64); !ok || p < 1 || p > 65535 {
							return fmt.Errorf("api.port must be between 1 and 65535")
						}
					}
				}
				return nil
			},
		},
	}

	// Validate configuration
	if err := cfg.Validate(rules); err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}

	// Print configuration
	fmt.Printf("Environment: %s\n", cfg.Environment)
	fmt.Printf("Version: %s\n", cfg.Version)
	fmt.Printf("Database Address: %s\n", cfg.GetStringMap("database")["address"])
	fmt.Printf("Database Port: %d\n", cfg.GetInt("database.port"))
	fmt.Printf("API Port: %d\n", cfg.GetInt("api.port"))
	fmt.Printf("API Timeout: %d\n", cfg.GetInt("api.timeout"))

	// Simulate configuration changes
	go func() {
		time.Sleep(2 * time.Second)
		newContent := `{
			"environment": "development",
			"version": "2.0.0",
			"settings": {
				"database": {
					"address": "localhost",
					"port": 5432,
					"password": "secret-password"
				},
				"api": {
					"port": 9090,
					"timeout": 30
				}
			}
		}`
		if err := os.WriteFile(tmpFile.Name(), []byte(newContent), 0644); err != nil {
			log.Printf("Failed to write new config: %v", err)
		}
	}()

	// Wait for changes
	time.Sleep(3 * time.Second)
}

func createTempFile(content string, ext string) *os.File {
	tmpFile, err := os.CreateTemp("", "config-*"+ext)
	if err != nil {
		log.Fatalf("Failed to create temp file: %v", err)
	}

	if _, err := tmpFile.WriteString(content); err != nil {
		log.Fatalf("Failed to write to temp file: %v", err)
	}

	if err := tmpFile.Close(); err != nil {
		log.Fatalf("Failed to close temp file: %v", err)
	}

	return tmpFile
}
