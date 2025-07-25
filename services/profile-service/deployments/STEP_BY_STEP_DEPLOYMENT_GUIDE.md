# Step-by-Step Kubernetes Deployment Guide

## Profile Service Multi-Worker Architecture

This guide walks you through deploying each Kubernetes manifest individually, helping you understand the impact of each component on your cluster.

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

- **Kind clusters**: Uses Kind-optimized settings (1 replica, reduced resources, local secrets)
- **Production clusters**: Uses full production settings (3 replicas, production resources)

### Option 2: Manual Commands (Educational)

Follow the detailed commands below to understand each step completely.

**⚠️ Note**: These manual commands use **production manifests** by default. For Kind development, consider using Option 1 or the Kustomize approach instead.

## 📋 Prerequisites

Ensure you have a kind cluster running and context set:

```bash
# Check if kind cluster exists
kind get clusters

# If not, create one (or use existing)
kind create cluster --name microservices

# Set context
kubectl config use-context kind-microservices

# Verify cluster access
kubectl cluster-info
kubectl get nodes
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

**What it does**: Creates sensitive configuration data (passwords, keys, tokens)

#### Deploy:

```bash
kubectl apply -f deployments/kubernetes/secrets.yaml
```

#### Observation Commands:

```bash
# 1. Check if secrets were created
kubectl get secrets
kubectl get secrets -l app=profile-service

# 2. Describe the secrets (shows metadata, not actual values)
kubectl describe secret profile-service-secrets
kubectl describe secret profile-service-secrets-local

# 3. Check secret data keys (still encoded)
kubectl get secret profile-service-secrets -o yaml

# 4. Decode a secret value (example - DON'T do this in production!)
kubectl get secret profile-service-secrets -o jsonpath='{.data.DB_USER}' | base64 --decode && echo

# 5. Watch for any events
kubectl get events --sort-by=.metadata.creationTimestamp --field-selector involvedObject.kind=Secret
```

**Expected Impact**: ✅ Two secrets created (`profile-service-secrets` and `profile-service-secrets-local`)

---

### Step 2: ⚙️ Deploy ConfigMaps (`configmap.yaml`)

**What it does**: Creates non-sensitive configuration data (routing keys, timeouts, etc.)

#### Deploy:

```bash
kubectl apply -f deployments/kubernetes/configmap.yaml
```

#### Observation Commands:

```bash
# 1. Check configmaps
kubectl get configmaps
kubectl get configmaps -l app=profile-service

# 2. Describe the configmaps to see their structure
kubectl describe configmap profile-service-config
kubectl describe configmap profile-service-routing-config

# 3. View the actual configuration data
kubectl get configmap profile-service-config -o yaml
kubectl get configmap profile-service-routing-config -o yaml

# 4. Check specific config values
kubectl get configmap profile-service-config -o jsonpath='{.data.ROUTING_KEY_PROFILE_UPDATE}' && echo
kubectl get configmap profile-service-config -o jsonpath='{.data.SUPPORTED_TASK_TYPES}' && echo

# 5. Watch events
kubectl get events --sort-by=.metadata.creationTimestamp --field-selector involvedObject.kind=ConfigMap
```

**Expected Impact**: ✅ Two ConfigMaps created with multi-worker routing configuration

---

### Step 3: 🔒 Deploy RBAC & Service (`service.yaml`)

**What it does**: Creates ServiceAccount, ClusterRole, Service, HPA, PDB, NetworkPolicy

#### Deploy:

```bash
kubectl apply -f deployments/kubernetes/service.yaml
```

#### Observation Commands:

```bash
# 1. Check all resources created by this manifest
kubectl get serviceaccounts,clusterroles,clusterrolebindings,services,hpa,pdb,networkpolicies -l app=profile-service

# 2. Focus on ServiceAccount and RBAC
kubectl describe serviceaccount profile-service
kubectl describe clusterrole profile-service-role
kubectl describe clusterrolebinding profile-service-binding

# 3. Examine the service
kubectl get services profile-service -o wide
kubectl describe service profile-service

# 4. Check HPA (will show no metrics initially)
kubectl get hpa profile-service-hpa
kubectl describe hpa profile-service-hpa

# 5. Check PodDisruptionBudget
kubectl get pdb profile-service-pdb
kubectl describe pdb profile-service-pdb

# 6. Check NetworkPolicy
kubectl get networkpolicy profile-service-netpol
kubectl describe networkpolicy profile-service-netpol

# 7. Check service endpoints (will be empty until deployment)
kubectl get endpoints profile-service

# 8. Watch events
kubectl get events --sort-by=.metadata.creationTimestamp --field-selector involvedObject.kind=Service
```

**Expected Impact**: ✅ Service, RBAC, scaling, and network policies created (but no pods yet)

---

### Step 3.5: 🗄️ Deploy Development Dependencies (Kind Only)

**What it does**: Creates temporary services needed for local development

⚠️ **For Kind/Local Development Only**: This step deploys temporary services that won't be needed in production.

#### Deploy:

```bash
# Only for local kind development
kubectl apply -f deployments/kind/redis-service.yaml
```

#### Observation Commands:

```bash
# 1. Check Redis service
kubectl get pods,services -l app=redis-service

# 2. Verify Redis is running
kubectl describe pod -l app=redis-service

# 3. Test Redis connectivity (optional)
kubectl run redis-test --rm -i --tty --image redis:7-alpine -- redis-cli -h redis-service -p 6379 -a local_redis_password ping
```

**Expected Impact**: ✅ Redis service running (temporary, for session management)

---

### Step 4: �� Deploy Application (Kind-Optimized)

**What it does**: Creates the Profile Service application pods optimized for Kind development

#### Deploy:

```bash
# Option A: Use Kustomize (Recommended for Kind)
kubectl apply -k deployments/kind/

# Option B: Apply only the deployment from kustomized output
kubectl kustomize deployments/kind/ | grep -A 200 "kind: Deployment" | kubectl apply -f -
```

**⚠️ Why Kind-Optimized?** This uses:

- **1 replica** (instead of 3) - suitable for single-node Kind
- **Reduced resources** - appropriate for local development
- **Local secrets** - uses `profile-service-secrets-local`
- **Debug logging** - easier troubleshooting

#### Critical Observation Commands:

```bash
# 1. Watch the deployment rollout in real-time
kubectl rollout status deployment/profile-service --timeout=300s

# 2. Check deployment status
kubectl get deployments profile-service
kubectl describe deployment profile-service

# 3. Check replica sets
kubectl get replicasets -l app=profile-service
kubectl describe replicaset -l app=profile-service

# 4. Check pods (the most important!)
kubectl get pods -l app=profile-service
kubectl get pods -l app=profile-service -o wide

# 5. Describe a specific pod to see what's wrong
kubectl describe pods -l app=profile-service

# 6. Check pod logs (this is where you'll see if the app is working)
kubectl logs -l app=profile-service --tail=50
kubectl logs -l app=profile-service -f  # Follow logs in real-time

# 7. Check if service endpoints are now populated
kubectl get endpoints profile-service
kubectl describe endpoints profile-service

# 8. Check resource usage (requires metrics-server)
kubectl top pods -l app=profile-service 2>/dev/null || echo "Metrics server not available"

# 9. Watch events (look for image pulls, pod creation, failures)
kubectl get events --sort-by=.metadata.creationTimestamp --field-selector involvedObject.kind=Pod

# 10. Check HPA now (should show current replicas)
kubectl get hpa profile-service-hpa
kubectl describe hpa profile-service-hpa

# 11. If pods are failing, check detailed pod status
kubectl get pods -l app=profile-service -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.phase}{"\t"}{.status.conditions[?(@.type=="Ready")].status}{"\n"}{end}'
```

**Expected Impact**: ✅ 3 pods created, running the Profile Service application

---

### Step 5: 📊 Deploy Monitoring (Kind-Optimized)

**What it does**: Sets up basic monitoring for Kind development

#### For Kind Development (Recommended):

```bash
# The kustomization we applied in Step 4 already includes monitoring
# But if you didn't use kustomize, apply the Kind-specific monitoring:
kubectl apply -f deployments/kind/monitoring-configmap.yaml
```

**✅ Kind Monitoring Includes**:

- Grafana dashboard ConfigMap (no Prometheus Operator required)
- Basic metrics configuration
- Development-friendly setup

#### For Production/Full Monitoring:

⚠️ **Requires Prometheus Operator**: Production monitoring needs Prometheus Operator CRDs.

```bash
# First, install Prometheus Operator (if not already installed)
kubectl apply -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/main/bundle.yaml
kubectl wait --for condition=established --timeout=300s crd/servicemonitors.monitoring.coreos.com

# Then apply full monitoring stack
kubectl apply -f deployments/monitoring/servicemonitor.yaml
```

#### Observation Commands:

```bash
# 1. Check if Prometheus Operator CRDs exist
kubectl get crd | grep monitoring.coreos.com

# 2. If CRDs exist, check ServiceMonitor
kubectl get servicemonitor profile-service-monitor 2>/dev/null || echo "ServiceMonitor CRD not available"

# 3. If CRDs exist, check PrometheusRule
kubectl get prometheusrule profile-service-alerts 2>/dev/null || echo "PrometheusRule CRD not available"

# 4. Check Grafana dashboard ConfigMap (should always work)
kubectl get configmap profile-service-grafana-dashboard
kubectl describe configmap profile-service-grafana-dashboard

# 5. Test metrics endpoint directly from a pod (if pods are running)
kubectl exec -it deployment/profile-service -- curl -s http://localhost:8080/metrics | head -20

# 6. Port-forward to access metrics from your local machine
kubectl port-forward service/profile-service 8080:8080 &
curl http://localhost:8080/metrics | head -10
pkill -f "kubectl port-forward"  # Clean up port-forward
```

#### Common Error and Solution:

```bash
# If you see this error:
# "no matches for kind ServiceMonitor in version monitoring.coreos.com/v1"
# "ensure CRDs are installed first"

# This means Prometheus Operator is not installed. Choose one:

# Solution A: Skip monitoring for local development
echo "Monitoring skipped - pods are running fine without it"

# Solution B: Install Prometheus Operator
kubectl apply -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/main/bundle.yaml
# Wait for CRDs, then retry the monitoring deployment
```

**Expected Impact**:

- ✅ **With Prometheus Operator**: ServiceMonitor, PrometheusRule, and ConfigMap created
- ✅ **Without Prometheus Operator**: ConfigMap created, ServiceMonitor/PrometheusRule skipped (this is fine for local dev)

---

## 🔍 Comprehensive Cluster State Commands

After deploying all manifests, use these commands to see the complete picture:

```bash
# 1. Overview of all resources
kubectl get all -l app=profile-service

# 2. Complete resource inventory
kubectl get secrets,configmaps,serviceaccounts,services,deployments,pods,hpa,pdb,networkpolicies -l app=profile-service

# 3. Pod health and readiness status
kubectl get pods -l app=profile-service -o wide

# 4. Service connectivity test
kubectl get services profile-service
kubectl get endpoints profile-service

# 5. Application logs (most important for debugging)
kubectl logs -l app=profile-service --tail=100 --all-containers=true

# 6. Resource usage (if metrics-server available)
kubectl top pods -l app=profile-service 2>/dev/null || echo "Metrics server not available"
kubectl top nodes

# 7. Check if the application is responding (requires port-forward)
kubectl port-forward service/profile-service 8080:8080 &
curl -f http://localhost:8080/health && echo "✅ Health check passed" || echo "❌ Health check failed"
curl -s http://localhost:8080/metrics | grep -c "profile_" && echo "✅ Metrics available" || echo "❌ No metrics found"
pkill -f "kubectl port-forward"  # Clean up

# 8. Complete cluster events (for troubleshooting)
kubectl get events --sort-by=.metadata.creationTimestamp --all-namespaces | tail -50

# 9. Describe everything for detailed troubleshooting
kubectl describe deployment profile-service
kubectl describe service profile-service
kubectl describe pods -l app=profile-service
```

---

## 🎯 What to Look For at Each Step

### Step 1 (Secrets):

- ✅ Two secrets created successfully
- ✅ No error events
- ✅ Secrets contain expected keys (DB_PASSWORD, JWT_SECRET_KEY, etc.)

### Step 2 (ConfigMaps):

- ✅ Two ConfigMaps with routing configuration
- ✅ Check that `SUPPORTED_TASK_TYPES` contains all three types
- ✅ Routing keys properly configured

### Step 3 (Service/RBAC):

- ✅ Service created with ClusterIP
- ✅ ServiceAccount has proper RBAC permissions
- ✅ HPA and PDB configured but no targets yet
- ✅ NetworkPolicy allows required traffic

### Step 4 (Deployment):

- ⚠️ **CRITICAL**: Pods transition: Pending → ContainerCreating → Running
- ⚠️ **CRITICAL**: 1 replica becomes Ready (1/1) - Kind uses single replica
- ✅ Service endpoints populated with pod IPs
- ✅ Application logs show successful startup
- ✅ Health endpoint responds
- ❌ **TROUBLESHOOT**: If pods stuck in Pending/CrashLoopBackOff

### Step 5 (Monitoring):

- ✅ ServiceMonitor created (if Prometheus Operator available)
- ✅ Metrics endpoint accessible
- ✅ Alert rules configured

---

## 🚨 Common Issues & Troubleshooting

### Architecture Mismatch (CRITICAL for Apple Silicon/ARM Macs)

**Issue**: Pods stuck in Pending with error:

```
0/4 nodes are available: 3 node(s) didn't match Pod's node affinity/selector
```

**Root Cause**: Deployment has `nodeSelector: kubernetes.io/arch=amd64` but your Mac uses ARM64.

**Solution**:

```bash
# Check your cluster architecture
kubectl get nodes --show-labels | grep kubernetes.io/arch

# If nodes show arm64, update deployment.yaml:
# Change: kubernetes.io/arch=amd64
# To:     kubernetes.io/arch=arm64
```

### Image Pull Issues

**Issue**: Pods in `ImagePullBackOff` state:

```
Failed to pull image "profile-service:latest": pull access denied, repository does not exist
```

**Root Cause**: Kubernetes tries to pull from Docker Hub instead of using locally loaded image.

**Solution**:

```bash
# 1. Ensure image is loaded into kind
kind load docker-image profile-service:latest --name microservices

# 2. Add to deployment.yaml:
spec:
  containers:
  - name: profile-service
    image: profile-service:latest
    imagePullPolicy: IfNotPresent  # This is the key fix
```

### Missing Dependencies and Secrets

**Issue**: Pods in `CreateContainerConfigError`:

```
Error: secret "redis-secret" not found
```

**Root Cause**: Deployment references secrets that don't exist.

**Solution**: Apply secrets first:

```bash
kubectl apply -f deployments/kubernetes/secrets.yaml
```

### Application Configuration Issues

**Issue**: Pods in `CrashLoopBackOff` with logs showing:

```
write error: can't open new logfile: open app.log: read-only file system
Failed to connect to Redis at localhost:6379
```

**Root Cause**: Wrong environment variable names or missing dependencies.

**Common Fixes**:

```bash
# 1. Check logs to identify the issue
kubectl logs -l app=profile-service --tail=20

# 2. Common environment variable mismatches:
# App expects: LOG_FILE (not LOG_FILE_PATH)
# App expects: REDIS_ADDR (not REDIS_HOST + REDIS_PORT)
# App expects: REDIS_PASSWORD for authentication

# 3. Missing Redis service - create temporary one:
kubectl apply -f deployments/kind/redis-service.yaml
```

### DNS Resolution Issues

**Issue**: Service-to-service communication fails:

```
dial tcp: lookup redis-service on 10.96.0.10:53: no such host
```

**Root Cause**: Service not yet created or DNS propagation delay.

**Solution**:

```bash
# 1. Verify service exists
kubectl get services redis-service

# 2. If service exists but still failing, restart deployment
kubectl rollout restart deployment/profile-service
```

### Monitoring/Prometheus Operator Issues

**Issue**: Monitoring deployment fails with:

```
no matches for kind "ServiceMonitor" in version "monitoring.coreos.com/v1"
ensure CRDs are installed first
```

**Root Cause**: Prometheus Operator CRDs not installed in kind cluster.

**Solutions**:

```bash
# Option A: Skip monitoring for local development (recommended)
echo "Skipping monitoring - use 'kubectl apply -k deployments/kind/' instead"

# Option B: Install Prometheus Operator
kubectl apply -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/main/bundle.yaml
kubectl wait --for condition=established --timeout=300s crd/servicemonitors.monitoring.coreos.com
```

### Pods Stuck in Pending State

```bash
# Check node resources
kubectl describe nodes
kubectl get events --field-selector reason=FailedScheduling

# Check if image is available
kubectl describe pods -l app=profile-service | grep -A5 -B5 "Failed"
```

### Pods in CrashLoopBackOff

```bash
# Check container logs
kubectl logs -l app=profile-service --previous --tail=100

# Check resource limits
kubectl describe pods -l app=profile-service | grep -A10 -B5 "Limits\|Requests"

# Check health probes
kubectl describe pods -l app=profile-service | grep -A10 -B5 "Liveness\|Readiness"
```

### Application Not Starting

```bash
# Check missing dependencies
kubectl logs -l app=profile-service | grep -i "error\|failed\|panic"

# Verify secrets and configmaps are mounted
kubectl describe pods -l app=profile-service | grep -A20 "Mounts:\|Environment"

# Check service dependencies
kubectl get services | grep -E "(queue-service|storage-service|auth-service|redis-service)"
```

### Service Not Accessible

```bash
# Verify service endpoints
kubectl get endpoints profile-service -o yaml

# Check service selector matches pod labels
kubectl get service profile-service -o yaml | grep -A5 selector
kubectl get pods -l app=profile-service --show-labels
```

### Environment Variable Debugging

**Issue**: Application not using correct configuration despite environment variables being set.

**Solution**: Debug environment variables inside the container:

```bash
# Check what environment variables are actually set
kubectl exec -it deployment/profile-service -- env | grep -E "(REDIS|LOG|CACHE)"

# Check specific variable
kubectl exec -it deployment/profile-service -- printenv REDIS_ADDR

# Common mistakes:
# ❌ LOG_FILE_PATH → ✅ LOG_FILE
# ❌ REDIS_HOST + REDIS_PORT → ✅ REDIS_ADDR
# Missing REDIS_PASSWORD for authenticated Redis
```

---

## 🧪 Quick Test Suite

After everything is deployed and running:

```bash
# 1. Basic connectivity test
kubectl port-forward service/profile-service 8080:8080 &
sleep 2

# 2. Health check
curl -f http://localhost:8080/health && echo "✅ Health OK" || echo "❌ Health Failed"

# 3. Test profile task submission
curl -X POST http://localhost:8080/api/v1/profiles/test-123/tasks \
  -H "Content-Type: application/json" \
  -d '{"type": "profile_update", "payload": {"action": "test"}}' \
  && echo "✅ Profile task OK" || echo "❌ Profile task Failed"

# 4. Test email task submission
curl -X POST http://localhost:8080/api/v1/profiles/test-123/tasks \
  -H "Content-Type: application/json" \
  -d '{"type": "email_notification", "payload": {"to": "test@example.com", "template": "welcome"}}' \
  && echo "✅ Email task OK" || echo "❌ Email task Failed"

# 5. Check metrics
curl -s http://localhost:8080/metrics | grep profile_ | wc -l | xargs echo "Profile metrics count:"

# Cleanup port-forward
pkill -f "kubectl port-forward"
```

---

## 📝 Notes

- **Image Availability**: Ensure `profile-service:latest` is loaded into your kind cluster using `kind load docker-image profile-service:latest --name <cluster-name>`
- **Dependencies**: The application expects external services (queue-service, etc.) - these may need to be mocked or deployed separately for local testing
- **Monitoring**: ServiceMonitor and PrometheusRule require Prometheus Operator to be installed in the cluster
- **Resource Requirements**: Adjust resource limits in `deployment.yaml` based on your local machine capabilities
- **Temporary Redis Service**:
  - A temporary Redis service (`deployments/kind/redis-service.yaml`) is deployed for local development
  - This provides session management functionality that will be replaced by the real cache-service in production
  - **Remove this file** when the production cache-service is available
  - Password is set to `local_redis_password` for development only
- **Architecture Compatibility**: The manifests are configured for ARM64 (Apple Silicon). For Intel Macs, change `nodeSelector` to `amd64`

---

Happy Kubernetes exploring! 🚀
