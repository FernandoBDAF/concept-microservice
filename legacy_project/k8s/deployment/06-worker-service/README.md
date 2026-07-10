# Worker Service – Kubernetes Deployment

**Service**: Multi-worker asynchronous task processing service  
**Port**: NodePort 30086 (HTTP monitoring)  
**Dependencies**: Queue Service (RabbitMQ)  
**Technology**: Go-based specialized workers with message routing

---

## 🧱 Components

| Resource          | Description                                |
| ----------------- | ------------------------------------------ |
| **Deployment**    | Email worker for email processing tasks    |
| **Deployment**    | Image worker for image processing tasks    |
| **Service**       | NodePort 30086 for monitoring/health       |
| **ConfigMap**     | Worker configuration and routing rules     |
| **Secret**        | RabbitMQ credentials and worker API keys   |
| **NetworkPolicy** | Security policies for worker communication |

## 🔁 Dependencies

- **Queue Service**: Required for message consumption and task routing
- **RabbitMQ**: Direct dependency for AMQP message consumption
- **Network connectivity**: Must reach `rabbitmq-service:5672`

## 🚨 Critical Requirements

### Cross-Platform Binary Compilation Required

```bash
# MUST compile Linux binaries and build Docker image before deployment
cd services/worker-service/

# Build Linux binaries (required for containers)
GOOS=linux GOARCH=amd64 go build -o email-worker-linux cmd/email-worker/main.go
GOOS=linux GOARCH=amd64 go build -o image-worker-linux cmd/image-worker/main.go

# Verify binaries are Linux x86-64
file email-worker-linux  # Should show: Linux x86-64 executable

# Build and load Docker image
docker build -t worker-service:latest .
kind load docker-image worker-service:latest --name microservices-kind

# Verify image is loaded
docker exec -it microservices-kind-control-plane crictl images | grep worker-service
```

## 🚀 Deployment

### Quick Deploy

```bash
# 1. Build Linux binaries and Docker image (CRITICAL)
cd services/worker-service/
GOOS=linux GOARCH=amd64 go build -o email-worker-linux cmd/email-worker/main.go
GOOS=linux GOARCH=amd64 go build -o image-worker-linux cmd/image-worker/main.go
docker build -t worker-service:latest .
kind load docker-image worker-service:latest --name microservices-kind

# 2. Deploy all components
cd ../../k8s/deployment/06-worker-service/
kubectl apply -f .

# 3. Wait for readiness
kubectl wait --for=condition=Available deployment/email-worker --timeout=300s
kubectl wait --for=condition=Available deployment/image-worker --timeout=300s
```

### Step-by-Step Deploy

```bash
# 1. Verify Queue Service is running
kubectl get pods -l app=queue-service
kubectl get pods -l app=rabbitmq
curl http://localhost:30084/health

# 2. Test RabbitMQ connectivity
kubectl exec rabbitmq-0 -- rabbitmq-diagnostics ping

# 3. Build cross-platform binaries
cd services/worker-service/
GOOS=linux GOARCH=amd64 go build -o email-worker-linux cmd/email-worker/main.go
GOOS=linux GOARCH=amd64 go build -o image-worker-linux cmd/image-worker/main.go

# 4. Build and load Docker image
docker build -t worker-service:latest .
kind load docker-image worker-service:latest --name microservices-kind

# 5. Deploy worker services
cd ../../k8s/deployment/06-worker-service/
kubectl apply -f configmap.yaml
kubectl apply -f secrets.yaml
kubectl apply -f deployment.yaml
kubectl apply -f service.yaml
kubectl apply -f network-policy.yaml

# 6. Verify deployment and connectivity
kubectl logs deployment/email-worker | grep -i "connected to rabbitmq"
kubectl logs deployment/image-worker | grep -i "connected to rabbitmq"
```

## 🔍 Verification

### Health Checks

```bash
# Basic health check
curl http://localhost:30086/health

# Check worker connectivity to RabbitMQ
kubectl exec deployment/email-worker -- nc -zv rabbitmq-service 5672
kubectl exec deployment/image-worker -- nc -zv rabbitmq-service 5672

# Verify workers are consuming from correct queues
kubectl exec rabbitmq-0 -- rabbitmqctl list_consumers
```

### Worker Testing

```bash
# Email worker testing
curl -X POST http://localhost:30084/api/v1/queues/publish \
  -H "Content-Type: application/json" \
  -d '{
    "routing_key": "email.send",
    "payload": {
      "user_id": "worker-test-123",
      "template": "welcome",
      "recipient": "test@example.com",
      "subject": "Welcome to the platform"
    },
    "metadata": {
      "source": "manual-test",
      "timestamp": "'$(date -Iseconds)'"
    }
  }'

# Verify email worker processes the message
kubectl logs deployment/email-worker --tail=20 | grep "worker-test-123"

# Image worker testing
curl -X POST http://localhost:30084/api/v1/queues/publish \
  -H "Content-Type: application/json" \
  -d '{
    "routing_key": "image.process",
    "payload": {
      "user_id": "worker-test-456",
      "operation": "resize",
      "source_url": "https://example.com/image.jpg",
      "dimensions": "800x600"
    }
  }'

# Verify image worker processes the message
kubectl logs deployment/image-worker --tail=20 | grep "worker-test-456"
```

### Multi-Worker Load Testing

```bash
# Publish multiple tasks of different types
for i in {1..5}; do
  # Email tasks
  curl -X POST http://localhost:30084/api/v1/queues/publish \
    -H "Content-Type: application/json" \
    -d "{\"routing_key\": \"email.send\", \"payload\": {\"user_id\": \"load-test-email-$i\"}}"

  # Image tasks
  curl -X POST http://localhost:30084/api/v1/queues/publish \
    -H "Content-Type: application/json" \
    -d "{\"routing_key\": \"image.process\", \"payload\": {\"user_id\": \"load-test-image-$i\"}}"
done

# Monitor processing across both workers
kubectl logs deployment/email-worker --tail=10 | grep "load-test"
kubectl logs deployment/image-worker --tail=10 | grep "load-test"
```

## 📊 Monitoring

### Metrics Endpoints

- **Health Status**: `http://localhost:30086/health`
- **Worker Status**: `http://localhost:30086/api/v1/workers/status`
- **Individual Worker Metrics**: Available via worker logs

### Key Metrics

- Message processing rates per worker type
- Queue consumption rates
- Task completion/failure rates
- Worker uptime and health status

### Worker Performance Monitoring

```bash
# Monitor worker resource usage
kubectl top pods -l app=email-worker
kubectl top pods -l app=image-worker

# Check processing rates
kubectl logs deployment/email-worker | grep "processed" | tail -10
kubectl logs deployment/image-worker | grep "processed" | tail -10

# Monitor queue depths
kubectl exec rabbitmq-0 -- rabbitmqctl list_queues name messages | grep -E "(email|image)"
```

## 🔧 Configuration

### Environment Variables

| Variable            | Description                   | Default            |
| ------------------- | ----------------------------- | ------------------ |
| `RABBITMQ_HOST`     | RabbitMQ hostname             | `rabbitmq-service` |
| `RABBITMQ_PORT`     | RabbitMQ AMQP port            | `5672`             |
| `RABBITMQ_USER`     | RabbitMQ username             | `worker_user`      |
| `RABBITMQ_PASSWORD` | RabbitMQ password             | From secret        |
| `WORKER_TYPE`       | Worker specialization         | `email` or `image` |
| `QUEUE_NAME`        | Primary queue for consumption | Worker-specific    |
| `CONCURRENCY`       | Concurrent message processing | `5`                |

### Message Routing

- **Email Worker**: Consumes `email.*` routing keys
  - `email.send` → Welcome emails, notifications
  - `email.bulk` → Bulk email processing
  - `email.template` → Template-based emails
- **Image Worker**: Consumes `image.*` routing keys
  - `image.process` → Image resizing, optimization
  - `image.upload` → Image upload processing
  - `image.thumbnail` → Thumbnail generation

### Resource Limits

- **Email Worker**: 300m CPU request, 800m limit; 256Mi memory request, 512Mi limit
- **Image Worker**: 300m CPU request, 800m limit; 256Mi memory request, 512Mi limit

## 🚨 Troubleshooting

### Common Issues

**Binary Compatibility Issues**

```bash
# Check binary architecture
kubectl exec deployment/email-worker -- file /app/email-worker-linux
# Should show: Linux x86-64 executable

# If exec format error, rebuild with correct GOOS/GOARCH
cd services/worker-service/
GOOS=linux GOARCH=amd64 go build -o email-worker-linux cmd/email-worker/main.go
GOOS=linux GOARCH=amd64 go build -o image-worker-linux cmd/image-worker/main.go
docker build -t worker-service:latest .
kind load docker-image worker-service:latest --name microservices-kind
kubectl rollout restart deployment/email-worker
kubectl rollout restart deployment/image-worker
```

**RabbitMQ Connection Issues**

```bash
# Check RabbitMQ credentials in secrets
kubectl get secret worker-service-secrets -o yaml

# Test connectivity from worker pods
kubectl exec deployment/email-worker -- nc -zv rabbitmq-service 5672

# Check RabbitMQ user permissions
kubectl exec rabbitmq-0 -- rabbitmqctl list_user_permissions worker_user

# Verify worker logs for connection status
kubectl logs deployment/email-worker | grep -E "(connection|rabbitmq)"
```

**Message Processing Issues**

```bash
# Check for dead letter queues
kubectl exec rabbitmq-0 -- rabbitmqctl list_queues | grep dlx

# Monitor message acknowledgments
kubectl logs deployment/email-worker | grep -E "(ack|nack|reject)"

# Check worker error logs
kubectl logs deployment/email-worker | grep -i error
kubectl logs deployment/image-worker | grep -i error

# Verify message routing
kubectl exec rabbitmq-0 -- rabbitmqctl list_bindings | grep -E "(email|image)"
```

## 🔒 Security

### Security Context

- **Non-root user**: UID 65534 (nobody)
- **Read-only filesystem**: Enabled where possible
- **No privilege escalation**: Enforced
- **Capabilities dropped**: ALL

### Network Security

- **Network Policies**: Restricts communication to RabbitMQ and DNS only
- **Worker Isolation**: Each worker type has dedicated labels and policies
- **Credential Management**: RabbitMQ credentials stored in Kubernetes secrets

### Network Policies

- **Ingress**: Allows traffic from monitoring systems only
- **Egress**: Allows DNS resolution and RabbitMQ communication only

## 📋 Service Information

| Aspect        | Details                                                           |
| ------------- | ----------------------------------------------------------------- |
| **Namespace** | `default`                                                         |
| **Labels**    | `app=email-worker`, `app=image-worker`, `worker-type=email/image` |
| **Replicas**  | 1 each (Kind optimized)                                           |
| **Strategy**  | RollingUpdate                                                     |
| **Image**     | Custom built (worker-service:latest)                              |

## ⚙️ Worker Types

### Email Worker

| Aspect                | Details                                           |
| --------------------- | ------------------------------------------------- |
| **Routing Keys**      | `email.*`                                         |
| **Primary Functions** | Email sending, template processing, notifications |
| **Queue**             | `email_queue`                                     |
| **Concurrency**       | 5 concurrent messages                             |

### Image Worker

| Aspect                | Details                                  |
| --------------------- | ---------------------------------------- |
| **Routing Keys**      | `image.*`                                |
| **Primary Functions** | Image processing, resizing, optimization |
| **Queue**             | `image_queue`                            |
| **Concurrency**       | 3 concurrent messages (CPU intensive)    |

## 🔄 Message Processing Flow

### Email Processing

1. **Message Reception**: Consume from `email.*` queues
2. **Template Resolution**: Load email templates
3. **Content Generation**: Generate personalized content
4. **Email Delivery**: Send via configured email provider
5. **Acknowledgment**: Confirm successful processing

### Image Processing

1. **Message Reception**: Consume from `image.*` queues
2. **Image Download**: Fetch source image
3. **Processing**: Resize, optimize, or transform
4. **Storage**: Save processed image
5. **Acknowledgment**: Confirm successful processing

---

**Status**: ✅ Production-ready for Kind deployment  
**Last Updated**: December 29, 2024  
**Documentation**: See [Services Deployment Guide](../../SERVICES_DEPLOYMENT_GUIDE.md#worker-service-guide)
