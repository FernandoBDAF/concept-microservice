package config

import (
	"os"
	"testing"
	"time"
)

func TestReloader(t *testing.T) {
	// Create a temporary config file
	jsonContent := `{
		"environment": "test",
		"version": "1.0.0",
		"settings": {
			"port": 8080
		}
	}`
	tmpFile := createTempFile(t, jsonContent, ".json")
	defer os.Remove(tmpFile.Name())

	// Load initial config
	config, err := LoadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Create reloader
	reloader, err := NewReloader(config, tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create reloader: %v", err)
	}

	// Start reloader
	if err := reloader.Start(); err != nil {
		t.Fatalf("Failed to start reloader: %v", err)
	}
	defer reloader.Stop()

	// Create a channel to receive reload notifications
	reloadChan := make(chan struct{})
	reloader.OnReload(func(cfg *Config) {
		reloadChan <- struct{}{}
	})

	// Update config file
	newContent := `{
		"environment": "test",
		"version": "1.0.0",
		"settings": {
			"port": 9090
		}
	}`
	if err := os.WriteFile(tmpFile.Name(), []byte(newContent), 0644); err != nil {
		t.Fatalf("Failed to write new config: %v", err)
	}

	// Wait for reload
	select {
	case <-reloadChan:
		// Reload successful
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for config reload")
	}

	// Verify new config
	newConfig := reloader.GetConfig()
	if port := newConfig.GetInt("port"); port != 9090 {
		t.Errorf("Expected port 9090, got %d", port)
	}
}

func TestReloader_Stop(t *testing.T) {
	// Create a temporary config file
	jsonContent := `{
		"environment": "test",
		"version": "1.0.0",
		"settings": {
			"port": 8080
		}
	}`
	tmpFile := createTempFile(t, jsonContent, ".json")
	defer os.Remove(tmpFile.Name())

	// Load initial config
	config, err := LoadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Create reloader
	reloader, err := NewReloader(config, tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create reloader: %v", err)
	}

	// Start reloader
	if err := reloader.Start(); err != nil {
		t.Fatalf("Failed to start reloader: %v", err)
	}

	// Stop reloader
	reloader.Stop()

	// Try to update config file
	newContent := `{
		"environment": "test",
		"version": "1.0.0",
		"settings": {
			"port": 9090
		}
	}`
	if err := os.WriteFile(tmpFile.Name(), []byte(newContent), 0644); err != nil {
		t.Fatalf("Failed to write new config: %v", err)
	}

	// Wait a bit to ensure no reload happens
	time.Sleep(500 * time.Millisecond)

	// Verify config hasn't changed
	if port := config.GetInt("port"); port != 8080 {
		t.Errorf("Expected port 8080, got %d", port)
	}
}

func TestReloader_MultipleCallbacks(t *testing.T) {
	// Create a temporary config file
	jsonContent := `{
		"environment": "test",
		"version": "1.0.0",
		"settings": {
			"port": 8080
		}
	}`
	tmpFile := createTempFile(t, jsonContent, ".json")
	defer os.Remove(tmpFile.Name())

	// Load initial config
	config, err := LoadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Create reloader
	reloader, err := NewReloader(config, tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create reloader: %v", err)
	}

	// Start reloader
	if err := reloader.Start(); err != nil {
		t.Fatalf("Failed to start reloader: %v", err)
	}
	defer reloader.Stop()

	// Create channels to receive reload notifications
	reloadChan1 := make(chan struct{})
	reloadChan2 := make(chan struct{})

	// Register multiple callbacks
	reloader.OnReload(func(cfg *Config) {
		reloadChan1 <- struct{}{}
	})
	reloader.OnReload(func(cfg *Config) {
		reloadChan2 <- struct{}{}
	})

	// Update config file
	newContent := `{
		"environment": "test",
		"version": "1.0.0",
		"settings": {
			"port": 9090
		}
	}`
	if err := os.WriteFile(tmpFile.Name(), []byte(newContent), 0644); err != nil {
		t.Fatalf("Failed to write new config: %v", err)
	}

	// Wait for both callbacks
	select {
	case <-reloadChan1:
		// First callback received
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for first callback")
	}

	select {
	case <-reloadChan2:
		// Second callback received
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for second callback")
	}
}
