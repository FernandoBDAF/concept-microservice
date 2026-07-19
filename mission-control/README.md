# Mission Control (v3 seed)

The v3 observability phase's thin status page (ADR-001.3) and the first
read-only sliver of `lab-controld` (ADR-005.1/.2). This is the seed of the
v6 Mission Control cockpit: the API shapes are built to grow, the
implementation is deliberately thin.

## What's here

| Component | Path | Port | Stack |
|---|---|---|---|
| `controld` | `mission-control/controld/` | `127.0.0.1:4900` | Go 1.24, stdlib only |
| `status-page` | `mission-control/status-page/` | `127.0.0.1:4901` | Next.js (App Router, TypeScript), plain CSS |

`controld` observes the two lab targets by shelling out to the same tools
you use by hand (`docker compose`, `kubectl`, `kind`) and summarizes the
results as JSON. The status page polls it every 5 s and renders a
terminal-style console: target switcher, service cards with
state/health badges, health probe results, and per-target links.

## Read-only guarantee

This phase performs **no control actions of any kind**. `controld` only
runs read-only commands (`docker compose ls/ps`, `kubectl get pods`,
`kind get clusters`) and HTTP GET health probes. There are no endpoints
that start, stop, restart, scale, or mutate anything. Both processes bind
`127.0.0.1` only, with no auth, per ADR-005.4 (localhost-only until remote
targets arrive). CORS is restricted to the status page's origin
(`127.0.0.1:4901` / `localhost:4901`).

## Run

```
make controld       # go run the daemon on 127.0.0.1:4900
make status-page    # next dev on 127.0.0.1:4901
```

Then open http://127.0.0.1:4901. Both targets down is a normal state; the
page shows "target unavailable" until compose or kind comes up.

Overrides:

- `CONTROLD_ADDR` (or `-addr`) — controld listen address, default
  `127.0.0.1:4900`. Keep it on localhost.
- `NEXT_PUBLIC_CONTROLD_URL` — where the page reaches controld, default
  `http://127.0.0.1:4900`.

## API

- `GET /healthz` — liveness, plain `ok`.
- `GET /api/targets` — `[{name, available}]` for `compose` and `kind`;
  availability probed via `docker compose ls` / `kind get clusters`,
  cached ~5 s.
- `GET /api/status?target=compose|kind` — compose: per-service
  `{name, state, health, image}` from `docker compose -p microservices ps`;
  kind: workload-level `{namespace, name, ready, status}` aggregated from
  pods in `lab-core`, `lab-infra`, `lab-obs` (pod hash suffixes collapsed).
- `GET /api/health?target=compose|kind` — compose: HTTP probes of the
  host-mapped health endpoints (api :8080, auth :3000, graphrag :8082;
  workers are not port-mapped and are skipped); kind: derived from pod
  readiness, no HTTP. Shape: `{service, ok, latency_ms, error?}`.
- `GET /api/links?target=compose|kind` — static per-target links map
  (Grafana, Prometheus, RabbitMQ, MinIO on compose; the `*.lab.local`
  ingress hosts on kind).

Every shell-out runs under a 10 s timeout; failures return
`{"error": "..."}` with HTTP 502 — a missing `docker` or `kubectl` never
crashes the daemon. Logs are structured JSON on stdout (`slog`).

## v6 growth path

- **Actions**: controld gains POST endpoints that invoke the existing make
  targets (ADR-005.2 — make stays the single source of truth), with
  streaming output over WS. The read-only endpoints above keep their
  shapes.
- **Experiments**: render the ADR-004.2 YAML experiment definitions, run
  them via make, embed each experiment's Watch dashboards, record outcomes
  to `documentation/experiments/`.
- **Targets**: the `{name, available}` target list grows an `aws` entry in
  v5; remote control arrives only together with the auth story
  (ADR-005.4 — minimum shared token + TLS).
- **UI**: the status page grows into the Mission Control cockpit on the
  same Next.js stack (ADR-005.1); this page is the de-risking exercise.
