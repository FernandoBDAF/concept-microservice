import os
import json
import uuid
import argparse
import logging
from pathlib import Path
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional, Tuple

from src.lib.error_handling.decorators import handle_errors

try:
    from bson import ObjectId  # type: ignore
    from bson.decimal128 import Decimal128  # type: ignore
except Exception:
    ObjectId = None  # type: ignore
    Decimal128 = None  # type: ignore

from src.domain.rag.services.retrieval import (
    vector_search,
    hybrid_search,
    keyword_search,
)
from src.domain.rag.services.indexes import (
    ensure_vector_search_index,
    ensure_hybrid_search_index,
)
from src.domain.rag.services.generation import answer_with_openai
from src.infrastructure.database.mongodb import get_mongo_client
from src.core.config.paths import DB_NAME, COLL_CHUNKS, COLL_MEMORY_LOGS
from src.domain.rag.agents.reference_answer import ReferenceAnswerAgent
from src.domain.rag.agents.topic_reference import TopicReferenceAgent
from src.domain.rag.agents.planner import PlannerAgent
from src.domain.ingestion.services.metadata import (
    build_catalog,
    build_insights,
    prune_catalog_for_query,
    extract_query_keywords,
    expand_filter_values,
)
from src.infrastructure.observability.log_utils import Timer


# -----------------------------
# Session & Memory Utilities
# -----------------------------

# ANSI colors (no external deps)
RESET = "\033[0m"
BOLD = "\033[1m"
DIM = "\033[2m"
RED = "\033[31m"
GREEN = "\033[32m"
YELLOW = "\033[33m"
BLUE = "\033[34m"
MAGENTA = "\033[35m"
CYAN = "\033[36m"


def generate_session_id() -> str:
    return str(uuid.uuid4())


def load_long_term_memory(session_id: str, limit: int = 20) -> List[Dict[str, Any]]:
    client = get_mongo_client()
    db = client[DB_NAME]
    cur = (
        db[COLL_MEMORY_LOGS]
        .find({"session_id": session_id})
        .sort("created_at", -1)
        .limit(int(limit))
    )
    return list(cur)


def upsert_vector_index() -> None:
    client = get_mongo_client()
    db = client[DB_NAME]
    col = db[COLL_CHUNKS]
    ensure_vector_search_index(col)


def setup_logger(session_id: str, log_dir: str = "chat_logs") -> logging.Logger:
    logger = logging.getLogger(f"chat_cli_{session_id}")
    if logger.handlers:
        return logger
    logger.setLevel(logging.INFO)
    # Ensure directory
    p = Path(log_dir)
    p.mkdir(parents=True, exist_ok=True)
    # File handler per session
    fh = logging.FileHandler(p / f"{session_id}.log", encoding="utf-8")
    fh.setLevel(logging.INFO)
    fmt = logging.Formatter("%(asctime)s | %(levelname)s | %(message)s")
    fh.setFormatter(fmt)
    logger.addHandler(fh)
    # Optional console handler with minimal format
    ch = logging.StreamHandler()
    ch.setLevel(logging.WARNING)
    ch.setFormatter(logging.Formatter("%(levelname)s: %(message)s"))
    logger.addHandler(ch)
    return logger


def cprint(text: str, color: str = RESET) -> None:
    print(f"{color}{text}{RESET}")


# -----------------------------
# Query Rewrite Agent
# -----------------------------


def _openai_available() -> bool:
    return bool(os.getenv("OPENAI_API_KEY"))


def rewrite_query(
    user_query: str,
    short_term_msgs: List[Dict[str, str]],
    long_term_logs: List[Dict[str, Any]],
    default_mode: str,
    default_k: int,
    catalog: Optional[Dict[str, Any]] = None,
    logger: Optional[logging.Logger] = None,
) -> Tuple[str, str, int, Optional[Dict[str, Any]]]:
    """Return (rewritten_query, tool_mode, top_k, filters).

    Falls back to identity rewrite when no LLM configured.
    """
    if not _openai_available():
        return user_query, default_mode, default_k, None

    try:
        from openai import OpenAI  # type: ignore

        client = OpenAI(api_key=os.getenv("OPENAI_API_KEY"))

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
            model=os.getenv("OPENAI_DEFAULT_MODEL", "gpt-5-nano"),
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
        return rq, tool, k, filters
    except Exception:
        return user_query, default_mode, default_k, None


# -----------------------------
# Retrieval Tooling
# -----------------------------


def run_retrieval(
    mode: str,
    query_text: str,
    top_k: int,
    filters: Optional[Dict[str, Any]],
) -> List[Dict[str, Any]]:
    client = get_mongo_client()
    db = client[DB_NAME]
    col = db[COLL_CHUNKS]

    # Ensure indexes exist once at first retrieval (silent if exists)
    import logging as _log

    _prev_level = _log.getLogger("pymongo").level
    _log.getLogger("pymongo").setLevel(_log.WARNING)
    ensure_vector_search_index(col)
    _log.getLogger("pymongo").setLevel(_prev_level)

    # Use hybrid only when no filters (knnBeta + $search filters use different syntax)
    # When filters present, use vector-only $vectorSearch which accepts our filter format
    if mode == "hybrid" and not filters:
        # hybrid_search expects both query_text and query_vector; we rely on rag.embed_query
        from src.domain.rag.services.core import (
            embed_query,
        )  # lazy import to reuse voyage logic

        qvec = embed_query(query_text)
        return hybrid_search(
            col, query_text=query_text, query_vector=qvec, top_k=top_k, filters=None
        )
    elif mode == "keyword":
        return keyword_search(col, query_text=query_text, top_k=top_k, filters=filters)
    else:
        # vector or auto->vector by default; also used when filters are present
        from src.domain.rag.services.core import embed_query  # lazy import

        qvec = embed_query(query_text)
        return vector_search(col, qvec, k=top_k, filters=filters)


def normalize_context_blocks(hits: List[Dict[str, Any]]) -> List[Dict[str, Any]]:
    """Ensure context blocks have a 'embedding_text' or 'text' field for answer composition."""
    out: List[Dict[str, Any]] = []
    for h in hits:
        block = dict(h)
        # If only 'text' exists, mirror into 'embedding_text' for the answer helper
        if "embedding_text" not in block and "text" in block:
            block["embedding_text"] = block.get("text")
        out.append(block)
    return out


# -----------------------------
# Filter Sanitization
# -----------------------------


def sanitize_filters(
    raw: Optional[Dict[str, Any]], full_catalog: Optional[Dict[str, List[Any]]] = None
) -> Optional[Dict[str, Any]]:
    """Sanitize filters - expand patterns to exact matches from catalog.

    Since $vectorSearch.filter doesn't support $regex, we expand filter values
    to include all matching variants from the full catalog, then use simple $in.
    E.g., "RAG" expands to ["RAG", "rag", "RAG framework", "Graph RAG", ...]
    """
    if not isinstance(raw, dict) or not raw:
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

    return safe or None


# -----------------------------
# Context Builder for Reference Answers
# -----------------------------


def _merge_hits_by_doc(
    hits: List[Dict[str, Any]], max_docs: int = 5
) -> List[Dict[str, Any]]:
    by_vid: Dict[str, List[Dict[str, Any]]] = {}
    for h in hits:
        vid = str(h.get("video_id"))
        by_vid.setdefault(vid, []).append(h)
    ranked_docs = sorted(
        by_vid.items(),
        key=lambda kv: max(
            float(
                x.get("final_score") or x.get("score") or x.get("search_score") or 0.0
            )
            for x in kv[1]
        ),
        reverse=True,
    )
    bundles: List[Dict[str, Any]] = []
    for vid, chunks in ranked_docs[:max_docs]:
        sel = [chunks[0]]
        if len(chunks) > 2:
            sel.append(chunks[len(chunks) // 2])
        if len(chunks) > 1:
            sel.append(chunks[-1])
        bundles.append({"video_id": vid, "chunks": sel})
    return bundles


def _anchor_from_chunk(c: Dict[str, Any]) -> Dict[str, Any]:
    meta = c.get("metadata", {}) or {}
    start_sec = meta.get("start_sec")
    vid = c.get("video_id")
    # fallback: parse timestamp_start like 'hh:mm:ss' if available
    if start_sec is None:
        ts = c.get("timestamp_start") or (
            meta.get("timestamp_start") if isinstance(meta, dict) else None
        )
        if isinstance(ts, str) and ts.count(":") >= 1:
            try:
                parts = [int(p) for p in ts.split(":")]
                if len(parts) == 3:
                    start_sec = parts[0] * 3600 + parts[1] * 60 + parts[2]
                elif len(parts) == 2:
                    start_sec = parts[0] * 60 + parts[1]
            except Exception:
                start_sec = None
    ts_param = f"&t={int(start_sec)}" if isinstance(start_sec, (int, float)) else ""
    url = f"https://www.youtube.com/watch?v={vid}{ts_param}" if vid else ""
    title = meta.get("title") or meta.get("channel_id") or str(vid)
    if isinstance(start_sec, (int, float)):
        mm = int(start_sec) // 60
        ss = int(start_sec) % 60
        hint = f"start {mm:02d}:{ss:02d} — entry point"
    else:
        hint = "start near this chunk in the transcript"
    return {"title": title, "url": url, "hint": hint}


def build_reference_bundles(hits: List[Dict[str, Any]]) -> List[Dict[str, Any]]:
    bundles: List[Dict[str, Any]] = []
    for d in _merge_hits_by_doc(hits, max_docs=5):
        chunks = d.get("chunks", [])
        if not chunks:
            continue
        anchors = [_anchor_from_chunk(c) for c in chunks[:1]]
        anchor = (
            anchors[0]
            if anchors
            else {"title": d.get("video_id"), "url": "", "hint": ""}
        )
        bullets: List[str] = []
        quotes: List[str] = []
        for c in chunks[:3]:
            text = c.get("embedding_text") or c.get("text") or ""
            if text:
                bullets.append(text[:220])
                quotes.append(text[:180])
        bundles.append(
            {
                "video_id": d.get("video_id"),
                "title": anchor.get("title"),
                "url": anchor.get("url"),
                "anchor_hint": anchor.get("hint"),
                "bullets": bullets,
                "quotes": quotes,
            }
        )
    return bundles


# -----------------------------
# Answer Agent
# -----------------------------


def answer_with_context(
    contexts: List[Dict[str, Any]],
    rewritten_query: str,
    short_term_msgs: List[Dict[str, str]],
) -> str:
    # Optionally prepend a tiny conversational hint into the question
    history_hint = "\n\nRecent context:\n" + "\n".join(
        f"{m['role']}: {m['content'][:140]}" for m in short_term_msgs[-4:]
    )
    question = rewritten_query + history_hint
    return answer_with_openai(contexts, question)


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


def format_citations(hits: List[Dict[str, Any]], max_items: int = 5) -> str:
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
    return "\n".join(items)


# -----------------------------
# Export Helpers
# -----------------------------


def export_last_turn(
    last_turn: Optional[Dict[str, Any]], fmt: str, path: Optional[str], session_id: str
) -> Optional[str]:
    if not last_turn:
        return None
    ts = datetime.now(timezone.utc).strftime("%Y%m%d_%H%M%S")
    fmt = (fmt or "json").lower()
    if fmt in {"text", "plain"}:
        fmt = "txt"
    if fmt not in {"json", "txt", "md"}:
        fmt = "json"
    default_name = f"export_{session_id}_{ts}.{fmt}"
    if path:
        out_path = Path(path)
        if not out_path.suffix:
            out_path = out_path.with_suffix(f".{fmt}")
    else:
        out_path = Path(default_name)
    out_path.parent.mkdir(parents=True, exist_ok=True)

    def to_plain(o: Any) -> Any:
        try:
            if ObjectId is not None and isinstance(o, ObjectId):
                return str(o)
        except Exception:
            pass
        try:
            if isinstance(o, datetime):
                return o.isoformat()
        except Exception:
            pass
        try:
            if Decimal128 is not None and isinstance(o, Decimal128):
                return float(o.to_decimal())
        except Exception:
            pass
        try:
            return str(o)
        except Exception:
            return None

    if fmt == "json":
        payload = {
            "session_id": session_id,
            **{k: v for k, v in last_turn.items()},
        }
        out_path.write_text(
            json.dumps(payload, ensure_ascii=False, indent=2, default=to_plain),
            encoding="utf-8",
        )
    else:
        q = last_turn.get("user_query_raw", "")
        rq = last_turn.get("user_query_rewritten", "")
        mode = last_turn.get("mode", "")
        k = last_turn.get("k", "")
        filters = last_turn.get("filters", {})
        answer = last_turn.get("answer", "")
        citations = format_citations(last_turn.get("hits", []) or [])
        if fmt == "txt":
            content = (
                f"Session: {session_id}\n\n"
                f"Question: {q}\nRewritten: {rq}\nMode: {mode}  k={k}\nFilters: {json.dumps(filters)}\n\n"
                f"Answer:\n{answer}\n\nCitations:\n{citations}\n"
            )
        else:  # md
            content = (
                f"# Export — Session {session_id}\n\n"
                f"## Question\n{q}\n\n"
                f"## Rewritten\n{rq}\n\n"
                f"## Retrieval\n- Mode: `{mode}`  k={k}\n- Filters: `{json.dumps(filters)}`\n\n"
                f"## Answer\n\n{answer}\n\n"
                f"## Citations\n\n{citations}\n"
            )
        out_path.write_text(content, encoding="utf-8")
    return str(out_path)


# -----------------------------
# CLI Orchestrator
# -----------------------------


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Agentic, Memory-Aware CLI Chat")
    parser.add_argument(
        "--session", type=str, required=False, help="Resume a session by id"
    )
    parser.add_argument("--top_k", type=int, default=8, help="Top-k retrieval")
    parser.add_argument(
        "--mode",
        type=str,
        default="auto",
        choices=["auto", "vector", "hybrid", "keyword"],
        help="Retrieval mode override",
    )
    parser.add_argument(
        "--log_dir", type=str, default="chat_logs", help="Directory for session logs"
    )
    return parser.parse_args()


@handle_errors(log_traceback=True, reraise=True)
def run_cli() -> None:
    args = parse_args()
    session_id = args.session or generate_session_id()
    print(f"Session: {session_id}")

    # Bootstrap index (no-op if exists)
    upsert_vector_index()
    # Also ensure the search index for hybrid paths
    try:
        client = get_mongo_client()
        db = client[DB_NAME]
        col = db[COLL_CHUNKS]
        ensure_hybrid_search_index(col)
    except Exception:
        pass

    # Load long-term memory snapshot
    cprint(f"Loading session memory...", DIM)
    long_term_logs = load_long_term_memory(session_id=session_id, limit=20)
    short_term_memory: List[Dict[str, str]] = []
    logger = setup_logger(session_id, args.log_dir)
    last_turn: Optional[Dict[str, Any]] = None
    # Build metadata catalog (best-effort)
    try:
        cprint("Building metadata catalog...", DIM)
        client = get_mongo_client()
        db = client[DB_NAME]
        col = db[COLL_CHUNKS]
        with Timer("catalog") as t1:
            catalog = build_catalog(col, limit=999999)  # unlimited for quality
        counts_str = ", ".join(
            f"{k.split('.')[-1]}={len(catalog.get(k, []))}" for k in catalog.keys()
        )
        cprint(f"✓ Catalog: {counts_str} (took {t1.elapsed:.1f}s)", DIM)
        logger.info(
            "catalog:samples context.tags=%s",
            json.dumps((catalog.get("context.tags") or [])[:20]),
        )
        logger.info(
            "catalog:samples concepts.name=%s",
            json.dumps((catalog.get("concepts.name") or [])[:20]),
        )
        logger.info(
            "catalog:samples entities.name=%s",
            json.dumps((catalog.get("entities.name") or [])[:20]),
        )
        logger.info(
            "catalog:samples relations.subject=%s",
            json.dumps((catalog.get("relations.subject") or [])[:20]),
        )
        with Timer("insights") as t2:
            insights = build_insights(col, limit=5000)
        cprint(
            f"✓ Insights: age_avg={insights.get('age_days_avg',0):.0f}d (took {t2.elapsed:.1f}s)",
            DIM,
        )
        logger.info("catalog:keys=%s", list(catalog.keys()))
        logger.info(
            "insights:trust_avg=%s age_days_avg=%s",
            insights.get("trust_avg"),
            insights.get("age_days_avg"),
        )
    except Exception:
        catalog = {}
        insights = {}
        cprint("⚠ Metadata catalog and insights not built", YELLOW)

    cprint("\n" + "=" * 60, CYAN)
    cprint("Commands: :exit, :new, :history, :id, :export <fmt> [path]", DIM)
    cprint("=" * 60 + "\n", CYAN)
    while True:
        try:
            raw = input("\n> ").strip()
        except (EOFError, KeyboardInterrupt):
            print("\nGoodbye!")
            break

        if not raw:
            continue

        if raw in {":exit", ":quit"}:
            print("Goodbye!")
            break
        if raw == ":id":
            print(f"Session: {session_id}")
            continue
        if raw.startswith(":export"):
            parts = raw.split()
            fmt = parts[1] if len(parts) >= 2 else "json"
            path = parts[2] if len(parts) >= 3 else None
            exported = export_last_turn(last_turn, fmt, path, session_id)
            if exported:
                cprint(f"Exported last turn to {exported}", GREEN)
                logger.info(f"export: path={exported} fmt={fmt}")
            else:
                cprint("No previous turn to export.", YELLOW)
            continue
        if raw == ":history":
            for m in short_term_memory[-10:]:
                role = m.get("role")
                content = m.get("content", "")[:200]
                print(f"- {role}: {content}")
            continue
        if raw == ":new":
            session_id = generate_session_id()
            long_term_logs = load_long_term_memory(session_id=session_id, limit=20)
            short_term_memory = []
            print(f"New session: {session_id}")
            logger = setup_logger(session_id, args.log_dir)
            last_turn = None
            continue

        # 1) Update short-term memory with user turn
        # Detect if this is a follow-up question
        is_followup = False
        followup_keywords = [
            "go deeper",
            "more details",
            "also",
            "what about",
            "and",
            "it",
            "this",
            "that",
            "they",
            "these",
        ]
        if short_term_memory and any(kw in raw.lower() for kw in followup_keywords):
            is_followup = True
            logger.info(
                "continuity:detected is_followup=true query_preview=%s", raw[:100]
            )
            # Show conversation summary in terminal
            last_user = next(
                (m for m in reversed(short_term_memory) if m["role"] == "user"), None
            )
            if last_user:
                last_q = last_user["content"][:80]
                cprint(f"   💭 Context: continuing from '{last_q}...'", DIM)

        short_term_memory.append({"role": "user", "content": raw})

        # 2) Rewrite query (uses memory)
        if is_followup and (short_term_memory or long_term_logs):
            memory_count = (
                len(short_term_memory) - 1 + len(long_term_logs)
            )  # -1 to exclude current user message
            cprint(
                f"[1/5] Rewriting query (with {memory_count} turns of memory)...", CYAN
            )
        else:
            cprint("[1/5] Rewriting query...", CYAN)
        logger.info("rewrite:start is_followup=%s", is_followup)
        # PlannerAgent decides route/mode/k/filters; rewrite retains role for paraphrasing only
        planner = PlannerAgent()
        allowed_keys = [
            "context.tags",
            "concepts.name",
            "entities.name",
            "relations.subject",
            "metadata.age_days",
            "published_at",
            "trust_score",
        ]

        # Prune catalog to query-relevant values before passing to planner
        keywords = extract_query_keywords(raw, min_len=3)
        logger.info("keywords:extracted=%s", json.dumps(keywords[:20]))
        cprint(f"   Keywords: {', '.join(keywords[:8])}...", DIM)
        pruned_catalog = prune_catalog_for_query(
            catalog, raw, top_n=20, insights=insights
        )
        pruned_counts = {
            k: len(v) if isinstance(v, list) else 0 for k, v in pruned_catalog.items()
        }
        logger.info("catalog:pruned counts=%s", json.dumps(pruned_counts))
        cprint(
            f"   Pruned catalog: {sum(pruned_counts.values())} values (from {sum(len(catalog.get(k,[])) for k in catalog.keys())})",
            DIM,
        )
        # Build conversation context summary for planner
        conversation_context = None
        if short_term_memory or long_term_logs:
            ctx_parts = []
            # Last 3 turns from short-term (exclude current user message)
            for msg in short_term_memory[-6:-1] if len(short_term_memory) > 1 else []:
                role_label = "User" if msg["role"] == "user" else "Assistant"
                ctx_parts.append(f"{role_label}: {msg['content'][:200]}...")
            # Last 2 turns from long-term with enhanced context
            for log in long_term_logs[:2]:
                q = log.get("user_query_raw", "")[:150]
                filters_used = log.get("filters", {})
                filters_str = ", ".join(filters_used.keys()) if filters_used else "none"
                # Try to extract topics from answer or filters
                topics = []
                if filters_used:
                    # Extract filter values as topic hints
                    for k, v in filters_used.items():
                        if isinstance(v, dict) and "$in" in v:
                            topics.extend(v["$in"][:3])
                        elif isinstance(v, list):
                            topics.extend(v[:3])
                topics_str = (
                    ", ".join(str(t) for t in topics[:3]) if topics else "unknown"
                )
                ctx_parts.append(
                    f"Previous Q: {q}\n  Filters: {filters_str}\n  Topics: {topics_str}"
                )
            if ctx_parts:
                conversation_context = "\n".join(ctx_parts)
                logger.info(
                    "planner:conversation_context turns=%s preview=%s",
                    len(ctx_parts),
                    conversation_context[:300],
                )
                if is_followup:
                    cprint(
                        f"   💡 Planner aware of {len(ctx_parts)} previous turns", DIM
                    )

        with Timer("planner") as tplan:
            decision = planner.decide(
                question=raw,
                catalog=pruned_catalog,  # Pass pruned catalog, not full
                constraints={
                    "allowed_keys": allowed_keys,
                    "defaults": {"mode": args.mode, "k": int(args.top_k)},
                },
                conversation_context=conversation_context,
            )
        # Log full planner context for troubleshooting
        logger.info(
            "planner:elapsed_s=%.3f decision=%s allowed_keys=%s defaults=%s",
            tplan.elapsed,
            json.dumps(decision, ensure_ascii=False),
            json.dumps(allowed_keys),
            json.dumps({"mode": args.mode, "k": int(args.top_k)}),
        )

        # CODE-LEVEL ENFORCEMENT: Validate filter continuity and auto-correct if needed
        if conversation_context and is_followup:
            # Extract previous filter values from long-term logs
            prev_filter_values = set()
            for log in long_term_logs[:2]:
                prev_filters = log.get("filters", {})
                for k, v in prev_filters.items():
                    if isinstance(v, dict) and "$in" in v:
                        prev_filter_values.update(str(x) for x in v["$in"][:3])
                    elif isinstance(v, list):
                        prev_filter_values.update(str(x) for x in v[:3])
                    elif isinstance(v, str):
                        prev_filter_values.add(str(v))

            # Check if current filters have ANY overlap with previous
            current_filters = decision.get("filters", {})
            current_filter_values = set()
            if current_filters:
                for k, v in current_filters.items():
                    if isinstance(v, list):
                        current_filter_values.update(str(x) for x in v)
                    elif isinstance(v, str):
                        current_filter_values.add(str(v))

            has_overlap = bool(prev_filter_values & current_filter_values)

            if prev_filter_values and not has_overlap and current_filters:
                # VIOLATION: No continuity! Auto-inject one previous filter
                logger.warning(
                    "continuity:violation planner_ignored_previous prev=%s current=%s",
                    list(prev_filter_values)[:5],
                    list(current_filter_values)[:5],
                )

                # Try to inject one relevant previous filter
                # Prefer context.tags as it's the most common field
                if "context.tags" in current_filters:
                    # Find most relevant previous filter value from FULL catalog (not pruned)
                    # Pruned catalog won't have previous topics if keywords changed
                    for prev_val in list(prev_filter_values)[:3]:
                        if prev_val in (catalog.get("context.tags") or []):
                            if isinstance(current_filters["context.tags"], list):
                                current_filters["context.tags"].insert(0, prev_val)
                            else:
                                current_filters["context.tags"] = [
                                    prev_val,
                                    current_filters["context.tags"],
                                ]
                            logger.info(
                                "continuity:auto_fix injected_filter=%s field=context.tags",
                                prev_val,
                            )
                            cprint(
                                f"   🔧 Auto-injected previous filter: {prev_val}", CYAN
                            )
                            # Update decision
                            decision["filters"] = current_filters
                            break
                elif "concepts.name" in current_filters:
                    # Fallback: try injecting into concepts.name
                    for prev_val in list(prev_filter_values)[:3]:
                        if prev_val in (catalog.get("concepts.name") or []):
                            if isinstance(current_filters["concepts.name"], list):
                                current_filters["concepts.name"].insert(0, prev_val)
                            else:
                                current_filters["concepts.name"] = [
                                    prev_val,
                                    current_filters["concepts.name"],
                                ]
                            logger.info(
                                "continuity:auto_fix injected_filter=%s field=concepts.name",
                                prev_val,
                            )
                            cprint(
                                f"   🔧 Auto-injected previous filter: {prev_val}", CYAN
                            )
                            decision["filters"] = current_filters
                            break
        # Also log planner raw prompts (development mode)
        try:
            logger.info("planner:system=%s", getattr(planner, "last_system_prompt", ""))
            logger.info("planner:user=%s", getattr(planner, "last_user_prompt", ""))
        except Exception:
            pass
        route = str(decision.get("route") or "reference_answer")
        retr = decision.get("retrieval") or {}
        tool_mode = str(retr.get("mode") or args.mode)
        k = int(retr.get("k") or int(args.top_k))
        filters = (
            decision.get("filters")
            if isinstance(decision.get("filters"), dict)
            else None
        )
        # If planner chose no filters but index is available and catalog present, try a light suggestion
        if not filters:
            try:
                # pick one tag that appears in the query as a minimal filter
                qlow = raw.lower()
                tag_candidates = (catalog.get("context.tags") or [])[:50]
                chosen = None
                for t in tag_candidates:
                    ts = str(t)
                    if len(ts) > 2 and ts.lower() in qlow:
                        chosen = ts
                        break
                if chosen:
                    filters = {"context.tags": chosen}
                    logger.info("filters:auto_suggested=%s", json.dumps(filters))
            except Exception:
                pass

        with Timer("rewrite") as trew:
            rewritten, _, _, _ = rewrite_query(
                user_query=raw,
                short_term_msgs=short_term_memory,
                long_term_logs=long_term_logs,
                default_mode=args.mode,
                default_k=int(args.top_k),
                catalog=catalog,
                logger=logger,
            )
        logger.info("rewrite:elapsed_s=%.3f", trew.elapsed)

        # CODE-LEVEL ENFORCEMENT: Auto-expand query if LLM ignored memory on follow-up
        if (
            is_followup
            and rewritten.strip() == raw.strip()
            and (short_term_memory or long_term_logs)
        ):
            # Extract topic from most recent turn
            topic_from_memory = None
            # Try long-term first (most recent question)
            if long_term_logs:
                prev_q = long_term_logs[0].get("user_query_raw", "")
                # Extract key terms (simple heuristic: take important words)
                words = [
                    w
                    for w in prev_q.split()
                    if len(w) > 4
                    and w.lower()
                    not in {"about", "would", "could", "should", "having", "creating"}
                ]
                if words:
                    topic_from_memory = " ".join(words[:5])
            # Fallback to short-term
            if not topic_from_memory and len(short_term_memory) > 1:
                for msg in reversed(short_term_memory[:-1]):
                    if msg["role"] == "user":
                        words = [w for w in msg["content"].split() if len(w) > 4]
                        if words:
                            topic_from_memory = " ".join(words[:5])
                        break

            # Force expansion using template
            if topic_from_memory:
                # Simple template-based expansion
                if "go deeper" in raw.lower():
                    rewritten = f"{raw} in the context of {topic_from_memory}"
                elif "what about" in raw.lower():
                    rewritten = f"{raw} for {topic_from_memory}"
                elif any(kw in raw.lower() for kw in ["it", "this", "that"]):
                    rewritten = f"{raw.replace('it', topic_from_memory).replace('this', topic_from_memory).replace('that', topic_from_memory)}"
                else:
                    rewritten = f"{raw} (in context of: {topic_from_memory})"

                logger.info(
                    "rewrite:auto_expanded topic=%s new_query=%s",
                    topic_from_memory,
                    rewritten[:200],
                )
                cprint(f"   🔧 Auto-expanded query with memory context", CYAN)
        # Force vector mode when filters are present (hybrid doesn't support filters)
        effective_mode = args.mode if args.mode != "auto" else tool_mode
        if effective_mode == "auto":
            effective_mode = "vector"  # default path
        if filters and effective_mode in {"hybrid", "auto"}:
            effective_mode = "vector"
            logger.info("mode:override hybrid→vector (filters present)")
        # Show if rewrite actually changed the query
        query_changed = rewritten.strip() != raw.strip()
        logger.info(
            "rewrite:done raw=%s rewritten=%s changed=%s mode=%s k=%s filters=%s",
            raw[:200],
            rewritten[:200],
            query_changed,
            effective_mode,
            k,
            json.dumps(filters or {}),
        )
        if query_changed and is_followup:
            cprint(f"   → Query expanded using memory context", DIM)
        elif (
            is_followup and not query_changed and (short_term_memory or long_term_logs)
        ):
            cprint(
                f"   ⚠️  Memory available but query not expanded (check logs)", YELLOW
            )
            logger.warning(
                "rewrite:memory_ignored short_term=%s long_term=%s",
                len(short_term_memory) - 1,
                len(long_term_logs),
            )
        if filters:
            cprint(f"[2/5] Plan: mode={effective_mode}, k={k}", BLUE)
            for field, vals in filters.items():
                if isinstance(vals, list):
                    vals_preview = ", ".join([str(v) for v in vals[:6]])
                    if len(vals) > 6:
                        vals_preview += f", ... ({len(vals)} total)"
                    cprint(f"     • {field}: {vals_preview}", DIM)
                else:
                    cprint(f"     • {field}: {vals}", DIM)
        else:
            cprint(f"[2/5] Plan: mode={effective_mode}, k={k} (no filters)", BLUE)

        # 3) Retrieve
        cprint("[3/5] Retrieving context...", MAGENTA)
        logger.info("retrieve:start mode=%s k=%s", effective_mode, k)
        sanitized = sanitize_filters(filters, full_catalog=catalog)
        logger.info(
            "filters:raw=%s sanitized=%s expanded_count=%s",
            json.dumps(filters or {}),
            json.dumps(sanitized or {}, default=str)[:500],
            sum(
                len(v) if isinstance(v, dict) and "$in" in v else 1
                for v in (sanitized or {}).values()
            ),
        )
        with Timer("retrieval") as tret:
            hits = run_retrieval(
                mode=effective_mode,
                query_text=rewritten,
                top_k=k,
                filters=sanitized,
            )
        logger.info(
            "retrieve:elapsed_s=%.3f hits=%s mode=%s k=%s",
            tret.elapsed,
            len(hits),
            effective_mode,
            k,
        )
        # Count unique videos
        video_ids = list(set(h.get("video_id") for h in hits if h.get("video_id")))
        cprint(
            f"[3/5] Retrieved {len(hits)} chunks from {len(video_ids)} videos ({tret.elapsed:.1f}s)",
            MAGENTA,
        )

        # 4) Answer via TopicReferenceAgent for overview/guide queries, else ReferenceAnswerAgent
        cprint("[4/5] Generating answer...", YELLOW)
        # Diversify context (MMR) before topic/reference agents
        try:
            from src.domain.rag.services.retrieval import mmr_diversify

            diversified = mmr_diversify(hits, lambda_=0.7, top_k=k)
            contexts = normalize_context_blocks(diversified)
            logger.info(
                "mmr:applied true count_before=%s count_after=%s",
                len(hits),
                len(diversified),
            )
        except Exception:
            contexts = normalize_context_blocks(hits)
            logger.info("mmr:applied false count=%s", len(hits))
        is_overview = (route == "topic_reference") or any(
            kw in rewritten.lower()
            for kw in [
                "guide",
                "overview",
                "blueprint",
                "best practices",
                "roadmap",
                "detailed technical document",
                "comprehensive",
                "all relevant topics",
                "plan",
            ]
        )
        if route == "topic_reference" or is_overview:
            logger.info("answer:start route=topic_reference")
            groups: Dict[str, Dict[str, Any]] = {}
            # Group by tags and concepts to form topics; fallback to video_id for diversity
            for h in contexts:
                meta = h.get("metadata", {}) or {}
                tags = (meta.get("tags") or [])[:3]
                concepts = h.get("concepts") or []
                cname = (
                    concepts[0].get("name")
                    if concepts and isinstance(concepts[0], dict)
                    else None
                )
                # Prefer tag, then concept, then video_id for grouping
                if tags and tags[0]:
                    topic = tags[0]
                elif cname:
                    topic = cname
                else:
                    # Fallback to video_id to avoid lumping all into "general"
                    topic = str(h.get("video_id") or "general")[:60]
                grp = groups.setdefault(
                    topic, {"topic": topic, "bullets": [], "quotes": [], "refs": []}
                )
                txt = h.get("embedding_text") or h.get("text") or ""
                if txt:
                    grp["bullets"].append(txt[:220])
                    grp["quotes"].append(txt[:160])
                anchor = _anchor_from_chunk(h)
                grp["refs"].append(
                    {
                        "title": anchor.get("title"),
                        "url": anchor.get("url"),
                        "hint": anchor.get("hint"),
                    }
                )
            topic_bundles = list(groups.values())[:6]
            logger.info("topic:doc_groups=%s", len(topic_bundles))
            topic_agent = TopicReferenceAgent()
            with Timer("answer_topic") as tans:
                answer = topic_agent.answer(rewritten, topic_bundles)
            logger.info(
                "answer:done route=topic_reference elapsed_s=%.3f chars=%s topics=%s",
                tans.elapsed,
                len(answer or ""),
                len(topic_bundles),
            )
        else:
            logger.info("answer:start route=reference_answer")
            ref_bundles = build_reference_bundles(contexts)
            logger.info("reference:doc_bundles=%s", len(ref_bundles))
            ref_agent = ReferenceAnswerAgent()
            with Timer("answer_ref") as rans:
                answer = ref_agent.answer(rewritten, ref_bundles)
            logger.info(
                "answer:done route=reference_answer elapsed_s=%.3f chars=%s refs_used=%s",
                rans.elapsed,
                len(answer or ""),
                len(ref_bundles),
            )

        # 5) Persist
        try:
            logger.info("persist:start")
            persist_turn(
                session_id=session_id,
                raw_query=raw,
                rewritten_query=rewritten,
                mode=effective_mode,
                top_k=k,
                filters=filters,
                hits=hits,
                answer=answer,
                agent=(
                    "topic_reference"
                    if (route == "topic_reference" or is_overview)
                    else "reference_answer"
                ),
            )
            # Refresh long-term cache
            long_term_logs = load_long_term_memory(session_id=session_id, limit=20)
            logger.info("persist:done")

            # Log retrieved chunks for ETL validation and debugging
            try:
                # Quick stats
                video_counts = {}
                for h in hits:
                    vid = h.get("video_id")
                    if vid:
                        video_counts[vid] = video_counts.get(vid, 0) + 1
                logger.info(
                    "chunks:total=%s unique_videos=%s", len(hits), len(video_counts)
                )
                logger.info(
                    "chunks:by_video=%s",
                    json.dumps({k: v for k, v in list(video_counts.items())[:10]}),
                )

                # First 10 chunks preview
                logger.info("=== CHUNK PREVIEW (first 10) ===")
                for idx, h in enumerate(hits[:10], 1):
                    chunk_text = h.get("embedding_text") or h.get("text") or ""
                    meta = h.get("metadata") or {}
                    logger.info(
                        "chunk:%s video_id=%s chunk_id=%s score=%.3f tags=%s text=%s",
                        idx,
                        h.get("video_id"),
                        h.get("chunk_id"),
                        float(h.get("score") or h.get("search_score") or 0.0),
                        (meta.get("tags") or [])[:3],
                        chunk_text[:300].replace("\n", " "),
                    )
                logger.info("=== END PREVIEW ===")

                # Full dump for thorough ETL validation
                logger.info(
                    "=== FULL CHUNK DUMP (all %s chunks for ETL validation) ===",
                    len(hits),
                )
                for idx, h in enumerate(hits, 1):
                    chunk_text = h.get("embedding_text") or h.get("text") or ""
                    meta = h.get("metadata") or {}
                    ctx = h.get("context") or {}
                    ents = h.get("entities") or []
                    concepts = h.get("concepts") or []
                    rels = h.get("relations") or []
                    logger.info(
                        "CHUNK[%s/%s] video_id=%s chunk_id=%s score=%.3f "
                        "meta.tags=%s ctx.tags=%s entities=%s concepts=%s relations=%s "
                        "text=%s",
                        idx,
                        len(hits),
                        h.get("video_id"),
                        h.get("chunk_id"),
                        float(h.get("score") or h.get("search_score") or 0.0),
                        meta.get("tags", []),
                        ctx.get("tags", []),
                        [e.get("name") for e in ents[:5]],
                        [c.get("name") for c in concepts[:5]],
                        [f"{r.get('subject')}→{r.get('object')}" for r in rels[:3]],
                        chunk_text[:600].replace("\n", " "),
                    )
                logger.info("=== END CHUNK DUMP ===")
            except Exception as e:
                logger.exception("chunk_logging:error %s", e)
        except Exception as e:
            print(f"[warn] failed to persist log: {e}")
            logger.exception("persist:error %s", e)

        # 6) Emit to CLI and update short-term memory
        cprint("\n" + "=" * 60, GREEN)
        cprint("[5/5] Answer:", GREEN)
        cprint("=" * 60 + "\n", GREEN)
        print(answer or "(no response)")
        if hits:
            cprint("\n" + "-" * 60, DIM)
            cprint("Top Citations:", DIM)
            cprint("-" * 60, DIM)
            print(format_citations(hits, max_items=3))
            cprint(
                f"\n✓ Retrieved {len(hits)} chunks from {len(set(h.get('video_id') for h in hits))} videos",
                DIM,
            )
        # Store assistant response with metadata for better memory
        short_term_memory.append(
            {
                "role": "assistant",
                "content": answer,
                "metadata": {
                    "filters_used": list(sanitized.keys()) if sanitized else [],
                    "topics_covered": list(
                        set(
                            (
                                h.get("context", {}).get("tags", [])[:1][0]
                                if h.get("context", {}).get("tags")
                                else None
                            )
                            for h in contexts[:10]
                            if h.get("context")
                        )
                    )[:5],
                    "num_chunks": len(hits),
                    "route": route,
                },
            }
        )

        # Update exportable snapshot
        last_turn = {
            "user_query_raw": raw,
            "user_query_rewritten": rewritten,
            "mode": effective_mode,
            "k": k,
            "filters": filters or {},
            "hits": hits,
            "answer": answer,
        }


if __name__ == "__main__":
    # TODO:
    # skip for now: in the frontend it has to be possible to exclude questions/answer from a conversation (from the short and long term memory)
    run_cli()
