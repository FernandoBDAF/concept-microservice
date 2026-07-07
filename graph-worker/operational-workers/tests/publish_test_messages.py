"""Manual smoke tool: publishes one test message per worker queue.

Payload shapes follow graph-worker/shared/contracts/MESSAGE_FORMAT.md
exactly. Run the workers first (they declare the exchanges/queues/DLQs on
startup) so these publishes land somewhere and actually get consumed;
otherwise a direct exchange with no bound queue silently drops the message.

Usage:
    pip install aio-pika
    RABBITMQ_HOST=localhost python tests/publish_test_messages.py
"""

import asyncio
import json
import os
import uuid
from datetime import datetime, timezone

import aio_pika


def base_message(message_type: str, payload: dict) -> dict:
    return {
        "id": str(uuid.uuid4()),
        "type": message_type,
        "timestamp": datetime.now(tz=timezone.utc).isoformat(),
        "payload": payload,
        "metadata": {"source": "test-script"},
    }


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

        email_exchange = await channel.declare_exchange(
            "email-tasks", aio_pika.ExchangeType.DIRECT, durable=True
        )
        image_exchange = await channel.declare_exchange(
            "image-tasks", aio_pika.ExchangeType.DIRECT, durable=True
        )
        profile_exchange = await channel.declare_exchange(
            "profile-tasks", aio_pika.ExchangeType.DIRECT, durable=True
        )

        # graph-worker/shared/contracts/MESSAGE_FORMAT.md "Email Payload"
        email_payload = {
            "email_type": "welcome",
            "recipient": "user@example.com",
            "subject": "Welcome",
            "template_id": "welcome-template",
            "variables": {"first_name": "Ada"},
        }

        # graph-worker/shared/contracts/MESSAGE_FORMAT.md "Image Payload"
        image_payload = {
            "operation": "resize",
            "source_url": "s3://bucket/path/image.png",
            "target_path": "processed/image.png",
            "width": 512,
            "height": 512,
            "quality": 85,
            "format": "png",
        }

        # graph-worker/shared/contracts/MESSAGE_FORMAT.md "Profile Payload"
        profile_payload = {
            "task_type": "sync",
            "profile_id": "profile-789",
            "user_id": "user-456",
            "data": {"source": "external-system"},
        }

        await email_exchange.publish(
            aio_pika.Message(body=json.dumps(base_message("email.send", email_payload)).encode()),
            routing_key="email.send",
        )
        await image_exchange.publish(
            aio_pika.Message(body=json.dumps(base_message("image.process", image_payload)).encode()),
            routing_key="image.process",
        )
        await profile_exchange.publish(
            aio_pika.Message(body=json.dumps(base_message("profile.task", profile_payload)).encode()),
            routing_key="profile.task",
        )

        print("Published email, image, and profile test messages")


if __name__ == "__main__":
    asyncio.run(main())
