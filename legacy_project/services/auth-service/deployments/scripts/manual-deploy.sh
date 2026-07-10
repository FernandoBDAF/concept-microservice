#!/bin/bash

# Manual Deployment Script for Auth Service
# Purpose: Step-by-step deployment with educational guidance
# Usage: ./manual-deploy.sh [--step-by-step] [--analyze] [--kind]

set -euo pipefail

SERVICE_NAME="auth-service"
NAMESPACE="default"
DEPLOYMENT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
STEP_BY_STEP=${STEP_BY_STEP:-false}
ANALYZE=${ANALYZE:-false}
KIND_MODE=${KIND_MODE:-false}

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

log_step() { echo -e "\n${CYAN}[STEP]${NC} $1"; }
log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_warning() { echo -e "${YELLOW}[WARNING]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

wait_for_user() {
    if [ "$STEP_BY_STEP" = true ]; then
        echo -e "\n${YELLOW}Press Enter to continue...${NC}"
        read -r
    fi
}

# Check cluster connectivity
check_cluster() {
    log_step "1. Checking Kubernetes cluster connectivity"
    
    if ! kubectl cluster-info &>/dev/null; then
        log_error "Cannot connect to Kubernetes cluster"
        exit 1
    fi
    
    local cluster_info=$(kubectl cluster-info | head -1)
    log_success "Connected to cluster: $cluster_info"
    
    if kubectl cluster-info | grep -q "kind"; then
        log_info "Kind cluster detected - enabling Kind mode"
        KIND_MODE=true
    fi
    
    wait_for_user
}

# Deploy using Kind or Production
deploy_application() {
    log_step "6. Deploying auth-service application"
    
    if [ "$KIND_MODE" = true ]; then
        log_info "Using Kind-specific deployment"
        log_info "Applying Kind deployment..."
        kubectl apply -k "$DEPLOYMENT_DIR/kind"
    else
        log_info "Using production deployment configuration"
        kubectl apply -f "$DEPLOYMENT_DIR/kubernetes/"
        kubectl apply -f "$DEPLOYMENT_DIR/kubernetes/hpa.yaml"
    fi
    
    log_success "Auth service deployment submitted"
    wait_for_user
}

# Verify deployment
verify_deployment() {
    log_step "7. Waiting for deployment rollout"
    
    log_info "Waiting for auth-service rollout..."
    kubectl rollout status deployment/auth-service -n $NAMESPACE --timeout=300s
    
    log_success "Rollout completed successfully!"
    
    kubectl get pods -l app=auth-service -n $NAMESPACE
    kubectl get service auth-service -n $NAMESPACE
    
    wait_for_user
}

# Test deployment
test_deployment() {
    log_step "8. Testing auth-service deployment"
    
    if [ "$KIND_MODE" = true ]; then
        local base_url="http://localhost:30080"
        log_info "Testing via NodePort..."
    else
        local base_url="http://localhost:8080"
        log_info "Setting up port forwarding..."
        kubectl port-forward service/auth-service 8080:8080 -n $NAMESPACE &
        local pf_pid=$!
        sleep 3
    fi
    
    if curl -f "$base_url/health" &>/dev/null; then
        log_success "✅ Health check passed"
    else
        log_error "❌ Health check failed"
    fi
    
    if [ -n "${pf_pid:-}" ]; then
        kill $pf_pid 2>/dev/null || true
    fi
    
    wait_for_user
}

# Show access information
show_access_info() {
    log_step "9. Deployment completed - Access information"
    
    log_success "🎉 Auth service deployed successfully!"
    
    if [ "$KIND_MODE" = true ]; then
        echo "  📡 Service URL: http://localhost:30080"
        echo "  📊 Metrics URL: http://localhost:30081"
        echo "  🔍 Health: curl http://localhost:30080/health"
        echo "  🔐 Login: POST http://localhost:30080/v1/auth/login"
    else
        echo "  🔗 Internal URL: http://auth-service:8080"
        echo "  🔍 Health: kubectl port-forward service/auth-service 8080:8080"
    fi
}

main() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --step-by-step) STEP_BY_STEP=true; shift ;;
            --analyze) ANALYZE=true; shift ;;
            --kind) KIND_MODE=true; shift ;;
            --help) 
                echo "Usage: $0 [--step-by-step] [--analyze] [--kind]"
                exit 0 ;;
            *) log_error "Unknown option: $1"; exit 1 ;;
        esac
    done
    
    echo -e "${CYAN}Auth Service Manual Deployment${NC}"
    
    check_cluster
    
    log_step "2-5. Deploying configuration, secrets, and RBAC"
    log_info "Applying base configuration..."
    kubectl apply -f "$DEPLOYMENT_DIR/kubernetes/configmap.yaml"
    kubectl apply -f "$DEPLOYMENT_DIR/kubernetes/secrets.yaml"
    kubectl apply -f "$DEPLOYMENT_DIR/kubernetes/service.yaml"
    log_success "Base components applied"
    wait_for_user
    
    deploy_application
    verify_deployment
    test_deployment
    show_access_info
    
    log_success "Manual deployment completed! 🎉"
}

main "$@" 