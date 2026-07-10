package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the cache service
type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Redis     RedisConfig     `mapstructure:"redis"`
	Logging   LoggingConfig   `mapstructure:"logging"`
	Metrics   MetricsConfig   `mapstructure:"metrics"`
	CircuitBr CircuitBrConfig `mapstructure:"circuit_breaker"`
	Cache     CacheConfig     `mapstructure:"cache"`
}

// ServerConfig holds HTTP and gRPC server configuration
type ServerConfig struct {
	HTTPPort        int           `mapstructure:"http_port"`
	GRPCPort        int           `mapstructure:"grpc_port"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

// RedisConfig holds Redis connection configuration
type RedisConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	Password        string        `mapstructure:"password"`
	Database        int           `mapstructure:"database"`
	MaxRetries      int           `mapstructure:"max_retries"`
	DialTimeout     time.Duration `mapstructure:"dial_timeout"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	PoolSize        int           `mapstructure:"pool_size"`
	MinIdleConns    int           `mapstructure:"min_idle_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	Enabled         bool          `mapstructure:"enabled"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level       string `mapstructure:"level"`
	Format      string `mapstructure:"format"` // json or console
	Development bool   `mapstructure:"development"`
}

// MetricsConfig holds Prometheus metrics configuration
type MetricsConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Path    string `mapstructure:"path"`
	Port    int    `mapstructure:"port"`
}

// CircuitBrConfig holds circuit breaker configuration
type CircuitBrConfig struct {
	MaxRequests uint32        `mapstructure:"max_requests"`
	Interval    time.Duration `mapstructure:"interval"`
	Timeout     time.Duration `mapstructure:"timeout"`
	ReadyToTrip uint32        `mapstructure:"ready_to_trip"`
}

// CacheConfig holds cache-specific configuration
type CacheConfig struct {
	DefaultTTL      time.Duration `mapstructure:"default_ttl"`
	ProfileTTL      time.Duration `mapstructure:"profile_ttl"`
	TaskTTL         time.Duration `mapstructure:"task_ttl"`
	SessionTTL      time.Duration `mapstructure:"session_ttl"`
	QueueMetricsTTL time.Duration `mapstructure:"queue_metrics_ttl"`
	WorkerStatusTTL time.Duration `mapstructure:"worker_status_ttl"`
	MaxKeySize      int           `mapstructure:"max_key_size"`
	MaxValueSize    int           `mapstructure:"max_value_size"`
	BatchSize       int           `mapstructure:"batch_size"`
	JSONCompression bool          `mapstructure:"json_compression"`
}

// Load loads configuration from environment variables and config files
func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./configs")

	// Set defaults
	setDefaults()

	// Bind environment variables
	viper.AutomaticEnv()
	viper.SetEnvPrefix("CACHE")

	// Explicit bindings for nested struct fields
	viper.BindEnv("redis.password", "CACHE_REDIS_PASSWORD")
	viper.BindEnv("redis.host", "CACHE_REDIS_HOST")
	viper.BindEnv("redis.port", "CACHE_REDIS_PORT")

	// Read config file if it exists
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return &config, nil
}

// setDefaults sets default configuration values
func setDefaults() {
	// Server defaults
	viper.SetDefault("server.http_port", 8080)
	viper.SetDefault("server.grpc_port", 9090)
	viper.SetDefault("server.read_timeout", "30s")
	viper.SetDefault("server.write_timeout", "30s")
	viper.SetDefault("server.shutdown_timeout", "10s")

	// Redis defaults
	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", 6379)
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.database", 0)
	viper.SetDefault("redis.max_retries", 3)
	viper.SetDefault("redis.dial_timeout", "5s")
	viper.SetDefault("redis.read_timeout", "3s")
	viper.SetDefault("redis.write_timeout", "3s")
	viper.SetDefault("redis.pool_size", 100)
	viper.SetDefault("redis.min_idle_conns", 10)
	viper.SetDefault("redis.max_idle_conns", 50)
	viper.SetDefault("redis.conn_max_lifetime", "300s")
	viper.SetDefault("redis.enabled", true)

	// Logging defaults
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")
	viper.SetDefault("logging.development", false)

	// Metrics defaults
	viper.SetDefault("metrics.enabled", true)
	viper.SetDefault("metrics.path", "/metrics")
	viper.SetDefault("metrics.port", 8081)

	// Circuit breaker defaults
	viper.SetDefault("circuit_breaker.max_requests", 100)
	viper.SetDefault("circuit_breaker.interval", "10s")
	viper.SetDefault("circuit_breaker.timeout", "60s")
	viper.SetDefault("circuit_breaker.ready_to_trip", 5)

	// Cache defaults
	viper.SetDefault("cache.default_ttl", "1h")
	viper.SetDefault("cache.profile_ttl", "30m")
	viper.SetDefault("cache.task_ttl", "15m")
	viper.SetDefault("cache.session_ttl", "30m")
	viper.SetDefault("cache.queue_metrics_ttl", "2m")
	viper.SetDefault("cache.worker_status_ttl", "10m")
	viper.SetDefault("cache.max_key_size", 512)
	viper.SetDefault("cache.max_value_size", 1048576)
	viper.SetDefault("cache.batch_size", 100)
	viper.SetDefault("cache.json_compression", false)
}

// GetRedisAddr returns the Redis connection address
func (r *RedisConfig) GetRedisAddr() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Server.HTTPPort <= 0 || c.Server.HTTPPort > 65535 {
		return fmt.Errorf("invalid HTTP port: %d", c.Server.HTTPPort)
	}

	if c.Server.GRPCPort <= 0 || c.Server.GRPCPort > 65535 {
		return fmt.Errorf("invalid gRPC port: %d", c.Server.GRPCPort)
	}

	if c.Redis.Host == "" {
		return fmt.Errorf("Redis host cannot be empty")
	}

	if c.Redis.Port <= 0 || c.Redis.Port > 65535 {
		return fmt.Errorf("invalid Redis port: %d", c.Redis.Port)
	}

	if c.Redis.PoolSize <= 0 {
		return fmt.Errorf("Redis pool size must be positive")
	}

	return nil
}
