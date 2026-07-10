"""
Metric Collectors.

Provides Counter, Gauge, and Histogram metric types for tracking operations.
Part of the CORE libraries - metrics library.
"""

import time
from typing import Any, Dict, List, Optional, Tuple
from collections import defaultdict


class Counter:
    """Counter metric - tracks cumulative counts.

    Use for: Events that accumulate (requests, errors, items processed).

    Example:
        processed = Counter('chunks_processed', labels=['stage_name'])
        processed.inc(labels={'stage_name': 'extraction'})
        processed.inc(amount=10, labels={'stage_name': 'extraction'})

        count = processed.get(labels={'stage_name': 'extraction'})  # Returns 11
    """

    def __init__(
        self, name: str, description: str = "", labels: Optional[List[str]] = None
    ):
        """Initialize Counter.

        Args:
            name: Metric name
            description: Human-readable description
            labels: List of label names for this metric
        """
        self.name = name
        self.description = description
        self.labels = labels or []
        self._values: Dict[Tuple, float] = defaultdict(float)

    def inc(self, amount: float = 1.0, labels: Optional[Dict[str, Any]] = None) -> None:
        """Increment counter.

        Args:
            amount: Amount to increment by (default: 1.0)
            labels: Label values as dict
        """
        key = self._make_key(labels)
        self._values[key] += amount

    def get(self, labels: Optional[Dict[str, Any]] = None) -> float:
        """Get current counter value.

        Args:
            labels: Label values to filter by

        Returns:
            Current counter value
        """
        key = self._make_key(labels)
        return self._values.get(key, 0.0)

    def reset(self, labels: Optional[Dict[str, Any]] = None) -> None:
        """Reset counter to zero.

        Args:
            labels: Label values to reset (if None, reset all)
        """
        if labels is None:
            self._values.clear()
        else:
            key = self._make_key(labels)
            if key in self._values:
                self._values[key] = 0.0

    def get_all(self) -> Dict[Tuple, float]:
        """Get all counter values with labels.

        Returns:
            Dict mapping label tuples to values
        """
        return dict(self._values)

    def _make_key(self, labels: Optional[Dict[str, Any]]) -> Tuple:
        """Create tuple key from labels dict.

        Args:
            labels: Label values as dict

        Returns:
            Tuple of (key, value) pairs sorted by key
        """
        if not labels:
            return ()

        # Sort by key for consistency
        return tuple(sorted(labels.items()))


class Gauge:
    """Gauge metric - tracks current value (can go up or down).

    Use for: Current state (queue size, active connections, memory usage).

    Example:
        queue_size = Gauge('queue_size', labels=['queue_name'])
        queue_size.set(100, labels={'queue_name': 'extraction'})
        queue_size.inc(10, labels={'queue_name': 'extraction'})  # Now 110
        queue_size.dec(5, labels={'queue_name': 'extraction'})   # Now 105

        current = queue_size.get(labels={'queue_name': 'extraction'})  # Returns 105
    """

    def __init__(
        self, name: str, description: str = "", labels: Optional[List[str]] = None
    ):
        """Initialize Gauge.

        Args:
            name: Metric name
            description: Human-readable description
            labels: List of label names
        """
        self.name = name
        self.description = description
        self.labels = labels or []
        self._values: Dict[Tuple, float] = {}

    def set(self, value: float, labels: Optional[Dict[str, Any]] = None) -> None:
        """Set gauge to specific value.

        Args:
            value: New value
            labels: Label values
        """
        key = self._make_key(labels)
        self._values[key] = value

    def inc(self, amount: float = 1.0, labels: Optional[Dict[str, Any]] = None) -> None:
        """Increment gauge value.

        Args:
            amount: Amount to increment by
            labels: Label values
        """
        key = self._make_key(labels)
        self._values[key] = self._values.get(key, 0.0) + amount

    def dec(self, amount: float = 1.0, labels: Optional[Dict[str, Any]] = None) -> None:
        """Decrement gauge value.

        Args:
            amount: Amount to decrement by
            labels: Label values
        """
        key = self._make_key(labels)
        self._values[key] = self._values.get(key, 0.0) - amount

    def get(self, labels: Optional[Dict[str, Any]] = None) -> float:
        """Get current gauge value.

        Args:
            labels: Label values

        Returns:
            Current value (0.0 if not set)
        """
        key = self._make_key(labels)
        return self._values.get(key, 0.0)

    def get_all(self) -> Dict[Tuple, float]:
        """Get all gauge values with labels."""
        return dict(self._values)

    def _make_key(self, labels: Optional[Dict[str, Any]]) -> Tuple:
        """Create tuple key from labels dict."""
        if not labels:
            return ()
        return tuple(sorted(labels.items()))


class Histogram:
    """Histogram metric - tracks distribution of values.

    Use for: Durations, sizes, counts that need statistics (min/max/avg/percentiles).

    Example:
        duration = Histogram('stage_duration_seconds', labels=['stage_name'])
        duration.observe(10.5, labels={'stage_name': 'extraction'})
        duration.observe(12.3, labels={'stage_name': 'extraction'})

        stats = duration.summary(labels={'stage_name': 'extraction'})
        # Returns: {'count': 2, 'sum': 22.8, 'min': 10.5, 'max': 12.3, 'avg': 11.4}
    """

    def __init__(
        self, name: str, description: str = "", labels: Optional[List[str]] = None
    ):
        """Initialize Histogram.

        Args:
            name: Metric name
            description: Human-readable description
            labels: List of label names
        """
        self.name = name
        self.description = description
        self.labels = labels or []
        self._observations: Dict[Tuple, List[float]] = defaultdict(list)

    def observe(self, value: float, labels: Optional[Dict[str, Any]] = None) -> None:
        """Observe a value.

        Args:
            value: Value to observe
            labels: Label values
        """
        key = self._make_key(labels)
        self._observations[key].append(value)

    def summary(self, labels: Optional[Dict[str, Any]] = None) -> Dict[str, float]:
        """Get statistical summary of observed values.

        Args:
            labels: Label values to filter by

        Returns:
            Dict with count, sum, min, max, avg
        """
        key = self._make_key(labels)
        values = self._observations.get(key, [])

        if not values:
            return {
                "count": 0,
                "sum": 0.0,
                "min": 0.0,
                "max": 0.0,
                "avg": 0.0,
            }

        return {
            "count": len(values),
            "sum": sum(values),
            "min": min(values),
            "max": max(values),
            "avg": sum(values) / len(values),
        }

    def percentile(self, p: float, labels: Optional[Dict[str, Any]] = None) -> float:
        """Calculate percentile.

        Args:
            p: Percentile (0.0-1.0), e.g., 0.95 for 95th percentile
            labels: Label values

        Returns:
            Value at percentile
        """
        key = self._make_key(labels)
        values = sorted(self._observations.get(key, []))

        if not values:
            return 0.0

        index = int(len(values) * p)
        if index >= len(values):
            index = len(values) - 1

        return values[index]

    def reset(self, labels: Optional[Dict[str, Any]] = None) -> None:
        """Reset histogram observations.

        Args:
            labels: Label values to reset (if None, reset all)
        """
        if labels is None:
            self._observations.clear()
        else:
            key = self._make_key(labels)
            if key in self._observations:
                self._observations[key] = []

    def get_all(self) -> Dict[Tuple, List[float]]:
        """Get all observations with labels."""
        return dict(self._observations)

    def _make_key(self, labels: Optional[Dict[str, Any]]) -> Tuple:
        """Create tuple key from labels dict."""
        if not labels:
            return ()
        return tuple(sorted(labels.items()))


class Timer:
    """Timer utility for measuring durations.

    Context manager for convenient timing.

    Example:
        duration_hist = Histogram('operation_duration')

        with Timer() as timer:
            do_work()

        duration_hist.observe(timer.elapsed())
    """

    def __init__(self):
        """Initialize Timer."""
        self.start_time: Optional[float] = None
        self.end_time: Optional[float] = None

    def __enter__(self):
        """Start timer."""
        self.start_time = time.time()
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        """Stop timer."""
        self.end_time = time.time()
        return False

    def elapsed(self) -> float:
        """Get elapsed time in seconds.

        Returns:
            Elapsed time, or 0.0 if not finished
        """
        if self.start_time is None:
            return 0.0

        end = self.end_time if self.end_time is not None else time.time()
        return end - self.start_time
