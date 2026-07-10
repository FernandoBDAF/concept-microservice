"""
Metrics Library - Cross-Cutting Concern.

Provides simple metrics collection and Prometheus export.
Part of the CORE libraries - Tier 1 (basic implementation).

Usage:
    from src.lib.metrics import Counter, Histogram, MetricRegistry

    # Create metrics
    processed = Counter('chunks_processed', labels=['stage'])
    duration = Histogram('stage_duration_seconds', labels=['stage'])

    # Register for export
    registry = MetricRegistry.get_instance()
    registry.register(processed)
    registry.register(duration)

    # Use in code
    processed.inc(labels={'stage': 'extraction'})
    duration.observe(10.5, labels={'stage': 'extraction'})

    # Export for Prometheus
    from src.lib.metrics import export_prometheus_text
    metrics_text = export_prometheus_text()
"""

from src.lib.metrics.collectors import Counter, Gauge, Histogram, Timer
from src.lib.metrics.registry import MetricRegistry
from src.lib.metrics.exporters import export_prometheus_text
from src.lib.metrics.cost_models import (
    estimate_llm_cost,
    add_model_pricing,
    LLM_PRICING,
)


__all__ = [
    # Collectors
    "Counter",
    "Gauge",
    "Histogram",
    "Timer",
    # Registry
    "MetricRegistry",
    # Exporters
    "export_prometheus_text",
    # Cost Models
    "estimate_llm_cost",
    "add_model_pricing",
    "LLM_PRICING",
]
