# Cluster lab manifests (PRD v2)

`make cluster-up` runs the entire v1 stack on kind. Compose
(`docker-compose.yml`) remains the behavioral reference and the fast inner
loop (ADR-002.4); this tree is its cluster twin, checked mechanically by
`make drift-check`.

## Layout (ADR-002.1)

```
deploy/
  kind/                 cluster profiles: single.yaml (daily), multinode.yaml
                        (1 control-plane + 3 workers — EXP-21 node drills)
  k8s/
    vendor/             pinned upstream: ingress-nginx (kind flavor),
                        cert-manager — applied by scripts/cluster/up.sh
    base/               kustomize base: namespaces, infra/ (postgres, redis,
                        rabbitmq, mongodb, minio), migrations/ (one-shot
                        Jobs), services/ (api, auth, graphrag, 3 workers),
                        ingress/ (TLS + hosts), netpols/ (zero-trust)
    overlays/
      kind-local/       daily overlay (passthrough today; diverges from the
                        aws overlay in v5)
      kind-multinode/   pins stores to the infrastructure node for node-kill
                        drills (local-path PVs carry node affinity)
```

Namespaces: `lab-core` (services), `lab-infra` (data stores + one-shot
jobs), `lab-obs` (reserved for v3 observability).

## Build note: LoadRestrictionsNone

The rabbitmq config and the migration SQL are **single-sourced** from
`scripts/compose/rabbitmq/` and `{api,auth}-service/migrations/` via
configMapGenerator — the whole point of the drift ADR is that these never
fork. kustomize's default load restrictor refuses files above the
kustomization root, so every build uses:

```
kustomize build --load-restrictor LoadRestrictionsNone deploy/k8s/overlays/kind-local
```

(`make cluster-up`, the drift check, and CI all pass the flag.)

## Hostnames & TLS (ADR-009.4)

Ingress serves `https://api.lab.local` and `https://auth.lab.local` with
certificates issued by the in-cluster **lab CA** (cert-manager: self-signed
root → `lab-ca` ClusterIssuer → per-host certs via ingress annotations).

Name resolution — the decision is `/etc/hosts` (offline-reliable, one sudo):

```
sudo sh -c 'echo "127.0.0.1 api.lab.local auth.lab.local" >> /etc/hosts'
```

No-sudo alternatives, used by scripts and write-ups:
- `curl --resolve api.lab.local:443:127.0.0.1 https://api.lab.local/...`
- k6 containers get `--add-host api.lab.local:host-gateway` (see the
  `cluster-sim-*` targets)
- `*.localtest.me` (public DNS → 127.0.0.1) would avoid /etc/hosts entirely
  but adds an internet dependency to every local request — rejected for a
  lab that must work offline.

The CA is self-signed: `curl -k` / `K6_INSECURE_SKIP_TLS_VERIFY=true`, or
trust it once with
`kubectl -n cert-manager get secret lab-root-ca -o jsonpath='{.data.ca\.crt}' | base64 -d > lab-ca.crt`
and add it to your keychain.

## Secrets (ADR-009.3)

`make init-secrets` generates credentials into `.lab-secrets.env`
(gitignored) and applies them as Secrets in both namespaces. Services embed
them via `secretKeyRef` + `$(VAR)` expansion for URL-shaped values (DSN,
AMQP/Mongo URIs). Compose mode keeps its documented lab defaults — the two
modes share *shape*, not values. `FORCE=1 make init-secrets` rotates
(requires `make cluster-up` to converge, and fresh PVCs to take effect in
the stores).

## Rate limiting (ADR-009.5)

Per-IP nginx annotations on the **auth** ingress only (`limit-rps: 20`,
429 via controller ConfigMap patch). The api ingress is unlimited — burst
experiments must flow. Token-validate traffic is pod-to-pod and never
touches the edge. Compose mode keeps the in-app limiter.

## Experiment parity (EXP-20)

| compose lever | cluster lever |
|---|---|
| `make sim-smoke/load/burst` | `make cluster-sim-smoke/load/burst` |
| `make queues` | `make cluster-queues` |
| `make scale S= N=` | `make cluster-scale S= N=` |
| `make logs S=` | `make cluster-logs S=` |
| `docker compose stop <svc>` | `kubectl -n lab-core scale deploy/<svc> --replicas=0` |
| `docker compose stop rabbitmq` | `kubectl -n lab-infra scale statefulset/rabbitmq --replicas=0` |
| `publish.py` (mgmt API) | `kubectl -n lab-infra port-forward svc/rabbitmq 15672:15672` + `RABBITMQ_MGMT_PASSWORD=$(grep RABBITMQ .lab-secrets.env | cut -d= -f2) python3 scripts/simulate/publish.py …` |
| MinIO console :9001 | `kubectl -n lab-infra port-forward svc/minio 9001:9001` |
| Grafana/Prometheus | stay on compose until v3 (`lab-obs` reserved) |
