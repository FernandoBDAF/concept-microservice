# Profile Service

## Overview

Profile Service serves as the primary entry point and orchestrator for the microservices task processing ecosystem. It handles profile operations and coordinates with multiple worker types through the queue-service.

## Architecture

The service integrates with the upgraded queue-service and multi-worker architecture, supporting three task types:

- **Profile tasks** (`profile.task`) - Profile updates, deletions, data synchronization
- **Email tasks** (`email.send`) - Welcome emails, notifications, alerts
- **Image tasks** (`image.process`) - Image resizing, format conversion, optimization

## Deployment Approaches

This service supports **two complementary deployment approaches**:

### 🔍 **Manual Deployment** (Analysis & Learning)

**Purpose**: Step-by-step analysis and understanding  
**Best for**: Learning, troubleshooting, detailed inspection

```bash
# Step-by-step manual deployment with analysis
cd deployments/scripts
./manual-deploy.sh --analyze

# Interactive deployment with prompts
./manual-deploy.sh --step-by-step

# Manual cleanup
./manual-cleanup.sh --step-by-step
```

**🎯 Smart Environment Detection**: The manual script automatically detects your cluster:

- **Kind clusters**: 1 replica, reduced resources, local secrets, debug logging
- **Production clusters**: 3 replicas, production resources, production secrets

### ⚡ **Kustomize Deployment** (Operations & Automation)

**Purpose**: Regular, consistent operations  
**Best for**: Daily operations, CI/CD, production deployments

```bash
# Quick kustomize deployment
cd deployments/kind
kubectl apply -k .

# Or using deployment script
./deploy-to-kind.sh
```

## When to Use Each Approach

| Scenario                  | Manual | Kustomize | Reason                             |
| ------------------------- | ------ | --------- | ---------------------------------- |
| **First deployment**      | ✅     | ❌        | Learn components step-by-step      |
| **Troubleshooting**       | ✅     | ❌        | Analyze each manifest individually |
| **Learning/Training**     | ✅     | ❌        | Understand Kubernetes resources    |
| **Daily development**     | ❌     | ✅        | Speed and consistency              |
| **CI/CD pipelines**       | ❌     | ✅        | Automation and reliability         |
| **Production deployment** | ❌     | ✅        | Consistency and safety             |
| **Problem diagnosis**     | ✅     | ❌        | Step-by-step analysis              |

## Quick Start

### Manual Approach (Recommended for First Time)

```bash
# 1. Understand each component step-by-step
cd deployments/scripts
./manual-deploy.sh --analyze

# 2. View detailed deployment guide
cat ../STEP_BY_STEP_DEPLOYMENT_GUIDE.md

# 3. Clean up when done
./manual-cleanup.sh
```

### Kustomize Approach (Recommended for Regular Use)

```bash
# 1. Quick deployment
cd deployments/kind
kubectl apply -k .

# 2. Check status
kubectl get pods -l app=profile-service

# 3. View logs
kubectl logs -l app=profile-service --tail=50 -f
```

## Multi-Worker Architecture

### Message Format Specification

The service uses queue-service compatible message format:

```go
type QueueMessage struct {
    ID         string            `json:"id"`
    Type       string            `json:"type"`
    Payload    json.RawMessage   `json:"payload"`
    Timestamp  time.Time         `json:"timestamp"`
    Metadata   map[string]string `json:"metadata"`
    RoutingKey string            `json:"routing_key"`
}
```

### Routing Key Determination

Task types are automatically mapped to appropriate routing keys:

```go
var RoutingKeyMap = map[string]string{
    "profile_update":     "profile.task",    // → Profile Worker
    "email_notification": "email.send",      // → Email Worker
    "image_processing":   "image.process",   // → Image Worker
}
```

### API Endpoints

#### Task Submission

```bash
POST /api/v1/profiles/:id/tasks
Content-Type: application/json

{
  "type": "profile_update",
  "payload": {
    "action": "update",
    "data": {...}
  }
}
```

#### Supported Task Types

1. **Profile Processing Tasks**:

   ```bash
   curl -X POST http://profile-service/api/v1/profiles/123/tasks \
     -H "Content-Type: application/json" \
     -d '{"type": "profile_update", "payload": {"action": "update"}}'
   ```

2. **Email Notification Tasks**:

   ```bash
   curl -X POST http://profile-service/api/v1/profiles/123/tasks \
     -H "Content-Type: application/json" \
     -d '{"type": "email_notification", "payload": {"to": "user@example.com", "template": "welcome"}}'
   ```

3. **Image Processing Tasks**:
   ```bash
   curl -X POST http://profile-service/api/v1/profiles/123/tasks \
     -H "Content-Type: application/json" \
     -d '{"type": "image_processing", "payload": {"image_url": "https://example.com/image.jpg", "operation": "resize"}}'
   ```

## Integration Patterns

### Queue-Service Integration

- **Communication Method**: HTTP API (not direct RabbitMQ)
- **Message Format**: JSON with standardized structure
- **Routing Strategy**: Automatic routing key determination based on task type
- **Error Handling**: Circuit breaker pattern with retry logic

### Worker Integration

- **Profile Worker**: Handles `profile.task` routing key
- **Email Worker**: Handles `email.send` routing key
- **Image Worker**: Handles `image.process` routing key

## Development

### Prerequisites

- Go 1.21+
- Kind (for local development)
- kubectl
- Docker

### Local Development Setup

```bash
# 1. Clone and setup
git clone <repository>
cd profile-service

# 2. Build Docker image
docker build -t profile-service:latest .

# 3. Deploy to kind cluster
cd deployments/kind
./deploy-to-kind.sh
```

### Testing

```bash
# Run all tests
go test ./...

# Run integration tests
go test ./test/integration/...

# Run performance tests
go test ./test/performance/...
```

## Monitoring & Observability

### Metrics

- Task submission rates by type
- Routing key distribution
- Queue service communication latency
- Error rates and response times

### Health Endpoints

- `GET /health` - Service health check
- `GET /metrics` - Prometheus metrics

### Logging

- Structured JSON logging
- Routing key context in all logs
- Task type distribution logging

## Configuration

### Environment Variables

#### Core Configuration

- `SERVER_HOST` - Server bind address (default: "0.0.0.0")
- `SERVER_PORT` - Server bind port (default: 8080)
- `LOG_LEVEL` - Logging level (default: "info")
- `LOG_FORMAT` - Log format (default: "json")

#### Queue Service Integration

- `QUEUE_SERVICE_URL` - Queue service endpoint
- `QUEUE_SERVICE_TIMEOUT` - Request timeout (default: "30s")
- `QUEUE_SERVICE_RETRIES` - Retry attempts (default: 3)

#### Task Configuration

- `PROFILE_TASK_TIMEOUT` - Profile task timeout (default: "5m")
- `EMAIL_TASK_TIMEOUT` - Email task timeout (default: "2m")
- `IMAGE_TASK_TIMEOUT` - Image task timeout (default: "10m")

#### Routing Keys

- `ROUTING_KEY_PROFILE_UPDATE` - Profile task routing key
- `ROUTING_KEY_EMAIL_NOTIFICATION` - Email task routing key
- `ROUTING_KEY_IMAGE_PROCESSING` - Image task routing key

## Production Deployment

For production deployment, use the production-ready manifests:

```bash
# Apply production manifests
kubectl apply -f deployments/kubernetes/
kubectl apply -f deployments/monitoring/
```

See `deployments/DEPLOYMENT.md` for comprehensive deployment documentation.

## Documentation

- [`DEPLOYMENT.md`](deployments/DEPLOYMENT.md) - Complete deployment guide
- [`STEP_BY_STEP_DEPLOYMENT_GUIDE.md`](deployments/STEP_BY_STEP_DEPLOYMENT_GUIDE.md) - Step-by-step deployment tutorial
- [`INTERFACE.md`](INTERFACE.md) - API specifications and integration contracts
- [`CONTEXT.md`](CONTEXT.md) - Technical implementation details and patterns
- [`TRACKER.md`](TRACKER.md) - Implementation progress tracking
