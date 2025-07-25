# Storage Service Technical Context

## Strategic Technical Position

The Storage Service serves as the **data persistence backbone** of the microservices task processing ecosystem, providing both **synchronous direct access** and **asynchronous queue-based operations**. This dual-mode architecture enables the service to support traditional request-response patterns while participating in the modern event-driven task processing workflows.

## Enhanced Architecture Overview

### Core Technical Transformation

**From**: Standalone data persistence service  
**To**: Integrated queue-aware storage component supporting:

- Synchronous HTTP/gRPC operations for direct access
- Asynchronous queue-based operations for task processing
- Batch operations for efficiency
- Event-driven completion notifications

### Integration Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│  Profile Service │────│   Queue Service  │────│    RabbitMQ     │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                                                         │
┌─────────────────┐    ┌──────────────────┐             │
│  Worker Service │────│   Storage Tasks  │◄────────────┘
└─────────────────┘    └──────────────────┘
                                │
                                ▼
                    ┌─────────────────────────┐
                    │    Storage Service      │
                    │  (Enhanced with Queue)  │
                    └─────────────────────────┘
                                │
                                ▼
                    ┌─────────────────────────┐
                    │    PostgreSQL DB        │
                    └─────────────────────────┘
```

## Directory Structure

```
services/storage-service/
├── cmd/
│   └── server/
│       └── main.go                 # Enhanced with queue consumer setup
├── internal/
│   ├── messaging/                  # NEW - Queue integration layer
│   │   ├── message.go             # Standardized message format
│   │   ├── processor.go           # Message processing logic
│   │   ├── consumer.go            # RabbitMQ consumer implementation
│   │   └── handlers.go            # Storage task handlers
│   ├── domain/
│   │   ├── models/
│   │   │   ├── profile.go         # Enhanced with async support
│   │   │   └── batch.go           # NEW - Batch operation models
│   │   └── service/
│   │       ├── profile.go         # Enhanced with async operations
│   │       ├── async_operations.go # NEW - Async operation logic
│   │       └── batch_operations.go # NEW - Batch processing logic
│   ├── infrastructure/
│   │   ├── database/              # Enhanced connection management
│   │   └── repository/            # Enhanced with batch operations
│   ├── api/
│   │   ├── rest/                  # Enhanced with batch endpoints
│   │   └── grpc/                  # Enhanced with batch operations
│   ├── config/
│   │   └── config.go              # Enhanced with queue configuration
│   └── pkg/
│       └── logger/                # Enhanced with async operation logging
└── deployments/
    └── kubernetes/                # Enhanced with queue configuration
```

## Core Technical Components

### 1. Enhanced Message Processing Layer

#### Standardized Message Format

```go
// internal/messaging/message.go
type Message struct {
    ID         string            `json:"id"`
    Type       string            `json:"type"`
    Payload    json.RawMessage   `json:"payload"`
    Timestamp  time.Time         `json:"timestamp"`
    Metadata   map[string]string `json:"metadata"`
    RoutingKey string            `json:"routing_key"`
}

type StorageTask struct {
    Operation   string            `json:"operation"`    // create, update, delete
    ProfileID   string            `json:"profile_id,omitempty"`
    ProfileData json.RawMessage   `json:"profile_data,omitempty"`
    Options     map[string]interface{} `json:"options,omitempty"`
}
```

#### Message Processor Implementation

```go
// internal/messaging/processor.go
type MessageProcessor struct {
    storageService *service.ProfileService
    batchService   *service.BatchService
    logger         *zap.Logger
    metrics        MetricsCollector
}

func (p *MessageProcessor) ProcessStorageMessage(ctx context.Context, msg *Message) error {
    // Message validation and routing
    switch msg.Type {
    case "storage.profile.create":
        return p.handleCreateOperation(ctx, msg)
    case "storage.profile.update":
        return p.handleUpdateOperation(ctx, msg)
    case "storage.profile.delete":
        return p.handleDeleteOperation(ctx, msg)
    case "storage.batch.mixed":
        return p.handleBatchOperation(ctx, msg)
    default:
        return fmt.Errorf("unsupported message type: %s", msg.Type)
    }
}
```

### 2. Queue Consumer Integration

#### RabbitMQ Consumer Setup

```go
// internal/messaging/consumer.go
type StorageConsumer struct {
    consumer      *commonQueue.Consumer
    processor     *MessageProcessor
    config        *ConsumerConfig
    logger        *zap.Logger
    metrics       MetricsCollector
    shutdown      chan os.Signal
    wg            sync.WaitGroup
}

func (c *StorageConsumer) Start() error {
    messages, err := c.consumer.Consume()
    if err != nil {
        return fmt.Errorf("failed to start consuming: %w", err)
    }

    c.wg.Add(1)
    go func() {
        defer c.wg.Done()
        for msg := range messages {
            c.processMessage(msg)
        }
    }()

    return nil
}
```

### 3. Enhanced Service Layer

#### Async Operations Service

```go
// internal/domain/service/async_operations.go
type AsyncOperationsService struct {
    repo               *repository.ProfileRepository
    transactionTimeout time.Duration
    logger             *zap.Logger
    metrics            MetricsCollector
}

func (s *AsyncOperationsService) ProcessAsyncCreate(ctx context.Context, task *StorageTask) error {
    // Create context with timeout
    ctx, cancel := context.WithTimeout(ctx, s.transactionTimeout)
    defer cancel()

    // Parse profile data
    var profileReq models.ProfileRequest
    if err := json.Unmarshal(task.ProfileData, &profileReq); err != nil {
        return fmt.Errorf("failed to unmarshal profile data: %w", err)
    }

    // Process creation with full validation
    profile, err := s.createProfileWithValidation(ctx, &profileReq)
    if err != nil {
        return fmt.Errorf("async create failed: %w", err)
    }

    s.logger.Info("Async profile creation completed",
        zap.String("profile_id", profile.ID.String()),
        zap.String("operation", "async_create"))

    return nil
}
```

#### Batch Operations Service

```go
// internal/domain/service/batch_operations.go
type BatchOperationsService struct {
    repo        *repository.ProfileRepository
    asyncSvc    *AsyncOperationsService
    logger      *zap.Logger
    metrics     MetricsCollector
    maxBatchSize int
}

func (s *BatchOperationsService) ProcessBatch(ctx context.Context, batch *BatchStorageTask) (*BatchResult, error) {
    if len(batch.Operations) > s.maxBatchSize {
        return nil, fmt.Errorf("batch size %d exceeds maximum %d", len(batch.Operations), s.maxBatchSize)
    }

    // Begin transaction for atomic batch processing
    tx, err := s.repo.BeginTx(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to begin batch transaction: %w", err)
    }
    defer tx.Rollback()

    result := &BatchResult{
        BatchID:       batch.BatchID,
        TotalOps:      len(batch.Operations),
        SuccessfulOps: 0,
        FailedOps:     0,
        Results:       make([]OperationResult, 0, len(batch.Operations)),
    }

    // Process each operation
    for i, op := range batch.Operations {
        opResult := s.processSingleOperation(ctx, tx, &op, i)
        result.Results = append(result.Results, opResult)

        if opResult.Status == "success" {
            result.SuccessfulOps++
        } else {
            result.FailedOps++
        }
    }

    // Commit transaction if all operations succeeded or if partial success is allowed
    if result.FailedOps == 0 || batch.Options.AllowPartialSuccess {
        if err := tx.Commit(); err != nil {
            return nil, fmt.Errorf("failed to commit batch transaction: %w", err)
        }
    }

    return result, nil
}
```

## Design Patterns Implementation

### 1. Clean Architecture Pattern

**Layers**:

- **Domain Layer**: Business logic and entities (models, services)
- **Application Layer**: Use cases and orchestration (messaging, handlers)
- **Infrastructure Layer**: External concerns (database, queue, API)
- **Interface Layer**: External interfaces (REST, gRPC, Queue Consumer)

### 2. Repository Pattern Enhancement

```go
// Enhanced repository with batch operations
type ProfileRepository struct {
    db      *sqlx.DB
    logger  *zap.Logger
    metrics MetricsCollector
}

func (r *ProfileRepository) BatchCreate(ctx context.Context, profiles []*models.Profile) error {
    query := `INSERT INTO profiles (id, first_name, last_name, email, phone, created_at, updated_at)
              VALUES (:id, :first_name, :last_name, :email, :phone, :created_at, :updated_at)`

    _, err := r.db.NamedExecContext(ctx, query, profiles)
    if err != nil {
        r.metrics.IncrementBatchErrors("create")
        return fmt.Errorf("batch create failed: %w", err)
    }

    r.metrics.IncrementBatchOperations("create", len(profiles))
    return nil
}
```

### 3. Factory Pattern for Message Handlers

```go
// internal/messaging/handlers.go
type HandlerFactory struct {
    storageService *service.ProfileService
    batchService   *service.BatchOperationsService
    logger         *zap.Logger
}

func (f *HandlerFactory) CreateHandler(messageType string) (MessageHandler, error) {
    switch messageType {
    case "storage.profile.create":
        return &CreateHandler{service: f.storageService, logger: f.logger}, nil
    case "storage.profile.update":
        return &UpdateHandler{service: f.storageService, logger: f.logger}, nil
    case "storage.profile.delete":
        return &DeleteHandler{service: f.storageService, logger: f.logger}, nil
    case "storage.batch.mixed":
        return &BatchHandler{service: f.batchService, logger: f.logger}, nil
    default:
        return nil, fmt.Errorf("unsupported message type: %s", messageType)
    }
}
```

### 4. Circuit Breaker Pattern

```go
// Database operations with circuit breaker
type CircuitBreaker struct {
    failures    int
    threshold   int
    timeout     time.Duration
    lastFailure time.Time
    state       CircuitState
    mutex       sync.RWMutex
}

func (cb *CircuitBreaker) Execute(operation func() error) error {
    if cb.shouldReject() {
        return ErrCircuitBreakerOpen
    }

    err := operation()
    cb.recordResult(err)
    return err
}
```

## Technical Configuration Management

### Enhanced Configuration Structure

```go
// internal/config/config.go
type Config struct {
    // Existing HTTP/gRPC configuration
    HTTPPort string `env:"HTTP_PORT" default:"8080"`
    GRPCPort string `env:"GRPC_PORT" default:"50051"`

    // Database configuration
    DatabaseURL        string `env:"DATABASE_URL" required:"true"`
    DatabaseMaxConns   int    `env:"DATABASE_MAX_CONNECTIONS" default:"100"`
    DatabaseIdleConns  int    `env:"DATABASE_IDLE_CONNECTIONS" default:"20"`

    // NEW - Queue configuration
    RabbitMQURL       string        `env:"RABBITMQ_URL" required:"true"`
    QueueName         string        `env:"QUEUE_NAME" default:"storage-processing"`
    ExchangeName      string        `env:"EXCHANGE_NAME" default:"tasks-exchange"`
    RoutingKey        string        `env:"ROUTING_KEY" default:"storage.*"`
    PrefetchCount     int           `env:"PREFETCH_COUNT" default:"5"`
    ProcessingTimeout time.Duration `env:"PROCESSING_TIMEOUT" default:"30s"`
    MaxRetries        int           `env:"MAX_RETRIES" default:"3"`

    // NEW - Batch configuration
    MaxBatchSize      int           `env:"MAX_BATCH_SIZE" default:"100"`
    BatchTimeout      time.Duration `env:"BATCH_TIMEOUT" default:"60s"`

    // Logging configuration
    LogLevel       string `env:"LOG_LEVEL" default:"info"`
    LogEnvironment string `env:"LOG_ENVIRONMENT" default:"production"`
    ServiceName    string `env:"SERVICE_NAME" default:"storage-service"`
}
```

### Configuration Validation

```go
func (c *Config) Validate() error {
    if c.DatabaseURL == "" {
        return errors.New("DATABASE_URL is required")
    }
    if c.RabbitMQURL == "" {
        return errors.New("RABBITMQ_URL is required")
    }
    if c.MaxBatchSize <= 0 || c.MaxBatchSize > 1000 {
        return errors.New("MAX_BATCH_SIZE must be between 1 and 1000")
    }
    if c.PrefetchCount <= 0 {
        return errors.New("PREFETCH_COUNT must be greater than 0")
    }
    return nil
}
```

## Error Handling Strategy

### Hierarchical Error Classification

```go
// internal/domain/errors/storage_errors.go
type StorageError struct {
    Code       string            `json:"code"`
    Message    string            `json:"message"`
    Details    map[string]string `json:"details,omitempty"`
    Cause      error             `json:"-"`
    Retryable  bool              `json:"retryable"`
    Timestamp  time.Time         `json:"timestamp"`
}

// Error categories
var (
    ErrValidation     = &StorageError{Code: "VALIDATION_ERROR", Retryable: false}
    ErrNotFound       = &StorageError{Code: "NOT_FOUND", Retryable: false}
    ErrDuplicateEmail = &StorageError{Code: "DUPLICATE_EMAIL", Retryable: false}
    ErrDatabase       = &StorageError{Code: "DATABASE_ERROR", Retryable: true}
    ErrTimeout        = &StorageError{Code: "TIMEOUT_ERROR", Retryable: true}
    ErrBatchPartial   = &StorageError{Code: "BATCH_PARTIAL_FAILURE", Retryable: false}
)
```

### Queue Message Error Handling

```go
func (p *MessageProcessor) handleProcessingError(delivery amqp.Delivery, err error) {
    var storageErr *StorageError
    if errors.As(err, &storageErr) {
        if storageErr.Retryable && p.shouldRetry(delivery) {
            // Increment retry count and requeue
            p.requeueWithBackoff(delivery)
            return
        }
    }

    // Send to DLQ for non-retryable errors or max retries exceeded
    p.sendToDLQ(delivery, err)
}
```

## Logging Strategy

### Structured Logging for Async Operations

```go
// Enhanced logging for async operations
func (s *AsyncOperationsService) logAsyncOperation(operation string, profileID string, duration time.Duration, err error) {
    fields := []zap.Field{
        zap.String("operation", operation),
        zap.String("profile_id", profileID),
        zap.Duration("duration", duration),
        zap.String("operation_type", "async"),
    }

    if err != nil {
        fields = append(fields, zap.Error(err))
        s.logger.Error("Async operation failed", fields...)
    } else {
        s.logger.Info("Async operation completed", fields...)
    }
}
```

### Correlation ID Tracking

```go
// Track operations across sync and async boundaries
func (p *MessageProcessor) extractCorrelationID(msg *Message) string {
    if correlationID, exists := msg.Metadata["correlation_id"]; exists {
        return correlationID
    }
    return uuid.New().String()
}

func (p *MessageProcessor) createContextWithCorrelation(ctx context.Context, correlationID string) context.Context {
    return context.WithValue(ctx, "correlation_id", correlationID)
}
```

## Security Implementation

### Message Validation and Sanitization

```go
func (p *MessageProcessor) validateMessage(msg *Message) error {
    // Basic message structure validation
    if msg.ID == "" || msg.Type == "" || len(msg.Payload) == 0 {
        return ErrInvalidMessage
    }

    // Message size validation
    if len(msg.Payload) > MaxMessageSize {
        return ErrMessageTooLarge
    }

    // Message type validation
    if !p.isValidMessageType(msg.Type) {
        return ErrUnsupportedMessageType
    }

    return nil
}

func (p *MessageProcessor) sanitizePayload(payload json.RawMessage) (json.RawMessage, error) {
    // Remove potentially dangerous fields
    // Validate data types and ranges
    // Apply business rule validation
    return payload, nil
}
```

### Database Security

```go
// Prepared statements for all database operations
const createProfileQuery = `
    INSERT INTO profiles (id, first_name, last_name, email, phone, created_at, updated_at)
    VALUES ($1, $2, $3, $4, $5, $6, $7)
    RETURNING id, created_at, updated_at
`

func (r *ProfileRepository) CreateProfile(ctx context.Context, profile *models.Profile) error {
    err := r.db.QueryRowContext(ctx, createProfileQuery,
        profile.ID, profile.FirstName, profile.LastName,
        profile.Email, profile.Phone, time.Now(), time.Now()).
        Scan(&profile.ID, &profile.CreatedAt, &profile.UpdatedAt)

    return err
}
```

## Performance Optimization

### Connection Pool Management

```go
// Enhanced connection pool configuration
func setupDatabasePool(cfg *Config) (*sqlx.DB, error) {
    db, err := sqlx.Connect("postgres", cfg.DatabaseURL)
    if err != nil {
        return nil, err
    }

    // Separate pools for sync and async operations
    db.SetMaxOpenConns(cfg.DatabaseMaxConns)
    db.SetMaxIdleConns(cfg.DatabaseIdleConns)
    db.SetConnMaxLifetime(5 * time.Minute)
    db.SetConnMaxIdleTime(1 * time.Minute)

    return db, nil
}
```

### Batch Processing Optimization

```go
// Optimized batch processing with chunking
func (s *BatchOperationsService) processBatchInChunks(ctx context.Context, operations []StorageOperation) error {
    chunkSize := 50 // Optimal chunk size for database operations

    for i := 0; i < len(operations); i += chunkSize {
        end := i + chunkSize
        if end > len(operations) {
            end = len(operations)
        }

        chunk := operations[i:end]
        if err := s.processChunk(ctx, chunk); err != nil {
            return fmt.Errorf("failed to process chunk %d-%d: %w", i, end-1, err)
        }
    }

    return nil
}
```

## Testing Strategy

### Integration Testing for Async Operations

```go
// tests/integration/async_operations_test.go
func TestAsyncProfileCreation(t *testing.T) {
    // Setup test environment with RabbitMQ
    testEnv := setupTestEnvironment(t)
    defer testEnv.Cleanup()

    // Create test message
    msg := &Message{
        ID:   uuid.New().String(),
        Type: "storage.profile.create",
        Payload: json.RawMessage(`{
            "operation": "create",
            "profile_data": {
                "first_name": "Test",
                "last_name": "User",
                "email": "test@example.com"
            }
        }`),
        Timestamp:  time.Now(),
        Metadata:   map[string]string{"correlation_id": "test-123"},
        RoutingKey: "storage.create",
    }

    // Publish message and wait for processing
    err := testEnv.PublishMessage(msg)
    assert.NoError(t, err)

    // Verify profile was created
    profile, err := testEnv.StorageService.GetProfileByEmail(context.Background(), "test@example.com")
    assert.NoError(t, err)
    assert.Equal(t, "Test", profile.FirstName)
}
```

## Deployment Configuration

### Enhanced Kubernetes Deployment

```yaml
# Enhanced deployment with queue configuration
apiVersion: apps/v1
kind: Deployment
metadata:
  name: storage-service
spec:
  replicas: 3
  template:
    spec:
      containers:
        - name: storage-service
          image: storage-service:latest
          env:
            - name: RABBITMQ_URL
              valueFrom:
                secretKeyRef:
                  name: rabbitmq-secret
                  key: url
            - name: QUEUE_NAME
              value: "storage-processing"
            - name: PREFETCH_COUNT
              value: "5"
            - name: MAX_BATCH_SIZE
              value: "100"
          resources:
            requests:
              memory: "256Mi"
              cpu: "250m"
            limits:
              memory: "512Mi"
              cpu: "500m"
```

This technical context provides a comprehensive foundation for implementing the storage-service integration with the task processing ecosystem, ensuring proper architecture, patterns, and operational excellence.
