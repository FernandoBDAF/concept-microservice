# Simple Service Example

This example demonstrates how to use the common packages together in a simple microservice. It shows the integration of configuration, logging, metrics, security, and error handling packages.

## Features

- User creation with validation
- Password hashing
- JWT token generation
- Request logging
- Metrics collection
- Error handling
- Configuration management

## Prerequisites

- Go 1.16 or later
- PostgreSQL (for database operations)
- Prometheus (for metrics collection)

## Configuration

The service uses a YAML configuration file (`config.yaml`) that includes settings for:

- Server configuration
- Logging configuration
- Security settings
- Metrics configuration
- Database connection

## Package Integration

### Configuration Package

```go
// Initialize configuration
cfg, err := config.Load("config.yaml")
if err != nil {
    return nil, errors.Wrap(err, "failed to load configuration")
}
```

### Logging Package

```go
// Initialize logger
logger := logging.NewLogger(logging.Options{
    Level:  logging.InfoLevel,
    Format: logging.JSONFormat,
})

// Log messages
logger.Info("Processing request", "method", r.Method, "path", r.URL.Path)
logger.Error("Failed to process request", "error", err)
```

### Metrics Package

```go
// Initialize metrics collector
collector := metrics.NewCollector()

// Record metrics
timer := collector.Timer("request_duration", "Request duration in seconds")
defer timer.Observe()

counter := collector.Counter("http_requests_total", "Total number of HTTP requests")
counter.Inc()
```

### Security Package

```go
// Initialize security manager
manager := security.NewManager()

// Hash password
hashedPassword, err := manager.HashPassword(password)

// Generate JWT token
token, err := manager.GenerateToken(userID, roles)

// Validate email
isValid := manager.IsValidEmail(email)
```

### Error Handling

```go
// Wrap errors with context
if err != nil {
    return nil, errors.Wrap(err, "failed to load configuration")
}

// Create new errors
if name == "" {
    return errors.New("name is required")
}
```

## Running the Example

1. Start the service:

   ```bash
   go run main.go
   ```

2. Create a user:

   ```bash
   curl -X POST http://localhost:8080/users \
     -H "Content-Type: application/json" \
     -d '{
       "name": "John Doe",
       "email": "john@example.com",
       "password": "password123"
     }'
   ```

3. Check metrics:
   ```bash
   curl http://localhost:9090/metrics
   ```

## Best Practices Demonstrated

1. **Configuration Management**

   - Centralized configuration
   - Environment-specific settings
   - Secure credential handling

2. **Logging**

   - Structured logging
   - Contextual information
   - Appropriate log levels

3. **Metrics**

   - Request timing
   - Request counting
   - Prometheus integration

4. **Security**

   - Password hashing
   - JWT authentication
   - Input validation

5. **Error Handling**
   - Error wrapping
   - Contextual errors
   - Proper error responses

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests
5. Submit a pull request

## License

This example is licensed under the MIT License - see the [LICENSE](../../LICENSE) file for details.

## Planned Improvements

To further exemplify the integration and cross-interaction of the @common packages, the following improvements are planned for the `simple-service`:

1. **Enhanced Middleware Integration**

   - Implement additional middleware (e.g., rate limiting, request tracing) to demonstrate cross-package interactions.
   - Create adapters for each middleware to ensure seamless integration with the logging and metrics packages.

2. **Advanced Configuration Management**

   - Introduce environment-specific configuration files (e.g., `config.dev.yaml`, `config.prod.yaml`) to showcase dynamic configuration loading.
   - Implement configuration validation to ensure all required settings are present and correctly formatted.

3. **Comprehensive Error Handling**

   - Expand error handling to include custom error types and error codes.
   - Implement error recovery strategies to gracefully handle unexpected errors.

4. **Structured Logging Enhancements**

   - Integrate log aggregation to centralize logs from multiple services.
   - Add log rotation to manage log file sizes and retention.

5. **Metrics and Monitoring**

   - Implement detailed metrics for each endpoint (e.g., request count, latency, error rate).
   - Integrate with Prometheus for real-time monitoring and alerting.

6. **Security Enhancements**

   - Implement JWT-based authentication and authorization.
   - Add input validation and sanitization to prevent common security vulnerabilities.

7. **Database Integration**

   - Integrate a database (e.g., PostgreSQL) to store user data.
   - Implement database migrations and connection pooling.

8. **Testing and CI/CD**
   - Add unit and integration tests to ensure reliability.
   - Implement CI/CD pipelines for automated testing and deployment.

These improvements will provide a comprehensive example of how to effectively integrate and utilize the @common packages in a microservice architecture.
