"""
Performance Metrics Handler - Business Logic

Pure functions for performance metrics operations.
No HTTP handling - that's in router.py

Achievement 6.3: Performance Dashboard
"""

import logging
import os
import sys
from datetime import datetime
from typing import Dict, Any, Optional

# Add project root to Python path for imports
_project_root = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "..", ".."))
if _project_root not in sys.path:
    sys.path.insert(0, _project_root)

from src.infrastructure.database.mongodb import get_mongo_client
from ..constants import DEFAULT_LIMIT

logger = logging.getLogger(__name__)


def get(
    db_name: str,
    pipeline_id: Optional[str] = None,
) -> Dict[str, Any]:
    """
    Get performance metrics for pipeline execution.

    Args:
        db_name: Database name
        pipeline_id: Optional pipeline ID to filter by

    Returns:
        Dictionary with performance metrics
    """
    client = get_mongo_client()
    db = client[db_name]
    tracking_coll = db.experiment_tracking

    # Build query
    query = {"pipeline_type": "graphrag"}
    if pipeline_id:
        query["experiment_id"] = pipeline_id

    # Get latest pipeline run
    latest_run = tracking_coll.find_one(query, sort=[("started_at", -1)])

    if not latest_run:
        return {
            "error": "No pipeline runs found",
            "stages": {},
        }

    # Get stage-level performance from metrics collection
    metrics_collection = db.graphrag_metrics
    stage_metrics = {}

    stages = ["graph_extraction", "entity_resolution", "graph_construction", "community_detection"]
    for stage in stages:
        stage_docs = list(metrics_collection.find({"stage": stage}).sort("timestamp", -1).limit(1))

        if stage_docs:
            doc = stage_docs[0]
            stage_metrics[stage] = {
                "timestamp": doc.get("timestamp"),
                "run_id": doc.get("run_id"),
            }

    # Calculate duration if available
    duration = None
    if latest_run.get("started_at") and latest_run.get("completed_at"):
        start = latest_run["started_at"]
        end = latest_run["completed_at"]
        if isinstance(start, datetime) and isinstance(end, datetime):
            duration = (end - start).total_seconds()
        elif isinstance(start, str) and isinstance(end, str):
            start_dt = datetime.fromisoformat(start.replace("Z", "+00:00"))
            end_dt = datetime.fromisoformat(end.replace("Z", "+00:00"))
            duration = (end_dt - start_dt).total_seconds()

    # Get chunk counts from tracking
    config = latest_run.get("configuration", {})
    total_chunks = config.get("total_chunks", 0)

    # Calculate throughput
    throughput = {}
    if duration and duration > 0 and total_chunks > 0:
        throughput["chunks_per_sec"] = total_chunks / duration
        throughput["chunks_per_min"] = (total_chunks / duration) * 60

    return {
        "pipeline_id": latest_run.get("experiment_id"),
        "status": latest_run.get("status"),
        "started_at": latest_run.get("started_at"),
        "completed_at": latest_run.get("completed_at"),
        "duration_seconds": duration,
        "total_chunks": total_chunks,
        "throughput": throughput,
        "stages": stage_metrics,
    }


def get_trends(db_name: str, limit: int = DEFAULT_LIMIT) -> Dict[str, Any]:
    """
    Get performance trends over multiple pipeline runs.

    Args:
        db_name: Database name
        limit: Maximum number of runs to include

    Returns:
        Dictionary with performance trends
    """
    client = get_mongo_client()
    db = client[db_name]
    tracking_coll = db.experiment_tracking

    # Get recent pipeline runs
    cursor = tracking_coll.find({"pipeline_type": "graphrag"}).sort("started_at", -1).limit(limit)

    runs = []
    for doc in cursor:
        duration = None
        if doc.get("started_at") and doc.get("completed_at"):
            start = doc["started_at"]
            end = doc["completed_at"]
            if isinstance(start, datetime) and isinstance(end, datetime):
                duration = (end - start).total_seconds()

        runs.append({
            "pipeline_id": doc.get("experiment_id"),
            "status": doc.get("status"),
            "started_at": doc.get("started_at"),
            "duration_seconds": duration,
            "total_chunks": doc.get("configuration", {}).get("total_chunks", 0),
        })

    return {
        "total_runs": len(runs),
        "runs": list(reversed(runs)),  # Oldest first
    }

