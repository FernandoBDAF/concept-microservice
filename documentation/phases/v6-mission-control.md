# Phase v6 — Mission Control

**Status:** code-complete — live exit runs deferred (see
[v6-DEFERRED.md](v6-DEFERRED.md)) · **Depends on:** v5 (all three
targets exist) ·
**Exit tag:** `lab-v6.0` · **Decisions in force:** ADR-005 (all), ADR-004.2
(experiment YAML), ADR-001.3 (grows from the v3 status page)

## Mission

One cockpit to **control and see the whole lab**: launch/stop systems, run
experiments from the library with their dashboards beside them, across
compose / kind / AWS targets — an entire practice session without touching a
terminal. Grows from the v3 thin status page; wraps `make` (never replaces
it).

## Context a fresh session needs

- v3 shipped the seed: a read-only Next.js status page + `lab-controld`
  read endpoints (ADR-005.1/.2). This phase makes both real.
- Experiment definitions are YAML + prose since v4 (ADR-004.2); the scored
  runner exists (`make experiment E=`). The UI *renders and invokes*, it does
  not reimplement.
- Targets and their entry points: compose (`make up/down/...`), kind
  (`make cluster-*`), AWS (`make aws-*`, session semantics per
  AWS_SESSION.md). The daemon invokes exactly these (ADR-005.2 — single
  source of truth).
- Guests: hello-guest exists with a launch definition (HOST_CONTRACT.md);
  design the "systems" model so v7's real guests are config, not code.

## Work breakdown

1. **lab-controld v1:** Go (or Node — pick once, document) daemon: REST +
   WebSocket; endpoints: targets & their status, systems catalog (lab,
   hello-guest — from launch definitions), actions (up/down/scale/experiment
   run) executed as make invocations with streamed stdout; run history
   persisted to disk (JSON lines) — no database.
2. **Systems model:** a `systems/` registry (YAML per system: name, targets
   supported, launch/stop/status commands, links, port block) — the host
   contract's machine face; lab + hello-guest entries.
3. **UI — visibility (rung 1, hardened):** target switcher (compose/kind/
   AWS-session), per-system cards (state, health, replicas), deep links
   (Grafana, Tempo, OpenSearch, RabbitMQ, MinIO/S3 console, Cost Explorer on
   AWS).
4. **UI — control (rung 2):** launch/stop systems per target, scale
   components, trigger migrations/resets; every action shows the underlying
   make command + streaming output (teaching surface); destructive actions
   confirm.
5. **UI — experiment library (rung 3):** browse the YAML catalog (goal,
   validates, needs); run guided (steps + embedded Watch dashboards via
   Grafana iframe/links + live assertion status) or scored; outcome recording
   appends to documentation/experiments/ (structured entry + free-text
   notes field).
6. **AWS target integration + auth gate (ADR-005.4):** the daemon reaching
   AWS actions requires the auth story to land first — minimum shared token
   + TLS on the daemon socket; localhost mode stays no-auth.
7. **Session recorder:** a "practice session" object grouping actions/
   experiments/notes with timestamps → renders a session summary you can
   paste into a write-up (the era-1 analyses workflow, tool-assisted).

## Out of scope

Real guests (v7 — but the systems model must accommodate them), multi-user
anything, replacing make/Grafana/CLIs, embedding loam (ADR-005.5 — patterns
only).

## Exit experiments

- **EXP-60 — Terminal-free session:** from a fresh browser: pick kind target
  → launch the lab → run EXP-04 guided (watch panels inline) → diagnose an
  induced fault (pick any v4 chaos drill) → record the outcome — zero
  terminal use. The recorded session artifact is the write-up.
- **EXP-61 — Target parity:** the same system card launches the lab on
  compose and kind, and (session up) shows correct state for AWS; actions
  stream real output; a failing make command surfaces as a failed action,
  not a silent success.
- **EXP-62 — Library round-trip:** run a scored experiment from the UI; its
  pass/fail matches `make experiment E=` for the same id; outcome lands in
  documentation/experiments/.
- **EXP-63 — Control-plane safety:** daemon bound to localhost refuses
  remote connections; with the AWS token gate enabled, a wrong token is
  rejected and logged.

## Acceptance

- [ ] EXP-60..63 pass, written up (EXP-60's session record is the showcase)
- [ ] Every UI action demonstrably delegates to make (spot-check logs)
- [ ] hello-guest launchable from the UI via its systems entry (v7 readiness)
- [ ] Tag `lab-v6.0`
