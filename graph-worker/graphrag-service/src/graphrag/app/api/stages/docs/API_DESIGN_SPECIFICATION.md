# Stages API - Detailed Design Specification

**Version:** 1.0  
**Created:** December 9, 2025  
**Purpose:** Comprehensive technical specification for implementing the Stages Configuration API

---

## Table of Contents

1. [Overview](#1-overview)
2. [Architecture](#2-architecture)
3. [API Endpoints Specification](#3-api-endpoints-specification)
4. [Data Models and Schemas](#4-data-models-and-schemas)
5. [Metadata System](#5-metadata-system)
6. [Validation System](#6-validation-system)
7. [Execution System](#7-execution-system)
8. [Error Handling](#8-error-handling)
9. [Implementation Guide](#9-implementation-guide)
10. [Testing Strategy](#10-testing-strategy)

---

## 1. Overview

### 1.1 Purpose

The Stages API provides a RESTful interface for:
- **Discovery:** List available pipeline stages and their configurations
- **Configuration:** Retrieve stage configuration schemas with metadata for UI rendering
- **Validation:** Validate pipeline configurations before execution
- **Execution:** Start, monitor, and control pipeline executions
- **Status:** Track real-time pipeline progress and results

### 1.2 Design Principles

1. **Introspection-First:** Extract metadata from existing dataclasses, no duplication
2. **Type Safety:** Leverage Python type hints for validation
3. **RESTful:** Follow REST conventions for predictable API behavior
4. **Error-Friendly:** Provide detailed error messages for debugging
5. **Backward Compatible:** Integrate seamlessly with existing pipeline infrastructure

### 1.3 Technology Stack

- **Language:** Python 3.10+
- **Data Validation:** Python dataclasses + type hints
- **Introspection:** `dataclasses.fields()`, `typing.get_type_hints()`
- **Concurrency:** Threading for background execution
- **Database:** MongoDB (via existing `get_mongo_client()`)
- **Serialization:** JSON

---

## 2. Architecture

### 2.1 Module Structure

```
app/stages-api/
├── __init__.py              # Package initialization
├── metadata.py              # Stage metadata extraction
├── validation.py            # Configuration validation
├── execution.py             # Pipeline execution management
├── schemas.py               # Pydantic/dataclass schemas for API
├── server.py                # HTTP server (optional, for testing)
├── utils.py                 # Utility functions
└── tests/                   # Unit tests
    ├── test_metadata.py
    ├── test_validation.py
    └── test_execution.py
```

### 2.2 Component Diagram

```
┌─────────────────────────────────────────────────────────┐
│                     HTTP Layer                           │
│              (External HTTP Server or                    │
│               Built-in SimpleHTTPServer)                 │
└───────────────────┬─────────────────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────────────────┐
│                  API Functions                           │
│  - list_stages()                                         │
│  - get_stage_config()                                    │
│  - validate_pipeline_config()                            │
│  - execute_pipeline()                                    │
│  - get_pipeline_status()                                 │
└───────┬──────────────┬──────────────┬───────────────────┘
        │              │              │
        ▼              ▼              ▼
┌────────────┐  ┌────────────┐  ┌────────────┐
│ metadata.py│  │validation.py│  │execution.py│
└─────┬──────┘  └─────┬──────┘  └─────┬──────┘
      │               │               │
      ▼               ▼               ▼
┌─────────────────────────────────────────────────────────┐
│           Existing Infrastructure                        │
│  - STAGE_REGISTRY (business/pipelines/runner.py)        │
│  - Stage Classes (business/stages/)                     │
│  - Config Classes (core/config/)                        │
│  - PipelineRunner (business/pipelines/runner.py)        │
└─────────────────────────────────────────────────────────┘
```

### 2.3 Data Flow

```
Client Request
    ↓
API Function (e.g., get_stage_config)
    ↓
Metadata Extraction (metadata.py)
    ↓
Config Schema Generation (introspection)
    ↓
JSON Response
```

---

## 3. API Endpoints Specification

### 3.1 Endpoint Overview

**Base URL:** `/api/v1/`

| Endpoint | Method | Function | Module | Status |
|----------|--------|----------|--------|--------|
| `/health` | GET | Health check | api.py | ✅ Implemented |
| `/stages` | GET | `list_stages()` | metadata.py | ✅ Implemented |
| `/stages/{pipeline}` | GET | `list_pipeline_stages()` | metadata.py | ✅ Implemented |
| `/stages/{stage_name}/config` | GET | `get_stage_config()` | metadata.py | ✅ Implemented |
| `/stages/{stage_name}/defaults` | GET | `get_stage_defaults()` | metadata.py | ✅ Implemented |
| `/stages/{stage_name}/validate` | POST | `validate_stage_config_only()` | validation.py | ✅ Implemented |
| `/pipelines/validate` | POST | `validate_pipeline_config()` | validation.py | ✅ Implemented |
| `/pipelines/execute` | POST | `execute_pipeline()` | execution.py | ✅ Implemented |
| `/pipelines/{pipeline_id}/status` | GET | `get_pipeline_status()` | execution.py | ✅ Implemented |
| `/pipelines/{pipeline_id}/cancel` | POST | `cancel_pipeline()` | execution.py | ✅ Implemented |
| `/pipelines/active` | GET | `list_active_pipelines()` | execution.py | ✅ Implemented |
| `/pipelines/history` | GET | `get_pipeline_history()` | execution.py | ✅ Implemented |

---

### 3.2 GET `/stages` - List All Stages

#### Purpose
Retrieve a complete list of available stages grouped by pipeline type.

#### Implementation
```python
# File: app/stages-api/metadata.py

from typing import Dict, Any, List
from business.pipelines.runner import STAGE_REGISTRY

# Pipeline grouping (could be extracted to config)
PIPELINE_GROUPS = {
    "ingestion": ["ingest", "clean", "chunk", "enrich", "embed", "redundancy", "trust"],
    "graphrag": ["graph_extraction", "entity_resolution", "graph_construction", "community_detection"]
}

def list_stages() -> Dict[str, Any]:
    """
    List all available stages grouped by pipeline.
    
    Returns:
        Dictionary containing pipeline metadata and stage information
    """
    result = {
        "pipelines": {},
        "stages": {}
    }
    
    # Build pipeline metadata
    for pipeline_name, stage_list in PIPELINE_GROUPS.items():
        result["pipelines"][pipeline_name] = {
            "name": _format_pipeline_name(pipeline_name),
            "description": _get_pipeline_description(pipeline_name),
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
            "description": stage_cls.description or "",
            "pipeline": pipeline,
            "config_class": stage_cls.ConfigCls.__name__,
            "dependencies": dependencies,
            "has_llm": _stage_uses_llm(stage_cls),
        }
    
    return result

def _format_pipeline_name(pipeline: str) -> str:
    """Convert pipeline key to display name"""
    return {
        "ingestion": "Ingestion Pipeline",
        "graphrag": "GraphRAG Pipeline"
    }.get(pipeline, pipeline.title())

def _get_pipeline_description(pipeline: str) -> str:
    """Get pipeline description"""
    return {
        "ingestion": "Process raw video data through cleaning, chunking, enrichment, and embedding",
        "graphrag": "Build knowledge graph from processed chunks with entity resolution and community detection"
    }.get(pipeline, "")

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
    
    from business.pipelines.graphrag import STAGE_DEPENDENCIES
    return STAGE_DEPENDENCIES.get(stage_name, [])

def _stage_uses_llm(stage_cls) -> bool:
    """Check if stage uses LLM by inspecting config"""
    config_cls = stage_cls.ConfigCls
    # Check if config has model_name or llm field
    from dataclasses import fields
    field_names = [f.name for f in fields(config_cls)]
    return "model_name" in field_names or "llm" in field_names
```

#### Response Schema
```json
{
  "pipelines": {
    "<pipeline_name>": {
      "name": "string",
      "description": "string",
      "stages": ["string"],
      "stage_count": "integer"
    }
  },
  "stages": {
    "<stage_name>": {
      "name": "string",
      "display_name": "string",
      "description": "string",
      "pipeline": "string",
      "config_class": "string",
      "dependencies": ["string"],
      "has_llm": "boolean"
    }
  }
}
```

#### Example Response
```json
{
  "pipelines": {
    "graphrag": {
      "name": "GraphRAG Pipeline",
      "description": "Build knowledge graph from processed chunks with entity resolution and community detection",
      "stages": ["graph_extraction", "entity_resolution", "graph_construction", "community_detection"],
      "stage_count": 4
    }
  },
  "stages": {
    "graph_extraction": {
      "name": "graph_extraction",
      "display_name": "Graph Extraction",
      "description": "Extract entities and relationships from text chunks",
      "pipeline": "graphrag",
      "config_class": "GraphExtractionConfig",
      "dependencies": [],
      "has_llm": true
    }
  }
}
```

#### Error Responses
- **500 Internal Server Error:** If STAGE_REGISTRY is not accessible
  ```json
  {
    "error": "Failed to load stage registry",
    "details": "Error message"
  }
  ```

---

### 3.3 GET `/stages/{stage_name}/config` - Get Stage Configuration Schema

#### Purpose
Retrieve detailed configuration schema for a specific stage, including field types, defaults, and UI metadata.

#### Implementation
```python
# File: app/stages-api/metadata.py

from dataclasses import fields, is_dataclass, MISSING
from typing import Any, Dict, get_type_hints, get_origin, get_args
import inspect

def get_stage_config(stage_name: str) -> Dict[str, Any]:
    """
    Get configuration schema for a specific stage.
    
    Args:
        stage_name: Name of the stage (e.g., "graph_extraction")
    
    Returns:
        Dictionary containing stage config schema
    
    Raises:
        ValueError: If stage not found or has no config class
    """
    # Validate stage exists
    stage_cls = STAGE_REGISTRY.get(stage_name)
    if not stage_cls:
        raise ValueError(f"Unknown stage: {stage_name}")
    
    config_cls = getattr(stage_cls, "ConfigCls", None)
    if not config_cls:
        raise ValueError(f"Stage {stage_name} has no ConfigCls")
    
    # Extract schema
    schema = _extract_config_schema(config_cls)
    
    return {
        "stage_name": stage_name,
        "config_class": config_cls.__name__,
        "description": config_cls.__doc__ or "",
        "fields": schema["fields"],
        "categories": schema["categories"],
        "field_count": len(schema["fields"])
    }

def _extract_config_schema(config_cls) -> Dict[str, Any]:
    """
    Extract configuration schema from dataclass using introspection.
    
    This is the core metadata extraction function.
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
        
        # Get field metadata
        metadata = field.metadata if hasattr(field, "metadata") else {}
        
        # Determine if field is from parent class
        is_inherited = field.name in parent_fields
        
        # Build field info
        field_info = {
            "name": field.name,
            "type": type_info["type"],
            "python_type": type_info["python_type"],
            "default": default_value,
            "required": not type_info["optional"] and default_value is None,
            "optional": type_info["optional"],
            "description": metadata.get("description", ""),
            "category": metadata.get("category", _infer_category(field.name, is_inherited)),
            "ui_type": metadata.get("ui_type", _infer_ui_type(type_info["python_type"])),
            "is_inherited": is_inherited,
        }
        
        # Add UI-specific metadata
        for key in ["min", "max", "step", "options", "placeholder", "pattern"]:
            if key in metadata:
                field_info[key] = metadata[key]
        
        schema_fields.append(field_info)
    
    # Group fields by category
    categories = _group_by_category(schema_fields)
    
    return {
        "fields": schema_fields,
        "categories": categories,
    }

def _parse_type(field_type) -> Dict[str, Any]:
    """
    Parse Python type hint into API-friendly format.
    
    Handles:
    - Optional[T] → {"type": "T", "optional": True}
    - List[T] → {"type": "array", "item_type": "T"}
    - Union types
    - Basic types (str, int, float, bool)
    """
    origin = get_origin(field_type)
    args = get_args(field_type)
    
    # Handle Optional[T] (which is Union[T, None])
    if origin is type(None) or (origin and args and type(None) in args):
        # Optional type
        actual_type = args[0] if args else field_type
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
    
    # Handle Union types (not Optional)
    if origin is type(None) or str(origin).startswith("typing.Union"):
        # Pick first non-None type
        non_none_types = [t for t in args if t is not type(None)]
        actual_type = non_none_types[0] if non_none_types else str
        return {
            "type": _get_type_name(actual_type),
            "python_type": actual_type,
            "optional": True
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
    
    # Map Python types to API types
    type_map = {
        "str": "string",
        "int": "integer",
        "float": "number",
        "bool": "boolean",
        "list": "array",
        "dict": "object",
    }
    
    return type_map.get(name, name)

def _infer_category(field_name: str, is_inherited: bool) -> str:
    """
    Infer field category from field name.
    
    Categories:
    - Common Fields: Inherited from BaseStageConfig
    - LLM Settings: model_name, temperature, etc.
    - Processing: concurrency, batch_size, timeout
    - Quality: thresholds, confidence scores
    - Algorithm: algorithm-specific parameters
    """
    if is_inherited:
        return "Common Fields"
    
    # Category patterns
    llm_patterns = ["model", "temperature", "token", "llm", "prompt"]
    processing_patterns = ["concurrency", "batch", "timeout", "max", "chunk"]
    quality_patterns = ["threshold", "confidence", "score", "coherence"]
    algorithm_patterns = ["algorithm", "resolution", "cluster", "strategy"]
    database_patterns = ["db", "collection", "coll"]
    
    name_lower = field_name.lower()
    
    if any(p in name_lower for p in llm_patterns):
        return "LLM Settings"
    elif any(p in name_lower for p in processing_patterns):
        return "Processing"
    elif any(p in name_lower for p in quality_patterns):
        return "Quality Thresholds"
    elif any(p in name_lower for p in algorithm_patterns):
        return "Algorithm Parameters"
    elif any(p in name_lower for p in database_patterns):
        return "Database Configuration"
    else:
        return "General"

def _infer_ui_type(python_type) -> str:
    """
    Infer UI input type from Python type.
    
    Returns: text, number, checkbox, select, slider, etc.
    """
    if python_type == bool:
        return "checkbox"
    elif python_type == int:
        return "number"
    elif python_type == float:
        return "slider"  # Floats often represent ratios/percentages
    elif python_type == str:
        return "text"
    elif python_type == list:
        return "multiselect"
    else:
        return "text"

def _group_by_category(fields: List[Dict]) -> List[Dict[str, Any]]:
    """Group fields by category for UI rendering"""
    categories = {}
    for field in fields:
        cat = field["category"]
        if cat not in categories:
            categories[cat] = []
        categories[cat].append(field["name"])
    
    # Sort categories (Common Fields first, then alphabetically)
    category_order = ["Common Fields", "LLM Settings", "Processing", "Quality Thresholds", 
                      "Algorithm Parameters", "Database Configuration", "General"]
    
    sorted_categories = []
    for cat_name in category_order:
        if cat_name in categories:
            sorted_categories.append({
                "name": cat_name,
                "fields": categories[cat_name],
                "field_count": len(categories[cat_name])
            })
    
    # Add any remaining categories not in the predefined order
    for cat_name, field_list in categories.items():
        if cat_name not in category_order:
            sorted_categories.append({
                "name": cat_name,
                "fields": field_list,
                "field_count": len(field_list)
            })
    
    return sorted_categories
```

#### Response Schema
```json
{
  "stage_name": "string",
  "config_class": "string",
  "description": "string",
  "fields": [
    {
      "name": "string",
      "type": "string",
      "python_type": "string",
      "default": "any",
      "required": "boolean",
      "optional": "boolean",
      "description": "string",
      "category": "string",
      "ui_type": "string",
      "is_inherited": "boolean",
      "min": "number (optional)",
      "max": "number (optional)",
      "step": "number (optional)",
      "options": ["string"] (optional),
      "placeholder": "string (optional)"
    }
  ],
  "categories": [
    {
      "name": "string",
      "fields": ["string"],
      "field_count": "integer"
    }
  ],
  "field_count": "integer"
}
```

#### Example Response
```json
{
  "stage_name": "graph_extraction",
  "config_class": "GraphExtractionConfig",
  "description": "Configuration for graph extraction stage",
  "fields": [
    {
      "name": "model_name",
      "type": "string",
      "python_type": "str",
      "default": "gpt-4o-mini",
      "required": false,
      "optional": false,
      "description": "OpenAI model name for entity extraction",
      "category": "LLM Settings",
      "ui_type": "select",
      "is_inherited": false,
      "options": ["gpt-4o-mini", "gpt-4", "gpt-3.5-turbo"]
    },
    {
      "name": "temperature",
      "type": "number",
      "python_type": "float",
      "default": 0.1,
      "required": false,
      "optional": false,
      "description": "LLM temperature (0-2, lower = more deterministic)",
      "category": "LLM Settings",
      "ui_type": "slider",
      "is_inherited": false,
      "min": 0.0,
      "max": 2.0,
      "step": 0.1
    },
    {
      "name": "max",
      "type": "integer",
      "python_type": "int",
      "default": null,
      "required": false,
      "optional": true,
      "description": "Maximum number of documents to process",
      "category": "Common Fields",
      "ui_type": "number",
      "is_inherited": true,
      "min": 1
    }
  ],
  "categories": [
    {
      "name": "Common Fields",
      "fields": ["max", "verbose", "dry_run", "db_name"],
      "field_count": 4
    },
    {
      "name": "LLM Settings",
      "fields": ["model_name", "temperature", "max_tokens", "llm_retries"],
      "field_count": 4
    }
  ],
  "field_count": 20
}
```

#### Error Responses
- **404 Not Found:** Stage doesn't exist
  ```json
  {
    "error": "Stage not found",
    "stage_name": "invalid_stage"
  }
  ```

---

### 3.4 GET `/stages/{stage_name}/defaults` - Get Default Configuration

#### Purpose
Get a complete default configuration object for a stage (ready to use as-is).

#### Implementation
```python
# File: app/stages-api/metadata.py

def get_stage_defaults(stage_name: str) -> Dict[str, Any]:
    """
    Get default configuration for a stage.
    
    Returns a complete config object with all defaults populated.
    This can be used directly or as a template for modifications.
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

def _config_to_dict(config_obj) -> Dict[str, Any]:
    """Convert dataclass config to dictionary, handling special types"""
    from dataclasses import asdict
    
    try:
        # asdict handles nested dataclasses
        return asdict(config_obj)
    except Exception:
        # Fallback: manual conversion
        result = {}
        for field in fields(config_obj):
            value = getattr(config_obj, field.name)
            # Skip None values to keep response clean
            if value is not None:
                result[field.name] = value
        return result
```

#### Response Schema
```json
{
  "stage_name": "string",
  "config_class": "string",
  "config": {
    "<field_name>": "any"
  }
}
```

#### Example Response
```json
{
  "stage_name": "graph_extraction",
  "config_class": "GraphExtractionConfig",
  "config": {
    "model_name": "gpt-4o-mini",
    "temperature": 0.1,
    "max_tokens": null,
    "llm_retries": 3,
    "llm_backoff_s": 1.0,
    "max_entities_per_chunk": 20,
    "max_relationships_per_chunk": 30,
    "min_entity_confidence": 0.3,
    "batch_size": 50,
    "verbose": false,
    "dry_run": false
  }
}
```

---

### 3.5 POST `/pipelines/validate` - Validate Pipeline Configuration

#### Purpose
Validate a pipeline configuration before execution to catch errors early.

#### Implementation
```python
# File: app/stages-api/validation.py

from typing import Dict, List, Any
from business.pipelines.runner import STAGE_REGISTRY

def validate_pipeline_config(
    pipeline: str,
    stages: List[str],
    config: Dict[str, Dict[str, Any]]
) -> Dict[str, Any]:
    """
    Validate pipeline configuration.
    
    Checks:
    1. Pipeline type is valid
    2. All stages exist in registry
    3. Stage dependencies are satisfied
    4. Configuration fields are valid
    5. Field types match expected types
    6. Required fields are present
    
    Args:
        pipeline: Pipeline type ("ingestion" or "graphrag")
        stages: List of stage names to execute
        config: Dictionary of stage configurations {stage_name: {field: value}}
    
    Returns:
        Validation result with errors, warnings, and execution plan
    """
    errors = []
    warnings = []
    
    # 1. Validate pipeline type
    if pipeline not in ["ingestion", "graphrag"]:
        errors.append({
            "code": "INVALID_PIPELINE",
            "message": f"Invalid pipeline type: {pipeline}",
            "field": "pipeline",
            "valid_options": ["ingestion", "graphrag"]
        })
        return {
            "valid": False,
            "errors": errors,
            "warnings": warnings
        }
    
    # 2. Validate stages exist
    for stage in stages:
        if stage not in STAGE_REGISTRY:
            errors.append({
                "code": "UNKNOWN_STAGE",
                "message": f"Unknown stage: {stage}",
                "field": "stages",
                "stage": stage
            })
    
    if errors:
        return {
            "valid": False,
            "errors": errors,
            "warnings": warnings
        }
    
    # 3. Validate dependencies (GraphRAG only)
    if pipeline == "graphrag":
        from business.pipelines.graphrag import STAGE_DEPENDENCIES, STAGE_ORDER
        
        missing_deps = []
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
    else:
        all_stages = stages
    
    # 4. Validate stage configurations
    for stage in all_stages:
        stage_cls = STAGE_REGISTRY[stage]
        config_cls = stage_cls.ConfigCls
        stage_config = config.get(stage, {})
        
        # Validate config fields
        validation_errors = _validate_stage_config(
            stage, config_cls, stage_config
        )
        errors.extend(validation_errors)
    
    # Build execution plan
    execution_plan = {
        "stages": all_stages,
        "stage_count": len(all_stages),
        "dependencies_satisfied": len(missing_deps) == 0,
        "execution_order": all_stages,
    }
    
    return {
        "valid": len(errors) == 0,
        "errors": errors,
        "warnings": warnings,
        "execution_plan": execution_plan
    }

def _validate_stage_config(
    stage_name: str,
    config_cls,
    config_dict: Dict[str, Any]
) -> List[Dict[str, Any]]:
    """
    Validate configuration for a single stage.
    
    Returns list of error objects.
    """
    errors = []
    
    # Get field definitions
    from dataclasses import fields, MISSING
    from typing import get_type_hints
    
    type_hints = get_type_hints(config_cls)
    valid_fields = {f.name: f for f in fields(config_cls)}
    
    # Check for unknown fields
    for field_name in config_dict.keys():
        if field_name not in valid_fields:
            errors.append({
                "code": "UNKNOWN_FIELD",
                "message": f"Unknown configuration field: {field_name}",
                "stage": stage_name,
                "field": field_name
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
                "actual_type": type(value).__name__
            })
    
    return errors

def _validate_field_type(field_name: str, value: Any, expected_type) -> str:
    """
    Validate that value matches expected type.
    
    Returns error message if invalid, None if valid.
    """
    from typing import get_origin, get_args
    
    # Handle Optional types
    origin = get_origin(expected_type)
    args = get_args(expected_type)
    
    if origin is type(None) or (origin and args and type(None) in args):
        # Optional type - check against non-None type
        if value is None:
            return None  # None is valid for Optional
        expected_type = args[0] if args else str
    
    # Type checking
    if expected_type == str and not isinstance(value, str):
        return f"Expected string, got {type(value).__name__}"
    elif expected_type == int and not isinstance(value, int):
        return f"Expected integer, got {type(value).__name__}"
    elif expected_type == float and not isinstance(value, (int, float)):
        return f"Expected number, got {type(value).__name__}"
    elif expected_type == bool and not isinstance(value, bool):
        return f"Expected boolean, got {type(value).__name__}"
    elif expected_type == list and not isinstance(value, list):
        return f"Expected array, got {type(value).__name__}"
    
    return None  # Valid
```

#### Request Schema
```json
{
  "pipeline": "string",
  "stages": ["string"],
  "config": {
    "<stage_name>": {
      "<field_name>": "any"
    }
  }
}
```

#### Response Schema (Internal Format)

The validation module returns errors as an array:

```json
{
  "valid": "boolean",
  "errors": [
    {
      "code": "string",
      "message": "string",
      "field": "string (optional)",
      "stage": "string (optional)"
    }
  ],
  "warnings": ["string"],
  "execution_plan": {
    "stages": ["string"],
    "resolved_dependencies": ["string"]
  }
}
```

#### Response Transformation (api.py)

**Note:** The API layer transforms the internal format to match the frontend contract.

**File:** `app/stages_api/api.py`

```python
def _transform_validation_result(result: Dict[str, Any]) -> Dict[str, Any]:
    """Transform validation result to match frontend API contract"""
    return {
        "valid": result.get("valid", False),
        "errors": _transform_errors(result.get("errors", [])),
        "warnings": _transform_warnings(result.get("warnings", [])),
        "execution_plan": result.get("execution_plan"),
    }

def _transform_errors(errors: List[Dict]) -> Dict[str, List[str]]:
    """Group errors by stage: {stage_name: [error_messages]}"""
    transformed = {}
    for error in errors:
        stage = error.get("stage", "general")
        msg = error.get("message", str(error))
        if stage not in transformed:
            transformed[stage] = []
        transformed[stage].append(msg)
    return transformed

def _transform_warnings(warnings: List) -> List[str]:
    """Convert warnings to string list"""
    return [w.get("message", str(w)) if isinstance(w, dict) else str(w) 
            for w in warnings]
```

#### Final Response Schema (After Transformation)

This is what the client receives:

```json
{
  "valid": "boolean",
  "errors": {
    "<stage_name>": ["error message 1", "error message 2"]
  },
  "warnings": ["warning message"],
  "execution_plan": {
    "stages": ["string"],
    "resolved_dependencies": ["string"]
  }
}
```

#### Example Request
```json
{
  "pipeline": "graphrag",
  "stages": ["entity_resolution"],
  "config": {
    "entity_resolution": {
      "similarity_threshold": 0.85,
      "temperature": "invalid"
    }
  }
}
```

#### Example Response (with errors)
```json
{
  "valid": false,
  "errors": [
    {
      "code": "TYPE_MISMATCH",
      "message": "Expected number, got string",
      "stage": "entity_resolution",
      "field": "temperature",
      "expected_type": "float",
      "actual_type": "str"
    }
  ],
  "warnings": [
    {
      "code": "MISSING_DEPENDENCY",
      "message": "Stage 'entity_resolution' depends on 'graph_extraction' which is not selected",
      "resolution": "'graph_extraction' will be auto-included in execution",
      "stage": "entity_resolution",
      "dependency": "graph_extraction"
    }
  ],
  "execution_plan": {
    "stages": ["graph_extraction", "entity_resolution"],
    "stage_count": 2,
    "dependencies_satisfied": false,
    "execution_order": ["graph_extraction", "entity_resolution"]
  }
}
```

---

### 3.6 POST `/pipelines/execute` - Execute Pipeline

#### Purpose
Start a pipeline execution with the provided configuration.

#### Implementation
```python
# File: app/stages-api/execution.py

import threading
import time
import uuid
from typing import Dict, Any, List
from datetime import datetime

# Thread-safe pipeline state storage
_active_pipelines: Dict[str, Dict[str, Any]] = {}
_pipeline_lock = threading.Lock()

def execute_pipeline(
    pipeline: str,
    stages: List[str],
    config: Dict[str, Dict[str, Any]],
    metadata: Dict[str, Any] = None
) -> Dict[str, Any]:
    """
    Execute a pipeline in the background.
    
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
    from app.stages_api.validation import validate_pipeline_config
    validation = validate_pipeline_config(pipeline, stages, config)
    
    if not validation["valid"]:
        return {
            "error": "Invalid configuration",
            "details": validation,
            "pipeline_id": None
        }
    
    # Use execution plan from validation (includes dependencies)
    execution_stages = validation["execution_plan"]["stages"]
    
    # Create pipeline configuration object
    try:
        pipeline_obj = _create_pipeline_object(pipeline, execution_stages, config, metadata)
    except Exception as e:
        return {
            "error": "Failed to create pipeline",
            "message": str(e),
            "pipeline_id": None
        }
    
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
            "progress": {
                "total_stages": len(execution_stages),
                "completed_stages": 0,
                "percent": 0.0
            }
        }
    
    # Start execution in background thread
    thread = threading.Thread(
        target=_run_pipeline_background,
        args=(pipeline_id, pipeline_obj, execution_stages),
        daemon=True
    )
    thread.start()
    
    return {
        "pipeline_id": pipeline_id,
        "status": "starting",
        "started_at": _active_pipelines[pipeline_id]["started_at"],
        "stages": execution_stages,
        "tracking_url": f"/api/v1/pipelines/{pipeline_id}/status"
    }

def _create_pipeline_object(
    pipeline: str,
    stages: List[str],
    config: Dict[str, Dict[str, Any]],
    metadata: Dict[str, Any]
):
    """Create pipeline object from configuration"""
    import os
    import argparse
    
    if pipeline == "graphrag":
        from business.pipelines.graphrag import GraphRAGPipeline
        from core.config.graphrag import GraphRAGPipelineConfig
        
        # Convert config dict to args/env format
        args = argparse.Namespace()
        env = dict(os.environ)
        
        # Apply metadata
        if metadata and "experiment_id" in metadata:
            env["EXPERIMENT_ID"] = metadata["experiment_id"]
        
        # Apply stage configs to env
        for stage_name, stage_config in config.items():
            _apply_config_to_env(stage_name, stage_config, env)
        
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
        
        # Apply stage configs
        for stage_name, stage_config in config.items():
            _apply_config_to_env(stage_name, stage_config, env)
        
        pipeline_config = IngestionPipelineConfig.from_args_env(
            args, env, os.getenv("DB_NAME", "mongo_hack")
        )
        
        return IngestionPipeline(pipeline_config)
    
    else:
        raise ValueError(f"Unknown pipeline: {pipeline}")

def _apply_config_to_env(stage_name: str, stage_config: Dict, env: Dict):
    """Apply stage configuration to environment variables"""
    # This is a simplified version - production should have proper mapping
    for key, value in stage_config.items():
        env_key = f"{stage_name.upper()}_{key.upper()}"
        env[env_key] = str(value)

def _run_pipeline_background(pipeline_id: str, pipeline_obj, stages: List[str]):
    """Run pipeline in background thread - runs each selected stage individually"""
    try:
        # Update status to running
        _update_pipeline_status(pipeline_id, "running")
        
        # Execute each selected stage individually
        completed_stages = []
        for idx, stage_name in enumerate(stages):
            # Check if cancelled
            if _is_cancelled(pipeline_id):
                return
            
            # Update current stage
            _update_pipeline_progress(
                pipeline_id,
                current_stage=stage_name,
                current_stage_index=idx,
            )
            
            # Run the individual stage
            exit_code = pipeline_obj.run_stage(stage_name)
            if exit_code != 0:
                _update_pipeline_error(pipeline_id, f"Stage {stage_name} failed", stage_name)
                return
            
            completed_stages.append(stage_name)
            _update_pipeline_progress(
                pipeline_id,
                completed_stages=completed_stages,
                percent=((idx + 1) / len(stages)) * 100,
            )
        
        # Update final status
        _update_pipeline_completion(pipeline_id, 0, stages)
    
    except Exception as e:
        # Handle errors
        _update_pipeline_error(pipeline_id, str(e), None)
```

#### Request Schema
```json
{
  "pipeline": "string",
  "stages": ["string"],
  "config": {
    "<stage_name>": {
      "<field_name>": "any"
    }
  },
  "metadata": {
    "experiment_id": "string (optional)",
    "description": "string (optional)",
    "... custom metadata ..."
  }
}
```

#### Response Schema (Success)
```json
{
  "pipeline_id": "string",
  "status": "string",
  "started_at": "ISO 8601 timestamp",
  "stages": ["string"],
  "tracking_url": "string"
}
```

#### Response Schema (Error)
```json
{
  "error": "string",
  "message": "string (optional)",
  "details": "object (optional)",
  "pipeline_id": null
}
```

---

### 3.7 GET `/pipelines/{pipeline_id}/status` - Get Pipeline Status

#### Purpose
Get real-time status of a running or completed pipeline.

#### Implementation
```python
# File: app/stages-api/execution.py

def get_pipeline_status(pipeline_id: str) -> Dict[str, Any]:
    """
    Get current status of a pipeline.
    
    Returns pipeline state including:
    - Overall status (starting/running/completed/failed/error)
    - Current stage
    - Progress percentage
    - Stage-specific stats (if available)
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
        if state["status"] in ["completed", "failed", "error"]:
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
    
    Note: This is a simplified implementation. Production version
    should properly signal the pipeline thread to stop gracefully.
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
                "status": state["status"]
            }
        
        # Mark as cancelled
        state["status"] = "cancelled"
        state["completed_at"] = datetime.utcnow().isoformat() + "Z"
        
        return {
            "pipeline_id": pipeline_id,
            "status": "cancelled",
            "message": "Pipeline cancellation requested"
        }
```

#### Response Schema
```json
{
  "pipeline_id": "string",
  "pipeline": "string",
  "status": "string",
  "started_at": "ISO 8601 timestamp",
  "completed_at": "ISO 8601 timestamp (optional)",
  "elapsed_seconds": "integer",
  "stages": ["string"],
  "current_stage": "string (optional)",
  "current_stage_index": "integer",
  "completed_stages": ["string"],
  "progress": {
    "total_stages": "integer",
    "completed_stages": "integer",
    "percent": "number"
  },
  "config": "object",
  "metadata": "object",
  "error": "string (if status=error)",
  "error_stage": "string (if error occurred)",
  "exit_code": "integer (if completed)"
}
```

---

### 3.8 GET `/health` - Health Check

#### Purpose
Check if the API server is running and responsive.

#### Implementation
```python
# File: app/stages_api/api.py

if method == "GET" and path == "health":
    return {
        "status": "healthy",
        "version": "1.0.0",
        "timestamp": datetime.utcnow().isoformat() + "Z",
        "active_pipelines": _get_active_pipeline_count(),
    }, 200

def _get_active_pipeline_count() -> int:
    """Get count of active pipelines for health check"""
    try:
        from .execution import list_active_pipelines
        result = list_active_pipelines()
        return result.get("count", 0)
    except Exception:
        return 0
```

#### Response Schema
```json
{
  "status": "healthy",
  "version": "string",
  "timestamp": "ISO 8601 timestamp",
  "active_pipelines": "integer"
}
```

#### Example Response
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "timestamp": "2025-12-09T18:29:43.623782Z",
  "active_pipelines": 2
}
```

---

### 3.9 POST `/pipelines/{pipeline_id}/cancel` - Cancel Pipeline

#### Purpose
Request cancellation of a running pipeline.

#### Implementation
See code sample in section 3.7 above.

#### Response Schema
```json
{
  "pipeline_id": "string",
  "status": "cancelled",
  "message": "string"
}
```

#### Error Response
```json
{
  "error": "Pipeline not found" | "Pipeline is not running",
  "pipeline_id": "string",
  "status": "string (current status)"
}
```

---

### 3.10 GET `/pipelines/active` - List Active Pipelines

#### Purpose
List all currently running or starting pipelines.

#### Implementation
```python
# File: app/stages_api/execution.py

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
                "progress": state["progress"],
            }
            for pid, state in _active_pipelines.items()
            if state["status"] in ["starting", "running"]
        }
    
    return {"count": len(active), "pipelines": active}
```

#### Response Schema
```json
{
  "count": "integer",
  "pipelines": {
    "<pipeline_id>": {
      "pipeline_id": "string",
      "pipeline": "string",
      "status": "string",
      "started_at": "ISO 8601 timestamp",
      "current_stage": "string",
      "progress": {
        "total_stages": "integer",
        "completed_stages": "integer",
        "percent": "number"
      }
    }
  }
}
```

---

### 3.11 GET `/pipelines/history` - Get Pipeline History

#### Purpose
Get recent pipeline executions with full details including configuration, timing, and errors.

**Note:** This endpoint queries MongoDB (`pipeline_executions` collection) for complete historical data.

#### Implementation
```python
# File: app/stages_api/execution.py

def get_pipeline_history(limit: int = 10) -> Dict[str, Any]:
    """Get recent pipeline executions from MongoDB"""
    try:
        from .repository import get_repository
        repo = get_repository()
        if repo is not None:
            db_pipelines = repo.list_history(limit)
            total = repo.count_all()
            
            result = []
            for state in db_pipelines:
                result.append({
                    "pipeline_id": state.get("pipeline_id"),
                    "pipeline": state.get("pipeline"),
                    "status": state.get("status"),
                    "started_at": state.get("started_at"),
                    "completed_at": state.get("completed_at"),
                    "stages": state.get("stages", []),
                    "progress": state.get("progress", {}),
                    "duration_seconds": state.get("duration_seconds"),
                    "exit_code": state.get("exit_code"),
                    "error": state.get("error"),
                    "error_stage": state.get("error_stage"),
                    "config": state.get("config", {}),
                    "metadata": state.get("metadata", {}),
                })
            
            return {"total": total, "returned": len(result), "pipelines": result}
    except Exception as e:
        # Fallback to in-memory
        return {"total": 0, "returned": 0, "pipelines": []}
```

#### Query Parameters
- `limit` (optional): Maximum number of results (default: 10)

#### Response Schema
```json
{
  "total": "integer (total count in database)",
  "returned": "integer (count in response)",
  "pipelines": [
    {
      "pipeline_id": "string",
      "pipeline": "string",
      "status": "string",
      "started_at": "ISO 8601 timestamp",
      "completed_at": "ISO 8601 timestamp (optional)",
      "stages": ["string"],
      "progress": {
        "total_stages": "integer",
        "completed_stages": "integer",
        "percent": "number"
      },
      "duration_seconds": "number (optional)",
      "exit_code": "integer (optional)",
      "error": "string (optional)",
      "error_stage": "string (optional)",
      "config": "object",
      "metadata": "object"
    }
  ]
}
```

#### Example Response
```json
{
  "total": 15,
  "returned": 10,
  "pipelines": [
    {
      "pipeline_id": "pipeline_1765303444_2f8122e5",
      "pipeline": "ingestion",
      "status": "completed",
      "started_at": "2025-12-09T18:04:04.960659Z",
      "completed_at": "2025-12-09T18:04:05.001973Z",
      "stages": ["clean"],
      "progress": {
        "total_stages": 1,
        "completed_stages": 1,
        "percent": 100.0
      },
      "duration_seconds": 0.041,
      "exit_code": 0,
      "error": null,
      "error_stage": null,
      "config": {
        "clean": {"max": 10, "llm": true}
      },
      "metadata": {
        "experiment_id": "exp_123"
      }
    }
  ]
}
```

---

### 3.12 MongoDB Persistence Layer

#### Purpose
All pipeline executions are persisted to MongoDB for recovery across server restarts.

#### Repository Implementation

**File:** `app/stages_api/repository.py`

```python
class PipelineRepository:
    """MongoDB repository for pipeline execution state"""
    
    def __init__(self, connection_string: str, db_name: str = "mongo_hack"):
        self.client = MongoClient(connection_string)
        self.db = self.client[db_name]
        self.collection = self.db.pipeline_executions
        self._ensure_indexes()
    
    def create(self, pipeline_state: Dict[str, Any]) -> str:
        """Create new pipeline execution record"""
        # ... implementation
    
    def get(self, pipeline_id: str) -> Optional[Dict[str, Any]]:
        """Get pipeline by ID"""
        # ... implementation
    
    def update_status(self, pipeline_id: str, status: str, 
                     progress: Optional[Dict] = None,
                     error: Optional[str] = None) -> bool:
        """Update pipeline status"""
        # ... implementation
    
    def list_history(self, limit: int = 10) -> List[Dict[str, Any]]:
        """Get recent pipeline executions"""
        # ... implementation
```

#### MongoDB Collection Schema

**Collection:** `pipeline_executions`  
**Database:** Configured via `MONGODB_DB` or `DB_NAME` environment variable

```javascript
{
  "_id": ObjectId,
  "pipeline_id": "pipeline_1765253690_b4371df8",
  "pipeline": "ingestion",
  "status": "completed",
  "stages": ["clean", "chunk"],
  "config": {
    "clean": {"max": 10, "llm": true},
    "chunk": {"chunk_size": 1000}
  },
  "metadata": {
    "experiment_id": "exp_123"
  },
  "current_stage": null,
  "completed_stages": ["clean", "chunk"],
  "progress": {
    "total_stages": 2,
    "completed_stages": 2,
    "percent": 100.0
  },
  "error": null,
  "error_stage": null,
  "exit_code": 0,
  "started_at": ISODate("2025-12-09T04:14:50.406Z"),
  "completed_at": ISODate("2025-12-09T04:14:50.783Z"),
  "duration_seconds": 0.377,
  "created_at": ISODate("2025-12-09T04:14:50.406Z"),
  "updated_at": ISODate("2025-12-09T04:14:50.783Z")
}
```

#### Indexes

```javascript
db.pipeline_executions.createIndex({ "pipeline_id": 1 }, { unique: true })
db.pipeline_executions.createIndex({ "status": 1 })
db.pipeline_executions.createIndex({ "started_at": -1 })
```

#### State Recovery on Server Restart

When the server starts, it automatically:
1. Queries MongoDB for pipelines with status `"running"`
2. Marks them as `"interrupted"` with error message
3. Logs recovery information

```python
# File: app/stages_api/execution.py

def recover_state_from_db():
    """Recover pipeline state from database on startup"""
    try:
        from .repository import get_repository
        repo = get_repository()
        if repo is None:
            return
        
        active = repo.list_active()
        for pipeline in active:
            if pipeline.get("status") == "running":
                repo.update_status(
                    pipeline["pipeline_id"],
                    "interrupted",
                    error="Server restarted during execution"
                )
                logger.warning(f"Pipeline {pipeline['pipeline_id']} marked as interrupted")
    except Exception as e:
        logger.warning(f"Failed to recover state: {e}")
```

---

## 4. Data Models and Schemas

### 4.1 Stage Metadata Model

```python
from typing import List, Dict, Any, Optional
from dataclasses import dataclass

@dataclass
class StageMetadata:
    """Metadata for a single stage"""
    name: str
    display_name: str
    description: str
    pipeline: str
    config_class: str
    dependencies: List[str]
    has_llm: bool

@dataclass
class PipelineMetadata:
    """Metadata for a pipeline"""
    name: str
    description: str
    stages: List[str]
    stage_count: int
```

### 4.2 Configuration Field Model

```python
@dataclass
class ConfigField:
    """Metadata for a configuration field"""
    name: str
    type: str  # API type (string, integer, number, boolean, array)
    python_type: str
    default: Any
    required: bool
    optional: bool
    description: str
    category: str
    ui_type: str  # text, number, slider, select, checkbox, etc.
    is_inherited: bool
    
    # UI-specific attributes (optional)
    min: Optional[float] = None
    max: Optional[float] = None
    step: Optional[float] = None
    options: Optional[List[str]] = None
    placeholder: Optional[str] = None
    pattern: Optional[str] = None
```

### 4.3 Validation Result Model

```python
@dataclass
class ValidationError:
    """Validation error"""
    code: str
    message: str
    field: Optional[str] = None
    stage: Optional[str] = None

@dataclass
class ValidationWarning:
    """Validation warning"""
    code: str
    message: str
    resolution: str

@dataclass
class ValidationResult:
    """Result of pipeline validation"""
    valid: bool
    errors: List[ValidationError]
    warnings: List[ValidationWarning]
    execution_plan: Dict[str, Any]
```

---

## 5. Metadata System

### 5.1 Metadata Registry (Optional Enhancement)

To avoid repetitive introspection, cache metadata on first access:

```python
# File: app/stages-api/metadata.py

_metadata_cache: Dict[str, Dict[str, Any]] = {}
_cache_lock = threading.Lock()

def get_cached_stage_metadata(stage_name: str) -> Dict[str, Any]:
    """Get stage metadata with caching"""
    with _cache_lock:
        if stage_name not in _metadata_cache:
            _metadata_cache[stage_name] = get_stage_metadata(stage_name)
        return _metadata_cache[stage_name].copy()

def clear_metadata_cache():
    """Clear metadata cache (useful for testing or hot-reload)"""
    with _cache_lock:
        _metadata_cache.clear()
```

### 5.2 Field Metadata Registry

For fields that need custom metadata (UI hints, descriptions), maintain a separate registry:

```python
# File: app/stages-api/field_metadata.py

"""
Field metadata registry for UI customization.

This supplements the automatic introspection with human-friendly
descriptions and UI hints.
"""

FIELD_METADATA = {
    # Common fields (BaseStageConfig)
    "max": {
        "description": "Maximum number of documents to process (for testing/debugging)",
        "ui_type": "number",
        "min": 1,
        "max": 10000,
        "placeholder": "Leave empty to process all documents"
    },
    "concurrency": {
        "description": "Number of parallel workers for concurrent processing",
        "ui_type": "number",
        "min": 1,
        "max": 500,
        "recommended": 50
    },
    "verbose": {
        "description": "Enable detailed logging output",
        "ui_type": "checkbox"
    },
    
    # GraphRAG-specific fields
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
    "similarity_threshold": {
        "description": "Minimum similarity score for entity resolution (0-1)",
        "ui_type": "slider",
        "min": 0.0,
        "max": 1.0,
        "step": 0.05
    },
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
    }
}

def get_field_metadata(field_name: str) -> Dict[str, Any]:
    """Get metadata for a field, returns empty dict if not found"""
    return FIELD_METADATA.get(field_name, {})
```

Update the schema extraction to use this registry:

```python
# In _extract_config_schema():

# Get field metadata from registry
from app.stages_api.field_metadata import get_field_metadata
registry_metadata = get_field_metadata(field.name)

# Merge with introspected metadata (registry takes precedence)
field_info = {
    "name": field.name,
    ...
    "description": registry_metadata.get("description", metadata.get("description", "")),
    "ui_type": registry_metadata.get("ui_type", metadata.get("ui_type", _infer_ui_type(...))),
    ...
}

# Add UI-specific metadata from registry
for key in ["min", "max", "step", "options", "placeholder", "recommended"]:
    if key in registry_metadata:
        field_info[key] = registry_metadata[key]
```

---

## 6. Validation System

### 6.1 Validation Rules

#### Type Validation
- String fields must be strings
- Integer fields must be integers (not floats)
- Float fields accept integers or floats
- Boolean fields must be true/false
- Array fields must be arrays

#### Range Validation
- Numeric fields with `min`/`max` metadata must be in range
- String fields with `pattern` metadata must match regex

#### Required Field Validation
- Fields without defaults must be provided
- Optional fields can be omitted

#### Dependency Validation
- GraphRAG stages must include dependencies
- Warnings issued for auto-included dependencies

### 6.2 Enhanced Validation (Optional)

```python
def _validate_field_value(
    field_name: str,
    value: Any,
    field_metadata: Dict[str, Any]
) -> Optional[str]:
    """
    Validate field value against metadata constraints.
    
    Returns error message if invalid, None if valid.
    """
    # Range validation for numbers
    if "min" in field_metadata and isinstance(value, (int, float)):
        if value < field_metadata["min"]:
            return f"Value must be >= {field_metadata['min']}"
    
    if "max" in field_metadata and isinstance(value, (int, float)):
        if value > field_metadata["max"]:
            return f"Value must be <= {field_metadata['max']}"
    
    # Options validation for selects
    if "options" in field_metadata:
        if value not in field_metadata["options"]:
            return f"Value must be one of: {', '.join(field_metadata['options'])}"
    
    # Pattern validation for strings
    if "pattern" in field_metadata and isinstance(value, str):
        import re
        if not re.match(field_metadata["pattern"], value):
            return f"Value does not match required pattern: {field_metadata['pattern']}"
    
    return None
```

---

## 7. Execution System

### 7.1 Pipeline State Management

```python
class PipelineState:
    """
    Thread-safe pipeline state container.
    
    Tracks:
    - Overall pipeline status
    - Current stage
    - Progress metrics
    - Stage-specific stats
    - Error information
    """
    
    def __init__(self, pipeline_id: str, pipeline: str, stages: List[str]):
        self.pipeline_id = pipeline_id
        self.pipeline = pipeline
        self.stages = stages
        self.status = "starting"
        self.current_stage = None
        self.current_stage_index = 0
        self.completed_stages = []
        self.started_at = datetime.utcnow()
        self.completed_at = None
        self.error = None
        self.error_stage = None
        self.exit_code = None
        self._lock = threading.Lock()
    
    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary for API response"""
        with self._lock:
            return {
                "pipeline_id": self.pipeline_id,
                "pipeline": self.pipeline,
                "status": self.status,
                "started_at": self.started_at.isoformat() + "Z",
                "completed_at": self.completed_at.isoformat() + "Z" if self.completed_at else None,
                "elapsed_seconds": self._elapsed_seconds(),
                "stages": self.stages,
                "current_stage": self.current_stage,
                "current_stage_index": self.current_stage_index,
                "completed_stages": self.completed_stages,
                "progress": self._calculate_progress(),
                "error": self.error,
                "error_stage": self.error_stage,
                "exit_code": self.exit_code
            }
    
    def _elapsed_seconds(self) -> int:
        """Calculate elapsed time"""
        end_time = self.completed_at or datetime.utcnow()
        return int((end_time - self.started_at).total_seconds())
    
    def _calculate_progress(self) -> Dict[str, Any]:
        """Calculate progress metrics"""
        total = len(self.stages)
        completed = len(self.completed_stages)
        return {
            "total_stages": total,
            "completed_stages": completed,
            "percent": (completed / total * 100.0) if total > 0 else 0.0
        }
```

### 7.2 Integration with Existing Pipeline System

The execution system should integrate with the existing `PipelineRunner` to track progress:

```python
# Enhanced pipeline execution with progress tracking

def _run_pipeline_with_tracking(
    pipeline_id: str,
    pipeline_obj,
    stages: List[str],
    state: PipelineState
):
    """Run pipeline with progress tracking"""
    
    # Update state to running
    with state._lock:
        state.status = "running"
    
    # Execute each stage with tracking
    for i, stage_name in enumerate(stages):
        with state._lock:
            state.current_stage = stage_name
            state.current_stage_index = i
        
        try:
            # Run stage
            exit_code = pipeline_obj.run_stage(stage_name)
            
            if exit_code != 0:
                with state._lock:
                    state.status = "failed"
                    state.exit_code = exit_code
                    state.error_stage = stage_name
                    state.completed_at = datetime.utcnow()
                return
            
            # Mark stage complete
            with state._lock:
                state.completed_stages.append(stage_name)
        
        except Exception as e:
            with state._lock:
                state.status = "error"
                state.error = str(e)
                state.error_stage = stage_name
                state.completed_at = datetime.utcnow()
            return
    
    # All stages completed
    with state._lock:
        state.status = "completed"
        state.exit_code = 0
        state.completed_at = datetime.utcnow()
```

---

## 8. Error Handling

### 8.1 Error Response Format

All errors follow a consistent format:

```json
{
  "error": "High-level error type",
  "message": "Detailed error message (optional)",
  "code": "ERROR_CODE (optional)",
  "details": {
    "... additional context ..."
  }
}
```

### 8.2 Error Codes

| Code | Description | HTTP Status |
|------|-------------|-------------|
| `STAGE_NOT_FOUND` | Stage doesn't exist in registry | 404 |
| `INVALID_PIPELINE` | Unknown pipeline type | 400 |
| `VALIDATION_ERROR` | Configuration validation failed | 400 |
| `EXECUTION_ERROR` | Pipeline execution failed | 500 |
| `PIPELINE_NOT_FOUND` | Pipeline ID not found | 404 |
| `PIPELINE_NOT_RUNNING` | Cannot cancel non-running pipeline | 400 |
| `INTERNAL_ERROR` | Unexpected server error | 500 |

### 8.3 Error Handling Pattern

```python
def api_error_handler(func):
    """Decorator for consistent error handling"""
    def wrapper(*args, **kwargs):
        try:
            return func(*args, **kwargs)
        except ValueError as e:
            return {
                "error": "Validation Error",
                "message": str(e),
                "code": "VALIDATION_ERROR"
            }, 400
        except KeyError as e:
            return {
                "error": "Not Found",
                "message": f"Resource not found: {e}",
                "code": "NOT_FOUND"
            }, 404
        except Exception as e:
            logger.exception(f"Unexpected error in {func.__name__}")
            return {
                "error": "Internal Server Error",
                "message": str(e),
                "code": "INTERNAL_ERROR"
            }, 500
    return wrapper
```

---

## 9. Implementation Guide

### 9.1 Implementation Order

1. **Phase 1: Metadata System** (Days 1-2)
   - Implement `metadata.py`
   - Implement introspection functions
   - Create field metadata registry
   - Test with all stages
   - Unit tests for metadata extraction

2. **Phase 2: Validation System** (Days 3-4)
   - Implement `validation.py`
   - Implement type validation
   - Implement dependency validation
   - Test with various configurations
   - Unit tests for validation logic

3. **Phase 3: Execution System** (Days 5-6)
   - Implement `execution.py`
   - Implement pipeline state management
   - Implement background execution
   - Integrate with PipelineRunner
   - Unit tests for execution logic

4. **Phase 4: API Integration** (Days 7-8)
   - Create HTTP server wrapper
   - Implement endpoint routing
   - Add error handling middleware
   - Test all endpoints
   - Integration tests

5. **Phase 5: Documentation & Polish** (Days 9-10)
   - Write API documentation
   - Add examples
   - Performance optimization
   - Security review
   - Final testing

### 9.2 Testing Strategy

#### Unit Tests

```python
# tests/test_metadata.py

def test_list_stages():
    """Test that all stages are listed"""
    result = list_stages()
    assert "pipelines" in result
    assert "stages" in result
    assert "graphrag" in result["pipelines"]
    assert len(result["stages"]) == 13  # Total stages

def test_get_stage_config():
    """Test configuration schema extraction"""
    result = get_stage_config("graph_extraction")
    assert result["stage_name"] == "graph_extraction"
    assert "fields" in result
    assert len(result["fields"]) > 0
    
    # Check field structure
    field = result["fields"][0]
    assert "name" in field
    assert "type" in field
    assert "default" in field

def test_get_stage_config_unknown():
    """Test error handling for unknown stage"""
    with pytest.raises(ValueError):
        get_stage_config("invalid_stage")
```

```python
# tests/test_validation.py

def test_validate_valid_config():
    """Test validation with valid configuration"""
    result = validate_pipeline_config(
        pipeline="graphrag",
        stages=["graph_extraction"],
        config={"graph_extraction": {"model_name": "gpt-4o-mini"}}
    )
    assert result["valid"] == True
    assert len(result["errors"]) == 0

def test_validate_type_mismatch():
    """Test type validation"""
    result = validate_pipeline_config(
        pipeline="graphrag",
        stages=["graph_extraction"],
        config={"graph_extraction": {"temperature": "invalid"}}
    )
    assert result["valid"] == False
    assert any("TYPE_MISMATCH" in e["code"] for e in result["errors"])

def test_validate_dependencies():
    """Test dependency validation"""
    result = validate_pipeline_config(
        pipeline="graphrag",
        stages=["entity_resolution"],  # Missing graph_extraction
        config={}
    )
    assert len(result["warnings"]) > 0
    assert "graph_extraction" in result["execution_plan"]["stages"]
```

#### Integration Tests

```python
# tests/test_integration.py

def test_full_pipeline_flow():
    """Test complete flow: validate → execute → status"""
    # 1. Validate
    validation = validate_pipeline_config(
        pipeline="graphrag",
        stages=["graph_extraction"],
        config={"graph_extraction": {"max": 10}}
    )
    assert validation["valid"]
    
    # 2. Execute
    execution = execute_pipeline(
        pipeline="graphrag",
        stages=["graph_extraction"],
        config={"graph_extraction": {"max": 10}}
    )
    assert "pipeline_id" in execution
    pipeline_id = execution["pipeline_id"]
    
    # 3. Check status
    status = get_pipeline_status(pipeline_id)
    assert status["pipeline_id"] == pipeline_id
    assert status["status"] in ["starting", "running", "completed"]
```

---

## 10. Testing Strategy

### 10.1 Test Coverage Goals

- **Unit Tests:** 80%+ coverage for core logic
- **Integration Tests:** All API endpoints
- **End-to-End Tests:** Full pipeline execution scenarios
- **Load Tests:** Concurrent pipeline execution

### 10.2 Test Data

Create fixtures for testing:

```python
# tests/fixtures.py

@pytest.fixture
def sample_config():
    """Sample valid configuration"""
    return {
        "graph_extraction": {
            "model_name": "gpt-4o-mini",
            "temperature": 0.1,
            "max": 10
        }
    }

@pytest.fixture
def invalid_config():
    """Sample invalid configuration"""
    return {
        "graph_extraction": {
            "temperature": "invalid",  # Should be float
            "unknown_field": "value"   # Unknown field
        }
    }
```

---

## 11. Security Considerations

### 11.1 Input Validation

- Validate all user inputs against schema
- Sanitize string inputs
- Limit numeric ranges
- Prevent code injection

### 11.2 Resource Limits

- Cap `max` parameter (e.g., max 1000 documents)
- Limit concurrent pipelines (e.g., max 5 concurrent)
- Set execution timeouts
- Monitor memory usage

### 11.3 Authentication (Future)

- Add API key authentication
- Rate limiting per API key
- Audit logging for all executions

---

## 12. Performance Optimizations

### 12.1 Caching

- Cache stage metadata (rarely changes)
- Cache default configurations
- Use thread-safe caching

### 12.2 Async Operations

- Execute pipelines in background threads
- Non-blocking status checks
- Consider async I/O for MongoDB

### 12.3 Database Optimization

- Index pipeline tracking collections
- Use projections to limit data transfer
- Batch status updates

---

## 13. Deployment Considerations

### 13.1 HTTP Server Options

**Option 1: Standalone Server**
```python
# app/stages-api/server.py

from http.server import HTTPServer, BaseHTTPRequestHandler
import json

class StagesAPIHandler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/api/v1/stages":
            result = list_stages()
            self._send_json(result)
        # ... more routes
    
    def _send_json(self, data, status=200):
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(json.dumps(data).encode())

def run_server(port=8080):
    server = HTTPServer(("", port), StagesAPIHandler)
    print(f"Stages API running on port {port}")
    server.serve_forever()
```

**Option 2: Flask Integration**
```python
from flask import Flask, jsonify, request

app = Flask(__name__)

@app.route("/api/v1/stages")
def get_stages():
    return jsonify(list_stages())

@app.route("/api/v1/stages/<stage_name>/config")
def get_config(stage_name):
    try:
        return jsonify(get_stage_config(stage_name))
    except ValueError as e:
        return jsonify({"error": str(e)}), 404

if __name__ == "__main__":
    app.run(port=8080)
```

### 13.2 Environment Configuration

**Note:** The server automatically loads `.env` file using `python-dotenv` at startup.

**File:** `app/stages_api/server.py` loads `.env` before importing any modules.

```bash
# .env

# Database Configuration (Required)
MONGODB_URI=mongodb://localhost:27017
# or for MongoDB Atlas:
# MONGODB_URI=mongodb+srv://user:pass@cluster.mongodb.net/?retryWrites=true&w=majority

# Database Name (supports both variables)
MONGODB_DB=2025-12              # Primary (used by Stages API)
DB_NAME=2025-12                 # Alternative

# API Configuration
STAGES_API_PORT=8080
STAGES_API_HOST=0.0.0.0

# Pipeline Execution
MAX_CONCURRENT_PIPELINES=5
PIPELINE_TIMEOUT_SECONDS=7200
MAX_DOCUMENTS_PER_RUN=1000

# Logging
LOG_LEVEL=INFO
LOG_FILE=logs/stages_api.log

# OpenAI Configuration (Required for LLM stages)
OPENAI_API_KEY=your_key_here
OPENAI_DEFAULT_MODEL=gpt-4o-mini
```

**Server Startup (.env loading):**

```python
# File: app/stages_api/server.py

import os

# Load .env BEFORE any other imports
try:
    from dotenv import load_dotenv
    possible_paths = [
        os.path.join(os.path.dirname(__file__), '..', '..', '.env'),
        os.path.join(os.getcwd(), '.env'),
        '.env',
    ]
    for env_path in possible_paths:
        if os.path.exists(env_path):
            load_dotenv(env_path, override=True)
            print(f"✓ Loaded environment from: {os.path.abspath(env_path)}")
            break
except ImportError:
    print("⚠ python-dotenv not installed")
```

---

## 14. Summary

### Implementation Status: ✅ COMPLETE

All API endpoints and features described in this document have been implemented and are operational as of December 9, 2025.

### What This API Provides:

1. **Complete Introspection:** ✅ Extracts metadata from existing dataclasses
2. **Type-Safe Validation:** ✅ Leverages Python type hints
3. **Flexible Configuration:** ✅ Supports all stage parameters
4. **Real-Time Monitoring:** ✅ Tracks pipeline execution progress
5. **Error-Friendly:** ✅ Detailed error messages
6. **Extensible:** ✅ Easy to add new stages and fields
7. **MongoDB Persistence:** ✅ State survives server restarts
8. **Health Monitoring:** ✅ Health check endpoint

### Integration Status

| Component | Status |
|-----------|--------|
| **Backend API** | ✅ All 12 endpoints implemented |
| **Frontend UI** | ✅ StagesUI operational |
| **MongoDB Persistence** | ✅ Repository layer complete |
| **Validation** | ✅ With transformation layer |
| **Execution** | ✅ Background + progress tracking |
| **Documentation** | ✅ Complete (this + CONFIGURATION_ARCHITECTURE.md) |

### Key Files

| File | Purpose | Lines |
|------|---------|-------|
| `api.py` | Request routing, transformations | ~327 |
| `metadata.py` | Schema extraction | ~487 |
| `validation.py` | Config validation | ~300 |
| `execution.py` | Pipeline execution | ~686 |
| `repository.py` | MongoDB persistence | ~175 |
| `server.py` | HTTP server + .env loading | ~147 |
| `constants.py` | Pipeline groups | ~60 |
| `field_metadata.py` | UI customization | ~300 |

### Related Documentation

- **Configuration Reference:** [CONFIGURATION_ARCHITECTURE.md](./CONFIGURATION_ARCHITECTURE.md)
- **Quick Reference:** [CONFIG_QUICK_REFERENCE.md](./CONFIG_QUICK_REFERENCE.md)
- **Session Summary:** [SESSION_SUMMARY.md](./SESSION_SUMMARY.md)
- **UI Design:** [UI_DESIGN_SPECIFICATION.md](./UI_DESIGN_SPECIFICATION.md)

---

**Last Updated:** December 9, 2025  
**Version:** 1.0 (Implemented)

**End of Document**

