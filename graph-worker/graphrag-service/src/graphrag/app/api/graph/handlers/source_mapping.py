"""
Source Mapping Handler - Entity to Video Mapping

Provides mapping between entities and their source videos.
Used by GraphDash to filter the knowledge graph by video source.

The mapping is derived from entity provenance data which contains
video_id for each entity's source chunks.
"""

import logging
import os
import sys
from typing import Dict, Any, Optional, List

# Add project root to Python path for imports
_project_root = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "..", ".."))
if _project_root not in sys.path:
    sys.path.insert(0, _project_root)

from src.infrastructure.database.mongodb import get_mongo_client
from src.domain.services.graphrag.indexes import get_graphrag_collections

logger = logging.getLogger(__name__)


def get_entity_video_mapping(
    db_name: str,
    entity_ids: Optional[List[str]] = None,
    video_ids: Optional[List[str]] = None,
) -> Dict[str, Any]:
    """
    Get mapping between entities and source videos.
    
    Uses the provenance field in entities which contains video_id directly,
    allowing efficient mapping without joining through video_chunks.
    
    Args:
        db_name: Database name containing the entities collection
        entity_ids: Optional list of entity IDs to filter to
        video_ids: Optional list of video IDs to filter results by
    
    Returns:
        Dictionary with:
        - mapping: Record<entity_id, video_ids[]>
        - total_entities: Count of entities in mapping
        - db_name: Database name
    """
    try:
        client = get_mongo_client()
        db = client[db_name]
        collections = get_graphrag_collections(db)
        entities_collection = collections["entities"]
        
        # Build aggregation pipeline to extract video_ids from provenance
        pipeline = []
        
        # Filter by entity_ids if provided
        if entity_ids:
            pipeline.append({
                "$match": {"entity_id": {"$in": entity_ids}}
            })
        
        # Project to extract video_ids from provenance array
        pipeline.append({
            "$project": {
                "entity_id": 1,
                "video_ids": {
                    "$map": {
                        "input": {"$ifNull": ["$provenance", []]},
                        "as": "p",
                        "in": "$$p.video_id"
                    }
                },
                "_id": 0
            }
        })
        
        # Remove duplicates and nulls from video_ids array
        pipeline.append({
            "$project": {
                "entity_id": 1,
                "video_ids": {
                    "$filter": {
                        "input": {"$setUnion": ["$video_ids", []]},  # Dedupe
                        "as": "vid",
                        "cond": {"$ne": ["$$vid", None]}  # Remove nulls
                    }
                }
            }
        })
        
        # Execute aggregation
        results = list(entities_collection.aggregate(pipeline))
        
        # Build mapping dict
        mapping = {}
        for doc in results:
            entity_id = doc.get("entity_id")
            video_ids_list = doc.get("video_ids", [])
            
            if entity_id and video_ids_list:
                # Filter by video_ids if provided
                if video_ids:
                    video_ids_set = set(video_ids)
                    filtered_videos = [v for v in video_ids_list if v in video_ids_set]
                    if filtered_videos:  # Only include if there are matching videos
                        mapping[entity_id] = filtered_videos
                else:
                    mapping[entity_id] = video_ids_list
        
        return {
            "mapping": mapping,
            "total_entities": len(mapping),
            "db_name": db_name,
        }
        
    except Exception as e:
        logger.exception(f"Error getting entity-video mapping for {db_name}")
        return {
            "error": str(e),
            "mapping": {},
            "total_entities": 0,
            "db_name": db_name,
        }


def get_video_entity_mapping(
    db_name: str,
    video_ids: List[str],
) -> Dict[str, Any]:
    """
    Get entities for specific videos (reverse mapping).
    
    Useful for finding which entities came from specific videos.
    
    Args:
        db_name: Database name
        video_ids: List of video IDs to find entities for
    
    Returns:
        Dictionary with:
        - mapping: Record<video_id, entity_ids[]>
        - total_videos: Count of videos with entities
        - total_entities: Count of unique entities
        - db_name: Database name
    """
    try:
        client = get_mongo_client()
        db = client[db_name]
        collections = get_graphrag_collections(db)
        entities_collection = collections["entities"]
        
        video_set = set(video_ids)
        
        # Find entities with provenance containing any of these video_ids
        pipeline = [
            {
                "$match": {
                    "provenance.video_id": {"$in": video_ids}
                }
            },
            {
                "$project": {
                    "entity_id": 1,
                    "video_ids": {
                        "$filter": {
                            "input": {
                                "$map": {
                                    "input": {"$ifNull": ["$provenance", []]},
                                    "as": "p",
                                    "in": "$$p.video_id"
                                }
                            },
                            "as": "vid",
                            "cond": {"$in": ["$$vid", video_ids]}
                        }
                    },
                    "_id": 0
                }
            }
        ]
        
        results = list(entities_collection.aggregate(pipeline))
        
        # Build reverse mapping: video_id -> entity_ids
        mapping: Dict[str, List[str]] = {vid: [] for vid in video_ids}
        all_entity_ids = set()
        
        for doc in results:
            entity_id = doc.get("entity_id")
            video_ids_list = doc.get("video_ids", [])
            
            if entity_id:
                all_entity_ids.add(entity_id)
                for vid in video_ids_list:
                    if vid in mapping:
                        mapping[vid].append(entity_id)
        
        return {
            "mapping": mapping,
            "total_videos": len([v for v in mapping.values() if v]),
            "total_entities": len(all_entity_ids),
            "db_name": db_name,
        }
        
    except Exception as e:
        logger.exception(f"Error getting video-entity mapping for {db_name}")
        return {
            "error": str(e),
            "mapping": {},
            "total_videos": 0,
            "total_entities": 0,
            "db_name": db_name,
        }

