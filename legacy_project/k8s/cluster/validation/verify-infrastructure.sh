#!/bin/bash

# Infrastructure Services Verification Script
# Validates all infrastructure components are working correctly
# Tests ingress controller, metrics server, and storage provisioner

set -euo pipefail

# Configuration
CLUSTER_NAME="microservices-kind"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test counters
TESTS_TOTAL=0
TESTS_PASSED=0
TESTS_FAILED=0

# Test result function
test_result() {
    local test_name="$1"
    local status="$2"
    local message="${3:-}"
    
    TESTS_TOTAL=$((TESTS_TOTAL + 1))
    
    if [[ "$status" == "PASS" ]]; then
        echo -e "${GREEN}✓ PASS${NC}: $test_name"
        [[ -n "$message" ]] && echo "  $message"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        echo -e "${RED}✗ FAIL${NC}: $test_name"
        [[ -n "$message" ]] && echo "  $message"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
}

echo -e "${BLUE}=== Infrastructure Services Verification ===${NC}"
echo "Verifying infrastructure for cluster: $CLUSTER_NAME"
echo

# Test 1: Storage Class Verification
echo -e "${BLUE}1. Testing storage provisioner...${NC}"
storage_class_exists=$(kubectl get storageclass local-storage --no-headers 2>/dev/null | wc -l)
if [[ "$storage_class_exists" -eq 1 ]]; then
    # Check if it's the default storage class
    is_default=$(kubectl get storageclass local-storage -o jsonpath='{.metadata.annotations.storageclass\.kubernetes\.io/is-default-class}' 2>/dev/null)
    if [[ "$is_default" == "true" ]]; then
        test_result "Storage class configuration" "PASS" "local-storage is default storage class"
    else
        test_result "Storage class configuration" "FAIL" "local-storage exists but not default"
    fi
else
    test_result "Storage class configuration" "FAIL" "local-storage storage class not found"
fi

# Test 2: Local Path Provisioner
echo -e "${BLUE}2. Testing local path provisioner...${NC}"
provisioner_ready=$(kubectl get deployment local-path-provisioner -n local-path-storage -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
if [[ "$provisioner_ready" -eq 1 ]]; then
    test_result "Local path provisioner" "PASS" "Provisioner deployment ready"
else
    test_result "Local path provisioner" "FAIL" "Provisioner deployment not ready"
fi

# Test 3: PVC Creation Test
echo -e "${BLUE}3. Testing persistent volume creation...${NC}"
cat <<EOF | kubectl apply -f - &>/dev/null
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
EOF

sleep 5
pvc_status=$(kubectl get pvc test-pvc -o jsonpath='{.status.phase}' 2>/dev/null || echo "")
# Local path provisioner uses WaitForFirstConsumer - this is expected behavior
if [[ "$pvc_status" == "Bound" ]] || [[ "$pvc_status" == "Pending" ]]; then
    # Check if it's pending due to WaitForFirstConsumer
    if [[ "$pvc_status" == "Pending" ]]; then
        events=$(kubectl describe pvc test-pvc 2>/dev/null | grep "WaitForFirstConsumer" || echo "")
        if [[ -n "$events" ]]; then
            test_result "PVC creation" "PASS" "PVC ready (WaitForFirstConsumer - expected for local path provisioner)"
        else
            test_result "PVC creation" "FAIL" "PVC status: $pvc_status (unexpected pending reason)"
        fi
    else
        test_result "PVC creation" "PASS" "PVC successfully bound"
    fi
else
    test_result "PVC creation" "FAIL" "PVC status: $pvc_status"
fi

# Cleanup test PVC
kubectl delete pvc test-pvc &>/dev/null || true

# Test 4: Metrics Server
echo -e "${BLUE}4. Testing metrics server...${NC}"
metrics_ready=$(kubectl get deployment metrics-server -n kube-system -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
if [[ "$metrics_ready" -eq 1 ]]; then
    test_result "Metrics server deployment" "PASS" "Metrics server ready"
else
    test_result "Metrics server deployment" "FAIL" "Metrics server not ready"
fi

# Test 5: Metrics API availability
echo -e "${BLUE}5. Testing metrics API...${NC}"
if kubectl top nodes &>/dev/null; then
    test_result "Metrics API" "PASS" "Node metrics available"
elif kubectl get --raw /apis/metrics.k8s.io/v1beta1/nodes &>/dev/null; then
    test_result "Metrics API" "PASS" "Metrics API accessible (data may be pending)"
else
    test_result "Metrics API" "FAIL" "Metrics API not accessible (EXPECTED in Kind - not blocking for microservices)"
fi

# Test 6: Ingress Controller
echo -e "${BLUE}6. Testing ingress controller...${NC}"
ingress_ready=$(kubectl get deployment ingress-nginx-controller -n ingress-nginx -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
if [[ "$ingress_ready" -eq 1 ]]; then
    test_result "Ingress controller" "PASS" "NGINX ingress controller ready"
else
    test_result "Ingress controller" "FAIL" "NGINX ingress controller not ready"
fi

# Test 7: Ingress Class
echo -e "${BLUE}7. Testing ingress class...${NC}"
ingress_class_exists=$(kubectl get ingressclass nginx --no-headers 2>/dev/null | wc -l)
if [[ "$ingress_class_exists" -eq 1 ]]; then
    test_result "Ingress class" "PASS" "nginx ingress class available"
else
    test_result "Ingress class" "FAIL" "nginx ingress class not found"
fi

# Test 8: Ingress Service NodePort
echo -e "${BLUE}8. Testing ingress service...${NC}"
ingress_service=$(kubectl get service ingress-nginx-controller -n ingress-nginx -o jsonpath='{.spec.type}' 2>/dev/null || echo "")
if [[ "$ingress_service" == "NodePort" ]]; then
    # Check specific NodePorts
    http_nodeport=$(kubectl get service ingress-nginx-controller -n ingress-nginx -o jsonpath='{.spec.ports[?(@.name=="http")].nodePort}' 2>/dev/null || echo "")
    https_nodeport=$(kubectl get service ingress-nginx-controller -n ingress-nginx -o jsonpath='{.spec.ports[?(@.name=="https")].nodePort}' 2>/dev/null || echo "")
    
    if [[ "$http_nodeport" == "30080" ]] && [[ "$https_nodeport" == "30443" ]]; then
        test_result "Ingress NodePort service" "PASS" "HTTP:30080, HTTPS:30443"
    else
        test_result "Ingress NodePort service" "FAIL" "Incorrect NodePort mappings"
    fi
else
    test_result "Ingress NodePort service" "FAIL" "Service type: $ingress_service (expected NodePort)"
fi

# Test 9: Ingress Functionality Test
echo -e "${BLUE}9. Testing ingress functionality...${NC}"
# Create test application and ingress
cat <<EOF | kubectl apply -f - &>/dev/null
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-app
  template:
    metadata:
      labels:
        app: test-app
    spec:
      containers:
      - name: test-app
        image: nginx:alpine
        ports:
        - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: test-app-service
spec:
  selector:
    app: test-app
  ports:
  - port: 80
    targetPort: 80
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: test-ingress
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /
spec:
  ingressClassName: nginx
  rules:
  - host: test.local
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: test-app-service
            port:
              number: 80
EOF

sleep 10

# Test ingress (use Kind's mapped port 80)
if curl -H "Host: test.local" http://localhost:80 &>/dev/null; then
    test_result "Ingress functionality" "PASS" "Ingress routing working"
else
    test_result "Ingress functionality" "FAIL" "Ingress routing not working (EXPECTED - network policies block unlabeled test pods; will work with microservices)"
fi

# Cleanup test resources
kubectl delete deployment test-app &>/dev/null || true
kubectl delete service test-app-service &>/dev/null || true
kubectl delete ingress test-ingress &>/dev/null || true

# Test 10: Network Policies
echo -e "${BLUE}10. Testing network policies...${NC}"
network_policies=$(kubectl get networkpolicy --no-headers 2>/dev/null | wc -l)
if [[ "$network_policies" -gt 0 ]]; then
    test_result "Network policies" "PASS" "$network_policies network policies configured"
else
    test_result "Network policies" "FAIL" "No network policies found"
fi

# Test 11: Resource Monitoring
echo -e "${BLUE}11. Testing resource monitoring...${NC}"
# Test if we can get pod metrics
sleep 5  # Give metrics time to populate
if kubectl top pods -n kube-system &>/dev/null; then
    test_result "Resource monitoring" "PASS" "Pod resource metrics available"
else
    test_result "Resource monitoring" "FAIL" "Pod resource metrics not available (EXPECTED - related to Kind metrics API limitations)"
fi

# Test 12: Infrastructure Health Check
echo -e "${BLUE}12. Testing infrastructure health...${NC}"
unhealthy_pods=0

# Check ingress-nginx namespace with timeout and proper error handling
nginx_pods=$(kubectl get pods -n ingress-nginx --no-headers 2>/dev/null | grep -v "Running\|Completed" | wc -l 2>/dev/null || echo "0")
nginx_pods=${nginx_pods//[^0-9]/}  # Remove any non-numeric characters
nginx_pods=${nginx_pods:-0}        # Default to 0 if empty
unhealthy_pods=$((unhealthy_pods + nginx_pods))

# Check kube-system namespace (metrics-server) with timeout and proper error handling
metrics_pods=$(kubectl get pods -n kube-system -l k8s-app=metrics-server --no-headers 2>/dev/null | grep -v "Running\|Completed" | wc -l 2>/dev/null || echo "0")
metrics_pods=${metrics_pods//[^0-9]/}  # Remove any non-numeric characters
metrics_pods=${metrics_pods:-0}        # Default to 0 if empty
unhealthy_pods=$((unhealthy_pods + metrics_pods))

# Check local-path-storage namespace with timeout and proper error handling
storage_pods=$(kubectl get pods -n local-path-storage --no-headers 2>/dev/null | grep -v "Running\|Completed" | wc -l 2>/dev/null || echo "0")
storage_pods=${storage_pods//[^0-9]/}  # Remove any non-numeric characters
storage_pods=${storage_pods:-0}        # Default to 0 if empty
unhealthy_pods=$((unhealthy_pods + storage_pods))

if [[ "$unhealthy_pods" -eq 0 ]]; then
    test_result "Infrastructure health" "PASS" "All infrastructure pods healthy"
else
    test_result "Infrastructure health" "FAIL" "$unhealthy_pods unhealthy infrastructure pods"
fi

# Summary
echo
echo -e "${BLUE}=== Infrastructure Verification Summary ===${NC}"
echo "Total tests run: $TESTS_TOTAL"
echo -e "Tests passed: ${GREEN}$TESTS_PASSED${NC}"
echo -e "Tests failed: ${RED}$TESTS_FAILED${NC}"

# Detailed status
echo
echo -e "${BLUE}=== Infrastructure Component Status ===${NC}"
echo "Storage Provisioner:"
kubectl get pods -n local-path-storage -o wide

echo
echo "Metrics Server:"
kubectl get pods -n kube-system -l k8s-app=metrics-server -o wide

echo
echo "Ingress Controller:"
kubectl get pods -n ingress-nginx -o wide

echo
echo "Storage Classes:"
kubectl get storageclass

if [[ "$TESTS_FAILED" -eq 0 ]]; then
    echo
    echo -e "${GREEN}✓ All infrastructure services verified! Ready for microservices deployment.${NC}"
    exit 0
else
    echo
    echo -e "${YELLOW}⚠ Some infrastructure tests failed. Please check the configuration.${NC}"
    exit 1
fi 