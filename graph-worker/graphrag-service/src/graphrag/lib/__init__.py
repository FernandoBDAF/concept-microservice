"""
Core Libraries - Cross-Cutting Technical Concerns.

This package provides reusable technical libraries that support all business domains.
Libraries are pure (no dependencies on app/ or business/ layers).

Architecture:
- Tier 1: Critical libraries (full implementation)
- Tier 2: Important libraries (simple implementation + TODOs)
- Tier 3: Nice-to-have libraries (stubs for future)

Usage:
    from src.lib.logging import setup_logging, get_logger
    from src.lib.error_handling import handle_errors
    from src.lib.retry import with_retry
    # etc.
"""

# Tier 1: Critical (Full Implementation)
# 1. logging - ✅ IMPLEMENTED
# 2. error_handling - TODO
# 3. retry - TODO
# 4. tracing - TODO
# 5. metrics - TODO

# Tier 2: Important (Simple + TODOs)
# 6. validation - TODO
# 7. configuration - TODO
# 8. caching - TODO
# 9. database - TODO
# 10. llm - TODO
# 11. concurrency - TODO (move from core/domain/)
# 12. rate_limiting - TODO (move from dependencies/llm/)
# 13. serialization - TODO
# 14. data_transform - TODO

# Tier 3: Nice-to-Have (Stubs)
# 15. health - STUB
# 16. context - STUB
# 17. di - STUB
# 18. feature_flags - STUB

__all__ = []  # Libraries export their own APIs
