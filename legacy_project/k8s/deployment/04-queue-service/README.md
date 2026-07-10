# Queue Service – Kubernetes Deployment

**Service**: RabbitMQ-backed asynchronous message processing service  
**Port**: NodePort 30084 (HTTP), 15672 (Management UI)  
**Dependencies**: RabbitMQ StatefulSet  
**Technology**: Node.js/Go with RabbitMQ 3.x backend

---

## 🧱 Components

| Resource        | Description                                |
| --------------- | ------------------------------------------ |
| **Deployment**  | Queue service app running on port 8080     |
| **Service**     | NodePort 30084 for external API access     |
| **ConfigMap**   | RabbitMQ and routing configuration         |
| **Secret**      | RabbitMQ credentials and API keys          |
| **StatefulSet** | RabbitMQ 3.x with persistent storage (2Gi) |

## 🔁 Dependencies

- **RabbitMQ StatefulSet**: Primary message broker for async processing
- **Storage Class**: `standard` for RabbitMQ persistent volumes
- **No upstream services**: Messaging backbone service

## 🚀 Deployment

### Quick Deploy

```bash
# Deploy all components
kubectl apply -f .

# Wait for readiness
kubectl wait --for=condition=Available deployment/queue-service --timeout=300s
kubectl wait --for=condition=Ready pod/rabbitmq-0 --timeout=300s
```

### Step-by-Step Deploy

```bash
# 1. Deploy RabbitMQ backend first
kubectl apply -f rabbitmq-statefulset.yaml
kubectl wait --for=condition=Ready pod/rabbitmq-0 --timeout=300s

# 2. Verify RabbitMQ is operational
kubectl exec rabbitmq-0 -- rabbitmq-diagnostics ping

# 3. Deploy application components
kubectl apply -f configmap.yaml
kubectl apply -f secrets.yaml
kubectl apply -f deployment.yaml
kubectl apply -f service.yaml

# 4. Verify deployment
kubectl get pods -l app=queue-service
kubectl get pods -l app=rabbitmq
```

## 🔍 Verification

### Health Checks

```bash
# Basic health check
curl http://localhost:30084/health

# RabbitMQ connectivity check
kubectl exec rabbitmq-0 -- rabbitmq-diagnostics ping

# Queue service to RabbitMQ connection
kubectl logs deployment/queue-service | grep -i "connected to rabbitmq"
```

### Message Testing

```bash
# Publish email task
curl -X POST http://localhost:30084/api/v1/queues/publish \
  -H "Content-Type: application/json" \
  -d '{
    "routing_key": "email.send",
    "payload": {
      "user_id": "123",
      "template": "welcome",
      "recipient": "user@example.com"
    },
    "metadata": {
      "source": "test",
      "timestamp": "'$(date -Iseconds)'"
    }
  }'

# Publish image processing task
curl -X POST http://localhost:30084/api/v1/queues/publish \
  -H "Content-Type: application/json" \
  -d '{
    "routing_key": "image.process",
    "payload": {
      "user_id": "123",
      "operation": "resize",
      "dimensions": "800x600"
    }
  }'

# Check queue status
curl http://localhost:30084/api/v1/queues/status
```

### RabbitMQ Management UI

```bash
# Forward management port
kubectl port-forward rabbitmq-0 15672:15672

# Access: http://localhost:15672
# Username: guest, Password: guest
```

## 📊 Monitoring

### Metrics Endpoints

- **Health Status**: `http://localhost:30084/health`
- **Queue Status**: `http://localhost:30084/api/v1/queues/status`
- **RabbitMQ Management**: `http://localhost:15672` (via port-forward)

### Key Metrics

- Message publish/consume rates
- Queue depths and message counts
- RabbitMQ connection status
- Dead letter queue monitoring

### RabbitMQ Monitoring

```bash
# Check queue status
kubectl exec rabbitmq-0 -- rabbitmqctl list_queues

# Check exchanges
kubectl exec rabbitmq-0 -- rabbitmqctl list_exchanges

# Check consumers
kubectl exec rabbitmq-0 -- rabbitmqctl list_consumers
```

## 🔧 Configuration

### Environment Variables

| Variable            | Description           | Default            |
| ------------------- | --------------------- | ------------------ |
| `RABBITMQ_HOST`     | RabbitMQ hostname     | `rabbitmq-service` |
| `RABBITMQ_PORT`     | RabbitMQ AMQP port    | `5672`             |
| `RABBITMQ_USER`     | RabbitMQ username     | `queue_user`       |
| `RABBITMQ_PASSWORD` | RabbitMQ password     | From secret        |
| `RABBITMQ_VHOST`    | RabbitMQ virtual host | `/`                |
| `SERVER_PORT`       | HTTP server port      | `8080`             |

### Message Routing

- **Email Queue**: `email.*` routing keys → Email Worker
- **Image Queue**: `image.*` routing keys → Image Worker
- **Profile Queue**: `profile.*` routing keys → Profile Worker
- **Dead Letter Exchange**: Failed messages with TTL

### Resource Limits

- **Queue Service**: 250m CPU request, 600m limit; 256Mi memory request, 512Mi limit
- **RabbitMQ**: 300m CPU request, 500m limit; 512Mi memory request, 1Gi limit

## 🚨 Troubleshooting

### Common Issues

**RabbitMQ Disk Space Alarm**

```bash
# Check RabbitMQ status
kubectl exec rabbitmq-0 -- rabbitmq-diagnostics status

# Check disk space
kubectl exec rabbitmq-0 -- df -h
kubectl exec rabbitmq-0 -- rabbitmq-diagnostics disk_free

# Fix disk space alarm (Kind-specific)
kubectl exec rabbitmq-0 -- rabbitmqctl set_disk_free_limit 0.1
```

**Message Flow Issues**

```bash
# Check message routing
kubectl exec rabbitmq-0 -- rabbitmqctl list_queues name messages
kubectl exec rabbitmq-0 -- rabbitmqctl list_bindings

# Monitor message flow
kubectl logs deployment/queue-service -f | grep -i message

# Check for dead letter messages
kubectl exec rabbitmq-0 -- rabbitmqctl list_queues | grep dlx
```

**Connection Issues**

```bash
# Check RabbitMQ connectivity from queue service
kubectl exec deployment/queue-service -- nc -zv rabbitmq-service 5672

# Check RabbitMQ user permissions
kubectl exec rabbitmq-0 -- rabbitmqctl list_user_permissions queue_user

# Verify RabbitMQ configuration
kubectl exec rabbitmq-0 -- cat /etc/rabbitmq/rabbitmq.conf
```

## 🔒 Security

### Security Context

- **Non-root user**: UID 65534 (nobody)
- **Read-only filesystem**: Enabled where possible
- **No privilege escalation**: Enforced
- **Capabilities dropped**: ALL

### RabbitMQ Security

- **User Authentication**: Dedicated `queue_user` with limited permissions
- **Virtual Host Isolation**: Separate vhost for microservices
- **Network Isolation**: RabbitMQ only accessible from queue service and workers
- **Management UI**: Secured with default credentials (guest/guest)

### Network Policies

- Ingress: Allows traffic from profile service, worker services, and monitoring
- Egress: Allows DNS and RabbitMQ communication only

## 📋 Service Information

| Aspect        | Details                                |
| ------------- | -------------------------------------- |
| **Namespace** | `default`                              |
| **Labels**    | `app=queue-service`, `component=queue` |
| **Replicas**  | 1 (Kind optimized)                     |
| **Strategy**  | RollingUpdate                          |
| **Protocol**  | HTTP (8080), AMQP (5672)               |

## 🐰 RabbitMQ Information

| Aspect                  | Details               |
| ----------------------- | --------------------- |
| **Version**             | RabbitMQ 3.x          |
| **Storage**             | 2Gi persistent volume |
| **Management UI**       | Port 15672            |
| **Default Credentials** | guest/guest           |
| **Disk Free Limit**     | 10% (Kind optimized)  |

## 🔗 API Endpoints

| Endpoint                          | Method | Description        |
| --------------------------------- | ------ | ------------------ |
| `/health`                         | GET    | Health check       |
| `/api/v1/queues/publish`          | POST   | Publish message    |
| `/api/v1/queues/status`           | GET    | Queue status       |
| `/api/v1/queues/{queue}/messages` | GET    | Get queue messages |

## 📨 Message Routing Patterns

### Routing Keys

- `email.send` → Email processing queue
- `email.bulk` → Bulk email processing
- `image.process` → Image processing queue
- `image.upload` → Image upload processing
- `profile.update` → Profile update notifications
- `profile.analytics` → Analytics processing

### Message Structure

```json
{
  "routing_key": "email.send",
  "payload": {
    "user_id": "123",
    "data": "message-specific-data"
  },
  "metadata": {
    "source": "service-name",
    "timestamp": "2024-12-29T10:00:00Z",
    "correlation_id": "uuid"
  }
}
```

---

**Status**: ✅ Production-ready for Kind deployment  
**Last Updated**: December 29, 2024  
**Documentation**: See [Services Deployment Guide](../../SERVICES_DEPLOYMENT_GUIDE.md#queue-service-guide)
