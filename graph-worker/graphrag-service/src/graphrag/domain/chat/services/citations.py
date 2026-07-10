"""
Chat Citation Formatting.

This module handles citation formatting for chat search results.
Part of the BUSINESS layer - chat services.
"""

import time
from typing import Any, Dict, List

from src.lib.error_handling.decorators import handle_errors
from src.lib.metrics import Counter, Histogram, MetricRegistry

# Initialize chat citations metrics
_chat_citations_calls = Counter(
    "chat_citations_calls", "Number of citation formatting operations", labels=["operation"]
)
_chat_citations_errors = Counter(
    "chat_citations_errors", "Number of citation formatting operation errors", labels=["operation"]
)
_chat_citations_duration = Histogram(
    "chat_citations_duration_seconds", "Citation formatting operation duration", labels=["operation"]
)

# Register metrics
_registry = MetricRegistry.get_instance()
_registry.register(_chat_citations_calls)
_registry.register(_chat_citations_errors)
_registry.register(_chat_citations_duration)


@handle_errors(fallback="", log_traceback=True, reraise=False)
def format_citations(hits: List[Dict[str, Any]], max_items: int = 5) -> str:
    """Format search hits as citations for display.

    Args:
        hits: List of search hit documents
        max_items: Maximum number of citations to format (default: 5)

    Returns:
        Formatted citation string with video_id:chunk_id and scores
    """
    start_time = time.time()
    labels = {"operation": "format_citations"}
    _chat_citations_calls.inc(labels=labels)
    
    try:
        items: List[str] = []
        seen: set[str] = set()

        for h in hits:
            vid = str(h.get("video_id"))
            cid = str(h.get("chunk_id"))
            key = f"{vid}:{cid}"

            if key in seen:
                continue
            seen.add(key)

            score = h.get("final_score") or h.get("score") or h.get("search_score")
            items.append(
                f"({key}) score={score:.3f}"
                if isinstance(score, (int, float))
                else f"({key})"
            )

            if len(items) >= max_items:
                break

        result = "\n".join(items)
        duration = time.time() - start_time
        _chat_citations_duration.observe(duration, labels=labels)
        return result
    except Exception as e:
        _chat_citations_errors.inc(labels=labels)
        duration = time.time() - start_time
        _chat_citations_duration.observe(duration, labels=labels)
        raise
