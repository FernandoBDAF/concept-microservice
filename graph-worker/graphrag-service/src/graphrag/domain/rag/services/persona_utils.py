import time
from typing import Dict, List
from src.lib.error_handling.decorators import handle_errors
from src.lib.metrics import Counter, Histogram, MetricRegistry

# Initialize persona utils metrics
_rag_persona_calls = Counter(
    "rag_persona_calls", "Number of persona operations", labels=["operation"]
)
_rag_persona_errors = Counter(
    "rag_persona_errors", "Number of persona operation errors", labels=["operation"]
)
_rag_persona_duration = Histogram(
    "rag_persona_duration_seconds", "Persona operation duration", labels=["operation"]
)

# Register metrics
_registry = MetricRegistry.get_instance()
_registry.register(_rag_persona_calls)
_registry.register(_rag_persona_errors)
_registry.register(_rag_persona_duration)


@handle_errors(fallback=[], log_traceback=True, reraise=False)
def infer_top_tags(db, session_id: str, limit: int = 5) -> List[str]:
    start_time = time.time()
    labels = {"operation": "infer_top_tags"}
    _rag_persona_calls.inc(labels=labels)
    
    try:
        tag_counts: Dict[str, int] = {}
        for r in (
            db["video_feedback"].find({"session_id": session_id}, {"tags": 1}).limit(200)
        ):
            for t in r.get("tags", []) or []:
                t2 = (t or "").strip().lower().replace("_", "-")
                if t2:
                    tag_counts[t2] = tag_counts.get(t2, 0) + 1
        result = [
            t
            for t, _ in sorted(tag_counts.items(), key=lambda x: x[1], reverse=True)[
                : int(limit)
            ]
        ]
        duration = time.time() - start_time
        _rag_persona_duration.observe(duration, labels=labels)
        return result
    except Exception as e:
        _rag_persona_errors.inc(labels=labels)
        duration = time.time() - start_time
        _rag_persona_duration.observe(duration, labels=labels)
        raise
