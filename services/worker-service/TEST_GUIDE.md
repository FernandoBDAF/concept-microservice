# Test Guide - Multi-Worker Architecture

## Overview

This guide explains how to test the multi-worker architecture with both **local Docker Compose** and **Kubernetes** environments. Tests validate that email and image workers process messages correctly with independent scaling.

---

## 🐳 **Local Testing (Docker Compose)**

### Quick Start

```bash
# Start complete environment
./scripts/dev-start.sh
```

### What It Does

1. **Builds** email-worker and image-worker Docker images
2. **Starts** RabbitMQ with management interface
3. **Sets up** exchanges and queues automatically
4. **Deploys** both workers with health checks
5. **Starts** Prometheus and Grafana monitoring
6. **Publishes** test messages automatically

### Expected Outcomes

#### ✅ **Services Running**

```bash
# Check all services are up
docker-compose ps

# Expected output:
NAME               STATUS
rabbitmq           Up 2 minutes (healthy)
rabbitmq-setup     Exited (0)
email-worker       Up 1 minute (healthy)
image-worker       Up 1 minute (healthy)
prometheus         Up 1 minute
grafana            Up 1 minute
test-publisher     Up 30 seconds
```

#### ✅ **Health Checks Pass**

```bash
# Email Worker Health
curl http://localhost:8081/health
# Expected: {"status":"ok","ready":true}

# Image Worker Health
curl http://localhost:8082/health
# Expected: {"status":"ok","ready":true}
```

#### ✅ **Message Processing**

```bash
# Watch worker logs
docker-compose logs -f email-worker image-worker

# Expected logs:
email-worker   | 📧 Sending WELCOME email to user@example.com (Priority: high)
email-worker   | ✅ Welcome email sent successfully to user@example.com
image-worker   | 🖼️ Processing RESIZE for image https://example.com/test.jpg (Priority: normal)
image-worker   | 🐍 Calling Python container: image-resize-service:latest
image-worker   | ✅ Python container image-resize-service:latest completed successfully
```

#### ✅ **RabbitMQ Queues**

- **URL**: http://localhost:15672 (guest/guest)
- **Expected**: `email-processing` and `image-processing` queues with messages processed

#### ✅ **Monitoring**

- **Prometheus**: http://localhost:9090 (metrics collected)
- **Grafana**: http://localhost:3000 (admin/admin, dashboards available)

### Manual Testing

```bash
# Publish additional test messages
./infrastructure/rabbitmq/example-publishers/publish-test-messages.sh

# Expected output:
📧 Publishing EMAIL WORKER test messages...
   Publishing welcome email message...
      ✅ Message published successfully
   Publishing notification email message...
      ✅ Message published successfully
   Publishing alert email message...
      ✅ Message published successfully

🖼️ Publishing IMAGE WORKER test messages...
   Publishing image resize message...
      ✅ Message published successfully
   Publishing image filter message...
      ✅ Message published successfully
   Publishing image analyze message...
      ✅ Message published successfully
```

### Cleanup

```bash
# Stop all services
docker-compose down

# Remove volumes (optional)
docker-compose down -v
```

---

## ☸️ **Kubernetes Testing (kind cluster)**

### Prerequisites

```bash
# Ensure kind cluster is running
kind create cluster

# Verify cluster access
kubectl cluster-info
```

### Quick Start

```bash
# Deploy and test complete architecture
./scripts/k8s-deploy.sh
```

### What It Does

1. **Builds** worker Docker images
2. **Loads** images into kind cluster
3. **Deploys** RabbitMQ with namespaces and config
4. **Sets up** exchanges and queues via API calls
5. **Deploys** email and image workers with scaling config
6. **Tests** health endpoints via port-forwarding
7. **Publishes** test messages
8. **Validates** deployment status

### Expected Outcomes

#### ✅ **Namespaces Created**

```bash
kubectl get namespaces

# Expected:
NAME      STATUS   AGE
rabbitmq  Active   2m
workers   Active   2m
```

#### ✅ **RabbitMQ Deployment**

```bash
kubectl get pods -n rabbitmq

# Expected:
NAME                        READY   STATUS    RESTARTS   AGE
rabbitmq-7c8b9d4f6b-abc123  1/1     Running   0          2m
```

#### ✅ **Worker Deployments**

```bash
kubectl get pods -n workers

# Expected:
NAME                            READY   STATUS    RESTARTS   AGE
email-worker-6d7f8b9c5a-def456  1/1     Running   0          1m
email-worker-6d7f8b9c5a-ghi789  1/1     Running   0          1m
image-worker-5c6d7e8f9b-jkl012  1/1     Running   0          1m
```

#### ✅ **Health Checks Pass**

```bash
# Script automatically tests health endpoints
# Expected output during deployment:
✅ Email worker health check passed
✅ Image worker health check passed
```

#### ✅ **Services Running**

```bash
kubectl get services -n workers

# Expected:
NAME                   TYPE        CLUSTER-IP       EXTERNAL-IP   PORT(S)    AGE
email-worker-service   ClusterIP   10.96.123.45     <none>        8080/TCP   1m
image-worker-service   ClusterIP   10.96.234.56     <none>        8080/TCP   1m
```

#### ✅ **Message Processing**

```bash
# Check worker logs
kubectl logs -f deployment/email-worker -n workers
kubectl logs -f deployment/image-worker -n workers

# Expected logs:
📧 Sending WELCOME email to test@example.com (Priority: normal)
✅ Welcome email sent successfully to test@example.com
🖼️ Processing RESIZE for image https://example.com/test.jpg (Priority: normal)
✅ Python container image-resize-service:latest completed successfully
```

### Manual Kubernetes Testing

#### Access RabbitMQ Management

```bash
kubectl port-forward -n rabbitmq service/rabbitmq 15672:15672
# Access: http://localhost:15672 (guest/guest)
```

#### Check Worker Metrics

```bash
kubectl port-forward -n workers service/email-worker-service 8080:8080
# Access: http://localhost:8080/metrics
```

#### View HPA Status

```bash
kubectl get hpa -n workers

# Expected:
NAME                REFERENCE               TARGETS         MINPODS   MAXPODS   REPLICAS   AGE
email-worker-hpa    Deployment/email-worker   <unknown>/70%   3         15        2          2m
image-worker-hpa    Deployment/image-worker   <unknown>/60%   1         8         1          2m
```

### Cleanup

```bash
# Remove all deployments
kubectl delete -f services/workers/email-worker/k8s/
kubectl delete -f services/workers/image-worker/k8s/
kubectl delete -f infrastructure/rabbitmq/k8s-rabbitmq-config.yaml

# Or delete cluster entirely
kind delete cluster
```

---

## 🧪 **Individual Component Testing**

### Test RabbitMQ Setup Only

```bash
# Local
docker-compose up -d rabbitmq rabbitmq-setup
./infrastructure/rabbitmq/rabbitmq-setup.sh

# Kubernetes
kubectl apply -f infrastructure/rabbitmq/k8s-rabbitmq-config.yaml
```

**Expected**: Exchanges `email-tasks` and `image-tasks` created with respective queues.

### Test Single Worker

```bash
# Email worker only (local)
docker-compose up -d rabbitmq rabbitmq-setup email-worker

# Image worker only (local)
docker-compose up -d rabbitmq rabbitmq-setup image-worker
```

**Expected**: Single worker starts and waits for messages.

### Test Message Publishing Only

```bash
# Ensure RabbitMQ is running first
./infrastructure/rabbitmq/example-publishers/publish-test-messages.sh
```

**Expected**: 6 messages published (3 email + 3 image types).

---

## 📊 **Performance and Scaling Tests**

### Load Testing

```bash
# Generate multiple messages
for i in {1..10}; do
  ./infrastructure/rabbitmq/example-publishers/publish-test-messages.sh
  sleep 1
done
```

**Expected**: Workers scale up automatically, process messages in parallel.

### Scaling Verification

```bash
# Monitor scaling (Kubernetes)
watch kubectl get pods -n workers

# Expected: Pod count increases under load, decreases when idle
```

### Resource Monitoring

```bash
# Check resource usage
kubectl top pods -n workers

# Expected:
# - Email workers: Low CPU/memory usage
# - Image workers: Higher CPU/memory usage during processing
```

---

## 🚨 **Troubleshooting**

### Common Issues

#### ❌ **Workers Not Starting**

```bash
# Check logs
docker-compose logs email-worker image-worker
kubectl logs deployment/email-worker -n workers

# Common causes:
# - RabbitMQ not ready
# - Go module dependencies not resolved
# - Environment variables missing
```

#### ❌ **Messages Not Processing**

```bash
# Check RabbitMQ queues
# Local: http://localhost:15672
# K8s: kubectl port-forward -n rabbitmq service/rabbitmq 15672:15672

# Verify:
# - Queues exist and have messages
# - Workers are connected as consumers
# - No error messages in worker logs
```

#### ❌ **Health Checks Failing**

```bash
# Test manually
curl http://localhost:8081/health  # Local
kubectl port-forward -n workers service/email-worker-service 8080:8080  # K8s

# Expected response:
# {"status":"ok","ready":true}
```

#### ❌ **Scaling Not Working**

```bash
# Check HPA status
kubectl describe hpa email-worker-hpa -n workers

# Common issues:
# - Metrics server not running
# - Resource requests not defined
# - KEDA not installed (for queue-based scaling)
```

---

## ✅ **Success Criteria Checklist**

### Functional Tests

- [ ] Both workers start and report healthy
- [ ] RabbitMQ exchanges and queues created correctly
- [ ] Email messages processed with mock email sending logs
- [ ] Image messages processed with mock Python container calls
- [ ] Different message types (welcome, notification, alert, resize, filter, analyze) handled
- [ ] Priority-based processing delays work correctly

### Performance Tests

- [ ] Email worker scales aggressively (2-15 replicas)
- [ ] Image worker scales conservatively (1-8 replicas)
- [ ] Messages processed without loss
- [ ] Health checks respond correctly under load
- [ ] Graceful shutdown works without message loss

### Monitoring Tests

- [ ] Prometheus scrapes metrics from both workers
- [ ] Grafana displays worker dashboards
- [ ] RabbitMQ management shows queue status
- [ ] Worker logs show processing details
- [ ] Alerts trigger on simulated failures

---

## 🎯 **Quick Test Commands**

```bash
# Complete local test
./scripts/dev-start.sh && docker-compose logs -f email-worker image-worker

# Complete Kubernetes test
./scripts/k8s-deploy.sh && kubectl logs -f deployment/email-worker -n workers

# Health check all services (local)
curl -s http://localhost:8081/health && curl -s http://localhost:8082/health

# Publish test messages
./infrastructure/rabbitmq/example-publishers/publish-test-messages.sh

# Monitor scaling (K8s)
watch kubectl get pods -n workers
```

---

**✅ All tests passing = Multi-worker architecture is ready for production deployment!**
