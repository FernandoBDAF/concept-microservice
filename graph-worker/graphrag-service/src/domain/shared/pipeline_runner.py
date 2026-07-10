from __future__ import annotations

import os
import logging
from dataclasses import dataclass
from typing import Any, Dict, List, Optional, Type, Union

from src.lib.error_handling.decorators import handle_errors
from src.lib.error_handling.exceptions import PipelineError
from src.lib.metrics import Counter, Histogram, Timer, MetricRegistry

logger = logging.getLogger(__name__)

# Initialize pipeline metrics (shared across all pipelines)
_pipeline_runs = Counter(
    "pipeline_runs", "Number of pipeline runs started", labels=["pipeline_type"]
)
_pipeline_completed = Counter(
    "pipeline_completed", "Number of pipeline runs completed", labels=["pipeline_type"]
)
_pipeline_failed = Counter(
    "pipeline_failed", "Number of pipeline runs failed", labels=["pipeline_type"]
)
_pipeline_duration = Histogram(
    "pipeline_duration_seconds", "Pipeline execution duration", labels=["pipeline_type"]
)
_pipeline_stage_failures = Counter(
    "pipeline_stage_failures",
    "Number of stage failures in pipelines",
    labels=["pipeline_type", "stage"],
)

# Register metrics
_registry = MetricRegistry.get_instance()
_registry.register(_pipeline_runs)
_registry.register(_pipeline_completed)
_registry.register(_pipeline_failed)
_registry.register(_pipeline_duration)
_registry.register(_pipeline_stage_failures)

# Stage imports (registry)
from src.domain.stages.ingestion.clean import CleanStage, CleanConfig
from src.domain.stages.ingestion.chunk import ChunkStage, ChunkConfig
from src.domain.stages.ingestion.enrich import EnrichStage, EnrichConfig
from src.domain.stages.ingestion.ingest import IngestStage, IngestConfig
from src.domain.stages.ingestion.embed import EmbedStage, EmbedConfig
from src.domain.stages.ingestion.redundancy import RedundancyStage, RedundancyConfig
from src.domain.stages.ingestion.trust import TrustStage, TrustConfig
from src.domain.stages.ingestion.compress import CompressStage, CompressConfig
from src.domain.stages.ingestion.backfill_transcript import (
    BackfillTranscriptStage,
    BackfillTranscriptConfig,
)

# GraphRAG stage imports
from src.domain.stages.graphrag.extraction import GraphExtractionStage
from src.domain.stages.graphrag.entity_resolution import EntityResolutionStage
from src.domain.stages.graphrag.graph_construction import GraphConstructionStage
from src.domain.stages.graphrag.community_detection import CommunityDetectionStage
from src.core.models.config import BaseStageConfig


StageClass = Union[
    Type[CleanStage],
    Type[ChunkStage],
    Type[EnrichStage],
    Type[IngestStage],
    Type[EmbedStage],
    Type[RedundancyStage],
    Type[TrustStage],
    Type[CompressStage],
    Type[BackfillTranscriptStage],
    Type[GraphExtractionStage],
    Type[EntityResolutionStage],
    Type[GraphConstructionStage],
    Type[CommunityDetectionStage],
]


STAGE_REGISTRY: Dict[str, StageClass] = {
    "clean": CleanStage,
    "chunk": ChunkStage,
    "enrich": EnrichStage,
    "ingest": IngestStage,
    "embed": EmbedStage,
    "redundancy": RedundancyStage,
    "trust": TrustStage,
    "compress": CompressStage,
    "backfill_transcript": BackfillTranscriptStage,
    "graph_extraction": GraphExtractionStage,
    "entity_resolution": EntityResolutionStage,
    "graph_construction": GraphConstructionStage,
    "community_detection": CommunityDetectionStage,
}


@dataclass
class StageSpec:
    """Strongly-typed stage specification.

    - stage can be a registry key or a Stage class
    - config must be an instance of the exact ConfigCls for the stage (or None for defaults)
    """

    stage: Union[str, StageClass]
    config: Optional[BaseStageConfig] = None


class PipelineRunner:
    def __init__(
        self,
        specs: List[StageSpec],
        default_read_db: Optional[str] = None,
        default_write_db: Optional[str] = None,
        stop_on_error: bool = True,
    ) -> None:
        self.specs = specs
        self.default_read_db = default_read_db
        self.default_write_db = default_write_db
        self.stop_on_error = stop_on_error

    def _resolve_stage_class(self, ref: Union[str, StageClass]) -> StageClass:
        if isinstance(ref, str):
            key = ref.strip().lower()
            if key not in STAGE_REGISTRY:
                raise KeyError(f"Unknown stage: {ref}")
            return STAGE_REGISTRY[key]
        return ref

    @handle_errors(log_traceback=True, capture_context=True, reraise=True)
    def run(self, pipeline_type: str = "unknown") -> int:
        """Run all pipeline stages with comprehensive error handling and metrics.

        Args:
            pipeline_type: Type of pipeline (e.g., "ingestion", "graphrag") for metrics labels
        """
        # Track pipeline start
        pipeline_labels = {"pipeline_type": pipeline_type}
        _pipeline_runs.inc(labels=pipeline_labels)

        with Timer() as timer:
            exit_codes: List[int] = []
            totals = {"stages": 0, "failed": 0}

            for i, spec in enumerate(self.specs, start=1):
                try:
                    stage_cls = self._resolve_stage_class(spec.stage)
                    stage = stage_cls()

                    # Build config object from the stage's ConfigCls
                    cfg_cls = getattr(stage, "ConfigCls", None)
                    if cfg_cls is None:
                        raise RuntimeError(f"Stage {stage_cls.__name__} missing ConfigCls")

                    if spec.config is None:
                        config = cfg_cls()  # type: ignore
                    else:
                        if not isinstance(spec.config, cfg_cls):  # type: ignore
                            raise TypeError(
                                f"Config type mismatch for stage {stage_cls.__name__}: "
                                f"expected {cfg_cls.__name__}, got {type(spec.config).__name__}"
                            )
                        config = spec.config

                    logger.info(
                        f"[PIPELINE] Starting stage {i}/{len(self.specs)}: {stage.name}"
                    )
                    print(
                        f"[pipeline] ({i}/{len(self.specs)}) Running {stage.name} with read_db={config.read_db_name or config.db_name} write_db={config.write_db_name or config.db_name}"
                    )

                    # Run stage with error context
                    code = stage.run(config)

                    exit_codes.append(code)
                    totals["stages"] += 1

                    if code != 0:
                        totals["failed"] += 1
                        # Track stage failure
                        _pipeline_stage_failures.inc(
                            labels={**pipeline_labels, "stage": stage.name}
                        )
                        logger.error(
                            f"[PIPELINE] Stage {stage.name} failed with exit code {code}"
                        )
                        print(f"[pipeline] Stage {stage.name} failed with code {code}")
                        if self.stop_on_error:
                            print("[pipeline] stop_on_error=True → stopping")
                            # Track pipeline failure
                            _pipeline_failed.inc(labels=pipeline_labels)
                            _pipeline_duration.observe(timer.elapsed(), labels=pipeline_labels)
                            return code
                    else:
                        logger.info(f"[PIPELINE] Stage {stage.name} completed successfully")

                except Exception as e:
                    # Capture stage execution errors with full context
                    totals["failed"] += 1
                    stage_name = (
                        spec.stage
                        if isinstance(spec.stage, str)
                        else getattr(spec.stage, "name", "unknown")
                    )

                    # Track stage failure
                    _pipeline_stage_failures.inc(
                        labels={**pipeline_labels, "stage": stage_name}
                    )

                    logger.error(
                        f"[PIPELINE] Stage {stage_name} crashed: {type(e).__name__}: {e}",
                        exc_info=True,
                    )

                    if self.stop_on_error:
                        # Track pipeline failure
                        _pipeline_failed.inc(labels=pipeline_labels)
                        _pipeline_duration.observe(timer.elapsed(), labels=pipeline_labels)
                        raise PipelineError(
                            f"Pipeline failed at stage {stage_name}",
                            context={
                                "stage": stage_name,
                                "stage_index": i,
                                "total_stages": len(self.specs),
                                "stages_completed": totals["stages"],
                            },
                            cause=e,
                        )
                    else:
                        # Continue to next stage
                        logger.warning(
                            f"[PIPELINE] Continuing to next stage despite failure"
                        )
                        exit_codes.append(1)

            succeeded = totals["stages"] - totals["failed"]
            
            # Track pipeline completion
            if totals["failed"] == 0:
                _pipeline_completed.inc(labels=pipeline_labels)
            else:
                _pipeline_failed.inc(labels=pipeline_labels)
            
            # Track pipeline duration
            _pipeline_duration.observe(timer.elapsed(), labels=pipeline_labels)
        
        logger.info(
            f"[PIPELINE] Completed: {succeeded}/{totals['stages']} stages succeeded, {totals['failed']} failed"
        )
        print(
            f"[pipeline] Completed stages: {succeeded}/{totals['stages']}; failures: {totals['failed']}"
        )
        return 0 if totals["failed"] == 0 else 1
