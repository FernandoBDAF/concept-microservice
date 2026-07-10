import time
from typing import Any, Dict, Optional
from src.lib.error_handling.decorators import handle_errors
from src.lib.metrics import Counter, Histogram, MetricRegistry

# Initialize filter service metrics
_rag_filter_calls = Counter(
    "rag_filter_calls", "Number of filter operations", labels=["operation"]
)
_rag_filter_errors = Counter(
    "rag_filter_errors", "Number of filter operation errors", labels=["operation"]
)
_rag_filter_duration = Histogram(
    "rag_filter_duration_seconds", "Filter operation duration", labels=["operation"]
)

# Register metrics
_registry = MetricRegistry.get_instance()
_registry.register(_rag_filter_calls)
_registry.register(_rag_filter_errors)
_registry.register(_rag_filter_duration)


@handle_errors(fallback={}, log_traceback=True, reraise=False)
def build_filters(
    topic: Optional[str] = None,
    channel: Optional[str] = None,
    max_age: Optional[int] = None,
    trust_min: Optional[float] = None,
    exclude_redundant: bool = False,
) -> Dict[str, Any]:
    """Build a Mongo/Atlas filter for common UI settings."""
    start_time = time.time()
    labels = {"operation": "build_filters"}
    _rag_filter_calls.inc(labels=labels)
    
    try:
        filters: Dict[str, Any] = {}
        if topic:
            filters = {"metadata.tags": {"$regex": topic, "$options": "i"}}
        if channel:
            if filters:
                filters = {
                    "$and": [
                        filters,
                        {"metadata.channel_id": {"$regex": channel, "$options": "i"}},
                    ]
                }
            else:
                filters = {"metadata.channel_id": {"$regex": channel, "$options": "i"}}
        if isinstance(max_age, int) and max_age > 0 and max_age < 10000:
            age_filter = {"metadata.age_days": {"$lte": max_age}}
            filters = {"$and": [filters, age_filter]} if filters else age_filter
        if isinstance(trust_min, (int, float)) and trust_min > 0:
            trust_filter = {"trust_score": {"$gte": float(trust_min)}}
            filters = {"$and": [filters, trust_filter]} if filters else trust_filter
        if exclude_redundant:
            red_filter = {"is_redundant": {"$ne": True}}
            filters = {"$and": [filters, red_filter]} if filters else red_filter
        duration = time.time() - start_time
        _rag_filter_duration.observe(duration, labels=labels)
        return filters
    except Exception as e:
        _rag_filter_errors.inc(labels=labels)
        duration = time.time() - start_time
        _rag_filter_duration.observe(duration, labels=labels)
        raise
