"""
Entity Resolution Stage

This stage resolves and canonicalizes entities extracted from chunks.
It groups similar entities and stores resolved entities in the entities collection.
"""

import logging
import os
import time
from typing import Dict, List, Any, Optional, Iterator
from src.core.base.stage import BaseStage
from src.core.config.graphrag import EntityResolutionConfig
from src.domain.agents.graphrag.entity_resolution import EntityResolutionAgent
from src.core.models.graphrag import ResolvedEntity
from src.domain.services.graphrag.indexes import get_graphrag_collections
from src.domain.services.graphrag.transformation_logger import TransformationLogger
from src.domain.services.graphrag.intermediate_data import IntermediateDataService
from src.core.config.paths import COLL_CHUNKS
from src.lib.database import batch_insert
from src.lib.rate_limiting import RateLimiter
from src.lib.error_handling.decorators import handle_errors
from pymongo import ReturnDocument
from pymongo.errors import DuplicateKeyError

logger = logging.getLogger(__name__)


class EntityResolutionStage(BaseStage):
    """
    Stage for resolving and canonicalizing entities across chunks.
    """

    name = "entity_resolution"
    description = "Resolve and canonicalize entities across chunks"
    ConfigCls = EntityResolutionConfig

    def __init__(self):
        """Initialize the Entity Resolution Stage."""
        super().__init__()
        # Don't initialize agent here - will be done in setup()

    def setup(self):
        """Setup the stage with config-dependent initialization."""
        super().setup()

        # Initialize OpenAI client for LLM operations
        from src.lib.llm import get_openai_client

        self.llm_client = get_openai_client(timeout=60)

        # Initialize the resolution agent now that we have access to self.config
        self.resolution_agent = EntityResolutionAgent(
            llm_client=self.llm_client,
            model_name=self.config.model_name,
            temperature=self.config.temperature,
            similarity_threshold=self.config.similarity_threshold,
            max_input_tokens=self.config.max_input_tokens_per_entity,
        )

        # Get GraphRAG collections (use write DB for output)
        self.graphrag_collections = get_graphrag_collections(self.db_write)

        # Initialize TransformationLogger for logging entity operations (Achievement 0.1)
        # Check if logging is enabled via environment variable
        import os

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

        logger.info(
            f"Initialized {self.name} with similarity threshold {self.config.similarity_threshold}"
        )

    def iter_docs(self) -> Iterator[Dict[str, Any]]:
        """
        Iterate over chunks that have completed entity extraction.

        Yields:
            Chunk documents that have been processed for entity extraction
        """
        src_db = self.config.read_db_name or self.config.db_name
        src_coll_name = self.config.read_coll or COLL_CHUNKS
        collection = self.get_collection(src_coll_name, io="read", db_name=src_db)

        # Query for chunks that have completed extraction but not resolution
        query = {
            "graphrag_extraction.status": "completed",
            "$or": [
                {"graphrag_resolution": {"$exists": False}},
                {"graphrag_resolution.status": {"$ne": "completed"}},
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

        logger.info(f"Querying chunks for entity resolution: {query}")

        cursor = collection.find(query)
        if self.config.max:
            cursor = cursor.limit(int(self.config.max))

        for doc in cursor:
            yield doc

    @handle_errors(fallback=None, log_traceback=True, reraise=False)
    def handle_doc(self, doc: Dict[str, Any]) -> Optional[Dict[str, Any]]:
        """
        Process a single chunk for entity resolution and write to database.

        Args:
            doc: Chunk document to process

        Returns:
            None (writes directly to database via update_one)
        """
        chunk_id = doc.get("chunk_id", "unknown")
        video_id = doc.get("video_id", "unknown")

        logger.debug(f"Processing chunk {chunk_id} from video {video_id} for entity resolution")

        try:
            # Extract entity data from the chunk
            extraction_data = doc.get("graphrag_extraction", {}).get("data", {})

            # Get trace_id for linking
            trace_id = getattr(self.config, "trace_id", None) or "unknown"

            # Achievement 0.2: Save raw entities (before resolution)
            if extraction_data and "entities" in extraction_data:
                raw_entities = extraction_data.get("entities", [])
                self.intermediate_data.save_entities_raw(
                    entities=raw_entities,
                    chunk_id=chunk_id,
                    video_id=video_id,
                    trace_id=trace_id,
                    extraction_method="llm",
                )

            if not extraction_data or "entities" not in extraction_data:
                logger.warning(f"No entity extraction data found in chunk {chunk_id}")
                # Log skip transformation (Achievement 0.1)
                trace_id = getattr(self.config, "trace_id", None) or "unknown"
                self.transformation_logger.log_entity_skip(
                    entity={"id": chunk_id, "name": f"chunk_{chunk_id}"},
                    reason="no_extraction_data",
                    confidence=0.0,
                    trace_id=trace_id,
                )
                return self._mark_resolution_failed(doc, "no_extraction_data")

            # Resolve entities for this chunk
            resolved_entities = self.resolution_agent.resolve_entities([extraction_data])

            if not resolved_entities:
                logger.warning(f"No entities resolved for chunk {chunk_id}")
                # Log skip transformation (Achievement 0.1)
                trace_id = getattr(self.config, "trace_id", None) or "unknown"
                self.transformation_logger.log_entity_skip(
                    entity={"id": chunk_id, "name": f"chunk_{chunk_id}"},
                    reason="no_entities_resolved",
                    confidence=0.0,
                    trace_id=trace_id,
                )
                return self._mark_resolution_failed(doc, "no_entities_resolved")

            # Store resolved entities in the entities collection
            # Returns id_map: {original_id → final_id} for correct mention mapping
            id_map = self._store_resolved_entities(resolved_entities, chunk_id, video_id)

            # Achievement 0.2: Save resolved entities (after resolution)
            resolved_entities_data = []
            for entity in resolved_entities:
                resolved_entities_data.append(
                    {
                        "entity_id": id_map.get(entity.entity_id, entity.entity_id),
                        "canonical_name": entity.canonical_name,
                        "type": (
                            entity.type.value if hasattr(entity.type, "value") else str(entity.type)
                        ),
                        "aliases": entity.aliases,
                        "confidence": entity.confidence,
                        "source_count": 1,
                    }
                )
            self.intermediate_data.save_entities_resolved(
                entities=resolved_entities_data,
                chunk_id=chunk_id,
                video_id=video_id,
                trace_id=trace_id,
                resolution_method="fuzzy_match",
            )

            # Store entity mentions using id_map to ensure correct entity_id
            self._store_entity_mentions(resolved_entities, chunk_id, video_id, id_map)

            # Prepare resolution payload
            resolution_payload = {
                "graphrag_resolution": {
                    "status": "completed",
                    "resolved_entities": len(resolved_entities),
                    "stored_entities": len(id_map),  # Count of entities stored (id_map keys)
                    "processed_at": time.time(),
                    "model_used": self.config.model_name,
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
                    {"graphrag_resolution.status": 1},
                )
                if (
                    existing
                    and existing.get("graphrag_resolution", {}).get("status") == "completed"
                ):
                    logger.debug(f"Skipping chunk {chunk_id} - already has completed resolution")
                    self.stats["skipped"] += 1
                    return None

            # Update the chunk with resolution results
            collection.update_one(
                {"video_id": video_id, "chunk_id": chunk_id},
                {"$set": resolution_payload},
                upsert=False,
            )

            self.stats["updated"] += 1
            logger.debug(
                f"Successfully resolved {len(resolved_entities)} entities " f"for chunk {chunk_id}"
            )

            return None

        except Exception as e:
            logger.error(f"Error processing chunk {chunk_id} for entity resolution: {e}")
            return self._mark_resolution_failed(doc, str(e))

    def _find_db_candidates(
        self, name: str, entity_type: str, aliases: List[str]
    ) -> List[Dict[str, Any]]:
        """
        Find candidate entities in the database that might match the given entity.

        Uses blocking keys to efficiently search for potential matches without
        scanning all entities. This enables cross-chunk entity resolution.

        Args:
            name: Entity canonical name
            entity_type: Entity type (e.g., "PERSON", "TECHNOLOGY")
            aliases: List of entity aliases

        Returns:
            List of candidate entity documents from database
        """
        entities_collection = self.graphrag_collections["entities"]

        # Generate blocking keys for the canonical name
        blocking_keys = self.resolution_agent._blocking_keys(name)

        # Also generate blocking keys for aliases
        for alias in aliases:
            alias_keys = self.resolution_agent._blocking_keys(alias)
            blocking_keys.extend(alias_keys)

        # Remove duplicates
        blocking_keys = list(set(blocking_keys))

        if not blocking_keys:
            return []

        # Normalize names and aliases for query
        normalized_name = self.resolution_agent._normalize_entity_name(name)
        normalized_aliases = [
            self.resolution_agent._normalize_entity_name(alias) for alias in aliases
        ]

        # Query for candidates using normalized fields
        # For now, check both canonical_name_normalized and aliases_normalized
        # If these fields don't exist yet (backward compatibility), we'll need to
        # normalize on the fly in _choose_match
        query = {
            "$or": [
                # Try normalized fields first (if they exist)
                {"canonical_name_normalized": {"$in": blocking_keys}},
                {"aliases_normalized": {"$in": blocking_keys}},
            ]
        }

        # Optionally filter by type (can be relaxed later)
        # For now, include type in query to avoid false matches
        # query["type"] = entity_type  # Commented out to allow cross-type discovery

        try:
            candidates = list(entities_collection.find(query).limit(20))
            logger.debug(
                f"Found {len(candidates)} candidate(s) for entity '{name}' "
                f"using {len(blocking_keys)} blocking keys"
            )
            return candidates
        except Exception as e:
            logger.warning(f"Error finding candidates for '{name}': {e}")
            return []

    def _choose_match(
        self, name: str, candidates: List[Dict[str, Any]]
    ) -> Optional[Dict[str, Any]]:
        """
        Choose the best matching candidate from a list of candidates.

        Uses fuzzy string matching with similarity threshold to find near-duplicates.
        Returns the best match if similarity >= threshold, otherwise None.

        Args:
            name: Entity name to match
            candidates: List of candidate entity documents

        Returns:
            Best matching candidate or None if no match found above threshold
        """
        if not candidates:
            return None

        threshold = self.resolution_agent.similarity_threshold
        best_match = None
        best_score = 0.0

        # First, try exact normalized match (fast path)
        normalized_name = self.resolution_agent._normalize_entity_name(name)
        for candidate in candidates:
            # Check normalized canonical name
            if "canonical_name_normalized" in candidate:
                if candidate["canonical_name_normalized"] == normalized_name:
                    return candidate  # Exact match, return immediately
            else:
                # Fallback: normalize on the fly
                candidate_normalized = self.resolution_agent._normalize_entity_name(
                    candidate.get("canonical_name", "")
                )
                if candidate_normalized == normalized_name:
                    return candidate  # Exact match, return immediately

            # Check normalized aliases
            aliases_to_check = candidate.get("aliases_normalized", [])
            if not aliases_to_check:
                # Fallback: normalize aliases on the fly
                aliases_to_check = [
                    self.resolution_agent._normalize_entity_name(alias)
                    for alias in candidate.get("aliases", [])
                ]

            if normalized_name in aliases_to_check:
                return candidate  # Exact match, return immediately

        # No exact match found, try fuzzy matching with multi-strategy scoring
        for candidate in candidates:
            # Score against canonical name using multi-strategy
            candidate_name = candidate.get("canonical_name", "")
            if candidate_name:
                score = self.resolution_agent._multi_strategy_score(name, candidate_name)
                if score > best_score:
                    best_score = score
                    best_match = candidate

            # Score against aliases (take best alias score)
            aliases = candidate.get("aliases", [])
            for alias in aliases:
                if alias:
                    score = self.resolution_agent._multi_strategy_score(name, alias)
                    if score > best_score:
                        best_score = score
                        best_match = candidate

        # Return best match only if it meets threshold
        if best_score >= threshold and best_match:
            # Check type compatibility if we have entity type information
            # Note: This is a basic check - full type checking would need entity type from extraction
            logger.debug(
                f"Fuzzy match found: '{name}' matches '{best_match.get('canonical_name', 'unknown')}' "
                f"with score {best_score:.3f} (threshold: {threshold:.3f})"
            )
            return best_match
        else:
            logger.debug(
                f"No fuzzy match above threshold: '{name}' best score {best_score:.3f} "
                f"(threshold: {threshold:.3f})"
            )
            return None

    def _store_resolved_entities(
        self, resolved_entities: List[ResolvedEntity], chunk_id: str, video_id: str
    ) -> Dict[str, str]:
        """
        Store resolved entities in the entities collection.

        Now includes cross-chunk candidate lookup to reuse existing entities.

        Args:
            resolved_entities: List of resolved entities
            chunk_id: Source chunk ID
            video_id: Source video ID

        Returns:
            Dictionary mapping original entity_id to final entity_id.
            When entities are merged via fuzzy matching, original_id maps to existing_entity_id.
            When entities are new, original_id maps to itself.
        """
        entities_collection = self.graphrag_collections["entities"]
        id_map: Dict[str, str] = {}

        for entity in resolved_entities:
            try:
                original_id = entity.entity_id

                # First, check if entity already exists by entity_id (existing logic)
                existing_entity = entities_collection.find_one({"entity_id": entity.entity_id})

                if existing_entity:
                    # Update existing entity using atomic upsert
                    self._upsert_entity(entity, chunk_id, video_id)

                    # Log entity update (same entity_id, no merge) (Achievement 0.1)
                    trace_id = getattr(self.config, "trace_id", None) or "unknown"
                    self.transformation_logger.log_entity_create(
                        entity={"id": original_id, "name": entity.canonical_name},
                        entity_type=entity.type.value,
                        sources=existing_entity.get("source_count", 1),
                        confidence=entity.confidence,
                        trace_id=trace_id,
                    )

                    id_map[original_id] = original_id  # No change, same ID
                else:
                    # NEW: Look for candidates across chunks before creating new entity
                    candidates = self._find_db_candidates(
                        name=entity.canonical_name,
                        entity_type=entity.type.value,
                        aliases=entity.aliases,
                    )

                    matched_candidate = self._choose_match(entity.canonical_name, candidates)

                    if matched_candidate:
                        # Reuse existing entity from another chunk
                        existing_entity_id = matched_candidate["entity_id"]
                        existing_canonical = matched_candidate.get("canonical_name", "")
                        existing_aliases = matched_candidate.get("aliases", [])

                        # Calculate similarity score for logging (Achievement 0.1)
                        similarity_score = self.resolution_agent._multi_strategy_score(
                            entity.canonical_name, existing_canonical
                        )

                        logger.debug(
                            f"Reusing existing entity {existing_entity_id} for "
                            f"'{entity.canonical_name}' (found via candidate lookup, "
                            f"existing: '{existing_canonical}')"
                        )

                        # Log entity merge transformation (Achievement 0.1)
                        trace_id = getattr(self.config, "trace_id", None) or "unknown"
                        self.transformation_logger.log_entity_merge(
                            entity_a={"id": original_id, "name": entity.canonical_name},
                            entity_b={"id": existing_entity_id, "name": existing_canonical},
                            result_entity={"id": existing_entity_id, "name": existing_canonical},
                            reason="fuzzy_match_above_threshold",
                            similarity=similarity_score,
                            confidence=entity.confidence,
                            trace_id=trace_id,
                        )

                        # Merge aliases: add new entity's name if different from existing
                        merged_aliases = list(existing_aliases) if existing_aliases else []

                        # Add new entity's canonical_name as alias if different
                        if entity.canonical_name and entity.canonical_name != existing_canonical:
                            if entity.canonical_name not in merged_aliases:
                                merged_aliases.append(entity.canonical_name)

                        # Add new entity's aliases
                        for alias in entity.aliases:
                            if (
                                alias
                                and alias not in merged_aliases
                                and alias != existing_canonical
                            ):
                                merged_aliases.append(alias)

                        # Keep existing canonical_name (more stable, already in DB)
                        # But update description if new one is better (higher confidence)
                        final_canonical = (
                            existing_canonical if existing_canonical else entity.canonical_name
                        )
                        final_description = (
                            entity.description
                        )  # Use new description (may be more recent/complete)

                        # Create a new ResolvedEntity with the matched entity_id and merged aliases
                        # This ensures entity_mentions reference the correct entity
                        merged_entity = ResolvedEntity(
                            entity_id=existing_entity_id,
                            canonical_name=final_canonical,
                            name=final_canonical,  # Use canonical as primary name
                            type=entity.type,
                            description=final_description,
                            confidence=entity.confidence,
                            source_count=entity.source_count,
                            resolution_methods=entity.resolution_methods,
                            aliases=merged_aliases,  # Merged aliases including new name
                        )

                        # Update the existing entity with new information (atomic upsert)
                        self._upsert_entity(
                            merged_entity,
                            chunk_id,
                            video_id,
                            existing_entity_id=existing_entity_id,
                        )
                        # Map original_id to existing_entity_id (fuzzy match merged)
                        id_map[original_id] = existing_entity_id
                    else:
                        # No candidate found, upsert new entity (atomic operation)
                        self._upsert_entity(entity, chunk_id, video_id)

                        # Log entity creation transformation (Achievement 0.1)
                        trace_id = getattr(self.config, "trace_id", None) or "unknown"
                        self.transformation_logger.log_entity_create(
                            entity={"id": original_id, "name": entity.canonical_name},
                            entity_type=entity.type.value,
                            sources=1,  # New entity from this chunk
                            confidence=entity.confidence,
                            trace_id=trace_id,
                        )

                        id_map[original_id] = original_id  # New entity, no change

            except Exception as e:
                logger.error(f"Failed to store entity {entity.entity_id}: {e}")
                # On error, still map entity_id to itself (no change)
                id_map[entity.entity_id] = entity.entity_id
                continue

        return id_map

    def _upsert_entity(
        self,
        resolved_entity: ResolvedEntity,
        chunk_id: str,
        video_id: str,
        existing_entity_id: Optional[str] = None,
    ) -> Dict[str, Any]:
        """
        Atomically upsert an entity using find_one_and_update.

        This replaces the separate _update_existing_entity and _insert_new_entity
        methods with a single atomic operation that eliminates race conditions.

        Uses proper merge policy:
        - $setOnInsert for immutable fields (created_at, first_seen, entity_id, type)
        - $set for updateable fields (description, updated_at, canonical_name, name)
        - $inc for counters (source_count)
        - $addToSet for arrays (aliases, aliases_normalized, source_chunks)
        - $max for confidence (keep highest)
        - $push with $slice for provenance (capped array, if implemented)

        Args:
            resolved_entity: Resolved entity to upsert
            chunk_id: Source chunk ID
            video_id: Source video ID
            existing_entity_id: Optional existing entity_id (if merging with existing)

        Returns:
            Updated entity document
        """
        entities_collection = self.graphrag_collections["entities"]
        now = time.time()

        # Use existing entity_id if provided (from candidate match), otherwise use resolved entity's ID
        entity_id = existing_entity_id or resolved_entity.entity_id

        # Generate normalized fields for efficient candidate lookup
        canonical_name_normalized = self.resolution_agent._normalize_entity_name(
            resolved_entity.canonical_name
        )
        aliases_normalized = [
            self.resolution_agent._normalize_entity_name(alias) for alias in resolved_entity.aliases
        ]

        # Check if entity exists and if chunk_id is already in source_chunks
        # This ensures source_count only increments for new chunks (Achievement 3.5.3)
        existing_entity = entities_collection.find_one({"entity_id": entity_id})
        is_new_chunk = True  # Default to True for new entities

        if existing_entity:
            source_chunks = existing_entity.get("source_chunks", [])
            is_new_chunk = chunk_id not in source_chunks
        else:
            # New entity, always increment source_count
            is_new_chunk = True

        # Build update document with proper merge policy
        update = {
            # $setOnInsert: Only set on insert (immutable fields)
            "$setOnInsert": {
                "entity_id": entity_id,
                "created_at": now,
                "first_seen": now,
                "type": resolved_entity.type.value,
                # NOTE: source_count NOT here - $inc handles it correctly for both insert and update
            },
            # $set: Always update (mutable fields)
            "$set": {
                "canonical_name": resolved_entity.canonical_name,
                "canonical_name_normalized": canonical_name_normalized,
                "name": resolved_entity.name,
                "updated_at": now,
                "last_seen": now,
                "description": resolved_entity.description,
            },
            # $addToSet: Add to arrays without duplicates
            "$addToSet": {
                "aliases": {"$each": resolved_entity.aliases},
                "aliases_normalized": {"$each": aliases_normalized},
                "source_chunks": chunk_id,
            },
            # $max: Keep highest confidence
            "$max": {"confidence": resolved_entity.confidence},
        }

        # Only increment source_count if this is a new chunk (Achievement 3.5.3)
        # This prevents source_count inflation on reruns
        if is_new_chunk:
            update["$inc"] = {"source_count": 1}

        # Add provenance entry if supported (for Achievement 2.3)
        # For now, we'll add a placeholder that can be enhanced later
        provenance_entry = {
            "video_id": video_id,
            "chunk_id": chunk_id,
            "method": ",".join(resolved_entity.resolution_methods),
            "at": now,
        }
        update["$push"] = {
            "provenance": {
                "$each": [provenance_entry],
                "$slice": -50,  # Cap at 50 entries
            }
        }

        # Perform atomic upsert
        result = entities_collection.find_one_and_update(
            {"entity_id": entity_id},
            update,
            upsert=True,
            return_document=ReturnDocument.AFTER,
        )

        return result

    def _store_entity_mentions(
        self,
        resolved_entities: List[ResolvedEntity],
        chunk_id: str,
        video_id: str,
        id_map: Optional[Dict[str, str]] = None,
    ) -> None:
        """
        Store entity mentions in the entity_mentions collection.

        Uses id_map to ensure mentions reference the correct entity_id when entities
        are merged via fuzzy matching.

        Args:
            resolved_entities: List of resolved entities
            chunk_id: Source chunk ID
            video_id: Source video ID
            id_map: Optional mapping from original entity_id to final entity_id.
                   If None, uses entity.entity_id directly (backward compatibility).
        """
        mentions_collection = self.graphrag_collections["entity_mentions"]
        id_map = id_map or {}  # Default to empty dict for backward compatibility

        mentions = []
        for entity in resolved_entities:
            # Use id_map to get final entity_id (handles fuzzy match merges)
            final_id = id_map.get(entity.entity_id, entity.entity_id)

            mention_doc = {
                "entity_id": final_id,  # Use final_id from id_map
                "chunk_id": chunk_id,
                "video_id": video_id,
                "confidence": entity.confidence,
                "position": 0,  # Could be enhanced to track position in chunk
                "created_at": time.time(),
            }
            mentions.append(mention_doc)

        if mentions:
            # Use batch_insert for better error handling and statistics
            # Duplicate mentions are handled by unique index (entity_id, chunk_id, position)
            # This ensures reruns are idempotent (Achievement 3.5.2)
            try:
                result = batch_insert(
                    collection=mentions_collection,
                    documents=mentions,
                    batch_size=1000,
                    ordered=False,  # Continue on errors (including duplicates)
                )
                logger.debug(
                    f"Inserted {result['inserted']}/{result['total']} entity mentions "
                    f"(chunk {chunk_id})"
                )
            except DuplicateKeyError as e:
                # Duplicate mentions are expected on reruns - unique index prevents them
                # Log but don't fail (idempotent operation)
                logger.debug(
                    f"Duplicate mentions detected (expected on reruns): {e}. "
                    f"Unique index prevented duplicates."
                )
                # batch_insert with ordered=False should handle duplicates gracefully,
                # but we catch here just in case

    def _mark_resolution_failed(self, doc: Dict[str, Any], error_message: str) -> None:
        """
        Mark resolution as failed for a document.

        Args:
            doc: Document to mark as failed
            error_message: Error message describing the failure
        """
        chunk_id = doc.get("chunk_id", "unknown")
        video_id = doc.get("video_id", "unknown")

        resolution_payload = {
            "graphrag_resolution": {
                "status": "failed",
                "error": error_message,
                "processed_at": time.time(),
                "model_used": self.config.model_name,
            }
        }

        # Write failure status to database
        dst_db = self.config.write_db_name or self.config.db_name
        dst_coll_name = self.config.write_coll or COLL_CHUNKS
        collection = self.get_collection(dst_coll_name, io="write", db_name=dst_db)

        collection.update_one(
            {"video_id": video_id, "chunk_id": chunk_id},
            {"$set": resolution_payload},
            upsert=False,
        )

        self.stats["failed"] += 1
        logger.warning(f"Marked chunk {chunk_id} resolution as failed: {error_message}")

        return None

    @handle_errors(fallback=[], log_traceback=True, reraise=False)
    def process_batch(self, docs: List[Dict[str, Any]]) -> List[Optional[Dict[str, Any]]]:
        """
        Process a batch of documents for entity resolution.

        Args:
            docs: List of documents to process

        Returns:
            List of processed documents (or None for failed processing)
        """
        logger.info(f"Processing batch of {len(docs)} chunks for entity resolution")

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

        successful_count = sum(1 for r in results if r is not None)
        logger.info(f"Batch processing completed: {successful_count}/{len(docs)} successful")

        return results

    def get_resolution_stats(self) -> Dict[str, Any]:
        """
        Get statistics about the resolution stage.

        Returns:
            Dictionary containing resolution statistics
        """
        src_db = self.config.read_db_name or self.config.db_name
        src_coll_name = self.config.read_coll or COLL_CHUNKS
        collection = self.get_collection(src_coll_name, io="read", db_name=src_db)
        entities_collection = self.graphrag_collections["entities"]

        # Count total chunks with extraction
        total_extracted = collection.count_documents({"graphrag_extraction.status": "completed"})

        # Count resolved chunks
        resolved_chunks = collection.count_documents({"graphrag_resolution.status": "completed"})

        # Count failed chunks
        failed_chunks = collection.count_documents({"graphrag_resolution.status": "failed"})

        # Count pending chunks
        pending_chunks = total_extracted - resolved_chunks - failed_chunks

        # Count total entities
        total_entities = entities_collection.count_documents({})

        # Count entity mentions
        mentions_collection = self.graphrag_collections["entity_mentions"]
        total_mentions = mentions_collection.count_documents({})

        return {
            "total_extracted_chunks": total_extracted,
            "resolved_chunks": resolved_chunks,
            "failed_chunks": failed_chunks,
            "pending_chunks": pending_chunks,
            "total_entities": total_entities,
            "total_mentions": total_mentions,
            "completion_rate": (resolved_chunks / total_extracted if total_extracted > 0 else 0),
            "failure_rate": (failed_chunks / total_extracted if total_extracted > 0 else 0),
        }

    def cleanup_failed_resolutions(self) -> int:
        """
        Clean up failed resolution records to allow retry.

        Returns:
            Number of failed resolutions cleaned up
        """
        src_db = self.config.read_db_name or self.config.db_name
        src_coll_name = self.config.read_coll or COLL_CHUNKS
        collection = self.get_collection(src_coll_name, io="read", db_name=src_db)

        result = collection.update_many(
            {"graphrag_resolution.status": "failed"},
            {"$unset": {"graphrag_resolution": 1}},
        )

        logger.info(f"Cleaned up {result.modified_count} failed resolutions")
        return result.modified_count

    # run() inherited from BaseStage - auto-detects concurrency and calls appropriate method
    # BaseStage.run() now handles:
    # - Concurrent + TPM tracking → _run_concurrent_with_tpm() [default]
    # - Concurrent only → _run_concurrent() if implemented
    # - Sequential → standard loop processing

    # _run_concurrent removed - BaseStage.run() handles concurrency automatically

    def estimate_tokens(self, doc: Dict[str, Any]) -> int:
        """Estimate tokens for entity resolution (override base method)."""
        extraction_data = doc.get("graphrag_extraction", {}).get("data", {})
        entities = extraction_data.get("entities", [])
        # Each entity description ~200 tokens, output ~500 tokens for resolution
        estimated = len(entities) * 200 + 500
        return max(estimated, 100)

    # process_doc_with_tracking uses default (calls handle_doc)
    # store_batch_results uses default (no-op, handle_doc writes directly)
