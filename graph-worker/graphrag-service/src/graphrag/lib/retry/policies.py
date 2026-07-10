"""
Retry Policies.

Provides backoff strategies for retry logic.
Part of the CORE libraries - retry library.
"""

import time
from typing import Optional, Type, Tuple


class RetryPolicy:
    """Base retry policy.

    Defines when to retry and how long to wait between attempts.
    """

    def __init__(self, max_attempts: int = 3):
        """Initialize retry policy.

        Args:
            max_attempts: Maximum number of attempts (default: 3)
        """
        self.max_attempts = max_attempts

    def should_retry(self, attempt: int, exception: Exception) -> bool:
        """Determine if operation should be retried.

        Args:
            attempt: Current attempt number (1-indexed)
            exception: Exception that occurred

        Returns:
            True if should retry
        """
        return attempt < self.max_attempts

    def get_delay(self, attempt: int) -> float:
        """Get delay before next retry attempt.

        Args:
            attempt: Current attempt number (1-indexed)

        Returns:
            Delay in seconds
        """
        return 0.0


class ExponentialBackoff(RetryPolicy):
    """Exponential backoff retry policy.

    Delay doubles after each retry: 1s, 2s, 4s, 8s, ...
    Most common and recommended for API calls.

    Example:
        policy = ExponentialBackoff(max_attempts=5, base_delay=1.0, max_delay=60.0)

        # Attempt 1 fails → wait 1s
        # Attempt 2 fails → wait 2s
        # Attempt 3 fails → wait 4s
        # Attempt 4 fails → wait 8s
        # Attempt 5 fails → give up
    """

    def __init__(
        self,
        max_attempts: int = 3,
        base_delay: float = 1.0,
        max_delay: float = 60.0,
        multiplier: float = 2.0,
    ):
        """Initialize exponential backoff policy.

        Args:
            max_attempts: Maximum attempts (default: 3)
            base_delay: Initial delay in seconds (default: 1.0)
            max_delay: Maximum delay in seconds (default: 60.0)
            multiplier: Backoff multiplier (default: 2.0)
        """
        super().__init__(max_attempts)
        self.base_delay = base_delay
        self.max_delay = max_delay
        self.multiplier = multiplier

    def get_delay(self, attempt: int) -> float:
        """Calculate exponential backoff delay.

        Args:
            attempt: Current attempt number

        Returns:
            Delay in seconds, capped at max_delay
        """
        delay = self.base_delay * (self.multiplier ** (attempt - 1))
        return min(delay, self.max_delay)


class FixedDelay(RetryPolicy):
    """Fixed delay retry policy.

    Same delay between all retries.
    Simple and predictable.

    Example:
        policy = FixedDelay(max_attempts=3, delay=2.0)

        # All retries wait 2 seconds
    """

    def __init__(self, max_attempts: int = 3, delay: float = 1.0):
        """Initialize fixed delay policy.

        Args:
            max_attempts: Maximum attempts
            delay: Fixed delay in seconds between retries
        """
        super().__init__(max_attempts)
        self.delay = delay

    def get_delay(self, attempt: int) -> float:
        """Return fixed delay.

        Args:
            attempt: Current attempt number

        Returns:
            Fixed delay in seconds
        """
        return self.delay


class NoRetry(RetryPolicy):
    """No retry policy - fail immediately.

    Useful for disabling retries when needed.
    """

    def __init__(self):
        """Initialize no-retry policy."""
        super().__init__(max_attempts=1)

    def should_retry(self, attempt: int, exception: Exception) -> bool:
        """Never retry.

        Returns:
            Always False
        """
        return False


# Default policy (used when none specified)
DEFAULT_POLICY = ExponentialBackoff(max_attempts=3, base_delay=1.0, max_delay=60.0)
