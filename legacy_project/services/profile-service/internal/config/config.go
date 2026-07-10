package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
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
	// ✅ NEW: Multi-worker task configuration for Phase 3
	Tasks TaskConfig
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
	Host    string        `env:"CACHE_SERVICE_HOST" default:"cache-service"`
	Port    int           `env:"CACHE_SERVICE_PORT" default:"8080"`
	Enabled bool          `env:"CACHE_ENABLED" default:"true"`
	Timeout time.Duration `env:"CACHE_SERVICE_TIMEOUT" default:"5s"`
	Retries int           `env:"CACHE_SERVICE_RETRIES" default:"3"`
	TTL     struct {
		Profile time.Duration `env:"CACHE_PROFILE_TTL" default:"1h"`
		Session time.Duration `env:"CACHE_SESSION_TTL" default:"24h"`
		Task    time.Duration `env:"CACHE_TASK_TTL" default:"30m"`
	}
}

// ✅ ENHANCED: QueueConfig with routing key support (removed QueueName - unused)
type QueueConfig struct {
	URL            string                   `mapstructure:"url"`
	Timeout        time.Duration            `mapstructure:"timeout"`
	Retries        int                      `mapstructure:"retries"`
	MaxRequestSize int64                    `mapstructure:"max_request_size"` // ✅ NEW
	CircuitBreaker CircuitBreakerConfig     `mapstructure:"circuit_breaker"`  // ✅ NEW
	RoutingKeys    map[string]string        `mapstructure:"routing_keys"`     // ✅ NEW
	TaskTimeouts   map[string]time.Duration `mapstructure:"task_timeouts"`    // ✅ NEW
}

// ✅ NEW: Circuit breaker configuration for enhanced error handling
type CircuitBreakerConfig struct {
	Enabled               bool          `mapstructure:"enabled"`
	FailureThreshold      int           `mapstructure:"failure_threshold"`
	RecoveryTimeout       time.Duration `mapstructure:"recovery_timeout"`
	MaxConcurrentRequests int           `mapstructure:"max_concurrent_requests"`
}

// ✅ NEW: Task configuration for multi-worker architecture
type TaskConfig struct {
	SupportedTypes  []string                `mapstructure:"supported_types"`
	TypeMappings    map[string]TaskTypeInfo `mapstructure:"type_mappings"`
	DefaultTimeout  time.Duration           `mapstructure:"default_timeout"`
	MaxPayloadSize  int64                   `mapstructure:"max_payload_size"`
	ValidationRules TaskValidationConfig    `mapstructure:"validation"`
	RetryPolicy     TaskRetryConfig         `mapstructure:"retry_policy"`
}

// ✅ NEW: Task type information with worker mapping
type TaskTypeInfo struct {
	RoutingKey   string        `mapstructure:"routing_key"`
	WorkerTarget string        `mapstructure:"worker_target"`
	Timeout      time.Duration `mapstructure:"timeout"`
	MaxRetries   int           `mapstructure:"max_retries"`
	Priority     int           `mapstructure:"priority"`
}

// ✅ NEW: Task validation configuration
type TaskValidationConfig struct {
	EnableStrictValidation bool          `mapstructure:"enable_strict"`
	MaxEmailsPerBurst      int           `mapstructure:"max_emails_per_burst"`
	MaxImageSizeMB         int           `mapstructure:"max_image_size_mb"`
	AllowedDomains         []string      `mapstructure:"allowed_domains"`
	ValidationTimeout      time.Duration `mapstructure:"validation_timeout"`
}

// ✅ NEW: Task retry policy configuration
type TaskRetryConfig struct {
	MaxRetries    int           `mapstructure:"max_retries"`
	InitialDelay  time.Duration `mapstructure:"initial_delay"`
	MaxDelay      time.Duration `mapstructure:"max_delay"`
	BackoffFactor float64       `mapstructure:"backoff_factor"`
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

// ✅ NEW: Configuration validation method
func (c *Config) Validate() error {
	if err := c.validateServer(); err != nil {
		return fmt.Errorf("server config validation failed: %w", err)
	}

	if err := c.validateQueue(); err != nil {
		return fmt.Errorf("queue config validation failed: %w", err)
	}

	if err := c.validateTasks(); err != nil {
		return fmt.Errorf("task config validation failed: %w", err)
	}

	return nil
}

// validateServer validates server configuration
func (c *Config) validateServer() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}
	return nil
}

// validateQueue validates queue configuration
func (c *Config) validateQueue() error {
	if c.Queue.URL == "" {
		return fmt.Errorf("queue service URL is required")
	}

	if c.Queue.Timeout <= 0 {
		return fmt.Errorf("queue timeout must be positive")
	}

	if c.Queue.Retries < 0 {
		return fmt.Errorf("queue retries cannot be negative")
	}

	// Validate routing keys
	requiredRoutingKeys := []string{"profile.task", "email.send", "image.process"}
	for _, key := range requiredRoutingKeys {
		if routingKey, exists := c.Queue.RoutingKeys[key]; !exists || routingKey == "" {
			return fmt.Errorf("missing routing key configuration for: %s", key)
		}
	}

	return nil
}

// validateTasks validates task configuration
func (c *Config) validateTasks() error {
	if len(c.Tasks.SupportedTypes) == 0 {
		return fmt.Errorf("no supported task types configured")
	}

	// Validate each supported type has mapping
	for _, taskType := range c.Tasks.SupportedTypes {
		if _, exists := c.Tasks.TypeMappings[taskType]; !exists {
			return fmt.Errorf("missing type mapping for supported task type: %s", taskType)
		}
	}

	// Validate task timeouts are positive
	for taskType, timeout := range c.Queue.TaskTimeouts {
		if timeout <= 0 {
			return fmt.Errorf("task timeout for %s must be positive", taskType)
		}
	}

	return nil
}

// LoadConfig loads the configuration from environment variables
func LoadConfig() *Config {
	env := getEnv("ENV", "development")

	config := &Config{
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
			Host:       getEnv("STORAGE_SERVICE_HOST", "storage-service"),
			Port:       getEnvAsInt("STORAGE_SERVICE_PORT", 8080),
			Database:   getEnv("STORAGE_DATABASE", "profile_storage"),
			Type:       getEnv("STORAGE_TYPE", "memory"),
			MaxRetries: getEnvAsInt("STORAGE_MAX_RETRIES", 3),
			RetryDelay: time.Duration(getEnvAsInt("STORAGE_RETRY_DELAY_MS", 100)) * time.Millisecond,
		},
		Cache: CacheConfig{
			Host:    getEnv("CACHE_SERVICE_HOST", "cache-service"),
			Port:    getEnvAsInt("CACHE_SERVICE_PORT", 8080),
			Enabled: getEnvBool("CACHE_ENABLED", true),
			Timeout: getDurationEnv("CACHE_SERVICE_TIMEOUT", "5s"),
			Retries: getEnvAsInt("CACHE_SERVICE_RETRIES", 3),
			TTL: struct {
				Profile time.Duration `env:"CACHE_PROFILE_TTL" default:"1h"`
				Session time.Duration `env:"CACHE_SESSION_TTL" default:"24h"`
				Task    time.Duration `env:"CACHE_TASK_TTL" default:"30m"`
			}{
				Profile: getDurationEnv("CACHE_PROFILE_TTL", "1h"),
				Session: getDurationEnv("CACHE_SESSION_TTL", "24h"),
				Task:    getDurationEnv("CACHE_TASK_TTL", "30m"),
			},
		},
		Queue: QueueConfig{
			URL:            getEnv("QUEUE_SERVICE_URL", "http://queue-service:80"),
			Timeout:        getDurationEnv("QUEUE_TIMEOUT", "5s"),
			Retries:        getEnvAsInt("QUEUE_RETRIES", 3),
			MaxRequestSize: getEnvAsInt64("QUEUE_MAX_REQUEST_SIZE", 1024*1024), // 1MB
			CircuitBreaker: CircuitBreakerConfig{
				Enabled:               getEnvBool("QUEUE_CIRCUIT_BREAKER_ENABLED", true),
				FailureThreshold:      getEnvAsInt("QUEUE_CIRCUIT_BREAKER_FAILURE_THRESHOLD", 5),
				RecoveryTimeout:       getDurationEnv("QUEUE_CIRCUIT_BREAKER_RECOVERY_TIMEOUT", "30s"),
				MaxConcurrentRequests: getEnvAsInt("QUEUE_CIRCUIT_BREAKER_MAX_CONCURRENT", 100),
			},
			// ✅ Default routing key mappings
			RoutingKeys: map[string]string{
				"profile_update":     getEnv("ROUTING_KEY_PROFILE", "profile.task"),
				"email_notification": getEnv("ROUTING_KEY_EMAIL", "email.send"),
				"image_processing":   getEnv("ROUTING_KEY_IMAGE", "image.process"),
			},
			// ✅ Task-specific timeouts
			TaskTimeouts: map[string]time.Duration{
				"profile_update":     getDurationEnv("TASK_TIMEOUT_PROFILE", "30s"),
				"email_notification": getDurationEnv("TASK_TIMEOUT_EMAIL", "60s"),
				"image_processing":   getDurationEnv("TASK_TIMEOUT_IMAGE", "300s"), // 5 minutes for image processing
			},
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
				RetryDelay: getDurationEnv("LOG_SHIPPING_RETRY_DELAY", "100ms"),
			},
		},
		// ✅ NEW: Multi-worker task configuration
		Tasks: TaskConfig{
			SupportedTypes: getEnvAsStringSlice("TASK_SUPPORTED_TYPES", []string{
				"profile_update",
				"email_notification",
				"image_processing",
			}),
			TypeMappings: map[string]TaskTypeInfo{
				"profile_update": {
					RoutingKey:   getEnv("ROUTING_KEY_PROFILE", "profile.task"),
					WorkerTarget: getEnv("WORKER_TARGET_PROFILE", "profile-worker"),
					Timeout:      getDurationEnv("TASK_TIMEOUT_PROFILE", "30s"),
					MaxRetries:   getEnvAsInt("TASK_MAX_RETRIES_PROFILE", 3),
					Priority:     getEnvAsInt("TASK_PRIORITY_PROFILE", 2),
				},
				"email_notification": {
					RoutingKey:   getEnv("ROUTING_KEY_EMAIL", "email.send"),
					WorkerTarget: getEnv("WORKER_TARGET_EMAIL", "email-worker"),
					Timeout:      getDurationEnv("TASK_TIMEOUT_EMAIL", "60s"),
					MaxRetries:   getEnvAsInt("TASK_MAX_RETRIES_EMAIL", 5),
					Priority:     getEnvAsInt("TASK_PRIORITY_EMAIL", 1), // Higher priority for emails
				},
				"image_processing": {
					RoutingKey:   getEnv("ROUTING_KEY_IMAGE", "image.process"),
					WorkerTarget: getEnv("WORKER_TARGET_IMAGE", "image-worker"),
					Timeout:      getDurationEnv("TASK_TIMEOUT_IMAGE", "300s"),
					MaxRetries:   getEnvAsInt("TASK_MAX_RETRIES_IMAGE", 2),
					Priority:     getEnvAsInt("TASK_PRIORITY_IMAGE", 3), // Lower priority for image processing
				},
			},
			DefaultTimeout: getDurationEnv("TASK_DEFAULT_TIMEOUT", "60s"),
			MaxPayloadSize: getEnvAsInt64("TASK_MAX_PAYLOAD_SIZE", 10*1024*1024), // 10MB
			ValidationRules: TaskValidationConfig{
				EnableStrictValidation: getEnvBool("TASK_VALIDATION_STRICT", true),
				MaxEmailsPerBurst:      getEnvAsInt("TASK_VALIDATION_MAX_EMAILS_BURST", 10),
				MaxImageSizeMB:         getEnvAsInt("TASK_VALIDATION_MAX_IMAGE_SIZE_MB", 50),
				AllowedDomains:         getEnvAsStringSlice("TASK_VALIDATION_ALLOWED_DOMAINS", []string{}),
				ValidationTimeout:      getDurationEnv("TASK_VALIDATION_TIMEOUT", "5s"),
			},
			RetryPolicy: TaskRetryConfig{
				MaxRetries:    getEnvAsInt("TASK_RETRY_MAX_RETRIES", 3),
				InitialDelay:  getDurationEnv("TASK_RETRY_INITIAL_DELAY", "1s"),
				MaxDelay:      getDurationEnv("TASK_RETRY_MAX_DELAY", "60s"),
				BackoffFactor: getEnvAsFloat64("TASK_RETRY_BACKOFF_FACTOR", 2.0),
			},
		},
	}

	return config
}

// ✅ NEW: GetRoutingKey returns the routing key for a task type
func (c *Config) GetRoutingKey(taskType string) string {
	if routingKey, exists := c.Queue.RoutingKeys[taskType]; exists {
		return routingKey
	}

	// Fallback to task type mappings
	if taskInfo, exists := c.Tasks.TypeMappings[taskType]; exists {
		return taskInfo.RoutingKey
	}

	return "profile.task" // Default fallback
}

// ✅ NEW: GetTaskTimeout returns the timeout for a specific task type
func (c *Config) GetTaskTimeout(taskType string) time.Duration {
	if timeout, exists := c.Queue.TaskTimeouts[taskType]; exists {
		return timeout
	}

	// Fallback to task type mappings
	if taskInfo, exists := c.Tasks.TypeMappings[taskType]; exists {
		return taskInfo.Timeout
	}

	return c.Tasks.DefaultTimeout
}

// ✅ NEW: GetWorkerTarget returns the worker target for a task type
func (c *Config) GetWorkerTarget(taskType string) string {
	if taskInfo, exists := c.Tasks.TypeMappings[taskType]; exists {
		return taskInfo.WorkerTarget
	}
	return "unknown-worker"
}

// ✅ NEW: IsTaskTypeSupported checks if a task type is supported
func (c *Config) IsTaskTypeSupported(taskType string) bool {
	for _, supportedType := range c.Tasks.SupportedTypes {
		if supportedType == taskType {
			return true
		}
	}
	return false
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

func getEnvAsInt64(key string, defaultValue int64) int64 {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvAsFloat64(key string, defaultValue float64) float64 {
	if value, exists := os.LookupEnv(key); exists {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
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

func getDurationEnv(key string, defaultValue string) time.Duration {
	value := getEnv(key, defaultValue)
	if duration, err := time.ParseDuration(value); err == nil {
		return duration
	}
	// If parsing fails, try to parse as milliseconds (backward compatibility)
	if ms, err := strconv.Atoi(value); err == nil {
		return time.Duration(ms) * time.Millisecond
	}
	// Final fallback - parse the default value
	if duration, err := time.ParseDuration(defaultValue); err == nil {
		return duration
	}
	return 5 * time.Second // Hard fallback
}

func getEnvAsStringSlice(key string, defaultValue []string) []string {
	if value, exists := os.LookupEnv(key); exists {
		if value == "" {
			return []string{}
		}
		return strings.Split(value, ",")
	}
	return defaultValue
}
