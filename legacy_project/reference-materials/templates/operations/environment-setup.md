# Environment Configuration Guide

## Primary Purpose and Main Goals

### Primary Purpose

This guide provides comprehensive instructions for configuring and managing different environments (Development, Staging, and Production) for the Profile Service Microservices, ensuring consistent and secure deployments.

### Main Goals

1. Standardize environment setup
2. Document configuration requirements
3. Ensure environment isolation
4. Maintain security standards
5. Facilitate environment management

## Environment Types

### 1. Development Environment

#### Purpose

- Local development
- Feature testing
- Integration testing
- Performance testing

#### Configuration

```yaml
# config-dev.yaml
environment: development
debug: true
logging:
  level: debug
  format: text

database:
  host: localhost
  port: 5432
  name: profile_dev
  user: dev_user
  ssl: false

cache:
  enabled: true
  type: local
  ttl: 300

queue:
  type: local
  retry: 3
  timeout: 30

monitoring:
  enabled: true
  level: debug
  sampling: 1.0
```

### 2. Staging Environment

#### Purpose

- Integration testing
- Performance testing
- User acceptance testing
- Pre-production validation

#### Configuration

```yaml
# config-staging.yaml
environment: staging
debug: false
logging:
  level: info
  format: json

database:
  host: staging-db.profile-service.com
  port: 5432
  name: profile_staging
  user: staging_user
  ssl: true

cache:
  enabled: true
  type: redis
  ttl: 600

queue:
  type: rabbitmq
  retry: 5
  timeout: 60

monitoring:
  enabled: true
  level: info
  sampling: 0.5
```

### 3. Production Environment

#### Purpose

- Live service
- High availability
- Performance optimization
- Security hardening

#### Configuration

```yaml
# config-prod.yaml
environment: production
debug: false
logging:
  level: warn
  format: json

database:
  host: prod-db.profile-service.com
  port: 5432
  name: profile_prod
  user: prod_user
  ssl: true

cache:
  enabled: true
  type: redis
  ttl: 3600

queue:
  type: rabbitmq
  retry: 10
  timeout: 120

monitoring:
  enabled: true
  level: warn
  sampling: 0.1
```

## Configuration Management

### 1. Environment Variables

```bash
# Development
export ENV=development
export DEBUG=true
export LOG_LEVEL=debug

# Staging
export ENV=staging
export DEBUG=false
export LOG_LEVEL=info

# Production
export ENV=production
export DEBUG=false
export LOG_LEVEL=warn
```

### 2. Secrets Management

#### Development

- Use local environment variables
- Store in .env file (git-ignored)
- Use development credentials

#### Staging/Production

- Use Kubernetes secrets
- Use vault for sensitive data
- Rotate credentials regularly

## Environment Setup

### 1. Development Setup

```bash
# Clone repository
git clone https://github.com/org/profile-service.git
cd profile-service

# Install dependencies
make install

# Setup local environment
make setup-dev

# Start services
make start-dev
```

### 2. Staging Setup

```bash
# Create namespace
kubectl create namespace profile-staging

# Apply configurations
kubectl apply -f k8s/staging/

# Deploy services
helm upgrade --install profile-service ./helm \
  -f helm/values-staging.yaml \
  --namespace profile-staging
```

### 3. Production Setup

```bash
# Create namespace
kubectl create namespace profile-prod

# Apply configurations
kubectl apply -f k8s/production/

# Deploy services
helm upgrade --install profile-service ./helm \
  -f helm/values-prod.yaml \
  --namespace profile-prod
```

## Environment Validation

### 1. Health Checks

```bash
# Development
curl http://localhost:8080/health

# Staging
curl https://staging-api.profile-service.com/health

# Production
curl https://api.profile-service.com/health
```

### 2. Configuration Verification

```bash
# Check environment
kubectl get configmap -n profile-system
```

## Cross-References

- [Kubernetes Tools](../../development/tools/kubernetes/README.md)
- [Helm Guide](../../development/tools/kubernetes/helm.md)
- [Kustomize Guide](../../development/tools/kubernetes/kustomize.md)
- [Tool Comparison](../../development/tools/kubernetes/comparison.md)
- [Production Deployment](production-deployment.md)
- [Architecture Template](architecture-template.md)

## Notes

- Regular environment reviews
- Configuration updates
- Security audits
- Documentation maintenance
