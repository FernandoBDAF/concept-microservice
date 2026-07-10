"""
Logging Library - Cross-Cutting Concern.

Provides centralized, structured logging with context propagation.
Part of the CORE libraries - Tier 1 (full implementation).

Usage:
    from src.lib.logging import setup_logging, get_logger, set_log_context

    # Setup at application start
    setup_logging(verbose=True, log_file='logs/app.log')

    # In any module
    logger = get_logger(__name__)
    set_log_context(session_id='abc-123')
    logger.info("Processing query")  # Includes session_id
"""

from src.lib.logging.setup import (
    setup_logging,
    get_logger,
    create_timestamped_log_path,
    configure_logger_for_component,
    setup_session_logger,
)

from src.lib.logging.context import (
    set_log_context,
    get_log_context,
    clear_log_context,
    add_to_log_context,
    get_context_logger,
    with_session_context,
    with_request_context,
    with_trace_context,
    ContextLoggerAdapter,
)

from src.lib.logging.formatters import (
    JSONFormatter,
    ColoredFormatter,
    CompactFormatter,
)

from src.lib.logging.loki_formatter import (
    LokiFormatter,
)

from src.lib.logging.operations import (
    log_operation_start,
    log_operation_context,
    log_operation_complete,
    log_operation_progress,
)

from src.lib.logging.exceptions import (
    log_exception,
    format_exception_for_log,
)


__all__ = [
    # Setup
    "setup_logging",
    "get_logger",
    "create_timestamped_log_path",
    "configure_logger_for_component",
    "setup_session_logger",
    # Context
    "set_log_context",
    "get_log_context",
    "clear_log_context",
    "add_to_log_context",
    "get_context_logger",
    "with_session_context",
    "with_request_context",
    "with_trace_context",
    "ContextLoggerAdapter",
    # Formatters
    "JSONFormatter",
    "ColoredFormatter",
    "CompactFormatter",
    "LokiFormatter",
    # Operations
    "log_operation_start",
    "log_operation_context",
    "log_operation_complete",
    "log_operation_progress",
    # Exceptions
    "log_exception",
    "format_exception_for_log",
]
