"""
Graph Construction Stage

This stage builds the knowledge graph by resolving relationships and storing them
in the relations collection. It also calculates basic graph metrics.
"""

import logging
import os
import time
from typing import Dict, List, Any, Optional, Iterator, Set
from collections import defaultdict
import numpy as np
from src.core.base.stage import BaseStage
from src.core.config.graphrag import GraphConstructionConfig
from src.domain.agents.graphrag.relationship_resolution import RelationshipResolutionAgent
from src.core.models.graphrag import ResolvedRelationship
from src.domain.services.graphrag.indexes import get_graphrag_collections
from src.domain.services.graphrag.transformation_logger import TransformationLogger
from src.domain.services.graphrag.intermediate_data import IntermediateDataService
from src.core.config.paths import COLL_CHUNKS
from src.lib.database import batch_insert
from src.lib.error_handling.decorators import handle_errors
from pymongo import ReturnDocument
from pymongo.errors import DuplicateKeyError

logger = logging.getLogger(__name__)


class GraphConstructionStage(BaseStage):
    """
    Stage for constructing the knowledge graph from resolved entities and relationships.
    """

    name = "graph_construction"
    description = "Build knowledge graph from resolved entities and relationships"
    ConfigCls = GraphConstructionConfig

    def __init__(self):
        """Initialize the Graph Construction Stage."""
        super().__init__()
        # Don't initialize agent here - will be done in setup()
        # Initialize comprehensive metrics tracking (Achievement 3.3)
        self.post_processing_stats = {
            "co_occurrence": {"added": 0, "skipped": 0, "capped": 0},
            "semantic_similarity": {"added": 0, "skipped": 0, "capped": 0},
            "cross_chunk": {"added": 0, "skipped": 0},
            "bidirectional": {"added": 0, "skipped": 0},
            "predicted": {"added": 0, "skipped": 0},
        }

    def setup(self):
        """Setup the stage with config-dependent initialization."""
        super().setup()

        # Initialize OpenAI client for LLM operations
        from src.lib.llm import get_openai_client

        self.llm_client = get_openai_client(timeout=60)

        # Initialize the relationship resolution agent now that we have access to self.config
        self.relationship_agent = RelationshipResolutionAgent(
            llm_client=self.llm_client,
            model_name="gpt-4o-mini",  # Default model for relationship resolution
            temperature=0.1,
        )

        # Get GraphRAG collections (use write DB for output)
        self.graphrag_collections = get_graphrag_collections(self.db_write)

        # Initialize TransformationLogger for logging relationship operations (Achievement 0.1)
        logging_enabled = os.getenv("GRAPHRAG_TRANSFORMATION_LOGGING", "true").lower() == "true"
        self.transformation_logger = TransformationLogger(self.db_write, enabled=logging_enabled)

        # Initialize IntermediateDataService for saving intermediate data (Achievement 0.2)
        intermediate_data_enabled = (
            os.getenv("GRAPHRAG_SAVE_INTERMEDIATE_DATA", "false").lower() == "true"
        )
        self.intermediate_data = IntermediateDataService(
            self.db_write,
            enabled=intermediate_data_enabled,
            ttl_days=int(os.getenv("GRAPHRAG_INTERMEDIATE_DATA_TTL_DAYS", "7")),
        )

        # Load ontology for predicate metadata (Achievement 3.1)
        try:
            from src.lib.ontology.loader import load_ontology

            self.ontology = load_ontology()
            logger.info(
                f"Loaded ontology: {len(self.ontology.get('canonical_predicates', set()))} canonical predicates, "
                f"{len(self.ontology.get('symmetric_predicates', set()))} symmetric predicates"
            )
        except Exception as e:
            logger.warning(f"Failed to load ontology: {e}. Continuing with fallback.")
            self.ontology = {
                "canonical_predicates": set(),
                "symmetric_predicates": set(),
                "predicate_map": {},
            }

        logger.info(f"Initialized {self.name}")

    def _get_entity_degree(self, entity_id: str, relationship_type: Optional[str] = None) -> int:
        """
        Get the degree (number of relationships) for an entity.

        Args:
            entity_id: Entity ID to check
            relationship_type: Optional filter by relationship_type (e.g., "co_occurrence", "semantic_similarity")

        Returns:
            Number of relationships where entity is subject or object
        """
        relations_collection = self.graphrag_collections["relations"]

        query = {
            "$or": [
                {"subject_id": entity_id},
                {"object_id": entity_id},
            ]
        }

        if relationship_type:
            query["relationship_type"] = relationship_type

        return relations_collection.count_documents(query)

    def _get_reverse_predicate(self, predicate: str) -> Optional[str]:
        """
        Get the reverse predicate for a given predicate using ontology data.

        Args:
            predicate: The predicate to get reverse for

        Returns:
            Reverse predicate name, or None if:
            - Predicate is symmetric (already bidirectional)
            - No reverse predicate found in ontology
        """
        # Check if predicate is symmetric (no reverse needed)
        symmetric_predicates = self.ontology.get("symmetric_predicates", set())
        if predicate in symmetric_predicates:
            return None  # Symmetric predicates don't need reverse

        # Build reverse mapping from predicate_map (if it contains reverse pairs)
        # For now, use a fallback pattern-based approach
        # Common patterns: "X" → "X_by", "X" → "Xed_by", etc.
        reverse_patterns = {
            "uses": "used_by",
            "teaches": "taught_by",
            "creates": "created_by",
            "develops": "developed_by",
            "implements": "implemented_by",
            "contains": "contained_in",
            "has": "belongs_to",
            "manages": "managed_by",
            "leads": "led_by",
            "explains": "explained_by",
            "demonstrates": "demonstrated_by",
            "requires": "required_by",
            "depends_on": "dependency_of",
            "applies_to": "applied_by",
            "works_at": "employs",
            "part_of": "has_part",
            "subtype_of": "has_subtype",
            "is_a": "has_instance",
        }

        # Check fallback mapping
        return reverse_patterns.get(predicate)

    def _build_attribution(
        self, stage_name: str, algorithm: str, params: Optional[Dict[str, Any]] = None
    ) -> Dict[str, Any]:
        """
        Build attribution dictionary for relationships (Achievement 3.2).

        Args:
            stage_name: Name of the stage creating the relationship
            algorithm: Algorithm used (e.g., "co_occurrence", "semantic_similarity")
            params: Optional dictionary of algorithm parameters

        Returns:
            Attribution dictionary with created_by_stage, algorithm, algorithm_version, params
        """
        return {
            "created_by_stage": stage_name,
            "algorithm": algorithm,
            "algorithm_version": "1.0",  # Can be made configurable later
            "params": params or {},
            "created_at": time.time(),
        }

    def iter_docs(self) -> Iterator[Dict[str, Any]]:
        """
        Iterate over chunks that have completed entity resolution.

        Yields:
            Chunk documents that have been processed for entity resolution
        """
        src_db = self.config.read_db_name or self.config.db_name
        src_coll_name = self.config.read_coll or COLL_CHUNKS
        collection = self.get_collection(src_coll_name, io="read", db_name=src_db)

        # Query for chunks that have completed resolution but not graph construction
        query = {
            "graphrag_resolution.status": "completed",
            "$or": [
                {"graphrag_construction": {"$exists": False}},
                {"graphrag_construction.status": {"$ne": "completed"}},
            ],
        }

        # Skip chunks marked for exclusion (used in random chunk testing)
        query["_test_exclude"] = {"$exists": False}

        # Apply video filtering from source selection
        if self.config.video_id:
            # Single video filter (CLI argument)
            query["video_id"] = self.config.video_id
        elif self.config.input_video_ids:
            # Multiple video filter (source selection filter)
            query["video_id"] = {"$in": self.config.input_video_ids}
            logger.info(f"Filtering to {len(self.config.input_video_ids)} video(s) from source selection")

        logger.info(f"Querying chunks for graph construction: {query}")

        cursor = collection.find(query)
        if self.config.max:
            cursor = cursor.limit(int(self.config.max))

        for doc in cursor:
            yield doc

    @handle_errors(fallback=False, log_traceback=True, reraise=False)
    def handle_doc(self, doc: Dict[str, Any]) -> bool:
        """
        Process a single chunk for graph construction and write to database.

        Args:
            doc: Chunk document to process

        Returns:
            True if processing succeeded, False if it failed (Achievement 0.3)
        """
        chunk_id = doc.get("chunk_id", "unknown")
        video_id = doc.get("video_id", "unknown")

        logger.debug(f"Processing chunk {chunk_id} from video {video_id} for graph construction")

        try:
            # Extract relationship data from the chunk
            extraction_data = doc.get("graphrag_extraction", {}).get("data", {})

            # Get trace_id for linking
            trace_id = getattr(self.config, "trace_id", None) or "unknown"

            # Achievement 0.2: Save raw relationships (before post-processing)
            if extraction_data and "relationships" in extraction_data:
                raw_relationships = extraction_data.get("relationships", [])
                self.intermediate_data.save_relations_raw(
                    relationships=raw_relationships,
                    chunk_id=chunk_id,
                    video_id=video_id,
                    trace_id=trace_id,
                    extraction_method="llm",
                )

            if not extraction_data or "relationships" not in extraction_data:
                logger.warning(f"No relationship extraction data found in chunk {chunk_id}")
                # Achievement 0.1: Log relationship skip (no extraction data)
                trace_id = getattr(self.config, "trace_id", None) or "unknown"
                self.transformation_logger.log_relationship_filter(
                    relationship={"source": chunk_id, "target": chunk_id},
                    reason="no_extraction_data",
                    confidence=0.0,
                    threshold=0.0,  # FIX: No threshold applies when there's no data
                    trace_id=trace_id,
                )
                return self._mark_construction_failed(doc, "no_extraction_data")

            # Get entity name → ID mapping for relationship resolution
            entity_name_to_id = self._get_entity_name_to_id_mapping()

            # Resolve relationships for this chunk
            resolved_relationships = self.relationship_agent.resolve_relationships(
                [extraction_data], entity_name_to_id=entity_name_to_id
            )

            if not resolved_relationships:
                logger.warning(f"No relationships resolved for chunk {chunk_id}")
                # Achievement 0.1: Log relationship filter (no relationships resolved)
                trace_id = getattr(self.config, "trace_id", None) or "unknown"
                self.transformation_logger.log_relationship_filter(
                    relationship={"source": chunk_id, "target": chunk_id},
                    reason="no_relationships_resolved",
                    confidence=0.0,
                    threshold=0.0,  # FIX: No threshold applies when resolution fails
                    trace_id=trace_id,
                )
                return self._mark_construction_failed(doc, "no_relationships_resolved")

            # Get existing entity IDs for validation
            entity_ids = self._get_existing_entity_ids()

            # Validate relationships against existing entities
            validated_relationships = self.relationship_agent.validate_entity_existence(
                resolved_relationships, entity_ids
            )

            if not validated_relationships:
                logger.warning(f"No validated relationships for chunk {chunk_id}")
                return self._mark_construction_failed(doc, "no_validated_relationships")

            # Store relationships in the relations collection
            stored_relationships = self._store_resolved_relationships(
                validated_relationships, chunk_id, video_id
            )

            # Achievement 0.2: Save final relationships (after post-processing)
            final_relationships_data = []
            for rel_id in stored_relationships:
                # Get the stored relationship from database
                rel_doc = self.graphrag_collections["relations"].find_one({"_id": rel_id})
                if rel_doc:
                    final_relationships_data.append(
                        {
                            "source_entity_id": rel_doc.get("subject_id", ""),
                            "target_entity_id": rel_doc.get("object_id", ""),
                            "relation_type": rel_doc.get("relationship_type", ""),
                            "weight": rel_doc.get("weight", 1.0),
                            "confidence": rel_doc.get("confidence", 0.0),
                            "co_occurrences": rel_doc.get("co_occurrences", 1),
                        }
                    )
            self.intermediate_data.save_relations_final(
                relationships=final_relationships_data,
                chunk_id=chunk_id,
                video_id=video_id,
                trace_id=trace_id,
                processing_method="post_processing",
            )

            # Prepare construction payload
            construction_payload = {
                "graphrag_construction": {
                    "status": "completed",
                    "resolved_relationships": len(resolved_relationships),
                    "validated_relationships": len(validated_relationships),
                    "stored_relationships": len(stored_relationships),
                    "processed_at": time.time(),
                }
            }

            # Write to database
            dst_db = self.config.write_db_name or self.config.db_name
            dst_coll_name = self.config.write_coll or COLL_CHUNKS
            collection = self.get_collection(dst_coll_name, io="write", db_name=dst_db)

            # Check if already processed (unless upsert_existing is True)
            if not self.config.upsert_existing:
                existing = collection.find_one(
                    {"video_id": video_id, "chunk_id": chunk_id},
                    {"graphrag_construction.status": 1},
                )
                if (
                    existing
                    and existing.get("graphrag_construction", {}).get("status") == "completed"
                ):
                    logger.debug(f"Skipping chunk {chunk_id} - already has completed construction")
                    self.stats["skipped"] += 1
                    return True  # Skipped is considered success (already processed)

            # Update the chunk with construction results
            collection.update_one(
                {"video_id": video_id, "chunk_id": chunk_id},
                {"$set": construction_payload},
                upsert=False,
            )

            self.stats["updated"] += 1
            logger.debug(
                f"Successfully constructed {len(stored_relationships)} relationships "
                f"for chunk {chunk_id}"
            )

            return True  # Success (Achievement 0.3)

        except Exception as e:
            logger.error(f"Error processing chunk {chunk_id} for graph construction: {e}")
            return self._mark_construction_failed(doc, str(e))

    def _get_existing_entity_ids(self) -> Set[str]:
        """
        Get set of existing entity IDs for validation.

        Returns:
            Set of existing entity IDs
        """
        entities_collection = self.graphrag_collections["entities"]

        entity_docs = entities_collection.find({}, {"entity_id": 1})
        entity_ids = {doc["entity_id"] for doc in entity_docs}

        logger.debug(f"Found {len(entity_ids)} existing entities for validation")
        return entity_ids

    def _get_entity_name_to_id_mapping(self) -> Dict[str, str]:
        """
        Get mapping of entity names to entity IDs from the entities collection.
        Also builds reverse mapping (ID → name) for logging purposes.

        Returns:
            Dictionary mapping entity names (normalized) to entity IDs (32-char MD5 hashes)
        
        Side effect:
            Sets self._entity_id_to_name for use in logging
        """
        entities_collection = self.graphrag_collections["entities"]

        entity_docs = entities_collection.find(
            {}, {"entity_id": 1, "name": 1, "canonical_name": 1, "aliases": 1}
        )

        name_to_id = {}
        id_to_name = {}  # Reverse mapping for logging
        
        for doc in entity_docs:
            entity_id = doc["entity_id"]
            # Get display name (prefer name over canonical_name)
            display_name = doc.get("name") or doc.get("canonical_name") or entity_id[:8]
            
            # Build reverse mapping (ID → display name) for logging
            id_to_name[entity_id] = display_name
            
            # Map canonical_name
            if "canonical_name" in doc:
                canonical_name = doc["canonical_name"].lower().strip()
                if canonical_name:
                    name_to_id[canonical_name] = entity_id
            # Map name
            if "name" in doc:
                name = doc["name"].lower().strip()
                if name:
                    name_to_id[name] = entity_id
            # Map aliases
            if "aliases" in doc:
                for alias in doc["aliases"]:
                    alias_normalized = alias.lower().strip()
                    if alias_normalized:
                        name_to_id[alias_normalized] = entity_id

        # Store reverse mapping for logging use
        self._entity_id_to_name = id_to_name
        
        logger.debug(f"Created entity name → ID mapping with {len(name_to_id)} entries")
        logger.debug(f"Created entity ID → name mapping with {len(id_to_name)} entries (for logging)")
        return name_to_id

    def _store_resolved_relationships(
        self,
        resolved_relationships: List[ResolvedRelationship],
        chunk_id: str,
        video_id: str,
    ) -> List[str]:
        """
        Store resolved relationships in the relations collection.

        Args:
            resolved_relationships: List of resolved relationships
            chunk_id: Source chunk ID
            video_id: Source video ID

        Returns:
            List of stored relationship IDs
        """
        relations_collection = self.graphrag_collections["relations"]
        stored_relationship_ids = []

        for relationship in resolved_relationships:
            try:
                # Check if relationship already exists
                existing_relationship = relations_collection.find_one(
                    {"relationship_id": relationship.relationship_id}
                )

                if existing_relationship:
                    # Update existing relationship
                    self._update_existing_relationship(
                        existing_relationship, relationship, chunk_id, video_id
                    )
                else:
                    # Insert new relationship
                    self._insert_new_relationship(relationship, chunk_id, video_id)

                stored_relationship_ids.append(relationship.relationship_id)

            except Exception as e:
                logger.error(f"Failed to store relationship {relationship.relationship_id}: {e}")
                continue

        return stored_relationship_ids

    def _update_existing_relationship(
        self,
        existing_relationship: Dict[str, Any],
        resolved_relationship: ResolvedRelationship,
        chunk_id: str,
        video_id: str,
    ) -> None:
        """
        Update an existing relationship with new information.

        Args:
            existing_relationship: Existing relationship document
            resolved_relationship: New resolved relationship
            chunk_id: Source chunk ID
            video_id: Source video ID
        """
        relations_collection = self.graphrag_collections["relations"]

        # Check if chunk_id is already in source_chunks
        # Only increment source_count if this is a new chunk (Achievement 0.2)
        source_chunks = existing_relationship.get("source_chunks", [])
        is_new_chunk = chunk_id not in source_chunks

        # Build update document
        update_data = {
            "$addToSet": {"source_chunks": chunk_id},
            "$set": {"updated_at": time.time()},
        }

        # Only increment source_count if chunk_id is new
        if is_new_chunk:
            update_data["$inc"] = {"source_count": 1}

        # Update confidence if new relationship has higher confidence
        if resolved_relationship.confidence > existing_relationship.get("confidence", 0):
            update_data["$set"]["confidence"] = resolved_relationship.confidence

        # Update description if new relationship has more comprehensive description
        if len(resolved_relationship.description) > len(
            existing_relationship.get("description", "")
        ):
            update_data["$set"]["description"] = resolved_relationship.description

        relations_collection.update_one(
            {"relationship_id": resolved_relationship.relationship_id}, update_data
        )

        # Achievement 0.1: Log relationship augmentation (existing relationship updated)
        self.transformation_logger.log_relationship_augment(
            subject_id=resolved_relationship.subject_id,
            object_id=resolved_relationship.object_id,
            predicate=resolved_relationship.predicate,
            method="source_augmentation",
            confidence=resolved_relationship.confidence,
            trace_id=self.config.trace_id if hasattr(self.config, "trace_id") else None,
        )

    def _insert_new_relationship(
        self, resolved_relationship: ResolvedRelationship, chunk_id: str, video_id: str
    ) -> None:
        """
        Insert a new relationship into the relations collection.

        Args:
            resolved_relationship: Resolved relationship to insert
            chunk_id: Source chunk ID
            video_id: Source video ID
        """
        relations_collection = self.graphrag_collections["relations"]

        relationship_doc = {
            "relationship_id": resolved_relationship.relationship_id,
            "subject_id": resolved_relationship.subject_id,
            "object_id": resolved_relationship.object_id,
            "predicate": resolved_relationship.predicate,
            "description": resolved_relationship.description,
            "confidence": resolved_relationship.confidence,
            "source_count": 1,  # New relationship starts with source_count = 1
            "source_chunks": [chunk_id],
            "video_id": video_id,
            "created_at": time.time(),
            "updated_at": time.time(),
            "relationship_type": "extracted",
            # Attribution (Achievement 3.2)
            **self._build_attribution(
                stage_name=self.name,
                algorithm="extraction",
                params={"chunk_id": chunk_id, "video_id": video_id},
            ),
        }

        relations_collection.insert_one(relationship_doc)

        # Achievement 0.1: Log relationship creation
        # Use reverse mapping for human-readable entity names in logs
        subject_name = getattr(self, '_entity_id_to_name', {}).get(
            resolved_relationship.subject_id, resolved_relationship.subject_id[:8]
        )
        object_name = getattr(self, '_entity_id_to_name', {}).get(
            resolved_relationship.object_id, resolved_relationship.object_id[:8]
        )
        self.transformation_logger.log_relationship_create(
            relationship={
                "subject": {"id": resolved_relationship.subject_id, "name": subject_name},
                "predicate": resolved_relationship.predicate,
                "object": {"id": resolved_relationship.object_id, "name": object_name},
            },
            relationship_type="extracted",
            confidence=resolved_relationship.confidence,
            trace_id=self.config.trace_id if hasattr(self.config, "trace_id") else None,
        )

    def _add_co_occurrence_relationships(self) -> int:
        """
        Add co-occurrence relationships for entities that appear in the same chunks.

        This method creates 'co_occurs_with' relationships between entities that
        are mentioned in the same chunk, which helps improve graph connectivity
        and discover implicit relationships.

        Returns:
            Number of co-occurrence relationships added
        """
        logger.info("Starting co-occurrence relationship post-processing")

        mentions_collection = self.graphrag_collections["entity_mentions"]
        relations_collection = self.graphrag_collections["relations"]

        # Group entities by chunk_id
        chunk_entities = defaultdict(set)
        logger.debug("Grouping entities by chunk_id")

        for mention in mentions_collection.find():
            chunk_id = mention.get("chunk_id")
            entity_id = mention.get("entity_id")
            if chunk_id and entity_id:
                chunk_entities[chunk_id].add(entity_id)

        logger.info(f"Found {len(chunk_entities)} chunks with entity mentions")

        # Get edge cap from environment (Achievement 2.3)
        max_cooc_per_entity = int(os.getenv("GRAPHRAG_MAX_COOC_PER_ENTITY", "200"))

        # Collect all relationships to insert in batch
        relationships_to_insert = []
        skipped_count = 0
        capped_count = 0  # Track edges skipped due to degree cap

        for chunk_id, entity_ids in chunk_entities.items():
            if len(entity_ids) < 2:
                continue

            # Create relationships between all pairs
            entity_list = list(entity_ids)
            for i, entity1_id in enumerate(entity_list):
                for entity2_id in entity_list[i + 1 :]:
                    # Check if relationship already exists (include predicate to allow multiple predicates per pair)
                    existing = relations_collection.find_one(
                        {
                            "subject_id": entity1_id,
                            "object_id": entity2_id,
                            "predicate": "co_occurs_with",
                        }
                    )

                    if existing:
                        skipped_count += 1
                        continue

                    # Check degree caps (Achievement 2.3)
                    degree1 = self._get_entity_degree(entity1_id, "co_occurrence")
                    degree2 = self._get_entity_degree(entity2_id, "co_occurrence")

                    if degree1 >= max_cooc_per_entity or degree2 >= max_cooc_per_entity:
                        capped_count += 1
                        continue

                    # Create co-occurrence relationship
                    relationship_id = ResolvedRelationship.generate_relationship_id(
                        entity1_id, entity2_id, "co_occurs_with"
                    )

                    # Create relationship document
                    relationship_doc = {
                        "relationship_id": relationship_id,
                        "subject_id": entity1_id,
                        "object_id": entity2_id,
                        "predicate": "co_occurs_with",
                        "description": f"These entities appear together in chunk {chunk_id}",
                        "confidence": 0.7,  # Moderate confidence for co-occurrence
                        "source_count": 1,
                        "source_chunks": [chunk_id],
                        "created_at": time.time(),
                        "updated_at": time.time(),
                        "relationship_type": "co_occurrence",  # Mark as auto-generated
                        # Attribution (Achievement 3.2)
                        **self._build_attribution(
                            stage_name=self.name,
                            algorithm="co_occurrence",
                            params={"chunk_id": chunk_id},
                        ),
                    }

                    relationships_to_insert.append(relationship_doc)

        # Batch insert all relationships for better performance
        added_count = 0
        if relationships_to_insert:
            logger.info(
                f"Inserting {len(relationships_to_insert)} co-occurrence relationships in batch"
            )
            try:
                result = batch_insert(
                    collection=relations_collection,
                    documents=relationships_to_insert,
                    batch_size=500,
                    ordered=False,  # Continue on errors
                )
                added_count = result["inserted"]
                logger.info(
                    f"Co-occurrence batch insert: {result['inserted']}/{result['total']} successful, "
                    f"{result['failed']} failed"
                )

                # Achievement 0.1: Log relationship augmentation for co-occurrence
                for rel_doc in relationships_to_insert[: result["inserted"]]:
                    self.transformation_logger.log_relationship_augment(
                        subject_id=rel_doc["subject_id"],
                        object_id=rel_doc["object_id"],
                        predicate=rel_doc["predicate"],
                        method="co_occurrence",
                        confidence=rel_doc["confidence"],
                        trace_id=self.config.trace_id if hasattr(self.config, "trace_id") else None,
                    )
            except DuplicateKeyError:
                # Expected on reruns (idempotency) - unique index on relationship_id prevents duplicates
                logger.debug(
                    "Duplicate key error in co-occurrence batch insert (expected on reruns)"
                )
                # Count successful inserts by checking which ones exist
                for rel_doc in relationships_to_insert:
                    existing = relations_collection.find_one(
                        {"relationship_id": rel_doc["relationship_id"]}
                    )
                    if existing:
                        added_count += 1
                logger.info(
                    f"Co-occurrence batch insert: {added_count}/{len(relationships_to_insert)} relationships already exist (idempotent)"
                )

        logger.info(
            f"Co-occurrence post-processing complete: "
            f"added {added_count} relationships, skipped {skipped_count} existing, "
            f"capped {capped_count} due to degree limits"
        )

        # Update metrics (Achievement 3.3)
        self.post_processing_stats["co_occurrence"]["added"] += added_count
        self.post_processing_stats["co_occurrence"]["skipped"] += skipped_count
        self.post_processing_stats["co_occurrence"]["capped"] += capped_count

        return added_count

    def _add_semantic_similarity_relationships(self, similarity_threshold: float = 0.85) -> int:
        """
        Add semantic similarity relationships between entities based on embeddings.

        Args:
            similarity_threshold: Minimum cosine similarity to create relationship

        Returns:
            Number of similarity relationships added
        """
        logger.info("Starting semantic similarity relationship post-processing")

        entities_collection = self.graphrag_collections["entities"]
        relations_collection = self.graphrag_collections["relations"]

        # Step 1: Get all entities without embeddings and generate them
        entities_to_embed = list(entities_collection.find({"entity_embedding": {"$exists": False}}))

        if entities_to_embed:
            logger.info(f"Generating embeddings for {len(entities_to_embed)} entities")

            for entity in entities_to_embed:
                # Create embedding text from name + description
                embedding_text = f"{entity['name']}: {entity.get('description', '')}"

                try:
                    from src.domain.stages.ingestion.embed import embed_texts

                    embedding = embed_texts([embedding_text])[0]

                    # Normalize embedding once at write time (Achievement 2.2)
                    embedding_array = np.array(embedding)
                    embedding_norm = np.linalg.norm(embedding_array)
                    if embedding_norm > 0:
                        embedding_normalized = (embedding_array / embedding_norm).tolist()
                    else:
                        embedding_normalized = embedding

                    entities_collection.update_one(
                        {"entity_id": entity["entity_id"]},
                        {
                            "$set": {
                                "entity_embedding": embedding_normalized,
                                "entity_embedding_text": embedding_text,
                                "entity_embedding_dim": len(embedding_normalized),
                                "entity_embedding_norm": 1.0,  # Flag: normalized
                            }
                        },
                    )
                except Exception as e:
                    logger.error(f"Failed to embed entity {entity['entity_id']}: {e}")

        # Step 2: Get all entities with embeddings
        entities_with_embeddings = list(
            entities_collection.find({"entity_embedding": {"$exists": True}})
        )

        logger.info(f"Calculating similarity for {len(entities_with_embeddings)} entities")

        # Get edge cap from environment (Achievement 2.3)
        max_sim_per_entity = int(os.getenv("GRAPHRAG_MAX_SIM_PER_ENTITY", "200"))

        # Step 3: Calculate pairwise cosine similarity and collect relationships
        relationships_to_insert = []
        skipped_count = 0
        capped_count = 0  # Track edges skipped due to degree cap

        from itertools import combinations

        for entity1, entity2 in combinations(entities_with_embeddings, 2):
            entity1_id = entity1["entity_id"]
            entity2_id = entity2["entity_id"]

            # Check if relationship already exists (include predicate to allow multiple predicates per pair)
            existing = relations_collection.find_one(
                {
                    "subject_id": entity1_id,
                    "object_id": entity2_id,
                    "predicate": "semantically_similar_to",
                }
            )

            if existing:
                skipped_count += 1
                continue

            # Calculate cosine similarity (optimized for normalized embeddings - Achievement 2.2)
            emb1 = np.array(entity1["entity_embedding"])
            emb2 = np.array(entity2["entity_embedding"])

            # Check if embeddings are normalized (flag or check norm)
            norm1 = entity1.get("entity_embedding_norm", None)
            norm2 = entity2.get("entity_embedding_norm", None)

            if norm1 == 1.0 and norm2 == 1.0:
                # Both normalized: use dot product directly (2-3× faster)
                similarity = np.dot(emb1, emb2)
            else:
                # Not normalized: use standard cosine formula (backward compatibility)
                similarity = np.dot(emb1, emb2) / (np.linalg.norm(emb1) * np.linalg.norm(emb2))

            if similarity >= similarity_threshold:
                # Create similarity relationship
                relationship_id = ResolvedRelationship.generate_relationship_id(
                    entity1_id, entity2_id, "semantically_similar_to"
                )

                relationship_doc = {
                    "relationship_id": relationship_id,
                    "subject_id": entity1_id,
                    "object_id": entity2_id,
                    "predicate": "semantically_similar_to",
                    "description": f"Entities are semantically similar (cosine similarity: {similarity:.3f})",
                    "confidence": float(similarity),
                    "source_count": 1,
                    "source_chunks": [],
                    "created_at": time.time(),
                    "updated_at": time.time(),
                    "relationship_type": "semantic_similarity",
                    "similarity_score": float(similarity),
                    # Attribution (Achievement 3.2)
                    **self._build_attribution(
                        stage_name=self.name,
                        algorithm="semantic_similarity",
                        params={
                            "similarity_threshold": similarity_threshold,
                            "similarity_score": float(similarity),
                        },
                    ),
                }

                relationships_to_insert.append(relationship_doc)

        # Batch insert all similarity relationships
        added_count = 0
        if relationships_to_insert:
            logger.info(
                f"Inserting {len(relationships_to_insert)} semantic similarity relationships in batch"
            )
            try:
                result = batch_insert(
                    collection=relations_collection,
                    documents=relationships_to_insert,
                    batch_size=500,
                    ordered=False,
                )
                added_count = result["inserted"]
                logger.info(
                    f"Semantic similarity batch insert: {result['inserted']}/{result['total']} successful, "
                    f"{result['failed']} failed"
                )

                # Achievement 0.1: Log relationship augmentation for semantic similarity
                for rel_doc in relationships_to_insert[: result["inserted"]]:
                    self.transformation_logger.log_relationship_augment(
                        subject_id=rel_doc["subject_id"],
                        object_id=rel_doc["object_id"],
                        predicate=rel_doc["predicate"],
                        method="semantic_similarity",
                        confidence=rel_doc["confidence"],
                        similarity=rel_doc.get("similarity_score"),
                        trace_id=self.config.trace_id if hasattr(self.config, "trace_id") else None,
                    )
            except DuplicateKeyError:
                # Expected on reruns (idempotency) - unique index on relationship_id prevents duplicates
                logger.debug(
                    "Duplicate key error in semantic similarity batch insert (expected on reruns)"
                )
                # Count successful inserts by checking which ones exist
                for rel_doc in relationships_to_insert:
                    existing = relations_collection.find_one(
                        {"relationship_id": rel_doc["relationship_id"]}
                    )
                    if existing:
                        added_count += 1
                logger.info(
                    f"Semantic similarity batch insert: {added_count}/{len(relationships_to_insert)} relationships already exist (idempotent)"
                )

        logger.info(
            f"Semantic similarity post-processing complete: "
            f"added {added_count} relationships, skipped {skipped_count} existing, "
            f"capped {capped_count} due to degree limits"
        )

        # Update metrics (Achievement 3.3)
        self.post_processing_stats["semantic_similarity"]["added"] += added_count
        self.post_processing_stats["semantic_similarity"]["skipped"] += skipped_count
        self.post_processing_stats["semantic_similarity"]["capped"] += capped_count

        return added_count

    def _add_cross_chunk_relationships(self) -> int:
        """
        Add relationships between entities in nearby chunks (chunk-proximity based).

        Instead of connecting ALL entities in the same video, this connects entities
        only in chunks that are temporally close (within a window of N chunks).
        This preserves local context while avoiding creating a complete graph.

        Returns:
            Number of cross-chunk relationships added
        """
        logger.info("Starting cross-chunk relationship post-processing")

        mentions_collection = self.graphrag_collections["entity_mentions"]
        relations_collection = self.graphrag_collections["relations"]
        entities_collection = self.graphrag_collections["entities"]

        # Get chunk window size from environment (can override adaptive sizing)
        # If not set, will calculate adaptively per video
        chunk_window_override = os.getenv("GRAPHRAG_CROSS_CHUNK_WINDOW")
        use_adaptive_window = chunk_window_override is None

        # Step 1: Get chunk metadata (video_id, timestamp) for all chunks
        chunks_collection = self.get_collection(COLL_CHUNKS, io="read")

        chunk_metadata = {}
        for chunk in chunks_collection.find(
            {"video_id": {"$exists": True}},
            {"chunk_id": 1, "video_id": 1, "timestamp_start": 1},
        ):
            chunk_id = chunk.get("chunk_id")
            video_id = chunk.get("video_id")
            timestamp = chunk.get("timestamp_start", "00:00:00")

            if chunk_id and video_id:
                chunk_metadata[chunk_id] = {
                    "video_id": video_id,
                    "timestamp": timestamp,
                }

        # Step 2: Group entity mentions by chunk
        chunk_entities = defaultdict(set)

        for mention in mentions_collection.find():
            chunk_id = mention.get("chunk_id")
            entity_id = mention.get("entity_id")
            if chunk_id and entity_id:
                chunk_entities[chunk_id].add(entity_id)

        logger.info(f"Found {len(chunk_entities)} chunks with entity mentions")

        # Step 3: Organize chunks by video and sort by timestamp
        video_chunks = defaultdict(list)

        for chunk_id, metadata in chunk_metadata.items():
            if chunk_id in chunk_entities:
                video_id = metadata["video_id"]
                timestamp = metadata["timestamp"]
                video_chunks[video_id].append((timestamp, chunk_id))

        logger.info(f"Found {len(video_chunks)} videos with chunks")

        # Step 4: For each video, only connect entities in nearby chunks
        relationships_to_insert = []
        skipped_count = 0

        for video_id, chunks in video_chunks.items():
            # Sort chunks by timestamp
            chunks.sort()

            # Calculate adaptive window size based on video length
            if use_adaptive_window:
                total_chunks = len(chunks)

                if total_chunks <= 15:
                    chunk_window = 1  # Very short videos: only adjacent chunks
                elif total_chunks <= 30:
                    chunk_window = 2  # Short videos
                elif total_chunks <= 60:
                    chunk_window = 3  # Medium videos
                else:
                    chunk_window = 5  # Long videos (>60 chunks)

                logger.debug(
                    f"Video {video_id}: {total_chunks} chunks, "
                    f"using adaptive window={chunk_window}"
                )
            else:
                chunk_window = int(chunk_window_override)
                logger.debug(
                    f"Video {video_id}: {len(chunks)} chunks, "
                    f"using override window={chunk_window}"
                )

            # Only connect entities in chunks within the window
            for i, (ts1, chunk1_id) in enumerate(chunks):
                # Look at next N chunks within the window
                for j in range(i + 1, min(i + chunk_window + 1, len(chunks))):
                    ts2, chunk2_id = chunks[j]

                    # Get entities from both chunks
                    entities1 = chunk_entities.get(chunk1_id, set())
                    entities2 = chunk_entities.get(chunk2_id, set())

                    # Create relationships between entities in nearby chunks
                    for entity1_id in entities1:
                        for entity2_id in entities2:
                            if entity1_id == entity2_id:
                                continue

                            # Get entity types to determine relationship type first
                            entity1 = entities_collection.find_one({"entity_id": entity1_id})
                            entity2 = entities_collection.find_one({"entity_id": entity2_id})

                            if not entity1 or not entity2:
                                continue

                            # Create cross-chunk relationship based on entity types
                            predicate = self._determine_cross_chunk_predicate(entity1, entity2)

                            if not predicate:
                                continue

                            # Check if relationship already exists (include predicate to allow multiple predicates per pair)
                            existing = relations_collection.find_one(
                                {
                                    "subject_id": entity1_id,
                                    "object_id": entity2_id,
                                    "predicate": predicate,
                                }
                            )

                            if existing:
                                skipped_count += 1
                                continue

                            relationship_id = ResolvedRelationship.generate_relationship_id(
                                entity1_id, entity2_id, predicate
                            )

                            # Calculate chunk distance for confidence adjustment
                            chunk_distance = j - i
                            # Confidence decreases with distance (0.6 for adjacent, 0.4 for window edge)
                            confidence = max(0.4, 0.6 - (chunk_distance - 1) * 0.05)

                            relationship_doc = {
                                "relationship_id": relationship_id,
                                "subject_id": entity1_id,
                                "object_id": entity2_id,
                                "predicate": predicate,
                                "description": f"Entities mentioned in nearby chunks (distance: {chunk_distance})",
                                "confidence": confidence,
                                "source_count": 1,
                                "source_chunks": [chunk1_id, chunk2_id],
                                "video_id": video_id,
                                "chunk_distance": chunk_distance,
                                "created_at": time.time(),
                                "updated_at": time.time(),
                                "relationship_type": "cross_chunk",
                                # Attribution (Achievement 3.2)
                                **self._build_attribution(
                                    stage_name=self.name,
                                    algorithm="cross_chunk",
                                    params={
                                        "chunk_window": (
                                            chunk_window_override
                                            if chunk_window_override
                                            else "adaptive"
                                        ),
                                        "chunk_distance": chunk_distance,
                                        "video_id": video_id,
                                    },
                                ),
                            }

                            relationships_to_insert.append(relationship_doc)

        # Batch insert all cross-chunk relationships
        added_count = 0
        if relationships_to_insert:
            logger.info(
                f"Inserting {len(relationships_to_insert)} cross-chunk relationships in batch"
            )
            try:
                result = batch_insert(
                    collection=relations_collection,
                    documents=relationships_to_insert,
                    batch_size=500,
                    ordered=False,
                )
                added_count = result["inserted"]
                logger.info(
                    f"Cross-chunk batch insert: {result['inserted']}/{result['total']} successful, "
                    f"{result['failed']} failed"
                )
            except DuplicateKeyError:
                # Expected on reruns (idempotency) - unique index on relationship_id prevents duplicates
                logger.debug("Duplicate key error in cross-chunk batch insert (expected on reruns)")
                # Count successful inserts by checking which ones exist
                for rel_doc in relationships_to_insert:
                    existing = relations_collection.find_one(
                        {"relationship_id": rel_doc["relationship_id"]}
                    )
                    if existing:
                        added_count += 1
                logger.info(
                    f"Cross-chunk batch insert: {added_count}/{len(relationships_to_insert)} relationships already exist (idempotent)"
                )

        window_mode = "adaptive" if use_adaptive_window else f"override={chunk_window_override}"
        logger.info(
            f"Cross-chunk post-processing complete: "
            f"added {added_count} relationships, skipped {skipped_count} existing "
            f"(window mode: {window_mode})"
        )

        # Update metrics (Achievement 3.3)
        self.post_processing_stats["cross_chunk"]["added"] += added_count
        self.post_processing_stats["cross_chunk"]["skipped"] += skipped_count

        return added_count

    def _determine_cross_chunk_predicate(self, entity1: Dict, entity2: Dict) -> Optional[str]:
        """
        Determine appropriate predicate for cross-chunk relationship based on entity types.

        Args:
            entity1: First entity document
            entity2: Second entity document

        Returns:
            Predicate string or None if no relationship should be created
        """
        type1 = entity1.get("type", "OTHER")
        type2 = entity2.get("type", "OTHER")

        # Define type-based relationship patterns
        type_patterns = {
            ("PERSON", "CONCEPT"): "discusses",
            ("PERSON", "TECHNOLOGY"): "uses",
            ("PERSON", "ORGANIZATION"): "affiliated_with",
            ("CONCEPT", "CONCEPT"): "related_to",
            ("CONCEPT", "TECHNOLOGY"): "implemented_in",
            ("TECHNOLOGY", "TECHNOLOGY"): "works_with",
            ("ORGANIZATION", "TECHNOLOGY"): "develops",
        }

        # Try both directions
        predicate = type_patterns.get((type1, type2))
        if not predicate:
            predicate = type_patterns.get((type2, type1))

        return predicate if predicate else "mentioned_together"

    def _add_bidirectional_relationships(self) -> int:
        """
        Create reverse relationships for asymmetric relationships to make graph more navigable.

        This creates bidirectional edges for relationships like:
        - "Algorithm uses Data Structure" → "Data Structure used_by Algorithm"
        - "Person teaches Concept" → "Concept taught_by Person"

        Returns:
            Number of reverse relationships added
        """
        logger.info("Starting bidirectional relationship post-processing")

        relations_collection = self.graphrag_collections["relations"]

        # Collect all reverse relationships to insert in batch
        relationships_to_insert = []
        skipped_count = 0

        for relationship in relations_collection.find():
            predicate = relationship.get("predicate", "")

            # Get reverse predicate from ontology (Achievement 3.1)
            reverse_predicate = self._get_reverse_predicate(predicate)

            if reverse_predicate is None:
                continue  # Symmetric or no reverse found
            subject_id = relationship["subject_id"]
            object_id = relationship["object_id"]

            # Generate reverse relationship_id
            reverse_relationship_id = ResolvedRelationship.generate_relationship_id(
                object_id, subject_id, reverse_predicate
            )

            # Check if reverse relationship already exists by relationship_id (Achievement 1.2)
            existing_reverse = relations_collection.find_one(
                {"relationship_id": reverse_relationship_id}
            )

            if existing_reverse:
                # Merge existing reverse relationship (Achievement 1.2)
                # Merge policy: max confidence, union source_chunks, longest description
                now = time.time()
                original_desc = relationship.get("description", "")
                reverse_desc = existing_reverse.get("description", "")
                new_desc = (
                    reverse_desc
                    if len(reverse_desc) > len(original_desc)
                    else f"Reverse of: {original_desc}"
                )

                # Build merge update
                update = {
                    "$set": {
                        "description": new_desc,
                        "updated_at": now,
                    },
                    "$max": {
                        "confidence": max(
                            relationship.get("confidence", 0.7),
                            existing_reverse.get("confidence", 0.7),
                        )
                    },
                    "$addToSet": {
                        "source_chunks": {"$each": relationship.get("source_chunks", [])}
                    },
                }

                # Atomic upsert to merge
                relations_collection.find_one_and_update(
                    {"relationship_id": reverse_relationship_id},
                    update,
                    upsert=False,  # Should exist, but safe
                    return_document=ReturnDocument.AFTER,
                )
                skipped_count += 1  # Count as skipped (merged, not inserted)
                continue

            # Create new reverse relationship
            reverse_relationship_doc = {
                "relationship_id": reverse_relationship_id,
                "subject_id": object_id,
                "object_id": subject_id,
                "predicate": reverse_predicate,
                "description": f"Reverse of: {relationship.get('description', '')}",
                "confidence": relationship.get("confidence", 0.7),
                "source_count": relationship.get("source_count", 1),
                "source_chunks": relationship.get("source_chunks", []),
                "created_at": time.time(),
                "updated_at": time.time(),
                "relationship_type": "bidirectional",
                "original_relationship_id": relationship["relationship_id"],
                # Attribution (Achievement 3.2)
                **self._build_attribution(
                    stage_name=self.name,
                    algorithm="bidirectional",
                    params={
                        "original_predicate": predicate,
                        "reverse_predicate": reverse_predicate,
                    },
                ),
            }

            relationships_to_insert.append(reverse_relationship_doc)

        # Batch insert all reverse relationships
        added_count = 0
        if relationships_to_insert:
            logger.info(f"Inserting {len(relationships_to_insert)} reverse relationships in batch")
            try:
                result = batch_insert(
                    collection=relations_collection,
                    documents=relationships_to_insert,
                    batch_size=500,
                    ordered=False,
                )
                added_count = result["inserted"]
                logger.info(
                    f"Bidirectional batch insert: {result['inserted']}/{result['total']} successful, "
                    f"{result['failed']} failed"
                )
            except DuplicateKeyError:
                # Expected on reruns (idempotency) - unique index on relationship_id prevents duplicates
                logger.debug(
                    "Duplicate key error in bidirectional batch insert (expected on reruns)"
                )
                # Count successful inserts by checking which ones exist
                for rel_doc in relationships_to_insert:
                    existing = relations_collection.find_one(
                        {"relationship_id": rel_doc["relationship_id"]}
                    )
                    if existing:
                        added_count += 1
                logger.info(
                    f"Bidirectional batch insert: {added_count}/{len(relationships_to_insert)} relationships already exist (idempotent)"
                )

        logger.info(
            f"Bidirectional relationship post-processing complete: "
            f"added {added_count} relationships, skipped {skipped_count} existing"
        )

        # Update metrics (Achievement 3.3)
        self.post_processing_stats["bidirectional"]["added"] += added_count
        self.post_processing_stats["bidirectional"]["skipped"] += skipped_count

        return added_count

    def _calculate_current_graph_density(self) -> float:
        """
        Calculate current graph density to prevent over-connection.

        Density is calculated as: unique_unordered_pairs / max_possible_pairs

        We count unique unordered pairs (subject_id, object_id) rather than total
        relationships because:
        - Graph can have multiple predicates per pair (e.g., "teaches" and "mentors")
        - We want to measure connectivity, not relationship count
        - Undirected denominator n*(n-1)/2 is correct for pair-based density

        Returns:
            Graph density (0.0 to 1.0)
        """
        entities_collection = self.graphrag_collections["entities"]
        relations_collection = self.graphrag_collections["relations"]

        num_entities = entities_collection.count_documents({})

        if num_entities < 2:
            return 0.0

        # Count unique unordered pairs (subject_id, object_id)
        # Use aggregation to normalize pairs: min(s,o) | max(s,o)
        pair_pipeline = [
            {
                "$project": {
                    "pair_key": {
                        "$concat": [
                            {
                                "$cond": [
                                    {"$lt": ["$subject_id", "$object_id"]},
                                    "$subject_id",
                                    "$object_id",
                                ]
                            },
                            "|",
                            {
                                "$cond": [
                                    {"$gt": ["$subject_id", "$object_id"]},
                                    "$subject_id",
                                    "$object_id",
                                ]
                            },
                        ]
                    }
                }
            },
            {"$group": {"_id": "$pair_key"}},
            {"$count": "unique_pairs"},
        ]

        pair_result = list(relations_collection.aggregate(pair_pipeline))
        unique_pairs = pair_result[0]["count"] if pair_result else 0

        # Maximum possible unordered pairs in undirected graph
        max_possible = num_entities * (num_entities - 1) / 2

        # Current density: unique pairs / max possible pairs
        density = unique_pairs / max_possible if max_possible > 0 else 0.0

        return density

    def _add_predicted_relationships(self) -> int:
        """
        Add predicted relationships using graph link prediction.

        Returns:
            Number of predicted relationships added
        """
        logger.info("Starting link prediction post-processing")

        from src.domain.agents.graphrag.link_prediction import GraphLinkPredictionAgent

        entities_collection = self.graphrag_collections["entities"]
        relations_collection = self.graphrag_collections["relations"]

        # Get all entities and relationships
        entities = list(entities_collection.find())
        relationships = list(relations_collection.find())

        # Initialize link prediction agent
        link_predictor = GraphLinkPredictionAgent(
            confidence_threshold=float(os.getenv("GRAPHRAG_LINK_PREDICTION_THRESHOLD", "0.65")),
            max_predictions_per_entity=int(os.getenv("GRAPHRAG_MAX_PREDICTIONS_PER_ENTITY", "5")),
            use_structural_features=True,
        )

        # Predict missing links
        predictions = link_predictor.predict_missing_links(entities, relationships)

        logger.info(f"Got {len(predictions)} link predictions")

        # Collect all predicted relationships to insert in batch
        relationships_to_insert = []

        for subject_id, object_id, predicate, confidence in predictions:
            # Check if relationship already exists
            existing = relations_collection.find_one(
                {
                    "$or": [
                        {"subject_id": subject_id, "object_id": object_id},
                        {"subject_id": object_id, "object_id": subject_id},
                    ]
                }
            )

            if existing:
                continue

            # Create predicted relationship
            relationship_id = ResolvedRelationship.generate_relationship_id(
                subject_id, object_id, predicate
            )

            relationship_doc = {
                "relationship_id": relationship_id,
                "subject_id": subject_id,
                "object_id": object_id,
                "predicate": predicate,
                "description": f"Predicted relationship (confidence: {confidence:.3f})",
                "confidence": float(confidence),
                "source_count": 0,  # No direct source
                "source_chunks": [],
                "created_at": time.time(),
                "updated_at": time.time(),
                "relationship_type": "predicted",
                "prediction_confidence": float(confidence),
                # Attribution (Achievement 3.2)
                **self._build_attribution(
                    stage_name=self.name,
                    algorithm="link_prediction",
                    params={
                        "confidence_threshold": float(
                            os.getenv("GRAPHRAG_LINK_PREDICTION_THRESHOLD", "0.65")
                        ),
                        "max_predictions_per_entity": int(
                            os.getenv("GRAPHRAG_MAX_PREDICTIONS_PER_ENTITY", "5")
                        ),
                        "predicted_confidence": float(confidence),
                    },
                ),
            }

            relationships_to_insert.append(relationship_doc)

        # Batch insert all predicted relationships
        added_count = 0
        if relationships_to_insert:
            logger.info(
                f"Inserting {len(relationships_to_insert)} predicted relationships in batch"
            )
            try:
                result = batch_insert(
                    collection=relations_collection,
                    documents=relationships_to_insert,
                    batch_size=500,
                    ordered=False,
                )
                added_count = result["inserted"]
                logger.info(
                    f"Link prediction batch insert: {result['inserted']}/{result['total']} successful, "
                    f"{result['failed']} failed"
                )
            except DuplicateKeyError:
                # Expected on reruns (idempotency) - unique index on relationship_id prevents duplicates
                logger.debug(
                    "Duplicate key error in predicted relationships batch insert (expected on reruns)"
                )
                # Count successful inserts by checking which ones exist
                for rel_doc in relationships_to_insert:
                    existing = relations_collection.find_one(
                        {"relationship_id": rel_doc["relationship_id"]}
                    )
                    if existing:
                        added_count += 1
                logger.info(
                    f"Link prediction batch insert: {added_count}/{len(relationships_to_insert)} relationships already exist (idempotent)"
                )

        logger.info(f"Link prediction post-processing complete: added {added_count} relationships")

        # Update metrics (Achievement 3.3)
        self.post_processing_stats["predicted"]["added"] += added_count

        return added_count

    def _mark_construction_failed(self, doc: Dict[str, Any], error_message: str) -> bool:
        """
        Mark construction as failed for a document.

        Args:
            doc: Document to mark as failed
            error_message: Error message describing the failure
        """
        chunk_id = doc.get("chunk_id", "unknown")
        video_id = doc.get("video_id", "unknown")

        construction_payload = {
            "graphrag_construction": {
                "status": "failed",
                "error": error_message,
                "processed_at": time.time(),
            }
        }

        # Write failure status to database
        dst_db = self.config.write_db_name or self.config.db_name
        dst_coll_name = self.config.write_coll or COLL_CHUNKS
        collection = self.get_collection(dst_coll_name, io="write", db_name=dst_db)

        collection.update_one(
            {"video_id": video_id, "chunk_id": chunk_id},
            {"$set": construction_payload},
            upsert=False,
        )

        self.stats["failed"] += 1
        logger.warning(f"Marked chunk {chunk_id} construction as failed: {error_message}")

        return False  # Failure (Achievement 0.3)

    @handle_errors(fallback=[], log_traceback=True, reraise=False)
    def process_batch(self, docs: List[Dict[str, Any]]) -> List[Optional[Dict[str, Any]]]:
        """
        Process a batch of documents for graph construction.

        Args:
            docs: List of documents to process

        Returns:
            List of processed documents (or None for failed processing)
        """
        logger.info(f"Processing batch of {len(docs)} chunks for graph construction")

        results = []
        for i, doc in enumerate(docs):
            logger.debug(f"Processing document {i + 1}/{len(docs)}")

            try:
                result = self.handle_doc(doc)
                results.append(result)

                # Add small delay to avoid rate limiting
                if i < len(docs) - 1:
                    time.sleep(0.1)

            except Exception as e:
                logger.error(f"Error processing document {i + 1}: {e}")
                results.append(None)

        successful_count = sum(1 for r in results if r is True)
        logger.info(f"Batch processing completed: {successful_count}/{len(docs)} successful")

        return results

    def calculate_graph_metrics(self) -> Dict[str, Any]:
        """
        Calculate basic graph metrics for the constructed graph.

        Returns:
            Dictionary containing graph metrics
        """
        entities_collection = self.graphrag_collections["entities"]
        relations_collection = self.graphrag_collections["relations"]

        # Basic counts
        total_entities = entities_collection.count_documents({})
        total_relationships = relations_collection.count_documents({})

        # Calculate entity degrees
        entity_degrees = {}

        # Count outgoing relationships
        outgoing_pipeline = [{"$group": {"_id": "$subject_id", "outgoing_count": {"$sum": 1}}}]
        outgoing_results = list(relations_collection.aggregate(outgoing_pipeline))

        # Count incoming relationships
        incoming_pipeline = [{"$group": {"_id": "$object_id", "incoming_count": {"$sum": 1}}}]
        incoming_results = list(relations_collection.aggregate(incoming_pipeline))

        # Combine degrees
        for result in outgoing_results:
            entity_id = result["_id"]
            entity_degrees[entity_id] = entity_degrees.get(entity_id, 0) + result["outgoing_count"]

        for result in incoming_results:
            entity_id = result["_id"]
            entity_degrees[entity_id] = entity_degrees.get(entity_id, 0) + result["incoming_count"]

        # Calculate centrality scores
        if entity_degrees:
            max_degree = max(entity_degrees.values())
            avg_degree = sum(entity_degrees.values()) / len(entity_degrees)

            # Update entities with centrality scores
            for entity_id, degree in entity_degrees.items():
                centrality_score = degree / max_degree if max_degree > 0 else 0
                entities_collection.update_one(
                    {"entity_id": entity_id},
                    {"$set": {"centrality_score": centrality_score, "degree": degree}},
                )
        else:
            max_degree = 0
            avg_degree = 0

        logger.info(
            f"Calculated graph metrics: {total_entities} entities, {total_relationships} relationships"
        )

        return {
            "total_entities": total_entities,
            "total_relationships": total_relationships,
            "max_degree": max_degree,
            "avg_degree": avg_degree,
            "entities_with_relationships": len(entity_degrees),
            "isolated_entities": total_entities - len(entity_degrees),
            "calculated_at": time.time(),
        }

    def get_construction_stats(self) -> Dict[str, Any]:
        """
        Get statistics about the construction stage.

        Returns:
            Dictionary containing construction statistics
        """
        src_db = self.config.read_db_name or self.config.db_name
        src_coll_name = self.config.read_coll or COLL_CHUNKS
        collection = self.get_collection(src_coll_name, io="read", db_name=src_db)
        relations_collection = self.graphrag_collections["relations"]

        # Count total chunks with resolution
        total_resolved = collection.count_documents({"graphrag_resolution.status": "completed"})

        # Count constructed chunks
        constructed_chunks = collection.count_documents(
            {"graphrag_construction.status": "completed"}
        )

        # Count failed chunks
        failed_chunks = collection.count_documents({"graphrag_construction.status": "failed"})

        # Count pending chunks
        pending_chunks = total_resolved - constructed_chunks - failed_chunks

        # Count total relationships
        total_relationships = relations_collection.count_documents({})

        return {
            "total_resolved_chunks": total_resolved,
            "constructed_chunks": constructed_chunks,
            "failed_chunks": failed_chunks,
            "pending_chunks": pending_chunks,
            "total_relationships": total_relationships,
            "completion_rate": (constructed_chunks / total_resolved if total_resolved > 0 else 0),
            "failure_rate": failed_chunks / total_resolved if total_resolved > 0 else 0,
        }

    def cleanup_failed_constructions(self) -> int:
        """
        Clean up failed construction records to allow retry.

        Returns:
            Number of failed constructions cleaned up
        """
        src_db = self.config.read_db_name or self.config.db_name
        src_coll_name = self.config.read_coll or COLL_CHUNKS
        collection = self.get_collection(src_coll_name, io="read", db_name=src_db)

        result = collection.update_many(
            {"graphrag_construction.status": "failed"},
            {"$unset": {"graphrag_construction": 1}},
        )

        logger.info(f"Cleaned up {result.modified_count} failed constructions")
        return result.modified_count

    # run() inherited from BaseStage - auto-detects concurrency and calls appropriate method
    # BaseStage.run() now handles:
    # - Concurrent + TPM tracking → _run_concurrent_with_tpm() [default]
    # - Concurrent only → _run_concurrent() if implemented
    # - Sequential → standard loop processing

    # _run_concurrent removed - BaseStage.run() handles concurrency automatically

    def estimate_tokens(self, doc: Dict[str, Any]) -> int:
        """Estimate tokens for graph construction (override base method)."""
        extraction_data = doc.get("graphrag_extraction", {}).get("data", {})
        relationships = extraction_data.get("relationships", [])
        # Each relationship ~300 tokens, output ~600 tokens
        estimated = len(relationships) * 300 + 600
        return max(estimated, 100)

    # process_doc_with_tracking uses default (calls handle_doc)
    # store_batch_results uses default (no-op, handle_doc writes directly)

    def finalize(self) -> None:
        """
        Finalize the graph construction stage with comprehensive post-processing.

        This method runs after all documents are processed and performs
        comprehensive post-processing to improve graph connectivity and quality.
        """
        logger.info("=" * 80)
        logger.info("Starting comprehensive graph post-processing")
        logger.info("=" * 80)

        total_added = 0
        max_density = float(os.getenv("GRAPHRAG_MAX_DENSITY", "0.3"))

        # 1. Co-occurrence relationships
        logger.info("[1/5] Adding co-occurrence relationships...")
        try:
            count = self._add_co_occurrence_relationships()
            total_added += count
            current_density = self._calculate_current_graph_density()
            logger.info(
                f"✓ Added {count} co-occurrence relationships " f"(density: {current_density:.4f})"
            )
        except Exception as e:
            logger.error(f"✗ Failed to add co-occurrence relationships: {e}")

        # Check density before continuing
        current_density = self._calculate_current_graph_density()
        if current_density >= max_density:
            logger.warning(
                f"Graph density ({current_density:.4f}) reached maximum ({max_density}). "
                f"Skipping remaining post-processing to prevent over-connection."
            )
            logger.info("=" * 80)
            logger.info(f"Graph post-processing stopped early: added {total_added} relationships")
            logger.info("=" * 80)
            super().finalize()
            return

        # 2. Semantic similarity relationships
        logger.info("[2/5] Adding semantic similarity relationships...")
        try:
            similarity_threshold = float(os.getenv("GRAPHRAG_SIMILARITY_THRESHOLD", "0.92"))
            count = self._add_semantic_similarity_relationships(similarity_threshold)
            total_added += count
            current_density = self._calculate_current_graph_density()
            logger.info(
                f"✓ Added {count} semantic similarity relationships "
                f"(threshold: {similarity_threshold}, density: {current_density:.4f})"
            )
        except Exception as e:
            logger.error(f"✗ Failed to add semantic similarity relationships: {e}")

        # Check density before continuing
        current_density = self._calculate_current_graph_density()
        if current_density >= max_density:
            logger.warning(
                f"Graph density ({current_density:.4f}) reached maximum ({max_density}). "
                f"Skipping remaining post-processing to prevent over-connection."
            )
            logger.info("=" * 80)
            logger.info(f"Graph post-processing stopped early: added {total_added} relationships")
            logger.info("=" * 80)
            super().finalize()
            return

        # 3. Cross-chunk relationships
        logger.info("[3/5] Adding cross-chunk relationships...")
        try:
            count = self._add_cross_chunk_relationships()
            total_added += count
            current_density = self._calculate_current_graph_density()
            logger.info(
                f"✓ Added {count} cross-chunk relationships " f"(density: {current_density:.4f})"
            )
        except Exception as e:
            logger.error(f"✗ Failed to add cross-chunk relationships: {e}")

        # Check density before bidirectional
        current_density = self._calculate_current_graph_density()
        if current_density >= max_density:
            logger.warning(
                f"Graph density ({current_density:.4f}) reached maximum ({max_density}). "
                f"Skipping bidirectional and link prediction."
            )
            logger.info("=" * 80)
            logger.info(f"Graph post-processing stopped early: added {total_added} relationships")
            logger.info("=" * 80)
            super().finalize()
            return

        # 4. Bidirectional relationships
        logger.info("[4/5] Adding bidirectional relationships...")
        try:
            count = self._add_bidirectional_relationships()
            total_added += count
            current_density = self._calculate_current_graph_density()
            logger.info(
                f"✓ Added {count} bidirectional relationships " f"(density: {current_density:.4f})"
            )
        except Exception as e:
            logger.error(f"✗ Failed to add bidirectional relationships: {e}")

        # Check density before link prediction
        current_density = self._calculate_current_graph_density()
        if current_density >= max_density:
            logger.warning(
                f"Graph density ({current_density:.4f}) reached maximum ({max_density}). "
                f"Skipping link prediction."
            )
            logger.info("=" * 80)
            logger.info(f"Graph post-processing complete: added {total_added} relationships")
            logger.info("=" * 80)
            # Log comprehensive metrics (Achievement 3.3)
            self._log_comprehensive_metrics()
            super().finalize()
            return

        # 5. Link prediction (optional, can be disabled)
        if os.getenv("GRAPHRAG_ENABLE_LINK_PREDICTION", "true").lower() == "true":
            logger.info("[5/5] Adding predicted relationships...")
            try:
                count = self._add_predicted_relationships()
                total_added += count
                current_density = self._calculate_current_graph_density()
                logger.info(
                    f"✓ Added {count} predicted relationships " f"(density: {current_density:.4f})"
                )
            except Exception as e:
                logger.error(f"✗ Failed to add predicted relationships: {e}")
        else:
            logger.info("[5/5] Link prediction disabled (GRAPHRAG_ENABLE_LINK_PREDICTION=false)")

        logger.info("=" * 80)
        logger.info(f"Graph post-processing complete: added {total_added} total relationships")
        logger.info("=" * 80)

        # Log comprehensive metrics (Achievement 3.3)
        self._log_comprehensive_metrics()

        # Call parent finalize to log statistics
        super().finalize()
