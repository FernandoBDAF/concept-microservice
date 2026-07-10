# GraphRAG Configuration Architecture

**Document Version:** 1.0.0  
**Last Updated:** December 9, 2025  
**Status:** Comprehensive Review & Analysis

---

## Executive Summary

This document provides a comprehensive analysis of the GraphRAG project's configuration architecture, documenting all stage configurations, their centralization status, and recommendations for maintaining a structured configuration system.

### Key Findings

✅ **Well-Structured:** Configuration system uses a centralized base class with inheritance  
✅ **Consistent:** All stages follow the same configuration pattern  
⚠️ **Partially Centralized:** Field metadata is centralized, but some stage configs are scattered  
⚠️ **Environment Variables:** Not fully documented in one place

---

## Table of Contents

1. [Configuration Architecture](#1-configuration-architecture)
2. [Base Configuration System](#2-base-configuration-system)
3. [Ingestion Pipeline Stages](#3-ingestion-pipeline-stages)
4. [GraphRAG Pipeline Stages](#4-graphrag-pipeline-stages)
5. [Configuration Flow](#5-configuration-flow)
6. [Environment Variables](#6-environment-variables)
7. [Field Metadata Registry](#7-field-metadata-registry)
8. [API Integration](#8-api-integration)
9. [Centralization Assessment](#9-centralization-assessment)
10. [Recommendations](#10-recommendations)

---

## 1. Configuration Architecture

### 1.1 Overview

The GraphRAG project uses a **dataclass-based configuration system** with the following principles:

```
BaseStageConfig (core/models/config.py)
    ↓ Inheritance
┌───────────────────┬────────────────────┐
│ Ingestion Configs │  GraphRAG Configs  │
└───────────────────┴────────────────────┘
         ↓                     ↓
   Pipeline Execution    Stage Registry
```

### 1.2 File Locations

| Component | Location | Purpose |
|-----------|----------|---------|
| **Base Config** | `core/models/config.py` | Common fields for all stages |
| **Ingestion Configs** | `business/stages/ingestion/*.py` | Stage-specific configs (inline with stages) |
| **GraphRAG Configs** | `core/config/graphrag.py` | All GraphRAG stage configs (centralized) |
| **Field Metadata** | `app/stages_api/field_metadata.py` | UI hints & descriptions |
| **Constants** | `app/stages_api/constants.py` | Pipeline groups & categories |
| **Registry** | `business/pipelines/runner.py` | Stage registry mapping |

---

## 2. Base Configuration System

### 2.1 BaseStageConfig Structure

**File:** `core/models/config.py`

```python
@dataclass
class BaseStageConfig:
    # Processing Limits
    max: Optional[int] = None          # Max documents to process (testing)
    llm: bool = False                  # Enable LLM processing
    verbose: bool = False              # Detailed logging
    dry_run: bool = False              # Simulate without writing
    
    # Database Configuration
    db_name: Optional[str] = None      # Primary database
    read_db_name: Optional[str] = None # Source DB (experiments)
    write_db_name: Optional[str] = None# Target DB (experiments)
    read_coll: Optional[str] = None    # Source collection
    write_coll: Optional[str] = None   # Target collection
    
    # Execution Parameters
    upsert_existing: bool = False      # Overwrite existing data
    video_id: Optional[str] = None     # Filter to specific video
    concurrency: Optional[int] = None  # Parallel workers
    
    # Tracing
    trace_id: Optional[str] = None     # Pipeline execution tracking
```

### 2.2 Configuration Retrieval Priority

1. **Command-line arguments** (highest priority)
2. **Environment variables**
3. **Function parameters** (default_db, default_read_coll, etc.)
4. **Dataclass defaults** (lowest priority)

### 2.3 BaseStageConfig.from_args_env() Method

All config classes implement this classmethod for loading configuration:

```python
@classmethod
def from_args_env(cls, args, env, default_db, 
                  default_read_coll=None, default_write_coll=None):
    # Load base config
    base = BaseStageConfig.from_args_env(args, env, default_db, ...)
    
    # Load stage-specific fields
    return cls(**vars(base), stage_specific_field=value)
```

---

## 3. Ingestion Pipeline Stages

### 3.1 Stage Registry

**File:** `business/pipelines/runner.py`

```python
STAGE_REGISTRY = {
    "ingest": IngestStage,
    "clean": CleanStage,
    "chunk": ChunkStage,
    "enrich": EnrichStage,
    "embed": EmbedStage,
    "redundancy": RedundancyStage,
    "trust": TrustStage,
    "compress": CompressStage,
    "backfill_transcript": BackfillTranscriptStage,
}
```

### 3.2 Ingest Stage

**File:** `business/stages/ingestion/ingest.py`

```python
@dataclass
class IngestConfig(BaseStageConfig):
    playlist_id: Optional[str] = None
    channel_id: Optional[str] = None
    video_ids: Optional[List[str]] = None
    max_ingestion: Optional[int] = None
    
    @classmethod
    def from_args_env(cls, args, env, default_db):
        base = BaseStageConfig.from_args_env(args, env, default_db)
        return cls(
            **vars(base),
            playlist_id=getattr(args, "playlist_id", None) or env.get("PLAYLIST_ID"),
            channel_id=getattr(args, "channel_id", None) or env.get("CHANNEL_ID"),
            video_ids=getattr(args, "video_ids", None),
            max_ingestion=getattr(args, "max_ingestion", None),
        )
```

**Environment Variables:**
- `PLAYLIST_ID` - YouTube playlist ID
- `CHANNEL_ID` - YouTube channel ID

### 3.3 Clean Stage

**File:** `business/stages/ingestion/clean.py`

```python
@dataclass
class CleanConfig(BaseStageConfig):
    use_llm: bool = True               # Enable LLM cleaning
    llm_retries: int = 1               # Retry attempts
    llm_backoff_s: float = 0.5         # Backoff delay
    llm_qps: Optional[float] = None    # Queries per second limit
    model_name: Optional[str] = None   # OpenAI model override
    
    @classmethod
    def from_args_env(cls, args, env, default_db):
        base = BaseStageConfig.from_args_env(args, env, default_db)
        use_llm = bool(getattr(args, "llm", True) or env.get("CLEAN_WITH_LLM") == "1")
        return cls(
            **vars(base),
            use_llm=use_llm,
            llm_retries=int(env.get("LLM_RETRIES", "1")),
            llm_backoff_s=float(env.get("LLM_BACKOFF_S", "0.5")),
            llm_qps=None,
            model_name=env.get("OPENAI_DEFAULT_MODEL"),
        )
```

**Environment Variables:**
- `CLEAN_WITH_LLM` - Enable LLM cleaning (1/0)
- `LLM_RETRIES` - Number of retry attempts
- `LLM_BACKOFF_S` - Backoff delay in seconds
- `OPENAI_DEFAULT_MODEL` - Model name (e.g., "gpt-4o-mini")

**Collections:**
- Read: `raw_videos` (COLL_RAW_VIDEOS)
- Write: `cleaned_transcripts` (COLL_CLEANED)

### 3.4 Chunk Stage

**File:** `business/stages/ingestion/chunk.py`

```python
@dataclass
class ChunkConfig(BaseStageConfig):
    strategy: str = "semantic"         # chunking strategy
    chunk_size: int = 1000             # target chunk size
    chunk_overlap: int = 100           # overlap between chunks
    min_chunk_size: int = 200          # minimum chunk size
    
    @classmethod
    def from_args_env(cls, args, env, default_db):
        base = BaseStageConfig.from_args_env(args, env, default_db)
        return cls(
            **vars(base),
            strategy=env.get("CHUNK_STRATEGY", "semantic"),
            chunk_size=int(env.get("CHUNK_SIZE", "1000")),
            chunk_overlap=int(env.get("CHUNK_OVERLAP", "100")),
            min_chunk_size=int(env.get("MIN_CHUNK_SIZE", "200")),
        )
```

**Environment Variables:**
- `CHUNK_STRATEGY` - "semantic" or "fixed"
- `CHUNK_SIZE` - Target chunk size in characters
- `CHUNK_OVERLAP` - Overlap between chunks
- `MIN_CHUNK_SIZE` - Minimum chunk size

**Collections:**
- Read: `cleaned_transcripts`
- Write: `chunks`

### 3.5 Enrich Stage

**File:** `business/stages/ingestion/enrich.py`

```python
@dataclass
class EnrichConfig(BaseStageConfig):
    use_llm: bool = True
    llm_retries: int = 2
    llm_backoff_s: float = 1.0
    model_name: Optional[str] = None
    enable_entities: bool = True
    enable_concepts: bool = True
    enable_summary: bool = True
    
    @classmethod
    def from_args_env(cls, args, env, default_db):
        base = BaseStageConfig.from_args_env(args, env, default_db)
        return cls(
            **vars(base),
            use_llm=bool(getattr(args, "llm", True)),
            llm_retries=int(env.get("LLM_RETRIES", "2")),
            llm_backoff_s=float(env.get("LLM_BACKOFF_S", "1.0")),
            model_name=env.get("OPENAI_DEFAULT_MODEL"),
            enable_entities=env.get("ENRICH_ENTITIES", "true").lower() == "true",
            enable_concepts=env.get("ENRICH_CONCEPTS", "true").lower() == "true",
            enable_summary=env.get("ENRICH_SUMMARY", "true").lower() == "true",
        )
```

**Environment Variables:**
- `ENRICH_ENTITIES` - Extract entities (true/false)
- `ENRICH_CONCEPTS` - Extract concepts (true/false)
- `ENRICH_SUMMARY` - Generate summaries (true/false)

**Collections:**
- Read: `chunks`
- Write: `chunks` (updates in place)

### 3.6 Embed Stage

**File:** `business/stages/ingestion/embed.py`

```python
@dataclass
class EmbedConfig(BaseStageConfig):
    embed_source: str = "chunk"        # "chunk" or "summary"
    use_hybrid_embedding_text: bool = True
    unit_normalize_embeddings: bool = True
    emit_multi_vectors: bool = False
    
    @classmethod
    def from_args_env(cls, args, env, default_db):
        base = BaseStageConfig.from_args_env(args, env, default_db)
        src = getattr(args, "embed_source", None) or env.get("EMBED_SOURCE", "chunk")
        return cls(
            **vars(base),
            embed_source=src if src in {"chunk", "summary"} else "chunk",
            use_hybrid_embedding_text=env.get("EMBED_USE_HYBRID", "true").lower() in {"1", "true", "yes", "on"},
            unit_normalize_embeddings=env.get("EMBED_UNIT_NORMALIZE", "true").lower() in {"1", "true", "yes", "on"},
            emit_multi_vectors=env.get("EMBED_MULTI_VECTORS", "false").lower() in {"1", "true", "yes", "on"},
        )
```

**Environment Variables:**
- `EMBED_SOURCE` - Source for embeddings ("chunk" or "summary")
- `EMBED_USE_HYBRID` - Use hybrid embedding text
- `EMBED_UNIT_NORMALIZE` - Normalize embeddings to unit length
- `EMBED_MULTI_VECTORS` - Emit multiple vectors per document

**Collections:**
- Read: `chunks`
- Write: `chunks` (updates with embedding field)

### 3.7 Redundancy Stage

**File:** `business/stages/ingestion/redundancy.py`

```python
@dataclass
class RedundancyConfig(BaseStageConfig):
    similarity_threshold: float = 0.85  # Cosine similarity threshold
    use_semantic: bool = True            # Use semantic similarity
    use_exact: bool = True               # Use exact text matching
    
    @classmethod
    def from_args_env(cls, args, env, default_db):
        base = BaseStageConfig.from_args_env(args, env, default_db)
        return cls(
            **vars(base),
            similarity_threshold=float(env.get("REDUNDANCY_THRESHOLD", "0.85")),
            use_semantic=env.get("REDUNDANCY_USE_SEMANTIC", "true").lower() == "true",
            use_exact=env.get("REDUNDANCY_USE_EXACT", "true").lower() == "true",
        )
```

**Environment Variables:**
- `REDUNDANCY_THRESHOLD` - Similarity threshold (0.0-1.0)
- `REDUNDANCY_USE_SEMANTIC` - Enable semantic matching
- `REDUNDANCY_USE_EXACT` - Enable exact text matching

**Collections:**
- Read: `chunks`
- Write: `chunks` (updates with redundancy flags)

### 3.8 Trust Stage

**File:** `business/stages/ingestion/trust.py`

```python
@dataclass
class TrustConfig(BaseStageConfig):
    use_llm: bool = True
    trust_algorithm: str = "composite"  # "composite", "llm", "rule_based"
    
    @classmethod
    def from_args_env(cls, args, env, default_db):
        base = BaseStageConfig.from_args_env(args, env, default_db)
        return cls(
            **vars(base),
            use_llm=bool(getattr(args, "llm", True)),
            trust_algorithm=env.get("TRUST_ALGORITHM", "composite"),
        )
```

**Environment Variables:**
- `TRUST_ALGORITHM` - Algorithm to use ("composite", "llm", "rule_based")

**Collections:**
- Read: `chunks`
- Write: `chunks` (updates with trust scores)

### 3.9 Compress Stage

**File:** `business/stages/ingestion/compress.py`

```python
@dataclass
class CompressConfig(BaseStageConfig):
    compression_ratio: float = 0.5      # Target compression ratio
    
    @classmethod
    def from_args_env(cls, args, env, default_db):
        base = BaseStageConfig.from_args_env(args, env, default_db)
        return cls(
            **vars(base),
            compression_ratio=float(env.get("COMPRESS_RATIO", "0.5")),
        )
```

**Environment Variables:**
- `COMPRESS_RATIO` - Target compression ratio (0.0-1.0)

### 3.10 Backfill Transcript Stage

**File:** `business/stages/ingestion/backfill_transcript.py`

```python
@dataclass
class BackfillTranscriptConfig(BaseStageConfig):
    # Uses only BaseStageConfig fields
    pass
```

---

## 4. GraphRAG Pipeline Stages

All GraphRAG stage configurations are **centralized** in one file:

**File:** `core/config/graphrag.py`

### 4.1 Graph Extraction Stage

```python
@dataclass
class GraphExtractionConfig(BaseStageConfig):
    # LLM Settings
    model_name: str = "gpt-4o-mini"
    temperature: float = 0.1
    max_tokens: Optional[int] = None
    llm_retries: int = 3
    llm_backoff_s: float = 1.0
    
    # Extraction Settings
    max_entities_per_chunk: int = 20
    max_relationships_per_chunk: int = 30
    min_entity_confidence: float = 0.3
    min_relationship_confidence: float = 0.3
    
    # Processing Settings
    batch_size: int = 50
    extraction_timeout: int = 30  # seconds per chunk
    
    # Entity types to extract
    entity_types: List[str] = None  # ["PERSON", "ORGANIZATION", ...]
```

**Environment Variables:**
- `GRAPHRAG_EXTRACTION_MODEL` - Model name
- `GRAPHRAG_EXTRACTION_TEMPERATURE` - Temperature (0.0-2.0)
- `GRAPHRAG_MAX_ENTITIES` - Max entities per chunk
- `GRAPHRAG_MAX_RELATIONSHIPS` - Max relationships per chunk
- `GRAPHRAG_MIN_ENTITY_CONFIDENCE` - Min entity confidence
- `GRAPHRAG_MIN_RELATIONSHIP_CONFIDENCE` - Min relationship confidence
- `GRAPHRAG_EXTRACTION_BATCH_SIZE` - Batch size
- `GRAPHRAG_EXTRACTION_TIMEOUT` - Timeout per chunk (seconds)

**Collections:**
- Read: `chunks`
- Write: `entities`, `relationships`

### 4.2 Entity Resolution Stage

```python
@dataclass
class EntityResolutionConfig(BaseStageConfig):
    # LLM Settings
    model_name: str = "gpt-4o-mini"
    temperature: float = 0.1
    max_tokens: Optional[int] = None
    llm_retries: int = 3
    llm_backoff_s: float = 1.0
    
    # Resolution Settings
    similarity_threshold: float = 0.85
    batch_size: int = 100
    max_comparisons_per_entity: int = 50
    
    # Resolution Strategy
    use_llm_verification: bool = True
    use_string_similarity: bool = True
    use_embedding_similarity: bool = True
    use_relationship_clustering: bool = False
    
    # Performance
    resolution_timeout: int = 60  # seconds per batch
```

**Environment Variables:**
- `GRAPHRAG_RESOLUTION_MODEL` - Model name
- `GRAPHRAG_RESOLUTION_TEMPERATURE` - Temperature
- `GRAPHRAG_SIMILARITY_THRESHOLD` - Similarity threshold (0.0-1.0)
- `GRAPHRAG_RESOLUTION_BATCH_SIZE` - Batch size
- `GRAPHRAG_MAX_COMPARISONS` - Max comparisons per entity
- `GRAPHRAG_USE_LLM_VERIFICATION` - Enable LLM verification (true/false)
- `GRAPHRAG_USE_STRING_SIMILARITY` - Enable string similarity (true/false)
- `GRAPHRAG_USE_EMBEDDING_SIMILARITY` - Enable embedding similarity (true/false)
- `GRAPHRAG_USE_RELATIONSHIP_CLUSTERING` - Enable relationship clustering (true/false)
- `GRAPHRAG_RESOLUTION_TIMEOUT` - Timeout per batch (seconds)

**Collections:**
- Read: `entities`, `relationships`
- Write: `resolved_entities`, `entity_clusters`

### 4.3 Graph Construction Stage

```python
@dataclass
class GraphConstructionConfig(BaseStageConfig):
    # Processing Settings
    batch_size: int = 200
    max_relationships_per_entity: int = 100
    
    # Graph Metrics
    calculate_centrality: bool = True
    calculate_degree: bool = True
    calculate_clustering: bool = False
    
    # Relationship Validation
    validate_entity_existence: bool = True
    min_relationship_confidence: float = 0.3
    max_relationship_distance: int = 3  # max hops
```

**Environment Variables:**
- `GRAPHRAG_CONSTRUCTION_BATCH_SIZE` - Batch size
- `GRAPHRAG_MAX_RELATIONSHIPS_PER_ENTITY` - Max relationships per entity
- `GRAPHRAG_CALCULATE_CENTRALITY` - Calculate centrality metrics (true/false)
- `GRAPHRAG_CALCULATE_DEGREE` - Calculate degree metrics (true/false)
- `GRAPHRAG_CALCULATE_CLUSTERING` - Calculate clustering coefficient (true/false)
- `GRAPHRAG_VALIDATE_ENTITY_EXISTENCE` - Validate entity existence (true/false)
- `GRAPHRAG_MIN_RELATIONSHIP_CONFIDENCE` - Min relationship confidence
- `GRAPHRAG_MAX_RELATIONSHIP_DISTANCE` - Max relationship distance (hops)

**Collections:**
- Read: `resolved_entities`, `relationships`
- Write: `knowledge_graph`

### 4.4 Community Detection Stage

```python
@dataclass
class CommunityDetectionConfig(BaseStageConfig):
    # Algorithm Selection
    algorithm: str = "louvain"  # "louvain", "leiden", "label_propagation"
    
    # Algorithm Parameters
    resolution: float = 1.0
    min_community_size: int = 3
    max_iterations: int = 100
    
    # Quality Thresholds
    min_coherence_score: float = 0.5
    min_density: float = 0.3
```

**Environment Variables:**
- `GRAPHRAG_COMMUNITY_ALGORITHM` - Algorithm ("louvain", "leiden", "label_propagation")
- `GRAPHRAG_COMMUNITY_RESOLUTION` - Resolution parameter
- `GRAPHRAG_MIN_COMMUNITY_SIZE` - Minimum community size
- `GRAPHRAG_MAX_ITERATIONS` - Maximum iterations
- `GRAPHRAG_MIN_COHERENCE_SCORE` - Minimum coherence score
- `GRAPHRAG_MIN_DENSITY` - Minimum community density

**Collections:**
- Read: `knowledge_graph`
- Write: `communities`, `community_summaries`

---

## 5. Configuration Flow

### 5.1 From UI to Pipeline Execution

```
┌─────────────────────────────────────────────────────────────┐
│ 1. StagesUI Frontend                                        │
│    - User selects pipeline & stages                         │
│    - User configures fields via UI forms                    │
│    - Sends JSON config to API                               │
└─────────────────────────────────────────────────────────────┘
                         ↓
┌─────────────────────────────────────────────────────────────┐
│ 2. Stages API (app/stages_api/)                             │
│    POST /api/v1/pipelines/execute                           │
│    {                                                         │
│      "pipeline": "ingestion",                               │
│      "stages": ["clean", "chunk"],                          │
│      "config": {                                             │
│        "clean": {"max": 10, "llm": true},                   │
│        "chunk": {"chunk_size": 1000}                        │
│      }                                                       │
│    }                                                         │
└─────────────────────────────────────────────────────────────┘
                         ↓
┌─────────────────────────────────────────────────────────────┐
│ 3. Execution Layer (app/stages_api/execution.py)            │
│    - Creates argparse.Namespace from config                 │
│    - Calls _create_pipeline_object()                        │
└─────────────────────────────────────────────────────────────┘
                         ↓
┌─────────────────────────────────────────────────────────────┐
│ 4. Pipeline Configuration (business/pipelines/)             │
│    - IngestionPipelineConfig.from_args_env(args, env, db)  │
│    - Creates individual stage configs                       │
└─────────────────────────────────────────────────────────────┘
                         ↓
┌─────────────────────────────────────────────────────────────┐
│ 5. Stage Configuration (business/stages/)                   │
│    - CleanConfig.from_args_env(args, env, db)              │
│    - Merges: args → env → defaults                          │
└─────────────────────────────────────────────────────────────┘
                         ↓
┌─────────────────────────────────────────────────────────────┐
│ 6. Stage Execution                                           │
│    - CleanStage(config)                                     │
│    - Executes with resolved configuration                   │
└─────────────────────────────────────────────────────────────┘
```

### 5.2 Configuration Application in execution.py

**File:** `app/stages_api/execution.py`

```python
def _apply_config_to_args_env(
    config: Dict[str, Dict[str, Any]],
    args: argparse.Namespace,
    env: Dict[str, str],
):
    """
    Apply stage configuration to args and environment.
    
    For each stage config:
      config = {"clean": {"max": 10, "llm": true}}
    
    Sets:
      args.max = 10
      args.llm = true
      env["CLEAN_MAX"] = "10"
      env["CLEAN_LLM"] = "True"
    """
    for stage_name, stage_config in config.items():
        for key, value in stage_config.items():
            # Set on args namespace
            setattr(args, key, value)
            
            # Also set in env for stages that read from env
            env_key = f"{stage_name.upper()}_{key.upper()}"
            if value is not None:
                env[env_key] = str(value)
```

---

## 6. Environment Variables

### 6.1 Complete Environment Variables Reference

#### Global Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `DB_NAME` | `mongo_hack` | Default database name |
| `MONGODB_DB` | - | Alternative to DB_NAME (used by Stages API) |
| `MONGODB_URI` | `mongodb://localhost:27017` | MongoDB connection string |
| `READ_DB_NAME` | - | Source database for experiments |
| `WRITE_DB_NAME` | - | Target database for experiments |
| `READ_COLL` | - | Source collection override |
| `WRITE_COLL` | - | Target collection override |

#### LLM Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `OPENAI_DEFAULT_MODEL` | `gpt-4o-mini` | Default OpenAI model |
| `OPENAI_API_KEY` | - | OpenAI API key (required) |
| `LLM_RETRIES` | `1` | Default retry attempts |
| `LLM_BACKOFF_S` | `0.5` | Default backoff delay (seconds) |

#### Ingestion Stages

| Variable | Stage | Default | Description |
|----------|-------|---------|-------------|
| `PLAYLIST_ID` | ingest | - | YouTube playlist ID |
| `CHANNEL_ID` | ingest | - | YouTube channel ID |
| `CLEAN_WITH_LLM` | clean | `1` | Enable LLM cleaning |
| `CHUNK_STRATEGY` | chunk | `semantic` | Chunking strategy |
| `CHUNK_SIZE` | chunk | `1000` | Target chunk size |
| `CHUNK_OVERLAP` | chunk | `100` | Chunk overlap |
| `MIN_CHUNK_SIZE` | chunk | `200` | Minimum chunk size |
| `ENRICH_ENTITIES` | enrich | `true` | Extract entities |
| `ENRICH_CONCEPTS` | enrich | `true` | Extract concepts |
| `ENRICH_SUMMARY` | enrich | `true` | Generate summaries |
| `EMBED_SOURCE` | embed | `chunk` | Embedding source |
| `EMBED_USE_HYBRID` | embed | `true` | Use hybrid embeddings |
| `EMBED_UNIT_NORMALIZE` | embed | `true` | Normalize embeddings |
| `EMBED_MULTI_VECTORS` | embed | `false` | Emit multiple vectors |
| `REDUNDANCY_THRESHOLD` | redundancy | `0.85` | Similarity threshold |
| `REDUNDANCY_USE_SEMANTIC` | redundancy | `true` | Semantic matching |
| `REDUNDANCY_USE_EXACT` | redundancy | `true` | Exact text matching |
| `TRUST_ALGORITHM` | trust | `composite` | Trust algorithm |
| `COMPRESS_RATIO` | compress | `0.5` | Compression ratio |

#### GraphRAG Stages

| Variable | Stage | Default | Description |
|----------|-------|---------|-------------|
| `GRAPHRAG_EXTRACTION_MODEL` | graph_extraction | `gpt-4o-mini` | Model for extraction |
| `GRAPHRAG_EXTRACTION_TEMPERATURE` | graph_extraction | `0.1` | LLM temperature |
| `GRAPHRAG_MAX_ENTITIES` | graph_extraction | `20` | Max entities per chunk |
| `GRAPHRAG_MAX_RELATIONSHIPS` | graph_extraction | `30` | Max relationships per chunk |
| `GRAPHRAG_MIN_ENTITY_CONFIDENCE` | graph_extraction | `0.3` | Min entity confidence |
| `GRAPHRAG_MIN_RELATIONSHIP_CONFIDENCE` | graph_extraction | `0.3` | Min relationship confidence |
| `GRAPHRAG_EXTRACTION_BATCH_SIZE` | graph_extraction | `50` | Batch size |
| `GRAPHRAG_EXTRACTION_TIMEOUT` | graph_extraction | `30` | Timeout (seconds) |
| `GRAPHRAG_RESOLUTION_MODEL` | entity_resolution | `gpt-4o-mini` | Model for resolution |
| `GRAPHRAG_SIMILARITY_THRESHOLD` | entity_resolution | `0.85` | Similarity threshold |
| `GRAPHRAG_RESOLUTION_BATCH_SIZE` | entity_resolution | `100` | Batch size |
| `GRAPHRAG_MAX_COMPARISONS` | entity_resolution | `50` | Max comparisons |
| `GRAPHRAG_USE_LLM_VERIFICATION` | entity_resolution | `true` | LLM verification |
| `GRAPHRAG_CONSTRUCTION_BATCH_SIZE` | graph_construction | `200` | Batch size |
| `GRAPHRAG_MAX_RELATIONSHIPS_PER_ENTITY` | graph_construction | `100` | Max relationships |
| `GRAPHRAG_CALCULATE_CENTRALITY` | graph_construction | `true` | Calculate centrality |
| `GRAPHRAG_CALCULATE_DEGREE` | graph_construction | `true` | Calculate degree |
| `GRAPHRAG_COMMUNITY_ALGORITHM` | community_detection | `louvain` | Detection algorithm |
| `GRAPHRAG_COMMUNITY_RESOLUTION` | community_detection | `1.0` | Resolution parameter |
| `GRAPHRAG_MIN_COMMUNITY_SIZE` | community_detection | `3` | Min community size |

---

## 7. Field Metadata Registry

### 7.1 Purpose

The field metadata registry provides **UI-specific information** that supplements the automatic schema introspection.

**File:** `app/stages_api/field_metadata.py`

### 7.2 Metadata Structure

```python
FIELD_METADATA: Dict[str, Dict[str, Any]] = {
    "field_name": {
        "description": "Human-readable description",
        "ui_type": "text|number|checkbox|slider|select|multiselect",
        "min": 0,                    # For number/slider
        "max": 100,                  # For number/slider
        "step": 0.1,                 # For slider
        "options": ["opt1", "opt2"], # For select/multiselect
        "placeholder": "hint text",  # For text inputs
        "category": "Category Name", # Override automatic categorization
        "recommended": "value",      # Recommended value
    }
}
```

### 7.3 Category Inference

**File:** `app/stages_api/constants.py`

```python
CATEGORY_PATTERNS = {
    "LLM Settings": ["model", "temperature", "token", "llm", "prompt"],
    "Processing": ["concurrency", "batch", "timeout", "max", "chunk"],
    "Quality Thresholds": ["threshold", "confidence", "score", "coherence"],
    "Algorithm Parameters": ["algorithm", "resolution", "cluster", "strategy"],
    "Database Configuration": ["db", "collection", "coll"],
}
```

Fields are automatically categorized based on their name matching these patterns.

---

## 8. API Integration

### 8.1 Available Endpoints

| Endpoint | Purpose | Response |
|----------|---------|----------|
| `GET /stages` | List all available stages | Stage registry |
| `GET /stages/{pipeline}` | Stages for specific pipeline | Filtered stage list |
| `GET /stages/{stage}/config` | Get config schema | Field definitions |
| `GET /stages/{stage}/defaults` | Get default values | Config with defaults |
| `POST /stages/{stage}/validate` | Validate stage config | Validation result |
| `POST /pipelines/validate` | Validate full pipeline | Validation result |
| `POST /pipelines/execute` | Execute pipeline | Pipeline ID + status |

### 8.2 Configuration Schema Endpoint

**GET `/api/v1/stages/{stage_name}/config`**

Returns complete schema for building UI forms:

```json
{
  "stage_name": "clean",
  "config_class": "CleanConfig",
  "description": "Clean transcripts into standardized text",
  "fields": [
    {
      "name": "max",
      "type": "integer",
      "python_type": "Optional[int]",
      "default": null,
      "required": false,
      "optional": true,
      "description": "Maximum number of documents to process",
      "category": "Common Fields",
      "ui_type": "number",
      "is_inherited": true,
      "min": 1,
      "max": 10000,
      "placeholder": "Leave empty to process all"
    },
    {
      "name": "llm",
      "type": "boolean",
      "python_type": "bool",
      "default": false,
      "required": false,
      "optional": false,
      "description": "Enable LLM processing",
      "category": "LLM Settings",
      "ui_type": "checkbox",
      "is_inherited": true
    },
    {
      "name": "use_llm",
      "type": "boolean",
      "python_type": "bool",
      "default": true,
      "required": false,
      "optional": false,
      "description": "Enable LLM cleaning for this stage",
      "category": "LLM Settings",
      "ui_type": "checkbox",
      "is_inherited": false
    }
  ],
  "categories": [
    {
      "name": "Common Fields",
      "fields": ["max", "verbose", "dry_run"],
      "field_count": 3
    },
    {
      "name": "LLM Settings",
      "fields": ["llm", "use_llm", "model_name"],
      "field_count": 3
    }
  ],
  "field_count": 12
}
```

---

## 9. Centralization Assessment

### 9.1 Centralization Score

| Component | Status | Score | Notes |
|-----------|--------|-------|-------|
| **BaseStageConfig** | ✅ Centralized | 10/10 | Single source in `core/models/config.py` |
| **GraphRAG Configs** | ✅ Centralized | 10/10 | All in `core/config/graphrag.py` |
| **Ingestion Configs** | ⚠️ Distributed | 6/10 | Each config with its stage file |
| **Field Metadata** | ✅ Centralized | 10/10 | Single registry in `app/stages_api/` |
| **Constants** | ✅ Centralized | 10/10 | Single file for API constants |
| **Stage Registry** | ✅ Centralized | 10/10 | Single registry in `runner.py` |
| **Env Var Docs** | ❌ Missing | 2/10 | Not documented centrally |

**Overall Score: 8.3/10 (Very Good)**

### 9.2 Strengths

1. **Consistent Inheritance:** All configs inherit from `BaseStageConfig`
2. **GraphRAG Centralized:** All GraphRAG configs in one file
3. **API Layer:** Well-structured metadata extraction
4. **Type Safety:** Full dataclass and type hints
5. **Field Metadata:** Centralized UI customization

### 9.3 Weaknesses

1. **Ingestion Configs Scattered:** Each config defined with its stage
2. **No Central Env Var Reference:** Variables spread across files
3. **Collection Names:** Defined in `core/config/paths.py` but not documented
4. **Config Validation:** Not centralized (each stage validates independently)

---

## 10. Recommendations

### 10.1 Short-Term (Already Good)

✅ **No critical changes needed** - The current architecture is solid and working well.

**Optional Improvements:**

1. **Document Environment Variables:**
   - Create `/GraphRAG/ENV_VARIABLES.md` with complete reference
   - Link from main README

2. **Add Config Examples:**
   - Create `/GraphRAG/configs/examples/` with sample configs
   - Document common use cases

### 10.2 Long-Term (If Refactoring)

**Consider consolidating ingestion configs:**

Create `/GraphRAG/core/config/ingestion.py` and move all ingestion configs there (similar to GraphRAG):

```python
# Current: Scattered
business/stages/ingestion/clean.py    → CleanConfig
business/stages/ingestion/chunk.py    → ChunkConfig
business/stages/ingestion/embed.py    → EmbedConfig

# Future: Consolidated
core/config/ingestion.py → All ingestion configs
```

**Benefits:**
- All configs in two files: `graphrag.py` + `ingestion.py`
- Easier to review and maintain
- Consistent with GraphRAG pattern

**Trade-offs:**
- Breaks co-location of config with stage code
- More imports needed in stage files
- Migration effort

**Recommendation:** Only do this if doing a major refactor. Current structure works fine.

### 10.3 Documentation Improvements

1. **This Document:** Keep updated as single source of truth
2. **Environment Variable Reference:** Create dedicated `.env.reference` file
3. **Configuration Examples:** Add to `/configs/examples/`
4. **API Documentation:** Link to this document from API specs

---

## Appendix A: File Reference Matrix

| Config Class | File Location | Size | Environment Prefix |
|--------------|---------------|------|-------------------|
| `BaseStageConfig` | `core/models/config.py` | 77 lines | - |
| `IngestConfig` | `business/stages/ingestion/ingest.py` | ~50 lines | `INGEST_` |
| `CleanConfig` | `business/stages/ingestion/clean.py` | ~20 lines | `CLEAN_` |
| `ChunkConfig` | `business/stages/ingestion/chunk.py` | ~20 lines | `CHUNK_` |
| `EnrichConfig` | `business/stages/ingestion/enrich.py` | ~30 lines | `ENRICH_` |
| `EmbedConfig` | `business/stages/ingestion/embed.py` | ~45 lines | `EMBED_` |
| `RedundancyConfig` | `business/stages/ingestion/redundancy.py` | ~15 lines | `REDUNDANCY_` |
| `TrustConfig` | `business/stages/ingestion/trust.py` | ~15 lines | `TRUST_` |
| `CompressConfig` | `business/stages/ingestion/compress.py` | ~10 lines | `COMPRESS_` |
| `BackfillTranscriptConfig` | `business/stages/ingestion/backfill_transcript.py` | ~5 lines | `BACKFILL_` |
| `GraphExtractionConfig` | `core/config/graphrag.py` | ~75 lines | `GRAPHRAG_EXTRACTION_` |
| `EntityResolutionConfig` | `core/config/graphrag.py` | ~80 lines | `GRAPHRAG_RESOLUTION_` |
| `GraphConstructionConfig` | `core/config/graphrag.py` | ~50 lines | `GRAPHRAG_CONSTRUCTION_` |
| `CommunityDetectionConfig` | `core/config/graphrag.py` | ~40 lines | `GRAPHRAG_COMMUNITY_` |

---

## Appendix B: Collection Name Reference

**File:** `core/config/paths.py`

| Constant | Value | Used By |
|----------|-------|---------|
| `COLL_RAW_VIDEOS` | `raw_videos` | ingest (write), clean (read) |
| `COLL_CLEANED` | `cleaned_transcripts` | clean (write), chunk (read) |
| `COLL_CHUNKS` | `chunks` | chunk (write), enrich/embed/redundancy/trust (read/write) |
| `COLL_ENTITIES` | `entities` | graph_extraction (write), entity_resolution (read) |
| `COLL_RELATIONSHIPS` | `relationships` | graph_extraction (write), graph_construction (read) |
| `COLL_RESOLVED_ENTITIES` | `resolved_entities` | entity_resolution (write), graph_construction (read) |
| `COLL_KNOWLEDGE_GRAPH` | `knowledge_graph` | graph_construction (write), community_detection (read) |
| `COLL_COMMUNITIES` | `communities` | community_detection (write) |

---

## Document Maintenance

**Last Review:** December 9, 2025  
**Next Review:** When adding new stages or configuration fields  
**Maintained By:** Development Team

**Change Log:**
- 2025-12-09: Initial comprehensive review (v1.0.0)

---

**END OF DOCUMENT**

