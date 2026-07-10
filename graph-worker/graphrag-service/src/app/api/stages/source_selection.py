"""
Source Selection API Module

Provides endpoints for filtering and grouping raw_videos collection
before pipeline execution. This enables users to curate which videos
will be processed by the pipeline.

Endpoints:
- GET /source-selection/channels/{db} - Channel statistics
- POST /source-selection/preview - Filter preview
- GET /source-selection/filters/{db} - List saved filters
- GET /source-selection/filters/{db}/{id} - Get filter details
- POST /source-selection/filters/{db} - Save new filter
- PUT /source-selection/filters/{db}/{id} - Update filter
- DELETE /source-selection/filters/{db}/{id} - Delete filter
- POST /source-selection/filters/{db}/{id}/duplicate - Clone filter
- POST /source-selection/resolve - Resolve filter to video IDs

Reference: SOURCE_SELECTION_TECHNICAL_STUDY.md
"""

import logging
import re
from datetime import datetime
from typing import Any, Dict, List, Optional, Tuple

from bson import ObjectId
from bson.errors import InvalidId

# Import shared database utilities
from .db import get_mongo_client, SYSTEM_DATABASES

# Import collection constants from core config
from src.core.config.paths import (
    COLL_RAW_VIDEOS,
    COLL_INPUT_FILTERS,
    CONSTANT_DB_NAME,
    get_db_for_collection,
)

logger = logging.getLogger(__name__)

# =============================================================================
# Constants
# =============================================================================

# Maximum videos to return in a resolve operation
MAX_RESOLVE_LIMIT = 10000

# Default sample size for preview
DEFAULT_SAMPLE_LIMIT = 5
MAX_SAMPLE_LIMIT = 20

# Supported sort fields for raw_videos
SUPPORTED_SORT_FIELDS = {
    "published_at",
    "stats.viewCount",
    "stats.likeCount",
    "engagement_score",
    "duration_seconds",
    "created_at",
}

# Default sort configuration
DEFAULT_SORT = {"field": "published_at", "order": "desc"}

# Warning thresholds
WARNING_LOW_VIDEO_COUNT = 5
WARNING_HIGH_VIDEO_COUNT = 500
WARNING_LOW_TRANSCRIPT_COVERAGE = 80  # percent


# =============================================================================
# Validation Helpers
# =============================================================================


def _validate_filter_definition(filter_def: Dict[str, Any]) -> List[str]:
    """
    Validate filter definition structure and values.
    
    Returns list of validation errors (empty if valid).
    """
    errors = []
    
    if not isinstance(filter_def, dict):
        return ["Filter definition must be an object"]
    
    # Validate channels
    if "channels" in filter_def:
        channels = filter_def["channels"]
        if not isinstance(channels, list):
            errors.append("'channels' must be an array of channel IDs")
        elif not all(isinstance(c, str) for c in channels):
            errors.append("All channel IDs must be strings")
    
    # Validate date_range
    if "date_range" in filter_def:
        dr = filter_def["date_range"]
        if not isinstance(dr, dict):
            errors.append("'date_range' must be an object with 'start' and/or 'end'")
        else:
            for field in ["start", "end"]:
                if field in dr and dr[field]:
                    # Try to parse as ISO date
                    try:
                        if isinstance(dr[field], str):
                            # Basic ISO date validation
                            datetime.fromisoformat(dr[field].replace("Z", "+00:00"))
                    except ValueError:
                        errors.append(f"'date_range.{field}' must be a valid ISO date string")
    
    # Validate engagement
    if "engagement" in filter_def:
        eng = filter_def["engagement"]
        if not isinstance(eng, dict):
            errors.append("'engagement' must be an object")
        else:
            for field in ["min_views", "max_views"]:
                if field in eng and eng[field] is not None:
                    if not isinstance(eng[field], (int, float)) or eng[field] < 0:
                        errors.append(f"'engagement.{field}' must be a non-negative number")
            if "min_engagement_score" in eng and eng["min_engagement_score"] is not None:
                score = eng["min_engagement_score"]
                if not isinstance(score, (int, float)) or score < 0 or score > 1:
                    errors.append("'engagement.min_engagement_score' must be between 0 and 1")
    
    # Validate content
    if "content" in filter_def:
        content = filter_def["content"]
        if not isinstance(content, dict):
            errors.append("'content' must be an object")
        else:
            for field in ["keywords_any", "keywords_all"]:
                if field in content and content[field]:
                    if not isinstance(content[field], list):
                        errors.append(f"'content.{field}' must be an array of strings")
                    elif not all(isinstance(k, str) for k in content[field]):
                        errors.append(f"All items in 'content.{field}' must be strings")
            if "has_transcript" in content:
                if not isinstance(content["has_transcript"], bool):
                    errors.append("'content.has_transcript' must be a boolean")
    
    # Validate duration
    if "duration" in filter_def:
        dur = filter_def["duration"]
        if not isinstance(dur, dict):
            errors.append("'duration' must be an object")
        else:
            for field in ["min_seconds", "max_seconds"]:
                if field in dur and dur[field] is not None:
                    if not isinstance(dur[field], (int, float)) or dur[field] < 0:
                        errors.append(f"'duration.{field}' must be a non-negative number")
    
    # Validate sort
    if "sort" in filter_def:
        sort = filter_def["sort"]
        if not isinstance(sort, dict):
            errors.append("'sort' must be an object with 'field' and 'order'")
        else:
            if "field" in sort and sort["field"] not in SUPPORTED_SORT_FIELDS:
                errors.append(
                    f"'sort.field' must be one of: {', '.join(sorted(SUPPORTED_SORT_FIELDS))}"
                )
            if "order" in sort and sort["order"] not in ["asc", "desc"]:
                errors.append("'sort.order' must be 'asc' or 'desc'")
    
    # Validate limit
    if "limit" in filter_def:
        limit = filter_def["limit"]
        if not isinstance(limit, int) or limit < 1:
            errors.append("'limit' must be a positive integer")
        elif limit > MAX_RESOLVE_LIMIT:
            errors.append(f"'limit' cannot exceed {MAX_RESOLVE_LIMIT}")
    
    return errors


def _validate_filter_name(name: str) -> List[str]:
    """Validate filter name."""
    errors = []
    if not name or not name.strip():
        errors.append("Filter name is required")
    elif len(name.strip()) > 100:
        errors.append("Filter name cannot exceed 100 characters")
    elif not re.match(r'^[\w\s\-_.]+$', name.strip()):
        errors.append("Filter name can only contain letters, numbers, spaces, hyphens, underscores, and periods")
    return errors


# =============================================================================
# MongoDB Query Builder
# =============================================================================


def _filter_to_mongo_query(filter_def: Dict[str, Any]) -> Dict[str, Any]:
    """
    Convert filter definition to MongoDB query.
    
    Handles:
    - channels: list of channel_ids → $in query
    - date_range: {start, end} → $gte/$lte on published_at
    - engagement: {min_views, max_views, min_engagement_score} → numeric comparisons
    - content: {keywords_any, keywords_all, has_transcript} → array/existence queries
    - duration: {min_seconds, max_seconds} → numeric comparisons
    - playlist_ids: list → $in query
    
    Args:
        filter_def: Filter definition dictionary
    
    Returns:
        MongoDB query dictionary
    """
    query: Dict[str, Any] = {}
    
    # Channel filter
    if channels := filter_def.get("channels"):
        if isinstance(channels, list) and len(channels) > 0:
            query["channel_id"] = {"$in": channels}
    
    # Date range
    if date_range := filter_def.get("date_range"):
        date_query: Dict[str, Any] = {}
        if start := date_range.get("start"):
            date_query["$gte"] = start
        if end := date_range.get("end"):
            date_query["$lte"] = end
        if date_query:
            query["published_at"] = date_query
    
    # Engagement
    if engagement := filter_def.get("engagement"):
        if min_views := engagement.get("min_views"):
            if "stats.viewCount" not in query:
                query["stats.viewCount"] = {}
            query["stats.viewCount"]["$gte"] = int(min_views)
        if max_views := engagement.get("max_views"):
            if "stats.viewCount" not in query:
                query["stats.viewCount"] = {}
            query["stats.viewCount"]["$lte"] = int(max_views)
        if min_eng := engagement.get("min_engagement_score"):
            query["engagement_score"] = {"$gte": float(min_eng)}
    
    # Content
    if content := filter_def.get("content"):
        if keywords_any := content.get("keywords_any"):
            if isinstance(keywords_any, list) and len(keywords_any) > 0:
                # Case-insensitive regex match on title or keywords array
                query["$or"] = [
                    {"keywords": {"$in": [k.lower() for k in keywords_any]}},
                    {"title": {"$regex": "|".join(re.escape(k) for k in keywords_any), "$options": "i"}},
                ]
        if keywords_all := content.get("keywords_all"):
            if isinstance(keywords_all, list) and len(keywords_all) > 0:
                # All keywords must be present
                query["keywords"] = {"$all": [k.lower() for k in keywords_all]}
        if content.get("has_transcript") is True:
            query["$and"] = query.get("$and", []) + [
                {"transcript_raw": {"$exists": True}},
                {"transcript_raw": {"$ne": None}},
                {"transcript_raw": {"$ne": ""}},
            ]
        elif content.get("has_transcript") is False:
            query["$or"] = query.get("$or", []) + [
                {"transcript_raw": {"$exists": False}},
                {"transcript_raw": None},
                {"transcript_raw": ""},
            ]
    
    # Duration
    if duration := filter_def.get("duration"):
        dur_query: Dict[str, Any] = {}
        if min_sec := duration.get("min_seconds"):
            dur_query["$gte"] = int(min_sec)
        if max_sec := duration.get("max_seconds"):
            dur_query["$lte"] = int(max_sec)
        if dur_query:
            query["duration_seconds"] = dur_query
    
    # Playlist
    if playlist_ids := filter_def.get("playlist_ids"):
        if isinstance(playlist_ids, list) and len(playlist_ids) > 0:
            query["playlist_id"] = {"$in": playlist_ids}
    
    return query


def _get_sort_spec(filter_def: Dict[str, Any]) -> Tuple[str, int]:
    """
    Extract sort specification from filter definition.
    
    Returns:
        Tuple of (field_name, direction) where direction is 1 (asc) or -1 (desc)
    """
    sort_config = filter_def.get("sort", DEFAULT_SORT)
    field = sort_config.get("field", DEFAULT_SORT["field"])
    order = sort_config.get("order", DEFAULT_SORT["order"])
    
    # Validate field
    if field not in SUPPORTED_SORT_FIELDS:
        field = DEFAULT_SORT["field"]
    
    direction = -1 if order == "desc" else 1
    return field, direction


# =============================================================================
# Channel Statistics
# =============================================================================


def get_channels(db_name: str) -> Tuple[Dict[str, Any], int]:
    """
    Get channel statistics from raw_videos collection.
    
    Note: raw_videos is a constant collection stored in system_data database,
    not in the pipeline-specific database. The db_name parameter is kept for
    API compatibility but raw_videos always comes from CONSTANT_DB_NAME.
    
    Aggregates data for each channel including:
    - Video count
    - Total views and likes
    - Average engagement score
    - Date range of videos
    - Transcript coverage percentage
    
    Args:
        db_name: Database name (for API compatibility, raw_videos uses system_data)
    
    Returns:
        Tuple of (response_dict, status_code)
    """
    try:
        client = get_mongo_client()
        
        # raw_videos is always in the constant database (system_data)
        source_db_name = get_db_for_collection(COLL_RAW_VIDEOS, db_name)
        
        # Check if database exists
        if source_db_name not in client.list_database_names():
            return {"error": f"Database '{source_db_name}' not found"}, 404
        
        db = client[source_db_name]
        
        # Check if collection exists
        if COLL_RAW_VIDEOS not in db.list_collection_names():
            return {
                "error": f"Collection '{COLL_RAW_VIDEOS}' not found in database '{source_db_name}'"
            }, 404
        
        coll = db[COLL_RAW_VIDEOS]
        
        # Aggregate channel statistics
        pipeline = [
            {
                "$group": {
                    "_id": "$channel_id",
                    "channel_title": {"$first": "$channel_title"},
                    "video_count": {"$sum": 1},
                    "total_views": {"$sum": {"$ifNull": ["$stats.viewCount", 0]}},
                    "total_likes": {"$sum": {"$ifNull": ["$stats.likeCount", 0]}},
                    "avg_engagement": {"$avg": {"$ifNull": ["$engagement_score", 0]}},
                    "avg_duration": {"$avg": {"$ifNull": ["$duration_seconds", 0]}},
                    "latest_video": {"$max": "$published_at"},
                    "earliest_video": {"$min": "$published_at"},
                    "has_transcript_count": {
                        "$sum": {
                            "$cond": [
                                {"$and": [
                                    {"$ne": ["$transcript_raw", None]},
                                    {"$ne": ["$transcript_raw", ""]},
                                ]},
                                1,
                                0
                            ]
                        }
                    }
                }
            },
            {
                "$match": {
                    "_id": {"$ne": None}  # Filter out null channel_ids
                }
            },
            {
                "$sort": {"video_count": -1}
            },
            {
                "$project": {
                    "channel_id": "$_id",
                    "channel_title": {"$ifNull": ["$channel_title", "Unknown Channel"]},
                    "video_count": 1,
                    "total_views": 1,
                    "total_likes": 1,
                    "avg_engagement": {"$round": [{"$ifNull": ["$avg_engagement", 0]}, 4]},
                    "avg_duration_minutes": {
                        "$round": [
                            {"$divide": [{"$ifNull": ["$avg_duration", 0]}, 60]},
                            1
                        ]
                    },
                    "date_range": {
                        "earliest": "$earliest_video",
                        "latest": "$latest_video"
                    },
                    "transcript_coverage": {
                        "$round": [
                            {"$multiply": [
                                {"$cond": [
                                    {"$eq": ["$video_count", 0]},
                                    0,
                                    {"$divide": ["$has_transcript_count", "$video_count"]}
                                ]},
                                100
                            ]},
                            1
                        ]
                    },
                    "_id": 0
                }
            }
        ]
        
        channels = list(coll.aggregate(pipeline))
        
        # Get total stats
        total_videos = sum(c["video_count"] for c in channels)
        total_channels = len(channels)
        
        return {
            "db_name": db_name,
            "source_db": source_db_name,  # Actual database where raw_videos lives
            "collection": COLL_RAW_VIDEOS,
            "channels": channels,
            "summary": {
                "total_videos": total_videos,
                "total_channels": total_channels
            },
            "timestamp": datetime.utcnow().isoformat() + "Z"
        }, 200
        
    except Exception as e:
        logger.exception(f"Error getting channels from {source_db_name}")
        return {"error": str(e)}, 500


# =============================================================================
# Videos for Channel
# =============================================================================


def get_videos_for_channel(
    db_name: str,
    channel_id: str,
    limit: int = 100,
    offset: int = 0,
    sort_by: str = "published_at",
    order: str = "desc"
) -> Tuple[Dict[str, Any], int]:
    """
    Get videos for a specific channel from raw_videos collection.
    
    This endpoint is used by GraphDash to fetch videos when a channel
    is expanded in the SourcePanel for filtering the knowledge graph.
    
    Args:
        db_name: Database name (for API compatibility, raw_videos uses system_data)
        channel_id: YouTube channel ID
        limit: Maximum number of videos to return (default 100)
        offset: Pagination offset (default 0)
        sort_by: Sort field (default "published_at")
        order: Sort order "asc" or "desc" (default "desc")
    
    Returns:
        Tuple of (response_dict, status_code)
    """
    try:
        client = get_mongo_client()
        
        # raw_videos is always in the constant database (system_data)
        source_db_name = get_db_for_collection(COLL_RAW_VIDEOS, db_name)
        
        # Check if database exists
        if source_db_name not in client.list_database_names():
            return {"error": f"Database '{source_db_name}' not found"}, 404
        
        db = client[source_db_name]
        
        # Check if collection exists
        if COLL_RAW_VIDEOS not in db.list_collection_names():
            return {
                "error": f"Collection '{COLL_RAW_VIDEOS}' not found in database '{source_db_name}'"
            }, 404
        
        coll = db[COLL_RAW_VIDEOS]
        
        # Query videos for channel
        query = {"channel_id": channel_id}
        
        # Get total count
        total = coll.count_documents(query)
        
        if total == 0:
            return {
                "channel_id": channel_id,
                "channel_title": None,
                "videos": [],
                "total": 0,
                "limit": limit,
                "offset": offset,
                "has_more": False,
                "timestamp": datetime.utcnow().isoformat() + "Z"
            }, 200
        
        # Get channel title from first document
        first_doc = coll.find_one(query, {"channel_title": 1})
        channel_title = first_doc.get("channel_title", "Unknown Channel") if first_doc else "Unknown Channel"
        
        # Validate sort field
        valid_sort_fields = {"published_at", "title", "duration_seconds", "stats.viewCount", "engagement_score"}
        if sort_by not in valid_sort_fields:
            sort_by = "published_at"
        
        # Sort direction
        sort_dir = -1 if order == "desc" else 1
        
        # Get videos with pagination
        cursor = coll.find(
            query,
            {
                "video_id": 1,
                "title": 1,
                "published_at": 1,
                "duration_seconds": 1,
                "stats.viewCount": 1,
                "engagement_score": 1,
                "thumbnail_url": 1,
                "transcript_raw": 1,  # To check if has transcript
                "_id": 0
            }
        ).sort(sort_by, sort_dir).skip(offset).limit(limit)
        
        videos = []
        for doc in cursor:
            videos.append({
                "video_id": doc.get("video_id"),
                "title": doc.get("title", "Untitled"),
                "published_at": doc.get("published_at"),
                "duration_seconds": doc.get("duration_seconds", 0),
                "views": doc.get("stats", {}).get("viewCount", 0) if doc.get("stats") else 0,
                "engagement_score": round(doc.get("engagement_score", 0) or 0, 4),
                "thumbnail_url": doc.get("thumbnail_url"),
                "has_transcript": bool((doc.get("transcript_raw") or "").strip())
            })
        
        return {
            "channel_id": channel_id,
            "channel_title": channel_title,
            "videos": videos,
            "total": total,
            "limit": limit,
            "offset": offset,
            "has_more": (offset + limit) < total,
            "timestamp": datetime.utcnow().isoformat() + "Z"
        }, 200
        
    except Exception as e:
        logger.exception(f"Error getting videos for channel {channel_id}")
        return {"error": str(e)}, 500


# =============================================================================
# Filter Preview
# =============================================================================


def preview_filter(
    db_name: str,
    filter_def: Dict[str, Any],
    sample_limit: int = DEFAULT_SAMPLE_LIMIT
) -> Tuple[Dict[str, Any], int]:
    """
    Preview filter results without saving.
    
    Provides statistics about videos matching the filter criteria
    without actually fetching all the data.
    
    Args:
        db_name: Database name
        filter_def: Filter definition dictionary
        sample_limit: Number of sample videos to return (default 5, max 20)
    
    Returns:
        Preview statistics and sample videos
    """
    try:
        # Validate filter definition
        if filter_def:
            validation_errors = _validate_filter_definition(filter_def)
            if validation_errors:
                return {
                    "error": "Invalid filter definition",
                    "validation_errors": validation_errors
                }, 400
        
        # Validate sample limit
        sample_limit = min(max(1, sample_limit), MAX_SAMPLE_LIMIT)
        
        client = get_mongo_client()
        
        # raw_videos is always in the constant database (system_data)
        source_db_name = get_db_for_collection(COLL_RAW_VIDEOS, db_name)
        
        # Check if database exists
        if source_db_name not in client.list_database_names():
            return {"error": f"Database '{source_db_name}' not found"}, 404
        
        db = client[source_db_name]
        coll = db[COLL_RAW_VIDEOS]
        
        # Build MongoDB query from filter
        query = _filter_to_mongo_query(filter_def or {})
        
        logger.debug(f"Preview filter query for {db_name}: {query}")
        
        # Count total matching
        total_matching = coll.count_documents(query)
        
        if total_matching == 0:
            return {
                "total_matching": 0,
                "channels": [],
                "date_range": None,
                "statistics": {
                    "total_duration_minutes": 0,
                    "avg_engagement": 0,
                    "avg_views": 0,
                    "transcript_coverage": 0
                },
                "sample_videos": [],
                "warnings": ["No videos match the current filter criteria"],
                "query": query,
                "timestamp": datetime.utcnow().isoformat() + "Z"
            }, 200
        
        # Get channel breakdown for matching videos
        channel_pipeline = [
            {"$match": query},
            {
                "$group": {
                    "_id": "$channel_id",
                    "title": {"$first": "$channel_title"},
                    "count": {"$sum": 1}
                }
            },
            {"$match": {"_id": {"$ne": None}}},
            {"$sort": {"count": -1}}
        ]
        channels = list(coll.aggregate(channel_pipeline))
        channels = [
            {"id": c["_id"], "title": c.get("title") or "Unknown", "count": c["count"]}
            for c in channels
        ]
        
        # Get statistics
        stats_pipeline = [
            {"$match": query},
            {
                "$group": {
                    "_id": None,
                    "earliest": {"$min": "$published_at"},
                    "latest": {"$max": "$published_at"},
                    "total_duration": {"$sum": {"$ifNull": ["$duration_seconds", 0]}},
                    "avg_engagement": {"$avg": {"$ifNull": ["$engagement_score", 0]}},
                    "avg_views": {"$avg": {"$ifNull": ["$stats.viewCount", 0]}},
                    "with_transcript": {
                        "$sum": {"$cond": [
                            {"$and": [
                                {"$ne": ["$transcript_raw", None]},
                                {"$ne": ["$transcript_raw", ""]}
                            ]},
                            1,
                            0
                        ]}
                    }
                }
            }
        ]
        stats_result = list(coll.aggregate(stats_pipeline))
        
        date_range = None
        total_duration_minutes = 0
        avg_engagement = 0
        avg_views = 0
        transcript_coverage = 0
        
        if stats_result:
            stats = stats_result[0]
            date_range = {
                "earliest": stats.get("earliest"),
                "latest": stats.get("latest")
            }
            total_duration_minutes = round((stats.get("total_duration", 0) or 0) / 60, 1)
            avg_engagement = round(stats.get("avg_engagement", 0) or 0, 4)
            avg_views = round(stats.get("avg_views", 0) or 0)
            with_transcript = stats.get("with_transcript", 0)
            transcript_coverage = round((with_transcript / total_matching) * 100, 1)
        
        # Get sample videos
        sort_field, sort_direction = _get_sort_spec(filter_def or {})
        
        sample_cursor = coll.find(
            query,
            {
                "video_id": 1,
                "title": 1,
                "channel_title": 1,
                "channel_id": 1,
                "published_at": 1,
                "thumbnail_url": 1,
                "stats": 1,
                "duration_seconds": 1,
                "engagement_score": 1,
                "_id": 0
            }
        ).sort(sort_field, sort_direction).limit(sample_limit)
        
        sample_videos = list(sample_cursor)
        
        # Generate warnings
        warnings = []
        if total_matching < WARNING_LOW_VIDEO_COUNT:
            warnings.append(f"Only {total_matching} videos match - consider broadening filters")
        if total_matching > WARNING_HIGH_VIDEO_COUNT:
            warnings.append(f"Large selection ({total_matching} videos) - processing may take a while")
        if transcript_coverage < WARNING_LOW_TRANSCRIPT_COVERAGE:
            warnings.append(f"Only {transcript_coverage}% of selected videos have transcripts")
        
        # Apply limit warning
        limit = (filter_def or {}).get("limit")
        if limit and total_matching > limit:
            warnings.append(f"Filter has limit of {limit}, but {total_matching} videos match")
        
        return {
            "total_matching": total_matching,
            "channels": channels,
            "date_range": date_range,
            "statistics": {
                "total_duration_minutes": total_duration_minutes,
                "avg_engagement": avg_engagement,
                "avg_views": avg_views,
                "transcript_coverage": transcript_coverage
            },
            "sample_videos": sample_videos,
            "warnings": warnings,
            "query": query,  # Include for debugging
            "timestamp": datetime.utcnow().isoformat() + "Z"
        }, 200
        
    except Exception as e:
        logger.exception(f"Error previewing filter for {db_name}")
        return {"error": str(e)}, 500


# =============================================================================
# Saved Filters CRUD
# =============================================================================


def list_saved_filters(db_name: str) -> Tuple[Dict[str, Any], int]:
    """
    List all saved filter sets for a database.
    
    Note: pipeline_input_filters is a constant collection stored in system_data.
    
    Args:
        db_name: Database name (for API compatibility)
    
    Returns:
        List of saved filters (without full filter_definition for performance)
    """
    try:
        client = get_mongo_client()
        # pipeline_input_filters is in the constant database
        filters_db_name = get_db_for_collection(COLL_INPUT_FILTERS, db_name)
        db = client[filters_db_name]
        coll = db[COLL_INPUT_FILTERS]
        
        filters = list(coll.find(
            {},
            {
                "name": 1,
                "description": 1,
                "created_at": 1,
                "updated_at": 1,
                "last_used_at": 1,
                "use_count": 1
            }
        ).sort("updated_at", -1))
        
        # Serialize ObjectIds
        for f in filters:
            f["id"] = str(f.pop("_id"))
        
        return {
            "db_name": db_name,
            "filters": filters,
            "count": len(filters),
            "timestamp": datetime.utcnow().isoformat() + "Z"
        }, 200
        
    except Exception as e:
        logger.exception(f"Error listing filters for {db_name}")
        return {"error": str(e)}, 500


def get_saved_filter(db_name: str, filter_id: str) -> Tuple[Dict[str, Any], int]:
    """
    Get a specific saved filter by ID including full filter_definition.
    
    Note: pipeline_input_filters is a constant collection stored in system_data.
    
    Args:
        db_name: Database name (for API compatibility)
        filter_id: Filter ObjectId as string
    
    Returns:
        Complete filter document
    """
    try:
        client = get_mongo_client()
        # pipeline_input_filters is in the constant database
        filters_db_name = get_db_for_collection(COLL_INPUT_FILTERS, db_name)
        db = client[filters_db_name]
        coll = db[COLL_INPUT_FILTERS]
        
        try:
            obj_id = ObjectId(filter_id)
        except InvalidId:
            return {"error": "Invalid filter ID format"}, 400
        
        filter_doc = coll.find_one({"_id": obj_id})
        
        if not filter_doc:
            return {"error": "Filter not found"}, 404
        
        filter_doc["id"] = str(filter_doc.pop("_id"))
        
        return {
            "db_name": db_name,
            "filter": filter_doc
        }, 200
        
    except Exception as e:
        logger.exception(f"Error getting filter {filter_id}")
        return {"error": str(e)}, 500


def save_filter(
    db_name: str,
    name: str,
    filter_definition: Dict[str, Any],
    description: Optional[str] = None
) -> Tuple[Dict[str, Any], int]:
    """
    Save a new filter set.
    
    Note: pipeline_input_filters is a constant collection stored in system_data.
    
    Args:
        db_name: Database name (for API compatibility)
        name: Human-readable filter name
        filter_definition: Filter criteria
        description: Optional description
    
    Returns:
        Created filter document
    """
    try:
        # Validate name
        name_errors = _validate_filter_name(name or "")
        if name_errors:
            return {"error": name_errors[0], "validation_errors": name_errors}, 400
        
        # Validate filter definition
        if not filter_definition:
            return {"error": "Filter definition is required"}, 400
        
        filter_errors = _validate_filter_definition(filter_definition)
        if filter_errors:
            return {
                "error": "Invalid filter definition",
                "validation_errors": filter_errors
            }, 400
        
        client = get_mongo_client()
        # pipeline_input_filters is in the constant database
        filters_db_name = get_db_for_collection(COLL_INPUT_FILTERS, db_name)
        db = client[filters_db_name]
        coll = db[COLL_INPUT_FILTERS]
        
        # Check for duplicate name
        existing = coll.find_one({"name": name.strip()})
        if existing:
            return {"error": f"Filter with name '{name.strip()}' already exists"}, 409
        
        now = datetime.utcnow().isoformat() + "Z"
        filter_doc = {
            "name": name.strip(),
            "description": description.strip() if description else None,
            "filter_definition": filter_definition,
            "created_at": now,
            "updated_at": now,
            "last_used_at": None,
            "use_count": 0
        }
        
        result = coll.insert_one(filter_doc)
        filter_doc["id"] = str(result.inserted_id)
        filter_doc.pop("_id", None)
        
        logger.info(f"Saved filter '{name.strip()}' with id {filter_doc['id']} in {db_name}")
        
        return {
            "db_name": db_name,
            "filter": filter_doc,
            "message": "Filter saved successfully"
        }, 201
        
    except Exception as e:
        logger.exception(f"Error saving filter for {db_name}")
        return {"error": str(e)}, 500


def update_filter(
    db_name: str,
    filter_id: str,
    updates: Dict[str, Any]
) -> Tuple[Dict[str, Any], int]:
    """
    Update an existing filter.
    
    Note: pipeline_input_filters is a constant collection stored in system_data.
    
    Args:
        db_name: Database name (for API compatibility)
        filter_id: Filter ObjectId as string
        updates: Fields to update (name, description, filter_definition)
    
    Returns:
        Updated filter document
    """
    try:
        client = get_mongo_client()
        # pipeline_input_filters is in the constant database
        filters_db_name = get_db_for_collection(COLL_INPUT_FILTERS, db_name)
        db = client[filters_db_name]
        coll = db[COLL_INPUT_FILTERS]
        
        try:
            obj_id = ObjectId(filter_id)
        except InvalidId:
            return {"error": "Invalid filter ID format"}, 400
        
        # Only allow certain fields to be updated
        allowed_fields = {"name", "description", "filter_definition"}
        update_doc = {k: v for k, v in updates.items() if k in allowed_fields}
        
        if not update_doc:
            return {"error": "No valid fields to update"}, 400
        
        # Validate name if provided
        if "name" in update_doc:
            name_errors = _validate_filter_name(update_doc["name"] or "")
            if name_errors:
                return {"error": name_errors[0], "validation_errors": name_errors}, 400
            update_doc["name"] = update_doc["name"].strip()
            
            # Check for name conflict
            existing = coll.find_one({
                "name": update_doc["name"],
                "_id": {"$ne": obj_id}
            })
            if existing:
                return {"error": f"Filter with name '{update_doc['name']}' already exists"}, 409
        
        # Validate filter definition if provided
        if "filter_definition" in update_doc:
            filter_errors = _validate_filter_definition(update_doc["filter_definition"])
            if filter_errors:
                return {
                    "error": "Invalid filter definition",
                    "validation_errors": filter_errors
                }, 400
        
        # Clean description
        if "description" in update_doc and update_doc["description"]:
            update_doc["description"] = update_doc["description"].strip()
        
        update_doc["updated_at"] = datetime.utcnow().isoformat() + "Z"
        
        result = coll.find_one_and_update(
            {"_id": obj_id},
            {"$set": update_doc},
            return_document=True
        )
        
        if not result:
            return {"error": "Filter not found"}, 404
        
        result["id"] = str(result.pop("_id"))
        
        logger.info(f"Updated filter {filter_id} in {db_name}")
        
        return {
            "db_name": db_name,
            "filter": result,
            "message": "Filter updated successfully"
        }, 200
        
    except Exception as e:
        logger.exception(f"Error updating filter {filter_id}")
        return {"error": str(e)}, 500


def delete_filter(db_name: str, filter_id: str) -> Tuple[Dict[str, Any], int]:
    """
    Delete a saved filter.
    
    Note: pipeline_input_filters is a constant collection stored in system_data.
    
    Args:
        db_name: Database name (for API compatibility)
        filter_id: Filter ObjectId as string
    
    Returns:
        Success message
    """
    try:
        client = get_mongo_client()
        # pipeline_input_filters is in the constant database
        filters_db_name = get_db_for_collection(COLL_INPUT_FILTERS, db_name)
        db = client[filters_db_name]
        coll = db[COLL_INPUT_FILTERS]
        
        try:
            obj_id = ObjectId(filter_id)
        except InvalidId:
            return {"error": "Invalid filter ID format"}, 400
        
        result = coll.delete_one({"_id": obj_id})
        
        if result.deleted_count == 0:
            return {"error": "Filter not found"}, 404
        
        logger.info(f"Deleted filter {filter_id} from {db_name}")
        
        return {
            "db_name": db_name,
            "filter_id": filter_id,
            "message": "Filter deleted successfully"
        }, 200
        
    except Exception as e:
        logger.exception(f"Error deleting filter {filter_id}")
        return {"error": str(e)}, 500


def duplicate_filter(db_name: str, filter_id: str, new_name: str) -> Tuple[Dict[str, Any], int]:
    """
    Create a copy of an existing filter.
    
    Args:
        db_name: Database name
        filter_id: Source filter ObjectId as string
        new_name: Name for the copy
    
    Returns:
        Created filter document
    """
    try:
        # Validate new name
        name_errors = _validate_filter_name(new_name or "")
        if name_errors:
            return {"error": name_errors[0], "validation_errors": name_errors}, 400
        
        # Get original
        result, status = get_saved_filter(db_name, filter_id)
        if status != 200:
            return result, status
        
        original = result["filter"]
        
        # Save as new
        return save_filter(
            db_name=db_name,
            name=new_name.strip(),
            filter_definition=original["filter_definition"],
            description=f"Copy of: {original.get('description') or original['name']}"
        )
        
    except Exception as e:
        logger.exception(f"Error duplicating filter {filter_id}")
        return {"error": str(e)}, 500


# =============================================================================
# Video ID Resolution (for Pipeline Execution)
# =============================================================================


def resolve_filter_to_video_ids(
    db_name: str,
    filter_id: Optional[str] = None,
    filter_definition: Optional[Dict[str, Any]] = None
) -> Tuple[Dict[str, Any], int]:
    """
    Resolve a filter to a list of video IDs for pipeline execution.
    
    This is called by the execution module to get the actual video IDs
    that should be processed by the pipeline.
    
    Args:
        db_name: Database name
        filter_id: ID of saved filter (mutually exclusive with filter_definition)
        filter_definition: Ad-hoc filter definition (mutually exclusive with filter_id)
    
    Returns:
        List of video_ids matching the filter
    """
    try:
        if not db_name:
            return {"error": "db_name is required"}, 400
        
        client = get_mongo_client()
        
        # raw_videos is always in the constant database (system_data)
        source_db_name = get_db_for_collection(COLL_RAW_VIDEOS, db_name)
        source_db = client[source_db_name]
        
        # pipeline_input_filters may be in system_data or pipeline db depending on config
        filters_db_name = get_db_for_collection(COLL_INPUT_FILTERS, db_name)
        filters_db = client[filters_db_name]
        
        # Get filter definition
        if filter_id:
            result, status = get_saved_filter(db_name, filter_id)
            if status != 200:
                return result, status
            filter_def = result["filter"]["filter_definition"]
            
            # Update usage statistics
            filters_coll = filters_db[COLL_INPUT_FILTERS]
            try:
                filters_coll.update_one(
                    {"_id": ObjectId(filter_id)},
                    {
                        "$set": {"last_used_at": datetime.utcnow().isoformat() + "Z"},
                        "$inc": {"use_count": 1}
                    }
                )
            except Exception as e:
                logger.warning(f"Failed to update filter usage stats: {e}")
                
        elif filter_definition:
            # Validate ad-hoc filter
            filter_errors = _validate_filter_definition(filter_definition)
            if filter_errors:
                return {
                    "error": "Invalid filter definition",
                    "validation_errors": filter_errors
                }, 400
            filter_def = filter_definition
        else:
            return {"error": "Either filter_id or filter_definition is required"}, 400
        
        # Build query and fetch video IDs from raw_videos (in system_data)
        query = _filter_to_mongo_query(filter_def)
        sort_field, sort_direction = _get_sort_spec(filter_def)
        
        raw_videos_coll = source_db[COLL_RAW_VIDEOS]
        cursor = raw_videos_coll.find(query, {"video_id": 1, "_id": 0}).sort(sort_field, sort_direction)
        
        # Apply limit if specified (with safety cap)
        limit = filter_def.get("limit")
        if limit:
            cursor = cursor.limit(min(int(limit), MAX_RESOLVE_LIMIT))
        else:
            cursor = cursor.limit(MAX_RESOLVE_LIMIT)
        
        video_ids = [doc["video_id"] for doc in cursor if doc.get("video_id")]
        
        logger.info(f"Resolved filter to {len(video_ids)} video(s) from {source_db_name}")
        
        return {
            "db_name": db_name,
            "source_db": source_db_name,  # Actual database where raw_videos lives
            "video_ids": video_ids,
            "count": len(video_ids),
            "filter_applied": bool(query),
            "limit_applied": limit or MAX_RESOLVE_LIMIT,
            "timestamp": datetime.utcnow().isoformat() + "Z"
        }, 200
        
    except Exception as e:
        logger.exception("Error resolving filter to video IDs")
        return {"error": str(e)}, 500


# =============================================================================
# Playlist Statistics
# =============================================================================


def get_playlists(db_name: str) -> Tuple[Dict[str, Any], int]:
    """
    Get playlist statistics from raw_videos collection.
    
    Aggregates data for each playlist including:
    - Video count per playlist
    - Channel association
    - Date range
    - Average engagement
    
    Note: raw_videos is a constant collection stored in system_data database.
    
    Args:
        db_name: Database name (for API compatibility, raw_videos uses system_data)
    
    Returns:
        Tuple of (response_dict, status_code)
    """
    try:
        client = get_mongo_client()
        
        # raw_videos is always in the constant database (system_data)
        source_db_name = get_db_for_collection(COLL_RAW_VIDEOS, db_name)
        
        # Check if database exists
        if source_db_name not in client.list_database_names():
            return {"error": f"Database '{source_db_name}' not found"}, 404
        
        db = client[source_db_name]
        
        # Check if collection exists
        if COLL_RAW_VIDEOS not in db.list_collection_names():
            return {
                "error": f"Collection '{COLL_RAW_VIDEOS}' not found in database '{source_db_name}'"
            }, 404
        
        coll = db[COLL_RAW_VIDEOS]
        
        # Aggregate playlist statistics
        pipeline = [
            # Filter out videos without playlist_id
            {
                "$match": {
                    "playlist_id": {"$exists": True, "$ne": None, "$ne": ""}
                }
            },
            {
                "$group": {
                    "_id": "$playlist_id",
                    "playlist_title": {"$first": "$playlist_title"},
                    "channel_id": {"$first": "$channel_id"},
                    "channel_title": {"$first": "$channel_title"},
                    "video_count": {"$sum": 1},
                    "avg_engagement": {"$avg": {"$ifNull": ["$engagement_score", 0]}},
                    "latest_video": {"$max": "$published_at"},
                    "earliest_video": {"$min": "$published_at"},
                }
            },
            {
                "$match": {"_id": {"$ne": None}}
            },
            {
                "$sort": {"video_count": -1}
            },
            {
                "$project": {
                    "playlist_id": "$_id",
                    "playlist_title": {"$ifNull": ["$playlist_title", "Unknown Playlist"]},
                    "channel_id": 1,
                    "channel_title": {"$ifNull": ["$channel_title", "Unknown Channel"]},
                    "video_count": 1,
                    "avg_engagement": {"$round": [{"$ifNull": ["$avg_engagement", 0]}, 4]},
                    "date_range": {
                        "earliest": "$earliest_video",
                        "latest": "$latest_video"
                    },
                    "_id": 0
                }
            }
        ]
        
        playlists = list(coll.aggregate(pipeline))
        
        return {
            "db_name": db_name,
            "source_db": source_db_name,  # Actual database where raw_videos lives
            "playlists": playlists,
            "total_playlists": len(playlists),
            "timestamp": datetime.utcnow().isoformat() + "Z"
        }, 200
        
    except Exception as e:
        logger.exception(f"Error getting playlists from {db_name}")
        return {"error": str(e)}, 500



