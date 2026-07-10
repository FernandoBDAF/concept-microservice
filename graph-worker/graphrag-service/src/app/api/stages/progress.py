"""
Real-Time Pipeline Progress Monitoring

Migrated from app/api/pipeline_progress.py to stages_api.
Provides real-time progress updates for pipeline execution.

Achievement 1.3: Real-Time Progress Monitoring
"""

import json
import logging
import threading
import time
from typing import Dict, Any, Optional, Set
from queue import Queue, Empty

logger = logging.getLogger(__name__)

# Global progress store (in-memory, per pipeline_id)
_progress_store: Dict[str, Dict[str, Any]] = {}
_progress_subscribers: Dict[str, Set[Queue]] = {}  # pipeline_id -> set of queues
_lock = threading.Lock()


class ProgressUpdate:
    """Progress update message."""

    def __init__(
        self,
        pipeline_id: str,
        stage: Optional[str] = None,
        status: Optional[str] = None,
        progress: Optional[float] = None,
        message: Optional[str] = None,
        error: Optional[str] = None,
    ):
        self.pipeline_id = pipeline_id
        self.stage = stage
        self.status = status
        self.progress = progress
        self.message = message
        self.error = error
        self.timestamp = time.time()

    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary for JSON serialization."""
        return {
            "pipeline_id": self.pipeline_id,
            "stage": self.stage,
            "status": self.status,
            "progress": self.progress,
            "message": self.message,
            "error": self.error,
            "timestamp": self.timestamp,
        }

    def to_json(self) -> str:
        """Convert to JSON string."""
        return json.dumps(self.to_dict())


def update_progress(
    pipeline_id: str,
    stage: Optional[str] = None,
    status: Optional[str] = None,
    progress: Optional[float] = None,
    message: Optional[str] = None,
    error: Optional[str] = None,
) -> None:
    """
    Update pipeline progress and notify subscribers.

    Args:
        pipeline_id: Unique pipeline identifier
        stage: Current stage name
        status: Pipeline status (running, completed, failed)
        progress: Progress percentage (0.0-1.0)
        message: Status message
        error: Error message if any
    """
    with _lock:
        # Update progress store
        if pipeline_id not in _progress_store:
            _progress_store[pipeline_id] = {}

        _progress_store[pipeline_id].update({
            "stage": stage,
            "status": status,
            "progress": progress,
            "message": message,
            "error": error,
            "last_updated": time.time(),
        })

        # Notify subscribers
        if pipeline_id in _progress_subscribers:
            update = ProgressUpdate(
                pipeline_id=pipeline_id,
                stage=stage,
                status=status,
                progress=progress,
                message=message,
                error=error,
            )

            # Send to all subscribers
            dead_queues = set()
            for queue in _progress_subscribers[pipeline_id]:
                try:
                    queue.put_nowait(update)
                except Exception as e:
                    logger.warning(f"Failed to send progress update: {e}")
                    dead_queues.add(queue)

            # Remove dead queues
            _progress_subscribers[pipeline_id] -= dead_queues


def get_progress(pipeline_id: str) -> Optional[Dict[str, Any]]:
    """
    Get current progress for a pipeline.

    Args:
        pipeline_id: Unique pipeline identifier

    Returns:
        Progress dictionary or None if not found
    """
    with _lock:
        return _progress_store.get(pipeline_id, {}).copy()


def subscribe_progress(pipeline_id: str) -> Queue:
    """
    Subscribe to progress updates for a pipeline.

    Args:
        pipeline_id: Unique pipeline identifier

    Returns:
        Queue that will receive ProgressUpdate objects
    """
    queue = Queue(maxsize=100)  # Limit queue size

    with _lock:
        if pipeline_id not in _progress_subscribers:
            _progress_subscribers[pipeline_id] = set()
        _progress_subscribers[pipeline_id].add(queue)

    return queue


def unsubscribe_progress(pipeline_id: str, queue: Queue) -> None:
    """
    Unsubscribe from progress updates.

    Args:
        pipeline_id: Unique pipeline identifier
        queue: Queue to remove
    """
    with _lock:
        if pipeline_id in _progress_subscribers:
            _progress_subscribers[pipeline_id].discard(queue)


def clear_progress(pipeline_id: str) -> None:
    """
    Clear progress data for a pipeline.

    Args:
        pipeline_id: Unique pipeline identifier
    """
    with _lock:
        _progress_store.pop(pipeline_id, None)
        _progress_subscribers.pop(pipeline_id, None)


def get_all_active_progress() -> Dict[str, Dict[str, Any]]:
    """
    Get progress for all active pipelines.

    Returns:
        Dictionary of pipeline_id -> progress data
    """
    with _lock:
        return {
            pid: data.copy()
            for pid, data in _progress_store.items()
            if data.get("status") in ["running", "starting"]
        }

