"""Unit tests for the idempotency key shape and attempt scoping
(src/worker/idempotency.py).

Key shape mirrors the Go guard byte-for-byte:
    idem:<queue>:<envelope-id>            (KeyFor)
    idem:<queue>:<envelope-id>:<attempt>  (KeyForAttempt, used by the consumer)

Run from the graphrag-service directory:

    python3 -m unittest tests.test_idempotency
"""

import asyncio
import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from src.worker.idempotency import (  # noqa: E402
    DEFAULT_TTL_SECONDS,
    InMemoryGuard,
    key_for,
    key_for_attempt,
)

QUEUE = "document-processing"
ENV_ID = "abc-123"


class KeyShapeTest(unittest.TestCase):
    def test_key_for_matches_go_keyfor(self):
        # Go: "idem:" + queue + ":" + envelopeID
        self.assertEqual(key_for(QUEUE, ENV_ID), "idem:document-processing:abc-123")

    def test_key_for_attempt_matches_go_keyforattempt(self):
        # Go: KeyFor(...) + ":" + strconv.Itoa(attempt)
        self.assertEqual(
            key_for_attempt(QUEUE, ENV_ID, 0), "idem:document-processing:abc-123:0"
        )
        self.assertEqual(
            key_for_attempt(QUEUE, ENV_ID, 1), "idem:document-processing:abc-123:1"
        )
        self.assertEqual(
            key_for_attempt(QUEUE, ENV_ID, 2), "idem:document-processing:abc-123:2"
        )

    def test_attempt_key_extends_plain_key(self):
        base = key_for(QUEUE, ENV_ID)
        self.assertEqual(key_for_attempt(QUEUE, ENV_ID, 3), base + ":3")

    def test_ttl_is_24h(self):
        self.assertEqual(DEFAULT_TTL_SECONDS, 24 * 60 * 60)


class AttemptScopingTest(unittest.TestCase):
    """The whole point of attempt scoping: retries are NOT deduped, but a
    same-attempt redelivery IS."""

    def test_same_attempt_redelivery_dedupes(self):
        guard = InMemoryGuard()

        async def run():
            key = key_for_attempt(QUEUE, ENV_ID, 0)
            first = await guard.begin(key, DEFAULT_TTL_SECONDS)
            dup = await guard.begin(key, DEFAULT_TTL_SECONDS)
            return first, dup

        first, dup = asyncio.run(run())
        self.assertTrue(first)   # first delivery processes
        self.assertFalse(dup)    # crash-then-redeliver at same attempt dedupes

    def test_higher_attempt_is_admitted(self):
        guard = InMemoryGuard()

        async def run():
            # attempt 0 processed, then the timed retry brings it back as
            # attempt 1 -> must be admitted (distinct key), else the retry is
            # lost (breaks EXP-40).
            a0 = await guard.begin(key_for_attempt(QUEUE, ENV_ID, 0), DEFAULT_TTL_SECONDS)
            a1 = await guard.begin(key_for_attempt(QUEUE, ENV_ID, 1), DEFAULT_TTL_SECONDS)
            a2 = await guard.begin(key_for_attempt(QUEUE, ENV_ID, 2), DEFAULT_TTL_SECONDS)
            return a0, a1, a2

        a0, a1, a2 = asyncio.run(run())
        self.assertEqual((a0, a1, a2), (True, True, True))


if __name__ == "__main__":
    unittest.main()
