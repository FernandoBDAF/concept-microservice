# Phase documents — the development methodology

One document per PRD milestone. Each is a **self-contained implementation
brief**: a fresh working session should be able to execute a phase from its
document alone (plus the references it names). Shipped phases keep their doc
as the record of what exists and what validates it.

## The workflow

1. Start a new session; give it `documentation/phases/<phase>.md`.
2. Execute the work breakdown. Decisions are already made (ADR references
   inline) — don't relitigate them; if reality forces a change, write a
   superseding ADR entry first.
3. A phase is DONE when its **exit experiments pass** — every phase is
   validated by experiments (they extend [EXPERIMENTS.md](../../EXPERIMENTS.md)
   / the `experiments/` definitions once v4 lands).
4. Write the run-up in [documentation/experiments/](../experiments/), tag the
   repo `lab-vN` (ADR-010.5), update the phase doc status, move on.

## Sequence (resequenced 2026-07-10, ADR-001.2)

| Phase | Doc | Status | Exit tag |
|---|---|---|---|
| v1 — Foundation | [v1-foundation.md](v1-foundation.md) | ✅ shipped | (pre-tagging) |
| v1.1 — Guided experiments | [v1.1-experiments.md](v1.1-experiments.md) | ✅ shipped — owner catalog run pending | lab-v1.1 on catalog pass |
| v2 — Cluster lab (kind) | [v2-cluster-lab.md](v2-cluster-lab.md) | ✅ shipped | lab-v2.0 ✓ |
| v3 — Observability + status page + hello-guest | [v3-observability.md](v3-observability.md) | ✅ shipped — EXP-30..34 passed | lab-v3.0 ✓ |
| v4 — Hardening & scored experiments | [v4-hardening-and-assertions.md](v4-hardening-and-assertions.md) | merged to main — live runs (EXP-4x) pending | lab-v4.0 on live pass |
| v5 — AWS track | [v5-aws.md](v5-aws.md) | code-complete, in PR #4 — step-0 + EXP-50..55 pending | lab-v5.0 on live pass |
| v6 — Mission Control | [v6-mission-control.md](v6-mission-control.md) | pending | lab-v6.0 |
| v7 — Guest systems | [v7-guest-systems.md](v7-guest-systems.md) | pending | lab-v7.0 |
| v8 — Extraction & reuse | [v8-extraction.md](v8-extraction.md) | pending | lab-v8.0 |

Experiment ID convention: each phase owns a decade — EXP-2x = v2, EXP-3x = v3,
… EXP-8x = v8 (EXP-01..12 belong to v1/v1.1).

Decisions: [documentation/decisions/](../decisions/) (ADR-001..010 cover the
2026-07-10 Q&A that shaped all of this). Vision & roadmap rationale:
[PRD](../PRD.md). Architecture findings driving the v4 fixes:
[review/CONCEPTUAL_REVIEW.md](../review/CONCEPTUAL_REVIEW.md).
