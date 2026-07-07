# Deployment

**Local development:** use the root [`docker-compose.yml`](../docker-compose.yml)
(`make up` / `make infra` from the repo root). It runs all infrastructure,
applies both databases' migrations, creates the MinIO bucket, and starts every
service with the env vars pinned in [CONTRACTS.md](../CONTRACTS.md).

**Kubernetes:** manifests are owned per service:

- `api-service/deployments/kubernetes/`
- `graph-worker/graphrag-service/deployments/kubernetes/`
- `graph-worker/operational-workers/deployments/kubernetes/{email,image,profile}-worker/`

Cluster-level design docs in this folder:

- [CLUSTER_VISION.md](CLUSTER_VISION.md) — target cluster topology, data flows, validation checklist
- [IMPLEMENTATION_PLAN.md](IMPLEMENTATION_PLAN.md) — phased deployment plan

These two documents predate the 2026-07 refactor; treat the root compose file
and CONTRACTS.md as authoritative where they disagree.
