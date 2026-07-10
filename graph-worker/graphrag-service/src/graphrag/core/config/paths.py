"""
Centralized constants and lightweight helpers for the self-contained Mongo_Hack project.
This module must not import anything from outside Mongo_Hack/.
"""

import os
from typing import Final

# Database and collection names
DB_NAME: Final[str] = os.getenv("MONGODB_DB", "AWSAgentCoreDemoDB")

# Raw document collections - source-specific (one per document type)
# Each document type (YouTube, PDF, HTML, etc.) should have its own raw collection
# Examples: raw_videos, raw_pdfs, raw_html, etc.
COLL_RAW_VIDEOS: Final[str] = "raw_videos"
COLL_CLEANED: Final[str] = "cleaned_transcripts"
COLL_ENRICHED: Final[str] = "enriched_transcripts"
COLL_MULTIMODAL: Final[str] = "multimodal_segments"

# Unified chunks collection - contains chunks from all document sources
# All chunks should include a "source_type" field indicating origin (e.g., "youtube", "pdf", "html")
# and source-specific identifiers (e.g., "video_id" for YouTube, "document_id" for PDFs)
COLL_CHUNKS: Final[str] = "video_chunks"  # Note: Consider renaming to "document_chunks" in future
COLL_MEMORY_LOGS: Final[str] = "memory_logs"
COLL_VIDEO_FEEDBACK: Final[str] = "video_feedback"
COLL_CHUNK_FEEDBACK: Final[str] = "chunk_feedback"

# GraphRAG collections - LEGACY (original pipeline output)
# These collections contain data from the standard GraphRAG pipeline execution
COLL_ENTITIES: Final[str] = "entities"
COLL_RELATIONS: Final[str] = "relations"
COLL_COMMUNITIES: Final[str] = "communities"
COLL_ENTITY_MENTIONS: Final[str] = "entity_mentions"

# Source Selection collections - Pipeline input curation
COLL_INPUT_FILTERS: Final[str] = "pipeline_input_filters"  # Saved filter definitions

# GraphRAG collections - NEW OBSERVABILITY INFRASTRUCTURE
# These collections store intermediate data and transformation logs for analysis
# Achievement 0.1-0.4: Observability Infrastructure (from PLAN_GRAPHRAG-OBSERVABILITY-EXCELLENCE.md)
# They coexist with legacy collections, enabling gradual migration

# Transformation Logging (Achievement 0.1)
COLL_TRANSFORMATION_LOGS: Final[str] = (
    "transformation_logs"  # All pipeline transformations with reasoning
)

# Intermediate Data Collections (Achievement 0.2)
# Intermediate data at stage boundaries for before/after analysis
COLL_ENTITIES_RAW: Final[str] = "entities_raw"  # Extracted entities (before resolution)
COLL_ENTITIES_RESOLVED: Final[str] = (
    "entities_resolved"  # Resolved entities (after resolution, before graph)
)
COLL_RELATIONS_RAW: Final[str] = "relations_raw"  # Extracted relationships (before post-processing)
COLL_RELATIONS_FINAL: Final[str] = (
    "relations_final"  # Final relationships (after post-processing, before detection)
)
COLL_GRAPH_PRE_DETECTION: Final[str] = (
    "graph_pre_detection"  # Graph structure before community detection
)

# Quality Metrics (Achievement 0.3)
COLL_QUALITY_METRICS: Final[str] = (
    "quality_metrics"  # 23 quality metrics (merge_rate, entity_types, etc.)
)
COLL_GRAPHRAG_RUNS: Final[str] = "graphrag_runs"  # Pipeline run metadata and execution trace

# Collection grouping for organization
# Legacy collections used by standard pipeline
LEGACY_GRAPHRAG_COLLECTIONS = [
    COLL_ENTITIES,
    COLL_RELATIONS,
    COLL_COMMUNITIES,
    COLL_ENTITY_MENTIONS,
]

# New collections used by observability infrastructure
OBSERVABILITY_COLLECTIONS = [
    COLL_TRANSFORMATION_LOGS,
    COLL_ENTITIES_RAW,
    COLL_ENTITIES_RESOLVED,
    COLL_RELATIONS_RAW,
    COLL_RELATIONS_FINAL,
    COLL_GRAPH_PRE_DETECTION,
    COLL_QUALITY_METRICS,
    COLL_GRAPHRAG_RUNS,
]

# COEXISTENCE STRATEGY (Option C)
# The new observability infrastructure (Achievements 0.1-0.4) coexists with legacy collections.
# This approach:
# - Maintains backward compatibility (existing data and code unchanged)
# - Enables gradual migration (no breaking changes)
# - Allows parallel operation (both schemas functional simultaneously)
# - Supports future consolidation (when migration is complete)
#
# Migration path: Legacy → Observability → Consolidated (future)
# Current status: Coexistence (both functional, legacy as primary data source)
#
# See: documentation/Collection-Compatibility-Matrix.md for detailed mapping

# Vector index constants
VECTOR_INDEX_NAME: Final[str] = "embedding_index"
VECTOR_PATH: Final[str] = "embedding"
VECTOR_DIM: Final[int] = 1024
VECTOR_SIMILARITY: Final[str] = "cosine"


# =============================================================================
# SYSTEM DATABASE ARCHITECTURE
# =============================================================================
# Some collections are "constant" - they don't change per pipeline and should
# live in a central database (system_data) rather than being duplicated across
# pipeline-specific databases.
#
# Benefits:
# - Single source of truth for raw_videos (no duplication)
# - Centralized observability data for cross-pipeline analysis
# - Reduced storage costs and maintenance overhead

# System database name (configurable via environment)
CONSTANT_DB_NAME: Final[str] = os.getenv("SYSTEM_DB_NAME", "system_data")

# Collections that belong in the constant (system) database
# These are shared across all pipelines and should not be duplicated
CONSTANT_COLLECTIONS: Final[frozenset] = frozenset({
    "raw_videos",                # Source data - read by all pipelines
    "pipeline_input_filters",    # Saved filter definitions
    "transformation_logs",       # Pipeline transformation logs
    "pipeline_executions",       # Execution history
    "stage_metrics",             # Stage performance metrics
    "entity_resolution_log",     # Entity resolution audit trail
    "quality_metrics",           # Quality metrics from all runs
    "graphrag_runs",             # Run metadata
})


def get_db_for_collection(collection_name: str, pipeline_db: str) -> str:
    """
    Determine which database a collection belongs to.
    
    Constant collections (raw_videos, observability data) are routed to the
    system database. All other collections use the pipeline-specific database.
    
    Args:
        collection_name: Name of the collection
        pipeline_db: Current pipeline database name
        
    Returns:
        Database name to use (either CONSTANT_DB_NAME or pipeline_db)
        
    Example:
        >>> get_db_for_collection("raw_videos", "mongo_hack")
        'system_data'
        >>> get_db_for_collection("cleaned_transcripts", "mongo_hack")
        'mongo_hack'
    """
    if collection_name in CONSTANT_COLLECTIONS:
        return CONSTANT_DB_NAME
    return pipeline_db
