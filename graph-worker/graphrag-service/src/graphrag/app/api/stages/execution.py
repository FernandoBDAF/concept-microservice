"""
Pipeline Execution Management Module

Reference: API_DESIGN_SPECIFICATION.md Section 3.6, 3.7, 7.1, 7.2

Provides:
- Background pipeline execution
- Thread-safe state management
- Progress tracking
- Pipeline cancellation
- MongoDB persistence for state recovery
"""

import argparse
import logging
import os
import threading
import time
import uuid
from datetime import datetime
from typing import Any, Dict, List, Optional

logger = logging.getLogger(__name__)

# ============================================
# Grafana Annotations
# ============================================
# Lazy import to avoid circular imports and allow graceful degradation
def _create_pipeline_annotation(
    event: str,
    pipeline_id: str,
    pipeline_type: str,
    details: Optional[str] = None,
    started_at: Optional[datetime] = None,
    completed_at: Optional[datetime] = None,
) -> None:
    """Create a Grafana annotation for a pipeline event (non-blocking)."""
    try:
        from .annotations import annotate_pipeline_event
        result = annotate_pipeline_event(
            event=event,
            pipeline_id=pipeline_id,
            pipeline_type=pipeline_type,
            details=details,
            started_at=started_at,
            completed_at=completed_at,
        )
        if result.get("error"):
            logger.debug(f"[{pipeline_id}] Annotation error: {result['error']}")
        elif result.get("skipped"):
            logger.debug(f"[{pipeline_id}] Annotation skipped: {result.get('reason')}")
        else:
            logger.debug(f"[{pipeline_id}] Created annotation for {event}")
    except Exception as e:
        # Annotations should never fail the pipeline
        logger.debug(f"[{pipeline_id}] Failed to create annotation: {e}")

# ============================================
# Observability Metrics
# ============================================
# NOTE: Metrics are defined in app/stages_api/metrics.py to avoid duplicate registrations.
# We use the StagesMetricsTracker from that module for all metrics operations.

def _get_metrics_tracker():
    """Lazy import to avoid circular imports."""
    from .metrics import get_metrics_tracker
    return get_metrics_tracker()

# Thread-safe pipeline state storage (in-memory cache)
_active_pipelines: Dict[str, Dict[str, Any]] = {}
_pipeline_lock = threading.Lock()

# Flag to track if recovery has been attempted
_recovery_done = False


# ============================================
# Database Persistence Helpers
# ============================================


def _sync_to_db(pipeline_id: str, state: Dict[str, Any]) -> None:
    """
    Sync pipeline state to database for persistence.
    
    Fails silently to allow graceful degradation when DB is unavailable.
    """
    try:
        from .repository import get_repository
        repo = get_repository()
        if repo is None:
            logger.warning(f"[{pipeline_id}] No repository available for DB sync")
            return
        
        # Make a copy to avoid modifying original state
        db_state = state.copy()
        
        # Convert datetime objects to ISO strings for MongoDB
        if isinstance(db_state.get("started_timestamp"), float):
            # Keep as-is for DB
            pass
        
        if repo.exists(pipeline_id):
            repo.update(pipeline_id, db_state)
            logger.debug(f"[{pipeline_id}] Updated in DB (status: {state.get('status')})")
        else:
            repo.create(db_state)
            logger.debug(f"[{pipeline_id}] Created in DB (status: {state.get('status')})")
            
    except Exception as e:
        logger.warning(f"Failed to sync pipeline {pipeline_id} to DB: {e}")


def _load_from_db(pipeline_id: str) -> Optional[Dict[str, Any]]:
    """Load pipeline state from database."""
    try:
        from .repository import get_repository
        repo = get_repository()
        if repo is None:
            logger.debug(f"[{pipeline_id}] No repository for DB load")
            return None
        result = repo.get(pipeline_id)
        if result:
            logger.debug(f"[{pipeline_id}] Loaded from DB (status: {result.get('status')})")
        else:
            logger.debug(f"[{pipeline_id}] Not found in DB")
        return result
    except Exception as e:
        logger.warning(f"Failed to load pipeline {pipeline_id} from DB: {e}")
        return None


def recover_state_from_db() -> None:
    """
    Recover pipeline state from database on startup.
    
    Any pipelines that were "running" when the server died are marked as "interrupted".
    """
    global _recovery_done
    
    if _recovery_done:
        return
    
    _recovery_done = True
    
    try:
        from .repository import get_repository
        repo = get_repository()
        if repo is None:
            logger.info("Pipeline persistence not available - skipping recovery")
            return
        
        # Find pipelines that were running when server stopped
        active = repo.list_active()
        interrupted_count = 0
        
        for pipeline in active:
            if pipeline.get("status") == "running":
                repo.update_status(
                    pipeline["pipeline_id"],
                    "interrupted",
                    error="Server restarted during execution"
                )
                interrupted_count += 1
                logger.warning(
                    f"Pipeline {pipeline['pipeline_id']} marked as interrupted (was running)"
                )
        
        if interrupted_count > 0:
            logger.info(f"Recovered {interrupted_count} interrupted pipelines from database")
        else:
            logger.info("No running pipelines found during recovery")
            
    except Exception as e:
        logger.warning(f"Failed to recover state from DB: {e}")


# ============================================
# Public API Functions
# ============================================


def execute_pipeline(
    pipeline: str,
    stages: List[str],
    config: Dict[str, Dict[str, Any]],
    metadata: Optional[Dict[str, Any]] = None,
) -> Dict[str, Any]:
    """
    Execute a pipeline in the background.

    Reference: API_DESIGN_SPECIFICATION.md Section 3.6

    Args:
        pipeline: Pipeline type ("ingestion" or "graphrag")
        stages: List of stage names to execute
        config: Stage configurations
        metadata: Optional metadata (experiment_id, description, etc.)

    Returns:
        Pipeline execution info with pipeline_id and tracking URL
    """
    # Log received config for debugging
    logger.info(f"[execute_pipeline] Received config: {config}")
    logger.info(f"[execute_pipeline] Stages: {stages}")
    
    # Ensure recovery has been done
    recover_state_from_db()
    
    # Generate unique pipeline ID
    pipeline_id = f"pipeline_{int(time.time())}_{uuid.uuid4().hex[:8]}"

    # Validate configuration first
    from .validation import validate_pipeline_config

    validation = validate_pipeline_config(pipeline, stages, config)

    if not validation["valid"]:
        return {"error": "Invalid configuration", "details": validation, "pipeline_id": None}

    # Use execution plan from validation (includes dependencies)
    execution_stages = validation["execution_plan"]["stages"]

    # ================================================
    # Source Selection: Resolve filter to video IDs
    # ================================================
    db_name = os.getenv("DB_NAME") or os.getenv("MONGODB_DB") or "mongo_hack"
    input_video_ids = None
    input_filter_info = None
    
    if metadata:
        # Option 1: Saved filter by ID
        if filter_id := metadata.get("input_filter_id"):
            from .source_selection import resolve_filter_to_video_ids
            result, status = resolve_filter_to_video_ids(db_name, filter_id=filter_id)
            if status != 200:
                return {
                    "error": f"Failed to resolve filter: {result.get('error', 'Unknown error')}",
                    "pipeline_id": None
                }
            input_video_ids = result.get("video_ids")
            input_filter_info = {"type": "saved", "filter_id": filter_id, "count": len(input_video_ids)}
            logger.info(f"[{pipeline_id}] Resolved saved filter {filter_id} to {len(input_video_ids)} video(s)")
        
        # Option 2: Ad-hoc filter definition
        elif filter_def := metadata.get("input_filter"):
            from .source_selection import resolve_filter_to_video_ids
            result, status = resolve_filter_to_video_ids(db_name, filter_definition=filter_def)
            if status != 200:
                return {
                    "error": f"Failed to resolve filter: {result.get('error', 'Unknown error')}",
                    "pipeline_id": None
                }
            input_video_ids = result.get("video_ids")
            input_filter_info = {"type": "adhoc", "count": len(input_video_ids)}
            logger.info(f"[{pipeline_id}] Resolved ad-hoc filter to {len(input_video_ids)} video(s)")
    
    # Validate we have videos to process (only if filter was applied)
    if input_video_ids is not None and len(input_video_ids) == 0:
        return {
            "error": "Filter matched no videos - nothing to process",
            "pipeline_id": None
        }
    
    # Pass input video IDs to config for stages to use
    if input_video_ids:
        config["_input_video_ids"] = input_video_ids

    # Initialize pipeline state
    state = {
        "pipeline_id": pipeline_id,
        "pipeline": pipeline,
        "status": "starting",
        "started_at": datetime.utcnow().isoformat() + "Z",
        "started_timestamp": time.time(),
        "stages": execution_stages,
        "config": config,
        "metadata": metadata or {},
        "current_stage": None,
        "current_stage_index": 0,
        "completed_stages": [],
        "failed_stages": [],
        "progress": {
            "total_stages": len(execution_stages),
            "completed_stages": 0,
            "percent": 0.0,
        },
        "error": None,
        "error_stage": None,
        "exit_code": None,
        # Source selection info
        "input_filter": input_filter_info,
        "input_video_count": len(input_video_ids) if input_video_ids else None,
    }
    
    with _pipeline_lock:
        _active_pipelines[pipeline_id] = state
    
    # Track active pipeline (observability)
    _get_metrics_tracker().pipeline_started(pipeline)
    
    # Create Grafana annotation for pipeline start
    _create_pipeline_annotation(
        event="started",
        pipeline_id=pipeline_id,
        pipeline_type=pipeline,
        started_at=datetime.utcnow(),
    )
    
    # Persist to database
    _sync_to_db(pipeline_id, state)

    # Start execution in background thread
    thread = threading.Thread(
        target=_run_pipeline_background,
        args=(pipeline_id, pipeline, execution_stages, config, metadata),
        daemon=True,
        name=f"Pipeline-{pipeline_id}",
    )
    thread.start()

    return {
        "pipeline_id": pipeline_id,
        "status": "starting",
        "started_at": state["started_at"],
        "stages": execution_stages,
        "tracking_url": f"/api/v1/pipelines/{pipeline_id}/status",
    }


def get_pipeline_status(pipeline_id: str) -> Dict[str, Any]:
    """
    Get current status of a pipeline.

    Reference: API_DESIGN_SPECIFICATION.md Section 3.7

    Returns pipeline state including:
    - Overall status
    - Current stage
    - Progress percentage
    - Timing information
    """
    # Ensure recovery has been done
    recover_state_from_db()
    
    state = None
    
    # First check in-memory cache
    with _pipeline_lock:
        all_ids = list(_active_pipelines.keys())
        logger.debug(f"[{pipeline_id}] Checking memory. Active pipelines: {all_ids}")
        
        if pipeline_id in _active_pipelines:
            state = _active_pipelines[pipeline_id].copy()
            logger.debug(f"[{pipeline_id}] Found in memory (status: {state.get('status')})")
        else:
            logger.debug(f"[{pipeline_id}] NOT in memory cache")
    
    # If not in memory, check database
    if state is None:
        logger.info(f"[{pipeline_id}] Not in memory, checking DB...")
        state = _load_from_db(pipeline_id)
        if state is None:
            logger.warning(f"[{pipeline_id}] Not found in memory or DB - returning 404")
            return {"error": "Pipeline not found", "pipeline_id": pipeline_id}
        else:
            logger.info(f"[{pipeline_id}] Found in DB (status: {state.get('status')})")

    # Calculate elapsed time
    if "started_timestamp" in state:
        if state["status"] in ["completed", "failed", "error", "cancelled", "interrupted"]:
            elapsed = state.get("duration_seconds", 0)
        else:
            elapsed = time.time() - state["started_timestamp"]
        state["elapsed_seconds"] = int(elapsed)

    # Remove internal fields
    state.pop("started_timestamp", None)
    state.pop("completed_timestamp", None)
    state.pop("_id", None)  # MongoDB ObjectId
    state.pop("created_at", None)
    state.pop("updated_at", None)

    return state


def cancel_pipeline(pipeline_id: str) -> Dict[str, Any]:
    """
    Cancel a running pipeline.

    Note: This marks the pipeline as cancelled. The running thread
    should check this status and stop gracefully.
    """
    with _pipeline_lock:
        if pipeline_id not in _active_pipelines:
            # Check database
            state = _load_from_db(pipeline_id)
            if state is None:
                return {"error": "Pipeline not found", "pipeline_id": pipeline_id}
            
            if state["status"] not in ["starting", "running"]:
                return {
                    "error": "Pipeline is not running",
                    "pipeline_id": pipeline_id,
                    "current_status": state["status"],
                }
            
            # Pipeline was in DB but not in memory - unusual state
            return {
                "error": "Pipeline found in history but not actively running",
                "pipeline_id": pipeline_id,
                "current_status": state["status"],
            }

        state = _active_pipelines[pipeline_id]

        if state["status"] not in ["starting", "running"]:
            return {
                "error": "Pipeline is not running",
                "pipeline_id": pipeline_id,
                "current_status": state["status"],
            }

        # Mark as cancelled
        state["status"] = "cancelled"
        state["completed_at"] = datetime.utcnow().isoformat() + "Z"
        state["completed_timestamp"] = time.time()
        state["duration_seconds"] = (
            state["completed_timestamp"] - state["started_timestamp"]
        )

    # Persist to database
    _sync_to_db(pipeline_id, state)

    return {
        "pipeline_id": pipeline_id,
        "status": "cancelled",
        "message": "Pipeline cancellation requested",
    }


def list_active_pipelines() -> Dict[str, Any]:
    """List all active (running/starting) pipelines"""
    # Ensure recovery has been done
    recover_state_from_db()
    
    with _pipeline_lock:
        active = {
            pid: {
                "pipeline_id": state["pipeline_id"],
                "pipeline": state["pipeline"],
                "status": state["status"],
                "started_at": state["started_at"],
                "current_stage": state["current_stage"],
                "progress": state["progress"],
            }
            for pid, state in _active_pipelines.items()
            if state["status"] in ["starting", "running"]
        }

    return {"count": len(active), "pipelines": active}


def get_pipeline_history(limit: int = 10) -> Dict[str, Any]:
    """Get recent pipeline executions"""
    # Ensure recovery has been done
    recover_state_from_db()
    
    # Try to get from database first for complete history
    try:
        from .repository import get_repository
        repo = get_repository()
        if repo is not None:
            db_pipelines = repo.list_history(limit)
            total = repo.count_all()
            
            # Clean up for response - include useful details
            result = []
            for state in db_pipelines:
                result.append({
                    "pipeline_id": state.get("pipeline_id"),
                    "pipeline": state.get("pipeline"),
                    "status": state.get("status"),
                    "started_at": state.get("started_at"),
                    "completed_at": state.get("completed_at"),
                    "stages": state.get("stages", []),
                    "progress": state.get("progress", {}),
                    # Additional useful fields
                    "duration_seconds": state.get("duration_seconds"),
                    "exit_code": state.get("exit_code"),
                    "error": state.get("error"),
                    "error_stage": state.get("error_stage"),
                    "config": state.get("config", {}),
                    "metadata": state.get("metadata", {}),
                })
            
            return {"total": total, "returned": len(result), "pipelines": result}
    except Exception as e:
        logger.warning(f"Failed to get history from DB, falling back to memory: {e}")
    
    # Fallback to in-memory
    with _pipeline_lock:
        all_pipelines = list(_active_pipelines.values())

    # Sort by start time (most recent first)
    all_pipelines.sort(key=lambda x: x.get("started_timestamp", 0), reverse=True)

    # Limit results
    recent = all_pipelines[:limit]

    # Clean up for response - include useful details
    result = []
    for state in recent:
        result.append({
            "pipeline_id": state["pipeline_id"],
            "pipeline": state["pipeline"],
            "status": state["status"],
            "started_at": state["started_at"],
            "completed_at": state.get("completed_at"),
            "stages": state["stages"],
            "progress": state["progress"],
            # Additional useful fields
            "duration_seconds": state.get("duration_seconds"),
            "exit_code": state.get("exit_code"),
            "error": state.get("error"),
            "error_stage": state.get("error_stage"),
            "config": state.get("config", {}),
            "metadata": state.get("metadata", {}),
        })

    return {"total": len(all_pipelines), "returned": len(result), "pipelines": result}


# ============================================
# Background Execution
# ============================================


def _run_pipeline_background(
    pipeline_id: str,
    pipeline: str,
    stages: List[str],
    config: Dict[str, Dict[str, Any]],
    metadata: Optional[Dict[str, Any]],
):
    """
    Run pipeline in background thread.

    Reference: API_DESIGN_SPECIFICATION.md Section 7.2
    """
    try:
        # Update status to running
        _update_pipeline_status(pipeline_id, "running")

        # Create pipeline object
        pipeline_obj = _create_pipeline_object(pipeline, stages, config, metadata)

        if pipeline_obj is None:
            _update_pipeline_error(pipeline_id, "Failed to create pipeline object", None)
            return

        # Execute pipeline - run each SELECTED stage individually
        logger.info(f"[{pipeline_id}] Starting pipeline execution with stages: {stages}")
        
        completed_stages = []
        exit_code = 0
        
        for idx, stage_name in enumerate(stages):
            # Check if cancelled
            with _pipeline_lock:
                if pipeline_id in _active_pipelines:
                    if _active_pipelines[pipeline_id]["status"] == "cancelled":
                        logger.info(f"[{pipeline_id}] Pipeline cancelled, stopping execution")
                        return
            
            # Update current stage
            _update_pipeline_progress(
                pipeline_id,
                current_stage=stage_name,
                current_stage_index=idx,
            )
            
            logger.info(f"[{pipeline_id}] Running stage {idx+1}/{len(stages)}: {stage_name}")
            
            # Run the individual stage
            try:
                stage_exit_code = pipeline_obj.run_stage(stage_name)
                if stage_exit_code != 0:
                    logger.error(f"[{pipeline_id}] Stage {stage_name} failed with exit code {stage_exit_code}")
                    exit_code = stage_exit_code
                    _update_pipeline_error(pipeline_id, f"Stage {stage_name} failed", stage_name)
                    return
                
                completed_stages.append(stage_name)
                _update_pipeline_progress(
                    pipeline_id,
                    completed_stages=completed_stages,
                    percent=((idx + 1) / len(stages)) * 100,
                )
                logger.info(f"[{pipeline_id}] Stage {stage_name} completed successfully")
                
            except Exception as stage_error:
                logger.exception(f"[{pipeline_id}] Stage {stage_name} raised exception")
                _update_pipeline_error(pipeline_id, str(stage_error), stage_name)
                return

        # Update final status
        _update_pipeline_completion(pipeline_id, exit_code, stages)

        logger.info(f"[{pipeline_id}] Pipeline completed with exit code: {exit_code}")

    except Exception as e:
        logger.exception(f"[{pipeline_id}] Pipeline execution failed")
        _update_pipeline_error(pipeline_id, str(e), None)


def _create_pipeline_object(
    pipeline: str,
    stages: List[str],
    config: Dict[str, Dict[str, Any]],
    metadata: Optional[Dict[str, Any]],
):
    """Create pipeline object from configuration"""
    try:
        # Extract input_video_ids from config (set by source selection filter)
        input_video_ids = config.get("_input_video_ids")
        if input_video_ids:
            logger.info(f"[create_pipeline] Source selection filter: {len(input_video_ids)} video(s)")
        
        if pipeline == "graphrag":
            from src.domain.pipelines.graphrag import GraphRAGPipeline
            from src.core.config.graphrag import GraphRAGPipelineConfig

            # Create args namespace
            args = argparse.Namespace()
            env = dict(os.environ)
            
            # Pass input_video_ids to args for stages to use in filtering
            if input_video_ids:
                args.input_video_ids = input_video_ids

            # Apply metadata
            if metadata and "experiment_id" in metadata:
                env["EXPERIMENT_ID"] = metadata["experiment_id"]

            # Apply stage configs to env/args
            _apply_config_to_args_env(config, args, env)

            # Get database name from env (support both DB_NAME and MONGODB_DB)
            db_name = os.getenv("DB_NAME") or os.getenv("MONGODB_DB") or "mongo_hack"
            
            # Create pipeline config
            pipeline_config = GraphRAGPipelineConfig.from_args_env(
                args, env, db_name
            )

            # Set selected stages
            pipeline_config.selected_stages = ",".join(stages)

            return GraphRAGPipeline(pipeline_config)

        elif pipeline == "ingestion":
            from src.domain.pipelines.ingestion import (
                IngestionPipeline,
                IngestionPipelineConfig,
            )
            from src.domain.stages.ingestion.ingest import IngestConfig
            from src.domain.stages.ingestion.clean import CleanConfig
            from src.domain.stages.ingestion.chunk import ChunkConfig
            from src.domain.stages.ingestion.enrich import EnrichConfig
            from src.domain.stages.ingestion.embed import EmbedConfig
            from src.domain.stages.ingestion.redundancy import RedundancyConfig
            from src.domain.stages.ingestion.trust import TrustConfig

            # Get database name from env (support both DB_NAME and MONGODB_DB)
            db_name = os.getenv("DB_NAME") or os.getenv("MONGODB_DB") or "mongo_hack"
            
            # Create args/env for each stage individually to avoid cross-contamination
            def create_stage_config(stage_name: str, config_cls):
                """Create config for a single stage from API config"""
                args = argparse.Namespace()
                env = dict(os.environ)
                
                # Apply only this stage's config (skip internal keys)
                stage_config_dict = config.get(stage_name, {})
                if isinstance(stage_config_dict, dict):
                    logger.info(f"[{stage_name}] Creating config with: {stage_config_dict}")
                    for key, value in stage_config_dict.items():
                        # Skip input_video_ids from stage config - we'll set it from filter below
                        if key == "input_video_ids":
                            continue
                        setattr(args, key, value)
                        # Also set in env with stage prefix
                        env_key = f"{stage_name.upper()}_{key.upper()}"
                        if value is not None:
                            env[env_key] = str(value)
                else:
                    logger.warning(f"[{stage_name}] Config is not a dict: {type(stage_config_dict)}")
                
                # Pass input_video_ids to args for stages to use in filtering
                # Set AFTER stage config to ensure filter takes precedence
                if input_video_ids:
                    args.input_video_ids = input_video_ids
                
                # Create config using the stage's from_args_env
                result = config_cls.from_args_env(args, env, db_name)
                logger.info(f"[{stage_name}] Final config: token_size={getattr(result, 'token_size', 'N/A')}, max={getattr(result, 'max', 'N/A')}, input_video_ids={len(getattr(result, 'input_video_ids', []) or [])} videos")
                return result
            
            # Create individual stage configs
            ingest_config = create_stage_config("ingest", IngestConfig)
            clean_config = create_stage_config("clean", CleanConfig)
            chunk_config = create_stage_config("chunk", ChunkConfig)
            enrich_config = create_stage_config("enrich", EnrichConfig)
            embed_config = create_stage_config("embed", EmbedConfig)
            redundancy_config = create_stage_config("redundancy", RedundancyConfig)
            trust_config = create_stage_config("trust", TrustConfig)
            
            # Create pipeline config directly (bypass from_args_env overrides)
            pipeline_config = IngestionPipelineConfig(
                db_name=db_name,
                continue_on_error=True,
                verbose=False,
                dry_run=False,
                ingest_config=ingest_config,
                clean_config=clean_config,
                enrich_config=enrich_config,
                chunk_config=chunk_config,
                embed_config=embed_config,
                redundancy_config=redundancy_config,
                trust_config=trust_config,
            )
            
            logger.info(f"Ingestion pipeline configured with db: {db_name}, selected stages: {stages}")
            logger.info(f"Clean config: max={clean_config.max}, llm={clean_config.llm}, use_llm={clean_config.use_llm}")

            return IngestionPipeline(pipeline_config)

        else:
            logger.error(f"Unknown pipeline type: {pipeline}")
            return None

    except Exception as e:
        logger.exception(f"Failed to create pipeline object: {e}")
        return None


def _apply_config_to_args_env(
    config: Dict[str, Dict[str, Any]],
    args: argparse.Namespace,
    env: Dict[str, str],
):
    """Apply stage configuration to args and environment"""
    for stage_name, stage_config in config.items():
        # Skip internal keys (like _input_video_ids)
        if stage_name.startswith("_"):
            continue
        
        # Ensure stage_config is a dict
        if not isinstance(stage_config, dict):
            logger.warning(f"Skipping non-dict config for stage '{stage_name}': {type(stage_config)}")
            continue
            
        for key, value in stage_config.items():
            # Set on args namespace
            setattr(args, key, value)

            # Also set in env for stages that read from env
            env_key = f"{stage_name.upper()}_{key.upper()}"
            if value is not None:
                env[env_key] = str(value)


# ============================================
# State Update Helpers (with DB persistence)
# ============================================


def _update_pipeline_status(pipeline_id: str, status: str) -> None:
    """Update pipeline status in memory and database"""
    with _pipeline_lock:
        if pipeline_id in _active_pipelines:
            _active_pipelines[pipeline_id]["status"] = status
            state = _active_pipelines[pipeline_id]
    
    # Persist to database
    _sync_to_db(pipeline_id, {"status": status})


def _update_pipeline_progress(
    pipeline_id: str,
    current_stage: Optional[str] = None,
    current_stage_index: Optional[int] = None,
    completed_stages: Optional[List[str]] = None,
    percent: Optional[float] = None,
) -> None:
    """Update pipeline progress in memory and database"""
    updates: Dict[str, Any] = {}
    
    with _pipeline_lock:
        if pipeline_id not in _active_pipelines:
            return
        
        state = _active_pipelines[pipeline_id]
        
        if current_stage is not None:
            state["current_stage"] = current_stage
            updates["current_stage"] = current_stage
        
        if current_stage_index is not None:
            state["current_stage_index"] = current_stage_index
            updates["current_stage_index"] = current_stage_index
            # Calculate percent based on stage index
            total = state["progress"]["total_stages"]
            if total > 0:
                state["progress"]["percent"] = (current_stage_index / total) * 100
                updates["progress"] = state["progress"]
        
        if completed_stages is not None:
            state["completed_stages"] = completed_stages
            state["progress"]["completed_stages"] = len(completed_stages)
            updates["completed_stages"] = completed_stages
            updates["progress"] = state["progress"]
        
        if percent is not None:
            state["progress"]["percent"] = percent
            updates["progress"] = state["progress"]
    
    # Persist to database
    if updates:
        _sync_to_db(pipeline_id, updates)


def _update_pipeline_completion(pipeline_id: str, exit_code: int, stages: List[str]) -> None:
    """Update pipeline on completion in memory and database"""
    with _pipeline_lock:
        if pipeline_id in _active_pipelines:
            state = _active_pipelines[pipeline_id]
            final_status = "completed" if exit_code == 0 else "failed"
            state["status"] = final_status
            state["exit_code"] = exit_code
            state["completed_at"] = datetime.utcnow().isoformat() + "Z"
            state["completed_timestamp"] = time.time()
            duration = state["completed_timestamp"] - state["started_timestamp"]
            state["duration_seconds"] = duration
            state["completed_stages"] = (
                stages if exit_code == 0 else state.get("completed_stages", [])
            )
            state["progress"]["completed_stages"] = (
                len(stages) if exit_code == 0 else state["progress"]["completed_stages"]
            )
            state["progress"]["percent"] = (
                100.0 if exit_code == 0 else state["progress"]["percent"]
            )
            
            # Track observability metrics
            pipeline_type = state.get("pipeline", "unknown")
            tracker = _get_metrics_tracker()
            if exit_code == 0:
                tracker.pipeline_completed(pipeline_type, duration)
                # Track per-stage completion
                for stage_name in state.get("completed_stages", []):
                    tracker.stage_completed(stage_name, 0)  # Individual stage durations not tracked here
            else:
                tracker.pipeline_failed(pipeline_type, "exit_code_nonzero")
            
            # Create Grafana annotation for pipeline completion/failure
            started_at = datetime.fromisoformat(state["started_at"].replace("Z", "+00:00"))
            completed_at = datetime.fromisoformat(state["completed_at"].replace("Z", "+00:00"))
            _create_pipeline_annotation(
                event=final_status,
                pipeline_id=pipeline_id,
                pipeline_type=pipeline_type,
                details=state.get("error") if exit_code != 0 else f"Duration: {duration:.1f}s",
                started_at=started_at,
                completed_at=completed_at,
            )
            
            # Record to historical metrics for trend analysis
            try:
                from .history import get_historical_metrics
                hm = get_historical_metrics()
                hm.record_execution(
                    pipeline_id=pipeline_id,
                    pipeline=pipeline_type,
                    status=final_status,
                    duration_seconds=duration,
                    cost_usd=0,  # Will be enhanced when LLM cost metrics available
                    tokens_used=0,  # Will be enhanced when token metrics available
                    documents_processed=state.get("documents_processed", 0),
                    stages=stages,
                    stage_durations=state.get("stage_durations", {}),
                    error=state.get("error"),
                )
            except Exception as e:
                logger.warning(f"[{pipeline_id}] Failed to record historical metrics: {e}")
            
            # Persist to database
            _sync_to_db(pipeline_id, state)


def _update_pipeline_error(pipeline_id: str, error: str, stage: Optional[str]) -> None:
    """Update pipeline on error in memory and database"""
    with _pipeline_lock:
        if pipeline_id in _active_pipelines:
            state = _active_pipelines[pipeline_id]
            state["status"] = "error"
            state["error"] = error
            state["error_stage"] = stage
            state["completed_at"] = datetime.utcnow().isoformat() + "Z"
            state["completed_timestamp"] = time.time()
            duration = state["completed_timestamp"] - state["started_timestamp"]
            state["duration_seconds"] = duration
            
            # Track observability metrics
            pipeline_type = state.get("pipeline", "unknown")
            tracker = _get_metrics_tracker()
            error_type = type(error).__name__ if hasattr(error, '__class__') else "unknown"
            tracker.pipeline_failed(pipeline_type, error_type)
            
            # Track failed stage if specified
            if stage:
                tracker.stage_failed(stage, error_type)
            
            # Create Grafana annotation for pipeline error
            started_at = datetime.fromisoformat(state["started_at"].replace("Z", "+00:00"))
            completed_at = datetime.fromisoformat(state["completed_at"].replace("Z", "+00:00"))
            error_details = f"Stage: {stage}\nError: {error}" if stage else f"Error: {error}"
            _create_pipeline_annotation(
                event="failed",
                pipeline_id=pipeline_id,
                pipeline_type=pipeline_type,
                details=error_details,
                started_at=started_at,
                completed_at=completed_at,
            )
            
            # Record to historical metrics for trend analysis
            try:
                from .history import get_historical_metrics
                hm = get_historical_metrics()
                hm.record_execution(
                    pipeline_id=pipeline_id,
                    pipeline=pipeline_type,
                    status="error",
                    duration_seconds=duration,
                    cost_usd=0,
                    tokens_used=0,
                    documents_processed=state.get("documents_processed", 0),
                    stages=state.get("stages", []),
                    stage_durations=state.get("stage_durations", {}),
                    error=error,
                )
            except Exception as e:
                logger.warning(f"[{pipeline_id}] Failed to record historical metrics: {e}")
            
            # Persist to database
            _sync_to_db(pipeline_id, state)


# ============================================
# Module Initialization
# ============================================

# Note: We don't call recover_state_from_db() at module load time
# because it may happen before MongoDB is available.
# Instead, we call it lazily on first API call.
