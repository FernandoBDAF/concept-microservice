"""
Chat Filter Sanitization and Expansion.

This module handles filter sanitization for chat queries.
Part of the BUSINESS layer - chat services.
"""

import time
from typing import Any, Dict, List, Optional

from src.domain.services.ingestion.metadata import expand_filter_values
from src.lib.error_handling.decorators import handle_errors
from src.lib.metrics import Counter, Histogram, MetricRegistry

# Initialize chat filters metrics
_chat_filters_calls = Counter(
    "chat_filters_calls", "Number of filter sanitization operations", labels=["operation"]
)
_chat_filters_errors = Counter(
    "chat_filters_errors", "Number of filter sanitization operation errors", labels=["operation"]
)
_chat_filters_duration = Histogram(
    "chat_filters_duration_seconds", "Filter sanitization operation duration", labels=["operation"]
)

# Register metrics
_registry = MetricRegistry.get_instance()
_registry.register(_chat_filters_calls)
_registry.register(_chat_filters_errors)
_registry.register(_chat_filters_duration)


@handle_errors(fallback=None, log_traceback=True, reraise=False)
def sanitize_filters(
    raw: Optional[Dict[str, Any]], full_catalog: Optional[Dict[str, List[Any]]] = None
) -> Optional[Dict[str, Any]]:
    """Sanitize filters - expand patterns to exact matches from catalog.

    Since $vectorSearch.filter doesn't support $regex, we expand filter values
    to include all matching variants from the full catalog, then use simple $in.

    Example:
        "RAG" expands to ["RAG", "rag", "RAG framework", "Graph RAG", ...]

    Args:
        raw: Raw filters from query rewriter
        full_catalog: Full metadata catalog for expansion

    Returns:
        Sanitized filters dict or None if invalid

    Note:
        Only allows fields that are indexed for $vectorSearch filter.
    """
    start_time = time.time()
    labels = {"operation": "sanitize_filters"}
    _chat_filters_calls.inc(labels=labels)
    
    try:
        if not isinstance(raw, dict) or not raw:
            duration = time.time() - start_time
            _chat_filters_duration.observe(duration, labels=labels)
            return None

        # First expand filter values to include all semantic variants
        if full_catalog:
            expanded = expand_filter_values(raw, full_catalog)
        else:
            expanded = raw

        safe: Dict[str, Any] = {}

        # Only allow fields that are indexed for $vectorSearch filter
        allowed: Dict[str, str] = {
            "context.tags": "context.tags",
            "concepts.name": "concepts.name",
            "entities.name": "entities.name",
            "relations.subject": "relations.subject",
            "age_days": "metadata.age_days",
            "metadata.age_days": "metadata.age_days",
            "published_at": "published_at",
            "trust_score": "trust_score",
        }

        for k, v in expanded.items():
            key = allowed.get(k)
            if not key:
                continue

            # Coerce lists to $in, scalars to direct match
            if isinstance(v, list):
                scalars = [x for x in v if isinstance(x, (str, int, float, bool))]
                if scalars:
                    safe[key] = {"$in": scalars}
            elif isinstance(v, (str, int, float, bool)):
                safe[key] = v

        result = safe or None
        duration = time.time() - start_time
        _chat_filters_duration.observe(duration, labels=labels)
        return result
    except Exception as e:
        _chat_filters_errors.inc(labels=labels)
        duration = time.time() - start_time
        _chat_filters_duration.observe(duration, labels=labels)
        raise
