#!/bin/bash

# Automated Kind Deployment Script for Auth Service
# Purpose: Deploy auth-service to Kind cluster with dependencies
# Usage: ./deploy-to-kind.sh [--with-dependencies] [--cleanup]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
NAMESPACE="default"
SERVICE_NAME="auth-service"

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

# Check if Kind cluster is running
check_kind_cluster() {
    log_info "Checking Kind cluster status..."
    if ! kubectl cluster-info &>/dev/null; then
        log_error "Kind cluster is not running or kubectl is not configured"
        log_info "Please ensure Kind cluster is running: kind create cluster"
        exit 1
    fi
    log_success "Kind cluster is accessible"
}

# Check dependencies
check_dependencies() {
    log_info "Checking service dependencies..."

    # Check if storage-service is available
    if ! kubectl get service storage-service -n $NAMESPACE &>/dev/null; then
        log_warning "Storage service not found - auth-service will fail readiness checks"
        log_info "Deploy storage-service first or use --with-dependencies flag"
    else
        log_success "Storage service found"
    fi

    # Check if cache-service is available
    if ! kubectl get service cache-service -n $NAMESPACE &>/dev/null; then
        log_warning "Cache service not found - auth-service will have degraded performance"
        log_info "Deploy cache-service first or use --with-dependencies flag"
    else
        log_success "Cache service found"
    fi
}

# Deploy dependencies if requested
deploy_dependencies() {
    log_info "Deploying service dependencies..."

    # Deploy storage-service if not present
    if ! kubectl get service storage-service -n $NAMESPACE &>/dev/null; then
        log_info "Deploying storage-service..."
        if [ -f "../../storage-service/deployments/kind/kustomization.yaml" ]; then
            kubectl apply -k ../../storage-service/deployments/kind/
            log_success "Storage service deployed"
        else
            log_warning "Storage service deployment not found - creating placeholder"
            kubectl apply -f auth-dependencies.yaml
        fi
    fi

    # Deploy cache-service if not present
    if ! kubectl get service cache-service -n $NAMESPACE &>/dev/null; then
        log_info "Deploying cache-service..."
        if [ -f "../../cache-service/deployments/kind/kustomization.yaml" ]; then
            kubectl apply -k ../../cache-service/deployments/kind/
            log_success "Cache service deployed"
        else
            log_warning "Cache service deployment not found - creating placeholder"
            # Placeholder already included in auth-dependencies.yaml
        fi
    fi
}

# Deploy auth-service
deploy_auth_service() {
    log_info "Deploying auth-service to Kind cluster..."

    # Apply kustomization
    kubectl apply -k "$SCRIPT_DIR/"

    log_success "Auth service deployment submitted"

    # Wait for rollout
    log_info "Waiting for auth-service rollout..."
    kubectl rollout status deployment/auth-service -n $NAMESPACE --timeout=300s

    log_success "Auth service is running"
}

# Verify deployment
verify_deployment() {
    log_info "Verifying auth-service deployment..."

    # Check pod status
    log_info "Pod status:"
    kubectl get pods -l app=auth-service -n $NAMESPACE

    # Check service endpoints
    log_info "Service endpoints:"
    kubectl get endpoints auth-service -n $NAMESPACE

    # Test health endpoint
    log_info "Testing health endpoint..."
    if kubectl port-forward service/auth-service 8080:8080 -n $NAMESPACE &>/dev/null &
    then
        local pf_pid=$!
        sleep 3

        if curl -f http://localhost:8080/health &>/dev/null; then
            log_success "Health check passed"
        else
            log_warning "Health check failed - service may still be starting"
        fi

        kill $pf_pid 2>/dev/null || true
    fi

    # Show access information
    log_success "Auth service deployed successfully!"
    log_info "Access information:"
    echo "  - Service URL: http://localhost:30080 (NodePort)"
    echo "  - Metrics URL: http://localhost:30081 (NodePort)"
    echo "  - Health check: curl http://localhost:30080/health"
    echo "  - Login endpoint: curl -X POST http://localhost:30080/v1/auth/login"
}

# Cleanup deployment
cleanup_deployment() {
    log_info "Cleaning up auth-service deployment..."

    kubectl delete -k "$SCRIPT_DIR/" --ignore-not-found=true

    log_success "Auth service cleaned up"
}

# Show usage
show_usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --with-dependencies  Deploy storage and cache service dependencies"
    echo "  --cleanup           Remove auth-service deployment"
    echo "  --help              Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0                          # Deploy auth-service only"
    echo "  $0 --with-dependencies      # Deploy with dependencies"
    echo "  $0 --cleanup                # Remove deployment"
}

# Main execution
main() {
    local with_dependencies=false
    local cleanup=false

    # Parse arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --with-dependencies)
                with_dependencies=true
                shift
                ;;
            --cleanup)
                cleanup=true
                shift
                ;;
            --help)
                show_usage
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                show_usage
                exit 1
                ;;
        esac
    done

    # Execute based on options
    if [ "$cleanup" = true ]; then
        cleanup_deployment
        exit 0
    fi

    log_info "Starting auth-service deployment to Kind cluster..."

    check_kind_cluster

    if [ "$with_dependencies" = true ]; then
        deploy_dependencies
    else
        check_dependencies
    fi

    deploy_auth_service
    verify_deployment

    log_success "Deployment completed successfully!"
}

# Execute main function
main "$@" 