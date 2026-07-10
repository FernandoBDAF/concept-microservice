import time
from typing import Any, Dict, List, Tuple, Optional
from statistics import mean
from pymongo.collection import Collection
import os
import re

from src.lib.error_handling.decorators import handle_errors
from src.lib.metrics import Counter, Histogram, MetricRegistry

# Initialize metadata service metrics
_ingestion_metadata_calls = Counter(
    "ingestion_metadata_calls", "Number of metadata operations", labels=["operation"]
)
_ingestion_metadata_errors = Counter(
    "ingestion_metadata_errors", "Number of metadata operation errors", labels=["operation"]
)
_ingestion_metadata_duration = Histogram(
    "ingestion_metadata_duration_seconds", "Metadata operation duration", labels=["operation"]
)

# Register metrics
_registry = MetricRegistry.get_instance()
_registry.register(_ingestion_metadata_calls)
_registry.register(_ingestion_metadata_errors)
_registry.register(_ingestion_metadata_duration)


def top_distinct_values(col: Collection, field: str, limit: int = 50) -> List[Any]:
    pipeline = [
        (
            {"$unwind": f"${field}"}
            if field.endswith("[]")
            else {"$match": {"_id": {"$exists": True}}}
        ),
    ]
    # Simple distinct fallback
    try:
        fname = field[:-2] if field.endswith("[]") else field
        vals = col.distinct(fname)
        out = []
        for v in vals:
            if v is None:
                continue
            out.append(v)
            if len(out) >= limit:
                break
        return out
    except Exception:
        return []


@handle_errors(fallback={}, log_traceback=True, reraise=False)
def build_catalog(col: Collection, limit: int = 50) -> Dict[str, List[Any]]:
    """Build catalog of ALL distinct filterable values (unlimited for quality).

    Note: limit parameter is ignored for now to ensure planner sees full vocabulary.
    We retrieve all distinct values to maximize filter relevance and quality.

    Note: metadata.tags is excluded (empty in current data; see enrich_agent.py comments for future video-level tagging).
    """
    start_time = time.time()
    labels = {"operation": "build_catalog"}
    _ingestion_metadata_calls.inc(labels=labels)
    
    try:
        catalog: Dict[str, List[Any]] = {}
        # Filterable fields: removed metadata.tags (empty), relations.predicate/object (not useful)
        # Added entities.name, relations.subject for better semantic filtering
        for key in [
            "context.tags",
            "concepts.name",
            "entities.name",
            "relations.subject",
        ]:
            try:
                vals = col.distinct(key)
                cleaned = [v for v in vals if isinstance(v, (str, int, float))]
                # NO LIMIT - capture ALL values for best planner decisions
                catalog[key] = cleaned
            except Exception:
                catalog[key] = []
        # Persist full catalog snapshot for debugging (non-blocking best-effort)
        try:
            from pathlib import Path
            import json as _json

            # Save full catalog (not trimmed) to see all available filter values
            Path("chat_logs").mkdir(parents=True, exist_ok=True)
            Path("chat_logs/catalog_snapshot.json").write_text(
                _json.dumps(catalog, ensure_ascii=False, indent=2), encoding="utf-8"
            )
        except Exception:
            pass
        # Age buckets
        try:
            ages = col.distinct("metadata.age_days")
            ages = [a for a in ages if isinstance(a, (int, float))]
            buckets = [
                ("recent", 0, 30),
                ("quarter", 0, 90),
                ("year", 0, 365),
            ]
            catalog["metadata.age_days:buckets"] = [b[0] for b in buckets]
        except Exception:
            catalog["metadata.age_days:buckets"] = []
        duration = time.time() - start_time
        _ingestion_metadata_duration.observe(duration, labels=labels)
        return catalog
    except Exception as e:
        _ingestion_metadata_errors.inc(labels=labels)
        duration = time.time() - start_time
        _ingestion_metadata_duration.observe(duration, labels=labels)
        raise


@handle_errors(fallback={}, log_traceback=True, reraise=False)
def build_insights(col: Collection, limit: int = 50) -> Dict[str, Any]:
    """Fast, approximate insights using aggregation + sampling (non-blocking).

    Avoids N*count_documents loops that can stall the CLI on large corpora.
    """
    start_time = time.time()
    labels = {"operation": "build_insights"}
    _ingestion_metadata_calls.inc(labels=labels)
    
    try:
        insights: Dict[str, Any] = {}
        # sample size bounded to keep latency reasonable
        try:
            sample_size = int(os.getenv("INSIGHTS_SAMPLE_SIZE", "2000"))
        except Exception:
            sample_size = 2000
        sample_size = max(200, min(sample_size, 20000))

        def top_counts_array(field: str, out_key: str) -> None:
            try:
                pipeline = [
                    {"$sample": {"size": sample_size}},
                    {"$unwind": f"${field}"},
                    {"$match": {field: {"$type": "string"}}},
                    {"$group": {"_id": f"${field}", "c": {"$sum": 1}}},
                    {"$sort": {"c": -1}},
                    {"$limit": int(limit)},
                ]
                rows = list(col.aggregate(pipeline))
                insights[out_key] = [(str(r.get("_id")), int(r.get("c", 0))) for r in rows]
            except Exception:
                insights[out_key] = []

        def top_counts_concepts(out_key: str) -> None:
            try:
                pipeline = [
                    {"$sample": {"size": sample_size}},
                    {"$unwind": "$concepts"},
                    {"$match": {"concepts.name": {"$type": "string"}}},
                    {"$group": {"_id": "$concepts.name", "c": {"$sum": 1}}},
                    {"$sort": {"c": -1}},
                    {"$limit": int(limit)},
                ]
                rows = list(col.aggregate(pipeline))
                insights[out_key] = [(str(r.get("_id")), int(r.get("c", 0))) for r in rows]
            except Exception:
                insights[out_key] = []

        # Array-based counts via sampling
        top_counts_array("metadata.tags", "tag_counts")
        top_counts_array("context.tags", "context_tag_counts")
        top_counts_concepts("concept_counts")

        # Trust avg via aggregation
        try:
            pipeline = [
                {"$match": {"trust_score": {"$type": "number"}}},
                {"$sample": {"size": sample_size}},
                {"$group": {"_id": None, "avg": {"$avg": "$trust_score"}}},
            ]
            row = next(iter(col.aggregate(pipeline)), None)
            insights["trust_avg"] = (
                round(float(row.get("avg")), 3)
                if row and row.get("avg") is not None
                else None
            )
        except Exception:
            insights["trust_avg"] = None

        # Age days avg via aggregation
        try:
            pipeline = [
                {"$match": {"metadata.age_days": {"$type": "number"}}},
                {"$sample": {"size": sample_size}},
                {"$group": {"_id": None, "avg": {"$avg": "$metadata.age_days"}}},
            ]
            row = next(iter(col.aggregate(pipeline)), None)
            insights["age_days_avg"] = (
                round(float(row.get("avg")), 1)
                if row and row.get("avg") is not None
                else None
            )
        except Exception:
            insights["age_days_avg"] = None

        duration = time.time() - start_time
        _ingestion_metadata_duration.observe(duration, labels=labels)
        return insights
    except Exception as e:
        _ingestion_metadata_errors.inc(labels=labels)
        duration = time.time() - start_time
        _ingestion_metadata_duration.observe(duration, labels=labels)
        raise


def extract_query_keywords(query: str, min_len: int = 3) -> List[str]:
    """Extract meaningful keywords from query text.

    Simple approach: tokenize, lowercase, remove common stopwords, keep words >= min_len.
    """
    stopwords = {
        "a",
        "an",
        "and",
        "are",
        "as",
        "at",
        "be",
        "by",
        "for",
        "from",
        "has",
        "have",
        "in",
        "is",
        "it",
        "of",
        "on",
        "that",
        "the",
        "to",
        "was",
        "will",
        "with",
        "i",
        "am",
        "my",
        "me",
        "you",
        "your",
        "this",
        "these",
        "those",
        "would",
        "could",
        "should",
    }
    # Tokenize: split on non-alphanumeric
    tokens = re.findall(r"\b\w+\b", query.lower())
    keywords = [t for t in tokens if len(t) >= min_len and t not in stopwords]
    return keywords


def fuzzy_match_score(keyword: str, value: str) -> float:
    """Simple fuzzy match: substring match + edit distance heuristic.

    Returns score in [0, 1]. Higher = better match.
    """
    kw = keyword.lower()
    val = value.lower()

    # Exact substring match
    if kw in val or val in kw:
        return 1.0

    # Partial word match (common prefix/suffix)
    if val.startswith(kw) or val.endswith(kw):
        return 0.8
    if kw.startswith(val) or kw.endswith(val):
        return 0.8

    # Character overlap (simple Jaccard)
    kw_chars = set(kw)
    val_chars = set(val)
    if kw_chars and val_chars:
        overlap = len(kw_chars & val_chars) / len(kw_chars | val_chars)
        if overlap > 0.5:
            return overlap * 0.6

    return 0.0


def prune_catalog_for_query(
    catalog: Dict[str, List[Any]],
    query: str,
    top_n: int = 20,
    insights: Optional[Dict[str, Any]] = None,
) -> Dict[str, List[Any]]:
    """Prune catalog to top N most relevant values per field based on query keywords.

    Extracts keywords from query, fuzzy-matches against catalog values, returns top N
    per field. Uses insights frequency counts to boost popular terms when available.
    """
    keywords = extract_query_keywords(query, min_len=3)
    if not keywords:
        # If no keywords extracted, return first top_n of each field
        return {
            k: (v[:top_n] if isinstance(v, list) else v) for k, v in catalog.items()
        }

    pruned: Dict[str, List[Any]] = {}

    for field, values in catalog.items():
        if not isinstance(values, list) or not values:
            pruned[field] = values
            continue

        # Score each value against all keywords
        scored: List[Tuple[Any, float]] = []
        for val in values:
            if not isinstance(val, str):
                scored.append((val, 0.0))
                continue
            # Max score across all keywords
            max_score = max(
                (fuzzy_match_score(kw, val) for kw in keywords), default=0.0
            )
            # Boost by frequency if available in insights
            freq_boost = 0.0
            if insights:
                # Check if insights has counts for this field
                count_key = None
                if field == "metadata.tags":
                    count_key = "tag_counts"
                elif field == "context.tags":
                    count_key = "context_tag_counts"
                elif field == "concepts.name":
                    count_key = "concept_counts"

                if count_key and count_key in insights:
                    counts = insights[count_key]
                    if isinstance(counts, list):
                        # counts is [(value, count), ...]
                        for v, c in counts:
                            if v == val:
                                # Normalize count to [0, 0.2] boost
                                freq_boost = min(0.2, c / 1000.0)
                                break

            final_score = max_score + freq_boost
            scored.append((val, final_score))

        # Sort by score descending, take top N
        scored.sort(key=lambda x: x[1], reverse=True)
        pruned[field] = [v for v, s in scored[:top_n] if s > 0.0]

        # If no matches, keep first top_n alphabetically for fallback
        if not pruned[field]:
            pruned[field] = values[:top_n]

    return pruned


def expand_filter_values(
    filters: Dict[str, Any],
    full_catalog: Dict[str, List[Any]],
) -> Dict[str, Any]:
    """Expand filter values to include all semantic variants from full catalog.
    
    E.g., if filter is {"context.tags": ["RAG", "embedding"]}, expand to include
    all catalog values matching those patterns: ["RAG", "rag", "RAG framework", "embedding", "embeddings", ...]
    
    Uses same fuzzy logic as pruning but applies to filter expansion.
    """
    import re
    
    def matches_pattern(pattern: str, value: str) -> bool:
        """Check if value matches pattern (case-insensitive, word-boundary aware)."""
        if not isinstance(pattern, str) or not isinstance(value, str):
            return False
        p = pattern.lower()
        v = value.lower()
        
        # Short terms (≤3 chars): exact word boundary
        if len(p) <= 3:
            # Use word boundary regex
            regex = re.compile(r'\b' + re.escape(p) + r'\b', re.IGNORECASE)
            return bool(regex.search(v))
        else:
            # Longer terms: prefix match (embed matches embedding, embeddings, embedded)
            regex = re.compile(r'\b' + re.escape(p), re.IGNORECASE)
            return bool(regex.search(v))
    
    expanded: Dict[str, Any] = {}
    
    for field, filter_val in filters.items():
        catalog_vals = full_catalog.get(field, [])
        if not isinstance(catalog_vals, list):
            expanded[field] = filter_val
            continue
        
        # If filter is a list of strings, expand each
        if isinstance(filter_val, list):
            all_matches = []
            for pattern in filter_val:
                if not isinstance(pattern, str):
                    all_matches.append(pattern)
                    continue
                # Find all catalog values matching this pattern
                matches = [v for v in catalog_vals if matches_pattern(pattern, str(v))]
                all_matches.extend(matches)
            # Deduplicate
            expanded[field] = list(dict.fromkeys(all_matches)) if all_matches else filter_val
        elif isinstance(filter_val, str):
            # Single string pattern
            matches = [v for v in catalog_vals if matches_pattern(filter_val, str(v))]
            expanded[field] = matches if matches else [filter_val]
        else:
            # Non-string (numeric, etc.)
            expanded[field] = filter_val
    
    return expanded


from typing import Any, Dict, Optional

import os
from googleapiclient.discovery import build


@handle_errors(fallback={"video_id": None}, log_traceback=True, reraise=False)
def get_youtube_metadata(
    video_id: str, api_key: Optional[str] = None
) -> Dict[str, Any]:
    start_time = time.time()
    labels = {"operation": "get_youtube_metadata"}
    _ingestion_metadata_calls.inc(labels=labels)
    
    try:
        key = api_key or os.getenv("YOUTUBE_API_KEY")
        if not key:
            raise RuntimeError("YOUTUBE_API_KEY is not set")
        yt = build("youtube", "v3", developerKey=key)
        resp = (
            yt.videos()
            .list(part="snippet,contentDetails,statistics", id=video_id)
            .execute()
        )
        items = resp.get("items", [])
        if not items:
            duration = time.time() - start_time
            _ingestion_metadata_duration.observe(duration, labels=labels)
            return {"video_id": video_id}
        it = items[0]
        snippet = it.get("snippet", {})
        stats = it.get("statistics", {})
        content = it.get("contentDetails", {})
        thumbs = snippet.get("thumbnails", {}) or {}
        thumb_url = None
        for key in ["maxres", "standard", "high", "medium", "default"]:
            if key in thumbs and thumbs[key].get("url"):
                thumb_url = thumbs[key]["url"]
                break
        result = {
            "video_id": it.get("id"),
            "title": snippet.get("title"),
            "description": snippet.get("description"),
            "channel_id": snippet.get("channelId"),
            "channel_title": snippet.get("channelTitle"),
            "published_at": snippet.get("publishedAt"),
            "tags": snippet.get("tags", []) or [],
            "category_id": snippet.get("categoryId"),
            "thumbnail_url": thumb_url,
            "duration": content.get("duration"),
            "view_count": int(stats.get("viewCount", 0) or 0),
            "like_count": int(stats.get("likeCount", 0) or 0),
            "comment_count": int(stats.get("commentCount", 0) or 0),
        }
        duration = time.time() - start_time
        _ingestion_metadata_duration.observe(duration, labels=labels)
        return result
    except Exception as e:
        _ingestion_metadata_errors.inc(labels=labels)
        duration = time.time() - start_time
        _ingestion_metadata_duration.observe(duration, labels=labels)
        raise
