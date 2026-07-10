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
        """Establish async connection to RabbitMQ and return queue."""
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

        exchange = await self.channel.declare_exchange(
            exchange_name,
            aio_pika.ExchangeType.DIRECT,
            durable=True,
        )

        queue = await self.channel.declare_queue(
            queue_name,
            durable=True,
            arguments={
                "x-message-ttl": 12 * 60 * 60 * 1000,
                "x-dead-letter-exchange": f"{exchange_name}.dlx",
            },
        )

        await queue.bind(exchange, routing_key=routing_key)
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

                async with message.process(requeue=False):
                    try:
                        payload = json.loads(message.body.decode())
                    except json.JSONDecodeError as exc:
                        logger.error("Invalid JSON payload", exc_info=exc)
                        continue

                    logger.info("Received message", extra={"id": payload.get("id")})
                    await handler(payload)
                    logger.info("Processed message", extra={"id": payload.get("id")})

    async def close(self) -> None:
        """Close connection gracefully."""
        self._shutdown = True
        if self.connection:
            await self.connection.close()
            logger.info("RabbitMQ connection closed")
