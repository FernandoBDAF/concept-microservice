# Kustomize Usage Guide

> *Migrated from legacy_project/reference-materials/development/tools/kubernetes/kustomize.md*

## Overview

Kustomize is our tool for customizing Kubernetes configurations without modifying the original files. It allows us to manage environment-specific configurations, apply patches, and handle secrets and configmaps in a declarative way.

## Directory Structure

### 1. Base Structure

```
api-service/
├── base/
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── ingress.yaml
│   ├── hpa.yaml
│   └── kustomization.yaml
├── overlays/
│   ├── development/
│   │   ├── kustomization.yaml
│   │   ├── patch-deployment.yaml
│   │   └── patch-ingress.yaml
│   ├── staging/
│   │   ├── kustomization.yaml
│   │   ├── patch-deployment.yaml
│   │   └── patch-ingress.yaml
│   └── production/
│       ├── kustomization.yaml
│       ├── patch-deployment.yaml
│       └── patch-ingress.yaml
└── README.md
```

### 2. Base Configuration

```yaml
# base/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - deployment.yaml
  - service.yaml
  - ingress.yaml
  - hpa.yaml

labels:
  - pairs:
      app.kubernetes.io/part-of: api-service
      app.kubernetes.io/managed-by: kustomize

commonAnnotations:
  description: "API Service Base Configuration"
```

### 3. Base Deployment

```yaml
# base/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api-service
spec:
  replicas: 1
  selector:
    matchLabels:
      app: api-service
  template:
    metadata:
      labels:
        app: api-service
    spec:
      containers:
        - name: api-service
          image: api-service:latest
          ports:
            - containerPort: 8080
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: 500m
              memory: 512Mi
```

## Overlay Configuration

### 1. Development Overlay

```yaml
# overlays/development/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - ../../base

namespace: api-dev

labels:
  - pairs:
      environment: development

patches:
  - path: patch-deployment.yaml
  - path: patch-ingress.yaml

configMapGenerator:
  - name: api-service-config
    literals:
      - ENVIRONMENT=development
      - LOG_LEVEL=debug

secretGenerator:
  - name: api-service-secrets
    literals:
      - DB_PASSWORD=dev-password
```

### 2. Production Overlay

```yaml
# overlays/production/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - ../../base

namespace: api-prod

commonLabels:
  environment: production

patches:
  - path: patch-deployment.yaml
  - path: patch-ingress.yaml

configMapGenerator:
  - name: api-service-config
    literals:
      - ENVIRONMENT=production
      - LOG_LEVEL=info

secretGenerator:
  - name: api-service-secrets
    files:
      - secrets/db-password.txt
```

### 3. Deployment Patches

```yaml
# overlays/development/patch-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api-service
spec:
  replicas: 1
  template:
    spec:
      containers:
        - name: api-service
          resources:
            requests:
              cpu: 50m
              memory: 64Mi
            limits:
              cpu: 200m
              memory: 256Mi
```

```yaml
# overlays/production/patch-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api-service
spec:
  replicas: 3
  template:
    spec:
      containers:
        - name: api-service
          resources:
            requests:
              cpu: 200m
              memory: 256Mi
            limits:
              cpu: 1000m
              memory: 1Gi
```

## Best Practices

1. **Base Configuration**

   - Keep base configurations minimal
   - Use common labels and annotations
   - Document all resources
   - Follow naming conventions

2. **Overlay Management**

   - Use environment-specific overlays
   - Keep patches focused and small
   - Document all customizations
   - Use proper namespacing

3. **Secret Management**

   - Never store secrets in version control
   - Use external secret management
   - Rotate secrets regularly
   - Use proper RBAC

4. **Resource Organization**

   - Group related resources
   - Use consistent naming
   - Document dependencies
   - Maintain clear structure

## Common Issues and Solutions

1. **Patch Issues**

   - Problem: Incorrect patch format
   - Solution: Use `kustomize build` to validate

2. **Secret Issues**

   - Problem: Secret exposure
   - Solution: Use external secret management

3. **Resource Conflicts**
   - Problem: Resource name conflicts
   - Solution: Use proper namespacing and naming

## Usage Examples

```bash
# Build development configuration
kustomize build overlays/development

# Apply development configuration
kustomize build overlays/development | kubectl apply -f -

# Build production configuration
kustomize build overlays/production

# Apply production configuration
kustomize build overlays/production | kubectl apply -f -

# Verify deployment
kubectl get all -n api-prod
```

## Cross-References

- [Helm Guide](helm.md)
- [Tool Comparison](comparison.md)
- [Kubernetes Overview](../kubernetes.md)

## References

- [Kustomize Documentation](https://kustomize.io/)
- [Kustomize Best Practices](https://kubectl.docs.kubernetes.io/guides/config_management/best-practices/)
- [Kustomize Examples](https://github.com/kubernetes-sigs/kustomize/tree/master/examples)
