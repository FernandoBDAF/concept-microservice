#!/bin/bash

# Rollback Procedures Script for Auth Service
# Purpose: Emergency rollback and recovery procedures
# Usage: ./rollback-procedures.sh [--to-revision N] [--emergency]

set -euo pipefail

SERVICE_NAME="auth-service"
NAMESPACE="default"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_warning() { echo -e "${YELLOW}[WARNING]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Quick rollback to previous version
quick_rollback() {
    log_warning "Performing quick rollback to previous revision..."
    
    kubectl rollout undo deployment/auth-service -n $NAMESPACE
    
    log_info "Waiting for rollback to complete..."
    kubectl rollout status deployment/auth-service -n $NAMESPACE --timeout=300s
    
    log_success "Quick rollback completed!"
    show_status
}

# Rollback to specific revision
rollback_to_revision() {
    local revision=$1
    
    log_warning "Rolling back to revision $revision..."
    
    kubectl rollout undo deployment/auth-service --to-revision=$revision -n $NAMESPACE
    
    log_info "Waiting for rollback to complete..."
    kubectl rollout status deployment/auth-service -n $NAMESPACE --timeout=300s
    
    log_success "Rollback to revision $revision completed!"
    show_status
}

# Emergency procedures
emergency_procedures() {
    log_error "🚨 EMERGENCY PROCEDURES ACTIVATED 🚨"
    
    log_warning "1. Checking service status..."
    kubectl get pods -l app=auth-service -n $NAMESPACE
    
    log_warning "2. Checking recent events..."
    kubectl get events --sort-by=.metadata.creationTimestamp -n $NAMESPACE | tail -10
    
    log_warning "3. Scaling down to zero replicas..."
    kubectl scale deployment auth-service --replicas=0 -n $NAMESPACE
    
    log_warning "4. Waiting for all pods to terminate..."
    kubectl wait --for=delete pod -l app=auth-service -n $NAMESPACE --timeout=60s
    
    log_warning "5. Performing emergency rollback..."
    kubectl rollout undo deployment/auth-service -n $NAMESPACE
    
    log_info "6. Waiting for emergency rollback..."
    kubectl rollout status deployment/auth-service -n $NAMESPACE --timeout=300s
    
    log_success "Emergency procedures completed!"
    show_status
}

# Show current status
show_status() {
    echo -e "\n${CYAN}Current Status:${NC}"
    
    echo "Deployment status:"
    kubectl get deployment auth-service -n $NAMESPACE
    
    echo -e "\nPod status:"
    kubectl get pods -l app=auth-service -n $NAMESPACE
    
    echo -e "\nRollout history:"
    kubectl rollout history deployment/auth-service -n $NAMESPACE
    
    echo -e "\nService endpoints:"
    kubectl get endpoints auth-service -n $NAMESPACE
}

# Show usage
show_usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --to-revision N    Rollback to specific revision"
    echo "  --emergency        Emergency procedures (scale down + rollback)"
    echo "  --status           Show current deployment status"
    echo "  --help             Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0                      # Quick rollback to previous version"
    echo "  $0 --to-revision 3      # Rollback to revision 3"
    echo "  $0 --emergency          # Emergency procedures"
    echo "  $0 --status             # Show current status"
}

main() {
    local revision=""
    local emergency=false
    local status_only=false
    
    # Parse arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --to-revision)
                revision="$2"
                shift 2
                ;;
            --emergency)
                emergency=true
                shift
                ;;
            --status)
                status_only=true
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
    
    echo -e "${CYAN}Auth Service Rollback Procedures${NC}"
    
    # Check if deployment exists
    if ! kubectl get deployment auth-service -n $NAMESPACE &>/dev/null; then
        log_error "Auth service deployment not found!"
        exit 1
    fi
    
    if [ "$status_only" = true ]; then
        show_status
        exit 0
    fi
    
    if [ "$emergency" = true ]; then
        emergency_procedures
    elif [ -n "$revision" ]; then
        rollback_to_revision "$revision"
    else
        quick_rollback
    fi
    
    log_success "Rollback procedures completed! 🎉"
}

main "$@" 