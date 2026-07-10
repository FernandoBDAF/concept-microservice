package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the service
type Config struct {
	// Server configuration
	ServerPort string
	GRPCPort   string

	// Database configuration
	DBHost     string
	DBPort     string
	DBName     string
	DBUser     string
	DBPassword string

	// Connection pool configuration
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration

	// Health check configuration
	HealthCheckInterval time.Duration
	HealthCheckTimeout  time.Duration

	// Logging configuration
	LogEnvironment string
	LogLevel       string
	ServiceName    string

	// RabbitMQ configuration
	RabbitMQURL            string
	RabbitMQExchange       string
	RabbitMQQueue          string
	RabbitMQRoutingKey     string
	RabbitMQConsumerTag    string
	RabbitMQPrefetch       int
	RabbitMQReconnectDelay time.Duration
	RabbitMQProcessTimeout time.Duration

	// Dead Letter Queue configuration
	DLQEnabled      bool
	DLQExchangeName string
	DLQQueueName    string
	DLQMaxRetries   int

	// Queue processing configuration
	QueueEnabled      bool
	QueueMaxBatchSize int
	QueueBatchTimeout time.Duration
}

// New creates a new Config with values from environment variables
func New() *Config {
	return &Config{
		// Server configuration
		ServerPort: getEnv("SERVER_PORT", "8080"),
		GRPCPort:   getEnv("GRPC_PORT", "50052"),

		// Database configuration
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBName:     getEnv("DB_NAME", "profile_storage"),
		DBUser:     getEnv("DB_USER", "profile_user"),
		DBPassword: getEnv("DB_PASSWORD", "profile_password"),

		// Connection pool configuration
		MaxOpenConns:    getIntEnv("DB_MAX_OPEN_CONNS", 25),
		MaxIdleConns:    getIntEnv("DB_MAX_IDLE_CONNS", 5),
		ConnMaxLifetime: getDurationEnv("DB_CONN_MAX_LIFETIME", 5*time.Minute),
		ConnMaxIdleTime: getDurationEnv("DB_CONN_MAX_IDLE_TIME", 1*time.Minute),

		// Health check configuration
		HealthCheckInterval: getDurationEnv("HEALTH_CHECK_INTERVAL", 30*time.Second),
		HealthCheckTimeout:  getDurationEnv("HEALTH_CHECK_TIMEOUT", 5*time.Second),

		// Logging configuration
		LogEnvironment: getEnv("LOG_ENVIRONMENT", "development"),
		LogLevel:       getEnv("LOG_LEVEL", "info"),
		ServiceName:    getEnv("SERVICE_NAME", "profile-storage"),

		// RabbitMQ configuration
		RabbitMQURL:            getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		RabbitMQExchange:       getEnv("RABBITMQ_EXCHANGE", "storage-processing"),
		RabbitMQQueue:          getEnv("RABBITMQ_QUEUE", "storage-processing"),
		RabbitMQRoutingKey:     getEnv("RABBITMQ_ROUTING_KEY", "storage.*"),
		RabbitMQConsumerTag:    getEnv("RABBITMQ_CONSUMER_TAG", "storage-service-consumer"),
		RabbitMQPrefetch:       getIntEnv("RABBITMQ_PREFETCH", 10),
		RabbitMQReconnectDelay: getDurationEnv("RABBITMQ_RECONNECT_DELAY", 5*time.Second),
		RabbitMQProcessTimeout: getDurationEnv("RABBITMQ_PROCESS_TIMEOUT", 30*time.Second),

		// Dead Letter Queue configuration
		DLQEnabled:      getBoolEnv("DLQ_ENABLED", true),
		DLQExchangeName: getEnv("DLQ_EXCHANGE_NAME", "storage-dlq"),
		DLQQueueName:    getEnv("DLQ_QUEUE_NAME", "storage-dlq"),
		DLQMaxRetries:   getIntEnv("DLQ_MAX_RETRIES", 3),

		// Queue processing configuration
		QueueEnabled:      getBoolEnv("QUEUE_ENABLED", true),
		QueueMaxBatchSize: getIntEnv("QUEUE_MAX_BATCH_SIZE", 100),
		QueueBatchTimeout: getDurationEnv("QUEUE_BATCH_TIMEOUT", 30*time.Second),
	}
}

// GetDSN returns the database connection string
func (c *Config) GetDSN() string {
	return "host=" + c.DBHost +
		" port=" + c.DBPort +
		" dbname=" + c.DBName +
		" user=" + c.DBUser +
		" password=" + c.DBPassword +
		" sslmode=disable"
}

// Helper function to get environment variable with default value
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// Helper function to get duration from environment variable
func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value, exists := os.LookupEnv(key); exists {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

// Helper function to get integer from environment variable
func getIntEnv(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// Helper function to get boolean from environment variable
func getBoolEnv(key string, defaultValue bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		ServerPort: "8080",
		GRPCPort:   "50052",
		DBHost:     "192.168.86.115",
		DBPort:     "5432",
		DBName:     "profiles",
		DBUser:     "profile_user",
		DBPassword: "profile_password",
		// Add other default configurations as needed
	}
}
