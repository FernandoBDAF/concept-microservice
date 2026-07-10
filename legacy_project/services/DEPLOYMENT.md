# Deployment Strategy and Environments

This document provides comprehensive deployment guidance for the production-ready microservices ecosystem, covering infrastructure requirements, deployment strategies, and operational procedures.

## 🏗️ **Deployment Philosophy: Dual Approach**

Our deployment strategy follows a **dual approach** designed to serve different operational needs:

### **🔍 Manual Deployment for Analysis**

- **Purpose**: Step-by-step analysis and understanding of each manifest
- **Use Cases**: Learning, troubleshooting, detailed inspection, educational purposes
- **Benefits**: Complete visibility into each component, easier debugging, better understanding
- **When to Use**: Initial setup, problem diagnosis, training, manifest validation

### **⚡ Kustomize Deployment for Operations**

- **Purpose**: Regular, consistent, and automated deployments
- **Use Cases**: Daily operations, CI/CD pipelines, environment management
- **Benefits**: Consistency, automation, environment-specific customization, reduced errors
- **When to Use**: Regular deployments, production operations, automated workflows

**Both approaches are REQUIRED and complementary** - manual for understanding, Kustomize for efficiency.

## 🎯 **Service Deployment Status**

| Service     | Manual Deployment | Kustomize Deployment | Production Ready | Notes                     |
| ----------- | ----------------- | -------------------- | ---------------- | ------------------------- |
| **Profile** | ✅ Complete       | ✅ Complete          | ✅ Ready         | Full standardization      |
| **Cache**   | ✅ Complete       | ✅ Complete          | ✅ Ready         | Production optimized      |
| **Storage** | ✅ Complete       | ✅ Complete          | ✅ Ready         | Auth data integrated      |
| **Queue**   | ✅ Complete       | ✅ Complete          | ✅ Ready         | Multi-worker routing      |
| **Worker**  | ✅ Complete       | ✅ Complete          | ✅ Ready         | Multi-worker architecture |
| **Auth**    | ✅ Complete       | ✅ Complete          | ✅ Ready         | Microservices compliant   |

## 🏛️ **Infrastructure Requirements**

### **Core Infrastructure Services**

Before deploying application services, ensure these infrastructure components are available:

#### **PostgreSQL Database**

```yaml
# Required for storage-service
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: postgresql
spec:
  serviceName: postgresql
  replicas: 1
  template:
    spec:
      containers:
        - name: postgresql
          image: postgres:15
          env:
            - name: POSTGRES_DB
              value: microservices
            - name: POSTGRES_USER
              value: postgres
            - name: POSTGRES_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: postgresql-secret
                  key: password
          ports:
            - containerPort: 5432
          volumeMounts:
            - name: postgresql-data
              mountPath: /var/lib/postgresql/data
  volumeClaimTemplates:
    - metadata:
        name: postgresql-data
      spec:
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: 10Gi
```

#### **Redis Cache**

```yaml
# Required for cache-service backend
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: redis
spec:
  serviceName: redis
  replicas: 1
  template:
    spec:
      containers:
        - name: redis
          image: redis:7-alpine
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
            storage: 5Gi
```

#### **RabbitMQ Message Broker**

```yaml
# Required for queue-service and worker-service
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: rabbitmq
spec:
  serviceName: rabbitmq
  replicas: 1
  template:
    spec:
      containers:
        - name: rabbitmq
          image: rabbitmq:3-management
          env:
            - name: RABBITMQ_DEFAULT_USER
              value: admin
            - name: RABBITMQ_DEFAULT_PASS
              valueFrom:
                secretKeyRef:
                  name: rabbitmq-secret
                  key: password
          ports:
            - containerPort: 5672 # AMQP
            - containerPort: 15672 # Management UI
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
            storage: 5Gi
```

## 🚀 **Deployment Environments**

### **Development Environment (Kind)**

Local development using Kind clusters with optimized configurations:

```bash
# 1. Create Kind cluster
kind create cluster --config=deployment/kind/kind-config.yaml

# 2. Deploy infrastructure
kubectl apply -f deployment/kubernetes/infrastructure/

# 3. Deploy services using Kustomize
kubectl apply -k deployment/kind/

# 4. Verify deployment
kubectl get pods -A
kubectl port-forward service/profile-service 8080:8080
```

**Kind-Specific Optimizations**:

- **NodePort Services**: External access for testing
- **Reduced Resource Limits**: Optimized for local development
- **Development Dependencies**: Additional debugging tools
- **Local Storage**: EmptyDir volumes for rapid iteration

### **Staging Environment**

Pre-production environment for integration testing:

```bash
# 1. Deploy infrastructure services
kubectl apply -f deployment/kubernetes/infrastructure/ -n staging

# 2. Deploy application services
kubectl apply -f deployment/kubernetes/services/ -n staging

# 3. Configure ingress and networking
kubectl apply -f deployment/kubernetes/networking/ -n staging

# 4. Run integration tests
kubectl apply -f deployment/testing/integration-tests.yaml -n staging
```

### **Production Environment**

High-availability production deployment:

```bash
# 1. Deploy infrastructure with HA configuration
kubectl apply -f deployment/kubernetes/production/infrastructure/ -n production

# 2. Deploy services with production scaling
kubectl apply -f deployment/kubernetes/production/services/ -n production

# 3. Configure monitoring and alerting
kubectl apply -f deployment/kubernetes/production/monitoring/ -n production

# 4. Verify production readiness
kubectl get pods -n production
kubectl get hpa -n production
```

## 📋 **Standard Deployment Structure**

Every service follows the standardized deployment structure:

```
services/{service-name}/deployments/
├── README.md                          # Service-specific deployment guide
├── STEP_BY_STEP_DEPLOYMENT_GUIDE.md  # Detailed manual deployment
├── kubernetes/                        # Base production manifests
│   ├── deployment.yaml               # Production deployment
│   ├── service.yaml                  # Service + RBAC + HPA
│   ├── configmap.yaml                # Configuration
│   └── secrets.yaml                  # Secret templates
├── kind/                             # Kind-specific overlays
│   ├── kustomization.yaml            # Kind kustomization
│   ├── deployment-patch.yaml         # Kind patches
│   ├── service-patch.yaml            # NodePort patches
│   ├── {service}-dependencies.yaml   # Development dependencies
│   └── deploy-to-kind.sh             # Automated deployment
├── scripts/                          # Manual deployment scripts
│   ├── manual-deploy.sh              # Interactive deployment
│   ├── manual-cleanup.sh             # Cleanup procedures
│   └── rollback-procedures.sh        # Recovery procedures
└── monitoring/                       # Monitoring configuration
    └── servicemonitor.yaml           # Prometheus ServiceMonitor
```

## 🔧 **Manual Deployment Procedures**

### **Step-by-Step Manual Deployment**

Each service includes comprehensive manual deployment guides:

```bash
# Example: Profile Service Manual Deployment
cd services/profile-service/deployments

# 1. Review deployment guide
cat STEP_BY_STEP_DEPLOYMENT_GUIDE.md

# 2. Execute manual deployment script
./scripts/manual-deploy.sh
```

## ⚡ **Kustomize Deployment Procedures**

### **Automated Kustomize Deployment**

For operational efficiency, use Kustomize for consistent deployments:

```bash
# Deploy all services to Kind (development)
kubectl apply -k deployment/kind/

# Deploy specific service to staging
kubectl apply -k services/profile-service/deployments/kind/ -n staging

# Deploy to production with production overlays
kubectl apply -k deployment/production/
```

## 🔍 **Health Checks and Validation**

### **Deployment Validation Script**

```bash
#!/bin/bash
# deployment/scripts/validate-deployment.sh

echo "🔍 Validating Microservices Deployment"
echo "======================================"

# Check all pods are running
echo "Checking pod status..."
kubectl get pods -n microservices -o wide

# Check services are accessible
echo "Checking service endpoints..."
for service in auth-service profile-service cache-service storage-service queue-service; do
    echo "Testing $service health check..."
    kubectl exec -n microservices deployment/profile-service -- \
        curl -s http://$service:8080/health | jq '.status'
done

echo "✅ Deployment validation complete"
```

## 📊 **Monitoring and Observability**

### **Prometheus Integration**

All services include ServiceMonitor configurations for automatic discovery:

```yaml
# Example ServiceMonitor
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: profile-service
  labels:
    app: profile-service
spec:
  selector:
    matchLabels:
      app: profile-service
  endpoints:
    - port: http
      path: /metrics
      interval: 30s
      scrapeTimeout: 10s
```

## 🚨 **Operational Procedures**

### **Scaling Procedures**

```bash
# Manual scaling
kubectl scale deployment profile-service --replicas=5 -n microservices
kubectl scale deployment email-worker --replicas=10 -n microservices

# Auto-scaling configuration
kubectl autoscale deployment profile-service \
  --cpu-percent=70 \
  --min=2 \
  --max=10 \
  -n microservices

# Check HPA status
kubectl get hpa -n microservices
kubectl describe hpa profile-service -n microservices
```

### **Rolling Updates**

```bash
# Update service image
kubectl set image deployment/profile-service \
  profile-service=profile-service:v2.0.0 \
  -n microservices

# Monitor rollout
kubectl rollout status deployment/profile-service -n microservices

# Rollback if needed
kubectl rollout undo deployment/profile-service -n microservices
```

### **Troubleshooting Procedures**

```bash
# Check pod status and events
kubectl get pods -n microservices
kubectl describe pod <pod-name> -n microservices

# View service logs
kubectl logs -f deployment/profile-service -n microservices
kubectl logs -f deployment/auth-service -n microservices --previous

# Check service connectivity
kubectl exec -n microservices deployment/profile-service -- \
  curl -v http://auth-service:8080/health

# Check resource usage
kubectl top pods -n microservices
kubectl top nodes
```

## 🔐 **Security and Compliance**

### **RBAC Configuration**

```yaml
# Service-specific RBAC
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  namespace: microservices
  name: profile-service
rules:
  - apiGroups: [""]
    resources: ["pods", "services", "endpoints"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["apps"]
    resources: ["deployments"]
    verbs: ["get", "list", "watch"]
```

### **Network Policies**

```yaml
# Restrict inter-service communication
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: profile-service-netpol
  namespace: microservices
spec:
  podSelector:
    matchLabels:
      app: profile-service
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              name: ingress-nginx
      ports:
        - protocol: TCP
          port: 8080
```

## 🎯 **Deployment Success Criteria**

### **Validation Checklist**

- [ ] **Infrastructure Services**: PostgreSQL, Redis, RabbitMQ healthy
- [ ] **Application Services**: All 6 services running and ready
- [ ] **Health Checks**: All health endpoints responding correctly
- [ ] **Service Communication**: Inter-service HTTP calls working
- [ ] **Message Flow**: Queue publishing and worker consumption operational
- [ ] **Authentication**: JWT token flow working end-to-end
- [ ] **Monitoring**: Prometheus metrics collection active
- [ ] **Scaling**: HPA configuration working correctly
- [ ] **Security**: RBAC and network policies enforced

## 📚 **Deployment Documentation**

For detailed deployment procedures, see individual service deployment guides:

- **Service Details**: [SERVICES.md](./SERVICES.md)
- **Integration Patterns**: [INTEGRATION.md](./INTEGRATION.md)
- **Architecture Context**: [CONTEXT.md](./CONTEXT.md)
- **Individual Service Deployments**: Each service's `deployments/README.md`

---

**Deployment Status**: ✅ **PRODUCTION READY** - Complete deployment standardization with dual approach supporting both manual analysis and automated operations.
