"""
Pipeline Stage Statistics

Migrated from app/api/pipeline_stats.py to stages_api.
Provides per-stage statistics for GraphRAG pipeline.

Achievement 2.1: Stage Stats API
"""

import logging
import os
import sys
from typing import Dict, Any, Optional

# Add project root to Python path for imports
_project_root = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
if _project_root not in sys.path:
    sys.path.insert(0, _project_root)

logger = logging.getLogger(__name__)


def get_all_stage_stats(
    db_name: Optional[str] = None,
    read_db_name: Optional[str] = None,
    write_db_name: Optional[str] = None,
) -> Dict[str, Any]:
    """
    Get statistics from all GraphRAG pipeline stages.

    Args:
        db_name: Optional database name
        read_db_name: Optional read database name
        write_db_name: Optional write database name

    Returns:
        Dictionary containing stats for all stages
    """
    try:
        from src.core.config.graphrag import GraphRAGPipelineConfig
        from src.domain.stages.graphrag.extraction import GraphExtractionStage
        from src.domain.stages.graphrag.entity_resolution import EntityResolutionStage
        from src.domain.stages.graphrag.graph_construction import GraphConstructionStage
        from src.domain.stages.graphrag.community_detection import CommunityDetectionStage
    except ImportError as e:
        logger.warning(f"Could not import stage classes: {e}")
        return {"error": f"Stage classes not available: {e}"}

    # Create default config
    config = GraphRAGPipelineConfig()
    if db_name:
        config.extraction_config.db_name = db_name
    if read_db_name:
        config.extraction_config.read_db_name = read_db_name
    if write_db_name:
        config.extraction_config.write_db_name = write_db_name

    stats = {}

    try:
        # Stage 1: Graph Extraction
        extraction_stage = GraphExtractionStage(config.extraction_config)
        stats["graph_extraction"] = extraction_stage.get_processing_stats()
        stats["graph_extraction"]["stage_name"] = "graph_extraction"
        stats["graph_extraction"]["stage_display_name"] = "Graph Extraction"
    except Exception as e:
        logger.error(f"Error getting extraction stats: {e}")
        stats["graph_extraction"] = {"error": str(e)}

    try:
        # Stage 2: Entity Resolution
        resolution_stage = EntityResolutionStage(config.resolution_config)
        stats["entity_resolution"] = resolution_stage.get_resolution_stats()
        stats["entity_resolution"]["stage_name"] = "entity_resolution"
        stats["entity_resolution"]["stage_display_name"] = "Entity Resolution"
    except Exception as e:
        logger.error(f"Error getting resolution stats: {e}")
        stats["entity_resolution"] = {"error": str(e)}

    try:
        # Stage 3: Graph Construction
        construction_stage = GraphConstructionStage(config.construction_config)
        stats["graph_construction"] = construction_stage.get_construction_stats()
        stats["graph_construction"]["stage_name"] = "graph_construction"
        stats["graph_construction"]["stage_display_name"] = "Graph Construction"
    except Exception as e:
        logger.error(f"Error getting construction stats: {e}")
        stats["graph_construction"] = {"error": str(e)}

    try:
        # Stage 4: Community Detection
        detection_stage = CommunityDetectionStage(config.detection_config)
        stats["community_detection"] = detection_stage.get_detection_stats()
        stats["community_detection"]["stage_name"] = "community_detection"
        stats["community_detection"]["stage_display_name"] = "Community Detection"
    except Exception as e:
        logger.error(f"Error getting detection stats: {e}")
        stats["community_detection"] = {"error": str(e)}

    # Calculate aggregate statistics
    try:
        extraction = stats.get("graph_extraction", {})
        resolution = stats.get("entity_resolution", {})
        construction = stats.get("graph_construction", {})
        detection = stats.get("community_detection", {})

        stats["aggregate"] = {
            "total_chunks": extraction.get("total_chunks", 0),
            "total_entities": resolution.get("total_entities", 0),
            "total_relationships": construction.get("total_relationships", 0),
            "total_communities": detection.get("total_communities", 0),
            "stages_completed": sum(
                1
                for stage_stats in [extraction, resolution, construction, detection]
                if stage_stats.get("completion_rate", 0) >= 0.95
            ),
            "overall_completion_rate": _calculate_overall_completion(
                extraction, resolution, construction, detection
            ),
        }
    except Exception as e:
        logger.error(f"Error calculating aggregate stats: {e}")
        stats["aggregate"] = {"error": str(e)}

    return stats


def _calculate_overall_completion(
    extraction: Dict,
    resolution: Dict,
    construction: Dict,
    detection: Dict,
) -> float:
    """Calculate overall completion rate across all stages."""
    rates = [
        extraction.get("completion_rate", 0),
        resolution.get("completion_rate", 0),
        construction.get("completion_rate", 0),
        detection.get("completion_rate", 0),
    ]
    if any(rates):
        return sum(rates) / 4
    return 0.0


def get_stage_stats(stage_name: str, db_name: Optional[str] = None) -> Dict[str, Any]:
    """
    Get statistics for a specific stage.

    Args:
        stage_name: Stage name (graph_extraction, entity_resolution, etc.)
        db_name: Optional database name

    Returns:
        Dictionary containing stats for the stage
    """
    all_stats = get_all_stage_stats(db_name=db_name)
    
    if stage_name in all_stats:
        return all_stats[stage_name]
    
    return {"error": f"Unknown stage: {stage_name}"}

