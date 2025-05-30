# Queue API Implementation Prompt

## Task Overview

Implement a Queue API service that provides reliable message queuing capabilities using RabbitMQ as the message broker, following the established microservices architecture patterns.

The current Follow the folder structure suggested in the @README.md and follow the interface guidelines in @INTERFACE.MD 

## Technical Requirements

### 1. Core Components

#### Queue Service

- Implement using Go 1.21+
- Use Gin framework for HTTP endpoints
- Integrate with RabbitMQ 3.12
- Implement Prometheus metrics collection

#### Message Processing

- Support multiple message types:
  - Profile updates
  - Cache invalidation
  - Background jobs
- Implement message validation
- Handle message acknowledgments
- Support dead letter queues

### 2. API Endpoints

#### Required Endpoints

```yaml
endpoints:
  - path: /api/v1/queue/messages
    method: POST
    description: Publish a new message
    requestBody:
      type: object
      required: true
      content:
        application/json:
          schema:
            $ref: "#/components/schemas/QueueMessage"
    responses:
      202: Message accepted
      400: Invalid message format
      500: Internal server error

  - path: /api/v1/queue/status/{messageId}
    method: GET
    description: Get message status
    parameters:
      - name: messageId
        type: string
        required: true
    responses:
      200: Message status
      404: Message not found
      500: Internal server error
```

### 3. Message Models

```protobuf
message QueueMessage {
  string id = 1;
  string type = 2;
  google.protobuf.Timestamp timestamp = 3;
  string correlation_id = 4;
  oneof payload {
    ProfileUpdate update = 5;
    CacheInvalidation cache = 6;
    BackgroundJob job = 7;
  }
  map<string, string> headers = 8;
  int32 priority = 9;
}
```

### 4. Configuration

```yaml
service:
  name: queue-service
  version: 1.0.0
  port: 8080
  environment: development
  rabbitmq:
    cluster:
      nodes:
        - host: rabbitmq-1
          port: 5672
        - host: rabbitmq-2
          port: 5672
        - host: rabbitmq-3
          port: 5672
    options:
      prefetch_count: 10
      reconnect_interval: 5s
      max_retries: 3
  logging:
    level: info
    format: json
  metrics:
    enabled: true
    port: 9090
```

### 5. Dependencies

```yaml
dependencies:
  - name: github.com/gin-gonic/gin
    version: v1.9.1
    purpose: HTTP framework
  - name: github.com/streadway/amqp
    version: v1.0.0
    purpose: RabbitMQ client
  - name: github.com/prometheus/client_golang
    version: v1.17.0
    purpose: Metrics collection
```

## Common Components Integration

### 1. Configuration Management (`config/`)

- Use the common configuration package for service settings
- Implement environment-based configuration
- Integrate secret management for RabbitMQ credentials
- Use configuration validation for required settings

```go
import "github.com/your-org/common/config"

type QueueConfig struct {
    config.BaseConfig
    RabbitMQ struct {
        Hosts    []string `validate:"required"`
        Username string   `validate:"required"`
        Password string   `validate:"required"`
        VHost    string   `validate:"required"`
    }
}
```

### 2. Error Handling (`errors/`)

- Use standardized error types for queue operations
- Implement error wrapping with context
- Integrate with error logging
- Handle specific queue-related errors

```go
import "github.com/your-org/common/errors"

var (
    ErrQueueConnection = errors.New("queue connection error")
    ErrMessagePublish  = errors.New("message publish error")
    ErrMessageConsume  = errors.New("message consume error")
)
```

### 3. Logging (`logging/`)

- Use structured logging for all operations
- Include relevant context in log messages
- Implement log levels appropriately
- Configure JSON formatting

```go
import "github.com/your-org/common/logging"

logger := logging.NewLogger("queue-service")
logger.Info("processing message",
    "message_id", msg.ID,
    "queue", queueName,
    "type", msg.Type,
)
```

### 4. Metrics (`metrics/`)

- Use Prometheus metrics for monitoring
- Implement standard metric types
- Add appropriate labels
- Configure metric collection

```go
import "github.com/your-org/common/metrics"

metrics.NewCounter("queue_messages_total",
    "Total number of messages processed",
    []string{"queue", "type"},
)
```

### 5. Models (`models/`)

- Use common model interfaces
- Implement validation
- Use standard serialization
- Follow versioning guidelines

```go
import "github.com/your-org/common/models"

type QueueMessage struct {
    models.BaseModel
    Type    string                 `json:"type" validate:"required"`
    Payload map[string]interface{} `json:"payload" validate:"required"`
}
```

### 6. Security (`security/`)

- Implement authentication middleware
- Use JWT validation
- Configure rate limiting
- Set up proper access controls

```go
import "github.com/your-org/common/security"

middleware := security.NewAuthMiddleware(
    security.WithJWTValidation(),
    security.WithRateLimiting(100, time.Minute),
)
```

### 7. Utils (`utils/`)

- Use common utility functions
- Implement standard time handling
- Use provided string manipulation functions
- Follow utility usage guidelines

```go
import "github.com/your-org/common/utils"

messageID := utils.GenerateUUID()
timestamp := utils.GetCurrentTimestamp()
```

## Implementation Tasks

### Phase 1: Core Infrastructure (Week 1)

1. **Project Setup**

   - [ ] Initialize Go module
   - [ ] Set up project structure
   - [ ] Configure dependencies
   - [ ] Set up development environment

2. **RabbitMQ Integration**

   - [ ] Implement connection management
   - [ ] Set up channel handling
   - [ ] Configure queue declarations
   - [ ] Implement error recovery

3. **Message Processing**
   - [ ] Define message types
   - [ ] Implement message validation
   - [ ] Set up message acknowledgment
   - [ ] Configure dead letter queues

### Phase 2: API Implementation (Week 2)

1. **HTTP Endpoints**

   - [ ] Implement message publishing endpoint
   - [ ] Implement status checking endpoint
   - [ ] Add request validation
   - [ ] Implement error handling

2. **Message Handling**
   - [ ] Implement message routing
   - [ ] Set up message persistence
   - [ ] Configure message TTL
   - [ ] Implement retry mechanism

### Phase 3: Monitoring & Operations (Week 3)

1. **Metrics & Monitoring**

   - [ ] Set up Prometheus metrics
   - [ ] Implement health checks
   - [ ] Configure logging
   - [ ] Set up alerting

2. **Deployment**
   - [ ] Create Dockerfile
   - [ ] Set up Kubernetes manifests
   - [ ] Configure environment variables
   - [ ] Set up CI/CD pipeline

## Verification Steps

### 1. Unit Testing

- [ ] Test message validation
- [ ] Test queue operations
- [ ] Test error handling
- [ ] Test retry mechanism

### 2. Integration Testing

- [ ] Test RabbitMQ integration
- [ ] Test API endpoints
- [ ] Test message flow
- [ ] Test error scenarios

### 3. Performance Testing

- [ ] Test message throughput
- [ ] Test concurrent operations
- [ ] Test recovery scenarios
- [ ] Test resource usage

## Documentation Requirements

### 1. API Documentation

- [ ] OpenAPI/Swagger specification
- [ ] Endpoint documentation
- [ ] Request/response examples
- [ ] Error codes and handling

### 2. Operational Documentation

- [ ] Deployment guide
- [ ] Configuration reference
- [ ] Monitoring guide
- [ ] Troubleshooting guide

## Completion Checklist

### 1. Code Quality

- [ ] Code follows Go best practices
- [ ] Proper error handling
- [ ] Comprehensive logging
- [ ] Clean code structure

### 2. Testing

- [ ] Unit tests coverage > 80%
- [ ] Integration tests passing
- [ ] Performance tests meeting requirements
- [ ] Security tests completed

### 3. Documentation

- [ ] API documentation complete
- [ ] Operational documentation complete
- [ ] Code comments and documentation
- [ ] Architecture diagrams updated

### 4. Deployment

- [ ] Docker image built and tested
- [ ] Kubernetes manifests ready
- [ ] CI/CD pipeline configured
- [ ] Environment variables documented

## Security Considerations

### 1. Authentication & Authorization

- [ ] Implement mTLS for service-to-service communication
- [ ] Configure JWT validation for external access
- [ ] Set up proper access controls
- [ ] Implement rate limiting

### 2. Data Security

- [ ] Encrypt sensitive data
- [ ] Implement message signing
- [ ] Configure secure connections
- [ ] Set up audit logging

## Monitoring & Alerting

### 1. Metrics

```yaml
metrics:
  - name: queue_messages_total
    type: counter
    labels:
      - queue
      - type
  - name: queue_processing_duration_seconds
    type: histogram
    labels:
      - queue
      - type
  - name: queue_size
    type: gauge
    labels:
      - queue
```

### 2. Health Checks

```yaml
health_checks:
  - name: readiness
    path: /health/ready
    interval: 30s
    timeout: 5s
    checks:
      - rabbitmq_connection
      - message_processing
  - name: liveness
    path: /health/live
    interval: 30s
    timeout: 5s
```
