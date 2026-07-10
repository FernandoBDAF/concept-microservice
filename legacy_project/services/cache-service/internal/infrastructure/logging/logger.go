package logging

import (
	"os"

	"cache-service/internal/config"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewLogger creates a new structured logger based on configuration
func NewLogger(cfg *config.LoggingConfig) (*zap.Logger, error) {
	var logger *zap.Logger
	var err error

	if cfg.Development {
		// Development configuration with console output
		config := zap.NewDevelopmentConfig()
		config.Level = zap.NewAtomicLevelAt(parseLogLevel(cfg.Level))

		if cfg.Format == "json" {
			config.Encoding = "json"
		} else {
			config.Encoding = "console"
		}

		logger, err = config.Build()
	} else {
		// Production configuration
		config := zap.NewProductionConfig()
		config.Level = zap.NewAtomicLevelAt(parseLogLevel(cfg.Level))

		if cfg.Format == "console" {
			config.Encoding = "console"
		} else {
			config.Encoding = "json"
		}

		logger, err = config.Build()
	}

	if err != nil {
		return nil, err
	}

	return logger, nil
}

// parseLogLevel converts string log level to zapcore.Level
func parseLogLevel(level string) zapcore.Level {
	switch level {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn", "warning":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	case "fatal":
		return zapcore.FatalLevel
	default:
		return zapcore.InfoLevel
	}
}

// NewDefaultLogger creates a default logger for cases where config is not available
func NewDefaultLogger() *zap.Logger {
	config := zap.NewProductionConfig()
	config.OutputPaths = []string{"stdout"}
	config.ErrorOutputPaths = []string{"stderr"}

	logger, err := config.Build()
	if err != nil {
		// Fallback to basic logger
		logger = zap.NewNop()
	}

	return logger
}

// LoggerMiddleware creates fields for consistent logging across the application
func LoggerMiddleware(logger *zap.Logger) *zap.Logger {
	hostname, _ := os.Hostname()
	return logger.With(
		zap.String("service", "cache-service"),
		zap.String("hostname", hostname),
		zap.String("version", "1.0.0"),
	)
}
