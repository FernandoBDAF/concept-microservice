# Migration Guide for Common Packages

## Overview

This guide provides step-by-step instructions for migrating services to use the new common packages. The migration process is designed to be incremental and non-disruptive, allowing services to adopt the common packages at their own pace.

## Prerequisites

Before starting the migration, ensure you have:

1. Go 1.21 or later installed
2. Access to the common packages repository
3. Understanding of your service's current package usage
4. Test environment for validation

## Migration Steps

### 1. Update Dependencies

Add the common packages to your service's `go.mod`:

```go
require (
    github.com/your-org/common/config v0.1.0
    github.com/your-org/common/errors v0.1.0
    github.com/your-org/common/logging v0.1.0
    github.com/your-org/common/metrics v0.1.0
    github.com/your-org/common/models v0.1.0
    // Add other packages as needed
)
```

### 2. Package-Specific Migration

#### Configuration Management

Replace your current configuration handling with the common config package:

```go
// Before
type Config struct {
    // Your current config
}

// After
import "github.com/your-org/common/config"

func main() {
    cfg, err := config.Load("config.yaml")
    if err != nil {
        log.Fatal(err)
    }
}
```

#### Error Handling

Migrate to the common error handling:

```go
// Before
if err != nil {
    return fmt.Errorf("failed to process: %v", err)
}

// After
import "github.com/your-org/common/errors"

if err != nil {
    return errors.Wrap(err, "failed to process")
}
```

#### Logging

Replace your current logging with the common logging package:

```go
// Before
log.Printf("Processing request: %v", req)

// After
import "github.com/your-org/common/logging"

logger := logging.NewLogger()
logger.Info("Processing request", "request", req)
```

#### Metrics

Integrate the common metrics package:

```go
// Before
// Your current metrics implementation

// After
import "github.com/your-org/common/metrics"

func init() {
    metrics.RegisterCounter("requests_total", "Total number of requests")
    metrics.RegisterHistogram("request_duration", "Request duration in seconds")
}
```

#### Models

Use the common model interfaces and implementations:

```go
// Before
type User struct {
    // Your current user model
}

// After
import "github.com/your-org/common/models"

type User struct {
    models.BaseModel
    // Your user-specific fields
}
```

### 3. Testing

1. Run your test suite:

   ```bash
   go test ./...
   ```

2. Verify metrics collection:

   ```bash
   curl http://localhost:8080/metrics
   ```

3. Check logs for proper formatting

### 4. Deployment

1. Update your Dockerfile:

   ```dockerfile
   # Add common packages to your build
   COPY common /go/src/github.com/your-org/common
   ```

2. Update your deployment configuration:
   ```yaml
   # Add common package configurations
   config:
     path: /etc/your-service/config.yaml
   metrics:
     port: 8080
   ```

## Best Practices

### Configuration

1. Use environment variables for sensitive data
2. Keep configuration files in version control
3. Use different config files for different environments

### Error Handling

1. Always wrap errors with context
2. Use appropriate error types
3. Log errors with sufficient detail

### Logging

1. Use appropriate log levels
2. Include relevant context
3. Structure log messages consistently

### Metrics

1. Use descriptive metric names
2. Include appropriate labels
3. Document metric purposes

### Models

1. Implement all required interfaces
2. Use provided validation methods
3. Follow naming conventions

## Troubleshooting

### Common Issues

1. Import Path Issues

   - Solution: Verify module paths in go.mod
   - Check for case sensitivity in imports

2. Configuration Loading

   - Solution: Check file permissions
   - Verify config file format

3. Metrics Collection

   - Solution: Check Prometheus configuration
   - Verify metric registration

4. Logging Issues
   - Solution: Check log level configuration
   - Verify output format

## Support

For additional help:

1. Check package documentation
2. Review example implementations
3. Contact the development team

## Migration Checklist

- [ ] Update dependencies
- [ ] Migrate configuration
- [ ] Update error handling
- [ ] Integrate logging
- [ ] Add metrics
- [ ] Update models
- [ ] Run tests
- [ ] Verify deployment
- [ ] Update documentation

## Version Compatibility

| Common Package | Minimum Version | Notes                       |
| -------------- | --------------- | --------------------------- |
| config         | v0.1.0          | Basic configuration support |
| errors         | v0.1.0          | Error wrapping and types    |
| logging        | v0.1.0          | Structured logging          |
| metrics        | v0.1.0          | Prometheus integration      |
| models         | v0.1.0          | Base model interfaces       |

## Future Updates

1. Monitor package releases
2. Review changelog
3. Plan for breaking changes
4. Update dependencies regularly
