"""
Metric Registry.

Simple singleton registry for managing all metrics.
Part of the CORE libraries - metrics library.
"""

from typing import Dict, Optional, Union
from src.lib.metrics.collectors import Counter, Gauge, Histogram


# Global error counter (auto-populated by log_exception)
_errors_total = Counter(
    "errors_total",
    "Total errors by type and component",
    labels=["error_type", "component"],
)

# Global retry counter (auto-populated by @with_retry)
_retries_attempted = Counter(
    "retries_attempted",
    "Total retry attempts by function and error type",
    labels=["function", "error_type"],
)


class MetricRegistry:
    """Singleton registry for all application metrics.

    Provides centralized access to metrics for collection and export.

    Example:
        from src.lib.metrics import MetricRegistry, Counter

        registry = MetricRegistry.get_instance()

        # Create and register metric
        processed = Counter('chunks_processed')
        registry.register(processed)

        # Later, retrieve it
        metric = registry.get('chunks_processed')

        # Or collect all for export
        all_metrics = registry.collect_all()
    """

    _instance: Optional["MetricRegistry"] = None

    def __init__(self):
        """Initialize registry (use get_instance() instead)."""
        self.metrics: Dict[str, Union[Counter, Gauge, Histogram]] = {}
        # Auto-register global counters
        self.metrics["errors_total"] = _errors_total
        self.metrics["retries_attempted"] = _retries_attempted

    @classmethod
    def get_instance(cls) -> "MetricRegistry":
        """Get singleton registry instance.

        Returns:
            MetricRegistry instance
        """
        if cls._instance is None:
            cls._instance = MetricRegistry()
        return cls._instance

    def register(self, metric: Union[Counter, Gauge, Histogram]) -> None:
        """Register a metric.

        Args:
            metric: Metric instance to register
        """
        self.metrics[metric.name] = metric

    def get(self, name: str) -> Optional[Union[Counter, Gauge, Histogram]]:
        """Get metric by name.

        Args:
            name: Metric name

        Returns:
            Metric instance or None if not found
        """
        return self.metrics.get(name)

    def collect_all(self) -> Dict[str, Union[Counter, Gauge, Histogram]]:
        """Collect all registered metrics.

        Returns:
            Dict mapping metric names to metric instances
        """
        return dict(self.metrics)

    def reset_all(self) -> None:
        """Reset all metrics to initial state."""
        for metric in self.metrics.values():
            metric.reset()

    @classmethod
    def reset_instance(cls) -> None:
        """Reset singleton (useful for testing)."""
        cls._instance = None
