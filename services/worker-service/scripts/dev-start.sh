#!/bin/bash

# Local Development Startup Script for Multi-Worker Architecture
# This script starts the complete multi-worker system locally using Docker Compose

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "🚀 Starting Multi-Worker Architecture for Local Development"
echo "📍 Project Root: ${PROJECT_ROOT}"

cd "${PROJECT_ROOT}"

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo "❌ Docker is not running. Please start Docker and try again."
    exit 1
fi

# Build and start all services
echo ""
echo "🔨 Building worker images..."
docker-compose build --parallel

echo ""
echo "🐰 Starting RabbitMQ and setting up queues..."
docker-compose up -d rabbitmq rabbitmq-setup

echo ""
echo "⏳ Waiting for RabbitMQ setup to complete..."
docker-compose logs -f rabbitmq-setup

echo ""
echo "👥 Starting workers..."
docker-compose up -d email-worker image-worker

echo ""
echo "📊 Starting monitoring services..."
docker-compose up -d prometheus grafana

echo ""
echo "📨 Publishing test messages..."
docker-compose up -d test-publisher

echo ""
echo "✅ Multi-Worker Architecture started successfully!"
echo ""
echo "🔗 Service URLs:"
echo "   📧 Email Worker Health:    http://localhost:8081/health"
echo "   🖼️  Image Worker Health:    http://localhost:8082/health"
echo "   🐰 RabbitMQ Management:    http://localhost:15672 (guest/guest)"
echo "   📊 Prometheus:             http://localhost:9090"
echo "   📈 Grafana:                http://localhost:3000 (admin/admin)"
echo ""
echo "📋 Next steps:"
echo "   1. Check worker health endpoints"
echo "   2. Monitor RabbitMQ queues for message processing"
echo "   3. View worker logs: docker-compose logs -f email-worker image-worker"
echo "   4. Check Prometheus metrics for worker performance"
echo "   5. Create Grafana dashboards for visualization"
echo ""
echo "🛑 To stop all services: docker-compose down"
echo "🗑️  To clean up volumes: docker-compose down -v" 