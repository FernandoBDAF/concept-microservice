from __future__ import annotations

import time
from typing import Optional


class Timer:
    """Small context manager to measure elapsed seconds for logging."""

    def __init__(self, label: str = "") -> None:
        self.label = label
        self.start: Optional[float] = None
        self.elapsed: float = 0.0

    def __enter__(self) -> "Timer":
        self.start = time.perf_counter()
        return self

    def __exit__(self, exc_type, exc, tb) -> None:  # type: ignore[override]
        end = time.perf_counter()
        self.elapsed = end - (self.start or end)
