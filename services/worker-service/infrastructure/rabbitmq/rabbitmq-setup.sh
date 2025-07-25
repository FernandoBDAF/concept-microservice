#!/bin/bash

# RabbitMQ Setup Script for Multi-Worker Architecture
# This script sets up exchanges, queues, and dead letter queues for email and image workers

set -e

RABBITMQ_HOST=${RABBITMQ_HOST:-localhost}
RABBITMQ_PORT=${RABBITMQ_PORT:-15672}
RABBITMQ_USER=${RABBITMQ_USER:-guest}
RABBITMQ_PASSWORD=${RABBITMQ_PASSWORD:-guest}
RABBITMQ_MANAGEMENT_URL="http://${RABBITMQ_HOST}:${RABBITMQ_PORT}/api"

echo "🐰 Setting up RabbitMQ queues and exchanges..."
echo "📍 Management URL: ${RABBITMQ_MANAGEMENT_URL}"

# Function to make API calls to RabbitMQ Management
rabbitmq_api() {
    local method=$1
    local endpoint=$2
    local data=$3
    
    curl -s -u "${RABBITMQ_USER}:${RABBITMQ_PASSWORD}" \
         -X "${method}" \
         -H "Content-Type: application/json" \
         "${RABBITMQ_MANAGEMENT_URL}${endpoint}" \
         ${data:+-d "$data"}
}

# Wait for RabbitMQ to be ready
echo "⏳ Waiting for RabbitMQ to be ready..."
until rabbitmq_api GET "/whoami" > /dev/null 2>&1; do
    echo "   Waiting for RabbitMQ management interface..."
    sleep 2
done
echo "✅ RabbitMQ is ready!"

# Setup Email Worker Queues
echo ""
echo "📧 Setting up EMAIL WORKER queues..."

# Create email-tasks exchange
echo "   Creating email-tasks exchange..."
rabbitmq_api PUT "/exchanges/%2f/email-tasks" '{
    "type": "direct",
    "durable": true,
    "auto_delete": false,
    "arguments": {}
}'

# Create email-processing dead letter queue
echo "   Creating email-processing dead letter queue..."
rabbitmq_api PUT "/queues/%2f/email-processing.dlq" '{
    "durable": true,
    "auto_delete": false,
    "arguments": {}
}'

# Create email-processing queue with DLQ configuration
echo "   Creating email-processing queue..."
rabbitmq_api PUT "/queues/%2f/email-processing" '{
    "durable": true,
    "auto_delete": false,
    "arguments": {
        "x-dead-letter-exchange": "",
        "x-dead-letter-routing-key": "email-processing.dlq",
        "x-message-ttl": 3600000
    }
}'

# Bind email-processing queue to email-tasks exchange
echo "   Binding email-processing queue to email-tasks exchange..."
rabbitmq_api PUT "/bindings/%2f/e/email-tasks/q/email-processing" '{
    "routing_key": "email.send",
    "arguments": {}
}'

# Setup Image Worker Queues
echo ""
echo "🖼️  Setting up IMAGE WORKER queues..."

# Create image-tasks exchange
echo "   Creating image-tasks exchange..."
rabbitmq_api PUT "/exchanges/%2f/image-tasks" '{
    "type": "direct",
    "durable": true,
    "auto_delete": false,
    "arguments": {}
}'

# Create image-processing dead letter queue
echo "   Creating image-processing dead letter queue..."
rabbitmq_api PUT "/queues/%2f/image-processing.dlq" '{
    "durable": true,
    "auto_delete": false,
    "arguments": {}
}'

# Create image-processing queue with DLQ configuration
echo "   Creating image-processing queue..."
rabbitmq_api PUT "/queues/%2f/image-processing" '{
    "durable": true,
    "auto_delete": false,
    "arguments": {
        "x-dead-letter-exchange": "",
        "x-dead-letter-routing-key": "image-processing.dlq",
        "x-message-ttl": 21600000
    }
}'

# Bind image-processing queue to image-tasks exchange
echo "   Binding image-processing queue to image-tasks exchange..."
rabbitmq_api PUT "/bindings/%2f/e/image-tasks/q/image-processing" '{
    "routing_key": "image.process",
    "arguments": {}
}'

# Verify setup
echo ""
echo "🔍 Verifying setup..."

echo "   Checking exchanges:"
rabbitmq_api GET "/exchanges" | grep -E '"name":"(email-tasks|image-tasks)"' || echo "   ⚠️  Could not verify exchanges"

echo "   Checking queues:"
rabbitmq_api GET "/queues" | grep -E '"name":"(email-processing|image-processing|.*\.dlq)"' || echo "   ⚠️  Could not verify queues"

echo ""
echo "✅ RabbitMQ setup completed successfully!"
echo ""
echo "📋 Summary:"
echo "   📧 Email Worker:"
echo "      - Exchange: email-tasks (direct)"
echo "      - Queue: email-processing (TTL: 1 hour)"
echo "      - DLQ: email-processing.dlq"
echo "      - Routing Key: email.send"
echo ""
echo "   🖼️  Image Worker:"
echo "      - Exchange: image-tasks (direct)"
echo "      - Queue: image-processing (TTL: 6 hours)"
echo "      - DLQ: image-processing.dlq"
echo "      - Routing Key: image.process"
echo ""
echo "🚀 Workers can now be deployed and will connect to these queues!" 