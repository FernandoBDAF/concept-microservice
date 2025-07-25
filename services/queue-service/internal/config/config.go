package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// QueueConfig holds the configuration for the queue service
type QueueConfig struct {
	Service struct {
		Name        string
		Version     string
		Port        int
		Environment string
	}
	RabbitMQ struct {
		Cluster struct {
			Nodes []struct {
				Host string
				Port int
			}
		}
		Options struct {
			PrefetchCount     int
			ReconnectInterval time.Duration
			MaxRetries        int
			MessageTTL        time.Duration
			ConfirmTimeout    time.Duration
		}
		Workers struct {
			Profile struct {
				Prefetch      int
				TTL           time.Duration
				DeadLetterTTL time.Duration
				MaxRetries    int
			}
			Email struct {
				Prefetch      int
				TTL           time.Duration
				DeadLetterTTL time.Duration
				MaxRetries    int
			}
			Image struct {
				Prefetch      int
				TTL           time.Duration
				DeadLetterTTL time.Duration
				MaxRetries    int
			}
		}
	}
	Logging struct {
		Level  string
		Format string
	}
	Metrics struct {
		Enabled bool
		Port    int
	}
}

// NewConfig creates a new QueueConfig with default values
func NewConfig() *QueueConfig {
	config := &QueueConfig{}

	// Set default values
	config.Service.Name = "queue-service"
	config.Service.Version = "1.0.0"
	config.Service.Port = 8080
	config.Service.Environment = "development"

	// RabbitMQ defaults
	config.RabbitMQ.Options.PrefetchCount = 10
	config.RabbitMQ.Options.ReconnectInterval = 5 * time.Second
	config.RabbitMQ.Options.MaxRetries = 3
	config.RabbitMQ.Options.MessageTTL = 24 * time.Hour      // Default TTL of 24 hours
	config.RabbitMQ.Options.ConfirmTimeout = 5 * time.Second // Default confirm timeout

	// Worker-specific defaults matching DefaultRoutingMap
	config.RabbitMQ.Workers.Profile.Prefetch = 1
	config.RabbitMQ.Workers.Profile.TTL = 24 * time.Hour
	config.RabbitMQ.Workers.Profile.DeadLetterTTL = 7 * 24 * time.Hour
	config.RabbitMQ.Workers.Profile.MaxRetries = 3

	config.RabbitMQ.Workers.Email.Prefetch = 5
	config.RabbitMQ.Workers.Email.TTL = 1 * time.Hour
	config.RabbitMQ.Workers.Email.DeadLetterTTL = 24 * time.Hour
	config.RabbitMQ.Workers.Email.MaxRetries = 5

	config.RabbitMQ.Workers.Image.Prefetch = 1
	config.RabbitMQ.Workers.Image.TTL = 6 * time.Hour
	config.RabbitMQ.Workers.Image.DeadLetterTTL = 3 * 24 * time.Hour
	config.RabbitMQ.Workers.Image.MaxRetries = 2

	// Logging defaults
	config.Logging.Level = "info"
	config.Logging.Format = "json"

	// Metrics defaults
	config.Metrics.Enabled = true
	config.Metrics.Port = 9090

	return config
}

// LoadFromEnv loads configuration from environment variables
func (c *QueueConfig) LoadFromEnv() error {
	// Service configuration
	if port := os.Getenv("SERVICE_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			c.Service.Port = p
		}
	}
	if env := os.Getenv("SERVICE_ENV"); env != "" {
		c.Service.Environment = env
	}

	// RabbitMQ configuration
	if nodes := os.Getenv("RABBITMQ_NODES"); nodes != "" {
		nodeList := strings.Split(nodes, ",")
		c.RabbitMQ.Cluster.Nodes = make([]struct {
			Host string
			Port int
		}, len(nodeList))

		for i, node := range nodeList {
			parts := strings.Split(node, ":")
			if len(parts) != 2 {
				return fmt.Errorf("invalid RabbitMQ node format: %s", node)
			}
			c.RabbitMQ.Cluster.Nodes[i].Host = parts[0]
			if port, err := strconv.Atoi(parts[1]); err == nil {
				c.RabbitMQ.Cluster.Nodes[i].Port = port
			} else {
				return fmt.Errorf("invalid RabbitMQ port: %s", parts[1])
			}
		}
	}

	// Message TTL configuration
	if ttl := os.Getenv("RABBITMQ_MESSAGE_TTL"); ttl != "" {
		if t, err := time.ParseDuration(ttl); err == nil {
			c.RabbitMQ.Options.MessageTTL = t
		} else {
			return fmt.Errorf("invalid message TTL: %s", ttl)
		}
	}

	// Confirm timeout configuration
	if timeout := os.Getenv("RABBITMQ_CONFIRM_TIMEOUT"); timeout != "" {
		if t, err := time.ParseDuration(timeout); err == nil {
			c.RabbitMQ.Options.ConfirmTimeout = t
		} else {
			return fmt.Errorf("invalid confirm timeout: %s", timeout)
		}
	}

	// Worker-specific configuration
	// Profile worker configuration
	if prefetch := os.Getenv("RABBITMQ_PROFILE_PREFETCH"); prefetch != "" {
		if p, err := strconv.Atoi(prefetch); err == nil {
			c.RabbitMQ.Workers.Profile.Prefetch = p
		}
	}
	if ttl := os.Getenv("RABBITMQ_PROFILE_TTL"); ttl != "" {
		if t, err := time.ParseDuration(ttl); err == nil {
			c.RabbitMQ.Workers.Profile.TTL = t
		} else {
			return fmt.Errorf("invalid profile TTL: %s", ttl)
		}
	}
	if dlTtl := os.Getenv("RABBITMQ_PROFILE_DL_TTL"); dlTtl != "" {
		if t, err := time.ParseDuration(dlTtl); err == nil {
			c.RabbitMQ.Workers.Profile.DeadLetterTTL = t
		} else {
			return fmt.Errorf("invalid profile dead letter TTL: %s", dlTtl)
		}
	}
	if retries := os.Getenv("RABBITMQ_PROFILE_MAX_RETRIES"); retries != "" {
		if r, err := strconv.Atoi(retries); err == nil {
			c.RabbitMQ.Workers.Profile.MaxRetries = r
		}
	}

	// Email worker configuration
	if prefetch := os.Getenv("RABBITMQ_EMAIL_PREFETCH"); prefetch != "" {
		if p, err := strconv.Atoi(prefetch); err == nil {
			c.RabbitMQ.Workers.Email.Prefetch = p
		}
	}
	if ttl := os.Getenv("RABBITMQ_EMAIL_TTL"); ttl != "" {
		if t, err := time.ParseDuration(ttl); err == nil {
			c.RabbitMQ.Workers.Email.TTL = t
		} else {
			return fmt.Errorf("invalid email TTL: %s", ttl)
		}
	}
	if dlTtl := os.Getenv("RABBITMQ_EMAIL_DL_TTL"); dlTtl != "" {
		if t, err := time.ParseDuration(dlTtl); err == nil {
			c.RabbitMQ.Workers.Email.DeadLetterTTL = t
		} else {
			return fmt.Errorf("invalid email dead letter TTL: %s", dlTtl)
		}
	}
	if retries := os.Getenv("RABBITMQ_EMAIL_MAX_RETRIES"); retries != "" {
		if r, err := strconv.Atoi(retries); err == nil {
			c.RabbitMQ.Workers.Email.MaxRetries = r
		}
	}

	// Image worker configuration
	if prefetch := os.Getenv("RABBITMQ_IMAGE_PREFETCH"); prefetch != "" {
		if p, err := strconv.Atoi(prefetch); err == nil {
			c.RabbitMQ.Workers.Image.Prefetch = p
		}
	}
	if ttl := os.Getenv("RABBITMQ_IMAGE_TTL"); ttl != "" {
		if t, err := time.ParseDuration(ttl); err == nil {
			c.RabbitMQ.Workers.Image.TTL = t
		} else {
			return fmt.Errorf("invalid image TTL: %s", ttl)
		}
	}
	if dlTtl := os.Getenv("RABBITMQ_IMAGE_DL_TTL"); dlTtl != "" {
		if t, err := time.ParseDuration(dlTtl); err == nil {
			c.RabbitMQ.Workers.Image.DeadLetterTTL = t
		} else {
			return fmt.Errorf("invalid image dead letter TTL: %s", dlTtl)
		}
	}
	if retries := os.Getenv("RABBITMQ_IMAGE_MAX_RETRIES"); retries != "" {
		if r, err := strconv.Atoi(retries); err == nil {
			c.RabbitMQ.Workers.Image.MaxRetries = r
		}
	}

	// Logging configuration
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		c.Logging.Level = level
	}
	if format := os.Getenv("LOG_FORMAT"); format != "" {
		c.Logging.Format = format
	}

	// Metrics configuration
	if enabled := os.Getenv("METRICS_ENABLED"); enabled != "" {
		c.Metrics.Enabled = enabled == "true"
	}
	if port := os.Getenv("METRICS_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			c.Metrics.Port = p
		}
	}

	return nil
}

// Validate checks if the configuration is valid
func (c *QueueConfig) Validate() error {
	if len(c.RabbitMQ.Cluster.Nodes) == 0 {
		return fmt.Errorf("at least one RabbitMQ node must be configured")
	}

	if c.Service.Port <= 0 {
		return fmt.Errorf("invalid service port: %d", c.Service.Port)
	}

	if c.Metrics.Enabled && c.Metrics.Port <= 0 {
		return fmt.Errorf("invalid metrics port: %d", c.Metrics.Port)
	}

	if c.RabbitMQ.Options.MessageTTL <= 0 {
		return fmt.Errorf("invalid message TTL: %v", c.RabbitMQ.Options.MessageTTL)
	}

	if c.RabbitMQ.Options.ConfirmTimeout <= 0 {
		return fmt.Errorf("invalid confirm timeout: %v", c.RabbitMQ.Options.ConfirmTimeout)
	}

	// Validate worker-specific configurations
	if c.RabbitMQ.Workers.Profile.Prefetch <= 0 {
		return fmt.Errorf("invalid profile prefetch count: %d", c.RabbitMQ.Workers.Profile.Prefetch)
	}
	if c.RabbitMQ.Workers.Profile.TTL <= 0 {
		return fmt.Errorf("invalid profile TTL: %v", c.RabbitMQ.Workers.Profile.TTL)
	}
	if c.RabbitMQ.Workers.Profile.DeadLetterTTL <= 0 {
		return fmt.Errorf("invalid profile dead letter TTL: %v", c.RabbitMQ.Workers.Profile.DeadLetterTTL)
	}
	if c.RabbitMQ.Workers.Profile.MaxRetries < 0 {
		return fmt.Errorf("invalid profile max retries: %d", c.RabbitMQ.Workers.Profile.MaxRetries)
	}

	if c.RabbitMQ.Workers.Email.Prefetch <= 0 {
		return fmt.Errorf("invalid email prefetch count: %d", c.RabbitMQ.Workers.Email.Prefetch)
	}
	if c.RabbitMQ.Workers.Email.TTL <= 0 {
		return fmt.Errorf("invalid email TTL: %v", c.RabbitMQ.Workers.Email.TTL)
	}
	if c.RabbitMQ.Workers.Email.DeadLetterTTL <= 0 {
		return fmt.Errorf("invalid email dead letter TTL: %v", c.RabbitMQ.Workers.Email.DeadLetterTTL)
	}
	if c.RabbitMQ.Workers.Email.MaxRetries < 0 {
		return fmt.Errorf("invalid email max retries: %d", c.RabbitMQ.Workers.Email.MaxRetries)
	}

	if c.RabbitMQ.Workers.Image.Prefetch <= 0 {
		return fmt.Errorf("invalid image prefetch count: %d", c.RabbitMQ.Workers.Image.Prefetch)
	}
	if c.RabbitMQ.Workers.Image.TTL <= 0 {
		return fmt.Errorf("invalid image TTL: %v", c.RabbitMQ.Workers.Image.TTL)
	}
	if c.RabbitMQ.Workers.Image.DeadLetterTTL <= 0 {
		return fmt.Errorf("invalid image dead letter TTL: %v", c.RabbitMQ.Workers.Image.DeadLetterTTL)
	}
	if c.RabbitMQ.Workers.Image.MaxRetries < 0 {
		return fmt.Errorf("invalid image max retries: %d", c.RabbitMQ.Workers.Image.MaxRetries)
	}

	return nil
}
