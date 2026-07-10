package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// MigrationBackup creates a backup of the configuration file before migration
func MigrationBackup(configPath string) (string, error) {
	// Create backup directory if it doesn't exist
	backupDir := filepath.Join(filepath.Dir(configPath), "backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create backup directory: %v", err)
	}

	// Generate backup filename with timestamp
	timestamp := time.Now().Format("20060102_150405")
	backupName := fmt.Sprintf("%s_%s%s",
		filepath.Base(configPath),
		timestamp,
		filepath.Ext(configPath))
	backupPath := filepath.Join(backupDir, backupName)

	// Read original file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", fmt.Errorf("failed to read config file: %v", err)
	}

	// Write backup file
	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write backup file: %v", err)
	}

	return backupPath, nil
}

// MigrationRollback restores a configuration from a backup file
func MigrationRollback(backupPath string, configPath string) error {
	// Read backup file
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("failed to read backup file: %v", err)
	}

	// Write to config file
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}

	return nil
}

// MigrationCleanup removes old backup files
func MigrationCleanup(configPath string, maxBackups int) error {
	backupDir := filepath.Join(filepath.Dir(configPath), "backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %v", err)
	}

	// List backup files
	files, err := os.ReadDir(backupDir)
	if err != nil {
		return fmt.Errorf("failed to read backup directory: %v", err)
	}

	// Sort files by modification time (newest first)
	type fileInfo struct {
		path    string
		modTime time.Time
	}
	var fileInfos []fileInfo

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		path := filepath.Join(backupDir, file.Name())
		info, err := file.Info()
		if err != nil {
			continue
		}

		fileInfos = append(fileInfos, fileInfo{
			path:    path,
			modTime: info.ModTime(),
		})
	}

	// Sort by modification time (newest first)
	for i := 0; i < len(fileInfos)-1; i++ {
		for j := i + 1; j < len(fileInfos); j++ {
			if fileInfos[i].modTime.Before(fileInfos[j].modTime) {
				fileInfos[i], fileInfos[j] = fileInfos[j], fileInfos[i]
			}
		}
	}

	// Remove old backups
	for i := maxBackups; i < len(fileInfos); i++ {
		if err := os.Remove(fileInfos[i].path); err != nil {
			return fmt.Errorf("failed to remove old backup: %v", err)
		}
	}

	return nil
}

// MigrationReport generates a report of configuration changes
type MigrationReport struct {
	OriginalVersion string
	NewVersion      string
	Changes         []string
	BackupPath      string
	Timestamp       time.Time
}

// GenerateMigrationReport creates a report of configuration changes
func GenerateMigrationReport(config *Config, originalVersion string, backupPath string) *MigrationReport {
	return &MigrationReport{
		OriginalVersion: originalVersion,
		NewVersion:      config.Version,
		Changes:         []string{}, // TODO: Implement change tracking
		BackupPath:      backupPath,
		Timestamp:       time.Now(),
	}
}

// SaveMigrationReport saves a migration report to a file
func SaveMigrationReport(report *MigrationReport, configPath string) error {
	reportDir := filepath.Join(filepath.Dir(configPath), "migration_reports")
	if err := os.MkdirAll(reportDir, 0755); err != nil {
		return fmt.Errorf("failed to create report directory: %v", err)
	}

	reportName := fmt.Sprintf("migration_%s_to_%s_%s.txt",
		report.OriginalVersion,
		report.NewVersion,
		report.Timestamp.Format("20060102_150405"))
	reportPath := filepath.Join(reportDir, reportName)

	// Format report
	content := fmt.Sprintf("Migration Report\n"+
		"===============\n\n"+
		"Original Version: %s\n"+
		"New Version: %s\n"+
		"Timestamp: %s\n"+
		"Backup Path: %s\n\n"+
		"Changes:\n",
		report.OriginalVersion,
		report.NewVersion,
		report.Timestamp.Format(time.RFC3339),
		report.BackupPath)

	for _, change := range report.Changes {
		content += fmt.Sprintf("- %s\n", change)
	}

	// Write report file
	if err := os.WriteFile(reportPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write report file: %v", err)
	}

	return nil
}
