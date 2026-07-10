"""
Database Operations Library - Cross-Cutting Concern.

Provides database operation helpers for batch operations and queries.
Part of the CORE libraries - Tier 2.

Usage:
    from src.lib.database import batch_insert, batch_update, batch_delete

    # Batch insert
    result = batch_insert(
        collection=db.entities,
        documents=entities,
        batch_size=1000,
        ordered=False  # Continue on errors
    )
    print(f"Inserted: {result['inserted']}/{result['total']}")

    # Batch update
    updates = [
        {
            'filter': {'_id': entity_id},
            'update': {'$set': {'status': 'processed'}}
        }
        for entity_id in entity_ids
    ]
    result = batch_update(
        collection=db.entities,
        updates=updates,
        batch_size=500,
        upsert=False
    )

    # Batch delete
    filters = [{'_id': id} for id in ids_to_delete]
    result = batch_delete(
        collection=db.old_data,
        filters=filters,
        batch_size=500
    )
"""

from src.lib.database.operations import (
    batch_insert,
    batch_update,
    batch_delete,
    get_collection,
    get_database,
)

__all__ = [
    "batch_insert",
    "batch_update",
    "batch_delete",
    "get_collection",
    "get_database",
]
