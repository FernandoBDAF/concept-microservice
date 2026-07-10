# Profile Service – Kubernetes Deployment

**Service**: Profile management orchestrator service with user management capabilities  
**Port**: NodePort 30085 (HTTP)  
**Dependencies**: Auth, Cache, Storage, Queue services  
**Technology**: Node.js/Go orchestration service with full integration and enhanced auth client

---

## 🧱 Components

| Resource       | Description                                                     |
| -------------- | --------------------------------------------------------------- |
| **Deployment** | Profile service app running on port 8080                        |
| **Service**    | NodePort 30085 for external access                              |
| **ConfigMap**  | Service integration, user management, and business logic config |
| **Secret**     | API keys for all service dependencies                           |

## 🔁 Dependencies

- **Auth Service**: Required for JWT token validation, user authentication, and user management operations
- **Cache Service**: Required for profile caching, session management, and performance optimization
- **Storage Service**: Required for persistent profile data storage (profiles only)
- **Queue Service**: Required for asynchronous task processing
- **All backend services**: Redis, PostgreSQL, RabbitMQ (indirect dependencies)

## 🚨 Critical Requirements

### Docker Image Build Required

```bash
# MUST build and load custom Docker image before deployment
cd services/profile-service/
docker build -t profile-service:latest .
kind load docker-image profile-service:latest --name microservices-kind

# Verify image is loaded
docker exec -it microservices-kind-control-plane crictl images | grep profile-service
```

## 🚀 Deployment

### Quick Deploy

```bash
# 1. Build and load Docker image (CRITICAL)
cd services/profile-service/
docker build -t profile-service:latest .
kind load docker-image profile-service:latest --name microservices-kind

# 2. Deploy all components
cd ../../k8s/deployment/05-profile-service/
kubectl apply -f .

# 3. Wait for readiness
kubectl wait --for=condition=Available deployment/profile-service --timeout=300s
```

### Step-by-Step Deploy

```bash
# 1. Verify all dependencies are running
kubectl get pods -l app=auth-service
kubectl get pods -l app=cache-service
kubectl get pods -l app=storage-service
kubectl get pods -l app=queue-service

# 2. Test dependency connectivity
for port in 30081 30082 30083 30084; do
  echo "Testing service on port $port:"
  curl -s http://localhost:$port/health | jq .status
done

# 3. Build and load Docker image
cd services/profile-service/
docker build -t profile-service:latest .
kind load docker-image profile-service:latest --name microservices-kind

# 4. Deploy profile service
cd ../../k8s/deployment/05-profile-service/
kubectl apply -f secrets.yaml
kubectl apply -f configmap.yaml
kubectl apply -f deployment.yaml
kubectl apply -f service.yaml

# 5. Verify deployment and integration
kubectl logs deployment/profile-service | grep -E "(auth|cache|storage|queue).*connected"
```

## 🔍 Verification

### Health Checks

```bash
# Basic health check
curl http://localhost:30085/health
# Expected: All dependencies should show as "connected"

# Check service integrations
kubectl exec deployment/profile-service -- curl -s http://auth-service:8080/health
kubectl exec deployment/profile-service -- curl -s http://cache-service:8080/health
kubectl exec deployment/profile-service -- curl -s http://storage-service:8080/health
kubectl exec deployment/profile-service -- curl -s http://queue-service:8080/health
```

### Integration Testing

```bash
# Complete authentication flow
# 1. Create user via Profile Service
curl -X POST http://localhost:30085/api/v1/profiles \
  -H "Content-Type: application/json" \
  -d '{
    "email": "integration@test.com",
    "password": "testpass123",
    "first_name": "Integration",
    "last_name": "Test"
  }'

# 2. Login via Auth Service integration
curl -X POST http://localhost:30085/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "integration@test.com",
    "password": "testpass123"
  }'

# 3. Use token for authenticated operations
TOKEN="<jwt-token-from-response>"
curl -X GET http://localhost:30085/api/v1/profiles/me \
  -H "Authorization: Bearer $TOKEN"
```

### Cache Integration Testing

```bash
# Profile caching workflow
curl -X GET http://localhost:30085/api/v1/profiles/123 \
  -H "Authorization: Bearer $TOKEN"
# First call: Cache miss, fetches from storage
# Second call: Cache hit, faster response

# Cache invalidation
curl -X PUT http://localhost:30085/api/v1/profiles/123 \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"first_name": "Updated"}'
# Should invalidate cache and update storage
```

## 📊 Monitoring

### Metrics Endpoints

- **Health Status**: `http://localhost:30085/health`
- **Service Dependencies**: Included in health endpoint
- **Business Metrics**: Profile operations, cache hit rates

### Key Metrics

- Profile CRUD operation rates
- Authentication success rates
- Cache hit/miss ratios
- Queue task publishing rates
- Service dependency health status

## 🔧 Configuration

### Environment Variables

| Variable                  | Description              | Default                       |
| ------------------------- | ------------------------ | ----------------------------- |
| `AUTH_SERVICE_URL`        | Auth service endpoint    | `http://auth-service:8080`    |
| `CACHE_SERVICE_URL`       | Cache service endpoint   | `http://cache-service:8080`   |
| `STORAGE_SERVICE_URL`     | Storage service endpoint | `http://storage-service:8080` |
| `QUEUE_SERVICE_URL`       | Queue service endpoint   | `http://queue-service:8080`   |
| `AUTH_SERVICE_API_KEY`    | Auth service API key     | From secret                   |
| `CACHE_SERVICE_API_KEY`   | Cache service API key    | From secret                   |
| `STORAGE_SERVICE_API_KEY` | Storage service API key  | From secret                   |
| `QUEUE_SERVICE_API_KEY`   | Queue service API key    | From secret                   |
| `SERVER_PORT`             | HTTP server port         | `8080`                        |

### Business Logic Configuration

- **Profile Caching**: Cache-aside pattern implementation
- **Authentication**: JWT token validation on all endpoints
- **Async Processing**: Queue integration for background tasks
- **Data Consistency**: Coordinated updates across cache and storage

### Resource Limits

- **CPU**: 400m request, 1200m limit
- **Memory**: 512Mi request, 1Gi limit

## 🚨 Troubleshooting

### Common Issues

**Docker Image Issues**

```bash
# Check if image exists in Kind cluster
docker exec -it microservices-kind-control-plane crictl images | grep profile-service

# If missing, rebuild and reload
cd services/profile-service/
docker build -t profile-service:latest .
kind load docker-image profile-service:latest --name microservices-kind

# Force pod restart
kubectl rollout restart deployment/profile-service
```

**Service Integration Issues**

```bash
# Check dependency service health
for service in auth-service cache-service storage-service queue-service; do
  echo "Testing $service:"
  kubectl exec deployment/profile-service -- nc -zv $service 8080
done

# Check API key configuration
kubectl get secret profile-service-secrets -o yaml
kubectl logs deployment/profile-service | grep -i "api.*key"
```

**Authentication Flow Issues**

```bash
# Debug JWT token handling
kubectl logs deployment/profile-service | grep -i jwt

# Test auth service integration
kubectl exec deployment/profile-service -- curl -s http://auth-service:8080/health

# Check JWT secret configuration
kubectl logs deployment/profile-service | grep -i "jwt.*secret"
```

## 🔒 Security

### Security Context

- **Non-root user**: UID 65534 (nobody)
- **Read-only filesystem**: Enabled
- **No privilege escalation**: Enforced
- **Capabilities dropped**: ALL

### Integration Security

- **API Key Authentication**: Secure communication with all services
- **JWT Token Validation**: All endpoints require valid tokens
- **Network Policies**: Restricted communication patterns
- **Secrets Management**: All API keys stored in Kubernetes secrets

### Network Policies

- Ingress: Allows traffic from ingress controller and monitoring
- Egress: Allows DNS and communication with auth, cache, storage, queue services only

## 📋 Service Information

| Aspect        | Details                                         |
| ------------- | ----------------------------------------------- |
| **Namespace** | `default`                                       |
| **Labels**    | `app=profile-service`, `component=orchestrator` |
| **Replicas**  | 1 (Kind optimized)                              |
| **Strategy**  | RollingUpdate                                   |
| **Image**     | Custom built (profile-service:latest)           |

## 🔗 API Endpoints

| Endpoint                 | Method | Description              | Auth Required |
| ------------------------ | ------ | ------------------------ | ------------- |
| `/health`                | GET    | Health check             | No            |
| `/api/v1/profiles`       | POST   | Create profile           | Yes           |
| `/api/v1/profiles`       | GET    | List profiles            | Yes           |
| `/api/v1/profiles/{id}`  | GET    | Get profile              | Yes           |
| `/api/v1/profiles/{id}`  | PUT    | Update profile           | Yes           |
| `/api/v1/profiles/{id}`  | DELETE | Delete profile           | Yes           |
| `/api/v1/profiles/me`    | GET    | Get current user profile | Yes           |
| `/api/v1/auth/login`     | POST   | Login (proxy to auth)    | No            |
| `/api/v1/profiles/tasks` | POST   | Trigger async task       | Yes           |

## 🔄 Integration Patterns

### Cache-Aside Pattern

1. **Read**: Check cache → If miss, read from storage → Update cache
2. **Write**: Update storage → Invalidate cache → Next read repopulates cache
3. **Delete**: Remove from storage → Remove from cache

### Authentication Flow

1. **Token Validation**: Validate JWT with auth service
2. **User Context**: Extract user info from token
3. **Authorization**: Check user permissions for requested operation
4. **Business Logic**: Execute profile operations with user context

### Async Task Processing

1. **Task Creation**: Profile operations trigger background tasks
2. **Queue Publishing**: Send tasks to appropriate queues via queue service
3. **Worker Processing**: Workers consume and process tasks
4. **Result Handling**: Handle task completion notifications

---

**Status**: ✅ Production-ready for Kind deployment  
**Last Updated**: December 29, 2024  
**Documentation**: See [Services Deployment Guide](../../SERVICES_DEPLOYMENT_GUIDE.md#profile-service-guide)
