"""
Entity Handler - Business Logic

Pure functions for entity operations.
No HTTP handling - that's in router.py

Achievement 3.1: Entity Browser
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
    query: Optional[str] = None,
    entity_type: Optional[str] = None,
    min_confidence: Optional[float] = None,
    min_source_count: Optional[int] = None,
    limit: int = DEFAULT_LIMIT,
    offset: int = DEFAULT_OFFSET,
) -> Dict[str, Any]:
    """
    Search entities with filters.

    Args:
        db_name: Database name
        query: Search query (searches name, canonical_name, aliases)
        entity_type: Filter by entity type
        min_confidence: Minimum confidence threshold
        min_source_count: Minimum source_count threshold
        limit: Maximum number of results
        offset: Pagination offset

    Returns:
        Dictionary with results and pagination info
    """
    client = get_mongo_client()
    db = client[db_name]
    collections = get_graphrag_collections(db)
    entities_collection = collections["entities"]

    # Build query filter
    filter_query = {}

    if query:
        # Search in name, canonical_name, and aliases
        filter_query["$or"] = [
            {"name": {"$regex": query, "$options": "i"}},
            {"canonical_name": {"$regex": query, "$options": "i"}},
            {"aliases": {"$regex": query, "$options": "i"}},
        ]

    if entity_type:
        filter_query["type"] = entity_type

    if min_confidence is not None:
        filter_query["confidence"] = {"$gte": min_confidence}

    if min_source_count is not None:
        filter_query["source_count"] = {"$gte": min_source_count}

    # Get total count
    total = entities_collection.count_documents(filter_query)

    # Get paginated results
    cursor = (
        entities_collection.find(filter_query)
        .skip(offset)
        .limit(limit)
        .sort("source_count", -1)
    )

    entities = []
    for doc in cursor:
        entity = {
            "entity_id": doc.get("entity_id"),
            "name": doc.get("name"),
            "canonical_name": doc.get("canonical_name"),
            "type": doc.get("type"),
            "description": doc.get("description", ""),
            "confidence": doc.get("confidence", 0.0),
            "source_count": doc.get("source_count", 0),
            "aliases": doc.get("aliases", []),
            "created_at": doc.get("created_at"),
            "updated_at": doc.get("updated_at"),
        }
        entities.append(entity)

    return {
        "entities": entities,
        "total": total,
        "limit": limit,
        "offset": offset,
        "has_more": (offset + limit) < total,
    }


def get_types(db_name: str) -> Dict[str, Any]:
    """
    Get unique entity types with counts.

    Args:
        db_name: Database name

    Returns:
        Dictionary with entity types and their counts
    """
    client = get_mongo_client()
    db = client[db_name]
    collections = get_graphrag_collections(db)
    entities_collection = collections["entities"]

    # Aggregate by type
    pipeline = [
        {"$group": {"_id": "$type", "count": {"$sum": 1}}},
        {"$sort": {"count": -1}},
        {"$project": {"type": "$_id", "count": 1, "_id": 0}},
    ]

    types = list(entities_collection.aggregate(pipeline))

    # Calculate total entities
    total_entities = sum(t["count"] for t in types)

    return {
        "types": types,
        "total_types": len(types),
        "total_entities": total_entities,
    }


def get_details(
    db_name: str,
    entity_id: str,
) -> Optional[Dict[str, Any]]:
    """
    Get detailed information about a specific entity.

    Args:
        db_name: Database name
        entity_id: Entity ID

    Returns:
        Entity details including relationships, or None if not found
    """
    client = get_mongo_client()
    db = client[db_name]
    collections = get_graphrag_collections(db)
    entities_collection = collections["entities"]
    relations_collection = collections["relations"]

    # Get entity
    entity_doc = entities_collection.find_one({"entity_id": entity_id})
    if not entity_doc:
        return None

    # Get relationships where this entity is subject or object
    relationships = []

    # As subject
    for rel in relations_collection.find({"subject_id": entity_id}):
        relationships.append({
            "relationship_id": rel.get("relationship_id"),
            "direction": "outgoing",
            "predicate": rel.get("predicate"),
            "object_id": rel.get("object_id"),
            "description": rel.get("description", ""),
            "confidence": rel.get("confidence", 0.0),
            "source_count": rel.get("source_count", 0),
        })

    # As object
    for rel in relations_collection.find({"object_id": entity_id}):
        relationships.append({
            "relationship_id": rel.get("relationship_id"),
            "direction": "incoming",
            "predicate": rel.get("predicate"),
            "subject_id": rel.get("subject_id"),
            "description": rel.get("description", ""),
            "confidence": rel.get("confidence", 0.0),
            "source_count": rel.get("source_count", 0),
        })

    # Get entity names for related entities
    related_entity_ids = set()
    for rel in relationships:
        if "subject_id" in rel:
            related_entity_ids.add(rel["subject_id"])
        if "object_id" in rel:
            related_entity_ids.add(rel["object_id"])

    related_entity_names = {}
    if related_entity_ids:
        for entity_doc_rel in entities_collection.find(
            {"entity_id": {"$in": list(related_entity_ids)}},
            {"entity_id": 1, "name": 1, "canonical_name": 1, "type": 1},
        ):
            related_entity_names[entity_doc_rel["entity_id"]] = {
                "name": entity_doc_rel.get("name"),
                "canonical_name": entity_doc_rel.get("canonical_name"),
                "type": entity_doc_rel.get("type"),
            }

    # Add entity names to relationships
    for rel in relationships:
        if "subject_id" in rel:
            rel["subject_name"] = related_entity_names.get(rel["subject_id"], {})
        if "object_id" in rel:
            rel["object_name"] = related_entity_names.get(rel["object_id"], {})

    return {
        "entity_id": entity_doc.get("entity_id"),
        "name": entity_doc.get("name"),
        "canonical_name": entity_doc.get("canonical_name"),
        "type": entity_doc.get("type"),
        "description": entity_doc.get("description", ""),
        "confidence": entity_doc.get("confidence", 0.0),
        "source_count": entity_doc.get("source_count", 0),
        "aliases": entity_doc.get("aliases", []),
        "source_chunks": entity_doc.get("source_chunks", []),
        "created_at": entity_doc.get("created_at"),
        "updated_at": entity_doc.get("updated_at"),
        "relationships": relationships,
        "relationship_count": len(relationships),
    }

