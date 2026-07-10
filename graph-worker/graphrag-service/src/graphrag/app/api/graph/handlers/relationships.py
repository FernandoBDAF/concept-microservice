"""
Relationship Handler - Business Logic

Pure functions for relationship operations.
No HTTP handling - that's in router.py

Achievement 3.2: Relationship Viewer
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


def get_types(db_name: str) -> Dict[str, Any]:
    """
    Get unique predicate types with counts.

    Args:
        db_name: Database name

    Returns:
        Dictionary with predicate types and their counts
    """
    client = get_mongo_client()
    db = client[db_name]
    collections = get_graphrag_collections(db)
    relations_collection = collections["relations"]

    # Aggregate by predicate
    pipeline = [
        {"$group": {"_id": "$predicate", "count": {"$sum": 1}}},
        {"$sort": {"count": -1}},
        {"$project": {"predicate": "$_id", "count": 1, "_id": 0}},
    ]

    types = list(relations_collection.aggregate(pipeline))

    # Calculate total relationships
    total_relationships = sum(t["count"] for t in types)

    return {
        "types": types,
        "total_types": len(types),
        "total_relationships": total_relationships,
    }


def search(
    db_name: str,
    predicate: Optional[str] = None,
    entity_type: Optional[str] = None,
    min_confidence: Optional[float] = None,
    subject_id: Optional[str] = None,
    object_id: Optional[str] = None,
    limit: int = DEFAULT_LIMIT,
    offset: int = DEFAULT_OFFSET,
) -> Dict[str, Any]:
    """
    Search relationships with filters.

    Args:
        db_name: Database name
        predicate: Filter by predicate type
        entity_type: Filter by entity type (subject or object)
        min_confidence: Minimum confidence threshold
        subject_id: Filter by subject entity ID
        object_id: Filter by object entity ID
        limit: Maximum number of results
        offset: Pagination offset

    Returns:
        Dictionary with results and pagination info
    """
    client = get_mongo_client()
    db = client[db_name]
    collections = get_graphrag_collections(db)
    relations_collection = collections["relations"]
    entities_collection = collections["entities"]

    # Build query filter
    filter_query = {}

    if predicate:
        filter_query["predicate"] = {"$regex": predicate, "$options": "i"}

    if min_confidence is not None:
        filter_query["confidence"] = {"$gte": min_confidence}

    if subject_id:
        filter_query["subject_id"] = subject_id

    if object_id:
        filter_query["object_id"] = object_id

    # Get total count
    total = relations_collection.count_documents(filter_query)

    # Get paginated results
    cursor = (
        relations_collection.find(filter_query)
        .skip(offset)
        .limit(limit)
        .sort("source_count", -1)
    )

    relationships = []
    entity_ids = set()

    for doc in cursor:
        entity_ids.add(doc.get("subject_id"))
        entity_ids.add(doc.get("object_id"))

        relationships.append({
            "relationship_id": doc.get("relationship_id"),
            "subject_id": doc.get("subject_id"),
            "object_id": doc.get("object_id"),
            "predicate": doc.get("predicate"),
            "description": doc.get("description", ""),
            "confidence": doc.get("confidence", 0.0),
            "source_count": doc.get("source_count", 0),
            "source_chunks": doc.get("source_chunks", []),
            "created_at": doc.get("created_at"),
            "updated_at": doc.get("updated_at"),
        })

    # Get entity names
    entity_names = {}
    if entity_ids:
        for entity_doc in entities_collection.find(
            {"entity_id": {"$in": list(entity_ids)}},
            {"entity_id": 1, "name": 1, "canonical_name": 1, "type": 1}
        ):
            entity_names[entity_doc["entity_id"]] = {
                "name": entity_doc.get("name"),
                "canonical_name": entity_doc.get("canonical_name"),
                "type": entity_doc.get("type"),
            }

    # Add entity names to relationships
    for rel in relationships:
        rel["subject_name"] = entity_names.get(rel["subject_id"], {})
        rel["object_name"] = entity_names.get(rel["object_id"], {})

    # Filter by entity type if specified (post-processing)
    if entity_type:
        relationships = [
            rel for rel in relationships
            if rel["subject_name"].get("type") == entity_type 
            or rel["object_name"].get("type") == entity_type
        ]

    return {
        "relationships": relationships,
        "total": total,
        "limit": limit,
        "offset": offset,
        "has_more": (offset + limit) < total,
    }

