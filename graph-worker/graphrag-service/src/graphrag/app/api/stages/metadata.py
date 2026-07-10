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
from typing import Any, Dict, List, get_type_hints, get_origin, get_args, Union

from src.domain.pipelines.runner import STAGE_REGISTRY
from .constants import PIPELINE_GROUPS, PIPELINE_INFO, CATEGORY_PATTERNS
from .field_metadata import get_field_metadata


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
    result = {"pipelines": {}, "stages": {}}

    # Build pipeline metadata
    for pipeline_name, stage_list in PIPELINE_GROUPS.items():
        info = PIPELINE_INFO.get(pipeline_name, {})
        result["pipelines"][pipeline_name] = {
            "name": info.get("name", pipeline_name.title()),
            "description": info.get("description", ""),
            "stages": stage_list,
            "stage_count": len(stage_list),
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
            "config_class": _get_config_class_name(stage_cls),
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
        "field_count": len(schema["fields"]),
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
        Dictionary with default configuration values (flat, no wrapper)
        
    Note:
        Returns only the actual config field values, not metadata.
        This allows the frontend to directly use the response as config defaults.
    """
    stage_cls = STAGE_REGISTRY.get(stage_name)
    if not stage_cls:
        raise ValueError(f"Unknown stage: {stage_name}")

    config_cls = getattr(stage_cls, "ConfigCls", None)
    if not config_cls:
        raise ValueError(f"Stage {stage_name} has no ConfigCls")

    # Instantiate with defaults
    try:
        config_instance = config_cls()
    except Exception as e:
        raise ValueError(f"Failed to create default config: {e}")

    # Convert to dictionary and return flat (no wrapper)
    # This matches the expected format for config submission
    return _config_to_dict(config_instance)


def list_pipeline_stages(pipeline: str) -> Dict[str, Any]:
    """
    List stages for a specific pipeline.

    Args:
        pipeline: Pipeline name ("ingestion" or "graphrag")

    Returns:
        Dictionary with pipeline info and stage details
    """
    if pipeline not in PIPELINE_GROUPS:
        raise ValueError(
            f"Unknown pipeline: {pipeline}. Valid options: {list(PIPELINE_GROUPS.keys())}"
        )

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
        "stages": pipeline_stages,
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
        from src.domain.pipelines.graphrag import STAGE_DEPENDENCIES

        return STAGE_DEPENDENCIES.get(stage_name, [])
    except ImportError:
        return []


def _get_config_class_name(stage_cls) -> str:
    """Get config class name safely"""
    config_cls = getattr(stage_cls, "ConfigCls", None)
    if config_cls:
        return config_cls.__name__
    return "Unknown"


def _stage_uses_llm(stage_cls) -> bool:
    """Check if stage uses LLM by inspecting config"""
    try:
        config_cls = getattr(stage_cls, "ConfigCls", None)
        if not config_cls:
            return False
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
            "description": registry_metadata.get(
                "description", dc_metadata.get("description", "")
            ),
            "category": registry_metadata.get(
                "category",
                dc_metadata.get("category", _infer_category(field.name, is_inherited)),
            ),
            "ui_type": registry_metadata.get(
                "ui_type",
                dc_metadata.get("ui_type", _infer_ui_type(type_info["python_type"])),
            ),
            "is_inherited": is_inherited,
        }

        # Add UI-specific metadata
        for key in [
            "min",
            "max",
            "step",
            "options",
            "placeholder",
            "pattern",
            "recommended",
        ]:
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
    if field_type is type(None):
        return {"type": "null", "python_type": type(None), "optional": True}

    # Check for Union with None (Optional)
    if origin is Union and args and type(None) in args:
        non_none_types = [t for t in args if t is not type(None)]
        actual_type = non_none_types[0] if non_none_types else str
        return {
            "type": _get_type_name(actual_type),
            "python_type": actual_type,
            "optional": True,
        }

    # Handle List[T]
    if origin is list:
        item_type = args[0] if args else str
        return {
            "type": "array",
            "python_type": list,
            "item_type": _get_type_name(item_type),
            "optional": False,
        }

    # Basic type
    return {
        "type": _get_type_name(field_type),
        "python_type": field_type,
        "optional": False,
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
        "NoneType": "null",
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
        "General",
    ]

    sorted_categories = []
    for cat_name in category_order:
        if cat_name in categories:
            sorted_categories.append(
                {
                    "name": cat_name,
                    "fields": categories[cat_name],
                    "field_count": len(categories[cat_name]),
                }
            )

    # Add remaining categories
    for cat_name, field_list in categories.items():
        if cat_name not in category_order:
            sorted_categories.append(
                {
                    "name": cat_name,
                    "fields": field_list,
                    "field_count": len(field_list),
                }
            )

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

