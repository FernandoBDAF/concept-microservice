"""
Concurrency execution helpers for parallel and async operations.
"""

import time
import random
import logging
from concurrent.futures import ThreadPoolExecutor, as_completed
from typing import Any, Callable, Iterable, List, Optional, Sequence, Tuple, Dict

logger = logging.getLogger(__name__)


def run_concurrent_map(
    items: Sequence[Any],
    worker_fn: Callable[[Any], Any],
    max_workers: int = 4,
    preserve_order: bool = True,
    on_error: Optional[Callable[[Exception, Any], Any]] = None,
    desc: Optional[str] = None,
) -> List[Any]:
    """Run worker_fn over items concurrently.

    Args:
        items: Items to process
        worker_fn: Function to apply to each item
        max_workers: Maximum number of concurrent workers
        preserve_order: If True, returns results in same order as items
        on_error: Error handler that can return fallback value
        desc: Optional description for logging

    Returns:
        List of results in same order as input (if preserve_order=True)
    """
    if desc:
        logger.debug(f"Starting concurrent processing: {desc} ({len(items)} items)")

    results: List[Tuple[int, Any]] = []
    with ThreadPoolExecutor(max_workers=max(1, int(max_workers or 1))) as pool:
        futures = []
        for idx, it in enumerate(items):
            futures.append((idx, pool.submit(worker_fn, it)))

        for idx, fut in futures:
            try:
                res = fut.result()
            except Exception as e:
                if on_error is not None:
                    res = on_error(e, items[idx])
                else:
                    raise
            results.append((idx, res))

    if preserve_order:
        results.sort(key=lambda x: x[0])

    if desc:
        logger.debug(f"Completed concurrent processing: {desc}")

    return [r for _, r in results]


def run_llm_concurrent(
    chunks: Sequence[Any],
    agent_factory: Callable[[], Any],
    method_name: str,
    max_workers: int = 4,
    retries: int = 1,
    backoff_s: float = 0.5,
    qps: Optional[float] = None,
    jitter: bool = False,
    on_error: Optional[Callable[[Exception, Any], Any]] = None,
    preserve_order: bool = True,
) -> List[Any]:
    """Concurrent LLM calls with retries, QPS throttle, and ordering.

    Args:
        chunks: Items to process
        agent_factory: Factory function that returns a fresh agent (thread-safe)
        method_name: Method name to call on agent
        max_workers: Maximum number of concurrent workers
        retries: Number of retries per item
        backoff_s: Initial backoff in seconds
        qps: Optional queries per second limit
        jitter: Whether to add jitter to backoff
        on_error: Error handler for failed items
        preserve_order: If True, returns results in same order as input

    Returns:
        List of results

    Note:
        agent_factory must return a fresh agent for thread-safe isolation.
    """

    def _throttle(qps_limit: Optional[float], last_ts: List[float]) -> None:
        """Throttle based on QPS limit."""
        if not qps_limit or qps_limit <= 0:
            return
        now = time.time()
        min_interval = 1.0 / qps_limit
        elapsed = now - last_ts[0]
        if elapsed < min_interval:
            time.sleep(min_interval - elapsed)
        last_ts[0] = time.time()

    def _worker(payload: Tuple[int, Any]) -> Tuple[int, Any]:
        """Worker function with retry logic."""
        idx, chunk = payload
        agent = agent_factory()
        method = getattr(agent, method_name)
        last_ts = [0.0]
        attempt = 0

        while True:
            try:
                _throttle(qps, last_ts)
                return idx, method(chunk)
            except Exception as e:
                attempt += 1
                if attempt > max(0, retries):
                    if on_error:
                        return idx, on_error(e, chunk)
                    raise

                sleep_s = backoff_s * (2 ** (attempt - 1))
                if jitter:
                    sleep_s *= 0.75 + random.random() * 0.5
                time.sleep(sleep_s)

    items_with_index = list(enumerate(chunks))
    ordered = run_concurrent_map(
        items_with_index,
        _worker,
        max_workers=max_workers,
        preserve_order=True,
        on_error=None,
    )

    # ordered contains (idx, value)
    return [val for _, val in ordered]


def run_concurrent_with_limit(
    func: Callable,
    items: Sequence[Any],
    max_workers: int = 10,
    desc: Optional[str] = None,
) -> List[Any]:
    """Simple concurrent execution with worker limit.

    Args:
        func: Function to apply to each item
        items: Items to process
        max_workers: Maximum number of concurrent workers
        desc: Optional description for logging

    Returns:
        List of results
    """
    return run_concurrent_map(
        items=items,
        worker_fn=func,
        max_workers=max_workers,
        preserve_order=True,
        desc=desc,
    )
