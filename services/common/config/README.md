# Configuration Package

A flexible and secure configuration management package for Go microservices.

## Overview

The configuration package provides a standardized way to handle configuration across services, supporting multiple configuration sources and formats. It's designed to be secure, flexible, and easy to use.

## Features

- Multiple configuration sources:
  - Environment variables
  - Configuration files (YAML, JSON, TOML)
  - Command-line flags
  - Default values
- Type-safe configuration access
- Configuration validation
- Secret management
- Hot reloading support
- Environment-specific configurations

## Installation

```bash
go get github.com/your-org/common/config
```

## Quick Start

```go
package main

import (
    "log"
    "github.com/your-org/common/config"
)

func main() {
    // Load configuration
    cfg, err := config.Load("config.yaml")
    if err != nil {
        log.Fatal(err)
    }

    // Access configuration values
    port := cfg.GetInt("server.port")
    host := cfg.GetString("server.host")
    debug := cfg.GetBool("debug")
}
```

## Configuration Structure

### Basic Configuration

```yaml
# config.yaml
server:
  host: localhost
  port: 8080
  timeout: 30s

database:
  host: localhost
  port: 5432
  name: mydb
  user: ${DB_USER}
  password: ${DB_PASSWORD}

logging:
  level: info
  format: json
```

### Environment Variables

The package supports environment variable substitution:

```yaml
database:
  user: ${DB_USER}
  password: ${DB_PASSWORD}
```

### Secret Management

For sensitive data, use the secret management features:

```go
cfg := config.New()
cfg.SetSecret("database.password", "my-secret-password")
```

## API Reference

### Configuration Loading

```go
// Load from file
cfg, err := config.Load("config.yaml")

// Load with options
cfg, err := config.LoadWithOptions("config.yaml", config.Options{
    EnvPrefix: "APP_",
    HotReload: true,
})
```

### Accessing Values

```go
// Get string value
host := cfg.GetString("server.host")

// Get integer value
port := cfg.GetInt("server.port")

// Get boolean value
debug := cfg.GetBool("debug")

// Get duration
timeout := cfg.GetDuration("server.timeout")

// Get with default value
retries := cfg.GetIntWithDefault("retries", 3)
```

### Validation

```go
// Define validation rules
rules := config.ValidationRules{
    "server.port": config.Required | config.Port,
    "database.host": config.Required,
    "database.port": config.Required | config.Port,
}

// Validate configuration
err := cfg.Validate(rules)
```

## Best Practices

1. **Configuration Organization**

   - Group related settings
   - Use meaningful names
   - Document all options

2. **Security**

   - Never commit secrets
   - Use environment variables
   - Implement proper access control

3. **Environment Management**

   - Use different configs per environment
   - Validate environment-specific settings
   - Document environment requirements

4. **Error Handling**
   - Always check for errors
   - Provide meaningful error messages
   - Log configuration issues

## Examples

### Basic Server Configuration

```go
package main

import (
    "log"
    "github.com/your-org/common/config"
)

func main() {
    cfg, err := config.Load("config.yaml")
    if err != nil {
        log.Fatal(err)
    }

    server := &Server{
        Host:    cfg.GetString("server.host"),
        Port:    cfg.GetInt("server.port"),
        Timeout: cfg.GetDuration("server.timeout"),
    }

    server.Start()
}
```

### Database Configuration

```go
package main

import (
    "log"
    "github.com/your-org/common/config"
)

func main() {
    cfg, err := config.Load("config.yaml")
    if err != nil {
        log.Fatal(err)
    }

    db := &Database{
        Host:     cfg.GetString("database.host"),
        Port:     cfg.GetInt("database.port"),
        Name:     cfg.GetString("database.name"),
        User:     cfg.GetString("database.user"),
        Password: cfg.GetString("database.password"),
    }

    db.Connect()
}
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests
5. Submit a pull request

## License

This package is licensed under the MIT License - see the [LICENSE](../LICENSE) file for details.
