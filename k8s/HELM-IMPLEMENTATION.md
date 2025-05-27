# Helm Implementation Plan

## Overview

This document outlines the plan to migrate our Kubernetes deployments to Helm charts, providing better configuration management, templating, and deployment control. It is a plan that we will keep just in case we need to implement Helm, once we will be using kustomize.

## Current State

- Multiple Kubernetes manifests across different services
- Environment-specific configurations managed through Kustomize
- Services: profile-api, profile-storage, auth, profile-cache, profile-queue, profile-worker
- Monitoring and debugging tools in place

## Implementation Phases

### Phase 1: Base Chart Setup

#### 1.1 Chart Structure

```yaml
# Chart.yaml
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

#### 1.2 Base Values

```yaml
# values.yaml
global:
  environment: development
  imagePullSecrets: []
  nameOverride: ""
  fullnameOverride: ""

commonLabels:
  app.kubernetes.io/part-of: profile-service
  app.kubernetes.io/managed-by: helm

commonAnnotations:
  description: "Profile Service Microservices"

# Service-specific configurations
profileApi:
  enabled: true
  replicaCount: 2
  image:
    repository: profile-api
    tag: latest
    pullPolicy: IfNotPresent
  resources:
    requests:
      cpu: 200m
      memory: 256Mi
    limits:
      cpu: 500m
      memory: 512Mi

profileStorage:
  enabled: true
  replicaCount: 2
  image:
    repository: profile-storage
    tag: latest
    pullPolicy: IfNotPresent
  resources:
    requests:
      cpu: 200m
      memory: 256Mi
    limits:
      cpu: 500m
      memory: 512Mi

auth:
  enabled: true
  replicaCount: 2
  image:
    repository: auth
    tag: latest
    pullPolicy: IfNotPresent
  resources:
    requests:
      cpu: 200m
      memory: 256Mi
    limits:
      cpu: 500m
      memory: 512Mi
```

### Phase 2: Service Templates

#### 2.1 Common Templates

```yaml
# templates/_helpers.tpl
{{/*
Expand the name of the chart.
*/}}
{{- define "profile-service.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "profile-service.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}
```

#### 2.2 Service Templates

```yaml
# templates/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: { { include "profile-service.fullname" . } }
  labels: { { - include "profile-service.labels" . | nindent 4 } }
spec:
  replicas: { { .Values.replicaCount } }
  selector:
    matchLabels:
      { { - include "profile-service.selectorLabels" . | nindent 6 } }
  template:
    metadata:
      labels: { { - include "profile-service.selectorLabels" . | nindent 8 } }
    spec:
      containers:
        - name: { { .Chart.Name } }
          image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
          imagePullPolicy: { { .Values.image.pullPolicy } }
          ports:
            - name: http
              containerPort: { { .Values.service.port } }
          resources: { { - toYaml .Values.resources | nindent 12 } }
```

### Phase 3: Environment Configuration

#### 3.1 Development Environment

```yaml
# values-dev.yaml
global:
  environment: development

profileApi:
  replicaCount: 1
  resources:
    requests:
      cpu: 100m
      memory: 128Mi
    limits:
      cpu: 200m
      memory: 256Mi

profileStorage:
  replicaCount: 1
  resources:
    requests:
      cpu: 100m
      memory: 128Mi
    limits:
      cpu: 200m
      memory: 256Mi

auth:
  replicaCount: 1
  resources:
    requests:
      cpu: 100m
      memory: 128Mi
    limits:
      cpu: 200m
      memory: 256Mi
```

#### 3.2 Production Environment

```yaml
# values-prod.yaml
global:
  environment: production

profileApi:
  replicaCount: 3
  resources:
    requests:
      cpu: 200m
      memory: 256Mi
    limits:
      cpu: 1000m
      memory: 1Gi

profileStorage:
  replicaCount: 3
  resources:
    requests:
      cpu: 200m
      memory: 256Mi
    limits:
      cpu: 1000m
      memory: 1Gi

auth:
  replicaCount: 3
  resources:
    requests:
      cpu: 200m
      memory: 256Mi
    limits:
      cpu: 1000m
      memory: 1Gi
```

### Phase 4: Testing and Validation

#### 4.1 Testing Steps

1. Template validation
2. Configuration testing
3. Deployment testing
4. Integration testing

#### 4.2 Validation Commands

```bash
# Template validation
helm template profile-service . -f values-dev.yaml

# Dry run
helm upgrade --install profile-service . \
  -f values-dev.yaml \
  --dry-run

# Install/Upgrade
helm upgrade --install profile-service . \
  -f values-dev.yaml \
  --namespace profile-dev \
  --create-namespace

# Rollback
helm rollback profile-service 1 \
  --namespace profile-dev
```

## Implementation Tasks

### High Priority

1. [ ] Create base chart structure
2. [ ] Convert existing manifests to templates
3. [ ] Set up environment-specific values
4. [ ] Implement secret management

### Medium Priority

1. [ ] Add monitoring configuration
2. [ ] Set up network policies
3. [ ] Configure resource management
4. [ ] Implement health checks

### Low Priority

1. [ ] Add documentation
2. [ ] Create deployment scripts
3. [ ] Set up CI/CD integration
4. [ ] Add testing automation

## Migration Strategy

### Step 1: Preparation

1. Create Helm chart structure
2. Set up development environment
3. Test basic deployment
4. Document current state

### Step 2: Service Migration

1. Convert profile-api
2. Convert profile-storage
3. Convert auth service
4. Test service integration

### Step 3: Environment Setup

1. Configure development environment
2. Set up staging environment
3. Configure production environment
4. Test environment-specific features

### Step 4: Validation

1. Test all services
2. Verify configurations
3. Check monitoring
4. Validate security

## Best Practices

### Chart Development

1. Use semantic versioning
2. Document all values
3. Include NOTES.txt
4. Use template helpers
5. Validate templates

### Value Management

1. Use environment-specific values
2. Document all values
3. Use value validation
4. Keep secrets secure

### Template Design

1. Use template helpers
2. Implement conditionals
3. Follow DRY principle
4. Use proper indentation

### Security

1. Never store secrets in values
2. Use proper RBAC
3. Implement network policies
4. Follow security best practices

## Next Steps

### Immediate Actions

1. [ ] Create base chart structure
2. [ ] Set up development environment
3. [ ] Convert first service
4. [ ] Test basic deployment

### Short-term Goals

1. [ ] Complete service migration
2. [ ] Set up all environments
3. [ ] Implement monitoring
4. [ ] Document deployment process

### Long-term Objectives

1. [ ] Implement advanced features
2. [ ] Set up automated testing
3. [ ] Create deployment pipelines
4. [ ] Implement monitoring integration
