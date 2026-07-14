# Readiness that checks every dependency turns a broker outage into a full API outage

**Date:** 2026-07-10 · **Experiment:** EXP-20 step 6 (broker outage on kind)
· **Status:** documented; design question routed to CONCEPTUAL_REVIEW §12 /
v4 hardening.

## The compose behavior (EXP-09, v1.1 exit run)

Stop RabbitMQ under compose and the system degrades *partially*: profile
CRUD keeps returning 200 (Postgres/Redis path), task submissions fail with
5xx, `/ready` reports `degraded`. Clients talking straight to the container
port never notice the readiness endpoint at all.

## The same failure on Kubernetes

`kubectl -n lab-infra scale statefulset/rabbitmq --replicas=0`, then probe
through the ingress:

| Probe | Result |
|---|---|
| auth register (`https://auth.lab.local/v1/users`) | **201** — auth's `/ready` checks only Postgres |
| api `/ready` | 503 (correct — rabbitmq down) |
| api CRUD (`GET /api/v1/profiles`) | **503 from nginx** — no endpoints |
| api pods | both `0/1 NotReady` |

Kubernetes *acts* on readiness: both api pods fail their readinessProbe
(`/ready` checks PG+Redis+RMQ), the Service loses all endpoints, and the
ingress has nowhere to route. The 90%-healthy CRUD surface — which compose
kept serving — is now fully down. **The `/ready` semantics didn't change;
their consequences did.**

Recovery is automatic and clean: broker back → probes pass → endpoints
return (observed end-to-end, including a 50-message persistent backlog that
survived the broker pod's deletion and drained on recovery).

## Why this is a design finding, not a k8s bug

Readiness gates *traffic*. A readiness endpoint that ANDs every dependency
declares "if any dependency is down, send me nothing" — the right contract
for a pure queue consumer, but wrong for a service whose endpoints have
heterogeneous dependency sets (CRUD needs PG+Redis; only the task-submit
paths need the broker).

Options for the v4 hardening pass (pick there, not here):
1. `/ready` gates only on what *all* routes need (PG); RabbitMQ health moves
   to a metric/alert (v3 gives it a page to live on).
2. Degraded-mode readiness: stay Ready, return 503 per-route on
   broker-dependent endpoints (the app already does this today — publish
   failures 500 — so this is nearly free).
3. Keep the strict contract and accept full-stop semantics (defensible for
   a lab; indefensible for the v8 template).

The v8 API template should ship with option 1 or 2 documented — this run is
the argument.

## Also observed on the way (fixed in-tree)

- **Probe economics:** `rabbitmq-diagnostics status|check_port_connectivity`
  spawn an Erlang VM per call; under a 500m CPU limit they blow their
  timeouts and the liveness probe kill-loops the broker (5 restarts/30 min,
  19 probe timeouts — the EXP-20 smoke's 9.5% failures were this loop's
  wake). Fixed: readiness = TCP:5672 (the exact consumer-visible property),
  liveness = `rabbitmq-diagnostics -q ping` with 15 s timeout, CPU limit
  1000m. Compose never sees this class — it has no CPU limits.
- **Rolling-update blip:** endpoint propagation lags SIGTERM, so a rolling
  restart briefly routed new connections to a terminating pod (2/200 probe
  failures in EXP-22). Fixed with a 5 s `preStop` sleep on api/auth;
  re-probed 150/150.
