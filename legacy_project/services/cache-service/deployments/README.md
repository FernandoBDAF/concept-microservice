# Cache Service Deployment Guide

This directory contains all deployment configurations and scripts for the Cache Service, a high-performance Redis-based caching service for the microservices ecosystem.

## 📁 Directory Structure

```
deployments/
├── README.md                          # This deployment guide
├── STEP_BY_STEP_DEPLOYMENT_GUIDE.md  # Comprehensive manual deployment guide
├── k8s/                              # Production Kubernetes manifests
│   ├── deployment.yaml               # Main service deployment
│   ├── service.yaml                  # Service and RBAC configuration
│   ├── configmap.yaml                # Configuration management
│   ├── secret.yaml                   # Secret management
│   ├── redis-statefulset.yaml       # Redis backend StatefulSet
│   ├── hpa.yaml                      # Horizontal Pod Autoscaler
│   ├── monitoring.yaml               # Prometheus and Grafana configuration
│   └── redis-backup.yaml             # Automated backup jobs
├── scripts/                          # Manual deployment scripts
│   ├── manual-deploy.sh              # Interactive deployment script
│   └── manual-cleanup.sh             # Interactive cleanup script
├── kind/                             # Kind local development
│   ├── kustomization.yaml            # Kind overlay configuration
│   ├── deployment-patch.yaml         # Resource patches for local dev
│   ├── service-patch.yaml            # NodePort patches for local access
│   ├── redis-dependencies.yaml       # Redis for development
│   └── deploy-to-kind.sh             # Automated Kind deployment
└── monitoring/                       # Monitoring configuration
    └── servicemonitor.yaml           # Prometheus integration
```

## 🚀 Quick Start

### Production Deployment

```bash
# Step-by-step guided deployment
cd deployments/scripts
./manual-deploy.sh --step-by-step --analyze

# Or follow the comprehensive guide
open deployments/STEP_BY_STEP_DEPLOYMENT_GUIDE.md
```

### Local Development (Kind)

```bash
# One-command local deployment
cd deployments/kind
./deploy-to-kind.sh --with-redis

# Access endpoints:
# - Cache Service: http://localhost:30082
# - Metrics: http://localhost:30083
```

## 📊 Monitoring and Observability

### ServiceMonitor Deployment

Deploy monitoring configuration for Prometheus:

```bash
# Deploy ServiceMonitor and PrometheusRule
kubectl apply -f monitoring/servicemonitor.yaml
```

### Available Metrics

Cache-specific metrics exposed at `/metrics`:

- `cache_operations_total`: Total cache operations (hit/miss/error)
- `cache_latency_seconds`: Cache operation latency histogram
- `cache_redis_connections_active`: Active Redis connections
- `cache_circuit_breaker_state`: Circuit breaker state (0=closed, 1=open)
- `cache_memory_usage_bytes`: Cache memory usage
- `cache_keys_total`: Total number of cached keys

### Alerts

Predefined alerts for cache service:

- **CacheHighMissRate**: Cache miss rate > 80%
- **CacheHighLatency**: 95th percentile latency > 10ms
- **CacheRedisConnectionFailure**: Redis connection errors
- **CacheCircuitBreakerOpen**: Circuit breaker activated

### Monitoring Dashboard

Import cache service dashboard for Grafana:

```bash
# Dashboard JSON available in monitoring/grafana-dashboard.json
```

### Local Monitoring (Kind)

For Kind clusters, access metrics directly:

```bash
# Port forward to metrics endpoint
kubectl port-forward service/kind-cache-service 8081:8081

# View metrics
curl http://localhost:8081/metrics
```

## 🎯 Deployment Options

### 1. Production Kubernetes

**Use Case**: Production workloads with high availability and monitoring
**Components**: StatefulSet Redis, HPA, monitoring, backup

```bash
# Deploy all production components
kubectl apply -f k8s/
```

### 2. Manual Deployment Scripts

**Use Case**: Educational deployment, troubleshooting, step-by-step learning
**Features**: Interactive prompts, detailed analysis, Redis health checks

```bash
# Interactive deployment with analysis
./scripts/manual-deploy.sh --step-by-step --analyze

# Cleanup with data preservation
./scripts/manual-cleanup.sh --preserve-data --step-by-step
```

### 3. Kind Local Development

**Use Case**: Local development, testing, CI/CD validation
**Features**: Reduced resources, NodePort access, Redis included

```bash
# Complete local setup
./kind/deploy-to-kind.sh --with-redis --build-image
```

## 🔧 Configuration Management

### Environment Variables

Key configuration variables:

```yaml
# Redis Configuration
CACHE_REDIS_HOST: "redis-service"
CACHE_REDIS_POOL_SIZE: "100" # Production: 100, Kind: 10
CACHE_REDIS_MIN_IDLE_CONNS: "25" # Production: 25, Kind: 2

# Cache Settings
CACHE_CACHE_PROFILE_TTL: "30m" # Profile cache TTL
CACHE_CACHE_TASK_TTL: "15m" # Task cache TTL
CACHE_CACHE_SESSION_TTL: "30m" # Session cache TTL

# Circuit Breaker
CACHE_CIRCUIT_BREAKER_ENABLED: "true"
CACHE_CIRCUIT_BREAKER_TIMEOUT: "30s"

# Logging
CACHE_LOG_LEVEL: "info" # Production: info, Kind: debug
CACHE_LOGGING_DEVELOPMENT: "false" # Production: false, Kind: true
```

### Secrets Management

Required secrets:

```yaml
# Redis Authentication
CACHE_REDIS_PASSWORD: "<secure-password>"

# JWT Integration
JWT_SECRET_KEY: "<jwt-secret-key>"
```

**Production Secret Generation**:

```bash
# Generate secure Redis password
kubectl create secret generic cache-service-secret \
  --from-literal=CACHE_REDIS_PASSWORD="$(openssl rand -base64 32)" \
  --from-literal=JWT_SECRET_KEY="$(openssl rand -base64 64)"
```

## 🏗️ Architecture Components

### Cache Service

- **Type**: Deployment (3 replicas in production, 1 in Kind)
- **Resources**: 256Mi-1Gi memory, 250m-1000m CPU
- **Ports**: 8080 (HTTP), 9090 (gRPC), 8081 (metrics)
- **Health Checks**: Liveness, readiness, startup probes

### Redis Backend

- **Type**: StatefulSet (3 replicas in production, 1 in Kind)
- **Persistence**: 10Gi PVC per replica
- **Configuration**: AOF + RDB persistence, allkeys-lru eviction
- **Monitoring**: Redis exporter sidecar

### Monitoring Stack

- **ServiceMonitor**: Prometheus metrics scraping
- **PrometheusRule**: 4 cache-specific alerts
- **Grafana Dashboard**: Performance and health panels
- **SLI/SLO Tracking**: Availability and latency targets

## 🧪 Testing and Validation

### Health Checks

```bash
# Service health
curl http://cache-service:8080/health
curl http://cache-service:8080/ready

# Redis connectivity
kubectl exec -it redis-0 -- redis-cli ping
```

### Cache Operations

```bash
# Basic operations
curl -X POST http://cache-service:8080/api/v1/cache/test \
  -H "Content-Type: application/octet-stream" \
  -d "test-value"

curl http://cache-service:8080/api/v1/cache/test

# Batch operations
curl -X POST http://cache-service:8080/api/v1/cache/batch/get \
  -H "Content-Type: application/json" \
  -d '{"keys":["key1","key2"]}'
```

### Performance Testing

```bash
# Run performance tests
./scripts/performance_test.sh

# Check metrics
curl http://cache-service:8081/metrics | grep cache_
```

## 🚨 Troubleshooting

### Common Issues

1. **Redis Connection Failed**

   - Check Redis pod status: `kubectl get pods -l app=redis`
   - Verify service: `kubectl get service redis-service`
   - Test connectivity: `kubectl exec -it redis-0 -- redis-cli ping`

2. **Cache High Latency**

   - Check Redis performance: `kubectl exec -it redis-0 -- redis-cli info stats`
   - Monitor connection pool: Check `cache_redis_connections_active` metric
   - Verify resource limits: `kubectl describe pod -l app=cache-service`

3. **Circuit Breaker Open**
   - Check circuit breaker state: `cache_circuit_breaker_state` metric
   - Verify Redis health: `kubectl logs -l app=cache-service | grep circuit`
   - Check error rates: `cache_operations_total{status="error"}`

### Debug Commands

```bash
# View cache service logs
kubectl logs -f deployment/cache-service

# Redis performance analysis
kubectl exec -it redis-0 -- redis-cli info memory
kubectl exec -it redis-0 -- redis-cli info clients

# Connection pool status
curl -s http://cache-service:8081/metrics | grep redis_pool

# Circuit breaker status
curl -s http://cache-service:8081/metrics | grep circuit_breaker
```

## 📚 Additional Resources

- [Step-by-Step Deployment Guide](STEP_BY_STEP_DEPLOYMENT_GUIDE.md) - Comprehensive manual deployment
- [Cache Service API Documentation](../api/openapi.yaml) - Complete REST API specification
- [Operations Guide](../docs/OPERATIONS.md) - Production operations and maintenance
- [Performance Testing](../scripts/performance_test.sh) - Load testing and optimization

## 🤝 Contributing

When modifying deployment configurations:

1. Test changes in Kind environment first
2. Update both production and Kind configurations
3. Validate monitoring and alerting changes
4. Update documentation for any new features

---

**Last Updated**: January 2025  
**Maintainers**: Cache Service Development Team
