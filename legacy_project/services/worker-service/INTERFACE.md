# Worker Service Interface Documentation

## Service Interface Overview

The Worker Service operates as a **multi-worker message consumer** in the microservices architecture, implementing specialized asynchronous task processing through RabbitMQ message queues. It provides operational HTTP endpoints for health monitoring while consuming messages from worker-specific queues populated by the Queue Service.

## Multi-Worker Architecture

### Worker Types and Interfaces

#### 📧 **Email Worker**

- **Queue**: `email-processing`
- **Exchange**: `email-tasks`
- **Routing Key**: `email.send`
- **HTTP Port**: 8081 (local), 8080 (Kubernetes)
- **Characteristics**: I/O-intensive, burst processing, high throughput

#### 🖼️ **Image Worker**

- **Queue**: `image-processing`
- **Exchange**: `image-tasks`
- **Routing Key**: `image.process`
- **HTTP Port**: 8082 (local), 8080 (Kubernetes)
- **Characteristics**: CPU/memory intensive, resource-heavy processing

## External Service Connections

### 1. RabbitMQ Message Broker

**Connection Type**: Direct AMQP Consumer (per worker type)
**Purpose**: Primary message consumption interface

#### Connection Configuration

```go
// Environment-based connection (shared across workers)
URL: amqp://{RABBITMQ_USER}:{RABBITMQ_PASSWORD}@{RABBITMQ_HOST}:{RABBITMQ_PORT}/
```

#### Email Worker Queue Specifications

| Property           | Value              | Purpose                             |
| ------------------ | ------------------ | ----------------------------------- |
| **Exchange**       | `email-tasks`      | Direct exchange for email routing   |
| **Queue**          | `email-processing` | Email processing queue              |
| **Routing Key**    | `email.send`       | Email message routing identifier    |
| **Durability**     | `true`             | Persist messages to disk            |
| **Auto-Delete**    | `false`            | Queue survives consumer disconnects |
| **Prefetch Count** | `5`                | Process multiple messages (burst)   |

#### Image Worker Queue Specifications

| Property           | Value              | Purpose                             |
| ------------------ | ------------------ | ----------------------------------- |
| **Exchange**       | `image-tasks`      | Direct exchange for image routing   |
| **Queue**          | `image-processing` | Image processing queue              |
| **Routing Key**    | `image.process`    | Image message routing identifier    |
| **Durability**     | `true`             | Persist messages to disk            |
| **Auto-Delete**    | `false`            | Queue survives consumer disconnects |
| **Prefetch Count** | `1`                | Process one message (resource mgmt) |

#### Dead Letter Queue Configuration

| Worker Type | DLX Exchange      | DLQ Queue              | Message TTL |
| ----------- | ----------------- | ---------------------- | ----------- |
| **Email**   | `email-tasks.dlx` | `email-processing.dlq` | `1 hour`    |
| **Image**   | `image-tasks.dlx` | `image-processing.dlq` | `6 hours`   |

### 2. Queue Service (Indirect)

**Connection Type**: Indirect via RabbitMQ
**Purpose**: Receives messages published by Queue Service

#### Message Flow

```
Queue Service → RabbitMQ Exchange → Worker-Specific Queue → Specialized Worker
                      ↓
            ┌─────────────────────────┼─────────────────────────┐
            ↓                         ↓                         ↓
      email-tasks                image-tasks              [future-tasks]
            ↓                         ↓                         ↓
   email-processing            image-processing          [future-processing]
            ↓                         ↓                         ↓
      Email Worker              Image Worker              [Future Workers]
```

## HTTP Interface Endpoints

### 1. Health Check Endpoints (Per Worker)

#### Email Worker Health Check

```http
GET /health
Host: email-worker:8080 (K8s) | localhost:8081 (local)
```

**Response**:

```json
{
  "status": "ok",
  "ready": true,
  "worker_type": "email",
  "queue": "email-processing",
  "timestamp": "2024-12-19T10:30:00Z"
}
```

#### Image Worker Health Check

```http
GET /health
Host: image-worker:8080 (K8s) | localhost:8082 (local)
```

**Response**:

```json
{
  "status": "ok",
  "ready": true,
  "worker_type": "image",
  "queue": "image-processing",
  "timestamp": "2024-12-19T10:30:00Z"
}
```

### 2. Readiness Probe Endpoints

#### Readiness Check (Both Workers)

```http
GET /ready
```

**Response (Healthy)**:

```json
{
  "ready": true,
  "checks": {
    "rabbitmq_connection": "healthy",
    "queue_accessible": "healthy",
    "processor_ready": "healthy"
  }
}
```

**Response (Unhealthy)**:

```json
{
  "ready": false,
  "checks": {
    "rabbitmq_connection": "unhealthy",
    "queue_accessible": "timeout",
    "processor_ready": "healthy"
  }
}
```

### 3. Metrics Endpoints (Prometheus)

#### Metrics Collection (Per Worker)

```http
GET /metrics
```

**Key Metrics Exposed**:

```prometheus
# Email Worker Metrics
worker_messages_processed_total{worker_type="email"} 1234
worker_processing_duration_seconds{worker_type="email"} 0.5
worker_errors_total{worker_type="email"} 5
worker_queue_depth{queue="email-processing"} 10

# Image Worker Metrics
worker_messages_processed_total{worker_type="image"} 456
worker_processing_duration_seconds{worker_type="image"} 2.3
worker_errors_total{worker_type="image"} 2
worker_queue_depth{queue="image-processing"} 3
```

## Message Consumption Interface

### 1. Email Message Format

#### Expected Message Structure

```json
{
  "id": "123e4567-e89b-12d3-a456-426614174000",
  "type": "email",
  "timestamp": "2024-12-19T10:30:00Z",
  "correlation_id": "req-456",
  "payload": {
    "user_id": "user123",
    "email_type": "welcome",
    "recipient": "user@example.com",
    "template": "welcome_template",
    "data": {
      "user_name": "John Doe",
      "activation_link": "https://app.com/activate/xyz"
    },
    "priority": "high"
  },
  "metadata": {
    "source_service": "profile-service",
    "retry_count": "0",
    "correlation_id": "req-456"
  }
}
```

#### Email Processing Types

| Email Type     | Description                | Priority | Processing Time |
| -------------- | -------------------------- | -------- | --------------- |
| `welcome`      | New user welcome emails    | high     | ~100ms          |
| `notification` | General notifications      | normal   | ~500ms          |
| `alert`        | System alerts and warnings | low      | ~1s             |

### 2. Image Message Format

#### Expected Message Structure

```json
{
  "id": "123e4567-e89b-12d3-a456-426614174000",
  "type": "image",
  "timestamp": "2024-12-19T10:30:00Z",
  "correlation_id": "req-789",
  "payload": {
    "user_id": "user123",
    "image_url": "https://storage.com/images/original.jpg",
    "processing_type": "resize",
    "parameters": {
      "width": 800,
      "height": 600,
      "quality": 85,
      "format": "jpeg"
    },
    "callback_url": "https://api.com/callback/image-processed",
    "priority": "normal"
  },
  "metadata": {
    "source_service": "media-service",
    "retry_count": "0",
    "correlation_id": "req-789"
  }
}
```

#### Image Processing Types

| Processing Type | Description                 | Priority | Processing Time |
| --------------- | --------------------------- | -------- | --------------- |
| `resize`        | Image resizing operations   | normal   | ~2s             |
| `filter`        | Apply filters and effects   | normal   | ~3s             |
| `analyze`       | Image analysis and metadata | low      | ~4s             |

## Scaling and Performance Interface

### 1. Horizontal Pod Autoscaler (HPA) Configuration

#### Email Worker Scaling

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: email-worker-hpa
spec:
  scaleTargetRef:
    name: email-worker
  minReplicas: 2
  maxReplicas: 15
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          averageUtilization: 60 # Aggressive scaling
```

#### Image Worker Scaling

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: image-worker-hpa
spec:
  scaleTargetRef:
    name: image-worker
  minReplicas: 1
  maxReplicas: 8
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          averageUtilization: 70 # Conservative scaling
```

### 2. Resource Allocation Patterns

#### Email Worker Resources

```yaml
resources:
  requests:
    cpu: 50m # Low CPU for I/O operations
    memory: 64Mi # Moderate memory usage
  limits:
    cpu: 200m
    memory: 256Mi
```

#### Image Worker Resources

```yaml
resources:
  requests:
    cpu: 500m # High CPU for processing
    memory: 512Mi # High memory for image data
  limits:
    cpu: 1000m
    memory: 1Gi
```

### 3. Performance Targets

| Worker Type | Throughput Target | Latency Target | Scaling Response |
| ----------- | ----------------- | -------------- | ---------------- |
| **Email**   | 100+ msgs/sec     | 100ms-1s       | < 30 seconds     |
| **Image**   | 10-20 msgs/sec    | 2-4 seconds    | < 60 seconds     |

## Monitoring and Observability Interface

### 1. ServiceMonitor Configuration

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: workers-metrics
spec:
  selector:
    matchLabels:
      component: worker
  endpoints:
    - port: http
      path: /metrics
      interval: 30s
```

### 2. PrometheusRule Alerts

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: worker-alerts
spec:
  groups:
    - name: worker.rules
      rules:
        - alert: WorkerHighErrorRate
          expr: rate(worker_errors_total[5m]) > 0.1
          labels:
            severity: warning
          annotations:
            summary: "High error rate in {{ $labels.worker_type }} worker"

        - alert: WorkerQueueBacklog
          expr: worker_queue_depth > 100
          labels:
            severity: warning
          annotations:
            summary: "Queue backlog in {{ $labels.queue }}"
```

## Testing and Validation Interface

### 1. Local Testing Environment

#### Docker Compose Services

```yaml
services:
  email-worker:
    build: ./services/workers/email-worker
    ports:
      - "8081:8080"
    environment:
      - RABBITMQ_HOST=rabbitmq
      - QUEUE_NAME=email-processing

  image-worker:
    build: ./services/workers/image-worker
    ports:
      - "8082:8080"
    environment:
      - RABBITMQ_HOST=rabbitmq
      - QUEUE_NAME=image-processing
```

#### Test Commands

```bash
# Start complete environment
./scripts/dev-start.sh

# Health check both workers
curl http://localhost:8081/health  # Email worker
curl http://localhost:8082/health  # Image worker

# Publish test messages
./infrastructure/rabbitmq/example-publishers/publish-test-messages.sh
```

### 2. Kubernetes Testing Environment

#### Deployment Commands

```bash
# Deploy complete multi-worker architecture
./scripts/k8s-deploy.sh

# Check worker status
kubectl get pods -n workers

# Monitor scaling
watch kubectl get hpa -n workers
```

#### Expected Deployment Results

```bash
# Namespaces
kubectl get namespaces
# Expected: rabbitmq, workers

# Pods
kubectl get pods -n workers
# Expected:
# email-worker-xxx (2 replicas)
# image-worker-xxx (1 replica)

# Services
kubectl get services -n workers
# Expected:
# email-worker-service
# image-worker-service
```

## Integration Patterns

### 1. Message Processing Flow

```
Queue Service → Publish with routing key
                      ↓
RabbitMQ → Route to appropriate exchange
                      ↓
Worker-specific queue → Consumed by specialized worker
                      ↓
Business logic processing → Mock external service calls
                      ↓
Message acknowledgment → Success/retry handling
```

### 2. Error Handling Pattern

```
Message processing error → Log error with worker type
                        ↓
Increment error metrics → Return error to trigger requeue
                        ↓
Dead letter queue → After max retries exceeded
```

### 3. Graceful Shutdown Pattern

```
SIGTERM received → Stop accepting new messages
                 ↓
Complete current message processing → Close RabbitMQ connection
                                   ↓
Shutdown HTTP server → Exit gracefully
```

## Related Documentation

- `README.md` - Service overview and multi-worker architecture
- `CONTEXT.md` - Technical implementation details and patterns
- `TRACKER.md` - Implementation progress and task completion
- `IMPLEMENTATION_HISTORY.md` - Complete development history

---

**Status**: 🎉 **MULTI-WORKER ARCHITECTURE COMPLETE** - Email and image workers fully implemented with independent scaling and specialized processing capabilities.
