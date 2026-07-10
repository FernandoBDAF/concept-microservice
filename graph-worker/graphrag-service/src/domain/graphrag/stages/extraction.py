"""
Graph Extraction Stage

This stage extracts entities and relationships from video chunks using LLM-based extraction.
Extends the BaseStage to integrate with the existing pipeline architecture.

OPTIMIZATION: Now supports concurrent processing with configurable workers for 10-60x speedup.
"""

import logging
import os
import time
from typing import Dict, List, Any, Optional, Iterator
from src.core.base.stage import BaseStage
from src.core.config.graphrag import GraphExtractionConfig
from src.domain.agents.graphrag.extraction import GraphExtractionAgent
from src.core.models.graphrag import KnowledgeModel
from src.core.config.paths import COLL_CHUNKS
from src.lib.concurrency import run_llm_concurrent
from src.lib.rate_limiting import RateLimiter
from src.lib.error_handling.decorators import handle_errors
import json

logger = logging.getLogger(__name__)


class GraphExtractionStage(BaseStage):
    """
    Stage for extracting entities and relationships from video chunks.
    """

    name = "graph_extraction"
    description = "Extract entities and relationships from text chunks"
    ConfigCls = GraphExtractionConfig

    def __init__(self):
        """Initialize the Graph Extraction Stage."""
        super().__init__()
        # Don't initialize agent here - will be done in setup()

    def setup(self):
        """Setup the stage with config-dependent initialization."""
        super().setup()

        # Initialize OpenAI client for LLM operations
        from src.lib.llm import get_openai_client

        self.llm_client = get_openai_client(timeout=60)

        # Initialize the extraction agent now that we have access to self.config
        self.extraction_agent = GraphExtractionAgent(
            llm_client=self.llm_client,
            model_name=self.config.model_name,
            temperature=self.config.temperature,
            max_tokens=self.config.max_tokens,
        )

        logger.info(f"Initialized {self.name} with model {self.config.model_name}")

    def iter_docs(self) -> Iterator[Dict[str, Any]]:
        """
        Iterate over chunks that need entity extraction.

        Yields:
            Chunk documents that need processing
        """
        src_db = self.config.read_db_name or self.config.db_name
        src_coll_name = self.config.read_coll or COLL_CHUNKS
        collection = self.get_collection(src_coll_name, io="read", db_name=src_db)

        # Query for chunks that haven't been processed for entity extraction
        # Exclude: completed, skipped (these are handled gracefully)
        query = {
            "chunk_text": {"$exists": True, "$ne": ""},
            "$or": [
                {"graphrag_extraction": {"$exists": False}},
                {"graphrag_extraction.status": {"$nin": ["completed", "skipped"]}},
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

        logger.info(f"Querying chunks for entity extraction: {query}")

        cursor = collection.find(query)
        if self.config.max:
            cursor = cursor.limit(int(self.config.max))

        for doc in cursor:
            yield doc

    @handle_errors(fallback=None, log_traceback=True, reraise=False)
    def handle_doc(self, doc: Dict[str, Any]) -> Optional[Dict[str, Any]]:
        """
        Extract entities and relationships from a single chunk and write to database.

        Args:
            doc: Chunk document to process

        Returns:
            None (writes directly to database via update_one)
        """
        chunk_id = doc.get("chunk_id", "unknown")
        video_id = doc.get("video_id", "unknown")

        logger.debug(f"Processing chunk {chunk_id} from video {video_id}")

        # Pre-filter: Skip chunks that are too short or contain only noise
        chunk_text = doc.get("chunk_text", "").strip()
        if not chunk_text:
            logger.debug(f"Skipping chunk {chunk_id}: empty text")
            return self._mark_extraction_skipped(doc, "chunk_empty")

        # Skip chunks with very short text (likely fragments/noise)
        # Threshold: 50 characters (can be configured)
        MIN_CHUNK_LENGTH = 50
        if len(chunk_text) < MIN_CHUNK_LENGTH:
            # Check if it's only punctuation/whitespace
            import string

            text_without_punct = chunk_text.translate(
                str.maketrans("", "", string.punctuation + string.whitespace)
            )
            if len(text_without_punct) == 0:
                logger.debug(
                    f"Skipping chunk {chunk_id}: only punctuation/whitespace ({len(chunk_text)} chars)"
                )
                return self._mark_extraction_skipped(doc, "chunk_noise_only")
            elif len(chunk_text) < 20:
                logger.debug(
                    f"Skipping chunk {chunk_id}: too short ({len(chunk_text)} chars, threshold: {MIN_CHUNK_LENGTH})"
                )
                return self._mark_extraction_skipped(doc, "chunk_too_short")

        try:
            # Extract entities and relationships
            knowledge_model = self.extraction_agent.extract_from_chunk(doc)

            if knowledge_model is None:
                # Check if it's a "no_entities" case (graceful skip) vs actual failure
                # The agent returns None for both, but we can distinguish by checking
                # if the chunk was marked as "no_entities" in the extraction result
                # For now, we'll mark as "no_entities" if text is short, otherwise "failed"
                if len(chunk_text) < 100:
                    logger.debug(
                        f"Chunk {chunk_id} returned no entities (likely none to extract)"
                    )
                    return self._mark_extraction_skipped(doc, "no_entities")
                else:
                    logger.warning(f"Failed to extract entities from chunk {chunk_id}")
                    return self._mark_extraction_failed(doc, "extraction_failed")

            # Convert to serializable format
            extraction_data = {
                "entities": [
                    {
                        "name": entity.name,
                        "type": entity.type.value,
                        "description": entity.description,
                        "confidence": entity.confidence,
                    }
                    for entity in knowledge_model.entities
                ],
                "relationships": [
                    {
                        "source_entity": {
                            "name": rel.source_entity.name,
                            "type": rel.source_entity.type.value,
                            "description": rel.source_entity.description,
                            "confidence": rel.source_entity.confidence,
                        },
                        "target_entity": {
                            "name": rel.target_entity.name,
                            "type": rel.target_entity.type.value,
                            "description": rel.target_entity.description,
                            "confidence": rel.target_entity.confidence,
                        },
                        "relation": rel.relation,
                        "description": rel.description,
                        "confidence": rel.confidence,
                    }
                    for rel in knowledge_model.relationships
                ],
                "extraction_stats": {
                    "entity_count": len(knowledge_model.entities),
                    "relationship_count": len(knowledge_model.relationships),
                    "extraction_timestamp": time.time(),
                },
            }

            # Prepare extraction payload
            extraction_payload = {
                "graphrag_extraction": {
                    "status": "completed",
                    "data": extraction_data,
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
                    {"graphrag_extraction.status": 1},
                )
                if (
                    existing
                    and existing.get("graphrag_extraction", {}).get("status")
                    == "completed"
                ):
                    logger.debug(
                        f"Skipping chunk {chunk_id} - already has completed extraction"
                    )
                    self.stats["skipped"] += 1
                    return None

            # Update the chunk with extraction results
            collection.update_one(
                {"video_id": video_id, "chunk_id": chunk_id},
                {"$set": extraction_payload},
                upsert=False,
            )

            self.stats["updated"] += 1
            logger.debug(
                f"Successfully extracted {len(knowledge_model.entities)} entities "
                f"and {len(knowledge_model.relationships)} relationships from chunk {chunk_id}"
            )

            return None

        except Exception as e:
            logger.error(f"Error processing chunk {chunk_id}: {e}")
            return self._mark_extraction_failed(doc, str(e))

    def _mark_extraction_failed(self, doc: Dict[str, Any], error_message: str) -> None:
        """
        Mark extraction as failed for a document.

        Args:
            doc: Document to mark as failed
            error_message: Error message describing the failure
        """
        chunk_id = doc.get("chunk_id", "unknown")
        video_id = doc.get("video_id", "unknown")

        extraction_payload = {
            "graphrag_extraction": {
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
            {"$set": extraction_payload},
            upsert=False,
        )

        self.stats["failed"] += 1
        logger.warning(f"Marked chunk {chunk_id} as failed: {error_message}")

        return None

    def _mark_extraction_skipped(self, doc: Dict[str, Any], reason: str) -> None:
        """
        Mark extraction as skipped for a document (graceful skip, not failure).

        Args:
            doc: Document to mark as skipped
            reason: Reason for skipping (e.g., "chunk_too_short", "no_entities", "chunk_empty")
        """
        chunk_id = doc.get("chunk_id", "unknown")
        video_id = doc.get("video_id", "unknown")

        extraction_payload = {
            "graphrag_extraction": {
                "status": "skipped",
                "reason": reason,
                "processed_at": time.time(),
                "model_used": self.config.model_name,
            }
        }

        # Write skip status to database
        dst_db = self.config.write_db_name or self.config.db_name
        dst_coll_name = self.config.write_coll or COLL_CHUNKS
        collection = self.get_collection(dst_coll_name, io="write", db_name=dst_db)

        collection.update_one(
            {"video_id": video_id, "chunk_id": chunk_id},
            {"$set": extraction_payload},
            upsert=False,
        )

        # Track skipped separately from failed
        if "skipped" not in self.stats:
            self.stats["skipped"] = 0
        self.stats["skipped"] += 1
        logger.debug(f"Skipped chunk {chunk_id}: {reason}")

        return None

    @handle_errors(fallback=[], log_traceback=True, reraise=False)
    def process_batch(
        self, docs: List[Dict[str, Any]]
    ) -> List[Optional[Dict[str, Any]]]:
        """
        Process a batch of documents with concurrent extraction.

        Args:
            docs: List of documents to process

        Returns:
            List of processed documents (or None for failed processing)
        """
        logger.info(f"Processing batch of {len(docs)} chunks for entity extraction")

        results = []
        for i, doc in enumerate(docs):
            logger.debug(f"Processing document {i + 1}/{len(docs)}")

            try:
                result = self.handle_doc(doc)
                results.append(result)

                # Add small delay to avoid rate limiting
                if i < len(docs) - 1:  # Don't delay after the last document
                    time.sleep(0.1)

            except Exception as e:
                logger.error(f"Error processing document {i + 1}: {e}")
                results.append(None)

        successful_count = sum(1 for r in results if r is not None)
        failed_count = sum(1 for r in results if r is None)
        skipped_count = self.stats.get("skipped", 0)
        logger.info(
            f"Batch processing completed: {successful_count}/{len(docs)} successful "
            f"(updated={self.stats.get('updated', 0)}, failed={failed_count}, skipped={skipped_count})"
        )

        return results

    def get_processing_stats(self) -> Dict[str, Any]:
        """
        Get statistics about the processing stage.

        Returns:
            Dictionary containing processing statistics
        """
        src_db = self.config.read_db_name or self.config.db_name
        src_coll_name = self.config.read_coll or COLL_CHUNKS
        collection = self.get_collection(src_coll_name, io="read", db_name=src_db)

        # Count total chunks
        total_chunks = collection.count_documents(
            {"chunk_text": {"$exists": True, "$ne": ""}}
        )

        # Count processed chunks
        processed_chunks = collection.count_documents(
            {"graphrag_extraction.status": "completed"}
        )

        # Count failed chunks
        failed_chunks = collection.count_documents(
            {"graphrag_extraction.status": "failed"}
        )

        # Count pending chunks
        pending_chunks = total_chunks - processed_chunks - failed_chunks

        return {
            "total_chunks": total_chunks,
            "processed_chunks": processed_chunks,
            "failed_chunks": failed_chunks,
            "pending_chunks": pending_chunks,
            "completion_rate": (
                processed_chunks / total_chunks if total_chunks > 0 else 0
            ),
            "failure_rate": failed_chunks / total_chunks if total_chunks > 0 else 0,
        }

    def cleanup_failed_extractions(self) -> int:
        """
        Clean up failed extraction records to allow retry.

        Returns:
            Number of failed extractions cleaned up
        """
        src_db = self.config.read_db_name or self.config.db_name
        src_coll_name = self.config.read_coll or COLL_CHUNKS
        collection = self.get_collection(src_coll_name, io="read", db_name=src_db)

        result = collection.update_many(
            {"graphrag_extraction.status": "failed"},
            {"$unset": {"graphrag_extraction": 1}},
        )

        logger.info(f"Cleaned up {result.modified_count} failed extractions")
        return result.modified_count

    # =============================================================================
    # Concurrency Implementation
    # =============================================================================
    # run() inherited from BaseStage - auto-detects concurrency and calls appropriate method
    #
    # Template methods (override base class behavior):
    # - estimate_tokens() - estimates tokens for extraction
    # - process_doc_with_tracking() - calls extraction agent
    # - store_batch_results() - stores extraction results
    #
    # BaseStage.run() automatically:
    # - Detects GRAPHRAG_USE_TPM_TRACKING=true (default) → calls _run_concurrent_with_tpm()
    # - Detects --concurrency > 1 → calls _run_concurrent() or _run_concurrent_with_tpm()
    # - Falls back to sequential if no concurrency
    # =============================================================================

    # _run_concurrent removed - BaseStage.run() handles concurrency automatically
    # If GRAPHRAG_USE_TPM_TRACKING is disabled, BaseStage falls back to _run_concurrent_with_tpm()
    # which works fine for all cases

    def estimate_tokens(self, doc: Dict[str, Any]) -> int:
        """Estimate tokens for extraction (override base method)."""
        text = doc.get("chunk_text", "")
        input_tokens = len(text) / 4  # ~4 chars per token
        output_tokens = 1000  # Average extraction output
        return int(input_tokens + output_tokens)

    def process_doc_with_tracking(self, doc: Dict[str, Any]) -> Any:
        """Process chunk with extraction agent (override base method)."""
        return self.extraction_agent.extract_from_chunk(doc)

    def store_batch_results(
        self, batch_results: List[Any], batch_docs: List[Dict[str, Any]]
    ) -> None:
        """Store batch extraction results (override base method)."""
        self._store_concurrent_results(batch_docs, batch_results)

    def _store_concurrent_results(
        self, docs: List[Dict[str, Any]], results: List[Optional[KnowledgeModel]]
    ) -> None:
        """Store results from concurrent extraction."""
        dst_db = self.config.write_db_name
        dst_coll_name = self.config.write_coll
        collection = self.get_collection(dst_coll_name, io="write", db_name=dst_db)

        for idx, (doc, knowledge_model) in enumerate(zip(docs, results), start=1):
            chunk_id = doc.get("chunk_id", "unknown")
            video_id = doc.get("video_id", "unknown")

            try:
                if knowledge_model is None:
                    # Extraction failed
                    extraction_payload = {
                        "graphrag_extraction": {
                            "status": "failed",
                            "error": "extraction_failed",
                            "processed_at": time.time(),
                            "model_used": self.config.model_name,
                        }
                    }
                    self.stats["failed"] += 1
                else:
                    # Extraction succeeded - convert to dict
                    extraction_data = {
                        "entities": [
                            {
                                "name": entity.name,
                                "type": entity.type.value,
                                "description": entity.description,
                                "confidence": entity.confidence,
                            }
                            for entity in knowledge_model.entities
                        ],
                        "relationships": [
                            {
                                "source_entity": {
                                    "name": rel.source_entity.name,
                                    "type": rel.source_entity.type.value,
                                    "description": rel.source_entity.description,
                                    "confidence": rel.source_entity.confidence,
                                },
                                "target_entity": {
                                    "name": rel.target_entity.name,
                                    "type": rel.target_entity.type.value,
                                    "description": rel.target_entity.description,
                                    "confidence": rel.target_entity.confidence,
                                },
                                "relation": rel.relation,
                                "description": rel.description,
                                "confidence": rel.confidence,
                            }
                            for rel in knowledge_model.relationships
                        ],
                        "extraction_stats": {
                            "entity_count": len(knowledge_model.entities),
                            "relationship_count": len(knowledge_model.relationships),
                            "extraction_timestamp": time.time(),
                        },
                    }

                    extraction_payload = {
                        "graphrag_extraction": {
                            "status": "completed",
                            "data": extraction_data,
                            "processed_at": time.time(),
                            "model_used": self.config.model_name,
                        }
                    }
                    self.stats["updated"] += 1

                # Update chunk
                collection.update_one(
                    {"video_id": video_id, "chunk_id": chunk_id},
                    {"$set": extraction_payload},
                    upsert=False,
                )

                # Log progress
                if idx % 100 == 0:
                    logger.info(
                        f"[graph_extraction] Stored {idx}/{len(docs)} results "
                        f"(updated={self.stats['updated']}, failed={self.stats['failed']})"
                    )

            except Exception as e:
                logger.error(f"Failed to store result for chunk {chunk_id}: {e}")
                self.stats["failed"] += 1

        # Log final summary
        logger.info(
            f"[graph_extraction] Storage complete: "
            f"updated={self.stats['updated']}, failed={self.stats['failed']}, "
            f"skipped={self.stats.get('skipped', 0)}"
        )

    # _run_concurrent_with_tpm now inherited from BaseStage
    # Template methods override base behavior:
    # - estimate_tokens() - estimates tokens for extraction
    # - process_doc_with_tracking() - calls extraction agent
    # - store_batch_results() - stores extraction results

    def get_extraction_summary(self, video_id: Optional[str] = None) -> Dict[str, Any]:
        """
        Get a summary of extraction results for a specific video or all videos.

        Args:
            video_id: Optional video ID to filter by

        Returns:
            Dictionary containing extraction summary
        """
        src_db = self.config.read_db_name or self.config.db_name
        src_coll_name = self.config.read_coll or COLL_CHUNKS
        collection = self.get_collection(src_coll_name, io="read", db_name=src_db)

        query = {"graphrag_extraction.status": "completed"}
        if video_id:
            query["video_id"] = video_id

        pipeline = [
            {"$match": query},
            {
                "$project": {
                    "entity_count": {"$size": "$graphrag_extraction.data.entities"},
                    "relationship_count": {
                        "$size": "$graphrag_extraction.data.relationships"
                    },
                    "video_id": 1,
                    "chunk_id": 1,
                }
            },
            {
                "$group": {
                    "_id": "$video_id" if not video_id else None,
                    "total_chunks": {"$sum": 1},
                    "total_entities": {"$sum": "$entity_count"},
                    "total_relationships": {"$sum": "$relationship_count"},
                    "avg_entities_per_chunk": {"$avg": "$entity_count"},
                    "avg_relationships_per_chunk": {"$avg": "$relationship_count"},
                }
            },
        ]

        results = list(collection.aggregate(pipeline))

        if video_id:
            return (
                results[0]
                if results
                else {
                    "total_chunks": 0,
                    "total_entities": 0,
                    "total_relationships": 0,
                    "avg_entities_per_chunk": 0,
                    "avg_relationships_per_chunk": 0,
                }
            )
        else:
            return {"videos": results, "total_videos": len(results)}
