# Helm Charts Configuration Guide

## Primary Purpose and Main Goals

### Primary Purpose

This guide provides comprehensive instructions for managing Profile Service Microservices deployments using Helm charts, ensuring consistent and maintainable deployments across environments.

### Main Goals

1. Standardize Helm chart usage
2. Document chart structure
3. Manage configuration values
4. Control release process
5. Maintain version control

## Chart Structure

### 1. Directory Structure

```
profile-service/
├── Chart.yaml
├── values.yaml
├── values-dev.yaml
├── values-staging.yaml
├── values-prod.yaml
├── templates/
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── ingress.yaml
│   ├── hpa.yaml
│   ├── serviceaccount.yaml
│   ├── _helpers.tpl
│   └── NOTES.txt
└── charts/
```

### 2. Chart.yaml

```yaml
apiVersion: v2
name: profile-service
description: Profile Service Microservices Helm Chart
type: application
version: 0.1.0
appVersion: "1.0.0"
dependencies:
  - name: common
    version: 1.x.x
    repository: https://charts.bitnami.com/bitnami
```

### 3. Values Structure

```yaml
# values.yaml
replicaCount: 3

image:
  repository: profile-service
  tag: latest
  pullPolicy: IfNotPresent

service:
  type: ClusterIP
  port: 80
  targetPort: 8080

ingress:
  enabled: true
  className: nginx
  hosts:
    - host: api.profile-service.com
      paths:
        - path: /
          pathType: Prefix

resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 500m
    memory: 512Mi

autoscaling:
  enabled: true
  minReplicas: 3
  maxReplicas: 10
  targetCPUUtilizationPercentage: 70
```

## Value Management

### 1. Environment-Specific Values

#### Development (values-dev.yaml)

```yaml
replicaCount: 1
resources:
  requests:
    cpu: 50m
    memory: 64Mi
  limits:
    cpu: 200m
    memory: 256Mi
```

#### Staging (values-staging.yaml)

```yaml
replicaCount: 2
resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 500m
    memory: 512Mi
```

#### Production (values-prod.yaml)

```yaml
replicaCount: 3
resources:
  requests:
    cpu: 200m
    memory: 256Mi
  limits:
    cpu: 1000m
    memory: 1Gi
```

### 2. Value Overrides

```bash
# Override specific values
helm upgrade profile-service . \
  --set replicaCount=5 \
  --set resources.limits.cpu=1000m

# Use environment-specific values
helm upgrade profile-service . \
  -f values-prod.yaml

# Combine multiple value files
helm upgrade profile-service . \
  -f values.yaml \
  -f values-prod.yaml \
  -f custom-values.yaml
```

## Release Process

### 1. Release Steps

1. Update Chart.yaml version
2. Update values.yaml
3. Test with helm template
4. Create release
5. Verify deployment

### 2. Release Commands

```bash
# Template validation
helm template profile-service . -f values-prod.yaml

# Dry run
helm upgrade --install profile-service . \
  -f values-prod.yaml \
  --dry-run

# Install/Upgrade
helm upgrade --install profile-service . \
  -f values-prod.yaml \
  --namespace profile-system \
  --create-namespace

# Rollback
helm rollback profile-service 1 \
  --namespace profile-system
```

## Version Control

### 1. Chart Versioning

- Follow semantic versioning
- Update appVersion with service version
- Document changes in Chart.yaml
- Tag releases in repository

### 2. Value File Management

- Keep environment-specific values
- Document value changes
- Review value updates
- Maintain value history

## Best Practices

### 1. Chart Development

- Use templates effectively
- Implement helpers
- Follow naming conventions
- Document dependencies

### 2. Value Management

- Use environment-specific values
- Document value purposes
- Validate value types
- Set reasonable defaults

### 3. Release Management

- Test before release
- Use dry-run
- Implement rollback
- Monitor deployments

## Troubleshooting

### 1. Common Issues

- Template errors
- Value conflicts
- Dependency issues
- Release failures

### 2. Debug Commands

```bash
# Check chart dependencies
helm dependency list

# Validate chart
helm lint .

# Debug template
helm template profile-service . --debug

# Check release history
helm history profile-service
```

## Cross-References

- [Architecture Template](architecture-template.md)
- [Testing Template](testing-template.md)
- [API Documentation](api-documentation.md)

## Notes

- Keep charts up to date
- Document all changes
- Regular chart reviews
- Maintain cross-references
