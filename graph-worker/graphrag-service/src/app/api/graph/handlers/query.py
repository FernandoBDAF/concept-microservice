"""
Graph Data API - Query Handler

Handles natural language query execution using GraphRAG.
Wraps the GraphRAGGenerationService to provide Q&A capabilities.

Usage:
    from src.app.api.graph.handlers import query
    
    result = query.execute(
        db_name="2025-12",
        query_text="What is machine learning?",
        mode="global",
        options={"top_k": 10}
    )
"""

import logging
import os
import uuid
import time
from typing import Dict, Any, Optional

from pymongo import MongoClient
from openai import OpenAI

from src.domain.services.graphrag.generation import GraphRAGGenerationService
from src.core.models.graphrag import GraphRAGResponse

logger = logging.getLogger(__name__)


# Valid query modes
QUERY_MODES = ["local", "global", "hybrid"]
DEFAULT_MODE = "global"

# Module-level MongoDB client (connection pooling)
_mongo_client: Optional[MongoClient] = None


def _get_mongo_client() -> MongoClient:
    """Get or create MongoDB client with connection pooling."""
    global _mongo_client
    if _mongo_client is None:
        _mongo_client = MongoClient(
            os.getenv("MONGODB_URI", "mongodb://localhost:27017"),
            maxPoolSize=10,
            serverSelectionTimeoutMS=5000,
        )
    return _mongo_client


def _get_db(db_name: str):
    """Get MongoDB database connection."""
    return _get_mongo_client()[db_name]


def _get_openai_client() -> OpenAI:
    """Get OpenAI client instance."""
    api_key = os.getenv("OPENAI_API_KEY")
    if not api_key:
        raise ValueError("OPENAI_API_KEY environment variable not set")
    return OpenAI(api_key=api_key)


def _serialize_entity(entity: Any) -> Optional[Dict[str, Any]]:
    """Serialize entity for API response. Returns None for invalid items."""
    if not isinstance(entity, dict):
        logger.warning(f"Expected dict for entity, got {type(entity).__name__}")
        return None
    return {
        "id": str(entity.get("_id", entity.get("entity_id", ""))),
        "name": entity.get("name", ""),
        "type": entity.get("type", "OTHER"),
        "description": entity.get("description", ""),
        "confidence": float(entity.get("confidence", 0.0) or 0.0),
        "trust_score": entity.get("trust_score"),
        "source_count": int(entity.get("source_count", 0) or 0),
    }


def _serialize_community(community: Any) -> Optional[Dict[str, Any]]:
    """Serialize community for API response. Returns None for invalid items."""
    if not isinstance(community, dict):
        logger.warning(f"Expected dict for community, got {type(community).__name__}")
        return None
    return {
        "id": str(community.get("_id", community.get("community_id", ""))),
        "title": community.get("title", ""),
        "summary": community.get("summary", ""),
        "level": int(community.get("level", 0) or 0),
        "entity_count": int(community.get("entity_count", 0) or 0),
        "coherence_score": community.get("coherence_score"),
    }


def execute(
    db_name: str,
    query_text: str,
    mode: str = DEFAULT_MODE,
    options: Optional[Dict[str, Any]] = None,
) -> Dict[str, Any]:
    """
    Execute a natural language query using GraphRAG.

    Args:
        db_name: Database name
        query_text: User's natural language question
        mode: Query mode - "local", "global", or "hybrid"
              - local: Focus on specific entity neighborhood
              - global: Use community summaries for broad context
              - hybrid: Combine both approaches (default)
        options: Additional options
            - top_k: Maximum number of results (default: 10)
            - include_sources: Include source chunks (default: True)
            - include_communities: Include community context (default: True)
            - model: LLM model to use (default: gpt-4o-mini)
            - temperature: LLM temperature (default: 0.3)

    Returns:
        Query result with answer, entities, communities, and metadata
    """
    start_time = time.time()
    query_id = f"query_{uuid.uuid4().hex[:12]}"
    
    options = options or {}
    
    # Validate mode
    if mode not in QUERY_MODES:
        return {
            "error": f"Invalid mode: {mode}",
            "valid_modes": QUERY_MODES,
            "query_id": query_id,
        }
    
    # Validate query
    if not query_text or not query_text.strip():
        return {
            "error": "Query text is required",
            "query_id": query_id,
        }
    
    try:
        query_preview = query_text[:100] + "..." if len(query_text) > 100 else query_text
        logger.info(f"[{query_id}] Executing query: '{query_preview}' with mode={mode}")
        
        # Get database connection
        db = _get_db(db_name)
        
        # Get OpenAI client
        llm_client = _get_openai_client()
        
        # Extract options
        model_name = options.get("model", "gpt-4o-mini")
        temperature = options.get("temperature", 0.3)
        include_sources = options.get("include_sources", True)
        include_communities = options.get("include_communities", True)
        
        # Initialize generation service
        generation_service = GraphRAGGenerationService(
            llm_client=llm_client,
            model_name=model_name,
            temperature=temperature,
        )
        
        # Execute query
        # Note: The mode parameter is passed for future extensibility.
        # Currently, the underlying service always uses hybrid search internally.
        response: Optional[GraphRAGResponse] = generation_service.process_query_with_generation(
            query_text=query_text,
            db=db,
            use_traditional_rag=(mode == "hybrid"),
        )
        
        processing_time_ms = int((time.time() - start_time) * 1000)
        
        # Handle None response (can happen if @handle_errors decorator catches exception)
        if response is None:
            logger.error(f"[{query_id}] GraphRAG service returned None response")
            return {
                "error": "Query processing failed",
                "message": "The query service returned an empty response. Please try again.",
                "query_id": query_id,
                "processing_time_ms": processing_time_ms,
            }
        
        # Defensive: ensure we have lists for iteration
        entities_list = response.entities if isinstance(response.entities, list) else []
        communities_list = response.communities if isinstance(response.communities, list) else []
        sources_list = response.context_sources if isinstance(response.context_sources, list) else []
        
        # Serialize entities (filter out None results from invalid items)
        serialized_entities = [e for e in (_serialize_entity(item) for item in entities_list) if e is not None]
        
        # Serialize communities (filter out None results from invalid items)
        serialized_communities = []
        if include_communities:
            serialized_communities = [c for c in (_serialize_community(item) for item in communities_list) if c is not None]
        
        # Prepare sources
        sources = sources_list if include_sources else []
        
        confidence_value = float(response.confidence or 0.0)
        logger.info(f"[{query_id}] Query completed in {processing_time_ms}ms, confidence={confidence_value:.2f}")
        
        return {
            "answer": response.answer or "",
            "confidence": confidence_value,
            "entities": serialized_entities,
            "communities": serialized_communities,
            "sources": sources,
            "meta": {
                "query_id": query_id,
                "processing_time_ms": processing_time_ms,
                "mode_used": mode,
                "model": model_name,
                "entity_count": len(serialized_entities),
                "community_count": len(serialized_communities),
            },
        }
        
    except ValueError as e:
        # Configuration errors (e.g., missing API key)
        logger.error(f"[{query_id}] Configuration error: {e}")
        return {
            "error": "Configuration error",
            "message": str(e),
            "query_id": query_id,
        }
    except Exception as e:
        logger.exception(f"[{query_id}] Error executing query")
        return {
            "error": "Query execution failed",
            "message": str(e),
            "query_id": query_id,
            "processing_time_ms": int((time.time() - start_time) * 1000),
        }


def get_query_modes() -> Dict[str, Any]:
    """
    Get available query modes and their descriptions.

    Returns:
        Dictionary with available modes and descriptions
    """
    return {
        "modes": [
            {
                "name": "local",
                "description": "Focus on specific entity neighborhoods. Best for questions about specific entities.",
            },
            {
                "name": "global",
                "description": "Use community summaries for broad context. Best for general/thematic questions.",
            },
            {
                "name": "hybrid",
                "description": "Combine local and global approaches. Best for complex questions.",
            },
        ],
        "default": DEFAULT_MODE,
    }


__all__ = ["execute", "get_query_modes", "QUERY_MODES", "DEFAULT_MODE"]

