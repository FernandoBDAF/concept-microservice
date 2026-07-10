"""
Grafana annotations API for pipeline events.

This module provides functions to create Grafana annotations that mark
pipeline events (started, completed, failed) on Grafana graphs. Annotations
appear as vertical lines with hover text, helping correlate pipeline
activity with metrics.

Reference: OBSERVABILITY_IMPLEMENTATION_ROADMAP_PART2.md Section 4.1

Environment Variables:
    GRAFANA_URL: Base URL for Grafana (default: http://localhost:3000)
    GRAFANA_API_KEY: API key for authentication (required for annotations)

Usage:
    # Create a pipeline annotation
    from src.app.api.stages.annotations import annotate_pipeline_event
    annotate_pipeline_event("started", "pipeline_123", "ingestion")
    
    # Create a custom annotation
    from src.app.api.stages.annotations import create_annotation
    create_annotation("Custom event", ["custom", "tag"])
"""
import os
import json
import logging
from datetime import datetime
from typing import Dict, Any, Optional, List
import urllib.request
import urllib.error

logger = logging.getLogger(__name__)

GRAFANA_URL = os.getenv("GRAFANA_URL", "http://localhost:3000")
GRAFANA_API_KEY = os.getenv("GRAFANA_API_KEY", "")


def create_annotation(
    text: str,
    tags: List[str],
    dashboard_uid: Optional[str] = None,
    panel_id: Optional[int] = None,
    time_from: Optional[datetime] = None,
    time_to: Optional[datetime] = None,
) -> Dict[str, Any]:
    """
    Create a Grafana annotation.
    
    Annotations appear as vertical lines on graphs with hover text.
    They can be global (appear on all dashboards) or specific to a 
    dashboard/panel.
    
    Args:
        text: The annotation text (shown on hover)
        tags: List of tags for filtering annotations
        dashboard_uid: Optional UID of dashboard to attach annotation to
        panel_id: Optional panel ID within dashboard
        time_from: Start time (defaults to now)
        time_to: End time for range annotations (optional)
        
    Returns:
        Dict with annotation result or error message
        
    Example:
        >>> create_annotation(
        ...     "Pipeline started",
        ...     ["pipeline", "ingestion"],
        ...     time_from=datetime.utcnow()
        ... )
        {"id": 123, "message": "Annotation created"}
    """
    if not GRAFANA_API_KEY:
        logger.debug("GRAFANA_API_KEY not configured, skipping annotation")
        return {"skipped": True, "reason": "GRAFANA_API_KEY not configured"}
    
    now_ms = int(datetime.utcnow().timestamp() * 1000)
    
    payload = {
        "text": text,
        "tags": tags,
        "time": int(time_from.timestamp() * 1000) if time_from else now_ms,
    }
    
    if time_to:
        payload["timeEnd"] = int(time_to.timestamp() * 1000)
    
    if dashboard_uid:
        payload["dashboardUID"] = dashboard_uid
    
    if panel_id:
        payload["panelId"] = panel_id
    
    try:
        req = urllib.request.Request(
            f"{GRAFANA_URL}/api/annotations",
            data=json.dumps(payload).encode("utf-8"),
            headers={
                "Content-Type": "application/json",
                "Authorization": f"Bearer {GRAFANA_API_KEY}",
            },
            method="POST",
        )
        
        with urllib.request.urlopen(req, timeout=5) as response:
            result = json.loads(response.read().decode())
            logger.debug(f"Created Grafana annotation: {result}")
            return result
    
    except urllib.error.HTTPError as e:
        error_msg = f"HTTP {e.code}: {e.reason}"
        try:
            error_body = e.read().decode()
            error_msg += f" - {error_body}"
        except Exception:
            pass
        logger.warning(f"Failed to create Grafana annotation: {error_msg}")
        return {"error": error_msg}
    
    except urllib.error.URLError as e:
        logger.warning(f"Failed to connect to Grafana: {e}")
        return {"error": str(e)}
    
    except Exception as e:
        logger.warning(f"Unexpected error creating annotation: {e}")
        return {"error": str(e)}


def annotate_pipeline_event(
    event: str,
    pipeline_id: str,
    pipeline_type: str,
    details: Optional[str] = None,
    started_at: Optional[datetime] = None,
    completed_at: Optional[datetime] = None,
) -> Dict[str, Any]:
    """
    Create annotation for a pipeline event.
    
    Creates a Grafana annotation with standardized text and tags for
    pipeline lifecycle events. For completed/failed events, creates
    a range annotation if both started_at and completed_at are provided.
    
    Args:
        event: Event type - one of: started, completed, failed, cancelled
        pipeline_id: Unique identifier for the pipeline execution
        pipeline_type: Type of pipeline (e.g., "ingestion", "graphrag")
        details: Optional additional details (e.g., error message)
        started_at: Pipeline start time (for range annotations)
        completed_at: Pipeline end time (for range annotations)
        
    Returns:
        Dict with annotation result or error message
        
    Example:
        >>> annotate_pipeline_event("started", "pipe_123", "ingestion")
        {"id": 456, "message": "Annotation created"}
        
        >>> annotate_pipeline_event(
        ...     "completed", "pipe_123", "ingestion",
        ...     started_at=start_time, completed_at=end_time
        ... )
        {"id": 457, "message": "Annotation created"}
    """
    emoji_map = {
        "started": "▶️",
        "completed": "✅",
        "failed": "❌",
        "cancelled": "⏹️",
        "stage_started": "🔄",
        "stage_completed": "✔️",
        "stage_failed": "⚠️",
    }
    
    text = f"{emoji_map.get(event, '📌')} Pipeline {event}: {pipeline_type}"
    if details:
        text += f"\n{details}"
    
    text += f"\nID: {pipeline_id}"
    
    tags = [
        "pipeline",
        f"pipeline:{pipeline_type}",
        f"event:{event}",
        f"id:{pipeline_id}",
    ]
    
    # For completed events with both times, create a range annotation
    time_from = started_at
    time_to = None
    
    if event in ("completed", "failed", "cancelled"):
        time_from = started_at
        time_to = completed_at
    elif event == "started":
        time_from = started_at or datetime.utcnow()
    
    return create_annotation(
        text=text,
        tags=tags,
        time_from=time_from,
        time_to=time_to,
    )


def annotate_stage_event(
    event: str,
    pipeline_id: str,
    stage_name: str,
    details: Optional[str] = None,
    duration_seconds: Optional[float] = None,
) -> Dict[str, Any]:
    """
    Create annotation for a stage event within a pipeline.
    
    Creates a Grafana annotation with standardized text and tags for
    stage lifecycle events.
    
    Args:
        event: Event type - one of: stage_started, stage_completed, stage_failed
        pipeline_id: Unique identifier for the pipeline execution
        stage_name: Name of the stage
        details: Optional additional details (e.g., error message)
        duration_seconds: Stage duration (for completed events)
        
    Returns:
        Dict with annotation result or error message
    """
    emoji_map = {
        "stage_started": "🔄",
        "stage_completed": "✔️",
        "stage_failed": "⚠️",
    }
    
    text = f"{emoji_map.get(event, '📌')} Stage {event.replace('stage_', '')}: {stage_name}"
    if duration_seconds is not None:
        text += f" ({duration_seconds:.1f}s)"
    if details:
        text += f"\n{details}"
    
    tags = [
        "stage",
        f"stage:{stage_name}",
        f"event:{event}",
        f"pipeline:{pipeline_id}",
    ]
    
    return create_annotation(text=text, tags=tags)


def get_annotations(
    from_time: Optional[datetime] = None,
    to_time: Optional[datetime] = None,
    tags: Optional[List[str]] = None,
    limit: int = 100,
) -> Dict[str, Any]:
    """
    Query existing Grafana annotations.
    
    Args:
        from_time: Start of time range (defaults to 1 hour ago)
        to_time: End of time range (defaults to now)
        tags: Filter by tags
        limit: Maximum number of annotations to return
        
    Returns:
        Dict with list of annotations or error message
    """
    if not GRAFANA_API_KEY:
        return {"skipped": True, "reason": "GRAFANA_API_KEY not configured"}
    
    now = datetime.utcnow()
    from_ms = int((from_time or datetime.utcnow().replace(hour=now.hour-1)).timestamp() * 1000)
    to_ms = int((to_time or datetime.utcnow()).timestamp() * 1000)
    
    params = f"from={from_ms}&to={to_ms}&limit={limit}"
    if tags:
        for tag in tags:
            params += f"&tags={tag}"
    
    try:
        req = urllib.request.Request(
            f"{GRAFANA_URL}/api/annotations?{params}",
            headers={
                "Authorization": f"Bearer {GRAFANA_API_KEY}",
            },
            method="GET",
        )
        
        with urllib.request.urlopen(req, timeout=5) as response:
            return json.loads(response.read().decode())
    
    except Exception as e:
        logger.warning(f"Failed to query annotations: {e}")
        return {"error": str(e)}

