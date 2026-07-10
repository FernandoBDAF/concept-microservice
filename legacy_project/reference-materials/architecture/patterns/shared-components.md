INITIAL CONTEXT FOR LLM - never change the context-----------------------------
-> THIS SECTION IS A GUIDELINE TO THE LLM CONSIDER BEFORE WORKING IN THIS FILE, DO NOT CHANGE THIS

-> GOES OF THE SHARED COMPONENTS PATTERN:

- This document describes the shared components and best practices used across all microservices
- It covers common utilities, models, and patterns that ensure consistency
- Includes implementation guidelines and code examples
- All components are implemented and tested in the current architecture
- For LLM-specific guidelines, refer to [LLM Integration Guide](../../../docs/llm/README.md)

-> CONSIDERER BEFORE UPDATING THIS FILE:

- This is a documentation file about shared components and best practices
- Never add fictional dates, version numbers, or metrics
- Changes should be incremental and based on verified information
- Add comments for clarification when needed
- Maintain LLM-friendly format

---

# Shared Components Pattern

## Context

- When to use: For implementing common functionality across all microservices
- Problem it solves: Ensures consistency, reduces code duplication, and improves maintainability
- Related patterns: Logging, Configuration, Health Checks, Error Handling

## Solution

### Logging Package

Features:

- Structured logging
- Context propagation
- Log levels
- Request tracing
- Error tracking

Implementation:

```go
// pkg/logging/logger.go
package logging

import (
    "context"
    "log"
    "os"
)

type Logger struct {
    *log.Logger
    level string
}

func NewLogger(level string) *Logger {
    return &Logger{
        Logger: log.New(os.Stdout, "", log.LstdFlags),
        level:  level,
    }
}

func (l *Logger) WithContext(ctx context.Context) *Logger {
    // Add context information to logs
    return l
}

func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
    // Add structured fields to logs
    return l
}
```

### Common Models

Features:

- Standard response formats
- Common data structures
- Validation rules
- Type definitions

Implementation:

```go
// pkg/models/common.go
package models

type BaseResponse struct {
    Status  string      `json:"status"`
    Data    interface{} `json:"data,omitempty"`
    Error   string      `json:"error,omitempty"`
    Message string      `json:"message,omitempty"`
}

type Pagination struct {
    Page     int `json:"page"`
    PageSize int `json:"page_size"`
    Total    int `json:"total"`
}
```

### Configuration Package

Features:

- Environment-based configuration
- Default values
- Validation
- Type safety

Implementation:

```go
// pkg/config/config.go
package config

type BaseConfig struct {
    Environment string
    LogLevel    string
    ServiceName string
    Version     string
}

func LoadBaseConfig() *BaseConfig {
    return &BaseConfig{
        Environment: getEnvOrDefault("ENV", "development"),
        LogLevel:    getEnvOrDefault("LOG_LEVEL", "info"),
        ServiceName: getEnvOrDefault("SERVICE_NAME", "unknown"),
        Version:     getEnvOrDefault("VERSION", "0.0.1"),
    }
}
```

### Health Check Package

Features:

- Standard health check interface
- Dependency checking
- Status reporting
- Metrics collection

Implementation:

```go
// pkg/health/checker.go
package health

type HealthChecker struct {
    checks map[string]Check
}

type Check func() error

func NewHealthChecker() *HealthChecker {
    return &HealthChecker{
        checks: make(map[string]Check),
    }
}

func (h *HealthChecker) AddCheck(name string, check Check) {
    h.checks[name] = check
}
```

## Benefits

- Consistent implementation across services
- Reduced code duplication
- Improved maintainability
- Standardized patterns
- Easier onboarding

## Drawbacks

- Need for version management
- Potential tight coupling
- Update coordination
- Testing complexity
- Documentation overhead

## Examples

### Logging Implementation

```go
func (h *Handler) CreateProfile(c *gin.Context) {
    logger := logging.FromContext(c)
    logger.Info("Creating new profile",
        "user_id", c.GetString("user_id"),
        "request_id", c.GetString("request_id"))
    // ... implementation
}
```

### Model Implementation

```go
// pkg/models/profile.go
type Profile struct {
    ID        string    `json:"id" validate:"required"`
    UserID    string    `json:"user_id" validate:"required"`
    FirstName string    `json:"first_name" validate:"required"`
    LastName  string    `json:"last_name" validate:"required"`
    Email     string    `json:"email" validate:"required,email"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

### Error Handling

```go
// pkg/errors/errors.go
type ServiceError struct {
    Code    string
    Message string
    Details map[string]interface{}
}

func NewServiceError(code string, message string) *ServiceError {
    return &ServiceError{
        Code:    code,
        Message: message,
        Details: make(map[string]interface{}),
    }
}
```

## Related Patterns

- Logging: For consistent logging across services
- Configuration: For standardized configuration management
- Health Checks: For service health monitoring
- Error Handling: For consistent error management
- Testing: For shared testing utilities

## Notes

- Keep shared components up to date
- Document changes thoroughly
- Maintain backward compatibility
- Test across services
- Review usage regularly
