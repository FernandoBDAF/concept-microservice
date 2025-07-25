# Storage Service Deployment Guide

## Overview

The Storage Service supports **dual deployment approaches** for maximum flexibility:

1. **Manual Step-by-Step Deployment** - Interactive deployment with validation at each step
2. **Automated Kustomize Deployment** - One-command deployment using Kustomize overlays

## Deployment Options

### 🔧 Manual Deployment (Recommended for Learning)

**Use Case**: When you want to understand each step and validate deployment components individually.

**Command**:

```bash
cd deployments/scripts
./manual-deploy.sh
```

**Benefits**:

- Interactive step-by-step process
- Validation at each stage
- Educational for understanding Kubernetes concepts
- Easy troubleshooting with guided prompts

### ⚡ Automated Deployment (Recommended for Production)

**Use Case**: When you want fast, consistent deployments.

**Commands**:

```bash
# For Kind (Development)
cd deployments/kind
./deploy-to-kind.sh

# For Production Kubernetes
kubectl apply -k deployments/kubernetes/
```

**Benefits**:

- Fast one-command deployment
- Consistent configuration management
- Environment-specific customizations
- GitOps-ready

## Prerequisites

### Required Tools

- `kubectl` (v1.20+)
- `kustomize` (v4.0+) or `kubectl` with kustomize support
- For Kind deployments: `kind` (v0.11+)

### Required Infrastructure

- **Database**: PostgreSQL 13+ (for persistent storage)
- **Message Queue**: RabbitMQ 3.8+ (for async processing)
- **Monitoring**: Prometheus (optional, for metrics)

### Required Permissions

- `cluster-admin` role or equivalent permissions for:
  - Creating deployments, services, configmaps, secrets
  - Managing RBAC (if enabled)
  - Creating HPA (if auto-scaling enabled)

## Environment-Specific Configuration

### Development (Kind)

- **Resources**: Reduced memory/CPU limits
- **Replicas**: 1-2 pods
- **Storage**: EmptyDir or local persistent volumes
- **Dependencies**: In-cluster PostgreSQL and RabbitMQ

### Staging/Production

- **Resources**: Full production resource allocation
- **Replicas**: 3+ pods with auto-scaling
- **Storage**: Persistent volumes with backups
- **Dependencies**: External managed services (RDS, Amazon MQ, etc.)

## Service Dependencies

The Storage Service requires the following components to be deployed and accessible:

### Core Dependencies (Required)

1. **PostgreSQL Database**

   - Version: 13+
   - Extensions: uuid-ossp
   - Database: `profiles`
   - User: `profile_user` with appropriate permissions

2. **RabbitMQ Message Queue**
   - Version: 3.8+
   - Exchange: `tasks-exchange`
   - Queue: `storage-processing`
   - User: Admin user with queue management permissions

### Optional Dependencies

1. **Prometheus** (for metrics collection)
2. **Grafana** (for dashboards)
3. **Auth Service** (for authentication integration)
4. **Cache Service** (for performance optimization)

## Quick Start

### For Kind (Development)

```bash
# 1. Create Kind cluster (if not exists)
kind create cluster --name microservices

# 2. Deploy Storage Service
cd deployments/kind
./deploy-to-kind.sh

# 3. Verify deployment
kubectl get pods -l app=storage-service
kubectl port-forward svc/storage-service 8080:8080
curl http://localhost:8080/health
```

### For Production Kubernetes

```bash
# 1. Review and customize configuration
cp deployments/kubernetes/secrets.yaml.template deployments/kubernetes/secrets.yaml
# Edit secrets.yaml with your actual credentials

# 2. Deploy
kubectl apply -k deployments/kubernetes/

# 3. Verify deployment
kubectl get pods -l app=storage-service
kubectl get service storage-service
```

## Configuration Management

### Secrets Management

- **Development**: Use provided templates with default values
- **Production**: Use external secret management (Vault, AWS Secrets Manager, etc.)

### Environment Variables

Key configuration options:

- `DATABASE_URL`: PostgreSQL connection string
- `RABBITMQ_URL`: RabbitMQ connection string
- `QUEUE_ENABLED`: Enable/disable async processing
- `METRICS_ENABLED`: Enable/disable Prometheus metrics

### Resource Configuration

Customize resource limits in:

- `deployments/kubernetes/deployment.yaml` (production)
- `deployments/kind/deployment-patch.yaml` (development)

## Monitoring and Health Checks

### Health Endpoints

- `/health`: Overall service health
- `/health/live`: Liveness probe endpoint
- `/health/ready`: Readiness probe endpoint
- `/metrics`: Prometheus metrics

### Monitoring Setup

1. **Prometheus**: Use provided ServiceMonitor (`deployments/monitoring/servicemonitor.yaml`)
2. **Grafana**: Import dashboards from monitoring directory
3. **Alerting**: Configure alerts based on service metrics

## Troubleshooting

### Common Issues

1. **Database Connection**: Check DATABASE_URL and network connectivity
2. **Queue Connection**: Verify RabbitMQ is accessible and credentials are correct
3. **Image Pull Errors**: Ensure image exists and registry is accessible
4. **Resource Constraints**: Check cluster resources and adjust limits

### Debugging Commands

```bash
# Check pod status
kubectl get pods -l app=storage-service

# View logs
kubectl logs -l app=storage-service

# Describe resources
kubectl describe deployment storage-service

# Port forward for testing
kubectl port-forward deployment/storage-service 8080:8080
```

### Recovery Procedures

See `deployments/scripts/rollback-procedures.sh` for automated recovery options.

## Additional Resources

- **Step-by-Step Guide**: See `STEP_BY_STEP_DEPLOYMENT_GUIDE.md`
- **Manual Scripts**: Available in `deployments/scripts/`
- **Kustomize Overlays**: Available in `deployments/kind/`
- **Monitoring Setup**: Available in `deployments/monitoring/`

## Support

For deployment issues:

1. Check the troubleshooting section above
2. Review logs using the debugging commands
3. Consult the step-by-step deployment guide for detailed procedures
4. Use the manual deployment scripts for interactive troubleshooting

---

**Note**: This deployment setup follows the Microservices Deployment Standard and provides both manual and automated approaches for maximum flexibility and operational excellence.
