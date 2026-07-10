"""
Ego Network Handler - Business Logic

Pure functions for ego network operations.
No HTTP handling - that's in router.py

Achievement 7.1: Ego Network Visualization
"""

import logging
import os
import sys
from typing import Dict, Any, Set, List

# Add project root to Python path for imports
_project_root = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "..", ".."))
if _project_root not in sys.path:
    sys.path.insert(0, _project_root)

from src.infrastructure.database.mongodb import get_mongo_client
from src.domain.services.graphrag.indexes import get_graphrag_collections
from ..constants import DEFAULT_MAX_HOPS, DEFAULT_MAX_NODES

logger = logging.getLogger(__name__)


def get(
    db_name: str,
    entity_id: str,
    max_hops: int = DEFAULT_MAX_HOPS,
    max_nodes: int = DEFAULT_MAX_NODES,
) -> Dict[str, Any]:
    """
    Get N-hop ego network around an entity.

    Args:
        db_name: Database name
        entity_id: Central entity ID
        max_hops: Maximum number of hops (1-hop = direct neighbors, etc.)
        max_nodes: Maximum number of nodes to return

    Returns:
        Dictionary with nodes and links for the ego network
    """
    client = get_mongo_client()
    db = client[db_name]
    collections = get_graphrag_collections(db)
    entities_collection = collections["entities"]
    relations_collection = collections["relations"]

    # Get central entity
    center_entity = entities_collection.find_one({"entity_id": entity_id})
    if not center_entity:
        return {
            "error": "Entity not found",
            "entity_id": entity_id,
        }

    # BFS to collect nodes at each hop level
    visited: Set[str] = {entity_id}
    nodes_by_hop: Dict[int, Set[str]] = {0: {entity_id}}
    all_relationships: List[Dict[str, Any]] = []

    # Get relationships for each hop level
    for hop in range(max_hops):
        current_level_nodes = nodes_by_hop.get(hop, set())
        if not current_level_nodes:
            break

        # Find all relationships where subject or object is in current level
        relationships = list(
            relations_collection.find({
                "$or": [
                    {"subject_id": {"$in": list(current_level_nodes)}},
                    {"object_id": {"$in": list(current_level_nodes)}},
                ]
            })
        )

        next_level_nodes: Set[str] = set()

        for rel in relationships:
            # Add relationship
            all_relationships.append({
                "subject_id": rel.get("subject_id"),
                "object_id": rel.get("object_id"),
                "predicate": rel.get("predicate"),
                "confidence": rel.get("confidence", 0.0),
                "hop": hop + 1,
            })

            # Add connected nodes to next level
            subj_id = rel.get("subject_id")
            obj_id = rel.get("object_id")

            if subj_id not in visited:
                next_level_nodes.add(subj_id)
                visited.add(subj_id)

            if obj_id not in visited:
                next_level_nodes.add(obj_id)
                visited.add(obj_id)

        # Limit nodes if we exceed max_nodes
        if len(visited) > max_nodes:
            visited_list = list(visited)
            visited = set(visited_list[:max_nodes])
            next_level_nodes = next_level_nodes.intersection(visited)
            break

        if next_level_nodes:
            nodes_by_hop[hop + 1] = next_level_nodes

    # Get all entity details
    entity_ids = list(visited)
    entity_docs = list(
        entities_collection.find(
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
        )
    )

    # Build nodes list with hop information
    nodes = []
    for doc in entity_docs:
        entity_id_doc = doc.get("entity_id")
        hop_level = 0
        for hop, node_set in nodes_by_hop.items():
            if entity_id_doc in node_set:
                hop_level = hop
                break

        nodes.append({
            "entity_id": entity_id_doc,
            "name": doc.get("name") or doc.get("canonical_name") or entity_id_doc,
            "canonical_name": doc.get("canonical_name"),
            "type": doc.get("type", "OTHER"),
            "description": doc.get("description", ""),
            "confidence": doc.get("confidence", 0.0),
            "source_count": doc.get("source_count", 0),
            "hop_level": hop_level,
            "is_center": entity_id_doc == entity_id,
        })

    # Filter relationships to only include nodes we have
    valid_node_ids = set(entity_ids)
    links = []
    for rel in all_relationships:
        if rel["subject_id"] in valid_node_ids and rel["object_id"] in valid_node_ids:
            links.append({
                "source": rel["subject_id"],
                "target": rel["object_id"],
                "predicate": rel["predicate"],
                "confidence": rel.get("confidence", 0.0),
                "hop": rel.get("hop", 1),
            })

    return {
        "center_entity": {
            "entity_id": entity_id,
            "name": center_entity.get("name") or center_entity.get("canonical_name") or entity_id,
            "type": center_entity.get("type", "OTHER"),
        },
        "nodes": nodes,
        "links": links,
        "max_hops": max_hops,
        "total_nodes": len(nodes),
        "total_links": len(links),
    }

