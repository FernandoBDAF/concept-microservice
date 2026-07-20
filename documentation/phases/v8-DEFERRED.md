# v8 deferral ledger — what the extraction session could not execute

**Status:** open (2026-07-19) · **Owner of closure:** the session that runs
the v8 exit experiments · **Companion:** [v8-HANDOFF.md](v8-HANDOFF.md)
(steps 1–5 executed; this ledger tracks 6–7 plus preconditions)

The v8 extraction session (branch `feat/v8-execution`) cut the three
templates, the pattern docs, the bootstrap tests, and the CI harness. Per
house rules, everything *not* executed is registered here, not hidden.

## Deferred items

### D1 — EXP-80 (bootstrap under a day) — not run
The headline metric. Requires a human-paced session: empty repo → auth +
worker + publisher assembled following only the READMEs, wall-clock logged
honestly. Nothing blocks it except time; run it after D4 closes so the
clock measures the *proven* templates.

### D2 — EXP-81 (template regression) — smokes green in CI; broken-copy half pending
The harness exists and its live half ran: all three `test/bootstrap.sh`
smokes **passed on their first execution** in PR #18's `templates` CI job
(2026-07-20 — auth 51s, api-publisher 1m09s, worker-go 1m24s; the
authoring environment had no docker daemon, so CI was the first run).
Still pending for EXP-81 proper: the deliberately-broken-copy
demonstration (a sabotaged copy must FAIL its bootstrap test) + the
write-up.

### D3 — EXP-82 (Helm exercise) — chart ready, exercise not run
`templates/worker-go/chart/` lints and is reconciled against the extracted
code, but the exercise is *using* it: `helm install` on kind with
overridden values, clean uninstall, kustomize-vs-helm write-up. Needs a
kind cluster (same docker gap).

### D4 — Precondition debt: v4's EXP-40..45 are authored, not live-run
The phase brief gates extraction on "v4 merged and proven". v4 is merged
(`a4c4fa1` + hotfix #15) but its live-run validation is still open in
`v4-DEFERRED.md`. Extraction proceeded on owner instruction (2026-07-19
session) with **honest citations**: every template README marks
v4-proven-by claims as "authored; live run pending". When EXP-40..45 pass,
re-check the extracted surfaces against any fixes the runs force, then
drop the pending markers. Until then the templates are
code-complete-and-reviewed, not experiment-beaten.

### D5 — Branch-base skew: extraction source was `main`, base is `phase/v8`
`phase/v8`'s own tree predates the v4 execution merge, so extraction
copied from `origin/main` (post-v4, `07bd8bd`) — the templates are
self-contained and do not depend on the stale in-branch service trees.
When the stack is reconciled (v7 session's PR #16 lands and the stack
rebases onto main), verify the in-branch service trees match what the
templates cite; no template change is expected.

### D6 — Tag `lab-v8.0` — deferred by rule
A phase is tagged only after its exit experiments pass (D1–D3) and the
phase doc status flips. Do not tag at merge of the extraction PR.

## Closure order
D2 (CI green) → D3 (kind) → D4 (v4 runs) → D1 (the clock) → write-ups in
`documentation/experiments/` → phase doc status → D6 tag.
