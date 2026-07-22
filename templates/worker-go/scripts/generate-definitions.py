#!/usr/bin/env python3
"""Generate a broker-owned RabbitMQ topology fragment for a worker (ADR-008.4).

TEMPLATE COPY. Adapted from the lab's scripts/rabbitmq/generate-definitions.py:
the lab script hardcodes its whole pipeline list; this copy takes the pipeline
on the command line so a consuming repo can generate the topology for ONE (or a
few) new queue(s) and merge the result into its own definitions.json.

For each --pipeline NAME:RK it emits, per ADR-008.1/.3, the exact queue-argument
conventions the Go consumer (internal/common/queue) depends on:

  exchanges: <NAME>-tasks, <NAME>-tasks.retry, <NAME>-tasks.dlx
  work queue <NAME>-processing   -> x-dead-letter-exchange <NAME>-tasks.dlx
                                    x-dead-letter-routing-key <RK>
  retry wait-queues <NAME>-processing.retry.{5s,30s,2m}
                                 -> x-message-ttl {5s,30s,2m}
                                    x-dead-letter-exchange <NAME>-tasks
                                    x-dead-letter-routing-key <RK>
  DLQ <NAME>-processing.dlq      -> x-message-ttl <dlq-ttl>
  bindings wiring all of the above, plus the shared task-results loop.

The retry tiers (5s -> 30s -> 2m) MUST stay in lockstep with
internal/common/queue/retry.go's RetryTiers — do not change one without the
other (broker args are destructive to change after messages are in flight).

Examples:
  # print a mergeable fragment {exchanges,queues,bindings} to stdout
  python3 scripts/generate-definitions.py --pipeline example:example.task

  # a complete, load_definitions-ready definitions.json (adds vhost + guest user)
  python3 scripts/generate-definitions.py --pipeline example:example.task \\
      --full -o deploy/rabbitmq/definitions.json
"""
import argparse
import base64
import hashlib
import json
import os
import sys

# Retry tiers in escalation order: (suffix, ttl_ms). MUST match
# internal/common/queue/retry.go RetryTiers = {"5s","30s","2m"}.
RETRY_TIERS = [("5s", 5_000), ("30s", 30_000), ("2m", 120_000)]

DEFAULT_DLQ_TTL_MS = 86_400_000  # 24h retention on the DLQ
GUEST_SALT = bytes.fromhex("cafebabe")  # fixed salt → reproducible --full output


def password_hash(password: str, salt: bytes | None = None) -> str:
    """RabbitMQ ``rabbit_password_hashing_sha256`` hash for *password*.

    Algorithm: ``base64(salt + sha256(salt + utf8(password)))`` with a 4-byte
    salt. Pass an explicit *salt* for a reproducible value; ``None`` draws a
    fresh 4-byte random salt.

    >>> password_hash("guest", bytes.fromhex("cafebabe"))
    'yv66vmXEgsNmsRvzj9HEqCuJmRcNVMyyo7lqSeHce+VyRjxH'
    """
    if salt is None:
        salt = os.urandom(4)
    return base64.b64encode(salt + hashlib.sha256(salt + password.encode("utf-8")).digest()).decode()


def parse_pipeline(spec: str):
    """Parse a NAME:RK pipeline spec into (name, rk, exchange, queue, consumer).

    NAME is the neutral service name (letters/digits/hyphens); RK is the AMQP
    routing key. exchange/queue/consumer are derived by the lab's naming
    convention: <name>-tasks / <name>-processing / <name>-worker.
    """
    if ":" not in spec:
        raise ValueError(f"pipeline {spec!r} must be NAME:RK (e.g. example:example.task)")
    name, rk = spec.split(":", 1)
    name, rk = name.strip(), rk.strip()
    if not name or not rk:
        raise ValueError(f"pipeline {spec!r} must be NAME:RK with both parts non-empty")
    return {
        "name": name,
        "rk": rk,
        "exchange": f"{name}-tasks",
        "queue": f"{name}-processing",
        "consumer": f"{name}-worker",
    }


def build_topology(pipelines, dlq_ttl_ms):
    """Build the {exchanges, queues, bindings} fragment for the pipelines."""
    exchanges, queues, bindings = [], [], []
    seen_ex = set()

    def exchange(name):
        if name in seen_ex:
            return
        seen_ex.add(name)
        exchanges.append({"name": name, "vhost": "/", "type": "direct",
                          "durable": True, "auto_delete": False,
                          "internal": False, "arguments": {}})

    def queue(name, args):
        queues.append({"name": name, "vhost": "/", "durable": True,
                       "auto_delete": False, "arguments": args})

    def bind(src, dest, rk):
        bindings.append({"source": src, "vhost": "/", "destination": dest,
                         "destination_type": "queue", "routing_key": rk,
                         "arguments": {}})

    for p in pipelines:
        ex, q, rk = p["exchange"], p["queue"], p["rk"]
        exchange(ex)
        exchange(f"{ex}.dlx")
        exchange(f"{ex}.retry")

        # Work queue: consumers publish poison to the DLX explicitly, so the
        # queue's own dead-letter path (rk unchanged) fires only for legacy
        # nacks / TTL expiry (ADR-008.5). No x-message-ttl by default.
        queue(q, {
            "x-dead-letter-exchange": f"{ex}.dlx",
            "x-dead-letter-routing-key": rk,
        })
        bind(ex, q, rk)

        # Retry wait-queues: TTL per tier, dead-letter back to the main exchange.
        for tier, ttl in RETRY_TIERS:
            rq = f"{q}.retry.{tier}"
            queue(rq, {
                "x-message-ttl": ttl,
                "x-dead-letter-exchange": ex,
                "x-dead-letter-routing-key": rk,
            })
            bind(f"{ex}.retry", rq, f"{rk}.retry.{tier}")

        # DLQ (poison terminus).
        queue(f"{q}.dlq", {"x-message-ttl": dlq_ttl_ms})
        bind(f"{ex}.dlx", f"{q}.dlq", rk)

    # Shared completion/failure feedback loop (ADR-008.3): every worker built
    # from this template publishes task.result here; add it once.
    exchange("task-results")
    queue("task-results", {})
    bind("task-results", "task-results", "task.result")

    return {"exchanges": exchanges, "queues": queues, "bindings": bindings}


def wrap_full(fragment, password):
    """Wrap a fragment in a complete, load_definitions-ready envelope.

    load_definitions at boot skips default-user creation, so the guest user
    must be declared here (lab-only credential, matches the compose snippet).
    """
    return {
        "rabbit_version": "3.12.0",
        "vhosts": [{"name": "/"}],
        "users": [{"name": "guest",
                   "password_hash": password_hash(
                       password, GUEST_SALT if password == "guest" else None),
                   "hashing_algorithm": "rabbit_password_hashing_sha256",
                   "tags": ["administrator"]}],
        "permissions": [{"user": "guest", "vhost": "/",
                         "configure": ".*", "write": ".*", "read": ".*"}],
        "topic_permissions": [],
        "parameters": [],
        "global_parameters": [],
        "policies": [],
        **fragment,
    }


def main(argv=None):
    ap = argparse.ArgumentParser(
        description="Generate a RabbitMQ topology fragment for a worker queue "
                    "(ADR-008.4). Prints a mergeable {exchanges,queues,bindings} "
                    "object by default; --full wraps it in a loadable "
                    "definitions.json.")
    ap.add_argument(
        "--pipeline", action="append", metavar="NAME:RK", required=True,
        help="pipeline to generate, NAME:RK (e.g. example:example.task). "
             "Repeatable. Derives exchange <NAME>-tasks, queue <NAME>-processing, "
             "consumer <NAME>-worker.")
    ap.add_argument(
        "--dlq-ttl-hours", type=int, default=24,
        help="retention (hours) of the DLQ x-message-ttl (default: 24).")
    ap.add_argument(
        "--full", action="store_true",
        help="emit a complete definitions.json (adds vhost + guest user) instead "
             "of just the mergeable topology fragment.")
    ap.add_argument(
        "--password", default="guest",
        help="password hashed into the guest user for --full output "
             "(default: guest — the compose credential; reproducible fixed salt).")
    ap.add_argument(
        "-o", "--out", metavar="PATH",
        help="write to PATH instead of stdout.")
    args = ap.parse_args(argv)

    try:
        pipelines = [parse_pipeline(s) for s in args.pipeline]
    except ValueError as e:
        ap.error(str(e))

    fragment = build_topology(pipelines, args.dlq_ttl_hours * 3_600_000)
    out = wrap_full(fragment, args.password) if args.full else fragment
    text = json.dumps(out, indent=2, sort_keys=False) + "\n"

    if args.out:
        os.makedirs(os.path.dirname(os.path.abspath(args.out)), exist_ok=True)
        with open(args.out, "w") as f:
            f.write(text)
        counts = (f"{len(fragment['exchanges'])} exchanges, "
                  f"{len(fragment['queues'])} queues, "
                  f"{len(fragment['bindings'])} bindings")
        print(f"wrote {args.out}: {counts}", file=sys.stderr)
    else:
        sys.stdout.write(text)


if __name__ == "__main__":
    main()
