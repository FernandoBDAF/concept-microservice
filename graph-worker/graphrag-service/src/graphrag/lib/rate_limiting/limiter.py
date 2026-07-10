"""
Rate limiting implementation using token bucket algorithm.
"""

import os
import random
import threading
import time
import logging
from typing import Optional
from functools import wraps

logger = logging.getLogger(__name__)


class RateLimiter:
    """Simple process-local rate limiter using token bucket algorithm.

    Enforces maximum requests per minute with optional jitter and supports
    explicit delays (e.g., Retry-After headers).

    Thread-safe for concurrent usage.
    """

    def __init__(
        self,
        rpm: Optional[int] = None,
        jitter_ms: Optional[int] = None,
        name: str = "default",
    ) -> None:
        """Initialize rate limiter.

        Args:
            rpm: Requests per minute limit (default: 20, configurable via env)
            jitter_ms: Jitter in milliseconds (default: 250, configurable via env)
            name: Name for this rate limiter (for logging)
        """
        rpm_val = int(os.getenv("RATE_LIMIT_RPM", str(rpm if rpm is not None else 20)))
        jitter_val = int(
            os.getenv(
                "RATE_LIMIT_JITTER_MS",
                str(jitter_ms if jitter_ms is not None else 250),
            )
        )

        self.rpm = rpm_val
        self.min_interval_seconds = 60.0 / max(1, rpm_val)
        self.jitter_seconds = max(0.0, jitter_val / 1000.0)
        self.name = name
        self._next_allowed_ts = 0.0
        self._lock = threading.Lock()

        logger.debug(
            f"Initialized rate limiter '{name}': {rpm_val} RPM, "
            f"{jitter_val}ms jitter"
        )

    def wait(self) -> None:
        """Wait until rate limit allows next request."""
        with self._lock:
            now = time.monotonic()
            sleep_for = max(0.0, self._next_allowed_ts - now)

            if sleep_for > 0:
                logger.debug(f"Rate limiter '{self.name}' waiting {sleep_for:.3f}s")
                time.sleep(sleep_for)

            # Schedule next slot
            self._next_allowed_ts = (
                time.monotonic()
                + self.min_interval_seconds
                + random.uniform(0.0, self.jitter_seconds)
            )

    def delay(self, seconds: float) -> None:
        """Add explicit delay to rate limiter (e.g., from Retry-After header).

        Args:
            seconds: Seconds to delay
        """
        with self._lock:
            self._next_allowed_ts = max(
                self._next_allowed_ts,
                time.monotonic()
                + max(0.0, seconds)
                + random.uniform(0.0, self.jitter_seconds),
            )
            logger.debug(f"Rate limiter '{self.name}' delayed by {seconds}s")

    def __enter__(self):
        """Context manager entry - wait for rate limit."""
        self.wait()
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        """Context manager exit."""
        return False


def rate_limit(
    max_calls: int = 60,
    period: float = 60.0,
    jitter_ms: int = 250,
    name: Optional[str] = None,
):
    """Decorator to rate limit function calls.

    Args:
        max_calls: Maximum calls allowed in period
        period: Time period in seconds (default: 60s)
        jitter_ms: Jitter in milliseconds
        name: Optional name for the rate limiter

    Usage:
        @rate_limit(max_calls=10, period=60)
        def call_api():
            # Limited to 10 calls per minute
            ...
    """
    rpm = int((max_calls / period) * 60)
    limiter_name = name or "decorator"
    limiter = RateLimiter(rpm=rpm, jitter_ms=jitter_ms, name=limiter_name)

    def decorator(func):
        @wraps(func)
        def wrapper(*args, **kwargs):
            limiter.wait()
            return func(*args, **kwargs)

        return wrapper

    return decorator
