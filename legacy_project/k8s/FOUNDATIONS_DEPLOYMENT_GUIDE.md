# Microservices Foundations Deployment Guide

**Date**: December 29, 2024  
**Purpose**: Foundation cluster setup and infrastructure deployment for Kind microservices  
**Audience**: Platform engineers and developers setting up the core Kubernetes infrastructure  
**Scope**: Kind cluster creation, infrastructure services deployment, and foundation validation

---

## 📋 **Table of Contents**

### **🎯 Quick Navigation**

- [🎯 Overview and Learning Objectives](#-overview-and-learning-objectives)
- [🚨 CRITICAL: Docker Image Build Requirements](#-critical-docker-image-build-requirements)
- [📋 Prerequisites and Setup](#-prerequisites-and-setup)
- [🏗️ Foundation Setup: Kind Cluster Creation](#-foundation-setup-kind-cluster-creation)
- [🏗️ Foundation Setup: Infrastructure Services](#-foundation-setup-infrastructure-services)
- [🔍 Foundation Validation](#-foundation-validation)
- [🎉 Foundation Setup Complete](#-foundation-setup-complete)

### **📊 Quick Reference Sections**

- [Docker Image Build Requirements](#docker-image-build-requirements)
- [Prerequisites Checklist](#prerequisites-checklist)
- [Kind Cluster Setup](#step-1-create-kind-cluster-)
- [Infrastructure Services Deployment](#step-2-deploy-infrastructure-services-)
- [Comprehensive Infrastructure Validation](#step-3-comprehensive-infrastructure-validation)

### **🔧 Infrastructure Components**

- [Kind Cluster Configuration](#kind-cluster-configuration) - Cluster setup with port mappings
- [Ingress Controller](#ingress-controller-setup) - NGINX ingress for external access
- [Metrics Server](#metrics-server-deployment) - Resource monitoring capabilities
- [Storage Provisioner](#storage-provisioner-validation) - Persistent volume management
- [Network Policies](#network-policy-framework) - Security and traffic control

### **⚙️ Validation Categories**

- [Cluster Validation](#cluster-readiness-check) - Node status and system components
- [Infrastructure Validation](#infrastructure-services-validation) - Core services health
- [Network Validation](#network-connectivity-testing) - Ingress and DNS functionality
- [Storage Validation](#storage-functionality-testing) - Persistent volume operations

---

## 🎯 **Overview and Learning Objectives**

This guide provides step-by-step instructions for setting up the foundational Kubernetes infrastructure before deploying the microservices. It focuses on creating a production-like Kind cluster with all necessary infrastructure components.

### **What You'll Learn**

- **Kind Cluster Setup**: Creating a production-like Kubernetes cluster locally
- **Infrastructure Services**: Deploying essential cluster services (ingress, metrics, storage)
- **Network Configuration**: Setting up ingress controllers and network policies
- **Storage Management**: Configuring persistent storage for stateful services
- **Validation Techniques**: Comprehensive testing of infrastructure components

### **Prerequisites**

- ✅ **Docker installed**: Version 20.10+ with sufficient resources (8GB+ RAM)
- ✅ **Kind installed**: Version 0.17+ for Kubernetes cluster management
- ✅ **kubectl installed**: Version 1.25+ for cluster interaction
- ✅ **System resources**: Minimum 8GB RAM, 4 CPU cores, 50GB disk space

---

## 🚨 **CRITICAL: Docker Image Build Requirements**

### **Docker Image Build Requirements**

**⚠️ IMPORTANT**: Some microservices require custom Docker images that must be built and loaded into the Kind cluster before deployment.

#### **Services Requiring Custom Images**

1. **Profile Service** (`services/profile-service/`)

   ```bash
   cd services/profile-service/
   docker build -t profile-service:latest .
   kind load docker-image profile-service:latest --name microservices-kind
   ```

2. **Worker Service** (`services/worker-service/`)
   ```bash
   cd services/worker-service/
   # CRITICAL: Cross-platform compilation required
   GOOS=linux GOARCH=amd64 go build -o email-worker-linux cmd/email-worker/main.go
   GOOS=linux GOARCH=amd64 go build -o image-worker-linux cmd/image-worker/main.go
   docker build -t worker-service:latest .
   kind load docker-image worker-service:latest --name microservices-kind
   ```

#### **Why This Is Critical**

- **Profile Service**: Custom Node.js/Go application requiring specific dependencies
- **Worker Service**: Go binaries must be compiled for Linux architecture (containers run Linux)
- **Kind Limitation**: Cannot pull from external registries without proper configuration
- **Deployment Failure**: Services will fail with `ErrImagePull` if images aren't pre-loaded

#### **Verification Commands**

```bash
# Verify images are loaded in Kind cluster
docker exec -it microservices-kind-control-plane crictl images | grep -E "(profile-service|worker-service)"

# Expected output:
# profile-service:latest
# worker-service:latest
```

---

## 📋 **Prerequisites and Setup**

### **Prerequisites Checklist**

#### **System Requirements**

- [ ] **Operating System**: macOS, Linux, or Windows with WSL2
- [ ] **CPU**: Minimum 4 cores (8+ recommended for full microservices)
- [ ] **RAM**: Minimum 8GB (16GB+ recommended)
- [ ] **Disk Space**: Minimum 50GB free space
- [ ] **Network**: Internet connection for image downloads

#### **Software Requirements**

- [ ] **Docker**: Version 20.10+

  ```bash
  docker --version
  # Expected: Docker version 20.10.x or higher
  ```

- [ ] **Kind**: Version 0.17+

  ```bash
  kind --version
  # Expected: kind v0.17.x or higher
  ```

- [ ] **kubectl**: Version 1.25+

  ```bash
  kubectl version --client
  # Expected: Client Version v1.25.x or higher
  ```

- [ ] **jq**: JSON processor for response parsing
  ```bash
  jq --version
  # Expected: jq-1.6 or higher
  ```

#### **Docker Configuration**

```bash
# Verify Docker is running and has sufficient resources
docker info | grep -E "(CPUs|Total Memory)"

# Expected output (minimum):
# CPUs: 4
# Total Memory: 8GiB
```

#### **Port Availability Check**

```bash
# Check that required ports are available
netstat -tuln | grep -E "(30081|30082|30083|30084|30085|30086|15672|80)"

# No output expected (ports should be free)
```

---

## 🏗️ **Foundation Setup: Kind Cluster Creation**

### **Step 1: Create Kind Cluster** ✅

#### **Kind Cluster Configuration**

The Kind cluster is configured with specific port mappings to enable external access to microservices:

```yaml
# k8s/cluster/kind-config.yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: microservices-kind

nodes:
  - role: control-plane
    kubeadmConfigPatches:
      - |
        kind: InitConfiguration
        nodeRegistration:
          kubeletExtraArgs:
            node-labels: "ingress-ready=true"

    extraPortMappings:
      # Ingress HTTP
      - containerPort: 80
        hostPort: 80
        protocol: TCP

      # Ingress HTTPS
      - containerPort: 443
        hostPort: 443
        protocol: TCP

      # Microservices NodePorts
      - containerPort: 30081
        hostPort: 30081
        protocol: TCP # Cache Service

      - containerPort: 30082
        hostPort: 30082
        protocol: TCP # Storage Service (HTTP)

      - containerPort: 30083
        hostPort: 30083
        protocol: TCP # Auth Service

      - containerPort: 30084
        hostPort: 30084
        protocol: TCP # Queue Service

      - containerPort: 30085
        hostPort: 30085
        protocol: TCP # Profile Service

      - containerPort: 30086
        hostPort: 30086
        protocol: TCP # Worker Service

      - containerPort: 30092
        hostPort: 30092
        protocol: TCP # Storage Service (gRPC)

      # RabbitMQ Management
      - containerPort: 15672
        hostPort: 15672
        protocol: TCP
```

#### **Cluster Creation Process**

```bash
# Navigate to cluster configuration directory
cd k8s/cluster/

# Create Kind cluster with custom configuration
echo "🏗️ Creating Kind cluster with microservices configuration..."
kind create cluster --config=kind-config.yaml --name=microservices-kind

# Verify cluster creation
kubectl cluster-info --context kind-microservices-kind

# Check node status
kubectl get nodes -o wide

# Expected output:
# NAME                              STATUS   ROLES           AGE   VERSION
# microservices-kind-control-plane  Ready    control-plane   1m    v1.25.x
```

#### **Cluster Readiness Check**

```bash
# Wait for all system pods to be ready
echo "⏳ Waiting for system pods to be ready..."
kubectl wait --for=condition=Ready pods --all -n kube-system --timeout=300s

# Verify system components
kubectl get componentstatuses

# Check system pod status
kubectl get pods -n kube-system

# All pods should be in Running or Completed status
```

---

## 🏗️ **Foundation Setup: Infrastructure Services**

### **Step 2: Deploy Infrastructure Services** ✅

#### **Ingress Controller Setup**

```bash
echo "🌐 Deploying NGINX Ingress Controller..."

# Deploy NGINX Ingress Controller
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml

# Wait for ingress controller to be ready
echo "⏳ Waiting for ingress controller to be ready..."
kubectl wait --namespace ingress-nginx \
  --for=condition=ready pod \
  --selector=app.kubernetes.io/component=controller \
  --timeout=300s

# Verify ingress controller
kubectl get pods -n ingress-nginx
kubectl get svc -n ingress-nginx

# Test ingress controller accessibility
curl -s http://localhost:80 || echo "Ingress controller ready (404 expected without apps)"
```

#### **Metrics Server Deployment**

```bash
echo "📊 Deploying Metrics Server..."

# Deploy Metrics Server with Kind-specific configuration
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: metrics-server
  namespace: kube-system
  labels:
    k8s-app: metrics-server
spec:
  selector:
    matchLabels:
      k8s-app: metrics-server
  template:
    metadata:
      labels:
        k8s-app: metrics-server
    spec:
      containers:
      - args:
        - --cert-dir=/tmp
        - --secure-port=4443
        - --kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname
        - --kubelet-use-node-status-port
        - --metric-resolution=15s
        - --kubelet-insecure-tls  # Required for Kind
        image: k8s.gcr.io/metrics-server/metrics-server:v0.6.2
        name: metrics-server
        ports:
        - containerPort: 4443
          name: https
          protocol: TCP
        readinessProbe:
          failureThreshold: 3
          httpGet:
            path: /readyz
            port: https
            scheme: HTTPS
          periodSeconds: 10
        livenessProbe:
          failureThreshold: 3
          httpGet:
            path: /livez
            port: https
            scheme: HTTPS
          periodSeconds: 10
        resources:
          requests:
            cpu: 100m
            memory: 200Mi
        securityContext:
          allowPrivilegeEscalation: false
          readOnlyRootFilesystem: true
          runAsNonRoot: true
          runAsUser: 1000
        volumeMounts:
        - mountPath: /tmp
          name: tmp-dir
      nodeSelector:
        kubernetes.io/os: linux
      priorityClassName: system-cluster-critical
      serviceAccountName: metrics-server
      volumes:
      - emptyDir: {}
        name: tmp-dir
---
apiVersion: v1
kind: Service
metadata:
  name: metrics-server
  namespace: kube-system
  labels:
    kubernetes.io/name: "Metrics-server"
    kubernetes.io/cluster-service: "true"
spec:
  selector:
    k8s-app: metrics-server
  ports:
  - port: 443
    protocol: TCP
    targetPort: https
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: metrics-server
  namespace: kube-system
  labels:
    k8s-app: metrics-server
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: system:metrics-server
  labels:
    k8s-app: metrics-server
rules:
- apiGroups:
  - ""
  resources:
  - nodes/metrics
  verbs:
  - get
- apiGroups:
  - ""
  resources:
  - pods
  - nodes
  verbs:
  - get
  - list
  - watch
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: system:metrics-server
  labels:
    k8s-app: metrics-server
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:metrics-server
subjects:
- kind: ServiceAccount
  name: metrics-server
  namespace: kube-system
EOF

# Wait for metrics server to be ready
echo "⏳ Waiting for metrics server to be ready..."
kubectl wait --for=condition=Available deployment/metrics-server -n kube-system --timeout=300s

# Test metrics server (may take a few minutes to start collecting metrics)
echo "📊 Testing metrics server..."
sleep 30
kubectl top nodes || echo "⚠️ Metrics collection starting (this is normal)"
```

#### **Storage Provisioner Validation**

```bash
echo "💾 Validating storage provisioner..."

# Check if local-path storage class exists
kubectl get storageclass

# Expected output should include:
# standard (default)   rancher.io/local-path   Delete          WaitForFirstConsumer   false

# Verify local-path-provisioner is running
kubectl get pods -n local-path-storage

# Test storage functionality with a test PVC
kubectl apply -f - <<EOF
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: test-pvc
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: standard
EOF

# Check PVC status (should be Pending until a pod uses it)
kubectl get pvc test-pvc

# Clean up test PVC
kubectl delete pvc test-pvc

echo "✅ Storage provisioner validated"
```

#### **Network Policy Framework**

```bash
echo "🔒 Setting up network policy framework..."

# Deploy basic network policies for microservices namespace
kubectl create namespace microservices 2>/dev/null || echo "Namespace already exists"

# Apply default deny-all network policy
kubectl apply -f - <<EOF
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: default-deny-all
  namespace: microservices
spec:
  podSelector: {}
  policyTypes:
  - Ingress
  - Egress
EOF

# Apply ingress controller access policy
kubectl apply -f - <<EOF
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-ingress-controller
  namespace: microservices
spec:
  podSelector: {}
  policyTypes:
  - Ingress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: ingress-nginx
EOF

# Label the ingress-nginx namespace
kubectl label namespace ingress-nginx name=ingress-nginx --overwrite

echo "✅ Network policy framework established"
```

---

## 🔍 **Foundation Validation**

### **Step 3: Comprehensive Infrastructure Validation**

#### **Cluster Readiness Check**

```bash
echo "🔍 Starting comprehensive infrastructure validation..."

# Check cluster info
echo "📊 Cluster Information:"
kubectl cluster-info

# Check node status
echo "🖥️ Node Status:"
kubectl get nodes -o wide

# Verify all system pods are running
echo "⚙️ System Pods Status:"
kubectl get pods -n kube-system

# Count running vs total system pods
SYSTEM_PODS_TOTAL=$(kubectl get pods -n kube-system --no-headers | wc -l)
SYSTEM_PODS_RUNNING=$(kubectl get pods -n kube-system --no-headers | grep " Running " | wc -l)

echo "System Pods: $SYSTEM_PODS_RUNNING/$SYSTEM_PODS_TOTAL running"

if [[ $SYSTEM_PODS_RUNNING -eq $SYSTEM_PODS_TOTAL ]]; then
  echo "✅ All system pods are running"
else
  echo "⚠️ Some system pods are not running - this may be normal during startup"
  kubectl get pods -n kube-system | grep -v " Running "
fi
```

#### **Infrastructure Services Validation**

```bash
echo "🏗️ Infrastructure Services Validation:"

# Test ingress controller
echo "🌐 Testing Ingress Controller:"
INGRESS_PODS=$(kubectl get pods -n ingress-nginx --no-headers | grep " Running " | wc -l)
if [[ $INGRESS_PODS -gt 0 ]]; then
  echo "✅ Ingress controller is running"

  # Test HTTP access
  HTTP_RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:80)
  if [[ $HTTP_RESPONSE -eq 404 ]]; then
    echo "✅ Ingress HTTP access working (404 expected without apps)"
  else
    echo "⚠️ Ingress HTTP response: $HTTP_RESPONSE"
  fi
else
  echo "❌ Ingress controller not running"
fi

# Test metrics server
echo "📊 Testing Metrics Server:"
METRICS_PODS=$(kubectl get pods -n kube-system -l k8s-app=metrics-server --no-headers | grep " Running " | wc -l)
if [[ $METRICS_PODS -gt 0 ]]; then
  echo "✅ Metrics server is running"

  # Test metrics API (may not be immediately available)
  kubectl top nodes >/dev/null 2>&1 && echo "✅ Metrics API is working" || echo "⚠️ Metrics API not ready yet (normal for new cluster)"
else
  echo "❌ Metrics server not running"
fi

# Test storage provisioner
echo "💾 Testing Storage Provisioner:"
STORAGE_PODS=$(kubectl get pods -n local-path-storage --no-headers | grep " Running " | wc -l)
if [[ $STORAGE_PODS -gt 0 ]]; then
  echo "✅ Storage provisioner is running"

  # Check storage class
  STORAGE_CLASS=$(kubectl get storageclass --no-headers | grep "(default)" | wc -l)
  if [[ $STORAGE_CLASS -gt 0 ]]; then
    echo "✅ Default storage class is configured"
  else
    echo "❌ No default storage class found"
  fi
else
  echo "❌ Storage provisioner not running"
fi
```

#### **Network Connectivity Testing**

```bash
echo "🔗 Network Connectivity Testing:"

# Test DNS resolution
echo "🌐 Testing DNS Resolution:"
kubectl run dns-test --image=busybox:1.35 --rm -i --restart=Never \
  --command -- nslookup kubernetes.default.svc.cluster.local >/dev/null 2>&1 && \
  echo "✅ DNS resolution working" || echo "❌ DNS resolution failed"

# Test ingress accessibility
echo "🌐 Testing Ingress Accessibility:"
curl -s --connect-timeout 5 http://localhost:80 >/dev/null && \
  echo "✅ Ingress is accessible" || echo "❌ Ingress not accessible"

# Test port mappings
echo "🔌 Testing Port Mappings:"
EXPECTED_PORTS=(80 443 30081 30082 30083 30084 30085 30086 30092 15672)

for port in "${EXPECTED_PORTS[@]}"; do
  nc -z localhost $port 2>/dev/null && echo "✅ Port $port: Available" || echo "⚠️ Port $port: Not responding (normal until services deployed)"
done
```

#### **Storage Functionality Testing**

```bash
echo "💾 Storage Functionality Testing:"

# Create test PVC and pod
kubectl apply -f - <<EOF
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: storage-test-pvc
  namespace: default
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: standard
---
apiVersion: v1
kind: Pod
metadata:
  name: storage-test-pod
  namespace: default
spec:
  containers:
  - name: test-container
    image: busybox:1.35
    command: ['sh', '-c', 'echo "Storage test successful" > /data/test.txt && sleep 30']
    volumeMounts:
    - name: test-volume
      mountPath: /data
  volumes:
  - name: test-volume
    persistentVolumeClaim:
      claimName: storage-test-pvc
  restartPolicy: Never
EOF

# Wait for pod to complete
echo "⏳ Testing storage functionality..."
kubectl wait --for=condition=Ready pod/storage-test-pod --timeout=60s || echo "Pod may still be starting"

# Check if file was created
sleep 10
kubectl exec storage-test-pod -- cat /data/test.txt 2>/dev/null && \
  echo "✅ Storage functionality verified" || echo "⚠️ Storage test pending"

# Clean up test resources
kubectl delete pod storage-test-pod --force --grace-period=0 2>/dev/null
kubectl delete pvc storage-test-pvc 2>/dev/null

echo "🧹 Storage test cleanup completed"
```

---

## 🎉 **Foundation Setup Complete**

### **Infrastructure Summary**

**Your Kind cluster foundation now has:**

- ✅ **Kind Cluster**: Production-like Kubernetes cluster with port mappings
- ✅ **Ingress Controller**: NGINX ingress for external HTTP/HTTPS access
- ✅ **Metrics Server**: Resource monitoring and `kubectl top` functionality
- ✅ **Storage Provisioner**: Local path storage for persistent volumes
- ✅ **Network Policies**: Security framework for microservices communication
- ✅ **Namespace**: Dedicated `microservices` namespace with network policies

### **Validation Results**

After completing the foundation setup, you should see:

1. **✅ Cluster Status**: All nodes ready and system pods running
2. **✅ Ingress Access**: HTTP requests to localhost:80 returning 404 (expected)
3. **✅ Metrics Collection**: `kubectl top nodes` working (may take a few minutes)
4. **✅ Storage Ready**: Default storage class available for persistent volumes
5. **✅ Network Security**: Network policies configured for zero-trust architecture

### **Next Steps**

**Your foundation is now ready for microservices deployment!**

1. **Individual Services**: Deploy services in dependency order (Cache → Storage → Auth → Queue → Profile → Worker)
2. **Service Testing**: Validate each service individually before integration testing
3. **Integration Testing**: Run end-to-end tests across all services
4. **Performance Monitoring**: Use the observability framework for ongoing monitoring

### **Troubleshooting Foundation Issues**

If you encounter issues during foundation setup:

```bash
# Check cluster status
kubectl get nodes
kubectl get pods --all-namespaces

# Restart Kind cluster if needed
kind delete cluster --name microservices-kind
kind create cluster --config=k8s/cluster/kind-config.yaml --name=microservices-kind

# Verify port mappings
docker port microservices-kind-control-plane

# Check resource usage
docker stats microservices-kind-control-plane
```

---

**Foundation Status**: ✅ **CLUSTER AND INFRASTRUCTURE READY**  
**Ingress Controller**: 🌐 **NGINX INGRESS DEPLOYED AND ACCESSIBLE**  
**Metrics Server**: 📊 **RESOURCE MONITORING ENABLED**  
**Storage Provisioner**: 💾 **PERSISTENT STORAGE CONFIGURED**  
**Network Security**: 🔒 **ZERO-TRUST POLICIES ESTABLISHED**

**🚀 Ready to deploy microservices!** Proceed with the [Services Deployment Guide](SERVICES_DEPLOYMENT_GUIDE.md) to deploy individual services in dependency order.
