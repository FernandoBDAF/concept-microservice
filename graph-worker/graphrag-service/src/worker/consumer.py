import asyncio
import json
import logging
import uuid
from datetime import datetime, timezone
from typing import Any, Awaitable, Callable, Optional

import aio_pika

from src.monitoring.tracing import consumer_span
from src.worker.errors import TopologyMissingError, UnretryableError
from src.worker.idempotency import (
    DEFAULT_TTL_SECONDS,
    InMemoryGuard,
    key_for_attempt,
)
from src.worker.retry import death_count, select_tier

logger = logging.getLogger(__name__)

# Reconnect backoff bounds for consume(), mirroring the Go operational-workers'
# connect-retry loop (DefaultRetryDelay 1s, maxBackoff 30s in
# operational-workers/internal/common/queue) so graphrag survives the same cold
# `make up` race instead of crash-looping into Docker's restart backoff.
INITIAL_BACKOFF_SECONDS = 1.0
MAX_BACKOFF_SECONDS = 30.0


def _now_iso() -> str:
    """ISO-8601 UTC timestamp, e.g. 2026-01-30T12:34:56Z.

    Matches the frozen envelope contract example
    (graph-worker/shared/contracts/MESSAGE_FORMAT.md) and Go's RFC3339 `Z`
    form, which the api-service task-results consumer parses.
    """
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


class AsyncRabbitMQConsumer:
    """Async RabbitMQ consumer for the GraphRAG worker (aio-pika).

    Topology is owned by the broker (deploy/rabbitmq/definitions.json loaded
    at RabbitMQ startup, ADR-008.4). This consumer never *declares* topology:
    it PASSIVE-declares the entities it depends on to verify they exist (and
    to obtain publish handles). If they are not present yet, connect() logs a
    pointed message and raises TopologyMissingError; consume() retries with
    backoff rather than crashing the process (see consume()). Failure handling
    follows ADR-008.1 (timed retry tiers -> DLX) and ADR-008.2 (idempotency
    guard); completion is reported on task-results (ADR-008.3).
    """

    def __init__(
        self,
        config: dict,
        metrics: Optional[Any] = None,
        idempotency: Optional[Any] = None,
    ) -> None:
        self.config = config
        self.metrics = metrics
        self.idempotency = idempotency or InMemoryGuard()
        self.connection: Optional[aio_pika.RobustConnection] = None
        self.channel: Optional[aio_pika.abc.AbstractChannel] = None
        self._shutdown = False

        # Names, all resolved from config (see src/config/worker_config.py) and
        # the `<exchange>.retry` / `<exchange>.dlx` convention in definitions.json.
        self._exchange_name = config.get("exchange", "document-tasks")
        self._queue_name = config.get("queue", "document-processing")
        self._routing_key = config.get("routing_key", "document.process")
        self._retry_exchange_name = f"{self._exchange_name}.retry"
        self._dlx_exchange_name = f"{self._exchange_name}.dlx"
        self._results_exchange_name = config.get("results_exchange", "task-results")
        self._results_routing_key = config.get("results_routing_key", "task.result")
        # x-death entries for these wait-queues count as taken retries.
        self._retry_queue_prefix = f"{self._queue_name}.retry."

        # Publish handles, populated by connect() from passive declares.
        self._exchange: Optional[aio_pika.abc.AbstractExchange] = None
        self._retry_exchange: Optional[aio_pika.abc.AbstractExchange] = None
        self._dlx_exchange: Optional[aio_pika.abc.AbstractExchange] = None
        self._results_exchange: Optional[aio_pika.abc.AbstractExchange] = None

    async def connect(self) -> aio_pika.abc.AbstractQueue:
        """Connect, PASSIVE-verify topology, and return the work queue.

        All declares are passive (passive=True): they assert the entity exists
        without creating or mutating it. A missing entity means definitions.json
        has not been loaded into the broker yet — we log a pointed message and
        raise TopologyMissingError so consume()'s reconnect loop retries with
        backoff (on a cold start this is usually a race with definitions.json
        being imported, not a real misconfiguration). Connection failures
        (broker not reachable yet) raise naturally from connect_robust; both are
        handled by the reconnect loop, never by exiting the process.
        """
        # Reconnect hygiene: drop a connection left over from a previous failed
        # attempt (e.g. a topology-missing race that opened the connection but
        # had its channel closed) before redialing, so retries don't leak it.
        if self.connection is not None:
            try:
                if not self.connection.is_closed:
                    await self.connection.close()
            except Exception:  # pragma: no cover - best-effort cleanup
                logger.debug(
                    "closing stale connection before reconnect failed",
                    exc_info=True,
                )
            self.connection = None

        url = self.config.get("url")
        if url:
            self.connection = await aio_pika.connect_robust(url)
        else:
            self.connection = await aio_pika.connect_robust(
                host=self.config["host"],
                port=self.config["port"],
                login=self.config["username"],
                password=self.config["password"],
                virtualhost=self.config.get("vhost", "/"),
            )

        self.channel = await self.connection.channel()
        await self.channel.set_qos(
            prefetch_count=self.config.get("prefetch_count", 1)
        )

        try:
            self._exchange = await self.channel.declare_exchange(
                self._exchange_name, passive=True
            )
            self._retry_exchange = await self.channel.declare_exchange(
                self._retry_exchange_name, passive=True
            )
            self._dlx_exchange = await self.channel.declare_exchange(
                self._dlx_exchange_name, passive=True
            )
            self._results_exchange = await self.channel.declare_exchange(
                self._results_exchange_name, passive=True
            )
            queue = await self.channel.declare_queue(self._queue_name, passive=True)
        except aio_pika.exceptions.ChannelClosed as exc:
            logger.error(
                "broker topology missing — is definitions.json loaded? "
                "(passive declare failed for the document-tasks topology: %s)",
                exc,
            )
            raise TopologyMissingError(
                "document-tasks topology not present (passive declare failed)"
            ) from exc

        logger.info(
            "Connected to RabbitMQ (passive topology verified)",
            extra={"queue": self._queue_name, "routing_key": self._routing_key},
        )
        return queue

    async def consume(self, handler: Callable[[dict], Awaitable[Any]]) -> None:
        """Consume messages, reconnecting with capped backoff on failure.

        The worker process — and the :8081 metrics server base_worker started
        before calling this — STAYS ALIVE across transient RabbitMQ failures
        (broker not reachable yet, or the passive topology not imported yet on a
        cold `make up`), retrying with exponential backoff instead of exiting
        non-zero into Docker's restart backoff. This mirrors the Go
        operational-workers' reconnect loop
        (operational-workers/internal/common/queue/consumer.go), which is why
        those three survive the same cold-start race that used to crash-loop
        graphrag. A genuinely-missing topology now surfaces as "connected,
        metrics up, not consuming, loud repeating log" — observable and
        alertable — rather than a crash-loop. Clean shutdown
        (self._shutdown / CancelledError) exits the loop without error.
        """
        backoff = INITIAL_BACKOFF_SECONDS
        while not self._shutdown:
            try:
                queue = await self.connect()
                # Reset backoff after every successful (re)connect.
                backoff = INITIAL_BACKOFF_SECONDS
                async with queue.iterator() as queue_iter:
                    async for message in queue_iter:
                        if self._shutdown:
                            break
                        await self._handle_delivery(message, handler)
            except asyncio.CancelledError:
                # Cooperative shutdown (base_worker cancels the consume task);
                # propagate so start() can close the connection cleanly.
                raise
            except (TopologyMissingError, aio_pika.exceptions.AMQPError, OSError) as exc:
                if self._shutdown:
                    break
                logger.warning(
                    "RabbitMQ connection/topology unavailable; retrying after backoff",
                    extra={"error": str(exc), "backoff_seconds": backoff},
                )
                await asyncio.sleep(backoff)
                backoff = min(backoff * 2, MAX_BACKOFF_SECONDS)
                continue
            # Reached only when the consume block exited without raising: a
            # clean shutdown (we broke out of the async-for). Exit the loop.
            break

    async def _handle_delivery(
        self,
        message: aio_pika.abc.AbstractIncomingMessage,
        handler: Callable[[dict], Awaitable[Any]],
    ) -> None:
        """Process one delivery: dedupe, process, then route + ack.

        Every classified outcome ACKs the original delivery after the follow-up
        publish (retry / DLX / result) — never reject-requeue (ADR-008.1). The
        outer except is reached only if our own publish/ack fails (a broker
        issue); there we requeue as a last resort so the message is not lost.
        """
        payload: Optional[dict] = None
        try:
            # 1. Parse. A parse failure can never succeed on retry -> DLX.
            try:
                payload = json.loads(message.body.decode())
            except (json.JSONDecodeError, UnicodeDecodeError) as exc:
                logger.error(
                    "Invalid message body; dead-lettering (unretryable)",
                    exc_info=True,
                )
                await self._dead_letter(message, None, error=f"parse error: {exc}")
                await message.ack()
                return

            envelope_id = payload.get("id")
            # Retry attempt = x-death count for our retry wait-queues. Computed
            # once and reused for BOTH the attempt-scoped idempotency key and
            # the retry-tier selection, so the two can never disagree.
            attempt = death_count(dict(message.headers or {}), self._retry_queue_prefix)

            # 2. Idempotency guard (SETNX), scoped to this attempt (ADR-008.2).
            if envelope_id and not await self._acquire(str(envelope_id), attempt):
                self._record("record_duplicate")
                logger.info(
                    "Duplicate delivery; acking without reprocessing",
                    extra={"id": envelope_id, "attempt": attempt},
                )
                await message.ack()
                return

            # 3. Process, traced, with the parent context from AMQP headers.
            metadata = payload.get("metadata") or {}
            try:
                with consumer_span(
                    "consume document-processing",
                    headers=dict(message.headers or {}),
                    attributes={
                        "messaging.system": "rabbitmq",
                        "messaging.operation": "process",
                        "messaging.destination.name": self._queue_name,
                        "messaging.rabbitmq.destination.routing_key": message.routing_key,
                        "messaging.message.id": envelope_id,
                        "app.envelope.trace_id": metadata.get("trace_id"),
                    },
                ):
                    logger.info("Received message", extra={"id": envelope_id})
                    result = await handler(payload)
                    logger.info("Processed message", extra={"id": envelope_id})
            except UnretryableError as exc:
                logger.warning(
                    "Unretryable failure; dead-lettering",
                    extra={"id": envelope_id, "error": str(exc)},
                )
                await self._dead_letter(message, payload, error=str(exc))
                await message.ack()
                return
            except Exception as exc:
                await self._retry_or_dead_letter(message, payload, exc, attempt)
                await message.ack()
                return

            # 4. Success (processing ran to completion) -> task.result + ack.
            await self._publish_result(payload, self._result_status(result))
            await message.ack()

        except Exception:
            # Not a classified path — our own publish/ack failed or an
            # unexpected bug. Requeue as a last resort (redelivered after
            # reconnect) rather than dropping the message.
            logger.exception(
                "Delivery handling failed before ack; requeuing",
                extra={"id": (payload or {}).get("id")},
            )
            try:
                await message.nack(requeue=True)
            except Exception:
                logger.exception("nack after delivery failure also failed")

    async def _acquire(self, envelope_id: str, attempt: int) -> bool:
        """True -> first delivery (process it); False -> duplicate (skip).

        The key is scoped to `attempt` (x-death count) so intentional retries
        are reprocessed while same-attempt redeliveries dedupe. On a guard/infra
        error we FAIL OPEN (return True, process anyway) and count a metric —
        dedupe is a net, not a lock (ADR-008.2).
        """
        key = key_for_attempt(self._queue_name, envelope_id, attempt)
        try:
            return await self.idempotency.begin(key, DEFAULT_TTL_SECONDS)
        except Exception:
            logger.warning(
                "Idempotency guard error; failing open (processing anyway)",
                exc_info=True,
                extra={"id": envelope_id},
            )
            self._record("record_idempotency_error")
            return True

    async def _retry_or_dead_letter(
        self,
        message: aio_pika.abc.AbstractIncomingMessage,
        payload: Optional[dict],
        exc: Exception,
        attempt: int,
    ) -> None:
        """Retryable failure: schedule the next tier, or DLX if exhausted.

        `attempt` is the same x-death count used for the idempotency key, so
        tier selection and dedupe scope stay in lock-step.
        """
        tier = select_tier(attempt)
        env_id = (payload or {}).get("id")
        if tier is None:
            logger.warning(
                "Retries exhausted; dead-lettering",
                extra={"id": env_id, "death_count": attempt, "error": str(exc)},
            )
            await self._dead_letter(message, payload, error=str(exc))
        else:
            logger.warning(
                "Retryable failure; scheduling retry",
                extra={"id": env_id, "tier": tier, "death_count": attempt, "error": str(exc)},
            )
            await self._publish_retry(message, tier)

    async def _publish_retry(
        self, message: aio_pika.abc.AbstractIncomingMessage, tier: str
    ) -> None:
        """Republish the body UNCHANGED to the retry exchange, headers copied.

        Copying message.headers preserves the accumulating x-death entries so
        the tier count keeps advancing across passes (ADR-008.1).
        """
        await self._retry_exchange.publish(
            aio_pika.Message(
                body=message.body,
                delivery_mode=aio_pika.DeliveryMode.PERSISTENT,
                headers=dict(message.headers or {}),
                content_type=message.content_type,
                message_id=message.message_id,
                correlation_id=message.correlation_id,
            ),
            routing_key=f"{self._routing_key}.retry.{tier}",
        )
        self._record("record_retry", tier)

    async def _dead_letter(
        self,
        message: aio_pika.abc.AbstractIncomingMessage,
        payload: Optional[dict],
        error: str,
    ) -> None:
        """Publish the body to the DLX (poison) and emit a failed task.result."""
        await self._dlx_exchange.publish(
            aio_pika.Message(
                body=message.body,
                delivery_mode=aio_pika.DeliveryMode.PERSISTENT,
                headers=dict(message.headers or {}),
                content_type=message.content_type,
                message_id=message.message_id,
                correlation_id=message.correlation_id,
            ),
            routing_key=self._routing_key,
        )
        self._record("record_dlq")
        # Terminal failure -> report it (skipped when the envelope was
        # unparseable and we have no ids to correlate on).
        if payload:
            await self._publish_result(payload, "failed", error=error)

    async def _publish_result(
        self, payload: Optional[dict], status: str, error: Optional[str] = None
    ) -> None:
        """Publish a task.result envelope to task-results (ADR-008.3).

        For document tasks the task id equals the envelope id (api-service's
        task.Service.Submit returns msg.ID), so task_id and envelope_id carry
        the same value; document_id comes from the document payload.
        """
        if not payload:
            return
        envelope_id = payload.get("id")
        inner = payload.get("payload") or {}
        result_payload = {
            "task_id": envelope_id,
            "task_type": payload.get("type"),
            "status": status,
            "envelope_id": envelope_id,
            "document_id": inner.get("document_id"),
        }
        if error:
            result_payload["error"] = error

        result = {
            "id": str(uuid.uuid4()),
            "type": "task.result",
            "timestamp": _now_iso(),
            "payload": result_payload,
            "metadata": {"source": "graphrag-service"},
        }

        await self._results_exchange.publish(
            aio_pika.Message(
                body=json.dumps(result).encode(),
                delivery_mode=aio_pika.DeliveryMode.PERSISTENT,
                content_type="application/json",
                message_id=result["id"],
            ),
            routing_key=self._results_routing_key,
        )

    @staticmethod
    def _result_status(result: Any) -> str:
        """Map a processor result to a task.result status.

        A returned result with status "failed" (e.g. the GraphRAG pipeline ran
        but exited non-zero) is a terminal failure; everything else that
        returned without raising (including the light-mode "stubbed" result)
        counts as completed.
        """
        if isinstance(result, dict) and result.get("status") == "failed":
            return "failed"
        return "completed"

    def _record(self, method: str, *args: Any) -> None:
        if self.metrics is not None:
            getattr(self.metrics, method)(*args)

    async def close(self) -> None:
        """Close connection gracefully."""
        self._shutdown = True
        if self.connection:
            await self.connection.close()
            logger.info("RabbitMQ connection closed")
