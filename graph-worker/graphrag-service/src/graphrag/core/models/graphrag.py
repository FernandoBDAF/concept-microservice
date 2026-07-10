"""
GraphRAG Pydantic Models

This module defines the structured data models used throughout the GraphRAG pipeline.
These models ensure type safety and validation for entity extraction, resolution,
and graph construction processes.
"""

from enum import Enum
from typing import List, Optional, Dict, Any
from pydantic import BaseModel, Field, field_validator
import hashlib


class EntityType(str, Enum):
    """Entity types for GraphRAG extraction."""

    PERSON = "PERSON"
    ORGANIZATION = "ORGANIZATION"
    TECHNOLOGY = "TECHNOLOGY"
    CONCEPT = "CONCEPT"
    LOCATION = "LOCATION"
    EVENT = "EVENT"
    OTHER = "OTHER"


class EntityModel(BaseModel):
    """Model representing an entity extracted from text."""

    name: str = Field(
        description="Name of the entity, capitalized", min_length=1, max_length=200
    )
    type: EntityType = Field(description="Type of the entity", default=EntityType.OTHER)
    description: str = Field(
        description="Comprehensive description of the entity's attributes and activities",
        min_length=10,
        max_length=2000,
    )
    confidence: float = Field(
        description="Confidence score for entity extraction",
        ge=0.0,
        le=1.0,
        default=0.0,
    )

    @field_validator("name")
    @classmethod
    def capitalize_name(cls, value: str) -> str:
        """Ensure entity name is properly capitalized."""
        return value.strip().title()

    @field_validator("description")
    @classmethod
    def validate_description(cls, value: str) -> str:
        """Ensure description is meaningful."""
        if len(value.strip()) < 10:
            raise ValueError("Description must be at least 10 characters long")
        return value.strip()


class RelationshipModel(BaseModel):
    """Model representing a relationship between two entities."""

    source_entity: EntityModel = Field(description="Source entity of the relationship")
    target_entity: EntityModel = Field(description="Target entity of the relationship")
    relation: str = Field(
        description="Type of relationship between entities",
        min_length=1,
        max_length=100,
    )
    description: str = Field(
        description="Explanation of why the entities are related",
        min_length=10,
        max_length=1000,
    )
    confidence: float = Field(
        description="Confidence score for relationship extraction",
        ge=0.0,
        le=1.0,
        default=0.0,
    )

    @field_validator("relation")
    @classmethod
    def validate_relation(cls, value: str) -> str:
        """Ensure relation is properly formatted."""
        return value.strip().lower()

    @field_validator("description")
    @classmethod
    def validate_description(cls, value: str) -> str:
        """Ensure description is meaningful."""
        if len(value.strip()) < 10:
            raise ValueError(
                "Relationship description must be at least 10 characters long"
            )
        return value.strip()


class KnowledgeModel(BaseModel):
    """
    Model representing extracted knowledge from a text chunk.
    Contains entities and relationships identified in the text.
    """

    entities: List[EntityModel] = Field(
        description="List of entities identified in the text", default_factory=list
    )
    relationships: List[RelationshipModel] = Field(
        description="List of relationships identified between entities",
        default_factory=list,
    )

    @field_validator("entities")
    @classmethod
    def validate_entities(cls, value: List[EntityModel]) -> List[EntityModel]:
        """Ensure we have at least one entity."""
        if not value:
            raise ValueError("At least one entity must be identified")
        return value


class KeywordsModel(BaseModel):
    """Model for query entity extraction and keyword generation."""

    keywords: List[str] = Field(
        description="List of synonyms or related keywords for entity search",
        min_length=1,
        max_length=10,
    )

    @field_validator("keywords")
    @classmethod
    def validate_keywords(cls, value: List[str]) -> List[str]:
        """Ensure keywords are properly formatted."""
        return [kw.strip().title() for kw in value if kw.strip()]


class ResolvedEntity(BaseModel):
    """Model for resolved/canonicalized entities."""

    entity_id: str = Field(
        description="Unique identifier for the resolved entity",
        min_length=32,
        max_length=32,
    )
    canonical_name: str = Field(
        description="Canonical name of the entity", min_length=1, max_length=200
    )
    name: str = Field(
        description="Primary name of the entity", min_length=1, max_length=200
    )
    type: EntityType = Field(description="Type of the entity")
    description: str = Field(
        description="Resolved comprehensive description", min_length=10, max_length=2000
    )
    confidence: float = Field(description="Resolution confidence score", ge=0.0, le=1.0)
    source_count: int = Field(
        description="Number of source chunks mentioning this entity", ge=1
    )
    resolution_methods: List[str] = Field(
        description="Methods used for entity resolution", default_factory=list
    )
    aliases: List[str] = Field(
        description="Alternative names for this entity", default_factory=list
    )

    @field_validator("entity_id")
    @classmethod
    def validate_entity_id(cls, value: str) -> str:
        """Ensure entity_id is a valid MD5 hash."""
        if len(value) != 32 or not all(c in "0123456789abcdef" for c in value.lower()):
            raise ValueError("entity_id must be a 32-character MD5 hash")
        return value.lower()

    @classmethod
    def generate_entity_id(
        cls, canonical_name: str, entity_type: Optional[EntityType] = None
    ) -> str:
        """
        Generate stable, deterministic MD5 hash for entity ID.

        Uses normalized canonical name + entity type to ensure the same entity
        always gets the same ID across chunks and runs, even if canonical name
        varies slightly. This prevents entity ID drift.

        Args:
            canonical_name: Canonical name of the entity
            entity_type: Type of the entity (optional for backward compatibility)

        Returns:
            32-character MD5 hash string
        """
        # Normalize the canonical name (lowercase, strip whitespace)
        normalized_name = canonical_name.lower().strip()

        # Include entity type in hash to ensure different types get different IDs
        # even if they have the same name (e.g., "Python" person vs "Python" language)
        if entity_type:
            # Use type value for consistency
            type_str = (
                entity_type.value
                if isinstance(entity_type, EntityType)
                else str(entity_type)
            )
            content = f"{normalized_name}|{type_str}"
        else:
            # Backward compatibility: use name only if type not provided
            content = normalized_name

        return hashlib.md5(content.encode("utf-8")).hexdigest()


class ResolvedRelationship(BaseModel):
    """Model for resolved/canonicalized relationships."""

    relationship_id: str = Field(
        description="Unique identifier for the relationship",
        min_length=32,
        max_length=32,
    )
    subject_id: str = Field(
        description="ID of the subject entity", min_length=32, max_length=32
    )
    object_id: str = Field(
        description="ID of the object entity", min_length=32, max_length=32
    )
    predicate: str = Field(
        description="Type of relationship", min_length=1, max_length=100
    )
    description: str = Field(
        description="Resolved relationship description", min_length=10, max_length=1000
    )
    confidence: float = Field(description="Resolution confidence score", ge=0.0, le=1.0)
    source_count: int = Field(
        description="Number of source chunks mentioning this relationship", ge=1
    )

    @field_validator("subject_id", "object_id")
    @classmethod
    def validate_entity_ids(cls, value: str) -> str:
        """Ensure entity IDs are valid MD5 hashes."""
        if len(value) != 32 or not all(c in "0123456789abcdef" for c in value.lower()):
            raise ValueError("Entity IDs must be 32-character MD5 hashes")
        return value.lower()

    @classmethod
    def generate_relationship_id(
        cls, subject_id: str, object_id: str, predicate: str
    ) -> str:
        """Generate MD5 hash for relationship ID."""
        content = f"{subject_id}:{object_id}:{predicate}".lower()
        return hashlib.md5(content.encode()).hexdigest()


class CommunitySummary(BaseModel):
    """Model for community summaries."""

    community_id: str = Field(
        description="Unique identifier for the community", min_length=1, max_length=100
    )
    level: int = Field(description="Hierarchical level of the community", ge=1, le=10)
    title: str = Field(
        description="Short descriptive title for the community",
        min_length=1,
        max_length=200,
    )
    summary: str = Field(
        description="Comprehensive summary of the community",
        min_length=50,
        max_length=5000,
    )
    entities: List[str] = Field(
        description="List of entity IDs in this community", min_length=1
    )
    entity_count: int = Field(description="Number of entities in the community", ge=1)
    relationship_count: int = Field(
        description="Number of relationships in the community", ge=0
    )
    coherence_score: float = Field(
        description="Coherence score for the community", ge=0.0, le=1.0
    )

    @field_validator("entities")
    @classmethod
    def validate_entities(cls, value: List[str]) -> List[str]:
        """Ensure entity IDs are valid."""
        for entity_id in value:
            if len(entity_id) != 32 or not all(
                c in "0123456789abcdef" for c in entity_id.lower()
            ):
                raise ValueError(f"Invalid entity ID: {entity_id}")
        return value


class GraphRAGQuery(BaseModel):
    """Model for GraphRAG query processing."""

    query_text: str = Field(
        description="Original user query text", min_length=1, max_length=1000
    )
    extracted_entities: List[str] = Field(
        description="Entities extracted from the query", default_factory=list
    )
    query_intent: str = Field(
        description="Detected intent of the query", default="general"
    )
    keywords: List[str] = Field(
        description="Keywords for entity search", default_factory=list
    )

    @field_validator("query_text")
    @classmethod
    def validate_query_text(cls, value: str) -> str:
        """Ensure query text is meaningful."""
        if len(value.strip()) < 1:
            raise ValueError("Query text cannot be empty")
        return value.strip()


class GraphRAGResponse(BaseModel):
    """Model for GraphRAG response."""

    answer: str = Field(description="Generated answer to the query", min_length=1)
    entities: List[Dict[str, Any]] = Field(
        description="Entities used in the response", default_factory=list
    )
    communities: List[Dict[str, Any]] = Field(
        description="Communities used in the response", default_factory=list
    )
    context_sources: List[str] = Field(
        description="Source chunks used for context", default_factory=list
    )
    confidence: float = Field(
        description="Confidence score for the response", ge=0.0, le=1.0
    )
    processing_time: float = Field(
        description="Time taken to process the query in seconds", ge=0.0
    )
