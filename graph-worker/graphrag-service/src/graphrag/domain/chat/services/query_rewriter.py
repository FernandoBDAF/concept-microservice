"""
Query Rewriter for Memory-Aware Chat.

This module provides LLM-powered query rewriting using conversation memory context.
Part of the BUSINESS layer - chat feature logic.
"""

import os
import json
import logging
import time
from typing import Any, Dict, List, Optional, Tuple

from src.lib.llm import get_openai_client
from src.lib.error_handling.decorators import handle_errors
from src.lib.metrics import Counter, Histogram, MetricRegistry

# Initialize chat query rewriter metrics
_chat_query_rewriter_calls = Counter(
    "chat_query_rewriter_calls", "Number of query rewrite operations", labels=["operation"]
)
_chat_query_rewriter_errors = Counter(
    "chat_query_rewriter_errors", "Number of query rewrite operation errors", labels=["operation"]
)
_chat_query_rewriter_duration = Histogram(
    "chat_query_rewriter_duration_seconds", "Query rewrite operation duration", labels=["operation"]
)

# Register metrics
_registry = MetricRegistry.get_instance()
_registry.register(_chat_query_rewriter_calls)
_registry.register(_chat_query_rewriter_errors)
_registry.register(_chat_query_rewriter_duration)


def is_openai_available() -> bool:
    """Check if OpenAI is configured.

    Returns:
        True if OPENAI_API_KEY is set

    Note:
        Uses the LLM library's is_openai_available for consistency.
    """
    from src.lib.llm import is_openai_available as llm_is_openai_available

    return llm_is_openai_available()


@handle_errors(
    fallback=lambda *args, **kwargs: (args[0], args[3], args[4], None),
    log_traceback=True,
    reraise=False,
)
def rewrite_query(
    user_query: str,
    short_term_msgs: List[Dict[str, str]],
    long_term_logs: List[Dict[str, Any]],
    default_mode: str,
    default_k: int,
    catalog: Optional[Dict[str, Any]] = None,
    logger: Optional[logging.Logger] = None,
) -> Tuple[str, str, int, Optional[Dict[str, Any]]]:
    """Rewrite user query using conversation memory and catalog context.

    Uses LLM to:
    - Expand follow-up queries with context from memory
    - Select appropriate retrieval mode (vector/hybrid/keyword)
    - Generate filters based on catalog
    - Adjust top_k based on query complexity

    Args:
        user_query: Original user query
        short_term_msgs: Recent conversation messages (role/content)
        long_term_logs: Longer-term conversation logs from DB
        default_mode: Default retrieval mode
        default_k: Default number of results
        catalog: Available filter values from metadata catalog
        logger: Optional logger for debugging

    Returns:
        Tuple of (rewritten_query, tool_mode, top_k, filters)

    Note:
        Falls back to identity rewrite when OpenAI is not configured.
    """
    start_time = time.time()
    labels = {"operation": "rewrite_query"}
    _chat_query_rewriter_calls.inc(labels=labels)
    
    try:
        if not is_openai_available():
            duration = time.time() - start_time
            _chat_query_rewriter_duration.observe(duration, labels=labels)
            return user_query, default_mode, default_k, None

        client = get_openai_client()

        # Build memory snippets (truncate for safety)
        recent_msgs = short_term_msgs[-8:]
        stm = "\n".join(f"{m['role']}: {m['content'][:400]}" for m in recent_msgs)
        ltm = "\n".join(
            f"Q: {log.get('user_query_raw','')[:200]}\nA: {str(log.get('answer',''))[:300]}"
            for log in long_term_logs[:8]
        )

        # Log memory context for debugging
        if logger:
            logger.info(
                "rewrite:memory short_term_count=%s long_term_count=%s",
                len(recent_msgs),
                len(long_term_logs[:8]),
            )
            if stm:
                logger.info("rewrite:SHORT_TERM=%s", stm[:500])
            if ltm:
                logger.info("rewrite:LONG_TERM=%s", ltm[:800])

        # FUTURE OPTIMIZATION: For conversations with 20+ turns, implement semantic history retrieval:
        # - Embed each previous turn (Q+A pairs)
        # - Semantically retrieve only 2-3 most relevant turns based on current query
        # - Reduces context size and improves relevance for long conversations
        # - Current: keeping last 8 turns is sufficient for typical use

        system_prompt = (
            "CRITICAL INSTRUCTION #1 - YOU MUST FOLLOW THIS:\n"
            "For queries containing 'go deeper', 'what about', 'also', 'it', 'this', 'that':\n"
            "- Extract the main topic from SHORT_TERM or LONG_TERM memory below\n"
            "- MERGE that topic into your rewritten query\n"
            "- DO NOT return the query unchanged - you MUST expand it with context\n\n"
            "Example:\n"
            "  Memory: 'Q: RAG systems and vector databases A: Graph RAG uses...'\n"
            "  Query: 'Go deeper in embeddings'\n"
            "  ❌ BAD: 'Go deeper in embeddings' (unchanged - WRONG!)\n"
            "  ✓ GOOD: 'Embedding models and techniques in RAG systems and vector databases'\n\n"
            "You are a query rewriter that improves user queries for retrieval using conversation memory.\n"
            "Return strict JSON with keys: query, tool, k, filters.\n"
            "tool in {auto, vector, hybrid, keyword}; k is positive int; filters is JSON object or null.\n"
            "Use only allowed filter keys: ['context.tags','concepts.name','entities.name','relations.subject','metadata.age_days','published_at','trust_score'].\n"
            "Select at most 2 filters; use catalog values only."
        )

        # Render compact catalog
        cat = catalog or {}

        def _join(key: str, lim: int = 12) -> str:
            try:
                vals = cat.get(key) or []
                s = ", ".join([str(v) for v in vals[:lim]])
                return s or "(none)"
            except Exception:
                return "(none)"

        cat_str = (
            "Allowed catalog (samples):\n"
            f"- metadata.tags: {_join('metadata.tags')}\n"
            f"- context.tags: {_join('context.tags')}\n"
            f"- concepts.name: {_join('concepts.name')}\n"
            f"- relations.predicate: {_join('relations.predicate')}\n"
            f"- relations.object: {_join('relations.object')}\n"
        )

        # Detect if this is a follow-up that needs special reminder
        is_followup_query = any(
            kw in user_query.lower()
            for kw in [
                "go deeper",
                "what about",
                "also",
                "it",
                "this",
                "that",
                "more details",
            ]
        )

        followup_reminder = ""
        if is_followup_query and (stm or ltm):
            followup_reminder = (
                "⚠️ IMPORTANT: This is a FOLLOW-UP query. You MUST use the memory below to expand it.\n"
                "Do NOT return the query unchanged. Extract the topic from memory and merge it.\n\n"
            )

        self_check = ""
        if is_followup_query and (stm or ltm):
            self_check = (
                "\n\nSELF-CHECK before responding:\n"
                "1. ✓ Did I read the SHORT_TERM and LONG_TERM memory above?\n"
                "2. ✓ Did I identify the main topic from previous turns?\n"
                "3. ✓ Did I MERGE that topic into my rewritten query?\n"
                "4. ✓ Is my rewritten query DIFFERENT from USER_QUERY?\n"
                "If you answered NO to any question, GO BACK and rewrite properly.\n\n"
            )

        user_prompt = (
            f"{followup_reminder}"
            f"USER_QUERY: {user_query}\n\n"
            f"SHORT_TERM:\n{stm or '(none)'}\n\n"
            f"LONG_TERM:\n{ltm or '(none)'}\n\n"
            f"{cat_str}\n"
            f'DEFAULTS: {{"tool": "{default_mode}", "k": {default_k}}}\n'
            f"{self_check}"
            "Respond with JSON only."
        )

        resp = client.chat.completions.create(
            model=os.getenv("OPENAI_DEFAULT_MODEL", "gpt-4o-mini"),
            messages=[
                {"role": "system", "content": system_prompt},
                {"role": "user", "content": user_prompt},
            ],
        )

        text = resp.choices[0].message.content.strip()
        data: Dict[str, Any] = json.loads(text)

        rq = str(data.get("query") or user_query)
        tool = str(data.get("tool") or default_mode).lower()
        if tool not in {"auto", "vector", "hybrid", "keyword"}:
            tool = default_mode

        k = int(data.get("k") or default_k)
        if k < 1:
            k = default_k

        filters = data.get("filters")
        if not isinstance(filters, dict):
            filters = None

        result = (rq, tool, k, filters)
        duration = time.time() - start_time
        _chat_query_rewriter_duration.observe(duration, labels=labels)
        return result
    except Exception as e:
        _chat_query_rewriter_errors.inc(labels=labels)
        duration = time.time() - start_time
        _chat_query_rewriter_duration.observe(duration, labels=labels)
        raise
