INITIAL CONTEXT FOR LLM - never change the context-----------------------------
-> THIS SECTION IS A GUIDELINE TO THE LLM CONSIDER BEFORE WORKING IN THIS FILE, DO NOT CHANGE THIS

-> GOES OF THE README FILE:

- This file serves as the technical documentation of the service, providing a comprehensive overview of the codebase
- It should document:
  - Service architecture and design decisions
  - Component structure and relationships
  - API endpoints and interfaces
  - Dependencies and integration points
  - Configuration and deployment details
- This is the primary reference for understanding the technical implementation
- This file should be in sync with the `/TRACKER&MANAGER.md` where development progress and tasks are tracked
- While TRACKER&MANAGER.md focuses on "what" and "when", this file focuses on "how" and "why"

-> CONSIDERER BEFORE UPDATING THIS FILE:

- Never add fictional dates, version numbers, or metrics. Only include real, verified information. If information is not available, mark it as "To be determined" or remove the section.
- The changes in this file need to be incremental or to update informations that you confidentilly have knowlegde, they should not be guesses. If there are questions or uncertanty add comments asking for clarification instead.
- Check the `../../microservices/docs` folder for comprehensive project details, including architecture, development guidelines, and integration points. This will help in making informed decisions, haver better context and updates to the development plan. Always compare the implementation of this project with the plan described in the docs and whenever there are inconsistancies, add comments.
- Consider structuring this documentation separating the components and describing each one of them and adding sections to how they interact with each other - because this will be very dinamic and updated during the development process it will make clear what to update after each change
- This documentation is focusing in the current component that is part of a larger project, to have a more sistemic view check `../../microservices/TRACKER&MANAGER` and `../../microservices/README`
- Do not forget to be LLM focus, so because this will be used
- For LLM-specific guidelines and patterns, refer to [LLM Integration Guide](../../docs/llm/README.md)

---

# Profile Service

## Overview

The Profile Service is the primary entry point for client applications, handling user profile management operations. It integrates with various shared libraries and API services to provide a robust and scalable solution.

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
