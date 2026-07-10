# Auth Service Deployment Guide

## Overview

This directory contains comprehensive deployment configurations for the auth-service, following the Enhanced Microservices Deployment Standard. The auth-service provides JWT-based authentication and authorization for the microservices ecosystem.

## 🏗️ Architecture

The auth-service is a **microservices orchestration layer** that integrates with:

- **Storage Service**: User data, authentication records, and audit logs
- **Cache Service**: Session management and JWT token blacklisting
- **Profile Service**: Token validation and user context

```
Client Applications → Auth Service → Storage Service (User Data)
                          ↓
                   Cache Service (Sessions)
```

## 🚀 Deployment Options

### Option 1: Automated Deployment (Recommended)

```bash
# For Kind clusters (local development)
cd kind/
./deploy-to-kind.sh --with-dependencies

# For production clusters
kubectl apply -k kubernetes/
```

### Option 2: Manual Step-by-Step Deployment (Educational)

```bash
cd scripts/
./manual-deploy.sh --step-by-step --analyze
```

## 📁 Directory Structure

```
deployments/
├── README.md                          # This file - deployment overview
├── STEP_BY_STEP_DEPLOYMENT_GUIDE.md  # Comprehensive manual guide
├── kubernetes/                        # Production manifests
│   ├── deployment.yaml               # Auth service deployment
│   ├── service.yaml                  # Service + RBAC + NetworkPolicy
│   ├── configmap.yaml                # Configuration management
│   ├── secrets.yaml                  # JWT keys and sensitive data
│   └── hpa.yaml                      # Horizontal Pod Autoscaler
├── kind/                             # Kind overlays for local development
│   ├── kustomization.yaml            # Kind-specific configuration
│   ├── deployment-patch.yaml         # Resource patches for local
│   ├── service-patch.yaml            # NodePort for local access
│   ├── auth-dependencies.yaml        # Storage/Cache dependencies
│   └── deploy-to-kind.sh             # Automated Kind deployment
├── scripts/                          # Manual deployment scripts
│   ├── manual-deploy.sh              # Interactive step-by-step
│   ├── manual-cleanup.sh             # Step-by-step cleanup
│   └── rollback-procedures.sh        # Emergency rollback
└── monitoring/                       # Monitoring configuration
    └── servicemonitor.yaml           # Prometheus ServiceMonitor
```

## ⚙️ Configuration

### Environment Variables

| Variable                  | Description                         | Default                       | Required |
| ------------------------- | ----------------------------------- | ----------------------------- | -------- |
| `STORAGE_SERVICE_URL`     | Storage service endpoint            | `http://storage-service:8080` | Yes      |
| `CACHE_SERVICE_URL`       | Cache service endpoint              | `http://cache-service:8080`   | Yes      |
| `SERVICE_TIMEOUT`         | Service call timeout (ms)           | `5000`                        | No       |
| `CIRCUIT_BREAKER_TIMEOUT` | Circuit breaker timeout (ms)        | `3000`                        | No       |
| `RATE_LIMIT_MAX_REQUESTS` | Max requests per window             | `5`                           | No       |
| `JWT_PRIVATE_KEY`         | RSA private key for JWT signing     | -                             | Yes      |
| `JWT_PUBLIC_KEY`          | RSA public key for JWT verification | -                             | Yes      |

### Dependencies

The auth-service requires the following services to be deployed:

- **Storage Service**: For user data and audit logging
- **Cache Service**: For session management (optional but recommended)

## 🔐 Security Configuration

### JWT Keys

Generate RSA key pair for production:

```bash
# Generate private key
openssl genrsa -out private.pem 2048

# Extract public key
openssl rsa -in private.pem -pubout -out public.pem

# Update secrets
kubectl create secret generic auth-service-secrets \
  --from-file=JWT_PRIVATE_KEY=private.pem \
  --from-file=JWT_PUBLIC_KEY=public.pem
```

### Network Security

The deployment includes:

- **NetworkPolicy**: Restricts ingress/egress traffic
- **RBAC**: Minimal permissions for service account
- **Security Context**: Non-root user, read-only filesystem

## 📊 Monitoring

### Health Checks

| Endpoint  | Purpose                | Kubernetes Probe |
| --------- | ---------------------- | ---------------- |
| `/health` | Overall service health | Startup          |
| `/ready`  | Readiness for traffic  | Readiness        |
| `/live`   | Service liveness       | Liveness         |

### Metrics

Prometheus metrics available at `/metrics` (port 8081):

- `auth_attempts_total`: Authentication attempts
- `auth_duration_seconds`: Authentication latency
- `auth_service_integration_duration_seconds`: Service call latency
- `auth_circuit_breaker_state`: Circuit breaker status

### Monitoring Setup

```bash
# Deploy ServiceMonitor for Prometheus
kubectl apply -f monitoring/servicemonitor.yaml
```

## 🚀 Quick Start

### Local Development (Kind)

```bash
# 1. Ensure Kind cluster is running
kind create cluster

# 2. Deploy dependencies (if not already deployed)
cd ../storage-service/deployments/kind && ./deploy-to-kind.sh
cd ../cache-service/deployments/kind && ./deploy-to-kind.sh

# 3. Deploy auth-service
cd kind/
./deploy-to-kind.sh

# 4. Test the deployment
curl http://localhost:30080/health
```

### Production Deployment

```bash
# 1. Update secrets with production JWT keys
kubectl apply -f kubernetes/secrets.yaml

# 2. Deploy auth-service
kubectl apply -f kubernetes/

# 3. Verify deployment
kubectl rollout status deployment/auth-service
kubectl get pods -l app=auth-service
```

## 🔍 Troubleshooting

### Common Issues

1. **Readiness Check Failing**

   - Verify storage-service is deployed and accessible
   - Check service URLs in configuration

2. **Authentication Failures**

   - Verify JWT keys are correctly configured
   - Check storage-service integration

3. **High Latency**
   - Monitor circuit breaker metrics
   - Check storage-service and cache-service performance

### Debug Commands

```bash
# Check pod status
kubectl get pods -l app=auth-service

# View logs
kubectl logs -l app=auth-service -f

# Check service endpoints
kubectl get endpoints auth-service

# Test connectivity
kubectl port-forward service/auth-service 8080:8080
curl http://localhost:8080/health
```

## 📝 API Endpoints

### Authentication

- `POST /v1/auth/login` - User authentication
- `POST /v1/auth/token/validate` - Token validation
- `POST /v1/auth/token/refresh` - Token refresh
- `POST /v1/auth/logout` - User logout

### User Management

- `GET /v1/users/me` - Current user profile
- `GET /v1/users/{id}` - User by ID (admin only)

### Health and Monitoring

- `GET /health` - Service health
- `GET /ready` - Readiness check
- `GET /live` - Liveness check
- `GET /metrics` - Prometheus metrics

## 🔄 Rollback Procedures

In case of deployment issues:

```bash
# Quick rollback to previous version
kubectl rollout undo deployment/auth-service

# Or use the rollback script
cd scripts/
./rollback-procedures.sh
```

## 📚 Additional Resources

- [Step-by-Step Deployment Guide](STEP_BY_STEP_DEPLOYMENT_GUIDE.md)
- [Auth Service Implementation](../MICROSERVICES_INTEGRATION_COMPLETE.md)
- [Microservices Deployment Standard](../../MICROSERVICES_DEPLOYMENT_STANDARD.md)

---

**Deployment Standard Compliance**: ✅ **FULLY COMPLIANT**  
**Last Updated**: December 2024  
**Version**: 1.0.0
