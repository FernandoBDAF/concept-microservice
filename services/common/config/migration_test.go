package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMigrationBackup(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test config file
	configPath := filepath.Join(tempDir, "config.yaml")
	configContent := []byte("version: 1.0.0\nsettings:\n  key: value")
	if err := os.WriteFile(configPath, configContent, 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Create backup
	backupPath, err := MigrationBackup(configPath)
	if err != nil {
		t.Fatalf("Failed to create backup: %v", err)
	}

	// Verify backup exists
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Error("Backup file does not exist")
	}

	// Verify backup content
	backupContent, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("Failed to read backup: %v", err)
	}

	if string(backupContent) != string(configContent) {
		t.Errorf("Expected backup content %s, got %s",
			configContent, backupContent)
	}
}

func TestMigrationRollback(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test config file
	configPath := filepath.Join(tempDir, "config.yaml")
	configContent := []byte("version: 1.0.0\nsettings:\n  key: value")
	if err := os.WriteFile(configPath, configContent, 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Create backup
	backupPath, err := MigrationBackup(configPath)
	if err != nil {
		t.Fatalf("Failed to create backup: %v", err)
	}

	// Modify config file
	newContent := []byte("version: 2.0.0\nsettings:\n  key: new_value")
	if err := os.WriteFile(configPath, newContent, 0644); err != nil {
		t.Fatalf("Failed to modify config: %v", err)
	}

	// Rollback
	if err := MigrationRollback(backupPath, configPath); err != nil {
		t.Fatalf("Failed to rollback: %v", err)
	}

	// Verify rollback
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	if string(content) != string(configContent) {
		t.Errorf("Expected content %s, got %s",
			configContent, content)
	}
}

func TestMigrationCleanup(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test config file
	configPath := filepath.Join(tempDir, "config.yaml")
	configContent := []byte("version: 1.0.0\nsettings:\n  key: value")
	if err := os.WriteFile(configPath, configContent, 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Create multiple backups
	backupPaths := make([]string, 5)
	for i := 0; i < 5; i++ {
		// Sleep to ensure different timestamps
		time.Sleep(time.Millisecond * 100)
		backupPath, err := MigrationBackup(configPath)
		if err != nil {
			t.Fatalf("Failed to create backup %d: %v", i, err)
		}
		backupPaths[i] = backupPath
	}

	// Cleanup old backups
	if err := MigrationCleanup(configPath, 3); err != nil {
		t.Fatalf("Failed to cleanup: %v", err)
	}

	// Verify only 3 newest backups exist
	for i, path := range backupPaths {
		_, err := os.Stat(path)
		if i < 2 {
			// Oldest backups should be removed
			if !os.IsNotExist(err) {
				t.Errorf("Expected backup %d to be removed", i)
			}
		} else {
			// Newest backups should exist
			if os.IsNotExist(err) {
				t.Errorf("Expected backup %d to exist", i)
			}
		}
	}
}

func TestMigrationReport(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test config file
	configPath := filepath.Join(tempDir, "config.yaml")
	configContent := []byte("version: 1.0.0\nsettings:\n  key: value")
	if err := os.WriteFile(configPath, configContent, 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Create backup
	backupPath, err := MigrationBackup(configPath)
	if err != nil {
		t.Fatalf("Failed to create backup: %v", err)
	}

	// Create test config
	config := &Config{
		Version: "2.0.0",
		Settings: map[string]interface{}{
			"key": "new_value",
		},
	}

	// Generate report
	report := GenerateMigrationReport(config, "1.0.0", backupPath)
	if report.OriginalVersion != "1.0.0" {
		t.Errorf("Expected original version 1.0.0, got %s",
			report.OriginalVersion)
	}

	if report.NewVersion != "2.0.0" {
		t.Errorf("Expected new version 2.0.0, got %s",
			report.NewVersion)
	}

	if report.BackupPath != backupPath {
		t.Errorf("Expected backup path %s, got %s",
			backupPath, report.BackupPath)
	}

	// Save report
	if err := SaveMigrationReport(report, configPath); err != nil {
		t.Fatalf("Failed to save report: %v", err)
	}

	// Verify report exists
	reportDir := filepath.Join(tempDir, "migration_reports")
	files, err := os.ReadDir(reportDir)
	if err != nil {
		t.Fatalf("Failed to read report directory: %v", err)
	}

	if len(files) != 1 {
		t.Errorf("Expected 1 report file, got %d", len(files))
	}

	// Verify report content
	reportPath := filepath.Join(reportDir, files[0].Name())
	content, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("Failed to read report: %v", err)
	}

	expectedContent := "Migration Report\n" +
		"===============\n\n" +
		"Original Version: 1.0.0\n" +
		"New Version: 2.0.0\n" +
		"Timestamp: " + report.Timestamp.Format(time.RFC3339) + "\n" +
		"Backup Path: " + backupPath + "\n\n" +
		"Changes:\n"

	if string(content) != expectedContent {
		t.Errorf("Expected report content:\n%s\nGot:\n%s",
			expectedContent, content)
	}
}
