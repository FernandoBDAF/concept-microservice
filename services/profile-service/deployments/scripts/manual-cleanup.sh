#!/bin/bash

# Manual Cleanup Script for Profile Service
# Purpose: Step-by-step cleanup for analysis and learning
# Usage: ./manual-cleanup.sh [--analyze] [--step-by-step]

set -euo pipefail

SERVICE_NAME="profile-service"
STEP_BY_STEP=${STEP_BY_STEP:-false}

# Colors (same as deploy script)
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_step() {
    echo -e "${CYAN}[STEP]${NC} $1"
}

wait_for_user() {
    if [[ "$STEP_BY_STEP" == "true" ]]; then
        echo -e "${YELLOW}Press Enter to continue to next step...${NC}"
        read -r
    fi
}

cleanup_resource() {
    local resource_type=$1
    local resource_name=$2
    local description=$3

    log_step "Cleaning up: $resource_type/$resource_name"
    echo -e "${CYAN}Description:${NC} $description"

    if kubectl get "$resource_type" "$resource_name" &>/dev/null; then
        log_info "Deleting $resource_type/$resource_name..."
        kubectl delete "$resource_type" "$resource_name"
        log_success "Deleted: $resource_type/$resource_name"
    else
        log_warning "$resource_type/$resource_name not found (already deleted or never existed)"
    fi

    echo
    wait_for_user
}

main() {
    log_info "Starting manual cleanup of $SERVICE_NAME"
    echo

    # Parse arguments
    if [[ "${1:-}" == "--analyze" ]] || [[ "${1:-}" == "--step-by-step" ]]; then
        STEP_BY_STEP=true
        log_warning "Step-by-step mode enabled. You will be prompted between each step."
        echo
    fi

    # Show current resources before cleanup
    log_step "Current Resources Analysis"
    log_info "Resources that will be deleted:"
    kubectl get all,configmap,secret,serviceaccount,clusterrole,clusterrolebinding,hpa,pdb,networkpolicy -l app="profile-service" 2>/dev/null || true
    echo
    wait_for_user

    # Reverse order cleanup (opposite of deployment)

    # Step 1: Cleanup Monitoring
    cleanup_resource "servicemonitor" "profile-service-monitor" \
        "Prometheus monitoring configuration"

    cleanup_resource "prometheusrule" "profile-service-alerts" \
        "Prometheus alert rules"

    cleanup_resource "configmap" "profile-service-grafana-dashboard" \
        "Grafana dashboard configuration"

    cleanup_resource "configmap" "profile-service-monitoring" \
        "Basic monitoring configuration"

    # Step 2: Cleanup Development Dependencies
    if kubectl get nodes | grep -q "kind"; then
        cleanup_resource "deployment" "redis-service" \
            "Development Redis dependency"
        cleanup_resource "service" "redis-service" \
            "Development Redis service"
    fi

    # Step 3: Cleanup Main Application
    cleanup_resource "deployment" "profile-service" \
        "Main profile service deployment"

    # Step 4: Cleanup Scaling and Policies
    cleanup_resource "hpa" "profile-service-hpa" \
        "Horizontal Pod Autoscaler"

    cleanup_resource "pdb" "profile-service-pdb" \
        "Pod Disruption Budget"

    cleanup_resource "networkpolicy" "profile-service-netpol" \
        "Network security policy"

    # Step 5: Cleanup Network & Security
    cleanup_resource "service" "profile-service" \
        "Service network endpoint"

    cleanup_resource "clusterrolebinding" "profile-service-binding" \
        "Cluster role binding for RBAC"

    cleanup_resource "clusterrole" "profile-service-role" \
        "Cluster role for RBAC"

    cleanup_resource "serviceaccount" "profile-service" \
        "Service account"

    # Step 6: Cleanup Configuration
    cleanup_resource "configmap" "profile-service-config" \
        "Main service configuration"

    cleanup_resource "configmap" "profile-service-routing-config" \
        "Routing configuration for multi-worker support"

    # Step 7: Cleanup Secrets (Last - most sensitive)
    cleanup_resource "secret" "profile-service-secrets" \
        "Production secrets"

    cleanup_resource "secret" "profile-service-secrets-local" \
        "Development secrets"

    cleanup_resource "secret" "redis-secret" \
        "Redis authentication secret"

    # Final verification
    log_step "Final Cleanup Verification"
    echo
    log_info "Remaining resources (should be empty):"
    kubectl get all,configmap,secret,serviceaccount,clusterrole,clusterrolebinding,hpa,pdb,networkpolicy -l app="profile-service" 2>/dev/null || log_success "No resources found - cleanup complete!"
    echo

    log_success "Manual cleanup of $SERVICE_NAME completed successfully!"
    log_info "All profile-service resources have been removed from the cluster"
}

# Help function
show_help() {
    echo "Manual Cleanup Script for Profile Service"
    echo
    echo "Usage: $0 [OPTIONS]"
    echo
    echo "Options:"
    echo "  --analyze        Enable detailed resource analysis during cleanup"
    echo "  --step-by-step   Enable step-by-step mode with user prompts"
    echo "  --help          Show this help message"
    echo
    echo "Examples:"
    echo "  $0                    # Normal manual cleanup"
    echo "  $0 --analyze          # Cleanup with resource analysis"
    echo "  $0 --step-by-step     # Interactive step-by-step cleanup"
    echo
    echo "This script provides step-by-step cleanup for learning and analysis."
    echo "For quick cleanup, use: kubectl delete -k ../kind/"
    echo
}

# Parse arguments and execute
case "${1:-}" in
    "--help")
        show_help
        ;;
    *)
        main "$@"
        ;;
esac 