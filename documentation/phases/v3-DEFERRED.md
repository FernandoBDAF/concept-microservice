# Phase v3 — deferred validation & follow-ups

**Status: executed 2026-07-19 — all items pass.** v3 was implemented in an
expedited pass (2026-07-19) with the exit experiments authored but not run;
this ledger tracked what remained before `lab-v3.0` could be tagged. The
deferred runs were executed the same day on a fresh Windows 11 + WSL2
machine and **every item passed**, finding six real defects on the way
(all fixed on the validation branch). Full evidence:
[2026-07-19-v3-obs-exit-runs.md](../experiments/2026-07-19-v3-obs-exit-runs.md).

## The ledger, closed

| Item | Result |
|---|---|
| EXP-30 one-trace | ✅ one W3C trace, 18 spans, 3 services; envelope trace_id = Tempo id = graphrag log id |
| EXP-31 log triage | ✅ documented Discover loop end-to-end; found+fixed the index-template race and the missing worker trace_id |
| EXP-32 page yourself | ✅ push at 20:31:41Z (5m43s after flood, before any dashboard), runbook link intact; resolved push after recovery |
| EXP-33 SLO calibration | ✅ SLO-BASELINE.md has the measured block; thresholds encoded (p99 3s, availability 99.5%, depth 500 kept deliberately) |
| EXP-34 hello-guest | ✅ both targets; found+fixed the guest scrape egress netpol and the EXP-HG-01 compose crash lever |
| Watch-without-terminal spot-check | ✅ EXP-04/05/06 watch steps all resolve to live Lab Overview panels |
| Status page live-truth check | ✅ controld truthful for both targets (kind workloads + compose services) |

## Caveats from the implementation pass — resolution

- **Alert routing deviates from the phase text** (Prometheus-native, not
  Grafana alerting): kept — EXP-32 validated the full pipeline; one
  alerting path is easier to drill than two.
- **SLO thresholds are placeholders**: resolved — calibrated 2026-07-19,
  see SLO-BASELINE.md worksheet (including the deliberate queue-depth
  override and its rationale).
- **OpenSearch resource pressure expected**: observed as designed — one
  transient probe-storm (host-side CPU stall) killed and cleanly restarted
  Prometheus; logged in the write-up as drill material, no OOM.
- **Helm chart versions / image pins unverified**: closed — every pin
  resolved and pulled on first `make obs-up` (kps 87.17.0, tempo 1.24.4,
  opensearch/dashboards 2.19.1, fluent-bit 3.2.10, ntfy v2.11.0).
- **`loam validate` workspace-hook noise on decisions/**: still open,
  still pre-existing on main; belongs to a workspace-config pass.

## Nice-to-haves consciously skipped (unchanged)

- Grafana exemplar wiring beyond what the dashboards ship with.
- OpenSearch ISM/retention policies (logs stay 3-day disposable by cap).
- Grafana → OpenSearch data source link (workflow uses OpenSearch
  Dashboards directly).
