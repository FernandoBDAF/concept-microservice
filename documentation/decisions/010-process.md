# ADR-010 — Process: CI, templates, legacy, records (2026-07-10)

## 010.1 Decision records: thematic lightweight ADRs (this folder)
**Decision:** one file per theme, sub-numbered decisions, supersession by
reference (see README). The 2026-07-10 Q&A is recorded as ADR-001..010.

## 010.2 CI: GitHub Actions, phased
**Decision:** Phase 1 (lands with v2): `make verify` + image builds + the
compose/kustomize drift check (ADR-002.4) on push. Phase 2 (after v4): scored
smoke-experiment subset against a compose stack in CI. Phase 3 (inside v5):
the OIDC→ECR→EKS deploy pipeline (ADR-006.7). **Consequences:** CI grows with
the lab instead of front-loading it.

## 010.3 Templates: in-repo first, split on adoption
**Decision:** v8 extracts pieces into `templates/<piece>` with READMEs and
copy instructions; a piece graduates to its own repo the first time a real
project adopts it. **Consequences:** no premature repo sprawl; adoption is
the promotion signal.

## 010.4 legacy_project: mine at v2, then archive branch
**Decision:** v2 ports what it needs (kind configs, network policies, k6
jobs/analyses); afterwards legacy_project moves to an `archive/era-1` branch
and is deleted from main. **Consequences:** clones drop ~209MB; history keeps
everything reachable.

## 010.5 Tag phase exits
**Decision:** when a phase's exit experiments pass, tag `lab-vN` (lab-v2.0,
lab-v3.0…); experiment write-ups cite tags. **Consequences:** every recorded
result maps to an exact, returnable lab state.
