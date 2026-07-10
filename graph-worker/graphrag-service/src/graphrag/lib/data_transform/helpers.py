"""
Data Transformation Helpers.

Common data transformation utilities.
Part of the CORE libraries - data_transform library.
"""

from typing import Any, Dict, List, Callable


def flatten(nested_dict: Dict[str, Any], separator: str = ".") -> Dict[str, Any]:
    """Flatten nested dictionary.

    Args:
        nested_dict: Nested dictionary to flatten
        separator: Separator for nested keys (default: ".")

    Returns:
        Flattened dictionary

    Example:
        nested = {'a': {'b': {'c': 1}}}
        flat = flatten(nested)
        # Returns: {'a.b.c': 1}
    """
    result = {}

    def _flatten(obj: Any, prefix: str = ""):
        if isinstance(obj, dict):
            for key, value in obj.items():
                new_key = f"{prefix}{separator}{key}" if prefix else key
                _flatten(value, new_key)
        else:
            result[prefix] = obj

    _flatten(nested_dict)
    return result


def group_by(items: List[Dict[str, Any]], key: str) -> Dict[Any, List[Dict[str, Any]]]:
    """Group list of dicts by key.

    Args:
        items: List of dictionaries
        key: Key to group by

    Returns:
        Dictionary mapping key values to lists of items

    Example:
        items = [{'type': 'A', 'val': 1}, {'type': 'A', 'val': 2}, {'type': 'B', 'val': 3}]
        grouped = group_by(items, 'type')
        # Returns: {'A': [{...}, {...}], 'B': [{...}]}
    """
    result: Dict[Any, List[Dict[str, Any]]] = {}

    for item in items:
        group_key = item.get(key)
        if group_key not in result:
            result[group_key] = []
        result[group_key].append(item)

    return result


def deduplicate(items: List[Dict[str, Any]], key: str) -> List[Dict[str, Any]]:
    """Remove duplicates by key.

    Keeps first occurrence of each unique key value.

    Args:
        items: List of dictionaries
        key: Key to check for uniqueness

    Returns:
        List with duplicates removed

    Example:
        items = [{'id': 1, 'val': 'a'}, {'id': 1, 'val': 'b'}, {'id': 2, 'val': 'c'}]
        unique = deduplicate(items, 'id')
        # Returns: [{'id': 1, 'val': 'a'}, {'id': 2, 'val': 'c'}]
    """
    seen = set()
    result = []

    for item in items:
        item_key = item.get(key)
        if item_key not in seen:
            seen.add(item_key)
            result.append(item)

    return result


def merge_dicts(
    dict1: Dict[str, Any], dict2: Dict[str, Any], deep: bool = False
) -> Dict[str, Any]:
    """Merge two dictionaries.

    Args:
        dict1: First dictionary
        dict2: Second dictionary (takes precedence)
        deep: If True, recursively merge nested dicts

    Returns:
        Merged dictionary

    Example:
        d1 = {'a': 1, 'b': {'c': 2}}
        d2 = {'b': {'d': 3}, 'e': 4}
        merged = merge_dicts(d1, d2, deep=True)
        # Returns: {'a': 1, 'b': {'c': 2, 'd': 3}, 'e': 4}
    """
    result = dict1.copy()

    for key, value in dict2.items():
        if (
            deep
            and key in result
            and isinstance(result[key], dict)
            and isinstance(value, dict)
        ):
            result[key] = merge_dicts(result[key], value, deep=True)
        else:
            result[key] = value

    return result
