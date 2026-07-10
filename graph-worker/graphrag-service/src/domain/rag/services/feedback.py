from __future__ import annotations

import time
from typing import Any, Dict, List, Optional, Tuple
from datetime import datetime, timezone

from src.core.config.paths import COLL_VIDEO_FEEDBACK, COLL_CHUNK_FEEDBACK
from src.lib.error_handling.decorators import handle_errors
from src.lib.metrics import Counter, Histogram, MetricRegistry

# Initialize feedback service metrics
_rag_feedback_calls = Counter(
    "rag_feedback_calls", "Number of feedback operations", labels=["operation"]
)
_rag_feedback_errors = Counter(
    "rag_feedback_errors", "Number of feedback operation errors", labels=["operation"]
)
_rag_feedback_duration = Histogram(
    "rag_feedback_duration_seconds", "Feedback operation duration", labels=["operation"]
)

# Register metrics
_registry = MetricRegistry.get_instance()
_registry.register(_rag_feedback_calls)
_registry.register(_rag_feedback_errors)
_registry.register(_rag_feedback_duration)


def _now() -> datetime:
    return datetime.now(timezone.utc)


def _sanitize_tags(tags: Optional[List[str]]) -> List[str]:
    if not tags:
        return []
    cleaned: List[str] = []
    for t in tags[:10]:  # cap max 10
        s = (t or "").strip()
        if s:
            cleaned.append(s[:64])
    return cleaned


@handle_errors(fallback=None, log_traceback=True, reraise=False)
def upsert_video_feedback(
    db,
    session_id: str,
    video_id: str,
    rating: int,
    tags: Optional[List[str]] = None,
    note: Optional[str] = None,
) -> None:
    start_time = time.time()
    labels = {"operation": "upsert_video_feedback"}
    _rag_feedback_calls.inc(labels=labels)
    
    try:
        doc = {
            "session_id": session_id,
            "video_id": video_id,
            "rating": max(1, min(5, int(rating))),
            "tags": _sanitize_tags(tags),
            "note": (note or "")[:2000],
            "updated_at": _now(),
        }
        coll = db[COLL_VIDEO_FEEDBACK]
        coll.update_one(
            {"session_id": session_id, "video_id": video_id},
            {"$set": doc, "$setOnInsert": {"created_at": _now()}},
            upsert=True,
        )
        duration = time.time() - start_time
        _rag_feedback_duration.observe(duration, labels=labels)
    except Exception as e:
        _rag_feedback_errors.inc(labels=labels)
        duration = time.time() - start_time
        _rag_feedback_duration.observe(duration, labels=labels)
        raise


@handle_errors(fallback=None, log_traceback=True, reraise=False)
def upsert_chunk_feedback(
    db,
    session_id: str,
    chunk_id: str,
    video_id: str,
    rating: int,
    tags: Optional[List[str]] = None,
    note: Optional[str] = None,
) -> None:
    start_time = time.time()
    labels = {"operation": "upsert_chunk_feedback"}
    _rag_feedback_calls.inc(labels=labels)
    
    try:
        doc = {
            "session_id": session_id,
            "chunk_id": chunk_id,
            "video_id": video_id,
            "rating": max(1, min(5, int(rating))),
            "tags": _sanitize_tags(tags),
            "note": (note or "")[:2000],
            "updated_at": _now(),
        }
        coll = db[COLL_CHUNK_FEEDBACK]
        coll.update_one(
            {"session_id": session_id, "chunk_id": chunk_id},
            {"$set": doc, "$setOnInsert": {"created_at": _now()}},
            upsert=True,
        )
        duration = time.time() - start_time
        _rag_feedback_duration.observe(duration, labels=labels)
    except Exception as e:
        _rag_feedback_errors.inc(labels=labels)
        duration = time.time() - start_time
        _rag_feedback_duration.observe(duration, labels=labels)
        raise


@handle_errors(fallback=None, log_traceback=True, reraise=False)
def get_video_feedback_for_session(
    db, session_id: str, video_id: str
) -> Optional[Dict[str, Any]]:
    start_time = time.time()
    labels = {"operation": "get_video_feedback_for_session"}
    _rag_feedback_calls.inc(labels=labels)
    
    try:
        result = db[COLL_VIDEO_FEEDBACK].find_one(
            {"session_id": session_id, "video_id": video_id}
        )
        duration = time.time() - start_time
        _rag_feedback_duration.observe(duration, labels=labels)
        return result
    except Exception as e:
        _rag_feedback_errors.inc(labels=labels)
        duration = time.time() - start_time
        _rag_feedback_duration.observe(duration, labels=labels)
        raise


@handle_errors(fallback=None, log_traceback=True, reraise=False)
def get_chunk_feedback_for_session(
    db, session_id: str, chunk_id: str
) -> Optional[Dict[str, Any]]:
    start_time = time.time()
    labels = {"operation": "get_chunk_feedback_for_session"}
    _rag_feedback_calls.inc(labels=labels)
    
    try:
        result = db[COLL_CHUNK_FEEDBACK].find_one(
            {"session_id": session_id, "chunk_id": chunk_id}
        )
        duration = time.time() - start_time
        _rag_feedback_duration.observe(duration, labels=labels)
        return result
    except Exception as e:
        _rag_feedback_errors.inc(labels=labels)
        duration = time.time() - start_time
        _rag_feedback_duration.observe(duration, labels=labels)
        raise


@handle_errors(fallback={"avg_rating": None, "count": 0, "top_tags": []}, log_traceback=True, reraise=False)
def aggregate_video_feedback(db, video_id: str) -> Dict[str, Any]:
    start_time = time.time()
    labels = {"operation": "aggregate_video_feedback"}
    _rag_feedback_calls.inc(labels=labels)
    
    try:
        cursor = db[COLL_VIDEO_FEEDBACK].aggregate(
            [
                {"$match": {"video_id": video_id}},
                {
                    "$group": {
                        "_id": "$video_id",
                        "avg_rating": {"$avg": "$rating"},
                        "count": {"$sum": 1},
                        "all_tags": {"$push": "$tags"},
                    }
                },
                {
                    "$project": {
                        "_id": 0,
                        "avg_rating": 1,
                        "count": 1,
                        "tags": {
                            "$reduce": {
                                "input": "$all_tags",
                                "initialValue": [],
                                "in": {"$concatArrays": ["$$value", "$$this"]},
                            }
                        },
                    }
                },
            ]
        )
        agg = next(cursor, None) or {"avg_rating": None, "count": 0, "tags": []}
        # compute top tags
        top_map: Dict[str, int] = {}
        for t in agg.get("tags", [])[:2000]:
            if t:
                top_map[t] = top_map.get(t, 0) + 1
        top = sorted(top_map.items(), key=lambda x: x[1], reverse=True)[:10]
        result = {
            "avg_rating": agg.get("avg_rating"),
            "count": agg.get("count"),
            "top_tags": top,
        }
        duration = time.time() - start_time
        _rag_feedback_duration.observe(duration, labels=labels)
        return result
    except Exception as e:
        _rag_feedback_errors.inc(labels=labels)
        duration = time.time() - start_time
        _rag_feedback_duration.observe(duration, labels=labels)
        raise


@handle_errors(fallback={"avg_rating": None, "count": 0, "top_tags": []}, log_traceback=True, reraise=False)
def aggregate_chunk_feedback(db, chunk_id: str) -> Dict[str, Any]:
    start_time = time.time()
    labels = {"operation": "aggregate_chunk_feedback"}
    _rag_feedback_calls.inc(labels=labels)
    
    try:
        cursor = db[COLL_CHUNK_FEEDBACK].aggregate(
            [
                {"$match": {"chunk_id": chunk_id}},
                {
                    "$group": {
                        "_id": "$chunk_id",
                        "avg_rating": {"$avg": "$rating"},
                        "count": {"$sum": 1},
                        "all_tags": {"$push": "$tags"},
                    }
                },
                {
                    "$project": {
                        "_id": 0,
                        "avg_rating": 1,
                        "count": 1,
                        "tags": {
                            "$reduce": {
                                "input": "$all_tags",
                                "initialValue": [],
                                "in": {"$concatArrays": ["$$value", "$$this"]},
                            }
                        },
                    }
                },
            ]
        )
        agg = next(cursor, None) or {"avg_rating": None, "count": 0, "tags": []}
        top_map: Dict[str, int] = {}
        for t in agg.get("tags", [])[:2000]:
            if t:
                top_map[t] = top_map.get(t, 0) + 1
        top = sorted(top_map.items(), key=lambda x: x[1], reverse=True)[:10]
        result = {
            "avg_rating": agg.get("avg_rating"),
            "count": agg.get("count"),
            "top_tags": top,
        }
        duration = time.time() - start_time
        _rag_feedback_duration.observe(duration, labels=labels)
        return result
    except Exception as e:
        _rag_feedback_errors.inc(labels=labels)
        duration = time.time() - start_time
        _rag_feedback_duration.observe(duration, labels=labels)
        raise
