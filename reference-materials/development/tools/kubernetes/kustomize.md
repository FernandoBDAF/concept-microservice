# Kustomize Usage Guide

## Overview

Kustomize is our tool for customizing Kubernetes configurations without modifying the original files. It allows us to manage environment-specific configurations, apply patches, and handle secrets and configmaps in a declarative way.

## Directory Structure

### 1. Base Structure

```
profile-service/
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

commonLabels:
  app: profile-service
  environment: base

commonAnnotations:
  description: "Profile Service Base Configuration"
```

### 3. Base Deployment

```yaml
# base/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: profile-service
spec:
  replicas: 1
  selector:
    matchLabels:
      app: profile-service
  template:
    metadata:
      labels:
        app: profile-service
    spec:
      containers:
        - name: profile-service
          image: profile-service:latest
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

namespace: profile-dev

commonLabels:
  environment: development

patches:
  - path: patch-deployment.yaml
  - path: patch-ingress.yaml

configMapGenerator:
  - name: profile-service-config
    literals:
      - ENVIRONMENT=development
      - LOG_LEVEL=debug

secretGenerator:
  - name: profile-service-secrets
    literals:
      - DB_PASSWORD=dev-password
```

### 2. Development Patches

```yaml
# overlays/development/patch-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: profile-service
spec:
  replicas: 1
  template:
    spec:
      containers:
        - name: profile-service
          resources:
            requests:
              cpu: 50m
              memory: 64Mi
            limits:
              cpu: 200m
              memory: 256Mi
```

### 3. Production Overlay

```yaml
# overlays/production/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - ../../base

namespace: profile-prod

commonLabels:
  environment: production

patches:
  - path: patch-deployment.yaml
  - path: patch-ingress.yaml

configMapGenerator:
  - name: profile-service-config
    literals:
      - ENVIRONMENT=production
      - LOG_LEVEL=info

secretGenerator:
  - name: profile-service-secrets
    files:
      - secrets/db-password.txt
```

## Customization Techniques

### 1. Resource Patching

```yaml
# patch-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: profile-service
spec:
  template:
    spec:
      containers:
        - name: profile-service
          env:
            - name: CUSTOM_VAR
              value: "custom-value"
```

### 2. ConfigMap Generation

```yaml
# kustomization.yaml
configMapGenerator:
  - name: profile-service-config
    literals:
      - KEY1=value1
      - KEY2=value2
    files:
      - config.properties
```

### 3. Secret Management

```yaml
# kustomization.yaml
secretGenerator:
  - name: profile-service-secrets
    literals:
      - DB_PASSWORD=password
    files:
      - secrets/db-password.txt
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

## Examples from Our Project

### Development Deployment

```bash
# Build development configuration
kustomize build overlays/development

# Apply development configuration
kustomize build overlays/development | kubectl apply -f -

# Verify deployment
kubectl get all -n profile-dev
```

### Production Deployment

```bash
# Build production configuration
kustomize build overlays/production

# Apply production configuration
kustomize build overlays/production | kubectl apply -f -

# Verify deployment
kubectl get all -n profile-prod
```

## References

- [Kustomize Documentation](https://kustomize.io/)
- [Kustomize Best Practices](https://kubectl.docs.kubernetes.io/guides/config_management/best-practices/)
- [Kustomize Examples](https://github.com/kubernetes-sigs/kustomize/tree/master/examples)
- [Kustomize Security](https://kubectl.docs.kubernetes.io/guides/config_management/security/)
