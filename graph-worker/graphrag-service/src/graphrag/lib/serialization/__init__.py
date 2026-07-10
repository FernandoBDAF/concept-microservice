"""
Serialization Library - Cross-Cutting Concern.

Provides Pydantic ↔ MongoDB conversion and JSON encoding helpers.
Part of the CORE libraries - Tier 2 (simple implementation).

Usage:
    from src.lib.serialization import to_dict, from_dict, json_encoder

    # Pydantic → MongoDB
    doc = to_dict(entity_model, for_mongodb=True)
    db.entities.insert_one(doc)

    # MongoDB → Pydantic
    doc = db.entities.find_one({'entity_id': '123'})
    entity = from_dict(doc, EntityModel)

    # JSON export
    json_str = json.dumps(doc, default=json_encoder)
"""

from src.lib.serialization.converters import (
    to_dict,
    from_dict,
    json_encoder,
)


__all__ = [
    "to_dict",
    "from_dict",
    "json_encoder",
]
