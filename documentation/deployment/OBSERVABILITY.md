# Observability stack (v3 · ADR-003)

The kind cluster's observability lives in `lab-obs`: kube-prometheus-stack
(metrics + alerting), Tempo (traces), OpenSearch + fluent-bit (logs), ntfy
(push alerts). Compose keeps its hand-rolled Prometheus/Grafana pair and
gains only a Tempo container (ADR-003.1).

## Install / remove

```bash
make cluster-up        # the app lab, as always
make obs-up            # helm charts (pinned) + deploy/obs/manifests
make obs-down          # uninstall everything; namespace and CRDs stay
```

`obs-up` is idempotent (`helm upgrade --install` + `kubectl apply`). The
pieces:

| What | How | Pinned |
|---|---|---|
| kube-prometheus-stack | Helm `prometheus-community/kube-prometheus-stack` | 87.17.0 |
| Tempo (single binary) | Helm `grafana/tempo` | 1.24.4 |
| postgres/redis exporters (ADR-003.5) | Helm `prometheus-community/…` | 8.2.0 / 6.27.0 |
| ServiceMonitors, alerts, OpenSearch, fluent-bit, ntfy(+relay), ingresses, netpols | `kustomize build deploy/obs/manifests` | images pinned in manifests |

The `ntfy-relay` image (`localhost:5001/ntfy-relay:dev`, source
`scripts/obs/ntfy-relay/`) is built and pushed by `make images` like every
lab service.

## URLs

Same hostname convention as the app (add to `/etc/hosts` or use
`curl --resolve`; TLS signed by the `lab-ca` issuer):

| URL | What | Login |
|---|---|---|
| https://grafana.lab.local | Lab Overview + node/kubelet dashboards, Tempo explore | admin / admin |
| https://prometheus.lab.local | targets, rules, TSDB | — |
| https://alertmanager.lab.local | routing, silences | — |
| https://opensearch.lab.local | OpenSearch Dashboards (Discover on `lab-logs-*`) | — (security plugin disabled) |
| https://ntfy.lab.local | ntfy server, topic `lab-alerts` | — |

```bash
sudo sh -c 'echo "127.0.0.1 grafana.lab.local prometheus.lab.local alertmanager.lab.local opensearch.lab.local ntfy.lab.local" >> /etc/hosts'
```

## Metrics

ServiceMonitors (`deploy/obs/manifests/servicemonitors.yaml`) mirror the
compose scrape topology (`scripts/compose/prometheus.yml`): api-service
:8081, auth-service :3000 (`/metrics` on the app port), graphrag :8081,
all three workers :8080 (one monitor, pod-level discovery — the EXP-07
undercount is gone), rabbitmq :15692. Prometheus is configured to pick up
**every** ServiceMonitor/PrometheusRule in the cluster, no labels required.

## Traces (ADR-003.2)

Services export OTLP/http to `tempo.lab-obs:4318` (`OTEL_*` env in the
service manifests). Grafana has a Tempo datasource (uid `tempo`); the Lab
Overview dashboard's "Traces" row searches it, and exemplars on the API
p99 panel deep-link sample points to their traces. First stop for EXP-30.

## Logs and the trace_id workflow (ADR-003.3, EXP-31)

fluent-bit tails `/var/log/containers/*.log` on every node, parses CRI
framing, merges each service's structured-JSON line to top-level fields and
ships to OpenSearch as daily `lab-logs-*` indices. An index template (Job
`lab-logs-index-template`) maps `trace_id`/`level`/`service` as keywords.

The triage loop, terminal-free:

1. Grafana → Lab Overview → **Dead-letter queues** panel shows growth.
2. Open OpenSearch Dashboards → Discover → index pattern `lab-logs-*`
   (create it once: Management → Index Patterns, time field `@timestamp`).
3. Filter the failing worker: `kubernetes.container_name: email-worker
   and level: error` — the error lines carry the envelope `trace_id`.
4. Pivot to the whole request: `trace_id: "<value>"` shows every log line
   from every service that touched that message; paste the same id into
   Grafana → Explore → Tempo for the span view.

## Alerts (ADR-003.4)

`PrometheusRule lab-alerts`: QueueDepthSustained, DLQGrowth,
AuthBreakerOpen, APIP99OverSLO (3 s) and APIAvailabilityBelowSLO (99.5%
non-5xx) — both calibrated 2026-07-19 per
`documentation/experiments/SLO-BASELINE.md` (ADR-003.6),
ScrapeTargetDown; node-pressure comes from the chart's default rule set.
Every alert carries a `runbook_url` into the matching EXPERIMENTS.md drill.

Flow: Prometheus → Alertmanager → webhook → **ntfy-relay** → ntfy topic
`lab-alerts` (critical→urgent, warning→high, resolved→default priority).

Subscribe (the EXP-32 "did you get paged?" dimension):

- Phone: install the ntfy app → add subscription → server
  `https://ntfy.lab.local` (trust the lab CA or use the ntfy.sh variant),
  topic `lab-alerts`.
- Desktop: `open https://ntfy.lab.local` and subscribe in the web UI, or
  `curl -sk -N https://ntfy.lab.local/lab-alerts/sse`.
- No lab CA on the phone? Point the relay at the hosted server instead:
  set `NTFY_URL=https://ntfy.sh` and a hard-to-guess `NTFY_TOPIC` in
  `deploy/obs/manifests/ntfy-relay.yaml` (egress :443 is already allowed)
  and subscribe to that topic in the app.

## Resource posture (ADR-003.3 — pressure is drill material)

The log stack is deliberately heavyweight for a laptop cluster:

- OpenSearch: single node, JVM pinned `-Xms512m -Xmx512m`, requests 1Gi /
  limits 1.5Gi, **emptyDir** — lab logs are disposable; a restart costs
  the indices and that is the accepted trade.
- Prometheus: 3d retention, no PV; Tempo: 24h retention, no PV.
- Expect memory pressure with everything running. That's on purpose:
  watch it in the node dashboards, and treat evictions as drills, not
  incidents.

## Network policy

`lab-obs` is zero-trust like the app namespaces
(`deploy/obs/manifests/netpols.yaml`): default-deny, DNS, one deliberate
relaxation (intra-namespace open — chart-owned pod labels aren't a stable
per-edge contract), plus explicit cross-namespace edges: prometheus →
app/rabbitmq metrics ports, exporters → postgres/redis, services → tempo
:4318 (allows on both sides), ingress-nginx → the UIs. EXP-23-style denial
checks apply here too.
