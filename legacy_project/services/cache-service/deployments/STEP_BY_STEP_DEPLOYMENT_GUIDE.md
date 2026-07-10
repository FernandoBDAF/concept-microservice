# Step-by-Step Kubernetes Deployment Guide

## Cache Service High-Performance Redis Architecture

The Cache Service is a **high-performance Redis-based caching service** that provides sub-millisecond operations for the microservices ecosystem. It serves as the central caching layer for Profile, Queue, Worker, and Storage services with specialized caching patterns and comprehensive monitoring.

**Key Features:**

- **Performance**: < 1ms GET, < 2ms SET operations, 10,000+ ops/second
- **Ecosystem Integration**: Profile caching, session management, task status caching
- **Resilience**: Circuit breaker patterns, connection pooling, automatic failover
- **Production-Ready**: Monitoring, alerting, backup/recovery, security

## 🚀 Two Ways to Follow This Guide

### Option 1: Automated Manual Deployment (Recommended)

```bash
cd deployments/scripts

# Interactive step-by-step deployment
./manual-deploy.sh --step-by-step

# With detailed manifest analysis
./manual-deploy.sh --analyze

# Cleanup when done
./manual-cleanup.sh --step-by-step
```

### Option 2: Manual Commands (Educational)

Follow the step-by-step commands below to understand each deployment phase.

## 📋 Prerequisites

### 1. Kubernetes Cluster

```bash
# Verify cluster access
kubectl cluster-info
kubectl version --client

# Ensure you have cluster-admin permissions
kubectl auth can-i create statefulsets
kubectl auth can-i create persistentvolumeclaims
```

### 2. Storage Classes

```bash
# Check available storage classes
kubectl get storageclass

# Verify fast storage for Redis (recommended)
kubectl describe storageclass fast-ssd || kubectl describe storageclass standard
```

### 3. Monitoring Stack (Optional)

```bash
# Check if Prometheus is available
kubectl get pods -n monitoring 2>/dev/null || echo "Monitoring namespace not found"
```

## 🚀 Deployment Sequence

### Step 1: 🔐 Deploy Secrets (`secret.yaml`)

**What it does**: Creates sensitive configuration data for Redis passwords and JWT secrets

#### Deploy:

```bash
kubectl apply -f deployments/kubernetes/secret.yaml
```

#### Verification Commands:

```bash
# 1. Check secret creation
kubectl get secret cache-service-secret

# 2. Verify secret structure (without revealing values)
kubectl describe secret cache-service-secret

# 3. Check secret is properly formatted
kubectl get secret cache-service-secret -o yaml | grep "data:" -A 5
```

**Expected Impact**: ✅ Secret `cache-service-secret` created with Redis password and JWT keys

**⚠️ Production Note**: Replace default passwords with secure values:

```bash
# Generate secure Redis password
kubectl create secret generic cache-service-secret \
  --from-literal=CACHE_REDIS_PASSWORD="$(openssl rand -base64 32)" \
  --from-literal=JWT_SECRET_KEY="$(openssl rand -base64 64)" \
  --dry-run=client -o yaml | kubectl apply -f -
```

---

### Step 2: ⚙️ Deploy ConfigMaps (`configmap.yaml`)

**What it does**: Creates non-sensitive configuration for cache service and Redis settings

#### Deploy:

```bash
kubectl apply -f deployments/kubernetes/configmap.yaml
```

#### Critical Observation Commands:

```bash
# 1. Check ConfigMap creation
kubectl get configmap cache-service-config

# 2. Review cache-specific configuration
kubectl get configmap cache-service-config -o yaml | grep -A 20 "cache:"

# 3. Verify Redis connection settings
kubectl get configmap cache-service-config -o yaml | grep -A 10 "REDIS"

# 4. Check performance settings
kubectl get configmap cache-service-config -o yaml | grep -E "(POOL_SIZE|TTL|CIRCUIT)"
```

**Expected Impact**: ✅ ConfigMap with optimized cache settings created

**🔧 Cache-Specific Settings to Verify**:

- `CACHE_REDIS_POOL_SIZE: "100"` (high-performance connection pooling)
- `CACHE_CACHE_PROFILE_TTL: "30m"` (profile-specific TTL)
- `CACHE_CIRCUIT_BREAKER_ENABLED: "true"` (resilience patterns)

---

### Step 3: 🔒 Deploy RBAC & Service (`service.yaml`)

**What it does**: Creates service discovery, load balancing, and security permissions

#### Deploy:

```bash
kubectl apply -f deployments/kubernetes/service.yaml
```

#### Critical Observation Commands:

```bash
# 1. Check service creation and endpoints
kubectl get service cache-service
kubectl get endpoints cache-service

# 2. Verify service ports (HTTP, gRPC, metrics)
kubectl describe service cache-service | grep -A 10 "Port:"

# 3. Check metrics service for monitoring
kubectl get service cache-service-metrics 2>/dev/null || echo "Metrics service not found"

# 4. Verify service selector matches deployment labels
kubectl describe service cache-service | grep -A 5 "Selector:"
```

**Expected Impact**: ✅ Service `cache-service` with HTTP (8080), gRPC (9090), and metrics (8081) ports

// this kind of comment is very good and should be all over the deployment documentation
// it makes clear what to look for in each manifest ipact and what is the consequence of every command
**⚠️ What to Look For**:

- Service should have `ClusterIP` type for internal communication
- Endpoints should show `<none>` until pods are deployed
- Port mappings: 8080 (HTTP), 9090 (gRPC), 8081 (metrics)

---

### Step 4: 🗄️ Deploy Redis Backend (`redis-statefulset.yaml`)

**What it does**: Creates Redis StatefulSet with persistence for cache storage

// great section
**⚠️ Why StatefulSet?** Redis requires:

- **Persistent storage** for data durability
- **Stable network identity** for cluster formation
- **Ordered deployment** for proper initialization

#### Deploy:

```bash
# Deploy Redis StatefulSet with persistence
kubectl apply -f deployments/kubernetes/redis-statefulset.yaml
```

<!-- // error with the command above: error: resource mapping not found for name: "redis-metrics" namespace: "default" from "deployments/kubernetes/redis-statefulset.yaml": no matches for kind "ServiceMonitor" in version "monitoring.coreos.com/v1" -->

#### Critical Observation Commands:

```bash
# 1. Watch StatefulSet rollout (Redis-specific)
kubectl rollout status statefulset/redis --timeout=300s

# 2. Check Redis pod status and persistence
kubectl get pods -l app=redis -o wide
kubectl get pvc -l app=redis

# 3. Test Redis connectivity and performance
kubectl exec -it redis-0 -- redis-cli ping
kubectl exec -it redis-0 -- redis-cli info memory

# 4. Verify Redis configuration and persistence
kubectl exec -it redis-0 -- redis-cli config get save
kubectl exec -it redis-0 -- redis-cli lastsave

# 5. Check Redis service and endpoints
kubectl get service redis-service
kubectl get endpoints redis-service

# 6. Verify Redis authentication
kubectl exec -it redis-0 -- redis-cli -a "$(kubectl get secret cache-service-secret -o jsonpath='{.data.CACHE_REDIS_PASSWORD}' | base64 -d)" ping
```

// most commands above returned: 
<!-- Defaulted container "redis" out of: redis, redis-exporter-->

**Expected Impact**: ✅ Redis StatefulSet running with persistent volumes

**🔧 Redis-Specific Validation**:

```bash
# Performance validation
kubectl exec -it redis-0 -- redis-cli info stats | grep -E "(total_commands_processed|instantaneous_ops_per_sec)"

# Memory configuration
kubectl exec -it redis-0 -- redis-cli config get maxmemory
kubectl exec -it redis-0 -- redis-cli config get maxmemory-policy

# Persistence validation
kubectl exec -it redis-0 -- redis-cli config get appendonly
kubectl exec -it redis-0 -- redis-cli config get save
```

---

### Step 5: 🚀 Deploy Cache Service Application

**What it does**: Deploys the main cache service with Redis connectivity

#### Deploy:

```bash
kubectl apply -f deployments/kubernetes/deployment.yaml
```

#### Critical Observation Commands:

```bash
# 1. Watch deployment rollout
kubectl rollout status deployment/cache-service --timeout=300s

# 2. Check pod status and ready state
kubectl get pods -l app=cache-service -o wide

# 3. Verify container logs for startup
kubectl logs -l app=cache-service --tail=20

# 4. Check service connectivity to Redis
kubectl logs -l app=cache-service | grep -i redis

# 5. Verify health checks are passing
kubectl describe pods -l app=cache-service | grep -A 10 "Conditions:"

# 6. Test application endpoints
kubectl port-forward service/cache-service 8080:8080 &
sleep 2
curl -s http://localhost:8080/health | jq '.' || curl -s http://localhost:8080/health
```

**Expected Impact**: ✅ Cache service pods running and connected to Redis

**🔧 Cache Service Specific Checks**:

```bash
# Circuit breaker status
kubectl logs -l app=cache-service | grep -i "circuit"

# Connection pool status
kubectl logs -l app=cache-service | grep -i "pool"

# Performance metrics availability
curl -s http://localhost:8080/metrics | grep cache_ | head -5
```

---

### Step 6: 📊 Deploy Monitoring

**What it does**: Sets up Prometheus alerts, Grafana dashboards, and SLI/SLO tracking

#### Deploy:

```bash
kubectl apply -f deployments/k8s/monitoring.yaml
```

#### Critical Observation Commands:

```bash
# 1. Check PrometheusRule creation
kubectl get prometheusrule cache-service-alerts

# 2. Verify ServiceMonitor for metrics scraping
kubectl get servicemonitor cache-service-metrics

# 3. Check Grafana dashboard ConfigMap
kubectl get configmap cache-service-dashboard

# 4. Verify SLI/SLO configuration
kubectl get configmap cache-service-slo

# 5. Test metrics collection
kubectl port-forward service/cache-service 8081:8081 &
sleep 2
curl -s http://localhost:8081/metrics | grep -E "(cache_operations_total|cache_hits_total)" | head -3
```

**Expected Impact**: ✅ Monitoring stack configured with cache-specific alerts and dashboards

---

### Step 7: 📈 Deploy Auto-Scaling (Optional)

**What it does**: Enables automatic scaling based on CPU, memory, and cache-specific metrics

#### Deploy:

```bash
kubectl apply -f deployments/k8s/hpa.yaml
```

#### Critical Observation Commands:

```bash
# 1. Check HPA status
kubectl get hpa cache-service-hpa

# 2. Verify metrics availability for scaling
kubectl describe hpa cache-service-hpa

# 3. Check current resource utilization
kubectl top pods -l app=cache-service
```

**Expected Impact**: ✅ HPA configured for automatic scaling (3-20 replicas)

## 🔍 Comprehensive Cluster State Commands

### Final Verification Suite

```bash
echo "🔍 Cache Service Deployment Verification"
echo "========================================"

# 1. All resources status
kubectl get all -l app=cache-service
kubectl get all -l app=redis

# 2. Storage verification
kubectl get pvc -l app=redis
kubectl get storageclass

# 3. Configuration verification
kubectl get configmap,secret | grep cache

# 4. Monitoring verification
kubectl get prometheusrule,servicemonitor | grep cache

# 5. Network verification
kubectl get service,endpoints | grep -E "(cache|redis)"

echo ""
echo "📊 Service Health Check"
echo "======================"
kubectl port-forward service/cache-service 8080:8080 &
PF_PID=$!
sleep 3

# Health endpoint
echo "Health Status:"
curl -s http://localhost:8080/health | jq '.status' 2>/dev/null || curl -s http://localhost:8080/health

# Basic cache test
echo "Cache Test:"
curl -s -X POST http://localhost:8080/api/v1/cache \
  -H "Content-Type: application/json" \
  -d '{"key":"test","value":"deployment-success","ttl":"1h"}' && echo " ✅ SET OK"

curl -s http://localhost:8080/api/v1/cache/test | jq '.value' 2>/dev/null && echo " ✅ GET OK"

kill $PF_PID 2>/dev/null
```

## 🎯 What to Look For at Each Step

### ✅ Success Indicators

| Step              | Success Criteria                                              |
| ----------------- | ------------------------------------------------------------- |
| **Secrets**       | Secret exists with proper data keys                           |
| **ConfigMaps**    | Cache configuration with Redis settings                       |
| **Services**      | ClusterIP services with correct port mappings                 |
| **Redis**         | StatefulSet ready, persistent volumes bound, Redis responsive |
| **Cache Service** | Deployment ready, health checks passing, Redis connected      |
| **Monitoring**    | PrometheusRule and ServiceMonitor created                     |
| **Auto-Scaling**  | HPA active with metrics available                             |

### ⚠️ Warning Signs

| Issue                        | Symptom                              | Quick Fix                          |
| ---------------------------- | ------------------------------------ | ---------------------------------- |
| **Redis Connection Failed**  | Cache service logs show Redis errors | Check Redis pod status and service |
| **Persistent Volume Issues** | StatefulSet pending                  | Verify storage class exists        |
| **Health Checks Failing**    | Pods not ready                       | Check application logs             |
| **Metrics Missing**          | No metrics at `/metrics`             | Verify Prometheus port 8081        |

## 🚨 Common Issues & Troubleshooting

### Redis Connection Issues

**Issue**: Cache service cannot connect to Redis

**Symptoms**:

```
Failed to connect to Redis: dial tcp redis-service:6379: connection refused
```

**Root Cause**: Redis not ready or service misconfiguration

**Solution**:

```bash
# Check Redis pod status
kubectl get pods -l app=redis
kubectl logs redis-0

# Verify Redis service configuration
kubectl get service redis-service -o yaml

# Test Redis connectivity from cache service pod
kubectl exec -it deployment/cache-service -- telnet redis-service 6379
```

### Cache Performance Issues

**Issue**: High cache latency or timeout errors

**Symptoms**:

```
Cache operation timeout: context deadline exceeded
```

**Root Cause**: Redis overload or connection pool exhaustion

**Solution**:

```bash
# Check Redis performance metrics
kubectl exec -it redis-0 -- redis-cli info stats
kubectl exec -it redis-0 -- redis-cli info clients

# Check cache service connection pool
kubectl logs -l app=cache-service | grep "connection pool"

# Verify resource limits
kubectl describe pod -l app=cache-service | grep -A5 -B5 "Limits\|Requests"
```

### Persistent Volume Issues

**Issue**: Redis StatefulSet stuck in pending state

**Symptoms**:

```
pod has unbound immediate PersistentVolumeClaims
```

**Root Cause**: Storage class not available or insufficient storage

**Solution**:

```bash
# Check storage class availability
kubectl get storageclass
kubectl describe storageclass fast-ssd

# Check PVC status
kubectl get pvc -l app=redis
kubectl describe pvc redis-data-redis-0

# Use different storage class if needed
kubectl patch statefulset redis -p '{"spec":{"volumeClaimTemplates":[{"metadata":{"name":"redis-data"},"spec":{"storageClassName":"standard"}}]}}'
```

### Circuit Breaker Activation

**Issue**: Circuit breaker open, blocking cache requests

**Symptoms**:

```
Circuit breaker is open, failing fast
```

**Root Cause**: Redis unavailability or high error rate

**Solution**:

```bash
# Check circuit breaker status
kubectl logs -l app=cache-service | grep -i "circuit breaker"

# Verify Redis health
kubectl exec -it redis-0 -- redis-cli ping

# Check error rates
kubectl port-forward service/cache-service 8081:8081 &
curl -s http://localhost:8081/metrics | grep circuit_breaker
```

## 🧪 Quick Test Suite

### Basic Functionality Test

```bash
#!/bin/bash
echo "🧪 Cache Service Test Suite"
echo "=========================="

# Port forward to cache service
kubectl port-forward service/cache-service 8080:8080 &
PF_PID=$!
sleep 2

# 1. Health checks (cache-specific)
echo "1. Health Checks:"
curl -f http://localhost:8080/health && echo "  ✅ Cache Health OK" || echo "  ❌ Cache Health Failed"
curl -f http://localhost:8080/ready && echo "  ✅ Cache Ready OK" || echo "  ❌ Cache Ready Failed"

# 2. Basic cache operations
echo "2. Basic Cache Operations:"
curl -X POST http://localhost:8080/api/v1/cache \
  -H "Content-Type: application/json" \
  -d '{"key":"test-key","value":"test-value","ttl":"1h"}' \
  && echo "  ✅ Cache SET OK" || echo "  ❌ Cache SET Failed"

curl -s http://localhost:8080/api/v1/cache/test-key | grep "test-value" \
  && echo "  ✅ Cache GET OK" || echo "  ❌ Cache GET Failed"

# 3. Profile-specific cache operations
echo "3. Profile Cache Operations:"
curl -X POST http://localhost:8080/api/v1/cache \
  -H "Content-Type: application/json" \
  -d '{"key":"profile:123","value":"{\"id\":\"123\",\"name\":\"Test User\"}","ttl":"30m"}' \
  && echo "  ✅ Profile Cache SET OK" || echo "  ❌ Profile Cache SET Failed"

curl -s http://localhost:8080/api/v1/cache/profile:123 | grep "Test User" \
  && echo "  ✅ Profile Cache GET OK" || echo "  ❌ Profile Cache GET Failed"

# 4. Batch operations
echo "4. Batch Operations:"
curl -X POST http://localhost:8080/api/v1/cache/batch/get \
  -H "Content-Type: application/json" \
  -d '{"keys":["test-key","profile:123"]}' \
  && echo "  ✅ Batch GET OK" || echo "  ❌ Batch GET Failed"

# 5. Cache metrics validation
echo "5. Metrics Validation:"
kubectl port-forward service/cache-service 8081:8081 &
METRICS_PID=$!
sleep 2
METRICS_COUNT=$(curl -s http://localhost:8081/metrics | grep cache_ | wc -l)
echo "  Cache metrics count: $METRICS_COUNT"
[ "$METRICS_COUNT" -gt 5 ] && echo "  ✅ Metrics OK" || echo "  ❌ Metrics Missing"

# 6. Redis backend validation
echo "6. Redis Backend:"
kubectl exec -it redis-0 -- redis-cli ping && echo "  ✅ Redis OK" || echo "  ❌ Redis Failed"
REDIS_KEYS=$(kubectl exec -it redis-0 -- redis-cli dbsize | tr -d '\r')
echo "  Redis keys count: $REDIS_KEYS"

# 7. Performance validation
echo "7. Performance Test:"
TIME_START=$(date +%s%3N)
curl -s http://localhost:8080/api/v1/cache/test-key > /dev/null
TIME_END=$(date +%s%3N)
LATENCY=$((TIME_END - TIME_START))
echo "  GET latency: ${LATENCY}ms"
[ "$LATENCY" -lt 100 ] && echo "  ✅ Latency OK" || echo "  ⚠️  Latency High"

# Cleanup
kill $PF_PID $METRICS_PID 2>/dev/null
echo ""
echo "🎯 Test Suite Complete"
```

## 📝 Cache-Specific Notes

### Cache Key Patterns

The cache service uses specific key patterns for different data types:

- **Profiles**: `profile:{profileID}` or `profile:email:{email}`
- **Sessions**: `session:{sessionID}` or `jwt:blacklist:{tokenID}`
- **Tasks**: `task:status:{taskID}` or `queue:metrics:{queueName}`
- **Worker**: `worker:status:{workerType}`

### TTL Strategy

Different data types have optimized TTL values:

- **Profile Data**: 30 minutes (frequently accessed, moderate update frequency)
- **Task Status**: 15 minutes (dynamic data, frequent updates)
- **Session Data**: 30 minutes (security-sensitive, moderate duration)
- **Queue Metrics**: 2 minutes (high-frequency updates)
- **Worker Status**: 10 minutes (infrastructure monitoring)

### Performance Monitoring

Key metrics to monitor during deployment:

```bash
# Cache hit ratio (target: >85%)
curl -s http://localhost:8081/metrics | grep cache_hits_total

# Operation latency (target: <1ms GET, <2ms SET)
curl -s http://localhost:8081/metrics | grep cache_operation_duration

# Error rate (target: <1%)
curl -s http://localhost:8081/metrics | grep cache_operations_total

# Connection pool utilization
curl -s http://localhost:8081/metrics | grep redis_pool
```

### Redis Configuration Notes

The Redis configuration is optimized for caching workloads:

- **Memory Policy**: `allkeys-lru` for automatic eviction
- **Persistence**: Both AOF and RDB for durability
- **Max Memory**: 1.5GB with proper eviction policies
- **Connection Limits**: 10,000 max clients with optimized timeouts

---

## 🔗 Related Documentation

- [**Cache Service README**](../README.md) - Service overview and features
- [**API Documentation**](../api/openapi.yaml) - Complete REST API specification
- [**Operations Guide**](../docs/OPERATIONS.md) - Production operations and troubleshooting
- [**Performance Testing**](../scripts/performance_test.sh) - Load testing and validation
- [**Architecture Context**](../CONTEXT.md) - Technical architecture and design decisions

---

**Last Updated**: January 2025  
**Guide Version**: 1.0  
**Maintainers**: Cache Service Development Team
