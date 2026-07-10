import asyncio
import json
import os
import uuid
from datetime import datetime, timezone

import aio_pika


async def main() -> None:
    host = os.getenv("RABBITMQ_HOST", "localhost")
    port = int(os.getenv("RABBITMQ_PORT", "5672"))
    username = os.getenv("RABBITMQ_USER", "guest")
    password = os.getenv("RABBITMQ_PASSWORD", "guest")
    vhost = os.getenv("RABBITMQ_VHOST", "/")

    connection = await aio_pika.connect_robust(
        host=host,
        port=port,
        login=username,
        password=password,
        virtualhost=vhost,
    )

    async with connection:
        channel = await connection.channel()
        exchange = await channel.declare_exchange(
            "document-tasks", aio_pika.ExchangeType.DIRECT, durable=True
        )

        message = {
            "id": str(uuid.uuid4()),
            "type": "document.process",
            "timestamp": datetime.now(tz=timezone.utc).isoformat(),
            "payload": {
                "document_id": "doc-123",
                "storage_bucket": "documents",
                "storage_path": "uploads/2026/01/30/doc-123.pdf",
                "file_type": "pdf",
                "user_id": "user-456",
            },
            "metadata": {
                "source": "test-script",
            },
        }

        await exchange.publish(
            aio_pika.Message(body=json.dumps(message).encode()),
            routing_key="document.process",
        )

        print("Published document.process test message")


if __name__ == "__main__":
    asyncio.run(main())
