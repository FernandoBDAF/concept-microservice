# Profile Service Troubleshooting Guide

## Overview

This document provides comprehensive troubleshooting guidance for the Profile Service's multi-worker architecture integration, covering common issues, diagnostic steps, and resolution strategies.

## Quick Diagnostic Commands

### Health Check

```bash
# Service health
curl -s http://profile-service:8080/health | jq

# Queue service connectivity
curl -s http://queue-service:8080/health | jq

# Check service logs
kubectl logs -f deployment/profile-service --tail=100
```

### Message Flow Verification

```bash
# Submit test task and monitor logs
curl -X POST http://profile-service:8080/api/v1/profiles/test-123/tasks \
  -H "Authorization: Bearer ${TOKEN}" \
  -d '{"type": "profile_update", "payload": {"action": "test"}}'

# Monitor queue service logs for message routing
kubectl logs -f deployment/queue-service --tail=50 | grep "profile.task"
```

---

## Common Issues & Resolutions

### 1. Task Submission Failures

#### Issue: "Profile task payload must be an object"

**Symptoms:**

```json
{
  "error": {
    "code": "INVALID_TASK_PAYLOAD",
    "message": "Profile task payload must be an object"
  }
}
```

**Root Cause:** Payload type mismatch in task request validation

**Resolution:**

```bash
# ✅ Correct format
curl -X POST http://profile-service:8080/api/v1/profiles/123/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "type": "profile_update",
    "payload": {
      "user_id": "123",
      "action": "update"
    }
  }'

# ❌ Incorrect format (payload as string)
# "payload": "invalid_string_payload"
```

**Prevention:**

- Always use object notation for payload
- Validate JSON structure before sending

#### Issue: "Unsupported task type"

**Symptoms:**

```json
{
  "error": {
    "code": "INVALID_TASK_TYPE",
    "message": "Unsupported task type: invalid_type",
    "details": {
      "supported_types": [
        "profile_update",
        "email_notification",
        "image_processing"
      ]
    }
  }
}
```

**Root Cause:** Invalid task type in request

**Resolution:**

```bash
# ✅ Valid task types
"type": "profile_update"      # Routes to profile.task
"type": "email_notification"  # Routes to email.send
"type": "image_processing"    # Routes to image.process

# ❌ Invalid task types
"type": "invalid_type"
"type": "background_job"      # Deprecated
"type": "cache_invalidation"  # Deprecated
```

**Prevention:**

- Use only supported task types
- Check API documentation for valid types

### 2. Queue Service Communication Issues

#### Issue: "Queue service unavailable"

**Symptoms:**

```
ERROR: Failed to publish message: connection refused
ERROR: Queue service health check failed
```

**Diagnostic Steps:**

```bash
# 1. Check queue service status
kubectl get pods -l app=queue-service

# 2. Check service endpoints
kubectl get svc queue-service

# 3. Test connectivity
kubectl exec -it deployment/profile-service -- curl http://queue-service:8080/health

# 4. Check network policies
kubectl get networkpolicies
```

**Resolution:**

```bash
# 1. Restart queue service if unhealthy
kubectl rollout restart deployment/queue-service

# 2. Verify service configuration
kubectl describe svc queue-service

# 3. Check environment variables
kubectl get deployment profile-service -o yaml | grep -A5 -B5 QUEUE_SERVICE_URL
```

#### Issue: "Message publishing timeout"

**Symptoms:**

```
ERROR: Context deadline exceeded while publishing message
WARNING: Queue service response time: 35000ms (threshold: 30000ms)
```

**Root Cause:** Queue service overloaded or network latency

**Resolution:**

```bash
# 1. Check queue service performance
kubectl top pods -l app=queue-service

# 2. Scale queue service if needed
kubectl scale deployment queue-service --replicas=3

# 3. Increase timeout (temporary)
kubectl set env deployment/profile-service QUEUE_SERVICE_TIMEOUT=60s

# 4. Check RabbitMQ status
kubectl exec -it deployment/queue-service -- rabbitmqctl status
```

### 3. Routing Key Issues

#### Issue: Messages not reaching correct worker

**Symptoms:**

```
# Profile tasks being processed by email worker
ERROR: Unexpected message type 'profile_update' in email worker

# Or messages being ignored
WARNING: No worker available for routing key 'invalid.key'
```

**Diagnostic Steps:**

```bash
# 1. Verify routing key mapping
curl -s http://profile-service:8080/health | jq '.dependencies.queue_service'

# 2. Check RabbitMQ queue bindings
kubectl exec -it deployment/queue-service -- rabbitmqctl list_bindings

# 3. Monitor message routing
kubectl logs -f deployment/queue-service | grep "routing_key"
```

**Resolution:**

```bash
# 1. Verify correct routing keys in profile service
kubectl exec -it deployment/profile-service -- env | grep ROUTING

# 2. Check worker queue configurations
kubectl get configmap worker-config -o yaml

# 3. Restart services to reload routing configuration
kubectl rollout restart deployment/profile-service
kubectl rollout restart deployment/queue-service
```

### 4. Authentication & Authorization Issues

#### Issue: "Invalid token" errors

**Symptoms:**

```json
{
  "error": {
    "code": "INVALID_TOKEN",
    "message": "JWT token invalid or expired"
  }
}
```

**Diagnostic Steps:**

```bash
# 1. Verify token format
echo $TOKEN | cut -d'.' -f2 | base64 -d | jq

# 2. Check auth service status
kubectl get pods -l app=auth-service

# 3. Test token validation
curl -H "Authorization: Bearer $TOKEN" http://auth-service:8080/validate
```

**Resolution:**

```bash
# 1. Get fresh token
TOKEN=$(curl -X POST http://auth-service:8080/login \
  -d '{"username":"user","password":"pass"}' | jq -r '.token')

# 2. Check token expiration
jwt-cli decode $TOKEN

# 3. Verify auth service configuration
kubectl describe deployment auth-service
```

### 5. Performance Issues

#### Issue: High response times

**Symptoms:**

```
WARNING: API response time: 2500ms (threshold: 50ms)
ERROR: Request timeout after 30s
```

**Diagnostic Steps:**

```bash
# 1. Check service resource usage
kubectl top pods profile-service

# 2. Check dependency response times
curl -s http://profile-service:8080/health | jq '.dependencies'

# 3. Monitor request patterns
kubectl logs -f deployment/profile-service | grep "duration"
```

**Resolution:**

```bash
# 1. Scale profile service
kubectl scale deployment profile-service --replicas=5

# 2. Increase resource limits
kubectl patch deployment profile-service -p '{"spec":{"template":{"spec":{"containers":[{"name":"profile-service","resources":{"limits":{"memory":"512Mi","cpu":"500m"}}}]}}}}'

# 3. Enable connection pooling
kubectl set env deployment/profile-service HTTP_MAX_IDLE_CONNS=100

# 4. Check database performance
kubectl exec -it deployment/storage-service -- pg_stat_activity
```

---

## Message Format Debugging

### Verify Message Structure

**Expected Format:**

```json
{
  "id": "uuid",
  "type": "profile_update|email_notification|image_processing",
  "payload": {...},
  "timestamp": "2024-01-01T00:00:00Z",
  "metadata": {
    "profile_id": "uuid",
    "source": "profile-service"
  },
  "routing_key": "profile.task|email.send|image.process"
}
```

**Validation Commands:**

```bash
# 1. Enable debug logging
kubectl set env deployment/profile-service LOG_LEVEL=debug

# 2. Monitor message creation
kubectl logs -f deployment/profile-service | grep "message_created"

# 3. Check message format in queue
kubectl exec -it deployment/queue-service -- rabbitmqctl list_queues name messages
```

### Common Format Issues

#### Issue: "Payload field missing"

```bash
# ❌ Missing payload
{"type": "profile_update"}

# ✅ Correct format
{"type": "profile_update", "payload": {"action": "update"}}
```

#### Issue: "Invalid timestamp format"

```bash
# ❌ String timestamp
{"timestamp": "2024-01-01 00:00:00"}

# ✅ ISO 8601 format
{"timestamp": "2024-01-01T00:00:00Z"}
```

---

## Worker-Specific Troubleshooting

### Profile Worker Issues

**Common Problems:**

```bash
# 1. Profile not found
ERROR: Profile with ID 'uuid' not found in storage service

# 2. Invalid profile data
ERROR: Profile validation failed: email format invalid

# 3. Storage service unavailable
ERROR: Failed to connect to storage service
```

**Resolution:**

```bash
# 1. Verify profile exists
curl -H "Authorization: Bearer $TOKEN" \
  http://profile-service:8080/api/v1/profiles/uuid

# 2. Check storage service
kubectl get pods -l app=storage-service

# 3. Test storage connectivity
kubectl exec -it deployment/profile-service -- \
  curl http://storage-service:8080/health
```

### Email Worker Issues

**Common Problems:**

```bash
# 1. Invalid email template
ERROR: Template 'unknown_template' not found

# 2. SMTP configuration issues
ERROR: Failed to connect to SMTP server

# 3. Missing template variables
ERROR: Required variable 'user_name' not provided
```

**Resolution:**

```bash
# 1. Check available templates
kubectl get configmap email-templates -o yaml

# 2. Verify SMTP settings
kubectl get secret smtp-config -o yaml | base64 -d

# 3. Validate email payload
echo '{"to":"test@example.com","template":"welcome","variables":{"user_name":"Test"}}' | jq
```

### Image Worker Issues

**Common Problems:**

```bash
# 1. Image URL inaccessible
ERROR: Failed to download image from URL

# 2. Unsupported image format
ERROR: Image format 'bmp' not supported

# 3. Processing timeout
ERROR: Image processing timeout after 10 minutes
```

**Resolution:**

```bash
# 1. Test image URL accessibility
curl -I https://example.com/image.jpg

# 2. Check supported formats
kubectl get configmap image-config -o yaml | grep formats

# 3. Increase processing timeout
kubectl set env deployment/image-worker IMAGE_PROCESSING_TIMEOUT=20m
```

---

## Monitoring & Alerting Issues

### Metrics Not Appearing

**Symptoms:**

```bash
# Prometheus metrics endpoint empty
curl http://profile-service:8080/metrics | wc -l
# Output: 0
```

**Resolution:**

```bash
# 1. Verify metrics enabled
kubectl get deployment profile-service -o yaml | grep METRICS_ENABLED

# 2. Check metrics endpoint
kubectl exec -it deployment/profile-service -- curl localhost:8080/metrics

# 3. Restart service to reload metrics
kubectl rollout restart deployment/profile-service
```

### Log Aggregation Issues

**Symptoms:**

```bash
# Logs not appearing in centralized logging
kubectl logs deployment/profile-service | wc -l
# Output: 0
```

**Resolution:**

```bash
# 1. Check log level
kubectl get deployment profile-service -o yaml | grep LOG_LEVEL

# 2. Verify log format
kubectl set env deployment/profile-service LOG_FORMAT=json

# 3. Check log shipping
kubectl get pods -n logging-system
```

---

## Emergency Procedures

### Service Recovery

**Complete Service Restart:**

```bash
# 1. Scale down to 0
kubectl scale deployment profile-service --replicas=0

# 2. Wait for termination
kubectl get pods -l app=profile-service

# 3. Scale back up
kubectl scale deployment profile-service --replicas=3

# 4. Verify health
kubectl get pods -l app=profile-service
curl http://profile-service:8080/health
```

**Rollback to Previous Version:**

```bash
# 1. Check rollout history
kubectl rollout history deployment/profile-service

# 2. Rollback to previous version
kubectl rollout undo deployment/profile-service

# 3. Verify rollback
kubectl rollout status deployment/profile-service
```

### Queue Recovery

**Clear Stuck Messages:**

```bash
# 1. Access RabbitMQ management
kubectl port-forward svc/rabbitmq-management 15672:15672

# 2. Purge specific queue
kubectl exec -it deployment/queue-service -- \
  rabbitmqctl purge_queue profile.task.queue

# 3. Restart workers
kubectl rollout restart deployment/profile-worker
```

**Reset RabbitMQ:**

```bash
# 1. Stop all workers
kubectl scale deployment profile-worker --replicas=0
kubectl scale deployment email-worker --replicas=0
kubectl scale deployment image-worker --replicas=0

# 2. Reset RabbitMQ
kubectl delete pod -l app=rabbitmq

# 3. Wait for RabbitMQ recovery
kubectl wait --for=condition=ready pod -l app=rabbitmq --timeout=300s

# 4. Restart workers
kubectl scale deployment profile-worker --replicas=2
kubectl scale deployment email-worker --replicas=2
kubectl scale deployment image-worker --replicas=2
```

---

## Preventive Measures

### Health Monitoring

**Set up comprehensive monitoring:**

```yaml
# monitoring/profile-service-monitor.yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: profile-service
spec:
  selector:
    matchLabels:
      app: profile-service
  endpoints:
    - port: metrics
      interval: 30s
      path: /metrics
```

### Automated Alerts

**Key alerts to configure:**

```yaml
# Alert for high error rate
- alert: ProfileServiceHighErrorRate
  expr: rate(profile_api_requests_total{status=~"5.."}[5m]) > 0.1
  for: 2m
  labels:
    severity: warning
  annotations:
    summary: "Profile Service error rate is high"

# Alert for queue communication failures
- alert: QueueServiceCommunicationFailed
  expr: profile_queue_service_requests_total{status="error"} > 0
  for: 1m
  labels:
    severity: critical
  annotations:
    summary: "Profile Service cannot communicate with Queue Service"
```

### Regular Maintenance

**Weekly checks:**

```bash
#!/bin/bash
# weekly-maintenance.sh

# 1. Check service health
kubectl get pods -l app=profile-service
kubectl get pods -l app=queue-service

# 2. Verify routing key distribution
kubectl logs deployment/profile-service --tail=1000 | grep "routing_key" | sort | uniq -c

# 3. Check message queue depths
kubectl exec -it deployment/queue-service -- rabbitmqctl list_queues

# 4. Review error logs
kubectl logs deployment/profile-service --tail=1000 | grep ERROR

# 5. Performance metrics review
curl -s http://profile-service:8080/metrics | grep duration
```

---

## Contact & Escalation

### Support Levels

**Level 1 - Service Issues:**

- API response problems
- Authentication failures
- Basic configuration issues

**Level 2 - Integration Issues:**

- Queue service communication
- Worker routing problems
- Message format issues

**Level 3 - Architecture Issues:**

- Multi-worker coordination
- Performance optimization
- Complex troubleshooting

### Documentation References

- **API Reference**: `/docs/INTERFACE.md`
- **Architecture Details**: `/docs/CONTEXT.md`
- **Testing Guide**: `/docs/TESTING.md`
- **Service README**: `/README.md`

### Emergency Contacts

- **Platform Team**: For infrastructure and Kubernetes issues
- **Backend Team**: For service logic and integration problems
- **DevOps Team**: For deployment and configuration issues

This troubleshooting guide provides comprehensive coverage for diagnosing and resolving issues in the Profile Service's multi-worker architecture integration.
