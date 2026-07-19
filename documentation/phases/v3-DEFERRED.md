# Phase v3 — deferred validation & follow-ups

**Context:** v3 was implemented in an expedited pass (2026-07-19): all code,
manifests, and docs landed and were smoke-checked (builds, unit tests,
kustomize renders), but the exit experiments that need a **running kind
cluster with the obs stack** were authored, not executed. This file is the
honest ledger of what remains before `lab-v3.0` can be tagged.

## Must run before tagging lab-v3.0

| Item | What to do | Doc/entry point |
|---|---|---|
| EXP-30 one-trace | `make cluster-up && make obs-up`, run it, screenshot into a write-up | EXPERIMENTS.md §EXP-30 |
| EXP-31 log triage | Poison batch → OpenSearch triage without kubectl | EXPERIMENTS.md §EXP-31 |
| EXP-32 page yourself | Alert fires to ntfy before Grafana is opened | EXPERIMENTS.md §EXP-32 |
| EXP-33 SLO calibration | `bash scripts/simulate/slo-baseline.sh`; move measured values into the PrometheusRule (placeholders: p99>500ms, depth>500/5m) | documentation/experiments/SLO-BASELINE.md |
| EXP-34 hello-guest | `make guest-up G=hello-guest` on both targets; netpol proof | EXPERIMENTS.md §EXP-34 |
| Watch-without-terminal spot-check | Three EXPERIMENTS.md drills' "Watch" steps via UIs only | acceptance list in phase doc |
| Status page live-truth check | Both targets up → status page shows both correctly | mission-control/README.md |

## Known caveats shipped knowingly

- **Alert routing deviates from the phase text:** alerts are Prometheus-
  native (PrometheusRule → Alertmanager → in-repo ntfy-relay → ntfy), not
  Grafana-managed alerting. Rationale: the rules must live in
  PrometheusRule anyway (phase item 4), and one alerting pipeline is easier
  to drill than two. Revisit only if Grafana-side alert UX is wanted.
- **SLO thresholds are placeholders** until EXP-33 runs (marked in the
  rules and in SLO-BASELINE.md).
- **OpenSearch resource pressure is expected** (ADR-003.3): single node,
  JVM capped at 512m, memory limit 1.5Gi, emptyDir storage — logs are
  disposable. Observe and note per the ADR.
- **Helm chart versions** were pinned at authoring time; `make obs-up`
  fetches them — first run needs network.
- **`loam validate` (workspace hook) errors on `documentation/decisions/*`**
  — pre-existing on main (ADRs have no frontmatter; the forest-level loam
  config claims that directory as a knowledge area). Not introduced by v3;
  fix belongs in a workspace-config or ADR-frontmatter pass of its own.

## Nice-to-haves consciously skipped

- Grafana exemplar wiring beyond what the dashboards ship with ("where
  cheap" per the phase doc).
- OpenSearch ISM/retention policies (logs are 3-day disposable by cap).
- Grafana → OpenSearch data source link (documented workflow uses
  OpenSearch Dashboards directly).
