# Auth Service Step-by-Step Deployment Guide

## 📚 Educational Deployment Walkthrough

This guide provides comprehensive step-by-step instructions for deploying the auth-service with detailed explanations for educational purposes. Perfect for learning Kubernetes deployments and microservices architecture.

## 🎯 Learning Objectives

By completing this guide, you will understand:

- **Microservices Architecture**: Auth service as an orchestration layer
- **JWT-based Authentication**: Token generation, validation, and lifecycle
- **Circuit Breaker Patterns**: Resilience and fault tolerance
- **Kubernetes Security**: RBAC, NetworkPolicies, and SecurityContexts
- **Service Integration**: HTTP-based microservices communication
- **Production Deployment**: Scalability, monitoring, and operational practices

## 🏗️ Architecture Overview

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Client Apps   │───▶│   Auth Service  │───▶│ Storage Service │
└─────────────────┘    │  (Orchestrator) │    │  (User Data)    │
                       └─────────┬───────┘    └─────────────────┘
                                 │
                                 ▼
                       ┌─────────────────┐
                       │ Cache Service   │
                       │ (Sessions)      │
                       └─────────────────┘
```

**Key Components:**

- **Auth Service**: JWT token management and user authentication
- **Storage Service**: User data, credentials, and audit logs
- **Cache Service**: Session management and token blacklisting
- **Circuit Breakers**: Fault tolerance and graceful degradation

## 📋 Prerequisites

### Required Tools

```bash
# Verify kubectl
kubectl version --client

# Verify Kind (for local deployment)
kind version

# Verify curl (for testing)
curl --version

# Optional: Verify openssl (for JWT key generation)
openssl version
```

### Cluster Requirements

- **Kind**: Local development and testing
- **Production**: Kubernetes 1.20+ with RBAC enabled
- **Resources**: 512Mi memory, 0.5 CPU minimum per replica
- **Storage**: Persistent volumes for JWT keys (production)

## 🚀 Phase 1: Pre-Deployment Setup

### Step 1.1: Verify Cluster Connectivity

```bash
# Check cluster status
kubectl cluster-info

# Verify default namespace
kubectl get ns default

# Check available resources
kubectl top nodes  # (if metrics-server is installed)
```

**Learning Point**: Always verify cluster connectivity and resource availability before deployment.

### Step 1.2: Understand Dependencies

The auth-service requires two external services:

```bash
# Check if storage-service exists
kubectl get service storage-service -n default

# Check if cache-service exists
kubectl get service cache-service -n default
```

**Learning Point**: Microservices have explicit dependencies that must be satisfied for proper operation.

## 🔐 Phase 2: Security Configuration

### Step 2.1: JWT Key Generation (Production Only)

```bash
# Generate RSA private key (2048-bit)
openssl genrsa -out auth-private.pem 2048

# Extract public key
openssl rsa -in auth-private.pem -pubout -out auth-public.pem

# View key contents (DO NOT share private key)
echo "Private key (first 3 lines):"
head -3 auth-private.pem

echo "Public key:"
cat auth-public.pem
```

**Learning Point**: RSA key pairs enable secure JWT signing and verification without shared secrets.

### Step 2.2: Create Kubernetes Secrets

```bash
# Create secret from files (production)
kubectl create secret generic auth-service-secrets \
  --from-file=JWT_PRIVATE_KEY=auth-private.pem \
  --from-file=JWT_PUBLIC_KEY=auth-public.pem \
  --from-literal=SERVICE_API_KEY=$(openssl rand -hex 32) \
  --from-literal=METRICS_AUTH_TOKEN=$(openssl rand -hex 16)

# Verify secret creation
kubectl describe secret auth-service-secrets

# For Kind/development, secrets are auto-generated in kustomization
```

**Learning Point**: Kubernetes secrets provide secure storage for sensitive configuration data.

## ⚙️ Phase 3: Configuration Management

### Step 3.1: Review ConfigMap

```bash
# View the configuration
cat deployments/kubernetes/configmap.yaml

# Key configurations explained:
# - STORAGE_SERVICE_URL: Internal service communication
# - CIRCUIT_BREAKER_TIMEOUT: Fault tolerance settings
# - RATE_LIMIT_MAX_REQUESTS: Security and performance
```

**Learning Point**: ConfigMaps separate configuration from code, enabling environment-specific deployments.

### Step 3.2: Apply Configuration

```bash
# Apply ConfigMap
kubectl apply -f deployments/kubernetes/configmap.yaml

# Verify configuration
kubectl get configmap auth-service-config -o yaml
```

## 🛡️ Phase 4: RBAC and Security

### Step 4.1: Understand RBAC Components

```bash
# Review RBAC configuration
cat deployments/kubernetes/service.yaml

# Components explained:
# - ServiceAccount: Identity for the auth-service pods
# - Role: Minimal permissions (configmaps, secrets read-only)
# - RoleBinding: Links ServiceAccount to Role
# - NetworkPolicy: Network-level security controls
```

**Learning Point**: RBAC follows the principle of least privilege for enhanced security.

### Step 4.2: Deploy RBAC and Service

```bash
# Apply service and RBAC
kubectl apply -f deployments/kubernetes/service.yaml

# Verify ServiceAccount
kubectl get serviceaccount auth-service

# Verify RBAC
kubectl describe role auth-service
kubectl describe rolebinding auth-service

# Verify NetworkPolicy
kubectl get networkpolicy auth-service-network-policy
```

## 🚀 Phase 5: Application Deployment

### Step 5.1: Review Deployment Manifest

```bash
# Examine deployment configuration
cat deployments/kubernetes/deployment.yaml

# Key features explained:
# - replicas: 3 (high availability)
# - rollingUpdate: Zero-downtime deployments
# - securityContext: Non-root user, read-only filesystem
# - resources: Memory/CPU limits and requests
# - probes: Health, readiness, and startup checks
# - affinity: Pod distribution across nodes
```

**Learning Point**: Production deployments require careful resource management and health checks.

### Step 5.2: Deploy Application

```bash
# Choose deployment method:

# Option A: Production deployment
kubectl apply -f deployments/kubernetes/deployment.yaml

# Option B: Kind/local deployment
kubectl apply -k deployments/kind/

# Wait for rollout completion
kubectl rollout status deployment/auth-service --timeout=300s
```

### Step 5.3: Verify Deployment

```bash
# Check pod status
kubectl get pods -l app=auth-service -w

# Check deployment status
kubectl get deployment auth-service

# View pod details
kubectl describe pod -l app=auth-service

# Check resource usage
kubectl top pod -l app=auth-service  # (if metrics-server available)
```

## 📊 Phase 6: Horizontal Pod Autoscaler (Production)

### Step 6.1: Deploy HPA

```bash
# Apply HPA (production only)
kubectl apply -f deployments/kubernetes/hpa.yaml

# Check HPA status
kubectl get hpa auth-service-hpa

# Monitor HPA behavior
kubectl describe hpa auth-service-hpa
```

**Learning Point**: HPA automatically scales pods based on CPU/memory utilization for handling traffic spikes.

## 🔍 Phase 7: Health Check Verification

### Step 7.1: Test Health Endpoints

```bash
# Port forward to access service (if not using NodePort)
kubectl port-forward service/auth-service 8080:8080 &
PORT_FORWARD_PID=$!

# For Kind deployment, use NodePort
# Service available at: http://localhost:30080

# Test health endpoints
curl -s http://localhost:8080/health | jq '.'
curl -s http://localhost:8080/ready | jq '.'
curl -s http://localhost:8080/live | jq '.'

# Test service info
curl -s http://localhost:8080/ | jq '.'

# Clean up port forward
kill $PORT_FORWARD_PID
```

**Learning Point**: Health checks enable Kubernetes to make intelligent scheduling and traffic routing decisions.

### Step 7.2: Understand Health Check Results

```bash
# Healthy response (storage and cache available):
{
  "status": "healthy",
  "dependencies": {
    "storage": "healthy",
    "cache": "healthy"
  }
}

# Degraded response (dependencies unavailable):
{
  "status": "degraded",
  "dependencies": {
    "storage": "unhealthy",
    "cache": "unhealthy"
  }
}
```

**Learning Point**: The auth-service gracefully degrades when dependencies are unavailable but continues operating.

## 🧪 Phase 8: Integration Testing

### Step 8.1: Test Authentication Flow

```bash
# Test authentication endpoint structure
curl -X POST http://localhost:30080/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "test@example.com",
    "password": "testpassword"
  }'

# Expected response structure:
{
  "status": "error",
  "message": "Invalid credentials"  # Expected without real storage-service
}
```

### Step 8.2: Test Token Validation

```bash
# Test token validation endpoint
curl -X POST http://localhost:30080/v1/auth/token/validate \
  -H "Content-Type: application/json" \
  -d '{"token": "invalid-token"}'

# Expected response:
{
  "status": "error",
  "message": "Invalid token"
}
```

**Learning Point**: API endpoints return consistent JSON responses following microservices conventions.

## 📈 Phase 9: Monitoring Setup

### Step 9.1: Deploy Prometheus ServiceMonitor

```bash
# Apply ServiceMonitor (requires Prometheus Operator)
kubectl apply -f deployments/monitoring/servicemonitor.yaml

# Verify ServiceMonitor
kubectl get servicemonitor auth-service -o yaml
```

### Step 9.2: Test Metrics Endpoint

```bash
# Access metrics endpoint
curl -s http://localhost:30081/metrics | head -20

# Key metrics to monitor:
# - auth_attempts_total: Authentication attempt counters
# - auth_duration_seconds: Authentication latency
# - auth_circuit_breaker_state: Circuit breaker status
# - process_* : Standard Node.js metrics
```

**Learning Point**: Prometheus metrics provide observability into service performance and health.

## 🛠️ Phase 10: Operational Procedures

### Step 10.1: View Logs

```bash
# Stream logs from all auth-service pods
kubectl logs -l app=auth-service -f --tail=50

# View logs from specific pod
kubectl logs deployment/auth-service --tail=100

# Search logs for authentication attempts
kubectl logs -l app=auth-service | grep "Authentication attempt"
```

### Step 10.2: Debug Common Issues

```bash
# Check service endpoints
kubectl get endpoints auth-service

# Check service connectivity
kubectl run debug-pod --image=curlimages/curl --rm -i --tty -- sh
# Inside debug pod:
curl http://auth-service:8080/health

# Check circuit breaker status in logs
kubectl logs -l app=auth-service | grep "circuit breaker"
```

### Step 10.3: Scale Operations

```bash
# Manual scaling
kubectl scale deployment auth-service --replicas=5

# Check scaling progress
kubectl get pods -l app=auth-service -w

# Scale back down
kubectl scale deployment auth-service --replicas=3
```

## 🔄 Phase 11: Rollback Procedures

### Step 11.1: View Rollout History

```bash
# Check deployment history
kubectl rollout history deployment/auth-service

# View specific revision
kubectl rollout history deployment/auth-service --revision=2
```

### Step 11.2: Perform Rollback

```bash
# Quick rollback to previous version
kubectl rollout undo deployment/auth-service

# Rollback to specific revision
kubectl rollout undo deployment/auth-service --to-revision=1

# Monitor rollback progress
kubectl rollout status deployment/auth-service
```

## 🧹 Phase 12: Cleanup (Optional)

### Step 12.1: Remove Deployment

```bash
# Use cleanup script
cd deployments/scripts/
./manual-cleanup.sh --step-by-step

# Or manual cleanup
kubectl delete -f deployments/kubernetes/
kubectl delete secret auth-service-secrets
```

## 📚 Learning Summary

### Key Concepts Covered

1. **Microservices Orchestration**: Auth-service as a thin coordination layer
2. **JWT Security**: RSA-based token signing and validation
3. **Circuit Breaker Pattern**: Fault tolerance in distributed systems
4. **Kubernetes Security**: RBAC, NetworkPolicies, SecurityContexts
5. **Service Discovery**: Internal service communication patterns
6. **Health Checks**: Liveness, readiness, and startup probes
7. **Horizontal Scaling**: Automatic scaling based on resource utilization
8. **Observability**: Metrics collection and monitoring integration
9. **Operational Procedures**: Logging, debugging, and rollback strategies

### Production Considerations

- **Security**: Generate unique JWT keys for each environment
- **Resources**: Monitor memory/CPU usage and adjust limits
- **Dependencies**: Ensure storage and cache services are highly available
- **Monitoring**: Set up alerts for authentication failures and circuit breakers
- **Backup**: Secure backup of JWT private keys
- **Network**: Review NetworkPolicy rules for security compliance

### Next Steps

1. **Deploy Dependencies**: Set up storage-service and cache-service
2. **End-to-End Testing**: Test complete authentication flows
3. **Performance Testing**: Load test with realistic traffic patterns
4. **Security Review**: Audit RBAC permissions and network policies
5. **Monitoring Setup**: Configure alerting rules and dashboards

---

**Congratulations!** 🎉 You have successfully deployed the auth-service and learned key microservices and Kubernetes concepts.

For advanced topics, explore:

- Multi-environment deployments with GitOps
- Service mesh integration (Istio)
- Advanced monitoring with Grafana dashboards
- Automated testing in CI/CD pipelines
