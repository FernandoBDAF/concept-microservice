# Phase v3 — Observability depth (+ status page + hello-guest)

**Status:** implemented and **validated** (expedited 2026-07-19; deferred
runs executed same day — EXP-30..34 all pass, six defects found and fixed;
ledger closed in [v3-DEFERRED.md](v3-DEFERRED.md), evidence in
[2026-07-19-v3-obs-exit-runs.md](../experiments/2026-07-19-v3-obs-exit-runs.md))
· **Depends on:** v2 · **Exit tag:** `lab-v3.0` (tag at merge) ·
**Decisions in force:** ADR-003 (all), ADR-001.3 (status page), ADR-001.4
(hello-guest), ADR-005.1/.2 seeds, ADR-007.1/.2 (contract draft)

## Mission

Every drill becomes diagnosable **from UIs alone**: metrics (operator-managed),
traces (end-to-end through the queues), logs (searchable, correlated), alerts
(pushed to you). Plus two seeds this phase plants: the thin status page
(Mission Control rung 1) and the hello-guest fixture that proves the host
contract.

## Context a fresh session needs

- v2's kind cluster runs the full stack; compose remains first-class.
- Services already emit: Prometheus metrics everywhere, structured JSON logs
  everywhere, and a `metadata.trace_id` in queue envelopes that is currently
  **decorative** (generated per message, not propagated from HTTP).
- Scrape topology reference: `scripts/compose/prometheus.yml`. Dashboard
  reference: `scripts/compose/grafana/dashboards/lab-overview.json`.

## Work breakdown

1. **kube-prometheus-stack (ADR-003.1)** into `lab-obs`: ServiceMonitors for
   every lab service (+RabbitMQ), PrometheusRule for the alert set below;
   port the Lab Overview dashboard; add node/kubelet dashboards. Postgres +
   redis exporters only (ADR-003.5).
2. **Tracing (ADR-003.2):** OpenTelemetry SDKs in api-service (gin +
   database/sql + amqp publish), auth-service (express), workers and graphrag
   (consume side): extract context from AMQP headers, so envelope
   `metadata.trace_id` = real W3C traceparent. Tempo in lab-obs; Grafana
   trace panels + exemplars where cheap.
3. **Logs (ADR-003.3 — owner chose the heavyweight):** OpenSearch single-node
   with hard JVM caps + fluent-bit DaemonSet shipping container logs;
   index template for the lab's JSON shape; OpenSearch Dashboards; document
   the trace_id search workflow (and add a Grafana data source link if
   practical). Resource pressure is expected — cap it, observe it, note it.
4. **Alerts (ADR-003.4):** rules — queue depth sustained, DLQ growth, breaker
   open, p99 over SLO, scrape target down, node pressure; route Grafana
   alerting → ntfy (container or ntfy.sh topic); runbook link per alert
   pointing at the matching experiment.
5. **SLO calibration (ADR-003.6):** run the calibration experiment (steady
   `sim-load` on kind), record baselines (p50/p95/p99, drain rates, resource
   envelopes) in `documentation/experiments/SLO-BASELINE.md`; set SLOs =
   baseline+margin; encode them where the alert rules read them.
6. **Thin status page (ADR-001.3, ADR-005.1/.2 seed):** minimal Next.js app +
   the first sliver of `lab-controld` (read-only endpoints: compose ps /
   kubectl get summarized, health probes, links to Grafana/Tempo/OpenSearch/
   RabbitMQ/MinIO). Binds 127.0.0.1. **No control actions.**
7. **Hello-guest fixture (ADR-001.4) + host contract v0 (ADR-007.1/.2):**
   write `documentation/HOST_CONTRACT.md` (containers, health+metrics, launch
   definition, port block, one experiment, deployment note) and build
   `guests/hello-guest/` (tiny web + tiny worker, its own namespace/compose
   project, port block 41xx) conforming to it; it appears on the status page
   and in Grafana like any lab service.

## Out of scope

Scored assertions (v4), Chaos Mesh (v4), any control actions in the UI (v6),
messaging/auth changes (v4), AWS (v5).

## Exit experiments

- **EXP-30 — One trace, whole system:** `make demo-document`; find the single
  trace spanning HTTP upload → publish → graphrag consume in Tempo; screenshot
  into the write-up. Assert: trace has spans from ≥3 services.
- **EXP-31 — Log triage:** inject a poison batch (EXP-05 levers); starting
  from the Grafana DLQ panel, reach the exact failing payload via OpenSearch
  using trace_id/queue labels — without touching kubectl/docker logs.
- **EXP-32 — Page yourself:** stop all email-workers under flood; the
  queue-depth alert must fire and arrive via ntfy before you look at Grafana;
  recovery clears it.
- **EXP-33 — SLO baseline:** the calibration run itself, producing
  SLO-BASELINE.md (this experiment's artifact IS the acceptance).
- **EXP-34 — Hello-guest onboarding:** `make guest-up G=hello-guest` (or
  cluster equivalent): namespace/isolation correct (netpol proof), health+
  metrics visible in the shared Grafana, its port block honored, and its one
  guest experiment passes.

## Acceptance

- [x] EXP-30..34 pass and are written up (2026-07-19 exit runs)
- [x] Every EXPERIMENTS.md drill's "Watch" achievable without terminal access
      (spot-checked EXP-04/05/06 against live Lab Overview panels)
- [x] Status page shows live truth for both compose and kind targets
- [x] HOST_CONTRACT.md v0 exists, proven by hello-guest (incl. the netpol
      and crash-lever fixes the proof demanded)
- [ ] Tag `lab-v3.0` (at merge — owner's call)
