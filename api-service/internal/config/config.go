package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the API service
type Config struct {
	Server         ServerConfig         `mapstructure:"server"`
	Postgres       PostgresConfig       `mapstructure:"postgres"`
	Redis          RedisConfig          `mapstructure:"redis"`
	RabbitMQ       RabbitMQConfig       `mapstructure:"rabbitmq"`
	Auth           AuthConfig           `mapstructure:"auth"`
	Logging        LoggingConfig        `mapstructure:"logging"`
	Metrics        MetricsConfig        `mapstructure:"metrics"`
	CircuitBreaker CircuitBreakerConfig `mapstructure:"circuit_breaker"`
	Cache          CacheConfig          `mapstructure:"cache"`
	MinIO          MinIOConfig          `mapstructure:"minio"`
}

type ServerConfig struct {
	HTTPPort        int           `mapstructure:"http_port"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

type PostgresConfig struct {
	DSN             string        `mapstructure:"dsn"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
}

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
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	Enabled         bool          `mapstructure:"enabled"`
}

type RabbitMQConfig struct {
	Hosts            []string      `mapstructure:"hosts"`
	Username         string        `mapstructure:"username"`
	Password         string        `mapstructure:"password"`
	VHost            string        `mapstructure:"vhost"`
	PrefetchCount    int           `mapstructure:"prefetch_count"`
	ReconnectTimeout time.Duration `mapstructure:"reconnect_timeout"`
	MaxRetries       int           `mapstructure:"max_retries"`
	MessageTTL       time.Duration `mapstructure:"message_ttl"`
	ConfirmTimeout   time.Duration `mapstructure:"confirm_timeout"`
}

type AuthConfig struct {
	URL     string        `mapstructure:"url"`
	Timeout time.Duration `mapstructure:"timeout"`
	// StrictIntrospection switches token validation back to per-request
	// HTTP introspection against auth-service (the pre-v4 path, breaker
	// included). Default false: local JWKS verification (ADR-009.1).
	// Env: API_AUTH_STRICT_INTROSPECTION.
	StrictIntrospection bool `mapstructure:"strict_introspection"`
}

type LoggingConfig struct {
	Level       string `mapstructure:"level"`
	Format      string `mapstructure:"format"`
	Development bool   `mapstructure:"development"`
}

type MetricsConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Path    string `mapstructure:"path"`
	Port    int    `mapstructure:"port"`
}

type CircuitBreakerConfig struct {
	MaxRequests uint32        `mapstructure:"max_requests"`
	Interval    time.Duration `mapstructure:"interval"`
	Timeout     time.Duration `mapstructure:"timeout"`
	ReadyToTrip uint32        `mapstructure:"ready_to_trip"`
}

type CacheConfig struct {
	ProfileTTL time.Duration `mapstructure:"profile_ttl"`
	ListTTL    time.Duration `mapstructure:"list_ttl"`
}

type MinIOConfig struct {
	Endpoint        string `mapstructure:"endpoint"`
	AccessKeyID     string `mapstructure:"access_key_id"`
	SecretAccessKey string `mapstructure:"secret_access_key"`
	UseSSL          bool   `mapstructure:"use_ssl"`
	BucketName      string `mapstructure:"bucket_name"`
	MaxUploadSize   int64  `mapstructure:"max_upload_size"`
}

// Load loads configuration from environment variables and config files
func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./configs")

	setDefaults()

	viper.AutomaticEnv()
	viper.SetEnvPrefix("API")

	viper.BindEnv("postgres.dsn", "API_POSTGRES_DSN")
	viper.BindEnv("redis.password", "API_REDIS_PASSWORD")
	viper.BindEnv("redis.host", "API_REDIS_HOST")
	viper.BindEnv("redis.port", "API_REDIS_PORT")
	viper.BindEnv("rabbitmq.hosts", "API_RABBITMQ_HOSTS")
	viper.BindEnv("rabbitmq.password", "API_RABBITMQ_PASSWORD")
	viper.BindEnv("auth.url", "API_AUTH_URL")
	viper.BindEnv("auth.strict_introspection", "API_AUTH_STRICT_INTROSPECTION")
	viper.BindEnv("minio.endpoint", "MINIO_ENDPOINT")
	viper.BindEnv("minio.access_key_id", "MINIO_ACCESS_KEY")
	viper.BindEnv("minio.secret_access_key", "MINIO_SECRET_KEY")
	viper.BindEnv("minio.use_ssl", "MINIO_USE_SSL")
	viper.BindEnv("minio.bucket_name", "MINIO_BUCKET_NAME")
	viper.BindEnv("minio.max_upload_size", "MINIO_MAX_UPLOAD_SIZE")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func setDefaults() {
	viper.SetDefault("server.http_port", 8080)
	viper.SetDefault("server.read_timeout", "30s")
	viper.SetDefault("server.write_timeout", "30s")
	viper.SetDefault("server.shutdown_timeout", "10s")

	viper.SetDefault("postgres.dsn", "postgres://postgres:postgres@localhost:5432/api_db?sslmode=disable")
	viper.SetDefault("postgres.max_open_conns", 50)
	viper.SetDefault("postgres.max_idle_conns", 10)
	viper.SetDefault("postgres.conn_max_lifetime", "5m")
	viper.SetDefault("postgres.conn_max_idle_time", "1m")

	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", 6379)
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.database", 0)
	viper.SetDefault("redis.max_retries", 3)
	viper.SetDefault("redis.dial_timeout", "5s")
	viper.SetDefault("redis.read_timeout", "3s")
	viper.SetDefault("redis.write_timeout", "3s")
	viper.SetDefault("redis.pool_size", 50)
	viper.SetDefault("redis.min_idle_conns", 5)
	viper.SetDefault("redis.conn_max_lifetime", "300s")
	viper.SetDefault("redis.enabled", true)

	viper.SetDefault("rabbitmq.hosts", []string{"localhost:5672"})
	viper.SetDefault("rabbitmq.username", "guest")
	viper.SetDefault("rabbitmq.password", "guest")
	viper.SetDefault("rabbitmq.vhost", "/")
	viper.SetDefault("rabbitmq.prefetch_count", 1)
	viper.SetDefault("rabbitmq.reconnect_timeout", "5s")
	viper.SetDefault("rabbitmq.max_retries", 3)
	viper.SetDefault("rabbitmq.message_ttl", "24h")
	viper.SetDefault("rabbitmq.confirm_timeout", "5s")

	viper.SetDefault("auth.url", "http://auth-service:3000")
	viper.SetDefault("auth.timeout", "5s")
	viper.SetDefault("auth.strict_introspection", false)

	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")
	viper.SetDefault("logging.development", false)

	viper.SetDefault("metrics.enabled", true)
	viper.SetDefault("metrics.path", "/metrics")
	viper.SetDefault("metrics.port", 8081)

	viper.SetDefault("circuit_breaker.max_requests", 100)
	viper.SetDefault("circuit_breaker.interval", "10s")
	viper.SetDefault("circuit_breaker.timeout", "60s")
	viper.SetDefault("circuit_breaker.ready_to_trip", 5)

	viper.SetDefault("cache.profile_ttl", "15m")
	viper.SetDefault("cache.list_ttl", "5m")

	viper.SetDefault("minio.endpoint", "minio:9000")
	viper.SetDefault("minio.access_key_id", "")
	viper.SetDefault("minio.secret_access_key", "")
	viper.SetDefault("minio.use_ssl", false)
	viper.SetDefault("minio.bucket_name", "documents-raw")
	viper.SetDefault("minio.max_upload_size", 104857600)
}

func (c *Config) Validate() error {
	if c.Server.HTTPPort <= 0 || c.Server.HTTPPort > 65535 {
		return fmt.Errorf("invalid HTTP port: %d", c.Server.HTTPPort)
	}
	if c.Postgres.DSN == "" {
		return fmt.Errorf("postgres DSN cannot be empty")
	}
	if c.Redis.Host == "" {
		return fmt.Errorf("redis host cannot be empty")
	}
	if c.Redis.Port <= 0 || c.Redis.Port > 65535 {
		return fmt.Errorf("invalid redis port: %d", c.Redis.Port)
	}
	if c.Auth.URL == "" {
		return fmt.Errorf("auth url cannot be empty")
	}
	if c.MinIO.Endpoint != "" {
		// Both keys empty is valid: the client falls back to the ambient AWS
		// credential chain (EKS IRSA / instance role). Both keys set selects
		// static creds (compose/kind/MinIO). Only a partial config is an error.
		hasAccess := c.MinIO.AccessKeyID != ""
		hasSecret := c.MinIO.SecretAccessKey != ""
		if hasAccess != hasSecret {
			return fmt.Errorf("minio access key and secret key must both be set (static creds) or both be empty (ambient/IRSA creds)")
		}
	}
	return nil
}

func (r *RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}
