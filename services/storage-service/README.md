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

# Profile Storage Service

## Overview

The Profile Storage Service is responsible for data persistence and database operations. It integrates with shared libraries to provide robust data storage capabilities with proper monitoring, logging, and error handling.

## Architecture

### Core Components

1. **Storage Layer**

   - Database operations
   - Data persistence
   - Transaction management
   - Connection pooling

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
   logger := logging.NewLogger("profile-storage")

   // Usage example
   logger.Info("Processing storage operation",
       logging.WithField("operation", "update"),
       logging.WithField("profile_id", profileID))
   ```

2. **Monitoring Library**

   ```go
   // Initialize collector
   monitor := monitoring.NewCollector("profile-storage")

   // Usage example
   monitor.IncStorageOperations("update")
   defer monitor.ObserveDuration("storage_update")
   ```

3. **Storage Library**

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
    ServiceName: "profile-storage",
    Discovery:   "kubernetes",
})

// Register health checks
integration.RegisterHealthCheck("database", func() error {
    return db.HealthCheck(ctx)
})

// Use circuit breaker
breaker := integration.NewCircuitBreaker("database", integration.CircuitBreakerConfig{
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

### 1. Transaction Management

```go
func (s *StorageService) UpdateProfile(ctx context.Context, profile *Profile) error {
    // Create transaction context
    txCtx := integration.NewTransactionContext(ctx)
    defer txCtx.Cleanup()

    // Begin transaction
    if err := txCtx.Begin(); err != nil {
        return err
    }

    // Update profile
    if err := s.db.UpdateProfile(txCtx, profile); err != nil {
        txCtx.Rollback()
        return err
    }

    // Update related data
    if err := s.db.UpdateProfileMetadata(txCtx, profile.ID, profile.Metadata); err != nil {
        txCtx.Rollback()
        return err
    }

    return txCtx.Commit()
}
```

### 2. Batch Processing

```go
func (s *StorageService) BatchUpdateProfiles(ctx context.Context, profiles []*Profile) error {
    // Setup batch processor
    processor := integration.NewBatchProcessor(integration.BatchProcessorConfig{
        BatchSize: 100,
        Workers:   5,
    })

    // Process profiles in batches
    return processor.Process(ctx, profiles, func(ctx context.Context, batch []*Profile) error {
        return s.db.BatchUpdateProfiles(ctx, batch)
    })
}
```

## Configuration

### Base Configuration

```yaml
service:
  name: profile-storage
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
database:
  host: postgres
  port: 5432
  name: profiles
  user: profile_storage
  password: ${DB_PASSWORD}
  max_connections: 20
  connection_timeout: 5s
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
    case errors.Is(err, db.ErrNotFound):
        logger.Warn("Profile not found", logging.WithError(err))
        monitor.IncNotFound()
    case errors.Is(err, db.ErrConnection):
        logger.Error("Database connection error", logging.WithError(err))
        monitor.IncConnectionErrors()
    case errors.Is(err, db.ErrTimeout):
        logger.Error("Database timeout", logging.WithError(err))
        monitor.IncTimeoutErrors()
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
integration.RegisterHealthCheck("database", func() error {
    return db.HealthCheck(ctx)
})
```

## Metrics

### Standard Metrics

```go
// Storage metrics
monitor.IncStorageOperations("update")
monitor.ObserveStorageLatency("update", latency)

// Database metrics
monitor.IncDatabaseOperations("update")
monitor.ObserveDatabaseLatency("update", latency)
monitor.ObserveConnectionPoolSize(poolSize)
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
   go build -o profile-storage ./cmd/profile-storage
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
   k6 run ./tests/load/profile-storage.js
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
   kubectl logs -n microservices -l app=profile-storage
   ```

### Docker

1. Build image:

   ```bash
   docker build -t profile-storage:latest .
   ```

2. Run container:
   ```bash
   docker run -p 8080:8080 profile-storage:latest
   ```

## Monitoring

### Prometheus Metrics

- Storage operation rates
- Error rates
- Latency percentiles
- Connection pool metrics
- Transaction metrics

### Grafana Dashboards

- Service overview
- Error rates
- Latency trends
- Database performance
- Connection pool status

## Logging

### Log Levels

- ERROR: Service errors
- WARN: Database warnings
- INFO: Request processing
- DEBUG: Detailed operations

### Log Fields

- service: profile-storage
- trace_id: Request tracing
- profile_id: Profile identifier
- operation: Operation type
- duration: Operation time
- error: Error details

## Security

### Authentication

- JWT token validation
- Service-to-service authentication
- API key management

### Authorization

- Role-based access control
- Permission management
- Resource access control

## Dependencies

### External Services

- PostgreSQL Database
- Storage API Service

### Shared Libraries

- Logging Library
- Monitoring Library
- Storage Client Library
- Service Integration Library

## API Documentation

### OpenAPI Specification

```yaml
openapi: 3.0.0
info:
  title: Profile Storage API
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

1. **Storage Layer**

   - [x] Database operations implementation
   - [x] Data persistence
   - [x] Transaction management
   - [x] Connection pooling
   - [x] Connection health checks
   - [x] Retry mechanisms with backoff

2. **Service Layer**

   - [x] Business logic implementation
   - [x] Data transformation
   - [x] Shared libraries integration
   - [x] Error handling
   - [x] Email uniqueness validation
   - [x] Request correlation tracking

3. **Integration Layer**
   - [x] Shared libraries integration
   - [x] API services communication
   - [x] Circuit breaking
   - [x] Retry mechanisms
   - [x] Request body validation
   - [x] Connection health monitoring

### Recent Improvements

1. **Request Handling**

   - Added content type validation
   - Implemented request body buffering
   - Added request size limits (1MB)
   - Improved error handling and logging
   - Added correlation IDs for request tracking
   - Enhanced request validation

2. **Connection Management**

   - Configured connection pool settings
   - Added connection health checks
   - Implemented retry logic with exponential backoff
   - Added connection backoff strategy
   - Improved error handling for connection issues
   - Added connection metrics

3. **Data Validation**
   - Implemented email uniqueness check
   - Added request body validation
   - Enhanced error handling for validation failures
   - Added detailed error logging
   - Improved error categorization

### Implementation Details

1. **Request Processing**

```go
// Request validation and processing
func (h *ProfileHandler) createProfile(w http.ResponseWriter, r *http.Request) {
    // Validate content type
    if r.Header.Get("Content-Type") != "application/json" {
        h.sendError(w, http.StatusBadRequest, "Content-Type must be application/json", nil)
        return
    }

    // Validate content length
    if r.ContentLength == 0 {
        h.sendError(w, http.StatusBadRequest, "Empty request body", nil)
        return
    }

    // Read and buffer request body
    body, err := io.ReadAll(r.Body)
    if err != nil {
        h.sendError(w, http.StatusBadRequest, "Failed to read request body", err)
        return
    }

    // Validate body size
    const maxBodySize = 1 << 20 // 1MB
    if len(body) > maxBodySize {
        h.sendError(w, http.StatusRequestEntityTooLarge, "Request body too large", nil)
        return
    }
}
```

2. **Connection Management**

```go
// Connection pool configuration
func NewProfileRepository(db *sqlx.DB) *ProfileRepository {
    // Configure connection pool
    db.SetMaxOpenConns(100)
    db.SetMaxIdleConns(20)
    db.SetConnMaxLifetime(5 * time.Minute)
    db.SetConnMaxIdleTime(1 * time.Minute)
    return &ProfileRepository{db: db}
}

// Connection health check
func (r *ProfileRepository) checkConnectionHealth(ctx context.Context) error {
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    var result int
    err := r.db.QueryRowContext(ctx, "SELECT 1").Scan(&result)
    if err != nil {
        return fmt.Errorf("database health check failed: %w", err)
    }
    return nil
}
```

3. **Email Uniqueness Validation**

```go
// Email uniqueness check
func (s *ProfileService) CreateProfile(ctx context.Context, req *models.ProfileRequest) (*models.Profile, error) {
    // Check if email is already in use
    existingProfile, err := s.repo.GetByEmail(ctx, req.Email)
    if err != nil {
        if !errors.Is(err, repository.ErrNotFound) {
            return nil, fmt.Errorf("%w: %v", ErrDatabaseOperation, err)
        }
    }

    if existingProfile != nil {
        return nil, ErrDuplicateEmail
    }
}
```

### Error Handling

1. **Request Errors**

   - Invalid content type
   - Empty request body
   - Request body too large
   - Invalid JSON format
   - Missing required fields

2. **Connection Errors**

   - Connection timeout
   - Connection reset
   - Connection refused
   - Broken pipe
   - I/O timeout

3. **Validation Errors**
   - Duplicate email
   - Invalid email format
   - Missing required fields
   - Invalid field values

### Monitoring and Metrics

1. **Request Metrics**

   - Request duration
   - Request size
   - Error rates by type
   - Success rates

2. **Connection Metrics**

   - Connection pool size
   - Connection health status
   - Connection errors
   - Connection latency

3. **Validation Metrics**
   - Validation errors by type
   - Duplicate email attempts
   - Invalid request rates

## API Endpoints

### 1. Profile Storage

```http
GET /api/v1/storage/profiles
GET /api/v1/storage/profiles/{id}
POST /api/v1/storage/profiles
PUT /api/v1/storage/profiles/{id}
DELETE /api/v1/storage/profiles/{id}
```

### 2. Batch Operations

```http
POST /api/v1/storage/profiles/batch
PUT /api/v1/storage/profiles/batch
DELETE /api/v1/storage/profiles/batch
```

### 3. Query Operations

```http
POST /api/v1/storage/profiles/query
GET /api/v1/storage/profiles/search
```

### 4. Health and Metrics

```http
GET /health
GET /ready
GET /metrics
```

## Error Types

### 1. Database Errors

- Connection errors
- Query errors
- Transaction errors
- Constraint errors
- Timeout errors

### 2. Storage Errors

- File system errors
- Permission errors
- Space errors
- IO errors
- Lock errors

### Recovery Strategies

### 1. Database Recovery

- Connection retry
- Query retry
- Transaction rollback
- Connection pool recovery
- Error logging

### 2. Storage Recovery

- File system recovery
- Permission recovery
- Space management
- IO retry
- Lock recovery

## Cross-References

- [Storage Service Patterns](../../reference-materials/development/patterns/storage-service-patterns.md)
- [Service Integration Patterns](../../reference-materials/development/patterns/service-integration-patterns.md)
- [Monitoring Patterns](../../reference-materials/development/patterns/monitoring-patterns.md)
- [Security Patterns](../../reference-materials/development/patterns/security-patterns.md)
- [Error Handling Patterns](../../reference-materials/development/patterns/error-handling-patterns.md)
