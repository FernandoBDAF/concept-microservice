"""
Management API Handlers

Database operations, maintenance, and inspection utilities.
Supports the StagesUI Management module.
"""

import os
import uuid
import logging
import threading
from datetime import datetime
from typing import Dict, Any, Optional, List, Tuple

from src.core.config.paths import COLL_CHUNKS

# Import shared MongoDB client
from .db import get_mongo_client, SYSTEM_DATABASES

logger = logging.getLogger(__name__)

# =============================================================================
# Operation Tracking (In-Memory, Thread-Safe)
# =============================================================================

_active_operations: Dict[str, Dict[str, Any]] = {}
_operations_lock = threading.Lock()


def get_operation_status(operation_id: str) -> Tuple[Dict[str, Any], int]:
    """
    Get status of a long-running operation.
    
    Args:
        operation_id: The operation ID returned from a long-running operation
        
    Returns:
        Tuple of (operation_status_dict, http_status_code)
    """
    with _operations_lock:
        if operation_id not in _active_operations:
            return {"error": "Operation not found", "operation_id": operation_id}, 404
        
        return _active_operations[operation_id].copy(), 200


def _create_operation(
    operation_type: str,
    params: Dict[str, Any],
    total: int = 0,
) -> str:
    """Create a new operation tracking entry."""
    operation_id = f"op_{uuid.uuid4().hex[:8]}"
    with _operations_lock:
        _active_operations[operation_id] = {
            "operation_id": operation_id,
            "type": operation_type,
            "status": "pending",
            "progress": {"processed": 0, "total": total, "percent": 0},
            "started_at": datetime.utcnow().isoformat() + "Z",
            "params": params,
        }
    return operation_id


def _update_operation(
    operation_id: str,
    status: Optional[str] = None,
    processed: Optional[int] = None,
    error: Optional[str] = None,
) -> None:
    """Update operation status."""
    with _operations_lock:
        if operation_id not in _active_operations:
            return
        
        op = _active_operations[operation_id]
        
        if status:
            op["status"] = status
            if status == "completed":
                op["completed_at"] = datetime.utcnow().isoformat() + "Z"
        
        if processed is not None:
            op["progress"]["processed"] = processed
            total = op["progress"]["total"]
            if total > 0:
                op["progress"]["percent"] = min(100, int((processed / total) * 100))
        
        if error:
            op["error"] = error


# =============================================================================
# Database Inspection
# =============================================================================

# GraphRAG-related collections we care about
GRAPHRAG_COLLECTIONS = [
    "raw_videos",
    "cleaned_transcripts",
    "enriched_transcripts",
    "video_chunks",
    "entities",
    "relations",
    "communities",
    "entity_mentions",
    "graphrag_runs",
    "pipeline_executions",
]


def inspect_databases() -> Tuple[Dict[str, Any], int]:
    """
    Inspect all databases and their GraphRAG-related collections.
    
    Returns:
        Database inventory with collection counts
    """
    try:
        client = get_mongo_client()
        
        # Get all database names (exclude system DBs)
        db_names = [
            name for name in client.list_database_names()
            if name not in SYSTEM_DATABASES
        ]
        
        databases = []
        for db_name in db_names:
            db = client[db_name]
            collections = []
            
            for coll_name in db.list_collection_names():
                # Only include GraphRAG-related collections
                if coll_name in GRAPHRAG_COLLECTIONS or coll_name.startswith("experiment"):
                    try:
                        count = db[coll_name].estimated_document_count()
                        collections.append({
                            "name": coll_name,
                            "count": count,
                        })
                    except Exception as e:
                        logger.warning(f"Failed to count {db_name}.{coll_name}: {e}")
            
            if collections:  # Only include DBs with relevant collections
                databases.append({
                    "name": db_name,
                    "collections": sorted(collections, key=lambda x: x["name"]),
                })
        
        return {
            "databases": sorted(databases, key=lambda x: x["name"]),
            "timestamp": datetime.utcnow().isoformat() + "Z",
        }, 200
        
    except Exception as e:
        logger.exception("Failed to inspect databases")
        return {"error": str(e), "operation": "inspect_databases"}, 500


# =============================================================================
# Copy Collection
# =============================================================================

def copy_collection(
    source_db: str,
    target_db: str,
    collection: str,
    max_documents: Optional[int] = None,
    clear_target: bool = True,
) -> Tuple[Dict[str, Any], int]:
    """
    Copy collection from one database to another.
    
    Args:
        source_db: Source database name
        target_db: Target database name  
        collection: Collection name to copy
        max_documents: Maximum documents to copy (None = all)
        clear_target: Whether to clear target collection first
        
    Returns:
        Operation status with progress info
    """
    # Validation
    if not source_db or not target_db:
        return {"error": "source_db and target_db are required", "operation": "copy_collection"}, 400
    
    if source_db == target_db:
        return {"error": "Source and target databases must be different", "operation": "copy_collection"}, 400
    
    if not collection:
        return {"error": "collection is required", "operation": "copy_collection"}, 400
    
    try:
        client = get_mongo_client()
        source_coll = client[source_db][collection]
        target_coll = client[target_db][collection]
        
        # Count source documents
        total = source_coll.count_documents({})
        
        if total == 0:
            return {"error": f"Source collection {source_db}.{collection} is empty", "operation": "copy_collection"}, 400
        
        if max_documents and max_documents > 0:
            total = min(max_documents, total)
        
        # Create operation tracking
        operation_id = _create_operation(
            operation_type="copy_collection",
            params={
                "source_db": source_db,
                "target_db": target_db,
                "collection": collection,
                "max_documents": max_documents,
                "clear_target": clear_target,
            },
            total=total,
        )
        
        # For small operations (<1000 docs), execute synchronously
        if total < 1000:
            return _copy_collection_sync(
                operation_id, source_coll, target_coll, total, clear_target
            )
        
        # For large operations, execute in background
        thread = threading.Thread(
            target=_copy_collection_background,
            args=(operation_id, source_coll, target_coll, total, clear_target),
            daemon=True,
        )
        thread.start()
        
        _update_operation(operation_id, status="running")
        
        return {
            "operation_id": operation_id,
            "status": "running",
            "message": f"Copying {total} documents in background",
        }, 202
        
    except Exception as e:
        logger.exception("Failed to start copy operation")
        return {"error": str(e), "operation": "copy_collection"}, 500


def _copy_collection_sync(
    operation_id: str,
    source_coll,
    target_coll,
    total: int,
    clear_target: bool,
) -> Tuple[Dict[str, Any], int]:
    """Synchronous copy for small collections."""
    try:
        _update_operation(operation_id, status="running")
        
        if clear_target:
            target_coll.delete_many({})
        
        docs = list(source_coll.find({}).limit(total))
        
        if docs:
            # Remove _id to allow MongoDB to generate new ones
            for doc in docs:
                doc.pop("_id", None)
            target_coll.insert_many(docs)
        
        _update_operation(operation_id, status="completed", processed=len(docs))
        
        return {
            "operation_id": operation_id,
            "status": "completed",
            "documents_copied": len(docs),
        }, 200
        
    except Exception as e:
        _update_operation(operation_id, status="failed", error=str(e))
        return {"error": str(e), "operation": "copy_collection"}, 500


def _copy_collection_background(
    operation_id: str,
    source_coll,
    target_coll,
    total: int,
    clear_target: bool,
) -> None:
    """Background copy for large collections."""
    try:
        if clear_target:
            target_coll.delete_many({})
        
        cursor = source_coll.find({}).limit(total)
        batch = []
        copied = 0
        batch_size = 1000
        
        for doc in cursor:
            doc.pop("_id", None)  # Remove _id
            batch.append(doc)
            
            if len(batch) >= batch_size:
                target_coll.insert_many(batch)
                copied += len(batch)
                _update_operation(operation_id, processed=copied)
                batch = []
        
        # Insert remaining documents
        if batch:
            target_coll.insert_many(batch)
            copied += len(batch)
        
        _update_operation(operation_id, status="completed", processed=copied)
        logger.info(f"Copy operation {operation_id} completed: {copied} documents")
        
    except Exception as e:
        logger.error(f"Background copy {operation_id} failed: {e}")
        _update_operation(operation_id, status="failed", error=str(e))


# =============================================================================
# Clean GraphRAG Data
# =============================================================================

# Default collections to drop when cleaning GraphRAG data
DEFAULT_GRAPHRAG_DROP = [
    "entities",
    "relations",
    "communities",
    "entity_mentions",
    "graphrag_runs",
]

# GraphRAG nested field names in chunks (the actual fields used in the codebase)
GRAPHRAG_CHUNK_FIELDS = [
    "graphrag_extraction",
    "graphrag_resolution",
    "graphrag_construction",
    "graphrag_communities",
]


def clean_graphrag_data(
    db_name: str,
    drop_collections: Optional[List[str]] = None,
    clear_chunk_metadata: bool = True,
) -> Tuple[Dict[str, Any], int]:
    """
    Clean GraphRAG data from database.
    
    Args:
        db_name: Database name
        drop_collections: List of collections to drop (default: entities, relations, etc.)
        clear_chunk_metadata: Whether to remove GraphRAG metadata from chunks
        
    Returns:
        Deletion counts and success status
    """
    if not db_name:
        return {"error": "db_name is required", "operation": "clean_graphrag_data"}, 400
    
    # Safety check - don't clean while pipeline is running on this DB
    try:
        from .execution import list_active_pipelines
        active = list_active_pipelines()
        running_on_db = [
            p for p in active.get("pipelines", {}).values()
            if p.get("config", {}).get("db_name") == db_name
        ]
        if running_on_db:
            return {
                "error": f"Cannot clean {db_name} - pipeline in progress",
                "active_pipelines": [p["pipeline_id"] for p in running_on_db],
                "operation": "clean_graphrag_data",
            }, 409  # Conflict
    except Exception as e:
        logger.warning(f"Could not check for active pipelines: {e}")
    
    if drop_collections is None:
        drop_collections = DEFAULT_GRAPHRAG_DROP
    
    try:
        client = get_mongo_client()
        db = client[db_name]
        
        results = {
            "db_name": db_name,
            "dropped_collections": [],
            "chunks_updated": 0,
            "success": True,
        }
        
        # Drop specified collections
        for coll_name in drop_collections:
            if coll_name in db.list_collection_names():
                count = db[coll_name].count_documents({})
                db[coll_name].drop()
                results["dropped_collections"].append({
                    "name": coll_name,
                    "documents_deleted": count,
                })
                logger.info(f"Dropped {db_name}.{coll_name} ({count} documents)")
        
        # Clear chunk metadata if requested
        if clear_chunk_metadata and COLL_CHUNKS in db.list_collection_names():
            # Remove GraphRAG-specific nested fields from chunks
            unset_fields = {field: "" for field in GRAPHRAG_CHUNK_FIELDS}
            
            update_result = db[COLL_CHUNKS].update_many(
                {},
                {"$unset": unset_fields}
            )
            results["chunks_updated"] = update_result.modified_count
            results["fields_removed"] = GRAPHRAG_CHUNK_FIELDS
            logger.info(f"Cleared metadata from {update_result.modified_count} chunks")
        
        results["timestamp"] = datetime.utcnow().isoformat() + "Z"
        return results, 200
        
    except Exception as e:
        logger.exception("Failed to clean GraphRAG data")
        return {"error": str(e), "success": False, "operation": "clean_graphrag_data"}, 500


# =============================================================================
# Clean Stage Status
# =============================================================================

# Mapping from stage names to chunk fields (nested objects)
STAGE_GRAPHRAG_FIELDS = {
    "extraction": "graphrag_extraction",
    "resolution": "graphrag_resolution",
    "construction": "graphrag_construction",
    "community": "graphrag_communities",
}


def clean_stage_status(
    db_name: str,
    stages: List[str],
) -> Tuple[Dict[str, Any], int]:
    """
    Clean GraphRAG stage processing data from chunks to allow re-processing.
    
    Args:
        db_name: Database name
        stages: List of stage names (extraction, resolution, construction, community)
        
    Returns:
        Modified count and success status
    """
    if not db_name:
        return {"error": "db_name is required", "operation": "clean_stage_status"}, 400
    
    if not stages:
        return {"error": "stages list is required", "operation": "clean_stage_status"}, 400
    
    # Validate stage names
    invalid_stages = [s for s in stages if s not in STAGE_GRAPHRAG_FIELDS]
    if invalid_stages:
        return {
            "error": f"Invalid stages: {invalid_stages}",
            "valid_stages": list(STAGE_GRAPHRAG_FIELDS.keys()),
            "operation": "clean_stage_status",
        }, 400
    
    # Safety check - don't clean while pipeline is running on this DB
    try:
        from .execution import list_active_pipelines
        active = list_active_pipelines()
        running_on_db = [
            p for p in active.get("pipelines", {}).values()
            if p.get("config", {}).get("db_name") == db_name
        ]
        if running_on_db:
            return {
                "error": f"Cannot clean {db_name} - pipeline in progress",
                "active_pipelines": [p["pipeline_id"] for p in running_on_db],
                "operation": "clean_stage_status",
            }, 409  # Conflict
    except Exception as e:
        logger.warning(f"Could not check for active pipelines: {e}")
    
    try:
        client = get_mongo_client()
        db = client[db_name]
        
        # Build unset operation for nested graphrag fields
        unset_fields = {}
        for stage in stages:
            field_name = STAGE_GRAPHRAG_FIELDS[stage]
            unset_fields[field_name] = ""
        
        # Update chunks
        result = db[COLL_CHUNKS].update_many(
            {},
            {"$unset": unset_fields}
        )
        
        return {
            "db_name": db_name,
            "stages_cleaned": stages,
            "fields_removed": list(unset_fields.keys()),
            "chunks_modified": result.modified_count,
            "success": True,
            "timestamp": datetime.utcnow().isoformat() + "Z",
        }, 200
        
    except Exception as e:
        logger.exception("Failed to clean stage status")
        return {"error": str(e), "success": False, "operation": "clean_stage_status"}, 500


# =============================================================================
# Setup Test Database
# =============================================================================

def setup_test_database(
    source_db: str,
    target_db: str,
    chunk_count: int = 50,
    diversity_mode: bool = True,
) -> Tuple[Dict[str, Any], int]:
    """
    Setup a test database with a diverse selection of chunks.
    
    Args:
        source_db: Source database name
        target_db: Target database name
        chunk_count: Number of chunks to select
        diversity_mode: If True, select one chunk per video for diversity
        
    Returns:
        Setup result with selected chunk info
    """
    if not source_db or not target_db:
        return {"error": "source_db and target_db are required", "operation": "setup_test_database"}, 400
    
    if source_db == target_db:
        return {"error": "Source and target databases must be different", "operation": "setup_test_database"}, 400
    
    try:
        client = get_mongo_client()
        source = client[source_db]
        target = client[target_db]
        
        # Check source has chunks
        if COLL_CHUNKS not in source.list_collection_names():
            return {"error": f"No {COLL_CHUNKS} collection in {source_db}", "operation": "setup_test_database"}, 400
        
        # Select chunks with diversity
        if diversity_mode:
            # Get distinct video IDs, then one chunk per video
            pipeline = [
                {"$group": {"_id": "$video_id", "chunk": {"$first": "$$ROOT"}}},
                {"$replaceRoot": {"newRoot": "$chunk"}},
                {"$limit": chunk_count},
            ]
            chunks = list(source[COLL_CHUNKS].aggregate(pipeline))
        else:
            # Random selection
            chunks = list(source[COLL_CHUNKS].aggregate([
                {"$sample": {"size": chunk_count}}
            ]))
        
        if not chunks:
            return {"error": "No chunks found in source database", "operation": "setup_test_database"}, 400
        
        # Clear target and insert selected chunks
        target[COLL_CHUNKS].delete_many({})
        
        for chunk in chunks:
            chunk.pop("_id", None)
            # Clear any existing GraphRAG status fields
            for field in GRAPHRAG_CHUNK_FIELDS:
                chunk.pop(field, None)
        
        target[COLL_CHUNKS].insert_many(chunks)
        
        # Get unique video count
        unique_videos = len(set(c.get("video_id", "") for c in chunks))
        
        return {
            "source_db": source_db,
            "target_db": target_db,
            "chunks_selected": len(chunks),
            "unique_videos": unique_videos,
            "diversity_mode": diversity_mode,
            "success": True,
            "timestamp": datetime.utcnow().isoformat() + "Z",
        }, 200
        
    except Exception as e:
        logger.exception("Failed to setup test database")
        return {"error": str(e), "success": False, "operation": "setup_test_database"}, 500


# =============================================================================
# Rebuild Indexes
# =============================================================================

# Index definitions for each collection
COLLECTION_INDEXES = {
    "entities": [
        {"keys": [("name", 1)], "unique": False},
        {"keys": [("type", 1)], "unique": False},
        {"keys": [("name", "text"), ("description", "text")], "unique": False},
    ],
    "relations": [
        {"keys": [("source_id", 1)], "unique": False},
        {"keys": [("target_id", 1)], "unique": False},
        {"keys": [("predicate", 1)], "unique": False},
    ],
    "communities": [
        {"keys": [("level", 1)], "unique": False},
        {"keys": [("member_ids", 1)], "unique": False},
    ],
    COLL_CHUNKS: [
        {"keys": [("video_id", 1)], "unique": False},
        {"keys": [("graphrag_extraction.status", 1)], "unique": False},
    ],
}


def rebuild_indexes(
    db_name: str,
    collections: Optional[List[str]] = None,
) -> Tuple[Dict[str, Any], int]:
    """
    Rebuild indexes for specified collections.
    
    Args:
        db_name: Database name
        collections: List of collection names (default: all GraphRAG collections)
        
    Returns:
        Index creation status
    """
    if not db_name:
        return {"error": "db_name is required", "operation": "rebuild_indexes"}, 400
    
    if collections is None:
        collections = list(COLLECTION_INDEXES.keys())
    
    try:
        client = get_mongo_client()
        db = client[db_name]
        
        results = {
            "db_name": db_name,
            "collections_indexed": [],
            "success": True,
        }
        
        for coll_name in collections:
            if coll_name not in db.list_collection_names():
                continue
            
            if coll_name not in COLLECTION_INDEXES:
                continue
            
            coll = db[coll_name]
            indexes_created = []
            
            # Drop existing indexes (except _id)
            coll.drop_indexes()
            
            # Create defined indexes
            for index_def in COLLECTION_INDEXES[coll_name]:
                try:
                    index_name = coll.create_index(
                        index_def["keys"],
                        unique=index_def.get("unique", False),
                    )
                    indexes_created.append(index_name)
                except Exception as e:
                    logger.warning(f"Failed to create index on {coll_name}: {e}")
            
            results["collections_indexed"].append({
                "name": coll_name,
                "indexes_created": indexes_created,
            })
        
        results["timestamp"] = datetime.utcnow().isoformat() + "Z"
        return results, 200
        
    except Exception as e:
        logger.exception("Failed to rebuild indexes")
        return {"error": str(e), "success": False, "operation": "rebuild_indexes"}, 500

