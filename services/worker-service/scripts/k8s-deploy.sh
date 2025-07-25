#!/bin/bash

# Kubernetes Deployment Script for Multi-Worker Architecture
# This script deploys and tests the multi-worker system in Kubernetes (kind cluster)

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "☸️  Deploying Multi-Worker Architecture to Kubernetes"
echo "📍 Project Root: ${PROJECT_ROOT}"

cd "${PROJECT_ROOT}"

# Check if kubectl is available and kind cluster is running
if ! kubectl cluster-info > /dev/null 2>&1; then
    echo "❌ Kubernetes cluster is not accessible. Please ensure kind cluster is running."
    echo "   To start kind cluster: kind create cluster"
    exit 1
fi

echo "✅ Kubernetes cluster is accessible"

# Build worker images
echo ""
echo "🔨 Building worker Docker images..."
docker build -t email-worker:latest -f services/workers/email-worker/Dockerfile services/workers/email-worker/
docker build -t image-worker:latest -f services/workers/image-worker/Dockerfile services/workers/image-worker/

# Load images into kind cluster
echo ""
echo "📦 Loading images into kind cluster..."
kind load docker-image email-worker:latest
kind load docker-image image-worker:latest

# Deploy RabbitMQ and configuration
echo ""
echo "🐰 Deploying RabbitMQ..."
kubectl apply -f infrastructure/rabbitmq/k8s-rabbitmq-config.yaml

echo ""
echo "⏳ Waiting for RabbitMQ to be ready..."
kubectl wait --for=condition=available --timeout=300s deployment/rabbitmq -n rabbitmq

# Setup RabbitMQ queues
echo ""
echo "⚙️  Setting up RabbitMQ queues..."
kubectl run rabbitmq-setup --rm -i --restart=Never --image=curlimages/curl:latest \
  --env="RABBITMQ_HOST=rabbitmq.rabbitmq.svc.cluster.local" \
  --env="RABBITMQ_PORT=15672" \
  --env="RABBITMQ_USER=guest" \
  --env="RABBITMQ_PASSWORD=guest" \
  -- sh -c "
    # Wait for RabbitMQ management interface
    until curl -s -u guest:guest http://rabbitmq.rabbitmq.svc.cluster.local:15672/api/whoami > /dev/null 2>&1; do
      echo 'Waiting for RabbitMQ management interface...'
      sleep 5
    done
    
    echo 'Setting up exchanges and queues...'
    
    # Email tasks setup
    curl -s -u guest:guest -X PUT -H 'Content-Type: application/json' \
      http://rabbitmq.rabbitmq.svc.cluster.local:15672/api/exchanges/%2f/email-tasks \
      -d '{\"type\":\"direct\",\"durable\":true,\"auto_delete\":false,\"arguments\":{}}'
    
    curl -s -u guest:guest -X PUT -H 'Content-Type: application/json' \
      http://rabbitmq.rabbitmq.svc.cluster.local:15672/api/queues/%2f/email-processing.dlq \
      -d '{\"durable\":true,\"auto_delete\":false,\"arguments\":{}}'
    
    curl -s -u guest:guest -X PUT -H 'Content-Type: application/json' \
      http://rabbitmq.rabbitmq.svc.cluster.local:15672/api/queues/%2f/email-processing \
      -d '{\"durable\":true,\"auto_delete\":false,\"arguments\":{\"x-dead-letter-exchange\":\"\",\"x-dead-letter-routing-key\":\"email-processing.dlq\",\"x-message-ttl\":3600000}}'
    
    curl -s -u guest:guest -X PUT -H 'Content-Type: application/json' \
      http://rabbitmq.rabbitmq.svc.cluster.local:15672/api/bindings/%2f/e/email-tasks/q/email-processing \
      -d '{\"routing_key\":\"email.send\",\"arguments\":{}}'
    
    # Image tasks setup
    curl -s -u guest:guest -X PUT -H 'Content-Type: application/json' \
      http://rabbitmq.rabbitmq.svc.cluster.local:15672/api/exchanges/%2f/image-tasks \
      -d '{\"type\":\"direct\",\"durable\":true,\"auto_delete\":false,\"arguments\":{}}'
    
    curl -s -u guest:guest -X PUT -H 'Content-Type: application/json' \
      http://rabbitmq.rabbitmq.svc.cluster.local:15672/api/queues/%2f/image-processing.dlq \
      -d '{\"durable\":true,\"auto_delete\":false,\"arguments\":{}}'
    
    curl -s -u guest:guest -X PUT -H 'Content-Type: application/json' \
      http://rabbitmq.rabbitmq.svc.cluster.local:15672/api/queues/%2f/image-processing \
      -d '{\"durable\":true,\"auto_delete\":false,\"arguments\":{\"x-dead-letter-exchange\":\"\",\"x-dead-letter-routing-key\":\"image-processing.dlq\",\"x-message-ttl\":21600000}}'
    
    curl -s -u guest:guest -X PUT -H 'Content-Type: application/json' \
      http://rabbitmq.rabbitmq.svc.cluster.local:15672/api/bindings/%2f/e/image-tasks/q/image-processing \
      -d '{\"routing_key\":\"image.process\",\"arguments\":{}}'
    
    echo 'RabbitMQ setup completed!'
  "

# Deploy workers
echo ""
echo "👥 Deploying Email Worker..."
kubectl apply -f services/workers/email-worker/k8s/

echo ""
echo "👥 Deploying Image Worker..."
kubectl apply -f services/workers/image-worker/k8s/

# Wait for deployments to be ready
echo ""
echo "⏳ Waiting for workers to be ready..."
kubectl wait --for=condition=available --timeout=300s deployment/email-worker -n workers
kubectl wait --for=condition=available --timeout=300s deployment/image-worker -n workers

# Test health endpoints
echo ""
echo "🏥 Testing worker health endpoints..."
kubectl port-forward -n workers service/email-worker-service 18081:8080 &
EMAIL_PF_PID=$!
kubectl port-forward -n workers service/image-worker-service 18082:8080 &
IMAGE_PF_PID=$!

sleep 5

# Test email worker health
if curl -s http://localhost:18081/health | grep -q '"status":"ok"'; then
    echo "✅ Email worker health check passed"
else
    echo "❌ Email worker health check failed"
fi

# Test image worker health
if curl -s http://localhost:18082/health | grep -q '"status":"ok"'; then
    echo "✅ Image worker health check passed"
else
    echo "❌ Image worker health check failed"
fi

# Clean up port forwards
kill $EMAIL_PF_PID $IMAGE_PF_PID 2>/dev/null || true

# Publish test messages
echo ""
echo "📨 Publishing test messages..."
kubectl run test-publisher --rm -i --restart=Never --image=curlimages/curl:latest \
  --env="RABBITMQ_HOST=rabbitmq.rabbitmq.svc.cluster.local" \
  --env="RABBITMQ_PORT=15672" \
  --env="RABBITMQ_USER=guest" \
  --env="RABBITMQ_PASSWORD=guest" \
  -- sh -c "
    echo 'Publishing test email message...'
    curl -s -u guest:guest -X POST -H 'Content-Type: application/json' \
      http://rabbitmq.rabbitmq.svc.cluster.local:15672/api/exchanges/%2f/email-tasks/publish \
      -d '{\"properties\":{\"delivery_mode\":2,\"content_type\":\"application/json\"},\"routing_key\":\"email.send\",\"payload\":\"{\\\"type\\\":\\\"email\\\",\\\"payload\\\":{\\\"recipient\\\":\\\"test@example.com\\\",\\\"email_type\\\":\\\"welcome\\\",\\\"template\\\":\\\"test_template\\\",\\\"data\\\":{\\\"name\\\":\\\"Test User\\\"},\\\"priority\\\":\\\"normal\\\"}}\",\"payload_encoding\":\"string\"}'
    
    echo 'Publishing test image message...'
    curl -s -u guest:guest -X POST -H 'Content-Type: application/json' \
      http://rabbitmq.rabbitmq.svc.cluster.local:15672/api/exchanges/%2f/image-tasks/publish \
      -d '{\"properties\":{\"delivery_mode\":2,\"content_type\":\"application/json\"},\"routing_key\":\"image.process\",\"payload\":\"{\\\"type\\\":\\\"image\\\",\\\"payload\\\":{\\\"image_url\\\":\\\"https://example.com/test.jpg\\\",\\\"processing_type\\\":\\\"resize\\\",\\\"parameters\\\":{\\\"width\\\":800,\\\"height\\\":600},\\\"callback_url\\\":\\\"https://example.com/callback\\\",\\\"timeout_seconds\\\":300,\\\"priority\\\":\\\"normal\\\"}}\",\"payload_encoding\":\"string\"}'
    
    echo 'Test messages published!'
  "

echo ""
echo "📊 Checking deployment status..."
kubectl get pods -n workers
kubectl get services -n workers

echo ""
echo "✅ Kubernetes deployment completed successfully!"
echo ""
echo "🔗 Useful commands:"
echo "   📧 Email worker logs:     kubectl logs -f deployment/email-worker -n workers"
echo "   🖼️  Image worker logs:     kubectl logs -f deployment/image-worker -n workers"
echo "   🐰 RabbitMQ management:   kubectl port-forward -n rabbitmq service/rabbitmq 15672:15672"
echo "   📊 Worker metrics:        kubectl port-forward -n workers service/email-worker-service 8080:8080"
echo ""
echo "🧹 To clean up: kubectl delete -f services/workers/email-worker/k8s/ && kubectl delete -f services/workers/image-worker/k8s/ && kubectl delete -f infrastructure/rabbitmq/k8s-rabbitmq-config.yaml" 