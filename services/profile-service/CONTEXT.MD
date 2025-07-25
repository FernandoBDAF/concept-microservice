# Profile Service Technical Context

This document provides comprehensive technical details about the Profile Service's internal architecture, design patterns, implementation decisions, and integration with the multi-worker ecosystem.

## Service Architecture Overview

The **Profile Service** serves as the **primary entry point and orchestrator** for the microservices task processing ecosystem, implementing a clean architecture pattern with domain-driven design principles.

### Strategic Technical Position

```
Client Applications → Profile Service → Queue Service → RabbitMQ → Multi-Workers
     ↑                      ↑                                         ↓
Entry Point &        Message Format                           profile|email|image
Orchestrator         & Routing Logic                              workers
```

**Key Technical Responsibilities**:

- **Message Format Translation**: Converts HTTP requests to queue-service compatible messages
- **Routing Key Determination**: Maps task types to appropriate worker routing keys
- **Integration Orchestration**: Manages HTTP-based communication with queue-service
- **Multi-Worker Coordination**: Enables task distribution across specialized workers

## Directory Structure

```
profile-service/
├── cmd/                    # Application entry points
│   └── main.go            # Service initialization and dependency wiring
├── internal/              # Private application code
│   ├── api/               # HTTP API layer
│   │   ├── handlers/      # HTTP request handlers
│   │   ├── middleware/    # Authentication, logging, metrics middleware
│   │   └── routes/        # Route definitions and configuration
│   ├── config/            # Configuration management
│   │   └── config.go      # Environment-based configuration
│   ├── domain/            # Business logic and models
│   │   ├── models/        # Core domain entities
│   │   └── services/      # Business logic implementation
│   ├── pkg/               # Internal packages
│   │   ├── logger/        # Structured logging utilities
│   │   └── messaging/     # Queue-service HTTP client
│   └── infrastructure/    # External service integrations
│       ├── auth/          # Auth service gRPC client
│       ├── cache/         # Redis cache client
│       └── storage/       # Storage service gRPC client
├── test/                  # Test utilities and fixtures
├── docs/                  # Service documentation
└── go.mod                 # Go module definition
```

## Core Technical Components

### 1. Queue-Service Integration Layer

**File**: `internal/pkg/messaging/queue_client.go`

**Purpose**: HTTP-based communication with the upgraded queue-service

**Key Technical Implementation**:

```go
// QueueMessage represents the standardized message format
// compatible with the upgraded queue-service
type QueueMessage struct {
    ID         string            `json:"id"`         // UUID for message tracking
    Type       string            `json:"type"`       // Task type for routing
    Payload    json.RawMessage   `json:"payload"`    // Serialized task data
    Timestamp  time.Time         `json:"timestamp"`  // Message creation time
    Metadata   map[string]string `json:"metadata"`   // Context information
    RoutingKey string            `json:"routing_key"` // Worker routing key
}

// QueueClient handles HTTP communication with queue-service
type QueueClient struct {
    client     *http.Client      // HTTP client with timeout/retry configuration
    baseURL    string           // Queue-service endpoint URL
    config     *QueueConfig     // Client configuration
    routingMap map[string]string // Task type to routing key mapping
}
```

**Critical Technical Details**:

- **Message Format Compatibility**: Uses `json.RawMessage` for payload to match worker expectations
- **Routing Key Logic**: Automatic determination based on task type
- **HTTP Transport**: Optimized with connection pooling and keep-alive
- **Error Handling**: Comprehensive retry logic with exponential backoff

### 2. Routing Key Determination Logic

**File**: `internal/domain/services/profile.go`

**Technical Implementation**:

```go
// Routing key mapping for multi-worker architecture
var RoutingKeyMap = map[string]string{
    "profile_update":     "profile.task",    // → Profile Worker
    "email_notification": "email.send",      // → Email Worker
    "image_processing":   "image.process",   // → Image Worker
}

// determineRoutingKey maps task types to appropriate routing keys
func (s *ProfileService) determineRoutingKey(messageType string) string {
    if routingKey, exists := RoutingKeyMap[messageType]; exists {
        return routingKey
    }

    // Fallback to profile worker for unknown types
    return "profile.task"
}
```

**Design Rationale**:

- **Static Mapping**: Predictable routing behavior for debugging
- **Fallback Strategy**: Unknown task types default to profile worker
- **Extensibility**: Easy to add new worker types and routing keys

### 3. Message Processing Pipeline

**Technical Flow**:

```go
// Complete message processing pipeline
func (s *ProfileService) SubmitTask(ctx context.Context, profileID string, req *TaskRequest) (*Task, error) {
    // 1. Validate task request
    if err := s.validateTaskRequest(req); err != nil {
        return nil, err
    }

    // 2. Determine routing key
    routingKey := s.determineRoutingKey(req.Type)

    // 3. Serialize payload
    payloadJSON, err := json.Marshal(req.Payload)
    if err != nil {
        return nil, fmt.Errorf("payload serialization failed: %w", err)
    }

    // 4. Create queue-compatible message
    msg := &messaging.QueueMessage{
        ID:         uuid.New().String(),
        Type:       req.Type,
        Payload:    json.RawMessage(payloadJSON),
        Timestamp:  time.Now(),
        Metadata:   s.buildMessageMetadata(profileID, req),
        RoutingKey: routingKey,
    }

    // 5. Publish to queue-service
    if err := s.queueClient.PublishMessage(ctx, msg); err != nil {
        return nil, fmt.Errorf("queue publish failed: %w", err)
    }

    // 6. Create task record
    return s.createTaskRecord(profileID, req, routingKey)
}
```

## Design Patterns Implementation

### 1. Clean Architecture Pattern

**Layer Separation**:

```
┌─────────────────┐
│   HTTP API      │ ← handlers, routes, middleware
├─────────────────┤
│   Domain        │ ← business logic, entities, services
├─────────────────┤
│ Infrastructure  │ ← external service clients, database
└─────────────────┘
```

**Dependency Direction**: Outer layers depend on inner layers, never the reverse

**Benefits**:

- **Testability**: Business logic isolated from external dependencies
- **Maintainability**: Clear separation of concerns
- **Flexibility**: Easy to swap external service implementations

### 2. Repository Pattern for External Services

**Implementation**:

```go
// Service interfaces for dependency injection
type QueueServiceInterface interface {
    PublishMessage(ctx context.Context, msg *QueueMessage) error
    GetRoutingKeys(ctx context.Context) ([]string, error)
    HealthCheck(ctx context.Context) error
}

type StorageServiceInterface interface {
    CreateProfile(ctx context.Context, profile *Profile) (*Profile, error)
    GetProfile(ctx context.Context, id string) (*Profile, error)
    UpdateProfile(ctx context.Context, profile *Profile) (*Profile, error)
    DeleteProfile(ctx context.Context, id string) error
}
```

**Benefits**:

- **Testability**: Easy to mock external services
- **Flexibility**: Can switch implementations without changing business logic
- **Maintainability**: Clear contracts between layers

### 3. Factory Pattern for Message Creation

**Implementation**:

```go
// MessageFactory creates standardized queue messages
type MessageFactory struct {
    routingMap map[string]string
}

func (f *MessageFactory) CreateQueueMessage(taskType string, payload interface{}, metadata map[string]string) (*QueueMessage, error) {
    payloadJSON, err := json.Marshal(payload)
    if err != nil {
        return nil, err
    }

    return &QueueMessage{
        ID:         uuid.New().String(),
        Type:       taskType,
        Payload:    json.RawMessage(payloadJSON),
        Timestamp:  time.Now(),
        Metadata:   metadata,
        RoutingKey: f.determineRoutingKey(taskType),
    }, nil
}
```

## Technical Configuration Management

### Environment-Based Configuration

**File**: `internal/config/config.go`

```go
type Config struct {
    Server   ServerConfig   `json:"server"`
    Queue    QueueConfig    `json:"queue"`
    Storage  StorageConfig  `json:"storage"`
    Auth     AuthConfig     `json:"auth"`
    Cache    CacheConfig    `json:"cache"`
    Tasks    TaskConfig     `json:"tasks"`
    Logging  LoggingConfig  `json:"logging"`
}

type QueueConfig struct {
    URL          string                    `json:"url" env:"QUEUE_SERVICE_URL"`
    Timeout      time.Duration            `json:"timeout" env:"QUEUE_SERVICE_TIMEOUT" default:"30s"`
    Retries      int                      `json:"retries" env:"QUEUE_SERVICE_RETRIES" default:"3"`
    RoutingMap   map[string]string        `json:"routing_map"`
    TaskTimeouts map[string]time.Duration `json:"task_timeouts"`
}
```

**Configuration Sources** (in priority order):

1. Environment variables
2. Configuration file (`config.yaml`)
3. Default values
4. Runtime overrides

### Multi-Worker Task Configuration

```go
type TaskConfig struct {
    DefaultTimeout time.Duration `json:"default_timeout" default:"5m"`
    TypeTimeouts   map[string]time.Duration `json:"type_timeouts"`
    RetryPolicy    RetryConfig   `json:"retry_policy"`
}

// Task-specific timeout configuration
var DefaultTaskTimeouts = map[string]time.Duration{
    "profile_update":     5 * time.Minute,  // Standard processing
    "email_notification": 2 * time.Minute,  // Fast delivery required
    "image_processing":   10 * time.Minute, // Resource-intensive
}
```

## Error Handling Strategy

### Layered Error Handling

```go
// Domain-specific error types
type ProfileError struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
    Err     error  `json:"-"`
}

type QueueError struct {
    Code       string `json:"code"`
    Message    string `json:"message"`
    RoutingKey string `json:"routing_key,omitempty"`
    TaskType   string `json:"task_type,omitempty"`
}
```

### Error Recovery Patterns

**Circuit Breaker for Queue Service**:

```go
type CircuitBreaker struct {
    failureThreshold int
    resetTimeout     time.Duration
    state           CircuitState
    failures        int
    lastFailureTime time.Time
}

func (cb *CircuitBreaker) Execute(operation func() error) error {
    if cb.state == Open {
        if time.Since(cb.lastFailureTime) > cb.resetTimeout {
            cb.state = HalfOpen
        } else {
            return ErrCircuitOpen
        }
    }

    err := operation()
    return cb.recordResult(err)
}
```

**Retry Logic with Exponential Backoff**:

```go
func (c *QueueClient) PublishMessageWithRetry(ctx context.Context, msg *QueueMessage) error {
    var lastErr error

    for attempt := 0; attempt < c.config.Retries; attempt++ {
        if err := c.publishMessage(ctx, msg); err == nil {
            return nil
        } else {
            lastErr = err
            if !isRetriableError(err) {
                break
            }

            backoff := time.Duration(attempt) * time.Second
            select {
            case <-time.After(backoff):
                continue
            case <-ctx.Done():
                return ctx.Err()
            }
        }
    }

    return fmt.Errorf("max retries exceeded: %w", lastErr)
}
```

## Logging and Observability

### Structured Logging Implementation

**File**: `internal/pkg/logger/logger.go`

```go
// Structured logging with contextual information
func LogTaskSubmission(ctx context.Context, profileID, taskID, taskType, routingKey string) {
    logger.LogInfo(ctx, "Task submitted to queue",
        zap.String("profile_id", profileID),
        zap.String("task_id", taskID),
        zap.String("task_type", taskType),
        zap.String("routing_key", routingKey),
        zap.String("queue_service_url", queueServiceURL),
        zap.Duration("processing_time", processingTime))
}
```

### Metrics Collection

**Prometheus Metrics Implementation**:

```go
var (
    // Task submission metrics
    tasksSubmittedTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "profile_tasks_submitted_total",
            Help: "Total number of tasks submitted by type",
        },
        []string{"type", "routing_key"},
    )

    // Queue service communication metrics
    queueServiceRequests = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "profile_queue_service_requests_total",
            Help: "Total queue service requests by status",
        },
        []string{"status"},
    )

    // API endpoint metrics
    apiRequestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "profile_api_request_duration_seconds",
            Help: "API request duration by endpoint and method",
        },
        []string{"method", "endpoint"},
    )
)
```

## Security Implementation

### JWT Token Validation

```go
// Middleware for JWT token validation
func (m *AuthMiddleware) ValidateToken(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := extractBearerToken(r.Header.Get("Authorization"))
        if token == "" {
            http.Error(w, "Missing authorization token", http.StatusUnauthorized)
            return
        }

        userInfo, err := m.authService.ValidateToken(r.Context(), token)
        if err != nil {
            http.Error(w, "Invalid token", http.StatusUnauthorized)
            return
        }

        // Add user context to request
        ctx := context.WithValue(r.Context(), "user", userInfo)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### Permission-Based Authorization

```go
// Permission validation for different operations
func (s *ProfileService) checkPermission(ctx context.Context, userID, resource, action string) error {
    allowed, err := s.authService.CheckPermission(ctx, userID, resource, action)
    if err != nil {
        return fmt.Errorf("permission check failed: %w", err)
    }

    if !allowed {
        return ErrInsufficientPermissions
    }

    return nil
}
```

## Performance Optimization

### HTTP Client Optimization

```go
// Optimized HTTP client for queue-service communication
func NewOptimizedHTTPClient(timeout time.Duration) *http.Client {
    transport := &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 100,
        IdleConnTimeout:     90 * time.Second,
        DisableCompression:  false,
        ForceAttemptHTTP2:   true,
    }

    return &http.Client{
        Timeout:   timeout,
        Transport: transport,
    }
}
```

### Connection Pooling

```go
// gRPC connection pool for storage service
type ConnectionPool struct {
    connections chan *grpc.ClientConn
    factory     func() (*grpc.ClientConn, error)
    mu          sync.Mutex
    closed      bool
}

func (p *ConnectionPool) Get() (*grpc.ClientConn, error) {
    select {
    case conn := <-p.connections:
        return conn, nil
    default:
        return p.factory()
    }
}
```

## Testing Strategy

### Unit Testing Approach

```go
// Example unit test with mocked dependencies
func TestProfileService_SubmitTask(t *testing.T) {
    // Arrange
    mockQueueClient := &MockQueueClient{}
    mockStorageService := &MockStorageService{}

    service := &ProfileService{
        queueClient:    mockQueueClient,
        storageService: mockStorageService,
    }

    // Act
    task, err := service.SubmitTask(context.Background(), "profile-123", &TaskRequest{
        Type: "profile_update",
        Payload: map[string]interface{}{"action": "update"},
    })

    // Assert
    assert.NoError(t, err)
    assert.Equal(t, "profile_update", task.Type)
    mockQueueClient.AssertCalled(t, "PublishMessage", mock.Anything, mock.MatchedBy(func(msg *QueueMessage) bool {
        return msg.Type == "profile_update" && msg.RoutingKey == "profile.task"
    }))
}
```

### Integration Testing

```go
// Integration test with real queue-service
func TestProfileService_IntegrationWithQueueService(t *testing.T) {
    // Setup test environment
    queueServiceURL := os.Getenv("TEST_QUEUE_SERVICE_URL")
    if queueServiceURL == "" {
        t.Skip("TEST_QUEUE_SERVICE_URL not set")
    }

    // Create real client
    client, err := messaging.NewQueueClient(&messaging.QueueConfig{
        URL:     queueServiceURL,
        Timeout: 30 * time.Second,
        Retries: 3,
    })
    require.NoError(t, err)

    // Test message publishing
    msg := &messaging.QueueMessage{
        Type:       "profile_update",
        Payload:    json.RawMessage(`{"test": true}`),
        RoutingKey: "profile.task",
    }

    err = client.PublishMessage(context.Background(), msg)
    assert.NoError(t, err)
}
```

## Deployment Configuration

### Docker Multi-Stage Build

```dockerfile
# Build stage
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o profile-service ./cmd/main.go

# Runtime stage
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/profile-service .
EXPOSE 8080
CMD ["./profile-service"]
```

### Kubernetes Deployment Configuration

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: profile-service
spec:
  replicas: 3
  selector:
    matchLabels:
      app: profile-service
  template:
    metadata:
      labels:
        app: profile-service
    spec:
      containers:
        - name: profile-service
          image: profile-service:latest
          ports:
            - containerPort: 8080
          env:
            - name: QUEUE_SERVICE_URL
              value: "http://queue-service:8080"
            - name: STORAGE_SERVICE_HOST
              value: "storage-service"
            - name: AUTH_SERVICE_HOST
              value: "auth-service"
          resources:
            requests:
              memory: "128Mi"
              cpu: "100m"
            limits:
              memory: "256Mi"
              cpu: "200m"
          livenessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 30
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 5
```

## Technical Decisions and Rationale

### 1. HTTP vs Direct RabbitMQ Communication

**Decision**: Use HTTP to communicate with queue-service instead of direct RabbitMQ

**Rationale**:

- **Separation of Concerns**: Queue-service handles RabbitMQ complexity
- **Simplified Client**: Profile-service focuses on business logic
- **Reliability**: Queue-service provides connection management and retry logic
- **Monitoring**: Centralized queue operations monitoring

### 2. Message Format Standardization

**Decision**: Use `json.RawMessage` for payload field

**Rationale**:

- **Worker Compatibility**: Matches worker-service expectations
- **Type Safety**: Prevents serialization/deserialization issues
- **Performance**: Avoids double JSON marshaling
- **Flexibility**: Workers can deserialize based on message type

### 3. Routing Key Strategy

**Decision**: Static mapping with fallback to profile worker

**Rationale**:

- **Predictability**: Easy to debug and trace message routing
- **Reliability**: Unknown message types don't fail, they get processed
- **Simplicity**: Clear mapping without complex routing logic
- **Extensibility**: Easy to add new worker types

## Future Technical Enhancements

### 1. Advanced Routing Logic

```go
// Future: Rule-based routing with conditions
type RoutingRule struct {
    Condition func(*TaskRequest) bool
    RoutingKey string
    Priority   int
}

func (s *ProfileService) determineRoutingKeyAdvanced(req *TaskRequest) string {
    for _, rule := range s.routingRules {
        if rule.Condition(req) {
            return rule.RoutingKey
        }
    }
    return "profile.task" // fallback
}
```

### 2. Message Batching

```go
// Future: Batch message publishing for performance
type MessageBatch struct {
    Messages  []*QueueMessage
    BatchSize int
    Timeout   time.Duration
}

func (c *QueueClient) PublishBatch(ctx context.Context, batch *MessageBatch) error {
    // Implement batch publishing logic
}
```

### 3. Event Sourcing Integration

```go
// Future: Event sourcing for task history
type TaskEvent struct {
    EventID   string    `json:"event_id"`
    TaskID    string    `json:"task_id"`
    EventType string    `json:"event_type"`
    Payload   interface{} `json:"payload"`
    Timestamp time.Time `json:"timestamp"`
}
```

This technical context document provides comprehensive coverage of the Profile Service's internal architecture, implementation patterns, and integration strategies, ensuring maintainable and scalable code that aligns with the multi-worker ecosystem.
