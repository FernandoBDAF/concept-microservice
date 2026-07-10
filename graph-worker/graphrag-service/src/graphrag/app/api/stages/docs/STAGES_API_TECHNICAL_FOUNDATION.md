# Stages API Technical Foundation Document

**Created:** December 9, 2025  
**Purpose:** Comprehensive technical review and design foundation for building a Stages Configuration API and UI

---

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [System Architecture Overview](#system-architecture-overview)
3. [Pipeline System Analysis](#pipeline-system-analysis)
4. [Stage Configuration System](#stage-configuration-system)
5. [Existing API Patterns](#existing-api-patterns)
6. [Stage Registry and Metadata](#stage-registry-and-metadata)
7. [Configuration Schema Design](#configuration-schema-design)
8. [API Endpoint Design](#api-endpoint-design)
9. [UI Requirements and Data Flow](#ui-requirements-and-data-flow)
10. [Implementation Roadmap](#implementation-roadmap)
11. [Technical Considerations](#technical-considerations)

---

## 1. Executive Summary

This document provides a comprehensive technical analysis of the GraphRAG codebase to support the development of a **Stages Configuration API** and accompanying **minimalist UI**. The goal is to enable users to:

1. **Discover available stages** (both Ingestion and GraphRAG pipelines)
2. **View stage configurations** with all available parameters
3. **Configure stage parameters** through a web form
4. **Execute pipelines** with custom stage configurations
5. **Monitor pipeline execution** in real-time

### Key Findings

- **Two Pipeline Types:** Ingestion (7 stages) and GraphRAG (4 stages)
- **13 Total Stages** with varying configuration complexity
- **Consistent Configuration Pattern** using dataclasses inheriting from `BaseStageConfig`
- **Stage Registry System** (`STAGE_REGISTRY`) for dynamic stage discovery
- **Existing API Infrastructure** in `app/api/` provides patterns to follow
- **Configuration Flexibility** through args/env/file with clear precedence

---

## 2. System Architecture Overview

### 2.1 High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      User Interfaces                         │
├─────────────────┬───────────────────┬───────────────────────┤
│  CLI (app/cli/) │  API (app/api/)   │  NEW: Stages UI       │
└────────┬────────┴──────────┬────────┴───────────┬───────────┘
         │                   │                    │
         └─────────┬─────────┴────────────────────┘
                   ▼
         ┌─────────────────────────────────────────┐
         │    Pipeline Layer (business/pipelines/)  │
         │  - GraphRAGPipeline                      │
         │  - IngestionPipeline                     │
         │  - PipelineRunner                        │
         └─────────────┬───────────────────────────┘
                       ▼
         ┌─────────────────────────────────────────┐
         │     Stage Layer (business/stages/)       │
         │  - Ingestion Stages (7)                  │
         │  - GraphRAG Stages (4)                   │
         │  - BaseStage (common interface)          │
         └─────────────┬───────────────────────────┘
                       ▼
         ┌─────────────────────────────────────────┐
         │  Configuration Layer (core/config/)      │
         │  - BaseStageConfig                       │
         │  - Stage-specific configs                │
         │  - Environment/CLI integration           │
         └─────────────┬───────────────────────────┘
                       ▼
         ┌─────────────────────────────────────────┐
         │    Data Layer (MongoDB)                  │
         │  - Raw videos, chunks, entities, etc.    │
         └─────────────────────────────────────────┘
```

### 2.2 Key Components

| Component | Location | Responsibility |
|-----------|----------|----------------|
| **Pipelines** | `business/pipelines/` | Orchestrate stage execution, manage dependencies |
| **Stages** | `business/stages/` | Individual processing units (clean, extract, etc.) |
| **Configuration** | `core/config/` | Centralized config management with dataclasses |
| **Stage Registry** | `business/pipelines/runner.py` | Maps stage names to classes |
| **API** | `app/api/` | REST endpoints for external access |
| **CLI** | `app/cli/` | Command-line interface for pipeline execution |

---

## 3. Pipeline System Analysis

### 3.1 Pipeline Types

#### Ingestion Pipeline
**Purpose:** Process raw video data through cleaning, chunking, enrichment, and embedding  
**Location:** `business/pipelines/ingestion.py`  
**Stages (7):**
1. **ingest** - Ingest raw video data from YouTube
2. **clean** - Clean transcripts using LLM
3. **chunk** - Split transcripts into semantic chunks
4. **enrich** - Extract entities and concepts from chunks
5. **embed** - Generate vector embeddings
6. **redundancy** - Detect and resolve redundant content
7. **trust** - Calculate trust scores

**Stage Order:** `ingest → clean → chunk → enrich → embed → redundancy → trust`

#### GraphRAG Pipeline
**Purpose:** Build knowledge graph from processed chunks  
**Location:** `business/pipelines/graphrag.py`  
**Stages (4):**
1. **graph_extraction** - Extract entities and relationships from chunks
2. **entity_resolution** - Resolve and merge duplicate entities
3. **graph_construction** - Build graph structure
4. **community_detection** - Detect communities using Louvain/Leiden

**Stage Order:** `graph_extraction → entity_resolution → graph_construction → community_detection`

### 3.2 Pipeline Runner Architecture

**File:** `business/pipelines/runner.py`

```python
class PipelineRunner:
    def __init__(
        self,
        specs: List[StageSpec],
        stop_on_error: bool = True,
    ):
        """
        specs: List of StageSpec(stage, config) objects
        stop_on_error: Whether to halt on stage failure
        """
        
    def run(self, pipeline_type: str) -> int:
        """
        Execute all stages in sequence
        Returns: 0 for success, non-zero for failure
        """
```

**Key Features:**
- **Stage Registry:** `STAGE_REGISTRY` maps stage names to classes
- **StageSpec:** Encapsulates stage + config
- **Error Handling:** Comprehensive error tracking with metrics
- **Metrics:** Prometheus-compatible metrics for monitoring

### 3.3 Stage Dependencies (GraphRAG)

```python
STAGE_DEPENDENCIES = {
    "graph_extraction": [],                    # No dependencies
    "entity_resolution": ["graph_extraction"], # Depends on extraction
    "graph_construction": ["entity_resolution"], # Depends on resolution
    "community_detection": ["graph_construction"], # Depends on construction
}
```

**Implications for API:**
- Must validate dependency chains
- Support partial pipeline runs
- Auto-include missing dependencies (optional)

---

## 4. Stage Configuration System

### 4.1 Configuration Hierarchy

```
BaseStageConfig (core/models/config.py)
    ↓
├── IngestConfig
├── CleanConfig
├── ChunkConfig
├── EnrichConfig
├── EmbedConfig
├── RedundancyConfig
├── TrustConfig
├── GraphExtractionConfig
├── EntityResolutionConfig
├── GraphConstructionConfig
└── CommunityDetectionConfig
```

### 4.2 BaseStageConfig Structure

**File:** `core/models/config.py`

```python
@dataclass
class BaseStageConfig:
    # Processing limits
    max: Optional[int] = None
    llm: bool = False
    verbose: bool = False
    dry_run: bool = False
    
    # Database configuration
    db_name: Optional[str] = None
    read_db_name: Optional[str] = None
    write_db_name: Optional[str] = None
    read_coll: Optional[str] = None
    write_coll: Optional[str] = None
    
    # Execution parameters
    upsert_existing: bool = False
    video_id: Optional[str] = None
    concurrency: Optional[int] = None
    
    # Tracing
    trace_id: Optional[str] = None
```

**Common Fields (All Stages):**
- `max` - Maximum documents to process (for testing)
- `llm` - Enable LLM processing
- `verbose` - Detailed logging
- `dry_run` - Simulate without writing
- `db_name` - Database name
- `read_db_name` - Source database (for experiments)
- `write_db_name` - Target database (for experiments)
- `concurrency` - Parallel workers
- `video_id` - Filter to specific video

### 4.3 Stage-Specific Configurations

#### GraphExtractionConfig

**File:** `core/config/graphrag.py`

```python
@dataclass
class GraphExtractionConfig(BaseStageConfig):
    # LLM settings
    model_name: str = "gpt-4o-mini"
    temperature: float = 0.1
    max_tokens: Optional[int] = None
    llm_retries: int = 3
    llm_backoff_s: float = 1.0
    
    # Extraction limits
    max_entities_per_chunk: int = 20
    max_relationships_per_chunk: int = 30
    min_entity_confidence: float = 0.3
    min_relationship_confidence: float = 0.3
    
    # Processing
    batch_size: int = 50
    extraction_timeout: int = 30  # seconds
    
    # Entity types
    entity_types: List[str] = ["PERSON", "ORGANIZATION", "TECHNOLOGY", ...]
```

**Configuration Categories:**
1. **LLM Parameters** - Model, temperature, retries
2. **Extraction Limits** - Max entities/relationships per chunk
3. **Quality Thresholds** - Minimum confidence scores
4. **Performance** - Batch size, timeouts, concurrency

#### EntityResolutionConfig

```python
@dataclass
class EntityResolutionConfig(BaseStageConfig):
    # LLM settings
    model_name: str = "gpt-4o-mini"
    temperature: float = 0.1
    
    # Resolution parameters
    similarity_threshold: float = 0.85
    max_aliases_per_entity: int = 10
    min_source_count: int = 1
    max_input_tokens_per_entity: Optional[int] = None
    
    # Processing
    batch_size: int = 100
    resolution_timeout: int = 60
    
    # Strategies
    use_fuzzy_matching: bool = True
    use_embedding_similarity: bool = True
    use_context_similarity: bool = True
    use_relationship_clustering: bool = True
```

#### CommunityDetectionConfig

```python
@dataclass
class CommunityDetectionConfig(BaseStageConfig):
    # LLM for summarization
    model_name: str = "gpt-4o-mini"
    temperature: float = 0.2
    
    # Algorithm parameters
    algorithm: str = "louvain"  # or "hierarchical_leiden"
    max_cluster_size: int = 50
    min_cluster_size: int = 2
    resolution_parameter: float = 1.0  # Louvain resolution
    max_iterations: int = 100
    
    # Hierarchical (leiden only)
    max_levels: int = 3
    level_size_threshold: int = 5
    
    # Summarization
    max_summary_length: int = 2000
    min_summary_length: int = 100
    summarization_timeout: int = 120
    
    # Quality
    min_coherence_score: float = 0.6
    min_entity_count: int = 2
```

### 4.4 Configuration Loading Pattern

**Priority Order (Highest to Lowest):**
1. **CLI Arguments** - `--read-db-name`, `--concurrency`, etc.
2. **Configuration File** - JSON file via `--config path/to/config.json`
3. **Environment Variables** - `GRAPHRAG_MODEL`, `GRAPHRAG_TEMPERATURE`, etc.
4. **Defaults** - Hardcoded in dataclass definitions

**Example: Loading from Args/Env**

```python
@classmethod
def from_args_env(cls, args, env, default_db):
    """Create config from CLI args and environment"""
    base = BaseStageConfig.from_args_env(
        args, env, default_db,
        default_read_coll=COLL_CHUNKS,
        default_write_coll=COLL_CHUNKS,
    )
    
    model_name = env.get("GRAPHRAG_MODEL") or "gpt-4o-mini"
    temperature = float(env.get("GRAPHRAG_TEMPERATURE", "0.1"))
    # ... more parameters
    
    return cls(**vars(base), model_name=model_name, temperature=temperature)
```

---

## 5. Existing API Patterns

### 5.1 Current APIs

**Location:** `app/api/`

| API File | Purpose |
|----------|---------|
| `pipeline_control.py` | Start/stop/status for pipelines |
| `pipeline_progress.py` | Track pipeline execution progress |
| `pipeline_stats.py` | Pipeline statistics and metrics |
| `entities.py` | Query entities from knowledge graph |
| `relationships.py` | Query relationships |
| `communities.py` | Query community data |
| `quality_metrics.py` | Quality metrics for GraphRAG |
| `performance_metrics.py` | Performance monitoring |

### 5.2 Pipeline Control API (Reference Implementation)

**File:** `app/api/pipeline_control.py`

**Key Functions:**
```python
def start_pipeline(config_dict: Dict, pipeline_id: str) -> Dict:
    """Start a new pipeline with given configuration"""
    
def get_pipeline_status(pipeline_id: str) -> Optional[Dict]:
    """Get current status of a running/completed pipeline"""
    
def pause_pipeline(pipeline_id: str) -> Dict:
    """Pause a running pipeline (not fully implemented)"""
    
def cancel_pipeline(pipeline_id: str) -> Dict:
    """Cancel a running pipeline"""
```

**Configuration Structure (Example):**
```json
{
  "experiment_id": "test_run_123",
  "read_db": "mongo_hack",
  "write_db": "mongo_hack",
  "concurrency": 300,
  "extraction": {
    "model_name": "gpt-4o-mini",
    "temperature": 0.1,
    "max_entities_per_chunk": 20
  },
  "resolution": {
    "similarity_threshold": 0.85,
    "use_fuzzy_matching": true
  },
  "detection": {
    "algorithm": "louvain",
    "resolution": 1.0,
    "min_cluster_size": 2
  }
}
```

### 5.3 API Architecture Pattern

All existing APIs follow a similar pattern:
1. **Function-based** (not class-based)
2. **Direct MongoDB access** via `get_mongo_client()`
3. **Error handling** with try/except blocks
4. **JSON responses** with consistent structure
5. **No web framework** - designed for HTTP server integration

---

## 6. Stage Registry and Metadata

### 6.1 Stage Registry

**File:** `business/pipelines/runner.py`

```python
STAGE_REGISTRY: Dict[str, StageClass] = {
    # Ingestion stages
    "ingest": IngestStage,
    "clean": CleanStage,
    "chunk": ChunkStage,
    "enrich": EnrichStage,
    "embed": EmbedStage,
    "redundancy": RedundancyStage,
    "trust": TrustStage,
    "compress": CompressStage,
    "backfill_transcript": BackfillTranscriptStage,
    
    # GraphRAG stages
    "graph_extraction": GraphExtractionStage,
    "entity_resolution": EntityResolutionStage,
    "graph_construction": GraphConstructionStage,
    "community_detection": CommunityDetectionStage,
}
```

### 6.2 Stage Metadata (Common Properties)

Each stage class has:
```python
class SomeStage(BaseStage):
    name = "stage_name"           # Registry key
    description = "What it does"  # Human-readable description
    ConfigCls = SomeStageConfig   # Associated config class
```

### 6.3 Dynamic Stage Discovery

**Proposed API Endpoint:** `GET /stages`

**Response Structure:**
```json
{
  "ingestion": [
    {
      "name": "ingest",
      "description": "Ingest raw video data from YouTube",
      "config_class": "IngestConfig",
      "dependencies": []
    },
    {
      "name": "clean",
      "description": "Clean transcripts using LLM",
      "config_class": "CleanConfig",
      "dependencies": ["ingest"]
    },
    ...
  ],
  "graphrag": [
    {
      "name": "graph_extraction",
      "description": "Extract entities and relationships from chunks",
      "config_class": "GraphExtractionConfig",
      "dependencies": []
    },
    ...
  ]
}
```

---

## 7. Configuration Schema Design

### 7.1 Metadata Extraction via Introspection

**Challenge:** Dataclass fields don't include descriptions  
**Solution:** Use field annotations + docstrings

**Proposed Approach:**

```python
from dataclasses import fields
from typing import get_type_hints

def get_config_schema(config_class):
    """Extract configuration schema from dataclass"""
    schema = {
        "fields": [],
        "description": config_class.__doc__,
        "class_name": config_class.__name__,
    }
    
    for field in fields(config_class):
        field_info = {
            "name": field.name,
            "type": str(field.type),
            "default": field.default,
            "required": field.default == field.default_factory,
        }
        schema["fields"].append(field_info)
    
    return schema
```

### 7.2 Enhanced Configuration Metadata

**Option 1: Add Metadata to Config Classes**

```python
@dataclass
class GraphExtractionConfig(BaseStageConfig):
    """Configuration for graph extraction stage"""
    
    # LLM settings
    model_name: str = field(
        default="gpt-4o-mini",
        metadata={
            "description": "OpenAI model name",
            "category": "LLM",
            "ui_type": "select",
            "options": ["gpt-4o-mini", "gpt-4", "gpt-3.5-turbo"]
        }
    )
    
    temperature: float = field(
        default=0.1,
        metadata={
            "description": "LLM temperature (0-2)",
            "category": "LLM",
            "ui_type": "slider",
            "min": 0.0,
            "max": 2.0,
            "step": 0.1
        }
    )
```

**Option 2: Separate Metadata Registry**

```python
# In stages-api/metadata.py
FIELD_METADATA = {
    "GraphExtractionConfig": {
        "model_name": {
            "description": "OpenAI model name",
            "category": "LLM",
            "ui_type": "select",
            "options": ["gpt-4o-mini", "gpt-4", "gpt-3.5-turbo"]
        },
        "temperature": {
            "description": "LLM temperature (0-2)",
            "category": "LLM",
            "ui_type": "slider",
            "min": 0.0, "max": 2.0, "step": 0.1
        },
        # ...
    }
}
```

### 7.3 Field Categories

Organize fields into logical groups for UI:

1. **Processing Limits**
   - `max`, `batch_size`, `concurrency`

2. **LLM Settings**
   - `model_name`, `temperature`, `max_tokens`, `llm_retries`

3. **Database Configuration**
   - `db_name`, `read_db_name`, `write_db_name`, `read_coll`, `write_coll`

4. **Quality Thresholds**
   - `min_entity_confidence`, `similarity_threshold`, `min_coherence_score`

5. **Algorithm Parameters**
   - `algorithm`, `resolution_parameter`, `chunk_strategy`

6. **Execution Flags**
   - `verbose`, `dry_run`, `upsert_existing`

---

## 8. API Endpoint Design

### 8.1 RESTful Endpoint Structure

**Base Path:** `/api/v1/stages`

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/stages` | GET | List all stages with metadata |
| `/stages/{pipeline}` | GET | List stages for specific pipeline |
| `/stages/{stage_name}/config` | GET | Get config schema for stage |
| `/stages/{stage_name}/defaults` | GET | Get default configuration |
| `/pipelines/validate` | POST | Validate pipeline configuration |
| `/pipelines/execute` | POST | Execute pipeline with config |
| `/pipelines/{pipeline_id}/status` | GET | Get execution status |
| `/pipelines/{pipeline_id}/cancel` | POST | Cancel running pipeline |

### 8.2 Endpoint Specifications

#### GET `/stages`

**Response:**
```json
{
  "pipelines": {
    "ingestion": {
      "name": "Ingestion Pipeline",
      "description": "Process raw video data",
      "stages": ["ingest", "clean", "chunk", "enrich", "embed", "redundancy", "trust"]
    },
    "graphrag": {
      "name": "GraphRAG Pipeline",
      "description": "Build knowledge graph",
      "stages": ["graph_extraction", "entity_resolution", "graph_construction", "community_detection"]
    }
  },
  "stages": {
    "graph_extraction": {
      "name": "graph_extraction",
      "display_name": "Graph Extraction",
      "description": "Extract entities and relationships from chunks",
      "pipeline": "graphrag",
      "config_class": "GraphExtractionConfig",
      "dependencies": []
    },
    "entity_resolution": {
      "name": "entity_resolution",
      "display_name": "Entity Resolution",
      "description": "Resolve and merge duplicate entities",
      "pipeline": "graphrag",
      "config_class": "EntityResolutionConfig",
      "dependencies": ["graph_extraction"]
    },
    ...
  }
}
```

#### GET `/stages/{stage_name}/config`

**Example:** `GET /stages/graph_extraction/config`

**Response:**
```json
{
  "stage_name": "graph_extraction",
  "config_class": "GraphExtractionConfig",
  "description": "Configuration for graph extraction stage",
  "fields": [
    {
      "name": "model_name",
      "type": "str",
      "default": "gpt-4o-mini",
      "required": false,
      "description": "OpenAI model name for extraction",
      "category": "LLM Settings",
      "ui_type": "select",
      "options": ["gpt-4o-mini", "gpt-4", "gpt-3.5-turbo"]
    },
    {
      "name": "temperature",
      "type": "float",
      "default": 0.1,
      "required": false,
      "description": "LLM temperature (0-2, lower = more deterministic)",
      "category": "LLM Settings",
      "ui_type": "slider",
      "min": 0.0,
      "max": 2.0,
      "step": 0.1
    },
    {
      "name": "max_entities_per_chunk",
      "type": "int",
      "default": 20,
      "required": false,
      "description": "Maximum entities to extract per chunk",
      "category": "Extraction Limits",
      "ui_type": "number",
      "min": 1,
      "max": 100
    },
    {
      "name": "similarity_threshold",
      "type": "float",
      "default": 0.85,
      "required": false,
      "description": "Similarity threshold for entity resolution",
      "category": "Quality Thresholds",
      "ui_type": "slider",
      "min": 0.0,
      "max": 1.0,
      "step": 0.05
    },
    ...
  ],
  "categories": [
    {
      "name": "LLM Settings",
      "fields": ["model_name", "temperature", "max_tokens", "llm_retries"]
    },
    {
      "name": "Extraction Limits",
      "fields": ["max_entities_per_chunk", "max_relationships_per_chunk", "batch_size"]
    },
    ...
  ]
}
```

#### POST `/pipelines/validate`

**Request Body:**
```json
{
  "pipeline": "graphrag",
  "stages": ["graph_extraction", "entity_resolution"],
  "config": {
    "graph_extraction": {
      "model_name": "gpt-4o-mini",
      "temperature": 0.1,
      "max_entities_per_chunk": 20
    },
    "entity_resolution": {
      "similarity_threshold": 0.85
    }
  }
}
```

**Response:**
```json
{
  "valid": true,
  "warnings": [
    "Stage 'entity_resolution' depends on 'graph_extraction' - both will be executed"
  ],
  "errors": [],
  "execution_plan": {
    "stages": ["graph_extraction", "entity_resolution"],
    "estimated_time": "15-30 minutes",
    "dependencies_satisfied": true
  }
}
```

#### POST `/pipelines/execute`

**Request Body:**
```json
{
  "pipeline": "graphrag",
  "stages": ["graph_extraction"],
  "config": {
    "graph_extraction": {
      "model_name": "gpt-4o-mini",
      "max": 100,
      "concurrency": 50
    }
  },
  "metadata": {
    "experiment_id": "test_run_123",
    "description": "Testing extraction with 100 chunks"
  }
}
```

**Response:**
```json
{
  "pipeline_id": "pipeline_1733780400_a3b2c1d4",
  "status": "running",
  "started_at": "2025-12-09T15:00:00Z",
  "stages": ["graph_extraction"],
  "tracking_url": "/api/v1/pipelines/pipeline_1733780400_a3b2c1d4/status"
}
```

#### GET `/pipelines/{pipeline_id}/status`

**Response:**
```json
{
  "pipeline_id": "pipeline_1733780400_a3b2c1d4",
  "status": "running",
  "started_at": "2025-12-09T15:00:00Z",
  "elapsed_seconds": 120,
  "current_stage": "graph_extraction",
  "stages": {
    "graph_extraction": {
      "status": "running",
      "progress": {
        "processed": 45,
        "total": 100,
        "percent": 45.0
      },
      "stats": {
        "entities_extracted": 890,
        "relationships_extracted": 1240,
        "avg_entities_per_chunk": 19.8
      }
    }
  }
}
```

---

## 9. UI Requirements and Data Flow

### 9.1 UI Components

**1. Pipeline Selector**
- Radio buttons or dropdown: "Ingestion Pipeline" / "GraphRAG Pipeline"
- Shows pipeline description

**2. Stage Multi-Select**
- Checkboxes for each stage in the selected pipeline
- Visual indication of dependencies
- Warning if dependencies not selected

**3. Stage Configuration Forms**
- Accordion or tabs for each selected stage
- Grouped fields by category
- Different input types: text, number, slider, select, checkbox
- Show default values clearly
- Validation on input

**4. Execution Panel**
- "Validate Configuration" button
- Shows validation results/warnings
- "Execute Pipeline" button
- Displays pipeline ID and tracking link

**5. Status Monitor (Optional Enhancement)**
- Real-time progress updates
- Current stage indicator
- Stats and metrics

### 9.2 UI Data Flow

```
User Interaction Flow:
1. Load page → Fetch available stages (GET /stages)
2. Select pipeline → Filter stages to pipeline type
3. Select stages → Show config forms for selected stages
4. Configure parameters → Populate form fields from defaults
5. Click "Validate" → POST /pipelines/validate
6. Show validation results → Display warnings/errors
7. Click "Execute" → POST /pipelines/execute
8. Redirect to status → GET /pipelines/{id}/status (polling)
```

### 9.3 Example UI Wireframe (Conceptual)

```
┌─────────────────────────────────────────────────────────────┐
│ Stage Configuration                                          │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│ Pipeline: ◉ GraphRAG  ○ Ingestion                           │
│                                                              │
│ Select Stages:                                               │
│  ☑ Graph Extraction                                          │
│  ☑ Entity Resolution (depends on: Graph Extraction)         │
│  ☐ Graph Construction                                        │
│  ☐ Community Detection                                       │
│                                                              │
│ ─────────────────────────────────────────────────────────── │
│                                                              │
│ ▼ Graph Extraction Configuration                            │
│                                                              │
│   LLM Settings:                                              │
│   Model: [gpt-4o-mini ▼]                                    │
│   Temperature: [0.1] ────────●────── (0.0 - 2.0)           │
│                                                              │
│   Extraction Limits:                                         │
│   Max Entities per Chunk: [20]                              │
│   Min Entity Confidence: [0.3] ───●────── (0.0 - 1.0)      │
│                                                              │
│   Processing:                                                │
│   Concurrency: [50]                                         │
│   Max Documents: [100] (leave empty for all)                │
│                                                              │
│ ▼ Entity Resolution Configuration                           │
│   ... (similar structure)                                    │
│                                                              │
│ ─────────────────────────────────────────────────────────── │
│                                                              │
│  [Validate Configuration]  [Execute Pipeline]                │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

---

## 10. Implementation Roadmap

### Phase 1: API Foundation (Week 1)

**1.1 Stage Metadata System**
- [ ] Create `app/stages-api/metadata.py`
- [ ] Implement `get_stage_info()` function
- [ ] Implement `get_config_schema()` function
- [ ] Add field metadata for all config classes
- [ ] Unit tests for metadata extraction

**1.2 Core API Endpoints**
- [ ] Create `app/stages-api/__init__.py`
- [ ] Implement `GET /stages`
- [ ] Implement `GET /stages/{stage_name}/config`
- [ ] Implement `GET /stages/{stage_name}/defaults`
- [ ] Basic HTTP server setup (or integrate with existing API server)

**1.3 Configuration Validation**
- [ ] Create `app/stages-api/validation.py`
- [ ] Implement dependency validation
- [ ] Implement config schema validation
- [ ] Implement `POST /pipelines/validate`

### Phase 2: Pipeline Execution (Week 2)

**2.1 Pipeline Execution API**
- [ ] Create `app/stages-api/execution.py`
- [ ] Implement `POST /pipelines/execute`
- [ ] Implement `GET /pipelines/{id}/status`
- [ ] Implement `POST /pipelines/{id}/cancel`
- [ ] Thread-safe pipeline state management

**2.2 Integration with Existing System**
- [ ] Integrate with `PipelineRunner`
- [ ] Integrate with existing pipeline tracking
- [ ] Test with Ingestion pipeline
- [ ] Test with GraphRAG pipeline

### Phase 3: UI Development (Week 3)

**3.1 Frontend Setup**
- [ ] Choose framework (React, Vue, or plain JS)
- [ ] Set up build system
- [ ] Create basic layout structure

**3.2 Core UI Components**
- [ ] Pipeline selector component
- [ ] Stage multi-select component
- [ ] Dynamic form generator (from schema)
- [ ] Validation results display
- [ ] Execution status panel

**3.3 Integration & Polish**
- [ ] Connect UI to API endpoints
- [ ] Add error handling
- [ ] Add loading states
- [ ] Basic styling (minimalist design)
- [ ] Responsive layout

### Phase 4: Testing & Documentation (Week 4)

**4.1 Testing**
- [ ] API endpoint tests
- [ ] UI component tests
- [ ] End-to-end tests
- [ ] Load testing for concurrent pipelines

**4.2 Documentation**
- [ ] API documentation (OpenAPI/Swagger)
- [ ] UI user guide
- [ ] Developer documentation
- [ ] Example configurations

---

## 11. Technical Considerations

### 11.1 Challenges & Solutions

| Challenge | Solution |
|-----------|----------|
| **Dynamic Config Schema** | Use dataclass introspection + metadata registry |
| **Field Descriptions** | Add metadata dict to each field or separate registry |
| **Type Validation** | Leverage Python type hints + runtime validation |
| **Dependencies** | Use `STAGE_DEPENDENCIES` dict, validate before execution |
| **Concurrent Pipelines** | Thread-safe state management + unique pipeline IDs |
| **Long-Running Tasks** | Background threads + polling endpoint for status |
| **Configuration Persistence** | Save configs to MongoDB + support config file export |

### 11.2 Security Considerations

1. **Input Validation**
   - Validate all user inputs against schema
   - Prevent injection attacks (e.g., MongoDB query injection)
   - Sanitize file paths if config includes file references

2. **Access Control**
   - Consider authentication if exposing publicly
   - Rate limiting for pipeline execution
   - Audit logging for pipeline executions

3. **Resource Limits**
   - Cap `max` parameter to prevent excessive resource use
   - Limit concurrent pipeline executions
   - Timeout for long-running pipelines

### 11.3 Performance Optimizations

1. **Caching**
   - Cache stage metadata (rarely changes)
   - Cache default configs
   - Cache pipeline templates

2. **Async Execution**
   - Execute pipelines in background threads
   - Use async polling for status updates
   - Consider task queue (Celery) for production

3. **Database Optimization**
   - Index on `pipeline_id`, `status`, `started_at`
   - Paginate large result sets
   - Use projections to limit data transfer

### 11.4 Extensibility

**Future Enhancements:**

1. **Configuration Templates**
   - Save/load predefined configurations
   - Share configurations across team
   - Version control for configurations

2. **Advanced Scheduling**
   - Schedule pipeline runs
   - Recurring executions
   - Conditional triggers

3. **Visualization**
   - Pipeline DAG visualization
   - Real-time metrics dashboard
   - Historical performance charts

4. **Multi-Pipeline Orchestration**
   - Chain pipelines (e.g., Ingestion → GraphRAG)
   - Parallel pipeline execution
   - Pipeline composition

---

## 12. Code Examples

### 12.1 Metadata Extraction Function

```python
# app/stages-api/metadata.py

from dataclasses import fields, is_dataclass
from typing import Any, Dict, List, get_type_hints, get_origin, get_args
from business.pipelines.runner import STAGE_REGISTRY

def get_stage_metadata(stage_name: str) -> Dict[str, Any]:
    """Get metadata for a specific stage"""
    stage_cls = STAGE_REGISTRY.get(stage_name)
    if not stage_cls:
        raise ValueError(f"Unknown stage: {stage_name}")
    
    config_cls = getattr(stage_cls, "ConfigCls", None)
    if not config_cls:
        raise ValueError(f"Stage {stage_name} missing ConfigCls")
    
    return {
        "name": stage_name,
        "display_name": stage_name.replace("_", " ").title(),
        "description": stage_cls.description or "",
        "config_class": config_cls.__name__,
        "config_schema": get_config_schema(config_cls),
    }

def get_config_schema(config_cls) -> Dict[str, Any]:
    """Extract configuration schema from dataclass"""
    if not is_dataclass(config_cls):
        raise ValueError(f"{config_cls} is not a dataclass")
    
    schema_fields = []
    type_hints = get_type_hints(config_cls)
    
    for field in fields(config_cls):
        field_type = type_hints.get(field.name, field.type)
        
        # Extract generic types (e.g., Optional[int] → int)
        origin = get_origin(field_type)
        args = get_args(field_type)
        
        if origin is type(None) or (origin and args and type(None) in args):
            # Optional type
            required = False
            actual_type = args[0] if args else field_type
        else:
            required = field.default is field.default_factory
            actual_type = field_type
        
        # Get metadata if available
        metadata = field.metadata if hasattr(field, "metadata") else {}
        
        field_info = {
            "name": field.name,
            "type": str(actual_type.__name__) if hasattr(actual_type, "__name__") else str(actual_type),
            "default": field.default if field.default != field.default_factory else None,
            "required": required,
            "description": metadata.get("description", ""),
            "category": metadata.get("category", "General"),
            "ui_type": metadata.get("ui_type", _infer_ui_type(actual_type)),
            **{k: v for k, v in metadata.items() if k not in ["description", "category", "ui_type"]}
        }
        
        schema_fields.append(field_info)
    
    # Group by category
    categories = {}
    for f in schema_fields:
        cat = f["category"]
        if cat not in categories:
            categories[cat] = []
        categories[cat].append(f["name"])
    
    return {
        "fields": schema_fields,
        "categories": [{"name": k, "fields": v} for k, v in categories.items()],
        "description": config_cls.__doc__ or "",
    }

def _infer_ui_type(field_type) -> str:
    """Infer UI type from Python type"""
    type_name = str(field_type).lower()
    
    if "bool" in type_name:
        return "checkbox"
    elif "int" in type_name:
        return "number"
    elif "float" in type_name:
        return "number"
    elif "str" in type_name:
        return "text"
    elif "list" in type_name:
        return "multiselect"
    else:
        return "text"
```

### 12.2 Validation Function

```python
# app/stages-api/validation.py

from typing import Dict, List, Any
from business.pipelines.graphrag import STAGE_DEPENDENCIES, STAGE_ORDER

def validate_pipeline_config(
    pipeline: str,
    stages: List[str],
    config: Dict[str, Dict[str, Any]]
) -> Dict[str, Any]:
    """Validate pipeline configuration"""
    
    errors = []
    warnings = []
    
    # Validate pipeline type
    if pipeline not in ["ingestion", "graphrag"]:
        errors.append(f"Invalid pipeline: {pipeline}")
        return {"valid": False, "errors": errors, "warnings": warnings}
    
    # Validate stages exist
    from business.pipelines.runner import STAGE_REGISTRY
    for stage in stages:
        if stage not in STAGE_REGISTRY:
            errors.append(f"Unknown stage: {stage}")
    
    if errors:
        return {"valid": False, "errors": errors, "warnings": warnings}
    
    # Validate dependencies (GraphRAG only)
    if pipeline == "graphrag":
        missing_deps = []
        for stage in stages:
            deps = STAGE_DEPENDENCIES.get(stage, [])
            for dep in deps:
                if dep not in stages:
                    missing_deps.append(dep)
                    warnings.append(
                        f"Stage '{stage}' depends on '{dep}' which is not selected. "
                        f"'{dep}' will be auto-included."
                    )
        
        # Auto-include dependencies
        all_stages = stages + missing_deps
        # Sort by order
        all_stages = [s for s in STAGE_ORDER if s in all_stages]
    else:
        all_stages = stages
    
    # Validate config for each stage
    for stage in all_stages:
        stage_cls = STAGE_REGISTRY[stage]
        config_cls = stage_cls.ConfigCls
        stage_config = config.get(stage, {})
        
        # Validate fields
        try:
            # Attempt to instantiate config
            config_cls(**stage_config)
        except TypeError as e:
            errors.append(f"Invalid config for stage '{stage}': {e}")
    
    return {
        "valid": len(errors) == 0,
        "errors": errors,
        "warnings": warnings,
        "execution_plan": {
            "stages": all_stages,
            "dependencies_satisfied": len(missing_deps) == 0
        }
    }
```

### 12.3 Execution Function

```python
# app/stages-api/execution.py

import threading
import time
import uuid
from typing import Dict, Any, List

_active_pipelines = {}
_pipeline_lock = threading.Lock()

def execute_pipeline(
    pipeline: str,
    stages: List[str],
    config: Dict[str, Dict[str, Any]],
    metadata: Dict[str, Any] = None
) -> Dict[str, Any]:
    """Execute pipeline in background thread"""
    
    pipeline_id = f"pipeline_{int(time.time())}_{uuid.uuid4().hex[:8]}"
    
    # Validate first
    from app.stages_api.validation import validate_pipeline_config
    validation = validate_pipeline_config(pipeline, stages, config)
    if not validation["valid"]:
        return {"error": "Invalid configuration", "details": validation}
    
    # Create pipeline
    if pipeline == "graphrag":
        from business.pipelines.graphrag import GraphRAGPipeline
        from core.config.graphrag import GraphRAGPipelineConfig
        
        pipeline_config = GraphRAGPipelineConfig.from_dict(config)
        pipeline_obj = GraphRAGPipeline(pipeline_config)
    elif pipeline == "ingestion":
        from business.pipelines.ingestion import IngestionPipeline
        from business.pipelines.ingestion import IngestionPipelineConfig
        
        pipeline_config = IngestionPipelineConfig.from_dict(config)
        pipeline_obj = IngestionPipeline(pipeline_config)
    else:
        return {"error": f"Unknown pipeline: {pipeline}"}
    
    # Store pipeline state
    with _pipeline_lock:
        _active_pipelines[pipeline_id] = {
            "id": pipeline_id,
            "pipeline": pipeline,
            "status": "starting",
            "started_at": time.time(),
            "stages": stages,
            "config": config,
            "metadata": metadata or {},
        }
    
    # Start execution in background thread
    def run_pipeline():
        try:
            with _pipeline_lock:
                _active_pipelines[pipeline_id]["status"] = "running"
            
            exit_code = pipeline_obj.run_full_pipeline()
            
            with _pipeline_lock:
                _active_pipelines[pipeline_id]["status"] = "completed" if exit_code == 0 else "failed"
                _active_pipelines[pipeline_id]["exit_code"] = exit_code
                _active_pipelines[pipeline_id]["completed_at"] = time.time()
        except Exception as e:
            with _pipeline_lock:
                _active_pipelines[pipeline_id]["status"] = "error"
                _active_pipelines[pipeline_id]["error"] = str(e)
    
    thread = threading.Thread(target=run_pipeline, daemon=True)
    thread.start()
    
    return {
        "pipeline_id": pipeline_id,
        "status": "starting",
        "started_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "tracking_url": f"/api/v1/pipelines/{pipeline_id}/status"
    }

def get_pipeline_status(pipeline_id: str) -> Dict[str, Any]:
    """Get status of running/completed pipeline"""
    with _pipeline_lock:
        if pipeline_id not in _active_pipelines:
            return {"error": "Pipeline not found"}
        return _active_pipelines[pipeline_id].copy()
```

---

## 13. Summary and Next Steps

### 13.1 Key Takeaways

1. **Well-Structured Codebase:** The GraphRAG system has a clean separation of concerns with pipelines, stages, and configurations.

2. **Configuration Flexibility:** The configuration system uses dataclasses with inheritance, making it easy to extend and introspect.

3. **Stage Registry:** The `STAGE_REGISTRY` provides a clean way to discover and instantiate stages dynamically.

4. **Existing Patterns:** The existing API code provides solid patterns to follow for the new stages API.

5. **Metadata Challenge:** The main technical challenge is extracting field metadata (descriptions, UI hints) from dataclasses. This can be solved with field metadata or a separate registry.

### 13.2 Recommended Next Steps

1. **Start with Metadata System:**
   - Implement `get_stage_metadata()` and `get_config_schema()`
   - Add metadata to key config classes (at least GraphRAG stages)
   - Test metadata extraction

2. **Build Core API:**
   - Implement `GET /stages` endpoint
   - Implement `GET /stages/{stage}/config` endpoint
   - Create simple HTTP server or integrate with existing API

3. **Test with Real Stages:**
   - Test with GraphRAG stages (simpler, fewer stages)
   - Validate schema extraction works correctly
   - Iterate on metadata structure

4. **Build Minimal UI:**
   - Create simple HTML form
   - Fetch stage metadata from API
   - Generate form fields dynamically
   - Test configuration submission

5. **Iterate and Expand:**
   - Add validation endpoint
   - Add execution endpoint
   - Add status monitoring
   - Polish UI

### 13.3 Success Criteria

The Stages API and UI will be considered successful when:

1. ✅ **All 13 stages** are discoverable via API
2. ✅ **Configuration schemas** are automatically generated from dataclasses
3. ✅ **UI form** dynamically renders all configuration options
4. ✅ **Validation** catches errors before execution
5. ✅ **Pipeline execution** can be triggered from UI
6. ✅ **Status monitoring** shows real-time progress
7. ✅ **Documentation** clearly explains how to use the system

---

## Appendices

### Appendix A: Complete Stage List

| Pipeline | Stage Name | Description | Config Class |
|----------|------------|-------------|--------------|
| Ingestion | `ingest` | Ingest raw video data | `IngestConfig` |
| Ingestion | `clean` | Clean transcripts with LLM | `CleanConfig` |
| Ingestion | `chunk` | Split into semantic chunks | `ChunkConfig` |
| Ingestion | `enrich` | Extract entities/concepts | `EnrichConfig` |
| Ingestion | `embed` | Generate embeddings | `EmbedConfig` |
| Ingestion | `redundancy` | Detect redundant content | `RedundancyConfig` |
| Ingestion | `trust` | Calculate trust scores | `TrustConfig` |
| GraphRAG | `graph_extraction` | Extract entities/relationships | `GraphExtractionConfig` |
| GraphRAG | `entity_resolution` | Resolve duplicate entities | `EntityResolutionConfig` |
| GraphRAG | `graph_construction` | Build graph structure | `GraphConstructionConfig` |
| GraphRAG | `community_detection` | Detect communities | `CommunityDetectionConfig` |

### Appendix B: Environment Variables Reference

| Variable | Description | Default |
|----------|-------------|---------|
| `DB_NAME` | Database name | `mongo_hack` |
| `GRAPHRAG_MODEL` | OpenAI model | `gpt-4o-mini` |
| `GRAPHRAG_TEMPERATURE` | LLM temperature | `0.1` |
| `GRAPHRAG_EXTRACTION_CONCURRENCY` | Extraction workers | `300` |
| `GRAPHRAG_ENTITY_RESOLUTION_THRESHOLD` | Resolution threshold | `0.85` |
| `GRAPHRAG_COMMUNITY_ALGORITHM` | Algorithm (louvain/leiden) | `louvain` |
| `GRAPHRAG_RESOLUTION_PARAMETER` | Louvain resolution | `1.0` |

### Appendix C: File Structure

```
app/
├── stages-api/                # NEW: Stages API
│   ├── __init__.py
│   ├── metadata.py           # Metadata extraction
│   ├── validation.py         # Config validation
│   ├── execution.py          # Pipeline execution
│   └── server.py             # HTTP server
├── api/                      # Existing APIs
│   ├── pipeline_control.py
│   ├── entities.py
│   └── ...
├── cli/
│   ├── graphrag.py
│   └── main.py
└── ui/                       # NEW: Stages UI
    ├── index.html
    ├── app.js
    └── styles.css
```

---

**End of Document**

*This technical foundation document should serve as the complete reference for implementing the Stages API and UI. All implementation should align with the patterns and principles described here.*

