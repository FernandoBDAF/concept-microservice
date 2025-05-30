# Profile Service

## Overview

The Profile Service is a microservice responsible for managing user profiles in the system. It provides a REST API for profile management operations and integrates with various other services to provide a complete profile management solution.

### Purpose

- Manage user profiles (create, read, update, delete)
- Handle profile-related tasks through a queue-based system
- Provide profile data to other services in the system
- Ensure data consistency and reliability

### Main Functionalities

1. Profile Management

   - Create new user profiles
   - Retrieve profile information
   - Update profile details
   - Delete profiles
   - List all profiles

2. Task Processing
   - Submit profile-related tasks to a queue
   - Track task status and progress
   - Handle task responses

### Service Integration

The Profile Service interacts with several other services:

- Queue Service: For asynchronous task processing
- Storage Service: For persistent profile data storage
- Auth Service: For user authentication and authorization
- Cache Service: For performance optimization

### Setup and Running

1. Prerequisites

   ```bash
   # Required environment variables
   export SERVER_HOST=0.0.0.0
   export SERVER_PORT=8080
   export QUEUE_URL=amqp://guest:guest@localhost:5672/
   export QUEUE_NAME=profile_queue
   export STORAGE_HOST=profile-storage
   export STORAGE_PORT=8080
   export AUTH_SERVICE_URL=http://auth-service
   ```

2. Installation

   ```bash
   # Clone the repository
   git clone https://github.com/fernandobarroso/microservices.git
   cd microservices/services/profile-service

   # Install dependencies
   go mod download
   ```

3. Running the Service

   ```bash
   # Development mode
   go run cmd/main.go

   # Production build
   go build -o profile-service ./cmd/main.go
   ./profile-service
   ```

4. Docker

   ```bash
   # Build the image
   docker build -t profile-service:latest .

   # Run the container
   docker run -p 8080:8080 profile-service:latest
   ```

### API Endpoints

1. Profile Management

   ```http
   GET    /api/v1/profiles          # List all profiles
   GET    /api/v1/profiles/{id}     # Get profile by ID
   POST   /api/v1/profiles          # Create new profile
   PUT    /api/v1/profiles/{id}     # Update profile
   DELETE /api/v1/profiles/{id}     # Delete profile
   ```

2. Task Management

   ```http
   POST   /api/v1/profiles/{id}/tasks  # Submit a new task
   ```

3. Health and Metrics
   ```http
   GET    /health                     # Health check
   GET    /metrics                    # Prometheus metrics
   ```

### Configuration

The service can be configured through environment variables or a configuration file. Key configuration options include:

```yaml
server:
  host: 0.0.0.0
  port: 8080

queue:
  url: amqp://guest:guest@localhost:5672/
  timeout: 5s
  retries: 3
  queue_name: profile_queue

storage:
  host: profile-storage
  port: 8080
  database: profile_service
  type: memory
  max_retries: 3
  retry_delay: 100ms

auth:
  host: localhost
  port: 80
  url: http://auth-service

logging:
  level: info
  format: text
  log_file: app.log
```

### Monitoring and Logging

The service includes built-in monitoring and logging capabilities:

1. Metrics (Prometheus)

   - Request rates
   - Error rates
   - Latency percentiles
   - Queue operation metrics
   - Storage operation metrics

2. Logging
   - Structured logging with Zap
   - Multiple log levels (DEBUG, INFO, WARN, ERROR)
   - Request tracing
   - Error tracking

### Development

1. Running Tests

   ```bash
   # Run all tests
   go test ./...

   # Run specific test
   go test ./internal/domain/services/...
   ```

2. Code Style
   ```bash
   # Run linter
   golangci-lint run
   ```

### Deployment

The service can be deployed using Docker or Kubernetes:

1. Docker

   ```bash
   docker build -t profile-service:latest .
   docker run -p 8080:8080 profile-service:latest
   ```

2. Kubernetes
   ```bash
   kubectl apply -f k8s/
   ```

### Security

The service implements several security measures:

- JWT-based authentication
- Role-based access control
- Input validation
- Rate limiting
- Secure configuration management

## Architecture

### Core Components

1. **API Layer**

   - REST API endpoints
   - Request validation
   - Response formatting
   - Error handling

2. **Service Layer**

   - Business logic
   - Data transformation
   - Integration with shared libraries
   - Error handling

3. **Integration Layer**
   - Shared libraries integration
   - API services communication
   - Circuit breaking
   - Retry mechanisms

### Shared Libraries Integration

1. **Logging Library**

   ```go
   // Initialize logger
   logger := logging.NewLogger("profile-service")

   // Usage example
   logger.Info("Processing request",
       logging.WithField("profile_id", profileID),
       logging.WithField("action", "update"))
   ```

2. **Monitoring Library**

   ```go
   // Initialize collector
   monitor := monitoring.NewCollector("profile-service")

   // Usage example
   monitor.IncRequests("profile_update")
   defer monitor.ObserveDuration("profile_update")
   ```

3. **Cache Client Library**

   ```go
   // Initialize cache client
   cacheClient := cache.NewAPIClient(cache.Config{
       Endpoint: "http://cache-api:8080",
       Timeout:  time.Second * 5,
   })

   // Usage example
   profile, err := cacheClient.Get(ctx, "profile:"+profileID)
   ```

4. **Queue Client Library**

   ```go
   // Initialize queue client
   queueClient := queue.NewAPIClient(queue.Config{
       Endpoint: "http://queue-api:8080",
       Timeout:  time.Second * 5,
   })

   // Usage example
   err = queueClient.Publish(ctx, "profile-updates", &queue.Message{
       Type: "profile_updated",
       Data: profileData,
   })
   ```

5. **Storage Client Library**

   ```go
   // Initialize storage client
   storageClient := storage.NewAPIClient(storage.Config{
       Endpoint: "http://storage-api:8080",
       Timeout:  time.Second * 5,
   })

   // Usage example
   err = storageClient.Update(ctx, "profiles", profileID, profileData)
   ```

### Service Integration Library

```go
// Initialize service integration
integration := integration.NewServiceIntegration(integration.Config{
    ServiceName: "profile-service",
    Discovery:   "kubernetes",
})

// Register health checks
integration.RegisterHealthCheck("cache", func() error {
    return cacheClient.HealthCheck(ctx)
})
integration.RegisterHealthCheck("queue", func() error {
    return queueClient.HealthCheck(ctx)
})
integration.RegisterHealthCheck("storage", func() error {
    return storageClient.HealthCheck(ctx)
})

// Use circuit breaker
breaker := integration.NewCircuitBreaker("cache-api", integration.CircuitBreakerConfig{
    Threshold: 5,
    Timeout:   time.Second * 30,
})

// Use retry mechanism
retry := integration.NewRetry(integration.RetryConfig{
    MaxAttempts: 3,
    Backoff:     time.Second * 2,
})
```

## Complex Integration Patterns

### 1. Distributed Transaction

```go
func (s *ProfileService) UpdateProfile(ctx context.Context, profile *Profile) error {
    // Create transaction context
    txCtx := integration.NewTransactionContext(ctx)
    defer txCtx.Cleanup()

    // Begin transaction
    if err := txCtx.Begin(); err != nil {
        return err
    }

    // Update storage
    if err := storageClient.Update(txCtx, "profiles", profile.ID, profile); err != nil {
        txCtx.Rollback()
        return err
    }

    // Invalidate cache
    if err := cacheClient.Delete(txCtx, "profile:"+profile.ID); err != nil {
        txCtx.Rollback()
        return err
    }

    // Publish event
    if err := queueClient.Publish(txCtx, "profile-updates", &queue.Message{
        Type: "profile_updated",
        Data: profile,
    }); err != nil {
        txCtx.Rollback()
        return err
    }

    return txCtx.Commit()
}
```

### 2. Circuit Breaker with Fallback

```go
func (s *ProfileService) GetProfile(ctx context.Context, profileID string) (*Profile, error) {
    // Setup circuit breaker with fallback
    breaker := integration.NewCircuitBreaker("cache-api", integration.CircuitBreakerConfig{
        Threshold: 5,
        Timeout:   time.Second * 30,
        Fallback: func(ctx context.Context) (interface{}, error) {
            return storageClient.Get(ctx, "profiles", profileID)
        },
    })

    // Execute with retry
    var profile *Profile
    err := breaker.Execute(ctx, func(ctx context.Context) error {
        var err error
        profile, err = cacheClient.Get(ctx, "profile:"+profileID)
        return err
    })

    return profile, err
}
```

## Configuration

### Base Configuration

```yaml
service:
  name: profile-service
  version: 1.0.0
  port: 8080

logging:
  level: info
  format: json
  output: stdout

monitoring:
  enabled: true
  prometheus:
    path: /metrics
    port: 9090

integration:
  service_discovery: kubernetes
  circuit_breaker:
    threshold: 5
    timeout: 30s
  retry:
    max_attempts: 3
    backoff: 2s
```

### Service-Specific Configuration

```yaml
cache:
  endpoint: http://cache-api:8080
  timeout: 5s
  circuit_breaker:
    threshold: 5
    timeout: 30s

queue:
  endpoint: http://queue-api:8080
  timeout: 5s
  circuit_breaker:
    threshold: 5
    timeout: 30s

storage:
  endpoint: http://storage-api:8080
  timeout: 5s
  circuit_breaker:
    threshold: 5
    timeout: 30s
```

## Error Handling

### Standard Error Patterns

```go
// Error handling with logging and monitoring
if err != nil {
    switch {
    case errors.Is(err, cache.ErrNotFound):
        logger.Warn("Cache miss", logging.WithError(err))
        monitor.IncCacheMisses()
    case errors.Is(err, queue.ErrQueueFull):
        logger.Error("Queue full", logging.WithError(err))
        monitor.IncQueueErrors()
    case errors.Is(err, storage.ErrConnection):
        logger.Error("Storage connection error", logging.WithError(err))
        monitor.IncStorageErrors()
    default:
        logger.Error("Unexpected error", logging.WithError(err))
        monitor.IncErrors()
    }
    return nil, err
}
```

## Health Checks

### Service Health

```go
// Register health checks
integration.RegisterHealthCheck("cache", func() error {
    return cacheClient.HealthCheck(ctx)
})
integration.RegisterHealthCheck("queue", func() error {
    return queueClient.HealthCheck(ctx)
})
integration.RegisterHealthCheck("storage", func() error {
    return storageClient.HealthCheck(ctx)
})
```

## Metrics

### Standard Metrics

```go
// Request metrics
monitor.IncRequests("profile_update")
defer monitor.ObserveDuration("profile_update")

// Cache metrics
monitor.IncCacheHits()
monitor.IncCacheMisses()

// Queue metrics
monitor.IncQueueMessages("profile-updates")
monitor.ObserveQueueLatency("profile-updates", latency)

// Storage metrics
monitor.IncStorageOperations("update")
monitor.ObserveStorageLatency("update", latency)
```

## Development

### Setup

1. Install dependencies:

   ```bash
   go mod download
   ```

2. Run tests:

   ```bash
   go test ./...
   ```

3. Build service:
   ```bash
   go build -o profile-service ./cmd/profile-service
   ```

### Testing

1. Unit tests:

   ```bash
   go test -v ./internal/...
   ```

2. Integration tests:

   ```bash
   go test -v ./tests/integration/...
   ```

3. Load tests:
   ```bash
   k6 run ./tests/load/profile-service.js
   ```

## Deployment

### Kubernetes

1. Apply configurations:

   ```bash
   kubectl apply -f k8s/
   ```

2. Verify deployment:

   ```bash
   kubectl get pods -n microservices
   ```

3. Check logs:
   ```bash
   kubectl logs -n microservices -l app=profile-service
   ```

### Docker

1. Build image:

   ```bash
   docker build -t profile-service:latest .
   ```

2. Run container:
   ```bash
   docker run -p 8080:8080 profile-service:latest
   ```

## Monitoring

### Prometheus Metrics

- Request rates
- Error rates
- Latency percentiles
- Cache hit/miss rates
- Queue depths
- Storage operation rates

### Grafana Dashboards

- Service overview
- Error rates
- Latency trends
- Cache performance
- Queue metrics
- Storage metrics

## Logging

### Log Levels

- ERROR: Service errors
- WARN: Cache misses, retries
- INFO: Request processing
- DEBUG: Detailed operations

### Log Fields

- service: profile-service
- trace_id: Request tracing
- profile_id: Profile identifier
- action: Operation type
- duration: Operation time
- error: Error details

## Security

### Authentication

- JWT token validation
- OAuth 2.0 integration
- Session management

### Authorization

- Role-based access control
- Permission management
- Resource access control

## Dependencies

### External Services

- Cache API Service
- Queue API Service
- Storage API Service
- Auth Service

### Shared Libraries

- Logging Library
- Monitoring Library
- Cache Client Library
- Queue Client Library
- Storage Client Library
- Service Integration Library

## API Documentation

### OpenAPI Specification

```yaml
openapi: 3.0.0
info:
  title: Profile API
  version: 1.0.0
paths:
  /profiles:
    get:
      summary: List profiles
      responses:
        "200":
          description: Success
    post:
      summary: Create profile
      responses:
        "201":
          description: Created
  /profiles/{id}:
    get:
      summary: Get profile
      responses:
        "200":
          description: Success
    put:
      summary: Update profile
      responses:
        "200":
          description: Success
    delete:
      summary: Delete profile
      responses:
        "204":
          description: No Content
```

## Contributing

1. Fork repository
2. Create feature branch
3. Commit changes
4. Push to branch
5. Create pull request

## License

MIT License

## Implementation Status

### Current State

1. **API Layer**

   - [ ] REST API endpoints implementation
   - [ ] Request validation
   - [ ] Response formatting
   - [ ] Error handling

2. **Service Layer**

   - [ ] Business logic implementation
   - [ ] Data transformation
   - [ ] Shared libraries integration
   - [ ] Error handling

3. **Integration Layer**
   - [ ] Shared libraries integration
   - [ ] API services communication
   - [ ] Circuit breaking
   - [ ] Retry mechanisms

### Implementation Plan

1. **Phase 1: Core Infrastructure**

   - [ ] Project structure setup
   - [ ] Configuration management
   - [ ] Logging integration
   - [ ] Metrics collection

2. **Phase 2: API Implementation**

   - [ ] HTTP server setup
   - [ ] Endpoint implementation
   - [ ] Request validation
   - [ ] Response formatting

3. **Phase 3: Service Integration**
   - [ ] Shared libraries integration
   - [ ] API services communication
   - [ ] Circuit breaking
   - [ ] Retry mechanisms

## API Endpoints

### 1. Profile Management

```http
GET /api/v1/profiles
GET /api/v1/profiles/{id}
POST /api/v1/profiles
PUT /api/v1/profiles/{id}
DELETE /api/v1/profiles/{id}
```

### 2. Profile Search

```http
GET /api/v1/profiles/search
POST /api/v1/profiles/query
```

### 3. Profile Batch Operations

```http
POST /api/v1/profiles/batch
PUT /api/v1/profiles/batch
DELETE /api/v1/profiles/batch
```

### 4. Health and Metrics

```http
GET /health
GET /ready
GET /metrics
```

## Error Types

### 1. API Errors

- Validation errors
- Authentication errors
- Authorization errors
- Rate limit errors
- System errors
- Timeout errors

### 2. Integration Errors

- Cache errors
- Queue errors
- Storage errors
- Service discovery errors
- Circuit breaker errors

### Recovery Strategies

### 1. API Recovery

- Request retry
- Error logging
- Status updates
- Alert generation
- Circuit breaking

### 2. Integration Recovery

- Service retry
- Fallback mechanisms
- Error logging
- Status updates
- Alert generation

## Cross-References

- [API Service Patterns](../../reference-materials/development/patterns/api-service-patterns.md)
- [Service Integration Patterns](../../reference-materials/development/patterns/service-integration-patterns.md)
- [Monitoring Patterns](../../reference-materials/development/patterns/monitoring-patterns.md)
- [Security Patterns](../../reference-materials/development/patterns/security-patterns.md)
- [Error Handling Patterns](../../reference-materials/development/patterns/error-handling-patterns.md)
