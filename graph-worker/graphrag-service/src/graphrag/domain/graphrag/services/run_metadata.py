"""
Run Metadata Service

This module provides utilities for tracking GraphRAG stage runs with full provenance,
enabling reproducible runs and preventing duplicate work.
"""

import logging
import hashlib
import json
import time
from typing import Dict, List, Any, Optional
from pymongo.database import Database
from pymongo.collection import Collection
from src.core.models.graphrag import ResolvedEntity, ResolvedRelationship

logger = logging.getLogger(__name__)


def compute_params_hash(params_dict: Dict[str, Any]) -> str:
    """
    Compute deterministic hash from parameters dictionary.

    Args:
        params_dict: Dictionary of parameters (will be sorted by keys)

    Returns:
        12-character hexadecimal hash string
    """
    # Sort keys to ensure deterministic order
    sorted_params = dict(sorted(params_dict.items()))

    # JSON encode with sorted keys
    params_json = json.dumps(sorted_params, sort_keys=True, default=str)

    # Compute SHA1 hash
    hash_obj = hashlib.sha1(params_json.encode())
    hash_hex = hash_obj.hexdigest()[:12]  # Use first 12 characters

    return hash_hex


def compute_graph_signature(
    entities: List[ResolvedEntity], relationships: List[ResolvedRelationship]
) -> str:
    """
    Compute deterministic signature from graph structure.

    Creates signature from sorted list of (subject_id, object_id, predicate, confidence).
    This detects when entities or relationships are added/removed/changed.

    Args:
        entities: List of entities (used for count validation)
        relationships: List of relationships (used for signature)

    Returns:
        12-character hexadecimal hash string
    """
    # Create sorted list of relationship tuples
    # Round confidence to 2 decimal places to avoid floating-point precision issues
    relationship_tuples = [
        (
            rel.subject_id,
            rel.object_id,
            rel.predicate,
            round(rel.confidence, 2),
        )
        for rel in relationships
    ]

    # Sort tuples (ensures deterministic order)
    sorted_tuples = sorted(relationship_tuples)

    # Create signature string
    signature = ",".join(f"{s}|{o}|{p}|{c:.2f}" for s, o, p, c in sorted_tuples)

    # Also include entity count for validation
    entity_count = len(entities)
    signature = f"{entity_count}:{signature}"

    # Compute SHA1 hash
    hash_obj = hashlib.sha1(signature.encode())
    hash_hex = hash_obj.hexdigest()[:12]  # Use first 12 characters

    return hash_hex


def create_run_document(
    db: Database,
    stage: str,
    params_hash: str,
    graph_signature: str,
    params: Dict[str, Any],
    ontology_version: str = "unknown",
    trace_id: Optional[str] = None,
) -> str:
    """
    Create a new run document in graphrag_runs collection.

    Args:
        db: MongoDB database instance
        stage: Stage name (e.g., "community_detection")
        params_hash: Computed params hash
        graph_signature: Computed graph signature
        params: Full parameters dictionary
        ontology_version: Ontology version string
        trace_id: Optional trace ID for linking transformations across pipeline run

    Returns:
        Run ID (MongoDB _id as string)
    """
    runs_collection = db.graphrag_runs

    # Create run document
    run_doc = {
        "stage": stage,
        "params_hash": params_hash,
        "graph_signature": graph_signature,
        "params": params,
        "ontology_version": ontology_version,
        "status": "started",
        "started_at": time.time(),
        "completed_at": None,
        "metrics": {},
    }
    
    # Add trace_id if provided (Achievement 0.1: Trace ID System Integration)
    if trace_id:
        run_doc["trace_id"] = trace_id

    # Insert document
    result = runs_collection.insert_one(run_doc)
    run_id = str(result.inserted_id)

    logger.info(
        f"Created run document: run_id={run_id}, stage={stage}, "
        f"params_hash={params_hash[:8]}..."
    )

    return run_id


def find_existing_run(
    db: Database,
    stage: str,
    params_hash: str,
    graph_signature: str,
) -> Optional[Dict[str, Any]]:
    """
    Find existing run with matching params_hash and graph_signature.

    Args:
        db: MongoDB database instance
        stage: Stage name
        params_hash: Computed params hash
        graph_signature: Computed graph signature

    Returns:
        Run document if found, None otherwise
    """
    runs_collection = db.graphrag_runs

    # Query for matching run
    query = {
        "stage": stage,
        "params_hash": params_hash,
        "graph_signature": graph_signature,
        "status": "completed",  # Only reuse completed runs
    }

    existing_run = runs_collection.find_one(query)

    if existing_run:
        logger.info(
            f"Found existing run: run_id={existing_run['_id']}, "
            f"params_hash={params_hash[:8]}..., graph_signature={graph_signature[:8]}..."
        )

    return existing_run


def update_run_document(
    db: Database,
    run_id: str,
    status: str = "completed",
    metrics: Optional[Dict[str, Any]] = None,
) -> None:
    """
    Update run document with completion status and metrics.

    Args:
        db: MongoDB database instance
        run_id: Run ID (MongoDB _id as string)
        status: Run status ("completed", "failed", etc.)
        metrics: Optional metrics dictionary
    """
    runs_collection = db.graphrag_runs

    update_data = {
        "$set": {
            "status": status,
            "completed_at": time.time(),
        }
    }

    if metrics:
        update_data["$set"]["metrics"] = metrics

    # Update document
    runs_collection.update_one(
        {"_id": run_id},
        update_data,
    )

    logger.info(f"Updated run document: run_id={run_id}, status={status}")


def get_run_document(db: Database, run_id: str) -> Optional[Dict[str, Any]]:
    """
    Get run document by ID.

    Args:
        db: MongoDB database instance
        run_id: Run ID (MongoDB _id as string)

    Returns:
        Run document if found, None otherwise
    """
    runs_collection = db.graphrag_runs

    # Convert string ID to ObjectId if needed
    from bson import ObjectId

    try:
        run_doc = runs_collection.find_one({"_id": ObjectId(run_id)})
    except Exception:
        # If conversion fails, try as string
        run_doc = runs_collection.find_one({"_id": run_id})

    return run_doc
