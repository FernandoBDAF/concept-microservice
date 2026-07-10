# Services Deployment Guide - Kind Microservices

**Date**: December 29, 2024  
**Purpose**: Comprehensive educational guide for deploying individual microservices  
**Audience**: Developers learning microservices architecture and Kubernetes deployment  
**Scope**: Individual service deployment with detailed explanations and testing procedures

---

## 📋 **Table of Contents**

### **🎯 Quick Navigation**

- [📚 Educational Overview](#-educational-overview)
- [🏗️ Service Architecture Overview](#-service-architecture-overview)
- [🚀 Quick Start Commands](#-quick-start-commands)
- [🚀 Service 1: Cache Service (Redis-backed)](#-service-1-cache-service-redis-backed)
- [🗄️ Service 2: Storage Service (PostgreSQL-backed)](#-service-2-storage-service-postgresql-backed)
- [🔐 Service 3: Auth Service (JWT-based)](#-service-3-auth-service-jwt-based)
- [📬 Service 4: Queue Service (RabbitMQ-backed)](#-service-4-queue-service-rabbitmq-backed)
- [🎭 Service 5: Profile Service (Orchestrator)](#-service-5-profile-service-orchestrator)
- [⚙️ Service 6: Worker Service (Multi-Worker Processing)](#-service-6-worker-service-multi-worker-processing)
- [🧪 Testing Strategy Overview](#-testing-strategy-overview)
- [🔧 Troubleshooting and Debugging](#-troubleshooting-and-debugging)

### **📊 Quick Reference Sections**

- [Service Dependencies Matrix](#service-dependencies)
- [Complete Deployment Sequence](#complete-deployment-sequence)
- [Verification Commands](#verification-commands)
- [Troubleshooting Quick Reference](#common-issues-and-solutions)
- [Deployment Checklist](#deployment-checklist)

### **🔧 Service-Specific Guides**

- [Cache Service Guide](#cache-service-guide) - Redis deployment and testing
- [Storage Service Guide](#storage-service-guide) - PostgreSQL setup and validation
- [Auth Service Guide](#auth-service-guide) - JWT authentication implementation
- [Queue Service Guide](#queue-service-guide) - RabbitMQ messaging setup
- [Profile Service Guide](#profile-service-guide) - Orchestrator deployment
- [Worker Service Guide](#worker-service-guide) - Multi-worker processing

---

## 📚 **Educational Overview**

This guide provides step-by-step instructions for deploying each microservice in the Kind-first microservices architecture. Each service section includes:

- **Architectural context** and service responsibilities
- **Detailed deployment procedures** with educational explanations
- **Comprehensive testing strategies** for validation
- **Troubleshooting guidance** based on real deployment experience
- **Integration points** with other services

### **🎯 Learning Objectives**

By following this guide, you will understand:

1. **Microservices Deployment Patterns**: How to deploy services with proper dependencies
2. **Kubernetes Resource Management**: StatefulSets, Deployments, Services, ConfigMaps, Secrets
3. **Service Integration**: How services communicate and depend on each other
4. **Testing Strategies**: From basic health checks to comprehensive integration testing
5. **Troubleshooting**: Common issues and their resolution patterns

---

## 🏗️ **Service Architecture Overview**

The microservices are deployed in dependency order to ensure proper service integration:

```
1. Cache Service (Redis) ←─── Foundation Layer
2. Storage Service (PostgreSQL) ←─── Data Layer
3. Auth Service (JWT + PostgreSQL) ←─── Security Layer with Database
4. Queue Service (RabbitMQ) ←─── Messaging Layer
5. Profile Service (Orchestrator) ←─── Business Logic Layer
6. Worker Service (Processors) ←─── Processing Layer
```

### **Service Dependencies**

- **Cache Service**: No dependencies (foundation)
- **Storage Service**: No dependencies (foundation)
- **Auth Service**: Self-contained with own PostgreSQL database
- **Queue Service**: No direct dependencies (messaging backbone)
- **Profile Service**: Depends on Auth, Cache, Storage, Queue services
- **Worker Service**: Depends on Queue Service for task consumption

---

## 🚀 **Quick Start Commands**

### **Complete Deployment Sequence**

```bash
# 1. Setup foundation cluster
cd k8s/cluster/
./setup-cluster.sh

# 2. Deploy services in dependency order
cd ../deployment/

# Cache Service (Foundation Layer)
cd 01-cache-service/
kubectl apply -f .
kubectl wait --for=condition=Available deployment/cache-service --timeout=300s
curl http://localhost:30081/health

# Storage Service (Data Layer)
cd ../02-storage-service/
kubectl apply -f .
kubectl wait --for=condition=Available deployment/storage-service --timeout=300s
curl http://localhost:30082/health

# Auth Service (Security Layer)
cd ../03-auth-service/
kubectl apply -f .
kubectl wait --for=condition=Available deployment/auth-service --timeout=300s
curl http://localhost:30083/health

# Queue Service (Messaging Layer)
cd ../04-queue-service/
kubectl apply -f .
kubectl wait --for=condition=Available deployment/queue-service --timeout=300s
curl http://localhost:30084/health

# Profile Service (Business Logic Layer)
# CRITICAL: Build Docker image first
cd ../../../services/profile-service/
docker build -t profile-service:latest .
kind load docker-image profile-service:latest --name microservices-kind
cd ../../k8s/deployment/05-profile-service/
kubectl apply -f .
kubectl wait --for=condition=Available deployment/profile-service --timeout=300s
curl http://localhost:30085/health

# Worker Service (Processing Layer)
# CRITICAL: Build Docker image with cross-platform binaries
cd ../../../services/worker-service/
GOOS=linux GOARCH=amd64 go build -o email-worker-linux cmd/email-worker/main.go
GOOS=linux GOARCH=amd64 go build -o image-worker-linux cmd/image-worker/main.go
docker build -t worker-service:latest .
kind load docker-image worker-service:latest --name microservices-kind
cd ../../k8s/deployment/06-worker-service/
kubectl apply -f .
kubectl wait --for=condition=Available deployment/email-worker --timeout=300s
kubectl wait --for=condition=Available deployment/image-worker --timeout=300s
curl http://localhost:30086/health
```

### **Verification Commands**

```bash
# Check all services status
kubectl get pods,svc --all-namespaces | grep -E "(cache|storage|auth|queue|profile|worker)"

# Test all service endpoints
for port in 30081 30082 30083 30084 30085 30086; do
  echo "Testing service on port $port:"
  curl -s http://localhost:$port/health | jq .status
done

# Check resource usage
kubectl top pods
kubectl top nodes

# Run comprehensive integration tests
cd k8s/scripts/integration/
./comprehensive-integration-test.sh
```

### **Useful Development Commands**

```bash
# Monitor all service logs
kubectl logs -f deployment/cache-service &
kubectl logs -f deployment/storage-service &
kubectl logs -f deployment/auth-service &
kubectl logs -f deployment/queue-service &
kubectl logs -f deployment/profile-service &
kubectl logs -f deployment/email-worker &
kubectl logs -f deployment/image-worker &

# Access cluster dashboard
kubectl proxy

# Complete reset (nuclear option)
kind delete cluster --name microservices-kind
cd k8s/cluster/
./setup-cluster.sh
```

---

## 🚀 **Service 1: Cache Service (Redis-backed)** {#cache-service-guide}

### **🎯 Service Overview**

The Cache Service provides high-performance, Redis-backed caching capabilities for the entire microservices ecosystem. It implements the cache-aside pattern and serves as the foundation for performance optimization across all services.

**Key Characteristics**:

- **Technology**: Redis 7.x with persistence
- **Access Pattern**: REST API + direct Redis access
- **Port**: NodePort 30081 (external), ClusterIP 8080 (internal)
- **Storage**: StatefulSet with persistent volumes
- **Dependencies**: None (foundation service)

### **📋 Prerequisites**

Before deploying the Cache Service, ensure:

```bash
# Verify cluster is ready
kubectl cluster-info --context kind-microservices-kind

# Check available storage classes
kubectl get storageclass

# Verify no conflicts on port 30081
kubectl get svc --all-namespaces | grep 30081
```

### **🏗️ Architecture Components**

#### **Redis StatefulSet** (`redis-statefulset.yaml`)

```yaml
# Persistent Redis instance with:
# - 1Gi persistent storage
# - Redis 7.x with optimized configuration
# - Headless service for stable network identity
# - Resource limits: 256Mi memory, 200m CPU
```

#### **Cache Service Deployment** (`deployment.yaml`)

```yaml
# Application service with:
# - Single replica (Kind optimization)
# - Security contexts (non-root user)
# - Resource requests/limits
# - Health and readiness probes
# - Environment variables for Redis connection
```

#### **Service Definitions** (`service.yaml`)

```yaml
# Dual service setup:
# - NodePort 30081: External access for testing
# - ClusterIP 8080: Internal service-to-service communication
# - Metrics endpoint on port 8081
```

### **🚀 Deployment Procedure**

#### **Step 1: Deploy Redis Backend**

```bash
# Navigate to cache service directory
cd k8s/deployment/01-cache-service/

# Deploy Redis StatefulSet first (dependency)
# (it won't work withou first deploying secrets.yaml)
kubectl apply -f redis-statefulset.yaml
# REVIEW: WE NEED TO DEPLOY SECRETS 1ST, OTHERWISE IT DO NOT WORK 

# Wait for Redis to be ready (critical for service startup)
kubectl wait --for=condition=Ready pod/redis-0 --timeout=300s

# Verify Redis is accessible
# REVIEW: no pong recieved - NOAUTH Authentication required.
kubectl exec redis-0 -- redis-cli ping
# Expected output: PONG
# got "NOAUTH Authentication required." instead
```

**Educational Note**: Redis is deployed as a StatefulSet because it requires persistent storage and stable network identity. The cache service depends on Redis being fully ready before it can start.

#### **Step 2: Deploy Cache Service Application**

```bash
# Deploy configuration and secrets
kubectl apply -f configmap.yaml
kubectl apply -f secrets.yaml  # If exists

# Deploy the application
kubectl apply -f deployment.yaml
kubectl apply -f service.yaml

# Alternative: Deploy all at once (after Redis is ready)
kubectl apply -f .
```

#### **Step 3: Verify Deployment**

```bash
# Check pod status (should show Running and Ready)
kubectl get pods -l app=cache-service
kubectl get pods -l app=redis

# Check service endpoints
kubectl get svc cache-service
kubectl describe svc cache-service

# Check persistent storage
kubectl get pvc -l app=redis
kubectl describe pvc redis-data-redis-0
```

### **🧪 Comprehensive Testing**

#### **Automated Testing**

```bash
# Run the comprehensive test suite
cd k8s/scripts/services/
./test-01-cache-service.sh

# Expected output:
# ✅ Health checks pass
# ✅ Cache operations (SET/GET/DELETE) work
# ✅ TTL functionality verified
# ✅ Error handling tested
# ✅ Performance benchmarks completed
```

#### **Manual API Testing**

**Basic Cache Operations**:

```bash
# Test SET operation (create cache entry)
curl -X POST http://localhost:30081/api/v1/cache/test-key \
  -H "Content-Type: application/json" \
  -d '{"value": "Hello, Cache!", "ttl": 300}'

# Expected response: {"status": "success", "key": "test-key"}

# Test GET operation (retrieve cache entry)
curl -X GET http://localhost:30081/api/v1/cache/test-key

# Expected response: {"key": "test-key", "value": "Hello, Cache!", "ttl": 298}

# Test DELETE operation (remove cache entry)
curl -X DELETE http://localhost:30081/api/v1/cache/test-key

# Expected response: {"status": "deleted", "key": "test-key"}

# Verify deletion (should return 404)
curl -X GET http://localhost:30081/api/v1/cache/test-key
```

**Health and Monitoring**:

```bash
# Health check endpoint
curl http://localhost:30081/health
# Expected: {"status": "healthy", "redis": "connected"}

# Readiness check
curl http://localhost:30081/ready
# Expected: {"status": "ready", "dependencies": ["redis"]}

# Internal metrics (requires network policy compliance)
# this one is not working
# REVIEW: NOT WORKING WELL
kubectl run debug-metrics-$(date +%s) --image=busybox:1.35 --rm -i --restart=Never \
  --labels="app=cache-service" \
  -- timeout 10s wget -qO- http://cache-service:8081/metrics
```

### **🔍 Troubleshooting Guide**

#### **Common Issues and Solutions**

**Issue**: Cache service pod in `CrashLoopBackOff`

```bash
# Check logs for Redis connection issues
kubectl logs deployment/cache-service

# Common causes:
# 1. Redis not ready - wait for Redis pod
# 2. Network policies blocking access
# 3. Configuration errors in environment variables
```

**Issue**: NodePort not accessible

```bash
# Verify Kind port mapping
docker port microservices-kind-control-plane | grep 30081

# If missing, recreate cluster with proper port mapping
# Check kind-config.yaml for extraPortMappings
```

**Issue**: Performance issues

```bash
# Check resource usage
kubectl top pods -l app=cache-service
kubectl top pods -l app=redis

# Check Redis performance
kubectl exec redis-0 -- redis-cli info stats
```

### **✅ Success Criteria**

- [ ] Redis StatefulSet running and ready
- [ ] Cache service deployment healthy
- [ ] NodePort 30081 accessible externally
- [ ] ClusterIP service accessible internally
- [ ] All API endpoints responding correctly
- [ ] Test suite passes with >90% success rate
- [ ] Persistent storage working (data survives pod restart)

---

## 🗄️ **Service 2: Storage Service (PostgreSQL-backed)** {#storage-service-guide}

### **🎯 Service Overview**

The Storage Service provides persistent data storage capabilities using PostgreSQL. It serves as the primary database for user profiles, authentication data, and business entities across the microservices ecosystem.

**Key Characteristics**:

- **Technology**: PostgreSQL 15.x with persistent storage
- **Access Pattern**: REST API + gRPC (dual protocol)
- **Ports**: NodePort 30082 (HTTP), NodePort 30092 (gRPC)
- **Storage**: StatefulSet with persistent volumes
- **Dependencies**: None (foundation service)

### **📋 Prerequisites**

```bash
# Verify cluster resources
kubectl get nodes -o wide
kubectl get storageclass

# Check for port conflicts
kubectl get svc --all-namespaces | grep -E "(30082|30092)"

# Verify sufficient resources (PostgreSQL requires more resources)
kubectl describe nodes | grep -A 5 "Allocated resources"
```

### **🏗️ Architecture Components**

#### **PostgreSQL StatefulSet** (`postgres-statefulset.yaml`)

```yaml
# Production-ready PostgreSQL with:
# - 2Gi persistent storage (larger than Redis)
# - Custom configuration for microservices workload
# - Initialization scripts for schema setup
# - Resource limits: 512Mi memory, 300m CPU
# - Health checks for database readiness
```

#### **Storage Service Deployment** (`deployment.yaml`)

```yaml
# Dual-protocol service with:
# - HTTP REST API (primary interface)
# - gRPC API (high-performance interface)
# - Database connection pooling
# - Comprehensive security contexts
# - Environment variables for database connection
```

### **🚀 Deployment Procedure**

#### **Step 1: Deploy PostgreSQL Backend**

```bash
# Navigate to storage service directory
cd k8s/deployment/02-storage-service/

# Deploy PostgreSQL StatefulSet
# REVIEW: NEED TO DEPLOY SECRETS 1ST
kubectl apply -f postgres-statefulset.yaml

# Wait for PostgreSQL to be ready
kubectl wait --for=condition=Ready pod/postgres-0 --timeout=300s

# Verify database connectivity
kubectl exec postgres-0 -- pg_isready -U profile_user
# Expected output: postgres-0:5432 - accepting connections
```

#### **Step 2: Initialize Database Schema**

```bash
# Check if initialization completed
kubectl logs postgres-0 | grep "database system is ready to accept connections"

# Verify database and user creation
kubectl exec postgres-0 -- psql -U profile_user -d profile_db -c "\dt"
# Should show initialized tables
# error: FATAL:  database "profile_db" does not exist
```

#### **Step 3: Deploy Storage Service**

```bash
# Deploy configuration and application
kubectl apply -f configmap.yaml
kubectl apply -f deployment.yaml
kubectl apply -f service.yaml

# Verify dual-service setup
kubectl get svc storage-service
kubectl get svc storage-service-grpc
# Error from server (NotFound): services "storage-service-grpc" not found
```

### **🧪 Comprehensive Testing**

#### **Automated Testing**

```bash
# Run comprehensive storage service tests
cd k8s/scripts/services/
./test-02-storage-service.sh

# Test categories covered:
# ✅ Database connectivity and schema validation
# ✅ REST API CRUD operations
# ✅ gRPC functionality (if accessible)
# ✅ Data persistence across pod restarts
# ✅ Concurrent access patterns
# ✅ Error handling and validation
```

#### **Manual API Testing**

**Profile CRUD Operations**:

```bash
# Create a new profile
# no /api/v1
curl -X POST http://localhost:30082/api/v1/profiles \
  -H "Content-Type: application/json" \
  -d '{
    "first_name": "John",
    "last_name": "Doe",
    "email": "john.doe@example.com",
    "bio": "Software Developer"
  }'

# Expected response: {"id": "uuid", "first_name": "John", ...}

# Retrieve profile by ID
PROFILE_ID="<uuid-from-creation>"
curl -X GET http://localhost:30082/api/v1/profiles/$PROFILE_ID

# Update profile
curl -X PUT http://localhost:30082/api/v1/profiles/$PROFILE_ID \
  -H "Content-Type: application/json" \
  -d '{
    "first_name": "John",
    "last_name": "Smith",
    "email": "john.smith@example.com"
  }'

# List all profiles
curl -X GET http://localhost:30082/api/v1/profiles

# Delete profile
curl -X DELETE http://localhost:30082/api/v1/profiles/$PROFILE_ID
```

**Database Direct Access**:

```bash
# Connect to database for verification
kubectl exec -it postgres-0 -- psql -U profile_user -d profile_storage

# Check tables and data
# Commands when connected to postgres:
\dt  -- List tables
SELECT * FROM profiles LIMIT 5;  -- Check profile data
\q   -- Exit

```

### **🔍 gRPC Testing (Advanced)**

```bash
# Install grpcurl for gRPC testing
brew install grpcurl  # macOS
# or download from: https://github.com/fullstorydev/grpcurl/releases

# Test gRPC reflection (may not work with NodePort due to networking)
# REVIEW: NOT WORKING CORRECTLY
grpcurl -plaintext localhost:30092 list
# Note: This may fail due to Kind networking limitations
# Failed to dial target host "localhost:30092": connection error: desc = "transport: error while dialing: dial tcp [::1]:30092: connect: connection refused"

# Alternative: Test gRPC internally
kubectl run grpc-test --image=fullstorydev/grpcurl:latest --rm -i --restart=Never \
  -- -plaintext storage-service-grpc:50052 list
```

### **🔍 Troubleshooting Guide**

#### **Database Issues**

```bash
# Check PostgreSQL logs
kubectl logs postgres-0

# Common issues:
# 1. Insufficient resources - increase limits
# 2. Persistent volume issues - check PVC status
# 3. Initialization failures - check init scripts

# Check database status
kubectl exec postgres-0 -- pg_isready -U profile_user
kubectl exec postgres-0 -- psql -U profile_user -d profile_db -c "SELECT version();"
```

#### **Service Connectivity Issues**

```bash
# Check service endpoints
kubectl get endpoints storage-service
kubectl describe svc storage-service

# Test internal connectivity
kubectl run debug-storage --image=busybox:1.35 --rm -i --restart=Never \
  -- nc -zv storage-service 8080
```

### **✅ Success Criteria**

- [ ] PostgreSQL StatefulSet running and accepting connections
- [ ] Database schema initialized correctly
- [ ] Storage service deployment healthy
- [ ] Both HTTP (30082) and gRPC (30092) ports accessible
- [ ] CRUD operations working correctly
- [ ] Data persistence verified
- [ ] Test suite passes with >85% success rate

---

## 🔐 **Service 3: Auth Service (JWT-based)** {#auth-service-guide}

### **🎯 Service Overview**

The Auth Service provides JWT-based authentication and authorization for the entire microservices ecosystem. It now has its own dedicated PostgreSQL database for user management, ensuring proper microservices separation of concerns.

**Key Characteristics**:

- **Technology**: Node.js with JWT tokens + PostgreSQL database
- **Access Pattern**: REST API with JWT token generation/validation
- **Port**: NodePort 30083
- **Dependencies**: Own PostgreSQL database (auth-postgres)
- **Security**: JWT with RS256 signing, refresh tokens
- **Data Ownership**: Complete user data management

### **📋 Prerequisites**

```bash
# Verify Cache and Storage Services are running (foundation services)
kubectl get pods -l app=cache-service
kubectl get pods -l app=storage-service

# Verify no conflicts on port 30083
kubectl get svc --all-namespaces | grep 30083

# Ensure sufficient storage for PostgreSQL
kubectl get storageclass
```

### **🏗️ Architecture Components**

#### **Database Layer** (`auth-postgres-statefulset.yaml`)

```yaml
# Dedicated PostgreSQL database for auth service:
# - StatefulSet with persistent storage
# - Database: auth_db, User: auth_user
# - Isolated from other services
# - Proper security contexts and resource limits
```

#### **Database Secret** (`auth-postgres-secret.yaml`)

```yaml
# PostgreSQL credentials (base64 encoded):
# - password: Database password for auth_user
```

#### **Secrets Management** (`secrets.yaml`)

```yaml
# JWT and API keys (base64 encoded):
# - JWT_SECRET: Token signing key
# - JWT_REFRESH_SECRET: Refresh token key
# - SERVICE_API_KEY: Inter-service authentication
# - STORAGE_SERVICE_API_KEY: Storage service access (for profiles)
```

#### **Auth Service Deployment** (`deployment.yaml`)

```yaml
# Node.js authentication service with:
# - Direct PostgreSQL database connection
# - JWT token generation and validation
# - User management endpoints
# - Rate limiting and security headers
# - Comprehensive security contexts
```

### **🚀 Deployment Procedure**

#### **Step 1: Deploy Database Infrastructure**

```bash
# Navigate to auth service directory
cd k8s/deployment/03-auth-service/

# Deploy database secret first
kubectl apply -f auth-postgres-secret.yaml

# Deploy PostgreSQL StatefulSet and Service
kubectl apply -f auth-postgres-statefulset.yaml

# Wait for database to be ready
kubectl wait --for=condition=Ready pod/auth-postgres-0 --timeout=300s

# Verify database is running
kubectl get pods -l app=auth-postgres
kubectl get svc auth-postgres-service
```

#### **Step 2: Deploy Auth Service Configuration**

```bash
# Deploy secrets (contains JWT keys)
kubectl apply -f secrets.yaml

# Deploy configuration (includes database connection)
kubectl apply -f configmap.yaml

# Verify secrets and config are created
kubectl get secrets auth-service-secrets auth-postgres-secret
kubectl get configmap auth-service-config
```

#### **Step 3: Deploy Auth Service**

```bash
# Deploy the authentication service
kubectl apply -f deployment.yaml
kubectl apply -f service.yaml

# Wait for deployment to be ready
kubectl wait --for=condition=Available deployment/auth-service --timeout=300s

# Check database connectivity
kubectl logs deployment/auth-service | grep -i "database\|postgres"
```

### **🧪 Comprehensive Testing**

#### **Automated Testing**

```bash
# Run auth service test suite
cd k8s/scripts/services/
./test-03-auth-service.sh

# Test categories:
# ✅ Health and readiness checks
# ✅ Database connectivity
# ✅ JWT token generation and validation
# ✅ User management operations (CRUD)
# ✅ Authentication flow
# ✅ Rate limiting functionality
# ✅ Error handling and security
```

#### **Manual Database Testing**

**Database Connectivity**:

```bash
# Test database connection
# REVIEW: maybe not working
kubectl exec deployment/auth-service -- curl -s http://localhost:8080/health
# Expected: {"status": "healthy", "database": "connected"}

# Check database tables (should include users table)
kubectl exec -it auth-postgres-0 -- psql -U auth_user -d auth_db -c "\dt"
```

#### **Manual Authentication Testing**

**User Management Operations**:

```bash
# Health check
curl http://localhost:30083/health
# Expected: {"status": "healthy", "database": "connected"}

# Create a new user (now handled by auth service directly)
curl -X POST http://localhost:30083/v1/users \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "testpassword123",
    "first_name": "Test",
    "last_name": "User"
  }'

# Create an admin
curl -X POST http://localhost:30083/v1/users \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@account.com",
    "password": "testpassword123",
    "first_name": "Test",
    "last_name": "Admin",
    "role": "admin"
  }'

# Expected response: {"status": "created", "user_id": "uuid"}

# Admin login
curl -X POST http://localhost:30083/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "admin@account.com",
    "password": "testpassword123"
  }'

# Expected response: {"token": "jwt-token", "refresh_token": "refresh-jwt"}

# Token validation
TOKEN="<jwt-token-from-login>"
curl -X POST http://localhost:30083/v1/auth/token/validate \
  -H "Authorization: Bearer $TOKEN"

# Expected: {"valid": true, "user": {...}}
```

**User Registration Flow**:

```bash
# Register new user (now creates user in auth service database)
curl -X POST http://localhost:30083/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "newuser@example.com",
    "password": "securepassword123",
    "first_name": "New",
    "last_name": "User"
  }'

# Expected: {"status": "created", "user_id": "uuid"}

# Get user by email
curl -X GET "http://localhost:30083/api/v1/users?email=newuser@example.com"
# Expected: {"user": {...}}
```

### **🔍 Integration Testing**

**Database Integration**:

```bash
# Verify auth service database connectivity
kubectl exec deployment/auth-service -- curl -s http://localhost:8080/health

# Check user creation in auth database
kubectl exec -it auth-postgres-0 -- psql -U auth_user -d auth_db \
  -c "SELECT email, first_name, last_name FROM users WHERE email='newuser@example.com';"
```

**Profile Service Integration**:

```bash
# Verify profile service can access auth service for user data
kubectl exec deployment/profile-service -- curl -s http://auth-service:8080/health

# Test profile service user endpoint (should proxy to auth service)
curl http://localhost:30085/api/v1/users/newuser@example.com
```

### **🔍 Troubleshooting Guide**

#### **JWT Token Issues**

```bash
# Check JWT secret configuration
kubectl get secret auth-service-secrets -o yaml

# Verify JWT secret is properly base64 encoded
kubectl get secret auth-service-secrets -o jsonpath='{.data.JWT_SECRET}' | base64 -d

# Check token validation logs
kubectl logs deployment/auth-service | grep -i jwt
```

#### **Storage Integration Issues**

```bash
# Check storage service connectivity
kubectl exec deployment/auth-service -- nc -zv storage-service 8080

# Verify API key configuration
kubectl logs deployment/auth-service | grep -i "storage.*connection"
```

### **✅ Success Criteria**

- [ ] Auth service deployment healthy and ready
- [ ] JWT secrets properly configured
- [ ] Storage service integration working
- [ ] Token generation and validation functional
- [ ] User registration creates profiles in Storage Service
- [ ] Rate limiting protecting endpoints
- [ ] Test suite passes with >80% success rate

---

## 📬 **Service 4: Queue Service (RabbitMQ-backed)** {#queue-service-guide}

### **🎯 Service Overview**

The Queue Service provides asynchronous message processing capabilities using RabbitMQ. It enables decoupled communication between services and supports specialized worker routing for different task types.

**Key Characteristics**:

- **Technology**: RabbitMQ 3.x with management interface
- **Access Pattern**: AMQP protocol + REST API
- **Port**: NodePort 30084 (API), 15672 (Management UI)
- **Dependencies**: None (messaging backbone)
- **Features**: Multi-worker routing, message persistence, dead letter queues

### **📋 Prerequisites**

```bash
# Verify cluster resources (RabbitMQ is resource-intensive)
kubectl describe nodes | grep -A 5 "Allocated resources"

# Check for port conflicts
kubectl get svc --all-namespaces | grep -E "(30084|15672)"

# Verify sufficient disk space for message persistence
kubectl get pv
```

### **🏗️ Architecture Components**

#### **RabbitMQ StatefulSet** (`rabbitmq-statefulset.yaml`)

```yaml
# Production RabbitMQ with:
# - 2Gi persistent storage for message durability
# - Management interface enabled
# - Custom configuration for microservices
# - Resource limits: 512Mi memory, 300m CPU
# - Disk space monitoring (disk_free_limit)
```

#### **Queue Service Deployment** (`deployment.yaml`)

```yaml
# Message routing service with:
# - REST API for message publishing
# - Integration with RabbitMQ backend
# - Support for multiple routing keys
# - Worker task distribution logic
```

### **🚀 Deployment Procedure**

#### **Step 1: Deploy RabbitMQ Backend**

```bash
# Navigate to queue service directory
cd k8s/deployment/04-queue-service/

# Deploy RabbitMQ StatefulSet
kubectl apply -f rabbitmq-statefulset.yaml

# Wait for RabbitMQ to be ready (may take 2-3 minutes)
kubectl wait --for=condition=Ready pod/rabbitmq-0 --timeout=300s

# Verify RabbitMQ is operational
kubectl exec rabbitmq-0 -- rabbitmq-diagnostics ping
# Expected: Ping succeeded
```

#### **Step 2: Configure RabbitMQ**

```bash
# Check RabbitMQ management interface
kubectl port-forward rabbitmq-0 15672:15672 &
# Access: http://localhost:15672 (guest/guest)

# Verify queue configuration
kubectl exec rabbitmq-0 -- rabbitmqctl list_queues
kubectl exec rabbitmq-0 -- rabbitmqctl list_exchanges
```

#### **Step 3: Deploy Queue Service**

```bash
# Deploy application components
kubectl apply -f configmap.yaml
kubectl apply -f secrets.yaml
kubectl apply -f deployment.yaml
kubectl apply -f service.yaml

# Wait for service to connect to RabbitMQ
kubectl logs deployment/queue-service | grep -i "connected to rabbitmq"
```

### **🧪 Comprehensive Testing**

#### **Automated Testing**

```bash
# Run queue service test suite
cd k8s/scripts/services/
./test-04-queue-service.sh

# Test categories:
# ✅ RabbitMQ connectivity and health
# ✅ Message publishing and routing
# ✅ Queue management operations
# ✅ Integration with Cache, Storage, Auth services
# ✅ Multi-worker task distribution
# ✅ Message persistence and durability
```

#### **Manual Message Testing**

**Basic Message Operations**:

```bash
# Publish email task
curl -X POST http://localhost:30084/api/v1/queue/messages \
  -H "Content-Type: application/json" \
  -d '{
    "type": "email_send",
    "payload": {
      "user_id": "123",
      "template": "welcome",
      "recipient": "user@example.com"
    },
    "metadata": {
      "source": "manual-test",
      "timestamp": "'$(date -Iseconds)'"
    }
  }'

# Publish image processing task
curl -X POST http://localhost:30084/api/v1/queue/messages \
  -H "Content-Type: application/json" \
  -d '{
    "type": "image_process",
    "payload": {
      "user_id": "123",
      "operation": "resize",
      "dimensions": "800x600"
    },
    "metadata": {
      "source": "manual-test"
    }
  }'

# Check queue status
curl http://localhost:30084/api/v1/queues/status
```

**RabbitMQ Management Interface**:

```bash
# Forward management port
kubectl port-forward rabbitmq-0 15672:15672

# Access web interface: http://localhost:15672
# Username: guest, Password: guest
# Monitor queues, exchanges, and message flow
```

### **🔍 Integration Testing**

**Service Integration Validation**:

```bash
# Test Cache Service publishing messages
curl -X POST http://localhost:30081/api/v1/cache/trigger-task \
  -H "Content-Type: application/json" \
  -d '{"task_type": "cache_invalidation"}'

# Test Storage Service publishing messages
curl -X POST http://localhost:30082/api/v1/profiles/123/tasks \
  -H "Content-Type: application/json" \
  -d '{"task_type": "profile_backup"}'

# Test Auth Service publishing messages
curl -X POST http://localhost:30083/api/v1/auth/tasks \
  -H "Content-Type: application/json" \
  -d '{"task_type": "password_reset"}'
```

### **🔍 Troubleshooting Guide**

#### **RabbitMQ Issues**

```bash
# Check RabbitMQ status
kubectl exec rabbitmq-0 -- rabbitmq-diagnostics status

# Check disk space (common issue)
kubectl exec rabbitmq-0 -- df -h
kubectl exec rabbitmq-0 -- rabbitmq-diagnostics disk_free

# Fix disk space alarm
kubectl exec rabbitmq-0 -- rabbitmqctl set_disk_free_limit 0.1
```

#### **Message Flow Issues**

```bash
# Check message routing
kubectl exec rabbitmq-0 -- rabbitmqctl list_queues name messages
kubectl exec rabbitmq-0 -- rabbitmqctl list_bindings

# Monitor message flow
kubectl logs deployment/queue-service -f
```

### **✅ Success Criteria**

- [ ] RabbitMQ StatefulSet running and healthy
- [ ] Queue service deployment connected to RabbitMQ
- [ ] Message publishing and routing working
- [ ] Management interface accessible
- [ ] Integration with other services functional
- [ ] Message persistence across pod restarts verified
- [ ] Test suite passes with >85% success rate

---

## 🎭 **Service 5: Profile Service (Orchestrator)** {#profile-service-guide}

### **🎯 Service Overview**

The Profile Service acts as the primary orchestrator in the microservices ecosystem. It integrates with all other services (Auth, Cache, Storage, Queue) to provide comprehensive profile management with authentication, caching, persistence, and asynchronous task processing.

**Key Characteristics**:

- **Technology**: Node.js/Go orchestration service
- **Access Pattern**: REST API with full service integration
- **Port**: NodePort 30085
- **Dependencies**: Auth, Cache, Storage, Queue services
- **Role**: Business logic orchestrator and service coordinator

### **📋 Prerequisites**

```bash
# Verify all dependency services are running
kubectl get pods -l app=auth-service
kubectl get pods -l app=cache-service
kubectl get pods -l app=storage-service
kubectl get pods -l app=queue-service

# Test dependency connectivity
for port in 30081 30082 30083 30084; do
  echo "Testing service on port $port:"
  curl -s http://localhost:$port/health | jq .status
done

# Verify Docker image is built and loaded
docker images | grep profile-service
```

### **🏗️ Architecture Components**

#### **🚨 CRITICAL: Docker Image Build Required**

**Profile Service requires custom Docker image**:

```bash
# Build the Profile Service image
cd services/profile-service/
docker build -t profile-service:latest .

# Load image into Kind cluster
kind load docker-image profile-service:latest --name microservices-kind

# Verify image is available
docker exec -it microservices-kind-control-plane crictl images | grep profile-service
```

#### **Secrets Management** (`secrets.yaml`)

```yaml
# Service integration credentials:
# - AUTH_SERVICE_API_KEY: Auth service access
# - CACHE_SERVICE_API_KEY: Cache service access
# - STORAGE_SERVICE_API_KEY: Storage service access
# - QUEUE_SERVICE_API_KEY: Queue service access
# - PROFILE_JWT_SECRET: Profile-specific JWT operations
```

#### **Profile Service Deployment** (`deployment.yaml`)

```yaml
# Orchestrator service with:
# - Integration with all 4 dependency services
# - Comprehensive environment variable configuration
# - Security contexts and resource limits
# - Health checks for all service dependencies
```

### **🚀 Deployment Procedure**

#### **Step 1: Build and Load Docker Image**

```bash
# Navigate to profile service source
cd services/profile-service/

# Build Docker image
docker build -t profile-service:latest .

# Load into Kind cluster
kind load docker-image profile-service:latest --name microservices-kind

# Verify image loading
kubectl run test-image --image=profile-service:latest --rm -i --restart=Never \
  --command -- echo "Image loaded successfully"
```

#### **Step 2: Deploy Profile Service**

```bash
# Navigate to deployment directory
cd k8s/deployment/05-profile-service/

# Deploy secrets and configuration
kubectl apply -f secrets.yaml
kubectl apply -f configmap.yaml

# Deploy the orchestrator service
kubectl apply -f deployment.yaml
kubectl apply -f service.yaml

# Wait for all dependencies to be ready
kubectl wait --for=condition=Available deployment/profile-service --timeout=300s
```

#### **Step 3: Verify Service Integration**

```bash
# Check service logs for dependency connections
kubectl logs deployment/profile-service | grep -E "(auth|cache|storage|queue).*connected"

# Verify all endpoints are accessible
curl http://localhost:30085/health
# Expected: All dependencies should show as "connected"
```

### **🧪 Comprehensive Testing**

#### **Automated Testing**

```bash
# Run basic profile service tests
cd k8s/scripts/services/
./test-05-profile-service.sh

# Run comprehensive integration tests
cd k8s/scripts/integration/
./profile-service-integration-test.sh

# Test categories covered:
# ✅ Service health and readiness
# ✅ Authentication integration (JWT tokens)
# ✅ Cache integration (profile caching)
# ✅ Storage integration (profile CRUD)
# ✅ Queue integration (async task publishing)
# ✅ End-to-end business workflows
```

#### **Manual Integration Testing**

**Complete Authentication Flow**:

```bash
# Authenticate
curl -X POST http://localhost:30085/api/v1/auth/token \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "user_id": "admin@account.com",
    "password": "testpassword123"
  }'
```

```bash
# Extract token from response
TOKEN="<jwt-token-from-response>"
```

```bash
# Get profiles
curl -X GET http://localhost:30085/api/v1/profiles \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN"
```

```bash
# Create user
curl -X POST http://localhost:30085/api/v1/profiles \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "email": "integration@test.com",
    "password": "testpass123",
    "first_name": "Integration",
    "last_name": "Test"
  }'
```

```bash
# 3. Use token for authenticated operations
curl -X GET http://localhost:30085/api/v1/profiles/me \
  -H "Authorization: Bearer $TOKEN"
```

**Cache Integration Testing**:

```bash
# Profile caching workflow
curl -X GET http://localhost:30085/api/v1/profiles/123 \
  -H "Authorization: Bearer $TOKEN"
# First call: Cache miss, fetches from storage
# Second call: Cache hit, faster response

# Cache invalidation
curl -X PUT http://localhost:30085/api/v1/profiles/123 \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"first_name": "Updated"}'
# Should invalidate cache and update storage
```

**Async Task Processing**:
```bash
# Send a email_notification task
curl -X POST http://localhost:30085/api/v1/profiles/6f5cd429-1a27-4670-93b7-93267e6f7828/tasks/email \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "template": "This is a test email",
    "to": "integration@test.com"
  }'
```

```bash
# Send a email_notification task
curl -X POST http://localhost:30085/api/v1/profiles/6f5cd429-1a27-4670-93b7-93267e6f7828/tasks/image \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "operation": "optimize",
    "image_url": "http://image.example"
  }'
```

```bash
# Verify task was published to queue
curl http://localhost:30084/api/v1/queues/status
```

### **🔍 Advanced Integration Patterns**

**Service Mesh Communication**:

```bash
# Test service-to-service communication patterns
kubectl exec deployment/profile-service -- curl -s http://auth-service:8080/health
kubectl exec deployment/profile-service -- curl -s http://cache-service:8080/health
kubectl exec deployment/profile-service -- curl -s http://storage-service:8080/health
kubectl exec deployment/profile-service -- curl -s http://queue-service:8080/health
```

**Performance and Load Testing**:

```bash
# Basic load test with Apache Bench
ab -n 100 -c 5 http://localhost:30085/health

# Profile operations load test
ab -n 50 -c 3 -H "Authorization: Bearer $TOKEN" \
  http://localhost:30085/api/v1/profiles/me
```

### **🔍 Troubleshooting Guide**

#### **Docker Image Issues**

```bash
# Check if image exists in Kind cluster
docker exec -it microservices-kind-control-plane crictl images | grep profile-service

# If missing, rebuild and reload
cd services/profile-service/
docker build -t profile-service:latest .
kind load docker-image profile-service:latest --name microservices-kind
```

#### **Service Integration Issues**

```bash
# Check dependency service health
for service in auth-service cache-service storage-service queue-service; do
  echo "Testing $service:"
  kubectl exec deployment/profile-service -- nc -zv $service 8080
done

# Check API key configuration
kubectl get secret profile-service-secrets -o yaml
kubectl logs deployment/profile-service | grep -i "api.*key"
```

#### **Authentication Flow Issues**

```bash
# Debug JWT token handling
kubectl logs deployment/profile-service | grep -i jwt

# Test auth service integration
kubectl exec deployment/profile-service -- curl -s http://auth-service:8080/health
```

### **✅ Success Criteria**

- [ ] Docker image built and loaded successfully
- [ ] Profile service deployment healthy and ready
- [ ] All 4 dependency services connected
- [ ] Authentication flow working end-to-end
- [ ] Cache integration functional (cache hits/misses)
- [ ] Storage integration working (CRUD operations)
- [ ] Queue integration working (task publishing)
- [ ] Basic test suite passes with >80% success rate
- [ ] Integration test suite passes with >70% success rate

---

## ⚙️ **Service 6: Worker Service (Multi-Worker Processing)** {#worker-service-guide}

### **🎯 Service Overview**

The Worker Service provides specialized asynchronous task processing through multiple worker types. It consumes messages from the Queue Service and processes them according to their routing keys, enabling scalable background processing.

**Key Characteristics**:

- **Technology**: Go-based workers with specialized processing
- **Access Pattern**: Message consumption from RabbitMQ
- **Port**: NodePort 30086 (monitoring/health)
- **Dependencies**: Queue Service (RabbitMQ)
- **Workers**: Email Worker, Image Worker (extensible)

### **📋 Prerequisites**

```bash
# Verify Queue Service is running and healthy
kubectl get pods -l app=queue-service
kubectl get pods -l app=rabbitmq
curl http://localhost:30084/health

# Test RabbitMQ connectivity
kubectl exec rabbitmq-0 -- rabbitmq-diagnostics ping

# Verify sufficient cluster resources
kubectl describe nodes | grep -A 5 "Allocated resources"
```

### **🏗️ Architecture Components**

#### **🚨 CRITICAL: Cross-Platform Binary Compilation**

**Worker Service requires platform-specific binaries**:

```bash
# Navigate to worker service source
cd services/worker-service/

# Build Linux binaries (required for containers)
GOOS=linux GOARCH=amd64 go build -o email-worker-linux cmd/email-worker/main.go
GOOS=linux GOARCH=amd64 go build -o image-worker-linux cmd/image-worker/main.go

# Build Docker image with Linux binaries
docker build -t worker-service:latest .

# Load into Kind cluster
kind load docker-image worker-service:latest --name microservices-kind
```

#### **Multi-Worker Deployment** (`deployment.yaml`)

```yaml
# Specialized worker deployments:
# - Email Worker: Handles email.* routing keys
# - Image Worker: Handles image.* routing keys
# - Shared RabbitMQ connection configuration
# - Independent scaling and resource allocation
```

#### **Network Policies** (`network-policy.yaml`)

```yaml
# Worker-specific network policies:
# - Allow connection to RabbitMQ
# - Restrict unnecessary network access
# - Enable monitoring and health checks
```

### **🚀 Deployment Procedure**

#### **Step 1: Build Cross-Platform Binaries**

```bash
# Navigate to worker service source
cd services/worker-service/

# Build Linux binaries for container compatibility
echo "Building email worker..."
GOOS=linux GOARCH=amd64 go build -o email-worker-linux cmd/email-worker/main.go

echo "Building image worker..."
GOOS=linux GOARCH=amd64 go build -o image-worker-linux cmd/image-worker/main.go

# Verify binaries are created
ls -la *-worker-linux
file email-worker-linux  # Should show: Linux x86-64 executable
```

#### **Step 2: Build and Load Docker Image**

```bash
# Build worker service image
docker build -t worker-service:latest .

# Load into Kind cluster
kind load docker-image worker-service:latest --name microservices-kind

# Verify image loading
docker exec -it microservices-kind-control-plane crictl images | grep worker-service
```

#### **Step 3: Deploy Worker Services**

```bash
# Navigate to deployment directory
cd k8s/deployment/06-worker-service/

# Deploy configuration and secrets
kubectl apply -f configmap.yaml
kubectl apply -f secrets.yaml

# Deploy multi-worker deployment
kubectl apply -f deployment.yaml
kubectl apply -f service.yaml

# Deploy network policies
kubectl apply -f network-policy.yaml

# Wait for workers to be ready
kubectl wait --for=condition=Available deployment/email-worker --timeout=300s
kubectl wait --for=condition=Available deployment/image-worker --timeout=300s
```

#### **Step 4: Verify Worker Connectivity**

```bash
# Check worker logs for RabbitMQ connection
kubectl logs deployment/email-worker | grep -i "connected to rabbitmq"
kubectl logs deployment/image-worker | grep -i "connected to rabbitmq"

# Verify workers are consuming from correct queues
kubectl exec rabbitmq-0 -- rabbitmqctl list_consumers
```

### **🧪 Comprehensive Testing**

#### **Automated Testing**

```bash
# Run worker service tests
cd k8s/scripts/services/
./test-06-worker-service.sh

# Run worker integration tests
cd k8s/scripts/integration/
./worker-integration-test.sh

# Test categories:
# ✅ Worker deployment health
# ✅ RabbitMQ connectivity
# ✅ Message consumption by routing key
# ✅ Specialized worker processing
# ✅ Error handling and retry logic
# ✅ Performance under load
```

#### **Manual Worker Testing**

**Email Worker Testing**:

```bash
# Publish email task via Queue Service
curl -X POST http://localhost:30084/api/v1/queues/publish \
  -H "Content-Type: application/json" \
  -d '{
    "routing_key": "email.send",
    "payload": {
      "user_id": "worker-test-123",
      "template": "welcome",
      "recipient": "test@example.com",
      "subject": "Welcome to the platform"
    },
    "metadata": {
      "source": "manual-test",
      "timestamp": "'$(date -Iseconds)'"
    }
  }'

# Verify email worker processes the message
kubectl logs deployment/email-worker --tail=20 | grep "worker-test-123"
# Expected: Processing log for the test message
```

**Image Worker Testing**:

```bash
# Publish image processing task
curl -X POST http://localhost:30084/api/v1/queues/publish \
  -H "Content-Type: application/json" \
  -d '{
    "routing_key": "image.process",
    "payload": {
      "user_id": "worker-test-456",
      "operation": "resize",
      "source_url": "https://example.com/image.jpg",
      "dimensions": "800x600"
    },
    "metadata": {
      "source": "manual-test"
    }
  }'

# Verify image worker processes the message
kubectl logs deployment/image-worker --tail=20 | grep "worker-test-456"
# Expected: Processing log for the image task
```

**Multi-Worker Load Testing**:

```bash
# Publish multiple tasks of different types
for i in {1..5}; do
  # Email tasks
  curl -X POST http://localhost:30084/api/v1/queues/publish \
    -H "Content-Type: application/json" \
    -d "{\"routing_key\": \"email.send\", \"payload\": {\"user_id\": \"load-test-email-$i\"}}"

  # Image tasks
  curl -X POST http://localhost:30084/api/v1/queues/publish \
    -H "Content-Type: application/json" \
    -d "{\"routing_key\": \"image.process\", \"payload\": {\"user_id\": \"load-test-image-$i\"}}"
done

# Monitor processing across both workers
kubectl logs deployment/email-worker --tail=10 | grep "load-test"
kubectl logs deployment/image-worker --tail=10 | grep "load-test"
```

### **🔍 Advanced Worker Patterns**

**Message Routing Verification**:

```bash
# Check RabbitMQ queue bindings
kubectl exec rabbitmq-0 -- rabbitmqctl list_bindings | grep -E "(email|image)"

# Monitor queue depths
kubectl exec rabbitmq-0 -- rabbitmqctl list_queues name messages | grep -E "(email|image)"

# Check consumer distribution
kubectl exec rabbitmq-0 -- rabbitmqctl list_consumers
```

**Worker Performance Monitoring**:

```bash
# Monitor worker resource usage
kubectl top pods -l app=email-worker
kubectl top pods -l app=image-worker

# Check processing rates
kubectl logs deployment/email-worker | grep "processed" | tail -10
kubectl logs deployment/image-worker | grep "processed" | tail -10
```

### **🔍 Troubleshooting Guide**

#### **Binary Compatibility Issues**

```bash
# Check binary architecture
kubectl exec deployment/email-worker -- file /app/email-worker-linux
# Should show: Linux x86-64 executable

# If exec format error, rebuild with correct GOOS/GOARCH
cd services/worker-service/
GOOS=linux GOARCH=amd64 go build -o email-worker-linux cmd/email-worker/main.go
docker build -t worker-service:latest .
kind load docker-image worker-service:latest --name microservices-kind
kubectl rollout restart deployment/email-worker
```

#### **RabbitMQ Connection Issues**

```bash
# Check RabbitMQ credentials in secrets
kubectl get secret worker-service-secrets -o yaml

# Test connectivity from worker pods
kubectl exec deployment/email-worker -- nc -zv rabbitmq-service 5672

# Check RabbitMQ user permissions
kubectl exec rabbitmq-0 -- rabbitmqctl list_user_permissions queue_user
```

#### **Message Processing Issues**

```bash
# Check for dead letter queues
kubectl exec rabbitmq-0 -- rabbitmqctl list_queues | grep dlx

# Monitor message acknowledgments
kubectl logs deployment/email-worker | grep -E "(ack|nack|reject)"

# Check worker error logs
kubectl logs deployment/email-worker | grep -i error
kubectl logs deployment/image-worker | grep -i error
```

### **✅ Success Criteria**

- [ ] Cross-platform binaries compiled successfully
- [ ] Docker image built and loaded into Kind cluster
- [ ] Both email and image workers deployed and running
- [ ] Workers connected to RabbitMQ successfully
- [ ] Message routing working by routing key
- [ ] Email worker processes `email.*` messages
- [ ] Image worker processes `image.*` messages
- [ ] Network policies applied correctly
- [ ] Basic test suite passes with >85% success rate
- [ ] Integration test suite passes with >75% success rate

---

## 🧪 **Testing Strategy Overview**

### **Testing Hierarchy**

```
1. Individual Service Tests (k8s/scripts/services/)
   ├── test-01-cache-service.sh      # Isolated cache functionality
   ├── test-02-storage-service.sh    # Database and API operations
   ├── test-03-auth-service.sh       # JWT and authentication
   ├── test-04-queue-service.sh      # Message queue operations
   ├── test-05-profile-service.sh    # Basic orchestration
   └── test-06-worker-service.sh     # Worker task processing

2. Integration Tests (k8s/scripts/integration/)
   ├── profile-service-integration-test.sh    # End-to-end workflows
   ├── worker-integration-test.sh             # Multi-service task processing
   └── comprehensive-integration-test.sh      # Complete ecosystem validation
```

### **Testing Best Practices**

#### **Service-Level Testing**

- **Health Checks**: Verify service startup and readiness
- **API Functionality**: Test all endpoints with various inputs
- **Error Handling**: Validate proper error responses
- **Performance**: Basic response time measurements
- **Dependencies**: Verify integration with backend services

#### **Integration Testing**

- **Authentication Flows**: Complete user registration → login → token usage
- **Data Persistence**: CRUD operations across service boundaries
- **Cache Patterns**: Cache-aside implementation validation
- **Async Processing**: Message publishing → consumption → processing
- **Business Workflows**: Real user scenarios end-to-end

#### **Test Execution Strategy**

```bash
# 1. Deploy services in dependency order
# 2. Run individual service tests after each deployment
cd k8s/scripts/services/
./test-01-cache-service.sh      # After cache deployment
./test-02-storage-service.sh    # After storage deployment
./test-03-auth-service.sh       # After auth deployment
./test-04-queue-service.sh      # After queue deployment
./test-05-profile-service.sh    # After profile deployment
./test-06-worker-service.sh     # After worker deployment

# 3. Run integration tests after all services are deployed
cd k8s/scripts/integration/
./profile-service-integration-test.sh    # Profile orchestration
./worker-integration-test.sh             # Worker processing
./comprehensive-integration-test.sh      # Full ecosystem
```

---

## 🔧 **Troubleshooting and Debugging**

### **Common Issues and Solutions**

#### **Cluster Issues**

**Problem**: Kind cluster fails to start

```bash
# Check Docker resources
docker system df
docker system prune

# Check Docker daemon status
docker info

# Recreate cluster
kind delete cluster --name microservices-kind
cd k8s/cluster/
./setup-cluster.sh
```

**Problem**: Pods stuck in Pending state

```bash
# Check node resources
kubectl describe nodes

# Check pod events for specific issues
kubectl describe pod <pod-name>

# Check resource requests vs available
kubectl top nodes
kubectl top pods

# Common causes and solutions:
# 1. Insufficient resources - increase Docker memory allocation
# 2. Image pull issues - verify Docker images are available
# 3. Volume mounting issues - check storage class and PVC status
```

**Problem**: Nodes not ready

```bash
# Check node status
kubectl get nodes -o wide

# Check node conditions
kubectl describe nodes

# Check kubelet logs (Kind-specific)
docker exec microservices-kind-control-plane journalctl -u kubelet
```

#### **Service Issues**

**Problem**: Service not accessible via NodePort

```bash
# Check service configuration
kubectl get svc <service-name> -o yaml

# Verify Kind port mapping
kind get clusters
docker port microservices-kind-control-plane

# Expected output should show port mappings like:
# 30081/tcp -> 0.0.0.0:30081

# Test internal service connectivity
kubectl run debug-$(date +%s) --image=busybox:1.35 --rm -it --restart=Never \
  --labels="app=debug" -- sh

# Inside debug pod:
# nc -zv <service-name> 8080
# wget -qO- http://<service-name>:8080/health
```

**Problem**: Persistent volumes not mounting

```bash
# Check storage class
kubectl get storageclass

# Check PVC status
kubectl get pvc
kubectl describe pvc <pvc-name>

# Check local path provisioner logs
kubectl logs -n local-path-storage deployment/local-path-provisioner

# Verify node storage
kubectl exec <pod-name> -- df -h
```

**Problem**: Docker image issues (ErrImagePull, ImagePullBackOff)

```bash
# For custom services (Profile, Worker)
# Check if image exists in Kind cluster
docker exec -it microservices-kind-control-plane crictl images | grep <service-name>

# If missing, rebuild and reload
cd services/<service-name>/
docker build -t <service-name>:latest .
kind load docker-image <service-name>:latest --name microservices-kind

# Force pod restart
kubectl rollout restart deployment/<service-name>
```

#### **Database and Backend Issues**

**Problem**: Redis authentication failures

```bash
# Check Redis configuration
kubectl exec redis-0 -- redis-cli ping
# If auth error, check Redis configuration

# Verify Redis password setup
kubectl get secret redis-secret -o yaml
kubectl exec redis-0 -- cat /etc/redis/redis.conf | grep requirepass

# Test Redis connectivity from cache service
kubectl exec deployment/cache-service -- redis-cli -h redis-service -p 6379 ping
```

**Problem**: PostgreSQL connection issues

```bash
# Check PostgreSQL status
kubectl exec postgres-0 -- pg_isready -U profile_user

# Test database connectivity
kubectl exec postgres-0 -- psql -U profile_user -d profile_db -c "SELECT version();"

# Check database initialization
kubectl logs postgres-0 | grep "database system is ready"

# Verify database schema
kubectl exec postgres-0 -- psql -U profile_user -d profile_db -c "\dt"
```

**Problem**: RabbitMQ disk space alarm

```bash
# Check RabbitMQ status
kubectl exec rabbitmq-0 -- rabbitmq-diagnostics status

# Check disk space
kubectl exec rabbitmq-0 -- df -h
kubectl exec rabbitmq-0 -- rabbitmq-diagnostics disk_free

# Fix disk space alarm (Kind-specific)
kubectl exec rabbitmq-0 -- rabbitmqctl set_disk_free_limit 0.1
```

#### **Network and Communication Issues**

**Problem**: Service-to-service communication failures

```bash
# Check network policies
kubectl get networkpolicies
kubectl describe networkpolicy <policy-name>

# Test pod-to-pod communication
kubectl exec <source-pod> -- nc -zv <target-service> <port>

# Check DNS resolution
kubectl exec <pod-name> -- nslookup <service-name>
kubectl exec <pod-name> -- nslookup <service-name>.<namespace>.svc.cluster.local

# Check service endpoints
kubectl get endpoints <service-name>
kubectl describe svc <service-name>
```

**Problem**: Ingress not working

```bash
# Check ingress controller status
kubectl get pods -n ingress-nginx
kubectl logs -n ingress-nginx deployment/ingress-nginx-controller

# Test ingress with proper host header
curl -H "Host: microservices.local" http://localhost:80/health

# Check ingress configuration
kubectl get ingress
kubectl describe ingress <ingress-name>
```

### **Debugging Procedures**

#### **Container Debugging**

```bash
# Access running container
kubectl exec -it <pod-name> -- /bin/sh

# Check container logs (current and previous)
kubectl logs <pod-name> -f
kubectl logs <pod-name> --previous

# Debug networking inside container
kubectl exec -it <pod-name> -- netstat -tuln
kubectl exec -it <pod-name> -- ss -tuln

# Check environment variables
kubectl exec <pod-name> -- env | sort

# Check file system
kubectl exec <pod-name> -- ls -la /
kubectl exec <pod-name> -- df -h
```

#### **Resource Investigation**

```bash
# Check resource usage
kubectl top pods --all-namespaces
kubectl top nodes

# Describe resources for detailed information
kubectl describe deployment <deployment-name>
kubectl describe pod <pod-name>
kubectl describe node <node-name>

# Check resource requests and limits
kubectl get pods -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.containers[*].resources}{"\n"}{end}'

# Check events for issues
kubectl get events --sort-by='.lastTimestamp'
kubectl get events --field-selector involvedObject.name=<pod-name>
```

#### **Service-Specific Debugging**

**Cache Service Debugging**:

```bash
# Check Redis connectivity
kubectl exec deployment/cache-service -- redis-cli -h redis-service ping

# Test cache operations directly
kubectl exec redis-0 -- redis-cli set test-key "test-value"
kubectl exec redis-0 -- redis-cli get test-key

# Check cache service logs for Redis connection
kubectl logs deployment/cache-service | grep -i redis
```

**Storage Service Debugging**:

```bash
# Check database connection from service
kubectl exec deployment/storage-service -- nc -zv postgres-service 5432

# Test database queries
kubectl exec postgres-0 -- psql -U profile_user -d profile_db -c "SELECT COUNT(*) FROM profiles;"

# Check for string formatting issues (known issue)
kubectl logs deployment/storage-service | grep -E "(error|panic|fatal)"
```

**Auth Service Debugging**:

```bash
# Check JWT secret configuration
kubectl get secret auth-service-secrets -o yaml

# Test JWT token generation
curl -X POST http://localhost:30083/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email": "debug@test.com", "password": "testpass"}'

# Check integration with storage service
kubectl exec deployment/auth-service -- curl -s http://storage-service:8080/health
```

**Queue Service Debugging**:

```bash
# Check RabbitMQ management interface
kubectl port-forward rabbitmq-0 15672:15672 &
# Access: http://localhost:15672 (guest/guest)

# Check queue status
kubectl exec rabbitmq-0 -- rabbitmqctl list_queues
kubectl exec rabbitmq-0 -- rabbitmqctl list_exchanges

# Monitor message flow
kubectl logs deployment/queue-service -f | grep -i message
```

**Profile Service Debugging**:

```bash
# Check all service dependencies
for service in auth-service cache-service storage-service queue-service; do
  echo "Testing $service:"
  kubectl exec deployment/profile-service -- nc -zv $service 8080
done

# Check API key configuration
kubectl get secret profile-service-secrets -o yaml

# Test authentication flow
kubectl logs deployment/profile-service | grep -E "(jwt|token|auth)"
```

**Worker Service Debugging**:

```bash
# Check binary architecture (common issue)
kubectl exec deployment/email-worker -- file /app/email-worker-linux
# Should show: Linux x86-64 executable

# Check RabbitMQ connectivity
kubectl exec deployment/email-worker -- nc -zv rabbitmq-service 5672

# Monitor message consumption
kubectl logs deployment/email-worker -f | grep -E "(consuming|processing|error)"
kubectl logs deployment/image-worker -f | grep -E "(consuming|processing|error)"
```

### **Recovery Procedures**

#### **Complete Cluster Reset**

```bash
# Nuclear option - complete reset
kind delete cluster --name microservices-kind

# Recreate from scratch
cd k8s/cluster/
./setup-cluster.sh

# Redeploy services in dependency order
cd ../deployment/01-cache-service/
kubectl apply -f .
# ... continue with other services
```

#### **Service-Specific Recovery**

```bash
# Delete and recreate specific service
kubectl delete -f k8s/deployment/01-cache-service/
kubectl apply -f k8s/deployment/01-cache-service/

# Force pod restart (preserves configuration)
kubectl rollout restart deployment/<deployment-name>

# Scale down and up (for troubleshooting)
kubectl scale deployment <deployment-name> --replicas=0
kubectl scale deployment <deployment-name> --replicas=1
```

#### **StatefulSet Recovery**

```bash
# For Redis, PostgreSQL, RabbitMQ
kubectl delete statefulset <statefulset-name> --cascade=orphan
kubectl apply -f <statefulset-yaml>

# If PVC issues persist
kubectl delete pvc <pvc-name>
kubectl delete statefulset <statefulset-name>
kubectl apply -f <statefulset-yaml>
```

### **Monitoring and Alerting**

#### **Health Check Automation**

```bash
#!/bin/bash
# health-check-all.sh - Monitor all services

services=("30081:cache" "30082:storage" "30083:auth" "30084:queue" "30085:profile" "30086:worker")

for service in "${services[@]}"; do
  port=$(echo $service | cut -d: -f1)
  name=$(echo $service | cut -d: -f2)

  status=$(curl -s --connect-timeout 5 http://localhost:$port/health | jq -r '.status // "unhealthy"' 2>/dev/null || echo "unreachable")

  if [[ "$status" == "healthy" ]] || [[ "$status" == "ok" ]]; then
    echo "✅ $name service: $status"
  else
    echo "❌ $name service: $status"
  fi
done
```

#### **Log Aggregation**

```bash
# Collect logs from all services
mkdir -p logs/$(date +%Y%m%d-%H%M%S)

kubectl logs deployment/cache-service > logs/cache-service.log
kubectl logs deployment/storage-service > logs/storage-service.log
kubectl logs deployment/auth-service > logs/auth-service.log
kubectl logs deployment/queue-service > logs/queue-service.log
kubectl logs deployment/profile-service > logs/profile-service.log
kubectl logs deployment/email-worker > logs/email-worker.log
kubectl logs deployment/image-worker > logs/image-worker.log

# Backend services
kubectl logs statefulset/redis > logs/redis.log
kubectl logs statefulset/postgres > logs/postgres.log
kubectl logs statefulset/rabbitmq > logs/rabbitmq.log
```

---

## 📋 **Deployment Checklist**

### **Pre-Deployment Checklist**

- [ ] Kind cluster created and healthy
- [ ] All prerequisite tools installed (kubectl, docker, kind)
- [ ] Sufficient system resources available
- [ ] No port conflicts on NodePorts 30081-30086

### **Service Deployment Checklist**

#### **Cache Service (01)**

- [ ] Redis StatefulSet deployed and ready
- [ ] Cache service deployment healthy
- [ ] NodePort 30081 accessible
- [ ] Basic cache operations working
- [ ] Test suite passes

#### **Storage Service (02)**

- [ ] PostgreSQL StatefulSet deployed and ready
- [ ] Database schema initialized
- [ ] Storage service deployment healthy
- [ ] NodePorts 30082 (HTTP) and 30092 (gRPC) accessible
- [ ] CRUD operations working
- [ ] Test suite passes

#### **Auth Service (03)**

- [ ] Secrets deployed correctly
- [ ] Auth service deployment healthy
- [ ] NodePort 30083 accessible
- [ ] JWT token generation/validation working
- [ ] Storage service integration working
- [ ] Test suite passes

#### **Queue Service (04)**

- [ ] RabbitMQ StatefulSet deployed and ready
- [ ] Queue service deployment healthy
- [ ] NodePort 30084 accessible
- [ ] Message publishing/routing working
- [ ] Management interface accessible
- [ ] Test suite passes

#### **Profile Service (05)**

- [ ] Docker image built and loaded
- [ ] Secrets and configuration deployed
- [ ] Profile service deployment healthy
- [ ] NodePort 30085 accessible
- [ ] All service integrations working
- [ ] Authentication flow end-to-end working
- [ ] Test suites pass

#### **Worker Service (06)**

- [ ] Cross-platform binaries compiled
- [ ] Docker image built and loaded
- [ ] Multi-worker deployments healthy
- [ ] RabbitMQ connectivity established
- [ ] Message consumption by routing key working
- [ ] Network policies applied
- [ ] Test suites pass

### **Post-Deployment Validation**

- [ ] All services healthy and ready
- [ ] All NodePorts accessible
- [ ] Service-to-service communication working
- [ ] Integration tests passing
- [ ] Performance within acceptable ranges
- [ ] No error logs indicating issues

---

## 🎓 **Learning Outcomes**

By completing this services deployment guide, you will have gained practical experience with:

### **Microservices Architecture Patterns**

- **Service Dependencies**: Understanding deployment order and service relationships
- **Service Discovery**: How services find and communicate with each other
- **Data Consistency**: Managing data across multiple services
- **Async Processing**: Implementing message-driven architecture

### **Kubernetes Deployment Patterns**

- **StatefulSets**: For persistent data services (Redis, PostgreSQL, RabbitMQ)
- **Deployments**: For stateless application services
- **Services**: NodePort for external access, ClusterIP for internal communication
- **ConfigMaps and Secrets**: Configuration and sensitive data management

### **Testing and Validation Strategies**

- **Unit Testing**: Individual service functionality
- **Integration Testing**: Cross-service communication and workflows
- **End-to-End Testing**: Complete business process validation
- **Performance Testing**: Response times and resource usage

### **Production Readiness Concepts**

- **Health Checks**: Liveness and readiness probes
- **Resource Management**: CPU and memory limits/requests
- **Security**: Non-root containers, network policies, secret management
- **Monitoring**: Metrics endpoints and log aggregation

---

**This guide represents a comprehensive educational journey through modern microservices deployment patterns and Kubernetes orchestration. Each service builds upon the previous ones, creating a complete, production-ready microservices ecosystem.** 🎉
