"""
Concurrency Library - Cross-Cutting Concern.

Provides parallel execution, thread pool management, and async helpers.
Part of the CORE libraries - Tier 2.

Usage:
    from src.lib.concurrency import run_concurrent_with_limit, run_concurrent_map

    # Simple concurrent execution
    results = run_concurrent_with_limit(
        func=process_item,
        items=items,
        max_workers=10
    )

    # Advanced concurrent with error handling
    results = run_concurrent_map(
        items=items,
        worker_fn=process_item,
        max_workers=10,
        preserve_order=True,
        on_error=lambda e, item: None,  # Fallback value
        desc="Processing items"
    )

    # LLM concurrent calls with retry and throttling
    results = run_llm_concurrent(
        chunks=chunks,
        agent_factory=lambda: MyAgent(),
        method_name='process',
        max_workers=5,
        retries=3,
        qps=10  # 10 queries per second max
    )
"""

from src.lib.concurrency.executor import (
    run_concurrent_map,
    run_concurrent_with_limit,
    run_llm_concurrent,
)
from src.lib.concurrency.tpm_processor import run_concurrent_with_tpm

__all__ = [
    "run_concurrent_map",
    "run_concurrent_with_limit",
    "run_llm_concurrent",
    "run_concurrent_with_tpm",
]
