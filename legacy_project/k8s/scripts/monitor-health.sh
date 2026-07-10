#!/bin/bash

# Comprehensive Health Monitoring for All Services
# Date: December 29, 2024
# Purpose: Real-time health monitoring and alerting for microservices

# Source common functions
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common-functions.sh"

# Configuration
SERVICES=(
    "cache-service:30081"
    "storage-service:30082"
    "auth-service:30083"
    "queue-service:30084"
    "profile-service:30085"
    "worker-service:30086"
)

INFRASTRUCTURE_SERVICES=(
    "redis:6379"
    "postgres:5432"
    "rabbitmq:5672"
)

# Health check results
HEALTHY_SERVICES=0
UNHEALTHY_SERVICES=0
WARNING_SERVICES=0

# Main monitoring function
main() {
    log_info "🏥 Starting Comprehensive Health Monitoring"
    echo "============================================="
    
    # Check prerequisites
    check_prerequisites || exit 1
    
    # Monitor all services
    monitor_application_services
    monitor_infrastructure_services
    monitor_resource_usage
    monitor_network_connectivity
    
    # Generate health report
    generate_health_report
}

# Monitor application services
monitor_application_services() {
    log_info "📊 Monitoring Application Services"
    echo "================================="
    
    for service_info in "${SERVICES[@]}"; do
        IFS=':' read -r service_name port <<< "$service_info"
        
        log_info "Checking $service_name..."
        
        # Check pod status
        if ! check_pod_status "$service_name"; then
            log_error "$service_name: Pod not running"
            ((UNHEALTHY_SERVICES++))
            continue
        fi
        
        # Check service endpoint
        if ! check_service_endpoint "$service_name" "$port"; then
            log_error "$service_name: Service endpoint not responding"
            ((UNHEALTHY_SERVICES++))
            continue
        fi
        
        # Check health endpoint
        if ! check_health_endpoint "$service_name" "$port"; then
            log_error "$service_name: Health endpoint failed"
            ((UNHEALTHY_SERVICES++))
            continue
        fi
        
        # Check readiness endpoint
        if check_readiness_endpoint "$service_name" "$port"; then
            log_success "$service_name: Healthy and ready"
            ((HEALTHY_SERVICES++))
        else
            log_warning "$service_name: Healthy but not ready"
            ((WARNING_SERVICES++))
        fi
    done
}

# Monitor infrastructure services
monitor_infrastructure_services() {
    log_info "🏗️ Monitoring Infrastructure Services"
    echo "===================================="
    
    # Check Redis
    check_redis_health
    
    # Check PostgreSQL
    check_postgres_health
    
    # Check RabbitMQ
    check_rabbitmq_health
}

# Monitor resource usage
monitor_resource_usage() {
    log_info "💾 Monitoring Resource Usage"
    echo "==========================="
    
    # Check node resources
    log_info "Node resource usage:"
    if kubectl top nodes 2>/dev/null; then
        log_success "Node resource monitoring available"
    else
        log_warning "Node resource monitoring requires metrics server"
    fi
    
    # Check pod resources
    log_info "Pod resource usage:"
    if kubectl top pods 2>/dev/null | grep -E "(cache|storage|auth|queue|profile|worker)"; then
        log_success "Pod resource monitoring available"
    else
        log_warning "Pod resource monitoring requires metrics server"
    fi
    
    # Check disk usage in pods
    check_disk_usage
    
    # Check memory pressure
    check_memory_pressure
}

# Monitor network connectivity
monitor_network_connectivity() {
    log_info "🌐 Monitoring Network Connectivity"
    echo "================================="
    
    # Test service-to-service connectivity
    test_service_connectivity
    
    # Check network policies
    check_network_policies
    
    # Test external connectivity
    test_external_connectivity
}

# Helper functions
check_pod_status() {
    local service_name="$1"
    
    local pod_status=$(kubectl get pods -l app="$service_name" -o jsonpath='{.items[0].status.phase}' 2>/dev/null)
    
    if [ "$pod_status" = "Running" ]; then
        # Check if pod is ready
        local ready_condition=$(kubectl get pods -l app="$service_name" -o jsonpath='{.items[0].status.conditions[?(@.type=="Ready")].status}' 2>/dev/null)
        
        if [ "$ready_condition" = "True" ]; then
            log_debug "$service_name pod is running and ready"
            return 0
        else
            log_warning "$service_name pod is running but not ready"
            return 1
        fi
    else
        log_error "$service_name pod status: $pod_status"
        return 1
    fi
}

check_service_endpoint() {
    local service_name="$1"
    local port="$2"
    
    # Check if service exists
    if ! kubectl get service "$service_name" &>/dev/null; then
        log_error "$service_name: Kubernetes service not found"
        return 1
    fi
    
    # Check if service has endpoints
    local endpoints=$(kubectl get endpoints "$service_name" -o jsonpath='{.subsets[0].addresses}' 2>/dev/null)
    
    if [ -n "$endpoints" ] && [ "$endpoints" != "null" ]; then
        log_debug "$service_name service has active endpoints"
        return 0
    else
        log_error "$service_name service has no active endpoints"
        return 1
    fi
}

check_health_endpoint() {
    local service_name="$1"
    local port="$2"
    
    local health_url="http://localhost:$port/health"
    
    if curl -s --max-time 10 "$health_url" > /dev/null 2>&1; then
        log_debug "$service_name health endpoint responding"
        return 0
    else
        log_error "$service_name health endpoint not responding"
        return 1
    fi
}

check_readiness_endpoint() {
    local service_name="$1"
    local port="$2"
    
    local ready_url="http://localhost:$port/ready"
    
    if curl -s --max-time 5 "$ready_url" > /dev/null 2>&1; then
        log_debug "$service_name readiness endpoint responding"
        return 0
    else
        log_warning "$service_name readiness endpoint not responding"
        return 1
    fi
}

check_redis_health() {
    log_info "Checking Redis health..."
    
    if kubectl get pods -l app=redis | grep -q Running; then
        # Test Redis connectivity from within cluster
        if kubectl run redis-health-check --image=redis:7-alpine --rm -i --restart=Never \
            -- sh -c "redis-cli -h redis-service -p 6379 ping" 2>/dev/null | grep -q "PONG"; then
            log_success "Redis is healthy and responding"
        else
            log_error "Redis is not responding to ping"
        fi
    else
        log_error "Redis pod is not running"
    fi
}

check_postgres_health() {
    log_info "Checking PostgreSQL health..."
    
    if kubectl get pods -l app=postgres | grep -q Running; then
        # Test PostgreSQL connectivity
        if kubectl run postgres-health-check --image=postgres:15-alpine --rm -i --restart=Never \
            --env="PGPASSWORD=profile_user" \
            -- sh -c "pg_isready -h postgres-service -p 5432 -U profile_user" 2>/dev/null | grep -q "accepting connections"; then
            log_success "PostgreSQL is healthy and accepting connections"
        else
            log_error "PostgreSQL is not accepting connections"
        fi
    else
        log_error "PostgreSQL pod is not running"
    fi
}

check_rabbitmq_health() {
    log_info "Checking RabbitMQ health..."
    
    if kubectl get pods -l app=rabbitmq | grep -q Running; then
        # Test RabbitMQ management API
        if kubectl exec -it rabbitmq-0 -- rabbitmq-diagnostics ping 2>/dev/null | grep -q "Ping succeeded"; then
            log_success "RabbitMQ is healthy and responding"
        else
            log_error "RabbitMQ health check failed"
        fi
    else
        log_error "RabbitMQ pod is not running"
    fi
}

check_disk_usage() {
    log_info "Checking disk usage in pods..."
    
    local services=("cache-service" "storage-service" "auth-service" "queue-service" "profile-service")
    
    for service in "${services[@]}"; do
        local pod_name=$(kubectl get pods -l app="$service" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
        
        if [ -n "$pod_name" ]; then
            local disk_usage=$(kubectl exec "$pod_name" -- df -h / 2>/dev/null | awk 'NR==2 {print $5}' | sed 's/%//')
            
            if [ -n "$disk_usage" ] && [ "$disk_usage" -lt 80 ]; then
                log_debug "$service disk usage: ${disk_usage}% (healthy)"
            elif [ -n "$disk_usage" ] && [ "$disk_usage" -lt 90 ]; then
                log_warning "$service disk usage: ${disk_usage}% (warning)"
            elif [ -n "$disk_usage" ]; then
                log_error "$service disk usage: ${disk_usage}% (critical)"
            else
                log_debug "$service disk usage check skipped (unable to determine)"
            fi
        fi
    done
}

check_memory_pressure() {
    log_info "Checking for memory pressure..."
    
    # Check node conditions for memory pressure
    local memory_pressure=$(kubectl get nodes -o jsonpath='{.items[*].status.conditions[?(@.type=="MemoryPressure")].status}' 2>/dev/null)
    
    if echo "$memory_pressure" | grep -q "True"; then
        log_error "Memory pressure detected on cluster nodes"
    else
        log_success "No memory pressure detected"
    fi
}

test_service_connectivity() {
    log_info "Testing service-to-service connectivity..."
    
    # Test cache to storage connectivity
    if timeout 30s kubectl run connectivity-test-$(date +%s) --image=busybox:1.35 --rm -i --restart=Never \
        --labels="app=cache-service" \
        -- sh -c "nc -z storage-service 8080" &>/dev/null; then
        log_success "Cache to Storage connectivity: OK"
    else
        log_warning "Cache to Storage connectivity: Failed"
    fi
    
    # Test auth to storage connectivity
    if timeout 30s kubectl run connectivity-test-$(date +%s) --image=busybox:1.35 --rm -i --restart=Never \
        --labels="app=auth-service" \
        -- sh -c "nc -z storage-service 8080" &>/dev/null; then
        log_success "Auth to Storage connectivity: OK"
    else
        log_warning "Auth to Storage connectivity: Failed"
    fi
}

check_network_policies() {
    log_info "Checking network policies..."
    
    local policies=$(kubectl get networkpolicies --no-headers 2>/dev/null | wc -l)
    
    if [ "$policies" -gt 0 ]; then
        log_success "Network policies are configured ($policies policies)"
        kubectl get networkpolicies --no-headers | while read -r policy rest; do
            log_debug "Network policy: $policy"
        done
    else
        log_warning "No network policies configured"
    fi
}

test_external_connectivity() {
    log_info "Testing external connectivity..."
    
    # Test if services can reach external endpoints (DNS resolution)
    if timeout 10s kubectl run external-test-$(date +%s) --image=busybox:1.35 --rm -i --restart=Never \
        -- sh -c "nslookup kubernetes.default.svc.cluster.local" &>/dev/null; then
        log_success "DNS resolution working"
    else
        log_warning "DNS resolution issues detected"
    fi
}

generate_health_report() {
    log_info "📋 Health Monitoring Summary"
    echo "============================"
    
    local total_services=$((HEALTHY_SERVICES + WARNING_SERVICES + UNHEALTHY_SERVICES))
    
    echo "Total services monitored: $total_services"
    log_success "Healthy services: $HEALTHY_SERVICES"
    [ $WARNING_SERVICES -gt 0 ] && log_warning "Warning services: $WARNING_SERVICES" || log_info "Warning services: $WARNING_SERVICES"
    [ $UNHEALTHY_SERVICES -gt 0 ] && log_error "Unhealthy services: $UNHEALTHY_SERVICES" || log_info "Unhealthy services: $UNHEALTHY_SERVICES"
    
    # Calculate health percentage
    if [ $total_services -gt 0 ]; then
        local health_percentage=$(( (HEALTHY_SERVICES * 100) / total_services ))
        
        if [ $health_percentage -ge 90 ]; then
            log_success "Overall system health: ${health_percentage}% (Excellent)"
            return 0
        elif [ $health_percentage -ge 75 ]; then
            log_success "Overall system health: ${health_percentage}% (Good)"
            return 0
        elif [ $health_percentage -ge 50 ]; then
            log_warning "Overall system health: ${health_percentage}% (Warning)"
            return 1
        else
            log_error "Overall system health: ${health_percentage}% (Critical)"
            return 1
        fi
    else
        log_error "No services found to monitor"
        return 1
    fi
}

# Run main function
main "$@" 