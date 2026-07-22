// Package config loads the API's runtime configuration from environment
// variables with sane local-dev defaults. It is deliberately tiny (stdlib
// only) — swap in viper/koanf if you outgrow it. Every field below has a
// documented env var; see CONTRACTS.md §4 for the deploy-time contract.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the whole configuration surface of the publisher.
type Config struct {
	Server   ServerConfig
	Postgres PostgresConfig
	RabbitMQ RabbitMQConfig
	Auth     AuthConfig
}

type ServerConfig struct {
	HTTPPort        int           // PORT (default 8080) — serves /tasks, /healthz, /metrics
	ReadTimeout     time.Duration // SERVER_READ_TIMEOUT
	WriteTimeout    time.Duration // SERVER_WRITE_TIMEOUT
	ShutdownTimeout time.Duration // SERVER_SHUTDOWN_TIMEOUT
}

type PostgresConfig struct {
	DSN             string        // POSTGRES_DSN
	MaxOpenConns    int           // POSTGRES_MAX_OPEN_CONNS
	MaxIdleConns    int           // POSTGRES_MAX_IDLE_CONNS
	ConnMaxLifetime time.Duration // POSTGRES_CONN_MAX_LIFETIME
	ConnMaxIdleTime time.Duration // POSTGRES_CONN_MAX_IDLE_TIME
}

type RabbitMQConfig struct {
	Hosts            []string      // RABBITMQ_HOSTS (comma-separated host:port)
	Username         string        // RABBITMQ_USERNAME
	Password         string        // RABBITMQ_PASSWORD
	VHost            string        // RABBITMQ_VHOST
	PrefetchCount    int           // RABBITMQ_PREFETCH_COUNT
	ReconnectTimeout time.Duration // RABBITMQ_RECONNECT_TIMEOUT
	ConfirmTimeout   time.Duration // RABBITMQ_CONFIRM_TIMEOUT
}

// AuthConfig points the JWKS verifier at the auth service. URL is the base
// URL; the verifier appends /.well-known/jwks.json (see internal/auth).
type AuthConfig struct {
	URL     string        // AUTH_URL
	Timeout time.Duration // AUTH_TIMEOUT (JWKS fetch timeout)
	// Disabled turns OFF token verification and stamps a fixed dev user id
	// on every request. It exists so the bootstrap smoke can run without an
	// auth service. NON-PRODUCTION — never set AUTH_DISABLED=true anywhere
	// real; startup logs a loud warning when it is on.
	Disabled bool // AUTH_DISABLED
}

// Load reads the configuration from the environment. It returns an error
// only when a value is present but malformed, or when validation fails.
func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			HTTPPort:        envInt("PORT", 8080),
			ReadTimeout:     envDuration("SERVER_READ_TIMEOUT", 30*time.Second),
			WriteTimeout:    envDuration("SERVER_WRITE_TIMEOUT", 30*time.Second),
			ShutdownTimeout: envDuration("SERVER_SHUTDOWN_TIMEOUT", 10*time.Second),
		},
		Postgres: PostgresConfig{
			DSN:             envStr("POSTGRES_DSN", "postgres://postgres:postgres@localhost:5432/api_db?sslmode=disable"),
			MaxOpenConns:    envInt("POSTGRES_MAX_OPEN_CONNS", 50),
			MaxIdleConns:    envInt("POSTGRES_MAX_IDLE_CONNS", 10),
			ConnMaxLifetime: envDuration("POSTGRES_CONN_MAX_LIFETIME", 5*time.Minute),
			ConnMaxIdleTime: envDuration("POSTGRES_CONN_MAX_IDLE_TIME", time.Minute),
		},
		RabbitMQ: RabbitMQConfig{
			Hosts:            envList("RABBITMQ_HOSTS", []string{"localhost:5672"}),
			Username:         envStr("RABBITMQ_USERNAME", "guest"),
			Password:         envStr("RABBITMQ_PASSWORD", "guest"),
			VHost:            envStr("RABBITMQ_VHOST", "/"),
			PrefetchCount:    envInt("RABBITMQ_PREFETCH_COUNT", 1),
			ReconnectTimeout: envDuration("RABBITMQ_RECONNECT_TIMEOUT", 5*time.Second),
			ConfirmTimeout:   envDuration("RABBITMQ_CONFIRM_TIMEOUT", 5*time.Second),
		},
		Auth: AuthConfig{
			URL:      envStr("AUTH_URL", "http://auth-service:3000"),
			Timeout:  envDuration("AUTH_TIMEOUT", 5*time.Second),
			Disabled: envBool("AUTH_DISABLED", false),
		},
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if c.Server.HTTPPort <= 0 || c.Server.HTTPPort > 65535 {
		return fmt.Errorf("invalid HTTP port: %d", c.Server.HTTPPort)
	}
	if c.Postgres.DSN == "" {
		return fmt.Errorf("POSTGRES_DSN cannot be empty")
	}
	if len(c.RabbitMQ.Hosts) == 0 {
		return fmt.Errorf("RABBITMQ_HOSTS cannot be empty")
	}
	if c.Auth.URL == "" {
		return fmt.Errorf("AUTH_URL cannot be empty")
	}
	return nil
}

func envStr(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func envBool(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func envList(key string, def []string) []string {
	v, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(v) == "" {
		return def
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return def
	}
	return out
}
