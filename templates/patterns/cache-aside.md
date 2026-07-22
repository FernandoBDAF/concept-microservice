# Pattern: Postgres-backed cache-aside with Redis

A read-through/write-around cache in front of a relational store, the way
`api-service` does it for profiles. The pattern is boring on the happy path;
its value here is that the lab has *measured* how it fails, so this doc leads
with the failure behavior a copying project must plan for.

Evidence is cited by experiment write-up filename and by lab code path. Every
concrete value below (key strings, timeouts, TTLs) was read from the
POST-v4 tree; adapt them, don't copy them blind.

## Context — when to use it

Reach for cache-aside when reads dominate, the authoritative data lives in a
SQL store, and staleness on the order of minutes is acceptable. The store
stays the source of truth; the cache is a disposable accelerator you can flush
at any time. If you cannot tolerate serving a value that is up to one TTL
stale, cache-aside is the wrong pattern.

## The pattern as the lab implements it

Read path (`api-service/internal/domain/profile/service.go`, `GetByID`/`List`):

1. Look up the key in Redis. Hit → decode and return.
2. Miss (or *any* cache error) → read Postgres, then best-effort `SetProfile`
   back into Redis, then return. Cache errors are swallowed and logged as
   ignorable — a read never fails because the cache is unavailable.

Write path (`Create`/`Update`/`Delete` in the same file):

- **Create/Update** write Postgres, then write-through the single key
  (`SetProfile`) *and* invalidate every list page.
- **Delete** writes Postgres, then deletes the single key and invalidates
  every list page.

Key naming (`api-service/internal/infrastructure/redis/cache.go`):

- Single entity: `profile:<uuid>` — literally `fmt.Sprintf("profile:%s", id)`.
- List pages: `profiles:list:<page>` — keyed by page number only (page size
  is **not** in the key; if you vary page size at runtime, add it to the key).
- Bulk list invalidation: `DeleteByPattern(ctx, "profiles:list:*")` via a
  Redis `SCAN` loop (`client.go`).

Invalidation discipline that matters for a copier:

- A single-entity write always fans out to invalidate *all* list pages,
  because any page could contain the changed row. This is `SCAN`-based and
  `O(keyspace)` — fine at lab scale, named as a known non-scaler in
  `documentation/review/CONCEPTUAL_REVIEW.md` §9. At high key counts, switch
  to versioned list keys (`profiles:list:v<n>:<page>` + bump a version
  counter) so invalidation is a single `INCR` instead of a scan.
- Create/Update are write-*through* on the single key (they store the fresh
  value); Delete is delete-on-write. Both are correct; write-through just
  saves the next reader a miss.

TTLs (defaults in `api-service/internal/config/config.go`):

- `cache.profile_ttl` = **15m**, `cache.list_ttl` = **5m**. TTL is the
  backstop that bounds staleness if an invalidation is ever missed — keep list
  TTL short precisely because list invalidation is the expensive path.

## Failure modes experiments found

### EXP-12 — cache outage is silent latency amplification, 0% errors

Source: `documentation/experiments/2026-07-10-cache-outage-latency-amplification.md`
(EXP-12, discovery run). With a profile cached and Redis then stopped
(`docker compose stop redis`), the measured penalty was:

| Probe | Redis up | Redis down |
|---|---|---|
| `GET /profiles/:id` (first) | 200 · 6.6 ms | 200 · **3 985 ms** |
| `GET /profiles/:id` (subsequent) | 200 · ~7 ms | 200 · **330–465 ms** |
| `GET /profiles?page=1` | 200 · 25 ms | 200 · **2 961 ms** |
| `POST /profiles` | 201 · ~30 ms | 201 · 331 ms |
| `/ready` | all ok | `redis: down`, status `degraded` ✓ |

The cache-aside contract held: no user-visible errors, reads fell through to
Postgres, writes landed. But every request first burned a go-redis dial
sequence ("failed to dial after 5 attempts") before falling back — the first
request after the outage paid ~4 s, later ones a few hundred ms as the pool
kept re-probing. As the write-up puts it: under load this is "a p95 explosion
with a **0% error rate**: a dashboard watching errors sees a healthy system
getting mysteriously slow."

Two observability wrinkles the write-up records, both still true in this tree:

1. **go-redis dial-failure logs bypass the structured (zap) logger** — they
   print in go-redis's own plain-text format to stderr, invisible to anything
   parsing the JSON stream; the app logs nothing at warn/error during the
   outage. Verified: no `redis.SetLogger`/zap wiring exists in `api-service`
   today, so the EXP-12 wrinkle is unfixed here.
2. **`/ready` is the only honest signal.** `api-service`'s readiness handler
   (`internal/api/handlers/health.go`) ANDs Redis into readiness: Redis down →
   HTTP 503 with `checks.redis = "down"` and overall `status: "degraded"`.
   Nothing scrapes readiness into metrics in the lab yet.

Recovery is clean: `docker compose start redis` → next request ~207 ms, back
to single-digit ms once the pool re-establishes. No app restart needed.

### Readiness coupling is a double-edged sword

Because `/ready` ANDs the cache (and broker) in, a dependency outage makes the
pod NotReady in Kubernetes, which pulls it from Service endpoints — see
`documentation/experiments/2026-07-10-v2-cluster-exit-runs.md` (EXP-20/23) for
the broker-outage version of this (both api pods NotReady → nginx 503s). The
readiness signal is honest, but coupling a *degradable* dependency (a cache
you can serve without) into readiness converts "slow" into "no endpoints."
Decide deliberately whether the cache belongs in readiness or only in a
separate `/health/detail`.

### Stampede — not handled in the lab

There is **no singleflight / stampede protection** in the lab code
(confirmed: `golang.org/x/sync/singleflight` is not a dependency; noted as an
accepted gap in `CONCEPTUAL_REVIEW.md` §9). On a hot-key expiry under
concurrency, every in-flight miss independently hits Postgres and independently
re-populates the key. Fine at lab scale; a real hot key needs a fix. The seam
to add it: wrap the "miss → load from Postgres → SetProfile" block in
`service.go`'s `GetByID`/`List` with a `singleflight.Group.Do(key, ...)` so
only one loader runs per key while the others wait on its result.

## Mitigation menu

Pick per your latency budget; EXP-12 recommends the first two for hardening.

- **Fail fast when the cache is known-dead.** A short `DialTimeout` plus
  `MaxRetries=0` on the cache client (go-redis `redis.Options`) turns the
  multi-second dial penalty into a microsecond skip. The lab defaults are
  `dial_timeout=5s`, `read_timeout=3s`, `write_timeout=3s`, `max_retries=3`,
  `pool_size=50` (`config.go`) — those are *tuned for reliability, not
  fail-fast*, which is exactly why the outage hurt. A cache is a good place to
  bias toward fail-fast.
- **Cache-path circuit breaker.** Trip a breaker on repeated dial failures and
  skip Redis entirely while open, re-probing periodically. The lab already
  vendors `sony/gobreaker` (used for the auth client, not the cache) — the
  same primitive drops onto the cache path.
- **Route the cache client's logs through your structured logger**
  (`redis.SetLogger`) so cache degradation shows up in the same stream as
  everything else — otherwise you are blind exactly when it matters.
- **Alert on latency, not just errors.** EXP-12's core lesson: an error-rate
  SLI would never page for this; a latency (p95/p99) SLI would. If you export
  one signal, make it latency.
- **Keep `/ready` honest but scoped.** Report cache health, but think twice
  before letting a degradable cache pull the pod out of rotation.

## Adaptation checklist

- [ ] Rename the key namespace (`profile:` / `profiles:list:`) to your entity;
      keep the `<type>:<id>` and `<type>:list:<page>` shape.
- [ ] Put page size (and any other list parameter) into the list key if you
      vary it at runtime.
- [ ] Decide TTLs deliberately — short for lists (invalidation is expensive),
      longer for single entities. Lab: 15m / 5m.
- [ ] On every write, invalidate the single key **and** all affected list
      pages. If your keyspace is large, switch to versioned list keys instead
      of `SCAN`-all.
- [ ] Set the cache client to fail fast (`MaxRetries=0`, short `DialTimeout`)
      or wrap it in a breaker — do not inherit reliability-tuned timeouts on a
      disposable dependency.
- [ ] Route the cache client's internal logs into your structured logger.
- [ ] Add a latency SLI/alert, not just an error-rate one.
- [ ] Decide whether the cache belongs in `/ready` (coupling) or a separate
      detail endpoint.
- [ ] If you have hot keys, add singleflight around the miss-load-repopulate
      block before you ship.
