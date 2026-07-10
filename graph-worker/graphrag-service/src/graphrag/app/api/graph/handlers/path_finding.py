"""
Path Finding Handler - Business Logic

Pure functions for finding shortest paths between entities.
No HTTP handling - that's in router.py

Uses BFS algorithm to find shortest path(s) between two entities
in the knowledge graph.
"""

import logging
import os
import sys
from collections import defaultdict, deque
from typing import Dict, Any, List, Optional, Set, Tuple

# Add project root to Python path for imports
_project_root = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "..", ".."))
if _project_root not in sys.path:
    sys.path.insert(0, _project_root)

from src.infrastructure.database.mongodb import get_mongo_client
from src.domain.services.graphrag.indexes import get_graphrag_collections
from ..constants import DEFAULT_PATH_MAX_HOPS, DEFAULT_PATH_MAX_PATHS

logger = logging.getLogger(__name__)


def find_shortest_paths(
    db_name: str,
    source_id: str,
    target_id: str,
    max_hops: int = DEFAULT_PATH_MAX_HOPS,
    max_paths: int = DEFAULT_PATH_MAX_PATHS,
) -> Dict[str, Any]:
    """
    Find shortest path(s) between two entities using BFS.

    Args:
        db_name: Database name
        source_id: Starting entity ID
        target_id: Target entity ID
        max_hops: Maximum path length (number of edges)
        max_paths: Maximum number of paths to return

    Returns:
        Dictionary with paths, source/target info, and metadata
    """
    client = get_mongo_client()
    db = client[db_name]
    collections = get_graphrag_collections(db)
    entities_collection = collections["entities"]
    relations_collection = collections["relations"]

    # Verify source entity exists
    source_entity = entities_collection.find_one(
        {"entity_id": source_id},
        {"entity_id": 1, "name": 1, "canonical_name": 1, "type": 1}
    )
    if not source_entity:
        return {
            "error": "Source entity not found",
            "entity_id": source_id,
        }

    # Verify target entity exists
    target_entity = entities_collection.find_one(
        {"entity_id": target_id},
        {"entity_id": 1, "name": 1, "canonical_name": 1, "type": 1}
    )
    if not target_entity:
        return {
            "error": "Target entity not found",
            "entity_id": target_id,
        }

    # Handle same source and target
    if source_id == target_id:
        return {
            "paths": [{
                "nodes": [_format_entity(source_entity)],
                "edges": [],
                "length": 0,
            }],
            "source": _format_entity(source_entity),
            "target": _format_entity(target_entity),
            "total_paths": 1,
            "shortest_length": 0,
            "max_hops": max_hops,
            "max_paths": max_paths,
        }

    # Build adjacency list from relations
    adjacency = _build_adjacency_list(relations_collection)

    # BFS to find shortest paths
    raw_paths = _bfs_find_paths(
        adjacency=adjacency,
        source_id=source_id,
        target_id=target_id,
        max_hops=max_hops,
        max_paths=max_paths,
    )

    # Enrich paths with entity and relation details
    enriched_paths = _enrich_paths(
        raw_paths=raw_paths,
        entities_collection=entities_collection,
        relations_collection=relations_collection,
    )

    # Calculate shortest length
    shortest_length = min((p["length"] for p in enriched_paths), default=None)

    return {
        "paths": enriched_paths,
        "source": _format_entity(source_entity),
        "target": _format_entity(target_entity),
        "total_paths": len(enriched_paths),
        "shortest_length": shortest_length,
        "max_hops": max_hops,
        "max_paths": max_paths,
    }


def _build_adjacency_list(
    relations_collection,
) -> Dict[str, List[Dict[str, Any]]]:
    """
    Build an adjacency list from the relations collection.
    
    Treats the graph as undirected - each relation creates edges in both directions.

    Args:
        relations_collection: MongoDB collection for relations

    Returns:
        Adjacency list mapping entity_id -> list of neighbor info
    """
    adjacency: Dict[str, List[Dict[str, Any]]] = defaultdict(list)

    # Iterate through all relations
    for rel in relations_collection.find(
        {},
        {
            "relationship_id": 1,
            "subject_id": 1,
            "object_id": 1,
            "predicate": 1,
            "confidence": 1,
        }
    ):
        subject_id = rel.get("subject_id")
        object_id = rel.get("object_id")
        relationship_id = rel.get("relationship_id")
        predicate = rel.get("predicate", "related_to")
        confidence = rel.get("confidence", 0.0)

        if not subject_id or not object_id:
            continue

        # Add edge from subject to object
        adjacency[subject_id].append({
            "target": object_id,
            "relationship_id": relationship_id,
            "predicate": predicate,
            "confidence": confidence,
            "direction": "outgoing",
        })

        # Add edge from object to subject (undirected graph)
        adjacency[object_id].append({
            "target": subject_id,
            "relationship_id": relationship_id,
            "predicate": predicate,
            "confidence": confidence,
            "direction": "incoming",
        })

    return adjacency


def _bfs_find_paths(
    adjacency: Dict[str, List[Dict[str, Any]]],
    source_id: str,
    target_id: str,
    max_hops: int,
    max_paths: int,
) -> List[Dict[str, Any]]:
    """
    Use BFS to find shortest paths from source to target.

    Args:
        adjacency: Adjacency list
        source_id: Starting entity ID
        target_id: Target entity ID
        max_hops: Maximum path length
        max_paths: Maximum number of paths to return

    Returns:
        List of raw paths (node_ids and edge_info lists)
    """
    if source_id not in adjacency:
        # Source has no connections
        return []

    # BFS queue: (current_node, path_of_nodes, path_of_edges)
    queue: deque = deque()
    queue.append((source_id, [source_id], []))

    # Track visited nodes at each distance to allow multiple shortest paths
    # visited[node] = minimum distance at which we've seen this node
    visited: Dict[str, int] = {source_id: 0}

    paths: List[Dict[str, Any]] = []
    shortest_found: Optional[int] = None

    while queue:
        current, path_nodes, path_edges = queue.popleft()
        current_distance = len(path_nodes) - 1

        # If we've found paths and current path is longer, stop exploring
        if shortest_found is not None and current_distance > shortest_found:
            continue

        # If we've exceeded max_hops, skip
        if current_distance >= max_hops:
            continue

        # Explore neighbors
        for neighbor_info in adjacency.get(current, []):
            neighbor_id = neighbor_info["target"]
            next_distance = current_distance + 1

            # Skip if we've already visited this node at a shorter distance
            # Allow same distance for multiple shortest paths
            if neighbor_id in visited and visited[neighbor_id] < next_distance:
                continue

            # Create new path
            new_path_nodes = path_nodes + [neighbor_id]
            new_path_edges = path_edges + [{
                "source": current,
                "target": neighbor_id,
                "relationship_id": neighbor_info["relationship_id"],
                "predicate": neighbor_info["predicate"],
                "confidence": neighbor_info["confidence"],
                "direction": neighbor_info["direction"],
            }]

            # Check if we've reached the target
            if neighbor_id == target_id:
                paths.append({
                    "node_ids": new_path_nodes,
                    "edge_info": new_path_edges,
                    "length": next_distance,
                })

                # Record shortest path length found
                if shortest_found is None:
                    shortest_found = next_distance

                # Stop if we have enough paths
                if len(paths) >= max_paths:
                    return paths

                continue

            # Mark as visited and add to queue
            visited[neighbor_id] = next_distance
            queue.append((neighbor_id, new_path_nodes, new_path_edges))

    return paths


def _enrich_paths(
    raw_paths: List[Dict[str, Any]],
    entities_collection,
    relations_collection,
) -> List[Dict[str, Any]]:
    """
    Enrich raw paths with entity and relation details.

    Args:
        raw_paths: List of paths with node_ids and edge_info
        entities_collection: MongoDB collection for entities
        relations_collection: MongoDB collection for relations

    Returns:
        List of enriched paths with full entity and edge details
    """
    if not raw_paths:
        return []

    # Collect all unique entity IDs across all paths
    all_entity_ids: Set[str] = set()
    for path in raw_paths:
        all_entity_ids.update(path["node_ids"])

    # Fetch all entity details in one query
    entity_map: Dict[str, Dict[str, Any]] = {}
    if all_entity_ids:
        for doc in entities_collection.find(
            {"entity_id": {"$in": list(all_entity_ids)}},
            {
                "entity_id": 1,
                "name": 1,
                "canonical_name": 1,
                "type": 1,
                "description": 1,
                "confidence": 1,
            }
        ):
            entity_map[doc["entity_id"]] = doc

    # Build enriched paths
    enriched_paths: List[Dict[str, Any]] = []

    for raw_path in raw_paths:
        # Build nodes list with full details
        nodes = []
        for node_id in raw_path["node_ids"]:
            entity_doc = entity_map.get(node_id, {})
            nodes.append({
                "entity_id": node_id,
                "name": entity_doc.get("name") or entity_doc.get("canonical_name") or node_id,
                "type": entity_doc.get("type", "OTHER"),
                "description": entity_doc.get("description", ""),
                "confidence": entity_doc.get("confidence", 0.0),
            })

        # Build edges list with full details
        edges = []
        for edge_info in raw_path["edge_info"]:
            edges.append({
                "source": edge_info["source"],
                "target": edge_info["target"],
                "predicate": edge_info["predicate"],
                "confidence": edge_info["confidence"],
                "direction": edge_info["direction"],
                "relationship_id": edge_info["relationship_id"],
            })

        enriched_paths.append({
            "nodes": nodes,
            "edges": edges,
            "length": raw_path["length"],
        })

    return enriched_paths


def _format_entity(entity_doc: Dict[str, Any]) -> Dict[str, Any]:
    """Format entity document for response."""
    return {
        "entity_id": entity_doc.get("entity_id"),
        "name": entity_doc.get("name") or entity_doc.get("canonical_name") or entity_doc.get("entity_id"),
        "type": entity_doc.get("type", "OTHER"),
    }

