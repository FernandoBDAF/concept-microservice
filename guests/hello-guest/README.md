# hello-guest

The reference guest (ADR-001.4): the smallest possible external system that
fully satisfies the host contract
([`documentation/HOST_CONTRACT.md`](../../documentation/HOST_CONTRACT.md)).
Two Go stdlib-only components — a **web** server that says hello and a
**worker** that "does a job" every 2 s — whose only real job is to prove that
an isolated guest (own compose project / own namespace, default-deny netpols)
is still fully visible in the lab's shared observability stack.

## 5-minute tour

```bash
# 0. The lab must be up (creates the microservices_default network + Prometheus):
make up                                                      # repo root

# 1. Bring the guest up — its OWN compose project, host ports 41xx:
docker compose -f guests/hello-guest/docker-compose.yml up -d --build

# 2. Poke it:
curl -s localhost:4100/          # {"message":"hello from guest",...}
curl -s localhost:4100/health    # ok
curl -s localhost:4100/ready     # ready
curl -s localhost:4100/metrics | grep hello_guest_web
curl -s localhost:4110/metrics | grep hello_guest_jobs_total   # ticking every 2 s

# 3. See it host-side — Prometheus http://localhost:9090/targets shows the
#    hello-guest job; Grafana Explore: rate(hello_guest_web_requests_total[1m]).

# 4. Run the drill: EXPERIMENTS.md → EXP-HG-01.

# 5. Same thing on kind (build/push to localhost:5001, apply base):
#    see launch.yaml targets.kind.up; then https://hello-guest.lab.local/
kubectl kustomize guests/hello-guest/k8s/base | head    # inspect first
```

Dev loop: `cd guests/hello-guest && go build ./... && go vet ./... && go test ./...`

## Contract evidence

| Contract clause (HOST_CONTRACT.md) | Evidence here |
|---|---|
| §1.1 containerized, pinned images | `Dockerfile` — multi-stage, `golang:1.24-alpine` / `alpine:3.19`, one `--build-arg CMD=web\|worker` per image, non-root UID 10001 |
| §1.2 /health /ready /metrics per component | `cmd/web/main.go`, `cmd/worker/main.go`; metrics hand-rolled in `internal/metrics` (exposition format unit-tested) |
| §1.3 `launch.yaml` | [`launch.yaml`](launch.yaml) — compose + kind up/down/status, components, ports |
| §1.4 assigned port block | 41xx: web → 4100, worker → 4110 (only host ports published) |
| §1.5 guest experiment | [`EXPERIMENTS.md`](EXPERIMENTS.md) EXP-HG-01 — burst visible in shared Grafana, worker kill → flatline + self-heal |
| §1.6 deployment note | below |
| §2 isolation (host side) | compose project `hello-guest` + own default network; `k8s/base` — namespace `hello-guest` (`lab.local/tier: guest`), default-deny netpols with only DNS, lab-obs→8080 scrape, ingress-nginx→web allowed |
| §2 observability (host side) | compose: aliases `hello-guest-web`/`hello-guest-worker` on `microservices_default` for the host Prometheus; k8s: `k8s/obs/` ServiceMonitors (separate from base — needs the obs CRDs) |
| §2 ingress (host side) | `k8s/base/ingress.yaml` — `hello-guest.lab.local`, TLS via `lab-ca` |

## Metrics

| Metric | Type | Component |
|---|---|---|
| `hello_guest_web_requests_total` | counter | web |
| `hello_guest_web_uptime_seconds` | gauge | web |
| `hello_guest_jobs_total` | counter | worker |
| `hello_guest_job_duration_seconds` | histogram | worker |
| `hello_guest_worker_uptime_seconds` | gauge | worker |

Names are contract-stable (§1.2) — shared dashboards may query them.

## Deployment note (contract §1.6)

How this would deploy for real: it wouldn't — hello-guest is a fixture, not a
product. But answering honestly is the clause: as a real service it would be
**standalone** (no shared-infra opt-in — it has no state, no queue, no
storage), deployed as the same two Deployments behind any ingress, with the
image pushed to a real registry instead of `localhost:5001`. What it lacks
for production: no TLS termination of its own, no config beyond
`HELLO_JOB_INTERVAL`, single replica, no alerting rules on its metrics.
Lab mode vs production mode differ only in registry, replica count, and
hostname — which is exactly the property the contract wants guests to state.
