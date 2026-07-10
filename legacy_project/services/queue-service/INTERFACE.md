# Queue Service Interface

## Production-Ready Interface

The Queue Service provides production-ready interfaces for multi-worker message publishing with full routing key support.

---

This document describes how the Queue Service connects with other services in the microservices architecture, including its enhanced HTTP API endpoints and multi-worker message queue interfaces.

## Service Overview

The Queue Service acts as a **central message publisher** for asynchronous communication between services. It provides reliable message delivery, routing key-based distribution, and dead letter queue handling for multiple worker types.

### **Production Architecture**

```
Client Services → Queue Service HTTP API → RabbitMQ Exchange → Worker Queues → Worker Services
     ↓                        ↓                    ↓                ↓              ↓
Profile Service     Routing Key Support    tasks-exchange    profile-processing  Profile Worker
Cache Service       Message Validation     email-tasks       email-processing    Email Worker
Job Service         Publisher Confirms     image-tasks       image-processing    Image Worker
```

## HTTP API Endpoints

### **1. Enhanced Message Publishing**

#### **Endpoint**

```http
POST /api/v1/queue/messages
Content-Type: application/json
```

#### **Request Format (New)**

```json
{
  "type": "profile_update",
  "routing_key": "profile.task",
  "payload": {
    "user_id": "123",
    "action": "update",
    "data": {
      "name": "John Doe",
      "email": "john@example.com"
    }
  },
  "metadata": {
    "correlation_id": "req-456",
    "source_service": "profile-service",
    "priority": "high",
    "retry_count": "0"
  }
}
```

#### **Response Format (Enhanced)**

```json
{
  "message_id": "123e4567-e89b-12d3-a456-426614174000",
  "status": "accepted",
  "routing_key": "profile.task",
  "target_queue": "profile-processing",
  "exchange": "tasks-exchange",
  "timestamp": "2024-01-15T10:30:00Z",
  "publisher_confirm": "pending"
}
```

#### **Supported Routing Keys**

| Routing Key     | Exchange       | Target Queue       | Worker Type    | Description             |
| --------------- | -------------- | ------------------ | -------------- | ----------------------- |
| `profile.task`  | tasks-exchange | profile-processing | Profile Worker | User profile operations |
| `email.send`    | email-tasks    | email-processing   | Email Worker   | Email notifications     |
| `image.process` | image-tasks    | image-processing   | Image Worker   | Image processing tasks  |

#### **Message Type Specifications**

##### **Profile Messages**

```json
{
  "type": "profile_update",
  "routing_key": "profile.task",
  "payload": {
    "user_id": "string",
    "action": "update|delete|create",
    "data": {
      "name": "string",
      "email": "string",
      "profile_data": "object"
    }
  }
}
```

##### **Email Messages** (Post-Upgrade)

```json
{
  "type": "email_notification",
  "routing_key": "email.send",
  "payload": {
    "recipient": "user@example.com",
    "email_type": "welcome|notification|alert",
    "template": "template_name",
    "data": {
      "user_name": "John Doe",
      "custom_data": "object"
    },
    "priority": "high|normal|low"
  }
}
```

##### **Image Processing Messages** (Post-Upgrade)

```json
{
  "type": "image_processing",
  "routing_key": "image.process",
  "payload": {
    "image_url": "https://example.com/image.jpg",
    "processing_type": "resize|filter|analyze",
    "parameters": {
      "width": 800,
      "height": 600,
      "quality": 85
    },
    "callback_url": "https://api.example.com/callback",
    "timeout_seconds": 300
  }
}
```

#### **Error Responses**

```json
// 400 Bad Request - Invalid routing key
{
  "error": "invalid_routing_key",
  "message": "Routing key 'invalid.key' is not supported",
  "supported_keys": ["profile.task", "email.send", "image.process"]
}

// 422 Unprocessable Entity - Invalid payload
{
  "error": "invalid_payload",
  "message": "Missing required field 'user_id' for profile.task routing key",
  "validation_errors": ["user_id is required"]
}

// 503 Service Unavailable - RabbitMQ connection issues
{
  "error": "service_unavailable",
  "message": "Unable to publish message - RabbitMQ connection unavailable",
  "retry_after": 30
}
```

### **2. Enhanced Message Status Tracking**

#### **Endpoint**

```http
GET /api/v1/queue/status/{messageId}
```

#### **Response Format (Enhanced)**

```json
{
  "id": "123e4567-e89b-12d3-a456-426614174000",
  "status": "delivered",
  "routing_key": "profile.task",
  "target_queue": "profile-processing",
  "exchange": "tasks-exchange",
  "publisher_confirm": "confirmed",
  "timestamps": {
    "created": "2024-01-15T10:30:00Z",
    "published": "2024-01-15T10:30:01Z",
    "confirmed": "2024-01-15T10:30:02Z"
  },
  "metadata": {
    "correlation_id": "req-456",
    "source_service": "profile-service",
    "retry_count": "0"
  }
}
```

#### **Status Values**

- `accepted` - Message received by queue service
- `published` - Message sent to RabbitMQ exchange
- `confirmed` - Publisher confirm received from RabbitMQ
- `delivered` - Message delivered to target queue
- `failed` - Message publishing failed
- `dead_lettered` - Message moved to dead letter queue

### **3. Health and Readiness Checks**

#### **Health Check**

```http
GET /health
```

**Response**:

```json
{
  "status": "healthy",
  "checks": {
    "rabbitmq_connection": "healthy",
    "rabbitmq_channel": "healthy",
    "exchange_status": "healthy",
    "publisher_confirms": "enabled"
  },
  "timestamp": "2024-01-15T10:30:00Z"
}
```

#### **Readiness Check**

```http
GET /ready
```

**Response**:

```json
{
  "ready": true,
  "services": {
    "rabbitmq": "connected",
    "exchanges": ["tasks-exchange", "email-tasks", "image-tasks"],
    "queues": ["profile-processing", "email-processing", "image-processing"]
  }
}
```

### **4. Metrics and Monitoring**

#### **Prometheus Metrics**

```http
GET /metrics
```

**Key Metrics**:

- `queue_messages_published_total{routing_key, worker_type}`
- `queue_messages_confirmed_total{routing_key}`
- `queue_publisher_confirm_duration_seconds{routing_key}`
- `queue_routing_key_distribution{routing_key}`
- `queue_connection_status{status}`
- `queue_exchange_health{exchange}`

## Message Queue Integration

### **1. Profile Processing Queue**

#### **Queue Configuration**

```yaml
Queue Name: profile-processing
Exchange: tasks-exchange
Routing Key: profile.task
TTL: 24 hours
Prefetch Count: 1
Dead Letter Queue: profile-processing.dlq
Durability: true
```

#### **Publishers**

- **Profile Service**: User profile changes, deletions, creations
- **Auth Service**: User authentication events
- **Admin Service**: Administrative profile operations

#### **Consumers**

- **Profile Worker**: Processes profile updates and deletions
- **Cache Service**: Updates user cache based on profile changes
- **Search Service**: Updates user search index

#### **Message Flow**

```
Profile Service → POST /api/v1/queue/messages (routing_key: profile.task)
                ↓
Queue Service → tasks-exchange (routing_key: profile.task)
                ↓
profile-processing queue → Profile Worker
```

### **2. Email Processing Queue** (Post-Upgrade)

#### **Queue Configuration**

```yaml
Queue Name: email-processing
Exchange: email-tasks
Routing Key: email.send
TTL: 1 hour
Prefetch Count: 5
Dead Letter Queue: email-processing.dlq
Durability: true
```

#### **Publishers**

- **Profile Service**: Welcome emails for new users
- **Notification Service**: Alert and notification emails
- **Marketing Service**: Campaign and promotional emails

#### **Consumers**

- **Email Worker**: Sends emails via SMTP/API providers
- **Analytics Service**: Tracks email delivery metrics

#### **Message Flow**

```
Notification Service → POST /api/v1/queue/messages (routing_key: email.send)
                     ↓
Queue Service → email-tasks (routing_key: email.send)
                     ↓
email-processing queue → Email Worker
```

### **3. Image Processing Queue** (Post-Upgrade)

#### **Queue Configuration**

```yaml
Queue Name: image-processing
Exchange: image-tasks
Routing Key: image.process
TTL: 6 hours
Prefetch Count: 1
Dead Letter Queue: image-processing.dlq
Durability: true
```

#### **Publishers**

- **Media Service**: Image uploads and transformations
- **Profile Service**: Avatar image processing
- **Content Service**: Content image optimization

#### **Consumers**

- **Image Worker**: Processes images via Python containers
- **CDN Service**: Uploads processed images to CDN
- **Thumbnail Service**: Generates image thumbnails

#### **Message Flow**

```
Media Service → POST /api/v1/queue/messages (routing_key: image.process)
              ↓
Queue Service → image-tasks (routing_key: image.process)
              ↓
image-processing queue → Image Worker
```

## Service Dependencies

### **Direct Dependencies**

- **RabbitMQ**: Message broker for all queue operations
- **Prometheus**: Metrics collection and monitoring
- **Common Queue Package**: Shared message format and utilities

### **Service Integrations**

#### **Upstream Services (Publishers)**

- **Profile Service**: Publishes profile-related messages
- **Notification Service**: Publishes email notification messages
- **Media Service**: Publishes image processing messages
- **Auth Service**: Publishes authentication-related messages

#### **Downstream Services (Consumers)**

- **Profile Worker**: Consumes profile processing messages
- **Email Worker**: Consumes email sending messages (planned)
- **Image Worker**: Consumes image processing messages (planned)

### **Integration Patterns**

#### **Request-Response Pattern**

```
Client Service → Queue Service → HTTP 202 Accepted → Message ID
Client Service → GET /status/{messageId} → Message Status
```

#### **Fire-and-Forget Pattern**

```
Client Service → Queue Service → RabbitMQ → Worker Service
```

#### **Publisher Confirms Pattern**

```
Queue Service → RabbitMQ Exchange → Publisher Confirm → Status Update
```

## Scaling and Performance

### **Horizontal Scaling**

#### **Queue Service Scaling**

- **Stateless Design**: Multiple queue service instances can run simultaneously
- **Load Balancing**: HTTP requests distributed across instances
- **Connection Pooling**: Each instance maintains its own RabbitMQ connection

#### **Worker Scaling**

- **Profile Worker**: 1-3 replicas (moderate processing load)
- **Email Worker**: 2-15 replicas (burst processing capability)
- **Image Worker**: 1-8 replicas (resource-intensive processing)

### **Performance Characteristics**

#### **Throughput Targets**

- **Profile Messages**: 100-500 messages/second
- **Email Messages**: 1000-5000 messages/second (burst)
- **Image Messages**: 10-50 messages/second (resource intensive)

#### **Latency Targets**

- **API Response Time**: < 100ms (message acceptance)
- **Publisher Confirm**: < 500ms (RabbitMQ confirmation)
- **End-to-End**: < 2 seconds (message to worker processing)

## Security and Authentication

### **API Security**

#### **Authentication** (Future Enhancement)

```http
Authorization: Bearer <JWT_TOKEN>
X-API-Key: <API_KEY>
```

#### **Message Validation**

- **Routing Key Validation**: Only allowed routing keys accepted
- **Payload Schema Validation**: Message format validation per routing key
- **Rate Limiting**: Per-service message rate limits
- **Input Sanitization**: Payload content sanitization

### **Transport Security**

- **HTTPS**: All HTTP API endpoints use TLS
- **AMQPS**: RabbitMQ connections use TLS (production)
- **Message Encryption**: Sensitive payload encryption (optional)

## Monitoring and Observability

### **Health Monitoring**

#### **Service Health Indicators**

- **RabbitMQ Connection**: Connection status and health
- **Exchange Status**: Exchange availability and configuration
- **Queue Status**: Queue depth and processing rates
- **Publisher Confirms**: Confirm success rates

#### **Alerting Thresholds**

- **Connection Failures**: > 5% failure rate
- **Publisher Confirm Failures**: > 1% failure rate
- **Queue Depth**: > 1000 messages pending
- **API Response Time**: > 500ms average

### **Metrics Collection**

#### **Business Metrics**

- **Message Volume**: Messages per routing key per hour
- **Processing Success Rate**: Successful message delivery rate
- **Worker Distribution**: Message distribution across worker types
- **Queue Performance**: Processing time per queue

#### **Technical Metrics**

- **API Performance**: Request latency and throughput
- **RabbitMQ Performance**: Connection, channel, and exchange metrics
- **Error Rates**: HTTP errors, publishing failures, validation errors

## Development and Testing

### **Integration Testing Endpoints**

#### **Test Message Publishing**

```bash
# Profile message test
curl -X POST http://localhost:8080/api/v1/queue/messages \
  -H "Content-Type: application/json" \
  -d '{
    "type": "profile_update",
    "routing_key": "profile.task",
    "payload": {"user_id": "test123", "action": "update"},
    "metadata": {"source": "integration_test"}
  }'

# Email message test (post-upgrade)
curl -X POST http://localhost:8080/api/v1/queue/messages \
  -H "Content-Type: application/json" \
  -d '{
    "type": "email_notification",
    "routing_key": "email.send",
    "payload": {"recipient": "test@example.com", "template": "welcome"},
    "metadata": {"source": "integration_test"}
  }'
```

#### **Status Verification**

```bash
# Check message status
curl http://localhost:8080/api/v1/queue/status/{message_id}

# Check service health
curl http://localhost:8080/health

# Check readiness
curl http://localhost:8080/ready
```

### **Mock Integration**

#### **Test Environment Setup**

```yaml
# k8s/debug/queue-service-integration/
├── rabbitmq.yaml          # RabbitMQ for testing
├── queue-service.yaml     # Queue service deployment
├── mock-workers.yaml      # Mock worker services
└── test-publisher.yaml    # Test message publisher
```

## Interface Evolution

### **Backward Compatibility**

#### **Legacy API Support**

- **Headers → Metadata**: API layer transforms headers to metadata
- **Message Type Enum**: Continues to accept enum values, converts to strings
- **Default Routing Keys**: Assigns default routing keys for legacy messages

#### **Migration Strategy**

1. **Phase 1**: Deploy new API with backward compatibility
2. **Phase 2**: Update client services to use new format
3. **Phase 3**: Remove legacy format support (future)

### **Future Enhancements**

#### **Advanced Routing**

- **Topic Exchanges**: Support for wildcard routing patterns
- **Message Filtering**: Content-based message filtering
- **Conditional Routing**: Route messages based on payload content

#### **Enhanced Features**

- **Message Scheduling**: Delayed message delivery
- **Message Batching**: Bulk message publishing
- **Message Transformation**: Automatic payload transformation
- **Multi-Tenancy**: Tenant-specific queues and routing

## Troubleshooting

### **Common Integration Issues**

#### **Message Format Errors**

```bash
# Check message validation errors
kubectl logs deployment/queue-service | grep "validation\|format"

# Test message format
curl -X POST http://localhost:8080/api/v1/queue/messages \
  -H "Content-Type: application/json" \
  -d '{"type": "test", "routing_key": "profile.task", "payload": {}}'
```

#### **Routing Key Issues**

```bash
# Check supported routing keys
curl http://localhost:8080/api/v1/queue/routing-keys

# Verify exchange bindings
kubectl exec -it rabbitmq-0 -- rabbitmqctl list_bindings
```

#### **Worker Integration Issues**

```bash
# Check worker consumption
kubectl exec -it rabbitmq-0 -- rabbitmqctl list_consumers

# Monitor queue depths
kubectl exec -it rabbitmq-0 -- rabbitmqctl list_queues name messages
```

## Related Documentation

- `README.md` - Service overview and development guide
- `TRACKER.md` - Implementation plan and task tracking
- `QUEUE_SERVICE_ANALYSIS.md` - Technical analysis and upgrade rationale
- `CONTEXT.md` - Technical implementation details
- `MIGRATION.md` - Upgrade procedures and compatibility guide

---

**Status**: 🎉 **PRODUCTION READY** - Complete multi-worker architecture with enhanced routing key integration.
