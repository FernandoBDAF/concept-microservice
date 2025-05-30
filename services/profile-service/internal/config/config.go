package config

import (
	"os"
	"strconv"
	"time"
)

// Config represents the application configuration
type Config struct {
	Server      ServerConfig
	Auth        AuthConfig
	Redis       RedisConfig
	Environment string
	Storage     StorageConfig
	Cache       CacheConfig
	Queue       QueueConfig
	Security    SecurityConfig
	Logging     LoggingConfig
}

// ServerConfig represents the server configuration
type ServerConfig struct {
	Host string
	Port int
}

// AuthConfig represents the auth service configuration
type AuthConfig struct {
	Host string
	Port int
	URL  string
}

// RedisConfig represents the Redis configuration
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

// StorageConfig holds the storage service configuration
type StorageConfig struct {
	Host       string
	Port       int
	Database   string
	Type       string
	MaxRetries int
	RetryDelay time.Duration
}

// CacheConfig holds the cache service configuration
type CacheConfig struct {
	Host    string
	Port    int
	Enabled bool
}

// QueueConfig holds the configuration for the queue service
type QueueConfig struct {
	URL       string        `mapstructure:"url"`
	Timeout   time.Duration `mapstructure:"timeout"`
	Retries   int           `mapstructure:"retries"`
	QueueName string        `mapstructure:"queue_name"`
}

// SecurityConfig holds the security configuration
type SecurityConfig struct {
	Enabled bool
}

// LoggingConfig holds the logging configuration
type LoggingConfig struct {
	Level    string
	Format   string
	LogFile  string
	Shipping LogShippingConfig
}

// LogShippingConfig holds the log shipping configuration
type LogShippingConfig struct {
	Enabled    bool
	Endpoint   string
	BufferSize int
	MaxRetries int
	RetryDelay time.Duration
}

// LoadConfig loads the configuration from environment variables
func LoadConfig() *Config {
	env := getEnv("ENV", "development")

	return &Config{
		Server: ServerConfig{
			Host: getEnv("SERVER_HOST", "0.0.0.0"),
			Port: getEnvAsInt("SERVER_PORT", 8080),
		},
		Auth: AuthConfig{
			Host: getEnv("AUTH_SERVICE_HOST", "localhost"),
			Port: getEnvAsInt("AUTH_SERVICE_PORT", 80),
			URL:  getEnv("AUTH_SERVICE_URL", "http://auth-service"),
		},
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvAsInt("REDIS_DB", 0),
		},
		Environment: env,
		Storage: StorageConfig{
			Host:       getEnv("STORAGE_HOST", "profile-storage"),
			Port:       getEnvAsInt("STORAGE_PORT", 8080),
			Database:   getEnv("STORAGE_DATABASE", "profile_service"),
			Type:       getEnv("STORAGE_TYPE", "memory"),
			MaxRetries: getEnvAsInt("STORAGE_MAX_RETRIES", 3),
			RetryDelay: time.Duration(getEnvAsInt("STORAGE_RETRY_DELAY_MS", 100)) * time.Millisecond,
		},
		Cache: CacheConfig{
			Host:    getEnv("CACHE_HOST", "localhost"),
			Port:    getEnvAsInt("CACHE_PORT", 6379),
			Enabled: getEnvBool("CACHE_ENABLED", false),
		},
		Queue: QueueConfig{
			URL:       getEnv("QUEUE_SERVICE_URL", "http://queue-service:80"),
			Timeout:   getDurationEnv("QUEUE_TIMEOUT_MS", 5*time.Second),
			Retries:   getEnvAsInt("QUEUE_RETRIES", 3),
			QueueName: getEnv("QUEUE_NAME", "profile_queue"),
		},
		Security: SecurityConfig{
			Enabled: getEnvBool("SECURITY_ENABLED", true),
		},
		Logging: LoggingConfig{
			Level:   getEnv("LOG_LEVEL", "info"),
			Format:  getEnv("LOG_FORMAT", "text"),
			LogFile: getEnv("LOG_FILE", "app.log"),
			Shipping: LogShippingConfig{
				Enabled:    getEnvBool("LOG_SHIPPING_ENABLED", false),
				Endpoint:   getEnv("LOG_SHIPPING_ENDPOINT", "http://log-shipping-service"),
				BufferSize: getEnvAsInt("LOG_SHIPPING_BUFFER_SIZE", 100),
				MaxRetries: getEnvAsInt("LOG_SHIPPING_MAX_RETRIES", 3),
				RetryDelay: getDurationEnv("LOG_SHIPPING_RETRY_DELAY_MS", 100*time.Millisecond),
			},
		},
	}
}

// Helper functions for environment variables
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		boolValue, err := strconv.ParseBool(value)
		if err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value, exists := os.LookupEnv(key); exists {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
