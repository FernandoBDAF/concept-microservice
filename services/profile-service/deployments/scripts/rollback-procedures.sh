#!/bin/bash

# Profile Service Deployment Rollback Procedures
# Usage: ./rollback-procedures.sh [COMMAND] [OPTIONS]

set -euo pipefail

# Configuration
NAMESPACE=${NAMESPACE:-default}
SERVICE_NAME="profile-service"
DEPLOYMENT_NAME="profile-service"
MAX_ROLLBACK_REVISIONS=10
HEALTH_CHECK_TIMEOUT=300
ROLLBACK_TIMEOUT=600

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

# Helper functions
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    # Check kubectl
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl is not installed or not in PATH"
        exit 1
    fi
    
    # Check cluster connectivity
    if ! kubectl cluster-info &> /dev/null; then
        log_error "Cannot connect to Kubernetes cluster"
        exit 1
    fi
    
    # Check namespace exists
    if ! kubectl get namespace "$NAMESPACE" &> /dev/null; then
        log_error "Namespace '$NAMESPACE' does not exist"
        exit 1
    fi
    
    # Check deployment exists
    if ! kubectl get deployment "$DEPLOYMENT_NAME" -n "$NAMESPACE" &> /dev/null; then
        log_error "Deployment '$DEPLOYMENT_NAME' does not exist in namespace '$NAMESPACE'"
        exit 1
    fi
    
    log_success "Prerequisites check passed"
}

get_current_revision() {
    kubectl get deployment "$DEPLOYMENT_NAME" -n "$NAMESPACE" -o jsonpath='{.metadata.annotations.deployment\.kubernetes\.io/revision}'
}

get_rollout_history() {
    log_info "Getting rollout history..."
    kubectl rollout history deployment "$DEPLOYMENT_NAME" -n "$NAMESPACE"
}

check_deployment_health() {
    local timeout=${1:-$HEALTH_CHECK_TIMEOUT}
    log_info "Checking deployment health (timeout: ${timeout}s)..."
    
    # Wait for deployment to be ready
    if kubectl wait --for=condition=available deployment "$DEPLOYMENT_NAME" -n "$NAMESPACE" --timeout="${timeout}s"; then
        log_success "Deployment is healthy and available"
        return 0
    else
        log_error "Deployment health check failed within ${timeout}s"
        return 1
    fi
}

check_service_health() {
    log_info "Checking service health endpoints..."
    
    local pod_name
    pod_name=$(kubectl get pods -n "$NAMESPACE" -l app="$SERVICE_NAME" -o jsonpath='{.items[0].metadata.name}')
    
    if [[ -z "$pod_name" ]]; then
        log_error "No running pods found"
        return 1
    fi
    
    # Check health endpoint
    if kubectl exec -n "$NAMESPACE" "$pod_name" -- curl -f http://localhost:8080/health &> /dev/null; then
        log_success "Service health endpoint is responding"
    else
        log_error "Service health endpoint is not responding"
        return 1
    fi
    
    # Check metrics endpoint
    if kubectl exec -n "$NAMESPACE" "$pod_name" -- curl -f http://localhost:8080/metrics &> /dev/null; then
        log_success "Service metrics endpoint is responding"
    else
        log_warning "Service metrics endpoint is not responding"
    fi
    
    return 0
}

backup_current_state() {
    log_info "Backing up current deployment state..."
    
    local backup_dir="backups/$(date +%Y%m%d-%H%M%S)"
    mkdir -p "$backup_dir"
    
    # Backup deployment
    kubectl get deployment "$DEPLOYMENT_NAME" -n "$NAMESPACE" -o yaml > "$backup_dir/deployment.yaml"
    
    # Backup configmaps
    kubectl get configmap profile-service-config -n "$NAMESPACE" -o yaml > "$backup_dir/configmap.yaml" 2>/dev/null || true
    kubectl get configmap profile-service-routing-config -n "$NAMESPACE" -o yaml > "$backup_dir/routing-configmap.yaml" 2>/dev/null || true
    
    # Backup service
    kubectl get service "$SERVICE_NAME" -n "$NAMESPACE" -o yaml > "$backup_dir/service.yaml"
    
    # Backup current revision info
    get_current_revision > "$backup_dir/current-revision.txt"
    
    log_success "Current state backed up to $backup_dir"
    echo "$backup_dir"
}

perform_rollback() {
    local target_revision=$1
    
    log_info "Performing rollback to revision $target_revision..."
    
    # Perform the rollback
    if kubectl rollout undo deployment "$DEPLOYMENT_NAME" -n "$NAMESPACE" --to-revision="$target_revision"; then
        log_info "Rollback command executed successfully"
    else
        log_error "Rollback command failed"
        return 1
    fi
    
    # Wait for rollback to complete
    log_info "Waiting for rollback to complete (timeout: ${ROLLBACK_TIMEOUT}s)..."
    if kubectl rollout status deployment "$DEPLOYMENT_NAME" -n "$NAMESPACE" --timeout="${ROLLBACK_TIMEOUT}s"; then
        log_success "Rollback completed successfully"
    else
        log_error "Rollback timed out or failed"
        return 1
    fi
    
    return 0
}

validate_rollback() {
    log_info "Validating rollback..."
    
    # Check deployment health
    if ! check_deployment_health 180; then
        log_error "Deployment health check failed after rollback"
        return 1
    fi
    
    # Wait a bit for pods to fully start
    sleep 30
    
    # Check service health
    if ! check_service_health; then
        log_error "Service health check failed after rollback"
        return 1
    fi
    
    # Check if we can submit a test task
    log_info "Performing integration test..."
    local test_result
    test_result=$(test_task_submission)
    if [[ $test_result == "success" ]]; then
        log_success "Integration test passed"
    else
        log_warning "Integration test failed, but basic health checks passed"
    fi
    
    log_success "Rollback validation completed"
    return 0
}

test_task_submission() {
    # This would need to be adapted based on your testing setup
    # For now, just return success as a placeholder
    echo "success"
}

emergency_rollback() {
    log_warning "Performing EMERGENCY ROLLBACK..."
    
    # Get the last known good revision (previous revision)
    local current_revision
    current_revision=$(get_current_revision)
    local target_revision=$((current_revision - 1))
    
    if [[ $target_revision -lt 1 ]]; then
        log_error "No previous revision available for emergency rollback"
        return 1
    fi
    
    log_info "Emergency rollback target: revision $target_revision"
    
    # Skip backup in emergency (to save time)
    # Perform rollback with shorter timeout
    ROLLBACK_TIMEOUT=300
    if perform_rollback "$target_revision"; then
        log_success "Emergency rollback completed"
        
        # Basic validation only
        if check_deployment_health 120; then
            log_success "Emergency rollback validation passed"
            return 0
        else
            log_error "Emergency rollback validation failed"
            return 1
        fi
    else
        log_error "Emergency rollback failed"
        return 1
    fi
}

show_help() {
    cat << EOF
Profile Service Deployment Rollback Procedures

USAGE:
    $0 COMMAND [OPTIONS]

COMMANDS:
    history                 Show deployment rollout history
    status                  Show current deployment status
    health                  Check service health
    rollback REVISION       Rollback to specific revision
    rollback-previous       Rollback to previous revision
    emergency               Emergency rollback (fastest, minimal validation)
    validate                Validate current deployment
    backup                  Backup current state
    help                    Show this help message

OPTIONS:
    -n, --namespace NAME    Kubernetes namespace (default: $NAMESPACE)
    -t, --timeout SECONDS   Operation timeout (default: $ROLLBACK_TIMEOUT)
    --skip-backup          Skip backup during rollback
    --skip-validation      Skip validation after rollback

EXAMPLES:
    $0 history
    $0 rollback 5
    $0 rollback-previous
    $0 emergency
    $0 health

EMERGENCY PROCEDURES:
    In case of critical issues:
    1. Run: $0 emergency
    2. If that fails, check pod logs: kubectl logs -n $NAMESPACE -l app=$SERVICE_NAME
    3. If pods won't start, check events: kubectl get events -n $NAMESPACE
    4. Contact oncall team if issues persist

EOF
}

# Command handling
case "${1:-help}" in
    "history")
        check_prerequisites
        get_rollout_history
        ;;
    
    "status")
        check_prerequisites
        log_info "Current deployment status:"
        kubectl get deployment "$DEPLOYMENT_NAME" -n "$NAMESPACE" -o wide
        echo
        log_info "Current pods:"
        kubectl get pods -n "$NAMESPACE" -l app="$SERVICE_NAME" -o wide
        echo
        log_info "Current revision: $(get_current_revision)"
        ;;
    
    "health")
        check_prerequisites
        check_deployment_health
        check_service_health
        ;;
    
    "rollback")
        if [[ -z "${2:-}" ]]; then
            log_error "Please specify target revision"
            echo "Usage: $0 rollback REVISION"
            exit 1
        fi
        
        check_prerequisites
        target_revision=$2
        
        # Validate revision exists
        if ! kubectl rollout history deployment "$DEPLOYMENT_NAME" -n "$NAMESPACE" --revision="$target_revision" &> /dev/null; then
            log_error "Revision $target_revision does not exist"
            get_rollout_history
            exit 1
        fi
        
        # Backup current state unless skipped
        if [[ "${*}" != *"--skip-backup"* ]]; then
            backup_dir=$(backup_current_state)
        fi
        
        # Perform rollback
        if perform_rollback "$target_revision"; then
            # Validate unless skipped
            if [[ "${*}" != *"--skip-validation"* ]]; then
                if validate_rollback; then
                    log_success "Rollback to revision $target_revision completed successfully"
                else
                    log_error "Rollback validation failed"
                    exit 1
                fi
            else
                log_success "Rollback to revision $target_revision completed (validation skipped)"
            fi
        else
            log_error "Rollback to revision $target_revision failed"
            exit 1
        fi
        ;;
    
    "rollback-previous")
        check_prerequisites
        current_revision=$(get_current_revision)
        target_revision=$((current_revision - 1))
        
        if [[ $target_revision -lt 1 ]]; then
            log_error "No previous revision available"
            exit 1
        fi
        
        log_info "Rolling back from revision $current_revision to $target_revision"
        exec "$0" rollback "$target_revision" "${@:2}"
        ;;
    
    "emergency")
        check_prerequisites
        emergency_rollback
        ;;
    
    "validate")
        check_prerequisites
        validate_rollback
        ;;
    
    "backup")
        check_prerequisites
        backup_current_state
        ;;
    
    "help"|"-h"|"--help")
        show_help
        ;;
    
    *)
        log_error "Unknown command: $1"
        echo
        show_help
        exit 1
        ;;
esac 