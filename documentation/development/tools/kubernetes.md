# Kubernetes Usage Guide

> *Migrated from legacy_project/reference-materials/development/tools/kubernetes.md*

## Overview

Kubernetes is an open-source container orchestration platform that automates the deployment, scaling, and management of containerized applications. In our architecture, we use Kubernetes to manage our services in production and staging environments.

## Key Features Used

### 1. Deployment Configuration

We use Kubernetes Deployments to manage our service pods:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api-service
  labels:
    app: api-service
spec:
  replicas: 3
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
              memory: "128Mi"
              cpu: "100m"
            limits:
              memory: "256Mi"
              cpu: "200m"
          env:
            - name: DB_HOST
              valueFrom:
                configMapKeyRef:
                  name: api-service-config
                  key: db_host
```

### 2. Service Configuration

For service discovery and load balancing:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: api-service
spec:
  selector:
    app: api-service
  ports:
    - port: 80
      targetPort: 8080
  type: ClusterIP
```

### 3. Ingress Configuration

For external access:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: api-service-ingress
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /
spec:
  rules:
    - host: api.example.com
      http:
        paths:
          - path: /profiles
            pathType: Prefix
            backend:
              service:
                name: api-service
                port:
                  number: 80
```

## Best Practices

1. **Resource Management**

   - Set resource requests and limits
   - Use horizontal pod autoscaling
   - Monitor resource usage
   - Implement proper quotas

2. **Security**

   - Use network policies
   - Implement RBAC
   - Secure secrets
   - Regular security audits

3. **High Availability**

   - Use multiple replicas
   - Implement pod disruption budgets
   - Use anti-affinity rules
   - Configure proper health checks

4. **Monitoring**
   - Use Prometheus for metrics
   - Implement proper logging
   - Set up alerts
   - Monitor cluster health

## Common Issues and Solutions

1. **Pod Scheduling Issues**

   - Problem: Pods not being scheduled
   - Solution: Check resource requests and node capacity

2. **Service Discovery**

   - Problem: Services can't communicate
   - Solution: Verify service selectors and network policies

3. **Resource Exhaustion**
   - Problem: Nodes running out of resources
   - Solution: Implement proper resource limits and autoscaling

## API Service Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api-service
spec:
  replicas: 3
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
              memory: "128Mi"
              cpu: "100m"
            limits:
              memory: "256Mi"
              cpu: "200m"
          livenessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /ready
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 10
```

## Cross-References

- [Helm Guide](kubernetes/helm.md)
- [Kustomize Guide](kubernetes/kustomize.md)
- [Tool Comparison](kubernetes/comparison.md)

## References

- [Kubernetes Official Documentation](https://kubernetes.io/docs/)
- [Kubernetes Best Practices](https://kubernetes.io/docs/concepts/configuration/overview/)
- [Kubernetes Security](https://kubernetes.io/docs/concepts/security/)
