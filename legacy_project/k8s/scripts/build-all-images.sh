#!/bin/bash

# Build All Docker Images for Kind Deployment
# Date: December 29, 2024
# Purpose: Build all custom Docker images required for microservices deployment

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Build counter
BUILDS_TOTAL=0
BUILDS_SUCCESS=0
BUILDS_FAILED=0

echo -e "${BLUE}🔨 Building All Docker Images for Kind Deployment${NC}"
echo "=================================================="

build_image() {
    local service_name="$1"
    local service_path="$2"
    local dockerfile="${3:-Dockerfile}"
    local image_tag="${4:-${service_name}:latest}"
    
    echo -e "\n${YELLOW}📦 Building ${service_name}...${NC}"
    echo "Service Path: ${service_path}"
    echo "Dockerfile: ${dockerfile}"
    echo "Image Tag: ${image_tag}"
    
    BUILDS_TOTAL=$((BUILDS_TOTAL + 1))
    
    if [ ! -d "${service_path}" ]; then
        echo -e "${RED}❌ ERROR: Service directory not found: ${service_path}${NC}"
        BUILDS_FAILED=$((BUILDS_FAILED + 1))
        return 1
    fi
    
    if [ ! -f "${service_path}/${dockerfile}" ]; then
        echo -e "${RED}❌ ERROR: Dockerfile not found: ${service_path}/${dockerfile}${NC}"
        BUILDS_FAILED=$((BUILDS_FAILED + 1))
        return 1
    fi
    
    # Build the image
    if docker build -f "${service_path}/${dockerfile}" -t "${image_tag}" "${service_path}"; then
        echo -e "${GREEN}✅ SUCCESS: ${service_name} built successfully${NC}"
        BUILDS_SUCCESS=$((BUILDS_SUCCESS + 1))
        
        # Verify image exists
        if docker images "${image_tag}" --format "table {{.Repository}}:{{.Tag}}\t{{.Size}}\t{{.CreatedAt}}" | grep -v "REPOSITORY"; then
            echo -e "${GREEN}   Image verified in local registry${NC}"
        fi
    else
        echo -e "${RED}❌ FAILED: ${service_name} build failed${NC}"
        BUILDS_FAILED=$((BUILDS_FAILED + 1))
        return 1
    fi
}

# Cross-platform compilation for Go services
cross_compile_go_service() {
    local service_name="$1"
    local service_path="$2"
    local binary_name="$3"
    
    echo -e "\n${YELLOW}🔧 Cross-compiling ${service_name} for Linux/AMD64...${NC}"
    
    if [ ! -f "${service_path}/go.mod" ]; then
        echo -e "${YELLOW}⚠️  No go.mod found, skipping cross-compilation for ${service_name}${NC}"
        return 0
    fi
    
    cd "${service_path}"
    
    # Cross-compile for Linux
    if GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o "${binary_name}-linux" ./cmd/main.go; then
        echo -e "${GREEN}✅ Cross-compilation successful: ${binary_name}-linux${NC}"
    else
        echo -e "${RED}❌ Cross-compilation failed for ${service_name}${NC}"
        return 1
    fi
    
    cd "${PROJECT_ROOT}"
}

echo -e "\n${BLUE}🏗️  Phase 1: Cross-Platform Compilation${NC}"
echo "========================================"

# Cross-compile Go services (if needed)
if [ -d "${PROJECT_ROOT}/services/worker-service/services/workers/email-worker" ]; then
    cross_compile_go_service "email-worker" "${PROJECT_ROOT}/services/worker-service/services/workers/email-worker" "email-worker"
fi

if [ -d "${PROJECT_ROOT}/services/worker-service/services/workers/image-worker" ]; then
    cross_compile_go_service "image-worker" "${PROJECT_ROOT}/services/worker-service/services/workers/image-worker" "image-worker"
fi

echo -e "\n${BLUE}🔨 Phase 2: Docker Image Builds${NC}"
echo "================================="

# 1. Cache Service
build_image "cache-service" "${PROJECT_ROOT}/services/cache-service"

# 2. Storage Service
build_image "storage-service" "${PROJECT_ROOT}/services/storage-service"

# 3. Auth Service
build_image "auth-service" "${PROJECT_ROOT}/services/auth-service"

# 4. Queue Service
build_image "queue-service" "${PROJECT_ROOT}/services/queue-service"

# 5. Profile Service
build_image "profile-service" "${PROJECT_ROOT}/services/profile-service"

# 6. Worker Services (Multi-worker architecture)
if [ -d "${PROJECT_ROOT}/services/worker-service/services/workers/email-worker" ]; then
    build_image "email-worker" "${PROJECT_ROOT}/services/worker-service/services/workers/email-worker" "Dockerfile.kind" "email-worker:latest"
fi

if [ -d "${PROJECT_ROOT}/services/worker-service/services/workers/image-worker" ]; then
    build_image "image-worker" "${PROJECT_ROOT}/services/worker-service/services/workers/image-worker" "Dockerfile.kind" "image-worker:latest"
fi

echo -e "\n${BLUE}📊 Build Summary${NC}"
echo "================="
echo "Total builds attempted: ${BUILDS_TOTAL}"
echo -e "${GREEN}Successful builds: ${BUILDS_SUCCESS}${NC}"
echo -e "${RED}Failed builds: ${BUILDS_FAILED}${NC}"

if [ ${BUILDS_FAILED} -eq 0 ]; then
    echo -e "\n${GREEN}🎉 All Docker images built successfully!${NC}"
    echo -e "${YELLOW}📋 Next step: Run ./load-images-to-kind.sh to load images into Kind cluster${NC}"
    exit 0
else
    echo -e "\n${RED}❌ Some builds failed. Please check the errors above.${NC}"
    exit 1
fi 