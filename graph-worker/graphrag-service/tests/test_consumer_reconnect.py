"""Unit tests for the consume() reconnect-with-backoff loop
(src/worker/consumer.py).

The loop is what keeps the graphrag worker process — and the :8081 metrics
server base_worker starts before consuming — ALIVE across a cold-start RabbitMQ
race (broker not reachable yet, or definitions.json not imported yet) instead
of crashing to exit 1 and hitting Docker's exponential restart backoff. It
mirrors the Go operational-workers' in-process reconnect loop
(operational-workers/internal/common/queue/consumer.go).

No broker is needed: aio_pika is stubbed at the import seam and
connect_robust / asyncio.sleep are patched, so the test is fast and
deterministic. Run from the graphrag-service directory:

    python3 -m unittest tests.test_consumer_reconnect
"""

import os
import sys
import types
import unittest
from unittest import mock

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))


# --- Stub aio_pika at the import seam ------------------------------------
# The real broker library is not importable in the unit environment. The
# reconnect loop only needs connect_robust (patched per test) and a real
# exception hierarchy for its `except` clause; consumer.py's function
# annotations (evaluated at import, no `from __future__ import annotations`)
# reference aio_pika.abc.* — a stub submodule resolves every such name to
# `object`, which is a valid annotation value.
class _StubModule(types.ModuleType):
    def __getattr__(self, name):  # any unknown attribute resolves to `object`
        return object


def _install_aio_pika_stub() -> None:
    aio_pika = _StubModule("aio_pika")

    exceptions = types.ModuleType("aio_pika.exceptions")

    class AMQPError(Exception):
        pass

    class AMQPConnectionError(AMQPError):
        pass

    class AMQPChannelError(AMQPError):
        pass

    class ChannelClosed(AMQPChannelError):
        pass

    exceptions.AMQPError = AMQPError
    exceptions.AMQPConnectionError = AMQPConnectionError
    exceptions.AMQPChannelError = AMQPChannelError
    exceptions.ChannelClosed = ChannelClosed

    async def _connect_robust(*args, **kwargs):  # replaced per-test
        raise AMQPConnectionError("stub connect_robust was not patched")

    abc = _StubModule("aio_pika.abc")

    aio_pika.exceptions = exceptions
    aio_pika.abc = abc
    aio_pika.connect_robust = _connect_robust

    sys.modules["aio_pika"] = aio_pika
    sys.modules["aio_pika.exceptions"] = exceptions
    sys.modules["aio_pika.abc"] = abc


try:  # Prefer the real library if it happens to be installed (e.g. in CI).
    import aio_pika  # noqa: F401
except ImportError:
    _install_aio_pika_stub()

# Import AFTER the stub is installed so consumer.py's `import aio_pika` binds
# to it.
from src.worker import consumer as consumer_mod  # noqa: E402
from src.worker.errors import TopologyMissingError  # noqa: E402


class ConsumeReconnectTest(unittest.IsolatedAsyncioTestCase):
    async def _never_called_handler(self, message):
        raise AssertionError(
            "handler must not run: no delivery is possible without a connection"
        )

    async def test_connection_error_retries_with_backoff_not_propagate(self):
        """connect_robust raising a connection error must be caught and retried
        with capped exponential backoff — never propagated out of consume()
        (which would kill the process)."""
        c = consumer_mod.AsyncRabbitMQConsumer({"url": "amqp://stub"})

        attempts = {"connect": 0}

        async def failing_connect_robust(*args, **kwargs):
            attempts["connect"] += 1
            raise consumer_mod.aio_pika.exceptions.AMQPConnectionError(
                "broker not reachable yet"
            )

        sleeps = []

        async def fake_sleep(delay):
            # Count the backoff waits without waiting; stop the otherwise
            # infinite retry loop deterministically after the 3rd attempt.
            sleeps.append(delay)
            if len(sleeps) >= 3:
                c._shutdown = True

        with mock.patch.object(
            consumer_mod.aio_pika, "connect_robust", failing_connect_robust
        ), mock.patch.object(consumer_mod.asyncio, "sleep", fake_sleep):
            # Returns (does not raise): the loop swallowed every connect error.
            await c.consume(self._never_called_handler)

        self.assertEqual(attempts["connect"], 3)  # retried each pass
        self.assertEqual(
            sleeps,
            [
                consumer_mod.INITIAL_BACKOFF_SECONDS,
                consumer_mod.INITIAL_BACKOFF_SECONDS * 2,
                consumer_mod.INITIAL_BACKOFF_SECONDS * 4,
            ],
        )
        self.assertTrue(c._shutdown)

    async def test_topology_missing_error_is_retried_not_process_exit(self):
        """The crux of the fix: connect() now raises TopologyMissingError
        instead of calling sys.exit(1). The loop must treat it as a transient,
        retryable condition, not let it (or a process exit) escape."""
        c = consumer_mod.AsyncRabbitMQConsumer({"url": "amqp://stub"})

        attempts = {"connect": 0}

        async def connect_raising_topology(*args, **kwargs):
            attempts["connect"] += 1
            raise TopologyMissingError("definitions.json not imported yet")

        sleeps = []

        async def fake_sleep(delay):
            sleeps.append(delay)
            if len(sleeps) >= 2:
                c._shutdown = True

        with mock.patch.object(c, "connect", connect_raising_topology), \
                mock.patch.object(consumer_mod.asyncio, "sleep", fake_sleep):
            await c.consume(self._never_called_handler)  # must not raise / exit

        self.assertEqual(attempts["connect"], 2)
        self.assertEqual(
            sleeps,
            [
                consumer_mod.INITIAL_BACKOFF_SECONDS,
                consumer_mod.INITIAL_BACKOFF_SECONDS * 2,
            ],
        )

    async def test_backoff_is_capped_at_max(self):
        """Repeated failures must not grow the backoff past MAX_BACKOFF_SECONDS."""
        c = consumer_mod.AsyncRabbitMQConsumer({"url": "amqp://stub"})

        async def always_fail(*args, **kwargs):
            raise TopologyMissingError("still not loaded")

        sleeps = []

        async def fake_sleep(delay):
            sleeps.append(delay)
            if len(sleeps) >= 12:  # well past the point where 1s doubles to 30s
                c._shutdown = True

        with mock.patch.object(c, "connect", always_fail), \
                mock.patch.object(consumer_mod.asyncio, "sleep", fake_sleep):
            await c.consume(self._never_called_handler)

        self.assertLessEqual(max(sleeps), consumer_mod.MAX_BACKOFF_SECONDS)
        self.assertEqual(sleeps[-1], consumer_mod.MAX_BACKOFF_SECONDS)  # reached the cap

    async def test_shutdown_before_start_returns_without_connecting(self):
        """A shutdown requested before consume() starts must exit the loop
        cleanly on the `while not self._shutdown` guard, never dialing."""
        c = consumer_mod.AsyncRabbitMQConsumer({"url": "amqp://stub"})
        c._shutdown = True

        called = {"connect": 0}

        async def connect_robust(*args, **kwargs):
            called["connect"] += 1
            raise AssertionError("must not connect once shutdown is requested")

        with mock.patch.object(
            consumer_mod.aio_pika, "connect_robust", connect_robust
        ):
            await c.consume(self._never_called_handler)  # returns immediately

        self.assertEqual(called["connect"], 0)


if __name__ == "__main__":
    unittest.main()
