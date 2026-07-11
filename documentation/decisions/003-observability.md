# ADR-003 — Observability stack (2026-07-10)

## 003.1 kube-prometheus-stack in-cluster
**Context:** v1 hand-rolled Prometheus/Grafana in compose; the operator +
ServiceMonitor/PrometheusRule CRDs are the professional workflow.
**Decision:** kube-prometheus-stack (Helm) on kind for v3; the compose stack
keeps its hand-rolled pair. **Consequences:** node/kubelet metrics and k8s
dashboards for free; CRD-driven scrape config practice; some black-box cost
accepted.

## 003.2 Grafana Tempo for traces
**Context:** v3 makes envelope `trace_id` real via OpenTelemetry (HTTP → AMQP
headers → workers); backend choice is UI/family, instrumentation is OTLP
either way. **Decision:** Tempo. **Consequences:** traces live in the same
Grafana as metrics; panel↔trace links work; no separate Jaeger console.

## 003.3 OpenSearch/ELK for logs — deliberately heavyweight
**Context:** Loki would be the light Grafana-family choice; owner explicitly
chose the enterprise stack for its practice value. **Decision:**
OpenSearch + dashboards + a collector (fluent-bit) as the v3 log pipeline.
**Consequences:** real full-text/index-management practice matching enterprise
stacks; significant RAM on a laptop cluster — run single-node with tight JVM
heap caps, and treat resource pressure itself as drill material.
trace_id⇄log correlation must be wired manually (structured JSON logs already
exist everywhere).

## 003.4 Alerts push via ntfy
**Context:** single-operator lab; alerts nobody receives teach nothing.
**Decision:** Grafana alert rules route to ntfy (hosted or self-hosted
container) push notifications; Alertmanager (bundled with 003.1 anyway)
remains available for later routing practice. **Consequences:** alerts feel
real on your phone/desktop; drills get a "did you get paged?" dimension.

## 003.5 Exporters: postgres + redis only
**Context:** each exporter costs pods/RAM; drills interrogate DB pools and
cache behavior most. **Decision:** postgres_exporter and redis_exporter only;
Mongo/MinIO native metrics scraped as-is; more only when an experiment needs
them. **Consequences:** lean cluster; EXP coverage for pool-saturation and
eviction drills.

## 003.6 SLOs derived from measured baselines
**Context:** v4's scored experiments need pass/fail numbers that mean
something on this hardware. **Decision:** v3 runs a calibration experiment
(steady-state measurements), sets SLO = baseline + margin, records both in a
doc the assertions reference. **Consequences:** honest thresholds; recalibrate
when hardware or architecture changes.
