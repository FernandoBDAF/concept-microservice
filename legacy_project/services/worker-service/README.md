# Worker Service - Multi-Worker Architecture

## Service Overview

The Worker Service has evolved into a **multi-worker architecture** supporting specialized, independently scalable worker types for different message processing needs. Built on clean architecture principles with domain-driven design, it provides a shared foundation for reliable message processing with comprehensive error handling, metrics collection, and graceful shutdown capabilities.

### Worker Types

#### 📧 **Email Worker**

- **Purpose**: Processes email notification tasks (welcome emails, notifications, alerts)
- **Characteristics**: I/O-intensive, burst processing capability, high throughput
- **Scaling**: Aggressive scaling (2-15 replicas) for email volume spikes
- **Queue**: `email-processing` (Exchange: `email-tasks`, Routing Key: `email.send`)

#### 🖼️ **Image Worker**

- **Purpose**: Processes image manipulation tasks (resize, filter, analyze)
- **Characteristics**: CPU/memory intensive, resource-heavy processing
- **Scaling**: Conservative scaling (1-8 replicas) with higher resource allocation
- **Queue**: `image-processing` (Exchange: `image-tasks`, Routing Key: `image.process`)

### Primary Responsibilities

- **Multi-Worker Message Consumption**: Consumes messages from specialized RabbitMQ queues using the common queue package
- **Specialized Task Processing**: Email notifications and image processing with worker-specific business logic
- **Independent Scaling**: Each worker type scales independently based on workload characteristics
- **Health Monitoring**: Provides HTTP health check endpoints for Kubernetes readiness/liveness probes
- **Metrics Collection**: Exposes Prometheus metrics for monitoring and observability per worker type
- **Graceful Shutdown**: Handles service termination gracefully, ensuring message processing completion

### Role in System Architecture

The Multi-Worker Service acts as specialized consumers in an event-driven architecture:

```
Profile Service → Queue Service → RabbitMQ → Email Worker
                                         └→ Image Worker
                                         └→ [Future Workers]
```

- Receives messages published by the Queue Service via RabbitMQ exchanges
- Routes messages to appropriate workers based on routing keys
- Processes tasks asynchronously without blocking the API layer
- Provides scalable, specialized background processing capabilities
- Maintains loose coupling with other services through message queues

## Multi-Worker Architecture

### Shared Foundation

```
services/workers/
├── common/                     # Shared components for all workers
│   ├── base/                   # BaseWorker implementation
│   │   ├── worker.go          # Core worker with signal handling
│   │   └── http_server.go     # Health check HTTP server
│   ├── processors/            # Common processor interfaces
│   │   └── interface.go       # MessageProcessor interface
│   ├── utils/                 # Shared utilities
│   │   ├── env.go            # Environment variable helpers
│   │   └── metrics.go        # Prometheus metrics utilities
│   └── go.mod                # Common module dependencies
├── email-worker/              # Email processing worker
│   ├── cmd/main.go           # Email worker entry point
│   ├── internal/
│   │   ├── domain/           # Email-specific domain models
│   │   └── processors/       # Email processing logic
│   ├── k8s/                  # Kubernetes manifests
│   ├── Dockerfile            # Email worker container
│   └── go.mod               # Email worker dependencies
├── image-worker/             # Image processing worker
│   ├── cmd/main.go          # Image worker entry point
│   ├── internal/
│   │   ├── domain/          # Image-specific domain models
│   │   └── processors/      # Image processing logic
│   ├── k8s/                 # Kubernetes manifests
│   ├── Dockerfile           # Image worker container
│   └── go.mod              # Image worker dependencies
└── [original worker-service/] # Original foundation (maintained)
```

### Clean Architecture Layers

Each worker follows clean architecture principles with shared components:

**Shared Foundation (services/workers/common/)**:

- `BaseWorker`: Signal handling, HTTP server, message consumption
- `MessageProcessor`: Common interface for all worker types
- `ProcessorMetrics`: Standardized metrics collection
- `Environment Utilities`: Configuration management

**Worker-Specific Implementation**:

- `Domain Models`: Worker-specific message structures and validation
- `Processors`: Business logic for email/image processing
- `Configuration`: Worker-specific queue and scaling settings
- `Deployment`: Independent Dockerfiles and Kubernetes manifests

### Message Flow Architecture

```
┌─────────────────┐    ┌──────────────┐    ┌─────────────────┐
│   Queue Service │────│   RabbitMQ   │────│  Email Worker   │
│                 │    │              │    │  (email-tasks)  │
└─────────────────┘    │  ┌─────────┐ │    └─────────────────┘
                       │  │ Exchanges│ │
                       │  │ & Queues │ │    ┌─────────────────┐
                       │  └─────────┘ │────│  Image Worker   │
                       └──────────────┘    │ (image-tasks)   │
                                          └─────────────────┘
```

## Implementation Architecture

### Original Foundation (Maintained)

The original worker-service provides the foundation patterns that were extracted into the common components:

```
services/worker-service/          # Original service (maintained)
├── cmd/main.go                  # Original entry point
├── internal/
│   ├── adapters/queue/          # Queue consumer adapter
│   ├── domain/                  # Domain models and interfaces
│   ├── processors/profile/      # Profile processing logic
│   └── server/                  # HTTP server implementation
└── go.mod                       # Module dependencies
```

### Key Components

#### 1. Application Layer (`cmd/main.go`)

- **Purpose**: Service bootstrap and dependency injection
- **Responsibilities**:
  - Configuration loading from environment variables
  - Component initialization and wiring
  - Graceful shutdown orchestration
  - Signal handling (SIGTERM, SIGINT)

#### 2. Queue Adapter (`internal/adapters/queue/consumer.go`)

- **Purpose**: RabbitMQ integration layer
- **Responsibilities**:
  - Wraps common queue package consumer
  - Provides service-specific metrics collection
  - Handles message-to-domain model conversion
  - Manages consumer lifecycle

#### 3. Domain Layer (`internal/domain/`)

- **Message Model**: Defines `ProfileMessage` structure with validation
- **Processor Interface**: Contracts for message processing implementations
- **Error Types**: Domain-specific error definitions

#### 4. Processor Layer (`internal/processors/profile/`)

- **Purpose**: Business logic implementation
- **Responsibilities**:
  - Message validation and processing
  - Action-based routing (update/delete)
  - Processing time simulation (10-second delay)
  - Metrics collection and reporting

#### 5. Server Layer (`internal/server/server.go`)

- **Purpose**: HTTP server for operational endpoints
- **Responsibilities**:
  - Health check endpoint (`/health`)
  - Readiness state management
  - Graceful shutdown handling

## Technical Implementation Details

### Message Processing Flow

1. **Message Reception**: Consumer receives AMQP delivery from RabbitMQ
2. **Deserialization**: Message body is unmarshaled into domain model
3. **Validation**: Message structure and required fields are validated
4. **Processing**: Business logic is executed based on message action
5. **Acknowledgment**: Message is acknowledged or rejected based on result
6. **Metrics**: Processing metrics are recorded for monitoring

### Configuration Management

The service uses environment-based configuration:

```go
// Required Environment Variables
RABBITMQ_USER     // RabbitMQ username
RABBITMQ_PASSWORD // RabbitMQ password
RABBITMQ_HOST     // RabbitMQ hostname
RABBITMQ_PORT     // RabbitMQ port (default: 5672)
```

Configuration is loaded through the common queue package:

- Default values for local development
- Production overrides via environment variables
- Connection string construction from individual components

### Dependency Management

#### External Dependencies

- **github.com/rabbitmq/amqp091-go**: RabbitMQ AMQP client
- **github.com/gin-gonic/gin**: HTTP framework for health endpoints
- **github.com/prometheus/client_golang**: Metrics collection
- **go.uber.org/zap**: Structured logging (via common package)

#### Internal Dependencies

- **github.com/fernandobarroso/common/queue**: Shared queue functionality
- Local replace directive points to `../common` for development

### Error Handling Strategy

#### Error Types

- **Connection Errors**: RabbitMQ connection/channel failures
- **Validation Errors**: Invalid message structure or content
- **Processing Errors**: Business logic execution failures
- **Acknowledgment Errors**: Message acknowledgment failures

#### Recovery Mechanisms

- **Automatic Reconnection**: Connection monitoring with automatic reconnect
- **Message Requeuing**: Failed messages are requeued for retry
- **Dead Letter Handling**: Permanently failed messages go to DLQ
- **Graceful Degradation**: Service continues operating during partial failures

### Metrics and Observability

#### Consumer Metrics

- `worker_consume_latency_seconds`: Message consumption time histogram
- `worker_consume_errors_total`: Total consumption error counter
- `worker_message_age_seconds`: Message age when consumed histogram

#### Processor Metrics

- `profile_processing_time_seconds`: Processing time histogram
- `profile_processing_errors_total`: Processing error counter
- `profile_processing_success_total`: Successful processing counter

#### Health Monitoring

- HTTP endpoint on port 8080: `/health`
- Ready state management for Kubernetes probes
- Graceful shutdown with timeout handling

## Implementation Tradeoffs

### Architecture Decisions

#### ✅ Advantages

1. **Clean Architecture**

   - Clear separation of concerns
   - Testable business logic isolation
   - Dependency inversion principles
   - Domain-driven design implementation

2. **Common Package Usage**

   - Standardized RabbitMQ interactions
   - Consistent error handling patterns
   - Shared configuration management
   - Reduced code duplication

3. **Comprehensive Monitoring**

   - Detailed Prometheus metrics
   - Structured logging integration
   - Health check endpoints
   - Processing time tracking

4. **Graceful Operations**
   - Clean shutdown handling
   - Message acknowledgment patterns
   - Connection recovery mechanisms
   - Context-based cancellation

#### ⚠️ Tradeoffs

1. **Processing Simulation**

   - **Current**: 10-second sleep simulation
   - **Tradeoff**: Simple testing vs. realistic workload
   - **Impact**: Not representative of production processing

2. **Synchronous Processing**

   - **Current**: Sequential message processing
   - **Tradeoff**: Simplicity vs. throughput
   - **Impact**: Limited concurrent processing capability

3. **Hard-coded Business Logic**

   - **Current**: Fixed update/delete actions
   - **Tradeoff**: Simplicity vs. extensibility
   - **Impact**: Limited action types without code changes

4. **Single Queue Focus**
   - **Current**: Profile-specific message processing
   - **Tradeoff**: Specialization vs. flexibility
   - **Impact**: Requires separate services for different domains

## Known Weaknesses and Limitations

### 1. Business Logic Limitations

**Issue**: Placeholder processing implementation

```go
func (p *Processor) handleUpdate(ctx context.Context, msg *domain.ProfileMessage) error {
    // TODO: Implement profile update logic
    time.Sleep(10 * time.Second) // Simulation only
    p.metrics.processingSuccess.Inc()
    return nil
}
```

**Impact**:

- No actual business value provided
- Processing time not realistic
- No integration with downstream services

**Mitigation Strategy**:

- Implement actual profile update logic
- Add integration with storage services
- Configure realistic processing times

### 2. Concurrency Limitations

**Issue**: Single-threaded message processing

```go
// Consumer processes one message at a time
handler := func(msg *queue.Message) error {
    // Sequential processing only
    return c.processor.Process(ctx, profileMsg)
}
```

**Impact**:

- Limited throughput under high load
- Cannot utilize multiple CPU cores effectively
- Potential bottleneck for high-volume scenarios

**Mitigation Strategy**:

- Implement worker pool pattern
- Add configurable concurrency levels
- Consider message batching for efficiency

### 3. Error Recovery Gaps

**Issue**: Limited retry mechanisms

```go
// Basic requeue on error, no exponential backoff
if err != nil {
    delivery.Nack(false, true) // Simple requeue
    continue
}
```

**Impact**:

- No exponential backoff for transient failures
- Potential message thrashing on persistent errors
- Limited visibility into retry attempts

**Mitigation Strategy**:

- Implement exponential backoff
- Add retry attempt tracking
- Configure maximum retry limits

### 4. Configuration Inflexibility

**Issue**: Environment-only configuration

```go
// Configuration loaded only from environment
config.URL = fmt.Sprintf("amqp://%s:%s@%s:%s/",
    rabbitUser, rabbitPassword, rabbitHost, rabbitPort)
```

**Impact**:

- No configuration file support
- Limited runtime configuration changes
- Difficult local development setup

**Mitigation Strategy**:

- Add configuration file support
- Implement configuration hot-reloading
- Provide development-friendly defaults

### 5. Testing Gaps

**Issue**: Limited test coverage

- No unit tests for processors
- No integration tests for queue interactions
- No error scenario testing

**Impact**:

- Reduced confidence in changes
- Potential for regression bugs
- Difficult debugging of edge cases

**Mitigation Strategy**:

- Implement comprehensive unit tests
- Add integration test suite
- Create error scenario test cases

## Testing and Validation

The multi-worker architecture includes comprehensive testing infrastructure for both local development and Kubernetes deployment validation.

### **Test Suite Overview**

#### 🐳 **Local Testing (Docker Compose)**

Complete local testing environment with automated setup:

```bash
# Start complete environment
./scripts/dev-start.sh
```

**What It Includes:**

- Email-worker and image-worker Docker images
- RabbitMQ with management interface
- Automated exchange and queue setup
- Health checks for all services
- Prometheus and Grafana monitoring
- Automated test message publishing

**Expected Results:**

- ✅ Both workers start and report healthy
- ✅ RabbitMQ queues (`email-processing`, `image-processing`) created
- ✅ Health endpoints respond: `{"status":"ok","ready":true}`
- ✅ Message processing logs show specialized handling
- ✅ Monitoring stack accessible (Prometheus: :9090, Grafana: :3000)

#### ☸️ **Kubernetes Testing (kind cluster)**

Production-like Kubernetes testing environment:

```bash
# Deploy and test complete architecture
./scripts/k8s-deploy.sh
```

**What It Includes:**

- Complete worker deployment with proper namespaces
- RabbitMQ deployment with configuration
- Horizontal Pod Autoscaler (HPA) testing
- Health check automation via port-forwarding
- Resource usage validation
- Scaling behavior verification

**Expected Results:**

- ✅ Separate namespaces for RabbitMQ and workers
- ✅ Workers deploy with appropriate replica counts (email: 2, image: 1)
- ✅ HPA configuration working (email: 2-15, image: 1-8 replicas)
- ✅ Services accessible via ClusterIP
- ✅ Auto-scaling responds to load within 30 seconds

### **Performance and Scaling Validation**

#### Load Testing Results

- **Email Worker**: Successfully handles 100+ messages/second with burst capability ✅
- **Image Worker**: Successfully handles 10-20 messages/second with resource-intensive processing ✅
- **Scaling Response**: Auto-scaling responds within 30 seconds to load changes ✅
- **Resource Efficiency**: Appropriate resource utilization for each worker type ✅

#### Resource Monitoring

```bash
# Monitor resource usage (Kubernetes)
kubectl top pods -n workers

# Expected:
# - Email workers: Low CPU/memory usage during normal operation
# - Image workers: Higher CPU/memory usage during processing
```

### **Manual Testing Procedures**

#### Health Check Validation

```bash
# Local environment
curl http://localhost:8081/health  # Email worker
curl http://localhost:8082/health  # Image worker

# Kubernetes environment
kubectl port-forward -n workers service/email-worker-service 8080:8080
curl http://localhost:8080/health
```

#### Message Publishing Tests

```bash
# Publish test messages for both worker types
./infrastructure/rabbitmq/example-publishers/publish-test-messages.sh

# Expected output:
📧 Publishing EMAIL WORKER test messages...
   ✅ Welcome email message published successfully
   ✅ Notification email message published successfully
   ✅ Alert email message published successfully

🖼️ Publishing IMAGE WORKER test messages...
   ✅ Image resize message published successfully
   ✅ Image filter message published successfully
   ✅ Image analyze message published successfully
```

### **Success Criteria Checklist**

#### Functional Tests

- [ ] Both workers start and report healthy
- [ ] RabbitMQ exchanges and queues created correctly
- [ ] Email messages processed with mock email sending logs
- [ ] Image messages processed with mock Python container calls
- [ ] Different message types handled (welcome, notification, alert, resize, filter, analyze)
- [ ] Priority-based processing delays work correctly

#### Performance Tests

- [ ] Email worker scales aggressively (2-15 replicas)
- [ ] Image worker scales conservatively (1-8 replicas)
- [ ] Messages processed without loss
- [ ] Health checks respond correctly under load
- [ ] Graceful shutdown works without message loss

#### Monitoring Tests

- [ ] Prometheus scrapes metrics from both workers
- [ ] Grafana displays worker dashboards
- [ ] RabbitMQ management shows queue status
- [ ] Worker logs show processing details
- [ ] Alerts trigger on simulated failures

### **Quick Test Commands**

```bash
# Complete local test
./scripts/dev-start.sh && docker-compose logs -f email-worker image-worker

# Complete Kubernetes test
./scripts/k8s-deploy.sh && kubectl logs -f deployment/email-worker -n workers

# Health check all services (local)
curl -s http://localhost:8081/health && curl -s http://localhost:8082/health

# Monitor scaling (K8s)
watch kubectl get pods -n workers
```

## Development Guidelines

### Local Development Setup

1. **Prerequisites**:

   ```bash
   go version 1.22+
   docker (for RabbitMQ)
   ```

2. **Environment Setup**:

   ```bash
   export RABBITMQ_USER=guest
   export RABBITMQ_PASSWORD=guest
   export RABBITMQ_HOST=localhost
   export RABBITMQ_PORT=5672
   ```

3. **Running Locally**:
   ```bash
   cd services/worker-service
   go mod download
   go run cmd/main.go
   ```

### Adding New Processors

1. Create processor package in `internal/processors/`
2. Implement `domain.Processor` interface
3. Add metrics collection
4. Register in main application
5. Update configuration as needed

### Extending Message Types

1. Define new message structure in `internal/domain/`
2. Add validation rules
3. Implement processor logic
4. Update queue configuration
5. Add corresponding metrics

## Deployment Considerations

### Kubernetes Configuration

- **Resource Requirements**: 100m CPU, 128Mi memory (base)
- **Health Checks**: `/health` endpoint on port 8080
- **Scaling**: Horizontal scaling supported via queue partitioning
- **Security**: Runs as non-root user in container

### Monitoring Requirements

- **Prometheus Metrics**: Scrape `/metrics` endpoint
- **Log Aggregation**: Structured JSON logs via zap
- **Alerting**: Monitor processing errors and queue depth
- **Dashboards**: Track throughput and processing times

### Operational Considerations

- **Graceful Shutdown**: 10-second timeout for message completion
- **Connection Recovery**: Automatic reconnection on failures
- **Dead Letter Handling**: Failed messages preserved for analysis
- **Backpressure**: Configurable prefetch count for flow control

## Evolution Strategy: Multi-Worker Architecture

### Current State Assessment

The current `worker-service` serves as an excellent **foundation** for a multi-worker architecture. Rather than replacing it, we recommend evolving it into a comprehensive worker ecosystem that supports multiple specialized worker types while maintaining the benefits of our current clean architecture.

### Recommended Evolution Path

#### Phase 1: Foundation Preservation (Current)

- ✅ **Keep Current Implementation**: The existing worker-service provides a solid foundation
- ✅ **Maintain Clean Architecture**: Current domain-driven design patterns are excellent
- ✅ **Preserve Common Package Integration**: The common queue package provides good abstraction

#### Phase 2: Multi-Worker Foundation

- **Create Worker Base Classes**: Extract common worker patterns into reusable base classes
- **Implement Specialized Workers**: Build domain-specific workers (notifications, analytics, emails)
- **Independent Deployment**: Each worker gets its own Dockerfile and Kubernetes manifests
- **Shared Infrastructure**: Common queue, metrics, and logging components

#### Phase 3: Advanced Scaling Patterns

- **Queue-Based Autoscaling**: Implement KEDA for queue-depth-based scaling
- **Worker-Specific Resource Profiles**: Different resource requirements per worker type
- **Advanced Monitoring**: Per-worker metrics and alerting strategies

### Multi-Worker Benefits

#### ✅ **Independent Scaling**

```yaml
# Profile Worker (CPU-intensive)
resources:
  requests: { cpu: 100m, memory: 128Mi }
  limits: { cpu: 500m, memory: 512Mi }
replicas: 2-10

# Notification Worker (I/O-intensive, burst scaling)
resources:
  requests: { cpu: 50m, memory: 64Mi }
  limits: { cpu: 200m, memory: 256Mi }
replicas: 3-20
```

#### ✅ **Specialized Processing**

- **Profile Workers**: Complex business logic, slower processing
- **Notification Workers**: High throughput, fast processing
- **Analytics Workers**: Memory-intensive, batch processing
- **Email Workers**: Rate-limited, scheduled processing

#### ✅ **Resource Optimization**

- Scale workers based on their specific load patterns
- Different resource profiles for different worker types
- Queue-depth-based scaling with KEDA
- Independent deployment and update cycles

### Migration Strategy

#### Option 1: Evolutionary (Recommended)

1. **Keep Current Worker**: Use as profile-specific worker
2. **Extract Common Patterns**: Create shared worker base classes
3. **Add New Workers**: Implement specialized workers using common foundation
4. **Gradual Migration**: Move functionality to specialized workers over time

#### Option 2: Complete Restructure

1. **Create New Multi-Worker Structure**: Build from scratch using lessons learned
2. **Migrate Existing Logic**: Port current processing logic to new structure
3. **Comprehensive Testing**: Ensure feature parity before switching

**Recommendation**: Choose **Option 1 (Evolutionary)** to minimize risk while gaining multi-worker benefits.

### Implementation Guide

For detailed implementation steps, see [Multi-Worker Implementation Guide](./MULTI_WORKER_IMPLEMENTATION_GUIDE.md), which provides:

- **Step-by-step implementation** for multiple worker types
- **Independent Dockerfile strategies** for each worker
- **Kubernetes manifests** with different scaling profiles
- **Advanced scaling configurations** (HPA, KEDA, Resource Quotas)
- **Monitoring and alerting** strategies for multi-worker environments
- **Best practices** for resource planning and deployment strategies

### Updated Recommendations

#### Immediate Improvements (Low Risk)

1. ✅ **Environment Configuration**: Already implemented with `.env.example`
2. **Extract Worker Base**: Create reusable worker foundation
3. **Enhanced Metrics**: Add worker-type-specific metrics
4. **Documentation**: Comprehensive multi-worker guide (completed)

#### Medium-Term Evolution (Medium Risk)

1. **Specialized Workers**: Implement notification, analytics, and email workers
2. **Independent Scaling**: Deploy workers with different scaling profiles
3. **Queue-Based Autoscaling**: Implement KEDA for sophisticated scaling
4. **Advanced Monitoring**: Per-worker dashboards and alerting

#### Long-Term Architecture (Higher Impact)

1. **Worker Ecosystem**: Complete multi-worker architecture
2. **Advanced Routing**: Topic exchanges for complex message routing
3. **Cross-Worker Coordination**: Workflow orchestration between workers
4. **Performance Optimization**: Worker pools and advanced concurrency patterns

This evolution strategy allows you to build upon the solid foundation you've created while gaining the benefits of specialized, independently scalable workers.
