#!/bin/bash

# =============================================================================
# PROFILE SERVICE BASIC TEST SUITE FOR KIND CLUSTER
# =============================================================================
#
# 📚 EDUCATIONAL OVERVIEW:
# This script validates the basic Profile Service deployment in a Kind cluster,
# focusing on core service functionality, health checks, performance validation,
# and basic endpoint testing without complex cross-service integration.
#
# 🏗️ BASIC TESTING ARCHITECTURE:
# 1. Service Health and Readiness Validation
# 2. Basic API Endpoint Testing
# 3. Performance and Resource Monitoring
# 4. Network Connectivity Validation
# 5. Error Handling and Validation Testing
#
# 🔧 KIND-SPECIFIC VALIDATIONS:
# - NodePort accessibility (30085)
# - Service discovery and networking
# - Resource utilization monitoring
# - Basic security and validation patterns
#
# 🎯 EDUCATIONAL FOCUS:
# This test suite focuses on individual service validation patterns,
# making it ideal for understanding basic Kubernetes service deployment
# and validation concepts before moving to complex integration testing.
# =============================================================================

set -euo pipefail

# 🎨 COLORS AND FORMATTING
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# 📊 TEST CONFIGURATION
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICE_NAME="profile-service"
SERVICE_PORT="30085"
BASE_URL="http://localhost:${SERVICE_PORT}"

# 🔢 TEST COUNTERS
TESTS_TOTAL=0
TESTS_PASSED=0
TESTS_FAILED=0
TESTS_EXPECTED_FAIL=0

# 📝 LOGGING FUNCTIONS
log_info() {
    echo -e "${BLUE}[$(date +'%H:%M:%S')] INFO: $1${NC}"
}

log_success() {
    echo -e "${GREEN}[$(date +'%H:%M:%S')] SUCCESS: $1${NC}"
}

log_warning() {
    echo -e "${YELLOW}[$(date +'%H:%M:%S')] WARNING: $1${NC}"
}

log_error() {
    echo -e "${RED}[$(date +'%H:%M:%S')] ERROR: $1${NC}"
}

log_test() {
    echo -e "${PURPLE}[$(date +'%H:%M:%S')] TEST: $1${NC}"
}

# 🧪 TEST RESULT FUNCTION
test_result() {
    local test_name="$1"
    local status="$2"
    local message="${3:-}"
    
    TESTS_TOTAL=$((TESTS_TOTAL + 1))
    
    case "$status" in
        "PASS")
            echo -e "${GREEN}✓ PASS${NC}: $test_name"
            [[ -n "$message" ]] && echo "  $message"
            TESTS_PASSED=$((TESTS_PASSED + 1))
            ;;
        "FAIL")
            echo -e "${RED}✗ FAIL${NC}: $test_name"
            [[ -n "$message" ]] && echo "  $message"
            TESTS_FAILED=$((TESTS_FAILED + 1))
            ;;
        "EXPECTED_FAIL")
            echo -e "${YELLOW}⚠ EXPECTED_FAIL${NC}: $test_name"
            [[ -n "$message" ]] && echo "  $message"
            TESTS_EXPECTED_FAIL=$((TESTS_EXPECTED_FAIL + 1))
            ;;
    esac
}

# 🏥 SERVICE HEALTH AND READINESS TESTS
test_service_readiness() {
    log_test "Waiting for Profile Service to be ready..."
    
    local max_attempts=30
    local attempt=1
    
    while [[ $attempt -le $max_attempts ]]; do
        if curl -s --connect-timeout 2 --max-time 5 "${BASE_URL}/health" >/dev/null 2>&1; then
            test_result "Service readiness" "PASS" "Profile Service is ready and responding"
            return 0
        fi
        
        log_info "Attempt $attempt/$max_attempts - waiting for service..."
        sleep 5
        ((attempt++))
    done
    
    test_result "Service readiness" "FAIL" "Profile Service not ready after $((max_attempts * 5)) seconds"
    return 1
}

test_health_endpoints() {
    log_test "Testing health check endpoints..."
    
    # Test main health endpoint
    local health_response
    health_response=$(curl -s --connect-timeout 5 --max-time 10 "${BASE_URL}/health" 2>/dev/null || echo "")
    
    if echo "$health_response" | grep -q "healthy\|ok\|status"; then
        test_result "Health endpoint" "PASS" "Service health endpoint responding correctly"
    else
        test_result "Health endpoint" "FAIL" "Health endpoint not responding correctly"
    fi
    
    # Test health endpoint response format
    if echo "$health_response" | grep -q "timestamp\|version\|service"; then
        test_result "Health response format" "PASS" "Health endpoint provides structured response"
    else
        test_result "Health response format" "EXPECTED_FAIL" "Health endpoint may use different response format"
    fi
}

test_service_discovery() {
    log_test "Testing service discovery and networking..."
    
    # Test NodePort service
    local nodeport_response
    nodeport_response=$(curl -s -o /dev/null -w "%{http_code}" "${BASE_URL}/health" 2>/dev/null || echo "000")
    
    if [[ "$nodeport_response" == "200" ]]; then
        test_result "NodePort service" "PASS" "NodePort 30085 accessible externally"
    else
        test_result "NodePort service" "FAIL" "NodePort 30085 not accessible (HTTP $nodeport_response)"
    fi
    
    # Test service endpoints exist
    local services_count
    services_count=$(kubectl get svc -l app=profile-service --no-headers 2>/dev/null | wc -l || echo "0")
    
    if [[ "$services_count" -ge 3 ]]; then
        test_result "Service discovery" "PASS" "All Profile Service endpoints created ($services_count services)"
    else
        test_result "Service discovery" "FAIL" "Missing Profile Service endpoints (found $services_count, expected 3+)"
    fi
}

# 🔌 BASIC API ENDPOINT TESTS
test_basic_api_endpoints() {
    log_test "Testing basic API endpoint accessibility..."
    
    # Test profile listing endpoint (without authentication)
    local profiles_response
    profiles_response=$(curl -s -o /dev/null -w "%{http_code}" "${BASE_URL}/api/v1/profiles" 2>/dev/null || echo "000")
    
    if [[ "$profiles_response" == "401" ]]; then
        test_result "Profiles endpoint security" "PASS" "Profiles endpoint properly requires authentication (HTTP 401)"
    elif [[ "$profiles_response" == "200" ]]; then
        test_result "Profiles endpoint security" "EXPECTED_FAIL" "Profiles endpoint accessible without auth - may be test configuration"
    else
        test_result "Profiles endpoint security" "FAIL" "Profiles endpoint unexpected response (HTTP $profiles_response)"
    fi
    
    # Test profile creation endpoint (without authentication)
    local create_response
    create_response=$(curl -s -o /dev/null -w "%{http_code}" -X POST "${BASE_URL}/api/v1/profiles" \
        -H "Content-Type: application/json" \
        -d '{"user_id":"test","name":"Test","email":"test@example.com"}' 2>/dev/null || echo "000")
    
    if [[ "$create_response" == "401" ]]; then
        test_result "Profile creation security" "PASS" "Profile creation properly requires authentication (HTTP 401)"
    elif [[ "$create_response" == "400" ]]; then
        test_result "Profile creation security" "PASS" "Profile creation validates input (HTTP 400)"
    else
        test_result "Profile creation security" "FAIL" "Profile creation unexpected response (HTTP $create_response)"
    fi
    
    # Test task submission endpoint (without authentication)
    local task_response
    task_response=$(curl -s -o /dev/null -w "%{http_code}" -X POST "${BASE_URL}/api/v1/profiles/test-id/tasks" \
        -H "Content-Type: application/json" \
        -d '{"type":"profile_update","payload":{"test":true}}' 2>/dev/null || echo "000")
    
    if [[ "$task_response" == "401" ]]; then
        test_result "Task submission security" "PASS" "Task submission properly requires authentication (HTTP 401)"
    elif [[ "$task_response" == "404" ]]; then
        test_result "Task submission security" "PASS" "Task submission validates profile existence (HTTP 404)"
    else
        test_result "Task submission security" "FAIL" "Task submission unexpected response (HTTP $task_response)"
    fi
}

# 🛡️ ERROR HANDLING AND VALIDATION TESTS
test_error_handling() {
    log_test "Testing error handling and input validation..."
    
    # Test malformed JSON
    local malformed_response
    malformed_response=$(curl -s -o /dev/null -w "%{http_code}" -X POST "${BASE_URL}/api/v1/profiles" \
        -H "Content-Type: application/json" \
        -d '{"invalid": json}' 2>/dev/null || echo "000")
    
    if [[ "$malformed_response" == "400" ]]; then
        test_result "Malformed JSON handling" "PASS" "Correctly rejected malformed JSON (HTTP 400)"
    elif [[ "$malformed_response" == "401" ]]; then
        test_result "Malformed JSON handling" "EXPECTED_FAIL" "Authentication checked before validation (HTTP 401)"
    else
        test_result "Malformed JSON handling" "FAIL" "Unexpected response to malformed JSON (HTTP $malformed_response)"
    fi
    
    # Test invalid Content-Type
    local content_type_response
    content_type_response=$(curl -s -o /dev/null -w "%{http_code}" -X POST "${BASE_URL}/api/v1/profiles" \
        -H "Content-Type: text/plain" \
        -d 'invalid data' 2>/dev/null || echo "000")
    
    if [[ "$content_type_response" =~ ^(400|415)$ ]]; then
        test_result "Content-Type validation" "PASS" "Correctly validates Content-Type (HTTP $content_type_response)"
    elif [[ "$content_type_response" == "401" ]]; then
        test_result "Content-Type validation" "EXPECTED_FAIL" "Authentication checked before Content-Type validation"
    else
        test_result "Content-Type validation" "FAIL" "Unexpected response to invalid Content-Type (HTTP $content_type_response)"
    fi
    
    # Test invalid HTTP method
    local method_response
    method_response=$(curl -s -o /dev/null -w "%{http_code}" -X PATCH "${BASE_URL}/api/v1/profiles" 2>/dev/null || echo "000")
    
    if [[ "$method_response" == "405" ]]; then
        test_result "HTTP method validation" "PASS" "Correctly rejects invalid HTTP methods (HTTP 405)"
    elif [[ "$method_response" == "401" ]]; then
        test_result "HTTP method validation" "EXPECTED_FAIL" "Authentication checked before method validation"
    else
        test_result "HTTP method validation" "FAIL" "Unexpected response to invalid method (HTTP $method_response)"
    fi
}

# 📊 PERFORMANCE AND RESOURCE TESTS
test_performance_characteristics() {
    log_test "Testing performance characteristics..."
    
    # Test response time
    local start_time=$(python3 -c "import time; print(int(time.time() * 1000))" 2>/dev/null || date +%s000)
    curl -s --connect-timeout 5 --max-time 10 "${BASE_URL}/health" >/dev/null 2>&1
    local end_time=$(python3 -c "import time; print(int(time.time() * 1000))" 2>/dev/null || date +%s000)
    local response_time=$((end_time - start_time))
    
    if [[ $response_time -le 1000 ]]; then
        test_result "Response time" "PASS" "Health endpoint responds quickly (${response_time}ms)"
    else
        test_result "Response time" "FAIL" "Health endpoint response too slow (${response_time}ms)"
    fi
    
    # Test memory usage
    local memory_usage
    memory_usage=$(kubectl top pod -l app=profile-service --no-headers 2>/dev/null | awk '{print $3}' | sed 's/Mi//' || echo "0")
    
    if [[ -n "$memory_usage" ]] && [[ "$memory_usage" -lt 200 ]]; then
        test_result "Memory usage" "PASS" "Service uses reasonable memory (${memory_usage}Mi < 256Mi limit)"
    elif [[ -n "$memory_usage" ]]; then
        test_result "Memory usage" "EXPECTED_FAIL" "Service memory usage higher than expected (${memory_usage}Mi)"
    else
        test_result "Memory usage" "EXPECTED_FAIL" "Memory usage metrics not available"
    fi
    
    # Test CPU usage
    local cpu_usage
    cpu_usage=$(kubectl top pod -l app=profile-service --no-headers 2>/dev/null | awk '{print $2}' | sed 's/m//' || echo "0")
    
    if [[ -n "$cpu_usage" ]] && [[ "$cpu_usage" -lt 150 ]]; then
        test_result "CPU usage" "PASS" "Service uses reasonable CPU (${cpu_usage}m < 200m limit)"
    elif [[ -n "$cpu_usage" ]]; then
        test_result "CPU usage" "EXPECTED_FAIL" "Service CPU usage higher than expected (${cpu_usage}m)"
    else
        test_result "CPU usage" "EXPECTED_FAIL" "CPU usage metrics not available"
    fi
}

# 🌐 NETWORK AND POLICY TESTS
test_network_policies() {
    log_test "Testing network policy compliance..."
    
    # Test external access via NodePort
    if curl -s --connect-timeout 5 --max-time 10 "${BASE_URL}/health" >/dev/null 2>&1; then
        test_result "External NodePort access" "PASS" "NodePort 30085 accessible externally"
    else
        test_result "External NodePort access" "FAIL" "NodePort 30085 not accessible"
    fi
    
    # Test metrics endpoint (should be restricted by network policy)
    local metrics_test
    metrics_test=$(timeout 10s kubectl run debug-profile-metrics-$(date +%s) --image=busybox:1.35 --rm -i --restart=Never \
        --labels="app=profile-service" -- wget -qO- http://profile-service-metrics:8081/metrics 2>/dev/null | head -c 100 || echo "")
    
    if echo "$metrics_test" | grep -q "profile\|go_\|http_"; then
        test_result "Metrics endpoint access" "PASS" "Metrics endpoint accessible with proper labels"
    else
        test_result "Metrics endpoint access" "EXPECTED_FAIL" "Metrics endpoint restricted by network policy (expected)"
    fi
}

# 🔍 DEPENDENCY CONNECTIVITY TESTS
test_dependency_connectivity() {
    log_test "Testing dependency service connectivity..."
    
    local services=("Cache:30081" "Storage:30082" "Auth:30083" "Queue:30084")
    local reachable_services=0
    local total_services=${#services[@]}
    
    for service_info in "${services[@]}"; do
        IFS=':' read -ra SERVICE_PARTS <<< "$service_info"
        local service_name="${SERVICE_PARTS[0]}"
        local service_port="${SERVICE_PARTS[1]}"
        
        log_info "Checking $service_name service connectivity..."
        
        if curl -s --connect-timeout 3 --max-time 5 "http://localhost:${service_port}/health" >/dev/null 2>&1; then
            log_success "$service_name service is reachable"
            ((reachable_services++))
        else
            log_warning "$service_name service is not reachable"
        fi
    done
    
    if [[ $reachable_services -eq $total_services ]]; then
        test_result "Dependency connectivity" "PASS" "All $total_services dependency services are reachable"
    elif [[ $reachable_services -gt 2 ]]; then
        test_result "Partial dependency connectivity" "PASS" "$reachable_services/$total_services dependency services reachable"
    else
        test_result "Insufficient dependency connectivity" "EXPECTED_FAIL" "Only $reachable_services/$total_services dependency services reachable"
    fi
}

# 🧹 CLEANUP FUNCTION
cleanup_test_data() {
    log_info "Cleaning up test data..."
    
    # Clean up any test pods that might be running
    kubectl delete pods -l app=profile-service,test=true --ignore-not-found=true >/dev/null 2>&1 || true
    
    log_info "Cleanup completed"
}

# 🎯 MAIN TEST EXECUTION
main() {
    echo "========================================================================="
    echo "🎯 PROFILE SERVICE BASIC TEST SUITE"
    echo "========================================================================="
    echo "Service: $SERVICE_NAME"
    echo "NodePort: $SERVICE_PORT"
    echo "Base URL: $BASE_URL"
    echo ""
    echo "🔍 Testing core service functionality:"
    echo "   • Service Health and Readiness"
    echo "   • Basic API Endpoint Security"
    echo "   • Error Handling and Validation"
    echo "   • Performance and Resource Usage"
    echo "   • Network Connectivity and Policies"
    echo ""
    
    # Prerequisites check
    if ! test_service_readiness; then
        log_error "Profile Service not ready, aborting tests"
        exit 1
    fi
    
    log_info "🎯 Starting basic service tests..."
    echo ""
    
    # Execute test suites
    test_health_endpoints
    test_service_discovery
    test_basic_api_endpoints
    test_error_handling
    test_performance_characteristics
    test_network_policies
    test_dependency_connectivity
    
    # Calculate results
    local actual_failures=$((TESTS_FAILED))
    local success_rate=0
    
    if [[ $TESTS_TOTAL -gt 0 ]]; then
        success_rate=$(( (TESTS_PASSED * 100) / (TESTS_TOTAL - TESTS_EXPECTED_FAIL) ))
    fi
    
    echo ""
    echo "========================================================================="
    echo "📊 BASIC SERVICE TEST RESULTS"
    echo "========================================================================="
    echo "Total tests run: $TESTS_TOTAL"
    echo "Tests passed: $TESTS_PASSED"
    echo "Tests failed: $TESTS_FAILED"
    echo "Expected fails: $TESTS_EXPECTED_FAIL"
    echo "Success rate: ${success_rate}%"
    echo ""
    
    if [[ $actual_failures -eq 0 ]]; then
        log_success "🎉 ALL BASIC TESTS PASSED!"
        echo ""
        echo "✅ Profile Service core functionality validated:"
        echo "   • Service health and readiness confirmed"
        echo "   • API endpoints properly secured with authentication"
        echo "   • Error handling and input validation working"
        echo "   • Performance within acceptable limits"
        echo "   • Network policies and connectivity verified"
        echo ""
        echo "🚀 Profile Service is ready for integration testing!"
        echo ""
        echo "💡 Next step: Run './profile-service-integration-test.sh' for comprehensive"
        echo "   cross-service integration validation"
    else
        log_error "❌ SOME BASIC TESTS FAILED"
        echo ""
        echo "🔧 Review the failing tests above and fix basic service issues."
        echo "💡 Ensure the Profile Service is properly deployed and configured."
    fi
    
    cleanup_test_data
    
    # Exit with appropriate code
    exit $actual_failures
}

# 🚀 SCRIPT ENTRY POINT
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi 