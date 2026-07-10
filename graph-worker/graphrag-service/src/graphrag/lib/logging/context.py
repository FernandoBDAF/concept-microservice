"""
Logging Context Management.

Part of the CORE libraries - logging library.
Provides context propagation for distributed logging.
"""

import logging
from contextvars import ContextVar
from typing import Any, Dict, Optional


# Context variables for thread-safe context storage
_log_context: ContextVar[Dict[str, Any]] = ContextVar("log_context", default={})


def set_log_context(**kwargs: Any) -> None:
    """Set logging context for current execution context.

    Context is propagated to all log messages in the current async/thread context.

    Args:
        **kwargs: Context key-value pairs (e.g., request_id='123', session_id='abc')

    Example:
        set_log_context(request_id='req-123', user_id='user-456')
        # All subsequent logs in this context will include these fields
    """
    current = _log_context.get().copy()
    current.update(kwargs)
    _log_context.set(current)


def get_log_context() -> Dict[str, Any]:
    """Get current logging context.

    Returns:
        Dict of context key-value pairs
    """
    return _log_context.get().copy()


def clear_log_context() -> None:
    """Clear logging context for current execution context."""
    _log_context.set({})


def add_to_log_context(**kwargs: Any) -> None:
    """Add fields to existing logging context.

    Args:
        **kwargs: Additional context key-value pairs
    """
    current = _log_context.get().copy()
    current.update(kwargs)
    _log_context.set(current)


class ContextLoggerAdapter(logging.LoggerAdapter):
    """Logger adapter that automatically includes context in all log messages.

    Usage:
        logger = ContextLoggerAdapter(logging.getLogger(__name__))
        set_log_context(request_id='123')
        logger.info("Processing")  # Will include request_id in log
    """

    def process(self, msg: str, kwargs: Any) -> tuple:
        """Process log message to include context.

        Args:
            msg: Log message
            kwargs: Log kwargs

        Returns:
            Tuple of (message, kwargs) with context added
        """
        # Get current context
        ctx = get_log_context()

        # Add context to extra
        extra = kwargs.get("extra", {})
        extra.update(ctx)
        kwargs["extra"] = extra

        # If context exists, append to message for non-JSON formatters
        if ctx and not isinstance(logging.root.handlers[0].formatter, object):
            context_str = " ".join(f"{k}={v}" for k, v in ctx.items())
            msg = f"{msg} [{context_str}]"

        return msg, kwargs


def get_context_logger(name: str) -> ContextLoggerAdapter:
    """Get a context-aware logger.

    Args:
        name: Logger name (typically __name__)

    Returns:
        ContextLoggerAdapter that automatically includes context

    Example:
        logger = get_context_logger(__name__)
        set_log_context(session_id='abc')
        logger.info("User query")  # Includes session_id automatically
    """
    base_logger = logging.getLogger(name)
    return ContextLoggerAdapter(base_logger, {})


# Convenience function for common contexts
def with_session_context(session_id: str) -> None:
    """Set session context for logging.

    Args:
        session_id: Session identifier
    """
    set_log_context(session_id=session_id)


def with_request_context(request_id: str, user_id: Optional[str] = None) -> None:
    """Set request context for logging.

    Args:
        request_id: Request identifier
        user_id: Optional user identifier
    """
    ctx = {"request_id": request_id}
    if user_id:
        ctx["user_id"] = user_id
    set_log_context(**ctx)


def with_trace_context(trace_id: str, span_id: Optional[str] = None) -> None:
    """Set trace context for logging.

    Args:
        trace_id: Trace identifier
        span_id: Optional span identifier
    """
    ctx = {"trace_id": trace_id}
    if span_id:
        ctx["span_id"] = span_id
    set_log_context(**ctx)
