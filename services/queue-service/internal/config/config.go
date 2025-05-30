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
	config.RabbitMQ.Options.MessageTTL = 24 * time.Hour // Default TTL of 24 hours

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

	return nil
}
