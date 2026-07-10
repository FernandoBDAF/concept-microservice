# Deployment - Implementation Plan

**Project:** deployment  
**Type:** Kubernetes manifests and automation  
**Status:** 📋 Structure planned  
**Session Focus:** Create Kubernetes deployment structure for the entire architecture

**Related Documents:**
- [`CLUSTER_VISION.md`](CLUSTER_VISION.md) - Final cluster architecture, data flows, and practical examples

---

## 1. Overview

This plan covers creating the `deployment/` folder containing:
- Kubernetes manifests for all services and infrastructure
- Kind cluster configuration for local development
- Automation scripts for deployment
- Monitoring setup (Prometheus, Grafana)

### Two-Phase Deployment Strategy

#### Phase 1: Local/Container-Based Infrastructure (Current)
**Goal:** Kubernetes cluster communicates with infrastructure running as containers

All infrastructure runs as StatefulSets within the Kubernetes cluster:
- PostgreSQL (api-postgres, auth-postgres) → StatefulSets in K8s
- Redis → StatefulSet in K8s
- RabbitMQ → StatefulSet in K8s
- MinIO → StatefulSet in K8s
- MongoDB Atlas → Already cloud (external)

**Benefits:**
- Full control over infrastructure
- No cloud costs for databases/cache
- Easy local development and testing
- Predictable resource usage

#### Phase 2: Cloud-Managed Services (Future)
**Goal:** Migrate to managed cloud services for production scalability

Future migration path to cloud services:
- PostgreSQL → AWS RDS / Azure Database / Google Cloud SQL
- Redis → AWS ElastiCache / Azure Cache / Google Memorystore
- RabbitMQ → AWS Amazon MQ / Azure Service Bus / Google Cloud Pub/Sub
- MinIO → AWS S3 / Azure Blob Storage / Google Cloud Storage
- MongoDB → Keep MongoDB Atlas (already cloud)

**Benefits:**
- Automatic backups and high availability
- Managed scaling and updates
- Reduced operational overhead
- Better production reliability

**Migration Strategy:** Configuration-driven approach allowing seamless transition between local containers and cloud services by changing environment variables and connection strings.

---

### Architecture Components to Deploy (Phase 1)

**Services:**
- auth-service (Node.js)
- api-service (Go)
- graphrag-service (Python)
- email-worker (Go)
- image-worker (Go)
- profile-worker (Go)

**Infrastructure (Containerized in K8s):**
- PostgreSQL (api-postgres, auth-postgres) - StatefulSets
- Redis - StatefulSet
- RabbitMQ - StatefulSet
- MinIO - StatefulSet

**External (Cloud):**
- MongoDB Atlas (already Phase 2)

---

## 2. Source Reference

| Component | Legacy Location | Notes |
|-----------|-----------------|-------|
| K8s Structure | `legacy_project/k8s/` | Base templates |
| Kind Config | `legacy_project/k8s/cluster/` | Cluster setup |
| Scripts | `legacy_project/k8s/scripts/` | Automation |

---

## 3. Directory Structure

```
deployment/
├── README.md
├── CLUSTER_VISION.md                 # Final architecture & practical examples
├── IMPLEMENTATION_PLAN.md            # This file
├── cluster/
│   ├── kind-config.yaml
│   ├── kind-multinode.yaml
│   ├── setup-cluster.sh
│   ├── infrastructure/
│   │   ├── ingress-nginx.yaml
│   │   ├── metrics-server.yaml
│   │   ├── network-policies.yaml
│   │   └── storage-class.yaml
│   └── validation/
│       ├── test-cluster.sh
│       └── verify-infrastructure.sh
├── services/
│   ├── 01-infrastructure/
│   │   ├── postgres-api/
│   │   │   ├── statefulset.yaml
│   │   │   ├── service.yaml
│   │   │   ├── configmap.yaml
│   │   │   └── secret.yaml
│   │   ├── postgres-auth/
│   │   │   ├── statefulset.yaml
│   │   │   ├── service.yaml
│   │   │   ├── configmap.yaml
│   │   │   └── secret.yaml
│   │   ├── redis/
│   │   │   ├── statefulset.yaml
│   │   │   ├── service.yaml
│   │   │   ├── configmap.yaml
│   │   │   └── secret.yaml
│   │   ├── rabbitmq/
│   │   │   ├── statefulset.yaml
│   │   │   ├── service.yaml
│   │   │   ├── configmap.yaml
│   │   │   └── secret.yaml
│   │   └── minio/
│   │       ├── statefulset.yaml
│   │       ├── service.yaml
│   │       ├── configmap.yaml
│   │       ├── secret.yaml
│   │       └── init-job.yaml
│   ├── 02-auth-service/
│   │   ├── deployment.yaml
│   │   ├── service.yaml
│   │   ├── configmap.yaml
│   │   ├── secret.yaml
│   │   ├── network-policy.yaml
│   │   └── hpa.yaml
│   ├── 03-api-service/
│   │   ├── deployment.yaml
│   │   ├── service.yaml
│   │   ├── configmap.yaml
│   │   ├── secret.yaml
│   │   ├── network-policy.yaml
│   │   └── hpa.yaml
│   ├── 04-graphrag-service/
│   │   ├── deployment.yaml
│   │   ├── service.yaml
│   │   ├── configmap.yaml
│   │   ├── secret.yaml
│   │   ├── network-policy.yaml
│   │   └── hpa.yaml
│   └── 05-operational-workers/
│       ├── email-worker/
│       │   ├── deployment.yaml
│       │   ├── service.yaml
│       │   ├── configmap.yaml
│       │   └── secret.yaml
│       ├── image-worker/
│       │   ├── deployment.yaml
│       │   ├── service.yaml
│       │   ├── configmap.yaml
│       │   └── secret.yaml
│       └── profile-worker/
│           ├── deployment.yaml
│           ├── service.yaml
│           ├── configmap.yaml
│           └── secret.yaml
├── external/
│   ├── mongodb-atlas/
│   │   ├── README.md
│   │   └── connection-secret.yaml
│   └── cloud-migration/          # Phase 2: Cloud service configs
│       ├── aws-rds.md
│       ├── aws-elasticache.md
│       ├── aws-amazonmq.md
│       └── aws-s3.md
├── ingress/
│   ├── ingress.yaml
│   └── tls-secret.yaml
├── monitoring/
│   ├── prometheus/
│   │   └── service-monitors.yaml
│   └── grafana/
│       └── dashboards/
└── scripts/
    ├── common-functions.sh
    ├── deploy-all.sh
    ├── build-all-images.sh
    ├── load-images-to-kind.sh
    ├── setup-infrastructure.sh
    ├── teardown.sh
    └── test/
        ├── test-auth.sh
        ├── test-api.sh
        └── integration-tests.sh
```

---

## 4. Implementation Tasks

### Phase 1: Cluster Setup (Day 1)

#### Task 1.1: Create Kind Configuration

**File:** `cluster/kind-config.yaml`

```yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: microservices-v2
nodes:
  - role: control-plane
    kubeadmConfigPatches:
      - |
        kind: InitConfiguration
        nodeRegistration:
          kubeletExtraArgs:
            node-labels: "ingress-ready=true"
    extraPortMappings:
      # API Service
      - containerPort: 30080
        hostPort: 8080
        protocol: TCP
      # Auth Service
      - containerPort: 30081
        hostPort: 8081
        protocol: TCP
      # RabbitMQ Management
      - containerPort: 30091
        hostPort: 15672
        protocol: TCP
      # MinIO Console
      - containerPort: 30093
        hostPort: 9001
        protocol: TCP
  - role: worker
  - role: worker
```

#### Task 1.2: Create Cluster Setup Script

**File:** `cluster/setup-cluster.sh`

```bash
#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../scripts/common-functions.sh"

echo "🚀 Setting up Kubernetes cluster..."

# Create Kind cluster
if ! kind get clusters | grep -q "microservices-v2"; then
    echo "Creating Kind cluster..."
    kind create cluster --config "$SCRIPT_DIR/kind-config.yaml"
else
    echo "Cluster already exists"
fi

# Set context
kubectl cluster-info --context kind-microservices-v2

# Install NGINX Ingress Controller
echo "Installing NGINX Ingress Controller..."
kubectl apply -f "$SCRIPT_DIR/infrastructure/ingress-nginx.yaml"

# Wait for ingress controller
echo "Waiting for ingress controller..."
kubectl wait --namespace ingress-nginx \
  --for=condition=ready pod \
  --selector=app.kubernetes.io/component=controller \
  --timeout=120s

# Install metrics server
echo "Installing metrics server..."
kubectl apply -f "$SCRIPT_DIR/infrastructure/metrics-server.yaml"

echo "✅ Cluster setup complete!"
```

---

### Phase 2: Infrastructure Manifests (Day 2-3)

#### Task 2.1: PostgreSQL for API Service

**File:** `services/01-infrastructure/postgres-api/statefulset.yaml`

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: postgres-api
  labels:
    app: postgres-api
spec:
  serviceName: postgres-api
  replicas: 1
  selector:
    matchLabels:
      app: postgres-api
  template:
    metadata:
      labels:
        app: postgres-api
    spec:
      containers:
        - name: postgres
          image: postgres:15-alpine
          ports:
            - containerPort: 5432
              name: postgres
          env:
            - name: POSTGRES_DB
              valueFrom:
                configMapKeyRef:
                  name: postgres-api-config
                  key: POSTGRES_DB
            - name: POSTGRES_USER
              valueFrom:
                secretKeyRef:
                  name: postgres-api-secret
                  key: POSTGRES_USER
            - name: POSTGRES_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: postgres-api-secret
                  key: POSTGRES_PASSWORD
          volumeMounts:
            - name: data
              mountPath: /var/lib/postgresql/data
          resources:
            requests:
              cpu: 250m
              memory: 512Mi
            limits:
              cpu: 1000m
              memory: 2Gi
          livenessProbe:
            exec:
              command:
                - pg_isready
                - -U
                - $(POSTGRES_USER)
            initialDelaySeconds: 30
            periodSeconds: 10
          readinessProbe:
            exec:
              command:
                - pg_isready
                - -U
                - $(POSTGRES_USER)
            initialDelaySeconds: 5
            periodSeconds: 5
  volumeClaimTemplates:
    - metadata:
        name: data
      spec:
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: 10Gi
```

**File:** `services/01-infrastructure/postgres-api/service.yaml`

```yaml
apiVersion: v1
kind: Service
metadata:
  name: postgres-api
  labels:
    app: postgres-api
spec:
  type: ClusterIP
  ports:
    - port: 5432
      targetPort: postgres
      name: postgres
  selector:
    app: postgres-api
```

**File:** `services/01-infrastructure/postgres-api/configmap.yaml`

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: postgres-api-config
data:
  POSTGRES_DB: api_db
```

**File:** `services/01-infrastructure/postgres-api/secret.yaml`

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: postgres-api-secret
type: Opaque
stringData:
  POSTGRES_USER: api_user
  POSTGRES_PASSWORD: changeme_api_password
```

#### Task 2.2: PostgreSQL for Auth Service

Similar to above with different names:
- `postgres-auth`
- Database: `auth_db`
- User: `auth_user`

#### Task 2.3: Redis

**File:** `services/01-infrastructure/redis/statefulset.yaml`

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: redis
  labels:
    app: redis
spec:
  serviceName: redis
  replicas: 1
  selector:
    matchLabels:
      app: redis
  template:
    metadata:
      labels:
        app: redis
    spec:
      containers:
        - name: redis
          image: redis:7-alpine
          ports:
            - containerPort: 6379
              name: redis
          command:
            - redis-server
            - --appendonly
            - "yes"
            - --maxmemory
            - "512mb"
            - --maxmemory-policy
            - "allkeys-lru"
          resources:
            requests:
              cpu: 100m
              memory: 256Mi
            limits:
              cpu: 500m
              memory: 1Gi
          volumeMounts:
            - name: data
              mountPath: /data
          livenessProbe:
            exec:
              command:
                - redis-cli
                - ping
            initialDelaySeconds: 10
            periodSeconds: 10
          readinessProbe:
            exec:
              command:
                - redis-cli
                - ping
            initialDelaySeconds: 5
            periodSeconds: 5
  volumeClaimTemplates:
    - metadata:
        name: data
      spec:
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: 1Gi
```

#### Task 2.4: RabbitMQ

**File:** `services/01-infrastructure/rabbitmq/statefulset.yaml`

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: rabbitmq
  labels:
    app: rabbitmq
spec:
  serviceName: rabbitmq
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
          image: rabbitmq:3.13-management
          ports:
            - containerPort: 5672
              name: amqp
            - containerPort: 15672
              name: management
            - containerPort: 15692
              name: metrics
          env:
            - name: RABBITMQ_DEFAULT_USER
              valueFrom:
                secretKeyRef:
                  name: rabbitmq-secret
                  key: RABBITMQ_USER
            - name: RABBITMQ_DEFAULT_PASS
              valueFrom:
                secretKeyRef:
                  name: rabbitmq-secret
                  key: RABBITMQ_PASSWORD
          resources:
            requests:
              cpu: 250m
              memory: 512Mi
            limits:
              cpu: 1000m
              memory: 2Gi
          volumeMounts:
            - name: data
              mountPath: /var/lib/rabbitmq
          livenessProbe:
            exec:
              command:
                - rabbitmq-diagnostics
                - -q
                - ping
            initialDelaySeconds: 60
            periodSeconds: 30
          readinessProbe:
            exec:
              command:
                - rabbitmq-diagnostics
                - -q
                - check_running
            initialDelaySeconds: 30
            periodSeconds: 10
  volumeClaimTemplates:
    - metadata:
        name: data
      spec:
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: 5Gi
```

**File:** `services/01-infrastructure/rabbitmq/service.yaml`

```yaml
apiVersion: v1
kind: Service
metadata:
  name: rabbitmq
  labels:
    app: rabbitmq
spec:
  type: ClusterIP
  ports:
    - port: 5672
      targetPort: amqp
      name: amqp
    - port: 15672
      targetPort: management
      name: management
    - port: 15692
      targetPort: metrics
      name: metrics
  selector:
    app: rabbitmq
---
apiVersion: v1
kind: Service
metadata:
  name: rabbitmq-nodeport
  labels:
    app: rabbitmq
spec:
  type: NodePort
  ports:
    - port: 15672
      targetPort: management
      nodePort: 30091
      name: management
  selector:
    app: rabbitmq
```

#### Task 2.5: MinIO

**File:** `services/01-infrastructure/minio/statefulset.yaml`

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: minio
  labels:
    app: minio
spec:
  serviceName: minio
  replicas: 1
  selector:
    matchLabels:
      app: minio
  template:
    metadata:
      labels:
        app: minio
    spec:
      containers:
        - name: minio
          image: minio/minio:latest
          args:
            - server
            - /data
            - --console-address
            - ":9001"
          ports:
            - containerPort: 9000
              name: api
            - containerPort: 9001
              name: console
          env:
            - name: MINIO_ROOT_USER
              valueFrom:
                secretKeyRef:
                  name: minio-secret
                  key: MINIO_ACCESS_KEY
            - name: MINIO_ROOT_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: minio-secret
                  key: MINIO_SECRET_KEY
          resources:
            requests:
              cpu: 250m
              memory: 512Mi
            limits:
              cpu: 1000m
              memory: 2Gi
          volumeMounts:
            - name: data
              mountPath: /data
          livenessProbe:
            httpGet:
              path: /minio/health/live
              port: api
            initialDelaySeconds: 30
            periodSeconds: 30
          readinessProbe:
            httpGet:
              path: /minio/health/ready
              port: api
            initialDelaySeconds: 10
            periodSeconds: 10
  volumeClaimTemplates:
    - metadata:
        name: data
      spec:
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: 50Gi
```

**File:** `services/01-infrastructure/minio/init-job.yaml`

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: minio-init
spec:
  template:
    spec:
      containers:
        - name: mc
          image: minio/mc:latest
          command:
            - /bin/sh
            - -c
            - |
              until mc alias set myminio http://minio:9000 $MINIO_ACCESS_KEY $MINIO_SECRET_KEY; do
                echo "Waiting for MinIO..."
                sleep 5
              done
              mc mb myminio/documents-raw --ignore-existing
              mc mb myminio/documents-processed --ignore-existing
              echo "Buckets created successfully"
          env:
            - name: MINIO_ACCESS_KEY
              valueFrom:
                secretKeyRef:
                  name: minio-secret
                  key: MINIO_ACCESS_KEY
            - name: MINIO_SECRET_KEY
              valueFrom:
                secretKeyRef:
                  name: minio-secret
                  key: MINIO_SECRET_KEY
      restartPolicy: OnFailure
  backoffLimit: 3
```

---

### Phase 3: Service Manifests (Day 4-5)

#### Task 3.1: Auth Service Deployment

**File:** `services/02-auth-service/deployment.yaml`

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: auth-service
  labels:
    app: auth-service
spec:
  replicas: 2
  selector:
    matchLabels:
      app: auth-service
  template:
    metadata:
      labels:
        app: auth-service
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8080"
        prometheus.io/path: "/metrics"
    spec:
      containers:
        - name: auth-service
          image: auth-service:latest
          imagePullPolicy: IfNotPresent
          ports:
            - containerPort: 8080
              name: http
          env:
            - name: NODE_ENV
              value: "production"
            - name: PORT
              value: "8080"
            - name: DATABASE_HOST
              value: "postgres-auth"
            - name: DATABASE_PORT
              value: "5432"
            - name: DATABASE_NAME
              value: "auth_db"
            - name: DATABASE_USER
              valueFrom:
                secretKeyRef:
                  name: postgres-auth-secret
                  key: POSTGRES_USER
            - name: DATABASE_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: postgres-auth-secret
                  key: POSTGRES_PASSWORD
            - name: JWT_SECRET
              valueFrom:
                secretKeyRef:
                  name: auth-service-secret
                  key: JWT_SECRET
            - name: JWT_ACCESS_TOKEN_EXPIRY
              value: "15m"
            - name: JWT_REFRESH_TOKEN_EXPIRY
              value: "7d"
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: 300m
              memory: 384Mi
          livenessProbe:
            httpGet:
              path: /live
              port: http
            initialDelaySeconds: 30
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /ready
              port: http
            initialDelaySeconds: 10
            periodSeconds: 5
```

#### Task 3.2: API Service Deployment

**File:** `services/03-api-service/deployment.yaml`

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api-service
  labels:
    app: api-service
spec:
  replicas: 2
  selector:
    matchLabels:
      app: api-service
  template:
    metadata:
      labels:
        app: api-service
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8080"
        prometheus.io/path: "/metrics"
    spec:
      containers:
        - name: api-service
          image: api-service:latest
          imagePullPolicy: IfNotPresent
          ports:
            - containerPort: 8080
              name: http
          env:
            - name: SERVER_PORT
              value: "8080"
            - name: POSTGRES_HOST
              value: "postgres-api"
            - name: POSTGRES_PORT
              value: "5432"
            - name: POSTGRES_DATABASE
              value: "api_db"
            - name: POSTGRES_USER
              valueFrom:
                secretKeyRef:
                  name: postgres-api-secret
                  key: POSTGRES_USER
            - name: POSTGRES_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: postgres-api-secret
                  key: POSTGRES_PASSWORD
            - name: REDIS_HOST
              value: "redis"
            - name: REDIS_PORT
              value: "6379"
            - name: RABBITMQ_HOST
              value: "rabbitmq"
            - name: RABBITMQ_PORT
              value: "5672"
            - name: RABBITMQ_USER
              valueFrom:
                secretKeyRef:
                  name: rabbitmq-secret
                  key: RABBITMQ_USER
            - name: RABBITMQ_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: rabbitmq-secret
                  key: RABBITMQ_PASSWORD
            - name: AUTH_SERVICE_URL
              value: "http://auth-service:8080"
            - name: MINIO_ENDPOINT
              value: "minio:9000"
            - name: MINIO_ACCESS_KEY
              valueFrom:
                secretKeyRef:
                  name: minio-secret
                  key: MINIO_ACCESS_KEY
            - name: MINIO_SECRET_KEY
              valueFrom:
                secretKeyRef:
                  name: minio-secret
                  key: MINIO_SECRET_KEY
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: 500m
              memory: 512Mi
          livenessProbe:
            httpGet:
              path: /health
              port: http
            initialDelaySeconds: 10
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /ready
              port: http
            initialDelaySeconds: 5
            periodSeconds: 5
```

---

### Phase 4: Automation Scripts (Day 6)

#### Task 4.1: Common Functions

**File:** `scripts/common-functions.sh`

```bash
#!/bin/bash

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

wait_for_pod() {
    local label=$1
    local namespace=${2:-default}
    local timeout=${3:-120}
    
    log_info "Waiting for pod with label $label..."
    kubectl wait --for=condition=ready pod \
        -l "$label" \
        -n "$namespace" \
        --timeout="${timeout}s"
}

check_kubectl() {
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl not found"
        exit 1
    fi
}

check_kind() {
    if ! command -v kind &> /dev/null; then
        log_error "kind not found"
        exit 1
    fi
}
```

#### Task 4.2: Deploy All Script

**File:** `scripts/deploy-all.sh`

```bash
#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_DIR="$(dirname "$SCRIPT_DIR")"

source "$SCRIPT_DIR/common-functions.sh"

echo "🚀 Deploying Microservices Architecture V2"

# Deploy infrastructure
log_info "Deploying infrastructure..."
kubectl apply -f "$DEPLOY_DIR/services/01-infrastructure/postgres-api/"
kubectl apply -f "$DEPLOY_DIR/services/01-infrastructure/postgres-auth/"
kubectl apply -f "$DEPLOY_DIR/services/01-infrastructure/redis/"
kubectl apply -f "$DEPLOY_DIR/services/01-infrastructure/rabbitmq/"
kubectl apply -f "$DEPLOY_DIR/services/01-infrastructure/minio/"

# Wait for infrastructure
log_info "Waiting for infrastructure..."
wait_for_pod "app=postgres-api"
wait_for_pod "app=postgres-auth"
wait_for_pod "app=redis"
wait_for_pod "app=rabbitmq"
wait_for_pod "app=minio"

# Initialize MinIO buckets
log_info "Initializing MinIO buckets..."
kubectl apply -f "$DEPLOY_DIR/services/01-infrastructure/minio/init-job.yaml"

# Deploy auth service
log_info "Deploying auth-service..."
kubectl apply -f "$DEPLOY_DIR/services/02-auth-service/"
wait_for_pod "app=auth-service"

# Deploy api service
log_info "Deploying api-service..."
kubectl apply -f "$DEPLOY_DIR/services/03-api-service/"
wait_for_pod "app=api-service"

# Deploy graphrag service (optional)
if [ -d "$DEPLOY_DIR/services/04-graphrag-service" ]; then
    log_info "Deploying graphrag-service..."
    kubectl apply -f "$DEPLOY_DIR/services/04-graphrag-service/"
fi

# Deploy workers (optional)
if [ -d "$DEPLOY_DIR/services/05-operational-workers" ]; then
    log_info "Deploying operational workers..."
    kubectl apply -f "$DEPLOY_DIR/services/05-operational-workers/email-worker/"
    kubectl apply -f "$DEPLOY_DIR/services/05-operational-workers/image-worker/"
    kubectl apply -f "$DEPLOY_DIR/services/05-operational-workers/profile-worker/"
fi

# Deploy ingress
log_info "Deploying ingress..."
kubectl apply -f "$DEPLOY_DIR/ingress/"

echo ""
echo "✅ Deployment complete!"
echo ""
echo "Services:"
kubectl get pods
echo ""
echo "Access:"
echo "  API Service: http://localhost:8080"
echo "  Auth Service: http://localhost:8081"
echo "  RabbitMQ: http://localhost:15672"
echo "  MinIO Console: http://localhost:9001"
```

#### Task 4.3: Build All Images Script

**File:** `scripts/build-all-images.sh`

```bash
#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$(dirname "$SCRIPT_DIR")")"

source "$SCRIPT_DIR/common-functions.sh"

echo "🔨 Building all Docker images..."

# Auth service
log_info "Building auth-service..."
docker build -t auth-service:latest "$PROJECT_ROOT/auth-service"

# API service
log_info "Building api-service..."
docker build -t api-service:latest "$PROJECT_ROOT/api-service"

# GraphRAG service
if [ -d "$PROJECT_ROOT/graph-worker/graphrag-service" ]; then
    log_info "Building graphrag-service..."
    docker build -t graphrag-service:latest "$PROJECT_ROOT/graph-worker/graphrag-service"
fi

# Operational workers
if [ -d "$PROJECT_ROOT/graph-worker/operational-workers" ]; then
    log_info "Building operational workers..."
    cd "$PROJECT_ROOT/graph-worker/operational-workers"
    docker build -f Dockerfile.email -t email-worker:latest .
    docker build -f Dockerfile.image -t image-worker:latest .
    docker build -f Dockerfile.profile -t profile-worker:latest .
fi

echo "✅ All images built!"
docker images | grep -E "auth-service|api-service|graphrag-service|email-worker|image-worker|profile-worker"
```

#### Task 4.4: Load Images to Kind

**File:** `scripts/load-images-to-kind.sh`

```bash
#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common-functions.sh"

CLUSTER_NAME=${1:-microservices-v2}

echo "📦 Loading images to Kind cluster: $CLUSTER_NAME"

images=(
    "auth-service:latest"
    "api-service:latest"
    "graphrag-service:latest"
    "email-worker:latest"
    "image-worker:latest"
    "profile-worker:latest"
)

for image in "${images[@]}"; do
    if docker image inspect "$image" &> /dev/null; then
        log_info "Loading $image..."
        kind load docker-image "$image" --name "$CLUSTER_NAME"
    else
        log_warn "Image $image not found, skipping"
    fi
done

echo "✅ Images loaded!"
```

---

### Phase 5: Ingress Configuration (Day 7)

**File:** `ingress/ingress.yaml`

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: microservices-ingress
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /
spec:
  ingressClassName: nginx
  rules:
    - http:
        paths:
          - path: /api/v1
            pathType: Prefix
            backend:
              service:
                name: api-service
                port:
                  number: 8080
          - path: /v1/auth
            pathType: Prefix
            backend:
              service:
                name: auth-service
                port:
                  number: 8080
          - path: /v1/users
            pathType: Prefix
            backend:
              service:
                name: auth-service
                port:
                  number: 8080
```

---

### Phase 6: MongoDB Atlas Configuration (Day 7)

**File:** `external/mongodb-atlas/README.md`

```markdown
# MongoDB Atlas Configuration

## Setup Steps

1. Create MongoDB Atlas Account
   - Go to https://www.mongodb.com/cloud/atlas
   - Create account or sign in

2. Create Cluster
   - Choose M10 tier (recommended for production)
   - Select region closest to Kubernetes cluster
   - Name: `graphrag-cluster`

3. Create Database User
   - Username: `graphrag_user`
   - Password: Generate secure password
   - Role: Read/Write to graphrag database

4. Configure Network Access
   - Add IP whitelist for Kubernetes cluster
   - For development, can use 0.0.0.0/0 (not for production)

5. Get Connection String
   - Format: `mongodb+srv://graphrag_user:<password>@graphrag-cluster.xxxxx.mongodb.net/graphrag?retryWrites=true&w=majority`

6. Create Kubernetes Secret
   ```bash
   kubectl create secret generic mongodb-atlas-secret \
     --from-literal=MONGODB_URI='mongodb+srv://...'
   ```

## Collections

The graphrag database will have:
- `entities` - Extracted entities
- `relationships` - Entity relationships
- `communities` - Community clusters
- `documents` - Document metadata

## Vector Search (Optional)

Enable Atlas Vector Search for semantic queries:
1. Go to Atlas Search
2. Create search index on `entities` collection
3. Configure vector field for embeddings
```

**File:** `external/mongodb-atlas/connection-secret.yaml`

```yaml
# Template - DO NOT commit with real credentials
apiVersion: v1
kind: Secret
metadata:
  name: mongodb-atlas-secret
type: Opaque
stringData:
  MONGODB_URI: "mongodb+srv://user:password@cluster.mongodb.net/graphrag"
```

---

---

## 5. Port Assignments

| Service | Container Port | NodePort | Purpose |
|---------|---------------|----------|---------|
| api-service | 8080 | 30080 | Main gateway |
| auth-service | 8080 | 30081 | Authentication |
| graphrag-service | 8080/8081 | 30082/30083 | Health/Metrics |
| email-worker | 8080 | 30084 | Health |
| image-worker | 8080 | 30085 | Health |
| profile-worker | 8080 | 30086 | Health |
| PostgreSQL (api) | 5432 | - | Internal |
| PostgreSQL (auth) | 5432 | - | Internal |
| Redis | 6379 | - | Internal |
| RabbitMQ | 5672/15672 | -/30091 | AMQP/Management |
| MinIO | 9000/9001 | -/30093 | API/Console |

---

## 6. File Checklist

### Cluster Setup
- [ ] `cluster/kind-config.yaml`
- [ ] `cluster/kind-multinode.yaml`
- [ ] `cluster/setup-cluster.sh`
- [ ] `cluster/infrastructure/ingress-nginx.yaml`
- [ ] `cluster/infrastructure/metrics-server.yaml`
- [ ] `cluster/validation/test-cluster.sh`

### Infrastructure
- [ ] `services/01-infrastructure/postgres-api/*.yaml`
- [ ] `services/01-infrastructure/postgres-auth/*.yaml`
- [ ] `services/01-infrastructure/redis/*.yaml`
- [ ] `services/01-infrastructure/rabbitmq/*.yaml`
- [ ] `services/01-infrastructure/minio/*.yaml`

### Services
- [ ] `services/02-auth-service/*.yaml`
- [ ] `services/03-api-service/*.yaml`
- [ ] `services/04-graphrag-service/*.yaml`
- [ ] `services/05-operational-workers/email-worker/*.yaml`
- [ ] `services/05-operational-workers/image-worker/*.yaml`
- [ ] `services/05-operational-workers/profile-worker/*.yaml`

### External
- [ ] `external/mongodb-atlas/README.md`
- [ ] `external/mongodb-atlas/connection-secret.yaml`

### Ingress
- [ ] `ingress/ingress.yaml`

### Scripts
- [ ] `scripts/common-functions.sh`
- [ ] `scripts/deploy-all.sh`
- [ ] `scripts/build-all-images.sh`
- [ ] `scripts/load-images-to-kind.sh`
- [ ] `scripts/setup-infrastructure.sh`
- [ ] `scripts/teardown.sh`

---

## 7. Testing Checklist

### Cluster Tests
```bash
# Create cluster
./cluster/setup-cluster.sh

# Verify nodes
kubectl get nodes

# Verify ingress
kubectl get pods -n ingress-nginx
```

### Infrastructure Tests
```bash
# Deploy infrastructure
./scripts/setup-infrastructure.sh

# Verify all running
kubectl get pods

# Test PostgreSQL
kubectl exec -it postgres-api-0 -- psql -U api_user -d api_db -c "SELECT 1"

# Test Redis
kubectl exec -it redis-0 -- redis-cli ping

# Test RabbitMQ
# Access http://localhost:15672 (guest/guest)

# Test MinIO
# Access http://localhost:9001
```

### Service Tests
```bash
# Deploy all
./scripts/deploy-all.sh

# Test health endpoints
curl http://localhost:8080/health
curl http://localhost:8081/health

# Test auth
curl -X POST http://localhost:8081/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"password"}'
```

---

---

## 8. Success Criteria (Phase 1)

- [ ] Kind cluster created successfully
- [ ] All infrastructure pods running
- [ ] All service pods running
- [ ] Health checks passing
- [ ] Services can communicate
- [ ] Ingress routes traffic correctly
- [ ] Scripts work without errors

---

## 9. Phase 2: Cloud Migration Strategy

### 9.1 Overview

This section documents the future migration path from containerized infrastructure (Phase 1) to cloud-managed services (Phase 2).

**Key Principle:** Configuration-driven migration with zero code changes required.

### 9.2 Migration Approach

#### Step 1: Configuration Abstraction

All services connect to infrastructure using environment variables:

```yaml
# Phase 1 (Local Containers)
POSTGRES_HOST: postgres-api
POSTGRES_PORT: 5432

# Phase 2 (Cloud)
POSTGRES_HOST: mydb.xxxx.rds.amazonaws.com
POSTGRES_PORT: 5432
```

Services don't need to know if they're connecting to local or cloud infrastructure.

#### Step 2: Gradual Migration

Migrate one component at a time:

1. **PostgreSQL** → AWS RDS
2. **Redis** → AWS ElastiCache
3. **RabbitMQ** → AWS Amazon MQ
4. **MinIO** → AWS S3

Each migration can be tested independently.

### 9.3 Component-by-Component Migration

#### 9.3.1 PostgreSQL → AWS RDS

**Phase 1 (Current):**
```yaml
# StatefulSet in Kubernetes
postgres-api:5432
```

**Phase 2 (Cloud):**
```yaml
# Environment variables change only
POSTGRES_HOST: mydb-api.xxxx.us-east-1.rds.amazonaws.com
POSTGRES_PORT: 5432
POSTGRES_SSL_MODE: require
```

**Migration Steps:**
1. Create RDS instance (PostgreSQL 15)
2. Export data from local PostgreSQL
3. Import to RDS
4. Update ConfigMap with new host
5. Restart services
6. Verify connection
7. Remove local StatefulSet

**Code Changes Required:** None (connection string only)

---

#### 9.3.2 Redis → AWS ElastiCache

**Phase 1 (Current):**
```yaml
# StatefulSet in Kubernetes
redis:6379
```

**Phase 2 (Cloud):**
```yaml
# Environment variables change only
REDIS_HOST: myredis.xxxx.cache.amazonaws.com
REDIS_PORT: 6379
REDIS_TLS_ENABLED: true
```

**Migration Steps:**
1. Create ElastiCache Redis cluster
2. Update ConfigMap with new endpoint
3. Restart api-service
4. Verify caching works
5. Remove local StatefulSet

**Code Changes Required:** May need TLS support in Redis client

---

#### 9.3.3 RabbitMQ → AWS Amazon MQ

**Phase 1 (Current):**
```yaml
# StatefulSet in Kubernetes
rabbitmq:5672
```

**Phase 2 (Cloud):**
```yaml
# Environment variables change only
RABBITMQ_HOST: b-xxxx.mq.us-east-1.amazonaws.com
RABBITMQ_PORT: 5671
RABBITMQ_USE_SSL: true
```

**Migration Steps:**
1. Create Amazon MQ broker (RabbitMQ)
2. Recreate queue topology in cloud
3. Update ConfigMap with new broker
4. Restart all workers and api-service
5. Verify message flow
6. Remove local StatefulSet

**Code Changes Required:** SSL/TLS configuration

---

#### 9.3.4 MinIO → AWS S3

**Phase 1 (Current):**
```yaml
# StatefulSet in Kubernetes
MINIO_ENDPOINT: minio:9000
```

**Phase 2 (Cloud):**
```yaml
# Environment variables change only
MINIO_ENDPOINT: s3.amazonaws.com
S3_REGION: us-east-1
S3_BUCKET: my-documents-bucket
```

**Migration Steps:**
1. Create S3 bucket
2. Copy existing documents from MinIO to S3
3. Update ConfigMap with S3 endpoint
4. Update MinIO client to use S3 SDK
5. Restart services
6. Verify uploads/downloads
7. Remove local StatefulSet

**Code Changes Required:** Minor - MinIO client is S3-compatible, but may need region config

---

### 9.4 Configuration Management for Both Phases

#### Option A: Environment-Based ConfigMaps

```yaml
# deployment/services/03-api-service/configmap-local.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: api-service-config
data:
  POSTGRES_HOST: postgres-api
  REDIS_HOST: redis
  RABBITMQ_HOST: rabbitmq
  MINIO_ENDPOINT: minio:9000
```

```yaml
# deployment/services/03-api-service/configmap-cloud.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: api-service-config
data:
  POSTGRES_HOST: mydb.xxxx.rds.amazonaws.com
  REDIS_HOST: myredis.xxxx.cache.amazonaws.com
  RABBITMQ_HOST: b-xxxx.mq.us-east-1.amazonaws.com
  MINIO_ENDPOINT: s3.amazonaws.com
```

Deploy with: `kubectl apply -f configmap-local.yaml` or `configmap-cloud.yaml`

#### Option B: Kustomize Overlays

```
deployment/
├── base/                    # Common manifests
│   └── api-service/
│       ├── deployment.yaml
│       └── service.yaml
├── overlays/
│   ├── local/              # Phase 1: Container infrastructure
│   │   └── kustomization.yaml
│   └── cloud/              # Phase 2: Cloud services
│       └── kustomization.yaml
```

Deploy with: `kubectl apply -k overlays/local/` or `overlays/cloud/`

---

### 9.5 Testing Cloud Migration

#### Pre-Migration Checklist

- [ ] Create cloud resources (RDS, ElastiCache, etc.)
- [ ] Verify network connectivity from K8s to cloud services
- [ ] Test SSL/TLS connections
- [ ] Backup all local data
- [ ] Document rollback procedure
- [ ] Set up cloud monitoring

#### Post-Migration Validation

```bash
# Test PostgreSQL connection
kubectl exec -it api-service-xxx -- psql -h $POSTGRES_HOST -U $POSTGRES_USER -d api_db -c "SELECT 1"

# Test Redis
kubectl exec -it api-service-xxx -- redis-cli -h $REDIS_HOST ping

# Test RabbitMQ
curl -u $RABBITMQ_USER:$RABBITMQ_PASSWORD https://$RABBITMQ_HOST:15671/api/overview

# Test S3
aws s3 ls s3://$S3_BUCKET/
```

#### Rollback Strategy

If migration fails, rollback by:
1. Redeploy local infrastructure StatefulSets
2. Revert ConfigMaps to local endpoints
3. Restart services
4. Restore data from backups if needed

---

### 9.6 Cost Comparison

#### Phase 1 (Local/K8s) - Monthly Costs

| Component | Kubernetes Resources | Cloud Cost |
|-----------|---------------------|------------|
| postgres-api | 1-2 cores, 2-4Gi | $0 (in K8s) |
| postgres-auth | 0.5-1 core, 1-2Gi | $0 (in K8s) |
| redis | 0.5 core, 512Mi-1Gi | $0 (in K8s) |
| rabbitmq | 1 core, 1-2Gi | $0 (in K8s) |
| minio | 0.5-1 core, 1-2Gi | $0 (in K8s) |
| mongodb-atlas | - | $57-100 |
| **Total** | **~4-6 cores, ~7-13Gi** | **$57-100/mo** |

#### Phase 2 (Cloud) - Monthly Costs (AWS Example)

| Component | Service | Configuration | Estimated Cost |
|-----------|---------|---------------|----------------|
| PostgreSQL | RDS | db.t3.small (2 instances) | $50-70 |
| Redis | ElastiCache | cache.t3.micro | $15-20 |
| RabbitMQ | Amazon MQ | mq.t3.micro | $40-50 |
| Storage | S3 | 100GB + requests | $10-15 |
| MongoDB | Atlas M10 | Managed | $57-100 |
| **Total** | | | **$172-255/mo** |

**Note:** Cloud costs provide managed backups, HA, and reduced operational overhead.

---

### 9.7 When to Migrate to Cloud

Consider migrating when:

- [ ] **Production traffic increases** - Need automatic scaling
- [ ] **High availability required** - Need multi-AZ deployment
- [ ] **Data growth exceeds local storage** - Need scalable storage
- [ ] **Team lacks ops expertise** - Need managed services
- [ ] **Compliance requires managed backups** - Need automated backup/recovery
- [ ] **Development velocity matters** - Need to focus on features, not infrastructure

**Recommendation:** Start with Phase 1 (local) for development and initial production. Migrate to Phase 2 when you reach 1000+ active users or need 99.9% uptime.

---

## 10. Implementation Checklist

### Phase 1 Checklist (This Plan)
- [ ] All infrastructure as StatefulSets in K8s
- [ ] Services connect via environment variables
- [ ] Configuration flexible for future cloud migration
- [ ] MongoDB Atlas as external cloud service

### Phase 2 Checklist (Future)
- [ ] Document cloud service requirements
- [ ] Create cloud infrastructure (RDS, ElastiCache, etc.)
- [ ] Test connectivity from K8s to cloud
- [ ] Create migration scripts
- [ ] Update ConfigMaps for cloud endpoints
- [ ] Validate each service after migration
- [ ] Remove local StatefulSets
- [ ] Monitor cloud costs

---

*Document Version: 1.1*  
*Created: January 2026*  
*Updated: Added Phase 2 cloud migration strategy*  
*Estimated Effort: 5-7 days (Phase 1), 3-5 days (Phase 2 migration)*
