"""
Field Metadata Registry for UI Customization

Reference: API_DESIGN_SPECIFICATION.md Section 5.2

This supplements automatic introspection with human-friendly
descriptions and UI hints for configuration fields.
"""

from typing import Dict, Any

# Field metadata registry
FIELD_METADATA: Dict[str, Dict[str, Any]] = {
    # ============================================
    # Common Fields (BaseStageConfig)
    # ============================================
    "max": {
        "description": "Maximum number of documents to process (for testing/debugging)",
        "ui_type": "number",
        "min": 1,
        "max": 10000,
        "placeholder": "Leave empty to process all documents",
    },
    "llm": {
        "description": "Enable LLM processing for this stage",
        "ui_type": "checkbox",
    },
    "verbose": {
        "description": "Enable detailed logging output",
        "ui_type": "checkbox",
    },
    "dry_run": {
        "description": "Simulate execution without writing to database",
        "ui_type": "checkbox",
    },
    "db_name": {
        "description": "Database name (defaults to environment DB_NAME)",
        "ui_type": "text",
        "placeholder": "mongo_hack",
    },
    "read_db_name": {
        "description": "Source database for reading data (for experiments)",
        "ui_type": "text",
        "placeholder": "Same as db_name if not specified",
    },
    "write_db_name": {
        "description": "Target database for writing results (for experiments)",
        "ui_type": "text",
        "placeholder": "Same as db_name if not specified",
    },
    "read_coll": {
        "description": "Source collection for reading data",
        "ui_type": "text",
        "placeholder": "Uses stage default if not specified",
    },
    "write_coll": {
        "description": "Target collection for writing results",
        "ui_type": "text",
        "placeholder": "Uses stage default if not specified",
    },
    "concurrency": {
        "description": "Number of parallel workers for concurrent processing",
        "ui_type": "number",
        "min": 1,
        "max": 500,
        "recommended": 50,
    },
    "video_id": {
        "description": "Filter processing to a specific video ID",
        "ui_type": "text",
        "placeholder": "Leave empty to process all videos",
    },
    "upsert_existing": {
        "description": "Update existing records instead of skipping",
        "ui_type": "checkbox",
    },
    "trace_id": {
        "description": "Trace ID for linking transformations across pipeline run",
        "ui_type": "text",
        "placeholder": "Auto-generated if not specified",
    },
    # ============================================
    # LLM Settings (GraphRAG Stages)
    # ============================================
    "model_name": {
        "description": "OpenAI model to use for LLM operations",
        "ui_type": "select",
        "options": ["gpt-4o-mini", "gpt-4o", "gpt-4", "gpt-4-turbo", "gpt-3.5-turbo", "gpt-5.2", "gpt-5-mini"],
        "recommended": "gpt-4o-mini",
    },
    "temperature": {
        "description": "LLM temperature (0 = deterministic, 2 = creative)",
        "ui_type": "slider",
        "min": 0.0,
        "max": 2.0,
        "step": 0.1,
    },
    "max_tokens": {
        "description": "Maximum tokens in LLM response",
        "ui_type": "number",
        "min": 100,
        "max": 16000,
        "placeholder": "Leave empty for model default",
    },
    "llm_retries": {
        "description": "Number of retry attempts for LLM API failures",
        "ui_type": "number",
        "min": 1,
        "max": 10,
    },
    "llm_backoff_s": {
        "description": "Backoff delay between retries (seconds)",
        "ui_type": "number",
        "min": 0.5,
        "max": 30.0,
        "step": 0.5,
    },
    # ============================================
    # Graph Extraction Settings
    # ============================================
    "max_entities_per_chunk": {
        "description": "Maximum entities to extract per chunk",
        "ui_type": "number",
        "min": 1,
        "max": 100,
    },
    "max_relationships_per_chunk": {
        "description": "Maximum relationships to extract per chunk",
        "ui_type": "number",
        "min": 1,
        "max": 200,
    },
    "min_entity_confidence": {
        "description": "Minimum confidence score for extracted entities (0-1)",
        "ui_type": "slider",
        "min": 0.0,
        "max": 1.0,
        "step": 0.05,
    },
    "min_relationship_confidence": {
        "description": "Minimum confidence score for extracted relationships (0-1)",
        "ui_type": "slider",
        "min": 0.0,
        "max": 1.0,
        "step": 0.05,
    },
    "batch_size": {
        "description": "Number of items to process per batch",
        "ui_type": "number",
        "min": 1,
        "max": 500,
    },
    "extraction_timeout": {
        "description": "Timeout for extraction per chunk (seconds)",
        "ui_type": "number",
        "min": 10,
        "max": 300,
    },
    "entity_types": {
        "description": "Types of entities to extract",
        "ui_type": "multiselect",
        "options": [
            "PERSON",
            "ORGANIZATION",
            "TECHNOLOGY",
            "CONCEPT",
            "LOCATION",
            "EVENT",
            "OTHER",
        ],
    },
    # ============================================
    # Entity Resolution Settings
    # ============================================
    "similarity_threshold": {
        "description": "Minimum similarity score for entity resolution (0-1)",
        "ui_type": "slider",
        "min": 0.0,
        "max": 1.0,
        "step": 0.05,
    },
    "max_aliases_per_entity": {
        "description": "Maximum aliases to store per entity",
        "ui_type": "number",
        "min": 1,
        "max": 50,
    },
    "min_source_count": {
        "description": "Minimum source count for entity to be considered valid",
        "ui_type": "number",
        "min": 1,
        "max": 10,
    },
    "max_input_tokens_per_entity": {
        "description": "Maximum input tokens per entity (None to disable)",
        "ui_type": "number",
        "min": 1000,
        "max": 16000,
        "placeholder": "Leave empty to disable token limit",
    },
    "use_fuzzy_matching": {
        "description": "Enable fuzzy string matching for entity resolution",
        "ui_type": "checkbox",
    },
    "use_embedding_similarity": {
        "description": "Use embedding-based similarity for entity resolution",
        "ui_type": "checkbox",
    },
    "use_context_similarity": {
        "description": "Use context-based similarity for entity resolution",
        "ui_type": "checkbox",
    },
    "use_relationship_clustering": {
        "description": "Use relationship-based clustering for entity resolution",
        "ui_type": "checkbox",
    },
    "resolution_timeout": {
        "description": "Timeout for resolution per batch (seconds)",
        "ui_type": "number",
        "min": 30,
        "max": 600,
    },
    # ============================================
    # Graph Construction Settings
    # ============================================
    "max_relationships_per_entity": {
        "description": "Maximum relationships to store per entity",
        "ui_type": "number",
        "min": 10,
        "max": 500,
    },
    "calculate_centrality": {
        "description": "Calculate centrality metrics for entities",
        "ui_type": "checkbox",
    },
    "calculate_degree": {
        "description": "Calculate degree metrics for entities",
        "ui_type": "checkbox",
    },
    "calculate_clustering": {
        "description": "Calculate clustering coefficient for entities",
        "ui_type": "checkbox",
    },
    "validate_entity_existence": {
        "description": "Validate that referenced entities exist",
        "ui_type": "checkbox",
    },
    "max_relationship_distance": {
        "description": "Maximum hops for relationship validation",
        "ui_type": "number",
        "min": 1,
        "max": 10,
    },
    # ============================================
    # Community Detection Settings
    # ============================================
    "algorithm": {
        "description": "Community detection algorithm",
        "ui_type": "select",
        "options": ["louvain", "hierarchical_leiden"],
        "recommended": "louvain",
    },
    "resolution_parameter": {
        "description": "Louvain resolution parameter (higher = more communities)",
        "ui_type": "slider",
        "min": 0.1,
        "max": 3.0,
        "step": 0.1,
    },
    "max_cluster_size": {
        "description": "Maximum community size (soft limit)",
        "ui_type": "number",
        "min": 5,
        "max": 500,
    },
    "min_cluster_size": {
        "description": "Minimum community size (for filtering)",
        "ui_type": "number",
        "min": 2,
        "max": 20,
    },
    "max_iterations": {
        "description": "Maximum iterations for community detection",
        "ui_type": "number",
        "min": 10,
        "max": 1000,
    },
    "max_levels": {
        "description": "Maximum hierarchical levels (Leiden only)",
        "ui_type": "number",
        "min": 1,
        "max": 10,
    },
    "level_size_threshold": {
        "description": "Size threshold for hierarchical levels",
        "ui_type": "number",
        "min": 2,
        "max": 50,
    },
    "max_summary_length": {
        "description": "Maximum characters in community summary",
        "ui_type": "number",
        "min": 500,
        "max": 10000,
    },
    "min_summary_length": {
        "description": "Minimum characters in community summary",
        "ui_type": "number",
        "min": 50,
        "max": 1000,
    },
    "summarization_timeout": {
        "description": "Timeout for summarization per community (seconds)",
        "ui_type": "number",
        "min": 30,
        "max": 600,
    },
    "min_coherence_score": {
        "description": "Minimum coherence score for valid community (0-1)",
        "ui_type": "slider",
        "min": 0.0,
        "max": 1.0,
        "step": 0.05,
    },
    "min_entity_count": {
        "description": "Minimum entities required per community",
        "ui_type": "number",
        "min": 2,
        "max": 50,
    },
    # ============================================
    # Ingestion Stage Settings
    # ============================================
    "chunk_strategy": {
        "description": "Strategy for splitting text into chunks",
        "ui_type": "select",
        "options": ["recursive", "sentence", "paragraph", "fixed"],
        "recommended": "recursive",
    },
    "token_size": {
        "description": "Target size of chunks in tokens",
        "ui_type": "number",
        "min": 100,
        "max": 5000,
    },
    "overlap_pct": {
        "description": "Overlap percentage between chunks (0-1)",
        "ui_type": "slider",
        "min": 0.0,
        "max": 0.5,
        "step": 0.05,
    },
    "use_hybrid_embedding_text": {
        "description": "Use hybrid text for embeddings",
        "ui_type": "checkbox",
    },
    "unit_normalize_embeddings": {
        "description": "Unit normalize embedding vectors",
        "ui_type": "checkbox",
    },
    "embed_source": {
        "description": "Source field for embedding generation",
        "ui_type": "select",
        "options": ["chunk", "summary", "both"],
        "recommended": "chunk",
    },
    "emit_multi_vectors": {
        "description": "Emit multiple vectors per chunk",
        "ui_type": "checkbox",
    },
    "split_chars": {
        "description": "Characters to split on for chunking",
        "ui_type": "multiselect",
        "options": [".", "?", "!", "\\n"],
    },
    "use_llm": {
        "description": "Enable LLM for this stage",
        "ui_type": "checkbox",
    },
    # ============================================
    # Prompt Management Settings
    # ============================================
    "prompt_id": {
        "description": "Prompt template to use for LLM operations. Select from available prompts or use the default hardcoded prompt.",
        "ui_type": "prompt_selector",
        "category": "LLM Settings",
        "placeholder": "Select a prompt template...",
    },
}


def get_field_metadata(field_name: str) -> Dict[str, Any]:
    """
    Get metadata for a field.

    Args:
        field_name: Name of the configuration field

    Returns:
        Dictionary with field metadata, or empty dict if not found
    """
    return FIELD_METADATA.get(field_name, {})


def has_field_metadata(field_name: str) -> bool:
    """Check if field has custom metadata defined"""
    return field_name in FIELD_METADATA

