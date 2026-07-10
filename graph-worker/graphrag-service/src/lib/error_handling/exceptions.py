"""
Exception Hierarchy for Error Handling Library.

Provides custom exceptions with context support and clear error messages.
Part of the CORE libraries - error_handling library.
"""

from typing import Any, Dict, Optional


class ApplicationError(Exception):
    """Base exception for all application errors.

    Provides context support and cause chaining to prevent empty error messages.

    Attributes:
        message: Human-readable error message
        context: Dict of contextual information (stage, chunk_id, etc.)
        cause: Original exception that caused this error (if any)

    Example:
        raise ApplicationError(
            "Failed to process chunk",
            context={'chunk_id': '123', 'stage': 'extraction'},
            cause=original_exception
        )

        # Results in:
        # "Failed to process chunk [Context: chunk_id=123, stage=extraction]
        #  [Cause: ValueError: Invalid data]"
    """

    def __init__(
        self,
        message: str,
        context: Optional[Dict[str, Any]] = None,
        cause: Optional[Exception] = None,
    ):
        """Initialize ApplicationError with context and cause.

        Args:
            message: Error message describing what failed
            context: Optional dict of contextual information
            cause: Optional original exception that caused this error
        """
        self.message = message
        self.context = context or {}
        self.cause = cause

        # Call parent with formatted message
        super().__init__(self._format_message())

    def _format_message(self) -> str:
        """Format complete error message with context and cause.

        Returns:
            Formatted error message string
        """
        msg = self.message

        # Add context if present
        if self.context:
            context_items = ", ".join(f"{k}={v}" for k, v in self.context.items())
            msg += f" [Context: {context_items}]"

        # Add cause if present
        if self.cause:
            cause_type = type(self.cause).__name__
            cause_msg = str(self.cause) or "(no message)"
            msg += f" [Cause: {cause_type}: {cause_msg}]"

        return msg

    def add_context(self, **kwargs: Any) -> "ApplicationError":
        """Add additional context to the error.

        Args:
            **kwargs: Additional context key-value pairs

        Returns:
            Self for chaining
        """
        self.context.update(kwargs)
        # Update the exception message
        self.args = (self._format_message(),)
        return self


class StageError(ApplicationError):
    """Exception raised during stage execution.

    Used for errors in pipeline stages (extraction, resolution, construction, etc.).

    Example:
        raise StageError(
            "Entity resolution failed",
            context={'stage': 'entity_resolution', 'chunks_processed': 1000},
            cause=original_error
        )
    """

    pass


class AgentError(ApplicationError):
    """Exception raised during agent execution.

    Used for errors in LLM-powered agents.

    Example:
        raise AgentError(
            "LLM extraction failed",
            context={'agent': 'GraphExtractionAgent', 'chunk_id': '123'},
            cause=openai_error
        )
    """

    pass


class PipelineError(ApplicationError):
    """Exception raised during pipeline orchestration.

    Used for errors in pipeline execution and coordination.

    Example:
        raise PipelineError(
            "Pipeline stage failed",
            context={'pipeline': 'graphrag', 'stage': 'entity_resolution', 'exit_code': 1}
        )
    """

    pass


class ConfigurationError(ApplicationError):
    """Exception raised for configuration validation errors.

    Used when configuration is invalid or missing.

    Example:
        raise ConfigurationError(
            "Invalid database configuration",
            context={'db_name': None, 'expected': 'mongo_hack'}
        )
    """

    pass


class DatabaseError(ApplicationError):
    """Exception raised for database operation errors.

    Used when MongoDB operations fail.

    Example:
        raise DatabaseError(
            "Failed to write to collection",
            context={'collection': 'entities', 'operation': 'insert_one'},
            cause=pymongo_error
        )
    """

    pass


class LLMError(ApplicationError):
    """Exception raised for LLM operation errors.

    Used when OpenAI or other LLM calls fail.

    Example:
        raise LLMError(
            "LLM call failed after retries",
            context={'model': 'gpt-4o-mini', 'attempts': 3},
            cause=openai_error
        )
    """

    pass


# Convenience function for wrapping exceptions
def wrap_exception(
    message: str,
    original: Exception,
    error_class: type = ApplicationError,
    **context: Any,
) -> ApplicationError:
    """Wrap an exception with additional context.

    Args:
        message: New error message
        original: Original exception to wrap
        error_class: Exception class to use (default: ApplicationError)
        **context: Additional context key-value pairs

    Returns:
        New exception with original as cause

    Example:
        try:
            risky_operation()
        except Exception as e:
            raise wrap_exception(
                "Operation failed",
                e,
                error_class=StageError,
                stage='extraction',
                chunk_id='123'
            )
    """
    return error_class(message, context=context, cause=original)


def format_exception_message(exception: Exception) -> str:
    """Format exception as 'ExceptionType: message'.

    Handles empty exception messages gracefully.

    Args:
        exception: Exception to format

    Returns:
        Formatted string like "ValueError: Invalid data" or "KeyError: (no message)"
    """
    error_type = type(exception).__name__
    error_msg = str(exception) or "(no message)"
    return f"{error_type}: {error_msg}"
