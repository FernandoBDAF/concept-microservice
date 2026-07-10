"""
Data Transform Library - Cross-Cutting Concern.

Common data transformation utilities.
Part of the CORE libraries - Tier 2 (simple implementation).

Usage:
    from src.lib.data_transform import flatten, group_by, deduplicate

    # Flatten nested dict
    flat = flatten({'a': {'b': {'c': 1}}})  # Returns: {'a.b.c': 1}

    # Group by key
    grouped = group_by(items, key='type')

    # Remove duplicates
    unique = deduplicate(items, key='entity_id')
"""

from src.lib.data_transform.helpers import (
    flatten,
    group_by,
    deduplicate,
    merge_dicts,
)


__all__ = [
    "flatten",
    "group_by",
    "deduplicate",
    "merge_dicts",
]
