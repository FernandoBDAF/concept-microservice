# Mission Control (v6)

The lab's cockpit: one browser surface to **control and see the whole lab** —
launch/stop systems, scale components, run experiments from the library with
their dashboards beside them, across compose / kind / AWS targets, a full
practice session without touching a terminal. It grows from the v3 thin status
page (ADR-001.3) and wraps `make` — it never replaces it (ADR-005.2: make is
the single source of truth).

> **Status (2026-07-19):** control plane + cockpit landed and unit-tested;
> exit runs EXP-60..63 are deferred — see
> [documentation/phases/v6-DEFERRED.md](../documentation/phases/v6-DEFERRED.md).

## Architecture

```
browser (status-page, Next.js)
    │  GET status/systems/experiments/runs · POST actions/outcomes/sessions
    │  SSE  /api/actions/{id}/stream
    ▼
lab-controld (Go, 127.0.0.1:4900)
    │  resolveCommand: request → registry command (the ONLY path to exec)
    │  exec.CommandContext("sh","-c", cmd)  from repo root
    ▼
make targets  ·  scripts/experiments/run.py  ·  kubectl (guest verbs)
```

The daemon holds **no logic of its own** about how to run the lab: every
action is a command drawn verbatim from the systems registry
(`systems/*.yaml`), which doubles as the action whitelist — nothing outside it
is ever invokable (ADR-005.2). The UI shows the resolved `make` command next
to the live stream for every action (the teaching surface).

| Component | Path | Port | Stack |
|---|---|---|---|
| `controld` | `mission-control/controld/` | `127.0.0.1:4900` | Go 1.24, stdlib + `gopkg.in/yaml.v3` |
| `status-page` | `mission-control/status-page/` | `127.0.0.1:4901` | Next.js (App Router, TypeScript), plain CSS |

The systems model is defined in [systems/README.md](../systems/README.md)
(schema v0); `lab.yaml` and `hello-guest.yaml` ship. Guests onboard by
dropping a YAML file there — v7's real guests become config, not code.

## API

Read endpoints (from v3, shapes unchanged):

| Method · Path | Returns |
|---|---|
| `GET /healthz` | liveness, plain `ok` |
| `GET /api/targets` | `[{name, available}]` for `compose`, `kind`, `aws`; probed + cached ~5 s. `aws` is stubbed `available:false` ("session check pending v5 integration") until v5 lands |
| `GET /api/status?target=` | per-service `{name,state,health,image}` (compose) / workload `{namespace,name,ready,status}` (kind) |
| `GET /api/health?target=` | HTTP health probes (compose) / pod-readiness derived (kind) |
| `GET /api/links?target=` | static per-target deep-links map |

Control endpoints (v6):

| Method · Path | Purpose |
|---|---|
| `GET /api/systems` | the parsed registry (`[]System`); reloaded on SIGHUP |
| `POST /api/actions` | start an action; `202 {id}`. Body: `{system,target,verb,params}` |
| `GET /api/actions/{id}` | `ActionRecord` — `state` (pending/running/succeeded/failed) + `exit_code` |
| `GET /api/actions/{id}/stream` | **SSE**: `event: line` per stdout/stderr line, `event: end` on completion |
| `GET /api/runs?limit=` | run history from the JSONL log (no DB) |

Wave-2 endpoints (experiments + sessions):

| Method · Path | Purpose |
|---|---|
| `GET /api/experiments` | the scored catalog, parsed from `experiments/*.yaml` |
| `POST /api/experiments/{id}/outcome` | append a structured entry + free notes under `documentation/experiments/` |
| `POST /api/sessions` | open a practice session `{title}` → `{id}` |
| `PATCH /api/sessions/{id}` | attach a note (actions/experiments auto-attach) |
| `GET /api/sessions/{id}/summary` | render a paste-ready markdown write-up (timeline, exit codes, outcomes, notes) |

**Verbs** (`ActionRequest.verb`): `up · down · status · scale · experiment`.
Placeholders are strictly validated before any value reaches the shell — `n`
is an integer 1..10, the experiment id matches `^exp-[a-z0-9-]+$`; no other
user input is ever passed to `sh -c`. **Destructive verbs** (`down`, resets)
require `params.confirm="true"`. The `experiment` verb runs
`make experiment E=<id>` — the runner's own exit code is the pass/fail.

**Concurrency:** one running action per `(system,target)` — a second request
gets `409`. **Timeouts** are per verb (aws `up`: 30 min; other `up`: 10 min;
`status`: 60 s). A non-zero make exit ends the action `state:failed` with the
code — a failing command surfaces as a failed action, never a silent success
(EXP-61). Every shell-out merges stdout+stderr line-scanned into a bounded
ring (last 2000 lines) for late SSE subscribers; logs are structured JSON on
stdout (`slog`).

## Auth modes (ADR-005.4)

| Mode | Trigger | Requirement |
|---|---|---|
| Localhost (default) | `127.0.0.1` bind, `CONTROLD_TOKEN` unset | none — zero friction locally |
| Token | `CONTROLD_TOKEN` set | `Authorization: Bearer <token>` on `/api/*`; SSE uses `?token=` (EventSource can't set headers). Wrong/missing → `401` + audit log line |
| AWS-enabled | `CONTROLD_ENABLE_AWS=1` | **token + TLS both required at startup** (`CONTROLD_TLS_CERT`/`CONTROLD_TLS_KEY`) or the daemon refuses to boot |

The localhost bind never listens off-loopback, so a remote connection is
refused outright (EXP-63). The hard gate exists because an AWS-triggering
control plane cannot stay open.

## Run

```
make controld       # go run the daemon on 127.0.0.1:4900
make status-page    # next dev on 127.0.0.1:4901
```

Then open http://127.0.0.1:4901. Both targets down is a normal state — the
cockpit shows "target unavailable" until compose or kind comes up.

Environment overrides:

| Var | Default | Meaning |
|---|---|---|
| `CONTROLD_ADDR` (or `-addr`) | `127.0.0.1:4900` | listen address — keep it on localhost |
| `CONTROLD_REPO_ROOT` | the checkout root | working directory for every `sh -c` invocation |
| `NEXT_PUBLIC_CONTROLD_URL` | `http://127.0.0.1:4900` | where the page reaches controld |
| `CONTROLD_TOKEN` | unset | when set, Bearer auth required on `/api/*` |
| `CONTROLD_ENABLE_AWS` | `0` | `1` enables the aws target — forces token + TLS |
| `CONTROLD_TLS_CERT` / `CONTROLD_TLS_KEY` | unset | TLS cert/key; required with `CONTROLD_ENABLE_AWS=1` |

## Run history (`runs/`)

Action history persists as **JSON lines, one file per day**, at
`mission-control/controld/runs/YYYY-MM-DD.jsonl` — no database (ADR-005.2).
The directory is gitignored; losing it loses history, not state (the live
target is always the truth). Each line is an `ActionRecord`:

```json
{"id":"…","request":{"system":"lab","target":"kind","verb":"up","params":{}},
 "command":"make cluster-up","state":"succeeded","exit_code":0,
 "started_at":"…","ended_at":"…"}
```

`GET /api/runs?limit=` reads back from these files; the `command` field is the
spot-check that every UI action delegated to make.

## See also

- [systems/README.md](../systems/README.md) — the registry schema (v0) and
  the two shipped system definitions.
- [documentation/phases/v6-mission-control.md](../documentation/phases/v6-mission-control.md)
  — the phase brief (mission, work breakdown, acceptance).
- [documentation/phases/v6-DEFERRED.md](../documentation/phases/v6-DEFERRED.md)
  — the honest ledger of what has not run yet and the seams on parallel v4/v5
  work; read it before tagging `lab-v6.0`.
