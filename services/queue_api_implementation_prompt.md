# Queue API Implementation Prompt

## Task Overview

Implement a Queue API service that provides reliable message queuing capabilities using RabbitMQ as the message broker, following the established microservices architecture patterns.

## Current Status

### Completed Components ✅

1. **Core Infrastructure**

   - Project structure and module setup
   - Basic RabbitMQ integration
   - Message type definitions
   - Basic HTTP endpoints
   - Configuration management
   - Health checks
   - Prometheus metrics
   - Docker setup
   - Kubernetes deployment
   - Development overlay
   - Resource configuration
   - Service replication
   - RabbitMQ AMQP test implementation
   - RabbitMQ connection configuration
   - Environment-based configuration
   - Kubernetes secret integration

2. **Documentation**
   - README.md with service overview
   - CONTEXT.md with technical details
   - TRACKER.MD with implementation status
   - OpenAPI specification
   - Kubernetes manifests
   - Development configuration
   - RabbitMQ test documentation

### Required Improvements 🔄

Based on the API test results, the following improvements are needed:

1. **Metrics Implementation**

   - Implement missing Prometheus metrics:
     - `queue_messages_total` with labels for queue and message type
     - `queue_processing_duration_seconds` as a histogram with queue and type labels
     - `queue_size` as a gauge with queue label
     - `queue_errors_total` with labels for queue, type, and error
   - Ensure metrics are properly registered and exposed
   - Add metrics documentation
   - Implement metrics validation in tests

2. **Message Validation**

   - Add comprehensive message validation:
     - Required fields: id, type, timestamp, payload, priority
     - Message type validation against allowed types
     - Priority range validation (0-9)
     - Payload structure validation
     - Correlation ID presence
   - Implement validation error responses
   - Add validation tests

3. **Queue Operations**

   - Improve message publishing:
     - Add message ID generation
     - Implement timestamp handling
     - Add correlation ID support
     - Support message priority
   - Enhance status endpoint:
     - Add timestamp to status response
     - Implement proper status tracking
     - Add status validation
   - Add operation tests

4. **Error Handling**

   - Implement proper error responses
   - Add error logging
   - Implement error metrics
   - Add error recovery
   - Add error tests

5. **Testing**
   - Add comprehensive test suite:
     - Health check tests
     - Queue operation tests
     - Metrics tests
     - Error handling tests
     - Message validation tests
   - Implement test automation
   - Add test documentation

### In Progress 🔄

1. **Message Processing**

   - Message persistence
   - Dead letter queues
   - Retry mechanism
   - Error recovery
   - Message validation
   - Message acknowledgments
   - Message TTL

2. **Security**

   - Authentication
   - Authorization
   - Rate limiting
   - Request validation

3. **Testing**
   - Unit tests
   - Integration tests
   - Performance tests

## Technical Requirements

### 1. Core Components

#### Queue Service

- Implement using Go 1.21+
- Use Gin framework for HTTP endpoints
- Integrate with RabbitMQ 3.13.7
- Implement Prometheus metrics collection
- Support graceful shutdown
- Implement health checks
- Use Kubernetes secrets for credentials
- Support environment-based configuration

#### Message Processing

- Support multiple message types:
  - Profile updates
  - Cache invalidation
  - Background jobs
- Implement message validation
- Handle message acknowledgments
- Support dead letter queues
- Implement message persistence
- Support message TTL
- Implement retry mechanism

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
      "202": Message accepted
      "400": Invalid message format
      "401": Unauthorized
      "429": Too many requests
      "500": Internal server error

  - path: /api/v1/queue/status/{messageId}
    method: GET
    description: Get message status
    parameters:
      - name: messageId
        type: string
        required: true
    responses:
      "200": Message status
      "401": Unauthorized
      "404": Message not found
      "429": Too many requests
      "500": Internal server error

  - path: /health
    method: GET
    description: Health check endpoint
    responses:
      "200": Service is healthy
      "503": Service is unhealthy

  - path: /metrics
    method: GET
    description: Prometheus metrics
    responses:
      "200": Metrics in Prometheus format
```

### 3. Configuration

```yaml
service:
  name: queue-service
  version: 1.0.0
  port: 8080
  environment: development
  rabbitmq:
    cluster:
      nodes:
        - host: rabbitmq.default.svc.cluster.local
          port: 5672
    options:
      prefetch_count: 10
      reconnect_interval: 5s
      max_retries: 3
      message_ttl: 86400
  logging:
    level: info
    format: json
  metrics:
    enabled: true
    port: 9090
```

### 4. Dependencies

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
  - name: github.com/golang-jwt/jwt
    version: v4.0.0
    purpose: JWT authentication
  - name: github.com/redis/go-redis
    version: v9.0.0
    purpose: Rate limiting
```

## Implementation Tasks

### Phase 1: Core Infrastructure (Week 1) ✅

1. **Project Setup** ✅

   - [x] Initialize Go module
   - [x] Set up project structure
   - [x] Configure dependencies
   - [x] Set up development environment

2. **RabbitMQ Integration** ✅

   - [x] Implement connection management
   - [x] Set up channel handling
   - [x] Configure queue declarations
   - [x] Implement error recovery
   - [x] Configure credentials from secrets
   - [x] Set up service discovery

3. **Message Processing** ✅
   - [x] Define message types
   - [x] Implement message validation
   - [x] Set up message acknowledgment
   - [ ] Configure dead letter queues

### Phase 2: API Implementation (Week 2) ✅

1. **HTTP Endpoints** ✅

   - [x] Implement message publishing endpoint
   - [x] Implement status checking endpoint
   - [ ] Add request validation
   - [x] Implement error handling

2. **Message Handling** ✅
   - [x] Implement message routing
   - [ ] Set up message persistence
   - [ ] Configure message TTL
   - [ ] Implement retry mechanism

### Phase 3: Monitoring & Operations (Week 3) ✅

1. **Metrics & Monitoring** ✅

   - [x] Set up Prometheus metrics
   - [x] Implement health checks
   - [x] Configure logging
   - [ ] Set up alerting

2. **Deployment** ✅
   - [x] Create Dockerfile
   - [x] Set up Kubernetes manifests
   - [x] Configure environment variables
   - [x] Set up development overlay

### Phase 4: Security & Testing (Week 4) 🔄

1. **Security Implementation**

   - [ ] Add JWT authentication
   - [ ] Implement rate limiting
   - [ ] Set up mTLS
   - [ ] Configure RBAC

2. **Testing**
   - [ ] Write unit tests
   - [ ] Add integration tests
   - [ ] Implement performance tests
   - [ ] Set up test automation

### Phase 5: RabbitMQ Client Improvements 🔄

1. **Message Persistence Implementation**

   - [ ] Configure persistent message delivery

   ```go
   amqp.Publishing{
       DeliveryMode: amqp.Persistent,
       ContentType: "application/json",
       Body: body,
   }
   ```

   - [ ] Ensure queues are declared as durable
   - [ ] Add persistence configuration options
   - [ ] Implement persistence metrics

2. **Dead Letter Queue (DLQ) Setup**

   - [ ] Create DLX (Dead Letter Exchange)

   ```go
   channel.ExchangeDeclare(
       "dlx",
       "direct",
       true,  // durable
       false, // auto-delete
       false, // internal
       false, // no-wait
       nil,   // arguments
   )
   ```

   - [ ] Configure DLQ with TTL

   ```go
   args := amqp.Table{
       "x-dead-letter-exchange":    "dlx",
       "x-dead-letter-routing-key": queueName,
       "x-message-ttl":             int32(24 * 60 * 60 * 1000), // 24 hours
   }
   ```

   - [ ] Set up DLQ message handling
   - [ ] Implement DLQ monitoring
   - [ ] Add DLQ metrics collection

3. **Message Priority Support**

   - [ ] Configure queue with priority support

   ```go
   args := amqp.Table{
       "x-max-priority": 10, // Support 10 priority levels
   }
   ```

   - [ ] Add priority to message publishing

   ```go
   amqp.Publishing{
       Priority: uint8(priority),
       // ... other properties
   }
   ```

   - [ ] Implement priority-based routing
   - [ ] Add priority metrics

4. **Error Handling & Recovery**

   - [ ] Implement retry mechanism

   ```go
   type RetryConfig struct {
       MaxRetries    int
       RetryDelay    time.Duration
       MaxRetryDelay time.Duration
   }
   ```

   - [ ] Add circuit breaker pattern
   - [ ] Implement backoff strategy
   - [ ] Add error metrics collection
   - [ ] Set up error monitoring

5. **Connection Management**

   - [ ] Implement connection pooling

   ```go
   type ConnectionPool struct {
       MaxConnections int
       MaxChannels    int
       IdleTimeout    time.Duration
   }
   ```

   - [ ] Add connection health checks
   - [ ] Implement automatic reconnection
   - [ ] Add connection metrics
   - [ ] Set up connection monitoring

6. **Message TTL Configuration**

   - [ ] Add TTL configuration

   ```go
   type TTLConfig struct {
       DefaultTTL    time.Duration
       MaxTTL        time.Duration
       PerQueueTTL   map[string]time.Duration
   }
   ```

   - [ ] Implement per-message TTL
   - [ ] Add TTL validation
   - [ ] Set up TTL monitoring
   - [ ] Add TTL metrics

7. **Monitoring & Metrics**

   - [ ] Add message processing metrics

   ```go
   metrics:
     - name: message_processing_duration
       type: histogram
       labels:
         - queue
         - priority
     - name: message_retry_count
       type: counter
       labels:
         - queue
         - error_type
   ```

   - [ ] Implement queue depth monitoring
   - [ ] Add consumer lag metrics
   - [ ] Set up alerting rules
   - [ ] Create monitoring dashboards

8. **Configuration Management**

   - [ ] Add comprehensive configuration options

   ```yaml
   rabbitmq:
     connection:
       pool_size: 5
       idle_timeout: 60s
     queue:
       durable: true
       auto_delete: false
       max_priority: 10
     message:
       persistent: true
       ttl: 24h
     dlq:
       enabled: true
       ttl: 72h
     retry:
       max_attempts: 3
       initial_delay: 1s
       max_delay: 30s
   ```

   - [ ] Implement configuration validation
   - [ ] Add configuration documentation
   - [ ] Set up configuration monitoring

9. **Testing Requirements**

   - [ ] Add unit tests for new features
   - [ ] Implement integration tests
   - [ ] Add performance tests
   - [ ] Create chaos tests
   - [ ] Set up load tests

10. **Documentation Updates**

    - [ ] Update API documentation
    - [ ] Add configuration guide
    - [ ] Create troubleshooting guide
    - [ ] Update monitoring guide
    - [ ] Add performance tuning guide

### Verification Steps for RabbitMQ Improvements

1. **Message Persistence**

   - [ ] Verify messages survive broker restart
   - [ ] Test persistence with different message sizes
   - [ ] Validate persistence metrics

2. **DLQ Functionality**

   - [ ] Test message routing to DLQ
   - [ ] Verify TTL expiration
   - [ ] Test DLQ message processing
   - [ ] Validate DLQ metrics

3. **Priority Handling**

   - [ ] Test priority-based message ordering
   - [ ] Verify priority limits
   - [ ] Test priority with different queue depths
   - [ ] Validate priority metrics

4. **Error Recovery**

   - [ ] Test retry mechanism
   - [ ] Verify circuit breaker functionality
   - [ ] Test backoff strategy
   - [ ] Validate error metrics

5. **Connection Management**

   - [ ] Test connection pooling
   - [ ] Verify reconnection logic
   - [ ] Test connection limits
   - [ ] Validate connection metrics

6. **TTL Handling**
   - [ ] Test message expiration
   - [ ] Verify TTL configuration
   - [ ] Test per-message TTL
   - [ ] Validate TTL metrics

### Success Criteria

1. **Performance**

   - Message persistence overhead < 10%
   - Priority handling latency < 100ms
   - Connection recovery time < 5s
   - TTL processing delay < 1s

2. **Reliability**

   - Message loss rate < 0.01%
   - DLQ processing success rate > 99.9%
   - Connection stability > 99.9%
   - Error recovery success rate > 99%

3. **Monitoring**

   - All metrics available in Prometheus
   - Alerting rules configured
   - Dashboards created
   - Logging properly configured

4. **Documentation**
   - All features documented
   - Configuration guide complete
   - Troubleshooting guide available
   - Performance tuning guide ready

## Next Steps

1. **Immediate**

   - Implement message persistence
   - Add request validation
   - Add basic tests
   - Implement dead letter queues

2. **Short Term**

   - Add authentication
   - Add rate limiting
   - Implement message TTL
   - Add retry mechanism

3. **Long Term**
   - Add distributed tracing
   - Implement message batching
   - Add performance optimizations
   - Set up CI/CD pipeline

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

- [x] OpenAPI/Swagger specification
- [x] Endpoint documentation
- [x] Request/response examples
- [x] Error codes and handling

### 2. Operational Documentation

- [x] Deployment guide
- [x] Configuration reference
- [x] Monitoring guide
- [ ] Troubleshooting guide

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
  - name: queue_errors_total
    type: counter
    labels:
      - queue
      - type
      - error
  - name: queue_retries_total
    type: counter
    labels:
      - queue
      - type
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

## Completion Checklist

### 1. Code Quality

- [x] Code follows Go best practices
- [x] Proper error handling
- [x] Comprehensive logging
- [x] Clean code structure
- [ ] Code documentation
- [ ] Code review completed
- [ ] Performance optimization
- [ ] Memory management

### 2. Testing

- [ ] Unit tests coverage > 80%
- [ ] Integration tests passing
- [ ] Performance tests meeting requirements
- [ ] Security tests completed
- [ ] Load testing completed
- [ ] Chaos testing completed
- [ ] End-to-end tests
- [ ] Test documentation

### 3. Documentation

- [x] API documentation complete
- [x] Operational documentation complete
- [x] Code comments and documentation
- [ ] Architecture diagrams updated
- [ ] API examples
- [ ] Troubleshooting guide
- [ ] Security guidelines
- [ ] Performance tuning guide

### 4. Deployment

- [x] Docker image built and tested
- [ ] Kubernetes manifests ready
- [ ] CI/CD pipeline configured
- [x] Environment variables documented
- [ ] Deployment automation
- [ ] Rollback procedures
- [ ] Backup procedures
- [ ] Disaster recovery plan

### 5. Security

- [ ] Security audit completed
- [ ] Vulnerability scanning
- [ ] Penetration testing
- [ ] Security documentation
- [ ] Access control review
- [ ] Data encryption review
- [ ] Security monitoring
- [ ] Incident response plan

### 6. Monitoring

- [x] Metrics collection
- [x] Health checks
- [ ] Alerting configured
- [ ] Logging configured
- [ ] Tracing configured
- [ ] Dashboard created
- [ ] SLA monitoring
- [ ] Capacity planning
