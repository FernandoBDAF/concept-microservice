# Cache Service – Kubernetes Deployment

**Service**: Redis-backed caching service  
**Port**: NodePort 30081 (HTTP), 30091 (gRPC)  
**Dependencies**: Redis StatefulSet  
**Technology**: Go/Node.js with Redis 7.x backend

---

## 🧱 Components

| Resource        | Description                                  |
| --------------- | -------------------------------------------- |
| **Deployment**  | Cache service app running on port 8080       |
| **Service**     | NodePort 30081 for external access           |
| **Service**     | Metrics service on ClusterIP for monitoring  |
| **ConfigMap**   | Application configuration and Redis settings |
| **Secret**      | Redis credentials and API keys               |
| **StatefulSet** | Redis 7.x with persistent storage (1Gi)      |

## 🔁 Dependencies

- **Redis StatefulSet**: Primary data store for caching operations
- **Storage Class**: `standard` for Redis persistent volumes
- **No upstream services**: Foundation layer service

## 🚀 Deployment

### Quick Deploy

```bash
# Deploy all components
kubectl apply -f .

# Wait for readiness
kubectl wait --for=condition=Available deployment/cache-service --timeout=300s
kubectl wait --for=condition=Ready pod/redis-0 --timeout=300s
```

### Step-by-Step Deploy

```bash
# 1. Deploy Redis backend first
kubectl apply -f redis-statefulset.yaml
kubectl wait --for=condition=Ready pod/redis-0 --timeout=300s

# 2. Deploy application components
kubectl apply -f configmap.yaml
kubectl apply -f secrets.yaml
kubectl apply -f deployment.yaml
kubectl apply -f service.yaml

# 3. Verify deployment
kubectl get pods -l app=cache-service
kubectl get pods -l app=redis
```

## 🔍 Verification

### Health Checks

```bash
# Basic health check
curl http://localhost:30081/health

# Readiness check
curl http://localhost:30081/ready

# Redis connectivity
kubectl exec redis-0 -- redis-cli ping
```

### API Testing

```bash
# Set cache entry
curl -X POST http://localhost:30081/api/v1/cache/test-key \
  -H "Content-Type: application/json" \
  -d '{"value": "test-value", "ttl": 300}'

# Get cache entry
curl -X GET http://localhost:30081/api/v1/cache/test-key

# Delete cache entry
curl -X DELETE http://localhost:30081/api/v1/cache/test-key
```

## 📊 Monitoring

### Metrics Endpoints

- **Application Metrics**: `http://cache-service-metrics:8081/metrics`
- **Health Status**: `http://localhost:30081/health`
- **Readiness**: `http://localhost:30081/ready`

### Key Metrics

- Cache hit/miss ratio
- Redis connection status
- Response times
- Memory usage

## 🔧 Configuration

### Environment Variables

| Variable         | Description           | Default         |
| ---------------- | --------------------- | --------------- |
| `REDIS_HOST`     | Redis server hostname | `redis-service` |
| `REDIS_PORT`     | Redis server port     | `6379`          |
| `REDIS_PASSWORD` | Redis authentication  | From secret     |
| `SERVER_PORT`    | HTTP server port      | `8080`          |
| `METRICS_PORT`   | Metrics server port   | `8081`          |

### Resource Limits

- **CPU**: 200m request, 500m limit
- **Memory**: 256Mi request, 512Mi limit
- **Redis**: 128Mi request, 256Mi limit

## 🚨 Troubleshooting

### Common Issues

**Pod CrashLoopBackOff**

```bash
# Check logs
kubectl logs deployment/cache-service

# Common cause: Redis not ready
kubectl get pods -l app=redis
kubectl logs redis-0
```

**Cache Operations Failing**

```bash
# Test Redis connectivity
kubectl exec deployment/cache-service -- redis-cli -h redis-service ping

# Check Redis authentication
kubectl get secret redis-secret -o yaml
```

**NodePort Not Accessible**

```bash
# Verify Kind port mapping
docker port microservices-kind-control-plane | grep 30081

# Test internal connectivity
kubectl run debug --image=busybox:1.35 --rm -it --restart=Never \
  -- wget -qO- http://cache-service:8080/health
```

## 🔒 Security

### Security Context

- **Non-root user**: UID 65534 (nobody)
- **Read-only filesystem**: Enabled
- **No privilege escalation**: Enforced
- **Capabilities dropped**: ALL

### Network Policies

- Ingress: Allows traffic from ingress controller and monitoring
- Egress: Allows DNS and Redis communication only

## 📋 Service Information

| Aspect                | Details                                |
| --------------------- | -------------------------------------- |
| **Namespace**         | `default`                              |
| **Labels**            | `app=cache-service`, `component=cache` |
| **Replicas**          | 1 (Kind optimized)                     |
| **Strategy**          | RollingUpdate                          |
| **Image Pull Policy** | IfNotPresent (Kind optimized)          |

---

**Status**: ✅ Production-ready for Kind deployment  
**Last Updated**: December 29, 2024  
**Documentation**: See [Services Deployment Guide](../../SERVICES_DEPLOYMENT_GUIDE.md#cache-service-guide)
