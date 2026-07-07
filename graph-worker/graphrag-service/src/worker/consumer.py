import json
import logging
from typing import Awaitable, Callable, Optional

import aio_pika

logger = logging.getLogger(__name__)


class AsyncRabbitMQConsumer:
    """Async RabbitMQ consumer for the GraphRAG worker (aio-pika)."""

    def __init__(self, config: dict) -> None:
        self.config = config
        self.connection: Optional[aio_pika.RobustConnection] = None
        self.channel: Optional[aio_pika.abc.AbstractChannel] = None
        self._shutdown = False

    async def connect(self) -> aio_pika.Queue:
        """Establish async connection to RabbitMQ and return queue.

        Topology is declared to exactly match the publisher (api-service,
        internal/infrastructure/rabbitmq/client.go ensureTopology): a direct
        durable exchange, a direct durable dead-letter exchange
        (`<exchange>.dlx`), the main queue (with x-dead-letter-exchange,
        x-dead-letter-routing-key, x-message-ttl, x-max-retries arguments),
        and a durable DLQ (`<queue>.dlq`) bound to the DLX. Declaring the same
        topology here (rather than relying solely on the publisher) lets this
        service consume correctly even if it is the first to connect, and
        RabbitMQ's idempotent declare is a no-op once api-service has already
        created it with matching arguments.
        """
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

        exchange_name = self.config.get("exchange", "document-tasks")
        queue_name = self.config.get("queue", "document-processing")
        routing_key = self.config.get("routing_key", "document.process")
        dlx_name = f"{exchange_name}.dlx"
        dlq_name = f"{queue_name}.dlq"

        exchange = await self.channel.declare_exchange(
            exchange_name,
            aio_pika.ExchangeType.DIRECT,
            durable=True,
        )
        dead_letter_exchange = await self.channel.declare_exchange(
            dlx_name,
            aio_pika.ExchangeType.DIRECT,
            durable=True,
        )

        queue = await self.channel.declare_queue(
            queue_name,
            durable=True,
            arguments={
                "x-dead-letter-exchange": dlx_name,
                "x-dead-letter-routing-key": routing_key,
                "x-message-ttl": 12 * 60 * 60 * 1000,  # 12h, per ROUTING_KEYS.md
                "x-max-retries": 3,  # informational, mirrors publisher's arg
            },
        )
        await queue.bind(exchange, routing_key=routing_key)

        dead_letter_queue = await self.channel.declare_queue(
            dlq_name,
            durable=True,
            arguments={"x-message-ttl": 7 * 24 * 60 * 60 * 1000},  # 7 days
        )
        await dead_letter_queue.bind(dead_letter_exchange, routing_key=routing_key)

        logger.info(
            "Connected to RabbitMQ", extra={"queue": queue_name, "routing_key": routing_key}
        )
        return queue

    async def consume(self, handler: Callable[[dict], Awaitable[None]]) -> None:
        """Start consuming messages with async handler."""
        queue = await self.connect()

        async with queue.iterator() as queue_iter:
            async for message in queue_iter:
                if self._shutdown:
                    break
                await self._handle_delivery(message, handler)

    async def _handle_delivery(
        self, message: aio_pika.abc.AbstractIncomingMessage, handler: Callable[[dict], Awaitable[None]]
    ) -> None:
        """Process a single delivery, isolating failures to this message only.

        `message.process(requeue=False)` acks on a clean exit and nacks
        without requeue (-> DLQ, per the dead-letter args above) if the body
        raises. That re-raises the original exception once the nack is sent,
        so it MUST be caught here rather than left to propagate: otherwise a
        single poison or failed message would escape the `async for` loop in
        `consume()` and kill the entire consumer.
        """
        try:
            async with message.process(requeue=False):
                try:
                    payload = json.loads(message.body.decode())
                except json.JSONDecodeError:
                    logger.error("Invalid JSON payload; rejecting to DLQ", exc_info=True)
                    raise

                logger.info("Received message", extra={"id": payload.get("id")})
                await handler(payload)
                logger.info("Processed message", extra={"id": payload.get("id")})
        except Exception:
            logger.exception("Message processing failed; nacked without requeue (-> DLQ)")

    async def close(self) -> None:
        """Close connection gracefully."""
        self._shutdown = True
        if self.connection:
            await self.connection.close()
            logger.info("RabbitMQ connection closed")
