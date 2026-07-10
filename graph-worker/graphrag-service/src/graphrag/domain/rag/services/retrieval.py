import os
import logging
import time
from typing import Any, Dict, List, Optional

from src.core.config.runtime import (
    RAG_WEIGHT_VECTOR,
    RAG_WEIGHT_TRUST,
    RAG_WEIGHT_RECENCY,
)

from pymongo.collection import Collection
from pymongo.errors import OperationFailure

from src.domain.services.rag.indexes import get_vector_index_name, SEARCH_INDEX_NAME
from src.lib.error_handling.decorators import handle_errors
from src.lib.metrics import Counter, Histogram, MetricRegistry

# Mongo Documentation on Vector Search: https://www.mongodb.com/docs/atlas/atlas-vector-search/vector-search-overview/

# Configure logging
logger = logging.getLogger(__name__)

# Initialize retrieval service metrics
_rag_retrieval_calls = Counter(
    "rag_retrieval_calls", "Number of retrieval calls", labels=["method"]
)
_rag_retrieval_errors = Counter(
    "rag_retrieval_errors", "Number of retrieval errors", labels=["method"]
)
_rag_retrieval_duration = Histogram(
    "rag_retrieval_duration_seconds", "Retrieval call duration", labels=["method"]
)

# Register metrics
_registry = MetricRegistry.get_instance()
_registry.register(_rag_retrieval_calls)
_registry.register(_rag_retrieval_errors)
_registry.register(_rag_retrieval_duration)

DEFAULT_VECTOR_INDEX = get_vector_index_name()


def mmr_diversify(
    hits: List[Dict[str, Any]],
    lambda_: float = 0.7,
    top_k: Optional[int] = None,
) -> List[Dict[str, Any]]:
    """Simple MMR-like diversification.

    Uses vector/keyword score and tag overlap as a crude diversity signal.
    """
    if not hits:
        return []
    k = min(len(hits), int(top_k) if top_k else len(hits))

    def base_score(h: Dict[str, Any]) -> float:
        s = h.get("score") or h.get("search_score") or 0.0
        try:
            return float(s)
        except Exception:
            return 0.0

    selected: List[Dict[str, Any]] = []
    candidates = hits[:]
    while candidates and len(selected) < k:
        best = None
        best_val = -1e9
        for h in candidates:
            s = base_score(h)
            penalty = 0.0
            tags = set(((h.get("metadata") or {}).get("tags") or [])[:6])
            for selt in selected:
                stags = set(((selt.get("metadata") or {}).get("tags") or [])[:6])
                if tags and stags:
                    overlap = len(tags & stags) / max(1, len(tags | stags))
                    penalty = max(penalty, overlap)
            val = lambda_ * s - (1 - lambda_) * penalty
            if val > best_val:
                best_val = val
                best = h
        if best is None:
            break
        selected.append(best)
        candidates.remove(best)
    return selected


@handle_errors(fallback=[], log_traceback=True, reraise=False)
def hybrid_search(
    col: Collection,
    query_text: str,
    query_vector: List[float],
    top_k: int = 8,
    filters: Optional[Dict[str, Any]] = None,
) -> List[Dict[str, Any]]:
    """Run a hybrid Atlas Search query (text + vector).

    Per Atlas docs, knnBeta must be a top-level operator under $search, not nested
    inside compound.should. We therefore place knnBeta at the top level and, when
    a lexical query is present, add a compound.should text operator alongside it.

    Note: $search filters use Search operators (equals/in/range). Since our app
    builds filters for $vectorSearch, we ignore filters here to avoid invalid
    mapping errors and rely on vector-only search when filters are present.
    """
    start_time = time.time()
    labels = {"method": "hybrid_search"}
    _rag_retrieval_calls.inc(labels=labels)
    
    try:
        if not query_vector:
            duration = time.time() - start_time
            _rag_retrieval_duration.observe(duration, labels=labels)
            return []

        search_spec: Dict[str, Any] = {
            "index": SEARCH_INDEX_NAME,
            "knnBeta": {
                "vector": query_vector,
                "path": "embedding",
                "k": max(1, int(top_k)),
            },
        }
        if query_text and query_text.strip():
            search_spec["compound"] = {
                "should": [
                    {"text": {"query": query_text, "path": ["text", "display_text"]}}
                ]
            }
        if filters:
            logger.info(
                "hybrid: ignoring filters for $search; using vector path when filters are required"
            )
        search_stage: Dict[str, Any] = {"$search": search_spec}

        # Include scoreDetails to approximate per-operator contributions when available
        pipeline = [
            search_stage,
            {
                "$project": {
                    "video_id": 1,
                    "chunk_id": 1,
                    "embedding_text": 1,
                    "text": {"$ifNull": ["$display_text", "$text"]},
                    "metadata": 1,
                    "context": 1,
                    "entities": 1,
                    "concepts": 1,
                    "relations": 1,
                    "trust_score": 1,
                    "search_score": {"$meta": "searchScore"},
                    "_sd": {"$meta": "searchScoreDetails"},
                }
            },
            {
                "$addFields": {
                    "keyword_score": {
                        "$let": {
                            "vars": {"d": "$_sd"},
                            "in": {
                                "$cond": [
                                    {"$gt": [{"$type": "$$d"}, "missing"]},
                                    {"$ifNull": ["$$d.textScore", None]},
                                    None,
                                ]
                            },
                        }
                    },
                    "vector_score": {
                        "$let": {
                            "vars": {"d": "$_sd"},
                            "in": {
                                "$cond": [
                                    {"$gt": [{"$type": "$$d"}, "missing"]},
                                    {"$ifNull": ["$$d.vectorSearchScore", None]},
                                    None,
                                ]
                            },
                        }
                    },
                }
            },
            {"$project": {"_sd": 0}},
            {"$limit": int(top_k)},
        ]
        logger.info(f"Executing hybrid search with knnBeta + text operators")
        results = list(col.aggregate(pipeline))
        duration = time.time() - start_time
        _rag_retrieval_duration.observe(duration, labels=labels)
        return results
    except Exception as e:
        _rag_retrieval_errors.inc(labels=labels)
        logger.warning(
            f"Hybrid search failed: {e}. Falling back to vector-only search."
        )
        # Fallback to vectorSearch only
        vs_stage: Dict[str, Any] = {
            "$vectorSearch": {
                "index": DEFAULT_VECTOR_INDEX,
                "path": "embedding",
                "queryVector": query_vector,
                "numCandidates": max(500, top_k * 10),
                "limit": top_k,
            }
        }
        if filters:
            try:
                if isinstance(filters, dict) and len(filters) > 0:
                    vs_stage["$vectorSearch"]["filter"] = filters
            except Exception:
                pass
        vs_pipeline = [
            vs_stage,
            {
                "$project": {
                    "video_id": 1,
                    "chunk_id": 1,
                    "embedding_text": 1,
                    "text": {"$ifNull": ["$display_text", "$text"]},
                    "metadata": 1,
                    "context": 1,
                    "entities": 1,
                    "concepts": 1,
                    "relations": 1,
                    "trust_score": 1,
                    "search_score": {"$meta": "vectorSearchScore"},
                    "vector_score": {"$meta": "vectorSearchScore"},
                    "keyword_score": None,
                }
            },
        ]
        try:
            fallback_results = list(col.aggregate(vs_pipeline))
            duration = time.time() - start_time
            _rag_retrieval_duration.observe(duration, labels=labels)
            return fallback_results
        except Exception as fallback_error:
            _rag_retrieval_errors.inc(labels=labels)
            duration = time.time() - start_time
            _rag_retrieval_duration.observe(duration, labels=labels)
            logger.error(f"Vector search fallback also failed: {fallback_error}")
            return []


def keyword_search(
    col: Collection,
    query_text: str,
    top_k: int = 8,
    filters: Optional[Dict[str, Any]] = None,
) -> List[Dict[str, Any]]:
    if not (query_text and query_text.strip()):
        return []
    stage: Dict[str, Any] = {
        "$search": {
            "text": {
                "query": query_text,
                "path": ["text", "display_text"],
            }
        }
    }
    if filters:
        stage["$search"]["filter"] = filters
    pipeline = [
        stage,
        {
            "$project": {
                "video_id": 1,
                "chunk_id": 1,
                "text": {"$ifNull": ["$display_text", "$text"]},
                "metadata": 1,
                "trust_score": 1,
                "search_score": {"$meta": "searchScore"},
            }
        },
        {"$limit": int(top_k)},
    ]
    try:
        logger.info(f"Executing keyword search for: '{query_text[:50]}...'")
        return list(col.aggregate(pipeline))
    except Exception as e:
        logger.error(f"Keyword search failed: {e}")
        return []


def structured_search(
    col: Collection,
    fields: Optional[List[str]] = None,
    filters: Optional[Dict[str, Any]] = None,
    sort_by: Optional[Dict[str, int]] = None,
    top_k: Optional[int] = None,
) -> List[Dict[str, Any]]:
    query = filters or {}
    projection = (
        {f: 1 for f in fields}
        if isinstance(fields, list)
        else (
            {f.strip(): 1 for f in fields.split(",")} if isinstance(fields, str) else {}
        )
    )
    try:
        logger.info(f"Executing structured search with query: {query}")
        if query:
            cur = col.find(query, projection)
        else:
            cur = col.find({}, projection)
        if sort_by:
            cur = cur.sort(list(sort_by.items()))
        if top_k:
            cur = cur.limit(int(top_k))
        results = list(cur)
        logger.info(f"Structured search returned {len(results)} results")
        return results
    except Exception as e:
        logger.error(f"Structured search failed: {e}")
        return []


@handle_errors(fallback=[], log_traceback=True, reraise=False)
def vector_search(
    col: Collection,
    query_vector: List[float],
    k: int = 8,
    filters: Optional[Dict[str, Any]] = None,
) -> List[Dict[str, Any]]:
    """Execute vector similarity search using $vectorSearch."""
    start_time = time.time()
    labels = {"method": "vector_search"}
    _rag_retrieval_calls.inc(labels=labels)
    
    try:
        vs_stage: Dict[str, Any] = {
            "$vectorSearch": {
                "index": DEFAULT_VECTOR_INDEX,
                "path": "embedding",
                "queryVector": query_vector,
                "numCandidates": max(500, k * 10),  # Dynamic numCandidates
                "limit": k,
            }
        }
        if filters:
            try:
                if isinstance(filters, Dict) and len(filters) > 0:
                    vs_stage["$vectorSearch"]["filter"] = filters
            except Exception:
                pass
        pipeline = [
        vs_stage,
        {
            "$project": {
                "video_id": 1,
                "chunk_id": 1,
                "embedding_text": 1,
                "text": 1,
                "metadata": 1,
                "context": 1,
                "entities": 1,
                "concepts": 1,
                "relations": 1,
                "trust_score": 1,
                "score": {"$meta": "vectorSearchScore"},
            }
        },
        ]
        logger.info(
            f"Executing vector search with k={k}, vector_dim={len(query_vector)}"
        )
        results = list(col.aggregate(pipeline))
        logger.info(f"Vector search returned {len(results)} results")
        duration = time.time() - start_time
        _rag_retrieval_duration.observe(duration, labels=labels)
        return results
    except Exception as e:
        _rag_retrieval_errors.inc(labels=labels)
        duration = time.time() - start_time
        _rag_retrieval_duration.observe(duration, labels=labels)
        logger.error(f"Vector search failed: {e}")
        return []


def rerank_hits(
    hits: List[Dict[str, Any]],
    w_vector: float = RAG_WEIGHT_VECTOR,
    w_trust: float = RAG_WEIGHT_TRUST,
    w_recency: float = RAG_WEIGHT_RECENCY,
) -> List[Dict[str, Any]]:
    """Rerank search hits using weighted scoring."""
    if not hits:
        logger.warning("No hits provided for reranking")
        return []

    logger.info(
        f"Reranking {len(hits)} hits with weights: vector={w_vector}, trust={w_trust}, recency={w_recency}"
    )

    ranked: List[Dict[str, Any]] = []
    for h in hits:
        meta = h.get("metadata", {})
        trust = float(h.get("trust_score", meta.get("trust_score", 0.5)) or 0.5)
        score = float(h.get("score", 0.0) or 0.0)
        age_days = float(meta.get("age_days", 180) or 180)
        recency = 1.0 / (1.0 + age_days / 180.0)
        final = w_vector * score + w_trust * trust + w_recency * recency
        h["final_score"] = final
        h["score_breakdown"] = {
            "vector_score": score,
            "trust_score": trust,
            "recency_score": recency,
            "weighted_vector": w_vector * score,
            "weighted_trust": w_trust * trust,
            "weighted_recency": w_recency * recency,
        }
        ranked.append(h)

    ranked.sort(key=lambda x: x.get("final_score", 0.0), reverse=True)
    logger.info(
        f"Reranking completed. Top score: {ranked[0].get('final_score', 0):.3f}"
        if ranked
        else "No results after reranking"
    )
    return ranked
