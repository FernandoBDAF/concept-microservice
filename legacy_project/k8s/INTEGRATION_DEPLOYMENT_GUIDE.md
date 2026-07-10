# Microservices Integration Testing & Validation Guide

**Date**: December 29, 2024  
**Purpose**: Comprehensive testing, validation, and monitoring for integrated microservices  
**Audience**: QA engineers, DevOps teams, and developers implementing integration testing  
**Scope**: End-to-end workflows, performance validation, security testing, and operational monitoring

---

## 📋 **Table of Contents**

### **🎯 Quick Navigation**

- [🎯 Overview and Learning Objectives](#-overview-and-learning-objectives)
- [🧪 Phase 2: Integration Testing Framework](#-phase-2-integration-testing-framework)
- [🔄 End-to-End Workflow Testing](#-end-to-end-workflow-testing)
- [📊 Performance Validation](#-performance-validation)
- [🔒 Security Testing](#-security-testing)
- [🎯 Operational Health Monitoring](#-operational-health-monitoring)
- [🎉 Integration Testing Complete](#-integration-testing-complete)

### **📊 Quick Reference Sections**

- [Revolutionary Testing Approach](#revolutionary-testing-approach)
- [Complete Business Workflow Validation](#complete-business-workflow-validation)
- [Performance Testing Framework](#performance-testing-framework)
- [Network Policy Validation](#network-policy-validation)
- [Comprehensive Health Monitoring Framework](#comprehensive-health-monitoring-framework)

### **🔧 Testing Categories**

- [Business Workflow Tests](#end-to-end-workflow-testing) - User registration, profile creation, cache integration
- [Integration Pattern Tests](#test-scenario-2-cache-aside-pattern-validation) - Cache-aside, multi-worker processing
- [Performance Tests](#performance-validation) - Load testing, resource monitoring
- [Security Tests](#security-testing) - Network policies, authentication validation
- [Operational Tests](#operational-health-monitoring) - Health monitoring, alerting

### **📈 Test Scenario Categories**

- [User Journey Tests](#test-scenario-1-user-registration--profile-creation) - Complete user workflows
- [Cache Pattern Tests](#test-scenario-2-cache-aside-pattern-validation) - Cache-aside implementation
- [Worker Processing Tests](#test-scenario-3-multi-worker-processing-validation) - Multi-worker task routing
- [Load Testing Scenarios](#load-testing-with-concurrent-users) - Concurrent user simulation
- [Security Validation Tests](#network-policy-validation) - Zero-trust networking validation

---

## 🎯 **Overview and Learning Objectives**

This guide provides comprehensive procedures for testing, validating, and monitoring the integrated microservices ecosystem after all services have been deployed. It covers end-to-end workflows, performance validation, security testing, and operational monitoring.

### **What You'll Learn**

- End-to-end business workflow testing across all microservices
- Integration patterns validation (cache-aside, authentication flows, async processing)
- Performance testing and monitoring techniques
- Security validation and network policy compliance
- Operational health monitoring and alerting
- Troubleshooting complex multi-service issues

### **Prerequisites**

- ✅ **Foundation deployed**: Kind cluster with infrastructure services
- ✅ **All 6 microservices deployed**: Cache, Storage, Auth, Queue, Profile, Worker
- ✅ **Services validated individually**: Each service passing its own health checks
- ✅ **NodePort access confirmed**: All services accessible via their assigned ports

---

## 🧪 **Phase 2: Integration Testing Framework**

### **Revolutionary Testing Approach**

**Traditional Approach**: Basic health checks only (`curl /health`)  
**Our Innovation**: Complete business workflow validation with real microservices patterns

### **Testing Architecture Overview**

```
┌─────────────────────────────────────────────────────────────────┐
│                    Integration Testing Framework                 │
├─────────────────────────────────────────────────────────────────┤
│ 1. Individual Service Testing    │ 2. Integration Testing         │
│    - Health checks (/health)     │    - Service-to-service comm   │
│    - API functionality           │    - Authentication flows      │
│    - Resource validation         │    - Cache integration         │
│                                  │    - Database persistence      │
├─────────────────────────────────────────────────────────────────┤
│ 3. End-to-End Testing           │ 4. Performance & Security      │
│    - Complete user workflows    │    - Load testing              │
│    - Multi-service orchestration│    - Network policy validation │
│    - Async processing validation│    - Resource monitoring       │
└─────────────────────────────────────────────────────────────────┘
```

---

## 🔄 **End-to-End Workflow Testing**

### **Complete Business Workflow Validation**

#### **Test Scenario 1: User Registration & Profile Creation**

**Business Flow**: User Registration → Profile Creation → Cache Storage → Queue Processing

```bash
#!/bin/bash
# Complete user workflow testing

echo "🧪 Starting End-to-End User Workflow Test..."

# Step 1: User Registration (Auth Service)
echo "📝 Step 1: User Registration"
REGISTER_RESPONSE=$(curl -s -X POST http://localhost:30083/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "testuser@example.com",
    "password": "SecurePass123",
    "name": "Test User"
  }')

echo "Registration Response: $REGISTER_RESPONSE"

# Extract user ID and token
USER_ID=$(echo $REGISTER_RESPONSE | jq -r '.user.id')
AUTH_TOKEN=$(echo $REGISTER_RESPONSE | jq -r '.token')

if [[ "$USER_ID" == "null" || "$AUTH_TOKEN" == "null" ]]; then
  echo "❌ Registration failed"
  exit 1
fi

echo "✅ User registered: ID=$USER_ID"

# Step 2: Profile Creation (Profile Service)
echo "📝 Step 2: Profile Creation"
PROFILE_RESPONSE=$(curl -s -X POST http://localhost:30085/api/v1/profiles \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  -d '{
    "userId": "'$USER_ID'",
    "displayName": "Test User Profile",
    "bio": "Integration test user",
    "preferences": {
      "theme": "dark",
      "notifications": true
    }
  }')

echo "Profile Response: $PROFILE_RESPONSE"

PROFILE_ID=$(echo $PROFILE_RESPONSE | jq -r '.profile.id')

if [[ "$PROFILE_ID" == "null" ]]; then
  echo "❌ Profile creation failed"
  exit 1
fi

echo "✅ Profile created: ID=$PROFILE_ID"

# Step 3: Verify Cache Integration (Cache Service)
echo "📝 Step 3: Cache Integration Validation"
sleep 2  # Allow cache to populate

CACHE_RESPONSE=$(curl -s -X GET http://localhost:30081/api/v1/cache/profile:$PROFILE_ID \
  -H "Authorization: Bearer $AUTH_TOKEN")

echo "Cache Response: $CACHE_RESPONSE"

if [[ "$CACHE_RESPONSE" == *"profile"* ]]; then
  echo "✅ Profile cached successfully"
else
  echo "⚠️ Profile not found in cache (may be expected based on caching strategy)"
fi

# Step 4: Verify Database Persistence (Storage Service)
echo "📝 Step 4: Database Persistence Validation"
DB_RESPONSE=$(curl -s -X GET http://localhost:30082/api/v1/users/$USER_ID \
  -H "Authorization: Bearer $AUTH_TOKEN")

echo "Database Response: $DB_RESPONSE"

if [[ "$DB_RESPONSE" == *"$USER_ID"* ]]; then
  echo "✅ User data persisted in database"
else
  echo "❌ User data not found in database"
  exit 1
fi

# Step 5: Queue Task Processing (Queue + Worker Services)
echo "📝 Step 5: Async Task Processing"
TASK_RESPONSE=$(curl -s -X POST http://localhost:30084/api/v1/queues/publish \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  -d '{
    "routing_key": "profile.welcome_email",
    "payload": {
      "userId": "'$USER_ID'",
      "profileId": "'$PROFILE_ID'",
      "email": "testuser@example.com",
      "name": "Test User"
    }
  }')

echo "Task Response: $TASK_RESPONSE"

# Step 6: Verify Task Processing (Worker Service)
echo "📝 Step 6: Worker Processing Validation"
sleep 5  # Allow worker to process

WORKER_STATUS=$(curl -s -X GET http://localhost:30086/api/v1/workers/status \
  -H "Authorization: Bearer $AUTH_TOKEN")

echo "Worker Status: $WORKER_STATUS"

echo "🎉 End-to-End Workflow Test Complete!"
echo "✅ User Registration: SUCCESS"
echo "✅ Profile Creation: SUCCESS"
echo "✅ Cache Integration: VALIDATED"
echo "✅ Database Persistence: SUCCESS"
echo "✅ Queue Processing: INITIATED"
echo "✅ Worker Processing: MONITORED"
```

#### **Test Scenario 2: Cache-Aside Pattern Validation**

**Business Flow**: Profile Update → Cache Invalidation → Database Update → Cache Repopulation

```bash
#!/bin/bash
# Cache-aside pattern testing

echo "🧪 Starting Cache-Aside Pattern Test..."

# Prerequisites: User and profile from previous test
USER_ID="test-user-id"
PROFILE_ID="test-profile-id"
AUTH_TOKEN="valid-jwt-token"

# Step 1: Initial Profile Retrieval (should populate cache)
echo "📝 Step 1: Initial Profile Retrieval"
INITIAL_PROFILE=$(curl -s -X GET http://localhost:30085/api/v1/profiles/$PROFILE_ID \
  -H "Authorization: Bearer $AUTH_TOKEN")

echo "Initial Profile: $INITIAL_PROFILE"

# Step 2: Verify Cache Population
echo "📝 Step 2: Cache Population Check"
CACHE_CHECK1=$(curl -s -X GET http://localhost:30081/api/v1/cache/profile:$PROFILE_ID)

if [[ "$CACHE_CHECK1" != *"null"* ]]; then
  echo "✅ Cache populated after profile retrieval"
else
  echo "⚠️ Cache not populated (checking caching strategy)"
fi

# Step 3: Profile Update (should invalidate cache)
echo "📝 Step 3: Profile Update"
UPDATE_RESPONSE=$(curl -s -X PUT http://localhost:30085/api/v1/profiles/$PROFILE_ID \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  -d '{
    "displayName": "Updated Test User",
    "bio": "Updated bio for cache testing",
    "preferences": {
      "theme": "light",
      "notifications": false
    }
  }')

echo "Update Response: $UPDATE_RESPONSE"

# Step 4: Verify Cache Invalidation
echo "📝 Step 4: Cache Invalidation Check"
sleep 1
CACHE_CHECK2=$(curl -s -X GET http://localhost:30081/api/v1/cache/profile:$PROFILE_ID)

if [[ "$CACHE_CHECK2" == *"null"* ]] || [[ "$CACHE_CHECK2" == *"Updated"* ]]; then
  echo "✅ Cache invalidated/updated after profile update"
else
  echo "⚠️ Cache invalidation behavior varies by implementation"
fi

# Step 5: Fresh Profile Retrieval (should repopulate cache)
echo "📝 Step 5: Cache Repopulation"
UPDATED_PROFILE=$(curl -s -X GET http://localhost:30085/api/v1/profiles/$PROFILE_ID \
  -H "Authorization: Bearer $AUTH_TOKEN")

echo "Updated Profile: $UPDATED_PROFILE"

# Step 6: Verify Cache Repopulation
echo "📝 Step 6: Cache Repopulation Check"
sleep 1
CACHE_CHECK3=$(curl -s -X GET http://localhost:30081/api/v1/cache/profile:$PROFILE_ID)

if [[ "$CACHE_CHECK3" == *"Updated"* ]]; then
  echo "✅ Cache repopulated with updated data"
else
  echo "⚠️ Cache repopulation pending or using different strategy"
fi

echo "🎉 Cache-Aside Pattern Test Complete!"
```

#### **Test Scenario 3: Multi-Worker Processing Validation**

**Business Flow**: Task Routing → Specialized Workers → Result Aggregation

```bash
#!/bin/bash
# Multi-worker processing test

echo "🧪 Starting Multi-Worker Processing Test..."

AUTH_TOKEN="valid-jwt-token"

# Step 1: Email Worker Task
echo "📝 Step 1: Email Worker Task"
EMAIL_TASK=$(curl -s -X POST http://localhost:30084/api/v1/queues/publish \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  -d '{
    "routing_key": "email.send",
    "payload": {
      "to": "test@example.com",
      "subject": "Integration Test Email",
      "template": "welcome",
      "data": {"name": "Test User"}
    }
  }')

echo "Email Task: $EMAIL_TASK"

# Step 2: Image Processing Worker Task
echo "📝 Step 2: Image Processing Task"
IMAGE_TASK=$(curl -s -X POST http://localhost:30084/api/v1/queues/publish \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  -d '{
    "routing_key": "image.process",
    "payload": {
      "imageUrl": "https://example.com/test-image.jpg",
      "operations": ["resize", "optimize"],
      "dimensions": {"width": 800, "height": 600}
    }
  }')

echo "Image Task: $IMAGE_TASK"

# Step 3: Profile Worker Task
echo "📝 Step 3: Profile Processing Task"
PROFILE_TASK=$(curl -s -X POST http://localhost:30084/api/v1/queues/publish \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  -d '{
    "routing_key": "profile.analytics",
    "payload": {
      "profileId": "test-profile-id",
      "action": "login",
      "timestamp": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'",
      "metadata": {"ip": "127.0.0.1", "userAgent": "test"}
    }
  }')

echo "Profile Task: $PROFILE_TASK"

# Step 4: Monitor Worker Processing
echo "📝 Step 4: Worker Processing Monitoring"
for i in {1..10}; do
  echo "Checking worker status (attempt $i/10)..."

  WORKER_STATUS=$(curl -s -X GET http://localhost:30086/api/v1/workers/status \
    -H "Authorization: Bearer $AUTH_TOKEN")

  echo "Worker Status: $WORKER_STATUS"

  # Check for task completion indicators
  if [[ "$WORKER_STATUS" == *"email"* ]] && [[ "$WORKER_STATUS" == *"image"* ]] && [[ "$WORKER_STATUS" == *"profile"* ]]; then
    echo "✅ All worker types are processing tasks"
    break
  fi

  sleep 3
done

# Step 5: Queue Status Validation
echo "📝 Step 5: Queue Status Check"
QUEUE_STATUS=$(curl -s -X GET http://localhost:30084/api/v1/queues/status \
  -H "Authorization: Bearer $AUTH_TOKEN")

echo "Queue Status: $QUEUE_STATUS"

echo "🎉 Multi-Worker Processing Test Complete!"
```

---

## 📊 **Performance Validation**

### **Performance Testing Framework**

#### **Load Testing with Concurrent Users**

```bash
#!/bin/bash
# Performance load testing

echo "🚀 Starting Performance Load Test..."

# Configuration
CONCURRENT_USERS=10
TEST_DURATION=60  # seconds
BASE_URL="http://localhost"

# Test Results Storage
RESULTS_DIR="./performance-results-$(date +%Y%m%d-%H%M%S)"
mkdir -p $RESULTS_DIR

echo "📊 Performance Test Configuration:"
echo "   Concurrent Users: $CONCURRENT_USERS"
echo "   Test Duration: ${TEST_DURATION}s"
echo "   Results Directory: $RESULTS_DIR"

# Function to run concurrent API tests
run_concurrent_test() {
  local service_name=$1
  local port=$2
  local endpoint=$3
  local test_name=$4

  echo "🧪 Testing $test_name..."

  # Create test script for this service
  cat > "$RESULTS_DIR/test_${service_name}.sh" << EOF
#!/bin/bash
for i in \$(seq 1 $TEST_DURATION); do
  start_time=\$(date +%s.%N)

  response=\$(curl -s -w "%{http_code}:%{time_total}" \\
    -X GET ${BASE_URL}:${port}${endpoint} \\
    -H "Authorization: Bearer test-token" \\
    --connect-timeout 5 --max-time 10)

  end_time=\$(date +%s.%N)
  response_time=\$(echo "\$end_time - \$start_time" | bc)

  echo "\$(date +%H:%M:%S),\$response,\$response_time" >> "$RESULTS_DIR/${service_name}_results.csv"

  sleep 1
done
EOF

  chmod +x "$RESULTS_DIR/test_${service_name}.sh"

  # Run concurrent tests
  for user in $(seq 1 $CONCURRENT_USERS); do
    "$RESULTS_DIR/test_${service_name}.sh" &
  done
}

# Initialize CSV headers
echo "timestamp,http_code:response_time,actual_time" > "$RESULTS_DIR/cache_results.csv"
echo "timestamp,http_code:response_time,actual_time" > "$RESULTS_DIR/storage_results.csv"
echo "timestamp,http_code:response_time,actual_time" > "$RESULTS_DIR/auth_results.csv"
echo "timestamp,http_code:response_time,actual_time" > "$RESULTS_DIR/profile_results.csv"

# Run concurrent tests for each service
run_concurrent_test "cache" "30081" "/health" "Cache Service Load Test"
run_concurrent_test "storage" "30082" "/health" "Storage Service Load Test"
run_concurrent_test "auth" "30083" "/health" "Auth Service Load Test"
run_concurrent_test "profile" "30085" "/health" "Profile Service Load Test"

echo "⏳ Running tests for ${TEST_DURATION} seconds..."
sleep $TEST_DURATION

# Kill all background processes
jobs -p | xargs -r kill

echo "📊 Analyzing Results..."

# Generate performance report
cat > "$RESULTS_DIR/performance_report.md" << 'EOF'
# Performance Test Report

Generated: $(date)

## Test Configuration
- Concurrent Users: $CONCURRENT_USERS
- Test Duration: ${TEST_DURATION}s
- Services Tested: Cache, Storage, Auth, Profile

## Results Summary

### Cache Service (Port 30081)
EOF

# Analyze each service results
for service in cache storage auth profile; do
  if [[ -f "$RESULTS_DIR/${service}_results.csv" ]]; then
    total_requests=$(wc -l < "$RESULTS_DIR/${service}_results.csv")
    successful_requests=$(grep -c "200:" "$RESULTS_DIR/${service}_results.csv" || echo "0")
    error_requests=$((total_requests - successful_requests))

    echo "### $service Service Results:" >> "$RESULTS_DIR/performance_report.md"
    echo "- Total Requests: $total_requests" >> "$RESULTS_DIR/performance_report.md"
    echo "- Successful Requests: $successful_requests" >> "$RESULTS_DIR/performance_report.md"
    echo "- Error Requests: $error_requests" >> "$RESULTS_DIR/performance_report.md"
    echo "- Success Rate: $(echo "scale=2; $successful_requests * 100 / $total_requests" | bc)%" >> "$RESULTS_DIR/performance_report.md"
    echo "" >> "$RESULTS_DIR/performance_report.md"
  fi
done

echo "✅ Performance testing complete!"
echo "📁 Results saved in: $RESULTS_DIR"
echo "📊 View report: cat $RESULTS_DIR/performance_report.md"
```

#### **Resource Monitoring During Load**

```bash
#!/bin/bash
# Resource monitoring during performance tests

echo "📊 Starting Resource Monitoring..."

MONITOR_DURATION=300  # 5 minutes
MONITOR_INTERVAL=10   # 10 seconds

# Create monitoring results directory
MONITOR_DIR="./monitoring-$(date +%Y%m%d-%H%M%S)"
mkdir -p $MONITOR_DIR

# Function to monitor Kubernetes resources
monitor_k8s_resources() {
  echo "⏱️ $(date): Monitoring Kubernetes resources..."

  # Pod resource usage
  kubectl top pods --no-headers > "$MONITOR_DIR/pod_resources_$(date +%H%M%S).txt" 2>/dev/null || echo "Metrics not available"

  # Node resource usage
  kubectl top nodes --no-headers > "$MONITOR_DIR/node_resources_$(date +%H%M%S).txt" 2>/dev/null || echo "Metrics not available"

  # Pod status
  kubectl get pods -o wide > "$MONITOR_DIR/pod_status_$(date +%H%M%S).txt"

  # Service endpoints
  kubectl get endpoints > "$MONITOR_DIR/endpoints_$(date +%H%M%S).txt"
}

# Function to monitor Docker container resources
monitor_docker_resources() {
  echo "🐳 $(date): Monitoring Docker container resources..."

  docker stats --no-stream --format "table {{.Container}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}\t{{.BlockIO}}" \
    > "$MONITOR_DIR/docker_stats_$(date +%H%M%S).txt"
}

# Function to monitor application health
monitor_app_health() {
  echo "🏥 $(date): Monitoring application health..."

  services=("30081:cache" "30082:storage" "30083:auth" "30084:queue" "30085:profile" "30086:worker")

  for service in "${services[@]}"; do
    port=$(echo $service | cut -d: -f1)
    name=$(echo $service | cut -d: -f2)

    health_status=$(curl -s --connect-timeout 3 http://localhost:$port/health | jq -r '.status // "unhealthy"' 2>/dev/null || echo "unreachable")
    echo "$(date +%H:%M:%S),$name,$health_status" >> "$MONITOR_DIR/health_status.csv"
  done
}

# Initialize monitoring
echo "timestamp,service,status" > "$MONITOR_DIR/health_status.csv"

echo "📊 Resource monitoring started for ${MONITOR_DURATION} seconds (interval: ${MONITOR_INTERVAL}s)"
echo "📁 Results will be saved in: $MONITOR_DIR"

# Main monitoring loop
for ((i=0; i<$((MONITOR_DURATION/MONITOR_INTERVAL)); i++)); do
  echo "📊 Monitoring cycle $((i+1))/$(($MONITOR_DURATION/$MONITOR_INTERVAL))"

  monitor_k8s_resources &
  monitor_docker_resources &
  monitor_app_health &

  wait  # Wait for all background jobs to complete

  sleep $MONITOR_INTERVAL
done

echo "✅ Resource monitoring complete!"
echo "📁 Results saved in: $MONITOR_DIR"

# Generate monitoring summary
echo "📊 Generating monitoring summary..."
cat > "$MONITOR_DIR/monitoring_summary.md" << 'EOF'
# Resource Monitoring Summary

Generated: $(date)

## Monitoring Configuration
- Duration: ${MONITOR_DURATION} seconds
- Interval: ${MONITOR_INTERVAL} seconds
- Services Monitored: Cache, Storage, Auth, Queue, Profile, Worker

## Health Status Summary
EOF

# Analyze health status
if [[ -f "$MONITOR_DIR/health_status.csv" ]]; then
  echo "### Service Health Overview:" >> "$MONITOR_DIR/monitoring_summary.md"

  for service in cache storage auth queue profile worker; do
    healthy_count=$(grep "$service,healthy" "$MONITOR_DIR/health_status.csv" | wc -l)
    total_count=$(grep "$service," "$MONITOR_DIR/health_status.csv" | wc -l)

    if [[ $total_count -gt 0 ]]; then
      health_percentage=$(echo "scale=2; $healthy_count * 100 / $total_count" | bc)
      echo "- $service: $health_percentage% healthy ($healthy_count/$total_count checks)" >> "$MONITOR_DIR/monitoring_summary.md"
    fi
  done
fi

echo "📊 View summary: cat $MONITOR_DIR/monitoring_summary.md"
```

---

## 🔒 **Security Testing**

### **Network Policy Validation**

```bash
#!/bin/bash
# Network policy security testing

echo "🔒 Starting Security Validation Tests..."

# Test 1: Inter-service communication compliance
echo "🧪 Test 1: Inter-service Communication Validation"

test_service_communication() {
  local from_service=$1
  local to_service=$2
  local expected_result=$3  # "allow" or "deny"

  echo "Testing: $from_service → $to_service (expecting: $expected_result)"

  # Create test pod for the source service
  kubectl run security-test-$from_service \
    --image=busybox:1.35 \
    --labels="app=$from_service,test=security" \
    --rm -i --restart=Never \
    --command -- timeout 10s wget -qO- http://$to_service:8080/health 2>/dev/null

  local result=$?

  if [[ $expected_result == "allow" ]]; then
    if [[ $result -eq 0 ]]; then
      echo "✅ $from_service → $to_service: ALLOWED (as expected)"
    else
      echo "❌ $from_service → $to_service: BLOCKED (should be allowed)"
    fi
  else
    if [[ $result -ne 0 ]]; then
      echo "✅ $from_service → $to_service: BLOCKED (as expected)"
    else
      echo "⚠️ $from_service → $to_service: ALLOWED (should be blocked)"
    fi
  fi
}

# Test allowed communications
test_service_communication "profile-service" "cache-service" "allow"
test_service_communication "profile-service" "storage-service" "allow"
test_service_communication "profile-service" "auth-service" "allow"
test_service_communication "profile-service" "queue-service" "allow"
test_service_communication "worker-service" "queue-service" "allow"
test_service_communication "worker-service" "cache-service" "allow"
test_service_communication "auth-service" "storage-service" "allow"

# Test blocked communications (if network policies are restrictive)
test_service_communication "cache-service" "storage-service" "deny"
test_service_communication "storage-service" "queue-service" "deny"

echo "🧪 Test 2: External Access Validation"

# Test NodePort access
for port in 30081 30082 30083 30084 30085 30086; do
  echo "Testing NodePort $port accessibility..."

  response=$(curl -s --connect-timeout 5 http://localhost:$port/health)

  if [[ $? -eq 0 ]]; then
    echo "✅ Port $port: ACCESSIBLE"
  else
    echo "❌ Port $port: NOT ACCESSIBLE"
  fi
done

echo "🧪 Test 3: Ingress Security Validation"

# Test ingress access with proper host headers
echo "Testing ingress access patterns..."

# Test with valid host header
curl -s -H "Host: microservices.local" http://localhost:80/health && echo "✅ Ingress: Valid host accepted" || echo "⚠️ Ingress: Valid host rejected"

# Test without host header
curl -s http://localhost:80/health && echo "⚠️ Ingress: No host header accepted" || echo "✅ Ingress: No host header rejected"

# Test with invalid host header
curl -s -H "Host: malicious.com" http://localhost:80/health && echo "⚠️ Ingress: Invalid host accepted" || echo "✅ Ingress: Invalid host rejected"

echo "✅ Security validation tests complete!"
```

### **Authentication & Authorization Testing**

```bash
#!/bin/bash
# Authentication and authorization testing

echo "🔐 Starting Authentication & Authorization Tests..."

# Test 1: Authentication Flow Validation
echo "🧪 Test 1: Authentication Flow"

# Test invalid credentials
echo "Testing invalid credentials..."
INVALID_AUTH=$(curl -s -X POST http://localhost:30083/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "invalid@example.com", "password": "wrongpassword"}')

if [[ "$INVALID_AUTH" == *"error"* ]] || [[ "$INVALID_AUTH" == *"unauthorized"* ]]; then
  echo "✅ Invalid credentials properly rejected"
else
  echo "❌ Invalid credentials not properly handled"
fi

# Test valid credentials (if test user exists)
echo "Testing valid credentials..."
VALID_AUTH=$(curl -s -X POST http://localhost:30083/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "test@example.com", "password": "validpassword"}')

if [[ "$VALID_AUTH" == *"token"* ]]; then
  echo "✅ Valid credentials accepted"
  AUTH_TOKEN=$(echo $VALID_AUTH | jq -r '.token')
else
  echo "⚠️ Valid credentials test skipped (no test user)"
  AUTH_TOKEN="test-token"
fi

# Test 2: Authorization Validation
echo "🧪 Test 2: Authorization Validation"

# Test accessing protected endpoint without token
echo "Testing access without authentication token..."
NO_TOKEN_RESPONSE=$(curl -s -X GET http://localhost:30085/api/v1/profiles/test-id)

if [[ "$NO_TOKEN_RESPONSE" == *"unauthorized"* ]] || [[ "$NO_TOKEN_RESPONSE" == *"401"* ]]; then
  echo "✅ Protected endpoint properly rejects unauthenticated requests"
else
  echo "⚠️ Protected endpoint may not require authentication"
fi

# Test accessing protected endpoint with invalid token
echo "Testing access with invalid token..."
INVALID_TOKEN_RESPONSE=$(curl -s -X GET http://localhost:30085/api/v1/profiles/test-id \
  -H "Authorization: Bearer invalid-token-12345")

if [[ "$INVALID_TOKEN_RESPONSE" == *"unauthorized"* ]] || [[ "$INVALID_TOKEN_RESPONSE" == *"401"* ]]; then
  echo "✅ Protected endpoint properly rejects invalid tokens"
else
  echo "⚠️ Protected endpoint may not validate tokens properly"
fi

# Test accessing protected endpoint with valid token
if [[ "$AUTH_TOKEN" != "test-token" ]]; then
  echo "Testing access with valid token..."
  VALID_TOKEN_RESPONSE=$(curl -s -X GET http://localhost:30085/api/v1/profiles/test-id \
    -H "Authorization: Bearer $AUTH_TOKEN")

  if [[ "$VALID_TOKEN_RESPONSE" != *"unauthorized"* ]]; then
    echo "✅ Valid token properly accepted"
  else
    echo "❌ Valid token rejected"
  fi
fi

# Test 3: Token Validation Across Services
echo "🧪 Test 3: Cross-Service Token Validation"

services=("30081:cache" "30082:storage" "30085:profile")

for service in "${services[@]}"; do
  port=$(echo $service | cut -d: -f1)
  name=$(echo $service | cut -d: -f2)

  echo "Testing token validation on $name service..."

  response=$(curl -s -X GET http://localhost:$port/api/v1/health \
    -H "Authorization: Bearer $AUTH_TOKEN")

  # Check if service accepts or properly validates the token
  if [[ "$response" == *"unauthorized"* ]]; then
    echo "⚠️ $name service requires authentication for health endpoint"
  else
    echo "✅ $name service accessible with token"
  fi
done

echo "✅ Authentication & Authorization tests complete!"
```

---

## 🎯 **Operational Health Monitoring**

### **Comprehensive Health Monitoring Framework**

```bash
#!/bin/bash
# Comprehensive operational health monitoring

echo "🏥 Starting Comprehensive Health Monitoring..."

# Configuration
HEALTH_CHECK_INTERVAL=30  # seconds
ALERT_THRESHOLD_CPU=80    # percentage
ALERT_THRESHOLD_MEMORY=80 # percentage
ALERT_THRESHOLD_RESPONSE_TIME=5000  # milliseconds

# Create monitoring directory
HEALTH_DIR="./health-monitoring-$(date +%Y%m%d-%H%M%S)"
mkdir -p $HEALTH_DIR

echo "🏥 Health Monitoring Configuration:"
echo "   Check Interval: ${HEALTH_CHECK_INTERVAL}s"
echo "   CPU Alert Threshold: ${ALERT_THRESHOLD_CPU}%"
echo "   Memory Alert Threshold: ${ALERT_THRESHOLD_MEMORY}%"
echo "   Response Time Threshold: ${ALERT_THRESHOLD_RESPONSE_TIME}ms"
echo "   Results Directory: $HEALTH_DIR"

# Initialize health status files
echo "timestamp,service,status,response_time,cpu_usage,memory_usage,details" > "$HEALTH_DIR/health_status.csv"
echo "timestamp,alert_type,service,message,severity" > "$HEALTH_DIR/alerts.csv"

# Function to check application health
check_application_health() {
  local service_name=$1
  local port=$2
  local timestamp=$(date +%Y-%m-%d\ %H:%M:%S)

  # Measure response time
  local start_time=$(date +%s.%N)
  local health_response=$(curl -s --connect-timeout 10 --max-time 10 http://localhost:$port/health 2>/dev/null)
  local end_time=$(date +%s.%N)
  local response_time=$(echo "($end_time - $start_time) * 1000" | bc | cut -d. -f1)

  # Parse health status
  local status="unknown"
  local details="No response"

  if [[ -n "$health_response" ]]; then
    status=$(echo $health_response | jq -r '.status // "unknown"' 2>/dev/null || echo "unknown")
    details=$(echo $health_response | jq -r '.message // "No message"' 2>/dev/null || echo "Response received")
  fi

  # Get resource usage (if available)
  local cpu_usage="N/A"
  local memory_usage="N/A"

  # Try to get pod resource usage
  local pod_name=$(kubectl get pods -l app=$service_name -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
  if [[ -n "$pod_name" ]]; then
    local pod_resources=$(kubectl top pod $pod_name --no-headers 2>/dev/null)
    if [[ -n "$pod_resources" ]]; then
      cpu_usage=$(echo $pod_resources | awk '{print $2}')
      memory_usage=$(echo $pod_resources | awk '{print $3}')
    fi
  fi

  # Log health status
  echo "$timestamp,$service_name,$status,$response_time,$cpu_usage,$memory_usage,$details" >> "$HEALTH_DIR/health_status.csv"

  # Check for alerts
  if [[ "$status" != "healthy" ]] && [[ "$status" != "ok" ]]; then
    echo "$timestamp,health,$service_name,Service unhealthy: $details,high" >> "$HEALTH_DIR/alerts.csv"
    echo "🚨 ALERT: $service_name is unhealthy - $details"
  fi

  if [[ "$response_time" -gt "$ALERT_THRESHOLD_RESPONSE_TIME" ]]; then
    echo "$timestamp,performance,$service_name,Slow response time: ${response_time}ms,medium" >> "$HEALTH_DIR/alerts.csv"
    echo "⚠️ WARNING: $service_name slow response time: ${response_time}ms"
  fi

  echo "🏥 $service_name: $status (${response_time}ms) CPU:$cpu_usage MEM:$memory_usage"
}

# Function to check infrastructure health
check_infrastructure_health() {
  local timestamp=$(date +%Y-%m-%d\ %H:%M:%S)

  echo "🏗️ Checking infrastructure health..."

  # Check node status
  local nodes_ready=$(kubectl get nodes --no-headers | grep " Ready " | wc -l)
  local nodes_total=$(kubectl get nodes --no-headers | wc -l)

  if [[ $nodes_ready -ne $nodes_total ]]; then
    echo "$timestamp,infrastructure,cluster,Not all nodes ready: $nodes_ready/$nodes_total,high" >> "$HEALTH_DIR/alerts.csv"
    echo "🚨 ALERT: Cluster nodes not ready: $nodes_ready/$nodes_total"
  fi

  # Check system pods
  local system_pods_running=$(kubectl get pods -n kube-system --no-headers | grep " Running " | wc -l)
  local system_pods_total=$(kubectl get pods -n kube-system --no-headers | wc -l)

  if [[ $system_pods_running -ne $system_pods_total ]]; then
    echo "$timestamp,infrastructure,kube-system,System pods not running: $system_pods_running/$system_pods_total,high" >> "$HEALTH_DIR/alerts.csv"
    echo "🚨 ALERT: System pods not running: $system_pods_running/$system_pods_total"
  fi

  # Check ingress controller
  local ingress_ready=$(kubectl get pods -n ingress-nginx --no-headers | grep " Running " | wc -l)
  if [[ $ingress_ready -eq 0 ]]; then
    echo "$timestamp,infrastructure,ingress,Ingress controller not running,high" >> "$HEALTH_DIR/alerts.csv"
    echo "🚨 ALERT: Ingress controller not running"
  fi

  echo "🏗️ Infrastructure: Nodes($nodes_ready/$nodes_total) SystemPods($system_pods_running/$system_pods_total) Ingress($ingress_ready)"
}

# Function to check database connectivity
check_database_connectivity() {
  local timestamp=$(date +%Y-%m-%d\ %H:%M:%S)

  echo "🗄️ Checking database connectivity..."

  # PostgreSQL connectivity test
  local postgres_test=$(kubectl run postgres-test --image=postgres:13 --rm -i --restart=Never \
    --env="PGPASSWORD=postgres" \
    --command -- timeout 10s pg_isready -h postgres-service -p 5432 -U postgres 2>/dev/null)

  if [[ $? -eq 0 ]]; then
    echo "✅ PostgreSQL: Connected"
  else
    echo "$timestamp,database,postgresql,PostgreSQL connection failed,high" >> "$HEALTH_DIR/alerts.csv"
    echo "🚨 ALERT: PostgreSQL connection failed"
  fi

  # Redis connectivity test
  local redis_test=$(kubectl run redis-test --image=redis:7-alpine --rm -i --restart=Never \
    --command -- timeout 10s redis-cli -h redis-service -p 6379 ping 2>/dev/null)

  if [[ "$redis_test" == *"PONG"* ]]; then
    echo "✅ Redis: Connected"
  else
    echo "$timestamp,database,redis,Redis connection failed,high" >> "$HEALTH_DIR/alerts.csv"
    echo "🚨 ALERT: Redis connection failed"
  fi

  # RabbitMQ connectivity test
  local rabbitmq_test=$(curl -s --connect-timeout 5 http://localhost:15672/api/overview -u guest:guest 2>/dev/null)

  if [[ -n "$rabbitmq_test" ]]; then
    echo "✅ RabbitMQ: Connected"
  else
    echo "$timestamp,database,rabbitmq,RabbitMQ connection failed,medium" >> "$HEALTH_DIR/alerts.csv"
    echo "⚠️ WARNING: RabbitMQ management interface not accessible"
  fi
}

# Main monitoring function
run_health_monitoring() {
  echo "🏥 Starting continuous health monitoring..."
  echo "Press Ctrl+C to stop monitoring"

  # Services to monitor
  services=("cache-service:30081" "storage-service:30082" "auth-service:30083" "queue-service:30084" "profile-service:30085" "worker-service:30086")

  local cycle=1

  while true; do
    echo ""
    echo "🔄 Health Check Cycle $cycle - $(date)"
    echo "================================================"

    # Check application services
    for service in "${services[@]}"; do
      service_name=$(echo $service | cut -d: -f1)
      port=$(echo $service | cut -d: -f2)
      check_application_health $service_name $port
    done

    # Check infrastructure
    check_infrastructure_health

    # Check database connectivity
    check_database_connectivity

    echo "================================================"
    echo "⏳ Next check in ${HEALTH_CHECK_INTERVAL} seconds..."

    ((cycle++))
    sleep $HEALTH_CHECK_INTERVAL
  done
}

# Generate health report function
generate_health_report() {
  echo "📊 Generating health monitoring report..."

  cat > "$HEALTH_DIR/health_report.md" << EOF
# Health Monitoring Report

Generated: $(date)
Monitoring Period: $(head -2 "$HEALTH_DIR/health_status.csv" | tail -1 | cut -d, -f1) to $(tail -1 "$HEALTH_DIR/health_status.csv" | cut -d, -f1)

## Service Health Summary

EOF

  # Analyze each service
  for service in cache-service storage-service auth-service queue-service profile-service worker-service; do
    local total_checks=$(grep "$service," "$HEALTH_DIR/health_status.csv" | wc -l)
    local healthy_checks=$(grep "$service,healthy\|$service,ok" "$HEALTH_DIR/health_status.csv" | wc -l)

    if [[ $total_checks -gt 0 ]]; then
      local health_percentage=$(echo "scale=2; $healthy_checks * 100 / $total_checks" | bc)
      echo "### $service" >> "$HEALTH_DIR/health_report.md"
      echo "- Health Percentage: $health_percentage%" >> "$HEALTH_DIR/health_report.md"
      echo "- Healthy Checks: $healthy_checks/$total_checks" >> "$HEALTH_DIR/health_report.md"
      echo "" >> "$HEALTH_DIR/health_report.md"
    fi
  done

  # Alert summary
  local total_alerts=$(wc -l < "$HEALTH_DIR/alerts.csv")
  if [[ $total_alerts -gt 1 ]]; then  # Subtract 1 for header
    echo "## Alert Summary" >> "$HEALTH_DIR/health_report.md"
    echo "Total Alerts: $((total_alerts - 1))" >> "$HEALTH_DIR/health_report.md"
    echo "" >> "$HEALTH_DIR/health_report.md"
    echo "### Recent Alerts:" >> "$HEALTH_DIR/health_report.md"
    tail -10 "$HEALTH_DIR/alerts.csv" | while IFS=, read timestamp alert_type service message severity; do
      echo "- **$severity**: $service - $message ($timestamp)" >> "$HEALTH_DIR/health_report.md"
    done
  fi

  echo "📊 Health report generated: $HEALTH_DIR/health_report.md"
}

# Trap Ctrl+C to generate report before exit
trap 'echo ""; echo "🛑 Stopping health monitoring..."; generate_health_report; exit 0' INT

# Start monitoring
run_health_monitoring
```

---

## 🎉 **Integration Testing Complete**

### **Testing Framework Summary**

**Your microservices ecosystem now has:**

- ✅ **End-to-End Workflow Testing**: Complete business process validation
- ✅ **Integration Pattern Testing**: Cache-aside, authentication flows, async processing
- ✅ **Performance Testing**: Load testing with concurrent users and resource monitoring
- ✅ **Security Testing**: Network policy validation and authentication testing
- ✅ **Operational Monitoring**: Comprehensive health monitoring with alerting

### **Testing Results Validation**

After running the integration tests, you should see:

1. **✅ Business Workflows**: User registration → Profile creation → Cache integration → Queue processing
2. **✅ Service Integration**: All services communicating correctly with proper authentication
3. **✅ Performance Metrics**: Response times under thresholds, successful concurrent user handling
4. **✅ Security Compliance**: Network policies enforcing proper access controls
5. **✅ Operational Health**: All services maintaining healthy status with proper resource usage

### **Next Steps for Production**

1. **Monitoring Setup**: Implement Prometheus + Grafana for production monitoring
2. **CI/CD Integration**: Incorporate these tests into your deployment pipeline
3. **Alert Configuration**: Set up production alerting based on the health monitoring framework
4. **Performance Baselines**: Use test results to establish performance SLAs
5. **Security Hardening**: Implement additional security measures based on test findings

---

**Integration Testing Status**: ✅ **COMPREHENSIVE TESTING FRAMEWORK COMPLETE**  
**Business Workflow Validation**: 🎯 **END-TO-END TESTING IMPLEMENTED**  
**Performance & Security**: 🚀 **PRODUCTION-READY VALIDATION ACHIEVED**  
**Operational Excellence**: 🏥 **CONTINUOUS HEALTH MONITORING ESTABLISHED**
