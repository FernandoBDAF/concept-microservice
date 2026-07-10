# Cache Service Operations Guide

This document provides comprehensive operational procedures, troubleshooting guides, and best practices for running the Cache Service in production.

## Table of Contents

1. [Service Overview](#service-overview)
2. [Deployment Guide](#deployment-guide)
3. [Monitoring and Alerting](#monitoring-and-alerting)
4. [Troubleshooting](#troubleshooting)
5. [Performance Tuning](#performance-tuning)
6. [Backup and Recovery](#backup-and-recovery)
7. [Security Operations](#security-operations)
8. [Maintenance Procedures](#maintenance-procedures)

## Service Overview

### Architecture

- **Service Type**: High-performance Redis-based cache
- **Performance Targets**: < 1ms GET, < 2ms SET, 10,000+ ops/sec
- **Dependencies**: Redis cluster, Kubernetes, Prometheus/Grafana
- **Ecosystem Role**: Caches profile, task, and session data for microservices

### Key Components

- **Cache Service**: Go application with REST/gRPC APIs
- **Redis Backend**: 3-node Redis cluster with persistence
- **Circuit Breaker**: Sony GoBreaker for resilience
- **Monitoring**: Prometheus metrics, Grafana dashboards

## Deployment Guide

### Prerequisites

1. **Kubernetes Cluster** (v1.24+)

   ```bash
   kubectl version --client
   kubectl cluster-info
   ```

2. **Storage Class** for Redis persistence

   ```bash
   kubectl get storageclass
   # Ensure 'fast-ssd' storage class exists
   ```

3. **Monitoring Stack** (Prometheus, Grafana)
   ```bash
   kubectl get pods -n monitoring
   ```

### Deployment Steps

1. **Deploy Redis Backend**

   ```bash
   kubectl apply -f deployments/k8s/redis-statefulset.yaml
   kubectl apply -f deployments/k8s/redis-backup.yaml
   ```

2. **Verify Redis Deployment**

   ```bash
   kubectl get statefulset redis
   kubectl get pods -l app=redis
   kubectl logs redis-0
   ```

3. **Deploy Cache Service**

   ```bash
   kubectl apply -f deployments/k8s/configmap.yaml
   kubectl apply -f deployments/k8s/secret.yaml
   kubectl apply -f deployments/k8s/deployment.yaml
   kubectl apply -f deployments/k8s/service.yaml
   kubectl apply -f deployments/k8s/hpa.yaml
   ```

4. **Deploy Monitoring**

   ```bash
   kubectl apply -f deployments/k8s/monitoring.yaml
   ```

5. **Verify Deployment**
   ```bash
   kubectl get all -l app=cache-service
   kubectl logs -l app=cache-service
   ```

### Health Verification

```bash
# Check service health
kubectl port-forward svc/cache-service 8080:8080
curl http://localhost:8080/health

# Check metrics
curl http://localhost:8080/metrics

# Verify cache operations
curl -X POST http://localhost:8080/api/v1/cache \
  -H "Content-Type: application/json" \
  -d '{"key":"test","value":"hello","ttl":"1h"}'

curl http://localhost:8080/api/v1/cache/test
```

## Monitoring and Alerting

### Key Metrics to Monitor

1. **Service Level Indicators (SLIs)**

   - `cache:sli:availability:5m` (target: > 99.9%)
   - `cache:sli:latency:p99:5m` (target: < 5ms)
   - `cache:sli:error_rate:5m` (target: < 1%)
   - `cache:sli:hit_ratio:5m` (target: > 85%)

2. **Resource Metrics**

   ```prometheus
   # Memory usage
   container_memory_working_set_bytes{pod=~"cache-service-.*"}

   # CPU usage
   rate(container_cpu_usage_seconds_total{pod=~"cache-service-.*"}[5m])

   # Redis connections
   redis_connected_clients

   # Cache operations
   rate(cache_operations_total[5m])
   ```

### Critical Alerts

1. **Service Down**

   ```yaml
   - alert: CacheServiceDown
     expr: up{job="cache-service"} == 0
     for: 1m
     severity: critical
   ```

2. **High Error Rate**

   ```yaml
   - alert: CacheHighErrorRate
     expr: cache:sli:error_rate:5m > 5
     for: 2m
     severity: critical
   ```

3. **High Latency**
   ```yaml
   - alert: CacheHighLatency
     expr: cache:sli:latency:p99:5m > 0.005
     for: 3m
     severity: critical
   ```

### Grafana Dashboard

Access the pre-configured dashboard:

- URL: `http://grafana.example.com/d/cache-service`
- Key panels: Availability, Latency, Hit Ratio, Operations/sec

## Troubleshooting

### Common Issues

#### 1. Service Won't Start

**Symptoms:**

- Pods in `CrashLoopBackOff` state
- Health check failures

**Diagnosis:**

```bash
kubectl describe pod <cache-service-pod>
kubectl logs <cache-service-pod> --previous
```

**Common Causes & Solutions:**

| Cause                    | Solution                                                               |
| ------------------------ | ---------------------------------------------------------------------- |
| Redis connection failure | Check Redis pod status: `kubectl get pods -l app=redis`                |
| Configuration errors     | Verify ConfigMap: `kubectl get configmap cache-service-config -o yaml` |
| Resource limits          | Check resource requests/limits in deployment                           |
| Secret mounting issues   | Verify secret exists: `kubectl get secret cache-service-secret`        |

#### 2. High Latency

**Symptoms:**

- P99 latency > 5ms
- Slow response times

**Diagnosis:**

```bash
# Check Redis latency
kubectl exec redis-0 -- redis-cli --latency-history -i 1

# Check connection pool metrics
curl http://localhost:8080/metrics | grep redis_pool

# Check for slow queries
kubectl exec redis-0 -- redis-cli slowlog get 10
```

**Solutions:**

- Increase Redis connection pool size
- Check network latency between pods
- Optimize Redis configuration
- Review cache key patterns for hot spots

#### 3. Memory Issues

**Symptoms:**

- OOMKilled pods
- High memory usage alerts

**Diagnosis:**

```bash
# Check pod memory usage
kubectl top pods -l app=cache-service

# Check Redis memory usage
kubectl exec redis-0 -- redis-cli info memory

# Check for memory leaks
kubectl exec redis-0 -- redis-cli --bigkeys
```

**Solutions:**

- Increase memory limits
- Configure Redis maxmemory and eviction policy
- Implement proper TTL management
- Review large key patterns

#### 4. Circuit Breaker Activation

**Symptoms:**

- `circuit_breaker_state` metric showing "open"
- Increased error rates

**Diagnosis:**

```bash
# Check circuit breaker metrics
curl http://localhost:8080/metrics | grep circuit_breaker

# Check Redis connectivity
kubectl exec redis-0 -- redis-cli ping
```

**Solutions:**

- Investigate Redis health issues
- Review circuit breaker thresholds
- Check network connectivity
- Restart unhealthy Redis pods if necessary

### Emergency Procedures

#### Service Recovery

1. **Immediate Response**

   ```bash
   # Scale down to zero and back up
   kubectl scale deployment cache-service --replicas=0
   kubectl scale deployment cache-service --replicas=3

   # Force pod restart
   kubectl rollout restart deployment cache-service
   ```

2. **Redis Recovery**

   ```bash
   # Check Redis cluster health
   kubectl exec redis-0 -- redis-cli cluster nodes

   # Restart Redis if needed
   kubectl delete pod redis-0
   ```

3. **Rollback Deployment**
   ```bash
   kubectl rollout history deployment cache-service
   kubectl rollout undo deployment cache-service
   ```

## Performance Tuning

### Connection Pool Optimization

```yaml
# Recommended Redis connection pool settings
CACHE_REDIS_POOL_SIZE: "100"
CACHE_REDIS_MIN_IDLE_CONNS: "25"
CACHE_REDIS_MAX_IDLE_CONNS: "50"
CACHE_REDIS_CONN_MAX_LIFETIME: "300s"
```

### Redis Configuration Tuning

```redis
# High-performance Redis settings
maxmemory-policy allkeys-lru
tcp-keepalive 300
timeout 0
tcp-backlog 511
```

### Application Performance

1. **Batch Operations**

   - Use batch endpoints for multiple keys
   - Limit batch size to 100 items
   - Implement proper error handling

2. **TTL Management**
   ```yaml
   # Optimized TTL settings
   CACHE_CACHE_PROFILE_TTL: "30m"
   CACHE_CACHE_TASK_TTL: "15m"
   CACHE_CACHE_SESSION_TTL: "30m"
   CACHE_CACHE_QUEUE_METRICS_TTL: "2m"
   ```

### Load Testing

```bash
# Run performance tests
./scripts/performance_test.sh

# Monitor during load testing
kubectl top pods -l app=cache-service
kubectl logs -f deployment/cache-service
```

## Backup and Recovery

### Automated Backups

Backups run daily at 2 AM UTC via CronJob:

```bash
# Check backup job status
kubectl get cronjob redis-backup
kubectl get jobs -l component=backup

# Manual backup
kubectl create job --from=cronjob/redis-backup redis-backup-manual
```

### Recovery Procedures

1. **List Available Backups**

   ```bash
   kubectl exec -it redis-backup-pvc-pod -- ls -la /backup
   ```

2. **Restore from Backup**

   ```bash
   # Set backup file name
   BACKUP_FILE="redis-backup-20240101_020000.tar.gz"

   # Update recovery job
   kubectl patch job redis-recovery -p \
     '{"spec":{"template":{"spec":{"containers":[{"name":"redis-recovery","env":[{"name":"BACKUP_FILE","value":"'$BACKUP_FILE'"}]}]}}}}'

   # Run recovery
   kubectl apply -f deployments/k8s/redis-backup.yaml
   kubectl logs -f job/redis-recovery
   ```

### Disaster Recovery

1. **Complete Redis Cluster Failure**

   ```bash
   # Delete failed StatefulSet
   kubectl delete statefulset redis

   # Recreate with fresh PVCs
   kubectl apply -f deployments/k8s/redis-statefulset.yaml

   # Restore from backup
   # (follow recovery procedures above)
   ```

## Security Operations

### Certificate Management

```bash
# Check TLS certificates (if enabled)
kubectl get secrets -l type='kubernetes.io/tls'

# Rotate certificates
kubectl create secret tls cache-service-tls \
  --cert=path/to/cert.pem \
  --key=path/to/key.pem
```

### Access Control

```bash
# Review RBAC permissions
kubectl describe clusterrole cache-service-role
kubectl describe clusterrolebinding cache-service-binding

# Audit secret access
kubectl get events --field-selector involvedObject.name=cache-service-secret
```

### Security Scanning

```bash
# Scan container images
trivy image cache-service:latest

# Check for security policies
kubectl get networkpolicies -l app=cache-service
```

## Maintenance Procedures

### Routine Maintenance

#### Weekly Tasks

1. **Review Metrics and Alerts**

   ```bash
   # Check alert history
   curl -G http://prometheus:9090/api/v1/alerts

   # Review performance trends
   curl -G http://prometheus:9090/api/v1/query \
     --data-urlencode 'query=cache:sli:availability:5m[7d]'
   ```

2. **Verify Backups**

   ```bash
   kubectl logs cronjob/redis-backup
   kubectl exec -it redis-backup-pvc-pod -- ls -la /backup
   ```

3. **Update Documentation**
   - Review and update runbooks
   - Document any configuration changes
   - Update emergency contact information

#### Monthly Tasks

1. **Capacity Planning**

   ```bash
   # Check resource utilization trends
   kubectl top nodes
   kubectl describe hpa cache-service-hpa
   ```

2. **Security Updates**

   ```bash
   # Update container images
   docker pull cache-service:latest
   kubectl set image deployment/cache-service cache-service=cache-service:latest
   ```

3. **Performance Review**

   ```bash
   # Run comprehensive performance tests
   ./scripts/performance_test.sh

   # Review and optimize configurations
   kubectl get configmap cache-service-config -o yaml
   ```

### Scaling Operations

#### Horizontal Scaling

```bash
# Manual scaling
kubectl scale deployment cache-service --replicas=5

# HPA status
kubectl get hpa cache-service-hpa
kubectl describe hpa cache-service-hpa
```

#### Vertical Scaling

```bash
# Update resource limits
kubectl patch deployment cache-service -p \
  '{"spec":{"template":{"spec":{"containers":[{"name":"cache-service","resources":{"limits":{"memory":"2Gi","cpu":"2000m"}}}]}}}}'
```

### Version Upgrades

1. **Pre-upgrade Checklist**

   - [ ] Backup current configuration
   - [ ] Review changelog and breaking changes
   - [ ] Plan rollback strategy
   - [ ] Schedule maintenance window

2. **Upgrade Process**

   ```bash
   # Create backup
   kubectl create job --from=cronjob/redis-backup redis-backup-pre-upgrade

   # Update image
   kubectl set image deployment/cache-service cache-service=cache-service:v1.1.0

   # Monitor rollout
   kubectl rollout status deployment cache-service

   # Verify functionality
   curl http://localhost:8080/health
   ./scripts/performance_test.sh
   ```

3. **Post-upgrade Verification**
   - Verify all health checks pass
   - Run performance tests
   - Check error rates and latency
   - Monitor for 24 hours

## Emergency Contacts

| Role                    | Contact      | Phone       | Email                  |
| ----------------------- | ------------ | ----------- | ---------------------- |
| On-call Engineer        | John Doe     | +1-555-0100 | john.doe@example.com   |
| Cache Service Team Lead | Jane Smith   | +1-555-0101 | jane.smith@example.com |
| DevOps Team             | DevOps Slack | N/A         | devops@example.com     |
| Production Support      | Support Team | +1-555-0199 | support@example.com    |

## Documentation Links

- [API Documentation](../api/openapi.yaml)
- [Service README](../README.md)
- [Architecture Context](../CONTEXT.md)
- [Interface Specifications](../INTERFACE.md)
- [Implementation Tracker](../TRACKER.md)
- [Performance Testing](../scripts/performance_test.sh)
- [Monitoring Scripts](../scripts/perf_monitor.sh)

---

**Last Updated:** January 2024  
**Document Version:** 1.0  
**Maintainers:** Cache Service Team
