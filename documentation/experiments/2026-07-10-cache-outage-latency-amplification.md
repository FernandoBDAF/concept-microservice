# Cache outage: correct fallback, silent ~70–600× latency amplification

**Date:** 2026-07-10 · **Experiment:** EXP-12 (discovery — first time run)
· **Status:** documented; fail-fast candidate noted for v4 hardening.

## The question EXP-12 asks

Should the API serve from Postgres when Redis is down (cache-aside says
yes)? Does it?

## Answer: yes, and yes — but at a price nothing reports

With a profile cached and baseline latencies measured, `docker compose stop
redis`, then probe:

| Probe | Redis up | Redis down |
|---|---|---|
| `GET /profiles/:id` (first) | 200 · 6.6 ms | 200 · **3 985 ms** |
| `GET /profiles/:id` (subsequent) | 200 · ~7 ms | 200 · **330–465 ms** |
| `GET /profiles?page=1` | 200 · 25 ms | 200 · **2 961 ms** |
| `POST /profiles` | 201 · ~30 ms | 201 · 331 ms |
| `/ready` | all ok | `redis: down`, status `degraded` ✓ |

So the cache-aside design holds: no user-visible errors, reads fall through
to Postgres, writes land. But every request first burns a go-redis dial
sequence ("failed to dial after 5 attempts") before falling back — the first
request after the outage pays ~4 s, later ones a few hundred ms as the pool
keeps re-probing. Under EXP-03-style load this is a p95 explosion with a
**0% error rate**: a dashboard watching errors sees a healthy system getting
mysteriously slow.

Two observability wrinkles:

1. **The go-redis client logs bypass the structured logger.** The dial
   failures print in go-redis's own plain-text format to stderr
   (`redis: connection pool: failed to dial…`), invisible to anything
   parsing the zap JSON stream. The app itself logs *nothing* at
   error/warn level during the outage.
2. **`/ready` is the only honest signal** (`redis: down`, `degraded`) —
   which validates the readiness design, but nothing scrapes readiness into
   metrics today (v3 blackbox/kube-state work will).

## Recovery

`docker compose start redis` → next request 200 in ~207 ms, back to
single-digit ms after the pool re-establishes. No API restart needed. All 68
`profile:*` keys survived the stop/start (RDB save on SIGTERM + container
filesystem intact) — a compose-specific comfort that won't hold for
`nuke` (no redis volume is declared) or for pod rescheduling in v2+.

## What to change (fed to CONCEPTUAL_REVIEW §12)

- **Fail fast when the cache is known-dead:** a short dial timeout + a
  cache-path breaker (or go-redis's `MaxRetries=0` + tight `DialTimeout`
  when a health probe says down) would turn 3–4 s penalties into
  microsecond skips. Candidate for v4 hardening alongside retry queues.
- **Route go-redis logging through zap** (`redis.SetLogger`) so cache
  degradation is visible in the same stream as everything else.
- **Latency panels, not just error panels**, are the tell for this class of
  failure — worth remembering when the v3 SLO baselines get defined
  (latency SLI would page here; error SLI never would).
