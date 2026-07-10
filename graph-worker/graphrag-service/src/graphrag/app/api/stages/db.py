"""
Shared Database Utilities for Stages API

Provides thread-safe MongoDB client and common database constants.
Extracted from management.py for reuse across viewer.py and other modules.
"""

import os
import threading
import logging
from typing import Optional

from pymongo import MongoClient

logger = logging.getLogger(__name__)

# =============================================================================
# Shared Constants
# =============================================================================

# Databases to exclude from user-facing listings
SYSTEM_DATABASES = ["admin", "config", "local"]

# Maximum documents per query (safety limit)
MAX_QUERY_LIMIT = 100

# Binary data threshold for inline encoding (bytes)
BINARY_INLINE_THRESHOLD = int(os.getenv("VIEWER_BINARY_THRESHOLD", "1000"))


# =============================================================================
# Thread-Safe MongoDB Client
# =============================================================================

_mongo_client: Optional[MongoClient] = None
_client_lock = threading.Lock()


def get_mongo_client() -> MongoClient:
    """
    Get or create MongoDB client with connection pooling.
    
    Thread-safe singleton pattern.
    """
    global _mongo_client
    with _client_lock:
        if _mongo_client is None:
            _mongo_client = MongoClient(
                os.getenv("MONGODB_URI", "mongodb://localhost:27017"),
                maxPoolSize=10,
                serverSelectionTimeoutMS=5000,
            )
            logger.info("Created shared MongoDB client")
        return _mongo_client


def close_mongo_client() -> None:
    """Close the MongoDB client connection."""
    global _mongo_client
    with _client_lock:
        if _mongo_client is not None:
            _mongo_client.close()
            _mongo_client = None
            logger.info("Closed MongoDB client")

