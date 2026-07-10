# Logging Package

A structured logging package for Go microservices with support for multiple backends and formats.

## Overview

The logging package provides a standardized way to handle logging across services, supporting multiple output formats, log levels, and backends. It's designed to be flexible, performant, and easy to integrate.

## Features

- Multiple log levels (DEBUG, INFO, WARN, ERROR, FATAL)
- Structured logging with JSON output
- Multiple backends (console, file, syslog)
- Context support
- Log rotation
- Performance optimized
- Integration with common logging systems
- Custom formatters
- Field support

## Installation

```bash
go get github.com/your-org/common/logging
```

## Quick Start

```go
package main

import (
    "github.com/your-org/common/logging"
)

func main() {
    // Create a new logger
    logger := logging.NewLogger()

    // Log messages
    logger.Info("Application started")
    logger.Debug("Processing request", "request_id", "123")
    logger.Error("Failed to process request", "error", err)

    // Log with fields
    logger.WithFields(map[string]interface{}{
        "user_id": "123",
        "action":  "login",
    }).Info("User logged in")
}
```

## Configuration

### Basic Configuration

```go
// Create logger with options
logger := logging.NewLogger(logging.Options{
    Level:      logging.InfoLevel,
    Format:     logging.JSONFormat,
    Output:     logging.ConsoleOutput,
    TimeFormat: time.RFC3339,
})
```

### File Output

```go
// Create logger with file output
logger := logging.NewLogger(logging.Options{
    Level:  logging.InfoLevel,
    Format: logging.JSONFormat,
    Output: logging.FileOutput,
    FileOptions: logging.FileOptions{
        Path:     "/var/log/app.log",
        MaxSize:  100, // MB
        MaxFiles: 5,
    },
})
```

## Log Levels

```go
// Debug level - detailed information
logger.Debug("Processing request", "request_id", "123")

// Info level - general information
logger.Info("Request processed", "request_id", "123")

// Warn level - warning messages
logger.Warn("Rate limit approaching", "current", 95, "limit", 100)

// Error level - error messages
logger.Error("Failed to process request", "error", err)

// Fatal level - fatal errors
logger.Fatal("Cannot start application", "error", err)
```

## Structured Logging

```go
// Log with fields
logger.WithFields(map[string]interface{}{
    "user_id":    "123",
    "action":     "login",
    "ip_address": "192.168.1.1",
    "user_agent": "Mozilla/5.0",
}).Info("User logged in")

// Log with context
ctx := context.WithValue(context.Background(), "request_id", "123")
logger.WithContext(ctx).Info("Processing request")
```

## Best Practices

1. **Log Levels**

   - Use appropriate log levels
   - Don't log sensitive information
   - Include relevant context
   - Be consistent with log messages

2. **Performance**

   - Use structured logging
   - Avoid string concatenation
   - Use appropriate log levels
   - Consider log rotation

3. **Security**

   - Don't log sensitive data
   - Sanitize log messages
   - Use appropriate log levels
   - Implement log retention

4. **Maintenance**
   - Use consistent formatting
   - Include timestamps
   - Add request IDs
   - Document log patterns

## Examples

### HTTP Handler

```go
package main

import (
    "net/http"
    "github.com/your-org/common/logging"
)

func handler(w http.ResponseWriter, r *http.Request) {
    logger := logging.FromContext(r.Context())

    logger.Info("Processing request",
        "method", r.Method,
        "path", r.URL.Path,
        "ip", r.RemoteAddr,
    )

    // Process request...

    logger.Info("Request processed",
        "method", r.Method,
        "path", r.URL.Path,
        "status", 200,
    )
}
```

### Database Operations

```go
package main

import (
    "database/sql"
    "github.com/your-org/common/logging"
)

func getUser(db *sql.DB, id string) (*User, error) {
    logger := logging.NewLogger()

    logger.Debug("Querying user", "id", id)

    user := &User{}
    err := db.QueryRow("SELECT * FROM users WHERE id = ?", id).Scan(&user.ID, &user.Name)
    if err != nil {
        logger.Error("Failed to query user",
            "id", id,
            "error", err,
        )
        return nil, err
    }

    logger.Debug("User found", "id", id, "name", user.Name)
    return user, nil
}
```

### Error Handling

```go
package main

import (
    "github.com/your-org/common/logging"
)

func processRequest() {
    logger := logging.NewLogger()

    defer func() {
        if err := recover(); err != nil {
            logger.Error("Recovered from panic",
                "error", err,
                "stack", debug.Stack(),
            )
        }
    }()

    // Process request...
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
