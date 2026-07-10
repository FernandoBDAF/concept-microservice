"""
Ontology Loader Utility

Loads canonical predicate definitions, predicate mappings, and type constraints
from YAML files for use in GraphRAG extraction normalization.
"""

import logging
import os
from pathlib import Path
from typing import Dict, List, Set, Any, Optional

try:
    import yaml
except ImportError:
    yaml = None

logger = logging.getLogger(__name__)

# Cache for loaded ontology data
_ontology_cache: Optional[Dict[str, Any]] = None


def load_ontology() -> Dict[str, Any]:
    """
    Load ontology files with fallback handling.

    Returns:
        Dictionary containing:
        - canonical_predicates: Set[str]
        - symmetric_predicates: Set[str]
        - predicate_map: Dict[str, str] (variant → canonical)
        - predicate_type_constraints: Dict[str, List[List[str]]] (predicate → allowed type pairs)
        - type_map: Dict[str, str] (optional, for future use)

    Note:
        Missing files will cause extraction to continue without filtering (backward compatible).
        A warning will be logged but processing will continue.
    """
    global _ontology_cache

    # Return cached data if available
    if _ontology_cache is not None:
        return _ontology_cache

    if yaml is None:
        logger.warning(
            "PyYAML not installed. Ontology loading disabled. "
            "Install with: pip install pyyaml"
        )
        return _get_empty_ontology()

    # Get ontology directory from environment
    ontology_dir = os.getenv("GRAPHRAG_ONTOLOGY_DIR", "ontology")
    ontology_path = Path(ontology_dir)

    if not ontology_path.is_absolute():
        # Relative to project root - robust base-dir resolver
        # loader.py is at: core/libraries/ontology/loader.py
        # Project root is 4 levels up
        loader_file = Path(__file__).resolve()
        project_root = loader_file.parent.parent.parent.parent
        ontology_path = project_root / ontology_dir

        # Validate the resolved path exists
        if not project_root.exists():
            logger.warning(
                f"Project root not found at {project_root}. "
                f"Trying current working directory instead."
            )
            # Fallback to current working directory
            import os as os_module

            cwd = Path(os_module.getcwd()).resolve()
            ontology_path = cwd / ontology_dir

    result = {
        "canonical_predicates": set(),
        "symmetric_predicates": set(),
        "predicate_map": {},
        "predicate_type_constraints": {},
        "type_map": {},
    }

    # Load canonical_predicates.yml
    canonical_file = ontology_path / "canonical_predicates.yml"
    if canonical_file.exists():
        try:
            with open(canonical_file, "r", encoding="utf-8") as f:
                data = yaml.safe_load(f) or {}

            # Validate and extract canonical predicates
            if "canonical_predicates" in data:
                if not isinstance(data["canonical_predicates"], list):
                    logger.warning(
                        "canonical_predicates must be a list. Skipping this section."
                    )
                else:
                    # Filter out comments (lines starting with #) and validate
                    predicates = []
                    for p in data["canonical_predicates"]:
                        if isinstance(p, str) and not p.strip().startswith("#"):
                            predicates.append(p)
                        elif not isinstance(p, str):
                            logger.warning(
                                f"Invalid entry in canonical_predicates: {p} (not a string). Skipping."
                            )
                    result["canonical_predicates"] = set(predicates)
                    logger.info(
                        f"Loaded {len(result['canonical_predicates'])} canonical predicates"
                    )

            # Validate and extract symmetric predicates
            if "symmetric_predicates" in data:
                if not isinstance(data["symmetric_predicates"], list):
                    logger.warning(
                        "symmetric_predicates must be a list. Skipping this section."
                    )
                else:
                    symmetric = []
                    for p in data["symmetric_predicates"]:
                        if isinstance(p, str) and not p.strip().startswith("#"):
                            symmetric.append(p)
                        elif not isinstance(p, str):
                            logger.warning(
                                f"Invalid entry in symmetric_predicates: {p} (not a string). Skipping."
                            )
                    result["symmetric_predicates"] = set(symmetric)
                    logger.info(
                        f"Loaded {len(result['symmetric_predicates'])} symmetric predicates"
                    )

            # Validate and extract type constraints
            if "predicate_type_constraints" in data:
                constraints = data["predicate_type_constraints"]
                if not isinstance(constraints, dict):
                    logger.warning(
                        "predicate_type_constraints must be a dict. Skipping this section."
                    )
                else:
                    # Convert list format to consistent format
                    for predicate, allowed_pairs in constraints.items():
                        if not isinstance(predicate, str):
                            logger.warning(
                                f"Invalid predicate key in type_constraints: {predicate}. Skipping."
                            )
                            continue

                        if not isinstance(allowed_pairs, list):
                            logger.warning(
                                f"Type constraints for '{predicate}' must be a list. Skipping."
                            )
                            continue

                        # Ensure all pairs are lists of length 2
                        normalized_pairs = []
                        for pair in allowed_pairs:
                            if isinstance(pair, list) and len(pair) == 2:
                                if all(isinstance(t, str) for t in pair):
                                    normalized_pairs.append(pair)
                                else:
                                    logger.warning(
                                        f"Type constraint pair for '{predicate}' contains non-string: {pair}. Skipping."
                                    )
                            else:
                                logger.warning(
                                    f"Invalid type constraint pair for '{predicate}': {pair} "
                                    f"(must be list of 2 strings). Skipping."
                                )

                        if normalized_pairs:
                            result["predicate_type_constraints"][
                                predicate
                            ] = normalized_pairs

                    logger.info(
                        f"Loaded type constraints for {len(result['predicate_type_constraints'])} predicates"
                    )

        except Exception as e:
            logger.warning(
                f"Failed to load canonical_predicates.yml from {canonical_file}: {e}. "
                "Continuing without predicate filtering."
            )
    else:
        logger.warning(
            f"Canonical predicates file not found at {canonical_file}. "
            "Continuing without predicate filtering."
        )

    # Load predicate_map.yml
    predicate_map_file = ontology_path / "predicate_map.yml"
    if predicate_map_file.exists():
        try:
            with open(predicate_map_file, "r", encoding="utf-8") as f:
                data = yaml.safe_load(f) or {}

            # Validate predicate_map structure (must be Dict[str, str])
            if not isinstance(data, dict):
                logger.warning(
                    "predicate_map.yml must contain a dictionary. Skipping this file."
                )
            else:
                # Filter out comments and empty values
                for key, value in data.items():
                    if isinstance(key, str) and not key.strip().startswith("#"):
                        if isinstance(value, str):
                            result["predicate_map"][key] = value
                        else:
                            logger.warning(
                                f"Invalid value in predicate_map for key '{key}': {value} "
                                f"(must be string). Skipping."
                            )

            logger.info(f"Loaded {len(result['predicate_map'])} predicate mappings")

        except Exception as e:
            logger.warning(
                f"Failed to load predicate_map.yml from {predicate_map_file}: {e}. "
                "Continuing without predicate mapping."
            )
    else:
        logger.warning(
            f"Predicate map file not found at {predicate_map_file}. "
            "Continuing without predicate mapping."
        )

    # Load types.yml or entity_types.yml (optional, for future use)
    # Try types.yml first, then entity_types.yml as fallback
    types_file = ontology_path / "types.yml"
    entity_types_file = ontology_path / "entity_types.yml"

    if types_file.exists():
        try:
            with open(types_file, "r", encoding="utf-8") as f:
                data = yaml.safe_load(f) or {}

            if isinstance(data, dict):
                if "type_map" in data:
                    if isinstance(data["type_map"], dict):
                        result["type_map"] = data["type_map"]
                        logger.info(
                            f"Loaded {len(result['type_map'])} type mappings from types.yml"
                        )
                    else:
                        logger.warning(
                            "type_map in types.yml must be a dictionary. Skipping."
                        )
        except Exception as e:
            logger.warning(
                f"Failed to load types.yml from {types_file}: {e}. "
                "Trying entity_types.yml as fallback."
            )
            types_file = None  # Mark as failed

    # Fallback to entity_types.yml if types.yml not found or failed
    if (not types_file or not types_file.exists()) and entity_types_file.exists():
        try:
            with open(entity_types_file, "r", encoding="utf-8") as f:
                data = yaml.safe_load(f) or {}

            if isinstance(data, dict):
                if "type_map" in data:
                    if isinstance(data["type_map"], dict):
                        result["type_map"] = data["type_map"]
                        logger.info(
                            f"Loaded {len(result['type_map'])} type mappings from entity_types.yml"
                        )
                    else:
                        logger.warning(
                            "type_map in entity_types.yml must be a dictionary. Skipping."
                        )
                elif "merge_suggestions" in data:
                    # Alternative structure: extract type_map from merge_suggestions
                    merge_suggestions = data.get("merge_suggestions", {})
                    if isinstance(merge_suggestions, dict):
                        # Convert merge_suggestions to type_map format if needed
                        type_map = {}
                        for non_canonical, canonical_info in merge_suggestions.items():
                            if isinstance(canonical_info, dict):
                                canonical = canonical_info.get("canonical")
                                if canonical:
                                    type_map[non_canonical] = canonical
                            elif isinstance(canonical_info, str):
                                type_map[non_canonical] = canonical_info
                        if type_map:
                            result["type_map"] = type_map
                            logger.info(
                                f"Loaded {len(result['type_map'])} type mappings from "
                                f"entity_types.yml merge_suggestions"
                            )
        except Exception as e:
            logger.warning(
                f"Failed to load entity_types.yml from {entity_types_file}: {e}. "
                "Continuing without type mapping."
            )
    # Don't warn if both files are missing - it's optional

    # Cache the result
    _ontology_cache = result

    # Log summary with validation status
    sections_loaded = []
    sections_failed = []

    if result["canonical_predicates"]:
        sections_loaded.append(
            f"{len(result['canonical_predicates'])} canonical predicates"
        )
    else:
        sections_failed.append("canonical_predicates")

    if result["symmetric_predicates"]:
        sections_loaded.append(
            f"{len(result['symmetric_predicates'])} symmetric predicates"
        )
    else:
        sections_failed.append("symmetric_predicates")

    if result["predicate_map"]:
        sections_loaded.append(f"{len(result['predicate_map'])} mappings")
    else:
        sections_failed.append("predicate_map")

    if result["predicate_type_constraints"]:
        sections_loaded.append(
            f"{len(result['predicate_type_constraints'])} type constraints"
        )
    else:
        sections_failed.append("predicate_type_constraints")

    if result["type_map"]:
        sections_loaded.append(f"{len(result['type_map'])} type mappings")
    else:
        sections_failed.append("type_map")

    if sections_loaded:
        logger.info(f"Ontology loaded successfully: {', '.join(sections_loaded)}")
        if sections_failed:
            logger.warning(
                f"Ontology sections not loaded: {', '.join(sections_failed)}. "
                "Some filtering may be disabled."
            )
    else:
        logger.warning(
            "No ontology data loaded. Extraction will proceed without predicate filtering."
        )

    return result


def _get_empty_ontology() -> Dict[str, Any]:
    """Return empty ontology structure for fallback."""
    return {
        "canonical_predicates": set(),
        "symmetric_predicates": set(),
        "predicate_map": {},
        "predicate_type_constraints": {},
        "type_map": {},
    }


def clear_cache():
    """Clear the ontology cache (useful for testing)."""
    global _ontology_cache
    _ontology_cache = None
