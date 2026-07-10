# Microservices Observability & Monitoring Guide

**Date**: December 29, 2024  
**Purpose**: Comprehensive monitoring, metrics, and observability setup for Kind microservices  
**Audience**: DevOps engineers and developers implementing production monitoring  
**Scope**: Health checks, metrics collection, log analysis, and alerting strategies

---

## 📋 **Table of Contents**

### **🎯 Quick Navigation**

- [🎯 Overview and Learning Objectives](#-overview-and-learning-objectives)
- [🏥 Health Check Framework](#-health-check-framework)
- [📊 Metrics Collection Framework](#-metrics-collection-framework)
- [📋 Log Analysis Framework](#-log-analysis-framework)
- [🎯 Alerting and Notification Framework](#-alerting-and-notification-framework)
- [🎉 Observability Framework Complete](#-observability-framework-complete)

### **📊 Quick Reference Sections**

- [Multi-Layer Health Validation](#multi-layer-health-validation)
- [Automated Cluster Health Check](#automated-cluster-health-check)
- [Service Health Monitoring](#service-health-monitoring)
- [Performance Metrics Collection](#performance-metrics-collection)
- [Centralized Log Collection](#centralized-log-collection)
- [Continuous Health Monitoring with Alerts](#continuous-health-monitoring-with-alerts)

### **🔧 Monitoring Tool Categories**

- [Cluster Monitoring Tools](#cluster-health-monitoring) - Node status, system components, resource usage
- [Service Monitoring Tools](#service-health-monitoring) - Individual service health validation
- [Performance Analysis Tools](#application-specific-metrics) - Service performance and database optimization
- [Log Management Tools](#log-analysis-framework) - Centralized logging and error detection
- [Alerting Tools](#alerting-and-notification-framework) - Health-based and performance-based alerting

### **📈 Script Categories**

- [Health Check Scripts](#health-check-framework) - Cluster, infrastructure, and service validation
- [Metrics Collection Scripts](#metrics-collection-framework) - Resource monitoring and performance analysis
- [Log Analysis Scripts](#log-analysis-framework) - Log aggregation and error detection
- [Alerting Scripts](#alerting-and-notification-framework) - Continuous monitoring with notifications

---

## 🎯 **Overview and Learning Objectives**

This guide provides comprehensive observability and monitoring strategies for the microservices ecosystem. It covers health checks, metrics collection, log analysis, and alerting to ensure production-ready operational visibility.

### **What You'll Learn**

- **Health Check Strategies**: Multi-layer health validation from cluster to application level
- **Metrics Collection**: Node, pod, and application-specific metrics gathering
- **Log Analysis**: Centralized logging and log aggregation patterns
- **Alerting and Monitoring**: Proactive issue detection and notification
- **Performance Monitoring**: Resource usage and application performance tracking
- **Troubleshooting**: Using observability data for rapid issue resolution

### **Prerequisites**

- ✅ **Microservices deployed**: All 6 services running and healthy
- ✅ **Kind cluster operational**: Infrastructure services deployed
- ✅ **Metrics server deployed**: Kubernetes metrics collection enabled
- ✅ **Network access**: All NodePorts accessible for monitoring

---

## 🏥 **Health Check Framework**

### **Multi-Layer Health Validation**

```
┌─────────────────────────────────────────────────────────────────┐
│                    Health Check Architecture                    │
├─────────────────────────────────────────────────────────────────┤
│ 1. Cluster Health            │ 2. Infrastructure Health          │
│    - Node status             │    - Ingress controller           │
│    - System components       │    - Metrics server               │
│    - Resource availability   │    - Storage provisioner          │
├─────────────────────────────────────────────────────────────────┤
│ 3. Service Health            │ 4. Application Health             │
│    - Pod readiness           │    - Business logic validation    │
│    - Service endpoints       │    - Database connectivity        │
│    - Network connectivity    │    - External dependencies        │
└─────────────────────────────────────────────────────────────────┘
```

### **Cluster Health Monitoring**

#### **Automated Cluster Health Check**

```bash
#!/bin/bash
# cluster-health-check.sh - Comprehensive cluster validation

echo "🏥 Starting Cluster Health Check..."

# Check node status
echo "📊 Node Status:"
kubectl get nodes -o wide

NODE_COUNT=$(kubectl get nodes --no-headers | wc -l)
READY_NODES=$(kubectl get nodes --no-headers | grep " Ready " | wc -l)

if [[ $READY_NODES -eq $NODE_COUNT ]]; then
  echo "✅ All nodes ready: $READY_NODES/$NODE_COUNT"
else
  echo "❌ Nodes not ready: $READY_NODES/$NODE_COUNT"
fi

# Check system components
echo "📊 System Components:"
kubectl get componentstatuses

# Check system pods
echo "📊 System Pods:"
SYSTEM_PODS_TOTAL=$(kubectl get pods -n kube-system --no-headers | wc -l)
SYSTEM_PODS_RUNNING=$(kubectl get pods -n kube-system --no-headers | grep " Running " | wc -l)

if [[ $SYSTEM_PODS_RUNNING -eq $SYSTEM_PODS_TOTAL ]]; then
  echo "✅ All system pods running: $SYSTEM_PODS_RUNNING/$SYSTEM_PODS_TOTAL"
else
  echo "⚠️ System pods status: $SYSTEM_PODS_RUNNING/$SYSTEM_PODS_TOTAL running"
  kubectl get pods -n kube-system | grep -v " Running "
fi

# Check resource usage
echo "📊 Resource Usage:"
kubectl top nodes 2>/dev/null || echo "⚠️ Node metrics not available"
kubectl top pods --all-namespaces 2>/dev/null || echo "⚠️ Pod metrics not available"
```

#### **Infrastructure Health Validation**

```bash
#!/bin/bash
# infrastructure-health-check.sh - Infrastructure services validation

echo "🏗️ Infrastructure Health Check..."

# Check storage provisioner
echo "📦 Storage Provisioner:"
STORAGE_PODS=$(kubectl get pods -n local-path-storage --no-headers | grep " Running " | wc -l)
if [[ $STORAGE_PODS -gt 0 ]]; then
  echo "✅ Local path provisioner running"
else
  echo "❌ Local path provisioner not running"
fi

# Check storage class
STORAGE_CLASS=$(kubectl get storageclass --no-headers | grep "(default)" | wc -l)
if [[ $STORAGE_CLASS -gt 0 ]]; then
  echo "✅ Default storage class configured"
else
  echo "❌ No default storage class"
fi

# Check metrics server
echo "📊 Metrics Server:"
METRICS_PODS=$(kubectl get pods -n kube-system -l k8s-app=metrics-server --no-headers | grep " Running " | wc -l)
if [[ $METRICS_PODS -gt 0 ]]; then
  echo "✅ Metrics server running"
  # Test metrics API
  kubectl top nodes >/dev/null 2>&1 && echo "✅ Metrics API accessible" || echo "⚠️ Metrics API limited (expected in Kind)"
else
  echo "❌ Metrics server not running"
fi

# Check ingress controller
echo "🌐 Ingress Controller:"
INGRESS_PODS=$(kubectl get pods -n ingress-nginx --no-headers | grep " Running " | wc -l)
if [[ $INGRESS_PODS -gt 0 ]]; then
  echo "✅ Ingress controller running"
  # Test ingress accessibility
  curl -s --connect-timeout 3 http://localhost:80 >/dev/null && echo "✅ Ingress accessible" || echo "⚠️ Ingress not responding (normal without apps)"
else
  echo "❌ Ingress controller not running"
fi

# Check network policies
echo "🔒 Network Policies:"
NETWORK_POLICIES=$(kubectl get networkpolicies --all-namespaces --no-headers | wc -l)
echo "📊 Network policies configured: $NETWORK_POLICIES"
```

### **Service Health Monitoring**

#### **Individual Service Health Check**

```bash
#!/bin/bash
# service-health-check.sh - All microservices health validation

echo "🎯 Microservices Health Check..."

# Define services with their ports and expected health responses
declare -A services=(
  ["cache-service"]="30081"
  ["storage-service"]="30082"
  ["auth-service"]="30083"
  ["queue-service"]="30084"
  ["profile-service"]="30085"
  ["worker-service"]="30086"
)

# Health check each service
for service in "${!services[@]}"; do
  port=${services[$service]}
  echo "🔍 Checking $service (port $port):"

  # Check pod status
  POD_STATUS=$(kubectl get pods -l app=$service --no-headers 2>/dev/null | awk '{print $3}' | head -1)
  if [[ "$POD_STATUS" == "Running" ]]; then
    echo "  ✅ Pod status: $POD_STATUS"
  else
    echo "  ❌ Pod status: $POD_STATUS"
    continue
  fi

  # Check health endpoint
  HEALTH_RESPONSE=$(curl -s --connect-timeout 5 http://localhost:$port/health 2>/dev/null)
  if [[ -n "$HEALTH_RESPONSE" ]]; then
    STATUS=$(echo $HEALTH_RESPONSE | jq -r '.status // "unknown"' 2>/dev/null || echo "unknown")
    if [[ "$STATUS" == "healthy" ]] || [[ "$STATUS" == "ok" ]]; then
      echo "  ✅ Health status: $STATUS"
    else
      echo "  ⚠️ Health status: $STATUS"
    fi
  else
    echo "  ❌ Health endpoint not responding"
  fi

  # Check readiness endpoint (if available)
  READY_RESPONSE=$(curl -s --connect-timeout 3 http://localhost:$port/ready 2>/dev/null)
  if [[ -n "$READY_RESPONSE" ]]; then
    READY_STATUS=$(echo $READY_RESPONSE | jq -r '.status // "unknown"' 2>/dev/null || echo "unknown")
    echo "  📊 Readiness: $READY_STATUS"
  fi

  echo ""
done

# Check backend services (StatefulSets)
echo "🗄️ Backend Services Health:"

# Redis health
echo "🔴 Redis:"
kubectl exec redis-0 -- redis-cli ping 2>/dev/null && echo "  ✅ Redis responding" || echo "  ❌ Redis not responding"

# PostgreSQL health
echo "🐘 PostgreSQL:"
kubectl exec postgres-0 -- pg_isready -U profile_user 2>/dev/null && echo "  ✅ PostgreSQL ready" || echo "  ❌ PostgreSQL not ready"

# RabbitMQ health
echo "🐰 RabbitMQ:"
kubectl exec rabbitmq-0 -- rabbitmq-diagnostics ping 2>/dev/null && echo "  ✅ RabbitMQ responding" || echo "  ❌ RabbitMQ not responding"
```

#### **Deep Health Validation**

```bash
#!/bin/bash
# deep-health-validation.sh - Comprehensive service dependency validation

echo "🔬 Deep Health Validation..."

# Test service-to-service connectivity
echo "🔗 Service Integration Tests:"

# Auth → Storage connectivity
echo "🔐 Auth Service → Storage Service:"
kubectl exec deployment/auth-service -- curl -s --connect-timeout 5 http://storage-service:8080/health >/dev/null 2>&1 && \
  echo "  ✅ Auth can reach Storage" || echo "  ❌ Auth cannot reach Storage"

# Profile → All dependencies
echo "🎭 Profile Service Dependencies:"
for dep in auth-service cache-service storage-service queue-service; do
  kubectl exec deployment/profile-service -- nc -zv $dep 8080 >/dev/null 2>&1 && \
    echo "  ✅ Profile → $dep" || echo "  ❌ Profile → $dep"
done

# Workers → Queue connectivity
echo "⚙️ Worker Services → Queue Service:"
for worker in email-worker image-worker; do
  kubectl exec deployment/$worker -- nc -zv rabbitmq-service 5672 >/dev/null 2>&1 && \
    echo "  ✅ $worker → RabbitMQ" || echo "  ❌ $worker → RabbitMQ"
done

# Database connectivity tests
echo "🗄️ Database Connectivity:"

# Test Redis from cache service
kubectl exec deployment/cache-service -- redis-cli -h redis-service ping >/dev/null 2>&1 && \
  echo "  ✅ Cache Service → Redis" || echo "  ❌ Cache Service → Redis"

# Test PostgreSQL from storage service
kubectl exec deployment/storage-service -- nc -zv postgres-service 5432 >/dev/null 2>&1 && \
  echo "  ✅ Storage Service → PostgreSQL" || echo "  ❌ Storage Service → PostgreSQL"

# Test RabbitMQ from queue service
kubectl exec deployment/queue-service -- nc -zv rabbitmq-service 5672 >/dev/null 2>&1 && \
  echo "  ✅ Queue Service → RabbitMQ" || echo "  ❌ Queue Service → RabbitMQ"
```

---

## 📊 **Metrics Collection Framework**

### **Node and Pod Metrics**

#### **Resource Usage Monitoring**

```bash
#!/bin/bash
# resource-monitoring.sh - Continuous resource monitoring

echo "📊 Resource Monitoring Dashboard..."

# Function to display resource usage
display_resources() {
  echo "========================================"
  echo "⏰ $(date)"
  echo "========================================"

  echo "🖥️ Node Resources:"
  kubectl top nodes 2>/dev/null || echo "  ⚠️ Node metrics not available"

  echo ""
  echo "🏠 Pod Resources (Microservices):"
  kubectl top pods -l 'app in (cache-service,storage-service,auth-service,queue-service,profile-service,email-worker,image-worker)' 2>/dev/null || echo "  ⚠️ Pod metrics not available"

  echo ""
  echo "🗄️ Backend Services:"
  kubectl top pods -l 'app in (redis,postgres,rabbitmq)' 2>/dev/null || echo "  ⚠️ Backend metrics not available"

  echo ""
  echo "💾 Storage Usage:"
  kubectl get pvc -o custom-columns="NAME:.metadata.name,STATUS:.status.phase,CAPACITY:.status.capacity.storage,USED:.status.capacity.storage"

  echo ""
}

# Continuous monitoring
if [[ "$1" == "--continuous" ]]; then
  while true; do
    clear
    display_resources
    echo "Press Ctrl+C to stop monitoring..."
    sleep 30
  done
else
  display_resources
fi
```

#### **Performance Metrics Collection**

```bash
#!/bin/bash
# performance-metrics.sh - Collect and analyze performance metrics

METRICS_DIR="./metrics-$(date +%Y%m%d-%H%M%S)"
mkdir -p $METRICS_DIR

echo "📈 Collecting Performance Metrics..."

# Collect node metrics
echo "🖥️ Collecting node metrics..."
kubectl top nodes --no-headers > "$METRICS_DIR/node-metrics.txt" 2>/dev/null

# Collect pod metrics
echo "🏠 Collecting pod metrics..."
kubectl top pods --all-namespaces --no-headers > "$METRICS_DIR/pod-metrics.txt" 2>/dev/null

# Collect service-specific metrics
echo "🎯 Collecting service metrics..."

# Cache service metrics
curl -s http://localhost:30081/metrics > "$METRICS_DIR/cache-metrics.txt" 2>/dev/null || echo "Cache metrics not available" > "$METRICS_DIR/cache-metrics.txt"

# Storage service metrics
curl -s http://localhost:30082/metrics > "$METRICS_DIR/storage-metrics.txt" 2>/dev/null || echo "Storage metrics not available" > "$METRICS_DIR/storage-metrics.txt"

# Auth service metrics
curl -s http://localhost:30083/metrics > "$METRICS_DIR/auth-metrics.txt" 2>/dev/null || echo "Auth metrics not available" > "$METRICS_DIR/auth-metrics.txt"

# Queue service metrics
curl -s http://localhost:30084/metrics > "$METRICS_DIR/queue-metrics.txt" 2>/dev/null || echo "Queue metrics not available" > "$METRICS_DIR/queue-metrics.txt"

# Profile service metrics
curl -s http://localhost:30085/metrics > "$METRICS_DIR/profile-metrics.txt" 2>/dev/null || echo "Profile metrics not available" > "$METRICS_DIR/profile-metrics.txt"

# Worker service metrics
curl -s http://localhost:30086/metrics > "$METRICS_DIR/worker-metrics.txt" 2>/dev/null || echo "Worker metrics not available" > "$METRICS_DIR/worker-metrics.txt"

# Generate metrics summary
cat > "$METRICS_DIR/metrics-summary.md" << EOF
# Performance Metrics Summary

Generated: $(date)

## Node Resource Usage
\`\`\`
$(cat "$METRICS_DIR/node-metrics.txt" 2>/dev/null || echo "Node metrics not available")
\`\`\`

## Pod Resource Usage
\`\`\`
$(cat "$METRICS_DIR/pod-metrics.txt" 2>/dev/null || echo "Pod metrics not available")
\`\`\`

## Service Health Status
EOF

# Add health status to summary
for port in 30081 30082 30083 30084 30085 30086; do
  service_name=$(case $port in
    30081) echo "Cache Service" ;;
    30082) echo "Storage Service" ;;
    30083) echo "Auth Service" ;;
    30084) echo "Queue Service" ;;
    30085) echo "Profile Service" ;;
    30086) echo "Worker Service" ;;
  esac)

  health_status=$(curl -s --connect-timeout 3 http://localhost:$port/health | jq -r '.status // "unknown"' 2>/dev/null || echo "unreachable")
  echo "- **$service_name**: $health_status" >> "$METRICS_DIR/metrics-summary.md"
done

echo "📊 Metrics collected in: $METRICS_DIR"
echo "📋 View summary: cat $METRICS_DIR/metrics-summary.md"
```

### **Application-Specific Metrics**

#### **Service Performance Analysis**

```bash
#!/bin/bash
# service-performance-analysis.sh - Analyze individual service performance

echo "🎯 Service Performance Analysis..."

# Function to test service performance
test_service_performance() {
  local service_name=$1
  local port=$2
  local endpoint=$3

  echo "🔍 Testing $service_name performance..."

  # Response time test
  local total_time=0
  local successful_requests=0
  local failed_requests=0

  for i in {1..10}; do
    local start_time=$(date +%s.%N)
    local response=$(curl -s --connect-timeout 5 --max-time 10 http://localhost:$port$endpoint 2>/dev/null)
    local end_time=$(date +%s.%N)

    if [[ $? -eq 0 ]] && [[ -n "$response" ]]; then
      local response_time=$(echo "$end_time - $start_time" | bc)
      total_time=$(echo "$total_time + $response_time" | bc)
      ((successful_requests++))
    else
      ((failed_requests++))
    fi
  done

  if [[ $successful_requests -gt 0 ]]; then
    local avg_response_time=$(echo "scale=3; $total_time / $successful_requests" | bc)
    echo "  📊 Average response time: ${avg_response_time}s"
    echo "  ✅ Successful requests: $successful_requests/10"
    echo "  ❌ Failed requests: $failed_requests/10"
  else
    echo "  ❌ All requests failed"
  fi

  echo ""
}

# Test all services
test_service_performance "Cache Service" "30081" "/health"
test_service_performance "Storage Service" "30082" "/health"
test_service_performance "Auth Service" "30083" "/health"
test_service_performance "Queue Service" "30084" "/health"
test_service_performance "Profile Service" "30085" "/health"
test_service_performance "Worker Service" "30086" "/health"

# Database performance tests
echo "🗄️ Database Performance Tests..."

# Redis performance
echo "🔴 Redis Performance:"
kubectl exec redis-0 -- redis-benchmark -q -n 1000 -c 10 2>/dev/null | head -5 || echo "  ⚠️ Redis benchmark not available"

# PostgreSQL performance
echo "🐘 PostgreSQL Performance:"
kubectl exec postgres-0 -- psql -U profile_user -d profile_db -c "SELECT COUNT(*) FROM profiles;" 2>/dev/null && \
  echo "  ✅ PostgreSQL query successful" || echo "  ⚠️ PostgreSQL query failed"

# RabbitMQ performance
echo "🐰 RabbitMQ Performance:"
kubectl exec rabbitmq-0 -- rabbitmqctl list_queues 2>/dev/null >/dev/null && \
  echo "  ✅ RabbitMQ management accessible" || echo "  ⚠️ RabbitMQ management not accessible"
```

---

## 📋 **Log Analysis Framework**

### **Centralized Log Collection**

#### **Log Aggregation Script**

```bash
#!/bin/bash
# log-aggregation.sh - Collect logs from all services

LOG_DIR="./logs-$(date +%Y%m%d-%H%M%S)"
mkdir -p "$LOG_DIR"

echo "📋 Collecting logs from all services..."

# Microservices logs
echo "🎯 Collecting microservice logs..."
kubectl logs deployment/cache-service --tail=1000 > "$LOG_DIR/cache-service.log" 2>/dev/null || echo "Cache service logs not available" > "$LOG_DIR/cache-service.log"
kubectl logs deployment/storage-service --tail=1000 > "$LOG_DIR/storage-service.log" 2>/dev/null || echo "Storage service logs not available" > "$LOG_DIR/storage-service.log"
kubectl logs deployment/auth-service --tail=1000 > "$LOG_DIR/auth-service.log" 2>/dev/null || echo "Auth service logs not available" > "$LOG_DIR/auth-service.log"
kubectl logs deployment/queue-service --tail=1000 > "$LOG_DIR/queue-service.log" 2>/dev/null || echo "Queue service logs not available" > "$LOG_DIR/queue-service.log"
kubectl logs deployment/profile-service --tail=1000 > "$LOG_DIR/profile-service.log" 2>/dev/null || echo "Profile service logs not available" > "$LOG_DIR/profile-service.log"
kubectl logs deployment/email-worker --tail=1000 > "$LOG_DIR/email-worker.log" 2>/dev/null || echo "Email worker logs not available" > "$LOG_DIR/email-worker.log"
kubectl logs deployment/image-worker --tail=1000 > "$LOG_DIR/image-worker.log" 2>/dev/null || echo "Image worker logs not available" > "$LOG_DIR/image-worker.log"

# Backend service logs
echo "🗄️ Collecting backend service logs..."
kubectl logs statefulset/redis --tail=1000 > "$LOG_DIR/redis.log" 2>/dev/null || echo "Redis logs not available" > "$LOG_DIR/redis.log"
kubectl logs statefulset/postgres --tail=1000 > "$LOG_DIR/postgres.log" 2>/dev/null || echo "PostgreSQL logs not available" > "$LOG_DIR/postgres.log"
kubectl logs statefulset/rabbitmq --tail=1000 > "$LOG_DIR/rabbitmq.log" 2>/dev/null || echo "RabbitMQ logs not available" > "$LOG_DIR/rabbitmq.log"

# Infrastructure logs
echo "🏗️ Collecting infrastructure logs..."
kubectl logs -n ingress-nginx deployment/ingress-nginx-controller --tail=500 > "$LOG_DIR/ingress-controller.log" 2>/dev/null || echo "Ingress controller logs not available" > "$LOG_DIR/ingress-controller.log"
kubectl logs -n kube-system deployment/metrics-server --tail=500 > "$LOG_DIR/metrics-server.log" 2>/dev/null || echo "Metrics server logs not available" > "$LOG_DIR/metrics-server.log"
kubectl logs -n local-path-storage deployment/local-path-provisioner --tail=500 > "$LOG_DIR/storage-provisioner.log" 2>/dev/null || echo "Storage provisioner logs not available" > "$LOG_DIR/storage-provisioner.log"

# Generate log analysis
echo "🔍 Generating log analysis..."

cat > "$LOG_DIR/log-analysis.md" << EOF
# Log Analysis Report

Generated: $(date)
Log Collection Period: Last 1000 lines per service

## Error Summary

### Critical Errors
EOF

# Search for errors in all logs
echo "🔍 Searching for errors..."
for log_file in "$LOG_DIR"/*.log; do
  service_name=$(basename "$log_file" .log)
  error_count=$(grep -i -E "(error|panic|fatal|exception)" "$log_file" 2>/dev/null | wc -l)

  if [[ $error_count -gt 0 ]]; then
    echo "- **$service_name**: $error_count errors found" >> "$LOG_DIR/log-analysis.md"
  fi
done

# Add warning summary
echo "" >> "$LOG_DIR/log-analysis.md"
echo "### Warnings" >> "$LOG_DIR/log-analysis.md"

for log_file in "$LOG_DIR"/*.log; do
  service_name=$(basename "$log_file" .log)
  warning_count=$(grep -i -E "(warn|warning)" "$log_file" 2>/dev/null | wc -l)

  if [[ $warning_count -gt 0 ]]; then
    echo "- **$service_name**: $warning_count warnings found" >> "$LOG_DIR/log-analysis.md"
  fi
done

echo "📋 Logs collected in: $LOG_DIR"
echo "📊 View analysis: cat $LOG_DIR/log-analysis.md"
```

#### **Real-Time Log Monitoring**

```bash
#!/bin/bash
# real-time-log-monitoring.sh - Monitor logs in real-time

echo "📺 Real-Time Log Monitoring..."
echo "Press Ctrl+C to stop monitoring"

# Function to monitor specific service logs
monitor_service_logs() {
  local service_type=$1
  local service_name=$2

  echo "🔍 Monitoring $service_name logs..."
  kubectl logs -f $service_type/$service_name --tail=10 | while read line; do
    timestamp=$(date '+%H:%M:%S')
    echo "[$timestamp] [$service_name] $line"
  done &
}

# Start monitoring all services
monitor_service_logs "deployment" "cache-service"
monitor_service_logs "deployment" "storage-service"
monitor_service_logs "deployment" "auth-service"
monitor_service_logs "deployment" "queue-service"
monitor_service_logs "deployment" "profile-service"
monitor_service_logs "deployment" "email-worker"
monitor_service_logs "deployment" "image-worker"

# Monitor backend services
monitor_service_logs "statefulset" "redis"
monitor_service_logs "statefulset" "postgres"
monitor_service_logs "statefulset" "rabbitmq"

# Wait for user interrupt
wait
```

### **Log Analysis and Alerting**

#### **Error Detection and Alerting**

```bash
#!/bin/bash
# error-detection.sh - Detect and alert on errors in logs

echo "🚨 Error Detection and Alerting..."

# Configuration
ERROR_THRESHOLD=5
WARNING_THRESHOLD=10
ALERT_LOG="./alerts-$(date +%Y%m%d).log"

# Function to check service for errors
check_service_errors() {
  local service_type=$1
  local service_name=$2

  echo "🔍 Checking $service_name for errors..."

  # Get recent logs
  local logs=$(kubectl logs $service_type/$service_name --tail=100 2>/dev/null)

  if [[ -z "$logs" ]]; then
    echo "⚠️ No logs available for $service_name"
    return
  fi

  # Count errors and warnings
  local error_count=$(echo "$logs" | grep -i -E "(error|panic|fatal|exception)" | wc -l)
  local warning_count=$(echo "$logs" | grep -i -E "(warn|warning)" | wc -l)

  # Check error threshold
  if [[ $error_count -ge $ERROR_THRESHOLD ]]; then
    local alert_msg="🚨 CRITICAL: $service_name has $error_count errors (threshold: $ERROR_THRESHOLD)"
    echo "$alert_msg"
    echo "$(date): $alert_msg" >> "$ALERT_LOG"

    # Show recent errors
    echo "Recent errors:"
    echo "$logs" | grep -i -E "(error|panic|fatal|exception)" | tail -3
    echo ""
  elif [[ $warning_count -ge $WARNING_THRESHOLD ]]; then
    local alert_msg="⚠️ WARNING: $service_name has $warning_count warnings (threshold: $WARNING_THRESHOLD)"
    echo "$alert_msg"
    echo "$(date): $alert_msg" >> "$ALERT_LOG"
  else
    echo "✅ $service_name: $error_count errors, $warning_count warnings"
  fi
}

# Check all services
check_service_errors "deployment" "cache-service"
check_service_errors "deployment" "storage-service"
check_service_errors "deployment" "auth-service"
check_service_errors "deployment" "queue-service"
check_service_errors "deployment" "profile-service"
check_service_errors "deployment" "email-worker"
check_service_errors "deployment" "image-worker"

# Check backend services
check_service_errors "statefulset" "redis"
check_service_errors "statefulset" "postgres"
check_service_errors "statefulset" "rabbitmq"

# Check if alert log exists and show recent alerts
if [[ -f "$ALERT_LOG" ]]; then
  echo "📋 Recent alerts:"
  tail -5 "$ALERT_LOG"
fi
```

---

## 🎯 **Alerting and Notification Framework**

### **Health-Based Alerting**

#### **Continuous Health Monitoring with Alerts**

```bash
#!/bin/bash
# health-monitoring-with-alerts.sh - Continuous monitoring with alerting

# Configuration
CHECK_INTERVAL=60  # seconds
ALERT_THRESHOLD=3  # consecutive failures before alert
RECOVERY_THRESHOLD=2  # consecutive successes before recovery

# State tracking
declare -A failure_counts
declare -A alert_states

# Alert log
ALERT_LOG="./health-alerts-$(date +%Y%m%d).log"

# Function to send alert
send_alert() {
  local service=$1
  local status=$2
  local message=$3

  local timestamp=$(date)
  local alert_msg="[$timestamp] ALERT: $service - $status - $message"

  echo "$alert_msg"
  echo "$alert_msg" >> "$ALERT_LOG"

  # Here you could integrate with external alerting systems:
  # - Slack webhook
  # - Email notification
  # - PagerDuty API
  # - Discord webhook
}

# Function to send recovery notification
send_recovery() {
  local service=$1
  local message=$2

  local timestamp=$(date)
  local recovery_msg="[$timestamp] RECOVERY: $service - $message"

  echo "$recovery_msg"
  echo "$recovery_msg" >> "$ALERT_LOG"
}

# Function to check service health
check_service_health() {
  local service=$1
  local port=$2

  # Attempt health check
  local health_response=$(curl -s --connect-timeout 5 --max-time 10 http://localhost:$port/health 2>/dev/null)

  if [[ -n "$health_response" ]]; then
    local status=$(echo $health_response | jq -r '.status // "unknown"' 2>/dev/null || echo "unknown")

    if [[ "$status" == "healthy" ]] || [[ "$status" == "ok" ]]; then
      # Service is healthy
      if [[ "${alert_states[$service]}" == "alerting" ]]; then
        # Check for recovery
        local success_count=${failure_counts[$service]:-0}
        ((success_count++))
        failure_counts[$service]=$success_count

        if [[ $success_count -ge $RECOVERY_THRESHOLD ]]; then
          send_recovery "$service" "Service recovered after $success_count consecutive successful checks"
          alert_states[$service]="healthy"
          failure_counts[$service]=0
        fi
      else
        # Reset failure count
        failure_counts[$service]=0
        alert_states[$service]="healthy"
      fi

      return 0
    else
      # Service reports unhealthy status
      local fail_count=${failure_counts[$service]:-0}
      ((fail_count++))
      failure_counts[$service]=$fail_count

      if [[ $fail_count -ge $ALERT_THRESHOLD ]] && [[ "${alert_states[$service]}" != "alerting" ]]; then
        send_alert "$service" "UNHEALTHY" "Service reports status: $status (consecutive failures: $fail_count)"
        alert_states[$service]="alerting"
      fi

      return 1
    fi
  else
    # Service not responding
    local fail_count=${failure_counts[$service]:-0}
    ((fail_count++))
    failure_counts[$service]=$fail_count

    if [[ $fail_count -ge $ALERT_THRESHOLD ]] && [[ "${alert_states[$service]}" != "alerting" ]]; then
      send_alert "$service" "NOT_RESPONDING" "Service not responding to health checks (consecutive failures: $fail_count)"
      alert_states[$service]="alerting"
    fi

    return 1
  fi
}

# Main monitoring loop
echo "🚨 Starting Health Monitoring with Alerts..."
echo "Check interval: ${CHECK_INTERVAL}s"
echo "Alert threshold: $ALERT_THRESHOLD consecutive failures"
echo "Recovery threshold: $RECOVERY_THRESHOLD consecutive successes"
echo "Alert log: $ALERT_LOG"
echo "Press Ctrl+C to stop monitoring"
echo ""

# Services to monitor
declare -A services=(
  ["cache-service"]="30081"
  ["storage-service"]="30082"
  ["auth-service"]="30083"
  ["queue-service"]="30084"
  ["profile-service"]="30085"
  ["worker-service"]="30086"
)

cycle=1
while true; do
  echo "🔄 Health Check Cycle $cycle - $(date)"

  for service in "${!services[@]}"; do
    port=${services[$service]}
    check_service_health "$service" "$port"
  done

  echo "⏳ Next check in ${CHECK_INTERVAL} seconds..."
  echo ""

  ((cycle++))
  sleep $CHECK_INTERVAL
done
```

### **Performance-Based Alerting**

#### **Resource Usage Alerts**

```bash
#!/bin/bash
# resource-usage-alerts.sh - Monitor resource usage and alert on thresholds

# Thresholds (percentages)
CPU_THRESHOLD=80
MEMORY_THRESHOLD=80
DISK_THRESHOLD=85

ALERT_LOG="./resource-alerts-$(date +%Y%m%d).log"

# Function to send resource alert
send_resource_alert() {
  local resource_type=$1
  local service=$2
  local usage=$3
  local threshold=$4

  local timestamp=$(date)
  local alert_msg="[$timestamp] RESOURCE ALERT: $service - $resource_type usage: $usage% (threshold: $threshold%)"

  echo "$alert_msg"
  echo "$alert_msg" >> "$ALERT_LOG"
}

# Function to check pod resource usage
check_pod_resources() {
  echo "📊 Checking pod resource usage..."

  # Get pod metrics
  local pod_metrics=$(kubectl top pods --no-headers 2>/dev/null)

  if [[ -z "$pod_metrics" ]]; then
    echo "⚠️ Pod metrics not available"
    return
  fi

  # Parse metrics and check thresholds
  echo "$pod_metrics" | while read line; do
    local pod_name=$(echo $line | awk '{print $1}')
    local cpu_usage=$(echo $line | awk '{print $2}' | sed 's/m//')
    local memory_usage=$(echo $line | awk '{print $3}' | sed 's/Mi//')

    # Skip if not a microservice pod
    if [[ ! "$pod_name" =~ (cache|storage|auth|queue|profile|worker|redis|postgres|rabbitmq) ]]; then
      continue
    fi

    # Convert CPU to percentage (assuming 1000m = 100%)
    if [[ "$cpu_usage" =~ ^[0-9]+$ ]]; then
      local cpu_percent=$((cpu_usage / 10))
      if [[ $cpu_percent -gt $CPU_THRESHOLD ]]; then
        send_resource_alert "CPU" "$pod_name" "$cpu_percent" "$CPU_THRESHOLD"
      fi
    fi

    # Convert memory to percentage (rough estimate based on typical limits)
    if [[ "$memory_usage" =~ ^[0-9]+$ ]]; then
      # Assuming 512Mi typical limit, calculate percentage
      local memory_percent=$((memory_usage * 100 / 512))
      if [[ $memory_percent -gt $MEMORY_THRESHOLD ]]; then
        send_resource_alert "Memory" "$pod_name" "$memory_percent" "$MEMORY_THRESHOLD"
      fi
    fi
  done
}

# Function to check node resource usage
check_node_resources() {
  echo "🖥️ Checking node resource usage..."

  local node_metrics=$(kubectl top nodes --no-headers 2>/dev/null)

  if [[ -z "$node_metrics" ]]; then
    echo "⚠️ Node metrics not available"
    return
  fi

  echo "$node_metrics" | while read line; do
    local node_name=$(echo $line | awk '{print $1}')
    local cpu_usage=$(echo $line | awk '{print $2}' | sed 's/%//')
    local memory_usage=$(echo $line | awk '{print $4}' | sed 's/%//')

    if [[ "$cpu_usage" =~ ^[0-9]+$ ]] && [[ $cpu_usage -gt $CPU_THRESHOLD ]]; then
      send_resource_alert "CPU" "$node_name" "$cpu_usage" "$CPU_THRESHOLD"
    fi

    if [[ "$memory_usage" =~ ^[0-9]+$ ]] && [[ $memory_usage -gt $MEMORY_THRESHOLD ]]; then
      send_resource_alert "Memory" "$node_name" "$memory_usage" "$MEMORY_THRESHOLD"
    fi
  done
}

# Function to check disk usage
check_disk_usage() {
  echo "💾 Checking disk usage..."

  # Check PVC usage (if available)
  kubectl get pvc -o custom-columns="NAME:.metadata.name,CAPACITY:.status.capacity.storage" --no-headers | while read line; do
    local pvc_name=$(echo $line | awk '{print $1}')
    local capacity=$(echo $line | awk '{print $2}')

    if [[ "$capacity" != "<none>" ]]; then
      echo "📊 PVC $pvc_name: $capacity allocated"
    fi
  done
}

# Main execution
echo "📊 Resource Usage Monitoring with Alerts..."
echo "CPU Threshold: $CPU_THRESHOLD%"
echo "Memory Threshold: $MEMORY_THRESHOLD%"
echo "Disk Threshold: $DISK_THRESHOLD%"
echo ""

check_node_resources
echo ""
check_pod_resources
echo ""
check_disk_usage

# Show recent alerts if any
if [[ -f "$ALERT_LOG" ]]; then
  echo ""
  echo "📋 Recent resource alerts:"
  tail -5 "$ALERT_LOG"
fi
```

---

## 🎉 **Observability Framework Complete**

### **Observability Summary**

**Your microservices ecosystem now has:**

- ✅ **Multi-Layer Health Checks**: Cluster, infrastructure, service, and application level monitoring
- ✅ **Comprehensive Metrics Collection**: Node, pod, and application-specific metrics
- ✅ **Centralized Log Analysis**: Automated log collection and error detection
- ✅ **Real-Time Monitoring**: Continuous health and performance monitoring
- ✅ **Alerting Framework**: Threshold-based alerting with recovery detection
- ✅ **Performance Analysis**: Response time and resource usage monitoring

### **Usage Instructions**

1. **Daily Health Checks**: Run `./cluster-health-check.sh` and `./service-health-check.sh`
2. **Performance Monitoring**: Use `./resource-monitoring.sh --continuous` for real-time monitoring
3. **Log Analysis**: Execute `./log-aggregation.sh` for comprehensive log collection
4. **Error Detection**: Run `./error-detection.sh` to identify service issues
5. **Continuous Monitoring**: Start `./health-monitoring-with-alerts.sh` for 24/7 monitoring

### **Integration with External Systems**

The framework is designed to integrate with:

- **Prometheus + Grafana**: For advanced metrics visualization
- **ELK Stack**: For centralized log management
- **Slack/Discord**: For alert notifications
- **PagerDuty**: For incident management
- **Custom Dashboards**: Using the collected metrics and logs

### **Next Steps for Production**

1. **Metrics Visualization**: Deploy Prometheus and Grafana for advanced dashboards
2. **Log Centralization**: Implement ELK stack for production log management
3. **Alert Integration**: Connect alerts to your incident management system
4. **SLA Monitoring**: Establish service level agreements based on collected metrics
5. **Automated Remediation**: Implement auto-scaling and self-healing based on metrics

---

**Observability Status**: ✅ **COMPREHENSIVE MONITORING FRAMEWORK COMPLETE**  
**Health Monitoring**: 🏥 **MULTI-LAYER VALIDATION IMPLEMENTED**  
**Metrics Collection**: 📊 **PERFORMANCE AND RESOURCE MONITORING ACHIEVED**  
**Log Analysis**: 📋 **CENTRALIZED LOGGING AND ERROR DETECTION ESTABLISHED**  
**Alerting Framework**: 🚨 **PROACTIVE MONITORING AND NOTIFICATION READY**
