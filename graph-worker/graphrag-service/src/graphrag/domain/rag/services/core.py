import os
import logging
import time
from typing import Any, Dict, Iterable, List, Optional

import requests
from pymongo import MongoClient

from src.infrastructure.database.mongodb import get_mongo_client
from src.lib.rate_limiting import RateLimiter  # Migrated from dependencies/llm/
from src.lib.error_handling.decorators import handle_errors
from src.lib.metrics import Counter, Histogram, MetricRegistry
from src.core.config.paths import DB_NAME, COLL_CHUNKS, COLL_MEMORY_LOGS
from src.core.config.runtime import RAG_WEIGHT_VECTOR, RAG_WEIGHT_TRUST, RAG_WEIGHT_RECENCY
from src.domain.services.rag.retrieval import vector_search, rerank_hits
from src.domain.services.rag.generation import (
    answer_with_openai,
    stream_answer_with_openai,
)
from src.domain.services.rag.retrieval import hybrid_search as _hybrid_search
import pandas as pd
from src.domain.services.rag.indexes import (
    ensure_vector_search_index,
    ensure_hybrid_search_index,
)

logger = logging.getLogger(__name__)

# Initialize RAG service metrics
_rag_service_calls = Counter(
    "rag_service_calls", "Number of RAG service calls", labels=["service", "method"]
)
_rag_service_errors = Counter(
    "rag_service_errors", "Number of RAG service errors", labels=["service", "method"]
)
_rag_service_duration = Histogram(
    "rag_service_duration_seconds", "RAG service call duration", labels=["service", "method"]
)
_rag_embedding_calls = Counter(
    "rag_embedding_calls", "Number of embedding API calls", labels=["model"]
)
_rag_embedding_errors = Counter(
    "rag_embedding_errors", "Number of embedding API errors", labels=["model"]
)
_rag_embedding_duration = Histogram(
    "rag_embedding_duration_seconds", "Embedding API call duration", labels=["model"]
)

# Register metrics
_registry = MetricRegistry.get_instance()
_registry.register(_rag_service_calls)
_registry.register(_rag_service_errors)
_registry.register(_rag_service_duration)
_registry.register(_rag_embedding_calls)
_registry.register(_rag_embedding_errors)
_registry.register(_rag_embedding_duration)


def embed_query(text: str) -> List[float]:
    api_key = os.getenv("VOYAGE_API_KEY")
    if not api_key:
        raise RuntimeError("VOYAGE_API_KEY is not set")
    model = os.getenv("VOYAGE_EMBED_MODEL", "voyage-2")
    limiter = RateLimiter()
    
    start_time = time.time()
    labels = {"model": model}
    _rag_embedding_calls.inc(labels=labels)
    
    try:
        # Prefer official client
        try:
            import voyageai  # type: ignore

            client = voyageai.Client(
                api_key=api_key,
                max_retries=int(os.getenv("VOYAGE_MAX_RETRIES", "4")),
                timeout=int(os.getenv("VOYAGE_TIMEOUT", "30")),
            )
            limiter.wait()
            res = client.embed([text], model=model, input_type="query")
            result = list(res.embeddings[0])
        except Exception:
            # Fallback HTTP
            limiter.wait()
            r = requests.post(
                "https://api.voyageai.com/v1/embeddings",
                headers={
                    "Authorization": f"Bearer {api_key}",
                    "Content-Type": "application/json",
                },
                json={"model": model, "input": [text]},
                timeout=30,
            )
            r.raise_for_status()
            payload = r.json()
            if "data" in payload:
                result = payload["data"][0]["embedding"]
            elif "embeddings" in payload:
                result = payload["embeddings"][0]
            else:
                raise RuntimeError("Unexpected Voyage embeddings response shape")
        
        duration = time.time() - start_time
        _rag_embedding_duration.observe(duration, labels=labels)
        return result
    except Exception as e:
        _rag_embedding_errors.inc(labels=labels)
        duration = time.time() - start_time
        _rag_embedding_duration.observe(duration, labels=labels)
        raise


@handle_errors(fallback={"answer": "An error occurred while generating the RAG answer.", "hits": []}, log_traceback=True, reraise=False)
def rag_answer(
    query: str,
    k: int = 8,
    filters: Optional[Dict[str, Any]] = None,
    weights: Optional[Dict[str, float]] = None,
    streaming: bool = False,
    session_id: Optional[str] = None,
) -> Dict[str, Any]:
    from src.lib.database import get_database, get_collection

    start_time = time.time()
    labels = {"service": "rag", "method": "rag_answer"}
    _rag_service_calls.inc(labels=labels)
    
    try:
        client: MongoClient = get_mongo_client()
        db = get_database(client, DB_NAME)
        col = get_collection(db, COLL_CHUNKS)
        logs = get_collection(db, COLL_MEMORY_LOGS)

        ensure_vector_search_index(col)
        # Hybrid index is useful for $search paths
        try:
            ensure_hybrid_search_index(col)
        except Exception:
            pass

        qvec = embed_query(query)
        hits = vector_search(col, qvec, k=k, filters=filters)

        hits_df = pd.DataFrame(hits)
        print(hits_df)

        # wv = (weights or {}).get("vector", RAG_WEIGHT_VECTOR)
        # wt = (weights or {}).get("trust", RAG_WEIGHT_TRUST)
        # wr = (weights or {}).get("recency", RAG_WEIGHT_RECENCY)
        # total = max(1e-8, float(wv + wt + wr))
        # hits = rerank_hits(
        #     hits, w_vector=wv / total, w_trust=wt / total, w_recency=wr / total
        # )
        if streaming:
            # For now, collect streamed tokens into a single string so UI remains simple
            buf: List[str] = []
            for token in stream_answer_with_openai(hits, query):
                buf.append(token)
            answer = "".join(buf)
        else:
            answer = answer_with_openai(hits, query)

        mode = "vector"  # base path for now; hybrid UI path remains separate
        # logs.insert_one(
        #     {
        #         "query": query,
        #         "mode": mode,
        #         "session_id": session_id,
        #         "weights": {"vector": wv, "trust": wt, "recency": wr},
        #         "retrieved": [
        #             {
        #                 "video_id": h.get("video_id"),
        #                 "chunk_id": h.get("chunk_id"),
        #                 "score": h.get("score"),
        #             }
        #             for h in hits
        #         ],
        #         "answer": answer,
        #     }
        # )

        result = {"answer": answer, "hits": hits}
        duration = time.time() - start_time
        _rag_service_duration.observe(duration, labels=labels)
        return result
    except Exception as e:
        _rag_service_errors.inc(labels=labels)
        duration = time.time() - start_time
        _rag_service_duration.observe(duration, labels=labels)
        raise


@handle_errors(fallback={"answer": "An error occurred while generating the hybrid RAG answer.", "hits": []}, log_traceback=True, reraise=False)
def rag_hybrid_answer(
    query: str,
    k: int = 8,
    filters: Optional[Dict[str, Any]] = None,
    weights: Optional[Dict[str, float]] = None,
    streaming: bool = False,
    session_id: Optional[str] = None,
) -> Dict[str, Any]:
    start_time = time.time()
    labels = {"service": "rag", "method": "rag_hybrid_answer"}
    _rag_service_calls.inc(labels=labels)
    
    try:
        client: MongoClient = get_mongo_client()
        db = client[DB_NAME]
        col = db[COLL_CHUNKS]
        logs = db[COLL_MEMORY_LOGS]

        # Embed query for knnBeta path and pass full text for keyword path
        qvec = embed_query(query)
        hits = _hybrid_search(
            col, query_text=query, query_vector=qvec, top_k=k, filters=filters
        )

        # Normalize score field for rerank: use search_score as the base vector component
        for h in hits:
            if "score" not in h and "search_score" in h:
                h["score"] = h.get("search_score")

        wv = (weights or {}).get("vector", RAG_WEIGHT_VECTOR)
        wt = (weights or {}).get("trust", RAG_WEIGHT_TRUST)
        wr = (weights or {}).get("recency", RAG_WEIGHT_RECENCY)
        total = max(1e-8, float(wv + wt + wr))
        hits = rerank_hits(
            hits, w_vector=wv / total, w_trust=wt / total, w_recency=wr / total
        )

        if streaming:
            buf: List[str] = []
            for token in stream_answer_with_openai(hits, query):
                buf.append(token)
            answer = "".join(buf)
        else:
            answer = answer_with_openai(hits, query)

        logs.insert_one(
            {
                "query": query,
                "mode": "hybrid",
                "session_id": session_id,
                "weights": {"vector": wv, "trust": wt, "recency": wr},
                "retrieved": [
                    {
                        "video_id": h.get("video_id"),
                        "chunk_id": h.get("chunk_id"),
                        "search_score": h.get("search_score"),
                        "keyword_score": h.get("keyword_score"),
                        "vector_score": h.get("vector_score"),
                    }
                    for h in hits
                ],
                "answer": answer,
            }
        )

        result = {"answer": answer, "hits": hits}
        duration = time.time() - start_time
        _rag_service_duration.observe(duration, labels=labels)
        return result
    except Exception as e:
        _rag_service_errors.inc(labels=labels)
        duration = time.time() - start_time
        _rag_service_duration.observe(duration, labels=labels)
        raise


@handle_errors(fallback={"answer": "An error occurred while generating the GraphRAG answer.", "hits": []}, log_traceback=True, reraise=False)
def rag_graphrag_answer(
    query: str,
    k: int = 8,
    filters: Optional[Dict[str, Any]] = None,
    weights: Optional[Dict[str, float]] = None,
    streaming: bool = False,
    session_id: Optional[str] = None,
    use_traditional_rag: bool = True,
) -> Dict[str, Any]:
    """
    Generate answer using GraphRAG with optional traditional RAG integration.

    Args:
        query: User's query
        k: Number of chunks to retrieve
        filters: Optional filters for retrieval
        weights: Optional weights for reranking
        streaming: Whether to stream the answer
        session_id: Optional session ID for logging
        use_traditional_rag: Whether to combine with traditional RAG results

    Returns:
        Dictionary containing answer and metadata
    """
    logger.info(f"Processing GraphRAG query: {query}")

    start_time = time.time()
    labels = {"service": "rag", "method": "rag_graphrag_answer"}
    _rag_service_calls.inc(labels=labels)
    
    try:
        # Import GraphRAG components
        from src.domain.services.graphrag.generation import GraphRAGGenerationService
        from src.domain.services.graphrag.query import GraphRAGQueryProcessor
        from src.domain.services.graphrag.retrieval import GraphRAGRetrievalEngine
        from src.lib.database import get_database
        from src.lib.llm import get_openai_client

        # Initialize GraphRAG components
        client = MongoClient(os.getenv("MONGODB_URI", "mongodb://localhost:27017"))
        db = get_database(client, DB_NAME)

        llm_client = get_openai_client()

        query_processor = GraphRAGQueryProcessor(llm_client)
        retrieval_engine = GraphRAGRetrievalEngine(db)
        generation_service = GraphRAGGenerationService(
            llm_client,
            query_processor=query_processor,
            retrieval_engine=retrieval_engine,
        )

        # Process query with GraphRAG
        graphrag_response = generation_service.process_query_with_generation(
            query, db, use_traditional_rag=use_traditional_rag
        )

        # Get traditional RAG results if requested
        traditional_hits = []
        if use_traditional_rag:
            try:
                qvec = embed_query(query)
                traditional_hits = vector_search(
                    db[COLL_CHUNKS], qvec, k=k, filters=filters
                )
            except Exception as e:
                logger.warning(f"Failed to get traditional RAG results: {e}")

        # Combine contexts if both are available
        if traditional_hits and graphrag_response.context_sources:
            # Merge traditional and GraphRAG contexts
            combined_context = f"""
## Traditional RAG Context:
{chr(10).join([hit.get('text', '') for hit in traditional_hits[:3]])}

## GraphRAG Context:
{graphrag_response.answer}
"""

            # Re-generate answer with combined context
            if use_traditional_rag:
                final_answer = generation_service.generate_answer(
                    query, combined_context, [], []
                )
            else:
                final_answer = graphrag_response.answer
        else:
            final_answer = graphrag_response.answer

        # Log the interaction
        logs = db[COLL_MEMORY_LOGS]
        logs.insert_one(
            {
                "query": query,
                "mode": "graphrag",
                "session_id": session_id,
                "graphrag_entities": len(graphrag_response.entities),
                "graphrag_communities": len(graphrag_response.communities),
                "traditional_hits": len(traditional_hits),
                "confidence": graphrag_response.confidence,
                "processing_time": graphrag_response.processing_time,
                "answer": final_answer,
            }
        )

        logger.info(
            f"GraphRAG answer generated with confidence {graphrag_response.confidence}"
        )

        result = {
            "answer": final_answer,
            "hits": traditional_hits,
            "graphrag_entities": graphrag_response.entities,
            "graphrag_communities": graphrag_response.communities,
            "confidence": graphrag_response.confidence,
            "processing_time": graphrag_response.processing_time,
            "mode": "graphrag",
        }
        duration = time.time() - start_time
        _rag_service_duration.observe(duration, labels=labels)
        return result

    except Exception as e:
        _rag_service_errors.inc(labels=labels)
        duration = time.time() - start_time
        _rag_service_duration.observe(duration, labels=labels)
        logger.error(f"Error in GraphRAG processing: {e}")

        # Fallback to traditional RAG
        logger.info("Falling back to traditional RAG")
        return rag_answer(
            query,
            k=k,
            filters=filters,
            weights=weights,
            streaming=streaming,
            session_id=session_id,
        )


@handle_errors(fallback={"answer": "An error occurred while generating the hybrid GraphRAG answer.", "hits": []}, log_traceback=True, reraise=False)
def rag_hybrid_graphrag_answer(
    query: str,
    k: int = 8,
    filters: Optional[Dict[str, Any]] = None,
    weights: Optional[Dict[str, float]] = None,
    streaming: bool = False,
    session_id: Optional[str] = None,
    graphrag_weight: float = 0.7,
) -> Dict[str, Any]:
    """
    Generate answer using both traditional RAG and GraphRAG with weighted combination.

    Args:
        query: User's query
        k: Number of chunks to retrieve
        filters: Optional filters for retrieval
        weights: Optional weights for reranking
        streaming: Whether to stream the answer
        session_id: Optional session ID for logging
        graphrag_weight: Weight for GraphRAG vs traditional RAG (0-1)

    Returns:
        Dictionary containing answer and metadata
    """
    logger.info(f"Processing hybrid GraphRAG query: {query}")

    start_time = time.time()
    labels = {"service": "rag", "method": "rag_hybrid_graphrag_answer"}
    _rag_service_calls.inc(labels=labels)
    
    try:
        # Get traditional RAG results
        traditional_result = rag_hybrid_answer(
            query,
            k=k,
            filters=filters,
            weights=weights,
            streaming=False,
            session_id=session_id,
        )

        # Get GraphRAG results
        graphrag_result = rag_graphrag_answer(
            query,
            k=k,
            filters=filters,
            weights=weights,
            streaming=False,
            session_id=session_id,
            use_traditional_rag=False,
        )

        # Combine answers based on weight
        if graphrag_weight > 0.5:
            # GraphRAG primary
            primary_answer = graphrag_result["answer"]
            secondary_answer = traditional_result["answer"]
            primary_hits = graphrag_result.get("hits", [])
            secondary_hits = traditional_result["hits"]
        else:
            # Traditional RAG primary
            primary_answer = traditional_result["answer"]
            secondary_answer = graphrag_result["answer"]
            primary_hits = traditional_result["hits"]
            secondary_hits = graphrag_result.get("hits", [])

        # Create combined answer
        combined_answer = f"""
{primary_answer}

## Additional Context:
{secondary_answer}
"""

        # Log the interaction
        client = MongoClient(os.getenv("MONGODB_URI", "mongodb://localhost:27017"))
        db = client[DB_NAME]
        logs = db[COLL_MEMORY_LOGS]

        logs.insert_one(
            {
                "query": query,
                "mode": "hybrid_graphrag",
                "session_id": session_id,
                "graphrag_weight": graphrag_weight,
                "traditional_hits": len(traditional_result["hits"]),
                "graphrag_entities": len(graphrag_result.get("graphrag_entities", [])),
                "graphrag_communities": len(
                    graphrag_result.get("graphrag_communities", [])
                ),
                "combined_answer": combined_answer,
            }
        )

        logger.info(f"Hybrid GraphRAG answer generated with weight {graphrag_weight}")

        result = {
            "answer": combined_answer,
            "hits": primary_hits + secondary_hits,
            "traditional_hits": traditional_result["hits"],
            "graphrag_entities": graphrag_result.get("graphrag_entities", []),
            "graphrag_communities": graphrag_result.get("graphrag_communities", []),
            "graphrag_weight": graphrag_weight,
            "mode": "hybrid_graphrag",
        }
        duration = time.time() - start_time
        _rag_service_duration.observe(duration, labels=labels)
        return result

    except Exception as e:
        _rag_service_errors.inc(labels=labels)
        duration = time.time() - start_time
        _rag_service_duration.observe(duration, labels=labels)
        logger.error(f"Error in hybrid GraphRAG processing: {e}")

        # Fallback to traditional RAG
        logger.info("Falling back to traditional RAG")
        return rag_hybrid_answer(
            query,
            k=k,
            filters=filters,
            weights=weights,
            streaming=streaming,
            session_id=session_id,
        )


@handle_errors(fallback={"graphrag_enabled": False, "error": "Failed to get status"}, log_traceback=True, reraise=False)
def get_graphrag_status() -> Dict[str, Any]:
    """
    Get the current status of GraphRAG components.

    Returns:
        Dictionary containing GraphRAG status information
    """
    start_time = time.time()
    labels = {"service": "rag", "method": "get_graphrag_status"}
    _rag_service_calls.inc(labels=labels)
    
    try:
        client = MongoClient(os.getenv("MONGODB_URI", "mongodb://localhost:27017"))
        db = client[DB_NAME]

        # Check GraphRAG collections
        from src.domain.services.graphrag.indexes import get_graphrag_collections

        collections = get_graphrag_collections(db)

        status = {"graphrag_enabled": True, "collections": {}, "total_counts": {}}

        for name, collection in collections.items():
            try:
                count = collection.count_documents({})
                status["collections"][name] = {"exists": True, "count": count}
                status["total_counts"][name] = count
            except Exception as e:
                status["collections"][name] = {"exists": False, "error": str(e)}

        # Check if GraphRAG pipeline has been run
        chunks_collection = db[COLL_CHUNKS]
        processed_chunks = chunks_collection.count_documents(
            {"graphrag_communities.status": "completed"}
        )

        status["pipeline_status"] = {
            "processed_chunks": processed_chunks,
            "pipeline_completed": processed_chunks > 0,
        }

        logger.info("GraphRAG status retrieved successfully")
        duration = time.time() - start_time
        _rag_service_duration.observe(duration, labels=labels)
        return status

    except Exception as e:
        _rag_service_errors.inc(labels=labels)
        duration = time.time() - start_time
        _rag_service_duration.observe(duration, labels=labels)
        logger.error(f"Error getting GraphRAG status: {e}")
        return {"graphrag_enabled": False, "error": str(e)}
