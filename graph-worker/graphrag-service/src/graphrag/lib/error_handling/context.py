"""
Error Context Management.

Provides context managers for enriching exceptions with operation context.
Part of the CORE libraries - error_handling library.
"""

import logging
from typing import Any, Dict, Optional


logger = logging.getLogger(__name__)


class error_context:
    """Context manager for automatic error enrichment.

    Wraps a block of code and automatically adds context to any exception
    that occurs, making debugging much easier.

    Example:
        with error_context("entity_resolution", chunks_processed=1000, stage="entity_resolution"):
            process_entities()

        # If error occurs:
        # "[ERROR] Exception in entity_resolution: ValueError: Invalid data
        #   chunks_processed: 1000
        #   stage: entity_resolution
        # [Full traceback]"
    """

    def __init__(self, operation: str, **context: Any):
        """Initialize error context.

        Args:
            operation: Name of the operation being performed
            **context: Additional context key-value pairs
        """
        self.operation = operation
        self.context = context

    def __enter__(self):
        """Enter context - no setup needed."""
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        """Exit context - log error if one occurred.

        Args:
            exc_type: Exception type (if any)
            exc_val: Exception value (if any)
            exc_tb: Exception traceback (if any)

        Returns:
            False to re-raise exception (don't suppress)
        """
        if exc_type is not None:
            # Build comprehensive error message
            error_type = exc_type.__name__
            error_msg = str(exc_val) or "(no message)"

            msg = f"Exception in {self.operation}: {error_type}: {error_msg}"

            # Add all context fields
            if self.context:
                msg += "\n  Context:"
                for key, value in self.context.items():
                    msg += f"\n    {key}: {value}"

            # Log with full traceback
            logger.error(msg, exc_info=True)

        # Don't suppress exception - let it propagate
        return False


class stage_context(error_context):
    """Specialized context manager for pipeline stages.

    Convenience wrapper that automatically adds stage-specific context.

    Example:
        with stage_context("graph_extraction", chunks_to_process=100):
            for chunk in chunks:
                process(chunk)
    """

    def __init__(self, stage_name: str, **context: Any):
        """Initialize stage context.

        Args:
            stage_name: Name of the stage
            **context: Additional context
        """
        context["stage"] = stage_name
        super().__init__(f"stage_{stage_name}", **context)


class agent_context(error_context):
    """Specialized context manager for agent operations.

    Convenience wrapper for LLM agent operations.

    Example:
        with agent_context("GraphExtractionAgent", chunk_id="abc-123"):
            result = agent.extract(chunk)
    """

    def __init__(self, agent_name: str, **context: Any):
        """Initialize agent context.

        Args:
            agent_name: Name of the agent
            **context: Additional context
        """
        context["agent"] = agent_name
        super().__init__(f"agent_{agent_name}", **context)


class db_operation_context(error_context):
    """Specialized context manager for database operations.

    Example:
        with db_operation_context("insert_entities", collection="entities", count=100):
            db.entities.insert_many(documents)
    """

    def __init__(self, operation: str, **context: Any):
        """Initialize database operation context.

        Args:
            operation: Database operation name
            **context: Additional context (collection, query, etc.)
        """
        super().__init__(f"db_{operation}", **context)


# NOTE: log_operation_context() and log_operation_complete() have been moved to
# src.lib.logging.operations for proper separation of concerns.
# Logging library owns all log output functionality.
