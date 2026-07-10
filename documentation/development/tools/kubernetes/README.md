# Kubernetes Tools

Kubernetes deployment tools and configuration guides.

## Contents

- [Helm](helm.md) - Helm chart development and usage
- [Kustomize](kustomize.md) - Kustomize overlays and patches
- [Comparison](comparison.md) - When to use Helm vs Kustomize

## Deployment Strategy

The api-service uses a combination of:
- **Helm** for templating and package management
- **Kustomize** for environment-specific overlays

## Quick Reference

```bash
# Deploy with Helm
helm install api-service ./charts/api-service

# Deploy with Kustomize
kubectl apply -k overlays/production
```

---

*Migrated from legacy_project/reference-materials/development/tools/kubernetes/*
