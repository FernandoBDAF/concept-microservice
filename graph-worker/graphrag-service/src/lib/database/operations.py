"""
Database operation helpers for batch operations and queries.
"""

import logging
from typing import Any, Dict, List, Optional, Sequence
from pymongo.collection import Collection
from pymongo.database import Database
from pymongo.errors import BulkWriteError

logger = logging.getLogger(__name__)


def batch_insert(
    collection: Collection,
    documents: Sequence[Dict[str, Any]],
    batch_size: int = 1000,
    ordered: bool = False,
) -> Dict[str, Any]:
    """Insert documents in batches for better performance.

    Args:
        collection: MongoDB collection
        documents: Documents to insert
        batch_size: Size of each batch
        ordered: If True, stop on first error. If False, continue inserting

    Returns:
        Dictionary with statistics:
            - total: Total documents attempted
            - inserted: Successfully inserted count
            - failed: Failed insertions count
            - errors: List of error messages

    Note:
        Using ordered=False allows continuing after errors, which is useful for
        bulk inserts where some documents might already exist.
    """
    total = len(documents)
    inserted = 0
    failed = 0
    errors = []

    logger.debug(
        f"Batch inserting {total} documents into {collection.name} "
        f"(batch_size={batch_size}, ordered={ordered})"
    )

    for i in range(0, total, batch_size):
        batch = documents[i : i + batch_size]

        try:
            result = collection.insert_many(batch, ordered=ordered)
            inserted += len(result.inserted_ids)
            logger.debug(
                f"Inserted batch {i // batch_size + 1}: "
                f"{len(result.inserted_ids)} documents"
            )

        except BulkWriteError as e:
            # Some documents inserted, some failed
            inserted += e.details.get("nInserted", 0)
            failed += len(batch) - e.details.get("nInserted", 0)

            error_msg = f"Bulk write errors in batch {i // batch_size + 1}: {e.details.get('nInserted', 0)}/{len(batch)} inserted"
            errors.append(error_msg)
            logger.warning(error_msg)

        except Exception as e:
            # Entire batch failed
            failed += len(batch)
            error_msg = f"Batch {i // batch_size + 1} failed completely: {str(e)}"
            errors.append(error_msg)
            logger.error(error_msg)

    logger.info(f"Batch insert completed: {inserted}/{total} inserted, {failed} failed")

    return {
        "total": total,
        "inserted": inserted,
        "failed": failed,
        "errors": errors,
        "success_rate": (inserted / total * 100) if total > 0 else 0.0,
    }


def batch_update(
    collection: Collection,
    updates: Sequence[Dict[str, Any]],
    batch_size: int = 500,
    upsert: bool = False,
) -> Dict[str, Any]:
    """Update documents in batches.

    Args:
        collection: MongoDB collection
        updates: List of update operations, each with:
            - filter: Query filter
            - update: Update operation
            - upsert: Whether to insert if not found (optional, overrides global)
        batch_size: Size of each batch
        upsert: Default upsert behavior if not specified per operation

    Returns:
        Dictionary with statistics

    Example:
        updates = [
            {
                'filter': {'_id': '123'},
                'update': {'$set': {'status': 'completed'}}
            },
            {
                'filter': {'_id': '456'},
                'update': {'$set': {'status': 'failed'}},
                'upsert': True  # Override default
            }
        ]
        batch_update(collection, updates)
    """
    total = len(updates)
    matched = 0
    modified = 0
    upserted = 0
    failed = 0
    errors = []

    logger.debug(
        f"Batch updating {total} documents in {collection.name} "
        f"(batch_size={batch_size}, upsert={upsert})"
    )

    for i in range(0, total, batch_size):
        batch = updates[i : i + batch_size]

        try:
            # Build bulk write operations
            from pymongo import UpdateOne

            operations = []
            for op in batch:
                operation = UpdateOne(
                    filter=op["filter"],
                    update=op["update"],
                    upsert=op.get("upsert", upsert),
                )
                operations.append(operation)

            # Execute bulk write
            result = collection.bulk_write(operations, ordered=False)

            matched += result.matched_count
            modified += result.modified_count
            upserted += result.upserted_count

            logger.debug(
                f"Updated batch {i // batch_size + 1}: "
                f"matched={result.matched_count}, "
                f"modified={result.modified_count}, "
                f"upserted={result.upserted_count}"
            )

        except BulkWriteError as e:
            # Some updates succeeded, some failed
            details = e.details
            matched += details.get("nMatched", 0)
            modified += details.get("nModified", 0)
            upserted += details.get("nUpserted", 0)
            failed += (
                len(batch) - details.get("nMatched", 0) - details.get("nUpserted", 0)
            )

            error_msg = f"Bulk write errors in batch {i // batch_size + 1}"
            errors.append(error_msg)
            logger.warning(error_msg)

        except Exception as e:
            # Entire batch failed
            failed += len(batch)
            error_msg = f"Batch {i // batch_size + 1} failed completely: {str(e)}"
            errors.append(error_msg)
            logger.error(error_msg)

    logger.info(
        f"Batch update completed: matched={matched}, modified={modified}, "
        f"upserted={upserted}, failed={failed}"
    )

    return {
        "total": total,
        "matched": matched,
        "modified": modified,
        "upserted": upserted,
        "failed": failed,
        "errors": errors,
    }


def batch_delete(
    collection: Collection,
    filters: Sequence[Dict[str, Any]],
    batch_size: int = 500,
) -> Dict[str, Any]:
    """Delete documents in batches.

    Args:
        collection: MongoDB collection
        filters: List of filter queries for documents to delete
        batch_size: Size of each batch

    Returns:
        Dictionary with statistics
    """
    total = len(filters)
    deleted = 0
    failed = 0
    errors = []

    logger.debug(
        f"Batch deleting {total} documents from {collection.name} "
        f"(batch_size={batch_size})"
    )

    for i in range(0, total, batch_size):
        batch = filters[i : i + batch_size]

        try:
            from pymongo import DeleteOne

            operations = [DeleteOne(filter_query) for filter_query in batch]
            result = collection.bulk_write(operations, ordered=False)

            deleted += result.deleted_count
            logger.debug(
                f"Deleted batch {i // batch_size + 1}: "
                f"{result.deleted_count} documents"
            )

        except BulkWriteError as e:
            # Some deletes succeeded, some failed
            deleted += e.details.get("nRemoved", 0)
            failed += len(batch) - e.details.get("nRemoved", 0)

            error_msg = f"Bulk write errors in batch {i // batch_size + 1}"
            errors.append(error_msg)
            logger.warning(error_msg)

        except Exception as e:
            # Entire batch failed
            failed += len(batch)
            error_msg = f"Batch {i // batch_size + 1} failed completely: {str(e)}"
            errors.append(error_msg)
            logger.error(error_msg)

    logger.info(f"Batch delete completed: deleted={deleted}/{total}, failed={failed}")

    return {
        "total": total,
        "deleted": deleted,
        "failed": failed,
        "errors": errors,
    }


def get_collection(
    db: Database,
    collection_name: str,
    create_if_missing: bool = False,
) -> Collection:
    """Get a MongoDB collection with optional creation.

    Args:
        db: MongoDB database instance
        collection_name: Name of the collection
        create_if_missing: If True, create collection if it doesn't exist

    Returns:
        Collection instance

    Example:
        from src.infrastructure.database.mongodb import get_mongo_client
        from src.core.config.paths import DB_NAME

        client = get_mongo_client()
        db = client[DB_NAME]
        col = get_collection(db, "video_chunks")
    """
    collection = db[collection_name]

    if create_if_missing:
        # Create collection if it doesn't exist (MongoDB creates lazily)
        # This is a no-op if collection already exists
        db.create_collection(collection_name, check_exists=True)

    return collection


def get_database(
    client: Any,  # MongoClient type from pymongo
    db_name: str,
    create_if_missing: bool = False,
) -> Database:
    """Get a MongoDB database with optional creation.

    Args:
        client: MongoDB client instance
        db_name: Name of the database
        create_if_missing: If True, create database if it doesn't exist (MongoDB creates lazily)

    Returns:
        Database instance

    Example:
        from src.infrastructure.database.mongodb import get_mongo_client
        from src.core.config.paths import DB_NAME

        client = get_mongo_client()
        db = get_database(client, DB_NAME)
    """
    db = client[db_name]

    if create_if_missing:
        # MongoDB creates databases lazily on first write
        # This is a no-op if database already exists
        # We can verify by listing collections (creates if needed)
        try:
            db.list_collection_names()
        except Exception:
            pass  # Database will be created on first write

    return db
