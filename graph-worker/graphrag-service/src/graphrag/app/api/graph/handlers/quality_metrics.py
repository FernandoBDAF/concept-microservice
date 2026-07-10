"""
Quality Metrics Handler - Business Logic

Pure functions for quality metrics operations.
No HTTP handling - that's in router.py

Achievement 6.1: Quality Metrics Dashboard
"""

import logging
import os
import sys
from typing import Dict, Any, Optional

# Add project root to Python path for imports
_project_root = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "..", ".."))
if _project_root not in sys.path:
    sys.path.insert(0, _project_root)

from src.infrastructure.database.mongodb import get_mongo_client
from src.domain.services.graphrag.indexes import get_graphrag_collections

logger = logging.getLogger(__name__)


def get_stage_metrics(
    db_name: str,
    stage: Optional[str] = None,
) -> Dict[str, Any]:
    """
    Get quality metrics for one or all stages.

    Args:
        db_name: Database name
        stage: Optional stage name (extraction, resolution, construction, detection)

    Returns:
        Dictionary with quality metrics per stage
    """
    try:
        from src.core.config.graphrag import GraphRAGPipelineConfig
        from src.domain.stages.graphrag.extraction import GraphExtractionStage
        from src.domain.stages.graphrag.entity_resolution import EntityResolutionStage
        from src.domain.stages.graphrag.graph_construction import GraphConstructionStage
        from src.domain.stages.graphrag.community_detection import CommunityDetectionStage
    except ImportError as e:
        logger.warning(f"Could not import stage classes: {e}")
        return {"error": "Stage classes not available"}

    client = get_mongo_client()
    db = client[db_name]
    collections = get_graphrag_collections(db)

    # Create default config
    config = GraphRAGPipelineConfig()
    config.extraction_config.db_name = db_name
    config.resolution_config.db_name = db_name
    config.construction_config.db_name = db_name
    config.detection_config.db_name = db_name

    metrics = {}

    # Extraction metrics
    if stage is None or stage == "extraction":
        try:
            extraction_stage = GraphExtractionStage(config.extraction_config)
            extraction_stats = extraction_stage.get_processing_stats()

            metrics["extraction"] = {
                "completion_rate": extraction_stats.get("completion_rate", 0),
                "failure_rate": extraction_stats.get("failure_rate", 0),
                "total_chunks": extraction_stats.get("total_chunks", 0),
                "processed_chunks": extraction_stats.get("processed_chunks", 0),
                "failed_chunks": extraction_stats.get("failed_chunks", 0),
                "canonical_ratio": 0.95,  # Placeholder
            }
        except Exception as e:
            logger.error(f"Error getting extraction metrics: {e}")
            metrics["extraction"] = {"error": str(e)}

    # Resolution metrics
    if stage is None or stage == "resolution":
        try:
            resolution_stage = EntityResolutionStage(config.resolution_config)
            resolution_stats = resolution_stage.get_resolution_stats()

            total_entities = resolution_stats.get("total_entities", 0)
            total_mentions = resolution_stats.get("total_mentions", 0)
            merge_rate = (
                (total_mentions - total_entities) / total_mentions if total_mentions > 0 else 0
            )

            metrics["resolution"] = {
                "completion_rate": resolution_stats.get("completion_rate", 0),
                "failure_rate": resolution_stats.get("failure_rate", 0),
                "total_entities": total_entities,
                "total_mentions": total_mentions,
                "merge_rate": merge_rate,
                "duplicate_reduction": merge_rate,
                "llm_call_rate": 0.1,  # Placeholder
            }
        except Exception as e:
            logger.error(f"Error getting resolution metrics: {e}")
            metrics["resolution"] = {"error": str(e)}

    # Construction metrics
    if stage is None or stage == "construction":
        try:
            construction_stage = GraphConstructionStage(config.construction_config)
            construction_stats = construction_stage.get_construction_stats()
            graph_metrics = construction_stage.calculate_graph_metrics()

            metrics["construction"] = {
                "completion_rate": construction_stats.get("completion_rate", 0),
                "failure_rate": construction_stats.get("failure_rate", 0),
                "total_relationships": construction_stats.get("total_relationships", 0),
                "graph_density": graph_metrics.get("density", 0),
                "avg_degree": graph_metrics.get("avg_degree", 0),
                "connected_components": graph_metrics.get("connected_components", 1),
            }
        except Exception as e:
            logger.error(f"Error getting construction metrics: {e}")
            metrics["construction"] = {"error": str(e)}

    # Detection metrics
    if stage is None or stage == "detection":
        try:
            detection_stage = CommunityDetectionStage(config.detection_config)
            detection_stats = detection_stage.get_detection_stats()

            metrics["detection"] = {
                "completion_rate": detection_stats.get("completion_rate", 0),
                "total_communities": detection_stats.get("total_communities", 0),
                "avg_community_size": detection_stats.get("avg_community_size", 0),
                "modularity": detection_stats.get("modularity", 0),
                "coverage": detection_stats.get("coverage", 0),
            }
        except Exception as e:
            logger.error(f"Error getting detection metrics: {e}")
            metrics["detection"] = {"error": str(e)}

    return metrics

