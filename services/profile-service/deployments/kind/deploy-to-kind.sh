#!/bin/bash

# Deploy Profile Service to Kind Cluster
# Usage: ./deploy-to-kind.sh [COMMAND]

set -euo pipefail

# Configuration
KIND_CLUSTER_NAME=${KIND_CLUSTER_NAME:-profile-service-dev}
DOCKER_IMAGE=${DOCKER_IMAGE:-profile-service:latest}

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
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

check_prerequisites() {
    log_info "Checking prerequisites..."
    
    # Check if kind is installed
    if ! command -v kind &> /dev/null; then
        log_error "kind is not installed. Please install kind first: https://kind.sigs.k8s.io/docs/user/quick-start/"
        exit 1
    fi
    
    # Check if kubectl is installed
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl is not installed. Please install kubectl first."
        exit 1
    fi
    
    # Check if docker is running
    if ! docker info &> /dev/null; then
        log_error "Docker is not running. Please start Docker first."
        exit 1
    fi
    
    # Check if kustomize is available
    if ! command -v kustomize &> /dev/null; then
        log_warning "kustomize not found, will use kubectl kustomize instead"
    fi
    
    log_success "Prerequisites check passed"
}

create_kind_cluster() {
    log_info "Creating kind cluster: $KIND_CLUSTER_NAME"
    
    # Check if cluster already exists
    if kind get clusters | grep -q "^${KIND_CLUSTER_NAME}$"; then
        log_info "Cluster $KIND_CLUSTER_NAME already exists"
        return 0
    fi
    
    # Create kind cluster with port mapping for NodePort services
    cat <<EOF | kind create cluster --name "$KIND_CLUSTER_NAME" --config=-
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
  - containerPort: 30080  # Profile service HTTP
    hostPort: 8080
    protocol: TCP
  - containerPort: 30081  # Profile service metrics
    hostPort: 8081
    protocol: TCP
EOF
    
    log_success "Kind cluster created successfully"
}

build_and_load_image() {
    log_info "Building and loading Docker image..."
    
    # Build the Docker image
    log_info "Building Docker image: $DOCKER_IMAGE"
    cd ../../  # Go to project root
    docker build -t "$DOCKER_IMAGE" .
    cd deployments/kind/  # Return to kind directory
    
    # Load image into kind cluster
    log_info "Loading image into kind cluster"
    kind load docker-image "$DOCKER_IMAGE" --name "$KIND_CLUSTER_NAME"
    
    log_success "Docker image built and loaded successfully"
}

deploy_application() {
    log_info "Deploying Profile Service to kind cluster..."
    
    # Set kubectl context to kind cluster
    kubectl config use-context "kind-${KIND_CLUSTER_NAME}"
    
    # Apply manifests using kustomize
    if command -v kustomize &> /dev/null; then
        kustomize build . | kubectl apply -f -
    else
        kubectl apply -k .
    fi
    
    log_success "Application deployed successfully"
}

wait_for_deployment() {
    log_info "Waiting for deployment to be ready..."
    
    # Wait for deployment to be ready
    kubectl wait --for=condition=available deployment/profile-service --timeout=300s
    
    # Wait for pods to be ready
    kubectl wait --for=condition=ready pod -l app=profile-service --timeout=300s
    
    log_success "Deployment is ready"
}

show_access_info() {
    log_info "Deployment completed successfully!"
    echo
    log_info "Access Information:"
    echo "  Profile Service API: http://localhost:8080"
    echo "  Health Check: http://localhost:8080/health"
    echo "  Metrics: http://localhost:8081/metrics"
    echo
    log_info "Useful Commands:"
    echo "  View pods: kubectl get pods -l app=profile-service"
    echo "  View logs: kubectl logs -f deployment/profile-service"
    echo "  Port forward: kubectl port-forward svc/profile-service 8080:8080"
    echo "  Delete cluster: kind delete cluster --name $KIND_CLUSTER_NAME"
}

cleanup() {
    log_info "Cleaning up kind cluster..."
    kind delete cluster --name "$KIND_CLUSTER_NAME"
    log_success "Cluster deleted successfully"
}

deploy() {
    check_prerequisites
    create_kind_cluster
    build_and_load_image
    deploy_application
    wait_for_deployment
    show_access_info
}

# Command handling
case "${1:-deploy}" in
    "deploy")
        deploy
        ;;
    "cleanup" | "delete")
        cleanup
        ;;
    "build")
        check_prerequisites
        build_and_load_image
        ;;
    "logs")
        kubectl logs -f deployment/profile-service
        ;;
    "status")
        kubectl get pods,svc -l app=profile-service
        ;;
    "help" | "-h" | "--help")
        echo "Usage: $0 [COMMAND]"
        echo ""
        echo "Commands:"
        echo "  deploy    Create kind cluster and deploy application (default)"
        echo "  cleanup   Delete the kind cluster"
        echo "  build     Build and load Docker image only"
        echo "  logs      Show application logs"
        echo "  status    Show deployment status"
        echo "  help      Show this help message"
        echo ""
        echo "Environment Variables:"
        echo "  KIND_CLUSTER_NAME   Name of the kind cluster (default: profile-service-dev)"
        echo "  DOCKER_IMAGE        Docker image name (default: profile-service:latest)"
        ;;
    *)
        log_error "Unknown command: $1"
        echo "Use '$0 help' for usage information"
        exit 1
        ;;
esac 