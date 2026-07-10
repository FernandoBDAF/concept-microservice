#!/bin/bash

# Kind-First Microservices Cluster Setup Script
# Automates Kind cluster creation and infrastructure deployment
# Provides error handling, logging, and rollback capabilities

set -euo pipefail

# Configuration
CLUSTER_NAME="microservices-kind"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOG_FILE="/tmp/kind-cluster-setup.log"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging function
log() {
    echo -e "${BLUE}[$(date +'%Y-%m-%d %H:%M:%S')]${NC} $1" | tee -a "$LOG_FILE"
}

log_error() {
    echo -e "${RED}[ERROR $(date +'%Y-%m-%d %H:%M:%S')]${NC} $1" | tee -a "$LOG_FILE"
}

log_success() {
    echo -e "${GREEN}[SUCCESS $(date +'%Y-%m-%d %H:%M:%S')]${NC} $1" | tee -a "$LOG_FILE"
}

log_warning() {
    echo -e "${YELLOW}[WARNING $(date +'%Y-%m-%d %H:%M:%S')]${NC} $1" | tee -a "$LOG_FILE"
}

# Check prerequisites
check_prerequisites() {
    log "Checking prerequisites..."
    
    # Check if Kind is installed
    if ! command -v kind &> /dev/null; then
        log_error "Kind is not installed. Please install Kind first."
        log "Installation instructions: https://kind.sigs.k8s.io/docs/user/quick-start/#installation"
        exit 1
    fi
    
    # Check if Docker is running
    if ! docker info &> /dev/null; then
        log_error "Docker is not running. Please start Docker first."
        exit 1
    fi
    
    # Check if kubectl is installed
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl is not installed. Please install kubectl first."
        exit 1
    fi
    
    log_success "All prerequisites met"
}

# Cleanup function for rollback
cleanup() {
    log_warning "Performing cleanup due to error or interrupt..."
    if kind get clusters | grep -q "$CLUSTER_NAME"; then
        log "Deleting Kind cluster: $CLUSTER_NAME"
        kind delete cluster --name "$CLUSTER_NAME" || true
    fi
    log "Cleanup completed"
}

# Set trap for cleanup on error or interrupt
trap cleanup ERR INT TERM

# Create Kind cluster
create_cluster() {
    log "Creating Kind cluster: $CLUSTER_NAME"
    
    # Check if cluster already exists
    if kind get clusters | grep -q "$CLUSTER_NAME"; then
        log_warning "Cluster $CLUSTER_NAME already exists"
        read -p "Do you want to delete and recreate it? (y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            log "Deleting existing cluster..."
            kind delete cluster --name "$CLUSTER_NAME"
        else
            log "Using existing cluster"
            return 0
        fi
    fi
    
    # Create cluster with configuration
    log "Creating cluster with multi-node configuration..."
    kind create cluster --name "$CLUSTER_NAME" --config "$SCRIPT_DIR/kind-config.yaml"
    
    # Wait for cluster to be ready
    log "Waiting for cluster to be ready..."
    kubectl cluster-info --context "kind-$CLUSTER_NAME" || {
        log_error "Failed to connect to cluster"
        exit 1
    }
    
    # Wait for nodes to be ready
    log "Waiting for all nodes to be ready..."
    kubectl wait --for=condition=Ready nodes --all --timeout=300s
    
    log_success "Kind cluster created successfully"
}

# Deploy infrastructure services
deploy_infrastructure() {
    log "Deploying infrastructure services..."
    
    # Deploy in specific order to handle dependencies
    local infrastructure_files=(
        "$SCRIPT_DIR/infrastructure/storage-class.yaml"
        "$SCRIPT_DIR/infrastructure/metrics-server.yaml"
        "$SCRIPT_DIR/infrastructure/ingress-nginx.yaml"
        "$SCRIPT_DIR/infrastructure/network-policies.yaml"
    )
    
    for file in "${infrastructure_files[@]}"; do
        if [[ -f "$file" ]]; then
            log "Deploying $(basename "$file")..."
            kubectl apply -f "$file"
        else
            log_error "Infrastructure file not found: $file"
            exit 1
        fi
    done
    
    log_success "Infrastructure services deployed"
}

# Wait for infrastructure to be ready
wait_for_infrastructure() {
    log "Waiting for infrastructure services to be ready..."
    
    # Wait for local-path-provisioner
    log "Waiting for storage provisioner..."
    kubectl wait --for=condition=Available deployment/local-path-provisioner \
        -n local-path-storage --timeout=300s
    
    # Wait for metrics-server
    log "Waiting for metrics server..."
    kubectl wait --for=condition=Available deployment/metrics-server \
        -n kube-system --timeout=300s
    
    # Wait for ingress-nginx
    log "Waiting for ingress controller..."
    kubectl wait --for=condition=Available deployment/ingress-nginx-controller \
        -n ingress-nginx --timeout=300s
    
    log_success "All infrastructure services are ready"
}

# Validate cluster setup
validate_cluster() {
    log "Validating cluster setup..."
    
    # Check nodes
    log "Checking node status..."
    kubectl get nodes -o wide
    
    # Check infrastructure pods
    log "Checking infrastructure pods..."
    kubectl get pods -n local-path-storage
    kubectl get pods -n kube-system | grep metrics-server
    kubectl get pods -n ingress-nginx
    
    # Check storage class
    log "Checking storage class..."
    kubectl get storageclass
    
    # Test metrics server
    log "Testing metrics server..."
    kubectl top nodes || log_warning "Metrics not yet available (this is normal initially)"
    
    log_success "Cluster validation completed"
}

# Run validation scripts if they exist
run_validation_scripts() {
    log "Running validation scripts..."
    
    local validation_dir="$SCRIPT_DIR/validation"
    
    if [[ -f "$validation_dir/test-cluster.sh" ]]; then
        log "Running cluster tests..."
        bash "$validation_dir/test-cluster.sh" || log_warning "Cluster tests had issues"
    fi
    
    if [[ -f "$validation_dir/verify-infrastructure.sh" ]]; then
        log "Running infrastructure verification..."
        bash "$validation_dir/verify-infrastructure.sh" || log_warning "Infrastructure verification had issues"
    fi
}

# Display cluster information
display_cluster_info() {
    log_success "=== Kind Cluster Setup Complete ==="
    echo
    log "Cluster Information:"
    echo "  Cluster Name: $CLUSTER_NAME"
    echo "  Context: kind-$CLUSTER_NAME"
    echo "  Nodes: $(kubectl get nodes --no-headers | wc -l)"
    echo
    log "Access Information:"
    echo "  NodePort Range: 30081-30086 (mapped to localhost)"
    echo "  HTTP Ingress: http://localhost:80"
    echo "  HTTPS Ingress: https://localhost:443"
    echo
    log "Next Steps:"
    echo "  1. Deploy services using: cd ../deployment/"
    echo "  2. Test cluster: ./validation/test-cluster.sh"
    echo "  3. Verify infrastructure: ./validation/verify-infrastructure.sh"
    echo
    log "Useful Commands:"
    echo "  kubectl get pods --all-namespaces"
    echo "  kubectl get nodes -o wide"
    echo "  kind delete cluster --name $CLUSTER_NAME"
    echo
    log "Log file: $LOG_FILE"
}

# Main execution
main() {
    log "Starting Kind-First Microservices Cluster Setup"
    log "Log file: $LOG_FILE"
    
    check_prerequisites
    create_cluster
    deploy_infrastructure
    wait_for_infrastructure
    validate_cluster
    run_validation_scripts
    display_cluster_info
    
    log_success "Setup completed successfully!"
}

# Handle script arguments
case "${1:-}" in
    "cleanup")
        log "Manual cleanup requested"
        cleanup
        exit 0
        ;;
    "validate")
        log "Running validation only"
        validate_cluster
        run_validation_scripts
        exit 0
        ;;
    "help"|"-h"|"--help")
        echo "Usage: $0 [cleanup|validate|help]"
        echo "  cleanup  - Delete the Kind cluster"
        echo "  validate - Run validation checks only"
        echo "  help     - Show this help message"
        echo "  (no args) - Full setup process"
        exit 0
        ;;
    "")
        # No arguments, run main setup
        main
        ;;
    *)
        log_error "Unknown argument: $1"
        echo "Use '$0 help' for usage information"
        exit 1
        ;;
esac 