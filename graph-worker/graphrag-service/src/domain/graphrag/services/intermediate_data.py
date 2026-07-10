"""
Intermediate Data Service

This module provides functionality for saving and querying intermediate data
at GraphRAG pipeline stage boundaries, enabling before/after analysis.

Achievement 0.2: Intermediate Data Collections
"""

import logging
import time
from typing import Dict, List, Any, Optional
from datetime import datetime, timezone, timedelta
from pymongo.database import Database
from pymongo.collection import Collection
from src.lib.error_handling.decorators import handle_errors

logger = logging.getLogger(__name__)


class IntermediateDataService:
    """
    Service for managing intermediate data collections at stage boundaries.

    Collections:
    - entities_raw: Entities as extracted (before resolution)
    - entities_resolved: Entities after resolution (before graph)
    - relations_raw: Relationships as extracted (before post-processing)
    - relations_final: Relationships after post-processing (before detection)
    - graph_pre_detection: Graph structure before community detection
    """

    # Collection names
    ENTITIES_RAW = "entities_raw"
    ENTITIES_RESOLVED = "entities_resolved"
    RELATIONS_RAW = "relations_raw"
    RELATIONS_FINAL = "relations_final"
    GRAPH_PRE_DETECTION = "graph_pre_detection"

    def __init__(self, db: Database, enabled: bool = True, ttl_days: int = 7):
        """
        Initialize the Intermediate Data Service.

        Args:
            db: MongoDB database instance
            enabled: Whether intermediate data saving is enabled
            ttl_days: Number of days to retain intermediate data (auto-delete after)
        """
        self.db = db
        self.enabled = enabled
        self.ttl_days = ttl_days
        self.collections: Dict[str, Collection] = {}

        if self.enabled:
            self._initialize_collections()
            self._ensure_indexes()

        logger.info(f"Initialized IntermediateDataService (enabled={enabled}, ttl_days={ttl_days})")

    def _initialize_collections(self):
        """Initialize all intermediate data collections."""
        collection_names = [
            self.ENTITIES_RAW,
            self.ENTITIES_RESOLVED,
            self.RELATIONS_RAW,
            self.RELATIONS_FINAL,
            self.GRAPH_PRE_DETECTION,
        ]

        for name in collection_names:
            self.collections[name] = self.db[name]
            logger.debug(f"Initialized collection: {name}")

    def _ensure_indexes(self):
        """Create indexes for all intermediate data collections."""
        if not self.collections:
            return

        try:
            # TTL in seconds
            ttl_seconds = self.ttl_days * 24 * 60 * 60

            for name, collection in self.collections.items():
                # Single field indexes
                collection.create_index("trace_id")
                collection.create_index("timestamp")
                collection.create_index("chunk_id")
                collection.create_index("video_id")

                # Compound indexes for common queries
                collection.create_index([("trace_id", 1), ("timestamp", 1)])
                collection.create_index([("chunk_id", 1), ("timestamp", 1)])

                # TTL index for automatic cleanup
                collection.create_index("timestamp", expireAfterSeconds=ttl_seconds)

                logger.debug(f"Created indexes for {name} (TTL: {self.ttl_days} days)")

        except Exception as e:
            logger.warning(f"Failed to create intermediate data indexes: {e}")

    @handle_errors()
    def save_entities_raw(
        self,
        entities: List[Dict[str, Any]],
        chunk_id: str,
        video_id: str,
        trace_id: str,
        extraction_method: str = "llm",
    ) -> int:
        """
        Save raw entities (before resolution).

        Args:
            entities: List of extracted entities
            chunk_id: Source chunk ID
            video_id: Source video ID
            trace_id: Trace ID for linking
            extraction_method: Method used for extraction

        Returns:
            Number of entities saved
        """
        if not self.enabled or not entities:
            return 0

        timestamp = time.time()
        documents = []

        for entity in entities:
            doc = {
                "trace_id": trace_id,
                "chunk_id": chunk_id,
                "video_id": video_id,
                "timestamp": timestamp,
                "datetime": datetime.now(timezone.utc).isoformat(),
                "stage": "extraction",
                "extraction_method": extraction_method,
                # Entity data
                "entity_name": entity.get("name", ""),
                "entity_type": entity.get("type", "OTHER"),
                "description": entity.get("description", ""),
                "confidence": entity.get("confidence", 0.0),
            }
            documents.append(doc)

        try:
            result = self.collections[self.ENTITIES_RAW].insert_many(documents)
            count = len(result.inserted_ids)
            logger.debug(f"Saved {count} raw entities for chunk {chunk_id} (trace_id: {trace_id})")
            return count
        except Exception as e:
            logger.error(f"Failed to save raw entities: {e}")
            return 0

    @handle_errors()
    def save_entities_resolved(
        self,
        entities: List[Dict[str, Any]],
        chunk_id: str,
        video_id: str,
        trace_id: str,
        resolution_method: str = "fuzzy_match",
    ) -> int:
        """
        Save resolved entities (after resolution, before graph).

        Args:
            entities: List of resolved entities
            chunk_id: Source chunk ID
            video_id: Source video ID
            trace_id: Trace ID for linking
            resolution_method: Method used for resolution

        Returns:
            Number of entities saved
        """
        if not self.enabled or not entities:
            return 0

        timestamp = time.time()
        documents = []

        for entity in entities:
            doc = {
                "trace_id": trace_id,
                "chunk_id": chunk_id,
                "video_id": video_id,
                "timestamp": timestamp,
                "datetime": datetime.now(timezone.utc).isoformat(),
                "stage": "resolution",
                "resolution_method": resolution_method,
                # Entity data
                "entity_id": entity.get("entity_id", ""),
                "canonical_name": entity.get("canonical_name", ""),
                "entity_type": entity.get("type", "OTHER"),
                "aliases": entity.get("aliases", []),
                "confidence": entity.get("confidence", 0.0),
                "source_count": entity.get("source_count", 1),
            }
            documents.append(doc)

        try:
            result = self.collections[self.ENTITIES_RESOLVED].insert_many(documents)
            count = len(result.inserted_ids)
            logger.debug(
                f"Saved {count} resolved entities for chunk {chunk_id} (trace_id: {trace_id})"
            )
            return count
        except Exception as e:
            logger.error(f"Failed to save resolved entities: {e}")
            return 0

    @handle_errors()
    def save_relations_raw(
        self,
        relationships: List[Dict[str, Any]],
        chunk_id: str,
        video_id: str,
        trace_id: str,
        extraction_method: str = "llm",
    ) -> int:
        """
        Save raw relationships (before post-processing).

        Args:
            relationships: List of extracted relationships
            chunk_id: Source chunk ID
            video_id: Source video ID
            trace_id: Trace ID for linking
            extraction_method: Method used for extraction

        Returns:
            Number of relationships saved
        """
        if not self.enabled or not relationships:
            return 0

        timestamp = time.time()
        documents = []

        for rel in relationships:
            doc = {
                "trace_id": trace_id,
                "chunk_id": chunk_id,
                "video_id": video_id,
                "timestamp": timestamp,
                "datetime": datetime.now(timezone.utc).isoformat(),
                "stage": "extraction",
                "extraction_method": extraction_method,
                # Relationship data
                "source_entity": rel.get("source_entity", {}),
                "target_entity": rel.get("target_entity", {}),
                "relation": rel.get("relation", ""),
                "description": rel.get("description", ""),
                "confidence": rel.get("confidence", 0.0),
            }
            documents.append(doc)

        try:
            result = self.collections[self.RELATIONS_RAW].insert_many(documents)
            count = len(result.inserted_ids)
            logger.debug(
                f"Saved {count} raw relationships for chunk {chunk_id} (trace_id: {trace_id})"
            )
            return count
        except Exception as e:
            logger.error(f"Failed to save raw relationships: {e}")
            return 0

    @handle_errors()
    def save_relations_final(
        self,
        relationships: List[Dict[str, Any]],
        chunk_id: str,
        video_id: str,
        trace_id: str,
        processing_method: str = "post_processing",
    ) -> int:
        """
        Save final relationships (after post-processing, before detection).

        Args:
            relationships: List of processed relationships
            chunk_id: Source chunk ID
            video_id: Source video ID
            trace_id: Trace ID for linking
            processing_method: Method used for post-processing

        Returns:
            Number of relationships saved
        """
        if not self.enabled or not relationships:
            return 0

        timestamp = time.time()
        documents = []

        for rel in relationships:
            doc = {
                "trace_id": trace_id,
                "chunk_id": chunk_id,
                "video_id": video_id,
                "timestamp": timestamp,
                "datetime": datetime.now(timezone.utc).isoformat(),
                "stage": "post_processing",
                "processing_method": processing_method,
                # Relationship data
                "source_entity_id": rel.get("source_entity_id", ""),
                "target_entity_id": rel.get("target_entity_id", ""),
                "relation_type": rel.get("relation_type", ""),
                "weight": rel.get("weight", 1.0),
                "confidence": rel.get("confidence", 0.0),
                "co_occurrences": rel.get("co_occurrences", 1),
            }
            documents.append(doc)

        try:
            result = self.collections[self.RELATIONS_FINAL].insert_many(documents)
            count = len(result.inserted_ids)
            logger.debug(
                f"Saved {count} final relationships for chunk {chunk_id} (trace_id: {trace_id})"
            )
            return count
        except Exception as e:
            logger.error(f"Failed to save final relationships: {e}")
            return 0

    @handle_errors()
    def save_graph_pre_detection(
        self,
        graph_data: Dict[str, Any],
        video_id: str,
        trace_id: str,
    ) -> bool:
        """
        Save graph structure before community detection.

        Args:
            graph_data: Graph structure data (nodes, edges, stats)
            video_id: Source video ID
            trace_id: Trace ID for linking

        Returns:
            True if saved successfully, False otherwise
        """
        if not self.enabled:
            return False

        timestamp = time.time()

        doc = {
            "trace_id": trace_id,
            "video_id": video_id,
            "timestamp": timestamp,
            "datetime": datetime.now(timezone.utc).isoformat(),
            "stage": "pre_detection",
            # Graph data
            "node_count": graph_data.get("node_count", 0),
            "edge_count": graph_data.get("edge_count", 0),
            "density": graph_data.get("density", 0.0),
            "avg_degree": graph_data.get("avg_degree", 0.0),
            "connected_components": graph_data.get("connected_components", 0),
            # Full graph structure (optional, can be large)
            "nodes": graph_data.get("nodes", []),
            "edges": graph_data.get("edges", []),
        }

        try:
            self.collections[self.GRAPH_PRE_DETECTION].insert_one(doc)
            logger.debug(f"Saved graph pre-detection for video {video_id} (trace_id: {trace_id})")
            return True
        except Exception as e:
            logger.error(f"Failed to save graph pre-detection: {e}")
            return False

    # Query methods for analysis

    def get_entities_raw(self, trace_id: str) -> List[Dict[str, Any]]:
        """Get raw entities for a trace ID."""
        if not self.enabled:
            return []
        return list(self.collections[self.ENTITIES_RAW].find({"trace_id": trace_id}))

    def get_entities_resolved(self, trace_id: str) -> List[Dict[str, Any]]:
        """Get resolved entities for a trace ID."""
        if not self.enabled:
            return []
        return list(self.collections[self.ENTITIES_RESOLVED].find({"trace_id": trace_id}))

    def get_relations_raw(self, trace_id: str) -> List[Dict[str, Any]]:
        """Get raw relationships for a trace ID."""
        if not self.enabled:
            return []
        return list(self.collections[self.RELATIONS_RAW].find({"trace_id": trace_id}))

    def get_relations_final(self, trace_id: str) -> List[Dict[str, Any]]:
        """Get final relationships for a trace ID."""
        if not self.enabled:
            return []
        return list(self.collections[self.RELATIONS_FINAL].find({"trace_id": trace_id}))

    def get_graph_pre_detection(self, trace_id: str) -> Optional[Dict[str, Any]]:
        """Get graph structure before detection for a trace ID."""
        if not self.enabled:
            return None
        return self.collections[self.GRAPH_PRE_DETECTION].find_one({"trace_id": trace_id})

    def compare_entities(self, trace_id: str) -> Dict[str, Any]:
        """
        Compare raw vs resolved entities for a trace ID.

        Returns:
            Dictionary with comparison statistics
        """
        raw = self.get_entities_raw(trace_id)
        resolved = self.get_entities_resolved(trace_id)

        return {
            "trace_id": trace_id,
            "raw_count": len(raw),
            "resolved_count": len(resolved),
            "reduction_rate": 1 - (len(resolved) / len(raw)) if raw else 0,
            "raw_entities": raw,
            "resolved_entities": resolved,
        }

    def compare_relations(self, trace_id: str) -> Dict[str, Any]:
        """
        Compare raw vs final relationships for a trace ID.

        Returns:
            Dictionary with comparison statistics
        """
        raw = self.get_relations_raw(trace_id)
        final = self.get_relations_final(trace_id)

        return {
            "trace_id": trace_id,
            "raw_count": len(raw),
            "final_count": len(final),
            "augmentation_rate": (len(final) / len(raw)) - 1 if raw else 0,
            "raw_relations": raw,
            "final_relations": final,
        }
