"""
Community Detection Stage

This stage detects communities in the knowledge graph and generates summaries
for each community using hierarchical Leiden algorithm and LLM-based summarization.
"""

import logging
import os
import time
from typing import Dict, List, Any, Optional, Iterator
from src.core.base.stage import BaseStage
from src.core.config.graphrag import CommunityDetectionConfig
from src.domain.agents.graphrag.community_detection import CommunityDetectionAgent
from src.domain.agents.graphrag.community_summarization import CommunitySummarizationAgent
from src.core.models.graphrag import ResolvedEntity, ResolvedRelationship, CommunitySummary
from src.domain.services.graphrag.indexes import get_graphrag_collections
from src.domain.services.graphrag.transformation_logger import TransformationLogger
from src.domain.services.graphrag.run_metadata import (
    compute_params_hash,
    compute_graph_signature,
    create_run_document,
    find_existing_run,
    update_run_document,
)
from src.core.config.paths import COLL_CHUNKS
from src.lib.error_handling.decorators import handle_errors

logger = logging.getLogger(__name__)


class CommunityDetectionStage(BaseStage):
    """
    Stage for detecting communities in the knowledge graph and generating summaries.
    """

    name = "community_detection"
    description = "Detect communities and generate summaries"
    ConfigCls = CommunityDetectionConfig

    def __init__(self):
        """Initialize the Community Detection Stage."""
        super().__init__()
        # Don't initialize agents here - will be done in setup()

    def setup(self):
        """Setup the stage with config-dependent initialization."""
        super().setup()

        # Initialize OpenAI client for LLM operations
        from src.lib.llm import get_openai_client

        self.llm_client = get_openai_client(timeout=60)

        # Initialize the community detection agent now that we have access to self.config
        self.detection_agent = CommunityDetectionAgent(
            algorithm=self.config.algorithm,
            max_cluster_size=self.config.max_cluster_size,
            min_cluster_size=self.config.min_cluster_size,
            resolution_parameter=self.config.resolution_parameter,
            max_iterations=self.config.max_iterations,
            max_levels=self.config.max_levels,
        )

        # Initialize the community summarization agent
        self.summarization_agent = CommunitySummarizationAgent(
            llm_client=self.llm_client,
            model_name=self.config.model_name,
            temperature=self.config.temperature,
            max_tokens=self.config.max_tokens,
            max_summary_length=self.config.max_summary_length,
            min_summary_length=self.config.min_summary_length,
        )

        # Get GraphRAG collections (use write DB for output)
        self.graphrag_collections = get_graphrag_collections(self.db_write)

        # Initialize TransformationLogger for logging community operations (Achievement 0.1)
        logging_enabled = os.getenv("GRAPHRAG_TRANSFORMATION_LOGGING", "true").lower() == "true"
        self.transformation_logger = TransformationLogger(self.db_write, enabled=logging_enabled)

        # Flag and lock to ensure community detection runs only once across all chunks
        # Critical for concurrent processing with 300 workers
        import threading

        self._communities_detected = False
        self._detection_lock = threading.Lock()

        logger.info(
            f"Initialized {self.name} with max_cluster_size={self.config.max_cluster_size}, "
            f"algorithm={self.config.algorithm}"
        )

    def iter_docs(self) -> Iterator[Dict[str, Any]]:
        """
        Iterate over chunks that have completed graph construction.

        Yields:
            Chunk documents that have been processed for graph construction
        """
        src_db = self.config.read_db_name or self.config.db_name
        src_coll_name = self.config.read_coll or COLL_CHUNKS
        collection = self.get_collection(src_coll_name, io="read", db_name=src_db)

        # Query for chunks that have completed construction but not community detection
        query = {
            "graphrag_construction.status": "completed",
            "$or": [
                {"graphrag_communities": {"$exists": False}},
                {"graphrag_communities.status": {"$ne": "completed"}},
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

        logger.info(f"Querying chunks for community detection: {query}")

        cursor = collection.find(query)
        if self.config.max:
            cursor = cursor.limit(int(self.config.max))

        for doc in cursor:
            yield doc

    @handle_errors(fallback=None, log_traceback=True, reraise=False)
    def handle_doc(self, doc: Dict[str, Any]) -> Optional[Dict[str, Any]]:
        """
        Process a single chunk for community detection and write to database.

        Args:
            doc: Chunk document to process

        Returns:
            None (writes directly to database via update_one)
        """
        chunk_id = doc.get("chunk_id", "unknown")
        video_id = doc.get("video_id", "unknown")

        logger.debug(f"Processing chunk {chunk_id} from video {video_id} for community detection")

        try:
            # Community detection should run ONCE for the entire graph, not per-chunk
            # Use lock to prevent race conditions with 300 concurrent workers
            with self._detection_lock:
                # Double-check inside lock to avoid redundant runs
                communities_collection = self.graphrag_collections["communities"]
                existing_communities = list(communities_collection.find({}).limit(1))

                # Check if already detected (in this run or database)
                if self._communities_detected or existing_communities:
                    # Communities already exist - all chunks updated by bulk operation
                    # Just skip (no need for individual update, batch update handles all)
                    logger.debug(
                        f"Communities already detected ({len(existing_communities)} found). "
                        f"Chunk {chunk_id} batch-updated, skipping."
                    )
                    self.stats["skipped"] += 1
                    return None

                # First time - detect communities for the entire graph (inside lock!)
                logger.info(
                    f"🔒 LOCK ACQUIRED - Running community detection on entire graph "
                    f"(triggered by chunk {chunk_id})"
                )

                # Get all entities and relationships from the database
                entities = self._get_all_entities()
                relationships = self._get_all_relationships()

                if not entities:
                    logger.warning("No entities found for community detection")
                    return self._mark_detection_failed(doc, "no_entities")

                # Compute run metadata (params_hash and graph_signature)
                params = self._collect_detection_params()
                params_hash = compute_params_hash(params)
                graph_signature = compute_graph_signature(entities, relationships)

                # Check for existing run with same params and graph
                existing_run = find_existing_run(
                    self.db_write,
                    "community_detection",
                    params_hash,
                    graph_signature,
                )

                if existing_run:
                    # Reuse existing run
                    run_id = str(existing_run["_id"])
                    logger.info(
                        f"✅ Found existing run: run_id={run_id}, "
                        f"params_hash={params_hash[:8]}..., graph_signature={graph_signature[:8]}... "
                        f"(skipping re-detection)"
                    )
                    # Use existing communities (they should already be stored)
                    # Just update chunks with run_id
                    metrics = existing_run.get("metrics", {})
                    detection_results = {
                        "communities": {},  # Not needed for batch update
                        "total_communities": metrics.get("total_communities", 0),
                        "levels": metrics.get("levels", 1),
                    }
                    self._batch_update_all_chunks(
                        detection_results,
                        metrics.get("stored_communities", 0),
                        run_id,
                        params_hash,
                    )
                    return None

                # Create new run document
                ontology_version = self._get_ontology_version()
                # Get trace_id from config if available (Achievement 0.1: Trace ID System Integration)
                trace_id = (
                    getattr(self.config, "trace_id", None) if hasattr(self, "config") else None
                )
                run_id = create_run_document(
                    self.db_write,
                    "community_detection",
                    params_hash,
                    graph_signature,
                    params,
                    ontology_version,
                    trace_id=trace_id,
                )
                logger.info(
                    f"📝 Created new run: run_id={run_id}, params_hash={params_hash[:8]}..."
                )

                # Detect communities
                detection_results = self.detection_agent.detect_communities(entities, relationships)

                if not detection_results.get("communities"):
                    logger.warning("No communities detected after organization")
                    return self._mark_detection_failed(doc, "no_communities_detected")

                logger.info(
                    f"✅ Detection successful: {detection_results['total_communities']} communities organized "
                    f"across {detection_results['levels']} level(s)"
                )

                # Generate community summaries (concurrent if enabled)
                use_concurrent = os.getenv("GRAPHRAG_USE_TPM_TRACKING", "true").lower() == "true"
                community_summaries = self.summarization_agent.summarize_communities(
                    detection_results["communities"],
                    entities,
                    relationships,
                    concurrent=use_concurrent,
                    max_workers=int(self.config.concurrency or 300),
                )

                if not community_summaries:
                    logger.warning("No community summaries generated")
                    return self._mark_detection_failed(doc, "no_summaries_generated")

                # Store communities in the communities collection (with run_id and params_hash)
                stored_communities = self._store_communities(
                    community_summaries, chunk_id, video_id, run_id, params_hash
                )

                # Update run document with completion status and metrics
                quality_metrics = detection_results.get("quality_metrics", {})
                graph_stats = detection_results.get("graph_stats", {})

                metrics = {
                    "total_communities": detection_results["total_communities"],
                    "levels": detection_results["levels"],
                    "stored_communities": len(stored_communities),
                    "modularity": quality_metrics.get("avg_coherence", 0),
                    "coverage": quality_metrics.get("coverage", 0),
                    "graph_stats": graph_stats,
                    # Full quality metrics for persistence
                    "quality_metrics": quality_metrics,
                }
                update_run_document(
                    self.db_write,
                    run_id,
                    status="completed",
                    metrics=metrics,
                )

                # Persist metrics to graphrag_metrics collection (Achievement 1.4)
                self._persist_quality_metrics(
                    run_id, params_hash, quality_metrics, graph_stats, detection_results
                )

                # Mark that communities have been detected (prevents re-detection for other chunks)
                self._communities_detected = True
                logger.info(
                    f"✅ Community detection complete: stored {len(stored_communities)} communities, "
                    f"run_id={run_id}"
                )

                # Update entities with community assignments
                self._update_entity_communities(community_summaries)

                # Batch update ALL chunks at once (much faster than one-by-one updates)
                # This includes the current triggering chunk, so no need for individual update
                logger.info("📝 Batch updating all chunks with community detection status...")
                self._batch_update_all_chunks(
                    detection_results, len(stored_communities), run_id, params_hash
                )

                # Done! All chunks updated in batch, return immediately
                logger.info("✅ All chunks updated. Community detection stage complete!")
                return None

            # This code should never be reached (batch update handles everything)
            # Prepare detection payload
            detection_payload = {
                "graphrag_communities": {
                    "status": "completed",
                    "detected_communities": len(detection_results["communities"]),
                    "total_communities": detection_results["total_communities"],
                    "levels": detection_results["levels"],
                    "stored_communities": len(stored_communities),
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
                    {"graphrag_communities.status": 1},
                )
                if (
                    existing
                    and existing.get("graphrag_communities", {}).get("status") == "completed"
                ):
                    logger.debug(f"Skipping chunk {chunk_id} - already has completed detection")
                    self.stats["skipped"] += 1
                    return None

            # Update the chunk with detection results
            collection.update_one(
                {"video_id": video_id, "chunk_id": chunk_id},
                {"$set": detection_payload},
                upsert=False,
            )

            self.stats["updated"] += 1
            logger.debug(
                f"Successfully detected {len(stored_communities)} communities "
                f"for chunk {chunk_id}"
            )

            return None

        except Exception as e:
            logger.error(f"Error processing chunk {chunk_id} for community detection: {e}")
            return self._mark_detection_failed(doc, str(e))

    def _collect_detection_params(self) -> Dict[str, Any]:
        """
        Collect all detection parameters for params_hash computation.

        Returns:
            Dictionary of all parameters affecting detection
        """
        # Get random seed from environment (default to 42)
        seed = os.getenv("GRAPHRAG_RANDOM_SEED", "42")

        params = {
            "algorithm": self.config.algorithm,
            "resolution_parameter": self.config.resolution_parameter,
            "min_cluster_size": self.config.min_cluster_size,
            "max_cluster_size": self.config.max_cluster_size,
            "max_iterations": self.config.max_iterations,
            "max_levels": self.config.max_levels,
            "seed": seed,
            "model_name": self.config.model_name,
            "temperature": self.config.temperature,
            "concurrency": self.config.concurrency or 300,
        }

        return params

    def _get_ontology_version(self) -> str:
        """
        Get ontology version string.

        Returns:
            Ontology version string (or "unknown" if unavailable)
        """
        try:
            from src.lib.ontology.loader import load_ontology

            ontology = load_ontology()
            # Compute a simple version hash from canonical predicates
            # This detects when ontology changes
            if ontology.get("canonical_predicates"):
                import hashlib

                predicates_str = ",".join(sorted(ontology["canonical_predicates"]))
                version_hash = hashlib.sha1(predicates_str.encode()).hexdigest()[:8]
                return f"hash-{version_hash}"
            return "unknown"
        except Exception as e:
            logger.debug(f"Could not determine ontology version: {e}")
            return "unknown"

    def _get_all_entities(self) -> List[ResolvedEntity]:
        """
        Get all entities from the entities collection.

        Returns:
            List of ResolvedEntity objects
        """
        entities_collection = self.graphrag_collections["entities"]

        entity_docs = entities_collection.find({})
        entities = []

        for doc in entity_docs:
            try:
                entity = ResolvedEntity(
                    entity_id=doc["entity_id"],
                    canonical_name=doc["canonical_name"],
                    name=doc["name"],
                    type=doc["type"],
                    description=doc["description"],
                    confidence=doc.get("confidence", 0.0),
                    source_count=doc.get("source_count", 1),
                    resolution_methods=doc.get("resolution_methods", []),
                    aliases=doc.get("aliases", []),
                )
                entities.append(entity)
            except Exception as e:
                logger.warning(f"Failed to parse entity document: {e}")
                continue

        logger.debug(f"Retrieved {len(entities)} entities")
        return entities

    def _get_all_relationships(self) -> List[ResolvedRelationship]:
        """
        Get all relationships from the relations collection.

        Returns:
            List of ResolvedRelationship objects
        """
        relations_collection = self.graphrag_collections["relations"]

        relationship_docs = relations_collection.find({})
        relationships = []

        for doc in relationship_docs:
            try:
                relationship = ResolvedRelationship(
                    relationship_id=doc["relationship_id"],
                    subject_id=doc["subject_id"],
                    object_id=doc["object_id"],
                    predicate=doc["predicate"],
                    description=doc["description"],
                    confidence=doc.get("confidence", 0.0),
                    source_count=doc.get("source_count", 1),
                )
                relationships.append(relationship)
            except Exception as e:
                logger.warning(f"Failed to parse relationship document: {e}")
                continue

        logger.debug(f"Retrieved {len(relationships)} relationships")
        return relationships

    def _store_communities(
        self,
        community_summaries: Dict[str, CommunitySummary],
        chunk_id: str,
        video_id: str,
        run_id: Optional[str] = None,
        params_hash: Optional[str] = None,
    ) -> List[str]:
        """
        Store community summaries in the communities collection.

        Args:
            community_summaries: Dictionary of community summaries
            chunk_id: Source chunk ID
            video_id: Source video ID

        Returns:
            List of stored community IDs
        """
        communities_collection = self.graphrag_collections["communities"]
        stored_community_ids = []

        for community_id, summary in community_summaries.items():
            try:
                # Check if community already exists
                existing_community = communities_collection.find_one({"community_id": community_id})

                if existing_community:
                    # Update existing community
                    self._update_existing_community(
                        existing_community,
                        summary,
                        chunk_id,
                        video_id,
                        run_id,
                        params_hash,
                    )
                else:
                    # Insert new community
                    self._insert_new_community(summary, chunk_id, video_id, run_id, params_hash)

                stored_community_ids.append(community_id)

            except Exception as e:
                logger.error(f"Failed to store community {community_id}: {e}")
                continue

        return stored_community_ids

    def _update_existing_community(
        self,
        existing_community: Dict[str, Any],
        summary: CommunitySummary,
        chunk_id: str,
        video_id: str,
    ) -> None:
        """
        Update an existing community with new information.

        Args:
            existing_community: Existing community document
            summary: New community summary
            chunk_id: Source chunk ID
            video_id: Source video ID
        """
        communities_collection = self.graphrag_collections["communities"]

        # Update summary if new one is more comprehensive
        update_data = {
            "$addToSet": {"source_chunks": chunk_id},
            "$set": {"updated_at": time.time()},
        }

        if len(summary.summary) > len(existing_community.get("summary", "")):
            update_data["$set"]["summary"] = summary.summary
            update_data["$set"]["title"] = summary.title

        # Update run metadata if provided (for provenance)
        if run_id:
            update_data["$set"]["run_id"] = run_id
        if params_hash:
            update_data["$set"]["params_hash"] = params_hash

        communities_collection.update_one({"community_id": summary.community_id}, update_data)

    def _insert_new_community(
        self,
        summary: CommunitySummary,
        chunk_id: str,
        video_id: str,
        run_id: Optional[str] = None,
        params_hash: Optional[str] = None,
    ) -> None:
        """
        Insert a new community into the communities collection.

        Args:
            summary: Community summary to insert
            chunk_id: Source chunk ID
            video_id: Source video ID
        """
        communities_collection = self.graphrag_collections["communities"]

        community_doc = {
            "community_id": summary.community_id,
            "level": summary.level,
            "title": summary.title,
            "summary": summary.summary,
            "entities": summary.entities,
            "entity_count": summary.entity_count,
            "relationship_count": summary.relationship_count,
            "coherence_score": summary.coherence_score,
            "source_chunks": [chunk_id],
            "video_id": video_id,
            "created_at": time.time(),
            "updated_at": time.time(),
            # Run metadata (for provenance and reproducibility)
            "run_id": run_id,
            "params_hash": params_hash,
        }

        communities_collection.insert_one(community_doc)

        # Achievement 0.1: Log community formation
        trace_id = self.config.trace_id if hasattr(self.config, "trace_id") else None
        self.transformation_logger.log_community_form(
            community_id=summary.community_id,
            entities=[{"id": eid, "name": ""} for eid in summary.entities],
            modularity=summary.coherence_score,
            coherence=summary.coherence_score,
            algorithm=self.config.algorithm,
            resolution_parameter=self.config.resolution_parameter,
            trace_id=trace_id,
        )

    def _update_entity_communities(self, community_summaries: Dict[str, CommunitySummary]) -> None:
        """
        Update entities with their community assignments.

        Args:
            community_summaries: Dictionary of community summaries
        """
        entities_collection = self.graphrag_collections["entities"]

        for community_id, summary in community_summaries.items():
            # Update entities with community assignment
            entities_collection.update_many(
                {"entity_id": {"$in": summary.entities}},
                {
                    "$addToSet": {f"community_assignments.level_{summary.level}": community_id},
                    "$set": {"community_updated_at": time.time()},
                },
            )

            # Achievement 0.1: Log entity cluster assignments
            trace_id = self.config.trace_id if hasattr(self.config, "trace_id") else None
            for entity_id in summary.entities:
                self.transformation_logger.log_entity_cluster(
                    entity={"id": entity_id, "name": ""},
                    community_id=community_id,
                    reason="algorithm_assignment",
                    neighbors=len(summary.entities) - 1,
                    trace_id=trace_id,
                )

    def _mark_detection_failed(self, doc: Dict[str, Any], error_message: str) -> None:
        """
        Mark detection as failed for a document.

        Args:
            doc: Document to mark as failed
            error_message: Error message describing the failure
        """
        chunk_id = doc.get("chunk_id", "unknown")
        video_id = doc.get("video_id", "unknown")

        detection_payload = {
            "graphrag_communities": {
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
            {"$set": detection_payload},
            upsert=False,
        )

        self.stats["failed"] += 1
        logger.warning(f"Marked chunk {chunk_id} detection as failed: {error_message}")

        return None

    @handle_errors(fallback=[], log_traceback=True, reraise=False)
    def process_batch(self, docs: List[Dict[str, Any]]) -> List[Optional[Dict[str, Any]]]:
        """
        Process a batch of documents for community detection.

        Args:
            docs: List of documents to process

        Returns:
            List of processed documents (or None for failed processing)
        """
        logger.info(f"Processing batch of {len(docs)} chunks for community detection")

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

    def _batch_update_all_chunks(
        self,
        detection_results: Dict[str, Any],
        stored_count: int,
        run_id: Optional[str] = None,
        params_hash: Optional[str] = None,
    ):
        """
        Batch update all chunks with community detection status (MASSIVE performance improvement).

        PERFORMANCE FIX (2024-11-04):
        ==============================
        - Before: Updated 12,959 chunks ONE BY ONE → ~17 minutes (silent, felt frozen)
        - After: Batch update all chunks at once → ~1 second
        - Improvement: ~1000× faster!

        Why this works:
        - Community detection is run ONCE for the entire graph (all entities/relationships)
        - All chunks should get the SAME detection status (communities detected)
        - MongoDB's update_many() is optimized for bulk operations

        Technical details:
        - Uses update_many() with query matching all completed construction chunks
        - Single database round-trip instead of 12,959 separate queries
        - Sets identical payload on all matching documents
        """
        dst_db = self.config.write_db_name or self.config.db_name
        dst_coll_name = self.config.write_coll or COLL_CHUNKS
        collection = self.get_collection(dst_coll_name, io="write", db_name=dst_db)

        # Prepare detection payload for all chunks
        detection_payload = {
            "graphrag_communities": {
                "status": "completed",
                "detected_communities": len(detection_results["communities"]),
                "total_communities": detection_results["total_communities"],
                "levels": detection_results["levels"],
                "stored_communities": stored_count,
                "processed_at": time.time(),
                "model_used": self.config.model_name,
                # Run metadata (for provenance and reproducibility)
                "run_id": run_id,
                "params_hash": params_hash,
            }
        }

        # Bulk update ALL chunks that have completed graph construction
        # This is MUCH faster than updating one by one (12959 chunks in ~1 second vs ~17 minutes)
        query = {
            "graphrag_construction.status": "completed",
            "$or": [
                {"graphrag_communities": {"$exists": False}},
                {"graphrag_communities.status": {"$ne": "completed"}},
            ],
        }

        result = collection.update_many(query, {"$set": detection_payload})

        logger.info(
            f"✅ Batch updated {result.modified_count} chunks with community detection status "
            f"(matched={result.matched_count})"
        )

        self.stats["updated"] += result.modified_count

    def get_detection_stats(self) -> Dict[str, Any]:
        """
        Get statistics about the detection stage.

        Returns:
            Dictionary containing detection statistics
        """
        src_db = self.config.read_db_name or self.config.db_name
        src_coll_name = self.config.read_coll or COLL_CHUNKS
        collection = self.get_collection(src_coll_name, io="read", db_name=src_db)
        communities_collection = self.graphrag_collections["communities"]

        # Count total chunks with construction
        total_constructed = collection.count_documents(
            {"graphrag_construction.status": "completed"}
        )

        # Count detected chunks
        detected_chunks = collection.count_documents({"graphrag_communities.status": "completed"})

        # Count failed chunks
        failed_chunks = collection.count_documents({"graphrag_communities.status": "failed"})

        # Count pending chunks
        pending_chunks = total_constructed - detected_chunks - failed_chunks

        # Count total communities
        total_communities = communities_collection.count_documents({})

        # Count communities by level
        level_pipeline = [{"$group": {"_id": "$level", "count": {"$sum": 1}}}]
        level_results = list(communities_collection.aggregate(level_pipeline))
        level_distribution = {str(result["_id"]): result["count"] for result in level_results}

        return {
            "total_constructed_chunks": total_constructed,
            "detected_chunks": detected_chunks,
            "failed_chunks": failed_chunks,
            "pending_chunks": pending_chunks,
            "total_communities": total_communities,
            "level_distribution": level_distribution,
            "completion_rate": (
                detected_chunks / total_constructed if total_constructed > 0 else 0
            ),
            "failure_rate": (failed_chunks / total_constructed if total_constructed > 0 else 0),
        }

    def cleanup_failed_detections(self) -> int:
        """
        Clean up failed detection records to allow retry.

        Returns:
            Number of failed detections cleaned up
        """
        src_db = self.config.read_db_name or self.config.db_name
        src_coll_name = self.config.read_coll or COLL_CHUNKS
        collection = self.get_collection(src_coll_name, io="read", db_name=src_db)

        result = collection.update_many(
            {"graphrag_communities.status": "failed"},
            {"$unset": {"graphrag_communities": 1}},
        )

        logger.info(f"Cleaned up {result.modified_count} failed detections")
        return result.modified_count

    def _persist_quality_metrics(
        self,
        run_id: str,
        params_hash: str,
        quality_metrics: Dict[str, Any],
        graph_stats: Dict[str, Any],
        detection_results: Dict[str, Any],
    ) -> None:
        """
        Persist quality metrics to graphrag_metrics collection.

        Achievement 1.4: Quality Metrics Persistence

        Args:
            run_id: Run ID
            params_hash: Parameters hash
            quality_metrics: Quality metrics dictionary
            graph_stats: Graph statistics dictionary
            detection_results: Full detection results
        """
        try:
            metrics_collection = self.db_write.graphrag_metrics

            # Calculate size distribution histogram
            size_distribution = {}
            for level, level_communities in detection_results.get("communities", {}).items():
                sizes = [
                    comm_data.get("entity_count", 0) for comm_data in level_communities.values()
                ]
                if sizes:
                    size_distribution[level] = {
                        "min": min(sizes),
                        "max": max(sizes),
                        "mean": sum(sizes) / len(sizes),
                        "median": sorted(sizes)[len(sizes) // 2],
                        "histogram": self._compute_size_histogram(sizes),
                    }

            # Create metrics document
            metrics_doc = {
                "run_id": run_id,
                "params_hash": params_hash,
                "stage": "community_detection",
                "timestamp": time.time(),
                # Graph metrics
                "graph": {
                    "nodes": graph_stats.get("nodes", 0),
                    "edges": graph_stats.get("edges", 0),
                    "density": graph_stats.get("density", 0),
                    "edge_to_node_ratio": (
                        graph_stats.get("edges", 0) / graph_stats.get("nodes", 1)
                        if graph_stats.get("nodes", 0) > 0
                        else 0
                    ),
                },
                # Detection metrics
                "detection": {
                    "total_communities": detection_results.get("total_communities", 0),
                    "levels": detection_results.get("levels", 0),
                    "modularity": quality_metrics.get("modularity", 0),
                    "coverage": quality_metrics.get("coverage", 0),
                    "size_distribution": size_distribution,
                },
                # Quality metrics
                "quality": {
                    "avg_coherence": quality_metrics.get("avg_coherence", 0),
                    "min_coherence": quality_metrics.get("min_coherence", 0),
                    "max_coherence": quality_metrics.get("max_coherence", 0),
                    "coherence_stats": quality_metrics.get("coherence_stats", {}),
                },
            }

            # Insert metrics document
            metrics_collection.insert_one(metrics_doc)

            logger.info(
                f"Persisted quality metrics for run_id={run_id}, "
                f"modularity={quality_metrics.get('modularity', 0):.4f}, "
                f"coverage={quality_metrics.get('coverage', 0):.4f}"
            )

        except Exception as e:
            logger.error(f"Failed to persist quality metrics: {e}")

    def _compute_size_histogram(self, sizes: List[int]) -> Dict[str, int]:
        """
        Compute size distribution histogram.

        Args:
            sizes: List of community sizes

        Returns:
            Histogram dictionary with bins
        """
        if not sizes:
            return {}

        bins = [0, 5, 10, 25, 50, 100, 500, 1000, float("inf")]
        histogram = {}

        for size in sizes:
            for i in range(len(bins) - 1):
                if bins[i] <= size < bins[i + 1]:
                    bin_label = f"{bins[i]}-{bins[i+1] if bins[i+1] != float('inf') else 'inf'}"
                    histogram[bin_label] = histogram.get(bin_label, 0) + 1
                    break

        return histogram
