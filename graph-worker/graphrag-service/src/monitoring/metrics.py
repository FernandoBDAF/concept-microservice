import time
from contextlib import contextmanager

from prometheus_client import Counter, Gauge, Histogram


class PrometheusMetrics:
    """Prometheus metrics for GraphRAG worker."""

    def __init__(self) -> None:
        self.messages_processed = Counter(
            "graphrag_messages_processed_total",
            "Total messages processed",
            ["status"],
        )

        self.documents_processed = Counter(
            "graphrag_documents_processed_total",
            "Total documents processed",
        )

        self.processing_duration = Histogram(
            "graphrag_processing_duration_seconds",
            "Document processing duration",
            buckets=[60, 120, 300, 600, 900, 1200, 1800, 3600],
        )

        self.current_processing = Gauge(
            "graphrag_currently_processing",
            "Number of documents currently being processed",
        )

        self.last_success_timestamp = Gauge(
            "graphrag_last_success_timestamp",
            "Timestamp of last successful processing",
        )

    def record_success(self) -> None:
        self.messages_processed.labels(status="success").inc()
        self.documents_processed.inc()
        self.last_success_timestamp.set_to_current_time()

    def record_error(self, error_type: str) -> None:
        self.messages_processed.labels(status=f"error_{error_type}").inc()

    @contextmanager
    def track_duration(self):
        self.current_processing.inc()
        start = time.time()
        try:
            yield
        finally:
            duration = time.time() - start
            self.processing_duration.observe(duration)
            self.current_processing.dec()
