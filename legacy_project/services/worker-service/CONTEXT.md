# Worker Service Technical Context

## Service Architecture Overview

The Worker Service implements a **multi-worker architecture** built on clean architecture principles with domain-driven design patterns. The service has evolved from a single-purpose worker to support specialized, independently scalable worker types for different message processing needs.

## Internal Structure

### Directory Structure

```
services/worker-service/
├── cmd/                              # Original foundation worker entry point
├── internal/                         # Original foundation worker implementation
├── services/workers/                 # Multi-worker implementation
│   ├── common/                       # Shared worker components
│   │   ├── base/                     # BaseWorker implementation
│   │   │   ├── worker.go             # Core worker with signal handling
│   │   │   └── http_server.go        # HTTP health check server
│   │   ├── processors/               # Common processor interfaces
│   │   │   └── interface.go          # MessageProcessor interface
│   │   └── utils/                    # Shared utilities
│   │       ├── metrics.go            # Prometheus metrics collection
│   │       └── config.go             # Configuration management
│   ├── email-worker/                 # Email processing worker
│   │   ├── cmd/main.go               # Email worker entry point
│   │   ├── internal/processors/      # Email-specific processing logic
│   │   ├── Dockerfile                # Independent Docker image
│   │   └── k8s/                      # Kubernetes manifests
│   └── image-worker/                 # Image processing worker
│       ├── cmd/main.go               # Image worker entry point
│       ├── internal/processors/      # Image-specific processing logic
│       ├── Dockerfile                # Independent Docker image
│       └── k8s/                      # Kubernetes manifests
├── infrastructure/                   # Testing and deployment infrastructure
│   ├── rabbitmq/                     # RabbitMQ configuration and testing
│   └── monitoring/                   # Prometheus and Grafana configuration
└── scripts/                          # Automation scripts
    ├── dev-start.sh                  # Local Docker Compose testing
    └── k8s-deploy.sh                 # Kubernetes deployment testing
```

## Key Technical Components

### 1. Shared Foundation (BaseWorker Pattern)

**Framework**: Custom Go implementation with signal handling
**Purpose**: Provides consistent worker behavior across all worker types

```go
// Core BaseWorker structure
type BaseWorker struct {
    config    *WorkerConfig           // Worker-specific configuration
    processor processors.MessageProcessor  // Business logic processor
    consumer  *commonQueue.Consumer   // RabbitMQ message consumer
    server    *HTTPServer             // Health check HTTP server
}

// Signal handling and graceful shutdown
func (w *BaseWorker) Run() error {
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()

    // Start worker components concurrently
    // Handle shutdown gracefully
}
```

**Key Features**:

- Signal-based graceful shutdown (SIGTERM, SIGINT)
- Concurrent HTTP server for health checks
- Prometheus metrics integration
- Common error handling and logging patterns

### 2. Message Processing Interface

**Pattern**: Strategy Pattern with common interface
**Purpose**: Standardize message processing across worker types

```go
type MessageProcessor interface {
    Process(ctx context.Context, msg *commonQueue.Message) error
    Type() string
    Validate(msg *commonQueue.Message) error
    HandleError(ctx context.Context, msg *commonQueue.Message, err error) error
}
```

**Implementation Strategy**:

- Each worker type implements the MessageProcessor interface
- Common validation and error handling patterns
- Worker-specific business logic encapsulation
- Metrics collection integration

### 3. Worker-Specific Implementations

#### Email Worker Technical Details

**Processing Characteristics**:

- **I/O Intensive**: Mock email sending with external API simulation
- **Burst Processing**: High prefetch count (5) for throughput
- **Priority Handling**: Different processing delays based on message priority

```go
// Email message processing with priority-based delays
func (p *EmailProcessor) sendEmail(ctx context.Context, msg *EmailMessage) error {
    switch msg.Priority {
    case "high":
        time.Sleep(100 * time.Millisecond)  // Fast processing
    case "normal":
        time.Sleep(500 * time.Millisecond)  // Standard processing
    case "low":
        time.Sleep(1 * time.Second)         // Slower processing
    }
    // Mock email sending logic
}
```

**Resource Configuration**:

- **CPU**: 50m-200m (low CPU requirements)
- **Memory**: 64Mi-256Mi (moderate memory usage)
- **Scaling**: 2-15 replicas (aggressive scaling for bursts)

#### Image Worker Technical Details

**Processing Characteristics**:

- **CPU/Memory Intensive**: Mock Python container integration
- **Resource Heavy**: Lower prefetch count (1) for resource management
- **External Process Simulation**: Mock calls to Python image processing containers

```go
// Image processing with mock Python container calls
func (p *ImageProcessor) processImage(ctx context.Context, msg *ImageMessage) error {
    // Simulate resource-intensive processing
    time.Sleep(2 * time.Second)

    // Mock Python container call
    containerName := fmt.Sprintf("image-%s-service:latest", msg.ProcessingType)
    log.Printf("🐍 Calling Python container: %s", containerName)

    // Simulate container processing time
    time.Sleep(1 * time.Second)
    return nil
}
```

**Resource Configuration**:

- **CPU**: 500m-1000m (high CPU requirements)
- **Memory**: 512Mi-1Gi (high memory usage)
- **Scaling**: 1-8 replicas (conservative scaling)

## Design Patterns and Conventions

### 1. Clean Architecture Implementation

**Layers**:

- **Domain Layer**: Message processors and business logic
- **Application Layer**: Worker coordination and orchestration
- **Infrastructure Layer**: RabbitMQ consumers, HTTP servers, metrics

**Dependency Direction**: All dependencies point inward toward the domain layer

### 2. Configuration Management

**Pattern**: Environment-based configuration with defaults
**Framework**: Custom configuration structs with environment variable mapping

```go
type WorkerConfig struct {
    WorkerType    string  // Worker identification
    QueueName     string  // RabbitMQ queue name
    ExchangeName  string  // RabbitMQ exchange name
    RoutingKey    string  // Message routing key
    PrefetchCount int     // Message prefetch count
    HTTPPort      string  // Health check server port
}
```

### 3. Error Handling Strategy

**Pattern**: Centralized error handling with worker-specific customization
**Approach**: Return errors to trigger message requeue, log for observability

```go
func (p *BaseProcessor) HandleError(ctx context.Context, msg *commonQueue.Message, err error) error {
    p.metrics.IncErrors(p.processorType)
    log.Printf("Processing error for %s: %v", p.processorType, err)
    return err // Return error to trigger message requeue
}
```

### 4. Metrics Collection

**Framework**: Prometheus with custom metrics per worker type
**Pattern**: Decorator pattern around message processing

```go
// Metrics collected per worker type
type ProcessorMetrics interface {
    IncProcessed(processorType string)
    IncErrors(processorType string)
    ObserveProcessingTime(processorType string, duration float64)
}
```

## Infrastructure Integration

### 1. RabbitMQ Integration

**Library**: Common queue package (`github.com/fernandobarroso/common/queue`)
**Pattern**: Consumer-based message processing with manual acknowledgment

**Queue Specifications**:

| Worker Type | Exchange    | Queue            | Routing Key   | Prefetch |
| ----------- | ----------- | ---------------- | ------------- | -------- |
| Email       | email-tasks | email-processing | email.send    | 5        |
| Image       | image-tasks | image-processing | image.process | 1        |

### 2. Kubernetes Integration

**Deployment Strategy**: Independent deployments per worker type
**Scaling**: Horizontal Pod Autoscaler (HPA) with worker-specific thresholds

```yaml
# Email Worker HPA - Aggressive scaling
spec:
  minReplicas: 2
  maxReplicas: 15
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 60  # Lower threshold for faster scaling

# Image Worker HPA - Conservative scaling
spec:
  minReplicas: 1
  maxReplicas: 8
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70  # Higher threshold for stability
```

### 3. Monitoring and Observability

**Metrics Framework**: Prometheus with custom metrics
**Health Checks**: HTTP endpoints for Kubernetes probes
**Logging**: Structured logging with worker type identification

**Key Metrics**:

- `worker_messages_processed_total{worker_type}`
- `worker_processing_duration_seconds{worker_type}`
- `worker_errors_total{worker_type}`
- `worker_queue_depth{queue_name}`

## Development and Testing Conventions

### 1. Local Development

**Environment**: Docker Compose with complete stack
**Testing**: Automated scripts for environment setup and validation
**Debugging**: Individual worker logs and health check endpoints

### 2. Testing Strategy

**Unit Tests**: Mock implementations for external dependencies
**Integration Tests**: Full stack testing with RabbitMQ
**Load Tests**: Performance validation with message volume simulation

### 3. Deployment Strategy

**Local**: Docker Compose for development and testing
**Kubernetes**: Production-like deployment with proper resource allocation
**Scaling**: Automated testing of HPA behavior under load

## Performance Characteristics

### Email Worker Performance

- **Throughput**: 100+ messages/second
- **Latency**: 100ms-1s per message (priority-dependent)
- **Resource Usage**: Low CPU, moderate memory
- **Scaling Behavior**: Rapid scale-up for burst processing

### Image Worker Performance

- **Throughput**: 10-20 messages/second
- **Latency**: 2-3 seconds per message (simulated processing)
- **Resource Usage**: High CPU and memory
- **Scaling Behavior**: Conservative scaling with resource constraints

## Service-Local Decisions

### 1. Mock Implementation Strategy

**Decision**: Use mock implementations for external dependencies
**Rationale**: Enable complete testing without external service dependencies
**Implementation**: Simulated delays and logging for realistic behavior

### 2. Independent Deployment Architecture

**Decision**: Separate Docker images and Kubernetes manifests per worker
**Rationale**: Enable independent scaling and deployment of worker types
**Trade-off**: Increased operational complexity for deployment flexibility

### 3. Shared Foundation Pattern

**Decision**: Common BaseWorker and processor interfaces
**Rationale**: Reduce code duplication while enabling specialization
**Implementation**: Go interfaces with shared concrete implementations

### 4. Environment-Based Configuration

**Decision**: Configuration via environment variables with sensible defaults
**Rationale**: Support different deployment environments without code changes
**Pattern**: Struct-based configuration with environment variable mapping

This technical context provides the foundation for understanding the multi-worker architecture implementation and guides future development and operational decisions.
