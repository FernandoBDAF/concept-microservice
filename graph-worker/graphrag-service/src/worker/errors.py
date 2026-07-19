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
