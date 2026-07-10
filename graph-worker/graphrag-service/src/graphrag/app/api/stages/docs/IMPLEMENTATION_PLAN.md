# Stages API - Implementation Plan

**Version:** 1.0  
**Created:** December 9, 2025  
**Reference:** [API Design Specification](./API_DESIGN_SPECIFICATION.md)

---

## Overview

This document provides a step-by-step implementation plan for building the Stages API. Each step includes:
- **Objective:** What we're building
- **Files:** Files to create/modify
- **Reference:** Section in API_DESIGN_SPECIFICATION.md
- **Tasks:** Specific implementation tasks
- **Validation:** How to verify the step is complete

**Estimated Total Time:** 5-7 days

---

## Phase 1: Project Setup & Foundation

### Step 1.1: Initialize Package Structure

**Objective:** Create the stages-api package with proper Python structure

**Files to Create:**
- `app/stages-api/__init__.py`
- `app/stages-api/constants.py`

**Tasks:**

1. Create `__init__.py`:
```python
"""
Stages API - Configuration and Execution API for GraphRAG Pipelines

This package provides RESTful endpoints for:
- Stage discovery and configuration schema
- Pipeline configuration validation
- Pipeline execution and monitoring
"""

__version__ = "1.0.0"
```

2. Create `constants.py` with pipeline groupings:
```python
"""
Constants for Stages API

Reference: API_DESIGN_SPECIFICATION.md Section 3.2
"""

# Pipeline stage groupings
PIPELINE_GROUPS = {
    "ingestion": ["ingest", "clean", "chunk", "enrich", "embed", "redundancy", "trust"],
    "graphrag": ["graph_extraction", "entity_resolution", "graph_construction", "community_detection"]
}

# Pipeline display names and descriptions
PIPELINE_INFO = {
    "ingestion": {
        "name": "Ingestion Pipeline",
        "description": "Process raw video data through cleaning, chunking, enrichment, and embedding"
    },
    "graphrag": {
        "name": "GraphRAG Pipeline", 
        "description": "Build knowledge graph from processed chunks with entity resolution and community detection"
    }
}

# Field category patterns for automatic inference
CATEGORY_PATTERNS = {
    "LLM Settings": ["model", "temperature", "token", "llm", "prompt"],
    "Processing": ["concurrency", "batch", "timeout", "max", "chunk"],
    "Quality Thresholds": ["threshold", "confidence", "score", "coherence"],
    "Algorithm Parameters": ["algorithm", "resolution", "cluster", "strategy"],
    "Database Configuration": ["db", "collection", "coll"]
}

# UI type inference rules
UI_TYPE_MAP = {
    "bool": "checkbox",
    "int": "number",
    "float": "slider",
    "str": "text",
    "list": "multiselect"
}
```

**Validation:**
- [ ] Package imports successfully: `from app.stages_api import constants`
- [ ] Constants are accessible

---

### Step 1.2: Create Field Metadata Registry

**Objective:** Create registry for UI-friendly field descriptions and hints

**Reference:** API_DESIGN_SPECIFICATION.md Section 5.2

**Files to Create:**
- `app/stages-api/field_metadata.py`

**Tasks:**

1. Create comprehensive field metadata for all common and stage-specific fields:

```python
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
        "placeholder": "Leave empty to process all documents"
    },
    "llm": {
        "description": "Enable LLM processing for this stage",
        "ui_type": "checkbox"
    },
    "verbose": {
        "description": "Enable detailed logging output",
        "ui_type": "checkbox"
    },
    "dry_run": {
        "description": "Simulate execution without writing to database",
        "ui_type": "checkbox"
    },
    "db_name": {
        "description": "Database name (defaults to environment DB_NAME)",
        "ui_type": "text",
        "placeholder": "mongo_hack"
    },
    "read_db_name": {
        "description": "Source database for reading data (for experiments)",
        "ui_type": "text",
        "placeholder": "Same as db_name if not specified"
    },
    "write_db_name": {
        "description": "Target database for writing results (for experiments)",
        "ui_type": "text",
        "placeholder": "Same as db_name if not specified"
    },
    "concurrency": {
        "description": "Number of parallel workers for concurrent processing",
        "ui_type": "number",
        "min": 1,
        "max": 500,
        "recommended": 50
    },
    "video_id": {
        "description": "Filter processing to a specific video ID",
        "ui_type": "text",
        "placeholder": "Leave empty to process all videos"
    },
    "upsert_existing": {
        "description": "Update existing records instead of skipping",
        "ui_type": "checkbox"
    },
    
    # ============================================
    # LLM Settings (GraphRAG Stages)
    # ============================================
    "model_name": {
        "description": "OpenAI model to use for LLM operations",
        "ui_type": "select",
        "options": ["gpt-4o-mini", "gpt-4", "gpt-4-turbo", "gpt-3.5-turbo"],
        "recommended": "gpt-4o-mini"
    },
    "temperature": {
        "description": "LLM temperature (0 = deterministic, 2 = creative)",
        "ui_type": "slider",
        "min": 0.0,
        "max": 2.0,
        "step": 0.1
    },
    "max_tokens": {
        "description": "Maximum tokens in LLM response",
        "ui_type": "number",
        "min": 100,
        "max": 16000,
        "placeholder": "Leave empty for model default"
    },
    "llm_retries": {
        "description": "Number of retry attempts for LLM API failures",
        "ui_type": "number",
        "min": 1,
        "max": 10
    },
    "llm_backoff_s": {
        "description": "Backoff delay between retries (seconds)",
        "ui_type": "number",
        "min": 0.5,
        "max": 30.0,
        "step": 0.5
    },
    
    # ============================================
    # Graph Extraction Settings
    # ============================================
    "max_entities_per_chunk": {
        "description": "Maximum entities to extract per chunk",
        "ui_type": "number",
        "min": 1,
        "max": 100
    },
    "max_relationships_per_chunk": {
        "description": "Maximum relationships to extract per chunk",
        "ui_type": "number",
        "min": 1,
        "max": 200
    },
    "min_entity_confidence": {
        "description": "Minimum confidence score for extracted entities (0-1)",
        "ui_type": "slider",
        "min": 0.0,
        "max": 1.0,
        "step": 0.05
    },
    "min_relationship_confidence": {
        "description": "Minimum confidence score for extracted relationships (0-1)",
        "ui_type": "slider",
        "min": 0.0,
        "max": 1.0,
        "step": 0.05
    },
    "batch_size": {
        "description": "Number of items to process per batch",
        "ui_type": "number",
        "min": 1,
        "max": 500
    },
    "extraction_timeout": {
        "description": "Timeout for extraction per chunk (seconds)",
        "ui_type": "number",
        "min": 10,
        "max": 300
    },
    "entity_types": {
        "description": "Types of entities to extract",
        "ui_type": "multiselect",
        "options": ["PERSON", "ORGANIZATION", "TECHNOLOGY", "CONCEPT", "LOCATION", "EVENT", "OTHER"]
    },
    
    # ============================================
    # Entity Resolution Settings
    # ============================================
    "similarity_threshold": {
        "description": "Minimum similarity score for entity resolution (0-1)",
        "ui_type": "slider",
        "min": 0.0,
        "max": 1.0,
        "step": 0.05
    },
    "max_aliases_per_entity": {
        "description": "Maximum aliases to store per entity",
        "ui_type": "number",
        "min": 1,
        "max": 50
    },
    "min_source_count": {
        "description": "Minimum source count for entity to be considered valid",
        "ui_type": "number",
        "min": 1,
        "max": 10
    },
    "use_fuzzy_matching": {
        "description": "Enable fuzzy string matching for entity resolution",
        "ui_type": "checkbox"
    },
    "use_embedding_similarity": {
        "description": "Use embedding-based similarity for entity resolution",
        "ui_type": "checkbox"
    },
    "use_context_similarity": {
        "description": "Use context-based similarity for entity resolution",
        "ui_type": "checkbox"
    },
    "use_relationship_clustering": {
        "description": "Use relationship-based clustering for entity resolution",
        "ui_type": "checkbox"
    },
    "resolution_timeout": {
        "description": "Timeout for resolution per batch (seconds)",
        "ui_type": "number",
        "min": 30,
        "max": 600
    },
    
    # ============================================
    # Graph Construction Settings
    # ============================================
    "max_relationships_per_entity": {
        "description": "Maximum relationships to store per entity",
        "ui_type": "number",
        "min": 10,
        "max": 500
    },
    "calculate_centrality": {
        "description": "Calculate centrality metrics for entities",
        "ui_type": "checkbox"
    },
    "calculate_degree": {
        "description": "Calculate degree metrics for entities",
        "ui_type": "checkbox"
    },
    "calculate_clustering": {
        "description": "Calculate clustering coefficient for entities",
        "ui_type": "checkbox"
    },
    "validate_entity_existence": {
        "description": "Validate that referenced entities exist",
        "ui_type": "checkbox"
    },
    "max_relationship_distance": {
        "description": "Maximum hops for relationship validation",
        "ui_type": "number",
        "min": 1,
        "max": 10
    },
    
    # ============================================
    # Community Detection Settings
    # ============================================
    "algorithm": {
        "description": "Community detection algorithm",
        "ui_type": "select",
        "options": ["louvain", "hierarchical_leiden"],
        "recommended": "louvain"
    },
    "resolution_parameter": {
        "description": "Louvain resolution parameter (higher = more communities)",
        "ui_type": "slider",
        "min": 0.1,
        "max": 3.0,
        "step": 0.1
    },
    "max_cluster_size": {
        "description": "Maximum community size (soft limit)",
        "ui_type": "number",
        "min": 5,
        "max": 500
    },
    "min_cluster_size": {
        "description": "Minimum community size (for filtering)",
        "ui_type": "number",
        "min": 2,
        "max": 20
    },
    "max_iterations": {
        "description": "Maximum iterations for community detection",
        "ui_type": "number",
        "min": 10,
        "max": 1000
    },
    "max_levels": {
        "description": "Maximum hierarchical levels (Leiden only)",
        "ui_type": "number",
        "min": 1,
        "max": 10
    },
    "max_summary_length": {
        "description": "Maximum characters in community summary",
        "ui_type": "number",
        "min": 500,
        "max": 10000
    },
    "min_summary_length": {
        "description": "Minimum characters in community summary",
        "ui_type": "number",
        "min": 50,
        "max": 1000
    },
    "summarization_timeout": {
        "description": "Timeout for summarization per community (seconds)",
        "ui_type": "number",
        "min": 30,
        "max": 600
    },
    "min_coherence_score": {
        "description": "Minimum coherence score for valid community (0-1)",
        "ui_type": "slider",
        "min": 0.0,
        "max": 1.0,
        "step": 0.05
    },
    "min_entity_count": {
        "description": "Minimum entities required per community",
        "ui_type": "number",
        "min": 2,
        "max": 50
    },
    
    # ============================================
    # Ingestion Stage Settings
    # ============================================
    "chunk_strategy": {
        "description": "Strategy for splitting text into chunks",
        "ui_type": "select",
        "options": ["recursive", "sentence", "paragraph", "fixed"],
        "recommended": "recursive"
    },
    "token_size": {
        "description": "Target size of chunks in tokens",
        "ui_type": "number",
        "min": 100,
        "max": 5000
    },
    "overlap_pct": {
        "description": "Overlap percentage between chunks (0-1)",
        "ui_type": "slider",
        "min": 0.0,
        "max": 0.5,
        "step": 0.05
    },
    "use_hybrid_embedding_text": {
        "description": "Use hybrid text for embeddings",
        "ui_type": "checkbox"
    },
    "unit_normalize_embeddings": {
        "description": "Unit normalize embedding vectors",
        "ui_type": "checkbox"
    }
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
```

**Validation:**
- [ ] `get_field_metadata("model_name")` returns expected dict
- [ ] `get_field_metadata("unknown_field")` returns empty dict
- [ ] All field names are lowercase and match config field names

---

## Phase 2: Metadata System

### Step 2.1: Implement Core Metadata Functions

**Objective:** Build introspection system to extract config schemas

**Reference:** API_DESIGN_SPECIFICATION.md Sections 3.2, 3.3, 3.4

**Files to Create:**
- `app/stages-api/metadata.py`

**Tasks:**

1. Create `metadata.py` with all introspection logic:

```python
"""
Stage Metadata Extraction Module

Reference: API_DESIGN_SPECIFICATION.md Section 3.2, 3.3, 3.4

Provides functions to:
- List all available stages
- Extract configuration schemas from dataclasses
- Infer UI types and categories
"""

import threading
from dataclasses import fields, is_dataclass, MISSING
from typing import Any, Dict, List, get_type_hints, get_origin, get_args

from business.pipelines.runner import STAGE_REGISTRY
from .constants import PIPELINE_GROUPS, PIPELINE_INFO, CATEGORY_PATTERNS, UI_TYPE_MAP
from .field_metadata import get_field_metadata, has_field_metadata


# ============================================
# Caching
# ============================================

_metadata_cache: Dict[str, Dict[str, Any]] = {}
_cache_lock = threading.Lock()


def clear_metadata_cache() -> None:
    """Clear metadata cache (useful for testing)"""
    with _cache_lock:
        _metadata_cache.clear()


# ============================================
# Public API Functions
# ============================================

def list_stages() -> Dict[str, Any]:
    """
    List all available stages grouped by pipeline.
    
    Reference: API_DESIGN_SPECIFICATION.md Section 3.2
    
    Returns:
        Dictionary containing pipeline metadata and stage information
    """
    result = {
        "pipelines": {},
        "stages": {}
    }
    
    # Build pipeline metadata
    for pipeline_name, stage_list in PIPELINE_GROUPS.items():
        info = PIPELINE_INFO.get(pipeline_name, {})
        result["pipelines"][pipeline_name] = {
            "name": info.get("name", pipeline_name.title()),
            "description": info.get("description", ""),
            "stages": stage_list,
            "stage_count": len(stage_list)
        }
    
    # Build stage metadata
    for stage_name, stage_cls in STAGE_REGISTRY.items():
        pipeline = _get_stage_pipeline(stage_name)
        dependencies = _get_stage_dependencies(stage_name, pipeline)
        
        result["stages"][stage_name] = {
            "name": stage_name,
            "display_name": stage_name.replace("_", " ").title(),
            "description": getattr(stage_cls, "description", "") or "",
            "pipeline": pipeline,
            "config_class": stage_cls.ConfigCls.__name__,
            "dependencies": dependencies,
            "has_llm": _stage_uses_llm(stage_cls),
        }
    
    return result


def get_stage_config(stage_name: str) -> Dict[str, Any]:
    """
    Get configuration schema for a specific stage.
    
    Reference: API_DESIGN_SPECIFICATION.md Section 3.3
    
    Args:
        stage_name: Name of the stage (e.g., "graph_extraction")
    
    Returns:
        Dictionary containing stage config schema
    
    Raises:
        ValueError: If stage not found or has no config class
    """
    # Check cache first
    with _cache_lock:
        cache_key = f"config_{stage_name}"
        if cache_key in _metadata_cache:
            return _metadata_cache[cache_key].copy()
    
    # Validate stage exists
    stage_cls = STAGE_REGISTRY.get(stage_name)
    if not stage_cls:
        raise ValueError(f"Unknown stage: {stage_name}")
    
    config_cls = getattr(stage_cls, "ConfigCls", None)
    if not config_cls:
        raise ValueError(f"Stage {stage_name} has no ConfigCls")
    
    # Extract schema
    schema = _extract_config_schema(config_cls)
    
    result = {
        "stage_name": stage_name,
        "config_class": config_cls.__name__,
        "description": config_cls.__doc__ or "",
        "fields": schema["fields"],
        "categories": schema["categories"],
        "field_count": len(schema["fields"])
    }
    
    # Cache result
    with _cache_lock:
        _metadata_cache[cache_key] = result.copy()
    
    return result


def get_stage_defaults(stage_name: str) -> Dict[str, Any]:
    """
    Get default configuration for a stage.
    
    Reference: API_DESIGN_SPECIFICATION.md Section 3.4
    
    Args:
        stage_name: Name of the stage
    
    Returns:
        Dictionary with default configuration values
    """
    stage_cls = STAGE_REGISTRY.get(stage_name)
    if not stage_cls:
        raise ValueError(f"Unknown stage: {stage_name}")
    
    config_cls = stage_cls.ConfigCls
    if not config_cls:
        raise ValueError(f"Stage {stage_name} has no ConfigCls")
    
    # Instantiate with defaults
    try:
        config_instance = config_cls()
    except Exception as e:
        raise ValueError(f"Failed to create default config: {e}")
    
    # Convert to dictionary
    config_dict = _config_to_dict(config_instance)
    
    return {
        "stage_name": stage_name,
        "config_class": config_cls.__name__,
        "config": config_dict
    }


def list_pipeline_stages(pipeline: str) -> Dict[str, Any]:
    """
    List stages for a specific pipeline.
    
    Args:
        pipeline: Pipeline name ("ingestion" or "graphrag")
    
    Returns:
        Dictionary with pipeline info and stage details
    """
    if pipeline not in PIPELINE_GROUPS:
        raise ValueError(f"Unknown pipeline: {pipeline}. Valid options: {list(PIPELINE_GROUPS.keys())}")
    
    all_stages = list_stages()
    
    stage_names = PIPELINE_GROUPS[pipeline]
    pipeline_stages = {
        name: all_stages["stages"][name]
        for name in stage_names
        if name in all_stages["stages"]
    }
    
    return {
        "pipeline": pipeline,
        "info": all_stages["pipelines"].get(pipeline, {}),
        "stages": pipeline_stages
    }


# ============================================
# Internal Helper Functions
# ============================================

def _get_stage_pipeline(stage_name: str) -> str:
    """Determine which pipeline a stage belongs to"""
    for pipeline, stages in PIPELINE_GROUPS.items():
        if stage_name in stages:
            return pipeline
    return "unknown"


def _get_stage_dependencies(stage_name: str, pipeline: str) -> List[str]:
    """Get stage dependencies (GraphRAG only)"""
    if pipeline != "graphrag":
        return []
    
    try:
        from business.pipelines.graphrag import STAGE_DEPENDENCIES
        return STAGE_DEPENDENCIES.get(stage_name, [])
    except ImportError:
        return []


def _stage_uses_llm(stage_cls) -> bool:
    """Check if stage uses LLM by inspecting config"""
    try:
        config_cls = stage_cls.ConfigCls
        field_names = [f.name for f in fields(config_cls)]
        return "model_name" in field_names or "llm" in field_names
    except Exception:
        return False


def _extract_config_schema(config_cls) -> Dict[str, Any]:
    """
    Extract configuration schema from dataclass using introspection.
    
    Reference: API_DESIGN_SPECIFICATION.md Section 3.3 Implementation
    """
    if not is_dataclass(config_cls):
        raise ValueError(f"{config_cls} is not a dataclass")
    
    schema_fields = []
    type_hints = get_type_hints(config_cls)
    
    # Get parent class fields (BaseStageConfig)
    parent_fields = set()
    for base in config_cls.__bases__:
        if is_dataclass(base):
            parent_fields.update(f.name for f in fields(base))
    
    for field in fields(config_cls):
        field_type = type_hints.get(field.name, field.type)
        
        # Parse type information
        type_info = _parse_type(field_type)
        
        # Get default value
        if field.default is not MISSING:
            default_value = field.default
        elif field.default_factory is not MISSING:
            default_value = None  # Factory defaults are not serializable
        else:
            default_value = None
        
        # Get field metadata from dataclass
        dc_metadata = dict(field.metadata) if hasattr(field, "metadata") else {}
        
        # Get field metadata from registry (takes precedence)
        registry_metadata = get_field_metadata(field.name)
        
        # Determine if field is from parent class
        is_inherited = field.name in parent_fields
        
        # Build field info (registry metadata takes precedence)
        field_info = {
            "name": field.name,
            "type": type_info["type"],
            "python_type": str(type_info["python_type"]),
            "default": _serialize_default(default_value),
            "required": not type_info["optional"] and default_value is None,
            "optional": type_info["optional"],
            "description": registry_metadata.get("description", dc_metadata.get("description", "")),
            "category": registry_metadata.get("category", dc_metadata.get("category", _infer_category(field.name, is_inherited))),
            "ui_type": registry_metadata.get("ui_type", dc_metadata.get("ui_type", _infer_ui_type(type_info["python_type"]))),
            "is_inherited": is_inherited,
        }
        
        # Add UI-specific metadata
        for key in ["min", "max", "step", "options", "placeholder", "pattern", "recommended"]:
            if key in registry_metadata:
                field_info[key] = registry_metadata[key]
            elif key in dc_metadata:
                field_info[key] = dc_metadata[key]
        
        schema_fields.append(field_info)
    
    # Group fields by category
    categories = _group_by_category(schema_fields)
    
    return {
        "fields": schema_fields,
        "categories": categories,
    }


def _parse_type(field_type) -> Dict[str, Any]:
    """Parse Python type hint into API-friendly format"""
    origin = get_origin(field_type)
    args = get_args(field_type)
    
    # Handle Optional[T] (which is Union[T, None])
    if origin is type(None):
        return {
            "type": "null",
            "python_type": type(None),
            "optional": True
        }
    
    # Check for Union with None (Optional)
    if args and type(None) in args:
        non_none_types = [t for t in args if t is not type(None)]
        actual_type = non_none_types[0] if non_none_types else str
        return {
            "type": _get_type_name(actual_type),
            "python_type": actual_type,
            "optional": True
        }
    
    # Handle List[T]
    if origin is list:
        item_type = args[0] if args else str
        return {
            "type": "array",
            "python_type": list,
            "item_type": _get_type_name(item_type),
            "optional": False
        }
    
    # Basic type
    return {
        "type": _get_type_name(field_type),
        "python_type": field_type,
        "optional": False
    }


def _get_type_name(python_type) -> str:
    """Get API-friendly type name"""
    if hasattr(python_type, "__name__"):
        name = python_type.__name__
    else:
        name = str(python_type)
    
    type_map = {
        "str": "string",
        "int": "integer",
        "float": "number",
        "bool": "boolean",
        "list": "array",
        "dict": "object",
        "NoneType": "null"
    }
    
    return type_map.get(name, name)


def _infer_category(field_name: str, is_inherited: bool) -> str:
    """Infer field category from field name"""
    if is_inherited:
        return "Common Fields"
    
    name_lower = field_name.lower()
    
    for category, patterns in CATEGORY_PATTERNS.items():
        if any(p in name_lower for p in patterns):
            return category
    
    return "General"


def _infer_ui_type(python_type) -> str:
    """Infer UI input type from Python type"""
    if python_type == bool:
        return "checkbox"
    elif python_type == int:
        return "number"
    elif python_type == float:
        return "slider"
    elif python_type == str:
        return "text"
    elif python_type == list:
        return "multiselect"
    else:
        return "text"


def _group_by_category(schema_fields: List[Dict]) -> List[Dict[str, Any]]:
    """Group fields by category for UI rendering"""
    categories = {}
    for field in schema_fields:
        cat = field["category"]
        if cat not in categories:
            categories[cat] = []
        categories[cat].append(field["name"])
    
    # Predefined category order
    category_order = [
        "Common Fields", 
        "LLM Settings", 
        "Processing", 
        "Quality Thresholds",
        "Algorithm Parameters", 
        "Database Configuration", 
        "General"
    ]
    
    sorted_categories = []
    for cat_name in category_order:
        if cat_name in categories:
            sorted_categories.append({
                "name": cat_name,
                "fields": categories[cat_name],
                "field_count": len(categories[cat_name])
            })
    
    # Add remaining categories
    for cat_name, field_list in categories.items():
        if cat_name not in category_order:
            sorted_categories.append({
                "name": cat_name,
                "fields": field_list,
                "field_count": len(field_list)
            })
    
    return sorted_categories


def _serialize_default(value: Any) -> Any:
    """Serialize default value for JSON"""
    if value is None or value is MISSING:
        return None
    if isinstance(value, (str, int, float, bool)):
        return value
    if isinstance(value, list):
        return value
    # For complex types, return None
    return None


def _config_to_dict(config_obj) -> Dict[str, Any]:
    """Convert dataclass config to dictionary"""
    from dataclasses import asdict
    
    try:
        return asdict(config_obj)
    except Exception:
        # Fallback: manual conversion
        result = {}
        for field in fields(config_obj):
            value = getattr(config_obj, field.name)
            result[field.name] = _serialize_default(value)
        return result
```

**Validation:**
- [ ] `list_stages()` returns all 11+ stages
- [ ] `get_stage_config("graph_extraction")` returns valid schema
- [ ] `get_stage_defaults("graph_extraction")` returns default config
- [ ] Field categories are correctly inferred
- [ ] UI types are correctly inferred
- [ ] Caching works correctly

---

## Phase 3: Validation System

### Step 3.1: Implement Validation Module

**Objective:** Create configuration validation with type checking and dependency validation

**Reference:** API_DESIGN_SPECIFICATION.md Sections 3.5, 6.1, 6.2

**Files to Create:**
- `app/stages-api/validation.py`

**Tasks:**

1. Create `validation.py`:

```python
"""
Pipeline Configuration Validation Module

Reference: API_DESIGN_SPECIFICATION.md Section 3.5, 6.1, 6.2

Provides:
- Pipeline type validation
- Stage existence validation
- Dependency validation (GraphRAG)
- Configuration field validation
- Type checking
"""

from dataclasses import fields, MISSING
from typing import Any, Dict, List, Optional, get_type_hints, get_origin, get_args

from business.pipelines.runner import STAGE_REGISTRY
from .constants import PIPELINE_GROUPS
from .field_metadata import get_field_metadata


def validate_pipeline_config(
    pipeline: str,
    stages: List[str],
    config: Dict[str, Dict[str, Any]]
) -> Dict[str, Any]:
    """
    Validate pipeline configuration.
    
    Reference: API_DESIGN_SPECIFICATION.md Section 3.5
    
    Checks:
    1. Pipeline type is valid
    2. All stages exist in registry
    3. Stage dependencies are satisfied
    4. Configuration fields are valid
    5. Field types match expected types
    
    Args:
        pipeline: Pipeline type ("ingestion" or "graphrag")
        stages: List of stage names to execute
        config: Dictionary of stage configurations {stage_name: {field: value}}
    
    Returns:
        Validation result with errors, warnings, and execution plan
    """
    errors: List[Dict[str, Any]] = []
    warnings: List[Dict[str, Any]] = []
    missing_deps: List[str] = []
    
    # 1. Validate pipeline type
    if pipeline not in PIPELINE_GROUPS:
        errors.append({
            "code": "INVALID_PIPELINE",
            "message": f"Invalid pipeline type: {pipeline}",
            "field": "pipeline",
            "valid_options": list(PIPELINE_GROUPS.keys())
        })
        return {
            "valid": False,
            "errors": errors,
            "warnings": warnings,
            "execution_plan": None
        }
    
    # 2. Validate stages exist and belong to pipeline
    valid_stages = PIPELINE_GROUPS[pipeline]
    for stage in stages:
        if stage not in STAGE_REGISTRY:
            errors.append({
                "code": "UNKNOWN_STAGE",
                "message": f"Unknown stage: {stage}",
                "field": "stages",
                "stage": stage
            })
        elif stage not in valid_stages:
            errors.append({
                "code": "STAGE_PIPELINE_MISMATCH",
                "message": f"Stage '{stage}' does not belong to '{pipeline}' pipeline",
                "field": "stages",
                "stage": stage,
                "expected_pipeline": _get_stage_pipeline(stage)
            })
    
    if errors:
        return {
            "valid": False,
            "errors": errors,
            "warnings": warnings,
            "execution_plan": None
        }
    
    # 3. Validate dependencies (GraphRAG only)
    if pipeline == "graphrag":
        try:
            from business.pipelines.graphrag import STAGE_DEPENDENCIES, STAGE_ORDER
            
            for stage in stages:
                deps = STAGE_DEPENDENCIES.get(stage, [])
                for dep in deps:
                    if dep not in stages and dep not in missing_deps:
                        missing_deps.append(dep)
                        warnings.append({
                            "code": "MISSING_DEPENDENCY",
                            "message": f"Stage '{stage}' depends on '{dep}' which is not selected",
                            "resolution": f"'{dep}' will be auto-included in execution",
                            "stage": stage,
                            "dependency": dep
                        })
            
            # Build complete stage list with dependencies
            all_stages = list(stages) + missing_deps
            # Sort by execution order
            all_stages = [s for s in STAGE_ORDER if s in all_stages]
        except ImportError:
            all_stages = stages
    else:
        # For ingestion, maintain the provided order
        all_stages = stages
    
    # 4. Validate stage configurations
    for stage in all_stages:
        stage_cls = STAGE_REGISTRY.get(stage)
        if not stage_cls:
            continue
            
        config_cls = stage_cls.ConfigCls
        stage_config = config.get(stage, {})
        
        # Validate config fields
        validation_errors = _validate_stage_config(stage, config_cls, stage_config)
        errors.extend(validation_errors)
    
    # Build execution plan
    execution_plan = {
        "stages": all_stages,
        "stage_count": len(all_stages),
        "dependencies_satisfied": len(missing_deps) == 0,
        "auto_included_dependencies": missing_deps,
        "execution_order": all_stages,
    }
    
    return {
        "valid": len(errors) == 0,
        "errors": errors,
        "warnings": warnings,
        "execution_plan": execution_plan
    }


def validate_stage_config_only(
    stage_name: str,
    config: Dict[str, Any]
) -> Dict[str, Any]:
    """
    Validate configuration for a single stage (without pipeline context).
    
    Useful for validating individual stage configs before full pipeline validation.
    """
    stage_cls = STAGE_REGISTRY.get(stage_name)
    if not stage_cls:
        return {
            "valid": False,
            "errors": [{
                "code": "UNKNOWN_STAGE",
                "message": f"Unknown stage: {stage_name}",
                "stage": stage_name
            }],
            "warnings": []
        }
    
    config_cls = stage_cls.ConfigCls
    errors = _validate_stage_config(stage_name, config_cls, config)
    
    return {
        "valid": len(errors) == 0,
        "errors": errors,
        "warnings": []
    }


def _validate_stage_config(
    stage_name: str,
    config_cls,
    config_dict: Dict[str, Any]
) -> List[Dict[str, Any]]:
    """
    Validate configuration for a single stage.
    
    Reference: API_DESIGN_SPECIFICATION.md Section 3.5 Implementation
    
    Returns list of error objects.
    """
    errors = []
    
    # Get field definitions
    type_hints = get_type_hints(config_cls)
    valid_fields = {f.name: f for f in fields(config_cls)}
    
    # Check for unknown fields
    for field_name in config_dict.keys():
        if field_name not in valid_fields:
            errors.append({
                "code": "UNKNOWN_FIELD",
                "message": f"Unknown configuration field: {field_name}",
                "stage": stage_name,
                "field": field_name,
                "valid_fields": list(valid_fields.keys())
            })
    
    # Check required fields and types
    for field_name, field_def in valid_fields.items():
        value = config_dict.get(field_name)
        
        # Skip if not provided and has default
        if value is None:
            if field_def.default is MISSING and field_def.default_factory is MISSING:
                errors.append({
                    "code": "MISSING_REQUIRED_FIELD",
                    "message": f"Required field missing: {field_name}",
                    "stage": stage_name,
                    "field": field_name
                })
            continue
        
        # Type validation
        expected_type = type_hints.get(field_name, field_def.type)
        type_error = _validate_field_type(field_name, value, expected_type)
        if type_error:
            errors.append({
                "code": "TYPE_MISMATCH",
                "message": type_error,
                "stage": stage_name,
                "field": field_name,
                "expected_type": str(expected_type),
                "actual_type": type(value).__name__,
                "actual_value": value
            })
            continue
        
        # Value validation (range, options, etc.)
        value_error = _validate_field_value(field_name, value)
        if value_error:
            errors.append({
                "code": "VALUE_OUT_OF_RANGE",
                "message": value_error,
                "stage": stage_name,
                "field": field_name,
                "actual_value": value
            })
    
    return errors


def _validate_field_type(field_name: str, value: Any, expected_type) -> Optional[str]:
    """
    Validate that value matches expected type.
    
    Returns error message if invalid, None if valid.
    """
    # Handle Optional types
    origin = get_origin(expected_type)
    args = get_args(expected_type)
    
    if args and type(None) in args:
        # Optional type - check against non-None type
        if value is None:
            return None  # None is valid for Optional
        non_none_types = [t for t in args if t is not type(None)]
        expected_type = non_none_types[0] if non_none_types else str
    
    # Type checking
    if expected_type == str:
        if not isinstance(value, str):
            return f"Expected string, got {type(value).__name__}"
    elif expected_type == int:
        if not isinstance(value, int) or isinstance(value, bool):
            return f"Expected integer, got {type(value).__name__}"
    elif expected_type == float:
        if not isinstance(value, (int, float)) or isinstance(value, bool):
            return f"Expected number, got {type(value).__name__}"
    elif expected_type == bool:
        if not isinstance(value, bool):
            return f"Expected boolean, got {type(value).__name__}"
    elif expected_type == list or get_origin(expected_type) is list:
        if not isinstance(value, list):
            return f"Expected array, got {type(value).__name__}"
    
    return None


def _validate_field_value(field_name: str, value: Any) -> Optional[str]:
    """
    Validate field value against metadata constraints.
    
    Reference: API_DESIGN_SPECIFICATION.md Section 6.2
    
    Returns error message if invalid, None if valid.
    """
    metadata = get_field_metadata(field_name)
    if not metadata:
        return None
    
    # Range validation for numbers
    if isinstance(value, (int, float)):
        if "min" in metadata and value < metadata["min"]:
            return f"Value {value} is less than minimum {metadata['min']}"
        if "max" in metadata and value > metadata["max"]:
            return f"Value {value} is greater than maximum {metadata['max']}"
    
    # Options validation for selects
    if "options" in metadata:
        if value not in metadata["options"]:
            return f"Value '{value}' must be one of: {', '.join(metadata['options'])}"
    
    return None


def _get_stage_pipeline(stage_name: str) -> str:
    """Determine which pipeline a stage belongs to"""
    for pipeline, stages in PIPELINE_GROUPS.items():
        if stage_name in stages:
            return pipeline
    return "unknown"
```

**Validation:**
- [ ] Valid config passes validation
- [ ] Invalid pipeline type returns error
- [ ] Unknown stage returns error
- [ ] Type mismatch returns error
- [ ] Missing dependency generates warning
- [ ] Auto-included dependencies appear in execution plan

---

## Phase 4: Execution System

### Step 4.1: Implement Execution Module

**Objective:** Build pipeline execution and status tracking system

**Reference:** API_DESIGN_SPECIFICATION.md Sections 3.6, 3.7, 7.1, 7.2

**Files to Create:**
- `app/stages-api/execution.py`

**Tasks:**

1. Create `execution.py`:

```python
"""
Pipeline Execution Management Module

Reference: API_DESIGN_SPECIFICATION.md Section 3.6, 3.7, 7.1, 7.2

Provides:
- Background pipeline execution
- Thread-safe state management
- Progress tracking
- Pipeline cancellation
"""

import logging
import os
import threading
import time
import uuid
from datetime import datetime
from typing import Any, Dict, List, Optional

logger = logging.getLogger(__name__)

# Thread-safe pipeline state storage
_active_pipelines: Dict[str, Dict[str, Any]] = {}
_pipeline_lock = threading.Lock()


def execute_pipeline(
    pipeline: str,
    stages: List[str],
    config: Dict[str, Dict[str, Any]],
    metadata: Optional[Dict[str, Any]] = None
) -> Dict[str, Any]:
    """
    Execute a pipeline in the background.
    
    Reference: API_DESIGN_SPECIFICATION.md Section 3.6
    
    Args:
        pipeline: Pipeline type ("ingestion" or "graphrag")
        stages: List of stage names to execute
        config: Stage configurations
        metadata: Optional metadata (experiment_id, description, etc.)
    
    Returns:
        Pipeline execution info with pipeline_id and tracking URL
    """
    # Generate unique pipeline ID
    pipeline_id = f"pipeline_{int(time.time())}_{uuid.uuid4().hex[:8]}"
    
    # Validate configuration first
    from .validation import validate_pipeline_config
    validation = validate_pipeline_config(pipeline, stages, config)
    
    if not validation["valid"]:
        return {
            "error": "Invalid configuration",
            "details": validation,
            "pipeline_id": None
        }
    
    # Use execution plan from validation (includes dependencies)
    execution_stages = validation["execution_plan"]["stages"]
    
    # Initialize pipeline state
    with _pipeline_lock:
        _active_pipelines[pipeline_id] = {
            "pipeline_id": pipeline_id,
            "pipeline": pipeline,
            "status": "starting",
            "started_at": datetime.utcnow().isoformat() + "Z",
            "started_timestamp": time.time(),
            "stages": execution_stages,
            "config": config,
            "metadata": metadata or {},
            "current_stage": None,
            "current_stage_index": 0,
            "completed_stages": [],
            "failed_stages": [],
            "progress": {
                "total_stages": len(execution_stages),
                "completed_stages": 0,
                "percent": 0.0
            },
            "error": None,
            "error_stage": None,
            "exit_code": None
        }
    
    # Start execution in background thread
    thread = threading.Thread(
        target=_run_pipeline_background,
        args=(pipeline_id, pipeline, execution_stages, config, metadata),
        daemon=True,
        name=f"Pipeline-{pipeline_id}"
    )
    thread.start()
    
    return {
        "pipeline_id": pipeline_id,
        "status": "starting",
        "started_at": _active_pipelines[pipeline_id]["started_at"],
        "stages": execution_stages,
        "tracking_url": f"/api/v1/pipelines/{pipeline_id}/status"
    }


def get_pipeline_status(pipeline_id: str) -> Dict[str, Any]:
    """
    Get current status of a pipeline.
    
    Reference: API_DESIGN_SPECIFICATION.md Section 3.7
    
    Returns pipeline state including:
    - Overall status
    - Current stage
    - Progress percentage
    - Timing information
    """
    with _pipeline_lock:
        if pipeline_id not in _active_pipelines:
            return {
                "error": "Pipeline not found",
                "pipeline_id": pipeline_id
            }
        
        # Return copy to avoid external modifications
        state = _active_pipelines[pipeline_id].copy()
    
    # Calculate elapsed time
    if "started_timestamp" in state:
        if state["status"] in ["completed", "failed", "error", "cancelled"]:
            elapsed = state.get("duration_seconds", 0)
        else:
            elapsed = time.time() - state["started_timestamp"]
        state["elapsed_seconds"] = int(elapsed)
    
    # Remove internal fields
    state.pop("started_timestamp", None)
    state.pop("completed_timestamp", None)
    
    return state


def cancel_pipeline(pipeline_id: str) -> Dict[str, Any]:
    """
    Cancel a running pipeline.
    
    Note: This marks the pipeline as cancelled. The running thread
    should check this status and stop gracefully.
    """
    with _pipeline_lock:
        if pipeline_id not in _active_pipelines:
            return {
                "error": "Pipeline not found",
                "pipeline_id": pipeline_id
            }
        
        state = _active_pipelines[pipeline_id]
        
        if state["status"] not in ["starting", "running"]:
            return {
                "error": "Pipeline is not running",
                "pipeline_id": pipeline_id,
                "current_status": state["status"]
            }
        
        # Mark as cancelled
        state["status"] = "cancelled"
        state["completed_at"] = datetime.utcnow().isoformat() + "Z"
        state["completed_timestamp"] = time.time()
        state["duration_seconds"] = state["completed_timestamp"] - state["started_timestamp"]
        
        return {
            "pipeline_id": pipeline_id,
            "status": "cancelled",
            "message": "Pipeline cancellation requested"
        }


def list_active_pipelines() -> Dict[str, Any]:
    """List all active (running/starting) pipelines"""
    with _pipeline_lock:
        active = {
            pid: {
                "pipeline_id": state["pipeline_id"],
                "pipeline": state["pipeline"],
                "status": state["status"],
                "started_at": state["started_at"],
                "current_stage": state["current_stage"],
                "progress": state["progress"]
            }
            for pid, state in _active_pipelines.items()
            if state["status"] in ["starting", "running"]
        }
    
    return {
        "count": len(active),
        "pipelines": active
    }


def get_pipeline_history(limit: int = 10) -> Dict[str, Any]:
    """Get recent pipeline executions"""
    with _pipeline_lock:
        all_pipelines = list(_active_pipelines.values())
    
    # Sort by start time (most recent first)
    all_pipelines.sort(key=lambda x: x.get("started_timestamp", 0), reverse=True)
    
    # Limit results
    recent = all_pipelines[:limit]
    
    # Clean up for response
    result = []
    for state in recent:
        result.append({
            "pipeline_id": state["pipeline_id"],
            "pipeline": state["pipeline"],
            "status": state["status"],
            "started_at": state["started_at"],
            "completed_at": state.get("completed_at"),
            "stages": state["stages"],
            "progress": state["progress"]
        })
    
    return {
        "total": len(all_pipelines),
        "returned": len(result),
        "pipelines": result
    }


# ============================================
# Background Execution
# ============================================

def _run_pipeline_background(
    pipeline_id: str,
    pipeline: str,
    stages: List[str],
    config: Dict[str, Dict[str, Any]],
    metadata: Optional[Dict[str, Any]]
):
    """
    Run pipeline in background thread.
    
    Reference: API_DESIGN_SPECIFICATION.md Section 7.2
    """
    try:
        # Update status to running
        _update_pipeline_status(pipeline_id, "running")
        
        # Create pipeline object
        pipeline_obj = _create_pipeline_object(pipeline, stages, config, metadata)
        
        if pipeline_obj is None:
            _update_pipeline_error(pipeline_id, "Failed to create pipeline object", None)
            return
        
        # Execute pipeline
        logger.info(f"[{pipeline_id}] Starting pipeline execution: {stages}")
        
        exit_code = pipeline_obj.run_full_pipeline()
        
        # Update final status
        _update_pipeline_completion(pipeline_id, exit_code, stages)
        
        logger.info(f"[{pipeline_id}] Pipeline completed with exit code: {exit_code}")
    
    except Exception as e:
        logger.exception(f"[{pipeline_id}] Pipeline execution failed")
        _update_pipeline_error(pipeline_id, str(e), None)


def _create_pipeline_object(
    pipeline: str,
    stages: List[str],
    config: Dict[str, Dict[str, Any]],
    metadata: Optional[Dict[str, Any]]
):
    """Create pipeline object from configuration"""
    import argparse
    
    try:
        if pipeline == "graphrag":
            from business.pipelines.graphrag import GraphRAGPipeline
            from core.config.graphrag import GraphRAGPipelineConfig
            
            # Create args namespace
            args = argparse.Namespace()
            env = dict(os.environ)
            
            # Apply metadata
            if metadata and "experiment_id" in metadata:
                env["EXPERIMENT_ID"] = metadata["experiment_id"]
            
            # Apply stage configs to env/args
            _apply_config_to_args_env(config, args, env)
            
            # Create pipeline config
            pipeline_config = GraphRAGPipelineConfig.from_args_env(
                args, env, os.getenv("DB_NAME", "mongo_hack")
            )
            
            # Set selected stages
            pipeline_config.selected_stages = ",".join(stages)
            
            return GraphRAGPipeline(pipeline_config)
        
        elif pipeline == "ingestion":
            from business.pipelines.ingestion import IngestionPipeline, IngestionPipelineConfig
            
            args = argparse.Namespace()
            env = dict(os.environ)
            
            _apply_config_to_args_env(config, args, env)
            
            pipeline_config = IngestionPipelineConfig.from_args_env(
                args, env, os.getenv("DB_NAME", "mongo_hack")
            )
            
            return IngestionPipeline(pipeline_config)
        
        else:
            logger.error(f"Unknown pipeline type: {pipeline}")
            return None
    
    except Exception as e:
        logger.exception(f"Failed to create pipeline object: {e}")
        return None


def _apply_config_to_args_env(
    config: Dict[str, Dict[str, Any]],
    args: argparse.Namespace,
    env: Dict[str, str]
):
    """Apply stage configuration to args and environment"""
    for stage_name, stage_config in config.items():
        for key, value in stage_config.items():
            # Set on args namespace
            setattr(args, key, value)
            
            # Also set in env for stages that read from env
            env_key = f"{stage_name.upper()}_{key.upper()}"
            if value is not None:
                env[env_key] = str(value)


def _update_pipeline_status(pipeline_id: str, status: str):
    """Update pipeline status"""
    with _pipeline_lock:
        if pipeline_id in _active_pipelines:
            _active_pipelines[pipeline_id]["status"] = status


def _update_pipeline_completion(pipeline_id: str, exit_code: int, stages: List[str]):
    """Update pipeline on completion"""
    with _pipeline_lock:
        if pipeline_id in _active_pipelines:
            state = _active_pipelines[pipeline_id]
            state["status"] = "completed" if exit_code == 0 else "failed"
            state["exit_code"] = exit_code
            state["completed_at"] = datetime.utcnow().isoformat() + "Z"
            state["completed_timestamp"] = time.time()
            state["duration_seconds"] = state["completed_timestamp"] - state["started_timestamp"]
            state["completed_stages"] = stages if exit_code == 0 else state.get("completed_stages", [])
            state["progress"]["completed_stages"] = len(stages) if exit_code == 0 else state["progress"]["completed_stages"]
            state["progress"]["percent"] = 100.0 if exit_code == 0 else state["progress"]["percent"]


def _update_pipeline_error(pipeline_id: str, error: str, stage: Optional[str]):
    """Update pipeline on error"""
    with _pipeline_lock:
        if pipeline_id in _active_pipelines:
            state = _active_pipelines[pipeline_id]
            state["status"] = "error"
            state["error"] = error
            state["error_stage"] = stage
            state["completed_at"] = datetime.utcnow().isoformat() + "Z"
            state["completed_timestamp"] = time.time()
            state["duration_seconds"] = state["completed_timestamp"] - state["started_timestamp"]
```

**Validation:**
- [ ] Pipeline execution starts in background thread
- [ ] Status updates correctly (starting → running → completed/failed)
- [ ] Progress tracking works
- [ ] Cancellation marks pipeline as cancelled
- [ ] Error handling captures exceptions

---

## Phase 5: API Integration

### Step 5.1: Create API Entry Point

**Objective:** Create unified API module that exposes all functions

**Files to Create:**
- `app/stages-api/api.py`

**Tasks:**

1. Create `api.py` as the main API module:

```python
"""
Stages API - Main Entry Point

This module provides the public API functions that can be:
1. Called directly from Python
2. Exposed via HTTP server
3. Integrated with existing API infrastructure

Reference: API_DESIGN_SPECIFICATION.md Section 3.1
"""

import logging
from typing import Any, Dict, List, Optional

from .metadata import (
    list_stages,
    get_stage_config,
    get_stage_defaults,
    list_pipeline_stages,
    clear_metadata_cache
)
from .validation import (
    validate_pipeline_config,
    validate_stage_config_only
)
from .execution import (
    execute_pipeline,
    get_pipeline_status,
    cancel_pipeline,
    list_active_pipelines,
    get_pipeline_history
)

logger = logging.getLogger(__name__)

__all__ = [
    # Metadata
    "list_stages",
    "get_stage_config",
    "get_stage_defaults",
    "list_pipeline_stages",
    "clear_metadata_cache",
    
    # Validation
    "validate_pipeline_config",
    "validate_stage_config_only",
    
    # Execution
    "execute_pipeline",
    "get_pipeline_status",
    "cancel_pipeline",
    "list_active_pipelines",
    "get_pipeline_history",
    
    # HTTP handlers
    "handle_request"
]


def handle_request(method: str, path: str, body: Optional[Dict] = None) -> tuple:
    """
    Handle HTTP-like requests.
    
    This function routes requests to appropriate handlers and returns
    (response_data, status_code) tuples.
    
    Can be used by:
    - SimpleHTTPServer
    - Flask/FastAPI adapters
    - Direct Python calls
    
    Args:
        method: HTTP method (GET, POST)
        path: URL path (e.g., "/stages", "/stages/graph_extraction/config")
        body: Request body for POST requests
    
    Returns:
        Tuple of (response_dict, status_code)
    """
    try:
        # Remove leading slash and split path
        path = path.lstrip("/")
        parts = path.split("/")
        
        # Route: GET /stages
        if method == "GET" and path == "stages":
            return list_stages(), 200
        
        # Route: GET /stages/{pipeline}
        if method == "GET" and len(parts) == 2 and parts[0] == "stages":
            pipeline = parts[1]
            if pipeline in ["ingestion", "graphrag"]:
                return list_pipeline_stages(pipeline), 200
            # Otherwise, treat as stage name for /stages/{stage_name}/config
        
        # Route: GET /stages/{stage_name}/config
        if method == "GET" and len(parts) == 3 and parts[0] == "stages" and parts[2] == "config":
            stage_name = parts[1]
            try:
                return get_stage_config(stage_name), 200
            except ValueError as e:
                return {"error": str(e), "stage_name": stage_name}, 404
        
        # Route: GET /stages/{stage_name}/defaults
        if method == "GET" and len(parts) == 3 and parts[0] == "stages" and parts[2] == "defaults":
            stage_name = parts[1]
            try:
                return get_stage_defaults(stage_name), 200
            except ValueError as e:
                return {"error": str(e), "stage_name": stage_name}, 404
        
        # Route: POST /pipelines/validate
        if method == "POST" and path == "pipelines/validate":
            if not body:
                return {"error": "Request body required"}, 400
            
            pipeline = body.get("pipeline")
            stages = body.get("stages", [])
            config = body.get("config", {})
            
            if not pipeline:
                return {"error": "pipeline field is required"}, 400
            if not stages:
                return {"error": "stages field is required"}, 400
            
            return validate_pipeline_config(pipeline, stages, config), 200
        
        # Route: POST /pipelines/execute
        if method == "POST" and path == "pipelines/execute":
            if not body:
                return {"error": "Request body required"}, 400
            
            pipeline = body.get("pipeline")
            stages = body.get("stages", [])
            config = body.get("config", {})
            metadata = body.get("metadata", {})
            
            if not pipeline:
                return {"error": "pipeline field is required"}, 400
            if not stages:
                return {"error": "stages field is required"}, 400
            
            result = execute_pipeline(pipeline, stages, config, metadata)
            
            if "error" in result:
                return result, 400
            return result, 202  # Accepted
        
        # Route: GET /pipelines/{pipeline_id}/status
        if method == "GET" and len(parts) == 3 and parts[0] == "pipelines" and parts[2] == "status":
            pipeline_id = parts[1]
            result = get_pipeline_status(pipeline_id)
            
            if "error" in result:
                return result, 404
            return result, 200
        
        # Route: POST /pipelines/{pipeline_id}/cancel
        if method == "POST" and len(parts) == 3 and parts[0] == "pipelines" and parts[2] == "cancel":
            pipeline_id = parts[1]
            result = cancel_pipeline(pipeline_id)
            
            if "error" in result:
                return result, 400
            return result, 200
        
        # Route: GET /pipelines/active
        if method == "GET" and path == "pipelines/active":
            return list_active_pipelines(), 200
        
        # Route: GET /pipelines/history
        if method == "GET" and path == "pipelines/history":
            limit = 10  # Could be parsed from query string
            return get_pipeline_history(limit), 200
        
        # Not found
        return {"error": f"Unknown endpoint: {method} /{path}"}, 404
    
    except Exception as e:
        logger.exception(f"Error handling request: {method} /{path}")
        return {"error": "Internal server error", "message": str(e)}, 500
```

**Validation:**
- [ ] All routes work correctly
- [ ] Error handling returns appropriate status codes
- [ ] Request body parsing works

---

### Step 5.2: Create HTTP Server (Optional)

**Objective:** Create standalone HTTP server for testing

**Files to Create:**
- `app/stages-api/server.py`

**Tasks:**

```python
"""
Stages API HTTP Server

Simple HTTP server for testing the Stages API.
For production, integrate with existing API infrastructure.

Usage:
    python -m app.stages_api.server --port 8080
"""

import argparse
import json
import logging
from http.server import HTTPServer, BaseHTTPRequestHandler
from urllib.parse import urlparse, parse_qs

from .api import handle_request

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


class StagesAPIHandler(BaseHTTPRequestHandler):
    """HTTP request handler for Stages API"""
    
    def do_GET(self):
        """Handle GET requests"""
        parsed = urlparse(self.path)
        path = parsed.path.replace("/api/v1/", "").replace("/api/", "")
        
        result, status = handle_request("GET", path)
        self._send_json(result, status)
    
    def do_POST(self):
        """Handle POST requests"""
        parsed = urlparse(self.path)
        path = parsed.path.replace("/api/v1/", "").replace("/api/", "")
        
        # Read request body
        content_length = int(self.headers.get("Content-Length", 0))
        body = None
        if content_length > 0:
            body_bytes = self.rfile.read(content_length)
            try:
                body = json.loads(body_bytes.decode("utf-8"))
            except json.JSONDecodeError:
                self._send_json({"error": "Invalid JSON body"}, 400)
                return
        
        result, status = handle_request("POST", path, body)
        self._send_json(result, status)
    
    def do_OPTIONS(self):
        """Handle CORS preflight requests"""
        self.send_response(200)
        self._add_cors_headers()
        self.end_headers()
    
    def _send_json(self, data, status=200):
        """Send JSON response"""
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self._add_cors_headers()
        self.end_headers()
        self.wfile.write(json.dumps(data, indent=2).encode())
    
    def _add_cors_headers(self):
        """Add CORS headers for browser access"""
        self.send_header("Access-Control-Allow-Origin", "*")
        self.send_header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
        self.send_header("Access-Control-Allow-Headers", "Content-Type")
    
    def log_message(self, format, *args):
        """Custom logging"""
        logger.info(f"{self.address_string()} - {format % args}")


def run_server(host: str = "0.0.0.0", port: int = 8080):
    """Run the HTTP server"""
    server = HTTPServer((host, port), StagesAPIHandler)
    logger.info(f"Stages API server running on http://{host}:{port}")
    logger.info(f"Endpoints:")
    logger.info(f"  GET  /api/v1/stages")
    logger.info(f"  GET  /api/v1/stages/{{stage_name}}/config")
    logger.info(f"  GET  /api/v1/stages/{{stage_name}}/defaults")
    logger.info(f"  POST /api/v1/pipelines/validate")
    logger.info(f"  POST /api/v1/pipelines/execute")
    logger.info(f"  GET  /api/v1/pipelines/{{pipeline_id}}/status")
    logger.info(f"  POST /api/v1/pipelines/{{pipeline_id}}/cancel")
    
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        logger.info("Server stopped")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Stages API Server")
    parser.add_argument("--host", default="0.0.0.0", help="Host to bind to")
    parser.add_argument("--port", type=int, default=8080, help="Port to listen on")
    args = parser.parse_args()
    
    run_server(args.host, args.port)
```

**Validation:**
- [ ] Server starts and accepts connections
- [ ] All endpoints respond correctly
- [ ] CORS headers allow browser access
- [ ] JSON parsing works correctly

---

## Phase 6: Testing

### Step 6.1: Create Test Suite

**Objective:** Write comprehensive tests for all modules

**Files to Create:**
- `app/stages-api/tests/__init__.py`
- `app/stages-api/tests/test_metadata.py`
- `app/stages-api/tests/test_validation.py`
- `app/stages-api/tests/test_execution.py`
- `app/stages-api/tests/test_api.py`

**Reference:** API_DESIGN_SPECIFICATION.md Sections 9.2, 10.1

**Tasks:** Create test files following the patterns in API_DESIGN_SPECIFICATION.md Section 9.2.

---

## Implementation Checklist

### Phase 1: Project Setup ✅
- [ ] Create `__init__.py`
- [ ] Create `constants.py`
- [ ] Create `field_metadata.py`

### Phase 2: Metadata System ✅
- [ ] Create `metadata.py`
- [ ] Implement `list_stages()`
- [ ] Implement `get_stage_config()`
- [ ] Implement `get_stage_defaults()`
- [ ] Implement caching

### Phase 3: Validation System ✅
- [ ] Create `validation.py`
- [ ] Implement pipeline validation
- [ ] Implement type validation
- [ ] Implement dependency validation
- [ ] Implement value range validation

### Phase 4: Execution System ✅
- [ ] Create `execution.py`
- [ ] Implement `execute_pipeline()`
- [ ] Implement `get_pipeline_status()`
- [ ] Implement `cancel_pipeline()`
- [ ] Implement background execution

### Phase 5: API Integration ✅
- [ ] Create `api.py`
- [ ] Implement request routing
- [ ] Create `server.py`
- [ ] Test all endpoints

### Phase 6: Testing ✅
- [ ] Create test suite
- [ ] Write unit tests
- [ ] Write integration tests
- [ ] Achieve 80%+ coverage

---

## Quick Start Commands

```bash
# Run the API server
python -m app.stages_api.server --port 8080

# Test endpoints
curl http://localhost:8080/api/v1/stages
curl http://localhost:8080/api/v1/stages/graph_extraction/config
curl http://localhost:8080/api/v1/stages/graph_extraction/defaults

# Validate a pipeline configuration
curl -X POST http://localhost:8080/api/v1/pipelines/validate \
  -H "Content-Type: application/json" \
  -d '{"pipeline": "graphrag", "stages": ["graph_extraction"], "config": {}}'

# Execute a pipeline
curl -X POST http://localhost:8080/api/v1/pipelines/execute \
  -H "Content-Type: application/json" \
  -d '{"pipeline": "graphrag", "stages": ["graph_extraction"], "config": {"graph_extraction": {"max": 10}}, "metadata": {"experiment_id": "test_001"}}'

# Check pipeline status
curl http://localhost:8080/api/v1/pipelines/{pipeline_id}/status

# Run tests
pytest app/stages-api/tests/ -v
```

---

## Notes for UI Integration

The API is designed to support the UI requirements defined in the technical foundation document:

1. **Stage Discovery**: `GET /stages` provides all information needed for pipeline/stage selection
2. **Dynamic Forms**: `GET /stages/{name}/config` provides complete field metadata for form generation
3. **Validation**: `POST /pipelines/validate` should be called before execution to show warnings
4. **Execution**: `POST /pipelines/execute` starts pipeline and returns tracking URL
5. **Status Polling**: `GET /pipelines/{id}/status` should be polled for progress updates

---

**End of Implementation Plan**

