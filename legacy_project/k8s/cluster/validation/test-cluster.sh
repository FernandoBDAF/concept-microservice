#!/bin/bash

# Kind Cluster Functionality Test Script
# Verifies cluster readiness, network connectivity, and basic operations
# Educational testing with comprehensive validation

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

echo -e "${BLUE}=== Kind Cluster Functionality Tests ===${NC}"
echo "Testing cluster: $CLUSTER_NAME"
echo

# Test 1: Cluster connectivity
echo -e "${BLUE}1. Testing cluster connectivity...${NC}"
if kubectl cluster-info --context "kind-$CLUSTER_NAME" &>/dev/null; then
    test_result "Cluster connectivity" "PASS" "Can connect to cluster API server"
else
    test_result "Cluster connectivity" "FAIL" "Cannot connect to cluster API server"
fi

# Test 2: Node readiness
echo -e "${BLUE}2. Testing node readiness...${NC}"
nodes_ready=$(kubectl get nodes --no-headers | grep " Ready " | wc -l)
nodes_total=$(kubectl get nodes --no-headers | wc -l)

if [[ "$nodes_ready" -eq "$nodes_total" ]] && [[ "$nodes_total" -ge 3 ]]; then
    test_result "Node readiness" "PASS" "$nodes_ready/$nodes_total nodes ready"
else
    test_result "Node readiness" "FAIL" "Only $nodes_ready/$nodes_total nodes ready"
fi

# Test 3: Node roles
echo -e "${BLUE}3. Testing node roles...${NC}"
control_plane_nodes=$(kubectl get nodes --no-headers | grep "control-plane" | wc -l)
worker_nodes=$(kubectl get nodes --no-headers | grep -v "control-plane" | wc -l)

if [[ "$control_plane_nodes" -eq 1 ]] && [[ "$worker_nodes" -eq 2 ]]; then
    test_result "Node roles" "PASS" "1 control-plane, 2 worker nodes"
else
    test_result "Node roles" "FAIL" "$control_plane_nodes control-plane, $worker_nodes worker nodes"
fi

# Test 4: System pods
echo -e "${BLUE}4. Testing system pods...${NC}"
system_pods_running=$(kubectl get pods -n kube-system --no-headers | grep "Running" | wc -l)
system_pods_total=$(kubectl get pods -n kube-system --no-headers | wc -l)

if [[ "$system_pods_running" -eq "$system_pods_total" ]] && [[ "$system_pods_total" -gt 0 ]]; then
    test_result "System pods" "PASS" "$system_pods_running/$system_pods_total system pods running"
else
    test_result "System pods" "FAIL" "Only $system_pods_running/$system_pods_total system pods running"
fi

# Test 5: DNS resolution
echo -e "${BLUE}5. Testing DNS resolution...${NC}"
if kubectl run dns-test --image=busybox:1.35 --rm -i --restart=Never \
    --command -- nslookup kubernetes.default.svc.cluster.local &>/dev/null; then
    test_result "DNS resolution" "PASS" "DNS resolution working"
else
    test_result "DNS resolution" "FAIL" "DNS resolution not working"
fi

# Test 6: Pod-to-pod communication
echo -e "${BLUE}6. Testing pod-to-pod communication...${NC}"

# Create temporary network policy for test pods
cat <<EOF | kubectl apply -f - &>/dev/null || true
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-test-pods
spec:
  podSelector:
    matchLabels:
      test: cluster-validation
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - podSelector:
        matchLabels:
          test: cluster-validation
  egress:
  - to:
    - podSelector:
        matchLabels:
          test: cluster-validation
  - ports:
    - protocol: UDP
      port: 53
EOF

# Create test pods with test labels
kubectl run test-pod-1 --image=nginx:alpine --port=80 --restart=Never \
    --labels="test=cluster-validation" &>/dev/null || true
kubectl run test-pod-2 --image=busybox:1.35 --restart=Never \
    --labels="test=cluster-validation" \
    --command -- sleep 3600 &>/dev/null || true

# Wait for pods to be ready
kubectl wait --for=condition=Ready pod/test-pod-1 --timeout=60s &>/dev/null || true
kubectl wait --for=condition=Ready pod/test-pod-2 --timeout=60s &>/dev/null || true

# Get the IP of test-pod-1
pod1_ip=$(kubectl get pod test-pod-1 -o jsonpath='{.status.podIP}' 2>/dev/null || echo "")

# Test communication using pod IP
if [[ -n "$pod1_ip" ]] && kubectl exec test-pod-2 -- wget -qO- "http://$pod1_ip" &>/dev/null; then
    test_result "Pod-to-pod communication" "PASS" "Pods can communicate"
else
    test_result "Pod-to-pod communication" "FAIL" "Pod communication failed"
fi

# Cleanup test pods (keep network policy for service discovery test)
kubectl delete pod test-pod-1 test-pod-2 &>/dev/null || true

# Test 7: Service discovery
echo -e "${BLUE}7. Testing service discovery...${NC}"

# Ensure network policy exists for this test
kubectl get networkpolicy allow-test-pods &>/dev/null || \
cat <<EOF | kubectl apply -f - &>/dev/null || true
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-test-pods
spec:
  podSelector:
    matchLabels:
      test: cluster-validation
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - podSelector:
        matchLabels:
          test: cluster-validation
  egress:
  - to:
    - podSelector:
        matchLabels:
          test: cluster-validation
  - ports:
    - protocol: UDP
      port: 53
EOF

# Create test service
kubectl create service clusterip test-service --tcp=80:80 &>/dev/null || true

# Wait a moment for service to be ready
sleep 2

# Test service discovery with timeout
if timeout 10s kubectl run test-client --image=busybox:1.35 --rm --restart=Never \
    --labels="test=cluster-validation" \
    --command -- nslookup test-service.default.svc.cluster.local &>/dev/null; then
    test_result "Service discovery" "PASS" "Service discovery working"
else
    test_result "Service discovery" "FAIL" "Service discovery not working"
fi

# Cleanup service and test network policy
kubectl delete service test-service &>/dev/null || true
kubectl delete networkpolicy allow-test-pods &>/dev/null || true

# Test 8: NodePort accessibility
echo -e "${BLUE}8. Testing NodePort accessibility...${NC}"
# This test checks if NodePorts are properly mapped
node_ports_mapped=0
for port in 30081 30082 30083 30084 30085 30086; do
    if netstat -tuln | grep ":$port " &>/dev/null; then
        node_ports_mapped=$((node_ports_mapped + 1))
    fi
done

if [[ "$node_ports_mapped" -eq 6 ]]; then
    test_result "NodePort accessibility" "PASS" "All 6 NodePorts (30081-30086) mapped"
elif [[ "$node_ports_mapped" -gt 0 ]]; then
    test_result "NodePort accessibility" "PASS" "$node_ports_mapped/6 NodePorts mapped (partial)"
else
    test_result "NodePort accessibility" "FAIL" "No NodePorts accessible (EXPECTED before microservices deployment)"
fi

# Test 9: Resource limits
echo -e "${BLUE}9. Testing resource limits...${NC}"
# Test if we can create pods with resource limits
kubectl run resource-test --image=busybox:1.35 --rm --restart=Never \
    --limits="cpu=100m,memory=128Mi" --requests="cpu=50m,memory=64Mi" \
    --command -- echo "Resource limits test" &>/dev/null

if [[ $? -eq 0 ]]; then
    test_result "Resource limits" "PASS" "Can create pods with resource limits"
else
    test_result "Resource limits" "FAIL" "Cannot create pods with resource limits"
fi

# Test 10: Persistent volume support
echo -e "${BLUE}10. Testing persistent volume support...${NC}"
# Check if storage class exists
if kubectl get storageclass local-storage &>/dev/null; then
    test_result "Persistent volume support" "PASS" "Storage class available"
else
    test_result "Persistent volume support" "FAIL" "No storage class found"
fi

# Summary
echo
echo -e "${BLUE}=== Test Summary ===${NC}"
echo "Total tests run: $TESTS_TOTAL"
echo -e "Tests passed: ${GREEN}$TESTS_PASSED${NC}"
echo -e "Tests failed: ${RED}$TESTS_FAILED${NC}"

if [[ "$TESTS_FAILED" -eq 0 ]]; then
    echo -e "${GREEN}✓ All tests passed! Cluster is ready for service deployment.${NC}"
    exit 0
else
    echo -e "${YELLOW}⚠ Some tests failed. Check the cluster configuration.${NC}"
    exit 1
fi 