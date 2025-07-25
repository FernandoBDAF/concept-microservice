#!/bin/bash

# Storage Service - Kind Deployment Script
# This script deploys the Storage Service to a Kind cluster with all dependencies

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
CLUSTER_NAME="microservices"
NAMESPACE="default"
STORAGE_SERVICE_IMAGE="storage-service:dev-latest"
TIMEOUT=300

# Helper functions
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

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    # Check if kind is installed
    if ! command -v kind &> /dev/null; then
        log_error "kind is not installed. Please install kind: https://kind.sigs.k8s.io/docs/user/quick-start/"
        exit 1
    fi
    
    # Check if kubectl is installed
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl is not installed. Please install kubectl"
        exit 1
    fi
    
    # Check if docker is running
    if ! docker info &> /dev/null; then
        log_error "Docker is not running. Please start Docker"
        exit 1
    fi
    
    log_success "Prerequisites check passed"
}

# Create or verify Kind cluster
setup_kind_cluster() {
    log_info "Setting up Kind cluster..."
    
    # Check if cluster already exists
    if kind get clusters | grep -q "^${CLUSTER_NAME}$"; then
        log_info "Kind cluster '${CLUSTER_NAME}' already exists"
        
        # Set kubectl context
        kubectl cluster-info --context "kind-${CLUSTER_NAME}" > /dev/null
        if [ $? -eq 0 ]; then
            log_success "Cluster is accessible"
        else
            log_warning "Cluster exists but not accessible, recreating..."
            kind delete cluster --name "${CLUSTER_NAME}"
            create_cluster
        fi
    else
        create_cluster
    fi
}

create_cluster() {
    log_info "Creating Kind cluster '${CLUSTER_NAME}'..."
    
    cat <<EOF | kind create cluster --name "${CLUSTER_NAME}" --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "ingress-ready=true"
  extraPortMappings:
  - containerPort: 80
    hostPort: 80
    protocol: TCP
  - containerPort: 443
    hostPort: 443
    protocol: TCP
  - containerPort: 30000
    hostPort: 30000
    protocol: TCP
- role: worker
- role: worker
EOF
    
    # Wait for cluster to be ready
    log_info "Waiting for cluster to be ready..."
    kubectl wait --for=condition=Ready nodes --all --timeout=${TIMEOUT}s
    
    log_success "Kind cluster created successfully"
}

# Build and load Docker image
build_and_load_image() {
    log_info "Building and loading Storage Service image..."
    
    # Build image (assuming we're in the project root)
    if [ -f "../../Dockerfile" ]; then
        log_info "Building Docker image..."
        docker build -t "${STORAGE_SERVICE_IMAGE}" -f ../../Dockerfile ../../
        
        # Load image into Kind cluster
        log_info "Loading image into Kind cluster..."
        kind load docker-image "${STORAGE_SERVICE_IMAGE}" --name "${CLUSTER_NAME}"
        
        log_success "Image built and loaded successfully"
    else
        log_warning "Dockerfile not found at ../../Dockerfile"
        log_warning "Assuming image ${STORAGE_SERVICE_IMAGE} already exists in Kind cluster"
    fi
}

# Deploy dependencies
deploy_dependencies() {
    log_info "Deploying dependencies (PostgreSQL, RabbitMQ)..."
    
    # Apply dependency manifests
    kubectl apply -f storage-dependencies.yaml
    
    # Wait for PostgreSQL to be ready
    log_info "Waiting for PostgreSQL to be ready..."
    kubectl wait --for=condition=available deployment/postgres-dev --timeout=${TIMEOUT}s
    
    # Wait for RabbitMQ to be ready
    log_info "Waiting for RabbitMQ to be ready..."
    kubectl wait --for=condition=available deployment/rabbitmq-dev --timeout=${TIMEOUT}s
    
    log_success "Dependencies deployed successfully"
}

# Deploy Storage Service
deploy_storage_service() {
    log_info "Deploying Storage Service..."
    
    # Apply Kustomized manifests
    kubectl apply -k .
    
    # Wait for deployment to be ready
    log_info "Waiting for Storage Service to be ready..."
    kubectl wait --for=condition=available deployment/storage-service-dev --timeout=${TIMEOUT}s
    
    log_success "Storage Service deployed successfully"
}

# Verify deployment
verify_deployment() {
    log_info "Verifying deployment..."
    
    # Check pod status
    log_info "Checking pod status..."
    kubectl get pods -l app=storage-service
    
    # Check service status
    log_info "Checking service status..."
    kubectl get services -l app=storage-service
    
    # Test health endpoint
    log_info "Testing health endpoint..."
    kubectl port-forward svc/storage-service-dev 8080:8080 &
    PORT_FORWARD_PID=$!
    
    # Wait for port forward to establish
    sleep 5
    
    # Test health endpoint
    if curl -f -s http://localhost:8080/health > /dev/null; then
        log_success "Health endpoint is accessible"
    else
        log_warning "Health endpoint test failed"
    fi
    
    # Clean up port forward
    kill $PORT_FORWARD_PID 2>/dev/null || true
    
    log_success "Deployment verification completed"
}

# Show access information
show_access_info() {
    log_info "Deployment completed successfully!"
    echo
    echo "Access Information:"
    echo "=================="
    echo
    echo "1. Check pod status:"
    echo "   kubectl get pods -l app=storage-service"
    echo
    echo "2. View logs:"
    echo "   kubectl logs -l app=storage-service -f"
    echo
    echo "3. Access service locally:"
    echo "   kubectl port-forward svc/storage-service-dev 8080:8080"
    echo "   curl http://localhost:8080/health"
    echo
    echo "4. Access PostgreSQL (for debugging):"
    echo "   kubectl port-forward svc/postgres-dev 5432:5432"
    echo
    echo "5. Access RabbitMQ Management (for debugging):"
    echo "   kubectl port-forward svc/rabbitmq-dev 15672:15672"
    echo "   http://localhost:15672 (admin/password)"
    echo
    echo "6. Run tests:"
    echo "   cd ../../tests && go test -v ./..."
    echo
}

# Cleanup function
cleanup() {
    log_info "Cleaning up port forwards..."
    pkill -f "kubectl port-forward" 2>/dev/null || true
}

# Main execution
main() {
    log_info "Starting Storage Service deployment to Kind cluster..."
    
    # Set up cleanup trap
    trap cleanup EXIT
    
    # Execute deployment steps
    check_prerequisites
    setup_kind_cluster
    build_and_load_image
    deploy_dependencies
    deploy_storage_service
    verify_deployment
    show_access_info
    
    log_success "Storage Service deployment completed successfully!"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --cluster-name)
            CLUSTER_NAME="$2"
            shift 2
            ;;
        --namespace)
            NAMESPACE="$2"
            shift 2
            ;;
        --image)
            STORAGE_SERVICE_IMAGE="$2"
            shift 2
            ;;
        --timeout)
            TIMEOUT="$2"
            shift 2
            ;;
        --help)
            echo "Usage: $0 [options]"
            echo
            echo "Options:"
            echo "  --cluster-name NAME    Kind cluster name (default: microservices)"
            echo "  --namespace NAMESPACE  Kubernetes namespace (default: default)"
            echo "  --image IMAGE         Storage service image (default: storage-service:dev-latest)"
            echo "  --timeout SECONDS     Timeout for operations (default: 300)"
            echo "  --help                Show this help message"
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Run main function
main "$@" 