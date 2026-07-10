"""
Iteration API Handlers

Provides endpoints for:
- Document version comparison
- Pipeline run timeline
- Re-run suggestions

Supports the data optimization workflow in StagesUI Viewer.
"""

import os
import re
import logging
from datetime import datetime
from difflib import SequenceMatcher
from typing import Dict, Any, Optional, List, Tuple

from bson import ObjectId
from bson.errors import InvalidId

from .db import get_mongo_client
from .viewer import serialize_document

logger = logging.getLogger(__name__)

# =============================================================================
# Configuration
# =============================================================================

# Collections that track processing history
TRACKED_COLLECTIONS = {
    "video_chunks": {
        "source_field": "video_id",
        "version_field": "processing_version",
        "timestamp_field": "updated_at",
        "stage_fields": ["graphrag_extraction", "graphrag_resolution", 
                         "graphrag_construction", "graphrag_communities"]
    },
    "cleaned_transcripts": {
        "source_field": "video_id",
        "version_field": "clean_version",
        "timestamp_field": "cleaned_at",
        "stage_fields": []
    },
    "enriched_transcripts": {
        "source_field": "video_id",
        "version_field": "enrich_version",
        "timestamp_field": "enriched_at",
        "stage_fields": []
    },
    "entities": {
        "source_field": "source_chunk_id",
        "version_field": "extraction_run",
        "timestamp_field": "created_at",
        "stage_fields": []
    }
}

# Re-run suggestion rules
RERUN_SUGGESTIONS = {
    "low_trust_entity": {
        "stage": "extraction",
        "params": {"increase_context_window": True, "entity_confidence_threshold": 0.3},
        "reason": "Low entity trust scores suggest insufficient context during extraction"
    },
    "missing_relations": {
        "stage": "extraction", 
        "params": {"enable_relation_extraction": True, "relation_depth": 2},
        "reason": "No relations extracted - try enabling deeper relation extraction"
    },
    "citation_not_removed": {
        "stage": "clean",
        "params": {"citation_removal_strength": "strict", "remove_urls": True},
        "reason": "Citations still present - use stricter cleaning parameters"
    },
    "fragmented_chunks": {
        "stage": "chunk",
        "params": {"min_chunk_size": 500, "overlap": 100},
        "reason": "Chunks too small - increase minimum size for better context"
    }
}


# =============================================================================
# Comparison Functions
# =============================================================================

def compare_documents(
    db_name: str,
    collection_name: str,
    doc_id_1: str,
    doc_id_2: str
) -> Tuple[Dict[str, Any], int]:
    """
    Compare two documents and return differences.
    
    Args:
        db_name: Database name
        collection_name: Collection name
        doc_id_1: First document ID (usually older)
        doc_id_2: Second document ID (usually newer)
        
    Response:
        {
            "left": {...},           # First document
            "right": {...},          # Second document
            "diff": {
                "added_fields": [],
                "removed_fields": [],
                "changed_fields": [
                    {
                        "field": "text",
                        "similarity": 0.87,
                        "changes": [
                            {"type": "delete", "text": "old text", "position": [10, 20]},
                            {"type": "insert", "text": "new text", "position": [10, 20]}
                        ]
                    }
                ],
                "unchanged_fields": []
            },
            "metrics": {
                "overall_similarity": 0.92,
                "field_count_delta": +2,
                "text_length_delta": +150
            }
        }
    """
    if not db_name or not collection_name:
        return {"error": "db_name and collection_name required"}, 400
    
    client = get_mongo_client()
    coll = client[db_name][collection_name]
    
    # Fetch both documents
    doc1 = _fetch_document(coll, doc_id_1)
    doc2 = _fetch_document(coll, doc_id_2)
    
    if not doc1:
        return {"error": f"Document {doc_id_1} not found"}, 404
    if not doc2:
        return {"error": f"Document {doc_id_2} not found"}, 404
    
    # Serialize for comparison
    doc1_serialized = serialize_document(doc1)
    doc2_serialized = serialize_document(doc2)
    
    # Calculate diff
    diff = _calculate_diff(doc1_serialized, doc2_serialized)
    metrics = _calculate_metrics(doc1_serialized, doc2_serialized)
    
    return {
        "left": doc1_serialized,
        "right": doc2_serialized,
        "diff": diff,
        "metrics": metrics,
        "timestamp": datetime.utcnow().isoformat() + "Z"
    }, 200


def _fetch_document(coll, doc_id: str) -> Optional[Dict]:
    """Fetch document by ID (ObjectId or string)."""
    try:
        query_id = ObjectId(doc_id)
    except (InvalidId, TypeError):
        query_id = doc_id
    
    return coll.find_one({"_id": query_id})


def _calculate_diff(doc1: Dict, doc2: Dict) -> Dict:
    """Calculate structural and content differences."""
    keys1 = set(doc1.keys())
    keys2 = set(doc2.keys())
    
    added = list(keys2 - keys1)
    removed = list(keys1 - keys2)
    common = keys1 & keys2
    
    changed = []
    unchanged = []
    
    for key in common:
        if key == "_id":
            continue
            
        val1 = doc1[key]
        val2 = doc2[key]
        
        if val1 == val2:
            unchanged.append(key)
        else:
            change_info = {
                "field": key,
                "left_value": val1,
                "right_value": val2,
            }
            
            # For strings, calculate similarity and changes
            if isinstance(val1, str) and isinstance(val2, str):
                similarity = SequenceMatcher(None, val1, val2).ratio()
                change_info["similarity"] = round(similarity, 3)
                
                # Only generate detailed diff for moderate changes
                if 0.3 < similarity < 0.99:
                    change_info["changes"] = _get_text_changes(val1, val2)
            
            changed.append(change_info)
    
    return {
        "added_fields": added,
        "removed_fields": removed,
        "changed_fields": changed,
        "unchanged_fields": unchanged
    }


def _get_text_changes(text1: str, text2: str) -> List[Dict]:
    """Get detailed text changes for highlighting."""
    changes = []
    matcher = SequenceMatcher(None, text1, text2)
    
    for tag, i1, i2, j1, j2 in matcher.get_opcodes():
        if tag == 'equal':
            continue
        elif tag == 'replace':
            changes.append({
                "type": "replace",
                "left_text": text1[i1:i2],
                "right_text": text2[j1:j2],
                "left_position": [i1, i2],
                "right_position": [j1, j2]
            })
        elif tag == 'delete':
            changes.append({
                "type": "delete",
                "text": text1[i1:i2],
                "position": [i1, i2]
            })
        elif tag == 'insert':
            changes.append({
                "type": "insert",
                "text": text2[j1:j2],
                "position": [j1, j2]
            })
    
    return changes[:50]  # Limit to first 50 changes


def _calculate_metrics(doc1: Dict, doc2: Dict) -> Dict:
    """Calculate comparison metrics."""
    # Overall similarity based on common fields
    common_keys = set(doc1.keys()) & set(doc2.keys())
    if not common_keys:
        return {"overall_similarity": 0.0, "field_count_delta": 0, "text_length_delta": 0}
    
    similarities = []
    for key in common_keys:
        if key == "_id":
            continue
        val1 = str(doc1.get(key, ""))
        val2 = str(doc2.get(key, ""))
        similarities.append(SequenceMatcher(None, val1, val2).ratio())
    
    overall = sum(similarities) / len(similarities) if similarities else 0.0
    
    # Text-specific metrics
    text_field = _find_text_field(doc1)
    text_delta = 0
    if text_field:
        text1 = str(doc1.get(text_field, ""))
        text2 = str(doc2.get(text_field, ""))
        text_delta = len(text2) - len(text1)
    
    return {
        "overall_similarity": round(overall, 3),
        "field_count_delta": len(doc2) - len(doc1),
        "text_length_delta": text_delta
    }


def _find_text_field(doc: Dict) -> Optional[str]:
    """Find the main text field in a document."""
    for field in ["text", "content", "transcript", "description"]:
        if field in doc and isinstance(doc[field], str):
            return field
    return None


def _get_document_timestamp(doc: Dict, timestamp_field: str) -> Optional[datetime]:
    """
    Get timestamp with fallback chain.
    
    Tries in order:
    1. Configured timestamp field
    2. created_at
    3. updated_at
    4. processed_at
    5. ObjectId generation time
    """
    # Try configured field first
    if timestamp_field in doc:
        ts = doc[timestamp_field]
        if isinstance(ts, datetime):
            return ts
    
    # Fallback chain
    for field in ["created_at", "updated_at", "processed_at"]:
        if field in doc:
            ts = doc[field]
            if isinstance(ts, datetime):
                return ts
    
    # Last resort: ObjectId generation time
    try:
        doc_id = doc.get("_id")
        if doc_id and isinstance(doc_id, ObjectId):
            return doc_id.generation_time
    except Exception:
        pass
    
    return None


# =============================================================================
# Timeline Functions
# =============================================================================

def get_document_timeline(
    db_name: str,
    collection_name: str,
    source_id: str,
    source_field: Optional[str] = None
) -> Tuple[Dict[str, Any], int]:
    """
    Get timeline of all versions of a document by source ID.
    
    Args:
        db_name: Database name
        collection_name: Collection name  
        source_id: Source identifier (e.g., video_id)
        source_field: Field containing source ID (auto-detected if not provided)
        
    Response:
        {
            "source_id": "video_abc",
            "collection": "video_chunks",
            "timeline": [
                {
                    "doc_id": "507f...",
                    "version": 1,
                    "timestamp": "2024-12-10T10:00:00Z",
                    "stage_status": {
                        "extraction": "completed",
                        "resolution": "completed",
                        "construction": "pending"
                    },
                    "changes_from_previous": {
                        "fields_changed": 3,
                        "similarity": 0.85
                    }
                },
                ...
            ],
            "total_versions": 5
        }
    """
    if not db_name or not collection_name:
        return {"error": "db_name and collection_name required"}, 400
    
    # Determine source field
    if not source_field:
        config = TRACKED_COLLECTIONS.get(collection_name, {})
        source_field = config.get("source_field", "_id")
    
    client = get_mongo_client()
    coll = client[db_name][collection_name]
    
    # Query all documents with this source ID
    query = {source_field: source_id}
    
    # Get config for timestamp and stage fields
    config = TRACKED_COLLECTIONS.get(collection_name, {})
    timestamp_field = config.get("timestamp_field", "updated_at")
    stage_fields = config.get("stage_fields", [])
    
    # Fetch documents sorted by timestamp
    cursor = coll.find(query).sort(timestamp_field, 1)
    documents = list(cursor)
    
    if not documents:
        # Try querying by _id if source_field query failed
        try:
            doc = coll.find_one({"_id": ObjectId(source_id)})
            if doc:
                documents = [doc]
        except Exception:
            pass
    
    if not documents:
        return {"error": f"No documents found for source_id: {source_id}"}, 404
    
    # Build timeline
    timeline = []
    prev_doc = None
    
    for i, doc in enumerate(documents):
        doc_serialized = serialize_document(doc)
        
        # Get timestamp with fallback chain
        ts = _get_document_timestamp(doc, timestamp_field)
        ts_str = ts.isoformat() + "Z" if ts else None
        
        entry = {
            "doc_id": str(doc["_id"]),
            "version": i + 1,
            "timestamp": ts_str,
        }
        
        # Extract stage status
        if stage_fields:
            stage_status = {}
            for stage in stage_fields:
                if stage in doc:
                    stage_data = doc[stage]
                    if isinstance(stage_data, dict):
                        stage_status[stage.replace("graphrag_", "")] = stage_data.get("status", "unknown")
            entry["stage_status"] = stage_status
        
        # Calculate changes from previous version
        if prev_doc:
            metrics = _calculate_metrics(prev_doc, doc_serialized)
            entry["changes_from_previous"] = {
                "similarity": metrics["overall_similarity"],
                "field_count_delta": metrics["field_count_delta"]
            }
        
        timeline.append(entry)
        prev_doc = doc_serialized
    
    return {
        "source_id": source_id,
        "collection": collection_name,
        "source_field": source_field,
        "timeline": timeline,
        "total_versions": len(timeline),
        "timestamp": datetime.utcnow().isoformat() + "Z"
    }, 200


# =============================================================================
# Re-run Suggestion Functions
# =============================================================================

def suggest_rerun(
    db_name: str,
    collection_name: str,
    doc_id: str,
    issue_type: Optional[str] = None
) -> Tuple[Dict[str, Any], int]:
    """
    Suggest parameters for re-running a stage based on detected issues.
    
    Args:
        db_name: Database name
        collection_name: Collection name
        doc_id: Document ID
        issue_type: Optional specific issue type
        
    Response:
        {
            "doc_id": "507f...",
            "detected_issues": [
                {
                    "type": "low_trust_entity",
                    "description": "Entity 'Company X' has trust=0.45",
                    "field": "entities[2]"
                }
            ],
            "suggestions": [
                {
                    "stage": "extraction",
                    "params": {...},
                    "reason": "...",
                    "confidence": 0.85
                }
            ],
            "execution_link": "/execution?stage=extraction&input=..."
        }
    """
    if not db_name or not collection_name or not doc_id:
        return {"error": "db_name, collection_name, and doc_id required"}, 400
    
    client = get_mongo_client()
    doc = _fetch_document(client[db_name][collection_name], doc_id)
    
    if not doc:
        return {"error": f"Document {doc_id} not found"}, 404
    
    # Detect issues in the document
    issues = _detect_issues(doc, collection_name)
    
    # Filter by issue_type if specified
    if issue_type:
        issues = [i for i in issues if i["type"] == issue_type]
    
    # Generate suggestions based on issues
    suggestions = []
    for issue in issues:
        if issue["type"] in RERUN_SUGGESTIONS:
            suggestion = RERUN_SUGGESTIONS[issue["type"]].copy()
            suggestion["confidence"] = issue.get("confidence", 0.7)
            suggestion["for_issue"] = issue["type"]
            suggestions.append(suggestion)
    
    # Build execution link
    primary_suggestion = suggestions[0] if suggestions else {"stage": "extraction", "params": {}}
    execution_link = (
        f"/execution?"
        f"stage={primary_suggestion['stage']}&"
        f"input={doc_id}&"
        f"collection={collection_name}&"
        f"db={db_name}&"
        f"returnTo=/viewer?doc={doc_id}"
    )
    
    return {
        "doc_id": doc_id,
        "collection": collection_name,
        "detected_issues": issues,
        "suggestions": suggestions,
        "execution_link": execution_link,
        "timestamp": datetime.utcnow().isoformat() + "Z"
    }, 200


def _detect_issues(doc: Dict, collection_name: str) -> List[Dict]:
    """Detect quality issues in a document."""
    issues = []
    
    # Check for low trust entities
    if "entities" in doc or collection_name == "entities":
        entities = doc.get("entities", [doc] if collection_name == "entities" else [])
        for i, entity in enumerate(entities if isinstance(entities, list) else [entities]):
            trust = entity.get("trust_score", entity.get("confidence", 1.0))
            if trust and trust < 0.6:
                issues.append({
                    "type": "low_trust_entity",
                    "description": f"Entity '{entity.get('name', 'unknown')}' has trust={trust:.2f}",
                    "field": f"entities[{i}]" if isinstance(entities, list) else "trust_score",
                    "confidence": 0.85
                })
    
    # Check for missing relations
    if collection_name == "entities" and not doc.get("relations"):
        issues.append({
            "type": "missing_relations",
            "description": "No relations extracted for this entity",
            "field": "relations",
            "confidence": 0.7
        })
    
    # Check for uncleaned text patterns
    text = doc.get("text", doc.get("transcript", doc.get("content", "")))
    if isinstance(text, str):
        # Check for citations
        citation_patterns = [r'\[CITATION\]', r'\[\d+\]', r'ibid\.', r'et al\.']
        for pattern in citation_patterns:
            if re.search(pattern, text, re.IGNORECASE):
                issues.append({
                    "type": "citation_not_removed",
                    "description": f"Citation pattern found: {pattern}",
                    "field": "text",
                    "confidence": 0.9
                })
                break
    
    # Check for processing failures
    stage_fields = ["graphrag_extraction", "graphrag_resolution", 
                    "graphrag_construction", "graphrag_communities"]
    for stage in stage_fields:
        if stage in doc:
            stage_data = doc[stage]
            if isinstance(stage_data, dict):
                status = stage_data.get("status", "")
                if status in ["failed", "error"]:
                    issues.append({
                        "type": f"{stage}_failed",
                        "description": f"Stage {stage} failed: {stage_data.get('error', 'unknown error')}",
                        "field": stage,
                        "confidence": 1.0
                    })
    
    return issues


# =============================================================================
# Pipeline History Functions  
# =============================================================================

def get_run_history(
    db_name: str,
    collection_name: str,
    doc_id: str,
    limit: int = 20
) -> Tuple[Dict[str, Any], int]:
    """
    Get pipeline execution history for a document.
    
    This queries the observability database for runs that affected this document.
    
    Response:
        {
            "doc_id": "507f...",
            "runs": [
                {
                    "run_id": "run_47",
                    "pipeline": "graphrag",
                    "stage": "extraction",
                    "status": "completed",
                    "started_at": "2024-12-11T10:00:00Z",
                    "completed_at": "2024-12-11T10:05:00Z",
                    "duration_seconds": 300,
                    "params": {...},
                    "result_doc_id": "507f..."
                }
            ],
            "total_runs": 5
        }
    """
    obs_db_name = os.getenv("OBSERVABILITY_DB", "graphrag_observability")
    
    try:
        client = get_mongo_client()
        obs_db = client[obs_db_name]
        
        # Try to find runs that reference this document
        # The exact query depends on pipeline_executions schema
        query = {
            "$or": [
                {"input_doc_id": doc_id},
                {"output_doc_ids": doc_id},
                {"context.doc_id": doc_id}
            ]
        }
        
        cursor = obs_db["pipeline_executions"].find(query).sort("started_at", -1).limit(limit)
        runs = []
        
        for run in cursor:
            # Format timestamps
            started_at = run.get("started_at", "unknown")
            completed_at = run.get("completed_at")
            if isinstance(started_at, datetime):
                started_at = started_at.isoformat() + "Z"
            if isinstance(completed_at, datetime):
                completed_at = completed_at.isoformat() + "Z"
            
            runs.append({
                "run_id": str(run["_id"]),
                "pipeline": run.get("pipeline_name", "unknown"),
                "stage": run.get("stage_name", run.get("stages", ["unknown"])[0] if isinstance(run.get("stages"), list) else "unknown"),
                "status": run.get("status", "unknown"),
                "started_at": started_at,
                "completed_at": completed_at,
                "duration_seconds": run.get("duration_seconds"),
                "params": run.get("context", {}).get("params", {})
            })
        
        return {
            "doc_id": doc_id,
            "collection": collection_name,
            "runs": runs,
            "total_runs": len(runs),
            "timestamp": datetime.utcnow().isoformat() + "Z"
        }, 200
        
    except Exception as e:
        logger.warning(f"Could not fetch run history: {e}")
        return {
            "doc_id": doc_id,
            "collection": collection_name,
            "runs": [],
            "total_runs": 0,
            "message": "Run history not available",
            "timestamp": datetime.utcnow().isoformat() + "Z"
        }, 200

