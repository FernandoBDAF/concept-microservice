"""Retry-tier selection from RabbitMQ x-death headers (ADR-008.1).

Pure functions, no I/O and no aio-pika dependency, so they are unit-testable
in isolation (tests/test_retry.py) and safe to import anywhere.

Retry model (matches deploy/rabbitmq/definitions.json for document-tasks):
a failed message is republished to the retry exchange with routing key
`<rk>.retry.<tier>`; the tier's wait-queue holds it for a TTL, then
dead-letters it back to the main exchange. Each pass through a wait-queue
adds/increments an `x-death` entry whose `queue` is that wait-queue. The
number of retries already taken is therefore the summed `count` of the
x-death entries whose queue starts with `<main-queue>.retry.`.

    death count -> action
    0           -> retry tier "5s"   (retry.5s wait-queue, 5s TTL)
    1           -> retry tier "30s"  (retry.30s wait-queue, 30s TTL)
    2           -> retry tier "2m"   (retry.2m wait-queue, 120s TTL)
    >= 3        -> exhausted -> dead-letter exchange (poison)
"""

from typing import Any, Dict, List, Optional

# Tier labels, indexed by the number of retries already taken. The labels
# are the suffixes used in both the routing key (`<rk>.retry.<label>`) and
# the wait-queue name (`<queue>.retry.<label>`) in definitions.json.
RETRY_TIERS: List[str] = ["5s", "30s", "2m"]

# After this many retries a message is considered poison and dead-lettered.
MAX_RETRIES: int = len(RETRY_TIERS)  # 3


def select_tier(death_count: int) -> Optional[str]:
    """Return the retry-tier label for the next attempt, or None if exhausted.

    `death_count` is the number of retries already taken (see death_count()).
    Returns None once the message has been through every tier (>= MAX_RETRIES),
    signalling the caller to dead-letter it instead of retrying.
    """
    if death_count < 0:
        death_count = 0
    if death_count >= MAX_RETRIES:
        return None
    return RETRY_TIERS[death_count]


def death_count(headers: Optional[Dict[str, Any]], queue_prefix: str) -> int:
    """Sum the `count` of x-death entries whose queue starts with queue_prefix.

    `headers` is the AMQP header table as aio-pika exposes it
    (message.headers); the `x-death` value, when present, is a list of
    tables (dicts). `queue_prefix` should be `"<main-queue>.retry."` so that
    only the timed retry wait-queues are counted (not the main queue's own
    dead-letter entry). Missing/malformed headers count as zero retries.
    """
    if not headers:
        return 0
    x_death = headers.get("x-death")
    if not isinstance(x_death, (list, tuple)):
        return 0

    total = 0
    for entry in x_death:
        if not isinstance(entry, dict):
            continue
        queue = entry.get("queue")
        if isinstance(queue, bytes):
            queue = queue.decode("utf-8", errors="replace")
        if not isinstance(queue, str) or not queue.startswith(queue_prefix):
            continue
        count = entry.get("count", 0)
        try:
            total += int(count)
        except (TypeError, ValueError):
            continue
    return total
