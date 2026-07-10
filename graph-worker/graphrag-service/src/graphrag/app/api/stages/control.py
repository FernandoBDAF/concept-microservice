"""
Pipeline Control Functions

Provides pause/resume functionality for pipeline execution.
Integrates with execution.py for centralized pipeline management.

Achievement 5.1: Pipeline Control API
"""

import logging
import threading
from typing import Dict, Any, Optional

logger = logging.getLogger(__name__)

# Pipeline pause state (thread-safe)
_pause_state: Dict[str, bool] = {}
_pause_lock = threading.Lock()


def pause_pipeline(pipeline_id: str) -> Dict[str, Any]:
    """
    Request a pipeline to pause execution.

    The pipeline will pause at the next safe checkpoint
    (typically after completing the current document/batch).

    Args:
        pipeline_id: Unique pipeline identifier

    Returns:
        Dictionary with pause status
    """
    with _pause_lock:
        _pause_state[pipeline_id] = True
        logger.info(f"[{pipeline_id}] Pause requested")
        
    return {
        "pipeline_id": pipeline_id,
        "status": "pause_requested",
        "message": "Pipeline will pause at next checkpoint",
    }


def resume_pipeline(pipeline_id: str) -> Dict[str, Any]:
    """
    Resume a paused pipeline.

    Args:
        pipeline_id: Unique pipeline identifier

    Returns:
        Dictionary with resume status
    """
    with _pause_lock:
        _pause_state[pipeline_id] = False
        logger.info(f"[{pipeline_id}] Resume requested")
        
    return {
        "pipeline_id": pipeline_id,
        "status": "resumed",
        "message": "Pipeline execution resumed",
    }


def is_paused(pipeline_id: str) -> bool:
    """
    Check if a pipeline is paused.

    Used by pipeline execution code to check if it should pause.

    Args:
        pipeline_id: Unique pipeline identifier

    Returns:
        True if pipeline should pause
    """
    with _pause_lock:
        return _pause_state.get(pipeline_id, False)


def clear_pause_state(pipeline_id: str) -> None:
    """
    Clear pause state for a pipeline.

    Called when pipeline completes or is cancelled.

    Args:
        pipeline_id: Unique pipeline identifier
    """
    with _pause_lock:
        _pause_state.pop(pipeline_id, None)


def get_pause_status(pipeline_id: str) -> Dict[str, Any]:
    """
    Get pause status for a pipeline.

    Args:
        pipeline_id: Unique pipeline identifier

    Returns:
        Dictionary with pause status
    """
    with _pause_lock:
        paused = _pause_state.get(pipeline_id, False)
        
    return {
        "pipeline_id": pipeline_id,
        "is_paused": paused,
    }


def get_all_paused_pipelines() -> Dict[str, bool]:
    """
    Get all pipelines with pause state.

    Returns:
        Dictionary of pipeline_id -> pause state
    """
    with _pause_lock:
        return dict(_pause_state)

