#!/bin/bash

# Cache Service Testing Script for Kind Clusters
# Tests cache operations: GET, SET, DELETE
# Validates service functionality and integration with Redis backend

set -euo pipefail

# Configuration
SERVICE_NAME="cache-service"
NODEPORT="30081"
BASE_URL="http://localhost:$NODEPORT"
API_VERSION="v1"
API_BASE="$BASE_URL/api/$API_VERSION/cache"

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

# Logging functions
log() {
    echo -e "${BLUE}[$(date +'%H:%M:%S')]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

# Test result tracking
test_result() {
    local test_name="$1"
    local status="$2"
    local message="${3:-}"
    
    TESTS_TOTAL=$((TESTS_TOTAL + 1))
    
    if [[ "$status" == "PASS" ]]; then
        log_success "$test_name"
        [[ -n "$message" ]] && echo "  $message"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        log_error "$test_name"
        [[ -n "$message" ]] && echo "  $message"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
}

# Wait for service to be ready
wait_for_service() {
    log "Waiting for cache service to be ready..."
    local max_attempts=30
    local attempt=1
    
    while [[ $attempt -le $max_attempts ]]; do
        if curl -s "$BASE_URL/health" >/dev/null 2>&1; then
            log_success "Cache service is ready"
            return 0
        fi
        
        log "Attempt $attempt/$max_attempts - waiting for service..."
        sleep 2
        attempt=$((attempt + 1))
    done
    
    log_error "Cache service failed to become ready"
    return 1
}

# Test 1: Health Check
test_health_check() {
    log "Testing health check endpoint..."
    
    local response
    local status_code
    
    response=$(curl -s -w "\n%{http_code}" "$BASE_URL/health" 2>/dev/null)
    status_code=$(echo "$response" | tail -n1)
    
    if [[ "$status_code" == "200" ]]; then
        test_result "Health check" "PASS" "Service is healthy (HTTP $status_code)"
    else
        test_result "Health check" "FAIL" "Health check failed (HTTP $status_code)"
    fi
}

# Test 2: Ready Check
test_ready_check() {
    log "Testing readiness endpoint..."
    
    local response
    local status_code
    
    response=$(curl -s -w "\n%{http_code}" "$BASE_URL/ready" 2>/dev/null)
    status_code=$(echo "$response" | tail -n1)
    
    if [[ "$status_code" == "200" ]]; then
        test_result "Readiness check" "PASS" "Service is ready (HTTP $status_code)"
    else
        test_result "Readiness check" "FAIL" "Readiness check failed (HTTP $status_code)"
    fi
}

# Test 3: Cache SET Operation
test_cache_set() {
    log "Testing cache SET operation..."
    
    local test_key="test-key-$(date +%s)"
    local test_value="test-value-$(date +%s)"
    local ttl=300
    
    local response
    local status_code
    
    response=$(curl -s -w "\n%{http_code}" \
        -X POST "$API_BASE/$test_key" \
        -H "Content-Type: application/json" \
        -d "{\"value\": \"$test_value\", \"ttl\": $ttl}" 2>/dev/null)
    
    status_code=$(echo "$response" | tail -n1)
    
    if [[ "$status_code" == "200" ]] || [[ "$status_code" == "201" ]]; then
        test_result "Cache SET operation" "PASS" "Successfully set key: $test_key"
        # Store key for cleanup
        echo "$test_key" >> /tmp/cache_test_keys.txt
    else
        test_result "Cache SET operation" "FAIL" "Failed to set key (HTTP $status_code)"
    fi
}

# Test 4: Cache GET Operation
test_cache_get() {
    log "Testing cache GET operation..."
    
    # First set a value
    local test_key="get-test-key-$(date +%s)"
    local test_value="get-test-value-$(date +%s)"
    
    # Set the value (using POST, not PUT)
    curl -s -X POST "$API_BASE/$test_key" \
        -H "Content-Type: application/json" \
        -d "{\"value\": \"$test_value\", \"ttl\": 300}" >/dev/null 2>&1
    
    sleep 1  # Give it a moment to be stored
    
    # Now get the value
    local response
    local status_code
    
    response=$(curl -s -w "\n%{http_code}" \
        -X GET "$API_BASE/$test_key" 2>/dev/null)
    
    status_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | sed '$d')  # Remove last line (status code)
    
    if [[ "$status_code" == "200" ]]; then
        if echo "$body" | grep -q "$test_value"; then
            test_result "Cache GET operation" "PASS" "Successfully retrieved key: $test_key"
        else
            test_result "Cache GET operation" "FAIL" "Retrieved key but value mismatch"
        fi
    else
        test_result "Cache GET operation" "FAIL" "Failed to get key (HTTP $status_code)"
    fi
    
    # Store key for cleanup
    echo "$test_key" >> /tmp/cache_test_keys.txt
}

# Test 5: Cache DELETE Operation
test_cache_delete() {
    log "Testing cache DELETE operation..."
    
    # First set a value
    local test_key="delete-test-key-$(date +%s)"
    local test_value="delete-test-value-$(date +%s)"
    
    # Set the value (using POST, not PUT)
    curl -s -X POST "$API_BASE/$test_key" \
        -H "Content-Type: application/json" \
        -d "{\"value\": \"$test_value\", \"ttl\": 300}" >/dev/null 2>&1
    
    sleep 1
    
    # Delete the value
    local response
    local status_code
    
    response=$(curl -s -w "\n%{http_code}" \
        -X DELETE "$API_BASE/$test_key" 2>/dev/null)
    
    status_code=$(echo "$response" | tail -n1)
    
    if [[ "$status_code" == "200" ]] || [[ "$status_code" == "204" ]]; then
        # Verify the key is actually deleted
        sleep 1
        local get_response
        local get_status
        
        get_response=$(curl -s -w "\n%{http_code}" \
            -X GET "$API_BASE/$test_key" 2>/dev/null)
        get_status=$(echo "$get_response" | tail -n1)
        
        if [[ "$get_status" == "404" ]]; then
            test_result "Cache DELETE operation" "PASS" "Successfully deleted key: $test_key"
        else
            test_result "Cache DELETE operation" "FAIL" "Delete succeeded but key still exists"
        fi
    else
        test_result "Cache DELETE operation" "FAIL" "Failed to delete key (HTTP $status_code)"
    fi
}

# Test 6: Cache TTL Functionality
test_cache_ttl() {
    log "Testing cache TTL functionality..."
    
    local test_key="ttl-test-key-$(date +%s)"
    local test_value="ttl-test-value"
    local ttl=3  # 3 seconds for quick test
    
    # Set value with short TTL (using POST, not PUT)
    local response
    local status_code
    
    response=$(curl -s -w "\n%{http_code}" \
        -X POST "$API_BASE/$test_key" \
        -H "Content-Type: application/json" \
        -d "{\"value\": \"$test_value\", \"ttl\": $ttl}" 2>/dev/null)
    
    status_code=$(echo "$response" | tail -n1)
    
    if [[ "$status_code" == "200" ]] || [[ "$status_code" == "201" ]]; then
        # Wait for TTL to expire
        sleep $(($ttl + 1))
        
        # Try to get the expired key
        local get_response
        local get_status
        
        get_response=$(curl -s -w "\n%{http_code}" \
            -X GET "$API_BASE/$test_key" 2>/dev/null)
        get_status=$(echo "$get_response" | tail -n1)
        
        if [[ "$get_status" == "404" ]]; then
            test_result "Cache TTL functionality" "PASS" "Key expired after TTL as expected"
        else
            test_result "Cache TTL functionality" "FAIL" "TTL not implemented in cache service (known limitation - Redis TTL works directly)"
        fi
    else
        test_result "Cache TTL functionality" "FAIL" "Failed to set key with TTL"
    fi
}

# Test 7: Metrics Endpoint
test_metrics_endpoint() {
    log "Testing metrics endpoint..."
    
    # Metrics service is ClusterIP only, test internally with unique pod name
    local pod_name="debug-metrics-$(date +%s)"
    local metrics_test
    
    # Use unique pod name and proper labels to avoid network policy issues
    metrics_test=$(kubectl run "$pod_name" --image=busybox:1.35 --rm -i --restart=Never \
        --labels="app=cache-service" \
        --command -- timeout 10s wget -qO- http://cache-service-metrics:8081/metrics 2>/dev/null || echo "Metrics test completed")
    
    if echo "$metrics_test" | grep -q "cache_\|http_\|go_"; then
        test_result "Metrics endpoint" "PASS" "Metrics available with application metrics"
    elif echo "$metrics_test" | grep -q "# HELP"; then
        test_result "Metrics endpoint" "PASS" "Metrics endpoint accessible (Prometheus format)"
    else
        test_result "Metrics endpoint" "FAIL" "Metrics endpoint not accessible (network policy restriction)"
    fi
}

# Test 8: Redis Backend Integration
test_redis_integration() {
    log "Testing Redis backend integration..."
    
    # Check if Redis is accessible (indirectly through cache service)
    local test_key="redis-integration-test-$(date +%s)"
    local test_value="redis-integration-value"
    
    # Set a value
    local set_response
    local set_status
    
    set_response=$(curl -s -w "\n%{http_code}" \
        -X POST "$API_BASE/$test_key" \
        -H "Content-Type: application/json" \
        -d "{\"value\": \"$test_value\", \"ttl\": 300}" 2>/dev/null)
    
    set_status=$(echo "$set_response" | tail -n1)
    
    if [[ "$set_status" == "200" ]] || [[ "$set_status" == "201" ]]; then
        # Get the value back
        local get_response
        local get_status
        
        get_response=$(curl -s -w "\n%{http_code}" \
            -X GET "$API_BASE/$test_key" 2>/dev/null)
        get_status=$(echo "$get_response" | tail -n1)
        
        if [[ "$get_status" == "200" ]]; then
            test_result "Redis backend integration" "PASS" "Successfully stored and retrieved from Redis"
            # Store key for cleanup
            echo "$test_key" >> /tmp/cache_test_keys.txt
        else
            test_result "Redis backend integration" "FAIL" "Could not retrieve value from Redis"
        fi
    else
        test_result "Redis backend integration" "FAIL" "Could not store value to Redis"
    fi
}

# Cleanup function
cleanup_test_keys() {
    if [[ -f /tmp/cache_test_keys.txt ]]; then
        log "Cleaning up test keys..."
        while IFS= read -r key; do
            curl -s -X DELETE "$API_BASE/$key" >/dev/null 2>&1 || true
        done < /tmp/cache_test_keys.txt
        rm -f /tmp/cache_test_keys.txt
        log "Cleanup completed"
    fi
}

# Main test execution
main() {
    echo -e "${BLUE}=== Cache Service Testing for Kind Cluster ===${NC}"
    echo "Service: $SERVICE_NAME"
    echo "NodePort: $NODEPORT"
    echo "Base URL: $BASE_URL"
    echo

    # Initialize cleanup file
    rm -f /tmp/cache_test_keys.txt
    touch /tmp/cache_test_keys.txt
    
    # Trap cleanup on exit
    trap cleanup_test_keys EXIT
    
    # Wait for service to be ready
    if ! wait_for_service; then
        log_error "Service not ready, aborting tests"
        exit 1
    fi
    
    echo
    log "Starting cache service tests..."
    echo
    
    # Run all tests
    test_health_check
    test_ready_check
    test_cache_set
    test_cache_get
    test_cache_delete
    test_cache_ttl
    test_metrics_endpoint
    test_redis_integration
    
    # Summary
    echo
    echo -e "${BLUE}=== Test Summary ===${NC}"
    echo "Total tests run: $TESTS_TOTAL"
    echo -e "Tests passed: ${GREEN}$TESTS_PASSED${NC}"
    echo -e "Tests failed: ${RED}$TESTS_FAILED${NC}"
    
    if [[ "$TESTS_FAILED" -eq 0 ]]; then
        echo
        log_success "All cache service tests passed! ✓"
        echo
        echo -e "${GREEN}Cache service is working correctly and ready for integration.${NC}"
        exit 0
    else
        echo
        log_error "Some tests failed. Please check the service configuration."
        exit 1
    fi
}

# Handle script arguments
case "${1:-}" in
    "help"|"-h"|"--help")
        echo "Usage: $0 [help]"
        echo "Tests the cache service functionality including:"
        echo "  - Health and readiness checks"
        echo "  - Cache operations (GET, SET, DELETE)"
        echo "  - TTL functionality"
        echo "  - Metrics endpoint"
        echo "  - Redis backend integration"
        exit 0
        ;;
    *)
        main
        ;;
esac 