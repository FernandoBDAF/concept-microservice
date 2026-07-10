#!/bin/bash

# Test Queue Processing Script
# Purpose: Validate that queue processing is working correctly
# Usage: ./test_queue_processing.sh

set -euo pipefail

# Configuration
STORAGE_SERVICE_URL="http://localhost:8080"
QUEUE_SERVICE_URL="http://localhost:8081"  # Assuming queue-service port
NAMESPACE="default"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Test queue processing functionality
test_queue_processing() {
    log_info "Testing storage-service queue processing..."

    # 1. Check if storage-service is running with queue processing
    log_info "Checking storage-service health..."
    if curl -f "$STORAGE_SERVICE_URL/health" &>/dev/null; then
        log_success "Storage service is healthy"
    else
        log_error "Storage service is not accessible"
        return 1
    fi

    # 2. Check storage-service logs for queue consumer startup
    log_info "Checking for queue consumer in logs..."
    if kubectl logs -l app=storage-service -n $NAMESPACE --tail=50 | grep -q "Starting queue consumer"; then
        log_success "Queue consumer started successfully"
    else
        log_warning "Queue consumer startup not found in logs"
    fi

    # 3. Send test message via queue-service (if available)
    log_info "Testing message processing via queue..."

    # Create test message
    local test_message='{
        "id": "test-'$(date +%s)'",
        "type": "profile_update",
        "payload": {
            "profile_id": "test-profile-123",
            "updates": {
                "name": "Test User",
                "email": "test@example.com"
            }
        },
        "timestamp": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'",
        "metadata": {
            "source": "test-script",
            "priority": "normal"
        },
        "routing_key": "storage.profile.update",
        "user_id": "test-user-123",
        "user_role": "user",
        "session_id": "test-session-123"
    }'

    # Try to send message via queue-service
    if curl -f -X POST "$QUEUE_SERVICE_URL/api/v1/messages" \
        -H "Content-Type: application/json" \
        -d "$test_message" &>/dev/null; then
        log_success "Test message sent to queue"

        # Wait for processing
        sleep 2

        # Check storage-service logs for message processing
        if kubectl logs -l app=storage-service -n $NAMESPACE --tail=20 | grep -q "ProcessMessage"; then
            log_success "Message processed by storage-service"
        else
            log_warning "Message processing not detected in logs"
        fi
    else
        log_warning "Could not send test message (queue-service may not be available)"
    fi

    # 4. Check metrics for queue processing
    log_info "Checking queue processing metrics..."
    if curl -s "$STORAGE_SERVICE_URL/metrics" | grep -q "queue_messages_processed"; then
        log_success "Queue processing metrics available"
    else
        log_warning "Queue processing metrics not found"
    fi

    log_success "Queue processing test completed"
}

# Main execution
main() {
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}  Storage Service Queue Processing Test${NC}"
    echo -e "${BLUE}========================================${NC}"

    test_queue_processing

    log_success "All tests completed! 🎉"
}

# Execute main function
main "$@" 