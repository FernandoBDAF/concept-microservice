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

        email_payload = {
            "type": "email",
            "payload": {
                "recipient": "user@example.com",
                "email_type": "welcome",
                "template": "welcome",
                "data": {"first_name": "Ada"},
                "priority": "normal",
            },
            "created_at": datetime.now(tz=timezone.utc).isoformat(),
        }
        image_payload = {
            "type": "image",
            "payload": {
                "image_url": "https://example.com/image.png",
                "processing_type": "resize",
                "parameters": {"width": 256, "height": 256},
                "callback_url": "",
                "timeout_seconds": 60,
                "priority": "normal",
            },
            "created_at": datetime.now(tz=timezone.utc).isoformat(),
        }
        profile_payload = {
            "type": "profile",
            "payload": {
                "task_type": "sync",
                "profile_id": "profile-789",
                "user_id": "user-456",
                "data": {"source": "test"},
            },
            "created_at": datetime.now(tz=timezone.utc).isoformat(),
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
