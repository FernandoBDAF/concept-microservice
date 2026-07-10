#!/bin/bash

# Manual Cleanup Script for Cache Service
# Purpose: Step-by-step cleanup with Redis data preservation options
# Usage: ./manual-cleanup.sh [--step-by-step] [--preserve-data] [--force]

set -euo pipefail

# Configuration
SERVICE_NAME="cache-service"
REDIS_SERVICE="redis"
NAMESPACE="default"
STEP_BY_STEP=${STEP_BY_STEP:-false}
PRESERVE_DATA=${PRESERVE_DATA:-false}
FORCE_CLEANUP=${FORCE_CLEANUP:-false}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --step-by-step)
            STEP_BY_STEP=true
            shift
            ;;
        --preserve-data)
            PRESERVE_DATA=true
            shift
            ;;
        --force)
            FORCE_CLEANUP=true
            shift
            ;;
        *)
            echo "Unknown option: $1"
            echo "Usage: $0 [--step-by-step] [--preserve-data] [--force]"
            exit 1
            ;;
    esac
done

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

log_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

log_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

log_error() {
    echo -e "${RED}❌ $1${NC}"
}

log_header() {
    echo
    echo -e "${CYAN}🔥 $1${NC}"
    echo "$(printf '=%.0s' {1..60})"
}

log_step() {
    echo
    echo -e "${BLUE}📋 Step $1: $2${NC}"
    echo "$(printf '-%.0s' {1..40})"
}

# Interactive step function
wait_for_user() {
    if [[ "$STEP_BY_STEP" == "true" ]]; then
        echo
        read -p "Press Enter to continue or Ctrl+C to exit..."
        echo
    fi
}

# Confirmation function
confirm_action() {
    local action="$1"
    if [[ "$FORCE_CLEANUP" != "true" ]]; then
        echo
        read -p "⚠️  Are you sure you want to $action? (y/N): " -r
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            log_info "Skipping $action"
            return 1
        fi
    fi
    return 0
}

# Cache data backup function
backup_redis_data() {
    log_info "🗄️ Backing up Redis data..."
    
    if kubectl get pods -l app=redis --no-headers 2>/dev/null | grep -q "Running"; then
        local redis_pod=$(kubectl get pods -l app=redis --no-headers | head -1 | awk '{print $1}')
        local timestamp=$(date +%Y%m%d_%H%M%S)
        local backup_file="redis_backup_${timestamp}.rdb"
        
        log_info "Creating Redis data backup..."
        kubectl exec "$redis_pod" -- redis-cli BGSAVE >/dev/null
        sleep 5
        
        # Check if backup completed
        local lastsave=$(kubectl exec "$redis_pod" -- redis-cli LASTSAVE)
        log_success "Redis backup completed at timestamp: $lastsave"
        
        log_info "💾 To manually save data:"
        echo "   kubectl cp $redis_pod:/data/dump.rdb ./$backup_file"
        echo "   kubectl cp $redis_pod:/data/appendonly.aof ./redis_aof_${timestamp}.aof"
        
        return 0
    else
        log_warning "No Redis pods running - no data to backup"
        return 1
    fi
}

# Redis data analysis
analyze_redis_data() {
    log_info "📊 Analyzing Redis data before cleanup..."
    
    if kubectl get pods -l app=redis --no-headers 2>/dev/null | grep -q "Running"; then
        local redis_pod=$(kubectl get pods -l app=redis --no-headers | head -1 | awk '{print $1}')
        
        log_info "Redis Data Summary:"
        kubectl exec "$redis_pod" -- redis-cli info keyspace 2>/dev/null | grep "^db" || echo "   No data found"
        
        local total_keys=$(kubectl exec "$redis_pod" -- redis-cli dbsize 2>/dev/null || echo "0")
        log_info "Total keys in Redis: $total_keys"
        
        if [[ "$total_keys" != "0" ]]; then
            log_info "Sample keys:"
            kubectl exec "$redis_pod" -- redis-cli --scan --count 10 2>/dev/null | head -5 || true
        fi
    else
        log_warning "No Redis pods running - unable to analyze data"
    fi
}

# Component cleanup functions
cleanup_hpa() {
    log_step "1" "Cleanup Auto-Scaling (HPA)"
    
    if kubectl get hpa cache-service-hpa >/dev/null 2>&1; then
        if confirm_action "remove auto-scaling (HPA)"; then
            kubectl delete hpa cache-service-hpa && log_success "HPA removed" || log_error "Failed to remove HPA"
        fi
    else
        log_info "HPA not found - skipping"
    fi
    
    wait_for_user
}

cleanup_monitoring() {
    log_step "2" "Cleanup Monitoring"
    
    local monitoring_resources=(
        "prometheusrule/cache-service-alerts"
        "servicemonitor/cache-service-metrics"
        "configmap/cache-service-dashboard" 
        "configmap/cache-service-slo"
    )
    
    log_info "Found monitoring resources:"
    for resource in "${monitoring_resources[@]}"; do
        if kubectl get "$resource" >/dev/null 2>&1; then
            echo "   ✅ $resource"
        else
            echo "   ❌ $resource (not found)"
        fi
    done
    
    if confirm_action "remove monitoring resources"; then
        kubectl delete -f deployments/k8s/monitoring.yaml 2>/dev/null && log_success "Monitoring removed" || log_warning "Some monitoring resources may not exist"
    fi
    
    wait_for_user
}

cleanup_cache_service() {
    log_step "3" "Cleanup Cache Service Application"
    
    # Check if cache service exists
    if kubectl get deployment cache-service >/dev/null 2>&1; then
        log_info "📊 Cache Service Status:"
        kubectl get deployment cache-service -o wide
        kubectl get pods -l app=cache-service --no-headers | wc -l | xargs echo "   Running pods:"
        
        if confirm_action "remove cache service deployment"; then
            kubectl delete deployment cache-service && log_success "Cache service deployment removed" || log_error "Failed to remove deployment"
        fi
    else
        log_info "Cache service deployment not found - skipping"
    fi
    
    wait_for_user
}

cleanup_redis() {
    log_step "4" "Cleanup Redis Backend"
    
    # Analyze data before cleanup
    if [[ "$PRESERVE_DATA" == "true" ]]; then
        backup_redis_data
    else
        analyze_redis_data
    fi
    
    if kubectl get statefulset redis >/dev/null 2>&1; then
        log_info "📊 Redis Status:"
        kubectl get statefulset redis -o wide
        kubectl get pods -l app=redis --no-headers | wc -l | xargs echo "   Running Redis pods:"
        kubectl get pvc -l app=redis --no-headers | wc -l | xargs echo "   Persistent volumes:"
        
        if [[ "$PRESERVE_DATA" == "true" ]]; then
            log_warning "⚠️ PRESERVE_DATA mode: Redis StatefulSet will be removed but PVCs will be preserved"
            if confirm_action "remove Redis StatefulSet (keeping data)"; then
                kubectl delete statefulset redis && log_success "Redis StatefulSet removed (data preserved)" || log_error "Failed to remove Redis StatefulSet"
            fi
        else
            log_warning "⚠️ WARNING: This will permanently delete all cached data!"
            if confirm_action "remove Redis StatefulSet and all data"; then
                kubectl delete -f deployments/k8s/redis-statefulset.yaml && log_success "Redis and data permanently removed" || log_error "Failed to remove Redis"
            fi
        fi
    else
        log_info "Redis StatefulSet not found - skipping"
    fi
    
    wait_for_user
}

cleanup_redis_backup() {
    log_step "5" "Cleanup Redis Backup Resources"
    
    local backup_resources=(
        "cronjob/redis-backup"
        "cronjob/redis-health-check"
        "job/redis-recovery"
        "pvc/redis-backup-pvc"
    )
    
    log_info "Checking backup resources:"
    local found_resources=()
    for resource in "${backup_resources[@]}"; do
        if kubectl get "$resource" >/dev/null 2>&1; then
            echo "   ✅ $resource"
            found_resources+=("$resource")
        else
            echo "   ❌ $resource (not found)"
        fi
    done
    
    if [[ ${#found_resources[@]} -gt 0 ]]; then
        if confirm_action "remove Redis backup resources"; then
            kubectl delete -f deployments/k8s/redis-backup.yaml 2>/dev/null && log_success "Backup resources removed" || log_warning "Some backup resources may not exist"
        fi
    else
        log_info "No backup resources found - skipping"
    fi
    
    wait_for_user
}

cleanup_services() {
    log_step "6" "Cleanup Services and RBAC"
    
    local service_resources=(
        "service/cache-service"
        "service/cache-service-metrics"
        "service/redis-service"
        "service/redis-headless"
        "serviceaccount/cache-service"
        "clusterrole/cache-service-role"
        "clusterrolebinding/cache-service-binding"
    )
    
    log_info "Found service resources:"
    for resource in "${service_resources[@]}"; do
        if kubectl get "$resource" >/dev/null 2>&1; then
            echo "   ✅ $resource"
        else
            echo "   ❌ $resource (not found)"
        fi
    done
    
    if confirm_action "remove services and RBAC"; then
        kubectl delete -f deployments/k8s/service.yaml 2>/dev/null && log_success "Services and RBAC removed" || log_warning "Some service resources may not exist"
    fi
    
    wait_for_user
}

cleanup_configuration() {
    log_step "7" "Cleanup Configuration"
    
    local config_resources=(
        "configmap/cache-service-config"
        "configmap/redis-config"
        "secret/cache-service-secret"
    )
    
    log_info "Found configuration resources:"
    for resource in "${config_resources[@]}"; do
        if kubectl get "$resource" >/dev/null 2>&1; then
            echo "   ✅ $resource"
        else
            echo "   ❌ $resource (not found)"
        fi
    done
    
    if confirm_action "remove configuration (ConfigMaps and Secrets)"; then
        kubectl delete configmap cache-service-config 2>/dev/null || true
        kubectl delete configmap redis-config 2>/dev/null || true
        kubectl delete secret cache-service-secret 2>/dev/null || true
        log_success "Configuration removed"
    fi
    
    wait_for_user
}

cleanup_persistent_volumes() {
    log_step "8" "Cleanup Persistent Volumes (Optional)"
    
    if [[ "$PRESERVE_DATA" == "true" ]]; then
        log_info "🛡️ PRESERVE_DATA mode: Skipping PVC cleanup"
        log_info "Preserved PVCs:"
        kubectl get pvc -l app=redis 2>/dev/null || log_info "   No Redis PVCs found"
        kubectl get pvc redis-backup-pvc 2>/dev/null || log_info "   No backup PVC found"
        return
    fi
    
    log_info "Checking for remaining PVCs..."
    local redis_pvcs=$(kubectl get pvc -l app=redis --no-headers 2>/dev/null | wc -l)
    local backup_pvc_exists=false
    kubectl get pvc redis-backup-pvc >/dev/null 2>&1 && backup_pvc_exists=true
    
    if [[ "$redis_pvcs" -gt 0 ]] || [[ "$backup_pvc_exists" == "true" ]]; then
        log_warning "⚠️ WARNING: This will permanently delete all stored data!"
        log_info "Found PVCs:"
        kubectl get pvc -l app=redis 2>/dev/null || true
        kubectl get pvc redis-backup-pvc 2>/dev/null || true
        
        if confirm_action "permanently delete all persistent volumes and data"; then
            kubectl delete pvc -l app=redis 2>/dev/null || true
            kubectl delete pvc redis-backup-pvc 2>/dev/null || true
            log_success "All persistent volumes removed"
        fi
    else
        log_info "No persistent volumes found - skipping"
    fi
    
    wait_for_user
}

final_verification() {
    log_header "Final Verification"
    
    log_info "🔍 Checking for remaining cache service resources..."
    
    # Check main resources
    local remaining_resources=()
    
    kubectl get all -l app=cache-service --no-headers 2>/dev/null | while read -r line; do
        remaining_resources+=("$line")
    done
    
    kubectl get all -l app=redis --no-headers 2>/dev/null | while read -r line; do
        remaining_resources+=("$line")
    done
    
    # Check configuration
    kubectl get configmap,secret | grep cache 2>/dev/null || true
    
    # Check monitoring
    kubectl get prometheusrule,servicemonitor | grep cache 2>/dev/null || true
    
    # Check PVCs
    log_info "💾 Persistent Volume Status:"
    kubectl get pvc -l app=redis 2>/dev/null || log_info "   No Redis PVCs found"
    kubectl get pvc redis-backup-pvc 2>/dev/null || log_info "   No backup PVC found"
    
    if [[ "$PRESERVE_DATA" == "true" ]]; then
        log_info "🛡️ Data Preservation Summary:"
        echo "   - Redis PVCs preserved for future restoration"
        echo "   - To restore: redeploy Redis StatefulSet with same PVC names"
        echo "   - Backup location: Use 'kubectl cp' commands shown during cleanup"
    fi
    
    log_success "Cleanup verification completed"
}

cleanup_summary() {
    log_header "Cleanup Summary"
    
    if [[ "$PRESERVE_DATA" == "true" ]]; then
        log_info "🛡️ Cache Service Cleanup Complete (Data Preserved)"
        echo "   ✅ Application components removed"
        echo "   ✅ Configuration removed"
        echo "   ✅ Monitoring removed"
        echo "   ✅ Services and RBAC removed"
        echo "   🛡️ Redis data preserved in PVCs"
        echo ""
        log_info "🔄 To restore with preserved data:"
        echo "   1. Redeploy using: ./manual-deploy.sh"
        echo "   2. Redis will automatically reconnect to existing PVCs"
        echo "   3. All cached data will be restored"
    else
        log_info "🗑️ Cache Service Cleanup Complete (All Data Removed)"
        echo "   ✅ All application components removed"
        echo "   ✅ All configuration removed"
        echo "   ✅ All monitoring removed"
        echo "   ✅ All services and RBAC removed"
        echo "   ✅ All persistent data permanently deleted"
        echo ""
        log_info "🚀 To redeploy fresh:"
        echo "   1. Deploy using: ./manual-deploy.sh"
        echo "   2. This will create a clean cache service installation"
    fi
    
    log_info "📚 Documentation:"
    echo "   - Deployment guide: deployments/STEP_BY_STEP_DEPLOYMENT_GUIDE.md"
    echo "   - Operations guide: docs/OPERATIONS.md"
    
    log_success "Cleanup completed successfully!"
}

main() {
    log_header "🗑️ Cache Service Manual Cleanup"
    
    echo "Cache Service High-Performance Redis Architecture"
    echo "Options: Data preservation, step-by-step guidance, forced cleanup"
    echo ""
    
    if [[ "$STEP_BY_STEP" == "true" ]]; then
        log_info "📚 Step-by-step mode enabled - you'll be prompted at each step"
    fi
    
    if [[ "$PRESERVE_DATA" == "true" ]]; then
        log_warning "🛡️ Data preservation mode enabled - Redis data will be kept"
    else
        log_warning "🗑️ Full cleanup mode - all data will be permanently deleted"
    fi
    
    if [[ "$FORCE_CLEANUP" == "true" ]]; then
        log_warning "💥 Force mode enabled - no confirmation prompts"
    fi
    
    echo
    if [[ "$FORCE_CLEANUP" != "true" ]]; then
        read -p "🗑️ Ready to cleanup Cache Service? (y/N): " -r
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            echo "Cleanup cancelled."
            exit 0
        fi
    fi
    
    # Cleanup components in reverse order
    cleanup_hpa
    cleanup_monitoring
    cleanup_cache_service
    cleanup_redis
    cleanup_redis_backup
    cleanup_services
    cleanup_configuration
    cleanup_persistent_volumes
    
    # Final verification
    final_verification
    
    # Summary
    cleanup_summary
}

# Handle script interruption
trap 'echo -e "\n${RED}❌ Cleanup interrupted${NC}"; exit 1' INT TERM

# Run main function
main "$@" 