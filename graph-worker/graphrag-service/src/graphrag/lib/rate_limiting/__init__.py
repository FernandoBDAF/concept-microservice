"""
Rate Limiting Library - Cross-Cutting Concern.

Provides rate limiting for any operation (LLM, DB, API calls).
Part of the CORE libraries - Tier 2.

Usage:
    from src.lib.rate_limiting import RateLimiter, rate_limit

    # As a context manager
    limiter = RateLimiter(rpm=100, jitter_ms=200, name="api_calls")
    with limiter:
        call_api()  # Automatically throttled

    # As a decorator
    @rate_limit(max_calls=10, period=60, name="external_api")
    def call_external_api():
        # Limited to 10 calls per minute
        ...

    # Manual wait
    limiter = RateLimiter(rpm=60, name="db_ops")
    limiter.wait()
    perform_operation()

    # Add explicit delay (e.g., from Retry-After header)
    limiter.delay(seconds=30)
"""

from src.lib.rate_limiting.limiter import RateLimiter, rate_limit

__all__ = [
    "RateLimiter",
    "rate_limit",
]
