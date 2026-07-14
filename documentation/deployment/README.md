# Deployment

## Local development (current, authoritative)

Use the root [`docker-compose.yml`](../../docker-compose.yml) via the root
Makefile (`make up` / `make infra` / `make down`). It runs all infrastructure,
applies both databases' migrations, creates the MinIO bucket, and starts every
service (plus Prometheus/Grafana) with the env vars pinned in
[CONTRACTS.md](../../CONTRACTS.md).

## Kubernetes (shipped, PRD v2)

`make cluster-up` runs the entire stack on kind — kustomize tree, local
registry, ingress+TLS, zero-trust network policies. Manifests and operations:
[deploy/k8s/README.md](../../deploy/k8s/README.md); cluster experiments:
EXPERIMENTS.md EXP-20..23.

The era-1 lab this restored (kind configs, ingress-nginx, metrics-server,
network policies, per-service manifests, k6 jobs) lives on branch
`archive/era-1` (`legacy_project/k8s/`) and in git history at `1efec36`
(`k8s/`) — ADR-010.4.

## Design documents

- [CLUSTER_VISION.md](CLUSTER_VISION.md) — target cluster topology, data flows, validation checklist
- [DEPLOYMENT_IMPLEMENTATION_PLAN.md](DEPLOYMENT_IMPLEMENTATION_PLAN.md) — phased deployment plan

> Both predate the 2026-07 refactor; where they disagree with the root compose
> file or CONTRACTS.md, the latter win. See also the
> [PRD](../PRD.md) for the current roadmap and open questions.
