package logger

import (
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// RotationConfig holds the configuration for log rotation
type RotationConfig struct {
	// MaxSize is the maximum size in megabytes of the log file before it gets rotated
	MaxSize int
	// MaxBackups is the maximum number of old log files to retain
	MaxBackups int
	// MaxAge is the maximum number of days to retain old log files
	MaxAge int
	// Compress determines if the rotated log files should be compressed
	Compress bool
	// LogDir is the directory where log files will be stored
	LogDir string
}

// DefaultRotationConfig returns the default rotation configuration
func DefaultRotationConfig() RotationConfig {
	return RotationConfig{
		MaxSize:    100,    // 100MB
		MaxBackups: 5,      // Keep 5 backup files
		MaxAge:     30,     // Keep logs for 30 days
		Compress:   true,   // Compress rotated files
		LogDir:     "logs", // Store logs in 'logs' directory
	}
}

// setupLogRotation configures log rotation for the given logger
func setupLogRotation(config RotationConfig) (zapcore.Core, error) {
	// Create log directory if it doesn't exist
	if err := os.MkdirAll(config.LogDir, 0755); err != nil {
		return nil, err
	}

	// Create a lumberjack logger for log rotation
	rotator := &lumberjack.Logger{
		Filename:   filepath.Join(config.LogDir, "storage-service.log"),
		MaxSize:    config.MaxSize, // megabytes
		MaxBackups: config.MaxBackups,
		MaxAge:     config.MaxAge, // days
		Compress:   config.Compress,
	}

	// Create a zapcore.WriteSyncer for the rotator
	writeSyncer := zapcore.AddSync(rotator)

	// Create encoder config
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// Create the core
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		writeSyncer,
		zapcore.InfoLevel,
	)

	return core, nil
}

// setupConsoleOutput configures console output for development
func setupConsoleOutput() zapcore.Core {
	// Create encoder config for console output
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalColorLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// Create the core for console output
	return zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderConfig),
		zapcore.AddSync(os.Stdout),
		zapcore.DebugLevel,
	)
}

// NewRotatedLogger creates a new logger with rotation support
func NewRotatedLogger(config RotationConfig) (*zap.Logger, error) {
	// Setup log rotation
	fileCore, err := setupLogRotation(config)
	if err != nil {
		return nil, err
	}

	// Setup console output for development
	consoleCore := setupConsoleOutput()

	// Create a multi-core logger that writes to both file and console
	core := zapcore.NewTee(fileCore, consoleCore)

	// Create the logger
	logger := zap.New(core,
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)

	return logger, nil
}

// CleanupOldLogs removes log files that are older than the specified age
func CleanupOldLogs(config RotationConfig) error {
	// Get the current time
	now := time.Now()

	// Walk through the log directory
	return filepath.Walk(config.LogDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check if the file is a log file
		if filepath.Ext(path) != ".log" && filepath.Ext(path) != ".gz" {
			return nil
		}

		// Check if the file is older than MaxAge
		if now.Sub(info.ModTime()) > time.Duration(config.MaxAge)*24*time.Hour {
			return os.Remove(path)
		}

		return nil
	})
}
