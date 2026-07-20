"""Worker error classification for the retry/DLQ routing (ADR-008.1).

Mirrors the Go side's `ErrUnretryable`
(operational-workers/internal/common/queue/errors.go): validation and
parse failures are *unretryable* (retrying can never make an invalid
message valid), so they go straight to the dead-letter exchange. Every
other failure (transient infra, LLM timeouts, MinIO/Mongo hiccups) is
retryable and flows through the timed retry tiers.
"""


class UnretryableError(Exception):
    """Raised by the handler when a message can never succeed on retry.

    The consumer's delivery handler treats this exactly like the Go
    workers treat `ErrUnretryable`: publish to the DLX and ack, never to
    the retry exchange. Validation failures (missing/invalid fields) wrap
    it; JSON parse failures are classified as unretryable directly in the
    consumer without needing this type.
    """


class TopologyMissingError(Exception):
    """Raised by connect() when a passive topology declare fails.

    Topology is owned by the broker (deploy/rabbitmq/definitions.json loaded
    at RabbitMQ startup, ADR-008.4); the consumer only PASSIVE-declares to
    verify it exists. A passive declare that fails means the topology is not
    present yet. On a cold `make up` this is almost always a RACE — graphrag
    connected after the broker opened its port but before definitions.json
    finished importing — not a real misconfiguration.

    connect() raises this (instead of exiting the process) so the reconnect
    loop in consume() retries with backoff, keeping the process and its :8081
    metrics server alive. This mirrors the Go operational-workers, whose
    in-process reconnect loop lets them survive the same cold-start race
    (operational-workers/internal/common/queue/consumer.go).
    """
