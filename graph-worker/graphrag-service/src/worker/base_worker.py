import asyncio
import logging
import signal
from typing import Any, Dict, Optional

from src.monitoring.health import set_ready, set_healthy, start_health_server
from src.monitoring.metrics import PrometheusMetrics
from src.worker.consumer import AsyncRabbitMQConsumer
from src.worker.errors import UnretryableError
from src.worker.idempotency import build_guard
from src.worker.processor import DocumentProcessor

logger = logging.getLogger(__name__)


class BaseWorker:
    """Base worker with async RabbitMQ consumer, health, and metrics."""

    def __init__(self, config: dict) -> None:
        self.config = config
        self.metrics = PrometheusMetrics()
        # Idempotency guard (ADR-008.2): Redis when REDIS_ADDR is set, else an
        # in-process fallback (single-replica only). Shared with the consumer,
        # which runs the SETNX before handing a delivery to _handle_message.
        self.guard = build_guard(config.get("redis_addr"))
        self.consumer = AsyncRabbitMQConsumer(
            config["rabbitmq"], metrics=self.metrics, idempotency=self.guard
        )
        self.processor = DocumentProcessor(config)

    async def start(self) -> None:
        """Start the worker (async)."""
        loop = asyncio.get_running_loop()
        for sig in (signal.SIGTERM, signal.SIGINT):
            loop.add_signal_handler(sig, self._handle_shutdown)

        start_health_server(
            port=self.config.get("health_port", 8080),
            metrics_port=self.config.get("metrics_port", 8081),
        )

        set_ready(True)
        set_healthy(True)
        logger.info("Worker starting")

        try:
            await self.consumer.consume(self._handle_message)
        except asyncio.CancelledError:
            logger.info("Worker cancelled")
        finally:
            await self.consumer.close()

    async def _handle_message(self, message: dict) -> Optional[Dict[str, Any]]:
        """Validate + process one envelope.

        Returns the processor result on success (the consumer maps its status
        to the task.result). Raises UnretryableError for validation failures
        (a malformed message can never succeed on retry -> DLX); other
        exceptions propagate as retryable (ADR-008.1).
        """
        message_id = message.get("id", "unknown")

        if not self.processor.validate(message):
            self.metrics.record_error("validation")
            logger.error("Invalid message", extra={"id": message_id})
            raise UnretryableError(f"validation failed for message {message_id}")

        with self.metrics.track_duration():
            try:
                result = await self.processor.process(message)
                self.metrics.record_success()
                logger.info("Message processed", extra={"id": message_id, "result": result})
                return result
            except Exception:
                self.metrics.record_error("processing")
                logger.exception("Processing failed", extra={"id": message_id})
                raise

    def _handle_shutdown(self) -> None:
        logger.info("Shutdown signal received")
        set_ready(False)
        set_healthy(False)
        for task in asyncio.all_tasks():
            if task is not asyncio.current_task():
                task.cancel()
