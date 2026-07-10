"""
Exception Logging Helpers.

Provides helpers for logging exceptions with guaranteed visibility.
Part of the CORE libraries - logging library.
"""

import logging
from typing import Any, Optional


def log_exception(
    logger: logging.Logger,
    message: str,
    exception: Exception,
    include_traceback: bool = True,
    context: Optional[dict] = None,
    track_metric: bool = True,
) -> None:
    """Log exception with guaranteed type and message visibility.

    Ensures exception type is ALWAYS shown, preventing empty error messages.
    Also automatically tracks error metrics (if enabled).

    Args:
        logger: Logger instance to use
        message: Descriptive message about what failed
        exception: The exception that occurred
        include_traceback: Include full traceback (default: True)
        context: Optional dict of contextual information
        track_metric: Auto-track error in metrics (default: True)

    Example:
        try:
            risky_operation()
        except Exception as e:
            log_exception(logger, "Operation failed", e)

        # Logs AND tracks metric:
        # 1. "ERROR - Operation failed: ValueError: Invalid data [Full traceback]"
        # 2. Increments errors_total{error_type="ValueError"}

        # With context:
        log_exception(logger, "Stage failed", e, context={'stage': 'extraction'})
        # Tracks: errors_total{error_type="StageError", component="extraction"}
    """
    # CRITICAL: Always capture exception type (prevents empty messages!)
    error_type = type(exception).__name__
    error_msg = str(exception) or "(no message)"

    # Build full message
    full_msg = f"{message}: {error_type}: {error_msg}"

    # Add context if provided
    if context:
        context_str = ", ".join(f"{k}={v}" for k, v in context.items())
        full_msg += f" [Context: {context_str}]"

    # Log with or without traceback
    if include_traceback:
        logger.error(full_msg, exc_info=True)
    else:
        logger.error(full_msg)

    # Auto-track error metric (INTEGRATION with metrics library)
    if track_metric:
        try:
            from src.lib.metrics import MetricRegistry

            registry = MetricRegistry.get_instance()
            error_counter = registry.get("errors_total")

            # If errors_total counter exists, track this error
            if error_counter:
                # Extract component from logger name (e.g., "stages.extraction" -> "extraction")
                component = (
                    logger.name.split(".")[-1] if "." in logger.name else logger.name
                )

                error_counter.inc(
                    labels={"error_type": error_type, "component": component}
                )
        except Exception:
            # Don't fail if metrics tracking fails
            pass


def format_exception_for_log(exception: Exception) -> str:
    """Format exception as 'ExceptionType: message' for logging.

    Handles empty exception messages gracefully.

    Args:
        exception: Exception to format

    Returns:
        Formatted string like "ValueError: Invalid data" or "KeyError: (no message)"
    """
    error_type = type(exception).__name__
    error_msg = str(exception) or "(no message)"
    return f"{error_type}: {error_msg}"
