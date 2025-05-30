package config

import (
	"os"
	"strings"
)

// Config represents the base configuration structure
type Config struct {
	Environment string         `json:"environment" yaml:"environment" toml:"environment"`
	Version     string         `json:"version" yaml:"version" toml:"version"`
	Settings    map[string]any `json:"settings" yaml:"settings" toml:"settings"`
}

// New creates a new Config instance
func New() *Config {
	return &Config{
		Settings: make(map[string]any),
	}
}

// LoadConfig loads configuration from a file and applies environment variable overrides
func LoadConfig(path string) (*Config, error) {
	config, err := loadFromFile(path)
	if err != nil {
		return nil, err
	}

	// Apply environment variable overrides
	config.applyEnvOverrides()

	return config, nil
}

// applyEnvOverrides applies environment variable overrides to the configuration
func (c *Config) applyEnvOverrides() {
	// Override environment
	if env := os.Getenv("APP_ENV"); env != "" {
		c.Environment = env
	}

	// Override version
	if version := os.Getenv("APP_VERSION"); version != "" {
		c.Version = version
	}

	// Override settings
	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, "APP_SETTING_") {
			continue
		}

		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimPrefix(parts[0], "APP_SETTING_")
		key = strings.ToLower(key)
		key = strings.ReplaceAll(key, "_", ".")
		c.Settings[key] = parts[1]
	}
}

// GetString retrieves a string value from settings
func (c *Config) GetString(key string) string {
	if val, ok := c.Settings[key].(string); ok {
		return val
	}
	return ""
}

// GetInt retrieves an integer value from settings
func (c *Config) GetInt(key string) int {
	if val, ok := c.Settings[key].(float64); ok {
		return int(val)
	}
	return 0
}

// GetBool retrieves a boolean value from settings
func (c *Config) GetBool(key string) bool {
	if val, ok := c.Settings[key].(bool); ok {
		return val
	}
	return false
}

// GetFloat retrieves a float64 value from settings
func (c *Config) GetFloat(key string) float64 {
	if val, ok := c.Settings[key].(float64); ok {
		return val
	}
	return 0.0
}

// GetStringSlice retrieves a string slice from settings
func (c *Config) GetStringSlice(key string) []string {
	if val, ok := c.Settings[key].([]interface{}); ok {
		result := make([]string, len(val))
		for i, v := range val {
			if str, ok := v.(string); ok {
				result[i] = str
			}
		}
		return result
	}
	return nil
}

// GetStringMap retrieves a map[string]interface{} from settings
func (c *Config) GetStringMap(key string) map[string]interface{} {
	if val, ok := c.Settings[key].(map[string]interface{}); ok {
		return val
	}
	return nil
}

// Set sets a value in settings
func (c *Config) Set(key string, value interface{}) {
	c.Settings[key] = value
}

// Has checks if a key exists in settings
func (c *Config) Has(key string) bool {
	_, exists := c.Settings[key]
	return exists
}

// Delete removes a key from settings
func (c *Config) Delete(key string) {
	delete(c.Settings, key)
}
