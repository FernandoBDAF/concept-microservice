"""
Constants for Stages API

Reference: API_DESIGN_SPECIFICATION.md Section 3.2

Defines pipeline groupings, category patterns, and UI type mappings
based on the actual STAGE_REGISTRY from business/pipelines/runner.py
"""

# Pipeline stage groupings (matches STAGE_REGISTRY in business/pipelines/runner.py)
PIPELINE_GROUPS = {
    "ingestion": [
        "ingest",
        "clean",
        "chunk",
        "enrich",
        "embed",
        "redundancy",
        "trust",
        "compress",
        "backfill_transcript",
    ],
    "graphrag": [
        "graph_extraction",
        "entity_resolution",
        "graph_construction",
        "community_detection",
    ],
}

# Pipeline display names and descriptions
PIPELINE_INFO = {
    "ingestion": {
        "name": "Ingestion Pipeline",
        "description": "Process raw video data through cleaning, chunking, enrichment, and embedding",
    },
    "graphrag": {
        "name": "GraphRAG Pipeline",
        "description": "Build knowledge graph from processed chunks with entity resolution and community detection",
    },
}

# Field category patterns for automatic inference
CATEGORY_PATTERNS = {
    "LLM Settings": ["model", "temperature", "token", "llm", "prompt"],
    "Processing": ["concurrency", "batch", "timeout", "max", "chunk"],
    "Quality Thresholds": ["threshold", "confidence", "score", "coherence"],
    "Algorithm Parameters": ["algorithm", "resolution", "cluster", "strategy"],
    "Database Configuration": ["db", "collection", "coll"],
}

# UI type inference rules
UI_TYPE_MAP = {
    "bool": "checkbox",
    "int": "number",
    "float": "slider",
    "str": "text",
    "list": "multiselect",
}

