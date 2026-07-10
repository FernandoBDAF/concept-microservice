"""
Error Handling Library - Cross-Cutting Concern.

Provides comprehensive error handling with context support, decorators, and context managers.
Part of the CORE libraries - Tier 1 (full implementation).

This library solves the "empty error message" problem by ensuring all exceptions
are logged with type, message, traceback, and contextual information.

Usage:
    # Import exceptions
    from src.lib.error_handling import ApplicationError, StageError, AgentError

    # Use decorators
    from src.lib.error_handling import handle_errors

    @handle_errors(log_traceback=True)
    def risky_operation():
        # Automatic error logging with full details
        ...

    # Use context managers
    from src.lib.error_handling import error_context

    with error_context("data_processing", chunk_id='123'):
        process_data()
        # If error: Shows operation name + context

    # Raise informative exceptions
    raise StageError(
        "Entity resolution failed",
        context={'chunks_processed': 1000},
        cause=original_error
    )

Key Features:
- Never have empty error messages (always shows exception type)
- Full tracebacks with exc_info=True
- Context preservation (what was being processed)
- Cause chaining (original exception preserved)
- Specialized exceptions (StageError, AgentError, etc.)
- Flexible decorators and context managers
"""

# Exception classes
from src.lib.error_handling.exceptions import (
    ApplicationError,
    StageError,
    AgentError,
    PipelineError,
    ConfigurationError,
    DatabaseError,
    LLMError,
    wrap_exception,
    format_exception_message,
)

# Decorators
from src.lib.error_handling.decorators import (
    handle_errors,
    handle_stage_errors,
    handle_agent_errors,
    log_and_suppress,
    require_success,
)

# Context managers
from src.lib.error_handling.context import (
    error_context,
    stage_context,
    agent_context,
    db_operation_context,
)


__all__ = [
    # Exceptions
    "ApplicationError",
    "StageError",
    "AgentError",
    "PipelineError",
    "ConfigurationError",
    "DatabaseError",
    "LLMError",
    "wrap_exception",
    # Decorators
    "handle_errors",
    "handle_stage_errors",
    "handle_agent_errors",
    "log_and_suppress",
    "require_success",
    # Context Managers
    "error_context",
    "stage_context",
    "agent_context",
    "db_operation_context",
    # Helper Functions
    "format_exception_message",
]
