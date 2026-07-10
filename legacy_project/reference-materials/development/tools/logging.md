# Logging Frameworks Guide

## Overview

This guide covers the logging frameworks and practices used in our microservices architecture. We use structured logging with Zap for Go services, along with ELK Stack (Elasticsearch, Logstash, Kibana) for log aggregation and analysis.

## Core Logging Tools

### 1. Zap Logger

Configuration and usage:

```go
// Logger configuration
type Logger struct {
    zap *zap.Logger
}

func NewLogger(env string) (*Logger, error) {
    var config zap.Config

    if env == "production" {
        config = zap.NewProductionConfig()
        config.EncoderConfig.TimeKey = "timestamp"
        config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
    } else {
        config = zap.NewDevelopmentConfig()
        config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
    }

    logger, err := config.Build()
    if err != nil {
        return nil, fmt.Errorf("failed to create logger: %w", err)
    }

    return &Logger{zap: logger}, nil
}

// Logger methods
func (l *Logger) Info(msg string, fields ...zap.Field) {
    l.zap.Info(msg, fields...)
}

func (l *Logger) Error(msg string, err error, fields ...zap.Field) {
    fields = append(fields, zap.Error(err))
    l.zap.Error(msg, fields...)
}

func (l *Logger) With(fields ...zap.Field) *Logger {
    return &Logger{zap: l.zap.With(fields...)}
}
```

### 2. Logstash Configuration

```yaml
# logstash.conf
input {
beats {
port => 5044
}
}

filter {
if [type] == "profile-service" {
json {
source => "message"
}
date {
match => [ "timestamp", "ISO8601" ]
target => "@timestamp"
}
}
}

output {
elasticsearch {
hosts => ["elasticsearch:9200"]
index => "profile-service-%{+YYYY.MM.dd}"
}
}
```

### 3. Elasticsearch Index Template

```json
{
  "index_patterns": ["profile-service-*"],
  "settings": {
    "number_of_shards": 3,
    "number_of_replicas": 1
  },
  "mappings": {
    "properties": {
      "timestamp": {
        "type": "date"
      },
      "level": {
        "type": "keyword"
      },
      "message": {
        "type": "text"
      },
      "service": {
        "type": "keyword"
      },
      "trace_id": {
        "type": "keyword"
      },
      "user_id": {
        "type": "keyword"
      },
      "error": {
        "type": "object",
        "properties": {
          "message": {
            "type": "text"
          },
          "stack": {
            "type": "text"
          }
        }
      }
    }
  }
}
```

## Logging Implementation

### 1. Service Logging

```go
// Service logging implementation
type Service struct {
    logger *Logger
}

func NewService(logger *Logger) *Service {
    return &Service{
        logger: logger.With(
            zap.String("service", "profile-service"),
            zap.String("version", "1.0.0"),
        ),
    }
}

func (s *Service) GetProfile(ctx context.Context, id string) (*Profile, error) {
    logger := s.logger.With(
        zap.String("trace_id", trace.SpanFromContext(ctx).SpanContext().TraceID().String()),
        zap.String("profile_id", id),
    )

    logger.Info("fetching profile")

    profile, err := s.repository.Get(ctx, id)
    if err != nil {
        logger.Error("failed to fetch profile", err)
        return nil, err
    }

    logger.Info("profile fetched successfully",
        zap.String("name", profile.Name),
        zap.String("email", profile.Email),
    )

    return profile, nil
}
```

### 2. Middleware Logging

```go
// Logging middleware
func LoggingMiddleware(logger *Logger) gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()
        path := c.Request.URL.Path
        method := c.Request.Method

        // Create request logger
        reqLogger := logger.With(
            zap.String("trace_id", c.GetHeader("X-Trace-ID")),
            zap.String("method", method),
            zap.String("path", path),
            zap.String("client_ip", c.ClientIP()),
        )

        // Log request
        reqLogger.Info("incoming request")

        // Process request
        c.Next()

        // Log response
        duration := time.Since(start)
        status := c.Writer.Status()

        reqLogger.Info("request completed",
            zap.Int("status", status),
            zap.Duration("duration", duration),
        )

        // Log errors
        if len(c.Errors) > 0 {
            reqLogger.Error("request failed",
                errors.New(c.Errors.String()),
                zap.Int("status", status),
            )
        }
    }
}
```

## Best Practices

1. **Log Structure**

   - Use structured logging
   - Include relevant context
   - Add correlation IDs
   - Maintain consistent format

2. **Log Levels**

   - ERROR: Application errors
   - WARN: Warning conditions
   - INFO: General information
   - DEBUG: Detailed information
   - TRACE: Very detailed information

3. **Performance**

   - Use async logging
   - Implement log batching
   - Configure appropriate buffers
   - Monitor log volume

4. **Security**
   - Sanitize sensitive data
   - Implement log rotation
   - Set proper permissions
   - Monitor log access

## Common Issues and Solutions

1. **High Log Volume**

   - Problem: Too many logs
   - Solution: Adjust log levels, implement sampling

2. **Performance Impact**

   - Problem: Logging affecting performance
   - Solution: Use async logging, optimize format

3. **Storage Issues**
   - Problem: Log storage growing too fast
   - Solution: Implement retention policies, compression

## References

- [Zap Documentation](https://pkg.go.dev/go.uber.org/zap)
- [ELK Stack Documentation](https://www.elastic.co/guide/index.html)
- [Logging Best Practices](https://www.elastic.co/guide/en/elasticsearch/reference/current/logging.html)
- [Structured Logging Guide](https://www.elastic.co/guide/en/ecs/current/ecs-log.html)
