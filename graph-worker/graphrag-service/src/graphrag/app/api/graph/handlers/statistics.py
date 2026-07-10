"""
Statistics Handler - Business Logic

Pure functions for graph statistics operations.
No HTTP handling - that's in router.py

Achievement 6.2: Graph Statistics Dashboard
"""

import logging
import os
import sys
from typing import Dict, Any

# Add project root to Python path for imports
_project_root = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "..", ".."))
if _project_root not in sys.path:
    sys.path.insert(0, _project_root)

from src.infrastructure.database.mongodb import get_mongo_client
from src.domain.services.graphrag.indexes import get_graphrag_collections
from ..constants import DEFAULT_LIMIT

logger = logging.getLogger(__name__)


def get(db_name: str) -> Dict[str, Any]:
    """
    Get comprehensive graph statistics.

    Args:
        db_name: Database name

    Returns:
        Dictionary with graph statistics
    """
    client = get_mongo_client()
    db = client[db_name]
    collections = get_graphrag_collections(db)
    entities_collection = collections["entities"]
    relations_collection = collections["relations"]

    # Basic counts
    total_entities = entities_collection.count_documents({})
    total_relationships = relations_collection.count_documents({})

    # Entity type distribution
    type_pipeline = [
        {"$group": {"_id": "$type", "count": {"$sum": 1}}},
        {"$sort": {"count": -1}},
    ]
    type_distribution = list(entities_collection.aggregate(type_pipeline))

    # Predicate distribution
    predicate_pipeline = [
        {"$group": {"_id": "$predicate", "count": {"$sum": 1}}},
        {"$sort": {"count": -1}},
    ]
    predicate_distribution = list(relations_collection.aggregate(predicate_pipeline))

    # Calculate entity degrees
    entity_degrees = {}

    # Outgoing relationships
    outgoing_pipeline = [{"$group": {"_id": "$subject_id", "count": {"$sum": 1}}}]
    for result in relations_collection.aggregate(outgoing_pipeline):
        entity_degrees[result["_id"]] = entity_degrees.get(result["_id"], 0) + result["count"]

    # Incoming relationships
    incoming_pipeline = [{"$group": {"_id": "$object_id", "count": {"$sum": 1}}}]
    for result in relations_collection.aggregate(incoming_pipeline):
        entity_degrees[result["_id"]] = entity_degrees.get(result["_id"], 0) + result["count"]

    # Degree statistics
    degrees = list(entity_degrees.values())
    avg_degree = sum(degrees) / len(degrees) if degrees else 0
    max_degree = max(degrees) if degrees else 0
    min_degree = min(degrees) if degrees else 0

    # Degree distribution (histogram)
    degree_buckets = {}
    for degree in degrees:
        bucket = (degree // 5) * 5  # Bucket by 5s
        degree_buckets[bucket] = degree_buckets.get(bucket, 0) + 1

    # Graph density
    potential_edges = total_entities * (total_entities - 1) / 2 if total_entities > 1 else 0
    graph_density = total_relationships / potential_edges if potential_edges > 0 else 0

    # Connected components (simplified - count isolated entities)
    isolated_entities = total_entities - len(entity_degrees)

    # Edge-to-node ratio
    edge_to_node_ratio = total_relationships / total_entities if total_entities > 0 else 0

    return {
        "total_entities": total_entities,
        "total_relationships": total_relationships,
        "graph_density": graph_density,
        "edge_to_node_ratio": edge_to_node_ratio,
        "isolated_entities": isolated_entities,
        "connected_entities": len(entity_degrees),
        "avg_degree": round(avg_degree, 2),
        "max_degree": max_degree,
        "min_degree": min_degree,
        "type_distribution": [{"type": t["_id"], "count": t["count"]} for t in type_distribution],
        "predicate_distribution": [
            {"predicate": p["_id"], "count": p["count"]}
            for p in predicate_distribution[:20]  # Top 20
        ],
        "degree_distribution": [
            {"degree": k, "count": v} for k, v in sorted(degree_buckets.items())
        ],
    }


def get_over_time(db_name: str, limit: int = DEFAULT_LIMIT) -> Dict[str, Any]:
    """
    Get graph statistics over time from metrics collection.

    Args:
        db_name: Database name
        limit: Maximum number of data points

    Returns:
        Dictionary with time series data
    """
    client = get_mongo_client()
    db = client[db_name]
    metrics_collection = db.graphrag_metrics

    # Get latest metrics over time
    cursor = metrics_collection.find({}).sort("timestamp", -1).limit(limit)

    time_series = []
    for doc in cursor:
        time_series.append({
            "timestamp": doc.get("timestamp"),
            "total_entities": doc.get("total_entities"),
            "total_relationships": doc.get("total_relationships"),
            "run_id": doc.get("run_id"),
        })

    return {
        "data_points": len(time_series),
        "time_series": list(reversed(time_series)),  # Oldest first
    }

