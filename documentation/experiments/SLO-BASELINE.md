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
| API p99 latency | _pending calibration_ | ×3 | _pending_ | `APIP99OverSLO` threshold |
| API availability (non-5xx) | _pending_ | −0.5pp | _pending_ | (rule TBD) |
| Queue depth sustained | _pending peak_ | ×2 | _pending_ | `QueueDepthSustained` threshold |
| Email drain rate | _pending_ | ÷2 | _pending_ | runbook context only |

> **Status:** no calibration run recorded yet. The alert rules ship with
> placeholder thresholds (p99 > 500ms, depth > 500 for 5m) marked as such.
> Running EXP-33 replaces them with measured values — see
> `documentation/phases/v3-DEFERRED.md`.

---

<!-- calibration results are appended below this line; do not edit by hand -->
