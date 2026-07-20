# Phase v8 handoff — extraction & reuse

**Audience:** the session that finishes v8. **Precondition:** v4 merged
and proven (templates ship only experiment-beaten patterns — extracting
before EXP-40..45 pass would ship unproven code); ideally v7 too.
Inherited on `phase/v8`: `templates/` scaffolds — per-template READMEs
(each names its exact extraction sources, trim list, proving experiments,
and bootstrap-test contract), pattern-doc outlines
(`templates/patterns/README.md`), the lint-clean `worker-go/chart/`
(ADR-002.1), and `templates/GRADUATION.md`.

Order: 1 → 2 → 3 (extraction, hardest first is auth) → 4 → 5 → 6 → 7.

> **Execution state (2026-07-19, `feat/v8-execution`):** §§1–5 done —
> templates cut from post-v4 `main` (`07bd8bd`; the phase branch's own
> service trees predate the v4 merge), all unit gates green, pattern docs
> written, bootstrap tests + template CI authored. §§6–7 and the v4
> live-run precondition (owner-approved deviation: extraction proceeded
> with "authored; live run pending" citations) are tracked in
> [v8-DEFERRED.md](v8-DEFERRED.md).

## 1–3 — Cut the three templates
For each of auth-service / worker-go / api-publisher, follow its README's
"Extract from + Trim" lists against the POST-v4 code. Method: copy the
source, delete lab-specific surface, rename to neutral module names
(`example.com/...`), keep tests that test the pattern (drop tests that
test lab domain logic). Every kept pattern keeps its experiment citation
in the README (that rule is the phase's soul — enforce it in review).

## 4 — Pattern docs
Write the three docs per `templates/patterns/README.md` outlines. Cite
write-ups in `documentation/experiments/` by filename.

## 5 — Bootstrap tests + template CI
Per README contracts: `test/bootstrap.sh` per template (compose-based
smoke, deliberately breakable), plus a minimal `.github/workflows/ci.yml`
INSIDE each template (build+test — runnable in a consuming repo). Lab CI
addition: a `templates` job running the three bootstrap tests on changes
under `templates/` (path filter).

## 6 — Helm exercise (EXP-82)
The chart exists and lints; the exercise is USING it: `helm install` the
worker template on kind with overridden values, uninstall clean, and
write the honest kustomize-vs-helm comparison into the EXP-82 write-up
(values design impressions are seeded as comments in values.yaml).

## 7 — Exit
- **EXP-80** — the headline: empty repo → auth + one worker + API
  publisher assembled following ONLY the READMEs; verify-equivalent green
  + one smoke experiment; wall-clock logged honestly (< 1 day target).
  Fix every friction found by editing the READMEs, then re-run the clock
  on the fixed portion.
- **EXP-81** — template CI green; a deliberately-broken copy fails its
  bootstrap test.
- **EXP-82** — above.
- GRADUATION.md refined if 80 surfaced gaps; phase doc status; tag
  `lab-v8.0`; PRD success metric checked off with the EXP-80 number.
