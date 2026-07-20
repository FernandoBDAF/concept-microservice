# Phase v6 handoff — Mission Control

> **Executed 2026-07-19..20 — kept as the design record.** The as-built
> differs in places: the stream gained SSE heartbeats and write timeouts;
> the aws availability check shipped as a read-only
> `terraform -chdir=deploy/aws/session output` probe, not the `aws-plan`
> cheap check named in §4; `PATCH /api/sessions/{id}` takes `{note, close}`.
> Current truth lives in
> [mission-control/README.md](../../mission-control/README.md) and the
> honest ledger in [v6-DEFERRED.md](v6-DEFERRED.md).

**Audience:** the session that finishes v6. Grow the v3 seed
(`mission-control/`) into the full cockpit; wrap `make`, never replace it
(ADR-005.2). Inherited on `phase/v6`: the systems registry
(`systems/README.md` schema + `lab.yaml` + `hello-guest.yaml`) and the
controld action contract (`mission-control/controld/actions.go` — types +
API surface are FINAL; bodies are the work).

Order: 1 → 2 → 3 → 4 (UI rungs in order) → 5 → 6 → 7.

## 1 — Registry loader
Add `gopkg.in/yaml.v3` to controld (first non-stdlib dep — acceptable,
decided). `registry.go`: load `systems/*.yaml` at startup + on SIGHUP;
validate against the schema (unknown target/verb keys are errors);
`GET /api/systems`. Unit-test with the two committed files + a broken
fixture.

## 2 — Action execution (the heart)
Implement `resolveCommand` + the exec loop per actions.go comments:
- whitelist = registry content, placeholders strictly validated
  (`n` int 1..10, experiment id regex) — no other user input ever reaches
  the shell; destructive verbs require `params.confirm=true`.
- `exec.CommandContext("sh","-c",cmd)` from repo root; merge stdout/stderr
  line-scanned into a bounded ring (last 2000 lines) + live SSE fanout;
  timeout per verb (up: 30 min for aws, 10 min else; status: 60s).
- ActionRecord JSONL append (`runs/YYYY-MM-DD.jsonl`, gitignored dir);
  `GET /api/actions/{id}`, `/stream` (SSE), `GET /api/runs?limit=`.
- Concurrency guard: one running action per (system,target) — 409 else.
- Tests: fixture registry + `sh -c 'echo ok'` fake commands; assert
  streaming, exit codes, 409, whitelist rejection (the EXP-61 "failing
  make surfaces as failed action" case: `sh -c 'exit 2'`).

## 3 — Experiment integration
`verb: experiment` runs `make experiment E=<id>` (the v4 runner; if v4
isn't merged yet, the action still works — the runner's own exit code is
the pass/fail). Parse the runner's RUNS.md append (or its junit XML) to
attach pass/fail per assertion to the ActionRecord. EXP-62 = UI result ==
CLI result for the same id.

## 4 — UI rungs (status-page grows into Mission Control)
Rung 1 (harden v3): per-system cards from `/api/systems` × `/api/status`,
target switcher incl. aws (available only when session up — `aws-plan`
cheap check).
Rung 2 (control): launch/stop/scale buttons → POST /api/actions; every
action modal shows the resolved `command` (teaching surface) + live
stream (EventSource); destructive confirm dialog.
Rung 3 (library): experiment browser reading the YAML catalog via a new
controld `GET /api/experiments` (parse `experiments/*.yaml`); guided mode
renders steps + embedded Grafana panels (iframe links from the system's
`links` map) + live assertion status; scored mode = the action from §3;
outcome recording POST → appends structured entry + free-notes to
`documentation/experiments/`.
Keep deps minimal (no UI framework beyond Next/React; charts stay in
Grafana iframes).

## 5 — Auth gate (ADR-005.4)
`CONTROLD_TOKEN` env + TLS (self-signed via the lab CA or plain
`crypto/tls` self-gen): required iff the aws target is enabled
(`CONTROLD_ENABLE_AWS=1`); localhost mode stays no-auth. Bearer check
middleware + audit log line on failures. EXP-63: remote connection refused
on localhost bind; wrong token → 401 + logged.

## 6 — Session recorder
`POST /api/sessions {title}` → open session; every subsequent action/
experiment/note (`PATCH /api/sessions/{id} {note}`) attaches; `GET
/api/sessions/{id}/summary` renders markdown (timeline, actions with exit
codes, experiment outcomes, notes) ready to paste into
`documentation/experiments/` — the era-1 write-up workflow, tool-assisted.
UI: session bar (start/stop, add note). Storage: same JSONL pattern.

## 7 — Exit
EXP-60 terminal-free session (the summary artifact IS the write-up),
EXP-61 target parity, EXP-62 library round-trip, EXP-63 safety. Spot-check
"every UI action delegates to make" via the runs log. Update phase doc
status; tag `lab-v6.0`.
