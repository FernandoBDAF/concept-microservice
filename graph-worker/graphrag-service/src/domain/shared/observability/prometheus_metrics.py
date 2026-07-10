"""
Prometheus Metrics Service for GraphRAG Pipeline

Achievement 1.1: Prometheus Metrics Export

Exports pipeline metrics in Prometheus format for scraping.
"""

import logging
from typing import Dict, Any, Optional
from src.lib.metrics import (
    Counter,
    Gauge,
    Histogram,
    MetricRegistry,
    export_prometheus_text,
)

logger = logging.getLogger(__name__)

# Pipeline-level metrics
_pipeline_status = Gauge(
    "graphrag_pipeline_status",
    "Current pipeline status (0=idle, 1=running, 2=completed, 3=failed)",
    labels=["pipeline_id"],
)

_pipeline_stage_progress = Gauge(
    "graphrag_pipeline_stage_progress",
    "Stage progress (chunks_processed / chunks_total)",
    labels=["pipeline_id", "stage"],
)

_pipeline_stage_chunks_total = Counter(
    "graphrag_pipeline_stage_chunks_total",
    "Total chunks to process per stage",
    labels=["pipeline_id", "stage"],
)

_pipeline_stage_chunks_processed = Counter(
    "graphrag_pipeline_stage_chunks_processed",
    "Chunks processed per stage",
    labels=["pipeline_id", "stage"],
)

_pipeline_stage_chunks_failed = Counter(
    "graphrag_pipeline_stage_chunks_failed",
    "Chunks failed per stage",
    labels=["pipeline_id", "stage"],
)

# Throughput metrics
_pipeline_throughput_entities = Gauge(
    "graphrag_pipeline_throughput_entities_per_sec",
    "Entities processed per second",
    labels=["pipeline_id", "stage"],
)

_pipeline_throughput_relationships = Gauge(
    "graphrag_pipeline_throughput_relationships_per_sec",
    "Relationships processed per second",
    labels=["pipeline_id", "stage"],
)

_pipeline_throughput_communities = Gauge(
    "graphrag_pipeline_throughput_communities_per_sec",
    "Communities processed per second",
    labels=["pipeline_id", "stage"],
)

# Latency metrics
_pipeline_stage_duration = Histogram(
    "graphrag_pipeline_stage_duration_seconds",
    "Stage execution duration in seconds",
    labels=["pipeline_id", "stage"],
)

_pipeline_chunk_processing_time = Histogram(
    "graphrag_pipeline_chunk_processing_time_seconds",
    "Average processing time per chunk in seconds",
    labels=["pipeline_id", "stage"],
)

# Error metrics
_pipeline_stage_errors = Counter(
    "graphrag_pipeline_stage_errors_total",
    "Total errors per stage",
    labels=["pipeline_id", "stage", "error_type"],
)

# Register all metrics
_registry = MetricRegistry.get_instance()
_registry.register(_pipeline_status)
_registry.register(_pipeline_stage_progress)
_registry.register(_pipeline_stage_chunks_total)
_registry.register(_pipeline_stage_chunks_processed)
_registry.register(_pipeline_stage_chunks_failed)
_registry.register(_pipeline_throughput_entities)
_registry.register(_pipeline_throughput_relationships)
_registry.register(_pipeline_throughput_communities)
_registry.register(_pipeline_stage_duration)
_registry.register(_pipeline_chunk_processing_time)
_registry.register(_pipeline_stage_errors)


class PipelineMetricsTracker:
    """Track pipeline metrics for Prometheus export."""

    def __init__(self, pipeline_id: str = "default"):
        """
        Initialize metrics tracker.

        Args:
            pipeline_id: Unique identifier for this pipeline run
        """
        self.pipeline_id = pipeline_id
        self._stage_start_times: Dict[str, float] = {}

    def set_pipeline_status(self, status: str) -> None:
        """
        Set pipeline status.

        Args:
            status: One of "idle", "running", "completed", "failed"
        """
        status_map = {"idle": 0, "running": 1, "completed": 2, "failed": 3}
        status_value = status_map.get(status.lower(), 0)
        _pipeline_status.set(status_value, labels={"pipeline_id": self.pipeline_id})

    def set_stage_chunks_total(self, stage: str, total: int) -> None:
        """
        Set total chunks for a stage.

        Args:
            stage: Stage name
            total: Total number of chunks
        """
        _pipeline_stage_chunks_total.inc(
            total, labels={"pipeline_id": self.pipeline_id, "stage": stage}
        )

    def inc_stage_chunks_processed(self, stage: str, count: int = 1) -> None:
        """
        Increment processed chunks for a stage.

        Args:
            stage: Stage name
            count: Number of chunks processed
        """
        _pipeline_stage_chunks_processed.inc(
            count, labels={"pipeline_id": self.pipeline_id, "stage": stage}
        )

    def inc_stage_chunks_failed(self, stage: str, count: int = 1) -> None:
        """
        Increment failed chunks for a stage.

        Args:
            stage: Stage name
            count: Number of chunks failed
        """
        _pipeline_stage_chunks_failed.inc(
            count, labels={"pipeline_id": self.pipeline_id, "stage": stage}
        )

    def update_stage_progress(self, stage: str, processed: int, total: int) -> None:
        """
        Update stage progress.

        Args:
            stage: Stage name
            processed: Number of chunks processed
            total: Total number of chunks
        """
        progress = processed / total if total > 0 else 0.0
        _pipeline_stage_progress.set(
            progress, labels={"pipeline_id": self.pipeline_id, "stage": stage}
        )

    def set_throughput_entities(self, stage: str, entities_per_sec: float) -> None:
        """
        Set entities throughput.

        Args:
            stage: Stage name
            entities_per_sec: Entities processed per second
        """
        _pipeline_throughput_entities.set(
            entities_per_sec,
            labels={"pipeline_id": self.pipeline_id, "stage": stage},
        )

    def set_throughput_relationships(self, stage: str, relationships_per_sec: float) -> None:
        """
        Set relationships throughput.

        Args:
            stage: Stage name
            relationships_per_sec: Relationships processed per second
        """
        _pipeline_throughput_relationships.set(
            relationships_per_sec,
            labels={"pipeline_id": self.pipeline_id, "stage": stage},
        )

    def set_throughput_communities(self, stage: str, communities_per_sec: float) -> None:
        """
        Set communities throughput.

        Args:
            stage: Stage name
            communities_per_sec: Communities processed per second
        """
        _pipeline_throughput_communities.set(
            communities_per_sec,
            labels={"pipeline_id": self.pipeline_id, "stage": stage},
        )

    def record_stage_duration(self, stage: str, duration_seconds: float) -> None:
        """
        Record stage execution duration.

        Args:
            stage: Stage name
            duration_seconds: Duration in seconds
        """
        _pipeline_stage_duration.observe(
            duration_seconds, labels={"pipeline_id": self.pipeline_id, "stage": stage}
        )

    def record_chunk_processing_time(self, stage: str, time_seconds: float) -> None:
        """
        Record chunk processing time.

        Args:
            stage: Stage name
            time_seconds: Processing time in seconds
        """
        _pipeline_chunk_processing_time.observe(
            time_seconds, labels={"pipeline_id": self.pipeline_id, "stage": stage}
        )

    def inc_stage_error(self, stage: str, error_type: str = "unknown") -> None:
        """
        Increment error count for a stage.

        Args:
            stage: Stage name
            error_type: Type of error (e.g., "validation", "llm_error", "timeout")
        """
        _pipeline_stage_errors.inc(
            labels={
                "pipeline_id": self.pipeline_id,
                "stage": stage,
                "error_type": error_type,
            }
        )

    def start_stage(self, stage: str) -> None:
        """Mark stage start time."""
        import time

        self._stage_start_times[stage] = time.time()

    def complete_stage(self, stage: str) -> None:
        """Record stage completion duration."""
        import time

        if stage in self._stage_start_times:
            duration = time.time() - self._stage_start_times[stage]
            self.record_stage_duration(stage, duration)
            del self._stage_start_times[stage]


def get_metrics_text() -> str:
    """
    Get all metrics in Prometheus text format.

    Achievement 1.1: Prometheus Metrics Export

    Returns:
        Metrics in Prometheus exposition format
    """
    return export_prometheus_text()


# Global tracker instance (can be replaced per pipeline run)
_global_tracker: Optional[PipelineMetricsTracker] = None


def get_metrics_tracker(pipeline_id: str = "default") -> PipelineMetricsTracker:
    """
    Get or create metrics tracker.

    Args:
        pipeline_id: Unique identifier for pipeline run

    Returns:
        PipelineMetricsTracker instance
    """
    global _global_tracker
    if _global_tracker is None or _global_tracker.pipeline_id != pipeline_id:
        _global_tracker = PipelineMetricsTracker(pipeline_id=pipeline_id)
    return _global_tracker


