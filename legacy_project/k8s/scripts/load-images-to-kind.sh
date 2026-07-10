#!/bin/bash

# Load All Docker Images to Kind Cluster
# Date: December 29, 2024
# Purpose: Load all custom Docker images into Kind cluster for deployment

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-microservices-kind}"
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Load counter
LOADS_TOTAL=0
LOADS_SUCCESS=0
LOADS_FAILED=0

echo -e "${BLUE}📦 Loading All Docker Images to Kind Cluster${NC}"
echo "=============================================="
echo "Kind Cluster: ${KIND_CLUSTER_NAME}"

# Check if Kind cluster exists
if ! kind get clusters | grep -q "^${KIND_CLUSTER_NAME}$"; then
    echo -e "${RED}❌ ERROR: Kind cluster '${KIND_CLUSTER_NAME}' not found${NC}"
    echo -e "${YELLOW}💡 Create cluster first: kind create cluster --name ${KIND_CLUSTER_NAME}${NC}"
    exit 1
fi

load_image() {
    local image_name="$1"
    local image_tag="${2:-latest}"
    local full_image="${image_name}:${image_tag}"
    
    echo -e "\n${YELLOW}🚀 Loading ${full_image}...${NC}"
    
    LOADS_TOTAL=$((LOADS_TOTAL + 1))
    
    # Check if image exists locally
    if ! docker images "${full_image}" --format "{{.Repository}}:{{.Tag}}" | grep -q "^${full_image}$"; then
        echo -e "${RED}❌ ERROR: Image ${full_image} not found locally${NC}"
        echo -e "${YELLOW}💡 Build image first: docker build -t ${full_image} .${NC}"
        LOADS_FAILED=$((LOADS_FAILED + 1))
        return 1
    fi
    
    # Load image to Kind cluster
    if kind load docker-image "${full_image}" --name "${KIND_CLUSTER_NAME}"; then
        echo -e "${GREEN}✅ SUCCESS: ${full_image} loaded to Kind cluster${NC}"
        LOADS_SUCCESS=$((LOADS_SUCCESS + 1))
        
        # Verify image is available in cluster
        if kubectl get nodes -o jsonpath='{.items[0].status.images[*].names}' | grep -q "${full_image}"; then
            echo -e "${GREEN}   Image verified in cluster registry${NC}"
        fi
    else
        echo -e "${RED}❌ FAILED: Failed to load ${full_image}${NC}"
        LOADS_FAILED=$((LOADS_FAILED + 1))
        return 1
    fi
}

echo -e "\n${BLUE}📥 Phase 1: Loading Application Services${NC}"
echo "=========================================="

# Load all application service images
load_image "cache-service" "latest"
load_image "storage-service" "latest"
load_image "auth-service" "latest"
load_image "queue-service" "latest"
load_image "profile-service" "latest"

echo -e "\n${BLUE}🔧 Phase 2: Loading Worker Services${NC}"
echo "===================================="

# Load worker service images
load_image "email-worker" "latest"
load_image "image-worker" "latest"

echo -e "\n${BLUE}🔍 Phase 3: Verification${NC}"
echo "========================="

echo -e "\n${YELLOW}📋 Verifying loaded images in Kind cluster...${NC}"

# List all images in the cluster
echo -e "\n${BLUE}Images available in Kind cluster:${NC}"
kubectl get nodes -o jsonpath='{.items[0].status.images[*].names}' | tr ' ' '\n' | grep -E "(cache-service|storage-service|auth-service|queue-service|profile-service|email-worker|image-worker)" | sort | uniq || echo "No custom images found in cluster"

echo -e "\n${BLUE}📊 Load Summary${NC}"
echo "==============="
echo "Total loads attempted: ${LOADS_TOTAL}"
echo -e "${GREEN}Successful loads: ${LOADS_SUCCESS}${NC}"
echo -e "${RED}Failed loads: ${LOADS_FAILED}${NC}"

if [ ${LOADS_FAILED} -eq 0 ]; then
    echo -e "\n${GREEN}🎉 All Docker images loaded successfully to Kind cluster!${NC}"
    echo -e "${YELLOW}📋 Next step: Deploy services with kubectl apply -f k8s/deployment/XX-service-name/${NC}"
    
    echo -e "\n${BLUE}🚀 Quick deployment commands:${NC}"
    echo "kubectl apply -f k8s/deployment/01-cache-service/"
    echo "kubectl apply -f k8s/deployment/02-storage-service/"
    echo "kubectl apply -f k8s/deployment/03-auth-service/"
    echo "kubectl apply -f k8s/deployment/04-queue-service/"
    echo "kubectl apply -f k8s/deployment/05-profile-service/"
    echo "kubectl apply -f k8s/deployment/06-worker-service/"
    
    exit 0
else
    echo -e "\n${RED}❌ Some image loads failed. Please check the errors above.${NC}"
    echo -e "${YELLOW}💡 Make sure all images are built first: ./build-all-images.sh${NC}"
    exit 1
fi 