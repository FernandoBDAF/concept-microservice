# Queue Service

## ⚠️ **CRITICAL UPGRADE IN PROGRESS** ⚠️

**Current Status**: The queue-service is undergoing a **critical architectural upgrade** to align with RabbitMQ best practices and support multi-worker architecture. See `TRACKER.md` for detailed implementation plan and `QUEUE_SERVICE_ANALYSIS.md` for technical analysis.

**Integration Status**: 🔴 **CURRENTLY INCOMPATIBLE** with worker-service. Upgrade required for proper integration.

---

A microservice that provides reliable message queuing capabilities using RabbitMQ as the message broker. This service acts as the central message publisher in a microservices architecture, routing messages to different worker types for asynchronous processing.

## Architecture Overview

### **Post-Upgrade Architecture** (Target State)

The service follows clean architecture principles with RabbitMQ best practices integration:

#### **Core Components**

- **Message Publisher**: Publishes messages to RabbitMQ exchanges with routing keys
- **Multi-Worker Support**: Routes messages to different worker types (profile, email, image)
- **HTTP API**: RESTful endpoints for message publishing and status tracking
- **Metrics & Monitoring**: Comprehensive Prometheus integration
- **Health Checks**: Kubernetes-ready health and readiness probes

#### **RabbitMQ Integration Pattern**

```
Profile Service → Queue Service HTTP API → RabbitMQ Exchange → Worker Queues
                                              ↓
                    ┌─────────────────────────┼─────────────────────────┐
                    ↓                         ↓                         ↓
            profile.task                 email.send              image.process
                    ↓                         ↓                         ↓
        profile-processing            email-processing         image-processing
                    ↓                         ↓                         ↓
           Profile Worker              Email Worker             Image Worker
```

#### **Exchange and Routing Strategy**

- **Single Exchange Approach**: Uses `tasks-exchange` with routing keys for message distribution
- **Semantic Routing**: Messages routed based on content type and target worker
- **Worker Isolation**: Each worker type has dedicated queues and processing logic

```go
// Routing Key Patterns
"profile.task"   → profile-processing queue → Profile Worker
"email.send"     → email-processing queue  → Email Worker
"image.process"  → image-processing queue  → Image Worker
```

## Features

### **Current Features**

- Message publishing via HTTP API
- Message status tracking
- Prometheus metrics integration
- Health check endpoints
- Graceful shutdown handling
- Environment-based configuration

### **Post-Upgrade Features** ✨

- **Multi-Worker Support**: Route messages to profile, email, and image workers
- **RabbitMQ Best Practices**: Single exchange with routing keys
- **Publisher Confirms**: Reliable message delivery with acknowledgments
- **Dynamic Queue Configuration**: Worker-specific queue properties
- **Enhanced Monitoring**: Per-worker-type metrics and routing key distribution
- **Backward Compatibility**: Existing API continues working during transition

## Message Format

### **New Message Format** (Post-Upgrade)

```json
{
  "id": "123e4567-e89b-12d3-a456-426614174000",
  "type": "profile_update",
  "routing_key": "profile.task",
  "payload": {
    "user_id": "user123",
    "action": "update",
    "data": {
      "name": "John Doe",
      "email": "john@example.com"
    }
  },
  "metadata": {
    "correlation_id": "req-456",
    "source_service": "profile-service",
    "priority": "normal"
  },
  "timestamp": "2024-01-15T10:30:00Z"
}
```

### **Supported Routing Keys**

| Routing Key     | Target Queue       | Worker Type    | Use Case                              |
| --------------- | ------------------ | -------------- | ------------------------------------- |
| `profile.task`  | profile-processing | Profile Worker | User profile updates, deletions       |
| `email.send`    | email-processing   | Email Worker   | Welcome emails, notifications, alerts |
| `image.process` | image-processing   | Image Worker   | Resize, filter, analyze operations    |

## API Endpoints

### **Message Publishing** (Enhanced)

```http
POST /api/v1/queue/messages
Content-Type: application/json

{
  "type": "profile_update",
  "routing_key": "profile.task",
  "payload": {
    "user_id": "123",
    "action": "update",
    "data": { "name": "John Doe" }
  },
  "metadata": {
    "priority": "high",
    "source": "profile-service"
  }
}
```

**Response**:

```json
{
  "message_id": "123e4567-e89b-12d3-a456-426614174000",
  "status": "accepted",
  "routing_key": "profile.task",
  "queue": "profile-processing"
}
```

### **Message Status Tracking**

```http
GET /api/v1/queue/status/{messageId}
```

**Response**:

```json
{
  "id": "123e4567-e89b-12d3-a456-426614174000",
  "status": "delivered",
  "timestamp": "2024-01-15T10:30:00Z",
  "routing_key": "profile.task",
  "queue": "profile-processing"
}
```

### **Health and Monitoring**

```http
GET /health          # Health check
GET /metrics         # Prometheus metrics
```

## Configuration

### **Environment Variables**

| Variable            | Description               | Default                            | Example                         |
| ------------------- | ------------------------- | ---------------------------------- | ------------------------------- |
| `SERVICE_PORT`      | HTTP server port          | 8080                               | 8080                            |
| `SERVICE_ENV`       | Environment               | development                        | production                      |
| `RABBITMQ_URL`      | RabbitMQ connection URL   | amqp://guest:guest@localhost:5672/ | amqp://user:pass@rabbitmq:5672/ |
| `RABBITMQ_EXCHANGE` | Default exchange name     | tasks-exchange                     | tasks-exchange                  |
| `LOG_LEVEL`         | Logging level             | info                               | debug                           |
| `METRICS_ENABLED`   | Enable Prometheus metrics | true                               | true                            |

### **Worker-Specific Configuration**

```yaml
# Profile Worker Queue
PROFILE_QUEUE_NAME: "profile-processing"
PROFILE_ROUTING_KEY: "profile.task"
PROFILE_TTL: "24h"
PROFILE_PREFETCH: "1"

# Email Worker Queue
EMAIL_QUEUE_NAME: "email-processing"
EMAIL_ROUTING_KEY: "email.send"
EMAIL_TTL: "1h"
EMAIL_PREFETCH: "5"

# Image Worker Queue
IMAGE_QUEUE_NAME: "image-processing"
IMAGE_ROUTING_KEY: "image.process"
IMAGE_TTL: "6h"
IMAGE_PREFETCH: "1"
```

## Integration with Worker Services

### **Profile Worker Integration**

```go
// Profile Service publishes message
POST /api/v1/queue/messages
{
  "type": "profile_update",
  "routing_key": "profile.task",
  "payload": { "user_id": "123", "action": "update" }
}

// Profile Worker consumes from profile-processing queue
// Uses common queue package with routing key "profile.task"
```

### **Email Worker Integration** (Planned)

```go
// Email messages
POST /api/v1/queue/messages
{
  "type": "email_notification",
  "routing_key": "email.send",
  "payload": {
    "recipient": "user@example.com",
    "template": "welcome",
    "data": { "name": "John" }
  }
}
```

### **Image Worker Integration** (Planned)

```go
// Image processing messages
POST /api/v1/queue/messages
{
  "type": "image_processing",
  "routing_key": "image.process",
  "payload": {
    "image_url": "https://example.com/image.jpg",
    "processing_type": "resize",
    "parameters": { "width": 800, "height": 600 }
  }
}
```

## Monitoring and Metrics

### **Enhanced Metrics** (Post-Upgrade)

#### **Message Metrics**

- `queue_messages_published_total{routing_key, worker_type}`
- `queue_messages_confirmed_total{routing_key}`
- `queue_messages_failed_total{routing_key, error_type}`
- `queue_routing_key_distribution{routing_key}`

#### **Performance Metrics**

- `queue_publish_duration_seconds{routing_key}`
- `queue_confirm_duration_seconds{routing_key}`
- `queue_connection_status{status}`
- `queue_channel_status{status}`

#### **Worker-Specific Metrics**

- `queue_worker_message_rate{worker_type}`
- `queue_worker_queue_depth{worker_type, queue}`
- `queue_worker_processing_time{worker_type}`

### **Monitoring Dashboard**

Key metrics to monitor:

- **Message Throughput**: Messages per second by routing key
- **Routing Distribution**: Message distribution across worker types
- **Publisher Confirms**: Success rate of message delivery
- **Queue Health**: Depth and processing rates per worker queue
- **Connection Status**: RabbitMQ connection and channel health

## Development

### **Prerequisites**

- Go 1.21+
- RabbitMQ 3.12+
- Docker (for local development)
- Access to common queue package

### **Local Development Setup**

```bash
# Clone repository and navigate to queue-service
cd services/queue-service

# Install dependencies
go mod download

# Set environment variables
export RABBITMQ_URL="amqp://user:password@localhost:5672/"
export LOG_LEVEL="debug"
export SERVICE_ENV="development"

# Run the service
go run cmd/main.go
```

### **Docker Development**

```bash
# Build image
docker build -t queue-service:latest .

# Run with Docker Compose (includes RabbitMQ)
docker-compose up -d
```

### **Testing Integration**

```bash
# Test message publishing
curl -X POST http://localhost:8080/api/v1/queue/messages \
  -H "Content-Type: application/json" \
  -d '{
    "type": "profile_update",
    "routing_key": "profile.task",
    "payload": {"user_id": "123", "action": "update"},
    "metadata": {"source": "test"}
  }'

# Check message status
curl http://localhost:8080/api/v1/queue/status/{message_id}

# Check health
curl http://localhost:8080/health

# Check metrics
curl http://localhost:8080/metrics
```

## Deployment

### **Kubernetes Deployment**

```bash
# Apply queue-service manifests
kubectl apply -k k8s/profile-service/base/queue-service/

# Check deployment status
kubectl get pods -l app=queue-service
kubectl logs -f deployment/queue-service
```

### **Environment-Specific Configuration**

```bash
# Development
kubectl apply -k k8s/profile-service/overlays/development/queue-service/

# Production (future)
kubectl apply -k k8s/profile-service/overlays/production/queue-service/
```

## Migration Guide

### **Upgrading from Current Version**

⚠️ **Important**: The upgrade involves breaking changes to message format and routing strategy.

1. **Pre-Upgrade Checklist**:

   - [ ] Backup current RabbitMQ state
   - [ ] Verify worker-service compatibility
   - [ ] Review routing key mappings
   - [ ] Plan maintenance window

2. **Upgrade Process**:

   ```bash
   # 1. Deploy new queue-service version
   kubectl apply -k k8s/profile-service/base/queue-service/

   # 2. Verify message compatibility
   # 3. Update client services to use routing keys
   # 4. Monitor message flow and metrics
   ```

3. **Post-Upgrade Validation**:
   - [ ] Messages routing to correct worker queues
   - [ ] Publisher confirms working
   - [ ] Metrics collection operational
   - [ ] Dead letter queues functioning

## Troubleshooting

### **Common Issues**

#### **Message Routing Problems**

```bash
# Check exchange and queue bindings
kubectl exec -it rabbitmq-0 -- rabbitmqctl list_bindings

# Verify routing key configuration
kubectl logs deployment/queue-service | grep "routing_key"
```

#### **Worker Integration Issues**

```bash
# Check message format compatibility
kubectl logs deployment/worker-service | grep "unmarshal\|parse"

# Verify queue consumption
kubectl exec -it rabbitmq-0 -- rabbitmqctl list_queues name messages
```

#### **Performance Issues**

```bash
# Check publisher confirm rates
curl http://queue-service:8080/metrics | grep confirm

# Monitor connection health
kubectl logs deployment/queue-service | grep "connection\|channel"
```

## Architecture Decision Records

### **ADR-001: Single Exchange with Routing Keys**

- **Decision**: Use single exchange with routing keys instead of per-queue exchanges
- **Rationale**: Aligns with RabbitMQ best practices, simplifies management, enables multi-worker support
- **Trade-offs**: Requires routing key discipline, but provides better scalability

### **ADR-002: Message Format Standardization**

- **Decision**: Standardize message format with common queue package
- **Rationale**: Ensures compatibility across all services, reduces integration complexity
- **Trade-offs**: Breaking change, but necessary for proper service integration

### **ADR-003: Publisher Confirms Implementation**

- **Decision**: Implement publisher confirms for reliable delivery
- **Rationale**: Ensures message delivery guarantees, improves reliability
- **Trade-offs**: Slight performance impact, but critical for production reliability

## Related Documentation

- `TRACKER.md` - Detailed implementation plan and task tracking
- `QUEUE_SERVICE_ANALYSIS.md` - Technical analysis and upgrade rationale
- `INTERFACE.md` - Service interfaces and integration patterns
- `CONTEXT.md` - Technical implementation details
- `MIGRATION.md` - Upgrade procedures and compatibility guide

## Support and Contributing

### **Getting Help**

- Check `TRACKER.md` for current implementation status
- Review `QUEUE_SERVICE_ANALYSIS.md` for technical details
- See integration examples in `k8s/debug/` directories

### **Contributing**

- Follow clean architecture principles
- Maintain RabbitMQ best practices alignment
- Update documentation for any API changes
- Include integration tests for new features

---

**Status**: 🔄 **Active Development** - Critical upgrade in progress for multi-worker architecture support.
