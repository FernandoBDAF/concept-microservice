"""
Serialization Converters.

Provides Pydantic ↔ MongoDB conversion and JSON encoding for MongoDB types.
Part of the CORE libraries - serialization library.
"""

import json
from typing import Any, Dict, Type, TypeVar
from datetime import datetime
from pydantic import BaseModel

try:
    from bson import ObjectId
    from bson.decimal128 import Decimal128
except ImportError:
    ObjectId = None
    Decimal128 = None


T = TypeVar("T", bound=BaseModel)


def to_dict(model: BaseModel, for_mongodb: bool = False) -> Dict[str, Any]:
    """Convert Pydantic model to dict.

    Args:
        model: Pydantic model instance (or None)
        for_mongodb: If True, convert ObjectId/datetime to MongoDB-compatible types

    Returns:
        Dictionary representation (or None if model is None)

    Example:
        entity = EntityModel(name="Python", type="TECHNOLOGY")
        doc = to_dict(entity, for_mongodb=True)
        db.collection.insert_one(doc)
    """
    if model is None:
        return None

    data = model.model_dump() if hasattr(model, "model_dump") else model.dict()

    if for_mongodb:
        data = _convert_for_mongodb(data)

    return data


def from_dict(model_class: Type[T], data: Dict[str, Any]) -> T:
    """Convert dict to Pydantic model.

    Args:
        model_class: Pydantic model class
        data: Dictionary (e.g., from MongoDB)

    Returns:
        Model instance

    Example:
        doc = db.collection.find_one({'entity_id': '123'})
        entity = from_dict(EntityModel, doc)
    """
    # Remove MongoDB-specific fields
    clean_data = {k: v for k, v in data.items() if not k.startswith("_")}

    return model_class(**clean_data)


def _convert_for_mongodb(obj: Any) -> Any:
    """Recursively convert objects for MongoDB storage."""
    if isinstance(obj, dict):
        return {k: _convert_for_mongodb(v) for k, v in obj.items()}
    elif isinstance(obj, list):
        return [_convert_for_mongodb(item) for item in obj]
    elif isinstance(obj, datetime):
        return obj  # MongoDB handles datetime natively
    elif ObjectId and isinstance(obj, ObjectId):
        return obj  # MongoDB handles ObjectId natively
    else:
        return obj


def json_encoder(obj: Any) -> Any:
    """JSON encoder for MongoDB types.

    Use with json.dumps(data, default=json_encoder)

    Args:
        obj: Object to encode

    Returns:
        JSON-serializable representation

    Example:
        doc = db.collection.find_one()
        json_str = json.dumps(doc, default=json_encoder)
    """
    if ObjectId and isinstance(obj, ObjectId):
        return str(obj)
    elif isinstance(obj, datetime):
        return obj.isoformat()
    elif Decimal128 and isinstance(obj, Decimal128):
        return float(obj.to_decimal())
    elif hasattr(obj, "__dict__"):
        return obj.__dict__
    else:
        # For basic JSON types, return as-is
        return obj
