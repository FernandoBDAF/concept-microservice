# Deployment Documentation

Kubernetes deployment and operations guides for the Profile Service.

## Contents

- [Kubernetes Setup](kubernetes.md) - Cluster setup and configuration
- [Configuration](configuration.md) - Environment variables and secrets
- [Monitoring](monitoring.md) - Prometheus, Grafana setup
- [Scaling](scaling.md) - HPA and resource management

## Quick Deploy

```bash
# Build and push image
cd api-service
make docker-build
make docker-push

# Deploy to Kubernetes
kubectl apply -f deployments/kubernetes/
```

## Kubernetes Resources

```
api-service/deployments/kubernetes/
├── deployment.yaml          # Service deployment
├── service.yaml             # ClusterIP service
└── configmap.yaml           # Configuration
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `API_POSTGRES_DSN` | PostgreSQL connection string | Required |
| `API_REDIS_HOST` | Redis hostname | `localhost` |
| `API_REDIS_PORT` | Redis port | `6379` |
| `API_RABBITMQ_URL` | RabbitMQ connection URL | Required |
| `API_AUTH_URL` | Auth service URL | Required |

## Legacy Deployment Guides

Detailed deployment guides are preserved in legacy documentation:

- [Kubernetes Tools](../../legacy_project/reference-materials/development/tools/kubernetes/)
- [Helm Guide](../../legacy_project/reference-materials/development/tools/kubernetes/helm.md)
- [Kustomize Guide](../../legacy_project/reference-materials/development/tools/kubernetes/kustomize.md)

> Note: These guides may reference old multi-service architecture. Adapt for single api-service deployment.
