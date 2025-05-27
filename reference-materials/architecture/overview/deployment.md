# Deployment Architecture

## Overview

This document describes the deployment architecture of the Profile Service Microservices, including the Kubernetes infrastructure, service deployment patterns, and operational considerations.

## Infrastructure Components

### 1. Kubernetes Cluster

- **Control Plane**
  - API Server
  - Scheduler
  - Controller Manager
  - etcd
- **Worker Nodes**
  - Multiple nodes for high availability
  - Auto-scaling configuration
  - Resource quotas and limits

### 2. Networking

- **Ingress Controller**
  - Nginx Ingress
  - SSL/TLS termination
  - Load balancing
  - Rate limiting
- **Service Mesh**
  - Istio for service-to-service communication
  - Traffic management
  - Security policies
  - Observability

### 3. Storage

- **Persistent Storage**
  - Database volumes
  - Message queue storage
  - Log storage
- **Caching Layer**
  - Redis cluster
  - Cache persistence

## Service Deployment

### 1. Profile API Service

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: profile-api
spec:
  replicas: 3
  selector:
    matchLabels:
      app: profile-api
  template:
    metadata:
      labels:
        app: profile-api
    spec:
      containers:
        - name: profile-api
          image: profile-api:latest
          ports:
            - containerPort: 8080
          resources:
            requests:
              memory: "256Mi"
              cpu: "200m"
            limits:
              memory: "512Mi"
              cpu: "500m"
          livenessProbe:
            httpGet:
              path: /health
              port: 8080
          readinessProbe:
            httpGet:
              path: /ready
              port: 8080
```

### 2. Profile Cache Service

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: profile-cache
spec:
  replicas: 2
  selector:
    matchLabels:
      app: profile-cache
  template:
    metadata:
      labels:
        app: profile-cache
    spec:
      containers:
        - name: profile-cache
          image: profile-cache:latest
          ports:
            - containerPort: 8080
          resources:
            requests:
              memory: "128Mi"
              cpu: "100m"
            limits:
              memory: "256Mi"
              cpu: "200m"
```

### 3. Profile Storage Service

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: profile-storage
spec:
  replicas: 2
  selector:
    matchLabels:
      app: profile-storage
  template:
    metadata:
      labels:
        app: profile-storage
    spec:
      containers:
        - name: profile-storage
          image: profile-storage:latest
          ports:
            - containerPort: 8080
          resources:
            requests:
              memory: "256Mi"
              cpu: "200m"
            limits:
              memory: "512Mi"
              cpu: "500m"
```

## Infrastructure Services

### 1. Redis Cluster

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: redis
spec:
  serviceName: redis
  replicas: 3
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
          image: redis:7.0
          ports:
            - containerPort: 6379
          volumeMounts:
            - name: redis-data
              mountPath: /data
  volumeClaimTemplates:
    - metadata:
        name: redis-data
      spec:
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: 10Gi
```

### 2. RabbitMQ Cluster

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: rabbitmq
spec:
  serviceName: rabbitmq
  replicas: 3
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
          image: rabbitmq:3.9-management
          ports:
            - containerPort: 5672
            - containerPort: 15672
          volumeMounts:
            - name: rabbitmq-data
              mountPath: /var/lib/rabbitmq
  volumeClaimTemplates:
    - metadata:
        name: rabbitmq-data
      spec:
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: 20Gi
```

## Monitoring Stack

### 1. Prometheus

```yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: prometheus
spec:
  serviceName: prometheus
  replicas: 1
  selector:
    matchLabels:
      app: prometheus
  template:
    metadata:
      labels:
        app: prometheus
    spec:
      containers:
        - name: prometheus
          image: prom/prometheus:v2.45.0
          ports:
            - containerPort: 9090
          volumeMounts:
            - name: prometheus-data
              mountPath: /prometheus
  volumeClaimTemplates:
    - metadata:
        name: prometheus-data
      spec:
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: 50Gi
```

### 2. Grafana

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: grafana
spec:
  replicas: 1
  selector:
    matchLabels:
      app: grafana
  template:
    metadata:
      labels:
        app: grafana
    spec:
      containers:
        - name: grafana
          image: grafana/grafana:9.5.2
          ports:
            - containerPort: 3000
          volumeMounts:
            - name: grafana-data
              mountPath: /var/lib/grafana
  volumeClaimTemplates:
    - metadata:
        name: grafana-data
      spec:
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: 10Gi
```

## Deployment Considerations

### 1. High Availability

- Multiple replicas for each service
- Anti-affinity rules for pod distribution
- Proper resource requests and limits
- Health checks and readiness probes

### 2. Scalability

- Horizontal Pod Autoscaling (HPA)
- Resource-based scaling
- Custom metrics for scaling
- Node auto-scaling

### 3. Security

- Network policies
- Pod security policies
- Secret management
- RBAC configuration

### 4. Monitoring

- Service mesh observability
- Custom metrics
- Log aggregation
- Alerting rules

## Next Steps

1. Review [Kubernetes Tools](../../development/tools/kubernetes/README.md) documentation
2. Implement [Helm Guide](../../development/tools/kubernetes/helm.md) for service packaging
3. Use [Kustomize Guide](../../development/tools/kubernetes/kustomize.md) for environment-specific changes
4. Follow [Tool Comparison](../../development/tools/kubernetes/comparison.md) for best practices
5. Set up monitoring dashboards
6. Configure alerting rules
7. Create backup strategies
