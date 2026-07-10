# Deployment

## Local development (current, authoritative)

Use the root [`docker-compose.yml`](../../docker-compose.yml) via the root
Makefile (`make up` / `make infra` / `make down`). It runs all infrastructure,
applies both databases' migrations, creates the MinIO bucket, and starts every
service (plus Prometheus/Grafana) with the env vars pinned in
[CONTRACTS.md](../../CONTRACTS.md).

## Kubernetes (target, see PRD v2)

Restoring the kind-based cluster lab is the PRD's v2 milestone — the original
lab (kind configs, ingress-nginx, metrics-server, network policies, per-service
manifests, k6 jobs) lives in git history at `1efec36` (`k8s/`) and in
`legacy_project/k8s/`.

Current per-service manifests (deployable pieces, not yet a full cluster):

- `api-service/deployments/kubernetes/`
- `graph-worker/graphrag-service/deployments/kubernetes/`
- `graph-worker/operational-workers/deployments/kubernetes/{email,image,profile}-worker/`

## Design documents

- [CLUSTER_VISION.md](CLUSTER_VISION.md) — target cluster topology, data flows, validation checklist
- [DEPLOYMENT_IMPLEMENTATION_PLAN.md](DEPLOYMENT_IMPLEMENTATION_PLAN.md) — phased deployment plan

> Both predate the 2026-07 refactor; where they disagree with the root compose
> file or CONTRACTS.md, the latter win. See also the
> [PRD](../PRD.md) for the current roadmap and open questions.
