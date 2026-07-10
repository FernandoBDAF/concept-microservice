# Helm Usage Guide

## Overview

Helm is our primary package manager for Kubernetes, providing a way to package, configure, and deploy applications and services to Kubernetes clusters. This guide covers our Helm implementation, best practices, and common patterns.

## Chart Structure

### 1. Directory Structure

```
profile-service/
├── Chart.yaml          # Chart metadata
├── values.yaml         # Default values
├── values-dev.yaml     # Development environment values
├── values-staging.yaml # Staging environment values
├── values-prod.yaml    # Production environment values
├── templates/          # Kubernetes resource templates
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── ingress.yaml
│   ├── hpa.yaml
│   ├── serviceaccount.yaml
│   ├── _helpers.tpl    # Template helpers
│   └── NOTES.txt       # Installation notes
└── charts/            # Chart dependencies
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

## Template Development

### 1. Basic Template

```yaml
# templates/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Release.Name }}
  labels:
    app: {{ .Release.Name }}
    chart: {{ .Chart.Name }}-{{ .Chart.Version }}
spec:
  replicas: {{ .Values.replicaCount }}
  selector:
    matchLabels:
      app: {{ .Release.Name }}
  template:
    metadata:
      labels:
        app: {{ .Release.Name }}
    spec:
      containers:
        - name: {{ .Chart.Name }}
          image: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
          imagePullPolicy: {{ .Values.image.pullPolicy }}
          ports:
            - containerPort: {{ .Values.service.targetPort }}
          resources:
            {{- toYaml .Values.resources | nindent 12 }}
```

### 2. Template Helpers

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

### 3. Conditional Resources

```yaml
# templates/ingress.yaml
{{- if .Values.ingress.enabled -}}
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: {{ include "profile-service.fullname" . }}
  annotations:
    {{- with .Values.ingress.annotations }}
    {{- toYaml . | nindent 4 }}
    {{- end }}
spec:
  {{- if .Values.ingress.className }}
  className: {{ .Values.ingress.className }}
  {{- end }}
  rules:
    {{- range .Values.ingress.hosts }}
    - host: {{ .host | quote }}
      http:
        paths:
          {{- range .paths }}
          - path: {{ .path }}
            pathType: {{ .pathType }}
            backend:
              service:
                name: {{ include "profile-service.fullname" $ }}
                port:
                  number: {{ $.Values.service.port }}
          {{- end }}
    {{- end }}
{{- end }}
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
    cpu: 400m
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

### 2. Secret Management

```yaml
# templates/secrets.yaml
apiVersion: v1
kind: Secret
metadata:
  name: {{ include "profile-service.fullname" . }}
type: Opaque
data:
  {{- range $key, $value := .Values.secrets }}
  {{ $key }}: {{ $value | b64enc }}
  {{- end }}
```

## Best Practices

1. **Chart Development**

   - Use semantic versioning
   - Document all values
   - Include NOTES.txt
   - Use template helpers
   - Validate templates

2. **Value Management**

   - Use environment-specific values
   - Document all values
   - Use value validation
   - Keep secrets secure

3. **Template Design**

   - Use template helpers
   - Implement conditionals
   - Follow DRY principle
   - Use proper indentation

4. **Security**

   - Never store secrets in values
   - Use proper RBAC
   - Implement network policies
   - Follow security best practices

## Common Issues and Solutions

1. **Template Issues**

   - Problem: Template syntax errors
   - Solution: Use `helm lint` and `helm template`

2. **Value Issues**

   - Problem: Missing or invalid values
   - Solution: Use value validation and defaults

3. **Deployment Issues**
   - Problem: Failed deployments
   - Solution: Use `helm rollback` and proper health checks

## Examples from Our Project

### Service Deployment

```bash
# Install/upgrade service
helm upgrade --install profile-service ./helm \
  -f helm/values-prod.yaml \
  --namespace profile-prod

# Verify deployment
helm list -n profile-prod
helm get values profile-service -n profile-prod
```

### Chart Development

```bash
# Create new chart
helm create profile-service

# Lint chart
helm lint ./helm/profile-service

# Template chart
helm template profile-service ./helm/profile-service

# Package chart
helm package ./helm/profile-service
```

## References

- [Helm Documentation](https://helm.sh/docs/)
- [Helm Best Practices](https://helm.sh/docs/chart_best_practices/)
- [Helm Security](https://helm.sh/docs/security/)
- [Helm Templates](https://helm.sh/docs/chart_template_guide/)
