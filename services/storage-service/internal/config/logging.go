package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"microservices/services/profile-storage/internal/logger"
)

// LoggingConfig holds the logging configuration
type LoggingConfig struct {
	// Environment is the current environment (development/production)
	Environment string
	// Level is the minimum log level to output
	Level string
	// ServiceName is the name of the service for log identification
	ServiceName string
	// Rotation is the log rotation configuration
	Rotation logger.RotationConfig
	// Aggregation is the log aggregation configuration
	Aggregation logger.AggregationConfig
}

// LoadLoggingConfig loads the logging configuration from environment variables
func LoadLoggingConfig() LoggingConfig {
	config := LoggingConfig{
		Environment: getEnvOrDefault("LOG_ENVIRONMENT", "development"),
		Level:       getEnvOrDefault("LOG_LEVEL", "info"),
		ServiceName: getEnvOrDefault("SERVICE_NAME", "profile-storage"),
		Rotation: logger.RotationConfig{
			MaxSize:    getEnvIntOrDefault("LOG_MAX_SIZE", 100),
			MaxBackups: getEnvIntOrDefault("LOG_MAX_BACKUPS", 5),
			MaxAge:     getEnvIntOrDefault("LOG_MAX_AGE", 30),
			Compress:   getEnvBoolOrDefault("LOG_COMPRESS", true),
			LogDir:     getEnvOrDefault("LOG_DIR", "logs"),
		},
		Aggregation: logger.AggregationConfig{
			Enabled:       getEnvBoolOrDefault("LOG_AGGREGATION_ENABLED", false),
			Endpoint:      getEnvOrDefault("LOG_AGGREGATION_ENDPOINT", "http://localhost:9200"),
			BatchSize:     getEnvIntOrDefault("LOG_AGGREGATION_BATCH_SIZE", 100),
			FlushInterval: getEnvDurationOrDefault("LOG_AGGREGATION_FLUSH_INTERVAL", 5*time.Second),
			RetryCount:    getEnvIntOrDefault("LOG_AGGREGATION_RETRY_COUNT", 3),
			RetryDelay:    getEnvDurationOrDefault("LOG_AGGREGATION_RETRY_DELAY", time.Second),
			Timeout:       getEnvDurationOrDefault("LOG_AGGREGATION_TIMEOUT", 5*time.Second),
		},
	}

	// Validate configuration
	validateLoggingConfig(&config)

	return config
}

// validateLoggingConfig validates the logging configuration
func validateLoggingConfig(config *LoggingConfig) {
	// Validate environment
	if config.Environment != "development" && config.Environment != "production" {
		config.Environment = "development"
	}

	// Validate log level
	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLevels[strings.ToLower(config.Level)] {
		config.Level = "info"
	}

	// Validate rotation settings
	if config.Rotation.MaxSize < 1 {
		config.Rotation.MaxSize = 100
	}
	if config.Rotation.MaxBackups < 1 {
		config.Rotation.MaxBackups = 5
	}
	if config.Rotation.MaxAge < 1 {
		config.Rotation.MaxAge = 30
	}

	// Validate aggregation settings
	if config.Aggregation.Enabled {
		if config.Aggregation.BatchSize < 1 {
			config.Aggregation.BatchSize = 100
		}
		if config.Aggregation.FlushInterval < time.Second {
			config.Aggregation.FlushInterval = 5 * time.Second
		}
		if config.Aggregation.RetryCount < 0 {
			config.Aggregation.RetryCount = 3
		}
		if config.Aggregation.RetryDelay < time.Millisecond {
			config.Aggregation.RetryDelay = time.Second
		}
		if config.Aggregation.Timeout < time.Second {
			config.Aggregation.Timeout = 5 * time.Second
		}
	}
}

// getEnvOrDefault gets an environment variable or returns a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvIntOrDefault gets an integer environment variable or returns a default value
func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvBoolOrDefault gets a boolean environment variable or returns a default value
func getEnvBoolOrDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

// getEnvDurationOrDefault gets a duration environment variable or returns a default value
func getEnvDurationOrDefault(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
