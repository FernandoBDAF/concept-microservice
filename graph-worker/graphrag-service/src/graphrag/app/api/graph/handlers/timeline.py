"""
Video Timeline Handler - Get entities in chronological order

Returns entities from a video sorted by their first appearance timestamp,
enabling timeline/story visualization in the UI.
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
from ..constants import COLL_CHUNKS, DEFAULT_MAX_CONTEXTS

logger = logging.getLogger(__name__)


def get_video_timeline(
    db_name: str,
    video_id: str,
    include_contexts: bool = True,
    max_contexts: int = DEFAULT_MAX_CONTEXTS,
) -> Dict[str, Any]:
    """
    Get entities for a video ordered by first appearance.
    
    Uses MongoDB aggregation to join entity_mentions, video_chunks, and entities
    collections, then groups by entity_id to get first/last mention times.
    
    Args:
        db_name: Database name
        video_id: YouTube video ID
        include_contexts: Whether to include text snippets from chunks
        max_contexts: Maximum number of context snippets per entity
    
    Returns:
        Dictionary with:
        - video_id: Video ID
        - video_title: Video title
        - total_entities: Number of unique entities
        - timeline: List of entities ordered by first appearance
    """
    try:
        client = get_mongo_client()
        db = client[db_name]
        collections = get_graphrag_collections(db)
        
        entity_mentions_collection = collections["entity_mentions"]
        chunks_collection = db[COLL_CHUNKS]
        entities_collection = collections["entities"]
        
        # Build aggregation pipeline
        pipeline = [
            # Step 1: Filter entity_mentions by video_id
            {
                "$match": {
                    "video_id": video_id
                }
            },
            
            # Step 2: Join with video_chunks to get timestamps
            {
                "$lookup": {
                    "from": COLL_CHUNKS,
                    "localField": "chunk_id",
                    "foreignField": "chunk_id",
                    "as": "chunk"
                }
            },
            {
                "$unwind": {
                    "path": "$chunk",
                    "preserveNullAndEmptyArrays": False
                }
            },
            
            # Step 3: Join with entities to get entity details
            {
                "$lookup": {
                    "from": "entities",
                    "localField": "entity_id",
                    "foreignField": "entity_id",
                    "as": "entity"
                }
            },
            {
                "$unwind": {
                    "path": "$entity",
                    "preserveNullAndEmptyArrays": False
                }
            },
            
            # Step 4: Project relevant fields
            {
                "$project": {
                    "entity_id": 1,
                    "entity_name": "$entity.canonical_name",
                    "entity_type": "$entity.type",
                    "timestamp_start": "$chunk.timestamp_start",
                    "timestamp_end": "$chunk.timestamp_end",
                    "chunk_text": "$chunk.chunk_text",
                    "chunk_id": 1,
                    "confidence": 1
                }
            },
            
            # Step 5: Sort by timestamp_start
            {
                "$sort": {
                    "timestamp_start": 1
                }
            },
            
            # Step 6: Group by entity_id to aggregate mentions
            {
                "$group": {
                    "_id": "$entity_id",
                    "entity_name": {"$first": "$entity_name"},
                    "entity_type": {"$first": "$entity_type"},
                    "first_mention": {"$first": "$timestamp_start"},
                    "last_mention": {"$last": "$timestamp_start"},
                    "mention_count": {"$sum": 1},
                    "contexts": {
                        "$push": {
                            "timestamp": "$timestamp_start",
                            "chunk_id": "$chunk_id",
                            "text_snippet": {
                                "$cond": {
                                    "if": {"$gt": [{"$strLenCP": "$chunk_text"}, 200]},
                                    "then": {"$substr": ["$chunk_text", 0, 200]},
                                    "else": "$chunk_text"
                                }
                            },
                            "confidence": "$confidence"
                        }
                    }
                }
            },
            
            # Step 7: Project final fields
            {
                "$project": {
                    "entity_id": "$_id",
                    "entity_name": 1,
                    "entity_type": 1,
                    "first_mention": 1,
                    "last_mention": 1,
                    "mention_count": 1,
                    "contexts": 1
                }
            },
            
            # Step 8: Final sort by first_mention
            {
                "$sort": {
                    "first_mention": 1
                }
            }
        ]
        
        # Get video metadata from chunks collection (always check, even if no entities)
        video_metadata = None
        sample_chunk = chunks_collection.find_one({"video_id": video_id})
        if sample_chunk:
            video_metadata = {
                "video_title": sample_chunk.get("video_title", ""),
                "video_url": sample_chunk.get("video_url", ""),
                "channel_name": sample_chunk.get("channel_name", "")
            }
        else:
            # Video not found in chunks - log diagnostic info
            logger.warning(f"Video {video_id} not found in {COLL_CHUNKS} collection")
            # Check if any chunks exist for this video_id
            chunk_count = chunks_collection.count_documents({"video_id": video_id})
            logger.info(f"Found {chunk_count} chunks for video_id {video_id}")
        
        # Check if there are any entity_mentions for this video
        mention_count = entity_mentions_collection.count_documents({"video_id": video_id})
        logger.info(f"Found {mention_count} entity mentions for video_id {video_id}")
        
        if mention_count == 0:
            logger.warning(f"No entity mentions found for video {video_id}. Video may not have been processed through GraphRAG pipeline.")
        
        # Execute aggregation
        results = list(entity_mentions_collection.aggregate(pipeline))
        logger.info(f"Aggregation returned {len(results)} entities after joins")
        
        # Limit contexts per entity in Python (can't use variables in aggregation)
        if not include_contexts or max_contexts <= 0:
            for doc in results:
                doc["contexts"] = []
        elif max_contexts > 0:
            for doc in results:
                contexts = doc.get("contexts", [])
                if len(contexts) > max_contexts:
                    doc["contexts"] = contexts[:max_contexts]
        
        # Build timeline array
        timeline = []
        for doc in results:
            timeline_item = {
                "entity_id": doc.get("entity_id"),
                "entity_name": doc.get("entity_name") or "Unknown",
                "entity_type": doc.get("entity_type") or "OTHER",
                "first_mention": doc.get("first_mention") or "00:00:00",
                "last_mention": doc.get("last_mention") or "00:00:00",
                "mention_count": doc.get("mention_count", 0),
            }
            
            # Add contexts if requested
            if include_contexts:
                contexts = doc.get("contexts", [])
                timeline_item["contexts"] = [
                    {
                        "timestamp": ctx.get("timestamp", "00:00:00"),
                        "chunk_id": ctx.get("chunk_id"),
                        "text_snippet": ctx.get("text_snippet", ""),
                        "confidence": ctx.get("confidence", 0.0)
                    }
                    for ctx in contexts
                ]
            
            timeline.append(timeline_item)
        
        response = {
            "video_id": video_id,
            "video_title": video_metadata.get("video_title", "") if video_metadata else "",
            "video_url": video_metadata.get("video_url", "") if video_metadata else "",
            "channel_name": video_metadata.get("channel_name", "") if video_metadata else "",
            "total_entities": len(timeline),
            "timeline": timeline,
        }
        
        # Add diagnostic info if no entities found
        if len(timeline) == 0:
            response["diagnostics"] = {
                "chunks_found": chunks_collection.count_documents({"video_id": video_id}) > 0,
                "mentions_found": mention_count > 0,
                "message": "No entities found. Ensure video has been processed through GraphRAG pipeline (extraction → resolution → construction)."
            }
        
        return response
        
    except Exception as e:
        logger.exception(f"Error getting video timeline for {video_id} in {db_name}")
        return {
            "error": str(e),
            "video_id": video_id,
            "video_title": "",
            "total_entities": 0,
            "timeline": [],
        }

