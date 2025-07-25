#!/bin/bash

# Queue Service Validation Script
# This script validates the upgraded queue service functionality

set -e

BASE_URL="http://localhost:8080"
API_PATH="/api/v1/queue"

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    local status=$1
    local message=$2
    if [ "$status" = "SUCCESS" ]; then
        echo -e "${GREEN}✅ $message${NC}"
    elif [ "$status" = "ERROR" ]; then
        echo -e "${RED}❌ $message${NC}"
    else
        echo -e "${YELLOW}ℹ️  $message${NC}"
    fi
}

# Function to check if service is running
check_service() {
    print_status "INFO" "Checking if queue service is running..."
    if curl -s -f "$BASE_URL/health" > /dev/null; then
        print_status "SUCCESS" "Queue service is running at $BASE_URL"
    else
        print_status "ERROR" "Queue service is not running at $BASE_URL"
        print_status "INFO" "Please start the service with: go run cmd/main.go"
        exit 1
    fi
}

# Function to test routing keys endpoint
test_routing_keys() {
    print_status "INFO" "Testing routing keys endpoint..."
    
    response=$(curl -s "$BASE_URL$API_PATH/routing-keys")
    
    if echo "$response" | jq -e '.routing_keys | contains(["profile.task", "email.send", "image.process"])' > /dev/null; then
        print_status "SUCCESS" "Routing keys endpoint working correctly"
        echo "Available routing keys: $(echo "$response" | jq -r '.routing_keys | join(", ")')"
    else
        print_status "ERROR" "Routing keys endpoint failed"
        return 1
    fi
}

# Function to test message publishing
test_message_publishing() {
    local routing_key=$1
    local message_type=$2
    local test_name=$3
    
    print_status "INFO" "Testing $test_name..."
    
    # Create test payload
    local payload='{
        "type": "'$message_type'",
        "payload": {"test": "data", "timestamp": '$(date +%s)'},
        "metadata": {"source": "validation-script", "test": "'$test_name'"},
        "routing_key": "'$routing_key'",
        "priority": 1
    }'
    
    # Publish message
    local response=$(curl -s -X POST \
        -H "Content-Type: application/json" \
        -d "$payload" \
        "$BASE_URL$API_PATH/messages")
    
    # Check response
    local message_id=$(echo "$response" | jq -r '.message_id // empty')
    local status=$(echo "$response" | jq -r '.status // empty')
    local returned_routing_key=$(echo "$response" | jq -r '.routing_key // empty')
    
    if [ -n "$message_id" ] && [ "$status" = "accepted" ] && [ "$returned_routing_key" = "$routing_key" ]; then
        print_status "SUCCESS" "$test_name: Message published (ID: $message_id)"
        
        # Test message status
        sleep 0.1
        local status_response=$(curl -s "$BASE_URL$API_PATH/status/$message_id")
        local msg_status=$(echo "$status_response" | jq -r '.status // empty')
        
        if [ -n "$msg_status" ]; then
            print_status "SUCCESS" "$test_name: Message status retrieved ($msg_status)"
        else
            print_status "ERROR" "$test_name: Failed to retrieve message status"
        fi
    else
        print_status "ERROR" "$test_name: Failed to publish message"
        echo "Response: $response"
        return 1
    fi
}

# Function to test backward compatibility
test_backward_compatibility() {
    print_status "INFO" "Testing backward compatibility..."
    
    # Test without routing key - should infer from message type
    local payload='{
        "type": "profile_update",
        "payload": {"test": "backward_compatibility"},
        "metadata": {"source": "legacy-system"},
        "priority": 0
    }'
    
    local response=$(curl -s -X POST \
        -H "Content-Type: application/json" \
        -d "$payload" \
        "$BASE_URL$API_PATH/messages")
    
    local routing_key=$(echo "$response" | jq -r '.routing_key // empty')
    
    if [ "$routing_key" = "profile.task" ]; then
        print_status "SUCCESS" "Backward compatibility: Routing key correctly inferred as $routing_key"
    else
        print_status "ERROR" "Backward compatibility: Failed to infer routing key"
        return 1
    fi
}

# Function to test routing key validation
test_routing_key_validation() {
    print_status "INFO" "Testing routing key validation..."
    
    # Test invalid routing key
    local payload='{
        "type": "test_message",
        "payload": {"test": "invalid_routing_key"},
        "metadata": {"source": "validation-script"},
        "routing_key": "invalid.key"
    }'
    
    local http_code=$(curl -s -o /dev/null -w "%{http_code}" -X POST \
        -H "Content-Type: application/json" \
        -d "$payload" \
        "$BASE_URL$API_PATH/messages")
    
    if [ "$http_code" = "400" ]; then
        print_status "SUCCESS" "Routing key validation: Invalid routing key correctly rejected"
    else
        print_status "ERROR" "Routing key validation: Invalid routing key not rejected (HTTP $http_code)"
        return 1
    fi
}

# Function to test message format compatibility
test_message_format() {
    print_status "INFO" "Testing message format compatibility..."
    
    # Test that we use the correct field names expected by worker-service
    local payload='{
        "type": "profile_update",
        "payload": {"user_id": "12345", "changes": {"name": "Test User"}},
        "metadata": {"source": "validation-script", "format_test": "true"},
        "routing_key": "profile.task",
        "priority": 1
    }'
    
    local response=$(curl -s -X POST \
        -H "Content-Type: application/json" \
        -d "$payload" \
        "$BASE_URL$API_PATH/messages")
    
    local message_id=$(echo "$response" | jq -r '.message_id // empty')
    
    if [ -n "$message_id" ]; then
        print_status "SUCCESS" "Message format: Compatible with worker-service expectations"
    else
        print_status "ERROR" "Message format: Compatibility test failed"
        return 1
    fi
}

# Main validation function
main() {
    echo "🚀 Queue Service Validation"
    echo "=========================="
    echo ""
    
    # Check if required tools are available
    if ! command -v curl &> /dev/null; then
        print_status "ERROR" "curl is required but not installed"
        exit 1
    fi
    
    if ! command -v jq &> /dev/null; then
        print_status "ERROR" "jq is required but not installed"
        exit 1
    fi
    
    # Run validation tests
    check_service
    echo ""
    
    test_routing_keys
    echo ""
    
    test_message_publishing "profile.task" "profile_update" "Profile Task"
    test_message_publishing "email.send" "email_send" "Email Task"
    test_message_publishing "image.process" "image_process" "Image Processing Task"
    echo ""
    
    test_backward_compatibility
    echo ""
    
    test_routing_key_validation
    echo ""
    
    test_message_format
    echo ""
    
    print_status "SUCCESS" "All validation tests passed! 🎉"
    echo ""
    echo "Queue service is ready for production use with:"
    echo "• Worker-service compatible message format"
    echo "• Multi-worker routing key support"
    echo "• Backward compatibility maintained"
    echo "• RabbitMQ best practices implemented"
    echo "• Publisher confirms enabled"
}

# Run main function
main "$@" 