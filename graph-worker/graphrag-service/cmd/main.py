#!/usr/bin/env python3
"""GraphRAG Worker - Document processing service."""

import asyncio
import logging
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent.parent))

from src.config.worker_config import load_config
from src.monitoring.health import set_ready, set_healthy
from src.worker.base_worker import BaseWorker


def setup_logging(level: str) -> None:
    """Configure structured JSON logging (per CONTRACTS.md section 5)."""
    try:
        from pythonjsonlogger.json import JsonFormatter  # python-json-logger >=3
    except ImportError:
        from pythonjsonlogger.jsonlogger import JsonFormatter  # python-json-logger 2.x

    handler = logging.StreamHandler()
    handler.setFormatter(
        JsonFormatter(
            "%(asctime)s %(name)s %(levelname)s %(message)s",
            rename_fields={"asctime": "timestamp", "levelname": "level", "name": "logger"},
        )
    )

    root = logging.getLogger()
    root.setLevel(getattr(logging, level.upper(), logging.INFO))
    root.handlers = [handler]


async def main() -> None:
    config = load_config()
    setup_logging(config.get("log_level", "INFO"))

    logger = logging.getLogger(__name__)
    logger.info("Starting GraphRAG worker")
    logger.info(
        "RabbitMQ config",
        extra={
            "host": config["rabbitmq"]["host"],
            "queue": config["rabbitmq"]["queue"],
            "routing_key": config["rabbitmq"]["routing_key"],
        },
    )

    worker = BaseWorker(config)

    set_ready(True)
    set_healthy(True)

    try:
        await worker.start()
    except KeyboardInterrupt:
        logger.info("Interrupted")
    except Exception:
        logger.exception("Worker error")
        set_healthy(False)
        raise
    finally:
        set_ready(False)
        logger.info("Worker stopped")


if __name__ == "__main__":
    asyncio.run(main())
