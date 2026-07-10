#!/bin/bash

# Manual Cleanup Script for Auth Service
# Purpose: Step-by-step cleanup with educational guidance
# Usage: ./manual-cleanup.sh [--step-by-step] [--preserve-secrets]

set -euo pipefail

SERVICE_NAME="auth-service"
NAMESPACE="default"
STEP_BY_STEP=${STEP_BY_STEP:-false}
PRESERVE_SECRETS=${PRESERVE_SECRETS:-false}

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

# Confirm cleanup
confirm_cleanup() {
    log_step "1. Confirming cleanup operation"
    
    log_warning "This will remove all auth-service resources from the cluster"
    
    if [ "$PRESERVE_SECRETS" = true ]; then
        log_info "Secrets will be preserved (--preserve-secrets flag used)"
    else
        log_warning "Secrets will be removed (use --preserve-secrets to keep them)"
    fi
    
    echo -e "\n${YELLOW}Resources to be removed:${NC}"
    echo "  - Deployment: auth-service"
    echo "  - Service: auth-service"
    echo "  - ConfigMap: auth-service-config"
    echo "  - ServiceAccount: auth-service"
    echo "  - RBAC: Role and RoleBinding"
    echo "  - NetworkPolicy: auth-service-network-policy"
    echo "  - HPA: auth-service-hpa (if exists)"
    if [ "$PRESERVE_SECRETS" = false ]; then
        echo "  - Secret: auth-service-secrets"
    fi
    
    if [ "$STEP_BY_STEP" = false ]; then
        read -p "Are you sure you want to proceed? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            log_info "Cleanup cancelled"
            exit 0
        fi
    fi
    
    wait_for_user
}

# Remove HPA
remove_hpa() {
    log_step "2. Removing Horizontal Pod Autoscaler"
    
    if kubectl get hpa auth-service-hpa -n $NAMESPACE &>/dev/null; then
        log_info "Removing HPA..."
        kubectl delete hpa auth-service-hpa -n $NAMESPACE
        log_success "HPA removed"
    else
        log_info "HPA not found - skipping"
    fi
    
    wait_for_user
}

# Remove deployment
remove_deployment() {
    log_step "3. Removing deployment"
    
    if kubectl get deployment auth-service -n $NAMESPACE &>/dev/null; then
        log_info "Scaling down deployment..."
        kubectl scale deployment auth-service --replicas=0 -n $NAMESPACE
        
        log_info "Waiting for pods to terminate..."
        kubectl wait --for=delete pod -l app=auth-service -n $NAMESPACE --timeout=60s
        
        log_info "Removing deployment..."
        kubectl delete deployment auth-service -n $NAMESPACE
        log_success "Deployment removed"
    else
        log_info "Deployment not found - skipping"
    fi
    
    wait_for_user
}

# Remove service and networking
remove_service_networking() {
    log_step "4. Removing service and networking"
    
    # Remove service
    if kubectl get service auth-service -n $NAMESPACE &>/dev/null; then
        log_info "Removing service..."
        kubectl delete service auth-service -n $NAMESPACE
        log_success "Service removed"
    else
        log_info "Service not found - skipping"
    fi
    
    # Remove network policy
    if kubectl get networkpolicy auth-service-network-policy -n $NAMESPACE &>/dev/null; then
        log_info "Removing network policy..."
        kubectl delete networkpolicy auth-service-network-policy -n $NAMESPACE
        log_success "Network policy removed"
    else
        log_info "Network policy not found - skipping"
    fi
    
    wait_for_user
}

# Remove RBAC
remove_rbac() {
    log_step "5. Removing RBAC resources"
    
    # Remove RoleBinding
    if kubectl get rolebinding auth-service -n $NAMESPACE &>/dev/null; then
        log_info "Removing RoleBinding..."
        kubectl delete rolebinding auth-service -n $NAMESPACE
        log_success "RoleBinding removed"
    else
        log_info "RoleBinding not found - skipping"
    fi
    
    # Remove Role
    if kubectl get role auth-service -n $NAMESPACE &>/dev/null; then
        log_info "Removing Role..."
        kubectl delete role auth-service -n $NAMESPACE
        log_success "Role removed"
    else
        log_info "Role not found - skipping"
    fi
    
    # Remove ServiceAccount
    if kubectl get serviceaccount auth-service -n $NAMESPACE &>/dev/null; then
        log_info "Removing ServiceAccount..."
        kubectl delete serviceaccount auth-service -n $NAMESPACE
        log_success "ServiceAccount removed"
    else
        log_info "ServiceAccount not found - skipping"
    fi
    
    wait_for_user
}

# Remove configuration
remove_configuration() {
    log_step "6. Removing configuration"
    
    # Remove ConfigMap
    if kubectl get configmap auth-service-config -n $NAMESPACE &>/dev/null; then
        log_info "Removing ConfigMap..."
        kubectl delete configmap auth-service-config -n $NAMESPACE
        log_success "ConfigMap removed"
    else
        log_info "ConfigMap not found - skipping"
    fi
    
    # Remove Kind-specific ConfigMaps
    if kubectl get configmap auth-service-local-config -n $NAMESPACE &>/dev/null; then
        log_info "Removing Kind ConfigMap..."
        kubectl delete configmap auth-service-local-config -n $NAMESPACE
        log_success "Kind ConfigMap removed"
    fi
    
    wait_for_user
}

# Remove secrets
remove_secrets() {
    log_step "7. Removing secrets"
    
    if [ "$PRESERVE_SECRETS" = true ]; then
        log_info "Preserving secrets (--preserve-secrets flag used)"
        log_warning "Secrets preserved:"
        kubectl get secrets -l app=auth-service -n $NAMESPACE 2>/dev/null || log_info "No auth-service secrets found"
        return
    fi
    
    # Remove main secrets
    if kubectl get secret auth-service-secrets -n $NAMESPACE &>/dev/null; then
        log_warning "Removing production secrets..."
        kubectl delete secret auth-service-secrets -n $NAMESPACE
        log_success "Production secrets removed"
    else
        log_info "Production secrets not found - skipping"
    fi
    
    # Remove Kind-specific secrets
    if kubectl get secret auth-service-secrets-local -n $NAMESPACE &>/dev/null; then
        log_info "Removing Kind secrets..."
        kubectl delete secret auth-service-secrets-local -n $NAMESPACE
        log_success "Kind secrets removed"
    fi
    
    # Remove template secrets
    if kubectl get secret auth-service-secrets-template -n $NAMESPACE &>/dev/null; then
        log_info "Removing template secrets..."
        kubectl delete secret auth-service-secrets-template -n $NAMESPACE
        log_success "Template secrets removed"
    fi
    
    wait_for_user
}

# Verify cleanup
verify_cleanup() {
    log_step "8. Verifying cleanup completion"
    
    log_info "Checking for remaining auth-service resources..."
    
    local remaining_resources=0
    
    # Check each resource type
    if kubectl get deployment auth-service -n $NAMESPACE &>/dev/null; then
        log_warning "❌ Deployment still exists"
        remaining_resources=$((remaining_resources + 1))
    else
        log_success "✅ Deployment removed"
    fi
    
    if kubectl get service auth-service -n $NAMESPACE &>/dev/null; then
        log_warning "❌ Service still exists"
        remaining_resources=$((remaining_resources + 1))
    else
        log_success "✅ Service removed"
    fi
    
    if [ "$PRESERVE_SECRETS" = false ]; then
        if kubectl get secret auth-service-secrets -n $NAMESPACE &>/dev/null; then
            log_warning "❌ Secrets still exist"
            remaining_resources=$((remaining_resources + 1))
        else
            log_success "✅ Secrets removed"
        fi
    else
        log_info "ℹ️  Secrets preserved as requested"
    fi
    
    # Check for any remaining pods
    local remaining_pods
    remaining_pods=$(kubectl get pods -l app=auth-service -n $NAMESPACE --no-headers 2>/dev/null | wc -l)
    if [ "$remaining_pods" -gt 0 ]; then
        log_warning "❌ $remaining_pods pods still exist"
        kubectl get pods -l app=auth-service -n $NAMESPACE
        remaining_resources=$((remaining_resources + 1))
    else
        log_success "✅ All pods removed"
    fi
    
    if [ $remaining_resources -eq 0 ]; then
        log_success "🎉 Cleanup completed successfully!"
    else
        log_warning "⚠️  $remaining_resources resources still exist"
        log_info "You may need to manually remove remaining resources"
    fi
    
    wait_for_user
}

# Show cleanup summary
show_cleanup_summary() {
    log_step "9. Cleanup summary"
    
    echo -e "\n${CYAN}Cleanup Summary:${NC}"
    echo "  🗑️  Auth service deployment removed"
    echo "  🗑️  Service and networking removed"
    echo "  🗑️  RBAC resources removed"
    echo "  🗑️  Configuration removed"
    
    if [ "$PRESERVE_SECRETS" = true ]; then
        echo "  💾 Secrets preserved"
        echo ""
        echo -e "${YELLOW}Note:${NC} Secrets were preserved. To remove them later:"
        echo "  kubectl delete secret auth-service-secrets -n $NAMESPACE"
    else
        echo "  🗑️  Secrets removed"
    fi
    
    echo ""
    echo -e "${CYAN}To redeploy auth-service:${NC}"
    echo "  ./manual-deploy.sh"
    echo ""
    echo -e "${CYAN}To check for any remaining resources:${NC}"
    echo "  kubectl get all -l app=auth-service -n $NAMESPACE"
}

main() {
    # Parse arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --step-by-step)
                STEP_BY_STEP=true
                shift
                ;;
            --preserve-secrets)
                PRESERVE_SECRETS=true
                shift
                ;;
            --help)
                echo "Usage: $0 [--step-by-step] [--preserve-secrets]"
                echo ""
                echo "Options:"
                echo "  --step-by-step      Pause between each step"
                echo "  --preserve-secrets  Keep secrets for redeployment"
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                exit 1
                ;;
        esac
    done
    
    echo -e "${CYAN}Auth Service Manual Cleanup${NC}"
    
    # Execute cleanup steps
    confirm_cleanup
    remove_hpa
    remove_deployment
    remove_service_networking
    remove_rbac
    remove_configuration
    remove_secrets
    verify_cleanup
    show_cleanup_summary
    
    log_success "Manual cleanup completed! 🎉"
}

main "$@" 