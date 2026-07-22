# Pattern docs

Standalone pattern guides distilled from the lab's hardened pieces (v8
extraction, v8-HANDOFF §4). Each doc is written for a *consuming* project:
context → the pattern → the failure modes experiments found → a mitigation
menu → an adaptation checklist. Claims are cited to an experiment write-up
filename or a lab code path; anything not yet proven is marked
expected/pending inline.

| Doc | What it covers |
|---|---|
| [cache-aside.md](cache-aside.md) | Postgres-backed cache-aside with Redis (`profile:<id>` / `profiles:list:<page>`, delete-on-write, 15m/5m TTLs). Leads with the EXP-12 finding: a cache outage is silent latency amplification at a 0% error rate; mitigation menu (fail-fast timeouts, cache-path breaker, latency SLI) and the missing-singleflight stampede caveat. |
| [storage-pipeline.md](storage-pipeline.md) | Object-storage document pipeline: upload → object store → metadata row → async processing → status lifecycle. Transactional outbox (publish-after-commit, EXP-42 target), compensating delete, `task-results` write-back, 15m presigned downloads, and the absent orphan-reconciliation sweep. |
| [deploy-shapes.md](deploy-shapes.md) | The kustomize layout that survived v2→v5: base + overlays, single-source `configMapGenerator` with `LoadRestrictionsNone`, per-service-class probes/limits/PDB defaults, the netpol starter (default-deny + dns + explicit allows, union-of-allows lesson from EXP-23), and drift-check as a CI institution. |

Experiment write-ups cited by these docs live in
`documentation/experiments/` in the lab.
