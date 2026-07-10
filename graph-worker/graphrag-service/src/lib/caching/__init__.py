"""
Caching Library - Cross-Cutting Concern.

Provides in-memory LRU caching with optional TTL support.
Part of the CORE libraries - Tier 2.

Usage:
    from src.lib.caching import LRUCache, cached

    # Manual cache usage
    cache = LRUCache(max_size=1000, ttl=3600, name="entities")
    cache.set('key', 'value')
    value = cache.get('key')

    # Decorator usage
    @cached(max_size=100, ttl=600)
    def get_entity(entity_id: str):
        # Result cached for 10 minutes
        # Cache size limited to 100 items (LRU eviction)
        ...

    # Custom cache key
    @cached(
        max_size=500,
        key_func=lambda user_id, date: f"{user_id}:{date}",
        name="user_data"
    )
    def get_user_data(user_id: str, date: str):
        ...

    # Check cache stats
    @cached(max_size=100)
    def my_func():
        ...

    stats = my_func.cache.stats()
    # {'hits': 50, 'misses': 10, 'hit_rate': 83.3, ...}
"""

from src.lib.caching.lru_cache import LRUCache, cached

__all__ = [
    "LRUCache",
    "cached",
]
