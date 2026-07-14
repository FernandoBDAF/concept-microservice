# Durable queue ≠ persistent message: broker restart silently dropped the backlog

**Date:** 2026-07-10 · **Experiment:** EXP-09 (broker outage) · **Severity:**
simulator defect + a durability lesson worth keeping · **Status:** fixed
(`scripts/simulate/publish.py` now sets `delivery_mode: 2`), re-verified.

## What happened

EXP-09 step 1 builds a 50-message backlog in `email-processing` (worker
stopped, `publish.py flood`), then bounces the broker. Expected: "the
50-message backlog survived the broker restart (durable queues) and drains."

Observed: the queue itself came back (durable metadata recovered) but with
**0 messages**, before any consumer was started. All 50 messages were lost in
the restart.

## Root cause

AMQP durability is two independent flags:

- **Queue durability** — the queue *definition* survives a broker restart.
- **Message persistence** (`delivery_mode: 2`) — the message *contents* are
  written to disk and reloaded on restart.

`publish.py` publishes through the RabbitMQ management HTTP API with
`properties: {"content_type": "application/json"}` — no `delivery_mode`, so
messages default to transient (mode 1). A clean `docker compose stop
rabbitmq` (SIGTERM) discards transient messages even from durable queues.
That is documented RabbitMQ behavior, not a bug in the broker.

The real publishers were never affected: both the api-service Go client
(`internal/infrastructure/rabbitmq/client.go:263`) and the worker publisher
(`internal/common/queue/publisher.go:96`) set `amqp.Persistent`. Production
paths keep their durability promise; only the simulator lied.

Why the catalog's calibration missed it: EXP-06/07 (worker outages) exercise
backlogs without restarting the *broker*, and the v1.1 enabler tests bounced
workers, not RabbitMQ. The first broker bounce with a synthetic backlog was
this exit run — exactly the kind of gap the "run the whole catalog in order"
rule exists to close.

## Fix and verification

`publish.py` now sends `"delivery_mode": 2` on every publish (flood and
poison). Re-run of the EXP-09 tail:

```
backlog before broker restart: 50
backlog after broker restart:  50
drained ✔ (persistent backlog survived)
```

## What to keep from this

- When testing durability claims, the *producer* is part of the claim. A
  test harness that cuts corners (transient publishes, auto-ack consumes)
  can silently validate the wrong property.
- The x-death/TTL drills (EXP-10) and this one bracket the two ways work
  vanishes without an error: expiry and transiency. Both are invisible at
  publish time.
- v4's YAML experiments should assert `delivery_mode` in their publish
  helpers so the property is load-bearing, not conventional.
