# Phase v6 — deferred validation & follow-ups

**Context:** v6 was implemented in an expedited pass (2026-07-19): the
`lab-controld` control plane (registry loader with SIGHUP reload, action
execution via `sh -c` from the repo root, SSE streaming, JSONL run history,
the one-action-per-(system,target) 409 guard, the destructive-verb confirm
gate), the ADR-005.4 auth gate, the wave-2 endpoints (experiments catalog,
outcome recording, session recorder), and the Mission Control cockpit UI all
landed and were unit-tested with fixture registries and fake (`sh -c 'echo
ok'` / `'exit 2'`) commands. But the exit experiments (EXP-60..63) need a
**running compose/kind stack and a browser**, and none ran this pass. This
file is the honest ledger of what remains before `lab-v6.0` can be tagged.

## Must run before tagging lab-v6.0

| Item | What to do | Acceptance check |
|---|---|---|
| EXP-60 terminal-free session | Fresh browser → kind target → launch lab → EXP-04 guided (panels inline) → induce a v4 chaos fault + diagnose → record the outcome, zero terminal | `GET /api/sessions/{id}/summary` renders a paste-ready write-up; it lands in `documentation/experiments/`; the runs log shows every step delegated to make (EXPERIMENTS.md §EXP-60) |
| EXP-61 target parity | Same lab card up/down on compose and kind; aws chip shows correct state with a session up; force a failing make and confirm the action fails loudly | Actions stream real stdout; a non-zero make exit → `state:failed` with the exit code, never a silent success (EXPERIMENTS.md §EXP-61) |
| EXP-62 library round-trip | Run a scored experiment from the UI, then `make experiment E=<id>` for the same id | Same pass/fail both ways; outcome appended under `documentation/experiments/` (EXPERIMENTS.md §EXP-62) |
| EXP-63 control-plane safety | Localhost-bound daemon refuses a remote connection; with `CONTROLD_ENABLE_AWS=1` + `CONTROLD_TOKEN`, hit `/api/*` with a wrong token | Remote connect refused on the localhost bind; wrong token → 401 **and** an audit log line (EXPERIMENTS.md §EXP-63) |
| "Every action delegates to make" spot-check | Drive a handful of UI actions, read `mission-control/controld/runs/YYYY-MM-DD.jsonl` | Every `ActionRecord.command` is a `make …` (or registry) invocation — no hidden shell (phase-doc acceptance) |
| hello-guest launchable from the UI | Launch hello-guest from its system card via its `systems/hello-guest.yaml` entry | Guest comes up on the target; proves the v7-readiness of the systems model (phase-doc acceptance) |

## Seams on parallel work (reconcile before the runs are meaningful)

These are honest dependencies on work that lives on other branches; they are
not defects. `phase/v6` does not yet carry the v4 execution (merged to main,
not into this stack) nor the v5 execution (on `phase/v5`, being finished in a
parallel session).

- **Scored runs surface as a failed action today — by design (v6-HANDOFF
  §3).** On this branch `scripts/experiments/run.py` is still the v4 skeleton
  (exits 3) and only `experiments/exp-02.yaml` exists. So a scored run from
  the UI runs `make experiment E=exp-02`, the runner exits non-zero, and the
  action honestly reports `state:failed` — the control path is proven even
  though the runner isn't. Real pass/fail and the RUNS.md / junit parsing that
  attaches per-assertion results to the `ActionRecord` light up when the stack
  reconciles with the v4 runner. **Check:** after reconcile, EXP-62's UI
  result == CLI result for a passing id.
- **AWS target is probe-stubbed.** `make aws-*` targets exist in
  `systems/lab.yaml`, but the availability probe returns
  `available:false` with the note "session check pending v5 integration"; the
  aws chip is present-but-disabled in the UI. Live aws parity (EXP-61's aws
  leg) and a live token+TLS drill against a real session are deferred to the
  v5 reconciliation round. **Check:** with v5 merged and a session up, the aws
  chip enables and shows correct state.
- **hello-guest cards read "unknown".** The read API tracks only the lab; a
  guest card shows state `unknown` until you run its `status` verb as an
  on-demand action ("Check status"). **Check:** the status action returns the
  guest's real state and the card updates.
- **No EXP-60..63 live run this pass.** All four need a running compose/kind
  stack and a browser; the code was exercised only against unit fixtures.
  **Check:** the table above, each row green with a write-up.

## Known caveats shipped knowingly

- **`runs/` is gitignored, per-day JSONL, no database** (ADR-005.2 — no new
  storage engine). History lives at
  `mission-control/controld/runs/YYYY-MM-DD.jsonl`; losing it loses history,
  not state — the lab's truth is always the live target.
- **Auth is off on localhost, by decision (ADR-005.4).** The default
  127.0.0.1 bind stays no-auth for zero-friction local use. Bearer auth
  engages only when `CONTROLD_TOKEN` is set; enabling the aws target
  (`CONTROLD_ENABLE_AWS=1`) *requires* a token **and** TLS
  (`CONTROLD_TLS_CERT`/`CONTROLD_TLS_KEY`) or the daemon refuses to boot —
  a hard gate before any remote reach, not a runtime warning.
- **The registry is the whole action whitelist.** Nothing outside
  `systems/*.yaml` is invokable; `{n}` is validated 1..10 and the experiment
  id against `^exp-[a-z0-9-]+$` before any placeholder reaches the shell
  (v6-HANDOFF §2). Destructive verbs (down, resets) require
  `params.confirm="true"`.
- **SSE, not WebSocket, for streaming** (actions.go): one-way stdout/stderr
  fanout, `EventSource` is enough; the SSE query-string carries `?token=` when
  auth is on (browsers can't set a Bearer header on `EventSource`).

## Nice-to-haves consciously skipped

- Embedded Grafana panels beyond iframe/deep links from each system's `links`
  map (charts stay in Grafana — v6-HANDOFF §4).
- Any UI framework beyond Next/React; no client-side charting library.
- Multi-user / RBAC anything (ADR-005.4 — single-operator local tool);
  loam integration stays patterns-only (ADR-005.5 — loam onboards as a v7
  guest system, not UI plumbing).

## Tag policy

`lab-v6.0` is tagged **only after EXP-60..63 pass and are written up** (EXP-60's
session record is the showcase). The acceptance checkboxes in
[v6-mission-control.md](v6-mission-control.md) stay unchecked until then —
they gate the tag.
