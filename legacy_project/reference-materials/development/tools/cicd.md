# CI/CD Tools Guide

## Overview

This guide covers the Continuous Integration and Continuous Deployment (CI/CD) tools and practices used in our microservices architecture. We use GitHub Actions for CI/CD pipelines, along with additional tools for code quality, security scanning, and deployment automation.

## Core CI/CD Tools

### 1. GitHub Actions

Our primary CI/CD platform:

```yaml
# .github/workflows/ci.yml
name: CI

on:
  push:
    branches: [main, develop]
  pull_request:
    branches: [main, develop]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: "1.21"

      - name: Run tests
        run: |
          go test -v -race -coverprofile=coverage.txt ./...
          go tool cover -func=coverage.txt

      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          file: ./coverage.txt

  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Run golangci-lint
        uses: golangci/golangci-lint-action@v3
        with:
          version: latest

  security:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Run gosec
        uses: securego/gosec@master
        with:
          args: ./...

      - name: Run dependency check
        uses: snyk/actions/golang@master
        env:
          SNYK_TOKEN: ${{ secrets.SNYK_TOKEN }}
```

### 2. Docker Build and Push

```yaml
# .github/workflows/docker.yml
name: Docker

on:
  push:
    branches: [main]
    tags: ["v*"]

jobs:
  build-and-push:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v2

      - name: Login to DockerHub
        uses: docker/login-action@v2
        with:
          username: ${{ secrets.DOCKERHUB_USERNAME }}
          password: ${{ secrets.DOCKERHUB_TOKEN }}

      - name: Build and push
        uses: docker/build-push-action@v4
        with:
          context: .
          push: true
          tags: |
            user/profile-service:latest
            user/profile-service:${{ github.sha }}
```

### 3. Kubernetes Deployment

```yaml
# .github/workflows/deploy.yml
name: Deploy

on:
  push:
    branches: [main]
    tags: ["v*"]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Install kubectl
        uses: azure/setup-kubectl@v3

      - name: Set up kubeconfig
        uses: azure/k8s-set-context@v3
        with:
          kubeconfig: ${{ secrets.KUBE_CONFIG }}

      - name: Deploy to Kubernetes
        run: |
          kubectl set image deployment/profile-service \
            profile-service=user/profile-service:${{ github.sha }}
          kubectl rollout status deployment/profile-service
```

## Code Quality Tools

### 1. golangci-lint

Configuration for code linting:

```yaml
# .golangci.yml
linters:
  enable:
    - gofmt
    - golint
    - govet
    - errcheck
    - staticcheck
    - gosimple
    - ineffassign
    - unconvert
    - gosec

issues:
  exclude-rules:
    - path: _test\.go
      linters:
        - errcheck

run:
  deadline: 5m
  tests: true
  skip-dirs:
    - vendor
```

### 2. SonarQube

Configuration for code quality analysis:

```yaml
# sonar-project.properties
sonar.projectKey=profile-service
sonar.sources=.
sonar.tests=.
sonar.exclusions=**/*_test.go
sonar.test.inclusions=**/*_test.go
sonar.go.coverage.reportPaths=coverage.txt
sonar.go.tests.reportPaths=test-report.json
```

## Security Scanning

### 1. Container Scanning

```yaml
# .github/workflows/security.yml
name: Security

on:
  push:
    branches: [main]
    tags: ["v*"]

jobs:
  container-scan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Run Trivy vulnerability scanner
        uses: aquasecurity/trivy-action@master
        with:
          image-ref: user/profile-service:latest
          format: "table"
          exit-code: "1"
          ignore-unfixed: true
          vuln-type: "os,library"
          severity: "CRITICAL,HIGH"
```

### 2. Dependency Scanning

```yaml
# .github/workflows/dependency-scan.yml
name: Dependency Scan

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  scan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Run Snyk to check for vulnerabilities
        uses: snyk/actions/golang@master
        env:
          SNYK_TOKEN: ${{ secrets.SNYK_TOKEN }}
```

## Deployment Strategies

### 1. Blue-Green Deployment

```yaml
# kubernetes/blue-green.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: profile-service-blue
spec:
  replicas: 3
  selector:
    matchLabels:
      app: profile-service
      version: blue
  template:
    metadata:
      labels:
        app: profile-service
        version: blue
    spec:
      containers:
        - name: profile-service
          image: user/profile-service:latest
---
apiVersion: v1
kind: Service
metadata:
  name: profile-service
spec:
  selector:
    app: profile-service
    version: blue
  ports:
    - port: 80
      targetPort: 8080
```

### 2. Canary Deployment

```yaml
# kubernetes/canary.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: profile-service-canary
spec:
  replicas: 1
  selector:
    matchLabels:
      app: profile-service
      version: canary
  template:
    metadata:
      labels:
        app: profile-service
        version: canary
    spec:
      containers:
        - name: profile-service
          image: user/profile-service:latest
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: profile-service
  annotations:
    nginx.ingress.kubernetes.io/canary: "true"
    nginx.ingress.kubernetes.io/canary-weight: "10"
spec:
  rules:
    - host: profile.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: profile-service
                port:
                  number: 80
```

## Best Practices

1. **Pipeline Organization**

   - Separate CI and CD pipelines
   - Use reusable workflows
   - Implement proper testing stages
   - Include security scanning

2. **Deployment Strategy**

   - Use blue-green or canary deployments
   - Implement rollback procedures
   - Monitor deployment health
   - Use feature flags

3. **Security**

   - Scan dependencies regularly
   - Check container vulnerabilities
   - Implement secret management
   - Use least privilege principle

4. **Monitoring**
   - Track pipeline metrics
   - Monitor deployment success
   - Alert on failures
   - Maintain deployment history

## Common Issues and Solutions

1. **Pipeline Failures**

   - Problem: Frequent pipeline failures
   - Solution: Implement proper testing, use retry mechanisms

2. **Deployment Issues**

   - Problem: Failed deployments
   - Solution: Use canary deployments, implement health checks

3. **Security Vulnerabilities**
   - Problem: Security issues in dependencies
   - Solution: Regular scanning, automated updates

## References

- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [Kubernetes Deployment Strategies](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/)
- [Docker Security Best Practices](https://docs.docker.com/engine/security/)
- [SonarQube Documentation](https://docs.sonarqube.org/)
