import time
from typing import Any, Dict, Optional, List
from datetime import datetime, timezone

from src.infrastructure.database.mongodb import get_mongo_client
from src.core.config.paths import DB_NAME
from src.lib.error_handling.decorators import handle_errors
from src.lib.metrics import Counter, Histogram, MetricRegistry

# Initialize profile service metrics
_rag_profile_calls = Counter(
    "rag_profile_calls", "Number of profile operations", labels=["operation"]
)
_rag_profile_errors = Counter(
    "rag_profile_errors", "Number of profile operation errors", labels=["operation"]
)
_rag_profile_duration = Histogram(
    "rag_profile_duration_seconds", "Profile operation duration", labels=["operation"]
)

# Register metrics
_registry = MetricRegistry.get_instance()
_registry.register(_rag_profile_calls)
_registry.register(_rag_profile_errors)
_registry.register(_rag_profile_duration)


@handle_errors(fallback=None, log_traceback=True, reraise=False)
def get_profile(session_id: str) -> Optional[Dict[str, Any]]:
    start_time = time.time()
    labels = {"operation": "get_profile"}
    _rag_profile_calls.inc(labels=labels)
    
    try:
        client = get_mongo_client()
        db = client[DB_NAME]
        doc = db["user_profiles"].find_one({"session_id": session_id})
        duration = time.time() - start_time
        _rag_profile_duration.observe(duration, labels=labels)
        return doc
    except Exception as e:
        _rag_profile_errors.inc(labels=labels)
        duration = time.time() - start_time
        _rag_profile_duration.observe(duration, labels=labels)
        raise


@handle_errors(fallback=None, log_traceback=True, reraise=False)
def upsert_profile(session_id: str, profile: Dict[str, Any]) -> None:
    start_time = time.time()
    labels = {"operation": "upsert_profile"}
    _rag_profile_calls.inc(labels=labels)
    
    try:
        client = get_mongo_client()
        db = client[DB_NAME]
        payload = {
            "session_id": session_id,
            **profile,
            "updated_at": datetime.now(timezone.utc),
        }
        db["user_profiles"].update_one(
            {"session_id": session_id}, {"$set": payload}, upsert=True
        )
        duration = time.time() - start_time
        _rag_profile_duration.observe(duration, labels=labels)
    except Exception as e:
        _rag_profile_errors.inc(labels=labels)
        duration = time.time() - start_time
        _rag_profile_duration.observe(duration, labels=labels)
        raise


@handle_errors(fallback=[], log_traceback=True, reraise=False)
def list_profiles(limit: int = 50) -> List[Dict[str, Any]]:
    start_time = time.time()
    labels = {"operation": "list_profiles"}
    _rag_profile_calls.inc(labels=labels)
    
    try:
        client = get_mongo_client()
        db = client[DB_NAME]
        rows = list(
            db["user_profiles"]
            .find({}, {"session_id": 1, "name": 1, "persona": 1})
            .sort("updated_at", -1)
            .limit(int(limit))
        )
        duration = time.time() - start_time
        _rag_profile_duration.observe(duration, labels=labels)
        return rows
    except Exception as e:
        _rag_profile_errors.inc(labels=labels)
        duration = time.time() - start_time
        _rag_profile_duration.observe(duration, labels=labels)
        raise


@handle_errors(fallback=None, log_traceback=True, reraise=False)
def delete_profile(session_id: str) -> None:
    start_time = time.time()
    labels = {"operation": "delete_profile"}
    _rag_profile_calls.inc(labels=labels)
    
    try:
        client = get_mongo_client()
        db = client[DB_NAME]
        db["user_profiles"].delete_one({"session_id": session_id})
        duration = time.time() - start_time
        _rag_profile_duration.observe(duration, labels=labels)
    except Exception as e:
        _rag_profile_errors.inc(labels=labels)
        duration = time.time() - start_time
        _rag_profile_duration.observe(duration, labels=labels)
        raise
