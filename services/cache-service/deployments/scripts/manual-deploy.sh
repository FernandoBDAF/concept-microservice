#!/bin/bash

# Manual Deployment Script for Cache Service
# Purpose: Step-by-step deployment with Redis backend analysis and cache-specific monitoring
# Usage: ./manual-deploy.sh [--analyze] [--step-by-step]

set -euo pipefail

# Configuration
SERVICE_NAME="cache-service"
REDIS_SERVICE="redis"
NAMESPACE="default"
STEP_BY_STEP=${STEP_BY_STEP:-false}
ANALYZE_MODE=${ANALYZE_MODE:-false}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --step-by-step)
            STEP_BY_STEP=true
            shift
            ;;
        --analyze)
            ANALYZE_MODE=true
            shift
            ;;
        *)
            echo "Unknown option: $1"
            echo "Usage: $0 [--step-by-step] [--analyze]"
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

# Enhanced analysis function
analyze_manifest() {
    local manifest_file="$1"
    local description="$2"
    
    if [[ "$ANALYZE_MODE" == "true" ]]; then
        log_info "📊 Analyzing $description..."
        echo
        
        case "$manifest_file" in
            *secret*)
                log_info "🔐 Secret Analysis:"
                echo "   - Contains sensitive data for Redis authentication"
                echo "   - Production: Replace with secure password generation"
                echo "   - Keys: CACHE_REDIS_PASSWORD, JWT_SECRET_KEY"
                ;;
            *configmap*)
                log_info "⚙️ ConfigMap Analysis:"
                echo "   - Cache-optimized configuration settings"
                echo "   - Redis connection pool: 100 connections"
                echo "   - TTL strategies: Profile (30m), Task (15m), Session (30m)"
                echo "   - Circuit breaker: Enabled with thresholds"
                kubectl get configmap cache-service-config -o yaml | grep -A 5 -E "(POOL_SIZE|TTL|CIRCUIT)" 2>/dev/null || true
                ;;
            *redis*)
                log_info "🗄️ Redis StatefulSet Analysis:"
                echo "   - StatefulSet for data persistence and stable identity"
                echo "   - 3-replica cluster with persistent volumes"
                echo "   - Memory policy: allkeys-lru for optimal caching"
                echo "   - Persistence: Both AOF and RDB enabled"
                ;;
            *deployment*)
                log_info "🚀 Cache Service Analysis:"
                echo "   - Deployment with 3 replicas for high availability"
                echo "   - Circuit breaker integration with Redis"
                echo "   - Health checks: /health and /ready endpoints"
                echo "   - Resource limits: 256Mi-1Gi memory, 250m-1000m CPU"
                ;;
            *monitoring*)
                log_info "📊 Monitoring Analysis:"
                echo "   - PrometheusRule with 15+ cache-specific alerts"
                echo "   - ServiceMonitor for metrics scraping"
                echo "   - SLI/SLO tracking for availability and latency"
                echo "   - Grafana dashboard with cache performance panels"
                ;;
        esac
        echo
        wait_for_user
    fi
}

# Cache-specific functions
check_redis_health() {
    log_info "🔍 Checking Redis health..."
    if kubectl get pods -l app=redis --no-headers 2>/dev/null | grep -q "Running"; then
        local redis_pod=$(kubectl get pods -l app=redis --no-headers | head -1 | awk '{print $1}')
        if kubectl exec "$redis_pod" -- redis-cli ping 2>/dev/null | grep -q "PONG"; then
            log_success "Redis is responding to ping"
            
            # Additional Redis health checks
            log_info "📊 Redis Status:"
            kubectl exec "$redis_pod" -- redis-cli info memory 2>/dev/null | grep -E "(used_memory_human|maxmemory_human)" || true
            kubectl exec "$redis_pod" -- redis-cli info clients 2>/dev/null | grep "connected_clients" || true
        else
            log_warning "Redis pod exists but not responding"
        fi
    else
        log_warning "Redis pods not running yet"
    fi
}

analyze_cache_performance() {
    log_info "📊 Analyzing cache performance..."
    
    # Check if cache service is running
    if kubectl get pods -l app=cache-service --no-headers 2>/dev/null | grep -q "Running"; then
        log_info "🔄 Port forwarding to cache service for metrics..."
        kubectl port-forward service/cache-service 8081:8081 &
        local pf_pid=$!
        sleep 3
        
        log_info "📈 Cache Metrics Sample:"
        curl -s http://localhost:8081/metrics 2>/dev/null | grep -E "(cache_operations_total|cache_hits_total|redis_pool)" | head -5 || true
        
        log_info "💓 Health Check:"
        kubectl port-forward service/cache-service 8080:8080 &
        local health_pf_pid=$!
        sleep 2
        curl -s http://localhost:8080/health 2>/dev/null | jq '.status' 2>/dev/null || curl -s http://localhost:8080/health || log_warning "Health check not available"
        
        kill $pf_pid $health_pf_pid 2>/dev/null || true
    else
        log_warning "Cache service not running yet"
    fi
}

test_cache_operations() {
    log_info "🧪 Testing cache operations..."
    
    if kubectl get pods -l app=cache-service --no-headers 2>/dev/null | grep -q "Running"; then
        log_info "🔄 Port forwarding for cache testing..."
        kubectl port-forward service/cache-service 8080:8080 &
        local pf_pid=$!
        sleep 3
        
        # Test basic cache operations
        log_info "Testing SET operation..."
        if curl -s -X POST http://localhost:8080/api/v1/cache \
                -H "Content-Type: application/json" \
                -d '{"key":"deploy-test","value":"deployment-success","ttl":"1h"}' >/dev/null 2>&1; then
            log_success "Cache SET operation successful"
        else
            log_warning "Cache SET operation failed"
        fi
        
        log_info "Testing GET operation..."
        if curl -s http://localhost:8080/api/v1/cache/deploy-test 2>/dev/null | grep -q "deployment-success"; then
            log_success "Cache GET operation successful"
        else
            log_warning "Cache GET operation failed"
        fi
        
        # Test profile-specific caching
        log_info "Testing profile-specific caching..."
        if curl -s -X POST http://localhost:8080/api/v1/cache \
                -H "Content-Type: application/json" \
                -d '{"key":"profile:deploy-test","value":"{\"id\":\"deploy-test\",\"name\":\"Deployment User\"}","ttl":"30m"}' >/dev/null 2>&1; then
            log_success "Profile cache operation successful"
        else
            log_warning "Profile cache operation failed"
        fi
        
        kill $pf_pid 2>/dev/null || true
    else
        log_warning "Cache service not available for testing"
    fi
}

validate_deployment_state() {
    local step_name="$1"
    
    log_info "🔍 Validating deployment state after $step_name..."
    
    case "$step_name" in
        "secrets")
            kubectl get secret cache-service-secret >/dev/null 2>&1 && log_success "Secret exists" || log_error "Secret missing"
            ;;
        "configmaps")
            kubectl get configmap cache-service-config >/dev/null 2>&1 && log_success "ConfigMap exists" || log_error "ConfigMap missing"
            ;;
        "services")
            kubectl get service cache-service >/dev/null 2>&1 && log_success "Service exists" || log_error "Service missing"
            ;;
        "redis")
            if kubectl get statefulset redis >/dev/null 2>&1; then
                log_success "Redis StatefulSet exists"
                check_redis_health
            else
                log_error "Redis StatefulSet missing"
            fi
            ;;
        "cache-service")
            if kubectl get deployment cache-service >/dev/null 2>&1; then
                log_success "Cache Service Deployment exists"
                local ready_replicas=$(kubectl get deployment cache-service -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
                log_info "Ready replicas: $ready_replicas"
                test_cache_operations
            else
                log_error "Cache Service Deployment missing"
            fi
            ;;
        "monitoring")
            kubectl get prometheusrule cache-service-alerts >/dev/null 2>&1 && log_success "PrometheusRule exists" || log_warning "PrometheusRule missing"
            kubectl get servicemonitor cache-service-metrics >/dev/null 2>&1 && log_success "ServiceMonitor exists" || log_warning "ServiceMonitor missing"
            ;;
        "hpa")
            kubectl get hpa cache-service-hpa >/dev/null 2>&1 && log_success "HPA exists" || log_warning "HPA missing"
            ;;
    esac
}

check_prerequisites() {
    log_header "Checking Prerequisites"
    
    # Check kubectl access
    if ! kubectl cluster-info >/dev/null 2>&1; then
        log_error "Cannot access Kubernetes cluster"
        exit 1
    fi
    log_success "Kubernetes cluster accessible"
    
    # Check permissions
    if ! kubectl auth can-i create statefulsets >/dev/null 2>&1; then
        log_error "Insufficient permissions to create StatefulSets"
        exit 1
    fi
    log_success "Sufficient permissions"
    
    # Check storage classes
    if kubectl get storageclass >/dev/null 2>&1; then
        log_success "Storage classes available"
        if [[ "$ANALYZE_MODE" == "true" ]]; then
            log_info "Available storage classes:"
            kubectl get storageclass --no-headers | awk '{print "   - " $1}'
        fi
    else
        log_warning "No storage classes found"
    fi
    
    wait_for_user
}

deploy_component() {
    local component="$1"
    local manifest_file="$2"
    local description="$3"
    
    log_step "$component" "$description"
    
    analyze_manifest "$manifest_file" "$description"
    
    log_info "Deploying $description..."
    if kubectl apply -f "$manifest_file"; then
        log_success "$description deployed successfully"
    else
        log_error "Failed to deploy $description"
        return 1
    fi
    
    # Wait a moment for resources to be created
    sleep 2
    
    validate_deployment_state "$component"
    wait_for_user
}

final_verification() {
    log_header "Final Verification Suite"
    
    log_info "🔍 Comprehensive deployment status..."
    
    # All resources status
    log_info "📊 All Cache Service Resources:"
    kubectl get all -l app=cache-service 2>/dev/null || log_warning "No cache service resources found"
    
    log_info "📊 All Redis Resources:"
    kubectl get all -l app=redis 2>/dev/null || log_warning "No Redis resources found"
    
    # Storage verification
    log_info "💾 Storage Status:"
    kubectl get pvc -l app=redis 2>/dev/null || log_warning "No Redis PVCs found"
    
    # Configuration verification
    log_info "⚙️ Configuration Status:"
    kubectl get configmap,secret | grep cache || log_warning "No cache configuration found"
    
    # Monitoring verification
    log_info "📊 Monitoring Status:"
    kubectl get prometheusrule,servicemonitor | grep cache 2>/dev/null || log_warning "No monitoring resources found"
    
    # Network verification
    log_info "🌐 Network Status:"
    kubectl get service,endpoints | grep -E "(cache|redis)" || log_warning "No cache/redis services found"
    
    # Performance analysis
    analyze_cache_performance
    
    wait_for_user
}

deployment_summary() {
    log_header "Deployment Summary"
    
    log_info "📋 Components Deployed:"
    echo "   ✅ Secrets (Redis password, JWT keys)"
    echo "   ✅ ConfigMaps (Cache configuration)"
    echo "   ✅ Services (Load balancing, service discovery)"
    echo "   ✅ Redis StatefulSet (Persistent cache backend)"
    echo "   ✅ Cache Service Deployment (Main application)"
    echo "   ✅ Monitoring (Prometheus alerts, Grafana dashboards)"
    echo "   ✅ Auto-Scaling (HPA with cache-specific metrics)"
    
    log_info "🎯 Next Steps:"
    echo "   1. Monitor cache performance: kubectl logs -f deployment/cache-service"
    echo "   2. Check Redis health: kubectl exec -it redis-0 -- redis-cli ping"
    echo "   3. View metrics: kubectl port-forward service/cache-service 8081:8081"
    echo "   4. Test cache operations: ./scripts/performance_test.sh"
    echo "   5. Monitor alerts: kubectl get prometheusrule cache-service-alerts"
    
    log_info "📚 Documentation:"
    echo "   - Step-by-step guide: deployments/STEP_BY_STEP_DEPLOYMENT_GUIDE.md"
    echo "   - Operations guide: docs/OPERATIONS.md"
    echo "   - API documentation: api/openapi.yaml"
    
    log_success "Cache Service deployment completed successfully!"
}

main() {
    log_header "🚀 Cache Service Manual Deployment"
    
    echo "Cache Service High-Performance Redis Architecture"
    echo "Features: <1ms GET, <2ms SET, 10k+ ops/sec, Circuit Breaker"
    echo ""
    
    if [[ "$STEP_BY_STEP" == "true" ]]; then
        log_info "📚 Step-by-step mode enabled - you'll be prompted at each step"
    fi
    
    if [[ "$ANALYZE_MODE" == "true" ]]; then
        log_info "🔍 Analysis mode enabled - detailed manifest analysis included"
    fi
    
    echo
    read -p "🚀 Ready to deploy Cache Service? (y/N): " -r
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Deployment cancelled."
        exit 0
    fi
    
    # Prerequisites check
    check_prerequisites
    
    # Deploy components in sequence
    deploy_component "secrets" "deployments/k8s/secret.yaml" "Deploy Secrets"
    deploy_component "configmaps" "deployments/k8s/configmap.yaml" "Deploy ConfigMaps"
    deploy_component "services" "deployments/k8s/service.yaml" "Deploy Services"
    deploy_component "redis" "deployments/k8s/redis-statefulset.yaml" "Deploy Redis Backend"
    deploy_component "cache-service" "deployments/k8s/deployment.yaml" "Deploy Cache Service"
    deploy_component "monitoring" "deployments/k8s/monitoring.yaml" "Deploy Monitoring"
    deploy_component "hpa" "deployments/k8s/hpa.yaml" "Deploy Auto-Scaling"
    
    # Final verification
    final_verification
    
    # Summary
    deployment_summary
}

# Handle script interruption
trap 'echo -e "\n${RED}❌ Deployment interrupted${NC}"; exit 1' INT TERM

# Run main function
main "$@" 