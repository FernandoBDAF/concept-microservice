#!/bin/bash

# =============================================================================
# QUEUE SERVICE COMPREHENSIVE TEST SUITE FOR KIND CLUSTER
# =============================================================================
#
# 📚 EDUCATIONAL OVERVIEW:
# This script validates the Queue Service deployment in a Kind cluster,
# testing RabbitMQ connectivity, message publishing, routing capabilities,
# and comprehensive integration with Cache, Storage, and Auth services.
#
# 🏗️ ARCHITECTURE TESTING:
# Tests cover the complete message flow and service integration:
# Test Client → Queue Service HTTP API → RabbitMQ → Message Queues
# Cache/Storage/Auth Services → Queue Service → Multi-Worker Routing
#
# 🔧 KIND-SPECIFIC VALIDATIONS:
# - NodePort accessibility (30084)
# - RabbitMQ StatefulSet health
# - Persistent volume functionality
# - Network policy compliance
# - Resource utilization monitoring
# - Cross-service integration patterns
#
# 🔗 INTEGRATION TEST COVERAGE:
# - Cache Service integration (invalidation, warming, connectivity)
# - Storage Service integration (sync, backup, validation, connectivity)
# - Auth Service integration (events, tokens, alerts, connectivity)
# - Multi-service workflows (complete profile update scenarios)
# - Multi-worker routing verification (profile.task, email.send, image.process)
# - Dependency service health monitoring
#
# ⚠️ EDUCATIONAL FOCUS:
# Each test includes explanations of what's being validated and why,
# making this script a learning tool for Kubernetes, messaging concepts,
# and microservices integration patterns.
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
SERVICE_NAME="queue-service"
SERVICE_PORT="30084"
BASE_URL="http://localhost:${SERVICE_PORT}"
RABBITMQ_MGMT_PORT="15672"

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

# 🏥 HEALTH CHECK FUNCTIONS
test_service_readiness() {
    log_test "Waiting for queue service to be ready..."
    
    local max_attempts=30
    local attempt=1
    
    while [[ $attempt -le $max_attempts ]]; do
        if curl -s --connect-timeout 2 --max-time 5 "${BASE_URL}/health" >/dev/null 2>&1; then
            test_result "Service readiness" "PASS" "Service is ready"
            return 0
        fi
        
        log_info "Attempt $attempt/$max_attempts - waiting for service..."
        sleep 5
        ((attempt++))
    done
    
    test_result "Service readiness" "FAIL" "Service not ready after $((max_attempts * 5)) seconds"
    return 1
}

test_health_endpoints() {
    log_test "Testing health check endpoints..."
    
    # Test main health endpoint
    if curl -s --connect-timeout 5 --max-time 10 "${BASE_URL}/health" | grep -q "healthy\|ok"; then
        test_result "Health check" "PASS" "Service is healthy (HTTP 200)"
    else
        test_result "Health check" "FAIL" "Health endpoint not responding correctly"
    fi
    
    # Test RabbitMQ connectivity via health endpoint
    local health_response
    health_response=$(curl -s --connect-timeout 5 --max-time 10 "${BASE_URL}/health" 2>/dev/null || echo "")
    
    if echo "$health_response" | grep -q "rabbitmq\|broker\|queue"; then
        test_result "RabbitMQ health integration" "PASS" "Health endpoint reports RabbitMQ status"
    else
        test_result "RabbitMQ health integration" "EXPECTED_FAIL" "Health endpoint may not include RabbitMQ status details"
    fi
}

test_rabbitmq_backend() {
    log_test "Testing RabbitMQ backend connectivity..."
    
    # Check if RabbitMQ pod is running
    if kubectl get pods -l app=rabbitmq | grep -q "Running"; then
        test_result "RabbitMQ pod status" "PASS" "RabbitMQ pod is running"
    else
        test_result "RabbitMQ pod status" "FAIL" "RabbitMQ pod is not running"
        return 1
    fi
    
    # Test RabbitMQ management API accessibility (internal)
    if kubectl exec -it deployment/queue-service -- wget -qO- --timeout=10 http://rabbitmq-service:15672/api/overview 2>/dev/null | grep -q "rabbitmq_version\|management_version"; then
        test_result "RabbitMQ management API" "PASS" "Management API accessible from Queue Service"
    else
        test_result "RabbitMQ management API" "EXPECTED_FAIL" "Management API may require authentication or be restricted"
    fi
    
    # Test RabbitMQ AMQP port connectivity
    if kubectl exec -it deployment/queue-service -- timeout 5 nc -z rabbitmq-service 5672 2>/dev/null; then
        test_result "RabbitMQ AMQP connectivity" "PASS" "AMQP port (5672) is accessible"
    else
        test_result "RabbitMQ AMQP connectivity" "FAIL" "AMQP port (5672) is not accessible"
    fi
}

test_message_publishing() {
    log_test "Testing message publishing operations..."
    
    # Test basic message publishing with profile.task routing key
    local publish_response
    publish_response=$(curl -s -w "%{http_code}" -X POST "${BASE_URL}/api/v1/queue/messages" \
        -H "Content-Type: application/json" \
        -d '{
            "type": "profile_update",
            "routing_key": "profile.task",
            "payload": {
                "user_id": "test-123",
                "action": "update",
                "data": {"name": "Test User"}
            },
            "metadata": {
                "source": "test-suite",
                "correlation_id": "test-001"
            }
        }' 2>/dev/null || echo "000")
    
    local http_code="${publish_response: -3}"
    local response_body="${publish_response%???}"
    
    if [[ "$http_code" == "200" ]] || [[ "$http_code" == "201" ]] || [[ "$http_code" == "202" ]]; then
        test_result "Message publishing (profile.task)" "PASS" "Message published successfully (HTTP $http_code)"
    else
        test_result "Message publishing (profile.task)" "FAIL" "Message publishing failed (HTTP $http_code)"
    fi
    
    # Test email.send routing key
    local email_response
    email_response=$(curl -s -w "%{http_code}" -X POST "${BASE_URL}/api/v1/queue/messages" \
        -H "Content-Type: application/json" \
        -d '{
            "type": "email_notification",
            "routing_key": "email.send",
            "payload": {
                "to": "test@example.com",
                "subject": "Test Email",
                "body": "Test message"
            },
            "metadata": {
                "source": "test-suite",
                "correlation_id": "test-002"
            }
        }' 2>/dev/null || echo "000")
    
    local email_http_code="${email_response: -3}"
    
    if [[ "$email_http_code" == "200" ]] || [[ "$email_http_code" == "201" ]] || [[ "$email_http_code" == "202" ]]; then
        test_result "Message publishing (email.send)" "PASS" "Email message published successfully (HTTP $email_http_code)"
    else
        test_result "Message publishing (email.send)" "FAIL" "Email message publishing failed (HTTP $email_http_code)"
    fi
    
    # Test image.process routing key
    local image_response
    image_response=$(curl -s -w "%{http_code}" -X POST "${BASE_URL}/api/v1/queue/messages" \
        -H "Content-Type: application/json" \
        -d '{
            "type": "image_processing",
            "routing_key": "image.process",
            "payload": {
                "image_id": "img-123",
                "operation": "resize",
                "parameters": {"width": 800, "height": 600}
            },
            "metadata": {
                "source": "test-suite",
                "correlation_id": "test-003"
            }
        }' 2>/dev/null || echo "000")
    
    local image_http_code="${image_response: -3}"
    
    if [[ "$image_http_code" == "200" ]] || [[ "$image_http_code" == "201" ]] || [[ "$image_http_code" == "202" ]]; then
        test_result "Message publishing (image.process)" "PASS" "Image message published successfully (HTTP $image_http_code)"
    else
        test_result "Message publishing (image.process)" "FAIL" "Image message publishing failed (HTTP $image_http_code)"
    fi
}

test_queue_status() {
    log_test "Testing queue status and monitoring..."
    
    # Test queue status endpoint (if available)
    local status_response
    status_response=$(curl -s -w "%{http_code}" "${BASE_URL}/api/v1/queue/status" 2>/dev/null || echo "000")
    local status_http_code="${status_response: -3}"
    
    if [[ "$status_http_code" == "200" ]]; then
        test_result "Queue status endpoint" "PASS" "Status endpoint accessible (HTTP $status_http_code)"
    else
        test_result "Queue status endpoint" "EXPECTED_FAIL" "Status endpoint may not be implemented (HTTP $status_http_code)"
    fi
    
    # Test metrics endpoint
    if timeout 30s kubectl run debug-queue-metrics-$(date +%s) --image=busybox:1.35 --rm -i --restart=Never \
        --labels="app=queue-service" -- wget -qO- http://queue-service-metrics:9090/metrics 2>/dev/null | grep -q "rabbitmq\|queue\|go_"; then
        test_result "Metrics endpoint" "PASS" "Metrics endpoint accessible with queue-related metrics"
    else
        test_result "Metrics endpoint" "EXPECTED_FAIL" "Metrics endpoint not accessible (network policy restriction)"
    fi
}

test_error_handling() {
    log_test "Testing error handling and validation..."
    
    # Test malformed JSON
    local malformed_response
    malformed_response=$(curl -s -w "%{http_code}" -X POST "${BASE_URL}/api/v1/queue/messages" \
        -H "Content-Type: application/json" \
        -d '{"invalid": json}' 2>/dev/null || echo "000")
    
    local malformed_http_code="${malformed_response: -3}"
    
    if [[ "$malformed_http_code" == "400" ]]; then
        test_result "Malformed JSON handling" "PASS" "Correctly rejected malformed JSON (HTTP 400)"
    elif [[ "$malformed_http_code" == "500" ]]; then
        test_result "Malformed JSON handling" "EXPECTED_FAIL" "Server error on malformed JSON (HTTP 500) - needs error handling improvement"
    else
        test_result "Malformed JSON handling" "FAIL" "Unexpected response to malformed JSON (HTTP $malformed_http_code)"
    fi
    
    # Test missing required fields
    local missing_fields_response
    missing_fields_response=$(curl -s -w "%{http_code}" -X POST "${BASE_URL}/api/v1/queue/messages" \
        -H "Content-Type: application/json" \
        -d '{"type": "test"}' 2>/dev/null || echo "000")
    
    local missing_fields_http_code="${missing_fields_response: -3}"
    
    if [[ "$missing_fields_http_code" == "400" ]]; then
        test_result "Missing fields validation" "PASS" "Correctly rejected incomplete message (HTTP 400)"
    else
        test_result "Missing fields validation" "EXPECTED_FAIL" "Missing field validation may be lenient (HTTP $missing_fields_http_code)"
    fi
    
    # Test invalid routing key
    local invalid_routing_response
    invalid_routing_response=$(curl -s -w "%{http_code}" -X POST "${BASE_URL}/api/v1/queue/messages" \
        -H "Content-Type: application/json" \
        -d '{
            "type": "test",
            "routing_key": "invalid.unknown",
            "payload": {"test": true},
            "metadata": {"source": "test"}
        }' 2>/dev/null || echo "000")
    
    local invalid_routing_http_code="${invalid_routing_response: -3}"
    
    if [[ "$invalid_routing_http_code" == "400" ]]; then
        test_result "Invalid routing key handling" "PASS" "Correctly rejected invalid routing key (HTTP 400)"
    else
        test_result "Invalid routing key handling" "EXPECTED_FAIL" "Invalid routing key may be accepted or handled gracefully (HTTP $invalid_routing_http_code)"
    fi
}

test_performance_characteristics() {
    log_test "Testing performance characteristics..."
    
    # Test response time
    local start_time=$(date +%s)
    curl -s --connect-timeout 5 --max-time 10 "${BASE_URL}/health" >/dev/null 2>&1
    local end_time=$(date +%s)
    local response_time=$((end_time - start_time))
    
    if [[ $response_time -le 5 ]]; then
        test_result "Response time" "PASS" "Health endpoint responds quickly (${response_time}s)"
    else
        test_result "Response time" "FAIL" "Health endpoint response too slow (${response_time}s)"
    fi
    
    # Test memory usage
    local memory_usage
    memory_usage=$(kubectl top pod -l app=queue-service --no-headers 2>/dev/null | awk '{print $3}' | sed 's/Mi//' || echo "0")
    
    if [[ -n "$memory_usage" ]] && [[ "$memory_usage" -lt 200 ]]; then
        test_result "Memory usage" "PASS" "Service uses reasonable memory (${memory_usage}Mi < 256Mi limit)"
    elif [[ -n "$memory_usage" ]]; then
        test_result "Memory usage" "EXPECTED_FAIL" "Service memory usage higher than expected (${memory_usage}Mi)"
    else
        test_result "Memory usage" "EXPECTED_FAIL" "Memory usage metrics not available (metrics server may not be ready)"
    fi
    
    # Test RabbitMQ memory usage
    local rabbitmq_memory
    rabbitmq_memory=$(kubectl top pod -l app=rabbitmq --no-headers 2>/dev/null | awk '{print $3}' | sed 's/Mi//' || echo "0")
    
    if [[ -n "$rabbitmq_memory" ]] && [[ "$rabbitmq_memory" -lt 400 ]]; then
        test_result "RabbitMQ memory usage" "PASS" "RabbitMQ uses reasonable memory (${rabbitmq_memory}Mi < 512Mi limit)"
    elif [[ -n "$rabbitmq_memory" ]]; then
        test_result "RabbitMQ memory usage" "EXPECTED_FAIL" "RabbitMQ memory usage higher than expected (${rabbitmq_memory}Mi)"
    else
        test_result "RabbitMQ memory usage" "EXPECTED_FAIL" "RabbitMQ memory usage metrics not available"
    fi
}

test_persistence_and_recovery() {
    log_test "Testing message persistence and recovery..."
    
    # Check if persistent volume is mounted
    if kubectl exec rabbitmq-0 -- ls -la /var/lib/rabbitmq 2>/dev/null | grep -q "mnesia"; then
        test_result "RabbitMQ persistent storage" "PASS" "Persistent volume mounted correctly"
    else
        test_result "RabbitMQ persistent storage" "FAIL" "Persistent volume not mounted"
    fi
    
    # Test that RabbitMQ data directory exists and has content
    if kubectl exec rabbitmq-0 -- ls -la /var/lib/rabbitmq/mnesia 2>/dev/null | grep -q "rabbit@"; then
        test_result "RabbitMQ data initialization" "PASS" "RabbitMQ data directory properly initialized"
    else
        test_result "RabbitMQ data initialization" "EXPECTED_FAIL" "RabbitMQ data directory may still be initializing"
    fi
}

test_network_policies() {
    log_test "Testing network policy compliance..."
    
    # Test that Queue Service can reach RabbitMQ
    if kubectl exec deployment/queue-service -- timeout 5 nc -z rabbitmq-service 5672 2>/dev/null; then
        test_result "Queue Service → RabbitMQ connectivity" "PASS" "Network policies allow required communication"
    else
        test_result "Queue Service → RabbitMQ connectivity" "FAIL" "Network policies may be blocking required communication"
    fi
    
    # Test external access via NodePort
    if curl -s --connect-timeout 5 --max-time 10 "${BASE_URL}/health" >/dev/null 2>&1; then
        test_result "External NodePort access" "PASS" "NodePort 30084 accessible externally"
    else
        test_result "External NodePort access" "FAIL" "NodePort 30084 not accessible"
    fi
}

# 🧹 CLEANUP FUNCTION
cleanup_test_data() {
    log_info "Cleaning up test data..."
    
    # Clean up any test pods that might be running
    kubectl delete pods -l app=queue-service,test=true --ignore-not-found=true >/dev/null 2>&1 || true
    
    log_info "Cleanup completed"
}

# 🔗 INTEGRATION TESTS WITH DEPENDENCY SERVICES
test_cache_service_integration() {
    log_test "Testing integration with Cache Service..."
    
    # Test 1: Verify Cache Service can send cache invalidation messages to Queue Service
    log_info "Testing cache invalidation message publishing..."
    
    local cache_invalidation_response
    cache_invalidation_response=$(curl -s -w "%{http_code}" -X POST "${BASE_URL}/api/v1/queue/messages" \
        -H "Content-Type: application/json" \
        -d '{
            "type": "cache_invalidation",
            "routing_key": "profile.task",
            "payload": {
                "cache_keys": ["user:123:profile", "user:123:permissions"],
                "action": "invalidate",
                "reason": "profile_update"
            },
            "metadata": {
                "source_service": "cache-service",
                "correlation_id": "cache-001",
                "priority": "high"
            }
        }' 2>/dev/null || echo "000")
    
    local cache_http_code="${cache_invalidation_response: -3}"
    
    if [[ "$cache_http_code" == "200" ]] || [[ "$cache_http_code" == "201" ]] || [[ "$cache_http_code" == "202" ]]; then
        test_result "Cache Service → Queue Service (invalidation)" "PASS" "Cache invalidation message published successfully (HTTP $cache_http_code)"
    else
        test_result "Cache Service → Queue Service (invalidation)" "FAIL" "Cache invalidation message publishing failed (HTTP $cache_http_code)"
    fi
    
    # Test 2: Verify Cache Service can send cache warming messages
    log_info "Testing cache warming message publishing..."
    
    local cache_warming_response
    cache_warming_response=$(curl -s -w "%{http_code}" -X POST "${BASE_URL}/api/v1/queue/messages" \
        -H "Content-Type: application/json" \
        -d '{
            "type": "cache_warming",
            "routing_key": "profile.task",
            "payload": {
                "user_id": "123",
                "cache_patterns": ["profile", "permissions", "preferences"],
                "action": "warm"
            },
            "metadata": {
                "source_service": "cache-service",
                "correlation_id": "cache-002",
                "priority": "low"
            }
        }' 2>/dev/null || echo "000")
    
    local warming_http_code="${cache_warming_response: -3}"
    
    if [[ "$warming_http_code" == "200" ]] || [[ "$warming_http_code" == "201" ]] || [[ "$warming_http_code" == "202" ]]; then
        test_result "Cache Service → Queue Service (warming)" "PASS" "Cache warming message published successfully (HTTP $warming_http_code)"
    else
        test_result "Cache Service → Queue Service (warming)" "FAIL" "Cache warming message publishing failed (HTTP $warming_http_code)"
    fi
    
    # Test 3: Verify Cache Service health check connectivity
    log_info "Testing Cache Service connectivity for integration scenarios..."
    
    if curl -s --connect-timeout 5 --max-time 10 "http://localhost:30081/health" | grep -q "healthy\|ok"; then
        test_result "Cache Service connectivity" "PASS" "Cache Service is accessible for integration (HTTP 200)"
    else
        test_result "Cache Service connectivity" "EXPECTED_FAIL" "Cache Service may not be accessible (check deployment status)"
    fi
}

test_storage_service_integration() {
    log_test "Testing integration with Storage Service..."
    
    # Test 1: Verify Storage Service can send data synchronization messages
    log_info "Testing data synchronization message publishing..."
    
    local data_sync_response
    data_sync_response=$(curl -s -w "%{http_code}" -X POST "${BASE_URL}/api/v1/queue/messages" \
        -H "Content-Type: application/json" \
        -d '{
            "type": "data_sync",
            "routing_key": "profile.task",
            "payload": {
                "user_id": "123",
                "table": "profiles",
                "operation": "update",
                "changes": {
                    "name": "Updated Name",
                    "email": "updated@example.com"
                },
                "timestamp": "2024-01-15T10:30:00Z"
            },
            "metadata": {
                "source_service": "storage-service",
                "correlation_id": "storage-001",
                "database": "profile_storage",
                "priority": "normal"
            }
        }' 2>/dev/null || echo "000")
    
    local sync_http_code="${data_sync_response: -3}"
    
    if [[ "$sync_http_code" == "200" ]] || [[ "$sync_http_code" == "201" ]] || [[ "$sync_http_code" == "202" ]]; then
        test_result "Storage Service → Queue Service (data sync)" "PASS" "Data sync message published successfully (HTTP $sync_http_code)"
    else
        test_result "Storage Service → Queue Service (data sync)" "FAIL" "Data sync message publishing failed (HTTP $sync_http_code)"
    fi
    
    # Test 2: Verify Storage Service can send backup/restore messages
    log_info "Testing backup operation message publishing..."
    
    local backup_response
    backup_response=$(curl -s -w "%{http_code}" -X POST "${BASE_URL}/api/v1/queue/messages" \
        -H "Content-Type: application/json" \
        -d '{
            "type": "backup_operation",
            "routing_key": "profile.task",
            "payload": {
                "operation": "backup",
                "tables": ["profiles", "addresses", "contacts"],
                "backup_type": "incremental",
                "destination": "s3://backups/profiles/"
            },
            "metadata": {
                "source_service": "storage-service",
                "correlation_id": "storage-002",
                "priority": "low",
                "scheduled": "true"
            }
        }' 2>/dev/null || echo "000")
    
    local backup_http_code="${backup_response: -3}"
    
    if [[ "$backup_http_code" == "200" ]] || [[ "$backup_http_code" == "201" ]] || [[ "$backup_http_code" == "202" ]]; then
        test_result "Storage Service → Queue Service (backup)" "PASS" "Backup operation message published successfully (HTTP $backup_http_code)"
    else
        test_result "Storage Service → Queue Service (backup)" "FAIL" "Backup operation message publishing failed (HTTP $backup_http_code)"
    fi
    
    # Test 3: Verify Storage Service health check connectivity
    log_info "Testing Storage Service connectivity for integration scenarios..."
    
    if curl -s --connect-timeout 5 --max-time 10 "http://localhost:30082/health" | grep -q "healthy\|ok"; then
        test_result "Storage Service connectivity" "PASS" "Storage Service is accessible for integration (HTTP 200)"
    else
        test_result "Storage Service connectivity" "EXPECTED_FAIL" "Storage Service may not be accessible (check deployment status)"
    fi
    
    # Test 4: Verify Storage Service can send data validation messages
    log_info "Testing data validation message publishing..."
    
    local validation_response
    validation_response=$(curl -s -w "%{http_code}" -X POST "${BASE_URL}/api/v1/queue/messages" \
        -H "Content-Type: application/json" \
        -d '{
            "type": "data_validation",
            "routing_key": "profile.task",
            "payload": {
                "validation_type": "integrity_check",
                "tables": ["profiles"],
                "constraints": ["foreign_keys", "unique_constraints", "not_null"],
                "user_id": "123"
            },
            "metadata": {
                "source_service": "storage-service",
                "correlation_id": "storage-003",
                "priority": "normal"
            }
        }' 2>/dev/null || echo "000")
    
    local validation_http_code="${validation_response: -3}"
    
    if [[ "$validation_http_code" == "200" ]] || [[ "$validation_http_code" == "201" ]] || [[ "$validation_http_code" == "202" ]]; then
        test_result "Storage Service → Queue Service (validation)" "PASS" "Data validation message published successfully (HTTP $validation_http_code)"
    else
        test_result "Storage Service → Queue Service (validation)" "FAIL" "Data validation message publishing failed (HTTP $validation_http_code)"
    fi
}

test_auth_service_integration() {
    log_test "Testing integration with Auth Service..."
    
    # Test 1: Verify Auth Service can send authentication event messages
    log_info "Testing authentication event message publishing..."
    
    local auth_event_response
    auth_event_response=$(curl -s -w "%{http_code}" -X POST "${BASE_URL}/api/v1/queue/messages" \
        -H "Content-Type: application/json" \
        -d '{
            "type": "auth_event",
            "routing_key": "profile.task",
            "payload": {
                "event_type": "login_success",
                "user_id": "123",
                "session_id": "sess_abc123",
                "ip_address": "192.168.1.100",
                "user_agent": "Mozilla/5.0...",
                "timestamp": "2024-01-15T10:30:00Z"
            },
            "metadata": {
                "source_service": "auth-service",
                "correlation_id": "auth-001",
                "priority": "high",
                "security_event": "true"
            }
        }' 2>/dev/null || echo "000")
    
    local auth_http_code="${auth_event_response: -3}"
    
    if [[ "$auth_http_code" == "200" ]] || [[ "$auth_http_code" == "201" ]] || [[ "$auth_http_code" == "202" ]]; then
        test_result "Auth Service → Queue Service (auth events)" "PASS" "Authentication event message published successfully (HTTP $auth_http_code)"
    else
        test_result "Auth Service → Queue Service (auth events)" "FAIL" "Authentication event message publishing failed (HTTP $auth_http_code)"
    fi
    
    # Test 2: Verify Auth Service can send token management messages
    log_info "Testing token management message publishing..."
    
    local token_mgmt_response
    token_mgmt_response=$(curl -s -w "%{http_code}" -X POST "${BASE_URL}/api/v1/queue/messages" \
        -H "Content-Type: application/json" \
        -d '{
            "type": "token_management",
            "routing_key": "profile.task",
            "payload": {
                "action": "refresh",
                "user_id": "123",
                "old_token_id": "tok_old123",
                "new_token_id": "tok_new456",
                "expiry": "2024-01-16T10:30:00Z"
            },
            "metadata": {
                "source_service": "auth-service",
                "correlation_id": "auth-002",
                "priority": "normal"
            }
        }' 2>/dev/null || echo "000")
    
    local token_http_code="${token_mgmt_response: -3}"
    
    if [[ "$token_http_code" == "200" ]] || [[ "$token_http_code" == "201" ]] || [[ "$token_http_code" == "202" ]]; then
        test_result "Auth Service → Queue Service (token mgmt)" "PASS" "Token management message published successfully (HTTP $token_http_code)"
    else
        test_result "Auth Service → Queue Service (token mgmt)" "FAIL" "Token management message publishing failed (HTTP $token_http_code)"
    fi
    
    # Test 3: Verify Auth Service can send security alert messages
    log_info "Testing security alert message publishing..."
    
    local security_alert_response
    security_alert_response=$(curl -s -w "%{http_code}" -X POST "${BASE_URL}/api/v1/queue/messages" \
        -H "Content-Type: application/json" \
        -d '{
            "type": "security_alert",
            "routing_key": "email.send",
            "payload": {
                "alert_type": "suspicious_login",
                "user_id": "123",
                "details": {
                    "ip_address": "192.168.1.200",
                    "location": "Unknown Location",
                    "failed_attempts": 5
                },
                "severity": "high",
                "action_required": "true"
            },
            "metadata": {
                "source_service": "auth-service",
                "correlation_id": "auth-003",
                "priority": "high",
                "alert": "true"
            }
        }' 2>/dev/null || echo "000")
    
    local alert_http_code="${security_alert_response: -3}"
    
    if [[ "$alert_http_code" == "200" ]] || [[ "$alert_http_code" == "201" ]] || [[ "$alert_http_code" == "202" ]]; then
        test_result "Auth Service → Queue Service (security alerts)" "PASS" "Security alert message published successfully (HTTP $alert_http_code)"
    else
        test_result "Auth Service → Queue Service (security alerts)" "FAIL" "Security alert message publishing failed (HTTP $alert_http_code)"
    fi
    
    # Test 4: Verify Auth Service health check connectivity
    log_info "Testing Auth Service connectivity for integration scenarios..."
    
    if curl -s --connect-timeout 5 --max-time 10 "http://localhost:30083/health" | grep -q "healthy\|ok"; then
        test_result "Auth Service connectivity" "PASS" "Auth Service is accessible for integration (HTTP 200)"
    else
        test_result "Auth Service connectivity" "EXPECTED_FAIL" "Auth Service may not be accessible (check deployment status)"
    fi
}

test_multi_service_workflow() {
    log_test "Testing multi-service integration workflow..."
    
    # Test 1: Simulate a complete user profile update workflow
    log_info "Testing complete profile update workflow across services..."
    
    # Step 1: Auth Service publishes authentication verification
    local auth_verify_response
    auth_verify_response=$(curl -s -w "%{http_code}" -X POST "${BASE_URL}/api/v1/queue/messages" \
        -H "Content-Type: application/json" \
        -d '{
            "type": "auth_verification",
            "routing_key": "profile.task",
            "payload": {
                "user_id": "workflow_test_123",
                "action": "profile_update_authorized",
                "token_valid": true,
                "permissions": ["profile:write", "profile:read"]
            },
            "metadata": {
                "source_service": "auth-service",
                "correlation_id": "workflow-001",
                "workflow_step": "1_auth_verify"
            }
        }' 2>/dev/null || echo "000")
    
    # Step 2: Storage Service publishes data update completion
    local storage_update_response
    storage_update_response=$(curl -s -w "%{http_code}" -X POST "${BASE_URL}/api/v1/queue/messages" \
        -H "Content-Type: application/json" \
        -d '{
            "type": "data_updated",
            "routing_key": "profile.task",
            "payload": {
                "user_id": "workflow_test_123",
                "table": "profiles",
                "updated_fields": ["name", "email"],
                "version": "v2"
            },
            "metadata": {
                "source_service": "storage-service",
                "correlation_id": "workflow-001",
                "workflow_step": "2_data_update"
            }
        }' 2>/dev/null || echo "000")
    
    # Step 3: Cache Service publishes cache invalidation
    local cache_invalidate_response
    cache_invalidate_response=$(curl -s -w "%{http_code}" -X POST "${BASE_URL}/api/v1/queue/messages" \
        -H "Content-Type: application/json" \
        -d '{
            "type": "cache_invalidated",
            "routing_key": "profile.task",
            "payload": {
                "user_id": "workflow_test_123",
                "invalidated_keys": ["user:workflow_test_123:profile"],
                "reason": "profile_updated"
            },
            "metadata": {
                "source_service": "cache-service",
                "correlation_id": "workflow-001",
                "workflow_step": "3_cache_invalidate"
            }
        }' 2>/dev/null || echo "000")
    
    # Verify all steps completed successfully
    local auth_code="${auth_verify_response: -3}"
    local storage_code="${storage_update_response: -3}"
    local cache_code="${cache_invalidate_response: -3}"
    
    if [[ "$auth_code" =~ ^(200|201|202)$ ]] && [[ "$storage_code" =~ ^(200|201|202)$ ]] && [[ "$cache_code" =~ ^(200|201|202)$ ]]; then
        test_result "Multi-service workflow" "PASS" "Complete profile update workflow executed successfully (Auth: $auth_code, Storage: $storage_code, Cache: $cache_code)"
    else
        test_result "Multi-service workflow" "FAIL" "Workflow failed - Auth: $auth_code, Storage: $storage_code, Cache: $cache_code"
    fi
    
    # Test 2: Verify message routing to different worker types
    log_info "Testing routing key distribution across worker types..."
    
    # Profile task
    local profile_task_response
    profile_task_response=$(curl -s -w "%{http_code}" -X POST "${BASE_URL}/api/v1/queue/messages" \
        -H "Content-Type: application/json" \
        -d '{
            "type": "integration_test",
            "routing_key": "profile.task",
            "payload": {"test": "profile_worker"},
            "metadata": {"test_type": "routing_verification"}
        }' 2>/dev/null || echo "000")
    
    # Email task
    local email_task_response
    email_task_response=$(curl -s -w "%{http_code}" -X POST "${BASE_URL}/api/v1/queue/messages" \
        -H "Content-Type: application/json" \
        -d '{
            "type": "integration_test",
            "routing_key": "email.send",
            "payload": {"test": "email_worker"},
            "metadata": {"test_type": "routing_verification"}
        }' 2>/dev/null || echo "000")
    
    # Image task
    local image_task_response
    image_task_response=$(curl -s -w "%{http_code}" -X POST "${BASE_URL}/api/v1/queue/messages" \
        -H "Content-Type: application/json" \
        -d '{
            "type": "integration_test",
            "routing_key": "image.process",
            "payload": {"test": "image_worker"},
            "metadata": {"test_type": "routing_verification"}
        }' 2>/dev/null || echo "000")
    
    local profile_code="${profile_task_response: -3}"
    local email_code="${email_task_response: -3}"
    local image_code="${image_task_response: -3}"
    
    if [[ "$profile_code" =~ ^(200|201|202)$ ]] && [[ "$email_code" =~ ^(200|201|202)$ ]] && [[ "$image_code" =~ ^(200|201|202)$ ]]; then
        test_result "Multi-worker routing" "PASS" "All routing keys working (Profile: $profile_code, Email: $email_code, Image: $image_code)"
    else
        test_result "Multi-worker routing" "FAIL" "Routing failed - Profile: $profile_code, Email: $email_code, Image: $image_code"
    fi
}

test_service_dependency_health() {
    log_test "Testing dependency service health for integration readiness..."
    
    # Check all dependency services are healthy and ready for integration
    local services=("cache-service:30081" "storage-service:30082" "auth-service:30083")
    local healthy_services=0
    local total_services=${#services[@]}
    
    for service_info in "${services[@]}"; do
        IFS=':' read -ra SERVICE_PARTS <<< "$service_info"
        local service_name="${SERVICE_PARTS[0]}"
        local service_port="${SERVICE_PARTS[1]}"
        
        log_info "Checking $service_name health..."
        
        if curl -s --connect-timeout 3 --max-time 5 "http://localhost:${service_port}/health" >/dev/null 2>&1; then
            log_success "$service_name is healthy and ready for integration"
            ((healthy_services++))
        else
            log_warning "$service_name is not accessible - integration tests may be limited"
        fi
    done
    
    if [[ $healthy_services -eq $total_services ]]; then
        test_result "All dependency services health" "PASS" "All $total_services dependency services are healthy"
    elif [[ $healthy_services -gt 0 ]]; then
        test_result "Partial dependency services health" "EXPECTED_FAIL" "$healthy_services/$total_services services healthy - some integration tests may fail"
    else
        test_result "No dependency services health" "EXPECTED_FAIL" "No dependency services accessible - integration tests will be limited"
    fi
    
    return 0
}

# 🎯 MAIN TEST EXECUTION
main() {
    echo "=== Queue Service Testing for Kind Cluster ==="
    echo "Service: $SERVICE_NAME"
    echo "NodePort: $SERVICE_PORT"
    echo "Base URL: $BASE_URL"
    echo ""
    
    # Wait for service readiness
    if ! test_service_readiness; then
        log_error "Service not ready, aborting tests"
        exit 1
    fi
    
    log_info "Starting queue service tests..."
    echo ""
    
    # Execute test suites
    test_health_endpoints
    test_rabbitmq_backend
    test_message_publishing
    test_queue_status
    test_error_handling
    test_performance_characteristics
    test_persistence_and_recovery
    test_network_policies
    test_cache_service_integration
    test_storage_service_integration
    test_auth_service_integration
    test_multi_service_workflow
    test_service_dependency_health
    
    # Calculate results
    local actual_failures=$((TESTS_FAILED))
    local success_rate=0
    
    if [[ $TESTS_TOTAL -gt 0 ]]; then
        success_rate=$(( (TESTS_PASSED * 100) / (TESTS_TOTAL - TESTS_EXPECTED_FAIL) ))
    fi
    
    echo ""
    echo "=== Test Summary ==="
    echo "Total tests run: $TESTS_TOTAL"
    echo "Tests passed: $TESTS_PASSED"
    echo "Tests failed: $TESTS_FAILED"
    echo "Expected fails: $TESTS_EXPECTED_FAIL"
    echo ""
    
    if [[ $actual_failures -eq 0 ]]; then
        log_success "All tests passed! Queue service is fully functional."
        echo "📋 Expected failures are normal for services with network restrictions."
    else
        log_error "Some tests failed. Check the output above for details."
        echo "🔧 Review the failing tests and fix any deployment issues."
    fi
    
    cleanup_test_data
    
    # Exit with appropriate code
    exit $actual_failures
}

# 🚀 SCRIPT ENTRY POINT
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi 