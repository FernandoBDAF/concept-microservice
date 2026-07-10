"""
MongoDB Client Adapter.

This module provides a centralized MongoDB client with connection management.
Part of the DEPENDENCIES layer - abstracts external database dependency.

Automatically initializes the system_data database (indexes, default prompts)
on first connection.
"""

import os
import logging
from typing import Optional
from pymongo import MongoClient
from pymongo.collection import Collection
from pymongo.database import Database

logger = logging.getLogger(__name__)


class MongoDBClient:
    """Singleton MongoDB client wrapper with auto-initialization."""

    _instance: Optional[MongoClient] = None
    _system_initialized: bool = False

    @classmethod
    def get_instance(cls, uri: Optional[str] = None) -> MongoClient:
        """Get MongoDB client instance (singleton).

        On first call, automatically initializes the system_data database
        with required indexes and default data.

        Args:
            uri: MongoDB URI. If None, reads from MONGODB_URI env var.

        Returns:
            MongoClient instance
        """
        if cls._instance is None:
            connection_uri = uri or os.getenv(
                "MONGODB_URI", os.getenv("MONGODB_ATLAS_URI", "mongodb://localhost:27017")
            )
            cls._instance = MongoClient(connection_uri)
            
            # Auto-initialize system database on first connection
            cls._ensure_system_initialized()
        
        return cls._instance

    @classmethod
    def _ensure_system_initialized(cls) -> None:
        """Ensure system_data database is initialized (indexes, prompts)."""
        if cls._system_initialized:
            return
        
        try:
            from src.domain.shared.system.initialization import (
                ensure_system_database_initialized,
            )
            ensure_system_database_initialized(cls._instance)
            cls._system_initialized = True
        except Exception as e:
            # Log but don't fail - system can work without indexes (just slower)
            logger.warning(f"System database initialization skipped: {e}")

    @classmethod
    def get_collection(cls, db_name: str, collection_name: str) -> Collection:
        """Get a collection handle.

        Args:
            db_name: Database name
            collection_name: Collection name

        Returns:
            Collection instance
        """
        client = cls.get_instance()
        return client[db_name][collection_name]

    @classmethod
    def get_database(cls, db_name: str) -> Database:
        """Get a database handle.

        Args:
            db_name: Database name

        Returns:
            Database instance
        """
        client = cls.get_instance()
        return client[db_name]

    @classmethod
    def reset_instance(cls) -> None:
        """Reset singleton instance (useful for testing)."""
        if cls._instance is not None:
            cls._instance.close()
            cls._instance = None
        cls._system_initialized = False


# Backward compatibility wrapper
def get_mongo_client(uri: Optional[str] = None) -> MongoClient:
    """Get MongoDB client instance.

    This function maintains backward compatibility with existing code.
    New code should use MongoDBClient.get_instance() directly.

    Args:
        uri: MongoDB URI (optional)

    Returns:
        MongoClient instance
    """
    return MongoDBClient.get_instance(uri)


def read_collection(
    db: Database,
    collection_name: str,
    video_id: Optional[str] = None,
    fields: Optional[list] = None,
    limit: int = 1000,
) -> list:
    """Read documents from a MongoDB collection.

    Backward compatibility function from app.services.utils.

    Args:
        db: A database handle from MongoClient[DB_NAME].
        collection_name: Name of the collection to read.
        video_id: If provided, filter by this video_id; when None, return all.
        fields: List of field names to project; when empty/None, return all fields.
        limit: Max documents to return (safety default 1000).

    Returns:
        A list of dict documents.
    """
    coll = db[collection_name]
    query = {"video_id": video_id} if video_id else {}
    projection = {f: 1 for f in fields} if fields else None
    cursor = coll.find(query, projection).limit(limit)
    return list(cursor)
