"""
Stages API Prometheus Metrics

Exports pipeline execution metrics in Prometheus format for monitoring.
These metrics support the stages-api-dashboard.json in Grafana.
"""

import logging
from typing import Optional
from src.lib.metrics import (
    Counter,
    Gauge,
    Histogram,
    MetricRegistry,
    export_prometheus_text,
)

logger = logging.getLogger(__name__)

# =============================================================================
# Metric Definitions
# =============================================================================

# Active pipelines (gauge - can go up/down)
_active_pipelines = Gauge(
    "stages_api_active_pipelines",
    "Number of currently active pipelines",
)

# Pipeline executions counter (by pipeline type and status)
_pipeline_executions = Counter(
    "stages_api_pipeline_executions",
    "Total pipeline executions",
    labels=["pipeline", "status"],
)

# Pipeline duration histogram (by pipeline type)
_pipeline_duration = Histogram(
    "stages_api_pipeline_duration_seconds",
    "Pipeline execution duration in seconds",
    labels=["pipeline"],
)

# Stage executions counter (by stage and status)
_stage_executions = Counter(
    "stages_api_stage_executions",
    "Total stage executions",
    labels=["stage", "status"],
)

# Stage duration histogram
_stage_duration = Histogram(
    "stages_api_stage_duration_seconds",
    "Stage execution duration in seconds",
    labels=["stage"],
)

# Error counter
_errors = Counter(
    "stages_api_errors_total",
    "Total errors by type",
    labels=["error_type", "stage"],
)

# Register all metrics
_registry = MetricRegistry.get_instance()
_registry.register(_active_pipelines)
_registry.register(_pipeline_executions)
_registry.register(_pipeline_duration)
_registry.register(_stage_executions)
_registry.register(_stage_duration)
_registry.register(_errors)


# =============================================================================
# Metrics Tracker
# =============================================================================

class StagesMetricsTracker:
    """Track stages API metrics for Prometheus export."""

    _instance: Optional["StagesMetricsTracker"] = None

    @classmethod
    def get_instance(cls) -> "StagesMetricsTracker":
        """Get singleton instance."""
        if cls._instance is None:
            cls._instance = StagesMetricsTracker()
        return cls._instance

    def pipeline_started(self, pipeline: str) -> None:
        """Record pipeline start."""
        _active_pipelines.inc()
        _pipeline_executions.inc(labels={"pipeline": pipeline, "status": "started"})

    def pipeline_completed(self, pipeline: str, duration_seconds: float) -> None:
        """Record pipeline completion."""
        _active_pipelines.dec()
        _pipeline_executions.inc(labels={"pipeline": pipeline, "status": "completed"})
        _pipeline_duration.observe(duration_seconds, labels={"pipeline": pipeline})

    def pipeline_failed(self, pipeline: str, error_type: str = "unknown") -> None:
        """Record pipeline failure."""
        _active_pipelines.dec()
        _pipeline_executions.inc(labels={"pipeline": pipeline, "status": "failed"})
        _errors.inc(labels={"error_type": error_type, "stage": "_pipeline"})

    def stage_started(self, stage: str) -> None:
        """Record stage start."""
        _stage_executions.inc(labels={"stage": stage, "status": "started"})

    def stage_completed(self, stage: str, duration_seconds: float) -> None:
        """Record stage completion."""
        _stage_executions.inc(labels={"stage": stage, "status": "completed"})
        _stage_duration.observe(duration_seconds, labels={"stage": stage})

    def stage_failed(self, stage: str, error_type: str = "unknown") -> None:
        """Record stage failure."""
        _stage_executions.inc(labels={"stage": stage, "status": "failed"})
        _errors.inc(labels={"error_type": error_type, "stage": stage})

    def set_active_pipelines(self, count: int) -> None:
        """Set active pipeline count directly (for reconciliation)."""
        _active_pipelines.set(count)


# =============================================================================
# Public API
# =============================================================================

def get_prometheus_metrics() -> str:
    """
    Get all stages API metrics in Prometheus text format.
    
    Returns:
        Metrics in Prometheus exposition format
    """
    try:
        return export_prometheus_text()
    except Exception as e:
        logger.error(f"Error exporting Prometheus metrics: {e}")
        return f"# Error: {str(e)}\n"


def get_metrics_tracker() -> StagesMetricsTracker:
    """
    Get the singleton metrics tracker.
    
    Returns:
        StagesMetricsTracker instance
    """
    return StagesMetricsTracker.get_instance()

