#!/bin/bash

# Deploy Cache Service to Kind
# Purpose: Automated deployment script for local Kind development
# Usage: ./deploy-to-kind.sh [--cluster-name name] [--build-image] [--reset]

set -euo pipefail

# Configuration
DEFAULT_CLUSTER_NAME="cache-service-dev"
CLUSTER_NAME=${CLUSTER_NAME:-$DEFAULT_CLUSTER_NAME}
BUILD_IMAGE=${BUILD_IMAGE:-false}
RESET_CLUSTER=${RESET_CLUSTER:-false}
IMAGE_NAME="cache-service"
IMAGE_TAG="latest"

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --cluster-name)
            CLUSTER_NAME="$2"
            shift 2
            ;;
        --build-image)
            BUILD_IMAGE=true
            shift
            ;;
        --reset)
            RESET_CLUSTER=true
            shift
            ;;
        *)
            echo "Unknown option: $1"
            echo "Usage: $0 [--cluster-name name] [--build-image] [--reset]"
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

# Prerequisite checks
check_prerequisites() {
    log_header "Checking Prerequisites"
    
    # Check if Kind is installed
    if ! command -v kind >/dev/null 2>&1; then
        log_error "Kind is not installed. Please install it from https://kind.sigs.k8s.io/"
        exit 1
    fi
    log_success "Kind is installed"
    
    # Check if kubectl is installed
    if ! command -v kubectl >/dev/null 2>&1; then
        log_error "kubectl is not installed. Please install it."
        exit 1
    fi
    log_success "kubectl is installed"
    
    # Check if Docker is running
    if ! docker info >/dev/null 2>&1; then
        log_error "Docker is not running. Please start Docker."
        exit 1
    fi
    log_success "Docker is running"
    
    # Check if kustomize is available
    if ! command -v kustomize >/dev/null 2>&1 && ! kubectl version --client | grep -q "kustomize"; then
        log_error "Kustomize is not available. Please install it or use a newer kubectl version."
        exit 1
    fi
    log_success "Kustomize is available"
}

# Create or reset Kind cluster
setup_kind_cluster() {
    log_header "Setting up Kind Cluster"
    
    if [[ "$RESET_CLUSTER" == "true" ]]; then
        log_warning "Resetting cluster '$CLUSTER_NAME'..."
        kind delete cluster --name "$CLUSTER_NAME" 2>/dev/null || true
    fi
    
    # Check if cluster already exists
    if kind get clusters | grep -q "^${CLUSTER_NAME}$"; then
        log_info "Kind cluster '$CLUSTER_NAME' already exists"
    else
        log_info "Creating Kind cluster '$CLUSTER_NAME'..."
        
        # Create Kind configuration for the cluster
        cat <<EOF > /tmp/kind-cache-service.yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: $CLUSTER_NAME
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "ingress-ready=true"
  extraPortMappings:
  - containerPort: 30080
    hostPort: 8080
    protocol: TCP
  - containerPort: 30081
    hostPort: 8081
    protocol: TCP
  - containerPort: 30082
    hostPort: 8082
    protocol: TCP
  - containerPort: 30090
    hostPort: 9090
    protocol: TCP
  - containerPort: 30379
    hostPort: 6379
    protocol: TCP
EOF
        
        kind create cluster --config /tmp/kind-cache-service.yaml
        rm /tmp/kind-cache-service.yaml
        log_success "Kind cluster '$CLUSTER_NAME' created"
    fi
    
    # Set kubectl context
    kubectl cluster-info --context kind-$CLUSTER_NAME
    log_success "Kubectl context set to kind-$CLUSTER_NAME"
}

# Build and load Docker image
build_and_load_image() {
    log_header "Building and Loading Docker Image"
    
    if [[ "$BUILD_IMAGE" == "true" ]]; then
        log_info "Building cache service Docker image..."
        
        # Change to project root directory
        cd "$(dirname "$0")/.."
        
        # Build the Docker image
        docker build -t $IMAGE_NAME:$IMAGE_TAG .
        log_success "Docker image built: $IMAGE_NAME:$IMAGE_TAG"
        
        # Load image into Kind cluster
        log_info "Loading image into Kind cluster..."
        kind load docker-image $IMAGE_NAME:$IMAGE_TAG --name $CLUSTER_NAME
        log_success "Image loaded into Kind cluster"
    else
        log_info "Skipping image build (use --build-image to build)"
        log_warning "Make sure the image '$IMAGE_NAME:$IMAGE_TAG' exists in Kind"
    fi
}

# Deploy cache service using Kustomize
deploy_cache_service() {
    log_header "Deploying Cache Service"
    
    # Change to Kind directory
    cd "$(dirname "$0")"
    
    log_info "Deploying with Kustomize..."
    kubectl apply -k . --context kind-$CLUSTER_NAME
    
    log_success "Cache service deployed to Kind cluster"
}

# Wait for deployments to be ready
wait_for_ready() {
    log_header "Waiting for Services to be Ready"
    
    log_info "Waiting for Redis development instance..."
    kubectl wait --for=condition=available deployment/kind-redis-dev \
        --timeout=120s --context kind-$CLUSTER_NAME || {
        log_error "Redis deployment failed to become ready"
        return 1
    }
    log_success "Redis is ready"
    
    log_info "Waiting for cache service..."
    kubectl wait --for=condition=available deployment/kind-cache-service \
        --timeout=180s --context kind-$CLUSTER_NAME || {
        log_error "Cache service deployment failed to become ready"
        return 1
    }
    log_success "Cache service is ready"
    
    log_info "Waiting for Redis Commander (optional)..."
    kubectl wait --for=condition=available deployment/kind-redis-commander \
        --timeout=60s --context kind-$CLUSTER_NAME 2>/dev/null || {
        log_warning "Redis Commander not ready (this is optional)"
    }
}

# Test deployment
test_deployment() {
    log_header "Testing Deployment"
    
    log_info "Testing cache service health..."
    
    # Test health endpoint
    local max_attempts=30
    local attempt=1
    
    while [[ $attempt -le $max_attempts ]]; do
        if curl -s http://localhost:8080/health >/dev/null 2>&1; then
            log_success "Health endpoint is responding"
            break
        else
            log_info "Attempt $attempt/$max_attempts: Waiting for health endpoint..."
            sleep 2
            ((attempt++))
        fi
    done
    
    if [[ $attempt -gt $max_attempts ]]; then
        log_error "Health endpoint not responding after $max_attempts attempts"
        return 1
    fi
    
    # Test basic cache operations
    log_info "Testing basic cache operations..."
    
    # Set a test key
    if curl -s -X POST http://localhost:8080/api/v1/cache \
            -H "Content-Type: application/json" \
            -d '{"key":"kind-test","value":"kind-deployment-success","ttl":"1h"}' >/dev/null; then
        log_success "Cache SET operation successful"
    else
        log_error "Cache SET operation failed"
        return 1
    fi
    
    # Get the test key
    if curl -s http://localhost:8080/api/v1/cache/kind-test | grep -q "kind-deployment-success"; then
        log_success "Cache GET operation successful"
    else
        log_error "Cache GET operation failed"
        return 1
    fi
    
    log_success "All tests passed!"
}

# Display connection information
show_connection_info() {
    log_header "Connection Information"
    
    log_info "🌐 Service Endpoints:"
    echo "   Cache Service HTTP:  http://localhost:8080"
    echo "   Cache Service gRPC:  localhost:9090"
    echo "   Cache Metrics:       http://localhost:8081/metrics"
    echo "   Redis Commander:     http://localhost:8082 (admin/admin)"
    echo "   Redis Direct:        localhost:6379 (password: dev-redis-password)"
    
    log_info "🔧 Development Commands:"
    echo "   Health Check:        curl http://localhost:8080/health"
    echo "   Cache Test:          curl -X POST http://localhost:8080/api/v1/cache -H 'Content-Type: application/json' -d '{\"key\":\"test\",\"value\":\"hello\",\"ttl\":\"1h\"}'"
    echo "   View Logs:           kubectl logs -f deployment/kind-cache-service"
    echo "   Redis CLI:           kubectl exec -it deployment/kind-redis-dev -- redis-cli -a dev-redis-password"
    
    log_info "🗑️ Cleanup Commands:"
    echo "   Delete Deployment:   kubectl delete -k kind/"
    echo "   Delete Cluster:      kind delete cluster --name $CLUSTER_NAME"
    
    log_info "📚 Documentation:"
    echo "   Deployment Guide:    deployments/STEP_BY_STEP_DEPLOYMENT_GUIDE.md"
    echo "   API Documentation:   api/openapi.yaml"
}

# Main deployment flow
main() {
    log_header "🚀 Cache Service Kind Deployment"
    
    echo "Cluster: $CLUSTER_NAME"
    echo "Image: $IMAGE_NAME:$IMAGE_TAG"
    echo "Build Image: $BUILD_IMAGE"
    echo "Reset Cluster: $RESET_CLUSTER"
    echo ""
    
    # Execute deployment steps
    check_prerequisites
    setup_kind_cluster
    build_and_load_image
    deploy_cache_service
    wait_for_ready
    test_deployment
    show_connection_info
    
    log_success "🎉 Cache Service successfully deployed to Kind!"
    log_info "Your cache service is now running and ready for development."
}

# Handle script interruption
trap 'echo -e "\n${RED}❌ Deployment interrupted${NC}"; exit 1' INT TERM

# Run main function
main "$@" 