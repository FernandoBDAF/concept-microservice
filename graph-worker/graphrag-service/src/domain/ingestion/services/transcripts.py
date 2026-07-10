import time
from typing import List, Dict, Optional

from langchain_community.document_loaders import YoutubeLoader
from src.lib.error_handling.decorators import handle_errors
from src.lib.metrics import Counter, Histogram, MetricRegistry

# Initialize ingestion service metrics
_ingestion_service_calls = Counter(
    "ingestion_service_calls", "Number of ingestion service calls", labels=["service", "method"]
)
_ingestion_service_errors = Counter(
    "ingestion_service_errors", "Number of ingestion service errors", labels=["service", "method"]
)
_ingestion_service_duration = Histogram(
    "ingestion_service_duration_seconds", "Ingestion service call duration", labels=["service", "method"]
)

# Register metrics
_registry = MetricRegistry.get_instance()
_registry.register(_ingestion_service_calls)
_registry.register(_ingestion_service_errors)
_registry.register(_ingestion_service_duration)


@handle_errors(fallback=[], log_traceback=True, reraise=False)
def get_transcript(
    video_url: str,
    languages: Optional[List[str]] = None,
    max_retries: int = 2,
) -> List[Dict]:
    """Fetch transcript via LangChain YoutubeLoader (transcript-only).

    Returns a list of {"text": str, "metadata": dict} items. Empty list if unavailable.
    """
    start_time = time.time()
    labels = {"service": "ingestion", "method": "get_transcript"}
    _ingestion_service_calls.inc(labels=labels)
    
    try:
        langs = languages or ["en", "en-US", "en-GB"]
        last_err: Optional[Exception] = None
        for _ in range(max_retries + 1):
            try:
                loader = YoutubeLoader.from_youtube_url(
                    video_url,
                    add_video_info=False,  # avoid pytube dependency/HTTP issues
                    language=langs,
                )
                docs = loader.load()
                print(f"transcript loaded for {video_url}")
                result = [
                    {"text": d.page_content or "", "metadata": d.metadata or {}}
                    for d in docs
                    if (d.page_content or "").strip()
                ]
                duration = time.time() - start_time
                _ingestion_service_duration.observe(duration, labels=labels)
                return result
            except Exception as e:  # pragma: no cover - best effort
                last_err = e
                print(f"error loading transcript for {video_url}: {e[:100]}")
                continue
        # Swallow error; upstream can decide how to handle missing transcript
        _ingestion_service_errors.inc(labels=labels)
        duration = time.time() - start_time
        _ingestion_service_duration.observe(duration, labels=labels)
        return []
    except Exception as e:
        _ingestion_service_errors.inc(labels=labels)
        duration = time.time() - start_time
        _ingestion_service_duration.observe(duration, labels=labels)
        return []


