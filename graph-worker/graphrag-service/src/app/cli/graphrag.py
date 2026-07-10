#!/usr/bin/env python3
"""
GraphRAG Pipeline Runner

This script provides a command-line interface for running the GraphRAG pipeline.
It supports running individual stages or the complete pipeline with various options.
"""

import argparse
import logging
import sys
import os

# Add the project root to the Python path
sys.path.append(
    os.path.dirname(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))))
)

from src.domain.graphrag.pipeline import create_graphrag_pipeline
from src.core.config.graphrag import GraphRAGPipelineConfig
from src.domain.graphrag.pipeline import GraphRAGPipeline
from src.lib.error_handling.context import error_context
from src.lib.error_handling.decorators import handle_errors
from src.lib.logging import log_exception
from dotenv import load_dotenv

# Load environment variables
load_dotenv()


def setup_logging(verbose: bool = False, log_file: str = None, stage: str = None) -> None:
    """
    Set up logging configuration for GraphRAG pipeline.

    Args:
        verbose: Enable verbose (DEBUG) logging
        log_file: Optional path to log file (default: logs/pipeline/graphrag_STAGE_TIMESTAMP.log)
        stage: Optional stage name to include in log filename

    Note:
        Uses the core logging library's setup_logging function for consistency.
    """
    from src.lib.logging import (
        setup_logging as core_setup_logging,
        create_timestamped_log_path,
    )

    # Create default log file path if not provided
    if log_file is None:
        prefix = f"graphrag_{stage}" if stage else "graphrag"
        log_file = create_timestamped_log_path(
            base_dir="logs/pipeline", prefix=prefix, extension="log"
        )

    # Use core library's setup_logging function
    core_setup_logging(
        verbose=verbose,
        log_file=log_file,
        silence_third_party=True,
    )


def create_config_from_args(args) -> GraphRAGPipelineConfig:
    """
    Create pipeline configuration from command line arguments.

    Priority (highest to lowest):
    1. CLI flags (--read-db-name, --write-db-name, etc.)
    2. Config file (--config path/to/config.json)
    3. Environment variables
    4. REQUIRED: read_db and write_db (NO defaults to prevent mistakes)
    """
    import json

    env = dict(os.environ)
    default_db = os.getenv("DB_NAME", "mongo_hack")

    # Load config file if provided (base layer)
    file_config = {}
    if hasattr(args, "config") and args.config:
        logger = logging.getLogger(__name__)
        logger.info(f"Loading configuration from: {args.config}")
        try:
            with open(args.config, "r") as f:
                file_config = json.load(f)
            logger.info(f"Loaded config: experiment_id={file_config.get('experiment_id', 'N/A')}")
        except Exception as e:
            logger.error(f"Failed to load config file '{args.config}': {e}")
            sys.exit(1)

    # Merge config file into environment (config file < env vars < CLI flags)
    # This allows config file to set defaults that env/CLI can override
    if file_config:
        # Apply file config to env if not already set
        for key, value in file_config.items():
            if key == "experiment_id":
                env.setdefault("EXPERIMENT_ID", value)
            elif key == "read_db":
                env.setdefault("GRAPHRAG_READ_DB", value)
            elif key == "write_db":
                env.setdefault("GRAPHRAG_WRITE_DB", value)
            elif key == "concurrency":
                env.setdefault("GRAPHRAG_CONCURRENCY", str(value))
            # Add stage-specific configs
            elif key == "community_detection" and isinstance(value, dict):
                for sub_key, sub_val in value.items():
                    env_key = f"GRAPHRAG_COMMUNITY_{sub_key.upper()}"
                    if sub_key == "algorithm":
                        env_key = "GRAPHRAG_COMMUNITY_ALGORITHM"
                    elif sub_key == "resolution":
                        env_key = "GRAPHRAG_RESOLUTION_PARAMETER"
                    env.setdefault(env_key, str(sub_val))

    # Use from_args_env to build config (CLI flags override everything)
    config = GraphRAGPipelineConfig.from_args_env(args, env, default_db)

    # Override with any command-line specific args not in from_args_env
    if hasattr(args, "log_file") and args.log_file:
        config.log_file = args.log_file

    # Achievement 0.1: Stage Selection & Partial Runs
    if hasattr(args, "stages") and args.stages:
        config.selected_stages = args.stages

    # Achievement 0.2: Resume from Failure
    if hasattr(args, "resume") and args.resume:
        config.resume_from_failure = True

    # Store experiment_id if provided (for tracking)
    if file_config.get("experiment_id"):
        config.experiment_id = file_config["experiment_id"]

    return config


def run_single_stage(pipeline: GraphRAGPipeline, stage_name: str) -> None:
    """Run a single stage of the GraphRAG pipeline."""
    logger = logging.getLogger(__name__)

    logger.info(f"Running GraphRAG stage: {stage_name}")

    try:
        exit_code = pipeline.run_stage(stage_name)

        if exit_code == 0:
            logger.info(f"Stage {stage_name} completed successfully")
        else:
            logger.error(f"Stage {stage_name} failed with exit code {exit_code}")
            sys.exit(exit_code)

    except Exception as e:
        log_exception(logger, f"Error running stage {stage_name}", e)
        sys.exit(1)


@handle_errors(log_traceback=True, capture_context=True, reraise=True)
def run_full_pipeline(pipeline: GraphRAGPipeline) -> None:
    """Run the complete GraphRAG pipeline with comprehensive error handling."""
    logger = logging.getLogger(__name__)

    logger.info("Starting full GraphRAG pipeline execution")

    with error_context("graphrag_full_pipeline", stages=4):
        exit_code = pipeline.run_full_pipeline()

        if exit_code == 0:
            logger.info("GraphRAG pipeline completed successfully")
        else:
            logger.error(f"GraphRAG pipeline failed with exit code {exit_code}")
            sys.exit(exit_code)


def show_pipeline_status(pipeline: GraphRAGPipeline) -> None:
    """
    Show the current status of the GraphRAG pipeline.

    Args:
        pipeline: GraphRAG pipeline instance
    """
    logger = logging.getLogger(__name__)

    try:
        status = pipeline.get_pipeline_status()

        logger.info("GraphRAG Pipeline Status:")
        logger.info(f"Pipeline Status: {status['pipeline_status']}")

        for stage_name, stage_status in status["stage_statuses"].items():
            logger.info(f"  {stage_name}: {stage_status}")

    except Exception as e:
        logger.error(f"Error getting pipeline status: {e}")
        sys.exit(1)


def cleanup_failed_stages(pipeline: GraphRAGPipeline) -> None:
    """
    Clean up failed stage records.

    Args:
        pipeline: GraphRAG pipeline instance
    """
    logger = logging.getLogger(__name__)

    try:
        cleanup_results = pipeline.cleanup_failed_stages()

        logger.info("Cleanup Results:")
        for stage_name, count in cleanup_results.items():
            logger.info(f"  {stage_name}: {count} records cleaned up")

    except Exception as e:
        logger.error(f"Error cleaning up failed stages: {e}")
        sys.exit(1)


def main():
    """Main function for the GraphRAG pipeline runner."""
    parser = argparse.ArgumentParser(
        description="GraphRAG Pipeline Runner",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
        Examples:
        # Run the complete pipeline
        python run_graphrag_pipeline.py

        # Run a specific stage
        python run_graphrag_pipeline.py --stage graph_extraction

        # Process a specific video
        python run_graphrag_pipeline.py --video-id video_123

        # Dry run with verbose logging
        python run_graphrag_pipeline.py --dry-run --verbose

        # Show pipeline status
        python run_graphrag_pipeline.py --status

        # Clean up failed stages
        python run_graphrag_pipeline.py --cleanup
        """,
    )

    # Configuration file
    parser.add_argument(
        "--config", type=str, help="Path to JSON config file (sets defaults, CLI flags override)"
    )

    # Main operation arguments
    parser.add_argument(
        "--stage",
        choices=[
            "graph_extraction",
            "entity_resolution",
            "graph_construction",
            "community_detection",
        ],
        help="Run specific stage only (legacy - use --stages for multiple stages)",
    )
    parser.add_argument(
        "--stages",
        type=str,
        help=(
            "Run selected stages (comma-separated names, range, or indices). "
            "Examples: 'extraction,resolution', '1-3', '1,3,4'. "
            "Dependencies are automatically included."
        ),
    )
    parser.add_argument(
        "--resume",
        action="store_true",
        help=(
            "Resume from last failure. Automatically detects completed stages "
            "and skips them, resuming from the first incomplete stage."
        ),
    )
    parser.add_argument("--status", action="store_true", help="Show pipeline status")
    parser.add_argument("--cleanup", action="store_true", help="Clean up failed stage records")

    # Processing arguments
    parser.add_argument("--video-id", help="Process specific video ID")
    parser.add_argument("--max", type=int, help="Maximum number of documents to process")
    parser.add_argument(
        "--dry-run", action="store_true", help="Dry run mode (no actual processing)"
    )

    # Configuration arguments
    parser.add_argument(
        "--model", default="gpt-4o-mini", help="LLM model to use (default: gpt-4o-mini)"
    )
    parser.add_argument(
        "--extraction-concurrency", type=int, help="Concurrency for extraction stage"
    )
    parser.add_argument(
        "--resolution-concurrency", type=int, help="Concurrency for resolution stage"
    )
    parser.add_argument(
        "--max-cluster-size",
        type=int,
        help="Maximum cluster size for community detection",
    )
    parser.add_argument(
        "--algorithm",
        choices=["louvain", "hierarchical_leiden"],
        help="Community detection algorithm (default: louvain)",
    )
    parser.add_argument(
        "--resolution", type=float, help="Louvain resolution parameter (0.5-2.0, default: 1.0)"
    )

    # Standard stage arguments (used by from_args_env)
    parser.add_argument("--db-name", help="Database name")
    parser.add_argument("--read-db-name", help="Read database name")
    parser.add_argument("--write-db-name", help="Write database name")
    parser.add_argument("--read-coll", help="Read collection name")
    parser.add_argument("--write-coll", help="Write collection name")
    parser.add_argument(
        "--concurrency",
        type=int,
        default=300,
        help="Concurrency level (default: 300 workers for optimal TPM utilization)",
    )
    parser.add_argument("--upsert-existing", action="store_true", help="Upsert existing documents")

    # Logging arguments
    parser.add_argument(
        "--verbose", action="store_true", help="Enable verbose logging (DEBUG level)"
    )
    parser.add_argument(
        "--log-file",
        help="Path to log file (default: logs/pipeline/graphrag_STAGE_TIMESTAMP.log)",
    )
    parser.add_argument("--quiet", action="store_true", help="Suppress output except errors")

    args = parser.parse_args()

    # Set up logging (with stage name for automatic log file naming)
    if args.quiet:
        logging.basicConfig(level=logging.ERROR)
    else:
        stage_name = args.stage if args.stage else "full_pipeline"
        setup_logging(args.verbose, getattr(args, "log_file", None), stage=stage_name)

    logger = logging.getLogger(__name__)

    # Validate arguments
    if args.stage and args.status:
        logger.error("Cannot specify both --stage and --status")
        sys.exit(1)

    if args.stage and args.cleanup:
        logger.error("Cannot specify both --stage and --cleanup")
        sys.exit(1)

    if args.status and args.cleanup:
        logger.error("Cannot specify both --status and --cleanup")
        sys.exit(1)

    try:
        # Create pipeline configuration
        config = create_config_from_args(args)

        # Create pipeline
        pipeline = create_graphrag_pipeline(config)

        # Execute based on arguments
        if args.status:
            show_pipeline_status(pipeline)
        elif args.cleanup:
            cleanup_failed_stages(pipeline)
        elif hasattr(args, "stages") and args.stages:
            # Achievement 0.1: Stage Selection & Partial Runs
            logger.info(f"🎯 Running selected stages: {args.stages}")
            result = pipeline.run_stages(args.stages)
            if result == 0:
                logger.info("✅ Selected stages completed successfully")
            else:
                logger.error(f"❌ Selected stages failed with exit code {result}")
            sys.exit(result)
        elif hasattr(args, "resume") and args.resume:
            # Achievement 0.2: Resume from Failure
            logger.info("🔄 Resume mode enabled")
            result = pipeline.run_full_pipeline(resume=True)
            if result == 0:
                logger.info("✅ Pipeline resumed and completed successfully")
            else:
                logger.error(f"❌ Pipeline resume failed with exit code {result}")
            sys.exit(result)
        elif args.stage:
            run_single_stage(pipeline, args.stage)
        else:
            run_full_pipeline(pipeline)

    except KeyboardInterrupt:
        logger.info("Pipeline execution interrupted by user")
        sys.exit(1)
    except Exception as e:
        log_exception(logger, "Unexpected error", e)
        sys.exit(1)


if __name__ == "__main__":
    main()
