"""
Ingestion Pipeline

This module implements the complete ingestion pipeline that orchestrates
all stages from raw document ingestion to processed chunks with quality scores.
"""

import logging
import os
import argparse
from typing import Dict, List, Any, Optional
from dataclasses import dataclass

from src.domain.pipelines.runner import StageSpec, PipelineRunner
from src.domain.stages.ingestion.ingest import IngestConfig
from src.domain.stages.ingestion.clean import CleanConfig
from src.domain.stages.ingestion.enrich import EnrichConfig
from src.domain.stages.ingestion.chunk import ChunkConfig
from src.domain.stages.ingestion.embed import EmbedConfig
from src.domain.stages.ingestion.redundancy import RedundancyConfig
from src.domain.stages.ingestion.trust import TrustConfig
from src.domain.stages.ingestion.backfill_transcript import BackfillTranscriptConfig

logger = logging.getLogger(__name__)


@dataclass
class IngestionPipelineConfig:
    """Configuration for the Ingestion Pipeline."""

    # Pipeline-level settings
    db_name: Optional[str] = None
    continue_on_error: bool = True
    verbose: bool = False
    dry_run: bool = False

    # Stage configurations
    ingest_config: Optional[IngestConfig] = None
    clean_config: Optional[CleanConfig] = None
    enrich_config: Optional[EnrichConfig] = None
    chunk_config: Optional[ChunkConfig] = None
    embed_config: Optional[EmbedConfig] = None
    redundancy_config: Optional[RedundancyConfig] = None
    trust_config: Optional[TrustConfig] = None
    backfill_transcript_config: Optional[BackfillTranscriptConfig] = None

    @classmethod
    def from_args_env(cls, args, env, default_db):
        """Create configuration from CLI arguments and environment variables.

        Matches the configuration from yt_clean_enrich.py for consistency.
        MongoDB URI is loaded from .env file via get_mongo_client().
        """
        # Get database name
        db_name = (
            getattr(args, "db_name", None)
            or env.get("DB_NAME")
            or env.get("MONGODB_DB")
            or env.get("ATLAS_DB_NAME")
            or default_db
            or os.getenv("MONGODB_DB")
            or os.getenv("ATLAS_DB_NAME")
            or os.getenv("DB_NAME", "mongo_hack")
        )

        # Create stage configurations with defaults
        ingest_config = IngestConfig.from_args_env(args, env, default_db or db_name)
        clean_config = CleanConfig.from_args_env(args, env, default_db or db_name)
        enrich_config = EnrichConfig.from_args_env(args, env, default_db or db_name)
        chunk_config = ChunkConfig.from_args_env(args, env, default_db or db_name)
        embed_config = EmbedConfig.from_args_env(args, env, default_db or db_name)
        redundancy_config = RedundancyConfig.from_args_env(
            args, env, default_db or db_name
        )
        trust_config = TrustConfig.from_args_env(args, env, default_db or db_name)
        backfill_transcript_config = BackfillTranscriptConfig.from_args_env(
            args, env, default_db or db_name
        )

        # Apply yt_clean_enrich.py matching configurations
        # These match the reference file to ensure consistent behavior

        # Clean: LLM always enabled (required), with proper concurrency and retries
        clean_config.use_llm = True  # Always enabled - no option to disable
        clean_config.concurrency = getattr(args, "concurrency", None) or 15
        clean_config.llm_retries = 4
        clean_config.llm_backoff_s = 10.0
        clean_config.model_name = (
            os.getenv("BEDROCK_MODEL_ID")
            or os.getenv("OPENAI_DEFAULT_MODEL")
            or "gpt-4o-mini"
        )

        # Chunk: Recursive strategy with larger tokens and overlap (matching yt_clean_enrich.py)
        chunk_config.chunk_strategy = "recursive"
        chunk_config.token_size = 1200  # Increased from 500 for coherent passages
        chunk_config.overlap_pct = (
            0.20  # Increased from 0.15 for better context preservation
        )
        chunk_config.split_chars = [".", "?", "!"]

        # Enrich: LLM always enabled (required), with proper concurrency and retries
        enrich_config.use_llm = True  # Always enabled - no option to disable
        enrich_config.concurrency = getattr(args, "concurrency", None) or 15
        enrich_config.llm_retries = 4
        enrich_config.llm_backoff_s = 10.0
        enrich_config.model_name = (
            os.getenv("BEDROCK_MODEL_ID")
            or os.getenv("OPENAI_DEFAULT_MODEL")
            or "gpt-4o-mini"
        )

        # Embed: Match yt_clean_enrich.py settings
        embed_config.embed_source = "chunk"
        embed_config.use_hybrid_embedding_text = True
        embed_config.unit_normalize_embeddings = True
        embed_config.emit_multi_vectors = False

        # Apply pipeline-level settings to stage configs if needed
        verbose = getattr(args, "verbose", False)
        if verbose:
            clean_config.verbose = True
            enrich_config.verbose = True
            chunk_config.verbose = True
            embed_config.verbose = True
            redundancy_config.verbose = True
            trust_config.verbose = True

        if getattr(args, "dry_run", False):
            clean_config.dry_run = True
            enrich_config.dry_run = True
            chunk_config.dry_run = True
            embed_config.dry_run = True
            redundancy_config.dry_run = True
            trust_config.dry_run = True

        # Apply upsert_existing flag to all stages if provided
        # Argparse converts --upsert-existing to upsert_existing attribute
        upsert_existing = getattr(args, "upsert_existing", False)
        logger.info(
            f"Checking upsert_existing: args.upsert_existing={upsert_existing}, redundancy_config.upsert_existing={redundancy_config.upsert_existing}"
        )
        if upsert_existing:
            logger.info(f"Applying upsert_existing=True to all stage configs")
            clean_config.upsert_existing = True
            enrich_config.upsert_existing = True
            chunk_config.upsert_existing = True
            embed_config.upsert_existing = True
            redundancy_config.upsert_existing = True
            trust_config.upsert_existing = True
            logger.info(
                f"After override: redundancy_config.upsert_existing={redundancy_config.upsert_existing}, chunk_config.upsert_existing={chunk_config.upsert_existing}"
            )
        else:
            # Also check if it was already set in stage configs (from BaseStageConfig.from_args_env)
            logger.debug(
                f"upsert_existing flag not set; redundancy_config.upsert_existing={redundancy_config.upsert_existing}"
            )

        # Note: LLM is always enabled for clean and enrich (required for quality)
        # Only redundancy and trust stages have optional LLM for edge cases
        # (they use heuristics by default, LLM only for borderline cases)

        return cls(
            db_name=db_name,
            continue_on_error=env.get("PIPELINE_CONTINUE_ON_ERROR", "true").lower()
            == "true",
            verbose=getattr(args, "verbose", False),
            dry_run=getattr(args, "dry_run", False),
            ingest_config=ingest_config,
            clean_config=clean_config,
            enrich_config=enrich_config,
            chunk_config=chunk_config,
            embed_config=embed_config,
            redundancy_config=redundancy_config,
            trust_config=trust_config,
            backfill_transcript_config=backfill_transcript_config,
        )


class IngestionPipeline:
    """
    Complete Ingestion pipeline orchestrating all stages from raw ingestion
    to processed chunks with quality scores.

    Pipeline Flow: ingest → clean → chunk → enrich → embed → redundancy → trust

    Note: Chunk comes before enrich because enrichment works on individual chunks,
    not whole transcripts. This allows chunk-specific entity/concept extraction.
    """

    def __init__(self, config: IngestionPipelineConfig):
        """
        Initialize the Ingestion Pipeline.

        Args:
            config: Configuration for the pipeline
        """
        self.config = config
        self.specs = self._create_stage_specs()
        self.runner = PipelineRunner(
            self.specs, stop_on_error=not config.continue_on_error
        )

        # Initialize database connection for setup()
        from src.infrastructure.database.mongodb import get_mongo_client
        from src.core.config.paths import DB_NAME

        self.client = get_mongo_client()
        db_name = config.db_name or DB_NAME
        self.db = self.client[db_name]

        logger.info("Initialized IngestionPipeline with PipelineRunner")

    def _create_stage_specs(self) -> List[StageSpec]:
        """
        Create stage specifications for the Ingestion pipeline using registry keys.

        Returns:
            List of stage specifications
        """
        return [
            StageSpec(
                stage="ingest",
                config=self.config.ingest_config,
            ),
            StageSpec(
                stage="clean",
                config=self.config.clean_config,
            ),
            StageSpec(
                stage="chunk",
                config=self.config.chunk_config,
            ),
            StageSpec(
                stage="enrich",
                config=self.config.enrich_config,
            ),
            StageSpec(
                stage="embed",
                config=self.config.embed_config,
            ),
            StageSpec(
                stage="redundancy",
                config=self.config.redundancy_config,
            ),
            StageSpec(
                stage="trust",
                config=self.config.trust_config,
            ),
            StageSpec(
                stage="backfill_transcript",
                config=self.config.backfill_transcript_config,
            ),
        ]

    def setup(self) -> None:
        """
        Set up the Ingestion pipeline by ensuring necessary collections and indexes exist.
        """
        logger.info("Setting up Ingestion pipeline...")

        try:
            # Ensure collections exist (they should be created by seed/index scripts)
            # This is a placeholder for any pipeline-specific setup
            from scripts.utilities.seed.seed_indexes import ensure_collections_and_indexes

            ensure_collections_and_indexes(self.db)

            logger.info("Ingestion pipeline setup completed successfully")

        except Exception as e:
            logger.error(f"Failed to setup Ingestion pipeline: {e}")
            # Non-fatal error - collections may already exist
            logger.warning("Continuing with pipeline execution...")

    def run_stage(self, stage_name: str) -> int:
        """Run a specific stage."""
        logger.info(f"Running Ingestion stage: {stage_name}")

        # Find stage spec
        for spec in self.specs:
            if spec.stage == stage_name:
                # Run single stage using PipelineRunner with metrics
                return PipelineRunner([spec]).run(pipeline_type="ingestion")

        raise ValueError(f"Unknown stage: {stage_name}")

    def run_full_pipeline(self) -> int:
        """Run the complete Ingestion pipeline."""
        logger.info("Starting full Ingestion pipeline execution")

        # Setup (ensure collections and indexes)
        self.setup()

        # Run pipeline using PipelineRunner with metrics
        exit_code = self.runner.run(pipeline_type="ingestion")

        if exit_code == 0:
            logger.info("Ingestion pipeline completed successfully")
        else:
            logger.error("Ingestion pipeline failed")

        return exit_code

    @classmethod
    def from_cli_args(cls, args, kwargs: Dict[str, Any]) -> "IngestionPipeline":
        """
        Create an IngestionPipeline instance from CLI arguments and keyword arguments.

        Args:
            args: Parsed CLI arguments
            kwargs: Additional keyword arguments (playlist_id, channel_id, video_ids, etc.)

        Returns:
            IngestionPipeline instance
        """
        import os

        env = dict(os.environ)
        default_db = (
            os.getenv("DB_NAME")
            or os.getenv("MONGODB_DB")
            or os.getenv("ATLAS_DB_NAME")
            or "mongo_hack"
        )

        # Merge kwargs into args namespace for from_args_env
        for key, value in kwargs.items():
            if not hasattr(args, key):
                setattr(args, key, value)

        config = IngestionPipelineConfig.from_args_env(args, env, default_db)
        return cls(config)


def create_ingestion_pipeline(
    config: Optional[IngestionPipelineConfig] = None,
) -> IngestionPipeline:
    """
    Create an Ingestion pipeline with default configuration.

    Args:
        config: Optional custom configuration

    Returns:
        IngestionPipeline instance
    """
    if config is None:
        import os

        env = dict(os.environ)
        default_db = os.getenv("DB_NAME", "mongo_hack")
        # Create a minimal args object
        args = argparse.Namespace()
        config = IngestionPipelineConfig.from_args_env(args, env, default_db)

    return IngestionPipeline(config)


if __name__ == "__main__":
    # CLI interface for running the Ingestion pipeline
    parser = argparse.ArgumentParser(description="Ingestion Pipeline Runner")
    parser.add_argument("--stage", help="Run specific stage only")
    parser.add_argument("--playlist_id", help="YouTube playlist ID")
    parser.add_argument("--channel_id", help="YouTube channel ID")
    parser.add_argument("--video_ids", nargs="*", help="List of video IDs")
    parser.add_argument(
        "--max", type=int, help="Maximum number of documents to process"
    )
    parser.add_argument(
        "--llm", action="store_true", help="Enable LLM for supported stages"
    )
    parser.add_argument("--dry-run", action="store_true", help="Dry run mode")
    parser.add_argument("--verbose", action="store_true", help="Verbose logging")
    parser.add_argument("--db-name", help="Database name override")

    args = parser.parse_args()

    # Configure logging
    log_level = logging.DEBUG if args.verbose else logging.INFO
    logging.basicConfig(level=log_level)

    # Create pipeline
    kwargs = {
        "playlist_id": args.playlist_id,
        "channel_id": args.channel_id,
        "video_ids": args.video_ids,
        "max": args.max,
        "llm": args.llm,
    }
    pipeline = IngestionPipeline.from_cli_args(args, kwargs)

    # Run pipeline
    if args.stage:
        result = pipeline.run_stage(args.stage)
    else:
        result = pipeline.run_full_pipeline()

    exit(result)
