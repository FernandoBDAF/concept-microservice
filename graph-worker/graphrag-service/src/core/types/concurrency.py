import time
import math
import random
from concurrent.futures import ThreadPoolExecutor
from typing import Any, Callable, Iterable, List, Optional, Sequence, Tuple


def run_concurrent_map(
    items: Sequence[Any],
    worker_fn: Callable[[Any], Any],
    max_workers: int = 4,
    preserve_order: bool = True,
    on_error: Optional[Callable[[Exception, Any], Any]] = None,
    desc: Optional[str] = None,
) -> List[Any]:
    """Run worker_fn over items concurrently.

    - preserve_order=True returns results in the same order as items
    - on_error(e, item) can return a fallback value per failure
    """
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
    return [r for _, r in results]


def _throttle(qps: Optional[float], last_ts: List[float]) -> None:
    if not qps or qps <= 0:
        return
    now = time.time()
    min_interval = 1.0 / qps
    elapsed = now - last_ts[0]
    if elapsed < min_interval:
        time.sleep(min_interval - elapsed)
    last_ts[0] = time.time()


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
    """Concurrent LLM calls with retries, optional QPS throttle, and ordering.

    agent_factory must return a fresh agent (thread-safe isolation).
    """

    def _worker(payload: Tuple[int, Any]) -> Tuple[int, Any]:
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
