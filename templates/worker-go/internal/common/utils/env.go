package utils

import (
	"os"
	"strconv"
)

// GetEnvOrDefault returns the environment variable value or a default if not set
func GetEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// GetEnvIntOrDefault returns the environment variable as int or a default if not set
func GetEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// GetEnvBoolOrDefault returns the environment variable as bool or a default if not set
func GetEnvBoolOrDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

// RequiredEnv returns the environment variable value or panics if not set
func RequiredEnv(key string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	panic("Required environment variable " + key + " is not set")
}

// RabbitMQConfig contains common RabbitMQ configuration
type RabbitMQConfig struct {
	Host     string
	Port     string
	User     string
	Password string
}

// GetRabbitMQConfig returns RabbitMQ configuration from environment variables
func GetRabbitMQConfig() RabbitMQConfig {
	return RabbitMQConfig{
		Host:     GetEnvOrDefault("RABBITMQ_HOST", "localhost"),
		Port:     GetEnvOrDefault("RABBITMQ_PORT", "5672"),
		User:     GetEnvOrDefault("RABBITMQ_USER", "guest"),
		Password: GetEnvOrDefault("RABBITMQ_PASSWORD", "guest"),
	}
}
