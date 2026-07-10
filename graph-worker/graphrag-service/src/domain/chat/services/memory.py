"""
Chat Memory and Session Management.

This module handles session management and long-term memory for chat conversations.
Part of the BUSINESS layer - chat feature business logic.
"""

import uuid
import logging
import time
from pathlib import Path
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional

from src.infrastructure.database.mongodb import get_mongo_client
from src.core.config.paths import DB_NAME, COLL_MEMORY_LOGS
from src.lib.error_handling.decorators import handle_errors
from src.lib.metrics import Counter, Histogram, MetricRegistry

# Initialize chat memory metrics
_chat_memory_calls = Counter(
    "chat_memory_calls", "Number of chat memory operations", labels=["operation"]
)
_chat_memory_errors = Counter(
    "chat_memory_errors", "Number of chat memory operation errors", labels=["operation"]
)
_chat_memory_duration = Histogram(
    "chat_memory_duration_seconds",
    "Chat memory operation duration",
    labels=["operation"],
)

# Register metrics
_registry = MetricRegistry.get_instance()
_registry.register(_chat_memory_calls)
_registry.register(_chat_memory_errors)
_registry.register(_chat_memory_duration)


def generate_session_id() -> str:
    """Generate a unique session ID.

    Returns:
        UUID string for session identification
    """
    return str(uuid.uuid4())


@handle_errors(fallback=[], log_traceback=True, reraise=False)
def load_long_term_memory(session_id: str, limit: int = 20) -> List[Dict[str, Any]]:
    """Load long-term memory logs for a session.

    Args:
        session_id: Session identifier
        limit: Maximum number of logs to retrieve (default: 20)

    Returns:
        List of memory log documents, most recent first
    """
    start_time = time.time()
    labels = {"operation": "load_long_term_memory"}
    _chat_memory_calls.inc(labels=labels)

    try:
        client = get_mongo_client()
        db = client[DB_NAME]
        cur = (
            db[COLL_MEMORY_LOGS]
            .find({"session_id": session_id})
            .sort("created_at", -1)
            .limit(int(limit))
        )
        result = list(cur)
        duration = time.time() - start_time
        _chat_memory_duration.observe(duration, labels=labels)
        return result
    except Exception as e:
        _chat_memory_errors.inc(labels=labels)
        duration = time.time() - start_time
        _chat_memory_duration.observe(duration, labels=labels)
        raise


def setup_chat_logger(session_id: str, log_dir: str = "chat_logs") -> logging.Logger:
    """Setup session-specific logger for chat.

    Args:
        session_id: Session identifier
        log_dir: Directory for chat logs (default: "chat_logs")

    Returns:
        Configured logger instance

    Note:
        Uses the logging library's setup_session_logger for consistency.
    """
    from src.lib.logging import setup_session_logger

    return setup_session_logger(
        session_id=session_id,
        log_dir=log_dir,
        level=logging.INFO,
        console_level=logging.WARNING,
        verbose=False,
    )


@handle_errors(log_traceback=True, reraise=False)
def persist_turn(
    session_id: str,
    raw_query: str,
    rewritten_query: str,
    mode: str,
    top_k: int,
    filters: Optional[Dict[str, Any]],
    hits: List[Dict[str, Any]],
    answer: str,
    agent: Optional[str] = "reference_answer",
) -> None:
    """Persist a conversation turn to long-term memory.

    Args:
        session_id: Session identifier
        raw_query: Original user query
        rewritten_query: Rewritten query (after context)
        mode: Retrieval mode used
        top_k: Number of results requested
        filters: Filters applied
        hits: Search result documents
        answer: Generated answer
        agent: Agent used for answering (default: "reference_answer")
    """
    start_time = time.time()
    labels = {"operation": "persist_turn"}
    _chat_memory_calls.inc(labels=labels)

    try:
        client = get_mongo_client()
        db = client[DB_NAME]

        retrieved = [
            {
                "video_id": h.get("video_id"),
                "chunk_id": h.get("chunk_id"),
                "score": h.get("score") or h.get("search_score"),
                "keyword_score": h.get("keyword_score"),
                "vector_score": h.get("vector_score"),
            }
            for h in hits
        ]

        doc = {
            "session_id": session_id,
            "user_query_raw": raw_query,
            "user_query_rewritten": rewritten_query,
            "mode": mode,
            "k": int(top_k),
            "filters": filters or {},
            "retrieved": retrieved,
            "answer": answer,
            "agent": agent,
            "created_at": datetime.now(timezone.utc),
        }

        db[COLL_MEMORY_LOGS].insert_one(doc)
        duration = time.time() - start_time
        _chat_memory_duration.observe(duration, labels=labels)
    except Exception as e:
        _chat_memory_errors.inc(labels=labels)
        duration = time.time() - start_time
        _chat_memory_duration.observe(duration, labels=labels)
        raise
