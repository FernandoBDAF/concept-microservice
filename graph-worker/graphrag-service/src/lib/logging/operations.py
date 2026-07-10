"""
Operation Lifecycle Logging.

Provides helpers for logging operation start, completion, and progress.
Part of the CORE libraries - logging library.
"""

import logging
from typing import Any, Optional


logger = logging.getLogger(__name__)


def log_operation_start(operation: str, **context: Any) -> None:
    """Log the start of an operation with context.

    Useful for tracking operation flow and debugging.

    Args:
        operation: Operation name
        **context: Context key-value pairs

    Example:
        log_operation_start("graph_extraction", chunks=100, model="gpt-4o-mini")
        # Logs: "[OPERATION] Starting graph_extraction (chunks=100, model=gpt-4o-mini)"
    """
    if context:
        context_str = ", ".join(f"{k}={v}" for k, v in context.items())
        logger.info(f"[OPERATION] Starting {operation} ({context_str})")
    else:
        logger.info(f"[OPERATION] Starting {operation}")


# Alias for compatibility
log_operation_context = log_operation_start


def log_operation_complete(
    operation: str, duration: Optional[float] = None, **results: Any
) -> None:
    """Log the completion of an operation with results.

    Args:
        operation: Operation name
        duration: Optional duration in seconds
        **results: Result key-value pairs

    Example:
        log_operation_complete("graph_extraction", duration=61.5, entities=100, relationships=200)
        # Logs: "[OPERATION] Completed graph_extraction in 61.5s (entities=100, relationships=200)"
    """
    msg = f"[OPERATION] Completed {operation}"
    if duration is not None:
        msg += f" in {duration:.1f}s"

    if results:
        results_str = ", ".join(f"{k}={v}" for k, v in results.items())
        msg += f" ({results_str})"

    logger.info(msg)


def log_operation_progress(
    operation: str, current: int, total: int, **metrics: Any
) -> None:
    """Log operation progress.

    Args:
        operation: Operation name
        current: Current item number
        total: Total items
        **metrics: Additional metrics to log

    Example:
        log_operation_progress("graph_extraction", 100, 1000, processed=95, failed=5)
        # Logs: "[OPERATION] graph_extraction: 100/1000 (10%) processed=95 failed=5"
    """
    pct = int((current / total) * 100) if total > 0 else 0
    msg = f"[OPERATION] {operation}: {current}/{total} ({pct}%)"

    if metrics:
        metrics_str = " ".join(f"{k}={v}" for k, v in metrics.items())
        msg += f" {metrics_str}"

    logger.info(msg)
