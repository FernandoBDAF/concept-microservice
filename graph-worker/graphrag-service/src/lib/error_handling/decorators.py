"""
Error Handling Decorators.

Provides decorators for automatic error handling, logging, and fallback behavior.
Part of the CORE libraries - error_handling library.
"""

import logging
import functools
from typing import Any, Callable, Optional, Tuple, Type


def handle_errors(
    fallback: Optional[Any] = None,
    log_traceback: bool = True,
    capture_context: bool = True,
    reraise: bool = True,
    log_level: int = logging.ERROR,
) -> Callable:
    """Decorator for comprehensive error handling.

    Ensures exceptions are ALWAYS logged with type and traceback,
    preventing the "empty error message" problem.

    Args:
        fallback: Value to return if error occurs and reraise=False (default: None)
        log_traceback: Include full traceback in logs (default: True)
        capture_context: Include function name/module in error message (default: True)
        reraise: Re-raise exception after logging (default: True)
        log_level: Logging level for errors (default: ERROR)

    Returns:
        Decorated function with error handling

    Example:
        @handle_errors(log_traceback=True, reraise=True)
        def risky_operation():
            # If this fails, get full error details in logs
            ...

        @handle_errors(fallback=[], reraise=False)
        def optional_operation():
            # If this fails, return [] and continue
            ...
    """

    def decorator(func: Callable) -> Callable:
        @functools.wraps(func)
        def wrapper(*args: Any, **kwargs: Any) -> Any:
            try:
                return func(*args, **kwargs)

            except Exception as e:
                # CRITICAL: Always capture exception type (prevents empty messages!)
                error_type = type(e).__name__
                error_msg = str(e) or "(no message)"
                full_msg = f"{error_type}: {error_msg}"

                # Add function context if requested
                if capture_context:
                    full_msg += f" [in {func.__module__}.{func.__name__}]"

                # Get logger for this module
                logger = logging.getLogger(func.__module__)

                # Log with or without traceback
                if log_traceback:
                    logger.log(log_level, full_msg, exc_info=True)
                else:
                    logger.log(log_level, full_msg)

                # Return fallback or re-raise
                if not reraise:
                    return fallback
                else:
                    raise

        return wrapper

    return decorator


def handle_stage_errors(stage_name: Optional[str] = None) -> Callable:
    """Specialized decorator for stage error handling.

    Convenience wrapper around handle_errors with stage-specific defaults.

    Args:
        stage_name: Optional stage name for context (default: use function name)

    Returns:
        Decorated function

    Example:
        @handle_stage_errors('graph_extraction')
        def run_stage():
            # Stage-specific error handling
            ...
    """

    def decorator(func: Callable) -> Callable:
        @functools.wraps(func)
        def wrapper(*args: Any, **kwargs: Any) -> Any:
            from src.lib.error_handling.exceptions import StageError

            try:
                return func(*args, **kwargs)
            except Exception as e:
                # Wrap in StageError with context
                name = stage_name or func.__name__
                raise StageError(
                    f"Stage '{name}' failed",
                    context={"stage": name, "function": func.__name__},
                    cause=e,
                )

        return wrapper

    return decorator


def handle_agent_errors(agent_name: Optional[str] = None) -> Callable:
    """Specialized decorator for agent error handling.

    Convenience wrapper for LLM agent errors.

    Args:
        agent_name: Optional agent name for context

    Returns:
        Decorated function

    Example:
        @handle_agent_errors('GraphExtractionAgent')
        def extract():
            # Agent-specific error handling
            ...
    """

    def decorator(func: Callable) -> Callable:
        @functools.wraps(func)
        def wrapper(*args: Any, **kwargs: Any) -> Any:
            from src.lib.error_handling.exceptions import AgentError

            try:
                return func(*args, **kwargs)
            except Exception as e:
                # Wrap in AgentError with context
                name = agent_name or func.__name__
                raise AgentError(
                    f"Agent '{name}' failed",
                    context={"agent": name, "function": func.__name__},
                    cause=e,
                )

        return wrapper

    return decorator


def log_and_suppress(
    fallback: Optional[Any] = None,
    log_level: int = logging.WARNING,
) -> Callable:
    """Decorator to log errors but continue execution.

    Useful for non-critical operations where you want to log errors
    but not stop the entire process.

    Args:
        fallback: Value to return on error (default: None)
        log_level: Logging level (default: WARNING)

    Returns:
        Decorated function

    Example:
        @log_and_suppress(fallback={})
        def fetch_optional_metadata():
            # If this fails, log warning and return {}
            ...
    """
    return handle_errors(
        fallback=fallback,
        log_traceback=True,
        capture_context=True,
        reraise=False,
        log_level=log_level,
    )


def require_success(error_class: Type[Exception] = Exception) -> Callable:
    """Decorator to ensure operation succeeds or raises specific error.

    Wraps any exception in the specified error class.

    Args:
        error_class: Exception class to raise on failure

    Returns:
        Decorated function

    Example:
        from src.lib.error_handling.exceptions import DatabaseError

        @require_success(DatabaseError)
        def critical_db_operation():
            # Any error becomes DatabaseError
            ...
    """

    def decorator(func: Callable) -> Callable:
        @functools.wraps(func)
        def wrapper(*args: Any, **kwargs: Any) -> Any:
            try:
                return func(*args, **kwargs)
            except error_class:
                # Already correct type, re-raise
                raise
            except Exception as e:
                # Wrap in specified error class
                from src.lib.error_handling.exceptions import wrap_exception

                raise wrap_exception(
                    f"{func.__name__} failed",
                    e,
                    error_class=error_class,
                    function=func.__name__,
                    module=func.__module__,
                )

        return wrapper

    return decorator
