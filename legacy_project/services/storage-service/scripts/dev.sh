#!/bin/bash

# Exit on error
set -e

# Function to display help
show_help() {
    echo "Profile Storage Service Development Script"
    echo ""
    echo "Usage: ./dev.sh [command]"
    echo ""
    echo "Commands:"
    echo "  start       Start the service and dependencies"
    echo "  stop        Stop the service and dependencies"
    echo "  build       Build the service"
    echo "  test        Run tests"
    echo "  proto       Generate protobuf code"
    echo "  clean       Clean up generated files and containers"
    echo "  help        Show this help message"
}

# Function to start services
start_services() {
    echo "Starting services..."
    docker-compose up -d
    echo "Services started. Waiting for PostgreSQL to be ready..."
    sleep 5
    echo "Services are ready!"
}

# Function to stop services
stop_services() {
    echo "Stopping services..."
    docker-compose down
    echo "Services stopped."
}

# Function to build the service
build_service() {
    echo "Building service..."
    docker-compose build
    echo "Service built."
}

# Function to run tests
run_tests() {
    echo "Running tests..."
    go test -v ./...
    echo "Tests completed."
}

# Function to generate protobuf code
generate_proto() {
    echo "Generating protobuf code..."
    ./scripts/generate_proto.sh
    echo "Protobuf code generated."
}

# Function to clean up
cleanup() {
    echo "Cleaning up..."
    docker-compose down -v
    rm -rf proto/profile/*.pb.go
    echo "Cleanup completed."
}

# Main script logic
case "$1" in
    "start")
        start_services
        ;;
    "stop")
        stop_services
        ;;
    "build")
        build_service
        ;;
    "test")
        run_tests
        ;;
    "proto")
        generate_proto
        ;;
    "clean")
        cleanup
        ;;
    "help"|"")
        show_help
        ;;
    *)
        echo "Unknown command: $1"
        show_help
        exit 1
        ;;
esac 