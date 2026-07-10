"""
Generic TPM-tracked concurrent processor.

Provides reusable concurrent processing with TPM/RPM tracking for any operation.
"""

import logging
import os
import time
import threading
from typing import List, Any, Callable, Tuple, Optional
from concurrent.futures import ThreadPoolExecutor, as_completed
from src.lib.rate_limiting import RateLimiter

logger = logging.getLogger(__name__)


def run_concurrent_with_tpm(
    items: List[Any],
    processor_fn: Callable[[Any], Any],
    estimate_tokens_fn: Callable[[Any], int],
    max_workers: int = 300,
    target_tpm: int = 950000,
    target_rpm: int = 20000,
    batch_size: Optional[int] = None,
    limiter_name: str = "default",
    progress_name: str = "items",
) -> List[Tuple[Any, Any]]:
    """
    Process items concurrently with TPM/RPM tracking.
    
    This is a generic utility that handles all TPM tracking, rate limiting,
    batching, and progress logging. Can be used by stages, agents, or any
    component that needs concurrent LLM processing.
    
    Args:
        items: List of items to process
        processor_fn: Function to process each item (can make LLM calls)
        estimate_tokens_fn: Function to estimate tokens for an item
        max_workers: Maximum concurrent workers
        target_tpm: Target tokens per minute
        target_rpm: Target requests per minute
        batch_size: Batch size (default: max_workers * 2, max 1000)
        limiter_name: Name for rate limiter (for logging)
        progress_name: Name for progress logging (e.g., "chunks", "communities")
    
    Returns:
        List of (item, result) tuples in original order
    """
    if not items:
        return []
    
    total = len(items)
    if batch_size is None:
        batch_size = min(max_workers * 2, 1000)
    
    logger.info(
        f"[{limiter_name}] Processing {total} {progress_name} with {max_workers} workers "
        f"(TPM={target_tpm:,}, RPM={target_rpm}, batch_size={batch_size})"
    )
    
    # Setup rate limiters
    rpm_limiter = RateLimiter(rpm=target_rpm, name=limiter_name)
    
    # TPM tracker
    token_window = []
    token_lock = threading.Lock()
    
    def wait_for_tpm_capacity(estimated_tokens: int) -> None:
        """Wait until TPM capacity available (optimized for throughput)."""
        with token_lock:
            now = time.time()
            cutoff = now - 60
            token_window[:] = [(ts, tok) for ts, tok in token_window if ts > cutoff]
            current_tpm = sum(tok for _, tok in token_window)
            
            # Reserve immediately (optimistic)
            token_window.append((now, estimated_tokens))
            
            # Only wait if way over limit (> 120%)
            if current_tpm > target_tpm * 1.2:
                time.sleep(0.05)
    
    def process_with_tracking(item: Any) -> Tuple[Any, Any]:
        """Process item with TPM/RPM tracking."""
        try:
            # Estimate and wait for capacity
            estimated = estimate_tokens_fn(item)
            wait_for_tpm_capacity(estimated)
            
            # Rate limit
            rpm_limiter.wait()
            
            # Process
            result = processor_fn(item)
            
            # Track actual tokens
            with token_lock:
                token_window.append((time.time(), estimated))
            
            return item, result
        
        except Exception as e:
            logger.error(f"[{limiter_name}] Error processing item: {e}")
            return item, None
    
    # Process in batches
    all_results = []
    overall_start = time.time()
    
    for batch_start in range(0, total, batch_size):
        batch_end = min(batch_start + batch_size, total)
        batch_items = items[batch_start:batch_end]
        batch_num = (batch_start // batch_size) + 1
        total_batches = (total + batch_size - 1) // batch_size
        
        logger.info(
            f"[{limiter_name}] Batch {batch_num}/{total_batches}: "
            f"Processing {progress_name} {batch_start+1}-{batch_end}"
        )
        
        batch_start_time = time.time()
        
        # Process batch concurrently
        with ThreadPoolExecutor(max_workers=max_workers) as executor:
            futures = {
                executor.submit(process_with_tracking, item): idx
                for idx, item in enumerate(batch_items)
            }
            
            # Collect results in order
            batch_results = [None] * len(batch_items)
            for future in as_completed(futures):
                idx = futures[future]
                item, result = future.result()
                batch_results[idx] = (item, result)
        
        all_results.extend(batch_results)
        
        batch_elapsed = time.time() - batch_start_time
        
        # Get current TPM
        with token_lock:
            now = time.time()
            cutoff = now - 60
            token_window[:] = [(ts, tok) for ts, tok in token_window if ts > cutoff]
            current_tpm = sum(tok for _, tok in token_window)
        
        successful = sum(1 for _, result in batch_results if result is not None)
        logger.info(
            f"[{limiter_name}] Batch {batch_num} complete in {batch_elapsed:.1f}s "
            f"({current_tpm/1000:.0f}k TPM, {successful}/{len(batch_items)} successful)"
        )
    
    overall_elapsed = time.time() - overall_start
    total_successful = sum(1 for _, result in all_results if result is not None)
    
    logger.info(
        f"[{limiter_name}] Complete: {total_successful}/{total} {progress_name} in {overall_elapsed:.1f}s "
        f"({overall_elapsed/60:.1f} minutes)"
    )
    
    return all_results

