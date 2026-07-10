package config

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

// Format represents supported configuration formats
type Format string

const (
	FormatJSON Format = "json"
	FormatYAML Format = "yaml"
	FormatTOML Format = "toml"
)

// FormatDetector detects the format based on file extension
func FormatDetector(path string) Format {
	ext := filepath.Ext(path)
	switch ext {
	case ".yaml", ".yml":
		return FormatYAML
	case ".toml":
		return FormatTOML
	default:
		return FormatJSON
	}
}

// loadFromFile loads configuration from a file based on its format
func loadFromFile(path string) (*Config, error) {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	format := FormatDetector(path)
	switch format {
	case FormatYAML:
		return loadFromYAML(file)
	case FormatTOML:
		return loadFromTOML(file)
	default:
		return loadFromJSON(file)
	}
}

// loadFromJSON loads configuration from a JSON file
func loadFromJSON(reader io.Reader) (*Config, error) {
	var config Config
	if err := json.NewDecoder(reader).Decode(&config); err != nil {
		return nil, err
	}
	return &config, nil
}

// loadFromYAML loads configuration from a YAML file
func loadFromYAML(reader io.Reader) (*Config, error) {
	var config Config
	if err := yaml.NewDecoder(reader).Decode(&config); err != nil {
		return nil, err
	}
	return &config, nil
}

// loadFromTOML loads configuration from a TOML file
func loadFromTOML(reader io.Reader) (*Config, error) {
	var config Config
	if err := toml.NewDecoder(reader).Decode(&config); err != nil {
		return nil, err
	}
	return &config, nil
}
