#!/bin/bash

# Enhanced Kind Cluster Setup with Comprehensive Error Handling
# Date: December 29, 2024
# Purpose: Create and configure Kind cluster with robust error handling and validation

# Source common functions
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common-functions.sh"

# Configuration
CLUSTER_NAME="${CLUSTER_NAME:-microservices-kind}"
KIND_CONFIG_FILE="${KIND_CONFIG_FILE:-k8s/cluster/kind-config.yaml}"
TIMEOUT_CLUSTER_READY=300
TIMEOUT_NODE_READY=180

# Main setup function
main() {
    log_info "🚀 Starting Enhanced Kind Cluster Setup"
    echo "========================================"
    
    # Phase 1: Prerequisites
    check_prerequisites || exit 1
    
    # Phase 2: Cluster Management
    manage_existing_cluster
    
    # Phase 3: Cluster Creation
    create_cluster || exit 1
    
    # Phase 4: Validation
    validate_cluster || exit 1
    
    # Phase 5: Infrastructure Setup
    setup_infrastructure || exit 1
    
    log_success "🎉 Kind cluster setup completed successfully!"
    show_cluster_info
}

# Enhanced prerequisite checking
check_prerequisites() {
    log_info "🔍 Checking prerequisites for cluster setup"
    
    # Check Kind installation
    if ! command -v kind &> /dev/null; then
        log_error "Kind is not installed"
        log_info "Install Kind:"
        log_info "  macOS: brew install kind"
        log_info "  Linux: curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.20.0/kind-linux-amd64"
        log_info "  Windows: https://kind.sigs.k8s.io/docs/user/quick-start/#installation"
        return 1
    fi
    
    local kind_version=$(kind version | head -n1)
    log_success "Kind installed: $kind_version"
    
    # Check Docker is running
    if ! docker info &> /dev/null; then
        log_error "Docker is not running"
        log_info "Start Docker and try again"
        return 1
    fi
    
    local docker_version=$(docker version --format '{{.Server.Version}}' 2>/dev/null)
    log_success "Docker running: version $docker_version"
    
    # Check kubectl installation
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl is not installed"
        log_info "Install kubectl:"
        log_info "  macOS: brew install kubectl"
        log_info "  Linux: https://kubernetes.io/docs/tasks/tools/install-kubectl-linux/"
        return 1
    fi
    
    local kubectl_version=$(kubectl version --client --short 2>/dev/null | cut -d' ' -f3)
    log_success "kubectl installed: $kubectl_version"
    
    # Check Kind config file exists
    if [ ! -f "$KIND_CONFIG_FILE" ]; then
        log_error "Kind config file not found: $KIND_CONFIG_FILE"
        log_info "Create the config file or check the path"
        return 1
    fi
    
    log_success "Kind config file found: $KIND_CONFIG_FILE"
    
    # Check available system resources
    check_system_resources
    
    log_success "All prerequisites satisfied"
    return 0
}

# Check system resources
check_system_resources() {
    log_info "Checking system resources..."
    
    # Check available memory (require at least 4GB)
    if command -v free &> /dev/null; then
        local available_mem_gb=$(free -g | awk '/^Mem:/{print $7}')
        if [ "$available_mem_gb" -lt 4 ]; then
            log_warning "Available memory: ${available_mem_gb}GB (recommended: 4GB+)"
        else
            log_success "Available memory: ${available_mem_gb}GB"
        fi
    fi
    
    # Check Docker resources
    local docker_mem=$(docker system info --format '{{.MemTotal}}' 2>/dev/null)
    if [ -n "$docker_mem" ]; then
        local docker_mem_gb=$((docker_mem / 1024 / 1024 / 1024))
        log_success "Docker memory limit: ${docker_mem_gb}GB"
    fi
    
    # Check disk space
    local available_disk=$(df -BG . | awk 'NR==2 {print $4}' | sed 's/G//')
    if [ "$available_disk" -lt 10 ]; then
        log_warning "Available disk space: ${available_disk}GB (recommended: 10GB+)"
    else
        log_success "Available disk space: ${available_disk}GB"
    fi
}

# Manage existing cluster
manage_existing_cluster() {
    log_info "🔄 Checking for existing cluster"
    
    if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
        log_warning "Cluster '$CLUSTER_NAME' already exists"
        
        # Check if cluster is healthy
        if kubectl cluster-info --context "kind-${CLUSTER_NAME}" &>/dev/null; then
            log_info "Existing cluster appears to be healthy"
            
            # Ask user what to do
            echo
            echo "Options:"
            echo "1) Keep existing cluster and skip creation"
            echo "2) Delete existing cluster and create new one"
            echo "3) Abort setup"
            echo
            
            while true; do
                read -p "Choose option (1-3): " -n 1 -r choice
                echo
                
                case $choice in
                    1)
                        log_info "Using existing cluster"
                        return 0
                        ;;
                    2)
                        log_info "Deleting existing cluster..."
                        delete_existing_cluster
                        break
                        ;;
                    3)
                        log_info "Setup aborted by user"
                        exit 0
                        ;;
                    *)
                        echo "Invalid choice. Please enter 1, 2, or 3."
                        ;;
                esac
            done
        else
            log_warning "Existing cluster appears to be unhealthy"
            log_info "Deleting unhealthy cluster..."
            delete_existing_cluster
        fi
    else
        log_info "No existing cluster found"
    fi
}

# Delete existing cluster with error handling
delete_existing_cluster() {
    log_info "Deleting existing cluster '$CLUSTER_NAME'..."
    
    # Show progress for long-running operation
    (
        if kind delete cluster --name="$CLUSTER_NAME"; then
            log_success "Cluster deleted successfully"
        else
            log_error "Failed to delete cluster"
            exit 1
        fi
    ) &
    
    local delete_pid=$!
    show_progress $delete_pid "Deleting cluster"
    wait $delete_pid
    
    # Verify cluster is deleted
    if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
        log_error "Cluster still exists after deletion attempt"
        return 1
    fi
    
    log_success "Cluster deleted successfully"
    return 0
}

# Create cluster with comprehensive error handling
create_cluster() {
    log_info "🏗️ Creating Kind cluster '$CLUSTER_NAME'"
    
    # Validate config file
    if ! kind create cluster --config="$KIND_CONFIG_FILE" --name="$CLUSTER_NAME" --dry-run &>/dev/null; then
        log_error "Kind config file validation failed"
        log_info "Check the config file syntax: $KIND_CONFIG_FILE"
        return 1
    fi
    
    log_info "Config file validated successfully"
    
    # Create cluster with timeout and progress indication
    log_info "Creating cluster (this may take 2-5 minutes)..."
    
    (
        if timeout $TIMEOUT_CLUSTER_READY kind create cluster \
            --config="$KIND_CONFIG_FILE" \
            --name="$CLUSTER_NAME" \
            --wait="${TIMEOUT_NODE_READY}s"; then
            log_success "Cluster created successfully"
        else
            log_error "Cluster creation failed or timed out"
            exit 1
        fi
    ) &
    
    local create_pid=$!
    show_progress $create_pid "Creating cluster"
    wait $create_pid
    local create_result=$?
    
    if [ $create_result -ne 0 ]; then
        log_error "Cluster creation failed"
        cleanup_failed_cluster
        return 1
    fi
    
    log_success "Cluster '$CLUSTER_NAME' created successfully"
    return 0
}

# Cleanup failed cluster creation
cleanup_failed_cluster() {
    log_info "Cleaning up failed cluster creation..."
    
    # Try to delete the failed cluster
    kind delete cluster --name="$CLUSTER_NAME" &>/dev/null || true
    
    # Clean up any dangling Docker containers
    docker ps -a --filter "label=io.x-k8s.kind.cluster=$CLUSTER_NAME" --format "{{.ID}}" | \
        xargs -r docker rm -f &>/dev/null || true
    
    log_info "Cleanup completed"
}

# Validate cluster is working properly
validate_cluster() {
    log_info "✅ Validating cluster functionality"
    
    # Set kubectl context
    local context="kind-${CLUSTER_NAME}"
    
    # Test cluster connectivity
    log_info "Testing cluster connectivity..."
    if ! timeout 30 kubectl cluster-info --context="$context" &>/dev/null; then
        log_error "Cluster connectivity test failed"
        return 1
    fi
    
    log_success "Cluster connectivity: OK"
    
    # Wait for all nodes to be ready
    log_info "Waiting for nodes to be ready..."
    if ! timeout $TIMEOUT_NODE_READY kubectl wait --for=condition=Ready nodes --all --context="$context"; then
        log_error "Nodes did not become ready within timeout"
        show_node_status
        return 1
    fi
    
    log_success "All nodes are ready"
    
    # Test DNS resolution
    log_info "Testing DNS resolution..."
    if ! test_dns_resolution; then
        log_warning "DNS resolution test failed (may be temporary)"
    else
        log_success "DNS resolution: OK"
    fi
    
    # Test pod scheduling
    log_info "Testing pod scheduling..."
    if ! test_pod_scheduling; then
        log_error "Pod scheduling test failed"
        return 1
    fi
    
    log_success "Pod scheduling: OK"
    
    log_success "Cluster validation completed successfully"
    return 0
}

# Test DNS resolution in cluster
test_dns_resolution() {
    local test_pod="dns-test-$(date +%s)"
    
    if timeout 60 kubectl run "$test_pod" --image=busybox:1.35 --rm -i --restart=Never \
        -- nslookup kubernetes.default.svc.cluster.local &>/dev/null; then
        return 0
    else
        return 1
    fi
}

# Test pod scheduling
test_pod_scheduling() {
    local test_pod="schedule-test-$(date +%s)"
    
    if timeout 60 kubectl run "$test_pod" --image=busybox:1.35 --rm -i --restart=Never \
        -- echo "scheduling test successful" &>/dev/null; then
        return 0
    else
        return 1
    fi
}

# Show node status for debugging
show_node_status() {
    log_info "Node status for debugging:"
    kubectl get nodes -o wide || true
    
    log_info "Node conditions:"
    kubectl describe nodes | grep -A 10 "Conditions:" || true
}

# Setup infrastructure components
setup_infrastructure() {
    log_info "🏗️ Setting up infrastructure components"
    
    # Apply infrastructure manifests if they exist
    local infrastructure_dirs=(
        "k8s/cluster/infrastructure"
        "k8s/infrastructure"
    )
    
    for infra_dir in "${infrastructure_dirs[@]}"; do
        if [ -d "$infra_dir" ]; then
            log_info "Applying infrastructure from: $infra_dir"
            
            # Apply all YAML files in the directory
            find "$infra_dir" -name "*.yaml" -o -name "*.yml" | while read -r manifest; do
                log_info "Applying: $(basename "$manifest")"
                
                if kubectl apply -f "$manifest"; then
                    log_success "Applied: $(basename "$manifest")"
                else
                    log_warning "Failed to apply: $(basename "$manifest")"
                fi
            done
        fi
    done
    
    # Wait for infrastructure pods to be ready
    log_info "Waiting for infrastructure pods to be ready..."
    sleep 10
    
    # Check if any infrastructure pods exist
    if kubectl get pods --all-namespaces | grep -E "(ingress|metrics|storage)" &>/dev/null; then
        kubectl wait --for=condition=Ready pods --all --timeout=120s --all-namespaces || \
            log_warning "Some infrastructure pods may not be ready yet"
    else
        log_info "No infrastructure pods detected (this is normal for basic setup)"
    fi
    
    log_success "Infrastructure setup completed"
}

# Show cluster information
show_cluster_info() {
    log_info "📋 Cluster Information"
    echo "======================"
    
    # Cluster details
    echo "Cluster Name: $CLUSTER_NAME"
    echo "Context: kind-$CLUSTER_NAME"
    echo "Config File: $KIND_CONFIG_FILE"
    
    # Node information
    log_info "Nodes:"
    kubectl get nodes -o wide
    
    # Kubernetes version
    local k8s_version=$(kubectl version --short 2>/dev/null | grep "Server Version" | cut -d' ' -f3)
    echo "Kubernetes Version: $k8s_version"
    
    # Cluster endpoints
    log_info "Cluster endpoints:"
    kubectl cluster-info
    
    # Port mappings (if any)
    log_info "Port mappings:"
    docker ps --filter "label=io.x-k8s.kind.cluster=$CLUSTER_NAME" \
        --format "table {{.Names}}\t{{.Ports}}" | grep -E ":[0-9]+->" || \
        echo "No port mappings configured"
    
    # Next steps
    echo
    log_info "🚀 Next Steps:"
    echo "1. Deploy services: kubectl apply -f k8s/deployment/"
    echo "2. Monitor health: ./k8s/scripts/monitor-health.sh"
    echo "3. Run integration tests: ./k8s/scripts/test-integration.sh"
    echo "4. Access services via NodePort (30081-30086)"
}

# Run main function
main "$@" 