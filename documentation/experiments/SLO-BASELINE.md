# SLO baseline (ADR-003.6 · EXP-33)

SLOs here are **derived from measurement, not aspiration**: run the
calibration workload, record what the healthy lab actually does, set
SLO = baseline + margin, and encode the numbers where the alert rules read
them (`deploy/obs/manifests/` PrometheusRule). Re-calibrate after any change
that plausibly moves the envelope (hardware, replica counts, resource
limits, major dependency bumps).

## How to run a calibration

```bash
make cluster-up && make obs-up        # or: make up (compose)
bash scripts/simulate/slo-baseline.sh # steady sim-load + Prometheus snapshot
```

The script runs `sim-load` at a steady rate (`SLO_VUS=10`,
`SLO_DURATION=10m` by default — override via env), waits for queues to
drain, then snapshots the metrics below from Prometheus
(`PROM_URL`, default `http://localhost:9090`) and **appends a dated results
block to this file**. Nothing is overwritten; history accumulates.

## What gets measured

| Measure | Query shape | Why it's the baseline |
|---|---|---|
| API latency p50/p95/p99 | `histogram_quantile(q, sum by (le) (rate(api_http_request_duration_seconds_bucket[<run>])))` | The request-path SLO |
| API request rate | `sum(rate(api_http_requests_total[<run>]))` | Load context for the quantiles |
| API error ratio | 5xx over total | Availability SLO |
| Worker drain rates | `rate(<type>_processing_success_total[<run>])` per worker | Queue-side capacity |
| Peak queue depth | `max_over_time(rabbitmq_queue_messages{queue=~".*-processing"}[<run>])` | Burst headroom context |
| Resource envelope | `container_memory_working_set_bytes` / CPU per pod (kind only) | Limits/requests calibration |

## Setting the SLOs (worksheet)

After a calibration run, fill this in and update the PrometheusRule:

| SLO | Baseline (measured) | Margin | SLO value | Encoded at |
|---|---|---|---|---|
| API p99 latency | 0.966 s (10 VU, TLS ingress) | ×3 | **3 s** | `APIP99OverSLO` threshold |
| API availability (non-5xx) | 100% (zero 5xx in run) | −0.5pp | **99.5%** | `APIAvailabilityBelowSLO` (ratio > 0.005) |
| Queue depth sustained | peak 6 | ×2 → 12, **overridden: 500** | **500 for 5m** | `QueueDepthSustained` threshold |
| Email drain rate | 7.6 msg/s (1 replica) | ÷2 | 3.8 msg/s | runbook context only |

> **Status:** calibrated 2026-07-19 (block below); thresholds encoded in
> `deploy/obs/manifests/prometheusrule.yaml`. The queue-depth override is
> deliberate: the peak-×2 formula assumed a load-test peak in the hundreds,
> but steady-state peaked at 6 — a threshold of 12 would page on every
> intentional flood drill (EXP-04/32 push 300–600 by design). 500-for-5m
> separates "consumers stuck" from "drill running", and EXP-32 proved the
> full fire→page→resolve loop at that value.

---

<!-- calibration results are appended below this line; do not edit by hand -->

## Calibration run — 2026-07-19 20:47–20:58Z

Conditions: target=cluster, VUs=10, duration=10m, host=fbarrosoaw.
Snapshot note: the in-run port-forward died before the snapshot; these
are the same instant queries re-evaluated at 20:57:50Z (&time=) over
the [10m] run window — numerically equivalent, repair documented in
the exit-runs write-up.

```
-- API latency quantiles (s, over the run window) --
p5 value: 0.0181
p95 value: 0.2198
p99 value: 0.9655
-- API request rate (req/s) --
value: 30.8374
-- API 5xx ratio --
-- Worker drain rates (msg/s) --
value: 7.6345
-- Peak work-queue depth --
email-processing: 6.0000
image-processing: 0.0000
profile-processing: 6.0000
document-processing: 0.0000
-- Pod memory working set (bytes, lab-core) --
api-service-6d965dc9d7-nc8fm: 19664896.0000
api-service-6d965dc9d7-nqwrf: 23121920.0000
auth-service-75445dbf85-kl7xh: 94158848.0000
graphrag-service-66d5b45fb9-5bh68: 52908032.0000
image-worker-747744f8-s5wh2: 11382784.0000
profile-worker-7457c696c4-zk7kc: 20557824.0000
email-worker-dc7c9d464-htvnb: 12095488.0000
```

_Next: transfer these into the worksheet above, add margins, update the PrometheusRule thresholds._
