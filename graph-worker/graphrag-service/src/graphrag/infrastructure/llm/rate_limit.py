import os
import random
import threading
import time


class RateLimiter:
    """Simple process-local rate limiter.

    Enforces ~RPM requests per minute with optional jitter and supports
    explicit delays (e.g., Retry-After).
    """

    def __init__(self, rpm: int | None = None, jitter_ms: int | None = None) -> None:
        rpm_val = int(os.getenv("VOYAGE_RPM", str(rpm if rpm is not None else 20)))
        jitter_val = int(
            os.getenv(
                "VOYAGE_JITTER_MS", str(jitter_ms if jitter_ms is not None else 250)
            )
        )
        self.min_interval_seconds = 60.0 / max(1, rpm_val)
        self.jitter_seconds = max(0.0, jitter_val / 1000.0)
        self._next_allowed_ts = 0.0
        self._lock = threading.Lock()

    def wait(self) -> None:
        with self._lock:
            now = time.monotonic()
            sleep_for = max(0.0, self._next_allowed_ts - now)
            if sleep_for > 0:
                time.sleep(sleep_for)
            # schedule next slot
            self._next_allowed_ts = (
                time.monotonic()
                + self.min_interval_seconds
                + random.uniform(0.0, self.jitter_seconds)
            )

    def delay(self, seconds: float) -> None:
        with self._lock:
            self._next_allowed_ts = max(
                self._next_allowed_ts,
                time.monotonic()
                + max(0.0, seconds)
                + random.uniform(0.0, self.jitter_seconds),
            )
