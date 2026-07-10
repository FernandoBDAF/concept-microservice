# Logging Best Practices

## Overview

This document outlines the logging best practices for microservices development, emphasizing the importance of implementing logging from the start of development and maintaining consistent logging patterns across all services.

## Why Early Logging Matters

1. **Debugging Efficiency**

   - Easier to track issues in development
   - Better context for production problems
   - Faster root cause analysis
   - Reduced debugging time

2. **Operational Visibility**

   - Better monitoring capabilities
   - Improved alerting
   - Enhanced troubleshooting
   - Better performance analysis

3. **Development Benefits**
   - Faster development cycles
   - Better code understanding
   - Easier onboarding
   - Improved code quality

## Logging Implementation

### 1. Structured Logging

```go
// Good
logger.Info("User login successful",
    "user_id", userID,
    "ip", requestIP,
    "timestamp", time.Now(),
    "request_id", requestID)

// Bad
log.Printf("User %s logged in from %s", userID, requestIP)
```

### 2. Log Levels

```go
// Debug - Detailed information for debugging
logger.Debug("Processing request",
    "request_id", requestID,
    "headers", headers)

// Info - General operational information
logger.Info("Request completed",
    "request_id", requestID,
    "duration", duration)

// Warn - Warning messages
logger.Warn("Rate limit approaching",
    "user_id", userID,
    "current_rate", currentRate)

// Error - Error conditions
logger.Error("Failed to process request",
    "request_id", requestID,
    "error", err)
```

### 3. Context Information

```go
// Add request context
func (h *Handler) ProcessRequest(c *gin.Context) {
    logger := logging.FromContext(c)
    logger = logger.WithFields(map[string]interface{}{
        "request_id": c.GetString("request_id"),
        "user_id":    c.GetString("user_id"),
        "ip":         c.ClientIP(),
    })

    // Use logger throughout the handler
    logger.Info("Processing request")
    // ... implementation
}
```

### 4. Error Logging

```go
// Good error logging
if err != nil {
    logger.Error("Failed to create profile",
        "user_id", userID,
        "error", err,
        "error_type", reflect.TypeOf(err).String(),
        "stack_trace", debug.Stack())
    return err
}

// Bad error logging
if err != nil {
    log.Printf("Error: %v", err)
    return err
}
```

## Best Practices

### 1. Log Everything Important

- Request/response information
- Performance metrics
- Error conditions
- State changes
- Security events
- Business events

### 2. Include Context

- Request IDs
- User IDs
- Timestamps
- Service names
- Environment information
- Correlation IDs

### 3. Use Appropriate Log Levels

- DEBUG: Detailed debugging information
- INFO: General operational information
- WARN: Warning messages
- ERROR: Error conditions
- FATAL: System is unusable

### 4. Structure Your Logs

- Use JSON format
- Include standard fields
- Add business context
- Maintain consistency

### 5. Performance Considerations

- Use async logging
- Batch log writes
- Set appropriate buffer sizes
- Monitor log volume

## Implementation Examples

### 1. Request Logging Middleware

```go
func LoggingMiddleware(logger *logging.Logger) gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()
        path := c.Request.URL.Path
        requestID := c.GetHeader("X-Request-ID")

        // Create request logger
        reqLogger := logger.WithFields(map[string]interface{}{
            "request_id": requestID,
            "path":      path,
            "method":    c.Request.Method,
            "ip":        c.ClientIP(),
        })

        // Process request
        c.Next()

        // Log response
        duration := time.Since(start)
        reqLogger.Info("Request completed",
            "status", c.Writer.Status(),
            "duration", duration,
            "user_agent", c.Request.UserAgent())
    }
}
```

### 2. Error Logging

```go
func LogError(logger *logging.Logger, err error, context map[string]interface{}) {
    logger.Error("Operation failed",
        "error", err,
        "error_type", reflect.TypeOf(err).String(),
        "stack_trace", debug.Stack(),
        "context", context)
}
```

### 3. Business Event Logging

```go
func LogBusinessEvent(logger *logging.Logger, event string, data map[string]interface{}) {
    logger.Info("Business event",
        "event_type", event,
        "event_data", data,
        "timestamp", time.Now())
}
```

## Monitoring and Analysis

### 1. Log Aggregation

- Use centralized logging
- Implement log shipping
- Set up log rotation
- Configure log retention

### 2. Log Analysis

- Set up alerts
- Create dashboards
- Monitor error rates
- Track performance

### 3. Log Security

- Sanitize sensitive data
- Implement access control
- Monitor log access
- Encrypt log data

## Tools and Resources

### 1. Logging Libraries

- [zap](https://github.com/uber-go/zap)
- [logrus](https://github.com/sirupsen/logrus)
- [zerolog](https://github.com/rs/zerolog)

### 2. Log Management

- ELK Stack
- Graylog
- Loki
- Fluentd

### 3. Monitoring Tools

- Prometheus
- Grafana
- Datadog
- New Relic

## Cross-References

### Related Documents

- [Error Handling Best Practices](./error-handling-best-practices.md)
- [API Design Best Practices](./api-design-best-practices.md)
- [Database Best Practices](./database-best-practices.md)
- [Caching Best Practices](./caching-best-practices.md)
- [Security Best Practices](./security-best-practices.md)
- [Development Best Practices](./best-practices.md)

### Related Tools

- [Logging Guide](../tools/logging.md)
- [Monitoring Guide](../tools/monitoring.md)
- [Grafana Guide](../tools/grafana.md)
- [Prometheus Guide](../tools/prometheus.md)
- [Jaeger Guide](../tools/jaeger.md)
- [Gin Guide](../tools/gin.md)

## References

- [Zap Documentation](https://pkg.go.dev/go.uber.org/zap)
- [Logrus Documentation](https://github.com/sirupsen/logrus)
- [Zerolog Documentation](https://github.com/rs/zerolog)
- [ELK Stack Documentation](https://www.elastic.co/guide/index.html)
- [Graylog Documentation](https://docs.graylog.org/)
- [Loki Documentation](https://grafana.com/docs/loki/latest/)
- [Fluentd Documentation](https://docs.fluentd.org/)

## Notes

- Implement logging early
- Maintain consistency
- Monitor log volume
- Regular reviews
