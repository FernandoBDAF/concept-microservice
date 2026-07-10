import os
import time
import argparse
import logging
import threading
from typing import Any, Dict, Iterable, Optional, List, Callable

from dotenv import load_dotenv
from pymongo import MongoClient
from concurrent.futures import ThreadPoolExecutor, as_completed

from src.lib.error_handling.decorators import handle_errors
from src.lib.error_handling.context import stage_context
from src.lib.logging import (
    log_operation_context,
    log_operation_complete,
    log_exception,
)
from src.lib.metrics import Counter, Histogram, Timer, MetricRegistry
from src.lib.rate_limiting import RateLimiter

try:
    from src.infrastructure.database.mongodb import get_mongo_client
except ModuleNotFoundError:
    import sys as _sys, os as _os

    _sys.path.append(_os.path.abspath(_os.path.join(_os.path.dirname(__file__), "..")))
    from src.infrastructure.database.mongodb import get_mongo_client

from src.core.config.paths import DB_NAME as DEFAULT_DB
from src.core.models.config import BaseStageConfig


# Initialize stage metrics (shared across all stages)
_stage_started = Counter(
    "stage_started", "Number of stage executions started", labels=["stage"]
)
_stage_completed = Counter(
    "stage_completed", "Number of stage executions completed", labels=["stage"]
)
_stage_failed = Counter(
    "stage_failed", "Number of stage executions failed", labels=["stage"]
)
_stage_duration = Histogram(
    "stage_duration_seconds", "Stage execution duration", labels=["stage"]
)
_documents_processed = Counter(
    "documents_processed", "Documents processed by stage", labels=["stage"]
)
_documents_failed = Counter(
    "documents_failed", "Documents failed in stage", labels=["stage"]
)

# Register metrics
_registry = MetricRegistry.get_instance()
_registry.register(_stage_started)
_registry.register(_stage_completed)
_registry.register(_stage_failed)
_registry.register(_stage_duration)
_registry.register(_documents_processed)
_registry.register(_documents_failed)


class BaseStage:
    name = "base"
    description = ""
    ConfigCls = BaseStageConfig

    def __init__(self, logger: Optional[logging.Logger] = None) -> None:
        self.args: argparse.Namespace = argparse.Namespace()
        self.config: BaseStageConfig = self.ConfigCls()
        self.client: MongoClient = None  # type: ignore
        self.db = None
        self.db_read = None
        self.db_write = None
        self.logger = logger or logging.getLogger(self.name)
        self.start_ts = time.time()
        self.stats = {"processed": 0, "skipped": 0, "failed": 0, "updated": 0}

    # NOTE: Stages should NOT be called directly - they are components called by pipelines
    # These argument parsing methods exist for backward compatibility only
    # TODO: Remove in future version when all stages are called exclusively via pipelines
    def build_parser(self, p: argparse.ArgumentParser) -> None:
        """DEPRECATED: Stages should be called by pipelines, not directly."""
        p.add_argument("--max", type=int)
        p.add_argument("--llm", action="store_true")
        p.add_argument("--verbose", action="store_true")
        p.add_argument("--dry_run", action="store_true")
        p.add_argument(
            "--db_name",
            help="Override DB name (defaults to config.paths.DB_NAME or $DB_NAME)",
        )
        p.add_argument("--upsert_existing", action="store_true")
        p.add_argument("--video_id", type=str)
        p.add_argument("--concurrency", type=int)

    def parse_args(self) -> None:
        """DEPRECATED: Stages should be called by pipelines, not directly."""
        p = argparse.ArgumentParser(description=self.description or self.name)
        self.build_parser(p)
        self.args = p.parse_args()

    def setup(self) -> None:
        load_dotenv()
        self.client = get_mongo_client()

        # Config fallbacks ARE necessary because config allows None values
        # from_args_env() may return None for read_db_name/write_db_name
        # These fallbacks ensure we always have valid database names
        default_db_name = self.config.db_name or DEFAULT_DB
        write_db_name = self.config.write_db_name or default_db_name
        read_db_name = self.config.read_db_name or default_db_name

        # Set up database handles
        self.db = self.client[default_db_name]  # Default (backward compatibility)
        self.db_write = self.client[write_db_name]  # Explicit write DB
        self.db_read = self.client[read_db_name]  # Explicit read DB

    def log(self, msg: str) -> None:
        self.logger.info(f"[{self.name}] {msg}")

    def env_bool(self, key: str, default: bool = False) -> bool:
        v = os.getenv(key, str(default)).strip().lower()
        return v in {"1", "true", "yes", "on"}

    def build_config_from_args_env(self) -> BaseStageConfig:
        return self.ConfigCls.from_args_env(self.args, dict(os.environ), DEFAULT_DB)

    # Convenience helper to fetch collection from the desired IO side
    def get_collection(
        self, name: str, io: str = "read", db_name: Optional[str] = None
    ):
        """Return a collection handle with automatic constant DB routing.

        Constant collections (raw_videos, observability data) are automatically
        routed to the system database regardless of the pipeline database.

        Args:
            name: Collection name
            io: "read" or "write" - determines which DB handle to use
            db_name: Explicit database override (still subject to routing)

        Returns:
            Collection handle from the appropriate database
        """
        from src.core.config.paths import get_db_for_collection
        
        # Determine base target database
        if db_name:
            target_db = db_name
        elif io == "write":
            target_db = self.config.write_db_name or self.config.db_name or DEFAULT_DB
        else:
            target_db = self.config.read_db_name or self.config.db_name or DEFAULT_DB
        
        # Route constant collections to system database
        final_db = get_db_for_collection(name, target_db)
        return self.client[final_db][name]

    def iter_docs(self) -> Iterable[Dict[str, Any]]:
        raise NotImplementedError

    def handle_doc(self, doc: Dict[str, Any]) -> None:
        raise NotImplementedError

    def finalize(self) -> None:
        dur = time.time() - self.start_ts
        self.log(
            f"Summary: processed={self.stats['processed']} updated={self.stats['updated']} "
            f"skipped={self.stats['skipped']} failed={self.stats['failed']} in {dur:.1f}s"
        )

    # ============================================================================
    # Concurrent Processing Support (Template Methods)
    # ============================================================================

    def estimate_tokens(self, doc: Dict[str, Any]) -> int:
        """
        Estimate tokens for a document (template method - override in stages).

        Args:
            doc: Document to estimate tokens for

        Returns:
            Estimated token count
        """
        # Default: simple text-based estimation
        text = doc.get("chunk_text", "")
        return int(len(text) / 4) + 1000  # ~4 chars per token + output estimate

    def process_doc_with_tracking(self, doc: Dict[str, Any]) -> Any:
        """
        Process a document with TPM/RPM tracking (template method - override in stages).

        Default implementation calls handle_doc(). Stages can override to call
        agent methods or perform custom processing.

        Args:
            doc: Document to process

        Returns:
            Processing result (may be None)
        """
        return self.handle_doc(doc)

    def store_batch_results(
        self, batch_results: List[Any], batch_docs: List[Dict[str, Any]]
    ) -> None:
        """
        Store batch processing results (template method - override in stages).

        Default: no-op (results are handled by handle_doc internally).
        Stages that need to store results incrementally should override this.

        Args:
            batch_results: Results from processing batch_docs
            batch_docs: Original documents that were processed
        """
        pass

    def _setup_tpm_tracking(self, limiter_name: str = "default") -> tuple:
        """
        Setup TPM/RPM tracking infrastructure.

        Args:
            limiter_name: Name for the rate limiter (for logging)

        Returns:
            Tuple of (target_tpm, target_rpm, rpm_limiter, token_window, token_lock)
        """
        target_tpm = int(os.getenv("GRAPHRAG_TARGET_TPM", "950000"))
        target_rpm = int(os.getenv("GRAPHRAG_TARGET_RPM", "20000"))

        rpm_limiter = RateLimiter(rpm=target_rpm, name=f"{self.name}_{limiter_name}")
        token_window = []  # (timestamp, tokens) tuples
        token_lock = threading.Lock()

        return target_tpm, target_rpm, rpm_limiter, token_window, token_lock

    def _wait_for_tpm_capacity(
        self,
        estimated_tokens: int,
        target_tpm: int,
        token_window: List[tuple],
        token_lock: threading.Lock,
    ) -> None:
        """
        Wait until TPM capacity is available (optimized for throughput).

        Uses optimistic reservation: only blocks if we're over 120% of target.

        Args:
            estimated_tokens: Estimated tokens for this operation
            target_tpm: Target tokens per minute
            token_window: List of (timestamp, tokens) tuples
            token_lock: Thread lock for token_window
        """
        now = time.time()
        with token_lock:
            # Clean old events (60 second window)
            cutoff = now - 60
            token_window[:] = [(ts, tok) for ts, tok in token_window if ts > cutoff]

            # Current TPM
            current_tpm = sum(tok for _, tok in token_window)

            # Reserve immediately (optimistic)
            token_window.append((now, estimated_tokens))

            # Only wait if way over limit (> 120%)
            if current_tpm > target_tpm * 1.2:
                time.sleep(0.05)  # Minimal delay

    def _run_concurrent_with_tpm(
        self,
        docs: List[Dict[str, Any]],
        limiter_name: str = "default",
    ) -> int:
        """
        Run stage with concurrent processing and TPM tracking.

        This is the main orchestration method that stages can call from their run() method.
        It uses template methods (estimate_tokens, process_doc_with_tracking, store_batch_results)
        that stages can override for stage-specific behavior.

        Args:
            docs: List of documents to process
            limiter_name: Name for rate limiter (for logging)

        Returns:
            0 on success, 1 on failure
        """
        try:
            total = len(docs)
            if total == 0:
                return 0

            # Dynamic batch size based on workers (2x workers, max 1000 for safety)
            concurrency = int(self.config.concurrency or 300)
            batch_size = min(concurrency * 2, 1000)

            # Setup TPM tracking
            target_tpm, target_rpm, rpm_limiter, token_window, token_lock = (
                self._setup_tpm_tracking(limiter_name)
            )

            self.logger.info(
                f"[{self.name}] Advanced TPM mode: {total} documents "
                f"(workers={concurrency}, TPM={target_tpm:,}, RPM={target_rpm})"
            )

            overall_start = time.time()

            # Process in batches
            for batch_start in range(0, total, batch_size):
                batch_end = min(batch_start + batch_size, total)
                batch_docs = docs[batch_start:batch_end]
                batch_num = (batch_start // batch_size) + 1
                total_batches = (total + batch_size - 1) // batch_size

                self.logger.info(
                    f"[{self.name}] Batch {batch_num}/{total_batches}: "
                    f"Processing documents {batch_start+1}-{batch_end}"
                )

                batch_start_time = time.time()

                # Create worker function with tracking
                def process_with_tracking(doc):
                    """Process document with TPM/RPM tracking."""
                    try:
                        estimated = self.estimate_tokens(doc)
                        self._wait_for_tpm_capacity(
                            estimated, target_tpm, token_window, token_lock
                        )
                        rpm_limiter.wait()
                        result = self.process_doc_with_tracking(doc)

                        # Track actual tokens (approximate)
                        with token_lock:
                            token_window.append((time.time(), estimated))

                        return result
                    except Exception as e:
                        self.stats["failed"] += 1
                        self.logger.error(
                            f"[{self.name}] Error processing document: {e}"
                        )
                        return None

                # Process batch concurrently
                batch_results = []
                with ThreadPoolExecutor(max_workers=concurrency) as executor:
                    future_to_idx = {
                        executor.submit(process_with_tracking, doc): i
                        for i, doc in enumerate(batch_docs)
                    }

                    # Collect results in order
                    results_dict = {}
                    for future in as_completed(future_to_idx):
                        idx = future_to_idx[future]
                        try:
                            result = future.result()
                            results_dict[idx] = result
                        except Exception as e:
                            self.logger.error(
                                f"[{self.name}] Future error for document {idx}: {e}"
                            )
                            results_dict[idx] = None

                    # Order by original index
                    batch_results = [results_dict[i] for i in range(len(batch_docs))]

                # Store batch results (if stage needs it)
                self.store_batch_results(batch_results, batch_docs)

                batch_elapsed = time.time() - batch_start_time

                # Get current TPM
                with token_lock:
                    now = time.time()
                    cutoff = now - 60
                    token_window[:] = [
                        (ts, tok) for ts, tok in token_window if ts > cutoff
                    ]
                    current_tpm = sum(tok for _, tok in token_window)

                self.logger.info(
                    f"[{self.name}] Batch {batch_num} complete: "
                    f"{len(batch_docs)} documents in {batch_elapsed:.1f}s "
                    f"(current TPM: {current_tpm:,}, "
                    f"updated={self.stats['updated']}, failed={self.stats['failed']})"
                )

            overall_elapsed = time.time() - overall_start
            self.logger.info(
                f"[{self.name}] Complete: {total} documents in {overall_elapsed:.1f}s "
                f"({overall_elapsed/60:.1f} minutes)"
            )

            # NOTE: finalize() NOT called here - user will call it separately when ready
            return 0

        except Exception as e:
            self.logger.error(f"[{self.name}] Fatal error: {e}")
            return 1

    @handle_errors(log_traceback=True, capture_context=True, reraise=False)
    def run(self, config: Optional[BaseStageConfig] = None) -> int:
        """Run stage with comprehensive error handling and metrics.

        Automatically detects concurrency settings and uses appropriate execution method:
        - Concurrent + TPM tracking: _run_concurrent_with_tpm() (default)
        - Concurrent only: _run_concurrent() if implemented
        - Sequential: standard loop processing
        """
        if config is None:
            self.parse_args()
            self.config = self.build_config_from_args_env()
        else:
            self.config = config

        # Track metrics
        stage_labels = {"stage": self.name}
        _stage_started.inc(labels=stage_labels)

        # Log stage start
        log_operation_context(
            f"stage_{self.name}",
            stage=self.name,
            max_docs=self.config.max if self.config.max else "unlimited",
        )

        with Timer() as timer:
            try:
                self.setup()

                docs = list(self.iter_docs())
                if self.config.max is not None:
                    docs = docs[: int(self.config.max)]

                total_docs = len(docs)

                if total_docs == 0:
                    self.logger.info(f"[{self.name}] No documents to process")
                    return 0

                # Auto-detect execution mode
                use_concurrent = self.config.concurrency and self.config.concurrency > 1
                use_tpm_tracking = (
                    os.getenv("GRAPHRAG_USE_TPM_TRACKING", "true").lower() == "true"
                )

                if use_concurrent and use_tpm_tracking:
                    # Advanced TPM mode (default for GraphRAG stages)
                    self.logger.info(
                        f"[{self.name}] Using ADVANCED TPM TRACKING: "
                        f"{total_docs} documents with {self.config.concurrency} workers"
                    )
                    return self._run_concurrent_with_tpm(docs, limiter_name=self.name)

                elif use_concurrent:
                    # Basic concurrent mode
                    self.logger.info(
                        f"[{self.name}] Using CONCURRENT processing: "
                        f"{total_docs} documents with {self.config.concurrency} workers"
                    )
                    # Check if stage has custom _run_concurrent implementation
                    if hasattr(self, "_run_concurrent") and callable(
                        getattr(self, "_run_concurrent")
                    ):
                        return self._run_concurrent(docs)
                    else:
                        # Fallback to TPM tracking (works for all stages)
                        return self._run_concurrent_with_tpm(
                            docs, limiter_name=self.name
                        )

                # Sequential mode - original implementation below
                if total_docs > 0:
                    self.logger.info(
                        f"[{self.name}] Using SEQUENTIAL processing: {total_docs} document(s) "
                        f"(max={self.config.max if self.config.max else 'unlimited'})"
                    )

                with stage_context(self.name, total_docs=total_docs):
                    for i, d in enumerate(docs):
                        if self.config.max and i >= int(self.config.max):
                            break
                        try:
                            # Log progress for batches (every 10% or every 10 items, whichever is more frequent)
                            if (
                                total_docs > 10
                                and (i + 1) % max(1, total_docs // 10) == 0
                            ):
                                progress_pct = int((i + 1) / total_docs * 100)
                                self.logger.info(
                                    f"[{self.name}] Progress: {i + 1}/{total_docs} ({progress_pct}%) "
                                    f"processed={self.stats['processed']} "
                                    f"updated={self.stats['updated']} "
                                    f"skipped={self.stats['skipped']} "
                                    f"failed={self.stats['failed']}"
                                )
                            self.handle_doc(d)
                            self.stats["processed"] += 1
                            _documents_processed.inc(labels=stage_labels)
                        except Exception as e:
                            self.stats["failed"] += 1
                            _documents_failed.inc(labels=stage_labels)
                            log_exception(
                                self.logger,
                                f"[{self.name}] Error processing document",
                                e,
                            )

                    self.finalize()

                    # Track metrics
                    _stage_completed.inc(labels=stage_labels)
                    _stage_duration.observe(timer.elapsed(), labels=stage_labels)

                    # Log stage completion
                    duration = time.time() - self.start_ts
                    log_operation_complete(
                        f"stage_{self.name}",
                        duration=duration,
                        processed=self.stats["processed"],
                        updated=self.stats["updated"],
                        failed=self.stats["failed"],
                    )

                    return 0

            except Exception as e:
                # Fatal stage error - track failure
                _stage_failed.inc(labels=stage_labels)
                _stage_duration.observe(timer.elapsed(), labels=stage_labels)
                log_exception(self.logger, f"[{self.name}] Fatal error", e)
                return 1
