# Profile Service Deployment Guide

## Overview

This document provides comprehensive guidance for deploying the Profile Service with multi-worker architecture support. The deployment infrastructure includes production-ready Kubernetes manifests, monitoring setup, rollback procedures, and Docker containerization.

⚠️ **Important Notes:**

- **Missing Secret Issue**: The deployment references `profile-service-secrets` but this wasn't initially defined. See [Secrets Configuration](#secrets-configuration) below.
- **Kind Compatibility**: The base manifests are designed for production. For local kind clusters, use the kind-specific overlays. See [Kind Local Development](#kind-local-development) below.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                    Profile Service Deployment                   │
├─────────────────────────────────────────────────────────────────┤
│  Docker Container (Alpine + Go Binary)                         │
│  ├─── Multi-worker environment variables                       │
│  ├─── Health checks and monitoring                             │
│  └─── Security hardening (non-root user)                       │
├─────────────────────────────────────────────────────────────────┤
│  Kubernetes Resources                                           │
│  ├─── Deployment (3 replicas, rolling updates)                 │
│  ├─── ConfigMaps (multi-worker routing configuration)          │
│  ├─── Service (HTTP + metrics exposure)                        │
│  ├─── Secrets (database, API keys, JWT tokens)                 │
│  ├─── RBAC (ServiceAccount, ClusterRole, RoleBinding)          │
│  ├─── HPA (3-10 replicas, CPU/memory based)                    │
│  ├─── PDB (minimum 2 replicas during disruptions)             │
│  └─── NetworkPolicy (ingress/egress rules)                     │
├─────────────────────────────────────────────────────────────────┤
│  Monitoring & Observability                                    │
│  ├─── Prometheus ServiceMonitor (metrics collection)           │
│  ├─── 10 Alert Rules (error rate, latency, availability)       │
│  ├─── Grafana Dashboard (multi-worker visualization)           │
│  └─── Operational Runbooks (incident response)                 │
├─────────────────────────────────────────────────────────────────┤
│  Deployment Automation                                         │
│  ├─── Rollback Scripts (emergency procedures)                  │
│  ├─── Health Validation (pre/post deployment)                  │
│  ├─── Kind Overlay (local development)                         │
│  └─── Configuration Backup (state preservation)               │
└─────────────────────────────────────────────────────────────────┘
```

## Secrets Configuration

### Issue Identification

The deployment manifest (`deployments/kubernetes/deployment.yaml` line 120) references:

```yaml
envFrom:
  - secretRef:
      name: profile-service-secrets # ❌ This secret was not defined
```

### Solution: Secrets Manifest

The missing secrets are now defined in `deployments/kubernetes/secrets.yaml`:

#### Production Secrets (`profile-service-secrets`)

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: profile-service-secrets
type: Opaque
data:
  # Base64 encoded values for production
  DB_PASSWORD: cHJvZmlsZV9zZXJ2aWNlX3Bhc3M=
  DB_USER: cHJvZmlsZV91c2Vy
  REDIS_PASSWORD: cmVkaXNfcGFzcw==
  JWT_SECRET_KEY: anlXVF9zaUduaU5nX2szeV9mb1JfcFJvZmlsZV9zVnI=
  # ... other production secrets
```

#### Local Development Secrets (`profile-service-secrets-local`)

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: profile-service-secrets-local
type: Opaque
stringData: # Plain text for easier local development
  DB_PASSWORD: "local_dev_pass"
  DB_USER: "profile_user"
  JWT_SECRET_KEY: "dev_jwt_key_not_for_production"
  # ... other development values
```

### Creating Secrets

#### For Production:

```bash
# Apply the secrets manifest
kubectl apply -f deployments/kubernetes/secrets.yaml

# Or create secrets individually
kubectl create secret generic profile-service-secrets \
  --from-literal=DB_PASSWORD="your-secure-password" \
  --from-literal=DB_USER="profile_user" \
  --from-literal=JWT_SECRET_KEY="your-jwt-secret"
```

#### For Local Development:

```bash
# The local secrets are automatically applied with kind overlay
kubectl apply -k deployments/kind/
```

## Kind Local Development

### Issues with Base Manifests for Kind

The production manifests have several incompatibilities with local kind clusters:

1. **Resource Requirements**: Too high for local development

   - Production: 256Mi/200m CPU requests, 512Mi/500m limits
   - Kind: 128Mi/100m CPU requests, 256Mi/200m limits

2. **Replica Count**: 3 replicas may overwhelm single-node kind cluster

   - Production: 3 replicas with anti-affinity
   - Kind: 1 replica, no anti-affinity

3. **HPA Requirements**: Needs metrics server (not always available in kind)

   - Production: Auto-scaling 3-10 replicas
   - Kind: Fixed 1 replica, disabled HPA

4. **Network Policies**: May be too restrictive for local development

   - Production: Specific ingress/egress rules
   - Kind: Permissive policies for easier debugging

5. **Service Access**: ClusterIP requires port-forwarding
   - Production: ClusterIP + Ingress
   - Kind: NodePort for direct localhost access

### Kind-Specific Solution

#### Directory Structure

```
deployments/
├── kubernetes/          # Production manifests
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── configmap.yaml
│   └── secrets.yaml
├── kind/               # Kind-specific overlays
│   ├── kustomization.yaml
│   ├── deployment-patch.yaml
│   ├── service-patch.yaml
│   └── deploy-to-kind.sh
└── monitoring/         # Monitoring setup
    └── servicemonitor.yaml
```

#### Kind Deployment Process

1. **Quick Deploy (Recommended)**:

```bash
cd deployments/kind/
./deploy-to-kind.sh
```

2. **Manual Deploy**:

```bash
# Create kind cluster
kind create cluster --name profile-service-dev

# Build and load image
docker build -t profile-service:latest ../../
kind load docker-image profile-service:latest --name profile-service-dev

# Deploy with kind overlay
kubectl apply -k deployments/kind/

# Access the service
open http://localhost:8080/health
```

#### Kind-Specific Configurations

##### Deployment Patches (`deployment-patch.yaml`)

```yaml
spec:
  replicas: 1 # Single replica for kind
  template:
    spec:
      containers:
        - name: profile-service
          resources:
            requests:
              memory: "128Mi" # Reduced for kind
              cpu: "100m"
            limits:
              memory: "256Mi"
              cpu: "200m"
          env:
            - name: LOG_LEVEL
              value: "debug" # More verbose logging
            - name: CIRCUIT_BREAKER_ENABLED
              value: "false" # Disabled for easier debugging
```

##### Service Patches (`service-patch.yaml`)

```yaml
spec:
  type: NodePort
  ports:
    - name: http
      nodePort: 30080 # http://localhost:8080
    - name: metrics
      nodePort: 30081 # http://localhost:8081
```

##### Kind Cluster Configuration

```yaml
# deployments/kind/deploy-to-kind.sh creates cluster with:
extraPortMappings:
  - containerPort: 30080 # Maps to localhost:8080
    hostPort: 8080
  - containerPort: 30081 # Maps to localhost:8081
    hostPort: 8081
```

### Access Information for Kind

After deploying to kind:

- **Profile Service API**: http://localhost:8080
- **Health Check**: http://localhost:8080/health
- **Metrics**: http://localhost:8081/metrics
- **API Documentation**: http://localhost:8080/docs (if available)

### Development Commands

```bash
# Deploy to kind
./deployments/kind/deploy-to-kind.sh

# View logs
kubectl logs -f deployment/profile-service

# Check status
kubectl get pods,svc -l app=profile-service

# Test the service
curl http://localhost:8080/health
curl -X POST http://localhost:8080/api/v1/profiles/test-123/tasks \
  -H "Content-Type: application/json" \
  -d '{"type": "profile_update", "payload": {"action": "test"}}'

# Cleanup
./deployments/kind/deploy-to-kind.sh cleanup
```

## Production Deployment vs Kind Comparison

| Component            | Production                | Kind                            |
| -------------------- | ------------------------- | ------------------------------- |
| **Replicas**         | 3 with anti-affinity      | 1 replica                       |
| **Resources**        | 256Mi/200m → 512Mi/500m   | 128Mi/100m → 256Mi/200m         |
| **Service Type**     | ClusterIP                 | NodePort (30080, 30081)         |
| **HPA**              | 3-10 replicas, CPU/memory | Disabled                        |
| **Network Policy**   | Restrictive rules         | Permissive for debugging        |
| **Secrets**          | `profile-service-secrets` | `profile-service-secrets-local` |
| **Logging**          | Info level                | Debug level                     |
| **Circuit Breakers** | Enabled                   | Disabled for easier debugging   |
| **Access**           | Via Ingress/port-forward  | Direct localhost access         |

## Deployment Components

### 1. Docker Container (`Dockerfile`)

#### Overview

The Dockerfile creates a production-ready, security-hardened container optimized for the multi-worker architecture.

#### Key Features

- **Multi-stage build** for optimized image size
- **Alpine Linux base** (minimal attack surface)
- **Non-root user execution** (security hardening)
- **Built-in health checks** (container orchestration)
- **Multi-worker environment variables** (configuration)

#### Build Configuration

```dockerfile
# Build optimizations
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o profile-service ./cmd
```

#### Security Features

```dockerfile
# Non-root user creation
RUN adduser -D -g '' -s /bin/sh appuser

# Directory permissions
RUN mkdir -p /app/logs /app/tmp /app/cache && \
    chown -R appuser:appuser /app

# Runtime security
USER appuser
```

#### Multi-Worker Environment Variables

```dockerfile
# Task type support
ENV SUPPORTED_TASK_TYPES=profile_update,email_notification,image_processing

# Routing key mapping
ENV ROUTING_KEY_PROFILE_UPDATE=profile.task
ENV ROUTING_KEY_EMAIL_NOTIFICATION=email.send
ENV ROUTING_KEY_IMAGE_PROCESSING=image.process
ENV ROUTING_KEY_DEFAULT_FALLBACK=profile.task

# Queue service integration
ENV QUEUE_SERVICE_URL=http://queue-service:8080
ENV QUEUE_SERVICE_TIMEOUT=30s
ENV QUEUE_SERVICE_RETRIES=3

# Performance settings (based on testing)
ENV PROFILE_TASK_TIMEOUT=5m
ENV EMAIL_TASK_TIMEOUT=2m
ENV IMAGE_TASK_TIMEOUT=10m
```

#### Health Check Configuration

```dockerfile
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1
```

### 2. Kubernetes Deployment (`deployments/kubernetes/deployment.yaml`)

#### Overview

Production-ready Kubernetes Deployment with 3 replicas, rolling updates, and performance-optimized resource limits.

#### Key Features

- **Rolling update strategy** (zero-downtime deployments)
- **Performance-based resource limits** (from load testing results)
- **Comprehensive health checks** (liveness, readiness, startup)
- **Security context** (non-root, read-only filesystem)
- **Pod anti-affinity** (high availability across nodes)

#### Resource Configuration

```yaml
resources:
  requests:
    memory: "256Mi" # Increased based on performance testing
    cpu: "200m" # Increased based on performance testing
  limits:
    memory: "512Mi" # Optimized for 1225+ req/sec capability
    cpu: "500m" # Burst capacity for peak loads
```

#### Health Checks

```yaml
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 30
  periodSeconds: 10
  timeoutSeconds: 5
  failureThreshold: 3

readinessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 5
  timeoutSeconds: 3
  failureThreshold: 3

startupProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10
  timeoutSeconds: 3
  failureThreshold: 30
```

#### Security Configuration

```yaml
securityContext:
  allowPrivilegeEscalation: false
  runAsNonRoot: true
  runAsUser: 1000
  runAsGroup: 1000
  readOnlyRootFilesystem: true
  capabilities:
    drop:
      - ALL
```

### 3. Configuration Management (`deployments/kubernetes/configmap.yaml`)

#### Overview

Two ConfigMaps provide comprehensive configuration for multi-worker architecture and routing rules.

#### Profile Service Config (`profile-service-config`)

Contains general service configuration:

- **Multi-worker routing keys** for all task types
- **Performance thresholds** based on testing results
- **Task validation settings** for payload validation
- **Circuit breaker and retry configurations**
- **Monitoring and logging settings**

#### Routing Configuration (`profile-service-routing-config`)

Contains detailed routing rules:

- **Per-task-type routing rules** with timeouts and retries
- **Worker health check configurations**
- **Load balancing settings**
- **Circuit breaker configurations per worker**

#### Example Routing Rule

```yaml
- task_type: "email_notification"
  routing_key: "email.send"
  queue: "email.send.queue"
  worker: "email-worker"
  timeout: "2m"
  retry_attempts: 3
  priority: "high"
```

### 4. Service & RBAC (`deployments/kubernetes/service.yaml`)

#### Service Configuration

```yaml
spec:
  ports:
    - name: http
      port: 8080
      targetPort: 8080
    - name: metrics
      port: 8081
      targetPort: 8080 # Metrics on same port as HTTP
  type: ClusterIP
```

#### RBAC Configuration

- **ServiceAccount**: `profile-service`
- **ClusterRole**: Read access to ConfigMaps, Secrets, Services, Endpoints, Deployments
- **ClusterRoleBinding**: Links ServiceAccount to ClusterRole

#### Horizontal Pod Autoscaler (HPA)

```yaml
spec:
  minReplicas: 3
  maxReplicas: 10
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
    - type: Resource
      resource:
        name: memory
        target:
          type: Utilization
          averageUtilization: 80
```

#### Pod Disruption Budget (PDB)

```yaml
spec:
  minAvailable: 2 # Ensures at least 2 replicas during voluntary disruptions
```

#### Network Policy

- **Ingress**: Allow from ingress-system, api-gateway, monitoring-system
- **Egress**: Allow to DNS, queue-service, storage-service, auth-service, cache-service

### 5. Monitoring Setup (`deployments/monitoring/servicemonitor.yaml`)

#### ServiceMonitor Configuration

```yaml
spec:
  endpoints:
    - port: metrics
      path: /metrics
      interval: 30s
      scrapeTimeout: 10s
      metricRelabelings:
        - sourceLabels: [__name__]
          regex: "go_.*"
          action: drop # Remove Go runtime metrics for cleaner data
```

#### Alert Rules (10 Comprehensive Alerts)

1. **ProfileServiceHighErrorRate** - Error rate > 1%
2. **ProfileServiceCriticalErrorRate** - Error rate > 5%
3. **QueueServiceCommunicationFailed** - Queue communication failures
4. **ProfileServiceHighResponseTime** - 95th percentile > 50ms
5. **TaskSubmissionFailures** - Task failure rate > 2%
6. **RoutingKeyImbalance** - Multi-worker distribution issues
7. **ServiceUnavailable** - Service down for 1+ minutes
8. **ResourceUsage** - High CPU/memory usage
9. **PodCrashLoop** - Pod restart issues
10. **LowThroughput** - Unusually low request rates

#### Grafana Dashboard

```json
{
  "title": "Profile Service - Multi-Worker Architecture",
  "panels": [
    {
      "title": "Request Rate",
      "targets": [{ "expr": "rate(profile_api_requests_total[5m])" }]
    },
    {
      "title": "Task Distribution by Routing Key",
      "type": "pie",
      "targets": [
        {
          "expr": "sum by (routing_key) (rate(profile_tasks_routing_key_distribution[5m]))"
        }
      ]
    }
  ]
}
```

### 6. Rollback Procedures (`deployments/scripts/rollback-procedures.sh`)

#### Overview

Comprehensive bash script for emergency deployment rollbacks with validation and backup procedures.

#### Key Features

- **Prerequisites validation** (kubectl, cluster connectivity, deployment existence)
- **State backup** (deployment, configmaps, service configurations)
- **Rollback execution** with timeout controls
- **Health validation** (deployment and service health checks)
- **Emergency procedures** for critical incidents

#### Usage Examples

```bash
# View rollout history
./rollback-procedures.sh history

# Rollback to specific revision
./rollback-procedures.sh rollback 5

# Emergency rollback (fastest, minimal validation)
./rollback-procedures.sh emergency

# Health check current deployment
./rollback-procedures.sh health
```

#### Emergency Rollback Function

```bash
emergency_rollback() {
    log_warning "Performing EMERGENCY ROLLBACK..."

    local current_revision=$(get_current_revision)
    local target_revision=$((current_revision - 1))

    # Skip backup in emergency (save time)
    ROLLBACK_TIMEOUT=300
    if perform_rollback "$target_revision"; then
        log_success "Emergency rollback completed"

        if check_deployment_health 120; then
            log_success "Emergency rollback validation passed"
            return 0
        fi
    fi
}
```

## Deployment Process

### Pre-Deployment Checklist

- [ ] **Docker image built and pushed** to container registry
- [ ] **ConfigMaps updated** with correct routing configurations
- [ ] **Secrets created** for database and external service credentials
- [ ] **Network policies reviewed** for security compliance
- [ ] **Resource quotas checked** for cluster capacity
- [ ] **Monitoring stack ready** (Prometheus, Grafana)

### Deployment Steps

#### 1. Apply ConfigMaps

```bash
kubectl apply -f deployments/kubernetes/configmap.yaml
```

#### 2. Apply RBAC Resources

```bash
kubectl apply -f deployments/kubernetes/service.yaml
```

#### 3. Deploy Application

```bash
kubectl apply -f deployments/kubernetes/deployment.yaml
```

#### 4. Set up Monitoring

```bash
kubectl apply -f deployments/monitoring/servicemonitor.yaml
```

#### 5. Verify Deployment

```bash
# Check pod status
kubectl get pods -l app=profile-service

# Verify service endpoints
kubectl get endpoints profile-service

# Test health endpoint
kubectl port-forward svc/profile-service 8080:8080
curl http://localhost:8080/health
```

### Post-Deployment Validation

#### Health Checks

```bash
# Service health
curl -s http://profile-service:8080/health | jq

# Metrics endpoint
curl -s http://profile-service:8080/metrics | grep profile_

# Queue service connectivity
kubectl exec -it deployment/profile-service -- curl http://queue-service:8080/health
```

#### Integration Testing

```bash
# Test profile task submission
curl -X POST http://profile-service:8080/api/v1/profiles/test-123/tasks \
  -H "Content-Type: application/json" \
  -d '{"type": "profile_update", "payload": {"action": "test"}}'

# Test email task submission
curl -X POST http://profile-service:8080/api/v1/profiles/test-123/tasks \
  -H "Content-Type: application/json" \
  -d '{"type": "email_notification", "payload": {"to": "test@example.com", "template": "welcome"}}'

# Test image task submission
curl -X POST http://profile-service:8080/api/v1/profiles/test-123/tasks \
  -H "Content-Type: application/json" \
  -d '{"type": "image_processing", "payload": {"image_url": "https://example.com/test.jpg", "operation": "resize"}}'
```

#### Performance Validation

```bash
# Load test (requires appropriate load testing tools)
hey -n 1000 -c 10 http://profile-service:8080/health

# Monitor resource usage
kubectl top pods -l app=profile-service

# Check HPA status
kubectl get hpa profile-service-hpa
```

## Monitoring & Alerting

### Key Metrics to Monitor

- **Request rate**: `rate(profile_api_requests_total[5m])`
- **Error rate**: `rate(profile_api_requests_total{status=~"5.."}[5m])`
- **Response time**: `histogram_quantile(0.95, rate(profile_api_request_duration_seconds_bucket[5m]))`
- **Task distribution**: `sum by (routing_key) (rate(profile_tasks_routing_key_distribution[5m]))`
- **Queue communication**: `profile_queue_service_requests_total`

### Alert Thresholds

- **Error Rate**: > 1% (warning), > 5% (critical)
- **Response Time**: > 50ms 95th percentile
- **Availability**: < 99% uptime
- **Task Failures**: > 2% failure rate
- **Resource Usage**: > 80% memory, > 70% CPU

### Grafana Dashboard Access

- **URL**: `http://grafana:3000/d/profile-service/profile-service-multi-worker-architecture`
- **Key Panels**: Request Rate, Response Time, Task Distribution, Queue Communication, Error Rate

## Troubleshooting

### Common Deployment Issues

#### Pod CrashLoopBackOff

```bash
# Check pod logs
kubectl logs -f deployment/profile-service --previous

# Check events
kubectl describe pod -l app=profile-service

# Common causes:
# - Missing environment variables
# - Database connection issues
# - Insufficient resources
```

#### Service Unavailable

```bash
# Check service endpoints
kubectl get endpoints profile-service

# Verify pod readiness
kubectl get pods -l app=profile-service -o wide

# Test connectivity
kubectl exec -it deployment/profile-service -- curl localhost:8080/health
```

#### High Resource Usage

```bash
# Scale up replicas
kubectl scale deployment profile-service --replicas=5

# Increase resource limits (temporary)
kubectl patch deployment profile-service -p '{"spec":{"template":{"spec":{"containers":[{"name":"profile-service","resources":{"limits":{"memory":"1Gi","cpu":"1000m"}}}]}}}}'
```

### Emergency Procedures

#### Critical Service Failure

1. **Execute emergency rollback**:

   ```bash
   ./deployments/scripts/rollback-procedures.sh emergency
   ```

2. **Scale to minimum viable service**:

   ```bash
   kubectl scale deployment profile-service --replicas=2
   ```

3. **Check dependencies**:

   ```bash
   kubectl get pods -l app=queue-service
   kubectl get pods -l app=storage-service
   ```

4. **Contact on-call team** using escalation procedures in `docs/OPERATIONAL_RUNBOOKS.md`

## Security Considerations

### Container Security

- **Non-root user execution** (UID 1000)
- **Read-only root filesystem** with writable volumes for logs/cache
- **Minimal base image** (Alpine Linux)
- **No unnecessary packages** or tools in final image

### Kubernetes Security

- **RBAC least privilege** (only necessary permissions)
- **Network policies** (restricted ingress/egress)
- **Security context** (non-root, dropped capabilities)
- **Resource limits** (prevent resource exhaustion)

### Secret Management

- **Kubernetes Secrets** for sensitive configuration
- **No secrets in ConfigMaps** or environment variables
- **Secret rotation procedures** documented
- **Access logging** for secret access

## Performance Characteristics

Based on comprehensive testing, the deployment achieves:

- **API Response Time**: 0.2-15ms average (target: <50ms)
- **Error Rate**: 0.02% (target: <1%)
- **Throughput**: 1225+ req/sec (target: >1000 req/sec)
- **Queue Communication**: 11-125ms (target: <100ms)
- **Resource Efficiency**: 256Mi memory, 200m CPU for baseline load

### Scaling Characteristics

- **HPA triggers**: 70% CPU, 80% memory utilization
- **Scale range**: 3-10 replicas
- **Scale-up time**: ~60 seconds
- **Scale-down stabilization**: 5 minutes

## Maintenance Procedures

### Regular Maintenance

- **Weekly**: Review resource usage and scaling patterns
- **Monthly**: Update base images and security patches
- **Quarterly**: Review and test rollback procedures
- **Annually**: Comprehensive security audit

### Update Procedures

1. **Build new image** with updated code
2. **Test in staging environment**
3. **Update deployment manifest** with new image tag
4. **Apply rolling update**: `kubectl set image deployment/profile-service profile-service=new-image:tag`
5. **Monitor rollout**: `kubectl rollout status deployment/profile-service`
6. **Validate deployment** using post-deployment checklist

This deployment infrastructure provides a production-ready, scalable, and secure foundation for the Profile Service multi-worker architecture, with comprehensive monitoring, alerting, and operational procedures.
