#!/bin/bash

# Test Message Publisher for Multi-Worker Architecture
# This script publishes example messages to test email and image workers

set -e

RABBITMQ_HOST=${RABBITMQ_HOST:-localhost}
RABBITMQ_PORT=${RABBITMQ_PORT:-15672}
RABBITMQ_USER=${RABBITMQ_USER:-guest}
RABBITMQ_PASSWORD=${RABBITMQ_PASSWORD:-guest}
RABBITMQ_MANAGEMENT_URL="http://${RABBITMQ_HOST}:${RABBITMQ_PORT}/api"

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

echo "📨 Publishing test messages to RabbitMQ..."
echo "📍 Management URL: ${RABBITMQ_MANAGEMENT_URL}"

# Function to publish message to exchange
publish_message() {
    local exchange=$1
    local routing_key=$2
    local message_file=$3
    local message_type=$4
    
    echo "   Publishing ${message_type} message..."
    
    # Read message content and escape for JSON
    local message_content=$(cat "${SCRIPT_DIR}/${message_file}")
    
    # Create the publish request
    local publish_data=$(cat <<EOF
{
    "properties": {
        "delivery_mode": 2,
        "content_type": "application/json",
        "timestamp": $(date +%s)000
    },
    "routing_key": "${routing_key}",
    "payload": ${message_content},
    "payload_encoding": "string"
}
EOF
)
    
    # Publish the message
    curl -s -u "${RABBITMQ_USER}:${RABBITMQ_PASSWORD}" \
         -X POST \
         -H "Content-Type: application/json" \
         "${RABBITMQ_MANAGEMENT_URL}/exchanges/%2f/${exchange}/publish" \
         -d "${publish_data}" | grep -q '"routed":true' && echo "      ✅ Message published successfully" || echo "      ❌ Failed to publish message"
}

# Wait for RabbitMQ to be ready
echo "⏳ Waiting for RabbitMQ to be ready..."
until curl -s -u "${RABBITMQ_USER}:${RABBITMQ_PASSWORD}" "${RABBITMQ_MANAGEMENT_URL}/whoami" > /dev/null 2>&1; do
    echo "   Waiting for RabbitMQ management interface..."
    sleep 2
done
echo "✅ RabbitMQ is ready!"

echo ""
echo "📧 Publishing EMAIL WORKER test messages..."

# Publish email messages
publish_message "email-tasks" "email.send" "publish-email.json" "welcome email"
publish_message "email-tasks" "email.send" "publish-email-notification.json" "notification email"
publish_message "email-tasks" "email.send" "publish-email-alert.json" "alert email"

echo ""
echo "🖼️  Publishing IMAGE WORKER test messages..."

# Publish image messages
publish_message "image-tasks" "image.process" "publish-image-resize.json" "image resize"
publish_message "image-tasks" "image.process" "publish-image-filter.json" "image filter"
publish_message "image-tasks" "image.process" "publish-image-analyze.json" "image analyze"

echo ""
echo "🔍 Checking queue status..."

# Function to get queue info
get_queue_info() {
    local queue_name=$1
    local info=$(curl -s -u "${RABBITMQ_USER}:${RABBITMQ_PASSWORD}" "${RABBITMQ_MANAGEMENT_URL}/queues/%2f/${queue_name}")
    local messages=$(echo "${info}" | grep -o '"messages":[0-9]*' | cut -d':' -f2)
    local consumers=$(echo "${info}" | grep -o '"consumers":[0-9]*' | cut -d':' -f2)
    echo "   ${queue_name}: ${messages:-0} messages, ${consumers:-0} consumers"
}

get_queue_info "email-processing"
get_queue_info "image-processing"

echo ""
echo "✅ Test messages published successfully!"
echo ""
echo "📋 Next steps:"
echo "   1. Deploy email and image workers to Kubernetes"
echo "   2. Watch worker logs to see message processing"
echo "   3. Monitor queue metrics in RabbitMQ management interface"
echo "   4. Check Prometheus metrics for worker performance"
echo ""
echo "🔗 RabbitMQ Management UI: http://${RABBITMQ_HOST}:${RABBITMQ_PORT}"
echo "   Username: ${RABBITMQ_USER}"
echo "   Password: ${RABBITMQ_PASSWORD}" 