#!/bin/bash

# Validate Deployment Enhancements
# Date: December 29, 2024
# Purpose: Validate all critical deployment fixes have been properly applied

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Test counters
TESTS_TOTAL=0
TESTS_PASSED=0
TESTS_FAILED=0

echo -e "${BLUE}🔍 Validating Deployment Enhancements${NC}"
echo "======================================"

test_result() {
    local test_name="$1"
    local status="$2"
    local message="${3:-}"
    
    TESTS_TOTAL=$((TESTS_TOTAL + 1))
    
    if [[ "$status" == "PASS" ]]; then
        echo -e "${GREEN}✅ PASS${NC}: $test_name"
        [[ -n "$message" ]] && echo "  $message"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        echo -e "${RED}❌ FAIL${NC}: $test_name"
        [[ -n "$message" ]] && echo "  $message"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
}

# Test 1: Security Context Validation
echo -e "\n${BLUE}🔐 Testing Security Context Implementation${NC}"
echo "=========================================="

test_security_context() {
    local service_name="$1"
    local deployment_file="$2"
    
    if [ ! -f "$deployment_file" ]; then
        test_result "Security context in $service_name" "FAIL" "Deployment file not found: $deployment_file"
        return
    fi
    
    # Check for pod-level security context
    if grep -q "runAsNonRoot: true" "$deployment_file" && \
       grep -q "runAsUser: 65534" "$deployment_file" && \
       grep -q "fsGroup: 65534" "$deployment_file"; then
        
        # Check for container-level security context
        if grep -q "allowPrivilegeEscalation: false" "$deployment_file" && \
           grep -q "ALL" "$deployment_file"; then
            test_result "Security context in $service_name" "PASS" "Pod-level and container-level security contexts implemented"
        else
            test_result "Security context in $service_name" "FAIL" "Missing container-level security context"
        fi
    else
        test_result "Security context in $service_name" "FAIL" "Missing pod-level security context"
    fi
}

# Test all services for security contexts
test_security_context "cache-service" "k8s/deployment/01-cache-service/deployment.yaml"
test_security_context "storage-service" "k8s/deployment/02-storage-service/deployment.yaml"
test_security_context "auth-service" "k8s/deployment/03-auth-service/deployment.yaml"
test_security_context "queue-service" "k8s/deployment/04-queue-service/deployment.yaml"
test_security_context "profile-service" "k8s/deployment/05-profile-service/deployment.yaml"

# Test 2: Resource Standardization
echo -e "\n${BLUE}💾 Testing Resource Standardization${NC}"
echo "===================================="

test_resource_standardization() {
    local service_name="$1"
    local deployment_file="$2"
    
    if [ ! -f "$deployment_file" ]; then
        test_result "Resource standardization in $service_name" "FAIL" "Deployment file not found: $deployment_file"
        return
    fi
    
    # Check for standardized resources (128Mi/100m requests, 256Mi/200m limits)
    if grep -A4 "requests:" "$deployment_file" | grep -q "memory: \"128Mi\"" && \
       grep -A4 "requests:" "$deployment_file" | grep -q "cpu: \"100m\"" && \
       grep -A4 "limits:" "$deployment_file" | grep -q "memory: \"256Mi\"" && \
       grep -A4 "limits:" "$deployment_file" | grep -q "cpu: \"200m\""; then
        test_result "Resource standardization in $service_name" "PASS" "Standard Kind-optimized resources (128Mi/100m → 256Mi/200m)"
    else
        test_result "Resource standardization in $service_name" "FAIL" "Non-standard resource specifications"
    fi
}

# Test all services for resource standardization
test_resource_standardization "cache-service" "k8s/deployment/01-cache-service/deployment.yaml"
test_resource_standardization "storage-service" "k8s/deployment/02-storage-service/deployment.yaml"
test_resource_standardization "auth-service" "k8s/deployment/03-auth-service/deployment.yaml"
test_resource_standardization "queue-service" "k8s/deployment/04-queue-service/deployment.yaml"
test_resource_standardization "profile-service" "k8s/deployment/05-profile-service/deployment.yaml"

# Test 3: Docker Image Configuration
echo -e "\n${BLUE}🐳 Testing Docker Image Configuration${NC}"
echo "====================================="

test_image_configuration() {
    local service_name="$1"
    local deployment_file="$2"
    local expected_image="$3"
    
    if [ ! -f "$deployment_file" ]; then
        test_result "Image configuration in $service_name" "FAIL" "Deployment file not found: $deployment_file"
        return
    fi
    
    # Check for correct image name and imagePullPolicy
    if grep -q "image: $expected_image" "$deployment_file" && \
       grep -q "imagePullPolicy: IfNotPresent" "$deployment_file"; then
        test_result "Image configuration in $service_name" "PASS" "Correct image ($expected_image) and pull policy (IfNotPresent)"
    else
        test_result "Image configuration in $service_name" "FAIL" "Incorrect image or pull policy configuration"
    fi
}

# Test all services for image configuration
test_image_configuration "cache-service" "k8s/deployment/01-cache-service/deployment.yaml" "cache-service:latest"
test_image_configuration "storage-service" "k8s/deployment/02-storage-service/deployment.yaml" "storage-service:latest"
test_image_configuration "auth-service" "k8s/deployment/03-auth-service/deployment.yaml" "auth-service:latest"
test_image_configuration "queue-service" "k8s/deployment/04-queue-service/deployment.yaml" "queue-service:latest"
test_image_configuration "profile-service" "k8s/deployment/05-profile-service/deployment.yaml" "profile-service:latest"

# Test 4: Documentation Quality
echo -e "\n${BLUE}📝 Testing Documentation Quality${NC}"
echo "=================================="

test_documentation_quality() {
    local service_name="$1"
    local deployment_file="$2"
    
    if [ ! -f "$deployment_file" ]; then
        test_result "Documentation quality in $service_name" "FAIL" "Deployment file not found: $deployment_file"
        return
    fi
    
    # Check for comprehensive comments and documentation
    local comment_count=$(grep -c "^[[:space:]]*#" "$deployment_file" || echo "0")
    local educational_comments=$(grep -c "Kind\|Educational\|Production" "$deployment_file" || echo "0")
    
    if [ "$comment_count" -gt 20 ] && [ "$educational_comments" -gt 3 ]; then
        test_result "Documentation quality in $service_name" "PASS" "Comprehensive comments ($comment_count total, $educational_comments educational)"
    else
        test_result "Documentation quality in $service_name" "FAIL" "Insufficient documentation ($comment_count comments, $educational_comments educational)"
    fi
}

# Test all services for documentation quality
test_documentation_quality "cache-service" "k8s/deployment/01-cache-service/deployment.yaml"
test_documentation_quality "storage-service" "k8s/deployment/02-storage-service/deployment.yaml"
test_documentation_quality "auth-service" "k8s/deployment/03-auth-service/deployment.yaml"
test_documentation_quality "queue-service" "k8s/deployment/04-queue-service/deployment.yaml"
test_documentation_quality "profile-service" "k8s/deployment/05-profile-service/deployment.yaml"

# Test 5: Helper Scripts Validation
echo -e "\n${BLUE}🔧 Testing Helper Scripts${NC}"
echo "=========================="

# Test build script exists and is executable
if [ -f "k8s/build-all-images.sh" ] && [ -x "k8s/build-all-images.sh" ]; then
    test_result "Build script availability" "PASS" "build-all-images.sh exists and is executable"
else
    test_result "Build script availability" "FAIL" "build-all-images.sh missing or not executable"
fi

# Test load script exists and is executable
if [ -f "k8s/load-images-to-kind.sh" ] && [ -x "k8s/load-images-to-kind.sh" ]; then
    test_result "Load script availability" "PASS" "load-images-to-kind.sh exists and is executable"
else
    test_result "Load script availability" "FAIL" "load-images-to-kind.sh missing or not executable"
fi

# Test critical fixes documentation
if [ -f "k8s/CRITICAL_DEPLOYMENT_FIXES.md" ]; then
    test_result "Critical fixes documentation" "PASS" "CRITICAL_DEPLOYMENT_FIXES.md exists"
else
    test_result "Critical fixes documentation" "FAIL" "CRITICAL_DEPLOYMENT_FIXES.md missing"
fi

# Test 6: Missing Secrets Implementation Validation
echo -e "\n${BLUE}🔐 Testing Missing Secrets Implementation${NC}"
echo "=========================================="

test_secrets_implementation() {
    local service_name="$1"
    local secrets_file="$2"
    local deployment_file="$3"
    local secret_name="$4"
    
    # Test 1: Check if secrets file exists
    if [ ! -f "$secrets_file" ]; then
        test_result "Secrets file for $service_name" "FAIL" "Secrets file not found: $secrets_file"
        return
    fi
    
    # Test 2: Check if secrets file has proper structure
    if grep -q "apiVersion: v1" "$secrets_file" && \
       grep -q "kind: Secret" "$secrets_file" && \
       grep -q "name: $secret_name" "$secrets_file"; then
        test_result "Secrets structure for $service_name" "PASS" "Proper Kubernetes Secret structure"
    else
        test_result "Secrets structure for $service_name" "FAIL" "Invalid Secret structure"
        return
    fi
    
    # Test 3: Check if deployment references the secrets
    if [ -f "$deployment_file" ] && grep -q "$secret_name" "$deployment_file"; then
        test_result "Secrets integration for $service_name" "PASS" "Deployment references secrets correctly"
    else
        test_result "Secrets integration for $service_name" "FAIL" "Deployment does not reference secrets"
    fi
    
    # Test 4: Check for base64 encoded data
    local data_entries=$(grep -c ": [A-Za-z0-9+/]*=" "$secrets_file" || echo "0")
    if [ "$data_entries" -gt 0 ]; then
        test_result "Secrets encoding for $service_name" "PASS" "Found $data_entries base64 encoded entries"
    else
        test_result "Secrets encoding for $service_name" "FAIL" "No properly encoded secret data found"
    fi
}

# Test auth service secrets (CRITICAL - was missing)
test_secrets_implementation "auth-service" \
    "k8s/deployment/03-auth-service/secrets.yaml" \
    "k8s/deployment/03-auth-service/deployment.yaml" \
    "auth-service-secrets"

# Test profile service secrets (CRITICAL - was missing) 
test_secrets_implementation "profile-service" \
    "k8s/deployment/05-profile-service/secrets.yaml" \
    "k8s/deployment/05-profile-service/deployment.yaml" \
    "profile-service-secrets"

# Test 7: Network Policy Label Alignment (if deployed)
echo -e "\n${BLUE}🔒 Testing Network Policy Configuration${NC}"
echo "======================================="

if kubectl get networkpolicies &>/dev/null; then
    # Check for worker service network policies
    if kubectl get networkpolicy allow-email-worker &>/dev/null; then
        test_result "Email worker network policy" "PASS" "allow-email-worker policy exists"
    else
        test_result "Email worker network policy" "FAIL" "allow-email-worker policy missing"
    fi
    
    if kubectl get networkpolicy allow-image-worker &>/dev/null; then
        test_result "Image worker network policy" "PASS" "allow-image-worker policy exists"
    else
        test_result "Image worker network policy" "FAIL" "allow-image-worker policy missing"
    fi
else
    test_result "Network policies" "SKIP" "Cluster not accessible or no policies deployed"
fi

# Test 8: RabbitMQ Configuration Alignment
echo -e "\n${BLUE}🐰 Testing RabbitMQ Configuration${NC}"
echo "=================================="

test_rabbitmq_config() {
    local service_name="$1"
    local deployment_file="$2"
    
    if [ ! -f "$deployment_file" ]; then
        test_result "RabbitMQ config in $service_name" "SKIP" "Deployment file not found"
        return
    fi
    
    # Check for standardized RabbitMQ environment variables
    if grep -q "RABBITMQ_USERNAME" "$deployment_file" || \
       grep -q "RABBITMQ_NODES" "$deployment_file" || \
       grep -q "rabbitmq-service" "$deployment_file"; then
        test_result "RabbitMQ config in $service_name" "PASS" "RabbitMQ configuration present"
    else
        test_result "RabbitMQ config in $service_name" "SKIP" "Service doesn't use RabbitMQ"
    fi
}

# Test RabbitMQ configuration in relevant services
test_rabbitmq_config "queue-service" "k8s/deployment/04-queue-service/deployment.yaml"
test_rabbitmq_config "worker-service" "k8s/deployment/06-worker-service/deployment.yaml"

# Final Summary
echo -e "\n${BLUE}📊 Validation Summary${NC}"
echo "====================="
echo "Total tests run: ${TESTS_TOTAL}"
echo -e "${GREEN}Tests passed: ${TESTS_PASSED}${NC}"
echo -e "${RED}Tests failed: ${TESTS_FAILED}${NC}"

# Calculate success rate
if [ ${TESTS_TOTAL} -gt 0 ]; then
    SUCCESS_RATE=$(( (TESTS_PASSED * 100) / TESTS_TOTAL ))
    echo "Success rate: ${SUCCESS_RATE}%"
fi

if [ ${TESTS_FAILED} -eq 0 ]; then
    echo -e "\n${GREEN}🎉 All deployment enhancements validated successfully!${NC}"
    echo -e "${YELLOW}📋 Ready for production deployment with enhanced security and reliability${NC}"
    exit 0
else
    echo -e "\n${RED}❌ Some validation tests failed. Please review and fix the issues above.${NC}"
    exit 1
fi 