# Error Handling Best Practices

## Overview

This document outlines the best practices for error handling in our microservices architecture, providing comprehensive guidelines for error types, propagation, handling, and logging.

## Error Types and Categorization

### 1. Domain Errors

```go
// Domain-specific errors
type DomainError struct {
    Code    string
    Message string
    Details map[string]interface{}
}

// Common domain error codes
const (
    ErrProfileNotFound     = "PROFILE_NOT_FOUND"
    ErrInvalidCredentials  = "INVALID_CREDENTIALS"
    ErrDuplicateEntry     = "DUPLICATE_ENTRY"
    ErrValidationFailed   = "VALIDATION_FAILED"
    ErrPermissionDenied   = "PERMISSION_DENIED"
)

// Error creation
func NewDomainError(code string, message string, details map[string]interface{}) *DomainError {
    return &DomainError{
        Code:    code,
        Message: message,
        Details: details,
    }
}
```

### 2. Infrastructure Errors

```go
// Infrastructure errors
type InfrastructureError struct {
    Type    string
    Message string
    Cause   error
}

// Common infrastructure error types
const (
    ErrDatabaseConnection = "DATABASE_CONNECTION"
    ErrCacheFailure      = "CACHE_FAILURE"
    ErrNetworkTimeout    = "NETWORK_TIMEOUT"
    ErrServiceUnavailable = "SERVICE_UNAVAILABLE"
)

// Error creation
func NewInfrastructureError(errType string, message string, cause error) *InfrastructureError {
    return &InfrastructureError{
        Type:    errType,
        Message: message,
        Cause:   cause,
    }
}
```

### 3. Validation Errors

```go
// Validation errors
type ValidationError struct {
    Field   string
    Message string
    Value   interface{}
}

// Error collection
type ValidationErrors struct {
    Errors []ValidationError
}

// Error creation
func NewValidationError(field string, message string, value interface{}) *ValidationError {
    return &ValidationError{
        Field:   field,
        Message: message,
        Value:   value,
    }
}
```

## Error Propagation Patterns

### 1. Error Wrapping

```go
// Error wrapping with context
func (s *Service) GetProfile(ctx context.Context, id string) (*Profile, error) {
    profile, err := s.repository.Get(ctx, id)
    if err != nil {
        return nil, fmt.Errorf("failed to get profile %s: %w", id, err)
    }
    return profile, nil
}

// Error unwrapping
func (s *Service) HandleError(err error) {
    if err == nil {
        return
    }

    // Unwrap error
    if domainErr, ok := err.(*DomainError); ok {
        // Handle domain error
        s.handleDomainError(domainErr)
    } else if infraErr, ok := err.(*InfrastructureError); ok {
        // Handle infrastructure error
        s.handleInfrastructureError(infraErr)
    } else {
        // Handle unknown error
        s.handleUnknownError(err)
    }
}
```

### 2. Error Context

```go
// Adding context to errors
func (s *Service) UpdateProfile(ctx context.Context, profile *Profile) error {
    if err := s.validateProfile(profile); err != nil {
        return fmt.Errorf("profile validation failed: %w", err)
    }

    if err := s.repository.Update(ctx, profile); err != nil {
        return fmt.Errorf("failed to update profile %s: %w", profile.ID, err)
    }

    return nil
}
```

## Error Handling Middleware

### 1. HTTP Error Handler

```go
// HTTP error handler middleware
func ErrorHandlerMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Next()

        // Check for errors
        if len(c.Errors) > 0 {
            err := c.Errors.Last().Err

            // Handle different error types
            switch e := err.(type) {
            case *DomainError:
                c.JSON(http.StatusBadRequest, gin.H{
                    "error": e.Code,
                    "message": e.Message,
                    "details": e.Details,
                })
            case *InfrastructureError:
                c.JSON(http.StatusServiceUnavailable, gin.H{
                    "error": e.Type,
                    "message": e.Message,
                })
            case *ValidationError:
                c.JSON(http.StatusUnprocessableEntity, gin.H{
                    "error": "VALIDATION_ERROR",
                    "field": e.Field,
                    "message": e.Message,
                })
            default:
                c.JSON(http.StatusInternalServerError, gin.H{
                    "error": "INTERNAL_ERROR",
                    "message": "An unexpected error occurred",
                })
            }
        }
    }
}
```

### 2. gRPC Error Handler

```go
// gRPC error handler
func (s *Service) handleError(err error) error {
    switch e := err.(type) {
    case *DomainError:
        return status.Error(codes.InvalidArgument, e.Message)
    case *InfrastructureError:
        return status.Error(codes.Unavailable, e.Message)
    case *ValidationError:
        return status.Error(codes.InvalidArgument, e.Message)
    default:
        return status.Error(codes.Internal, "An unexpected error occurred")
    }
}
```

## Error Logging Strategies

### 1. Structured Error Logging

```go
// Structured error logging
func (s *Service) logError(err error, context map[string]interface{}) {
    logger := s.logger.With(
        zap.String("error_type", reflect.TypeOf(err).String()),
        zap.Error(err),
    )

    // Add context
    for k, v := range context {
        logger = logger.With(zap.Any(k, v))
    }

    // Log based on error type
    switch e := err.(type) {
    case *DomainError:
        logger.Error("Domain error occurred",
            zap.String("error_code", e.Code),
            zap.Any("details", e.Details))
    case *InfrastructureError:
        logger.Error("Infrastructure error occurred",
            zap.String("error_type", e.Type),
            zap.Error(e.Cause))
    case *ValidationError:
        logger.Error("Validation error occurred",
            zap.String("field", e.Field),
            zap.Any("value", e.Value))
    default:
        logger.Error("Unknown error occurred")
    }
}
```

### 2. Error Metrics

```go
// Error metrics
type ErrorMetrics struct {
    errorCounter *prometheus.CounterVec
    errorLatency *prometheus.HistogramVec
}

func NewErrorMetrics() *ErrorMetrics {
    return &ErrorMetrics{
        errorCounter: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "error_total",
                Help: "Total number of errors by type",
            },
            []string{"error_type", "error_code"},
        ),
        errorLatency: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name:    "error_handling_duration_seconds",
                Help:    "Error handling duration in seconds",
                Buckets: prometheus.DefBuckets,
            },
            []string{"error_type"},
        ),
    }
}
```

## Best Practices

1. **Error Creation**

   - Use custom error types
   - Include relevant context
   - Maintain error hierarchy
   - Follow naming conventions

2. **Error Handling**

   - Handle errors at appropriate level
   - Don't ignore errors
   - Provide meaningful messages
   - Include error context

3. **Error Logging**

   - Log all errors
   - Include stack traces
   - Add relevant context
   - Use appropriate log levels

4. **Error Recovery**
   - Implement retry mechanisms
   - Use circuit breakers
   - Handle timeouts
   - Implement fallbacks

## Common Issues and Solutions

1. **Error Swallowing**

   - Problem: Errors are ignored
   - Solution: Always handle errors explicitly

2. **Lost Context**

   - Problem: Error context is lost
   - Solution: Use error wrapping

3. **Inconsistent Handling**
   - Problem: Different error handling patterns
   - Solution: Standardize error handling

## References

- [Go Error Handling](https://golang.org/doc/effective_go#errors)
- [Error Handling Patterns](https://blog.golang.org/error-handling-and-go)
- [Structured Error Handling](https://dave.cheney.net/2016/04/27/dont-just-check-errors-handle-them-gracefully)
- [Error Handling Best Practices](https://github.com/golang/go/wiki/Errors)
