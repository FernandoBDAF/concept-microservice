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

from src.domain.pipelines.runner import STAGE_REGISTRY
from .constants import PIPELINE_GROUPS
from .field_metadata import get_field_metadata


def validate_pipeline_config(
    pipeline: str, stages: List[str], config: Dict[str, Dict[str, Any]]
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
        errors.append(
            {
                "code": "INVALID_PIPELINE",
                "message": f"Invalid pipeline type: {pipeline}",
                "field": "pipeline",
                "valid_options": list(PIPELINE_GROUPS.keys()),
            }
        )
        return {
            "valid": False,
            "errors": errors,
            "warnings": warnings,
            "execution_plan": None,
        }

    # 2. Validate stages exist and belong to pipeline
    valid_stages = PIPELINE_GROUPS[pipeline]
    for stage in stages:
        if stage not in STAGE_REGISTRY:
            errors.append(
                {
                    "code": "UNKNOWN_STAGE",
                    "message": f"Unknown stage: {stage}",
                    "field": "stages",
                    "stage": stage,
                }
            )
        elif stage not in valid_stages:
            errors.append(
                {
                    "code": "STAGE_PIPELINE_MISMATCH",
                    "message": f"Stage '{stage}' does not belong to '{pipeline}' pipeline",
                    "field": "stages",
                    "stage": stage,
                    "expected_pipeline": _get_stage_pipeline(stage),
                }
            )

    if errors:
        return {
            "valid": False,
            "errors": errors,
            "warnings": warnings,
            "execution_plan": None,
        }

    # 3. Validate dependencies (GraphRAG only)
    # Note: Dependencies are now "data prerequisites" - stages can run independently
    # if the prerequisite data exists from a previous run
    if pipeline == "graphrag":
        try:
            from src.domain.pipelines.graphrag import STAGE_DEPENDENCIES, STAGE_ORDER

            for stage in stages:
                deps = STAGE_DEPENDENCIES.get(stage, [])
                for dep in deps:
                    if dep not in stages and dep not in missing_deps:
                        missing_deps.append(dep)
                        warnings.append(
                            {
                                "code": "DATA_PREREQUISITE",
                                "message": f"Stage '{stage}' requires data from '{dep}'",
                                "resolution": f"Ensure '{dep}' has been run previously, or select it to run now",
                                "stage": stage,
                                "dependency": dep,
                            }
                        )

            # Don't auto-include dependencies - user controls which stages to run
            # Just sort the selected stages by execution order
            all_stages = [s for s in STAGE_ORDER if s in stages]
        except ImportError:
            all_stages = stages
    else:
        # For ingestion, sort stages according to PIPELINE_GROUPS order
        pipeline_order = PIPELINE_GROUPS.get(pipeline, [])
        all_stages = [s for s in pipeline_order if s in stages]

    # 4. Validate stage configurations
    for stage in all_stages:
        stage_cls = STAGE_REGISTRY.get(stage)
        if not stage_cls:
            continue

        config_cls = getattr(stage_cls, "ConfigCls", None)
        if not config_cls:
            continue

        stage_config = config.get(stage, {})

        # Validate config fields
        validation_errors = _validate_stage_config(stage, config_cls, stage_config)
        errors.extend(validation_errors)

    # Build execution plan
    execution_plan = {
        "stages": all_stages,
        "stage_count": len(all_stages),
        "data_prerequisites_selected": len(missing_deps) == 0,
        "unselected_prerequisites": missing_deps,
        "execution_order": all_stages,
    }

    return {
        "valid": len(errors) == 0,
        "errors": errors,
        "warnings": warnings,
        "execution_plan": execution_plan,
    }


def validate_stage_config_only(stage_name: str, config: Dict[str, Any]) -> Dict[str, Any]:
    """
    Validate configuration for a single stage (without pipeline context).

    Useful for validating individual stage configs before full pipeline validation.
    """
    stage_cls = STAGE_REGISTRY.get(stage_name)
    if not stage_cls:
        return {
            "valid": False,
            "errors": [
                {
                    "code": "UNKNOWN_STAGE",
                    "message": f"Unknown stage: {stage_name}",
                    "stage": stage_name,
                }
            ],
            "warnings": [],
        }

    config_cls = getattr(stage_cls, "ConfigCls", None)
    if not config_cls:
        return {
            "valid": False,
            "errors": [
                {
                    "code": "NO_CONFIG_CLASS",
                    "message": f"Stage {stage_name} has no ConfigCls",
                    "stage": stage_name,
                }
            ],
            "warnings": [],
        }

    errors = _validate_stage_config(stage_name, config_cls, config)

    return {"valid": len(errors) == 0, "errors": errors, "warnings": []}


def _validate_stage_config(
    stage_name: str, config_cls, config_dict: Dict[str, Any]
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
            errors.append(
                {
                    "code": "UNKNOWN_FIELD",
                    "message": f"Unknown configuration field: {field_name}",
                    "stage": stage_name,
                    "field": field_name,
                    "valid_fields": list(valid_fields.keys()),
                }
            )

    # Check required fields and types
    for field_name, field_def in valid_fields.items():
        value = config_dict.get(field_name)

        # Skip if not provided and has default
        if value is None:
            if field_def.default is MISSING and field_def.default_factory is MISSING:
                # Only mark as error if it's truly required (no factory either)
                # Most fields have defaults, so we skip this check
                pass
            continue

        # Type validation
        expected_type = type_hints.get(field_name, field_def.type)
        type_error = _validate_field_type(field_name, value, expected_type)
        if type_error:
            errors.append(
                {
                    "code": "TYPE_MISMATCH",
                    "message": type_error,
                    "stage": stage_name,
                    "field": field_name,
                    "expected_type": str(expected_type),
                    "actual_type": type(value).__name__,
                    "actual_value": value,
                }
            )
            continue

        # Value validation (range, options, etc.)
        value_error = _validate_field_value(field_name, value)
        if value_error:
            errors.append(
                {
                    "code": "VALUE_OUT_OF_RANGE",
                    "message": value_error,
                    "stage": stage_name,
                    "field": field_name,
                    "actual_value": value,
                }
            )

    return errors


def _validate_field_type(
    field_name: str, value: Any, expected_type
) -> Optional[str]:
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
        if isinstance(value, list):
            # For multiselect, check each value
            invalid_values = [v for v in value if v not in metadata["options"]]
            if invalid_values:
                return f"Invalid values {invalid_values}. Must be from: {', '.join(metadata['options'])}"
        elif value not in metadata["options"]:
            return f"Value '{value}' must be one of: {', '.join(metadata['options'])}"

    return None


def _get_stage_pipeline(stage_name: str) -> str:
    """Determine which pipeline a stage belongs to"""
    for pipeline, stages in PIPELINE_GROUPS.items():
        if stage_name in stages:
            return pipeline
    return "unknown"

