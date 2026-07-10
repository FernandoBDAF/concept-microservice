"""
GraphRAG Pipeline

This module implements the complete GraphRAG pipeline that orchestrates
all stages from entity extraction to community detection and summarization.
"""

import logging
import time
import uuid
import argparse
from typing import Dict, List, Any, Optional
from src.core.config.graphrag import GraphRAGPipelineConfig
from src.domain.pipelines.runner import StageSpec, PipelineRunner
from src.domain.services.graphrag.indexes import (
    create_graphrag_indexes,
    ensure_graphrag_collections,
)
from src.domain.services.graphrag.quality_metrics import QualityMetricsService
from src.lib.error_handling.context import error_context
from src.lib.error_handling.decorators import handle_errors

logger = logging.getLogger(__name__)

# Stage dependencies for GraphRAG pipeline
# Achievement 0.1: Stage Selection & Partial Runs
STAGE_DEPENDENCIES = {
    "graph_extraction": [],  # No dependencies
    "entity_resolution": ["graph_extraction"],
    "graph_construction": ["entity_resolution"],
    "community_detection": ["graph_construction"],
}

# Stage name mappings (short names to full names)
STAGE_NAME_MAP = {
    "extraction": "graph_extraction",
    "resolution": "entity_resolution",
    "construction": "graph_construction",
    "detection": "community_detection",
    "graph_extraction": "graph_extraction",
    "entity_resolution": "entity_resolution",
    "graph_construction": "graph_construction",
    "community_detection": "community_detection",
}

# Stage order in pipeline
STAGE_ORDER = [
    "graph_extraction",
    "entity_resolution",
    "graph_construction",
    "community_detection",
]


class GraphRAGPipeline:
    """
    Complete GraphRAG pipeline orchestrating all stages.
    """

    def __init__(self, config: GraphRAGPipelineConfig):
        """
        Initialize the GraphRAG Pipeline.

        EXPERIMENT SAFETY (2024-11-04):
        - read_db and write_db MUST be explicitly specified
        - NO defaults to prevent accidental data mixing between experiments
        - Exception: If both missing, assume single-DB mode for backward compatibility

        Args:
            config: Configuration for the pipeline
        """
        self.config = config

        # Validate explicit DB specification for experiment mode
        # If running experiments, BOTH read_db and write_db must be explicit
        read_db = config.extraction_config.read_db_name
        write_db = config.extraction_config.write_db_name

        if read_db or write_db:  # At least one specified → experiment mode
            if not read_db:
                raise ValueError(
                    "❌ GraphRAG pipeline requires explicit --read-db-name when running experiments.\n"
                    "This prevents accidental data mixing. Specify the source database explicitly."
                )
            if not write_db:
                raise ValueError(
                    "❌ GraphRAG pipeline requires explicit --write-db-name when running experiments.\n"
                    "This prevents accidental data mixing. Specify the target database explicitly."
                )
            logger.info(f"🔬 Experiment mode: read_db={read_db}, write_db={write_db}")
            if config.experiment_id:
                logger.info(f"📊 Experiment ID: {config.experiment_id}")

        # Achievement 0.1: Stage Selection & Partial Runs
        # Parse stage selection if provided in config
        selected_stages = None
        if hasattr(config, "selected_stages") and config.selected_stages:
            selected_stages = self._resolve_stage_selection(
                self._parse_stage_selection(config.selected_stages), auto_include_deps=True
            )
            logger.info(f"🎯 Stage selection: {selected_stages}")

        self.specs = self._create_stage_specs(stage_filter=selected_stages)
        self.runner = PipelineRunner(self.specs, stop_on_error=not config.continue_on_error)

        # Initialize database connection for setup()
        from src.infrastructure.database.mongodb import get_mongo_client
        from src.core.config.paths import DB_NAME

        self.client = get_mongo_client()
        db_name = config.extraction_config.db_name or DB_NAME
        self.db = self.client[db_name]  # ✅ Now available for setup()

        # Achievement 0.1: Generate trace_id for transformation logging
        # Generate unique trace_id for this pipeline run
        self.trace_id = str(uuid.uuid4())
        logger.info(f"🔍 Trace ID generated: {self.trace_id}")

        # Set trace_id on all stage configs so stages can access it
        self._set_trace_id_on_configs()

        logger.info("Initialized GraphRAGPipeline with PipelineRunner")

        # Track experiment metadata if experiment_id is provided
        if config.experiment_id:
            self._track_experiment_start()

        # Achievement 1.1: Initialize metrics tracker
        from src.domain.services.observability.prometheus_metrics import get_metrics_tracker

        pipeline_id = config.experiment_id or f"pipeline_{int(time.time())}"
        self.metrics_tracker = get_metrics_tracker(pipeline_id=pipeline_id)

        # Achievement 0.4: Initialize quality metrics service
        import os

        metrics_enabled = os.getenv("GRAPHRAG_QUALITY_METRICS", "true").lower() == "true"
        self.quality_metrics = QualityMetricsService(self.db, enabled=metrics_enabled)
        logger.info(f"Quality metrics collection: {'enabled' if metrics_enabled else 'disabled'}")

    def _track_experiment_start(self):
        """
        Track experiment metadata for comparative analysis.

        Stores experiment configuration and start time in experiment_tracking collection.
        Useful for comparing multiple runs with different configurations.
        """
        if not self.config.experiment_id:
            return

        from datetime import datetime

        # Store in write_db if specified, otherwise use default db
        tracking_db_name = (
            self.config.extraction_config.write_db_name
            or self.config.extraction_config.db_name
            or "mongo_hack"
        )
        tracking_db = self.client[tracking_db_name]
        tracking_coll = tracking_db.experiment_tracking

        metadata = {
            "experiment_id": self.config.experiment_id,
            "pipeline_type": "graphrag",
            "started_at": datetime.utcnow(),
            "status": "running",
            "trace_id": getattr(
                self, "trace_id", None
            ),  # Achievement 0.1: Trace ID System Integration
            "configuration": {
                "read_db": self.config.extraction_config.read_db_name,
                "write_db": self.config.extraction_config.write_db_name,
                "concurrency": self.config.extraction_config.concurrency,
                "community_detection": {
                    "algorithm": self.config.detection_config.algorithm,
                    "resolution": self.config.detection_config.resolution_parameter,
                    "min_cluster_size": self.config.detection_config.min_cluster_size,
                    "max_cluster_size": self.config.detection_config.max_cluster_size,
                    "model": self.config.detection_config.model_name,
                },
            },
        }

        # Upsert experiment metadata
        tracking_coll.update_one(
            {"experiment_id": self.config.experiment_id},
            {"$set": metadata},
            upsert=True,
        )

        logger.info(f"📊 Experiment metadata tracked: {self.config.experiment_id}")

    def _set_trace_id_on_configs(self):
        """
        Set trace_id on all stage configs so stages can access it for transformation logging.

        Achievement 0.1: Trace ID System Integration
        """
        if hasattr(self, "trace_id") and self.trace_id:
            self.config.extraction_config.trace_id = self.trace_id
            self.config.resolution_config.trace_id = self.trace_id
            self.config.construction_config.trace_id = self.trace_id
            self.config.detection_config.trace_id = self.trace_id
            logger.debug(f"Set trace_id on all stage configs: {self.trace_id}")

    def _create_stage_specs(self, stage_filter: Optional[List[str]] = None) -> List[StageSpec]:
        """
        Create stage specifications for the GraphRAG pipeline using registry keys.

        Achievement 0.1: Stage Selection & Partial Runs
        - If stage_filter is provided, only include those stages
        - Maintains original stage order

        Args:
            stage_filter: Optional list of stage names to include

        Returns:
            List of stage specifications
        """
        all_specs = [
            StageSpec(
                stage="graph_extraction",  # ✅ Use registry key
                config=self.config.extraction_config,
            ),
            StageSpec(
                stage="entity_resolution",
                config=self.config.resolution_config,
            ),
            StageSpec(
                stage="graph_construction",
                config=self.config.construction_config,
            ),
            StageSpec(
                stage="community_detection",
                config=self.config.detection_config,
            ),
        ]

        if stage_filter is None:
            return all_specs

        # Filter specs based on selection, maintaining order
        filtered_specs = []
        for spec in all_specs:
            stage_name = spec.stage if isinstance(spec.stage, str) else spec.stage.name
            if stage_name in stage_filter:
                filtered_specs.append(spec)

        return filtered_specs

    def setup(self) -> None:
        """
        Set up the GraphRAG pipeline by creating necessary collections and indexes.
        """
        logger.info("Setting up GraphRAG pipeline...")

        try:
            # Ensure GraphRAG collections exist
            # Note: This handles existing collections gracefully
            ensure_graphrag_collections(self.db)

            # Create GraphRAG indexes
            # Note: Index creation is idempotent (duplicate indexes are ignored)
            create_graphrag_indexes(self.db)

            logger.info("GraphRAG pipeline setup completed successfully")

        except Exception as e:
            # Log error but don't fail if collections already exist
            error_msg = str(e).lower()
            if "already exists" in error_msg or "collection" in error_msg and "exists" in error_msg:
                logger.warning(
                    f"Some GraphRAG collections may already exist: {e}. "
                    f"Continuing with pipeline execution."
                )
            else:
                logger.error(f"Failed to setup GraphRAG pipeline: {e}")
                raise

    def run_stage(self, stage_name: str) -> int:
        """Run a specific stage."""
        logger.info(f"Running GraphRAG stage: {stage_name}")

        # Find stage spec
        for spec in self.specs:
            if spec.stage == stage_name:
                # Run single stage using PipelineRunner with metrics
                return PipelineRunner([spec]).run(pipeline_type="graphrag")

        raise ValueError(f"Unknown stage: {stage_name}")

    def _parse_stage_selection(self, stage_input: Optional[str]) -> Optional[List[str]]:
        """
        Parse stage selection input into list of stage names.

        Achievement 0.1: Stage Selection & Partial Runs

        Supports formats:
        - "extraction,resolution" (comma-separated names)
        - "1-3" (range)
        - "1,3,4" (indices)
        - "graph_extraction,entity_resolution" (full names)

        Args:
            stage_input: Stage selection string or None

        Returns:
            List of stage names or None (all stages)
        """
        if not stage_input or not stage_input.strip():
            return None

        stage_input = stage_input.strip()
        stages = []

        # Check for range format (e.g., "1-3")
        if "-" in stage_input and "," not in stage_input:
            try:
                start, end = map(int, stage_input.split("-"))
                if start < 1 or end > len(STAGE_ORDER) or start > end:
                    raise ValueError(
                        f"Invalid stage range: {stage_input}. "
                        f"Must be between 1 and {len(STAGE_ORDER)}"
                    )
                stages = [STAGE_ORDER[i - 1] for i in range(start, end + 1)]
            except ValueError as e:
                raise ValueError(f"Invalid stage range format: {stage_input}. {e}")

        # Check for comma-separated list
        elif "," in stage_input:
            parts = [p.strip() for p in stage_input.split(",")]
            for part in parts:
                # Check if it's a number (index)
                try:
                    idx = int(part)
                    if idx < 1 or idx > len(STAGE_ORDER):
                        raise ValueError(
                            f"Invalid stage index: {idx}. "
                            f"Must be between 1 and {len(STAGE_ORDER)}"
                        )
                    stages.append(STAGE_ORDER[idx - 1])
                except ValueError:
                    # Not a number, treat as name
                    stage_name = STAGE_NAME_MAP.get(part.lower())
                    if not stage_name:
                        raise ValueError(
                            f"Unknown stage name: {part}. "
                            f"Valid stages: {list(STAGE_NAME_MAP.keys())}"
                        )
                    stages.append(stage_name)

        # Single value
        else:
            # Check if it's a number
            try:
                idx = int(stage_input)
                if idx < 1 or idx > len(STAGE_ORDER):
                    raise ValueError(
                        f"Invalid stage index: {idx}. " f"Must be between 1 and {len(STAGE_ORDER)}"
                    )
                stages = [STAGE_ORDER[idx - 1]]
            except ValueError:
                # Not a number, treat as name
                stage_name = STAGE_NAME_MAP.get(stage_input.lower())
                if not stage_name:
                    raise ValueError(
                        f"Unknown stage name: {stage_input}. "
                        f"Valid stages: {list(STAGE_NAME_MAP.keys())}"
                    )
                stages = [stage_name]

        return stages

    def _get_stage_dependencies(self, stage_name: str) -> List[str]:
        """
        Get all dependencies for a stage (recursive).

        Achievement 0.1: Stage Selection & Partial Runs

        Args:
            stage_name: Name of the stage

        Returns:
            List of dependency stage names (in dependency order)
        """
        deps = STAGE_DEPENDENCIES.get(stage_name, [])
        # Recursively get dependencies of dependencies
        all_deps = []
        for dep in deps:
            all_deps.extend(self._get_stage_dependencies(dep))
            if dep not in all_deps:
                all_deps.append(dep)
        return all_deps

    def _validate_stage_dependencies(self, selected_stages: List[str]) -> List[str]:
        """
        Validate that all dependencies are met for selected stages.

        Achievement 0.1: Stage Selection & Partial Runs

        Args:
            selected_stages: List of selected stage names

        Returns:
            List of missing dependency stage names
        """
        missing = []
        selected_set = set(selected_stages)

        for stage in selected_stages:
            deps = self._get_stage_dependencies(stage)
            for dep in deps:
                if dep not in selected_set:
                    missing.append(dep)

        return missing

    def _warn_out_of_order(self, selected_stages: List[str]) -> None:
        """
        Warn if stages are selected out of order.

        Achievement 0.3: Stage Dependency Validation

        Args:
            selected_stages: List of selected stage names
        """
        if len(selected_stages) <= 1:
            return  # No ordering issue with single or no stages

        # Get indices in STAGE_ORDER
        stage_indices = {}
        for idx, stage in enumerate(STAGE_ORDER):
            stage_indices[stage] = idx

        # Check if selected stages are in order
        indices = []
        for stage in selected_stages:
            if stage in stage_indices:
                indices.append(stage_indices[stage])
            else:
                # Unknown stage, skip ordering check
                return

        # Check if indices are in ascending order
        if indices != sorted(indices):
            logger.warning(
                f"⚠️  Stages selected out of order: {selected_stages}. "
                f"Pipeline will run stages in correct order: {[STAGE_ORDER[i] for i in sorted(indices)]}"
            )

    def _resolve_stage_selection(
        self, selected_stages: Optional[List[str]], auto_include_deps: bool = True
    ) -> List[str]:
        """
        Resolve stage selection, optionally auto-including src.infrastructure.

        Achievement 0.1: Stage Selection & Partial Runs
        Achievement 0.3: Stage Dependency Validation (enhanced with warnings)

        Args:
            selected_stages: List of selected stage names (or None for all)
            auto_include_deps: If True, auto-include dependencies; if False, raise error

        Returns:
            Resolved list of stage names (with dependencies if auto_include_deps=True)

        Raises:
            ValueError: If dependencies are missing and auto_include_deps=False
        """
        if selected_stages is None:
            return STAGE_ORDER.copy()

        # Achievement 0.3: Warn if stages are out of order
        self._warn_out_of_order(selected_stages)

        # Get all dependencies
        missing = self._validate_stage_dependencies(selected_stages)

        if missing and not auto_include_deps:
            raise ValueError(
                f"Selected stages {selected_stages} are missing dependencies: {missing}. "
                f"Either include them or use auto_include_deps=True"
            )

        # Log dependency auto-inclusion if needed
        if missing and auto_include_deps:
            logger.info(f"📦 Auto-including missing dependencies for {selected_stages}: {missing}")

        # Combine selected stages and dependencies, maintaining order
        all_stages = selected_stages + missing
        # Remove duplicates while preserving order
        resolved = []
        seen = set()
        for stage in STAGE_ORDER:
            if stage in all_stages and stage not in seen:
                resolved.append(stage)
                seen.add(stage)

        return resolved

    def _filter_stage_specs(self, selected_stages: List[str]) -> List[StageSpec]:
        """
        Filter stage specs based on selected stages, maintaining order.

        Achievement 0.1: Stage Selection & Partial Runs

        Args:
            selected_stages: List of stage names to include

        Returns:
            Filtered list of stage specs in original order
        """
        return self._create_stage_specs(stage_filter=selected_stages)

    def run_stages(self, stage_selection: Optional[str] = None) -> int:
        """
        Run selected stages of the pipeline.

        Achievement 0.1: Stage Selection & Partial Runs

        Args:
            stage_selection: Stage selection string (e.g., "extraction,resolution" or "1-3")

        Returns:
            Exit code (0 for success, non-zero for failure)
        """
        # Parse and resolve stage selection
        parsed = self._parse_stage_selection(stage_selection)
        resolved = self._resolve_stage_selection(parsed, auto_include_deps=True)

        logger.info(f"🎯 Running selected stages: {resolved}")

        # Filter specs
        filtered_specs = self._filter_stage_specs(resolved)

        # Create new runner with filtered specs
        filtered_runner = PipelineRunner(
            filtered_specs, stop_on_error=not self.config.continue_on_error
        )

        # Run filtered pipeline
        return filtered_runner.run(pipeline_type="graphrag")

    def _detect_stage_completion(self) -> Dict[str, float]:
        """
        Detect which stages have completed by checking chunk status in database.

        Achievement 0.2: Resume from Failure

        Checks each stage's completion status by querying chunks collection.
        Returns completion ratio (0.0 to 1.0) for each stage.

        Returns:
            Dictionary mapping stage name to completion ratio (0.0-1.0)
        """
        from src.core.config.paths import COLL_CHUNKS

        # Get chunks collection from write_db (where stages write their status)
        # Use write_db if specified, otherwise use default db
        db_name = (
            self.config.extraction_config.write_db_name
            or self.config.extraction_config.db_name
            or "mongo_hack"
        )
        db = self.client[db_name]
        collection = db[COLL_CHUNKS]

        # Count total chunks
        total_chunks = collection.count_documents({})
        if total_chunks == 0:
            # No chunks, nothing is complete
            return {stage: 0.0 for stage in STAGE_ORDER}

        completion_ratios = {}

        # Check each stage's completion status
        stage_status_fields = {
            "graph_extraction": "graphrag_extraction.status",
            "entity_resolution": "graphrag_resolution.status",
            "graph_construction": "graphrag_construction.status",
            "community_detection": "graphrag_communities.status",
        }

        for stage_name in STAGE_ORDER:
            status_field = stage_status_fields[stage_name]
            # Count chunks with this stage completed
            completed_count = collection.count_documents({status_field: "completed"})
            completion_ratios[stage_name] = (
                completed_count / total_chunks if total_chunks > 0 else 0.0
            )

        return completion_ratios

    def _get_last_completed_stage(
        self, completion_ratios: Dict[str, float], threshold: float = 0.95
    ) -> Optional[str]:
        """
        Get the last stage that is considered complete.

        Achievement 0.2: Resume from Failure

        Args:
            completion_ratios: Dictionary of stage completion ratios
            threshold: Completion threshold (default 0.95 = 95%)

        Returns:
            Last completed stage name, or None if no stages are complete
        """
        last_completed = None
        for stage_name in STAGE_ORDER:
            if completion_ratios.get(stage_name, 0.0) >= threshold:
                last_completed = stage_name
            else:
                # First incomplete stage found, stop
                break

        return last_completed

    def _get_stages_to_run(
        self, completion_ratios: Dict[str, float], threshold: float = 0.95
    ) -> List[str]:
        """
        Get list of stages that need to be run (not yet complete).

        Achievement 0.2: Resume from Failure

        Args:
            completion_ratios: Dictionary of stage completion ratios
            threshold: Completion threshold (default 0.95 = 95%)

        Returns:
            List of stage names that need to be run (in order)
        """
        stages_to_run = []
        for stage_name in STAGE_ORDER:
            if completion_ratios.get(stage_name, 0.0) < threshold:
                stages_to_run.append(stage_name)

        return stages_to_run

    def run_with_resume(self, completion_threshold: float = 0.95) -> int:
        """
        Run pipeline with resume capability - skip completed stages.

        Achievement 0.2: Resume from Failure

        Detects which stages have completed and only runs incomplete stages.

        Args:
            completion_threshold: Threshold for considering a stage complete (default 0.95)

        Returns:
            Exit code (0 for success, non-zero for failure)
        """
        logger.info("🔄 Resume mode: Detecting completed stages...")

        # Detect stage completion
        completion_ratios = self._detect_stage_completion()

        # Log completion status
        for stage_name, ratio in completion_ratios.items():
            status = "✅ Complete" if ratio >= completion_threshold else "⏳ Incomplete"
            logger.info(f"  {stage_name}: {ratio:.1%} {status}")

        # Get stages to run
        stages_to_run = self._get_stages_to_run(completion_ratios, completion_threshold)

        if not stages_to_run:
            logger.info("✅ All stages already completed. Nothing to run.")
            return 0

        last_completed = self._get_last_completed_stage(completion_ratios, completion_threshold)
        if last_completed:
            logger.info(
                f"📌 Last completed stage: {last_completed}. " f"Resuming from: {stages_to_run[0]}"
            )
        else:
            logger.info(f"🔄 No stages completed. Running all stages from start.")

        # Convert stage list to selection string
        stage_selection = ",".join(stages_to_run)

        # Run only incomplete stages
        logger.info(f"🎯 Running stages: {stage_selection}")
        return self.run_stages(stage_selection)

    @handle_errors(log_traceback=True, capture_context=True, reraise=True)
    def run_full_pipeline(self, resume: bool = False) -> int:
        """
        Run the complete GraphRAG pipeline with comprehensive error handling.

        Achievement 0.2: Resume from Failure
        - If resume=True, automatically detects and skips completed stages

        Achievement 1.1: Prometheus Metrics Export
        - Tracks pipeline status, stage progress, throughput, and errors

        Args:
            resume: If True, resume from last failure (skip completed stages)

        Returns:
            Exit code (0 for success, non-zero for failure)
        """
        if resume:
            logger.info("🔄 Running pipeline with resume mode")
            return self.run_with_resume()

        logger.info("Starting full GraphRAG pipeline execution")

        # Achievement 1.1: Track pipeline start
        self.metrics_tracker.set_pipeline_status("running")

        with error_context(
            "graphrag_pipeline_execution",
            pipeline="graphrag",
            stages=len(self.specs),
        ):
            # Setup (create indexes, etc.)
            logger.info("[PIPELINE] Running setup (collections, indexes)")
            self.setup()
            logger.info("[PIPELINE] Setup complete")

            # Run pipeline using PipelineRunner with metrics
            logger.info(f"[PIPELINE] Starting {len(self.specs)} stages")
            exit_code = self.runner.run(pipeline_type="graphrag")

            # Achievement 1.1: Update pipeline status
            if exit_code == 0:
                logger.info("GraphRAG pipeline completed successfully")
                self.metrics_tracker.set_pipeline_status("completed")

                # Achievement 0.4: Calculate and store quality metrics
                logger.info(f"Calculating quality metrics for trace_id={self.trace_id}")
                try:
                    metrics = self.quality_metrics.calculate_all_metrics(self.trace_id)
                    self.quality_metrics.store_metrics(self.trace_id, metrics)

                    # Check for out-of-range metrics
                    warnings = self.quality_metrics.check_healthy_ranges(metrics)
                    for stage, stage_warnings in warnings.items():
                        if stage_warnings:
                            logger.warning(f"Quality warnings for {stage}: {stage_warnings}")

                    logger.info("Quality metrics calculated and stored successfully")
                except Exception as e:
                    logger.error(f"Failed to calculate quality metrics: {e}")
                    # Don't fail the pipeline if metrics calculation fails
            else:
                logger.error(f"GraphRAG pipeline failed with exit code {exit_code}")
                self.metrics_tracker.set_pipeline_status("failed")

            return exit_code

    # TODO: Implement retry logic in PipelineRunner for production reliability
    # Desired behavior:
    # - Configurable retry attempts via config.max_retries
    # - Exponential backoff via config.retry_delay
    # - Selective retry based on error types (transient vs permanent)
    # - Per-stage retry configuration
    # Example:
    #   PipelineRunner(specs, max_retries=3, retry_delay=5.0, retry_on_errors=[...])

    # TODO: Implement comprehensive statistics aggregation for pipeline monitoring
    # Desired behavior:
    # - Real-time stats collection during pipeline execution
    # - Aggregation across all stages (total processed, failed, skipped)
    # - Performance metrics (avg time per doc, throughput)
    # - Progress tracking (percentage complete, ETA)
    # - Integration with monitoring dashboard
    # Implementation should use callbacks or hooks in PipelineRunner

    # Note: _calculate_overall_stats() removed - functionality covered by statistics aggregation TODO above

    def get_pipeline_status(self) -> Dict[str, Any]:
        """Get current status of the GraphRAG pipeline."""
        try:
            from src.domain.pipelines.runner import STAGE_REGISTRY

            stage_statuses = {}

            for spec in self.specs:
                stage_name = spec.stage if isinstance(spec.stage, str) else spec.stage.name
                stage_cls = (
                    STAGE_REGISTRY.get(stage_name) if isinstance(spec.stage, str) else spec.stage
                )

                if not stage_cls:
                    stage_statuses[stage_name] = {"error": "Stage class not found in registry"}
                    continue

                try:
                    # Create temporary stage instance to get stats
                    stage = stage_cls()
                    stage.setup()  # Initialize to get stats methods

                    # Get stats using actual method names
                    stats = {}
                    if hasattr(stage, "get_processing_stats"):  # GraphExtractionStage
                        stats = stage.get_processing_stats()
                    elif hasattr(stage, "get_resolution_stats"):  # EntityResolutionStage
                        stats = stage.get_resolution_stats()
                    elif hasattr(stage, "get_construction_stats"):  # GraphConstructionStage
                        stats = stage.get_construction_stats()
                    elif hasattr(stage, "get_detection_stats"):  # CommunityDetectionStage
                        stats = stage.get_detection_stats()
                    else:
                        stats = {
                            "status": "unknown",
                            "message": "No stats method available",
                        }

                    stage_statuses[stage_name] = stats

                except Exception as e:
                    logger.warning(f"Failed to get stats for stage {stage_name}: {e}")
                    stage_statuses[stage_name] = {"error": str(e)}

            return {
                "pipeline_status": "active",
                "stage_statuses": stage_statuses,
                "timestamp": time.time(),
            }

        except Exception as e:
            logger.error(f"Failed to get pipeline status: {e}")
            return {
                "pipeline_status": "error",
                "error": str(e),
                "timestamp": time.time(),
            }

    def cleanup_failed_stages(self) -> Dict[str, int]:
        """Clean up failed stage records to allow retry."""
        from src.domain.pipelines.runner import STAGE_REGISTRY

        cleanup_results = {}

        for spec in self.specs:
            stage_name = spec.stage if isinstance(spec.stage, str) else spec.stage.name
            stage_cls = (
                STAGE_REGISTRY.get(stage_name) if isinstance(spec.stage, str) else spec.stage
            )

            if not stage_cls:
                logger.warning(f"Stage class not found for {stage_name}")
                cleanup_results[stage_name] = 0
                continue

            try:
                # Create temporary stage instance to call cleanup methods
                stage = stage_cls()

                # Apply config if available (needed for cleanup methods that query collections)
                if spec.config:
                    # Temporarily set config for cleanup operations
                    stage.config = spec.config

                # Call cleanup method based on stage type
                count = 0
                if hasattr(stage, "cleanup_failed_extractions"):
                    count = stage.cleanup_failed_extractions()
                elif hasattr(stage, "cleanup_failed_resolutions"):
                    count = stage.cleanup_failed_resolutions()
                elif hasattr(stage, "cleanup_failed_constructions"):
                    count = stage.cleanup_failed_constructions()
                elif hasattr(stage, "cleanup_failed_detections"):
                    count = stage.cleanup_failed_detections()
                else:
                    logger.warning(f"Stage {stage_name} has no cleanup method")

                cleanup_results[stage_name] = count

            except Exception as e:
                logger.error(f"Failed to cleanup stage {stage_name}: {e}")
                cleanup_results[stage_name] = 0

        return cleanup_results


def create_graphrag_pipeline(
    config: Optional[GraphRAGPipelineConfig] = None,
) -> GraphRAGPipeline:
    """
    Create a GraphRAG pipeline with default configuration.

    Args:
        config: Optional custom configuration

    Returns:
        GraphRAGPipeline instance
    """
    if config is None:
        config = GraphRAGPipelineConfig()

    return GraphRAGPipeline(config)


if __name__ == "__main__":
    # CLI interface for running the GraphRAG pipeline
    parser = argparse.ArgumentParser(description="GraphRAG Pipeline Runner")
    parser.add_argument("--stage", help="Run specific stage only")
    parser.add_argument("--video-id", help="Process specific video ID")
    parser.add_argument("--max", type=int, help="Maximum number of documents to process")
    parser.add_argument("--dry-run", action="store_true", help="Dry run mode")
    parser.add_argument("--verbose", action="store_true", help="Verbose logging")

    # Achievement 4.1: Testing infrastructure arguments
    parser.add_argument("--experiment-id", help="Experiment ID for tracking test runs")
    parser.add_argument("--db-name", help="Database name for pipeline operations")
    parser.add_argument("--read-db-name", help="Database name to read input data from")
    parser.add_argument("--write-db-name", help="Database name to write output data to")

    args = parser.parse_args()

    # Configure logging
    # Note: This is a basic fallback. For full logging features (file output,
    # third-party silencing, etc.), use run_graphrag_pipeline.py instead.
    log_level = logging.DEBUG if args.verbose else logging.INFO
    logging.basicConfig(
        level=log_level,
        format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
    )

    # Silencing noisy loggers (minimal, use run_graphrag_pipeline.py for full setup)
    for logger_name in ["numba", "graspologic", "pymongo", "urllib3", "httpcore"]:
        logging.getLogger(logger_name).setLevel(logging.WARNING)
    logging.getLogger("httpx").setLevel(logging.INFO)

    # Create pipeline config with CLI arguments
    # Achievement 4.1: Pass experiment_id and database arguments to config
    import os

    env = dict(os.environ)

    # Add experiment_id to env if provided
    if args.experiment_id:
        env["EXPERIMENT_ID"] = args.experiment_id

    # Get default database name
    from src.core.config.paths import DB_NAME

    default_db = args.db_name or env.get("DB_NAME") or DB_NAME

    # Create config from args and env
    # This will pass args to all stage configs via from_args_env()
    config = GraphRAGPipelineConfig.from_args_env(args, env, default_db)

    # Create pipeline
    pipeline = create_graphrag_pipeline(config)

    # Run pipeline
    if args.stage:
        result = pipeline.run_stage(args.stage)
    else:
        result = pipeline.run_full_pipeline()

    print(f"Pipeline result: {result}")
