"""
Stages API - Main Entry Point

This module provides the public API functions that can be:
1. Called directly from Python
2. Exposed via HTTP server
3. Integrated with existing API infrastructure

Reference: API_DESIGN_SPECIFICATION.md Section 3.1
"""

import logging
import os
from datetime import datetime
from typing import Any, Dict, List, Optional, Tuple

from .metadata import (
    list_stages,
    get_stage_config,
    get_stage_defaults,
    list_pipeline_stages,
    clear_metadata_cache,
)
from .validation import validate_pipeline_config, validate_stage_config_only
from .execution import (
    execute_pipeline,
    get_pipeline_status,
    cancel_pipeline,
    list_active_pipelines,
    get_pipeline_history,
)
from .progress import (
    get_progress,
    update_progress,
    get_all_active_progress,
)
from .stats import (
    get_all_stage_stats,
    get_stage_stats,
)
from .control import (
    pause_pipeline,
    resume_pipeline,
    get_pause_status,
)
from .metrics import get_prometheus_metrics
from .insights import get_insights_engine
from .management import (
    inspect_databases,
    copy_collection,
    clean_graphrag_data,
    clean_stage_status,
    setup_test_database,
    rebuild_indexes,
    get_operation_status,
)

from .viewer import (
    get_database_catalog,
    list_collections as viewer_list_collections,
    get_document,
    query_collection,
    get_collection_schema,
)

from .iteration import (
    compare_documents,
    get_document_timeline,
    suggest_rerun,
    get_run_history,
)

from .source_selection import (
    get_channels,
    get_playlists,
    get_videos_for_channel,
    preview_filter,
    list_saved_filters,
    get_saved_filter,
    save_filter,
    update_filter,
    delete_filter,
    duplicate_filter,
    resolve_filter_to_video_ids,
)

from .prompts import (
    list_prompts,
    get_prompt_detail,
    test_prompt,
    reload_prompts,
)

logger = logging.getLogger(__name__)


# ============================================================================
# Response Transformation Functions
# ============================================================================
# These functions transform internal validation results to match the
# frontend API contract defined in StagesUI/src/types/api.ts


def _transform_validation_result(result: Dict[str, Any]) -> Dict[str, Any]:
    """
    Transform validation result to match frontend API contract.
    
    Backend format:
        errors: List[Dict] with keys: code, message, stage, field, etc.
        warnings: List[Dict] with keys: code, message, stage, resolution, etc.
        execution_plan: {stages, unselected_prerequisites, ...}
    
    Frontend expects:
        errors: Record<string, string[]>  (stage_name -> error messages)
        warnings: string[]  (array of warning messages)
        execution_plan: {stages: string[], resolved_dependencies: string[]}
    """
    transformed = {
        "valid": result.get("valid", False),
        "errors": _transform_errors(result.get("errors", [])),
        "warnings": _transform_warnings(result.get("warnings", [])),
    }
    
    # Transform execution_plan if present
    exec_plan = result.get("execution_plan")
    if exec_plan:
        transformed["execution_plan"] = {
            "stages": exec_plan.get("stages", []),
            # Note: These are now just "unselected prerequisites" not "auto-included"
            # Kept as resolved_dependencies for API backward compatibility
            "resolved_dependencies": exec_plan.get("unselected_prerequisites", []),
        }
    
    return transformed


def _transform_errors(errors: List[Dict[str, Any]]) -> Dict[str, List[str]]:
    """
    Transform error list to Record<stage_name, error_messages[]>.
    
    Groups errors by stage and extracts human-readable messages.
    Errors without a stage are grouped under "_general".
    """
    grouped: Dict[str, List[str]] = {}
    
    for error in errors:
        # Get stage name (fallback to "_general" for non-stage errors)
        stage = error.get("stage", "_general")
        
        # Extract message - try multiple common keys
        message = (
            error.get("message")
            or error.get("msg")
            or error.get("error")
            or str(error)
        )
        
        # Initialize list if needed
        if stage not in grouped:
            grouped[stage] = []
        
        # Add message if not duplicate
        if message not in grouped[stage]:
            grouped[stage].append(message)
    
    return grouped


def _transform_warnings(warnings: List[Dict[str, Any]]) -> List[str]:
    """
    Transform warning objects to array of warning messages.
    
    Includes resolution hint if available.
    """
    messages: List[str] = []
    
    for warning in warnings:
        # Build message with optional resolution
        message = warning.get("message", str(warning))
        resolution = warning.get("resolution")
        
        if resolution:
            full_message = f"{message} ({resolution})"
        else:
            full_message = message
        
        if full_message not in messages:
            messages.append(full_message)
    
    return messages


def _get_active_pipeline_count() -> int:
    """Get count of active pipelines for health check"""
    try:
        from .execution import list_active_pipelines
        result = list_active_pipelines()
        return result.get("count", 0)
    except Exception:
        return 0


# ============================================================================
# Metrics Query Functions
# ============================================================================


def _get_gauge_value(registry, name: str) -> float:
    """Get current gauge value from registry."""
    try:
        metric = registry.get(name)
        if metric:
            all_values = metric.get_all()
            if all_values:
                # Sum all labeled values
                return sum(all_values.values())
            return 0.0
    except Exception:
        pass
    return 0.0


def _get_counter_sum(registry, name: str) -> float:
    """Get sum of all counter values from registry."""
    try:
        metric = registry.get(name)
        if metric:
            all_values = metric.get_all()
            if all_values:
                return sum(all_values.values())
    except Exception:
        pass
    return 0.0


def _get_counter_by_label(registry, name: str, label_key: str, label_value: str) -> float:
    """Get counter value for specific label."""
    try:
        metric = registry.get(name)
        if metric:
            all_values = metric.get_all()
            for labels, value in all_values.items():
                if labels:
                    label_dict = dict(labels)
                    if label_dict.get(label_key) == label_value:
                        return value
    except Exception:
        pass
    return 0.0


def _calculate_success_rate(registry) -> float:
    """Calculate pipeline success rate from metrics."""
    try:
        metric = registry.get("stages_api_pipeline_executions")
        if metric:
            all_values = metric.get_all()
            completed = 0.0
            total = 0.0
            for labels, value in all_values.items():
                total += value
                if labels:
                    label_dict = dict(labels)
                    if label_dict.get("status") == "completed":
                        completed += value
            if total > 0:
                return round((completed / total) * 100, 1)
    except Exception:
        pass
    return 100.0


def _get_stage_breakdown(registry, pipeline_id: Optional[str] = None) -> Dict[str, Any]:
    """Get per-stage metrics breakdown."""
    stages = {}
    try:
        # Get stage execution counts
        executions = registry.get("stages_api_stage_executions")
        if executions:
            for labels, value in executions.get_all().items():
                if labels:
                    label_dict = dict(labels)
                    stage_name = label_dict.get("stage", "unknown")
                    status = label_dict.get("status", "unknown")
                    
                    if stage_name not in stages:
                        stages[stage_name] = {
                            "duration": 0,
                            "executions": 0,
                            "errors": 0,
                            "status": "unknown"
                        }
                    
                    stages[stage_name]["executions"] += int(value)
                    if status == "failed":
                        stages[stage_name]["errors"] += int(value)
                    stages[stage_name]["status"] = status
        
        # Get stage durations
        durations = registry.get("stages_api_stage_duration_seconds")
        if durations:
            for labels, _ in durations.get_all().items():
                if labels:
                    label_dict = dict(labels)
                    stage_name = label_dict.get("stage", "unknown")
                    stats = durations.summary(labels=label_dict)
                    
                    if stage_name in stages:
                        stages[stage_name]["duration"] = stats.get("avg", 0)
                    else:
                        stages[stage_name] = {
                            "duration": stats.get("avg", 0),
                            "executions": 0,
                            "errors": 0,
                            "status": "unknown"
                        }
    except Exception:
        pass
    
    return stages


def _get_errors_by_type(registry) -> Dict[str, int]:
    """Get error counts by type."""
    errors = {}
    try:
        metric = registry.get("errors_total")
        if metric:
            for labels, value in metric.get_all().items():
                if labels:
                    label_dict = dict(labels)
                    error_type = label_dict.get("error_type", "unknown")
                    errors[error_type] = int(value)
    except Exception:
        pass
    return errors


# ============================================================================
# Historical Context Helper Functions
# ============================================================================


def _get_historical_context(pipeline_type: str, limit: int = 10) -> Dict[str, Any]:
    """
    Get historical context from history.py for comparison.
    
    Args:
        pipeline_type: Pipeline type (e.g., 'ingestion', 'graphrag')
        limit: Number of recent executions to consider
        
    Returns:
        Historical context with stats for duration, cost, tokens
    """
    try:
        from .history import get_historical_metrics
        hm = get_historical_metrics()
        return hm.get_historical_context(pipeline_type, limit)
    except Exception as e:
        logger.warning(f"Failed to get historical context: {e}")
        return {
            "sample_size": 0,
            "duration": {"avg": 0, "min": 0, "max": 0, "p90": 0},
            "cost": {"avg": 0, "min": 0, "max": 0, "p90": 0},
            "tokens": {"avg": 0, "min": 0, "max": 0, "p90": 0},
            "error": str(e),
        }


def _compare_to_average(current: float, avg: float) -> Dict[str, Any]:
    """
    Compare current value to historical average.
    
    Args:
        current: Current value
        avg: Historical average
        
    Returns:
        Dictionary with trend and percent_diff
    """
    if avg == 0:
        return {"trend": "unknown", "percent_diff": 0}
    
    percent_diff = ((current - avg) / avg) * 100
    
    if percent_diff < -10:
        trend = "faster"
    elif percent_diff > 10:
        trend = "slower"
    else:
        trend = "normal"
    
    return {"trend": trend, "percent_diff": round(percent_diff, 1)}


def _calculate_percentile(current: float, historical_stats: Dict[str, float]) -> int:
    """
    Calculate which percentile the current value falls into.
    
    Args:
        current: Current value
        historical_stats: Dictionary with avg, min, max, p90
        
    Returns:
        Percentile rank (0-100, higher is better for duration)
    """
    min_val = historical_stats.get("min", 0)
    max_val = historical_stats.get("max", 0)
    p90 = historical_stats.get("p90", 0)
    
    if min_val == max_val or max_val == 0:
        return 50  # Default if no variance
    
    if current <= min_val:
        return 100  # Best
    if current >= max_val:
        return 0  # Worst
    if current <= p90:
        return 90  # Within 90th percentile
    
    # Linear interpolation between min and max
    return max(0, min(100, int(100 - ((current - min_val) / (max_val - min_val)) * 100)))


def _generate_basic_insights(
    state: Dict[str, Any],
    historical: Dict[str, Any],
    comparison: Dict[str, Any],
) -> List[Dict[str, Any]]:
    """
    Generate insights using the InsightsEngine.
    
    Uses the full InsightsEngine for rule-based analysis including:
    - Slow stage detection
    - High cost warnings
    - Retry rate analysis
    - Throughput recommendations
    - Token efficiency checks
    - Error pattern detection
    
    Falls back to basic comparison insights if engine fails.
    
    Args:
        state: Current pipeline state
        historical: Historical context
        comparison: Comparison result from _compare_to_average
        
    Returns:
        List of insight dictionaries
    """
    insights = []
    
    # Try to use the full InsightsEngine
    try:
        engine = get_insights_engine()
        
        # Build current metrics from state
        current_metrics = {
            "cost_usd": state.get("metrics", {}).get("cost_usd", 0),
            "tokens_used": state.get("metrics", {}).get("tokens_used", 0),
            "documents_processed": state.get("progress", {}).get("documents_processed", 0),
            "duration_seconds": state.get("elapsed_seconds", 0),
            "retries": state.get("metrics", {}).get("retries", 0),
            "operations": state.get("metrics", {}).get("operations", 0),
            "errors_by_type": state.get("metrics", {}).get("errors_by_type", {}),
        }
        
        # Get stage durations
        stage_durations = state.get("stage_durations", {})
        
        # Get config from state
        config = state.get("config", {})
        
        # Generate insights using the engine
        engine_insights = engine.generate_insights(
            current_metrics=current_metrics,
            historical_context=historical,
            stage_durations=stage_durations,
            config=config,
        )
        
        # Convert to dictionary format
        for insight in engine_insights:
            insights.append(insight.to_dict())
            
    except Exception as e:
        logger.debug(f"InsightsEngine failed, using basic insights: {e}")
    
    # Always add basic comparison insights (augment engine results)
    trend = comparison.get("trend", "unknown")
    percent_diff = abs(comparison.get("percent_diff", 0))
    
    # Only add if not already covered by engine
    has_performance_insight = any(i.get("type") == "performance" for i in insights)
    
    if not has_performance_insight:
        if trend == "faster":
            insights.append({
                "type": "performance",
                "severity": "info",
                "title": "Faster than average",
                "message": f"Current run is {percent_diff:.0f}% faster than average",
                "suggestion": None,
                "config_change": None,
                "impact": None,
            })
        elif trend == "slower":
            insights.append({
                "type": "performance",
                "severity": "warning",
                "title": "Slower than average",
                "message": f"Current run is {percent_diff:.0f}% slower than average",
                "suggestion": "Check for resource contention or input data size",
                "config_change": None,
                "impact": None,
            })
    
    # Sample size insight
    sample_size = historical.get("sample_size", 0)
    if sample_size < 3:
        insights.append({
            "type": "info",
            "severity": "info",
            "title": "Limited historical data",
            "message": f"Only {sample_size} historical samples. Comparisons may not be accurate.",
            "suggestion": "Run more pipelines to build comparison data",
            "config_change": None,
            "impact": None,
        })
    
    return insights


def handle_metrics_query(pipeline_id: Optional[str] = None) -> Tuple[Dict[str, Any], int]:
    """
    Get metrics in JSON format for UI consumption.
    
    Reference: OBSERVABILITY_STAGESUI_OVERLAP_ANALYSIS.md Section 4.2.1
    """
    try:
        from src.lib.metrics import MetricRegistry
        
        registry = MetricRegistry.get_instance()
        
        response = {
            "timestamp": datetime.utcnow().isoformat() + "Z",
            "pipeline_id": pipeline_id,
            
            # Pipeline metrics
            "pipeline": {
                "active_count": int(_get_gauge_value(registry, "stages_api_active_pipelines")),
                "total_executions": int(_get_counter_sum(registry, "stages_api_pipeline_executions")),
                "success_rate": _calculate_success_rate(registry),
            },
            
            # LLM metrics (from core metrics)
            "llm": {
                "total_calls": int(_get_counter_sum(registry, "agent_llm_calls")),
                "total_cost_usd": _get_counter_sum(registry, "agent_llm_cost_usd"),
                "tokens": {
                    "prompt": int(_get_counter_by_label(registry, "agent_tokens_used", "token_type", "prompt")),
                    "completion": int(_get_counter_by_label(registry, "agent_tokens_used", "token_type", "completion")),
                    "total": int(_get_counter_by_label(registry, "agent_tokens_used", "token_type", "total")),
                }
            },
            
            # Processing metrics
            "processing": {
                "documents_processed": int(_get_counter_sum(registry, "documents_processed")),
                "documents_failed": int(_get_counter_sum(registry, "documents_failed")),
                "entities_extracted": int(_get_counter_sum(registry, "entities_extracted")),
                "relationships_created": int(_get_counter_sum(registry, "relationships_created")),
            },
            
            # Stage breakdown
            "stages": _get_stage_breakdown(registry, pipeline_id),
            
            # Error summary
            "errors": {
                "total": int(_get_counter_sum(registry, "errors_total")),
                "by_type": _get_errors_by_type(registry),
            }
        }
        
        return response, 200
        
    except Exception as e:
        logger.exception("Error fetching metrics")
        return {"error": "Failed to fetch metrics", "message": str(e)}, 500


__all__ = [
    # Metadata
    "list_stages",
    "get_stage_config",
    "get_stage_defaults",
    "list_pipeline_stages",
    "clear_metadata_cache",
    # Validation
    "validate_pipeline_config",
    "validate_stage_config_only",
    # Execution
    "execute_pipeline",
    "get_pipeline_status",
    "cancel_pipeline",
    "list_active_pipelines",
    "get_pipeline_history",
    # Progress
    "get_progress",
    "update_progress",
    "get_all_active_progress",
    # Stats
    "get_all_stage_stats",
    "get_stage_stats",
    # Control
    "pause_pipeline",
    "resume_pipeline",
    "get_pause_status",
    # HTTP handlers
    "handle_request",
]


def handle_request(method: str, path: str, body: Optional[Dict] = None) -> tuple:
    """
    Handle HTTP-like requests.

    This function routes requests to appropriate handlers and returns
    (response_data, status_code) tuples.

    Can be used by:
    - SimpleHTTPServer
    - Flask/FastAPI adapters
    - Direct Python calls

    Args:
        method: HTTP method (GET, POST)
        path: URL path (e.g., "/stages", "/stages/graph_extraction/config")
        body: Request body for POST requests

    Returns:
        Tuple of (response_dict, status_code)
    """
    try:
        # Remove leading slash
        path = path.lstrip("/")
        
        # Parse query parameters if present
        query_params = {}
        if "?" in path:
            path, query_string = path.split("?", 1)
            for param in query_string.split("&"):
                if "=" in param:
                    key, value = param.split("=", 1)
                    query_params[key] = value
        
        parts = path.split("/")

        # Route: GET /health
        if method == "GET" and path == "health":
            return {
                "status": "healthy",
                "version": "1.0.0",
                "timestamp": datetime.utcnow().isoformat() + "Z",
                "active_pipelines": _get_active_pipeline_count(),
            }, 200

        # Route: GET /metrics (JSON format for UI)
        if method == "GET" and path == "metrics":
            return handle_metrics_query()

        # Route: GET /metrics/{pipeline_id} (pipeline-specific JSON metrics)
        if method == "GET" and len(parts) == 2 and parts[0] == "metrics":
            pipeline_id = parts[1]
            return handle_metrics_query(pipeline_id)

        # Route: GET /stages
        if method == "GET" and path == "stages":
            return list_stages(), 200

        # Route: GET /stages/{pipeline}
        if method == "GET" and len(parts) == 2 and parts[0] == "stages":
            pipeline = parts[1]
            if pipeline in ["ingestion", "graphrag"]:
                return list_pipeline_stages(pipeline), 200
            # Otherwise, treat as stage name for /stages/{stage_name}/config

        # Route: GET /stages/{stage_name}/config
        if (
            method == "GET"
            and len(parts) == 3
            and parts[0] == "stages"
            and parts[2] == "config"
        ):
            stage_name = parts[1]
            try:
                return get_stage_config(stage_name), 200
            except ValueError as e:
                return {"error": str(e), "stage_name": stage_name}, 404

        # Route: GET /stages/{stage_name}/defaults
        if (
            method == "GET"
            and len(parts) == 3
            and parts[0] == "stages"
            and parts[2] == "defaults"
        ):
            stage_name = parts[1]
            try:
                return get_stage_defaults(stage_name), 200
            except ValueError as e:
                return {"error": str(e), "stage_name": stage_name}, 404

        # Route: POST /pipelines/validate
        if method == "POST" and path == "pipelines/validate":
            if not body:
                return {"error": "Request body required"}, 400

            pipeline = body.get("pipeline")
            stages = body.get("stages", [])
            config = body.get("config", {})

            if not pipeline:
                return {"error": "pipeline field is required"}, 400
            if not stages:
                return {"error": "stages field is required"}, 400

            result = validate_pipeline_config(pipeline, stages, config)
            return _transform_validation_result(result), 200

        # Route: POST /stages/{stage_name}/validate
        if (
            method == "POST"
            and len(parts) == 3
            and parts[0] == "stages"
            and parts[2] == "validate"
        ):
            stage_name = parts[1]
            if not body:
                return {"error": "Request body required"}, 400

            result = validate_stage_config_only(stage_name, body)
            return _transform_validation_result(result), 200

        # Route: POST /pipelines/execute
        if method == "POST" and path == "pipelines/execute":
            if not body:
                return {"error": "Request body required"}, 400

            pipeline = body.get("pipeline")
            stages = body.get("stages", [])
            config = body.get("config", {})
            metadata = body.get("metadata", {})

            if not pipeline:
                return {"error": "pipeline field is required"}, 400
            if not stages:
                return {"error": "stages field is required"}, 400

            result = execute_pipeline(pipeline, stages, config, metadata)

            if "error" in result:
                return result, 400
            return result, 202  # Accepted

        # Route: GET /pipelines/{pipeline_id}/status
        if (
            method == "GET"
            and len(parts) == 3
            and parts[0] == "pipelines"
            and parts[2] == "status"
        ):
            pipeline_id = parts[1]
            result = get_pipeline_status(pipeline_id)

            # Only return 404 if there's a real error message (not just null/None)
            if result.get("error") and result.get("pipeline_id") and result["error"] == "Pipeline not found":
                return result, 404
            
            # Enhance with historical context
            try:
                pipeline_type = result.get("pipeline", "unknown")
                elapsed = result.get("elapsed_seconds", 0)
                
                # Get historical context
                historical = _get_historical_context(pipeline_type, limit=10)
                
                # Calculate comparison
                avg_duration = historical.get("duration", {}).get("avg", 0)
                comparison = _compare_to_average(elapsed, avg_duration)
                
                # Calculate percentile
                percentile = _calculate_percentile(elapsed, historical.get("duration", {}))
                
                # Generate insights
                insights = _generate_basic_insights(result, historical, comparison)
                
                # Add context to response
                result["context"] = {
                    "historical": historical,
                    "comparison": {
                        "vs_average": comparison,
                        "percentile": percentile,
                    }
                }
                result["insights"] = insights
                
            except Exception as e:
                logger.warning(f"Failed to add historical context to status: {e}")
                result["context"] = {"error": str(e)}
                result["insights"] = []
            
            return result, 200

        # Route: POST /pipelines/{pipeline_id}/cancel
        if (
            method == "POST"
            and len(parts) == 3
            and parts[0] == "pipelines"
            and parts[2] == "cancel"
        ):
            pipeline_id = parts[1]
            result = cancel_pipeline(pipeline_id)

            if "error" in result:
                return result, 400
            return result, 200

        # Route: GET /pipelines/active
        if method == "GET" and path == "pipelines/active":
            return list_active_pipelines(), 200

        # Route: GET /pipelines/history
        if method == "GET" and path == "pipelines/history":
            limit = 10  # Could be parsed from query string
            return get_pipeline_history(limit), 200

        # ========================================
        # Pipeline Progress Routes (merged from app/api/pipeline_progress.py)
        # ========================================

        # Route: GET /pipelines/{pipeline_id}/progress
        if (
            method == "GET"
            and len(parts) == 3
            and parts[0] == "pipelines"
            and parts[2] == "progress"
        ):
            pipeline_id = parts[1]
            progress = get_progress(pipeline_id)
            if progress:
                return {"pipeline_id": pipeline_id, **progress}, 200
            return {"pipeline_id": pipeline_id, "message": "No progress data"}, 200

        # Route: GET /pipelines/progress/active
        if method == "GET" and path == "pipelines/progress/active":
            return {"pipelines": get_all_active_progress()}, 200

        # ========================================
        # Pipeline Stats Routes (merged from app/api/pipeline_stats.py)
        # ========================================

        # Route: GET /pipelines/stats
        if method == "GET" and path == "pipelines/stats":
            db_name = body.get("db_name") if body else None
            return get_all_stage_stats(db_name=db_name), 200

        # Route: GET /pipelines/stats/{stage_name}
        if (
            method == "GET"
            and len(parts) == 3
            and parts[0] == "pipelines"
            and parts[1] == "stats"
        ):
            stage_name = parts[2]
            db_name = body.get("db_name") if body else None
            return get_stage_stats(stage_name, db_name=db_name), 200

        # ========================================
        # Pipeline Control Routes (merged from app/api/pipeline_control.py)
        # ========================================

        # Route: POST /pipelines/{pipeline_id}/pause
        if (
            method == "POST"
            and len(parts) == 3
            and parts[0] == "pipelines"
            and parts[2] == "pause"
        ):
            pipeline_id = parts[1]
            return pause_pipeline(pipeline_id), 200

        # Route: POST /pipelines/{pipeline_id}/resume
        if (
            method == "POST"
            and len(parts) == 3
            and parts[0] == "pipelines"
            and parts[2] == "resume"
        ):
            pipeline_id = parts[1]
            return resume_pipeline(pipeline_id), 200

        # Route: GET /pipelines/{pipeline_id}/pause-status
        if (
            method == "GET"
            and len(parts) == 3
            and parts[0] == "pipelines"
            and parts[2] == "pause-status"
        ):
            pipeline_id = parts[1]
            return get_pause_status(pipeline_id), 200

        # ========================================
        # Metrics Routes (for Prometheus scraping)
        # ========================================

        # Route: GET /metrics
        if method == "GET" and path == "metrics":
            metrics_text = get_prometheus_metrics()
            return {"format": "prometheus", "data": metrics_text}, 200

        # ========================================
        # Observability Health Proxy Routes
        # ========================================
        # These endpoints proxy health checks to avoid CORS issues in browsers

        # Route: GET /observability/health
        if method == "GET" and path == "observability/health":
            import urllib.request
            import urllib.error
            
            prometheus_url = os.environ.get("PROMETHEUS_URL", "http://localhost:9090")
            grafana_url = os.environ.get("GRAFANA_URL", "http://localhost:3000")
            loki_url = os.environ.get("LOKI_URL", "http://localhost:3100")
            
            health_status = {
                "prometheus": {"status": "unknown", "url": prometheus_url},
                "grafana": {"status": "unknown", "url": grafana_url},
                "loki": {"status": "unknown", "url": loki_url},
            }
            
            # Check Prometheus
            try:
                req = urllib.request.Request(f"{prometheus_url}/-/healthy", method="GET")
                with urllib.request.urlopen(req, timeout=3) as resp:
                    health_status["prometheus"]["status"] = "healthy" if resp.status == 200 else "degraded"
            except Exception as e:
                health_status["prometheus"]["status"] = "down"
                health_status["prometheus"]["error"] = str(e)
            
            # Check Grafana
            try:
                req = urllib.request.Request(f"{grafana_url}/api/health", method="GET")
                with urllib.request.urlopen(req, timeout=3) as resp:
                    health_status["grafana"]["status"] = "healthy" if resp.status == 200 else "degraded"
            except Exception as e:
                health_status["grafana"]["status"] = "down"
                health_status["grafana"]["error"] = str(e)
            
            # Check Loki
            try:
                req = urllib.request.Request(f"{loki_url}/ready", method="GET")
                with urllib.request.urlopen(req, timeout=3) as resp:
                    health_status["loki"]["status"] = "healthy" if resp.status == 200 else "degraded"
            except Exception as e:
                health_status["loki"]["status"] = "down"
                health_status["loki"]["error"] = str(e)
            
            # Calculate overall status
            statuses = [s["status"] for s in health_status.values()]
            if all(s == "healthy" for s in statuses):
                overall = "healthy"
            elif any(s == "down" for s in statuses):
                overall = "degraded"
            else:
                overall = "unknown"
            
            return {
                "overall": overall,
                "services": health_status,
                "timestamp": datetime.utcnow().isoformat() + "Z"
            }, 200

        # ========================================
        # Management Routes
        # ========================================

        # Route: GET /management/inspect-databases
        if method == "GET" and path == "management/inspect-databases":
            return inspect_databases()

        # Route: GET /management/operations/{id}
        if method == "GET" and path.startswith("management/operations/"):
            operation_id = path.split("/")[-1]
            return get_operation_status(operation_id)

        # Route: POST /management/copy-collection
        if method == "POST" and path == "management/copy-collection":
            if not body:
                return {"error": "Request body required"}, 400
            return copy_collection(
                source_db=body.get("source_db"),
                target_db=body.get("target_db"),
                collection=body.get("collection"),
                max_documents=body.get("max_documents"),
                clear_target=body.get("clear_target", True),
            )

        # Route: POST /management/clean-graphrag
        if method == "POST" and path == "management/clean-graphrag":
            if not body:
                return {"error": "Request body required"}, 400
            return clean_graphrag_data(
                db_name=body.get("db_name"),
                drop_collections=body.get("drop_collections"),
                clear_chunk_metadata=body.get("clear_chunk_metadata", True),
            )

        # Route: POST /management/clean-stage-status
        if method == "POST" and path == "management/clean-stage-status":
            if not body:
                return {"error": "Request body required"}, 400
            return clean_stage_status(
                db_name=body.get("db_name"),
                stages=body.get("stages", []),
            )

        # Route: POST /management/setup-test-db
        if method == "POST" and path == "management/setup-test-db":
            if not body:
                return {"error": "Request body required"}, 400
            return setup_test_database(
                source_db=body.get("source_db"),
                target_db=body.get("target_db"),
                chunk_count=body.get("chunk_count", 50),
                diversity_mode=body.get("diversity_mode", True),
            )

        # Route: POST /management/rebuild-indexes
        if method == "POST" and path == "management/rebuild-indexes":
            if not body:
                return {"error": "Request body required"}, 400
            return rebuild_indexes(
                db_name=body.get("db_name"),
                collections=body.get("collections"),
            )

        # ========================================
        # Viewer Routes
        # ========================================

        # Route: GET /viewer/databases
        if method == "GET" and path == "viewer/databases":
            return get_database_catalog()

        # Route: GET /viewer/collections/{db}
        if method == "GET" and path.startswith("viewer/collections/"):
            db_name = path.replace("viewer/collections/", "")
            return viewer_list_collections(db_name)

        # Route: GET /viewer/document/{db}/{collection}/{id}
        if method == "GET" and path.startswith("viewer/document/"):
            parts = path.replace("viewer/document/", "").split("/")
            if len(parts) != 3:
                return {"error": "Invalid path: expected viewer/document/{db}/{collection}/{id}"}, 400
            db_name, collection_name, doc_id = parts
            return get_document(db_name, collection_name, doc_id)

        # Route: POST /viewer/query
        if method == "POST" and path == "viewer/query":
            if not body:
                return {"error": "Request body required"}, 400
            return query_collection(
                db_name=body.get("db_name"),
                collection_name=body.get("collection_name"),
                filter_query=body.get("filter"),
                projection=body.get("projection"),
                sort=body.get("sort"),
                skip=body.get("skip", 0),
                limit=body.get("limit", 20),
            )

        # Route: GET /viewer/schema/{db}/{collection}
        if method == "GET" and path.startswith("viewer/schema/"):
            parts = path.replace("viewer/schema/", "").split("/")
            if len(parts) != 2:
                return {"error": "Invalid path: expected viewer/schema/{db}/{collection}"}, 400
            db_name, collection_name = parts
            return get_collection_schema(db_name, collection_name)

        # ========================================
        # Iteration API Routes
        # ========================================

        # Route: GET /viewer/compare/{db}/{collection}/{id1}/{id2}
        if method == "GET" and path.startswith("viewer/compare/"):
            parts = path.replace("viewer/compare/", "").split("/")
            if len(parts) != 4:
                return {"error": "Expected: viewer/compare/{db}/{collection}/{id1}/{id2}"}, 400
            db_name, collection_name, doc_id_1, doc_id_2 = parts
            return compare_documents(db_name, collection_name, doc_id_1, doc_id_2)

        # Route: GET /viewer/timeline/{db}/{collection}/{source_id}
        if method == "GET" and path.startswith("viewer/timeline/"):
            parts = path.replace("viewer/timeline/", "").split("/")
            if len(parts) < 3:
                return {"error": "Expected: viewer/timeline/{db}/{collection}/{source_id}"}, 400
            db_name, collection_name = parts[0], parts[1]
            source_id = "/".join(parts[2:])  # Allow slashes in source_id
            source_field = query_params.get("source_field")
            return get_document_timeline(db_name, collection_name, source_id, source_field)

        # Route: POST /viewer/suggest-rerun
        if method == "POST" and path == "viewer/suggest-rerun":
            if not body:
                return {"error": "Request body required"}, 400
            return suggest_rerun(
                db_name=body.get("db_name"),
                collection_name=body.get("collection_name"),
                doc_id=body.get("doc_id"),
                issue_type=body.get("issue_type"),
            )

        # Route: GET /viewer/run-history/{db}/{collection}/{doc_id}
        if method == "GET" and path.startswith("viewer/run-history/"):
            parts = path.replace("viewer/run-history/", "").split("/")
            if len(parts) != 3:
                return {"error": "Expected: viewer/run-history/{db}/{collection}/{doc_id}"}, 400
            db_name, collection_name, doc_id = parts
            limit = int(query_params.get("limit", 20))
            return get_run_history(db_name, collection_name, doc_id, limit)

        # ========================================
        # Source Selection Routes
        # ========================================

        # Route: GET /source-selection/channels/{db}
        if method == "GET" and path.startswith("source-selection/channels/"):
            db_name = path.replace("source-selection/channels/", "")
            return get_channels(db_name)

        # Route: GET /source-selection/playlists/{db}
        if method == "GET" and path.startswith("source-selection/playlists/"):
            db_name = path.replace("source-selection/playlists/", "")
            return get_playlists(db_name)

        # Route: GET /source-selection/videos/{db}/{channel_id}
        if method == "GET" and path.startswith("source-selection/videos/"):
            parts = path.replace("source-selection/videos/", "").split("/")
            if len(parts) != 2:
                return {"error": "Expected: /source-selection/videos/{db}/{channel_id}"}, 400
            db_name, channel_id = parts
            return get_videos_for_channel(db_name, channel_id)

        # Route: POST /source-selection/preview
        if method == "POST" and path == "source-selection/preview":
            if not body:
                return {"error": "Request body required"}, 400
            return preview_filter(
                db_name=body.get("db_name"),
                filter_def=body.get("filter", {}),
                sample_limit=body.get("sample_limit", 5),
            )

        # Route: GET /source-selection/filters/{db}
        if method == "GET" and path.startswith("source-selection/filters/") and path.count("/") == 2:
            db_name = path.replace("source-selection/filters/", "")
            return list_saved_filters(db_name)

        # Route: GET /source-selection/filters/{db}/{id}
        if method == "GET" and path.startswith("source-selection/filters/") and path.count("/") == 3:
            parts = path.replace("source-selection/filters/", "").split("/")
            if len(parts) == 2:
                db_name, filter_id = parts
                return get_saved_filter(db_name, filter_id)

        # Route: POST /source-selection/filters/{db}
        if method == "POST" and path.startswith("source-selection/filters/") and path.count("/") == 2:
            db_name = path.replace("source-selection/filters/", "")
            if not body:
                return {"error": "Request body required"}, 400
            return save_filter(
                db_name=db_name,
                name=body.get("name"),
                filter_definition=body.get("filter_definition"),
                description=body.get("description"),
            )

        # Route: PUT /source-selection/filters/{db}/{id}
        if method == "PUT" and path.startswith("source-selection/filters/"):
            parts = path.replace("source-selection/filters/", "").split("/")
            if len(parts) == 2:
                db_name, filter_id = parts
                if not body:
                    return {"error": "Request body required"}, 400
                return update_filter(db_name, filter_id, body)

        # Route: DELETE /source-selection/filters/{db}/{id}
        if method == "DELETE" and path.startswith("source-selection/filters/"):
            parts = path.replace("source-selection/filters/", "").split("/")
            if len(parts) == 2:
                db_name, filter_id = parts
                return delete_filter(db_name, filter_id)

        # Route: POST /source-selection/filters/{db}/{id}/duplicate
        if method == "POST" and path.endswith("/duplicate"):
            # source-selection/filters/{db}/{id}/duplicate
            path_without_suffix = path.replace("/duplicate", "")
            parts = path_without_suffix.replace("source-selection/filters/", "").split("/")
            if len(parts) == 2:
                db_name, filter_id = parts
                if not body:
                    return {"error": "Request body required"}, 400
                return duplicate_filter(db_name, filter_id, body.get("name"))

        # Route: POST /source-selection/resolve
        if method == "POST" and path == "source-selection/resolve":
            if not body:
                return {"error": "Request body required"}, 400
            return resolve_filter_to_video_ids(
                db_name=body.get("db_name"),
                filter_id=body.get("filter_id"),
                filter_definition=body.get("filter_definition"),
            )

        # ========================================
        # Prompt Management Routes
        # ========================================

        # Route: GET /prompts - List all prompts
        if method == "GET" and path == "prompts":
            return list_prompts()

        # Route: GET /prompts/{agent_type} - List prompts for specific agent
        if method == "GET" and path.startswith("prompts/") and not path.startswith("prompts/detail/"):
            agent_type = path.replace("prompts/", "")
            return list_prompts(agent_type=agent_type)

        # Route: GET /prompts/detail/{prompt_id} - Get full prompt details
        if method == "GET" and path.startswith("prompts/detail/"):
            prompt_id = path.replace("prompts/detail/", "")
            return get_prompt_detail(prompt_id)

        # Route: POST /prompts/{prompt_id}/test - Test prompt with sample input
        if method == "POST" and path.endswith("/test") and path.startswith("prompts/"):
            # Extract prompt_id from path: prompts/{prompt_id}/test
            prompt_id = path.replace("prompts/", "").replace("/test", "")
            if not body:
                return {"error": "Request body required"}, 400
            return test_prompt(
                prompt_id=prompt_id,
                test_input=body.get("test_input", {}),
                model_name=body.get("model_name"),
            )

        # Route: POST /prompts/reload - Force reload prompts from database
        if method == "POST" and path == "prompts/reload":
            return reload_prompts()

        # Not found
        return {"error": f"Unknown endpoint: {method} /{path}"}, 404

    except Exception as e:
        logger.exception(f"Error handling request: {method} /{path}")
        return {"error": "Internal server error", "message": str(e)}, 500

