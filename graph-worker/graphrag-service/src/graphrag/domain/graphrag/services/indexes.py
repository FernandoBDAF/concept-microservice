"""
GraphRAG MongoDB Collections and Indexes

This module handles the creation and management of MongoDB collections
and indexes specifically for GraphRAG operations.
"""

import logging
from typing import Dict, Any, List
from pymongo.database import Database
from pymongo.collection import Collection
from pymongo.errors import OperationFailure

logger = logging.getLogger(__name__)


def create_graphrag_indexes(db: Database) -> None:
    """
    Create all GraphRAG-specific indexes.

    Args:
        db: MongoDB database instance
    """
    logger.info("Creating GraphRAG indexes...")

    try:
        # Create entities collection indexes
        _create_entities_indexes(db)

        # Create relations collection indexes
        _create_relations_indexes(db)

        # Create communities collection indexes
        _create_communities_indexes(db)

        # Create entity_mentions collection indexes
        _create_entity_mentions_indexes(db)

        # Create graphrag_runs collection indexes
        _create_graphrag_runs_indexes(db)

        logger.info("Successfully created all GraphRAG indexes")

    except Exception as e:
        logger.error(f"Failed to create GraphRAG indexes: {e}")
        raise


def _create_entities_indexes(db: Database) -> None:
    """Create indexes for the entities collection."""
    entities = db.entities

    # Compound index for name, type, and trust_score
    entities.create_index(
        [("name", 1), ("type", 1), ("trust_score", -1)], name="name_type_trust"
    )

    # Text index for name and canonical_name search
    entities.create_index(
        [("name", "text"), ("canonical_name", "text")], name="text_search"
    )

    # Sparse index for centrality_score (only entities with centrality scores)
    entities.create_index(
        [("centrality_score", -1)], name="centrality_score", sparse=True
    )

    # Index for entity_id lookups
    entities.create_index([("entity_id", 1)], name="entity_id", unique=True)

    # Index for type-based queries
    entities.create_index([("type", 1), ("confidence", -1)], name="type_confidence")

    # Index for source_count (for entity importance)
    entities.create_index([("source_count", -1)], name="source_count")

    # Index for normalized fields (for candidate lookup - Achievement 2.2)
    # Sparse index: only indexes documents that have the field
    entities.create_index(
        [("canonical_name_normalized", 1)],
        name="canonical_name_normalized",
        sparse=True,
    )

    # Multikey index: indexes each element in the aliases_normalized array
    entities.create_index([("aliases_normalized", 1)], name="aliases_normalized")

    # Index for last_seen (for cleanup and recent entity queries)
    entities.create_index([("last_seen", -1)], name="last_seen")

    logger.info("Created entities collection indexes")


def _create_relations_indexes(db: Database) -> None:
    """Create indexes for the relations collection."""
    relations = db.relations

    # Compound index for subject-object lookups
    relations.create_index(
        [("subject_id", 1), ("object_id", 1), ("confidence", -1)],
        name="subject_object_confidence",
    )

    # Reverse index for object-subject lookups
    relations.create_index(
        [("object_id", 1), ("subject_id", 1), ("confidence", -1)],
        name="object_subject_confidence",
    )

    # Index for relationship_id lookups
    relations.create_index(
        [("relationship_id", 1)], name="relationship_id", unique=True
    )

    # Index for predicate-based queries
    relations.create_index(
        [("predicate", 1), ("confidence", -1)], name="predicate_confidence"
    )

    # Index for source_count
    relations.create_index([("source_count", -1)], name="source_count")

    logger.info("Created relations collection indexes")


def _create_communities_indexes(db: Database) -> None:
    """Create indexes for the communities collection."""
    communities = db.communities

    # Compound index for entity lookups and level
    communities.create_index(
        [("entities", 1), ("level", 1), ("coherence_score", -1)],
        name="entities_level_coherence",
    )

    # Index for community_id lookups
    communities.create_index([("community_id", 1)], name="community_id", unique=True)

    # Index for level-based queries
    communities.create_index(
        [("level", 1), ("coherence_score", -1)], name="level_coherence"
    )

    # Index for entity_count (for community size queries)
    communities.create_index([("entity_count", -1)], name="entity_count")

    logger.info("Created communities collection indexes")


def _create_entity_mentions_indexes(db: Database) -> None:
    """Create indexes for the entity_mentions collection."""
    entity_mentions = db.entity_mentions

    # Unique index to prevent duplicate mentions (entity_id, chunk_id, position)
    # This ensures reruns are idempotent (Achievement 3.5.2)
    try:
        entity_mentions.create_index(
            [("entity_id", 1), ("chunk_id", 1), ("position", 1)],
            name="entity_chunk_position_unique",
            unique=True,
        )
        logger.info("Created unique index on (entity_id, chunk_id, position)")
    except Exception as e:
        # Index might already exist, log and continue
        logger.debug(f"Unique index may already exist: {e}")

    # Compound index for entity-chunk lookups
    entity_mentions.create_index(
        [("entity_id", 1), ("chunk_id", 1), ("confidence", -1)],
        name="entity_chunk_confidence",
    )

    # Index for chunk-based queries
    entity_mentions.create_index(
        [("chunk_id", 1), ("confidence", -1)], name="chunk_confidence"
    )

    # Index for entity-based queries
    entity_mentions.create_index(
        [("entity_id", 1), ("confidence", -1)], name="entity_confidence"
    )

    logger.info("Created entity_mentions collection indexes")


def _create_graphrag_runs_indexes(db: Database) -> None:
    """Create indexes for the graphrag_runs collection."""
    runs = db.graphrag_runs

    # Compound index for efficient run lookups (stage, params_hash, graph_signature)
    runs.create_index(
        [("stage", 1), ("params_hash", 1), ("graph_signature", 1)],
        name="stage_params_graph_lookup",
    )

    # Index for run_id lookups
    runs.create_index([("_id", 1)], name="run_id")

    # Index for status queries
    runs.create_index([("status", 1), ("started_at", -1)], name="status_started")

    logger.info("Created graphrag_runs collection indexes")


def ensure_graphrag_collections(db: Database) -> None:
    """
    Ensure all GraphRAG collections exist and have proper validation.

    Args:
        db: MongoDB database instance
    """
    logger.info("Ensuring GraphRAG collections exist...")

    # Create collections with validation schemas
    _ensure_entities_collection(db)
    _ensure_relations_collection(db)
    _ensure_communities_collection(db)
    _ensure_entity_mentions_collection(db)

    logger.info("All GraphRAG collections ensured")


def _ensure_entities_collection(db: Database) -> None:
    """Ensure entities collection exists with proper validation."""
    # Check if collection already exists
    existing_collections = set(db.list_collection_names())
    if "entities" in existing_collections:
        logger.info("Entities collection already exists")
        return

    # Create validation schema for entities
    validation_schema = {
        "$jsonSchema": {
            "bsonType": "object",
            "required": ["entity_id", "canonical_name", "name", "type", "description"],
            "properties": {
                "entity_id": {
                    "bsonType": "string",
                    "pattern": "^[a-f0-9]{32}$",
                    "description": "32-character MD5 hash",
                },
                "canonical_name": {
                    "bsonType": "string",
                    "minLength": 1,
                    "maxLength": 200,
                    "description": "Canonical entity name",
                },
                "name": {
                    "bsonType": "string",
                    "minLength": 1,
                    "maxLength": 200,
                    "description": "Primary entity name",
                },
                "type": {
                    "enum": [
                        "PERSON",
                        "ORGANIZATION",
                        "TECHNOLOGY",
                        "CONCEPT",
                        "LOCATION",
                        "EVENT",
                        "OTHER",
                    ],
                    "description": "Entity type",
                },
                "description": {
                    "bsonType": "string",
                    "minLength": 10,
                    "maxLength": 2000,
                    "description": "Entity description",
                },
                "confidence": {
                    "bsonType": "double",
                    "minimum": 0.0,
                    "maximum": 1.0,
                    "description": "Confidence score",
                },
                "source_count": {
                    "bsonType": "int",
                    "minimum": 1,
                    "description": "Number of source chunks",
                },
                "trust_score": {
                    "bsonType": "double",
                    "minimum": 0.0,
                    "maximum": 1.0,
                    "description": "Trust score from existing pipeline",
                },
                "centrality_score": {
                    "bsonType": "double",
                    "minimum": 0.0,
                    "maximum": 1.0,
                    "description": "Graph centrality score",
                },
            },
        }
    }

    try:
        db.create_collection("entities", validator=validation_schema)
        logger.info("Created entities collection with validation")
    except Exception as e:
        # Collection might have been created between check and creation, or validator error
        error_msg = str(e).lower()
        if (
            "already exists" in error_msg
            or "collection" in error_msg
            and "exists" in error_msg
        ):
            logger.info("Entities collection already exists")
        else:
            logger.warning(
                f"Failed to create entities collection: {e}. Collection may already exist."
            )
            # Continue - collection might exist without validator


def _ensure_relations_collection(db: Database) -> None:
    """Ensure relations collection exists with proper validation."""
    # Check if collection already exists
    existing_collections = set(db.list_collection_names())
    if "relations" in existing_collections:
        logger.info("Relations collection already exists")
        return

    # Create validation schema for relations
    validation_schema = {
        "$jsonSchema": {
            "bsonType": "object",
            "required": [
                "relationship_id",
                "subject_id",
                "object_id",
                "predicate",
                "description",
            ],
            "properties": {
                "relationship_id": {
                    "bsonType": "string",
                    "pattern": "^[a-f0-9]{32}$",
                    "description": "32-character MD5 hash",
                },
                "subject_id": {
                    "bsonType": "string",
                    "pattern": "^[a-f0-9]{32}$",
                    "description": "Subject entity ID",
                },
                "object_id": {
                    "bsonType": "string",
                    "pattern": "^[a-f0-9]{32}$",
                    "description": "Object entity ID",
                },
                "predicate": {
                    "bsonType": "string",
                    "minLength": 1,
                    "maxLength": 100,
                    "description": "Relationship predicate",
                },
                "description": {
                    "bsonType": "string",
                    "minLength": 10,
                    "maxLength": 1000,
                    "description": "Relationship description",
                },
                "confidence": {
                    "bsonType": "double",
                    "minimum": 0.0,
                    "maximum": 1.0,
                    "description": "Confidence score",
                },
                "source_count": {
                    "bsonType": "int",
                    "minimum": 1,
                    "description": "Number of source chunks",
                },
            },
        }
    }

    try:
        db.create_collection("relations", validator=validation_schema)
        logger.info("Created relations collection with validation")
    except Exception as e:
        # Collection might have been created between check and creation, or validator error
        error_msg = str(e).lower()
        if (
            "already exists" in error_msg
            or "collection" in error_msg
            and "exists" in error_msg
        ):
            logger.info("Relations collection already exists")
        else:
            logger.warning(
                f"Failed to create relations collection: {e}. Collection may already exist."
            )
            # Continue - collection might exist without validator


def _ensure_communities_collection(db: Database) -> None:
    """Ensure communities collection exists with proper validation."""
    # Check if collection already exists
    existing_collections = set(db.list_collection_names())
    if "communities" in existing_collections:
        logger.info("Communities collection already exists")
        return

    # Create validation schema for communities
    validation_schema = {
        "$jsonSchema": {
            "bsonType": "object",
            "required": ["community_id", "level", "title", "summary", "entities"],
            "properties": {
                "community_id": {
                    "bsonType": "string",
                    "minLength": 1,
                    "maxLength": 100,
                    "description": "Community identifier",
                },
                "level": {
                    "bsonType": "int",
                    "minimum": 1,
                    "maximum": 10,
                    "description": "Hierarchical level",
                },
                "title": {
                    "bsonType": "string",
                    "minLength": 1,
                    "maxLength": 200,
                    "description": "Community title",
                },
                "summary": {
                    "bsonType": "string",
                    "minLength": 50,
                    "maxLength": 5000,
                    "description": "Community summary",
                },
                "entities": {
                    "bsonType": "array",
                    "items": {"bsonType": "string", "pattern": "^[a-f0-9]{32}$"},
                    "minItems": 1,
                    "description": "Entity IDs in community",
                },
                "entity_count": {
                    "bsonType": "int",
                    "minimum": 1,
                    "description": "Number of entities",
                },
                "relationship_count": {
                    "bsonType": "int",
                    "minimum": 0,
                    "description": "Number of relationships",
                },
                "coherence_score": {
                    "bsonType": "double",
                    "minimum": 0.0,
                    "maximum": 1.0,
                    "description": "Community coherence score",
                },
            },
        }
    }

    try:
        db.create_collection("communities", validator=validation_schema)
        logger.info("Created communities collection with validation")
    except Exception as e:
        # Collection might have been created between check and creation, or validator error
        error_msg = str(e).lower()
        if (
            "already exists" in error_msg
            or "collection" in error_msg
            and "exists" in error_msg
        ):
            logger.info("Communities collection already exists")
        else:
            logger.warning(
                f"Failed to create communities collection: {e}. Collection may already exist."
            )
            # Continue - collection might exist without validator


def _ensure_entity_mentions_collection(db: Database) -> None:
    """Ensure entity_mentions collection exists with proper validation."""
    # Check if collection already exists
    existing_collections = set(db.list_collection_names())
    if "entity_mentions" in existing_collections:
        logger.info("Entity_mentions collection already exists")
        return

    # Create validation schema for entity_mentions
    validation_schema = {
        "$jsonSchema": {
            "bsonType": "object",
            "required": ["entity_id", "chunk_id", "confidence"],
            "properties": {
                "entity_id": {
                    "bsonType": "string",
                    "pattern": "^[a-f0-9]{32}$",
                    "description": "Entity ID",
                },
                "chunk_id": {
                    "bsonType": "string",
                    "minLength": 1,
                    "maxLength": 100,
                    "description": "Chunk ID",
                },
                "confidence": {
                    "bsonType": "double",
                    "minimum": 0.0,
                    "maximum": 1.0,
                    "description": "Mention confidence score",
                },
                "video_id": {
                    "bsonType": "string",
                    "minLength": 1,
                    "maxLength": 100,
                    "description": "Video ID",
                },
                "position": {
                    "bsonType": "int",
                    "minimum": 0,
                    "description": "Position in chunk",
                },
            },
        }
    }

    try:
        db.create_collection("entity_mentions", validator=validation_schema)
        logger.info("Created entity_mentions collection with validation")
    except Exception as e:
        # Collection might have been created between check and creation, or validator error
        error_msg = str(e).lower()
        if (
            "already exists" in error_msg
            or "collection" in error_msg
            and "exists" in error_msg
        ):
            logger.info("Entity_mentions collection already exists")
        else:
            logger.warning(
                f"Failed to create entity_mentions collection: {e}. Collection may already exist."
            )
            # Continue - collection might exist without validator


def get_graphrag_collections(db: Database) -> Dict[str, Collection]:
    """
    Get all GraphRAG collections.

    Args:
        db: MongoDB database instance

    Returns:
        Dictionary mapping collection names to Collection objects
    """
    return {
        "entities": db.entities,
        "relations": db.relations,
        "communities": db.communities,
        "entity_mentions": db.entity_mentions,
    }


def drop_graphrag_collections(db: Database) -> None:
    """
    Drop all GraphRAG collections (use with caution!).

    Args:
        db: MongoDB database instance
    """
    logger.warning("Dropping all GraphRAG collections...")

    collections = ["entities", "relations", "communities", "entity_mentions"]

    for collection_name in collections:
        try:
            db.drop_collection(collection_name)
            logger.info(f"Dropped collection: {collection_name}")
        except Exception as e:
            logger.error(f"Failed to drop collection {collection_name}: {e}")

    logger.warning("Finished dropping GraphRAG collections")
