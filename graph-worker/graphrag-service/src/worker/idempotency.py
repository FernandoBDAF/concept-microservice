"""Consumer dedupe guard (ADR-008.2), the Python mirror of the Go
`operational-workers/internal/common/idempotency` package.

First delivery of an envelope id wins; redeliveries within the TTL window
are skipped (ack without reprocessing). The guard is a *net, not a lock*:
on a Redis error the caller FAILS OPEN (processes anyway) and counts a
metric, because dropping a message on an infra hiccup is worse than a rare
double-process (which the downstream upserts / task-result dedupe absorb).

Key shape is byte-for-byte identical to the Go guard (guard.go). The
consumer uses the ATTEMPT-scoped key ``idem:<queue>:<envelope-id>:<attempt>``
(key_for_attempt), where attempt is the x-death retry count: a genuine
same-attempt redelivery (crash between process and ack) dedupes, but an
intentional retry (ADR-008.1 republish bumps the attempt) is admitted and
reprocessed — a plain key would ack retries unprocessed and lose the message.
"""

import logging
import time
from typing import Dict, Optional, Protocol

logger = logging.getLogger(__name__)

# 24h dedupe window, matching Go's idempotency.DefaultTTL. RabbitMQ
# redelivery is seconds/minutes away; 24h also covers operator replays.
DEFAULT_TTL_SECONDS = 24 * 60 * 60


def key_for(queue: str, envelope_id: str) -> str:
    """Namespace envelope ids per queue. Mirrors Go idempotency.KeyFor exactly."""
    return "idem:" + queue + ":" + envelope_id


def key_for_attempt(queue: str, envelope_id: str, attempt: int) -> str:
    """Scope key_for by the retry attempt (x-death count).

    Mirrors Go idempotency.KeyForAttempt byte-for-byte
    (KeyFor(...) + ":" + strconv.Itoa(attempt)): shape
    ``idem:<queue>:<envelope-id>:<attempt>``. Deduping is thus per-attempt, so
    the 5s/30s/2m retry passes (each a higher attempt) are reprocessed rather
    than misclassified as duplicates, while same-attempt redeliveries still
    dedupe. `attempt` must be the same value used for retry-tier selection
    (retry.death_count over the "<queue>.retry." prefix).
    """
    return key_for(queue, envelope_id) + ":" + str(attempt)


class Guard(Protocol):
    """The seam the consumer calls around processing.

    begin(key, ttl) -> True  == first time we've seen this key (process it)
                     -> False == duplicate within the TTL window (skip + ack)
                     -> raises == infra failure; caller fails OPEN and counts it
    """

    async def begin(self, key: str, ttl_seconds: int) -> bool: ...


class InMemoryGuard:
    """Process-local guard for single-replica dev / tests / no-Redis fallback.

    Mirrors Go's InMemoryGuard: a map of key -> expiry. Only correct within a
    single process (a second replica would not see these keys) — hence the
    loud warning when it is selected in place of Redis.
    """

    def __init__(self) -> None:
        self._seen: Dict[str, float] = {}

    async def begin(self, key: str, ttl_seconds: int) -> bool:
        now = time.monotonic()
        exp = self._seen.get(key)
        if exp is not None and now < exp:
            return False
        self._seen[key] = now + ttl_seconds
        return True


class RedisGuard:
    """Production guard: a single atomic SETNX with TTL against Redis.

    `client` is a redis.asyncio.Redis. `SET key 1 NX EX ttl` returns True when
    the key was created (we own this envelope) and None when it already existed
    (duplicate). Any exception propagates so the consumer can fail open.
    """

    def __init__(self, client: "object") -> None:
        self._client = client

    async def begin(self, key: str, ttl_seconds: int) -> bool:
        result = await self._client.set(key, 1, nx=True, ex=ttl_seconds)
        return bool(result)


def build_guard(redis_addr: Optional[str]) -> Guard:
    """Select a guard from REDIS_ADDR (host:port).

    Unset/empty -> InMemoryGuard + warning (single-replica only). If the redis
    library is not installed, also fall back to in-memory with a warning
    (compileall must still pass without the dependency present locally).
    """
    if not redis_addr:
        logger.warning(
            "REDIS_ADDR not set; using in-process idempotency guard "
            "(single-replica only, dedupe does not span worker replicas)"
        )
        return InMemoryGuard()

    try:
        import redis.asyncio as redis_asyncio
    except ImportError:
        logger.warning(
            "REDIS_ADDR=%s set but the redis library is not installed; "
            "falling back to in-process idempotency guard",
            redis_addr,
        )
        return InMemoryGuard()

    host, _, port = redis_addr.partition(":")
    client = redis_asyncio.Redis(
        host=host or "redis",
        port=int(port) if port else 6379,
        socket_timeout=2.0,
        socket_connect_timeout=2.0,
    )
    logger.info("Using Redis idempotency guard", extra={"redis_addr": redis_addr})
    return RedisGuard(client)
