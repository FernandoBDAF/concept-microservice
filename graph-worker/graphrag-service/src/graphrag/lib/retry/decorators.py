"""
Retry Decorators.

Provides decorators for automatic retry with configurable policies.
Part of the CORE libraries - retry library.
"""

import time
import logging
import functools
from typing import Callable, Optional, Tuple, Type

from src.lib.retry.policies import (
    RetryPolicy,
    ExponentialBackoff,
    DEFAULT_POLICY,
)


logger = logging.getLogger(__name__)


def _is_quota_error(error: Exception) -> bool:
    """
    Check if error is a quota/rate limit error that won't succeed on retry.

    OpenAI quota errors are terminal - they won't succeed on retry and should
    be detected immediately to avoid wasted API calls.

    Args:
        error: Exception to check

    Returns:
        True if this is a quota error that shouldn't be retried
    """
    # Check error type name
    error_type = type(error).__name__
    error_str = str(error).lower()

    # Check for OpenAI RateLimitError
    if error_type == "RateLimitError":
        # Check error message/content for quota-related errors
        if "insufficient_quota" in error_str:
            return True
        if "quota" in error_str and ("exceeded" in error_str or "limit" in error_str):
            return True

    # Check error attributes if available (OpenAI SDK structure)
    # OpenAI SDK RateLimitError may have error details in response or body
    if hasattr(error, "response"):
        try:
            # Try to get error body from response
            if hasattr(error.response, "json"):
                error_body = error.response.json()
                error_data = error_body.get("error", {})
                if error_data.get("code") == "insufficient_quota":
                    return True
                if error_data.get("type") == "insufficient_quota":
                    return True
        except Exception:
            pass

    # Also check if error has body attribute (some SDK versions)
    if hasattr(error, "body"):
        try:
            error_data = (
                error.body.get("error", {}) if isinstance(error.body, dict) else {}
            )
            if error_data.get("code") == "insufficient_quota":
                return True
        except Exception:
            pass

    return False


def _is_empty_entity_validation_error(error: Exception) -> bool:
    """
    Check if error is a ValidationError for empty entities (expected case, don't retry).

    This happens when the LLM correctly returns an empty entity list for chunks
    that have no extractable entities. Retrying won't help - it's the correct response.

    Args:
        error: Exception to check

    Returns:
        True if this is an empty entity validation error that shouldn't be retried
    """
    error_type = type(error).__name__
    error_msg = str(error)

    # Check if this is a ValidationError about empty entities
    if "ValidationError" in error_type:
        # Check error message for the specific empty entity validation message
        if "At least one entity must be identified" in error_msg:
            return True

        # Also check Pydantic ValidationError.errors() structure if available
        if hasattr(error, "errors"):
            try:
                errors_list = error.errors()
                if any(
                    "At least one entity must be identified" in str(err.get("msg", ""))
                    for err in errors_list
                ):
                    return True
            except (TypeError, AttributeError, Exception):
                pass

    return False


def with_retry(
    max_attempts: int = 3,
    backoff: str = "exponential",
    base_delay: float = 1.0,
    max_delay: float = 60.0,
    retry_on: Tuple[Type[Exception], ...] = (Exception,),
    policy: Optional[RetryPolicy] = None,
) -> Callable:
    """Decorator for automatic retry with configurable policy.

    Integrates with logging and metrics libraries.

    Args:
        max_attempts: Maximum retry attempts (default: 3)
        backoff: Backoff strategy - "exponential" or "fixed" (default: "exponential")
        base_delay: Base delay in seconds (default: 1.0)
        max_delay: Maximum delay for exponential backoff (default: 60.0)
        retry_on: Tuple of exception types to retry (default: all exceptions)
        policy: Custom RetryPolicy instance (overrides other params)

    Returns:
        Decorated function with retry logic

    Example:
        @with_retry(max_attempts=3, backoff="exponential")
        def call_external_api():
            # Automatically retries on failure with exponential backoff
            ...

        @with_retry(max_attempts=5, base_delay=2.0, retry_on=(TimeoutError, ConnectionError))
        def flaky_operation():
            # Only retries on specific errors
            ...
    """
    # Determine policy
    if policy is None:
        if backoff == "exponential":
            from src.lib.retry.policies import ExponentialBackoff

            policy = ExponentialBackoff(
                max_attempts=max_attempts, base_delay=base_delay, max_delay=max_delay
            )
        elif backoff == "fixed":
            from src.lib.retry.policies import FixedDelay

            policy = FixedDelay(max_attempts=max_attempts, delay=base_delay)
        else:
            policy = ExponentialBackoff(max_attempts=max_attempts)

    def decorator(func: Callable) -> Callable:
        @functools.wraps(func)
        def wrapper(*args, **kwargs):
            attempt = 0
            last_exception = None

            # Get logger for this function
            func_logger = logging.getLogger(func.__module__)

            while attempt < policy.max_attempts:
                attempt += 1

                try:
                    # Try the operation
                    result = func(*args, **kwargs)

                    # If successful and we retried, log success
                    if attempt > 1:
                        func_logger.info(
                            f"[RETRY] {func.__name__} succeeded on attempt {attempt}"
                        )

                    return result

                except retry_on as e:
                    last_exception = e

                    # Don't retry quota errors - they won't succeed
                    if _is_quota_error(e):
                        func_logger.error(
                            f"[RETRY] {func.__name__} failed with quota error. "
                            "Quota errors won't succeed on retry. Stopping immediately.",
                            exc_info=True,
                        )
                        raise

                    # Don't retry ValidationError for empty entities - LLM correctly returned empty list
                    # This is a known case where retrying won't help (chunk has no extractable entities)
                    if _is_empty_entity_validation_error(e):
                        func_logger.debug(
                            f"[RETRY] {func.__name__} failed with empty entity validation error. "
                            "This is expected for chunks with no extractable entities. Not retrying.",
                        )
                        raise

                    # Check if should retry
                    if not policy.should_retry(attempt, e):
                        # No more retries
                        func_logger.error(
                            f"[RETRY] {func.__name__} failed after {attempt} attempts",
                            exc_info=True,
                        )
                        raise

                    # Calculate delay
                    delay = policy.get_delay(attempt)

                    # Log retry with context
                    from src.lib.error_handling.exceptions import (
                        format_exception_message,
                    )

                    error_msg = format_exception_message(e)

                    func_logger.warning(
                        f"[RETRY] {func.__name__} attempt {attempt} failed: {error_msg}. "
                        f"Retrying in {delay:.1f}s... "
                        f"({policy.max_attempts - attempt} attempts remaining)"
                    )

                    # Track retry metric
                    try:
                        from src.lib.metrics import MetricRegistry

                        registry = MetricRegistry.get_instance()
                        retry_counter = registry.get("retries_attempted")
                        if retry_counter:
                            retry_counter.inc(
                                labels={
                                    "function": func.__name__,
                                    "error_type": type(e).__name__,
                                }
                            )
                    except Exception:
                        pass

                    # Wait before retry
                    time.sleep(delay)

            # Should never reach here, but just in case
            if last_exception:
                raise last_exception

        return wrapper

    return decorator


def retry_llm_call(max_attempts: int = 3) -> Callable:
    """Specialized retry decorator for LLM calls.

    Uses exponential backoff optimized for LLM rate limits.

    Args:
        max_attempts: Maximum retry attempts (default: 3)

    Returns:
        Decorated function

    Example:
        @retry_llm_call(max_attempts=5)
        def call_openai():
            # Retries with appropriate backoff for rate limits
            ...
    """
    return with_retry(
        max_attempts=max_attempts,
        backoff="exponential",
        base_delay=1.0,
        max_delay=60.0,
        retry_on=(Exception,),  # Retry on all LLM errors
    )
