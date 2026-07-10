package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// RotationConfig defines the configuration for log rotation
type RotationConfig struct {
	// MaxSize is the maximum size in bytes of the log file before rotation
	MaxSize int64
	// MaxAge is the maximum number of days to retain old log files
	MaxAge int
	// MaxBackups is the maximum number of old log files to retain
	MaxBackups int
	// Compress determines if rotated log files should be compressed
	Compress bool
	// LocalTime determines if the time used for formatting the filename is local time
	LocalTime bool
}

// DefaultRotationConfig returns a default rotation configuration
func DefaultRotationConfig() *RotationConfig {
	return &RotationConfig{
		MaxSize:    100 * 1024 * 1024, // 100MB
		MaxAge:     30,                // 30 days
		MaxBackups: 10,                // 10 backups
		Compress:   true,
		LocalTime:  true,
	}
}

// RotatingFileWriter implements io.Writer with log rotation
type RotatingFileWriter struct {
	mu       sync.Mutex
	filename string
	file     *os.File
	size     int64
	config   *RotationConfig
}

// NewRotatingFileWriter creates a new rotating file writer
func NewRotatingFileWriter(filename string, config *RotationConfig) (*RotatingFileWriter, error) {
	if config == nil {
		config = DefaultRotationConfig()
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %v", err)
	}

	// Open the log file
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %v", err)
	}

	// Get file info for size
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to get file info: %v", err)
	}

	return &RotatingFileWriter{
		filename: filename,
		file:     file,
		size:     info.Size(),
		config:   config,
	}, nil
}

// Write implements io.Writer
func (w *RotatingFileWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Check if we need to rotate
	if w.size+int64(len(p)) > w.config.MaxSize {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}

	// Write the data
	n, err = w.file.Write(p)
	if err == nil {
		w.size += int64(n)
	}
	return n, err
}

// Close implements io.Closer
func (w *RotatingFileWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file.Close()
}

// rotate performs the log rotation
func (w *RotatingFileWriter) rotate() error {
	// Close the current file
	if err := w.file.Close(); err != nil {
		return fmt.Errorf("failed to close current log file: %v", err)
	}

	// Generate the new filename with timestamp
	timestamp := time.Now().Format("2006-01-02-15-04-05")
	ext := filepath.Ext(w.filename)
	base := strings.TrimSuffix(w.filename, ext)
	newFilename := fmt.Sprintf("%s.%s%s", base, timestamp, ext)

	// Rename the current file
	if err := os.Rename(w.filename, newFilename); err != nil {
		return fmt.Errorf("failed to rename log file: %v", err)
	}

	// Open a new file
	file, err := os.OpenFile(w.filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open new log file: %v", err)
	}

	w.file = file
	w.size = 0

	// Clean up old log files
	if err := w.cleanup(); err != nil {
		return fmt.Errorf("failed to cleanup old log files: %v", err)
	}

	return nil
}

// cleanup removes old log files based on the retention policy
func (w *RotatingFileWriter) cleanup() error {
	// Get all log files
	dir := filepath.Dir(w.filename)
	base := strings.TrimSuffix(filepath.Base(w.filename), filepath.Ext(w.filename))
	pattern := filepath.Join(dir, base+".*"+filepath.Ext(w.filename))
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("failed to find log files: %v", err)
	}

	// Sort files by modification time (newest first)
	sort.Slice(matches, func(i, j int) bool {
		infoI, _ := os.Stat(matches[i])
		infoJ, _ := os.Stat(matches[j])
		return infoI.ModTime().After(infoJ.ModTime())
	})

	// Remove files that exceed MaxBackups
	if len(matches) > w.config.MaxBackups {
		for _, file := range matches[w.config.MaxBackups:] {
			if err := os.Remove(file); err != nil {
				return fmt.Errorf("failed to remove old log file: %v", err)
			}
		}
	}

	// Remove files older than MaxAge
	if w.config.MaxAge > 0 {
		cutoff := time.Now().AddDate(0, 0, -w.config.MaxAge)
		for _, file := range matches {
			info, err := os.Stat(file)
			if err != nil {
				continue
			}
			if info.ModTime().Before(cutoff) {
				if err := os.Remove(file); err != nil {
					return fmt.Errorf("failed to remove old log file: %v", err)
				}
			}
		}
	}

	return nil
}
