#!/bin/bash

# Manual Deployment Script for Profile Service
# Purpose: Step-by-step deployment for analysis and learning
# Usage: ./manual-deploy.sh [--analyze] [--step-by-step]

set -euo pipefail

# Configuration
SERVICE_NAME="profile-service"
NAMESPACE="default"
STEP_BY_STEP=${STEP_BY_STEP:-false}

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

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

log_step() {
    echo -e "${CYAN}[STEP]${NC} $1"
}

wait_for_user() {
    if [[ "$STEP_BY_STEP" == "true" ]]; then
        echo -e "${YELLOW}Press Enter to continue to next step...${NC}"
        read -r
    fi
}

analyze_manifest() {
    local file=$1
    local description=$2

    log_step "Analyzing: $file"
    echo -e "${CYAN}Description:${NC} $description"
    echo -e "${CYAN}Contents Preview:${NC}"
    echo "----------------------------------------"
    head -20 "$file" | sed 's/^/  /'
    echo "----------------------------------------"
    echo -e "${CYAN}Resource Count:${NC} $(grep -c '^---' "$file" 2>/dev/null || echo "1") resources"
    echo
}

deploy_manifest() {
    local file=$1
    local description=$2

    log_step "Deploying: $file"
    echo -e "${CYAN}Description:${NC} $description"

    if [[ "${1:-}" == "--analyze" ]] || [[ "$STEP_BY_STEP" == "true" && "${1:-}" == "--analyze" ]]; then
        analyze_manifest "$file" "$description"
        wait_for_user
    fi

    log_info "Applying manifest: $file"
    kubectl apply -f "$file"

    log_success "Successfully applied: $file"
    echo

    wait_for_user
}

verify_deployment() {
    local resource_type=$1
    local resource_name=$2
    local description=$3

    log_step "Verifying: $resource_type/$resource_name"
    echo -e "${CYAN}Description:${NC} $description"

    log_info "Checking $resource_type status..."
    kubectl get "$resource_type" "$resource_name" -o wide

    if [[ "$resource_type" == "deployment" ]]; then
        log_info "Checking rollout status..."
        kubectl rollout status "deployment/$resource_name" --timeout=60s
    fi

    echo
    wait_for_user
}

main() {
    log_info "Starting manual deployment of $SERVICE_NAME"
    log_info "Deployment mode: ${1:-normal} (use --analyze for detailed analysis)"
    echo

    # Parse arguments
    if [[ "${1:-}" == "--analyze" ]] || [[ "${1:-}" == "--step-by-step" ]]; then
        STEP_BY_STEP=true
        log_warning "Step-by-step mode enabled. You will be prompted between each step."
        echo
    fi

    # Step 1: Deploy Secrets (Foundation)
    deploy_manifest "../kubernetes/secrets.yaml" \
        "Secrets and credentials required by the profile service"

    verify_deployment "secret" "profile-service-secrets" \
        "Production secrets for authentication and configuration"

    verify_deployment "secret" "profile-service-secrets-local" \
        "Development secrets for local testing"

    # Step 2: Deploy ConfigMap (Configuration)
    deploy_manifest "../kubernetes/configmap.yaml" \
        "Service configuration including routing keys and task types"

    verify_deployment "configmap" "profile-service-config" \
        "Main service configuration data"

    # Step 3: Deploy Service & RBAC (Network & Security)
    deploy_manifest "../kubernetes/service.yaml" \
        "Service definition, RBAC, HPA, PDB, and network policies"

    verify_deployment "service" "profile-service" \
        "Service network endpoint and load balancing"

    verify_deployment "serviceaccount" "profile-service" \
        "Service account for RBAC and security"

    verify_deployment "hpa" "profile-service-hpa" \
        "Horizontal Pod Autoscaler for automatic scaling"

    # Step 4: Deploy Development Dependencies (Kind-specific)
    if kubectl get nodes | grep -q "kind"; then
        log_info "Kind cluster detected, deploying development dependencies..."

        if [[ -f "../kind/profile-dependencies.yaml" ]]; then
            deploy_manifest "../kind/profile-dependencies.yaml" \
                "Development Redis service for session management"
        elif [[ -f "../kind/redis-service.yaml" ]]; then
            deploy_manifest "../kind/redis-service.yaml" \
                "Development Redis service for session management"
        fi

        verify_deployment "service" "redis-service" \
            "Development Redis service"
    fi

    # Step 5: Deploy Application (Core Service)
    if kubectl get nodes | grep -q "kind"; then
        log_info "Kind cluster detected - deploying with Kind-optimized settings..."
        log_info "Using kustomized deployment (1 replica, reduced resources, local secrets)"
        
        # Apply the full Kind kustomization (this includes dependencies we may have already deployed, but kubectl will handle that)
        kubectl apply -k ../kind/
        
        log_success "✅ Kind-optimized deployment applied:"
        log_info "  • 1 replica (instead of 3)"
        log_info "  • Reduced resource requirements"
        log_info "  • Local development secrets"
        log_info "  • Debug logging enabled"
    else
        log_info "Production cluster detected - deploying with production settings..."
        deploy_manifest "../kubernetes/deployment.yaml" \
            "Production profile service deployment (3 replicas, production resources)"
    fi

    verify_deployment "deployment" "profile-service" \
        "Main profile service application"

    # Step 6: Deploy Monitoring (Observability)
    if [[ -f "../monitoring/servicemonitor.yaml" ]]; then
        if kubectl get crd servicemonitors.monitoring.coreos.com &>/dev/null; then
            deploy_manifest "../monitoring/servicemonitor.yaml" \
                "Prometheus ServiceMonitor for metrics collection"
        else
            log_warning "Prometheus Operator not detected, skipping ServiceMonitor"
            log_info "Deploying basic monitoring configmap instead..."
            if [[ -f "../kind/monitoring-configmap.yaml" ]]; then
                deploy_manifest "../kind/monitoring-configmap.yaml" \
                    "Basic monitoring configuration for development"
            fi
        fi
    fi

    # Final Verification
    log_step "Final Deployment Verification"
    echo
    log_info "All deployed resources:"
    kubectl get all -l app="profile-service"
    echo

    log_info "Service endpoints:"
    kubectl get svc "profile-service" -o wide
    echo

    log_info "Pod status and logs:"
    kubectl get pods -l app="profile-service"
    echo

    # Health check
    log_info "Testing service health..."
    kubectl port-forward service/profile-service 8080:8080 &
    PF_PID=$!
    sleep 3

    if curl -f http://localhost:8080/health &>/dev/null; then
        log_success "✅ Health check passed"
    else
        log_warning "❌ Health check failed - check pod logs"
    fi

    kill $PF_PID 2>/dev/null || true

    log_success "Manual deployment of $SERVICE_NAME completed successfully!"
    log_info "Use 'kubectl logs -l app=profile-service --tail=50 -f' to view logs"
    log_info "Use './manual-cleanup.sh' to remove all deployed resources"
    log_info "Use '../kind/deploy-to-kind.sh' for automated kustomize deployment"
}

# Help function
show_help() {
    echo "Manual Deployment Script for Profile Service"
    echo
    echo "Usage: $0 [OPTIONS]"
    echo
    echo "Options:"
    echo "  --analyze        Enable detailed manifest analysis with previews"
    echo "  --step-by-step   Enable step-by-step mode with user prompts"
    echo "  --help          Show this help message"
    echo
    echo "Examples:"
    echo "  $0                    # Normal manual deployment"
    echo "  $0 --analyze          # Deployment with manifest analysis"
    echo "  $0 --step-by-step     # Interactive step-by-step deployment"
    echo
    echo "This script provides manual deployment for learning and analysis."
    echo "For regular operations, use: kubectl apply -k ../kind/"
    echo
}

# Main execution
case "${1:-}" in
    "--help")
        show_help
        ;;
    *)
        main "$@"
        ;;
esac 