"""Unit tests for the x-death retry-tier selection (src/worker/retry.py).

Pure functions, no broker needed. Run from the graphrag-service directory:

    python3 -m unittest tests.test_retry
"""

import os
import sys
import unittest

# Make `src` importable when run as `python3 -m unittest` from the service dir.
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from src.worker.retry import (  # noqa: E402
    MAX_RETRIES,
    RETRY_TIERS,
    death_count,
    select_tier,
)

QUEUE_PREFIX = "document-processing.retry."


def _xdeath(*entries):
    """Wrap x-death entries into a header dict as aio-pika exposes it."""
    return {"x-death": list(entries)}


def _entry(queue, count, reason="expired"):
    return {
        "count": count,
        "reason": reason,
        "queue": queue,
        "exchange": "document-tasks.retry",
    }


class SelectTierTest(unittest.TestCase):
    def test_progression(self):
        self.assertEqual(select_tier(0), "5s")
        self.assertEqual(select_tier(1), "30s")
        self.assertEqual(select_tier(2), "2m")

    def test_exhausted_returns_none(self):
        self.assertIsNone(select_tier(MAX_RETRIES))
        self.assertIsNone(select_tier(3))
        self.assertIsNone(select_tier(9))

    def test_negative_clamped_to_first_tier(self):
        self.assertEqual(select_tier(-1), "5s")

    def test_tiers_match_definitions(self):
        # The labels must match the wait-queue suffixes in definitions.json.
        self.assertEqual(RETRY_TIERS, ["5s", "30s", "2m"])


class DeathCountTest(unittest.TestCase):
    def test_none_and_empty(self):
        self.assertEqual(death_count(None, QUEUE_PREFIX), 0)
        self.assertEqual(death_count({}, QUEUE_PREFIX), 0)
        self.assertEqual(death_count({"x-death": []}, QUEUE_PREFIX), 0)

    def test_first_retry(self):
        headers = _xdeath(_entry("document-processing.retry.5s", 1))
        self.assertEqual(death_count(headers, QUEUE_PREFIX), 1)
        self.assertEqual(select_tier(death_count(headers, QUEUE_PREFIX)), "30s")

    def test_two_tiers_sum(self):
        headers = _xdeath(
            _entry("document-processing.retry.30s", 1),
            _entry("document-processing.retry.5s", 1),
        )
        self.assertEqual(death_count(headers, QUEUE_PREFIX), 2)
        self.assertEqual(select_tier(death_count(headers, QUEUE_PREFIX)), "2m")

    def test_exhausted_after_three(self):
        headers = _xdeath(
            _entry("document-processing.retry.5s", 1),
            _entry("document-processing.retry.30s", 1),
            _entry("document-processing.retry.2m", 1),
        )
        self.assertEqual(death_count(headers, QUEUE_PREFIX), 3)
        self.assertIsNone(select_tier(death_count(headers, QUEUE_PREFIX)))

    def test_non_retry_entries_excluded(self):
        # A rejected/main-queue entry and a foreign-queue entry must not count.
        headers = _xdeath(
            _entry("document-processing", 5, reason="rejected"),
            _entry("email-processing.retry.5s", 4),
        )
        self.assertEqual(death_count(headers, QUEUE_PREFIX), 0)

    def test_count_greater_than_one_summed(self):
        headers = _xdeath(_entry("document-processing.retry.5s", 2))
        self.assertEqual(death_count(headers, QUEUE_PREFIX), 2)

    def test_bytes_queue_name(self):
        headers = _xdeath(_entry(b"document-processing.retry.5s", 1))
        self.assertEqual(death_count(headers, QUEUE_PREFIX), 1)

    def test_malformed_entries_ignored(self):
        headers = {"x-death": ["not-a-dict", {"queue": "document-processing.retry.5s"}]}
        # Missing count defaults to 0; string entry skipped.
        self.assertEqual(death_count(headers, QUEUE_PREFIX), 0)


if __name__ == "__main__":
    unittest.main()
