# Queue Service Technical Context

## Internal Structure

### Core Components

1. **Domain Layer** (`internal/domain/`)

   - Models: Message types and structures
   - Service: Core business logic
   - Ports: Interface definitions

2. **Adapters** (`internal/adapters/`)

   - HTTP: REST API implementation
   - RabbitMQ: Message broker integration

3. **Configuration** (`internal/config/`)
   - Environment-based configuration
   - Validation rules
   - Default values

### Design Patterns

1. **Clean Architecture**

   - Separation of concerns
   - Dependency inversion
   - Interface-based design

2. **Repository Pattern**

   - Message status storage
   - In-memory implementation

3. **Factory Pattern**

   - Service creation
   - Configuration initialization

4. **Observer Pattern**
   - Message status updates
   - Event handling

### Frameworks and Libraries

1. **Core**

   - Gin: HTTP framework
   - RabbitMQ: Message broker
   - Prometheus: Metrics collection

2. **Utilities**
   - UUID: Message identification
   - Time: Timestamp handling
   - JSON: Message serialization

## Implementation Details

### Message Processing

1. **Message Types**

   - Profile updates
   - Cache invalidation
   - Background jobs

2. **Message Flow**

   - Validation
   - Persistence
   - Publishing
   - Consumption
   - Acknowledgment

3. **Error Handling**
   - Retry mechanism
   - Dead letter queues
   - Error logging

### RabbitMQ Integration

1. **Connection Management**

   - Cluster support
   - Reconnection logic
   - Channel pooling

2. **Queue Configuration**

   - Durable queues
   - Priority queues
   - Dead letter queues

3. **Message Properties**
   - Persistence
   - Priority
   - Headers
   - Correlation ID

### HTTP API

1. **Endpoints**

   - Message publishing
   - Status checking
   - Health monitoring
   - Metrics collection

2. **Middleware**

   - Logging
   - Error handling
   - Metrics collection

3. **Response Format**
   - JSON responses
   - Error handling
   - Status codes

## Service-Local Decisions

### Message Storage

- In-memory storage for message status
- Future: Consider persistent storage for message history

### Error Handling

- Immediate rejection of invalid messages
- No retry for malformed messages
- Retry for processing errors

### Configuration

- Environment-based configuration
- No configuration files
- Validation on startup

### Monitoring

- Prometheus metrics
- Health check endpoint
- Structured logging

## Future Considerations

1. **Scalability**

   - Message partitioning
   - Load balancing
   - Horizontal scaling

2. **Reliability**

   - Message persistence
   - Transaction support
   - Backup and recovery

3. **Security**

   - Authentication
   - Authorization
   - Rate limiting

4. **Monitoring**
   - Distributed tracing
   - Advanced metrics
   - Alerting

## Development Guidelines

1. **Code Organization**

   - Clean architecture principles
   - Interface-based design
   - Dependency injection

2. **Testing**

   - Unit tests
   - Integration tests
   - Performance tests

3. **Documentation**

   - API documentation
   - Code comments
   - Architecture diagrams

4. **Error Handling**

   - Consistent error types
   - Proper error wrapping
   - Error logging

5. **Logging**
   - Structured logging
   - Log levels
   - Context information

## Metrics Configuration

### Prometheus Metrics

1. **Message Metrics**

   ```go
   metrics.NewCounter("queue_messages_total",
       "Total number of messages processed",
       []string{"queue", "type"},
   )
   metrics.NewHistogram("queue_processing_duration_seconds",
       "Message processing duration",
       []string{"queue", "type"},
   )
   metrics.NewGauge("queue_size",
       "Current queue size",
       []string{"queue"},
   )
   ```

2. **Error Metrics**

   ```go
   metrics.NewCounter("queue_errors_total",
       "Total number of errors",
       []string{"queue", "type", "error"},
   )
   metrics.NewCounter("queue_retries_total",
       "Total number of retries",
       []string{"queue", "type"},
   )
   ```

3. **Performance Metrics**
   ```go
   metrics.NewHistogram("queue_latency_seconds",
       "Message processing latency",
       []string{"queue", "type"},
   )
   metrics.NewGauge("queue_consumers",
       "Number of active consumers",
       []string{"queue"},
   )
   ```

## Health Checks

### Readiness Probe

```go
func (s *QueueService) ReadinessCheck() error {
    // Check RabbitMQ connection
    if !s.rmq.IsConnected() {
        return errors.New("RabbitMQ not connected")
    }

    // Check message processing
    if !s.isProcessingMessages() {
        return errors.New("Message processing not active")
    }

    return nil
}
```

### Liveness Probe

```go
func (s *QueueService) LivenessCheck() error {
    // Check service health
    if !s.isHealthy() {
        return errors.New("Service not healthy")
    }

    return nil
}
```

## Testing Strategy

### Unit Tests

1. **Message Processing**

   - Message validation
   - Message routing
   - Message persistence

2. **Queue Operations**

   - Publish messages
   - Consume messages
   - Message acknowledgment

3. **Error Handling**
   - Connection errors
   - Processing errors
   - Retry mechanism

### Integration Tests

1. **RabbitMQ Integration**

   - Connection management
   - Queue operations
   - Error recovery

2. **API Integration**

   - Endpoint testing
   - Request validation
   - Response handling

3. **Service Integration**
   - Message flow
   - Error scenarios
   - Performance testing

### Performance Tests

1. **Load Testing**

   - Message throughput
   - Concurrent operations
   - Resource usage

2. **Stress Testing**

   - High message volume
   - Connection failures
   - Recovery scenarios

3. **Endurance Testing**
   - Long-running operations
   - Memory usage
   - Resource leaks

## Security Implementation

### Authentication

1. **JWT Validation**

   ```go
   middleware := security.NewAuthMiddleware(
       security.WithJWTValidation(),
       security.WithRateLimiting(100, time.Minute),
   )
   ```

2. **mTLS Configuration**
   ```go
   tlsConfig := &tls.Config{
       MinVersion: tls.VersionTLS12,
       CipherSuites: []uint16{
           tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
           tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
       },
   }
   ```

### Authorization

1. **Role-Based Access**

   ```go
   type Role string

   const (
       RoleAdmin  Role = "admin"
       RoleUser   Role = "user"
       RoleSystem Role = "system"
   )
   ```

2. **Resource Permissions**
   ```go
   type Permission struct {
       Resource string
       Action   string
       Role     Role
   }
   ```

### Data Security

1. **Message Encryption**

   ```go
   type EncryptedMessage struct {
       Data      []byte
       Algorithm string
       KeyID     string
   }
   ```

2. **Audit Logging**
   ```go
   type AuditLog struct {
       Timestamp time.Time
       User      string
       Action    string
       Resource  string
       Status    string
   }
   ```

## DEPLOYMENT

### RabbitMQ Deployment

- **Overview**: RabbitMQ is deployed as a Kubernetes pod within the cluster. It is configured to handle message queuing and is accessible via the AMQP port (5672) and the management port (15672).
- **Manifest**: The RabbitMQ deployment is defined in the `k8s/profile-service/base/database/rabbitmq.yaml` file. It includes configurations for resource limits, environment variables sourced from a Kubernetes secret, and health checks.
- **Health Checks**: The deployment uses liveness and readiness probes to ensure that RabbitMQ is operational. The probes are configured to check the AMQP port for connectivity.
- **Resource Limits**: The RabbitMQ pod is allocated specific CPU and memory resources to ensure optimal performance and stability.

### Queue Service Deployment

- **Overview**: The queue-service is deployed as a Kubernetes pod that interacts with RabbitMQ. It is responsible for managing message queuing and processing.
- **Manifest**: The queue-service deployment is defined in the `k8s/profile-service/base/queue-service/deployment.yaml` file. It includes configurations for resource limits, environment variables, and health checks.
- **Environment Variables**: The service is configured with environment variables such as `RABBITMQ_NODES` to specify the RabbitMQ connection details.
- **Health Checks**: The deployment uses liveness, readiness, and startup probes to ensure that the queue-service is operational and can communicate with RabbitMQ.
- **Resource Limits**: The queue-service pod is allocated specific CPU and memory resources to ensure optimal performance and stability.
