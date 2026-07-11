# ADR-001 — Methodology & roadmap sequencing (2026-07-10)

## 001.1 Phase documents for every milestone, including shipped ones
**Context:** development proceeds phase-by-phase in fresh sessions; each needs
one self-contained brief. **Decision:** `documentation/phases/` holds one doc
per PRD milestone, v1 and v1.1 included — shipped docs are records + the
structural template; pending docs are implementation briefs. **Consequences:**
a new session starts from exactly one file; every phase names the experiments
that validate it.

## 001.2 AWS before Mission Control
**Context:** original order had the UI (v5) before AWS (v6); UI-first risks
abstractions built for targets that don't exist. **Decision:** swap — **v5 =
AWS track, v6 = Mission Control**. Cloud practice starts sooner via make
targets; the UI is designed once against all three targets (compose/kind/AWS).
**Consequences:** UI work waits; a thin status page (001.3) covers visibility
in the interim.

## 001.3 Thin read-only status page ships early (v3)
**Context:** every experiment session wants "what's running + links" at a
glance. **Decision:** Mission Control rung 1 (visibility only, no actions)
ships during v3, built with the same stack the full UI will use (Next.js,
ADR-005.1) to de-risk it cheaply. **Consequences:** v6 grows from a proven
seed instead of starting cold.

## 001.4 Hello-guest fixture before real guests
**Context:** the host-platform contract only hardens against a tenant, but the
real guests arrive at v7. **Decision:** build a deliberately trivial 2-container
"hello-guest" (web + worker) when the contract is drafted, during v3 — real
guests still onboard at v7. **Consequences:** contract, namespace isolation,
port policy and launch mechanics get proven cheaply; v7 onboards against a
tested contract.
