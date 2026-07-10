# Microservices Deployment Standard

## Executive Summary

**Document Purpose**: Define standardized deployment patterns for all microservices  
**Based On**: Profile Service deployment structure (validated and tested)  
**Strategy**: Dual deployment approach - Manual for analysis, Kustomize for operations  
**Target**: Consistent, maintainable, and scalable deployment across all services

This standard establishes the deployment patterns that all microservices should follow, based on the proven profile-service implementation that has been tested and validated in kind clusters.

## Deployment Philosophy: Dual Approach

### **Manual Deployment for Analysis**

- **Purpose**: Step-by-step analysis and understanding of each manifest
- **Use Cases**: Learning, troubleshooting, detailed inspection, educational purposes
- **Benefits**: Complete visibility into each component, easier debugging, better understanding
- **When to Use**: Initial setup, problem diagnosis, training, manifest validation

### **Kustomize Deployment for Operations**

- **Purpose**: Regular, consistent, and automated deployments
- **Use Cases**: Daily operations, CI/CD pipelines, environment management
- **Benefits**: Consistency, automation, environment-specific customization, reduced errors
- **When to Use**: Regular deployments, production operations, automated workflows

**Both approaches are REQUIRED and complementary** - manual for understanding, kustomize for efficiency.

## Standard Directory Structure

Every service MUST follow this exact directory structure supporting both deployment approaches:

```
services/{service-name}/deployments/
├── README.md                          # Service-specific deployment guide
├── STEP_BY_STEP_DEPLOYMENT_GUIDE.md  # Detailed MANUAL deployment instructions
├── kubernetes/                        # Base production-ready manifests
│   ├── deployment.yaml               # Production deployment configuration
│   ├── service.yaml                  # Service definition with RBAC
│   ├── configmap.yaml                # Service configuration
│   └── secrets.yaml                  # Secret templates
├── kind/                             # Kind-specific overlays
│   ├── kustomization.yaml            # Kind kustomization (AUTOMATED)
│   ├── deployment-patch.yaml         # Kind-specific patches
│   ├── service-patch.yaml            # Kind service patches
│   ├── {service}-dependencies.yaml   # Development dependencies
│   ├── monitoring-configmap.yaml     # Local monitoring config
│   └── deploy-to-kind.sh             # Automated deployment script
├── scripts/                          # Operational scripts
│   ├── manual-deploy.sh              # MANUAL deployment script
│   ├── manual-cleanup.sh             # MANUAL cleanup script
│   └── rollback-procedures.sh        # Rollback and recovery scripts
└── monitoring/                       # Monitoring configuration
    └── servicemonitor.yaml           # Prometheus ServiceMonitor
```

## Base Manifest Standards

### 1. **Deployment Configuration** (`kubernetes/deployment.yaml`)

#### **Required Metadata**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: { service-name }
  labels:
    app: { service-name }
    version: v1.0.0
    component: { api|worker|cache|storage }
    part-of: microservices
spec:
  replicas: 3 # Production default
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 1
      maxSurge: 1
```

#### **Required Container Configuration**

```yaml
spec:
  template:
    metadata:
      labels:
        app: {service-name}
        version: v1.0.0
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8080"
        prometheus.io/path: "/metrics"
    spec:
      containers:
        - name: {service-name}
          image: {service-name}:latest
          imagePullPolicy: IfNotPresent
          ports:
            - containerPort: 8080
              name: http
            - containerPort: 8081  # Optional: separate metrics port
              name: metrics
```

#### **Standard Environment Variables**

Every service MUST include these standard environment variables:

```yaml
env:
  # Server Configuration (REQUIRED)
  - name: SERVER_HOST
    value: "0.0.0.0"
  - name: SERVER_PORT
    value: "8080"

  # Logging Configuration (REQUIRED)
  - name: LOG_LEVEL
    value: "info" # Production default
  - name: ENV
    value: "production"

  # Service Discovery (REQUIRED for services with dependencies)
  - name: QUEUE_SERVICE_URL
    value: "http://queue-service:8080"
  - name: CACHE_SERVICE_URL
    value: "http://cache-service:8080"
  - name: STORAGE_SERVICE_URL
    value: "http://storage-service:8080"

  # Timeout Configuration (REQUIRED)
  - name: SERVICE_TIMEOUT
    value: "30s"
  - name: SERVICE_RETRIES
    value: "3"

  # Feature Flags (REQUIRED)
  - name: METRICS_ENABLED
    value: "true"
  - name: CIRCUIT_BREAKER_ENABLED
    value: "true"
  - name: RATE_LIMIT_ENABLED
    value: "true"
```

#### **Standard Resource Configuration**

Resource limits based on service type:

```yaml
# API Services (Profile, Queue, Cache, Storage)
resources:
  requests:
    memory: "256Mi"
    cpu: "200m"
  limits:
    memory: "512Mi"
    cpu: "500m"

# Worker Services
resources:
  requests:
    memory: "128Mi"
    cpu: "100m"
  limits:
    memory: "256Mi"
    cpu: "300m"
```

#### **Standard Health Checks**

```yaml
# Liveness Probe (REQUIRED)
livenessProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 30
  periodSeconds: 10
  timeoutSeconds: 5
  failureThreshold: 3
  successThreshold: 1

# Readiness Probe (REQUIRED)
readinessProbe:
  httpGet:
    path: /health # or /ready if different
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 5
  timeoutSeconds: 3
  failureThreshold: 3
  successThreshold: 1

# Startup Probe (REQUIRED for slow-starting services)
startupProbe:
  httpGet:
    path: /health
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10
  timeoutSeconds: 3
  failureThreshold: 30
  successThreshold: 1
```

#### **Standard Security Configuration**

```yaml
# Container Security Context (REQUIRED)
securityContext:
  allowPrivilegeEscalation: false
  runAsNonRoot: true
  runAsUser: 1000
  runAsGroup: 1000
  readOnlyRootFilesystem: true
  capabilities:
    drop:
      - ALL

# Pod Security Context (REQUIRED)
securityContext:
  fsGroup: 1000
  runAsNonRoot: true
  seccompProfile:
    type: RuntimeDefault

# Standard Volumes (REQUIRED)
volumes:
  - name: tmp
    emptyDir: {}
  - name: cache
    emptyDir:
      sizeLimit: 100Mi
  - name: logs
    emptyDir:
      sizeLimit: 50Mi

volumeMounts:
  - name: tmp
    mountPath: /tmp
  - name: cache
    mountPath: /app/cache
  - name: logs
    mountPath: /app/logs
```

#### **Standard Scheduling Configuration**

```yaml
# Service Account (REQUIRED)
serviceAccountName: { service-name }

# Node Selection (REQUIRED for ARM64 kind clusters)
nodeSelector:
  kubernetes.io/arch: arm64

# Pod Anti-Affinity (REQUIRED for production)
affinity:
  podAntiAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
      - weight: 100
        podAffinityTerm:
          labelSelector:
            matchExpressions:
              - key: app
                operator: In
                values:
                  - { service-name }
          topologyKey: kubernetes.io/hostname

# Standard Tolerations (REQUIRED)
tolerations:
  - key: "node.kubernetes.io/not-ready"
    operator: "Exists"
    effect: "NoExecute"
    tolerationSeconds: 300
  - key: "node.kubernetes.io/unreachable"
    operator: "Exists"
    effect: "NoExecute"
    tolerationSeconds: 300
```

### 2. **Service Configuration** (`kubernetes/service.yaml`)

#### **Standard Service Definition**

```yaml
apiVersion: v1
kind: Service
metadata:
  name: { service-name }
  labels:
    app: { service-name }
  annotations:
    prometheus.io/scrape: "true"
    prometheus.io/port: "8080"
    prometheus.io/path: "/metrics"
spec:
  selector:
    app: { service-name }
  ports:
    - name: http
      port: 8080
      targetPort: 8080
      protocol: TCP
    - name: metrics # Optional: if separate metrics port
      port: 8081
      targetPort: 8080
      protocol: TCP
  type: ClusterIP
  sessionAffinity: None
```

#### **Standard RBAC Configuration**

Every service MUST include RBAC configuration:

```yaml
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {service-name}
  labels:
    app: {service-name}
automountServiceAccountToken: true

---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: {service-name}-role
  labels:
    app: {service-name}-role
rules:
  - apiGroups: [""]
    resources: ["configmaps", "secrets"]
    verbs: ["get", "list", "watch"]
  - apiGroups: [""]
    resources: ["services", "endpoints"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["apps"]
    resources: ["deployments"]
    verbs: ["get", "list", "watch"]

---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: {service-name}-binding
  labels:
    app: {service-name}-binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: {service-name}-role
subjects:
  - kind: ServiceAccount
    name: {service-name}
    namespace: default
```

#### **Standard Auto-Scaling Configuration**

```yaml
---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: {service-name}-hpa
  labels:
    app: {service-name}
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: {service-name}
  minReplicas: 2
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
      stabilizationWindowSeconds: 0
      policies:
        - type: Percent
          value: 100
          periodSeconds: 15
        - type: Pods
          value: 4
          periodSeconds: 15
      selectPolicy: Max
```

### 3. **Configuration Management** (`kubernetes/configmap.yaml`)

#### **Standard ConfigMap Structure**

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: {service-name}-config
  labels:
    app: {service-name}
data:
  # Service-specific configuration
  service.yaml: |
    server:
      host: "0.0.0.0"
      port: 8080
      timeout: 30s

    logging:
      level: info
      format: json

    metrics:
      enabled: true
      port: 8080
      path: /metrics

    # Service dependencies
    dependencies:
      queue-service:
        url: http://queue-service:8080
        timeout: 30s
        retries: 3
      cache-service:
        url: http://cache-service:8080
        timeout: 10s
        retries: 3
      storage-service:
        url: http://storage-service:8080
        timeout: 30s
        retries: 3

    # Feature flags
    features:
      circuit_breaker: true
      rate_limiting: true
      request_tracing: true
      health_checks: true

  # Application properties (if needed)
  application.properties: |
    # Service-specific properties
    app.name={service-name}
    app.version=1.0.0
    app.environment=production
```

### 4. **Secrets Configuration** (`kubernetes/secrets.yaml`)

#### **Standard Secrets Template**

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: {service-name}-secrets
  labels:
    app: {service-name}
type: Opaque
data:
  # Database credentials (base64 encoded)
  DB_USERNAME: <base64-encoded-username>
  DB_PASSWORD: <base64-encoded-password>

  # Service authentication
  JWT_SECRET: <base64-encoded-jwt-secret>
  API_KEY: <base64-encoded-api-key>

  # External service credentials
  RABBITMQ_USERNAME: <base64-encoded-rabbitmq-user>
  RABBITMQ_PASSWORD: <base64-encoded-rabbitmq-password>
  REDIS_PASSWORD: <base64-encoded-redis-password>

  # Service-specific secrets
  ENCRYPTION_KEY: <base64-encoded-encryption-key>
  SIGNING_KEY: <base64-encoded-signing-key>

---
# Local development secrets (for kind)
apiVersion: v1
kind: Secret
metadata:
  name: {service-name}-secrets-local
  labels:
    app: {service-name}
    environment: development
type: Opaque
data:
  # Development credentials (base64 encoded)
  DB_USERNAME: ZGV2LXVzZXI=  # dev-user
  DB_PASSWORD: ZGV2LXBhc3M=  # dev-pass
  JWT_SECRET: ZGV2LWp3dC1zZWNyZXQ=  # dev-jwt-secret
  API_KEY: ZGV2LWFwaS1rZXk=  # dev-api-key
  RABBITMQ_USERNAME: Z3Vlc3Q=  # guest
  RABBITMQ_PASSWORD: Z3Vlc3Q=  # guest
  REDIS_PASSWORD: ""  # empty for development
  ENCRYPTION_KEY: ZGV2LWVuY3J5cHRpb24ta2V5  # dev-encryption-key
  SIGNING_KEY: ZGV2LXNpZ25pbmcta2V5  # dev-signing-key
```

## Kind Overlay Standards

### 1. **Kind Kustomization** (`kind/kustomization.yaml`)

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

# Base manifests from kubernetes/ directory
resources:
  - ../kubernetes/configmap.yaml
  - ../kubernetes/secrets.yaml
  - ../kubernetes/deployment.yaml
  - ../kubernetes/service.yaml
  # Development dependencies (service-specific)
  - {service}-dependencies.yaml
  # Monitoring (ConfigMap only - no Prometheus Operator required)
  - monitoring-configmap.yaml

# Kind-specific patches
patchesStrategicMerge:
  - deployment-patch.yaml
  - service-patch.yaml

# Use local secrets instead of production ones
replacements:
  - source:
      kind: Secret
      name: {service-name}-secrets-local
    targets:
      - select:
          kind: Deployment
          name: {service-name}
        fieldPaths:
          - spec.template.spec.containers.[name={service-name}].envFrom.[secretRef.name={service-name}-secrets].secretRef.name

# Kind-specific namespace
namespace: default

# Common labels for kind deployment
commonLabels:
  environment: local-kind
  deployment-tool: kustomize
```

### 2. **Kind Deployment Patches** (`kind/deployment-patch.yaml`)

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {service-name}
spec:
  # Reduce replicas for local development
  replicas: 1

  template:
    spec:
      containers:
        - name: {service-name}
          # Reduced resource requirements for local development
          resources:
            requests:
              memory: "128Mi"  # Reduced for kind
              cpu: "100m"      # Reduced for kind
            limits:
              memory: "256Mi"  # Reduced for kind
              cpu: "200m"      # Reduced for kind

          # Kind-specific environment variables
          env:
            # Service discovery with full DNS names for kind
            - name: QUEUE_SERVICE_URL
              value: "http://queue-service.default.svc.cluster.local:8080"
            - name: CACHE_SERVICE_URL
              value: "http://cache-service.default.svc.cluster.local:8080"
            - name: STORAGE_SERVICE_URL
              value: "http://storage-service.default.svc.cluster.local:8080"

            # Local development settings
            - name: LOG_LEVEL
              value: "debug"
            - name: ENV
              value: "development"

            # Disable production features for local development
            - name: CIRCUIT_BREAKER_ENABLED
              value: "false"  # Disable for easier debugging
            - name: RATE_LIMIT_ENABLED
              value: "false"  # Disable for local development
            - name: METRICS_ENABLED
              value: "true"   # Keep metrics for observability

          # Use local secrets
          envFrom:
            - configMapRef:
                name: {service-name}-config
            - secretRef:
                name: {service-name}-secrets-local

      # Remove production-specific scheduling for kind
      affinity: null
      nodeSelector: null
      tolerations: null
```

### 3. **Kind Service Patches** (`kind/service-patch.yaml`)

```yaml
apiVersion: v1
kind: Service
metadata:
  name: { service-name }
spec:
  type: NodePort # Use NodePort for kind access
  ports:
    - name: http
      port: 8080
      targetPort: 8080
      nodePort: 30080 # Service-specific NodePort (profile: 30080, queue: 30081, etc.)
      protocol: TCP
    - name: metrics
      port: 8081
      targetPort: 8080
      nodePort: 30081 # Metrics NodePort
      protocol: TCP
```

### 4. **Deployment Script** (`kind/deploy-to-kind.sh`)

Every service MUST include this standardized deployment script:

```bash
#!/bin/bash

# Deploy {Service Name} to Kind Cluster
# Usage: ./deploy-to-kind.sh [COMMAND]

set -euo pipefail

# Configuration
KIND_CLUSTER_NAME=${KIND_CLUSTER_NAME:-{service-name}-dev}
DOCKER_IMAGE=${DOCKER_IMAGE:-{service-name}:latest}
SERVICE_NAME="{service-name}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Standard functions (check_prerequisites, create_kind_cluster, build_and_load_image, etc.)
# ... (Implementation follows profile-service pattern)

main() {
    case "${1:-deploy}" in
        "deploy")
            deploy_service
            ;;
        "build")
            build_and_load_image
            ;;
        "clean")
            cleanup
            ;;
        "logs")
            show_logs
            ;;
        "status")
            show_status
            ;;
        *)
            show_usage
            ;;
    esac
}

main "$@"
```

## Monitoring Standards

### 1. **ServiceMonitor Configuration** (`monitoring/servicemonitor.yaml`)

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: { service-name }
  labels:
    app: { service-name }
    release: prometheus
spec:
  selector:
    matchLabels:
      app: { service-name }
  endpoints:
    - port: http
      path: /metrics
      interval: 30s
      scrapeTimeout: 10s
      honorLabels: true
      metricRelabelings:
        - sourceLabels: [__name__]
          regex: "go_.*"
          action: drop
        - sourceLabels: [__name__]
          regex: "promhttp_.*"
          action: drop
```

### 2. **Kind Monitoring ConfigMap** (`kind/monitoring-configmap.yaml`)

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: {service-name}-monitoring
  labels:
    app: {service-name}
data:
  prometheus.yml: |
    global:
      scrape_interval: 15s
      evaluation_interval: 15s

    scrape_configs:
      - job_name: '{service-name}'
        static_configs:
          - targets: ['{service-name}:8080']
        metrics_path: /metrics
        scrape_interval: 30s
```

## Service-Specific Dependencies

Each service MAY include development dependencies in kind overlay:

### **Profile Service Dependencies** (`kind/profile-dependencies.yaml`)

```yaml
# Temporary Redis for profile service (until cache-service integration)
apiVersion: apps/v1
kind: Deployment
metadata:
  name: redis-service
  labels:
    app: redis-service
    component: cache
    temporary: "true" # Mark as temporary dependency
spec:
  replicas: 1
  selector:
    matchLabels:
      app: redis-service
  template:
    metadata:
      labels:
        app: redis-service
    spec:
      containers:
        - name: redis
          image: redis:7-alpine
          ports:
            - containerPort: 6379
              name: redis
          resources:
            requests:
              memory: "64Mi"
              cpu: "50m"
            limits:
              memory: "128Mi"
              cpu: "100m"

---
apiVersion: v1
kind: Service
metadata:
  name: redis-service
  labels:
    app: redis-service
spec:
  selector:
    app: redis-service
  ports:
    - port: 6379
      targetPort: 6379
      name: redis
  type: ClusterIP
```

### **Worker Service Dependencies** (`kind/worker-dependencies.yaml`)

```yaml
# RabbitMQ for worker service
apiVersion: apps/v1
kind: Deployment
metadata:
  name: rabbitmq-service
  labels:
    app: rabbitmq-service
    component: messaging
spec:
  replicas: 1
  selector:
    matchLabels:
      app: rabbitmq-service
  template:
    metadata:
      labels:
        app: rabbitmq-service
    spec:
      containers:
        - name: rabbitmq
          image: rabbitmq:3-management-alpine
          ports:
            - containerPort: 5672
              name: amqp
            - containerPort: 15672
              name: management
          env:
            - name: RABBITMQ_DEFAULT_USER
              value: "guest"
            - name: RABBITMQ_DEFAULT_PASS
              value: "guest"
          resources:
            requests:
              memory: "128Mi"
              cpu: "100m"
            limits:
              memory: "256Mi"
              cpu: "200m"

---
apiVersion: v1
kind: Service
metadata:
  name: rabbitmq-service
  labels:
    app: rabbitmq-service
spec:
  selector:
    app: rabbitmq-service
  ports:
    - port: 5672
      targetPort: 5672
      name: amqp
    - port: 15672
      targetPort: 15672
      name: management
  type: ClusterIP
```

## Documentation Standards

### 1. **Service-Specific README** (`README.md`)

````markdown
# {Service Name} Deployment

## Deployment Approaches

This service supports **two complementary deployment approaches**:

### 🔍 **Manual Deployment** (Analysis & Learning)

**Purpose**: Step-by-step analysis and understanding  
**Best for**: Learning, troubleshooting, detailed inspection

```bash
# Step-by-step manual deployment with analysis
cd deployments/scripts
./manual-deploy.sh --analyze

# Interactive deployment with prompts
./manual-deploy.sh --step-by-step

# Manual cleanup
./manual-cleanup.sh --step-by-step
```

### ⚡ **Kustomize Deployment** (Operations & Automation)

**Purpose**: Regular, consistent operations  
**Best for**: Daily operations, CI/CD, production deployments

```bash
# Quick kustomize deployment
cd deployments/kind
kubectl apply -k .

# Or using deployment script
./deploy-to-kind.sh
```

## Quick Start

### Manual Approach (Recommended for First Time)

```bash
# 1. Understand each component step-by-step
cd deployments/scripts
./manual-deploy.sh --analyze

# 2. View detailed deployment guide
cat ../STEP_BY_STEP_DEPLOYMENT_GUIDE.md

# 3. Clean up when done
./manual-cleanup.sh
```

### Kustomize Approach (Recommended for Regular Use)

```bash
# 1. Quick deployment
cd deployments/kind
kubectl apply -k .

# 2. Check status
kubectl get pods -l app={service-name}

# 3. View logs
kubectl logs -l app={service-name} --tail=50 -f
```

## When to Use Each Approach

| Scenario                  | Manual | Kustomize | Reason                             |
| ------------------------- | ------ | --------- | ---------------------------------- |
| **First deployment**      | ✅     | ❌        | Learn components step-by-step      |
| **Troubleshooting**       | ✅     | ❌        | Analyze each manifest individually |
| **Learning/Training**     | ✅     | ❌        | Understand Kubernetes resources    |
| **Daily development**     | ❌     | ✅        | Speed and consistency              |
| **CI/CD pipelines**       | ❌     | ✅        | Automation and reliability         |
| **Production deployment** | ❌     | ✅        | Consistency and safety             |
| **Problem diagnosis**     | ✅     | ❌        | Step-by-step analysis              |
| **Environment promotion** | ❌     | ✅        | Consistent configuration           |

## Configuration

### Environment Variables

| Variable          | Description              | Default | Required |
| ----------------- | ------------------------ | ------- | -------- |
| SERVER_PORT       | HTTP server port         | 8080    | Yes      |
| LOG_LEVEL         | Logging level            | info    | Yes      |
| DEPLOYMENT_METHOD | Deployment approach used | -       | Auto-set |

### Health Checks

- **Liveness**: `GET /health`
- **Readiness**: `GET /ready` (or `/health`)
- **Metrics**: `GET /metrics`

## Troubleshooting

### Manual Deployment Issues

```bash
# Analyze specific manifest
cd deployments/scripts
./manual-deploy.sh --analyze

# Check individual components
kubectl describe deployment {service-name}
kubectl describe service {service-name}
kubectl describe configmap {service-name}-config
```

### Kustomize Deployment Issues

```bash
# Validate kustomization
kubectl kustomize deployments/kind/

# Dry run to check changes
kubectl apply -k deployments/kind/ --dry-run=client

# Compare with manual approach
cd deployments/scripts
./manual-deploy.sh --analyze
```

### Common Debugging Commands

```bash
# Check all resources
kubectl get all -l app={service-name}

# Detailed pod information
kubectl describe pod -l app={service-name}

# View recent logs
kubectl logs -l app={service-name} --tail=100

# Port forward for local access
kubectl port-forward svc/{service-name} 8080:8080
```
````

### 2. **Step-by-Step Guide Template** (`STEP_BY_STEP_DEPLOYMENT_GUIDE.md`)

Each service should include a comprehensive deployment guide following the profile-service pattern.

## Creating the Step-by-Step Deployment Guide

### **Purpose and Structure**

The `STEP_BY_STEP_DEPLOYMENT_GUIDE.md` serves as a comprehensive manual deployment tutorial that provides:

- **Educational Value**: Deep understanding of each Kubernetes component
- **Troubleshooting Aid**: Step-by-step analysis for problem isolation
- **Verification Procedures**: Commands to validate each deployment stage
- **Learning Resource**: Training material for team members

### **Template Reference**

All services MUST use the proven template from `services/profile-service/deployments/STEP_BY_STEP_DEPLOYMENT_GUIDE_TEMPLATE.md` as their foundation. This template has been battle-tested and provides comprehensive coverage of deployment scenarios.

### **Standard Guide Structure**

Every service's step-by-step guide MUST follow this standardized structure:

````markdown
# Step-by-Step Kubernetes Deployment Guide

## {Service Name} {Key Architecture Feature}

Brief description of the service and its role in the microservices ecosystem.

## 🚀 Two Ways to Follow This Guide

### Option 1: Automated Manual Deployment (Recommended)

Use the automated manual deployment script that follows this guide:

```bash
cd deployments/scripts

# Interactive step-by-step deployment
./manual-deploy.sh --step-by-step

# With detailed manifest analysis
./manual-deploy.sh --analyze

# Cleanup when done
./manual-cleanup.sh --step-by-step
```

**⚠️ Important Note**: The manual deployment script **automatically detects** your cluster type:

- **Kind clusters**: Uses Kind-optimized settings (reduced replicas, local secrets, debug logging)
- **Production clusters**: Uses full production settings (3 replicas, production resources)

### Option 2: Manual Commands (Educational)

Follow the detailed commands below to understand each step completely.

**⚠️ Note**: These manual commands use **Kind-optimized manifests**. For production deployment, use Option 1 or the kustomize approach.

## 📋 Prerequisites

### Cluster Verification

```bash
# Standard cluster checks that work for any service
kind get clusters
kubectl config use-context kind-{service-name}
kubectl cluster-info
kubectl get nodes

# Verify architecture compatibility (important for Apple Silicon)
kubectl get nodes --show-labels | grep kubernetes.io/arch
```

## 🚀 Deployment Sequence

**🎯 Target Environment**: This guide is optimized for **Kind (local development)** clusters.

For **production deployment**, use:

```bash
kubectl apply -f deployments/kubernetes/
kubectl apply -f deployments/monitoring/
```

The steps below walk through **Kind-optimized deployment** for educational purposes:

### Step 1: 🔐 Deploy Secrets (`secrets.yaml`)

### Step 2: ⚙️ Deploy ConfigMaps (`configmap.yaml`)

### Step 3: 🔒 Deploy RBAC & Service (`service.yaml`)

### Step 4: 🚀 Deploy Application (Kind-Optimized)

### Step 5: 📊 Deploy Monitoring

## 🔍 Comprehensive Cluster State Commands

## 🎯 What to Look For at Each Step

## 🚨 Common Issues & Troubleshooting

## 🧪 Quick Test Suite

## 📝 Service-Specific Notes
````

### **Detailed Step Template Requirements**

#### **1. Consistent Step Format**

Every deployment step MUST follow this exact format:

````markdown
### Step X: {Icon} {Action} (`{filename}`)

**What it does**: {Clear explanation of component purpose and impact}

#### Deploy:

```bash
{Kind-appropriate deployment commands}
```
````

**⚠️ Why {Optimization}?** {Explanation of Kind-specific optimizations}

#### Observation Commands:

```bash
# 1. {Primary verification} - {Brief explanation}
{command}

# 2. {Resource inspection} - {Brief explanation}
{command}

# 3. {Detailed analysis} - {Brief explanation}
{command}

# 4. {Cross-verification} - {Brief explanation}
{command}

# 5. {Event monitoring} - {Brief explanation}
{command}
```

**Expected Impact**: ✅ {Clear statement of what should be created/changed}

**Common Issues**:

- {Issue 1 and solution}
- {Issue 2 and solution}
- {Issue 3 and solution}

````

#### **2. Required Observation Commands Pattern**

Each step MUST include exactly 5+ observation commands following this pattern:

1. **Primary Verification**: Basic resource existence check
2. **Resource Inspection**: Detailed resource description
3. **Detailed Analysis**: Configuration and status analysis
4. **Cross-Verification**: Integration with other resources
5. **Event Monitoring**: Kubernetes events for troubleshooting

Example for Secrets:
```bash
# 1. Check if secrets were created
kubectl get secrets -l app={service-name}

# 2. Describe secrets (metadata only, not values)
kubectl describe secret {service-name}-secrets

# 3. Verify secret keys exist (still encoded)
kubectl get secret {service-name}-secrets -o yaml

# 4. Service-specific secret validation
{Add service-specific secret checks here}

# 5. Watch for secret-related events
kubectl get events --sort-by=.metadata.creationTimestamp --field-selector involvedObject.kind=Secret
````

#### **3. Deployment Step Specifications**

**Step 1: Secrets (🔐)**

- **Purpose**: Sensitive configuration data (passwords, API keys, certificates)
- **Kind Optimization**: Uses local development secrets with appropriate values
- **Key Verification**: Show how to check secret keys without exposing values
- **Common Issues**: Base64 encoding errors, missing required keys

**Step 2: ConfigMaps (⚙️)**

- **Purpose**: Non-sensitive configuration (service URLs, timeouts, feature flags)
- **Kind Optimization**: Debug logging enabled, local service discovery
- **Key Verification**: Show how to inspect configuration values
- **Common Issues**: YAML formatting errors, incorrect service URLs

**Step 3: RBAC & Service (🔒)**

- **Purpose**: Network service, RBAC permissions, scaling policies, security rules
- **Kind Optimization**: Simplified RBAC for local development
- **Key Verification**: Service endpoints, RBAC permissions, network policies
- **Common Issues**: Service selector mismatches, overly restrictive RBAC

**Step 4: Application Deployment (🚀)**

- **Purpose**: Main application pods and containers
- **Kind Optimization**: Single replica, reduced resources, local secrets
- **Key Verification**: Pod status, logs, health checks, service endpoints
- **Common Issues**: Image pull failures, configuration errors, resource constraints

**Step 5: Monitoring (📊)**

- **Purpose**: Metrics collection and monitoring setup
- **Kind Optimization**: ConfigMap-based monitoring (no Prometheus Operator required)
- **Key Verification**: Metrics endpoints, ServiceMonitor (if available)
- **Common Issues**: Missing Prometheus Operator, CRD availability

#### **4. Comprehensive Troubleshooting Requirements**

Every guide MUST include these troubleshooting categories:

**Architecture Compatibility Issues**

```markdown
**Issue**: Pods stuck in Pending with node affinity errors

**Symptoms**:
```

0/4 nodes are available: 3 node(s) didn't match Pod's node affinity/selector

````

**Root Cause**: Architecture mismatch (AMD64 vs ARM64)

**Solution**:
```bash
# Check cluster architecture
kubectl get nodes --show-labels | grep kubernetes.io/arch

# Update deployment.yaml nodeSelector:
# For Apple Silicon: kubernetes.io/arch=arm64
# For Intel: kubernetes.io/arch=amd64
````

````

**Image Pull Issues**
```markdown
**Issue**: Pods in `ImagePullBackOff` state

**Symptoms**:
````

Failed to pull image "{service-name}:latest": repository does not exist

````

**Root Cause**: Image not available in kind cluster

**Solution**:
```bash
# Load image into kind cluster
kind load docker-image {service-name}:latest --name {cluster-name}

# Ensure imagePullPolicy is set correctly
spec:
  containers:
  - name: {service-name}
    image: {service-name}:latest
    imagePullPolicy: IfNotPresent
````

````

**Configuration Issues**
- Missing secrets or configmaps
- Wrong environment variables
- Service dependency failures
- DNS resolution problems

**Resource Constraints**
- Insufficient node resources
- Resource limit conflicts
- Scheduling failures

**Service-Specific Issues**
- Each service must document its unique failure modes
- Include service-specific error messages and solutions
- Provide service-specific debugging commands

#### **5. Required Test Suite**

Every guide MUST include a comprehensive test suite:

```bash
## 🧪 Quick Test Suite

After everything is deployed and running:

```bash
# 1. Basic connectivity test
kubectl port-forward service/{service-name} {port}:{port} &
sleep 2

# 2. Health check
curl -f http://localhost:{port}/health && echo "✅ Health OK" || echo "❌ Health Failed"

# 3. Service-specific functionality tests
{Add service-specific API tests here}

# 4. Metrics validation
curl -s http://localhost:{port}/metrics | grep {service_prefix}_ | wc -l | xargs echo "Service metrics count:"

# 5. Integration tests (if applicable)
{Add integration test commands}

# Cleanup port-forward
pkill -f "kubectl port-forward"
````

### Service-Specific Test Examples

**API Services**:

```bash
# Test primary API endpoints
curl -X GET http://localhost:{port}/api/v1/{resource}
curl -X POST http://localhost:{port}/api/v1/{resource} \
  -H "Content-Type: application/json" \
  -d '{test-payload}'
```

**Worker Services**:

```bash
# Test worker health and queue connectivity
curl http://localhost:{port}/health
curl http://localhost:{port}/status
```

**Cache Services**:

```bash
# Test cache operations
curl -X PUT http://localhost:{port}/api/v1/cache/test-key \
  -H "Content-Type: application/json" \
  -d '{"value": "test-value", "ttl": 300}'
curl http://localhost:{port}/api/v1/cache/test-key
```

````

#### **6. Service-Specific Customization Requirements**

Each service MUST customize these sections based on its specific needs:

**Architecture Description**
- Highlight unique architectural features (e.g., "Multi-Worker Architecture", "Event-Driven Processing")
- Explain service's role in the microservices ecosystem
- Document key integration points

**Kind Optimizations**
- Document why specific optimizations are needed for Kind
- Explain resource reductions and their impact
- List any temporary dependencies (like Redis for development)

**Service Dependencies**
- List all external service dependencies
- Explain how to mock or substitute dependencies for local testing
- Document dependency health check procedures

**Configuration Highlights**
- Document critical configuration values
- Explain environment-specific configuration differences
- Highlight security-sensitive configuration items

**Integration Points**
- Document how this service integrates with others
- Provide integration testing procedures
- Explain service discovery and communication patterns

### **Guide Creation Process**

#### **Step 1: Template Preparation**
```bash
# Copy the proven template
cp services/profile-service/deployments/STEP_BY_STEP_DEPLOYMENT_GUIDE_TEMPLATE.md \
   services/{service-name}/deployments/STEP_BY_STEP_DEPLOYMENT_GUIDE.md
````

#### **Step 2: Service Customization**

1. Replace all `{service-name}` placeholders
2. Update architecture description and service role
3. Customize deployment steps for service-specific resources
4. Add service-specific observation commands
5. Include service-specific troubleshooting scenarios

#### **Step 3: Testing and Validation**

1. Test all commands on a real Kind cluster
2. Verify troubleshooting solutions work
3. Validate service-specific test suite
4. Ensure educational flow is logical

#### **Step 4: Review and Maintenance**

1. Review with team for completeness
2. Update as service evolves
3. Maintain consistency with deployment standard
4. Keep troubleshooting section current

### **Quality Assurance Checklist**

When creating or updating a step-by-step guide, verify:

#### ✅ **Content Requirements**

- [ ] All 5 deployment steps present and complete
- [ ] Each step has 5+ observation commands
- [ ] Comprehensive troubleshooting section included
- [ ] Service-specific test suite provided
- [ ] Production deployment references included

#### ✅ **Technical Accuracy**

- [ ] All commands tested on Kind cluster
- [ ] Resource names match actual manifest files
- [ ] Expected outputs verified
- [ ] Troubleshooting solutions validated
- [ ] Dependencies correctly documented

#### ✅ **Educational Value**

- [ ] Each step explains "what" and "why"
- [ ] Multiple observation perspectives provided
- [ ] Common failure modes addressed
- [ ] Learning progression maintained
- [ ] Cross-references to related concepts

#### ✅ **Consistency Standards**

- [ ] Follows template structure exactly
- [ ] Uses consistent command formats
- [ ] Maintains emoji and formatting style
- [ ] Cross-references deployment methods correctly

#### ✅ **Service-Specific Completeness**

- [ ] Architecture description accurate
- [ ] Service dependencies documented
- [ ] Integration points explained
- [ ] Unique failure modes covered
- [ ] Service-specific tests included

### **Maintenance and Updates**

The step-by-step guide should be updated whenever:

- **Manifest files change**: Update commands and expected outputs
- **New dependencies added**: Include new observation and testing commands
- **Common issues discovered**: Add to troubleshooting section
- **Kubernetes versions change**: Verify command compatibility
- **Service functionality evolves**: Update test suite and integration points

**Update Process**:

1. Test all commands in guide on current Kind cluster
2. Verify troubleshooting solutions still work
3. Update service-specific sections as needed
4. Validate educational flow remains logical
5. Review consistency with current deployment standard

### **Integration with Deployment Scripts**

The step-by-step guide must be tightly integrated with the manual deployment scripts:

- **Script References Guide**: The `manual-deploy.sh --step-by-step` option should follow the guide exactly
- **Guide References Script**: The guide should recommend using the script for automation
- **Consistent Commands**: Commands in the guide should match those used by the script
- **Shared Troubleshooting**: Both should reference the same troubleshooting procedures

This integration ensures that users can seamlessly move between manual learning and automated deployment while maintaining consistency and reliability.

---

**Reference Implementation**: See `services/profile-service/deployments/STEP_BY_STEP_DEPLOYMENT_GUIDE.md` for the complete, battle-tested implementation of these guidelines.
