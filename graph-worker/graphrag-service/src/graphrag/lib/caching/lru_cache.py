"""
LRU Cache implementation with TTL support.
"""

import time
import logging
import threading
from typing import Any, Optional, Dict, Callable
from collections import OrderedDict
from functools import wraps

logger = logging.getLogger(__name__)


class LRUCache:
    """Thread-safe LRU cache with optional TTL (Time-To-Live).

    Implements Least Recently Used eviction policy. When the cache is full,
    the least recently used item is evicted.
    """

    def __init__(
        self, max_size: int = 1000, ttl: Optional[float] = None, name: str = "default"
    ):
        """Initialize LRU cache.

        Args:
            max_size: Maximum number of items to store
            ttl: Time-to-live in seconds (None = no expiration)
            name: Name for this cache (for logging)
        """
        self.max_size = max(1, max_size)
        self.ttl = ttl
        self.name = name
        self._cache: OrderedDict[str, Dict[str, Any]] = OrderedDict()
        self._lock = threading.Lock()
        self._hits = 0
        self._misses = 0

        logger.debug(f"Initialized LRU cache '{name}': max_size={max_size}, ttl={ttl}s")

    def get(self, key: str, default: Any = None) -> Any:
        """Get value from cache.

        Args:
            key: Cache key
            default: Default value if key not found

        Returns:
            Cached value or default
        """
        with self._lock:
            if key not in self._cache:
                self._misses += 1
                return default

            entry = self._cache[key]

            # Check TTL expiration
            if self.ttl is not None:
                age = time.time() - entry["timestamp"]
                if age > self.ttl:
                    # Expired - remove and return default
                    del self._cache[key]
                    self._misses += 1
                    logger.debug(f"Cache '{self.name}' expired key: {key}")
                    return default

            # Move to end (most recently used)
            self._cache.move_to_end(key)
            self._hits += 1
            return entry["value"]

    def set(self, key: str, value: Any) -> None:
        """Set value in cache.

        Args:
            key: Cache key
            value: Value to cache
        """
        with self._lock:
            if key in self._cache:
                # Update existing - move to end
                self._cache[key] = {
                    "value": value,
                    "timestamp": time.time(),
                }
                self._cache.move_to_end(key)
            else:
                # Add new entry
                if len(self._cache) >= self.max_size:
                    # Evict least recently used (first item)
                    evicted_key = next(iter(self._cache))
                    del self._cache[evicted_key]
                    logger.debug(f"Cache '{self.name}' evicted key: {evicted_key}")

                self._cache[key] = {
                    "value": value,
                    "timestamp": time.time(),
                }

    def delete(self, key: str) -> bool:
        """Delete key from cache.

        Args:
            key: Cache key to delete

        Returns:
            True if key was deleted, False if not found
        """
        with self._lock:
            if key in self._cache:
                del self._cache[key]
                return True
            return False

    def clear(self) -> None:
        """Clear all items from cache."""
        with self._lock:
            self._cache.clear()
            self._hits = 0
            self._misses = 0
            logger.debug(f"Cache '{self.name}' cleared")

    def size(self) -> int:
        """Get current cache size."""
        with self._lock:
            return len(self._cache)

    def stats(self) -> Dict[str, Any]:
        """Get cache statistics.

        Returns:
            Dictionary with hits, misses, size, and hit rate
        """
        with self._lock:
            total = self._hits + self._misses
            hit_rate = (self._hits / total * 100) if total > 0 else 0.0

            return {
                "name": self.name,
                "hits": self._hits,
                "misses": self._misses,
                "size": len(self._cache),
                "max_size": self.max_size,
                "hit_rate": hit_rate,
                "ttl": self.ttl,
            }


def cached(
    max_size: int = 1000,
    ttl: Optional[float] = None,
    key_func: Optional[Callable] = None,
    name: Optional[str] = None,
):
    """Decorator to cache function results.

    Args:
        max_size: Maximum cache size
        ttl: Time-to-live in seconds
        key_func: Optional function to generate cache key from args/kwargs
        name: Optional name for the cache

    Usage:
        @cached(max_size=100, ttl=3600)
        def expensive_operation(param):
            # Result cached for 1 hour
            ...

        @cached(key_func=lambda user_id: f"user_{user_id}")
        def get_user(user_id):
            # Custom cache key
            ...
    """
    cache_name = name or "decorator"
    cache = LRUCache(max_size=max_size, ttl=ttl, name=cache_name)

    def _default_key_func(*args, **kwargs):
        """Generate cache key from function arguments."""
        key_parts = [str(arg) for arg in args]
        key_parts.extend(f"{k}={v}" for k, v in sorted(kwargs.items()))
        return ":".join(key_parts)

    key_generator = key_func or _default_key_func

    def decorator(func):
        @wraps(func)
        def wrapper(*args, **kwargs):
            # Generate cache key
            try:
                cache_key = key_generator(*args, **kwargs)
            except Exception as e:
                logger.warning(f"Cache key generation failed for {func.__name__}: {e}")
                # Bypass cache if key generation fails
                return func(*args, **kwargs)

            # Check cache
            cached_value = cache.get(cache_key)
            if cached_value is not None:
                logger.debug(f"Cache hit for {func.__name__}: {cache_key}")
                return cached_value

            # Call function and cache result
            result = func(*args, **kwargs)
            cache.set(cache_key, result)
            logger.debug(f"Cached result for {func.__name__}: {cache_key}")
            return result

        # Attach cache for inspection
        wrapper.cache = cache
        return wrapper

    return decorator
