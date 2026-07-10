#!/usr/bin/env python3
"""Queue-side simulator: publish task messages straight to RabbitMQ.

Uses the management HTTP API (localhost:15672) so it needs no AMQP client —
run it from the host with a stock python3 while the compose stack is up.

Modes:
  flood   valid envelopes, one routing key       (backlog / outage drills)
  poison  malformed + invalid messages, all keys (DLQ / triage drills)

Examples:
  python3 scripts/simulate/publish.py flood --routing-key email.send --count 100
  python3 scripts/simulate/publish.py poison --count 3
"""

import argparse
import base64
import json
import sys
import time
import urllib.request
import uuid
from datetime import datetime, timezone

MGMT = "http://localhost:15672"
AUTH = base64.b64encode(b"guest:guest").decode()

# routing key -> (exchange, valid payload factory)  — per shared/contracts/
ROUTES = {
    "email.send": (
        "email-tasks",
        lambda i: {
            "email_type": "welcome",
            "recipient": f"sim-{i}@lab.dev",
            "subject": "Simulated email",
            "template_id": "welcome-template",
            "variables": {"first_name": f"Sim{i}"},
        },
    ),
    "image.process": (
        "image-tasks",
        lambda i: {
            "operation": "resize",
            "source_url": f"s3://documents-raw/sim/{i}.png",
            "target_path": f"processed/sim/{i}.png",
            "width": 512,
            "height": 512,
            "quality": 85,
            "format": "png",
        },
    ),
    "profile.task": (
        "profile-tasks",
        lambda i: {
            "task_type": "sync",
            "profile_id": f"sim-profile-{i}",
            "user_id": f"sim-user-{i}",
            "data": {"source": "publish.py"},
        },
    ),
    "document.process": (
        "document-tasks",
        lambda i: {
            "document_id": f"sim-doc-{i}",
            "storage_bucket": "documents-raw",
            "storage_path": f"sim/{i}.pdf",
            "file_type": "pdf",
            "user_id": f"sim-user-{i}",
        },
    ),
}


def envelope(routing_key: str, payload: dict) -> dict:
    return {
        "id": str(uuid.uuid4()),
        "type": routing_key,
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "payload": payload,
        "metadata": {"source": "publish.py", "trace_id": str(uuid.uuid4())},
    }


def publish(exchange: str, routing_key: str, body: str, expiration_ms: int = 0) -> bool:
    properties = {"content_type": "application/json"}
    if expiration_ms > 0:
        # AMQP per-message TTL; the value is a string of milliseconds.
        properties["expiration"] = str(expiration_ms)
    req = urllib.request.Request(
        f"{MGMT}/api/exchanges/%2F/{exchange}/publish",
        data=json.dumps(
            {
                "properties": properties,
                "routing_key": routing_key,
                "payload": body,
                "payload_encoding": "string",
            }
        ).encode(),
        headers={"Authorization": f"Basic {AUTH}", "Content-Type": "application/json"},
        method="POST",
    )
    with urllib.request.urlopen(req, timeout=10) as resp:
        return json.load(resp).get("routed", False)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("mode", choices=["flood", "poison"])
    parser.add_argument("--routing-key", choices=sorted(ROUTES), default="email.send")
    parser.add_argument("--count", type=int, default=10)
    parser.add_argument(
        "--expiration-ms",
        type=int,
        default=0,
        help="flood only: per-message TTL in ms (EXPERIMENTS.md EXP-10)",
    )
    args = parser.parse_args()

    sent = unrouted = 0
    if args.mode == "flood":
        exchange, factory = ROUTES[args.routing_key]
        for i in range(args.count):
            body = json.dumps(envelope(args.routing_key, factory(i)))
            if publish(exchange, args.routing_key, body, args.expiration_ms):
                sent += 1
            else:
                unrouted += 1
        ttl_note = f" (TTL {args.expiration_ms}ms)" if args.expiration_ms else ""
        print(f"flood: {sent} routed, {unrouted} unrouted -> {exchange}/{args.routing_key}{ttl_note}")
    else:
        # Two poison flavors per routing key:
        #  1. not JSON at all            -> consumers nack straight to the DLQ
        #  2. envelope with empty payload -> fails payload validation
        #     (Go workers nack to DLQ; graphrag ACK-drops invalid envelopes by
        #     design, so expect document-processing.dlq to only get flavor 1)
        for routing_key, (exchange, _) in ROUTES.items():
            for i in range(args.count):
                ok1 = publish(exchange, routing_key, f"POISON {i} not-json {{{{{time.time()}")
                ok2 = publish(exchange, routing_key, json.dumps(envelope(routing_key, {})))
                sent += int(ok1) + int(ok2)
        print(f"poison: {sent} messages sent across {len(ROUTES)} exchanges "
              f"({args.count}x2 each) — watch the DLQ panel in Grafana")
    return 0 if unrouted == 0 else 1


if __name__ == "__main__":
    sys.exit(main())
