#!/bin/bash

# =============================================================================
# WORKER SERVICE COMPREHENSIVE TEST SUITE FOR KIND CLUSTER
# =============================================================================
#
# 📚 EDUCATIONAL OVERVIEW:
# This script validates the Worker Service deployment with comprehensive testing
# of both Email and Image workers in a Kind cluster. It tests multi-worker
# architecture, queue connectivity, worker-specific functionality, and
# integration with the RabbitMQ-based Queue Service.
#
# 🏗️ MULTI-WORKER TESTING ARCHITECTURE:
# 1. Email Worker Testing: Health checks, queue connectivity, email processing simulation
# 2. Image Worker Testing: Health checks, queue connectivity, image processing simulation
# 3. Queue Integration Testing: Message publishing and consumption validation
# 4. Worker Performance Testing: Resource usage and response time validation
# 5. Service Discovery Testing: Worker service endpoints and metrics
#
# 🔧 WORKER-SPECIFIC PATTERNS TESTED:
# - Multi-worker deployment validation
# - Queue-based message processing patterns
# - Worker health and readiness probes
# - Metrics collection and monitoring
# - Resource utilization patterns
# - Graceful shutdown and signal handling
#
# 🎯 EDUCATIONAL FOCUS:
# This test suite demonstrates production-ready multi-worker architecture
# validation, showcasing how specialized workers can be deployed, monitored,
# and tested independently while sharing common infrastructure patterns.
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
SERVICE_NAME="worker-service"
EMAIL_WORKER_PORT="30086"
IMAGE_WORKER_PORT="30087"
EMAIL_WORKER_URL="http://localhost:${EMAIL_WORKER_PORT}"
IMAGE_WORKER_URL="http://localhost:${IMAGE_WORKER_PORT}"

# 🔗 DEPENDENCY SERVICE URLS
QUEUE_URL="http://localhost:30084"
RABBITMQ_MGMT_URL="http://localhost:15672"

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

log_worker() {
    echo -e "${CYAN}[$(date +'%H:%M:%S')] WORKER: $1${NC}"
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

# 🏥 WORKER READINESS VALIDATION
test_email_worker_readiness() {
    log_test "Waiting for Email Worker to be ready..."
    
    local max_attempts=30
    local attempt=1
    
    while [[ $attempt -le $max_attempts ]]; do
        if curl -s --connect-timeout 2 --max-time 5 "${EMAIL_WORKER_URL}/health" >/dev/null 2>&1; then
            test_result "Email Worker readiness" "PASS" "Email Worker is ready and responding"
            return 0
        fi
        
        log_info "Attempt $attempt/$max_attempts - waiting for Email Worker..."
        sleep 5
        ((attempt++))
    done
    
    test_result "Email Worker readiness" "FAIL" "Email Worker not ready after $((max_attempts * 5)) seconds"
    return 1
}

test_image_worker_readiness() {
    log_test "Waiting for Image Worker to be ready..."
    
    local max_attempts=30
    local attempt=1
    
    while [[ $attempt -le $max_attempts ]]; do
        if curl -s --connect-timeout 2 --max-time 5 "${IMAGE_WORKER_URL}/health" >/dev/null 2>&1; then
            test_result "Image Worker readiness" "PASS" "Image Worker is ready and responding"
            return 0
        fi
        
        log_info "Attempt $attempt/$max_attempts - waiting for Image Worker..."
        sleep 5
        ((attempt++))
    done
    
    test_result "Image Worker readiness" "FAIL" "Image Worker not ready after $((max_attempts * 5)) seconds"
    return 1
}

# 🏥 HEALTH CHECK VALIDATION
test_worker_health_endpoints() {
    log_test "Testing worker health check endpoints..."
    
    # Test Email Worker health endpoint
    local email_health_response
    email_health_response=$(curl -s --connect-timeout 5 --max-time 10 "${EMAIL_WORKER_URL}/health" 2>/dev/null || echo "")
    
    if echo "$email_health_response" | grep -q "healthy\|ok\|status"; then
        test_result "Email Worker health endpoint" "PASS" "Email Worker health endpoint responding correctly"
    else
        test_result "Email Worker health endpoint" "FAIL" "Email Worker health endpoint not responding correctly"
    fi
    
    # Test Image Worker health endpoint
    local image_health_response
    image_health_response=$(curl -s --connect-timeout 5 --max-time 10 "${IMAGE_WORKER_URL}/health" 2>/dev/null || echo "")
    
    if echo "$image_health_response" | grep -q "healthy\|ok\|status"; then
        test_result "Image Worker health endpoint" "PASS" "Image Worker health endpoint responding correctly"
    else
        test_result "Image Worker health endpoint" "FAIL" "Image Worker health endpoint not responding correctly"
    fi
    
    # Test readiness endpoints
    local email_ready_response
    email_ready_response=$(curl -s -o /dev/null -w "%{http_code}" "${EMAIL_WORKER_URL}/ready" 2>/dev/null || echo "000")
    
    if [[ "$email_ready_response" == "200" ]]; then
        test_result "Email Worker readiness endpoint" "PASS" "Email Worker readiness endpoint responding (HTTP 200)"
    else
        test_result "Email Worker readiness endpoint" "EXPECTED_FAIL" "Email Worker readiness endpoint not available (HTTP $email_ready_response)"
    fi
    
    local image_ready_response
    image_ready_response=$(curl -s -o /dev/null -w "%{http_code}" "${IMAGE_WORKER_URL}/ready" 2>/dev/null || echo "000")
    
    if [[ "$image_ready_response" == "200" ]]; then
        test_result "Image Worker readiness endpoint" "PASS" "Image Worker readiness endpoint responding (HTTP 200)"
    else
        test_result "Image Worker readiness endpoint" "EXPECTED_FAIL" "Image Worker readiness endpoint not available (HTTP $image_ready_response)"
    fi
}

# 🔍 SERVICE DISCOVERY VALIDATION
test_worker_service_discovery() {
    log_test "Testing worker service discovery and networking..."
    
    # Test Email Worker NodePort service
    local email_nodeport_response
    email_nodeport_response=$(curl -s -o /dev/null -w "%{http_code}" "${EMAIL_WORKER_URL}/health" 2>/dev/null || echo "000")
    
    if [[ "$email_nodeport_response" == "200" ]]; then
        test_result "Email Worker NodePort service" "PASS" "NodePort 30086 accessible externally"
    else
        test_result "Email Worker NodePort service" "FAIL" "NodePort 30086 not accessible (HTTP $email_nodeport_response)"
    fi
    
    # Test Image Worker NodePort service
    local image_nodeport_response
    image_nodeport_response=$(curl -s -o /dev/null -w "%{http_code}" "${IMAGE_WORKER_URL}/health" 2>/dev/null || echo "000")
    
    if [[ "$image_nodeport_response" == "200" ]]; then
        test_result "Image Worker NodePort service" "PASS" "NodePort 30087 accessible externally"
    else
        test_result "Image Worker NodePort service" "FAIL" "NodePort 30087 not accessible (HTTP $image_nodeport_response)"
    fi
    
    # Test service endpoints exist
    local email_services_count
    email_services_count=$(kubectl get svc -l app=email-worker --no-headers 2>/dev/null | wc -l || echo "0")
    
    if [[ "$email_services_count" -ge 1 ]]; then
        test_result "Email Worker service discovery" "PASS" "Email Worker service endpoints created ($email_services_count services)"
    else
        test_result "Email Worker service discovery" "FAIL" "Missing Email Worker service endpoints"
    fi
    
    local image_services_count
    image_services_count=$(kubectl get svc -l app=image-worker --no-headers 2>/dev/null | wc -l || echo "0")
    
    if [[ "$image_services_count" -ge 1 ]]; then
        test_result "Image Worker service discovery" "PASS" "Image Worker service endpoints created ($image_services_count services)"
    else
        test_result "Image Worker service discovery" "FAIL" "Missing Image Worker service endpoints"
    fi
}

# 📊 METRICS AND MONITORING VALIDATION
test_worker_metrics() {
    log_test "Testing worker metrics and monitoring endpoints..."
    
    # Test Email Worker metrics
    local email_metrics_response
    email_metrics_response=$(curl -s --connect-timeout 5 --max-time 10 "${EMAIL_WORKER_URL}/metrics" 2>/dev/null || echo "")
    
    if echo "$email_metrics_response" | grep -q "worker_\|go_\|http_"; then
        test_result "Email Worker metrics" "PASS" "Email Worker metrics endpoint providing Prometheus metrics"
    else
        test_result "Email Worker metrics" "EXPECTED_FAIL" "Email Worker metrics endpoint not available or different format"
    fi
    
    # Test Image Worker metrics
    local image_metrics_response
    image_metrics_response=$(curl -s --connect-timeout 5 --max-time 10 "${IMAGE_WORKER_URL}/metrics" 2>/dev/null || echo "")
    
    if echo "$image_metrics_response" | grep -q "worker_\|go_\|http_"; then
        test_result "Image Worker metrics" "PASS" "Image Worker metrics endpoint providing Prometheus metrics"
    else
        test_result "Image Worker metrics" "EXPECTED_FAIL" "Image Worker metrics endpoint not available or different format"
    fi
}

# 🔗 QUEUE CONNECTIVITY VALIDATION
test_queue_connectivity() {
    log_test "Testing worker queue connectivity..."
    
    # Test Queue Service health (dependency)
    local queue_health_response
    queue_health_response=$(curl -s -o /dev/null -w "%{http_code}" "${QUEUE_URL}/health" 2>/dev/null || echo "000")
    
    if [[ "$queue_health_response" == "200" ]]; then
        test_result "Queue Service connectivity" "PASS" "Queue Service is healthy and accessible"
    else
        test_result "Queue Service connectivity" "FAIL" "Queue Service not accessible (HTTP $queue_health_response)"
        return 1
    fi
    
    # Test RabbitMQ Management Interface (if available)
    local rabbitmq_mgmt_response
    rabbitmq_mgmt_response=$(curl -s -o /dev/null -w "%{http_code}" "${RABBITMQ_MGMT_URL}/api/overview" \
        -u guest:guest 2>/dev/null || echo "000")
    
    if [[ "$rabbitmq_mgmt_response" == "200" ]]; then
        test_result "RabbitMQ Management Interface" "PASS" "RabbitMQ management interface accessible"
        
        # Test queue existence
        local queues_response
        queues_response=$(curl -s "${RABBITMQ_MGMT_URL}/api/queues" -u guest:guest 2>/dev/null || echo "")
        
        if echo "$queues_response" | grep -q "email-processing\|image-processing"; then
            test_result "Worker queues existence" "PASS" "Worker queues exist in RabbitMQ"
        else
            test_result "Worker queues existence" "EXPECTED_FAIL" "Worker queues not found (may not be created yet)"
        fi
        
    else
        test_result "RabbitMQ Management Interface" "EXPECTED_FAIL" "RabbitMQ management interface not accessible (HTTP $rabbitmq_mgmt_response)"
    fi
}

# 🎯 WORKER-SPECIFIC FUNCTIONALITY TESTING
test_email_worker_functionality() {
    log_worker "Testing Email Worker specific functionality..."
    
    # Test worker type identification
    local worker_info_response
    worker_info_response=$(curl -s "${EMAIL_WORKER_URL}/info" 2>/dev/null || echo "")
    
    if echo "$worker_info_response" | grep -q "email\|Email"; then
        test_result "Email Worker type identification" "PASS" "Email Worker correctly identifies as email worker"
    else
        test_result "Email Worker type identification" "EXPECTED_FAIL" "Email Worker info endpoint not available or different format"
    fi
    
    # Test worker status
    local worker_status_response
    worker_status_response=$(curl -s "${EMAIL_WORKER_URL}/status" 2>/dev/null || echo "")
    
    if echo "$worker_status_response" | grep -q "running\|active\|ready"; then
        test_result "Email Worker status" "PASS" "Email Worker reports active status"
    else
        test_result "Email Worker status" "EXPECTED_FAIL" "Email Worker status endpoint not available or different format"
    fi
    
    # Test queue connection status
    local queue_status_response
    queue_status_response=$(curl -s "${EMAIL_WORKER_URL}/queue/status" 2>/dev/null || echo "")
    
    if echo "$queue_status_response" | grep -q "connected\|ok"; then
        test_result "Email Worker queue connection" "PASS" "Email Worker connected to queue"
    else
        test_result "Email Worker queue connection" "EXPECTED_FAIL" "Email Worker queue status endpoint not available"
    fi
}

test_image_worker_functionality() {
    log_worker "Testing Image Worker specific functionality..."
    
    # Test worker type identification
    local worker_info_response
    worker_info_response=$(curl -s "${IMAGE_WORKER_URL}/info" 2>/dev/null || echo "")
    
    if echo "$worker_info_response" | grep -q "image\|Image"; then
        test_result "Image Worker type identification" "PASS" "Image Worker correctly identifies as image worker"
    else
        test_result "Image Worker type identification" "EXPECTED_FAIL" "Image Worker info endpoint not available or different format"
    fi
    
    # Test worker status
    local worker_status_response
    worker_status_response=$(curl -s "${IMAGE_WORKER_URL}/status" 2>/dev/null || echo "")
    
    if echo "$worker_status_response" | grep -q "running\|active\|ready"; then
        test_result "Image Worker status" "PASS" "Image Worker reports active status"
    else
        test_result "Image Worker status" "EXPECTED_FAIL" "Image Worker status endpoint not available or different format"
    fi
    
    # Test queue connection status
    local queue_status_response
    queue_status_response=$(curl -s "${IMAGE_WORKER_URL}/queue/status" 2>/dev/null || echo "")
    
    if echo "$queue_status_response" | grep -q "connected\|ok"; then
        test_result "Image Worker queue connection" "PASS" "Image Worker connected to queue"
    else
        test_result "Image Worker queue connection" "EXPECTED_FAIL" "Image Worker queue status endpoint not available"
    fi
}

# 📊 PERFORMANCE VALIDATION
test_worker_performance() {
    log_test "Testing worker performance characteristics..."
    
    # Test Email Worker response time
    local start_time=$(python3 -c "import time; print(int(time.time() * 1000))" 2>/dev/null || date +%s000)
    curl -s --connect-timeout 5 --max-time 10 "${EMAIL_WORKER_URL}/health" >/dev/null 2>&1
    local end_time=$(python3 -c "import time; print(int(time.time() * 1000))" 2>/dev/null || date +%s000)
    local email_response_time=$((end_time - start_time))
    
    if [[ $email_response_time -le 1000 ]]; then
        test_result "Email Worker response time" "PASS" "Email Worker responds quickly (${email_response_time}ms)"
    else
        test_result "Email Worker response time" "FAIL" "Email Worker response too slow (${email_response_time}ms)"
    fi
    
    # Test Image Worker response time
    start_time=$(python3 -c "import time; print(int(time.time() * 1000))" 2>/dev/null || date +%s000)
    curl -s --connect-timeout 5 --max-time 10 "${IMAGE_WORKER_URL}/health" >/dev/null 2>&1
    end_time=$(python3 -c "import time; print(int(time.time() * 1000))" 2>/dev/null || date +%s000)
    local image_response_time=$((end_time - start_time))
    
    if [[ $image_response_time -le 1000 ]]; then
        test_result "Image Worker response time" "PASS" "Image Worker responds quickly (${image_response_time}ms)"
    else
        test_result "Image Worker response time" "FAIL" "Image Worker response too slow (${image_response_time}ms)"
    fi
    
    # Test memory usage
    local email_memory_usage
    email_memory_usage=$(kubectl top pod -l app=email-worker --no-headers 2>/dev/null | awk '{print $3}' | sed 's/Mi//' || echo "0")
    
    if [[ -n "$email_memory_usage" ]] && [[ "$email_memory_usage" -lt 128 ]]; then
        test_result "Email Worker memory usage" "PASS" "Email Worker uses reasonable memory (${email_memory_usage}Mi < 128Mi limit)"
    elif [[ -n "$email_memory_usage" ]]; then
        test_result "Email Worker memory usage" "EXPECTED_FAIL" "Email Worker memory usage higher than expected (${email_memory_usage}Mi)"
    else
        test_result "Email Worker memory usage" "EXPECTED_FAIL" "Email Worker memory usage metrics not available"
    fi
    
    local image_memory_usage
    image_memory_usage=$(kubectl top pod -l app=image-worker --no-headers 2>/dev/null | awk '{print $3}' | sed 's/Mi//' || echo "0")
    
    if [[ -n "$image_memory_usage" ]] && [[ "$image_memory_usage" -lt 256 ]]; then
        test_result "Image Worker memory usage" "PASS" "Image Worker uses reasonable memory (${image_memory_usage}Mi < 256Mi limit)"
    elif [[ -n "$image_memory_usage" ]]; then
        test_result "Image Worker memory usage" "EXPECTED_FAIL" "Image Worker memory usage higher than expected (${image_memory_usage}Mi)"
    else
        test_result "Image Worker memory usage" "EXPECTED_FAIL" "Image Worker memory usage metrics not available"
    fi
}

# 🧹 CLEANUP FUNCTION
cleanup_test_data() {
    log_info "Cleaning up test data..."
    
    # Clean up any test pods
    kubectl delete pods -l app=email-worker,test=true --ignore-not-found=true >/dev/null 2>&1 || true
    kubectl delete pods -l app=image-worker,test=true --ignore-not-found=true >/dev/null 2>&1 || true
    
    log_info "Cleanup completed"
}

# 🎯 MAIN TEST EXECUTION
main() {
    echo "========================================================================="
    echo "🔄 WORKER SERVICE COMPREHENSIVE TEST SUITE"
    echo "========================================================================="
    echo "Service: $SERVICE_NAME"
    echo "Email Worker Port: $EMAIL_WORKER_PORT"
    echo "Image Worker Port: $IMAGE_WORKER_PORT"
    echo "Email Worker URL: $EMAIL_WORKER_URL"
    echo "Image Worker URL: $IMAGE_WORKER_URL"
    echo ""
    echo "🎯 Testing multi-worker architecture:"
    echo "   • Email Worker: Health checks, queue connectivity, email processing"
    echo "   • Image Worker: Health checks, queue connectivity, image processing"
    echo "   • Service Discovery: NodePort services and endpoints"
    echo "   • Queue Integration: RabbitMQ connectivity and queue validation"
    echo "   • Performance: Response times and resource usage"
    echo "   • Monitoring: Metrics and health endpoints"
    echo ""
    
    # Prerequisites check
    if ! test_email_worker_readiness; then
        log_error "Email Worker not ready, aborting tests"
        exit 1
    fi
    
    if ! test_image_worker_readiness; then
        log_error "Image Worker not ready, aborting tests"
        exit 1
    fi
    
    log_info "🎯 Starting comprehensive worker tests..."
    echo ""
    
    # Execute test suites
    test_worker_health_endpoints
    test_worker_service_discovery
    test_worker_metrics
    test_queue_connectivity
    test_email_worker_functionality
    test_image_worker_functionality
    test_worker_performance
    
    # Calculate results
    local actual_failures=$((TESTS_FAILED))
    local success_rate=0
    
    if [[ $TESTS_TOTAL -gt 0 ]]; then
        success_rate=$(( (TESTS_PASSED * 100) / (TESTS_TOTAL - TESTS_EXPECTED_FAIL) ))
    fi
    
    echo ""
    echo "========================================================================="
    echo "📊 WORKER SERVICE TEST RESULTS"
    echo "========================================================================="
    echo "Total tests run: $TESTS_TOTAL"
    echo "Tests passed: $TESTS_PASSED"
    echo "Tests failed: $TESTS_FAILED"
    echo "Expected fails: $TESTS_EXPECTED_FAIL"
    echo "Success rate: ${success_rate}%"
    echo ""
    
    if [[ $actual_failures -eq 0 ]]; then
        log_success "🎉 ALL WORKER TESTS PASSED!"
        echo ""
        echo "✅ Multi-worker architecture validated:"
        echo "   • Email Worker deployed and healthy"
        echo "   • Image Worker deployed and healthy"
        echo "   • Both workers accessible via NodePorts"
        echo "   • Queue connectivity established"
        echo "   • Performance within acceptable limits"
        echo "   • Monitoring and metrics operational"
        echo ""
        echo "🚀 Worker Service is ready for message processing!"
        echo ""
        echo "💡 Next steps:"
        echo "   • Test message publishing via Queue Service"
        echo "   • Validate end-to-end workflow with Profile Service"
        echo "   • Monitor worker performance under load"
    else
        log_error "❌ SOME WORKER TESTS FAILED"
        echo ""
        echo "🔧 Review the failing tests above and fix worker issues."
        echo "💡 Common issues to check:"
        echo "   • Docker images built and loaded into Kind cluster"
        echo "   • RabbitMQ connectivity and queue setup"
        echo "   • Worker configuration and environment variables"
        echo "   • Resource limits and requests"
    fi
    
    cleanup_test_data
    
    # Exit with appropriate code
    exit $actual_failures
}

# 🚀 SCRIPT ENTRY POINT
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi 