"""
Metric Exporters.

Simple Prometheus text format exporter.
Part of the CORE libraries - metrics library.
"""

from typing import Dict
from src.lib.metrics.collectors import Counter, Gauge, Histogram
from src.lib.metrics.registry import MetricRegistry


def export_prometheus_text() -> str:
    """Export all metrics in Prometheus text format.

    Returns:
        Metrics in Prometheus exposition format

    Example:
        text = export_prometheus_text()
        # Returns:
        # # TYPE chunks_processed counter
        # chunks_processed{stage="extraction"} 1000
        # # TYPE stage_duration_seconds histogram
        # stage_duration_seconds_sum{stage="extraction"} 3600.0
        # ...
    """
    registry = MetricRegistry.get_instance()
    metrics = registry.collect_all()

    lines = []

    for name, metric in metrics.items():
        if isinstance(metric, Counter):
            lines.extend(_export_counter(metric))
        elif isinstance(metric, Gauge):
            lines.extend(_export_gauge(metric))
        elif isinstance(metric, Histogram):
            lines.extend(_export_histogram(metric))

    return "\n".join(lines)


def _export_counter(counter: Counter) -> list:
    """Export counter in Prometheus format."""
    lines = []

    # Type declaration
    lines.append(f"# TYPE {counter.name} counter")
    if counter.description:
        lines.append(f"# HELP {counter.name} {counter.description}")

    # Values
    all_values = counter.get_all()
    if not all_values:
        # Export 0 if no values
        lines.append(f"{counter.name} 0")
    else:
        for label_tuple, value in all_values.items():
            if label_tuple:
                label_str = ",".join(f'{k}="{v}"' for k, v in label_tuple)
                lines.append(f"{counter.name}{{{label_str}}} {value}")
            else:
                lines.append(f"{counter.name} {value}")

    return lines


def _export_gauge(gauge: Gauge) -> list:
    """Export gauge in Prometheus format."""
    lines = []

    # Type declaration
    lines.append(f"# TYPE {gauge.name} gauge")
    if gauge.description:
        lines.append(f"# HELP {gauge.name} {gauge.description}")

    # Values
    all_values = gauge.get_all()
    if not all_values:
        lines.append(f"{gauge.name} 0")
    else:
        for label_tuple, value in all_values.items():
            if label_tuple:
                label_str = ",".join(f'{k}="{v}"' for k, v in label_tuple)
                lines.append(f"{gauge.name}{{{label_str}}} {value}")
            else:
                lines.append(f"{gauge.name} {value}")

    return lines


def _export_histogram(histogram: Histogram) -> list:
    """Export histogram in Prometheus format (simplified).

    Exports as summary type with _sum, _count, _min, _max, _avg suffixes.
    Note: Full Prometheus histograms use buckets - we use simplified version.
    """
    lines = []

    # Type declaration
    lines.append(f"# TYPE {histogram.name} summary")
    if histogram.description:
        lines.append(f"# HELP {histogram.name} {histogram.description}")

    # Get all label combinations
    all_observations = histogram.get_all()

    if not all_observations:
        # Export zeros if no observations
        lines.append(f"{histogram.name}_count 0")
        lines.append(f"{histogram.name}_sum 0")
    else:
        for label_tuple, _ in all_observations.items():
            # Reconstruct labels dict for summary call
            labels_dict = dict(label_tuple) if label_tuple else None
            stats = histogram.summary(labels=labels_dict)

            # Format labels
            if label_tuple:
                label_str = ",".join(f'{k}="{v}"' for k, v in label_tuple)
                label_part = f"{{{label_str}}}"
            else:
                label_part = ""

            # Export summary statistics
            lines.append(f"{histogram.name}_count{label_part} {stats['count']}")
            lines.append(f"{histogram.name}_sum{label_part} {stats['sum']}")
            lines.append(f"{histogram.name}_min{label_part} {stats['min']}")
            lines.append(f"{histogram.name}_max{label_part} {stats['max']}")
            lines.append(f"{histogram.name}_avg{label_part} {stats['avg']}")

    return lines


# TODO: HTTP server for /metrics endpoint
# For now, metrics can be exported to file or retrieved programmatically
# Full HTTP server can be added when needed (FastAPI endpoint, simple HTTP server, etc.)
