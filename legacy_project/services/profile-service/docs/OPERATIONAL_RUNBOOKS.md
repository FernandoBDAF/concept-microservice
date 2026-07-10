# Profile Service Operational Runbooks

## Overview

This document provides detailed operational runbooks for the Profile Service's multi-worker architecture, covering monitoring, alerting, and incident response procedures.

## Runbook Index

- [High Error Rate](#high-error-rate)
- [Critical Error Rate](#critical-error-rate)
- [Queue Service Communication Failure](#queue-service-communication-failure)
- [High Response Time](#high-response-time)
- [Task Submission Failures](#task-submission-failures)
- [Routing Key Imbalance](#routing-key-imbalance)
- [Pod Crash Loop](#pod-crash-loop)
- [High Resource Usage](#high-resource-usage)
- [Service Unavailable](#service-unavailable)
- [Low Throughput](#low-throughput)

---

## High Error Rate

**Alert**: `ProfileServiceHighErrorRate`  
**Severity**: Warning  
**Threshold**: Error rate > 1% for 2 minutes  
**SLA Impact**: Medium

### Immediate Actions (5 minutes)

1. **Check Current Status**

   ```bash
   # Check error rate across endpoints
   kubectl logs -n default -l app=profile-service --tail=100 | grep ERROR

   # Check recent deployments
   kubectl rollout history deployment/profile-service -n default
   ```

2. **Identify Error Patterns**

   ```bash
   # Check specific error types
   curl -s http://prometheus:9090/api/v1/query?query='rate(profile_api_requests_total{status=~"5.."}[5m])' | jq

   # Check affected endpoints
   kubectl logs -n default -l app=profile-service --tail=500 | grep -E "HTTP [45][0-9][0-9]" | sort | uniq -c
   ```

3. **Check Dependencies**

   ```bash
   # Verify queue service health
   curl -s http://queue-service:8080/health | jq

   # Check storage service connectivity
   kubectl exec -n default deployment/profile-service -- curl -f http://storage-service:8080/health
   ```

### Investigation Steps (15 minutes)

1. **Analyze Error Distribution**

   - Check if errors are distributed across all pods or specific instances
   - Identify if errors correlate with specific task types
   - Review recent configuration changes

2. **Resource Analysis**

   ```bash
   # Check pod resource usage
   kubectl top pods -n default -l app=profile-service

   # Check cluster resource availability
   kubectl describe nodes | grep -A5 "Allocated resources"
   ```

3. **Network Connectivity**
   ```bash
   # Test internal service connectivity
   kubectl exec -n default deployment/profile-service -- nslookup queue-service
   kubectl exec -n default deployment/profile-service -- nslookup storage-service
   ```

### Resolution Actions

#### If caused by recent deployment:

```bash
# Rollback to previous version
./deployments/scripts/rollback-procedures.sh rollback-previous
```

#### If caused by dependency issues:

```bash
# Restart dependent services
kubectl rollout restart deployment/queue-service -n default
kubectl rollout restart deployment/storage-service -n default
```

#### If caused by resource constraints:

```bash
# Scale up replicas temporarily
kubectl scale deployment profile-service --replicas=5 -n default

# Check HPA status
kubectl get hpa profile-service-hpa -n default -o wide
```

### Post-Incident Actions

1. **Verify Resolution**

   - Monitor error rate for 15 minutes
   - Confirm all health checks pass
   - Validate task submission functionality

2. **Documentation**
   - Update incident log with root cause
   - Document any configuration changes
   - Schedule post-mortem if error rate exceeded 5%

---

## Queue Service Communication Failure

**Alert**: `QueueServiceCommunicationFailed`  
**Severity**: Critical  
**Threshold**: Any failed queue service requests  
**SLA Impact**: High

### Immediate Actions (2 minutes)

1. **Emergency Assessment**

   ```bash
   # Check queue service status immediately
   kubectl get pods -n default -l app=queue-service -o wide

   # Check if profile service can resolve queue service
   kubectl exec -n default deployment/profile-service -- nslookup queue-service
   ```

2. **Check Service Endpoints**

   ```bash
   # Verify service endpoints
   kubectl get endpoints queue-service -n default

   # Test direct connectivity
   kubectl exec -n default deployment/profile-service -- curl -v http://queue-service:8080/health
   ```

### Investigation Steps (5 minutes)

1. **Queue Service Health Check**

   ```bash
   # Check queue service logs
   kubectl logs -n default -l app=queue-service --tail=100

   # Check queue service resource usage
   kubectl top pods -n default -l app=queue-service
   ```

2. **Network Policy Verification**

   ```bash
   # Check network policies
   kubectl get networkpolicies -n default

   # Verify DNS resolution
   kubectl exec -n default deployment/profile-service -- cat /etc/resolv.conf
   ```

3. **RabbitMQ Backend Check**

   ```bash
   # Check RabbitMQ status (if accessible)
   kubectl exec -n default deployment/queue-service -- rabbitmqctl status

   # Check RabbitMQ connections
   kubectl exec -n default deployment/queue-service -- rabbitmqctl list_connections
   ```

### Resolution Actions

#### If queue service is down:

```bash
# Restart queue service
kubectl rollout restart deployment/queue-service -n default

# Monitor restart progress
kubectl rollout status deployment/queue-service -n default --timeout=300s
```

#### If network connectivity issues:

```bash
# Check and restart CoreDNS if needed
kubectl rollout restart deployment/coredns -n kube-system

# Verify service discovery
kubectl get services queue-service -n default -o yaml
```

#### If RabbitMQ backend issues:

```bash
# Restart RabbitMQ (if directly accessible)
kubectl rollout restart deployment/rabbitmq -n default

# Check RabbitMQ persistent volumes
kubectl get pv,pvc -n default | grep rabbitmq
```

### Circuit Breaker Response

The Profile Service has built-in circuit breakers. Monitor the circuit breaker status:

```bash
# Check circuit breaker metrics
curl -s http://profile-service:8080/metrics | grep circuit_breaker
```

### Post-Incident Actions

1. **Service Validation**

   ```bash
   # Test task submission end-to-end
   curl -X POST http://profile-service:8080/api/v1/profiles/test-123/tasks \
     -H "Authorization: Bearer $TOKEN" \
     -d '{"type": "profile_update", "payload": {"action": "test"}}'
   ```

2. **Performance Validation**
   - Verify response times are within SLA
   - Check task distribution across workers
   - Monitor for any message backlog

---

## High Response Time

**Alert**: `ProfileServiceHighResponseTime`  
**Severity**: Warning  
**Threshold**: 95th percentile > 50ms for 3 minutes  
**SLA Impact**: Medium

### Immediate Actions (3 minutes)

1. **Check Current Performance**

   ```bash
   # Check recent response times
   curl -s http://prometheus:9090/api/v1/query?query='histogram_quantile(0.95, rate(profile_api_request_duration_seconds_bucket[5m]))'

   # Check active connections
   kubectl exec -n default deployment/profile-service -- netstat -an | grep :8080
   ```

2. **Resource Utilization Check**

   ```bash
   # Check CPU and memory usage
   kubectl top pods -n default -l app=profile-service

   # Check if HPA is scaling
   kubectl get hpa profile-service-hpa -n default -o wide
   ```

### Investigation Steps (10 minutes)

1. **Performance Analysis**

   ```bash
   # Check slow query patterns
   kubectl logs -n default -l app=profile-service --tail=200 | grep -E "duration.*[0-9]{3,}ms"

   # Analyze endpoint performance
   curl -s http://profile-service:8080/metrics | grep duration | head -20
   ```

2. **Dependency Response Times**

   ```bash
   # Check dependency health with timing
   time kubectl exec -n default deployment/profile-service -- curl -s http://queue-service:8080/health
   time kubectl exec -n default deployment/profile-service -- curl -s http://storage-service:8080/health
   ```

3. **Database Performance** (if applicable)
   ```bash
   # Check database connection pool
   kubectl logs -n default -l app=storage-service --tail=100 | grep -i "connection\|pool"
   ```

### Resolution Actions

#### If CPU/Memory constrained:

```bash
# Scale up immediately
kubectl scale deployment profile-service --replicas=6 -n default

# Increase resource limits if needed (temporary)
kubectl patch deployment profile-service -n default -p '{"spec":{"template":{"spec":{"containers":[{"name":"profile-service","resources":{"limits":{"memory":"1Gi","cpu":"1000m"}}}]}}}}'
```

#### If dependency latency issues:

```bash
# Check and potentially restart slow dependencies
kubectl rollout restart deployment/storage-service -n default

# Scale dependencies if needed
kubectl scale deployment queue-service --replicas=3 -n default
```

#### If connection pool exhaustion:

```bash
# Restart service to reset connections
kubectl rollout restart deployment/profile-service -n default

# Monitor connection recovery
kubectl logs -n default -l app=profile-service -f | grep -i connection
```

### Performance Optimization

1. **Connection Pooling Check**

   ```bash
   # Verify HTTP connection pooling settings
   kubectl get configmap profile-service-config -n default -o yaml | grep -A5 -B5 HTTP
   ```

2. **Cache Performance**
   ```bash
   # Check Redis cache performance
   kubectl exec -n default deployment/redis-service -- redis-cli info stats
   ```

### Post-Incident Actions

1. **Trend Analysis**

   - Review performance trends over last 24 hours
   - Identify any degradation patterns
   - Plan capacity scaling if needed

2. **Optimization Review**
   - Review database query performance
   - Analyze API endpoint efficiency
   - Consider caching improvements

---

## Task Submission Failures

**Alert**: `ProfileServiceTaskSubmissionFailures`  
**Severity**: Warning  
**Threshold**: Task failure rate > 2% for 2 minutes  
**SLA Impact**: High

### Immediate Actions (3 minutes)

1. **Check Task Failure Patterns**

   ```bash
   # Check recent task submission logs
   kubectl logs -n default -l app=profile-service --tail=200 | grep -i "task.*fail\|error.*task"

   # Check task type distribution
   curl -s http://profile-service:8080/metrics | grep task_type
   ```

2. **Verify Worker Connectivity**

   ```bash
   # Check all worker services
   kubectl get pods -n default -l component=worker -o wide

   # Test routing key distribution
   kubectl logs -n default -l app=queue-service --tail=100 | grep routing_key
   ```

### Investigation Steps (10 minutes)

1. **Task Type Analysis**

   ```bash
   # Check which task types are failing
   kubectl logs -n default -l app=profile-service --tail=500 | grep -E "profile_update|email_notification|image_processing" | grep -i error
   ```

2. **Routing Key Validation**

   ```bash
   # Verify routing key mapping
   kubectl get configmap profile-service-routing-config -n default -o yaml

   # Check RabbitMQ queue status
   kubectl exec -n default deployment/queue-service -- rabbitmqctl list_queues name messages consumers
   ```

3. **Worker Health Check**
   ```bash
   # Check individual worker health
   curl -s http://profile-worker:8080/health | jq
   curl -s http://email-worker:8080/health | jq
   curl -s http://image-worker:8080/health | jq
   ```

### Resolution Actions

#### If specific worker type failing:

```bash
# Restart failing worker type
kubectl rollout restart deployment/profile-worker -n default  # or email-worker, image-worker

# Scale up worker if needed
kubectl scale deployment profile-worker --replicas=3 -n default
```

#### If routing configuration issue:

```bash
# Reload configuration
kubectl rollout restart deployment/profile-service -n default

# Verify configuration reload
kubectl logs -n default -l app=profile-service --tail=20 | grep -i config
```

#### If validation errors:

```bash
# Check payload validation logs
kubectl logs -n default -l app=profile-service --tail=200 | grep -i "validation\|payload"

# Test with known good payload
curl -X POST http://profile-service:8080/api/v1/profiles/test-123/tasks \
  -H "Content-Type: application/json" \
  -d '{"type": "profile_update", "payload": {"user_id": "test", "action": "update"}}'
```

### Post-Incident Actions

1. **End-to-End Testing**

   ```bash
   # Test each task type
   ./test/integration/run-integration-tests.sh
   ```

2. **Performance Validation**
   - Verify task success rate returns to > 98%
   - Check task processing times
   - Monitor worker queue depths

---

## Service Unavailable

**Alert**: `ProfileServiceUnavailable`  
**Severity**: Critical  
**Threshold**: Service down for 1 minute  
**SLA Impact**: Critical

### Immediate Actions (1 minute)

1. **Emergency Assessment**

   ```bash
   # Check pod status immediately
   kubectl get pods -n default -l app=profile-service -o wide

   # Check service endpoints
   kubectl get endpoints profile-service -n default
   ```

2. **Quick Health Check**

   ```bash
   # Test service reachability
   curl -f http://profile-service:8080/health || echo "Service unreachable"

   # Check recent events
   kubectl get events -n default --sort-by='.lastTimestamp' | tail -10
   ```

### Emergency Response (2 minutes)

#### If all pods are down:

```bash
# Emergency restart
kubectl rollout restart deployment/profile-service -n default

# Force new deployment if stuck
kubectl patch deployment profile-service -n default -p '{"spec":{"template":{"metadata":{"annotations":{"kubectl.kubernetes.io/restartedAt":"'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"}}}}}'
```

#### If pods crash looping:

```bash
# Get crash details
kubectl describe pods -n default -l app=profile-service | grep -A10 -B10 "CrashLoopBackOff\|Error\|Failed"

# Check logs from crashed pod
kubectl logs -n default -l app=profile-service --previous --tail=50
```

#### If resource constraints:

```bash
# Emergency scale up
kubectl scale deployment profile-service --replicas=2 -n default

# Check node resources
kubectl describe nodes | grep -A10 "Allocated resources"
```

### Investigation Steps (5 minutes)

1. **Root Cause Analysis**

   ```bash
   # Check container logs
   kubectl logs -n default -l app=profile-service --tail=100

   # Check resource limits
   kubectl describe deployment profile-service -n default | grep -A10 resources
   ```

2. **Dependency Check**

   ```bash
   # Verify all dependencies are healthy
   kubectl get pods -n default | grep -E "queue-service|storage-service|auth-service|redis"
   ```

3. **Configuration Validation**
   ```bash
   # Check ConfigMap and Secret availability
   kubectl get configmap profile-service-config -n default
   kubectl get secret profile-service-secrets -n default
   ```

### Resolution Actions

#### For configuration issues:

```bash
# Emergency rollback
./deployments/scripts/rollback-procedures.sh emergency
```

#### For resource issues:

```bash
# Increase resource limits
kubectl patch deployment profile-service -n default -p '{"spec":{"template":{"spec":{"containers":[{"name":"profile-service","resources":{"limits":{"memory":"1Gi","cpu":"1000m"}}}]}}}}'
```

#### For dependency failures:

```bash
# Restart critical dependencies
kubectl rollout restart deployment/queue-service -n default
kubectl rollout restart deployment/storage-service -n default
```

### Post-Incident Actions

1. **Service Validation**

   ```bash
   # Comprehensive health check
   ./deployments/scripts/rollback-procedures.sh health

   # Test critical functionality
   curl -X POST http://profile-service:8080/api/v1/profiles/test/tasks \
     -H "Authorization: Bearer $TOKEN" \
     -d '{"type": "profile_update", "payload": {"action": "test"}}'
   ```

2. **Incident Documentation**
   - Record downtime duration
   - Document root cause
   - Update runbook with lessons learned
   - Schedule post-mortem meeting

---

## Monitoring Dashboard Links

### Grafana Dashboards

- **Profile Service Overview**: `http://grafana:3000/d/profile-service/profile-service-multi-worker-architecture`
- **Task Distribution**: `http://grafana:3000/d/task-distribution/task-routing-analysis`
- **Performance Metrics**: `http://grafana:3000/d/performance/profile-service-performance`

### Prometheus Queries

#### Key Metrics Queries

```promql
# Error Rate
rate(profile_api_requests_total{status=~"5.."}[5m]) / rate(profile_api_requests_total[5m])

# Response Time (95th percentile)
histogram_quantile(0.95, rate(profile_api_request_duration_seconds_bucket[5m]))

# Task Distribution
sum by (routing_key) (rate(profile_tasks_routing_key_distribution[5m]))

# Queue Service Health
up{job="queue-service"}

# Throughput
rate(profile_api_requests_total[5m])
```

---

## Escalation Procedures

### Severity Levels

**Critical (Page immediately)**

- Service completely unavailable
- Queue service communication failures
- Error rate > 10%

**Warning (Notify during business hours)**

- High response times
- Task submission failures
- Resource usage above 90%

**Info (Log only)**

- Performance degradation
- Configuration changes
- Planned maintenance

### Contact Information

- **On-call Engineer**: Use PagerDuty rotation
- **Platform Team**: Slack #platform-team
- **Backend Team**: Slack #backend-team
- **DevOps Team**: Slack #devops-team

### Escalation Matrix

1. **Level 1** (0-15 minutes): On-call engineer
2. **Level 2** (15-30 minutes): Platform team lead
3. **Level 3** (30+ minutes): Engineering manager

---

## Preventive Maintenance

### Daily Checks

- Review error rates and trends
- Check resource utilization
- Verify backup and monitoring systems

### Weekly Checks

- Review performance trends
- Update capacity planning
- Test rollback procedures

### Monthly Checks

- Review and update runbooks
- Conduct chaos engineering tests
- Update escalation procedures

This operational runbook provides comprehensive guidance for maintaining and troubleshooting the Profile Service's multi-worker architecture in production environments.
