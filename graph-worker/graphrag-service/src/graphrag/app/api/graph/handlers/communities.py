"""
Community Handler - Business Logic

Pure functions for community operations.
No HTTP handling - that's in router.py

Achievement 4.1: Community Explorer
"""

import logging
import os
import sys
from typing import Dict, Any, Optional

# Add project root to Python path for imports
_project_root = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "..", ".."))
if _project_root not in sys.path:
    sys.path.insert(0, _project_root)

from src.infrastructure.database.mongodb import get_mongo_client
from src.domain.services.graphrag.indexes import get_graphrag_collections
from ..constants import DEFAULT_LIMIT, DEFAULT_OFFSET

logger = logging.getLogger(__name__)


def search(
    db_name: str,
    level: Optional[int] = None,
    min_size: Optional[int] = None,
    max_size: Optional[int] = None,
    min_coherence: Optional[float] = None,
    limit: int = DEFAULT_LIMIT,
    offset: int = DEFAULT_OFFSET,
    sort_by: str = "entity_count",
) -> Dict[str, Any]:
    """
    Search communities with filters.

    Args:
        db_name: Database name
        level: Filter by community level
        min_size: Minimum entity count
        max_size: Maximum entity count
        min_coherence: Minimum coherence score
        limit: Maximum number of results
        offset: Pagination offset
        sort_by: Sort field ("entity_count", "coherence_score", "level")

    Returns:
        Dictionary with results and pagination info
    """
    client = get_mongo_client()
    db = client[db_name]
    collections = get_graphrag_collections(db)
    communities_collection = collections["communities"]

    # Build query filter
    filter_query = {}

    if level is not None:
        filter_query["level"] = level

    if min_size is not None or max_size is not None:
        size_filter = {}
        if min_size is not None:
            size_filter["$gte"] = min_size
        if max_size is not None:
            size_filter["$lte"] = max_size
        filter_query["entity_count"] = size_filter

    if min_coherence is not None:
        filter_query["coherence_score"] = {"$gte": min_coherence}

    # Determine sort order
    sort_order = -1 if sort_by in ["entity_count", "coherence_score"] else 1

    # Get total count
    total = communities_collection.count_documents(filter_query)

    # Get paginated results
    cursor = (
        communities_collection.find(filter_query)
        .skip(offset)
        .limit(limit)
        .sort(sort_by, sort_order)
    )

    communities = []
    for doc in cursor:
        community = {
            "community_id": doc.get("community_id"),
            "level": doc.get("level", 0),
            "title": doc.get("title", ""),
            "summary": doc.get("summary", ""),
            "entities": doc.get("entities", []),
            "entity_count": doc.get("entity_count", len(doc.get("entities", []))),
            "relationship_count": doc.get("relationship_count", 0),
            "coherence_score": doc.get("coherence_score", 0.0),
            "source_chunks": doc.get("source_chunks", []),
            "created_at": doc.get("created_at"),
            "updated_at": doc.get("updated_at"),
            "run_id": doc.get("run_id"),
            "params_hash": doc.get("params_hash"),
        }
        communities.append(community)

    return {
        "communities": communities,
        "total": total,
        "limit": limit,
        "offset": offset,
        "has_more": (offset + limit) < total,
    }


def get_details(
    db_name: str,
    community_id: str,
) -> Optional[Dict[str, Any]]:
    """
    Get detailed information about a specific community.

    Args:
        db_name: Database name
        community_id: Community ID

    Returns:
        Community details including entities and relationships, or None if not found
    """
    client = get_mongo_client()
    db = client[db_name]
    collections = get_graphrag_collections(db)
    communities_collection = collections["communities"]
    entities_collection = collections["entities"]
    relations_collection = collections["relations"]

    # Get community
    community_doc = communities_collection.find_one({"community_id": community_id})
    if not community_doc:
        return None

    entity_ids = community_doc.get("entities", [])

    # Get entity details
    entity_details = []
    if entity_ids:
        for entity_doc in entities_collection.find(
            {"entity_id": {"$in": entity_ids}},
            {
                "entity_id": 1,
                "name": 1,
                "canonical_name": 1,
                "type": 1,
                "description": 1,
                "confidence": 1,
                "source_count": 1,
            },
        ):
            entity_details.append({
                "entity_id": entity_doc.get("entity_id"),
                "name": entity_doc.get("name"),
                "canonical_name": entity_doc.get("canonical_name"),
                "type": entity_doc.get("type"),
                "description": entity_doc.get("description", ""),
                "confidence": entity_doc.get("confidence", 0.0),
                "source_count": entity_doc.get("source_count", 0),
            })

    # Get relationships within community (both subject and object are in community)
    relationships = []
    if entity_ids:
        for rel in relations_collection.find({
            "subject_id": {"$in": entity_ids},
            "object_id": {"$in": entity_ids},
        }):
            relationships.append({
                "relationship_id": rel.get("relationship_id"),
                "subject_id": rel.get("subject_id"),
                "object_id": rel.get("object_id"),
                "predicate": rel.get("predicate"),
                "description": rel.get("description", ""),
                "confidence": rel.get("confidence", 0.0),
                "source_count": rel.get("source_count", 0),
            })

    # Add entity names to relationships
    entity_map = {e["entity_id"]: e for e in entity_details}
    for rel in relationships:
        rel["subject_name"] = entity_map.get(rel["subject_id"], {})
        rel["object_name"] = entity_map.get(rel["object_id"], {})

    return {
        "community_id": community_doc.get("community_id"),
        "level": community_doc.get("level", 0),
        "title": community_doc.get("title", ""),
        "summary": community_doc.get("summary", ""),
        "entity_count": community_doc.get("entity_count", len(entity_ids)),
        "relationship_count": len(relationships),
        "coherence_score": community_doc.get("coherence_score", 0.0),
        "source_chunks": community_doc.get("source_chunks", []),
        "created_at": community_doc.get("created_at"),
        "updated_at": community_doc.get("updated_at"),
        "run_id": community_doc.get("run_id"),
        "params_hash": community_doc.get("params_hash"),
        "entities": entity_details,
        "relationships": relationships,
    }


def get_levels(db_name: str) -> Dict[str, Any]:
    """
    Get statistics about community levels.

    Args:
        db_name: Database name

    Returns:
        Dictionary with level statistics
    """
    client = get_mongo_client()
    db = client[db_name]
    collections = get_graphrag_collections(db)
    communities_collection = collections["communities"]

    # Aggregate by level
    pipeline = [
        {
            "$group": {
                "_id": "$level",
                "count": {"$sum": 1},
                "avg_size": {"$avg": "$entity_count"},
                "avg_coherence": {"$avg": "$coherence_score"},
                "total_entities": {"$sum": "$entity_count"},
            }
        },
        {"$sort": {"_id": 1}},
    ]

    level_stats = list(communities_collection.aggregate(pipeline))

    return {
        "levels": [
            {
                "level": stat["_id"],
                "count": stat["count"],
                "avg_size": round(stat["avg_size"], 2) if stat["avg_size"] else 0,
                "avg_coherence": round(stat["avg_coherence"], 4) if stat["avg_coherence"] else 0,
                "total_entities": stat["total_entities"],
            }
            for stat in level_stats
        ],
    }

