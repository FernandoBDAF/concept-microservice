"""
Viewer API Handlers

Read-only data access for MongoDB collections.
Supports the StagesUI Viewer module for data inspection.
"""

import logging
import base64
from datetime import datetime
from typing import Dict, Any, Optional, List, Tuple

from bson import ObjectId
from bson.errors import InvalidId
from pymongo import ASCENDING, DESCENDING

# Import shared database utilities
from .db import get_mongo_client, SYSTEM_DATABASES, MAX_QUERY_LIMIT, BINARY_INLINE_THRESHOLD

# Import collection constants from core config
from src.core.config.paths import (
    COLL_CLEANED,           # "cleaned_transcripts"
    COLL_RAW_VIDEOS,        # "raw_videos"
    COLL_ENRICHED,          # "enriched_transcripts"
    COLL_CHUNKS,            # "video_chunks"
    COLL_ENTITIES,          # "entities"
    COLL_RELATIONS,         # "relations"
    COLL_COMMUNITIES,       # "communities"
    COLL_ENTITY_MENTIONS,   # "entity_mentions"
)

# Import additional collection constants if available
try:
    from src.core.config.paths import (
        COLL_TRANSFORMATION_LOGS,
        COLL_ENTITIES_RAW,
        COLL_ENTITIES_RESOLVED,
        COLL_RELATIONS_RAW,
        COLL_RELATIONS_FINAL,
        COLL_GRAPH_PRE_DETECTION,
        COLL_QUALITY_METRICS,
        COLL_GRAPHRAG_RUNS,
        COLL_MULTIMODAL,
    )
    _HAS_EXTENDED_COLLECTIONS = True
except ImportError:
    _HAS_EXTENDED_COLLECTIONS = False

logger = logging.getLogger(__name__)

# =============================================================================
# Schema Detection
# =============================================================================

# Field type detection thresholds
LONG_TEXT_THRESHOLD = 500  # chars to consider "long text"

# Known collection schemas for renderer suggestions
# Using constants from paths.py for consistency
COLLECTION_SCHEMAS = {
    COLL_CLEANED: {"renderer": "long_text", "text_field": "transcript"},
    COLL_RAW_VIDEOS: {"renderer": "long_text", "text_field": "transcript"},
    COLL_ENRICHED: {"renderer": "long_text", "text_field": "transcript"},
    COLL_CHUNKS: {"renderer": "long_text", "text_field": "text"},
    COLL_ENTITIES: {"renderer": "json", "primary_field": "name"},
    COLL_RELATIONS: {"renderer": "json", "primary_field": "predicate"},
    COLL_COMMUNITIES: {"renderer": "json", "primary_field": "title"},
    COLL_ENTITY_MENTIONS: {"renderer": "table", "primary_field": "entity_name"},
}

# Add extended collection schemas if available
if _HAS_EXTENDED_COLLECTIONS:
    COLLECTION_SCHEMAS.update({
        COLL_TRANSFORMATION_LOGS: {"renderer": "json", "primary_field": "stage"},
        COLL_ENTITIES_RAW: {"renderer": "json", "primary_field": "name"},
        COLL_ENTITIES_RESOLVED: {"renderer": "json", "primary_field": "name"},
        COLL_RELATIONS_RAW: {"renderer": "json", "primary_field": "predicate"},
        COLL_RELATIONS_FINAL: {"renderer": "json", "primary_field": "predicate"},
        COLL_GRAPH_PRE_DETECTION: {"renderer": "json", "primary_field": "type"},
        COLL_QUALITY_METRICS: {"renderer": "json", "primary_field": "metric_name"},
        COLL_GRAPHRAG_RUNS: {"renderer": "json", "primary_field": "run_id"},
        COLL_MULTIMODAL: {"renderer": "long_text", "text_field": "content"},
    })


# =============================================================================
# Serialization Helpers
# =============================================================================

def serialize_document(doc: Optional[Dict[str, Any]]) -> Optional[Dict[str, Any]]:
    """
    Convert MongoDB document to JSON-serializable format.
    
    Handles:
    - ObjectId -> string
    - datetime -> ISO string
    - bytes -> base64 (if small) or "[binary data]"
    - Nested documents recursively
    """
    if doc is None:
        return None
    
    result = {}
    for key, value in doc.items():
        result[key] = serialize_value(value)
    return result


def serialize_value(value: Any) -> Any:
    """Serialize a single value."""
    if isinstance(value, ObjectId):
        return str(value)
    elif isinstance(value, datetime):
        return value.isoformat() + "Z"
    elif isinstance(value, bytes):
        if len(value) < BINARY_INLINE_THRESHOLD:
            return {"$binary": base64.b64encode(value).decode()}
        return "[binary data]"
    elif isinstance(value, dict):
        return serialize_document(value)
    elif isinstance(value, list):
        return [serialize_value(v) for v in value]
    return value


# =============================================================================
# Schema Detection Functions
# =============================================================================

def detect_document_type(doc: Dict[str, Any], collection_name: str) -> Dict[str, Any]:
    """
    Analyze document structure and suggest appropriate renderer.
    
    Returns:
        {
            "suggested_renderer": "long_text" | "json" | "table",
            "text_field": str | None,  # For long_text renderer
            "has_nested": bool,
            "field_count": int,
        }
    """
    # Check known collection schemas first
    if collection_name in COLLECTION_SCHEMAS:
        schema = COLLECTION_SCHEMAS[collection_name]
        return {
            "suggested_renderer": schema["renderer"],
            "text_field": schema.get("text_field"),
            "primary_field": schema.get("primary_field"),
            "has_nested": any(isinstance(v, (dict, list)) for v in doc.values()),
            "field_count": len(doc),
        }
    
    # Detect based on content analysis
    text_fields = []
    has_nested = False
    
    for key, value in doc.items():
        if key.startswith("_"):
            continue
        if isinstance(value, str) and len(value) > LONG_TEXT_THRESHOLD:
            text_fields.append(key)
        if isinstance(value, (dict, list)):
            has_nested = True
    
    # Determine renderer
    if text_fields:
        return {
            "suggested_renderer": "long_text",
            "text_field": text_fields[0],  # Primary long text field
            "text_fields": text_fields,
            "has_nested": has_nested,
            "field_count": len(doc),
        }
    elif has_nested:
        return {
            "suggested_renderer": "json",
            "has_nested": True,
            "field_count": len(doc),
        }
    else:
        return {
            "suggested_renderer": "table",
            "has_nested": False,
            "field_count": len(doc),
        }


def infer_collection_schema(
    db_name: str,
    collection_name: str,
    sample_size: int = 10,
) -> Dict[str, Any]:
    """
    Infer schema from collection by sampling documents.
    
    Returns field types, nested structure info, and renderer suggestion.
    """
    client = get_mongo_client()
    coll = client[db_name][collection_name]
    
    # Sample documents
    samples = list(coll.aggregate([{"$sample": {"size": sample_size}}]))
    
    if not samples:
        return {
            "collection": collection_name,
            "document_count": 0,
            "fields": {},
            "suggested_renderer": "json",
        }
    
    # Aggregate field information
    fields: Dict[str, Dict[str, Any]] = {}
    for doc in samples:
        for key, value in doc.items():
            if key not in fields:
                fields[key] = {
                    "types": set(),
                    "nullable": False,
                    "max_length": 0,
                }
            
            field_info = fields[key]
            
            if value is None:
                field_info["nullable"] = True
            else:
                field_info["types"].add(type(value).__name__)
                
                if isinstance(value, str):
                    field_info["max_length"] = max(
                        field_info["max_length"],
                        len(value)
                    )
    
    # Convert sets to lists for JSON serialization
    for field in fields.values():
        field["types"] = list(field["types"])
    
    # Detect document type from first sample
    doc_type = detect_document_type(samples[0], collection_name)
    
    return {
        "collection": collection_name,
        "document_count": coll.estimated_document_count(),
        "fields": fields,
        "sample_size": len(samples),
        "suggested_renderer": doc_type["suggested_renderer"],
        "text_field": doc_type.get("text_field"),
    }


# =============================================================================
# API Handlers
# =============================================================================

def get_database_catalog() -> Tuple[Dict[str, Any], int]:
    """
    List all databases with document counts.
    
    Excludes system databases (admin, config, local).
    
    Response:
        {
            "databases": [
                {"name": "2025-12", "collections": 8, "total_documents": 15420},
                ...
            ],
            "timestamp": "..."
        }
    """
    try:
        client = get_mongo_client()
        
        databases = []
        for db_name in client.list_database_names():
            if db_name in SYSTEM_DATABASES:
                continue
            
            db = client[db_name]
            collections = db.list_collection_names()
            
            # Count total documents across all collections
            total_docs = 0
            for coll_name in collections:
                try:
                    total_docs += db[coll_name].estimated_document_count()
                except Exception:
                    pass
            
            databases.append({
                "name": db_name,
                "collections": len(collections),
                "total_documents": total_docs,
            })
        
        return {
            "databases": sorted(databases, key=lambda x: x["name"]),
            "timestamp": datetime.utcnow().isoformat() + "Z",
        }, 200
        
    except Exception as e:
        logger.exception("Failed to list databases")
        return {"error": str(e)}, 500


def list_collections(db_name: str) -> Tuple[Dict[str, Any], int]:
    """
    List collections in a database with schema hints.
    
    Args:
        db_name: Database name
        
    Response:
        {
            "database": "2025-12",
            "collections": [
                {
                    "name": "cleaned_transcripts",
                    "document_count": 1234,
                    "suggested_renderer": "long_text",
                    "text_field": "transcript"
                },
                ...
            ]
        }
    """
    if not db_name:
        return {"error": "db_name is required"}, 400
    
    try:
        client = get_mongo_client()
        db = client[db_name]
        
        collections = []
        for coll_name in db.list_collection_names():
            coll = db[coll_name]
            
            # Get document count
            doc_count = coll.estimated_document_count()
            
            # Get schema hint from known schemas or sample
            if coll_name in COLLECTION_SCHEMAS:
                schema = COLLECTION_SCHEMAS[coll_name]
                renderer = schema["renderer"]
                text_field = schema.get("text_field")
            else:
                # Sample one document for type detection
                sample = coll.find_one()
                if sample:
                    doc_type = detect_document_type(sample, coll_name)
                    renderer = doc_type["suggested_renderer"]
                    text_field = doc_type.get("text_field")
                else:
                    renderer = "json"
                    text_field = None
            
            collections.append({
                "name": coll_name,
                "document_count": doc_count,
                "suggested_renderer": renderer,
                "text_field": text_field,
            })
        
        return {
            "database": db_name,
            "collections": sorted(collections, key=lambda x: x["name"]),
            "timestamp": datetime.utcnow().isoformat() + "Z",
        }, 200
        
    except Exception as e:
        logger.exception(f"Failed to list collections in {db_name}")
        return {"error": str(e)}, 500


def get_document(
    db_name: str,
    collection_name: str,
    document_id: str,
) -> Tuple[Dict[str, Any], int]:
    """
    Get a single document by ID.
    
    Args:
        db_name: Database name
        collection_name: Collection name
        document_id: Document ID (string representation of ObjectId or other ID)
        
    Response:
        {
            "document": { ... serialized document ... },
            "metadata": {
                "suggested_renderer": "long_text",
                "text_field": "transcript",
                "field_count": 12
            }
        }
    """
    if not db_name or not collection_name or not document_id:
        return {"error": "db_name, collection_name, and document_id are required"}, 400
    
    try:
        client = get_mongo_client()
        coll = client[db_name][collection_name]
        
        # Try to convert to ObjectId, fall back to string ID
        try:
            query_id = ObjectId(document_id)
        except InvalidId:
            query_id = document_id
        
        doc = coll.find_one({"_id": query_id})
        
        if not doc:
            return {"error": "Document not found", "document_id": document_id}, 404
        
        # Serialize and add metadata
        serialized = serialize_document(doc)
        doc_type = detect_document_type(doc, collection_name)
        
        return {
            "document": serialized,
            "metadata": {
                "suggested_renderer": doc_type["suggested_renderer"],
                "text_field": doc_type.get("text_field"),
                "field_count": doc_type["field_count"],
                "has_nested": doc_type.get("has_nested", False),
            },
        }, 200
        
    except Exception as e:
        logger.exception(f"Failed to get document {document_id}")
        return {"error": str(e)}, 500


def query_collection(
    db_name: str,
    collection_name: str,
    filter_query: Optional[Dict[str, Any]] = None,
    projection: Optional[List[str]] = None,
    sort: Optional[List[Dict[str, Any]]] = None,
    skip: int = 0,
    limit: int = 20,
) -> Tuple[Dict[str, Any], int]:
    """
    Query collection with filters, pagination, and sorting.
    
    Args:
        db_name: Database name
        collection_name: Collection name
        filter_query: MongoDB filter (simplified, no $where allowed)
        projection: List of fields to include
        sort: List of {"field": "name", "order": "asc"|"desc"}
        skip: Documents to skip (pagination)
        limit: Maximum documents to return (capped at MAX_QUERY_LIMIT)
        
    Response:
        {
            "documents": [ ... ],
            "total": 1234,
            "returned": 20,
            "skip": 0,
            "limit": 20,
            "has_more": true
        }
    """
    if not db_name or not collection_name:
        return {"error": "db_name and collection_name are required"}, 400
    
    # Security: Validate filter query
    if filter_query:
        # Disallow dangerous operators
        dangerous_ops = ["$where", "$function", "$accumulator"]
        filter_str = str(filter_query)
        for op in dangerous_ops:
            if op in filter_str:
                return {"error": f"Operator {op} is not allowed"}, 400
    
    # Cap limit
    limit = min(limit, MAX_QUERY_LIMIT)
    
    try:
        client = get_mongo_client()
        coll = client[db_name][collection_name]
        
        # Build query
        query_filter = filter_query or {}
        
        # Build projection
        proj = None
        if projection:
            proj = {field: 1 for field in projection}
            proj["_id"] = 1  # Always include _id
        
        # Build sort
        sort_spec = None
        if sort:
            sort_spec = []
            for s in sort:
                direction = ASCENDING if s.get("order", "asc") == "asc" else DESCENDING
                sort_spec.append((s["field"], direction))
        
        # Get total count (before pagination)
        total = coll.count_documents(query_filter)
        
        # Execute query
        cursor = coll.find(query_filter, proj)
        
        if sort_spec:
            cursor = cursor.sort(sort_spec)
        
        cursor = cursor.skip(skip).limit(limit)
        
        # Serialize results
        documents = [serialize_document(doc) for doc in cursor]
        
        return {
            "documents": documents,
            "total": total,
            "returned": len(documents),
            "skip": skip,
            "limit": limit,
            "has_more": skip + len(documents) < total,
        }, 200
        
    except Exception as e:
        logger.exception(f"Failed to query {db_name}.{collection_name}")
        return {"error": str(e)}, 500


def get_collection_schema(
    db_name: str,
    collection_name: str,
) -> Tuple[Dict[str, Any], int]:
    """
    Get inferred schema for a collection.
    
    Samples documents to determine field types and structure.
    """
    if not db_name or not collection_name:
        return {"error": "db_name and collection_name are required"}, 400
    
    try:
        schema = infer_collection_schema(db_name, collection_name)
        schema["database"] = db_name
        schema["timestamp"] = datetime.utcnow().isoformat() + "Z"
        return schema, 200
        
    except Exception as e:
        logger.exception(f"Failed to get schema for {db_name}.{collection_name}")
        return {"error": str(e)}, 500

