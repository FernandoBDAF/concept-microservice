# Storage Service - Step-by-Step Deployment Guide

## Overview

This guide provides a comprehensive, step-by-step approach to deploying the Storage Service. Each step includes validation commands to ensure successful deployment.

**Estimated Time**: 30-45 minutes  
**Difficulty**: Intermediate  
**Prerequisites**: Kubernetes cluster access, kubectl configured

## Table of Contents

1. [Pre-Deployment Verification](#step-1-pre-deployment-verification)
2. [Deploy Dependencies](#step-2-deploy-dependencies)
3. [Create Configuration](#step-3-create-configuration)
4. [Deploy Storage Service](#step-4-deploy-storage-service)
5. [Create Service and Networking](#step-5-create-service-and-networking)
6. [Configure Auto-Scaling](#step-6-configure-auto-scaling-optional)
7. [Setup Monitoring](#step-7-setup-monitoring-optional)
8. [Verify Complete Deployment](#step-8-verify-complete-deployment)
9. [Performance Testing](#step-9-performance-testing-optional)

---

## Step 1: Pre-Deployment Verification

### 1.1 Verify Kubernetes Access

```bash
# Check cluster connection
kubectl cluster-info

# Check available nodes
kubectl get nodes

# Check current namespace
kubectl config current-context
```

**Expected Output**: Cluster info displays correctly, nodes are Ready

### 1.2 Verify Required Permissions

```bash
# Test deployment creation (dry run)
kubectl auth can-i create deployments
kubectl auth can-i create services
kubectl auth can-i create configmaps
kubectl auth can-i create secrets

# Test HPA creation (if planning to use auto-scaling)
kubectl auth can-i create horizontalpodautoscalers
```

**Expected Output**: All commands return "yes"

### 1.3 Check Resource Availability

```bash
# Check cluster resource usage
kubectl top nodes

# Check available storage classes
kubectl get storageclass

# Check if metrics server is running (for HPA)
kubectl get pods -n kube-system | grep metrics-server
```

**Expected Output**: Sufficient resources available, storage classes present

---

## Step 2: Deploy Dependencies

### 2.1 Deploy PostgreSQL Database

```bash
# Create PostgreSQL deployment
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: postgres
  labels:
    app: postgres
spec:
  replicas: 1
  selector:
    matchLabels:
      app: postgres
  template:
    metadata:
      labels:
        app: postgres
    spec:
      containers:
      - name: postgres
        image: postgres:15-alpine
        ports:
        - containerPort: 5432
        env:
        - name: POSTGRES_DB
          value: "profiles"
        - name: POSTGRES_USER
          value: "profile_user"
        - name: POSTGRES_PASSWORD
          value: "profile_password"
        - name: PGDATA
          value: "/var/lib/postgresql/data/pgdata"
        volumeMounts:
        - name: postgres-storage
          mountPath: /var/lib/postgresql/data
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
      volumes:
      - name: postgres-storage
        emptyDir: {}
---
apiVersion: v1
kind: Service
metadata:
  name: postgres
  labels:
    app: postgres
spec:
  selector:
    app: postgres
  ports:
  - name: postgres
    port: 5432
    targetPort: 5432
  type: ClusterIP
EOF
```

### 2.2 Verify PostgreSQL Deployment

```bash
# Wait for PostgreSQL to be ready
kubectl wait --for=condition=available deployment/postgres --timeout=300s

# Check PostgreSQL pods
kubectl get pods -l app=postgres

# Test database connection
kubectl run postgres-test --rm -it --restart=Never --image=postgres:15-alpine -- psql -h postgres -U profile_user -d profiles -c "SELECT version();"
```

**Expected Output**: PostgreSQL pod is Running, connection test succeeds

### 2.3 Deploy RabbitMQ

```bash
# Create RabbitMQ deployment
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: rabbitmq
  labels:
    app: rabbitmq
spec:
  replicas: 1
  selector:
    matchLabels:
      app: rabbitmq
  template:
    metadata:
      labels:
        app: rabbitmq
    spec:
      containers:
      - name: rabbitmq
        image: rabbitmq:3.11-management-alpine
        ports:
        - containerPort: 5672
          name: amqp
        - containerPort: 15672
          name: management
        env:
        - name: RABBITMQ_DEFAULT_USER
          value: "admin"
        - name: RABBITMQ_DEFAULT_PASS
          value: "password"
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        volumeMounts:
        - name: rabbitmq-data
          mountPath: /var/lib/rabbitmq
      volumes:
      - name: rabbitmq-data
        emptyDir: {}
---
apiVersion: v1
kind: Service
metadata:
  name: rabbitmq
  labels:
    app: rabbitmq
spec:
  selector:
    app: rabbitmq
  ports:
  - name: amqp
    port: 5672
    targetPort: 5672
  - name: management
    port: 15672
    targetPort: 15672
  type: ClusterIP
EOF
```

### 2.4 Verify RabbitMQ Deployment

```bash
# Wait for RabbitMQ to be ready
kubectl wait --for=condition=available deployment/rabbitmq --timeout=300s

# Check RabbitMQ pods
kubectl get pods -l app=rabbitmq

# Test RabbitMQ connection (optional)
kubectl port-forward svc/rabbitmq 15672:15672 &
PORT_FORWARD_PID=$!
sleep 3
curl -u admin:password http://localhost:15672/api/overview
kill $PORT_FORWARD_PID 2>/dev/null || true
```

**Expected Output**: RabbitMQ pod is Running, API responds correctly

---

## Step 3: Create Configuration

### 3.1 Create ConfigMap

```bash
kubectl apply -f - <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: storage-service-config
  labels:
    app: storage-service
data:
  config.yaml: |
    server:
      host: "0.0.0.0"
      port: 8080
      grpc_port: 9090
      read_timeout: 30s
      write_timeout: 30s
      idle_timeout: 120s

    database:
      max_connections: 100
      idle_connections: 20
      max_lifetime: 3600s
      connection_timeout: 30s

    queue:
      prefetch_count: 5
      process_timeout: 30s
      max_retries: 3
      reconnect_delay: 5s

    metrics:
      enabled: true
      port: 8080
      path: "/metrics"

    logging:
      level: "info"
      format: "json"

    circuit_breaker:
      enabled: true
      timeout: 10s
      max_requests: 100
      interval: 30s
      ratio: 0.6
EOF
```

### 3.2 Create Secrets

```bash
kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: storage-service-secrets
  labels:
    app: storage-service
type: Opaque
data:
  # postgresql://profile_user:profile_password@postgres:5432/profiles
  database-url: cG9zdGdyZXNxbDovL3Byb2ZpbGVfdXNlcjpwcm9maWxlX3Bhc3N3b3JkQHBvc3RncmVzOjU0MzIvcHJvZmlsZXM=
  # amqp://admin:password@rabbitmq:5672/
  rabbitmq-url: YW1xcDovL2FkbWluOnBhc3N3b3JkQHJhYmJpdG1xOjU2NzIv
EOF
```

**Important**: In production, use proper secret management tools instead of base64 encoded secrets in manifests.

### 3.3 Verify Configuration

```bash
# Check ConfigMap
kubectl get configmap storage-service-config
kubectl describe configmap storage-service-config

# Check Secret
kubectl get secret storage-service-secrets
kubectl describe secret storage-service-secrets
```

**Expected Output**: ConfigMap and Secret are created successfully

---

## Step 4: Deploy Storage Service

### 4.1 Create Deployment

```bash
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: storage-service
  labels:
    app: storage-service
    service: storage-service
spec:
  replicas: 3
  selector:
    matchLabels:
      app: storage-service
  template:
    metadata:
      labels:
        app: storage-service
    spec:
      containers:
      - name: storage-service
        image: storage-service:latest
        ports:
        - containerPort: 8080
          name: http
        - containerPort: 9090
          name: grpc
        env:
        # Server Configuration
        - name: SERVER_HOST
          value: "0.0.0.0"
        - name: SERVER_PORT
          value: "8080"
        - name: GRPC_PORT
          value: "9090"

        # Service Discovery
        - name: AUTH_SERVICE_URL
          value: "http://auth-service:8080"
        - name: CACHE_SERVICE_URL
          value: "http://cache-service:8080"
        - name: QUEUE_SERVICE_URL
          value: "http://queue-service:8080"
        - name: PROFILE_SERVICE_URL
          value: "http://profile-service:8080"

        # Database Configuration
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: storage-service-secrets
              key: database-url
        - name: DATABASE_MAX_CONNECTIONS
          value: "100"
        - name: DATABASE_IDLE_CONNECTIONS
          value: "20"

        # Queue Configuration
        - name: RABBITMQ_URL
          valueFrom:
            secretKeyRef:
              name: storage-service-secrets
              key: rabbitmq-url
        - name: QUEUE_NAME
          value: "storage-processing"
        - name: EXCHANGE_NAME
          value: "tasks-exchange"
        - name: QUEUE_ENABLED
          value: "true"

        # Feature Flags
        - name: METRICS_ENABLED
          value: "true"
        - name: CIRCUIT_BREAKER_ENABLED
          value: "true"
        - name: AUTH_DATA_ENABLED
          value: "true"

        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"

        livenessProbe:
          httpGet:
            path: /health/live
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
          timeoutSeconds: 5
          failureThreshold: 3

        readinessProbe:
          httpGet:
            path: /health/ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
          timeoutSeconds: 3
          failureThreshold: 3

        # Mount configuration
        volumeMounts:
        - name: config
          mountPath: /app/config
          readOnly: true

      volumes:
      - name: config
        configMap:
          name: storage-service-config

      # Security context
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
        fsGroup: 1000

      # Restart policy
      restartPolicy: Always
EOF
```

### 4.2 Wait for Deployment

```bash
# Wait for deployment to be ready
kubectl wait --for=condition=available deployment/storage-service --timeout=300s

# Check deployment status
kubectl get deployment storage-service
kubectl describe deployment storage-service

# Check pods
kubectl get pods -l app=storage-service
kubectl describe pods -l app=storage-service
```

### 4.3 Check Logs

```bash
# View recent logs
kubectl logs -l app=storage-service --tail=50

# Follow logs in real-time
kubectl logs -l app=storage-service -f

# Check for specific log patterns
kubectl logs -l app=storage-service | grep -E "(ERROR|WARN|started|ready)"
```

**Expected Output**: Deployment is Available, pods are Running, logs show successful startup

---

## Step 5: Create Service and Networking

### 5.1 Create Service

```bash
kubectl apply -f - <<EOF
apiVersion: v1
kind: Service
metadata:
  name: storage-service
  labels:
    app: storage-service
spec:
  selector:
    app: storage-service
  ports:
  - name: http
    port: 8080
    targetPort: 8080
    protocol: TCP
  - name: grpc
    port: 9090
    targetPort: 9090
    protocol: TCP
  type: ClusterIP
EOF
```

### 5.2 Verify Service

```bash
# Check service
kubectl get service storage-service
kubectl describe service storage-service

# Check endpoints
kubectl get endpoints storage-service
```

### 5.3 Test Service Connectivity

```bash
# Port forward for testing
kubectl port-forward svc/storage-service 8080:8080 &
PORT_FORWARD_PID=$!

# Wait a moment for port forward to establish
sleep 3

# Test health endpoint
echo "Testing health endpoint..."
curl -s http://localhost:8080/health | jq .

# Test API endpoint
echo "Testing API endpoint..."
curl -s http://localhost:8080/api/v1/profiles | jq .

# Test metrics endpoint
echo "Testing metrics endpoint..."
curl -s http://localhost:8080/metrics | head -10

# Clean up port forward
kill $PORT_FORWARD_PID 2>/dev/null || true
```

**Expected Output**: Service is created, endpoints are available, API responses are successful

---

## Step 6: Configure Auto-Scaling (Optional)

### 6.1 Create Horizontal Pod Autoscaler

```bash
kubectl apply -f - <<EOF
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: storage-service-hpa
  labels:
    app: storage-service
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: storage-service
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
  behavior:
    scaleDown:
      stabilizationWindowSeconds: 300
      policies:
      - type: Percent
        value: 10
        periodSeconds: 60
    scaleUp:
      stabilizationWindowSeconds: 60
      policies:
      - type: Percent
        value: 50
        periodSeconds: 60
      - type: Pods
        value: 2
        periodSeconds: 60
      selectPolicy: Max
EOF
```

### 6.2 Verify HPA

```bash
# Check HPA status
kubectl get hpa storage-service-hpa
kubectl describe hpa storage-service-hpa

# Monitor HPA (optional)
kubectl get hpa storage-service-hpa --watch
```

**Expected Output**: HPA is created and shows current metrics

---

## Step 7: Setup Monitoring (Optional)

### 7.1 Create ServiceMonitor for Prometheus

```bash
kubectl apply -f - <<EOF
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: storage-service
  labels:
    app: storage-service
    release: prometheus
spec:
  selector:
    matchLabels:
      app: storage-service
  endpoints:
  - port: http
    path: /metrics
    interval: 30s
    scrapeTimeout: 10s
  namespaceSelector:
    matchNames:
    - default
EOF
```

**Note**: This requires Prometheus Operator to be installed in the cluster.

---

## Step 8: Verify Complete Deployment

### 8.1 Check All Resources

```bash
echo "=== Deployment Status ==="
kubectl get deployments -l app=storage-service

echo -e "\n=== Pod Status ==="
kubectl get pods -l app=storage-service -o wide

echo -e "\n=== Service Status ==="
kubectl get services -l app=storage-service

echo -e "\n=== HPA Status ==="
kubectl get hpa -l app=storage-service

echo -e "\n=== ConfigMap Status ==="
kubectl get configmaps -l app=storage-service

echo -e "\n=== Secret Status ==="
kubectl get secrets -l app=storage-service
```

### 8.2 Comprehensive Health Check

```bash
# Port forward
kubectl port-forward svc/storage-service 8080:8080 &
PORT_FORWARD_PID=$!
sleep 3

echo "=== Health Checks ==="

# Liveness check
echo "Liveness check:"
curl -s http://localhost:8080/health/live | jq .

# Readiness check
echo -e "\nReadiness check:"
curl -s http://localhost:8080/health/ready | jq .

# Overall health check
echo -e "\nOverall health check:"
curl -s http://localhost:8080/health | jq .

# API functionality test
echo -e "\nAPI functionality test:"
curl -s -X GET http://localhost:8080/api/v1/profiles | jq .

# Metrics check
echo -e "\nMetrics availability:"
curl -s http://localhost:8080/metrics | grep -E "^storage_" | head -5

# Clean up
kill $PORT_FORWARD_PID 2>/dev/null || true

echo -e "\n=== All Checks Complete ==="
```

**Expected Output**: All resources show healthy status, health checks pass, APIs respond correctly

---

## Step 9: Performance Testing (Optional)

### 9.1 Basic Load Test

```bash
# Install hey (HTTP load testing tool) if not available
# go install github.com/rakyll/hey@latest

# Port forward
kubectl port-forward svc/storage-service 8080:8080 &
PORT_FORWARD_PID=$!
sleep 3

# Run load test
echo "Running basic load test..."
hey -n 1000 -c 10 -m GET http://localhost:8080/health

# Clean up
kill $PORT_FORWARD_PID 2>/dev/null || true
```

### 9.2 Monitor During Load

```bash
# In separate terminals, monitor:

# 1. Pod resource usage
kubectl top pods -l app=storage-service

# 2. HPA behavior
kubectl get hpa storage-service-hpa --watch

# 3. Pod scaling
kubectl get pods -l app=storage-service --watch
```

---

## Troubleshooting

### Common Issues and Solutions

#### 1. Pods Not Starting

```bash
# Check pod status
kubectl get pods -l app=storage-service

# Check events
kubectl get events --sort-by=.metadata.creationTimestamp

# Check pod logs
kubectl logs -l app=storage-service

# Check pod description
kubectl describe pods -l app=storage-service
```

**Common causes:**

- Image pull errors
- Resource constraints
- Configuration errors
- Dependency unavailability

#### 2. Database Connection Issues

```bash
# Test database connectivity
kubectl run postgres-test --rm -it --restart=Never --image=postgres:15-alpine -- psql -h postgres -U profile_user -d profiles -c "SELECT 1;"

# Check database logs
kubectl logs -l app=postgres

# Verify database service
kubectl get service postgres
kubectl describe service postgres
```

#### 3. Service Not Accessible

```bash
# Check service endpoints
kubectl get endpoints storage-service

# Check service configuration
kubectl describe service storage-service

# Test internal connectivity
kubectl run test-pod --rm -it --restart=Never --image=curlimages/curl -- curl -v http://storage-service:8080/health
```

#### 4. High Resource Usage

```bash
# Check resource usage
kubectl top pods -l app=storage-service

# Check resource limits
kubectl describe pods -l app=storage-service | grep -A 5 -B 5 "Limits"

# Check HPA status
kubectl describe hpa storage-service-hpa
```

### Useful Commands for Debugging

```bash
# View all resources
kubectl get all -l app=storage-service

# Check resource usage
kubectl top nodes
kubectl top pods -l app=storage-service

# View logs with timestamps
kubectl logs -l app=storage-service --timestamps=true

# Follow logs from all pods
kubectl logs -l app=storage-service -f --all-containers=true

# Execute commands in pod
kubectl exec -it deployment/storage-service -- /bin/sh

# Port forward for debugging
kubectl port-forward deployment/storage-service 8080:8080

# Check network policies (if any)
kubectl get networkpolicies

# Check resource quotas
kubectl describe resourcequotas
```

---

## Cleanup

When you're done testing or need to remove the deployment:

```bash
# Remove storage service
kubectl delete deployment storage-service
kubectl delete service storage-service
kubectl delete hpa storage-service-hpa
kubectl delete configmap storage-service-config
kubectl delete secret storage-service-secrets

# Remove dependencies (optional)
kubectl delete deployment postgres
kubectl delete service postgres
kubectl delete deployment rabbitmq
kubectl delete service rabbitmq

# Remove monitoring (if deployed)
kubectl delete servicemonitor storage-service
```

---

## Next Steps

After successful deployment:

1. **Integrate with other services**: Deploy auth-service, cache-service, etc.
2. **Setup monitoring**: Configure Prometheus, Grafana dashboards
3. **Configure CI/CD**: Automate deployments with GitOps
4. **Security hardening**: Implement network policies, pod security policies
5. **Backup strategy**: Configure database backups
6. **Disaster recovery**: Plan for service recovery procedures

---

## Production Considerations

Before deploying to production:

1. **Security**:

   - Use proper secret management
   - Implement network policies
   - Enable pod security policies
   - Use non-root containers

2. **Reliability**:

   - Configure persistent storage for database
   - Implement proper backup strategies
   - Set up monitoring and alerting
   - Plan disaster recovery procedures

3. **Performance**:

   - Right-size resource requests and limits
   - Configure appropriate HPA settings
   - Implement database connection pooling
   - Monitor and optimize query performance

4. **Observability**:
   - Structured logging
   - Distributed tracing
   - Comprehensive metrics
   - Health checks and monitoring

This completes the comprehensive step-by-step deployment guide for the Storage Service.
