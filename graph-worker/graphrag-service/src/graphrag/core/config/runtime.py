# Runtime configuration defaults

# Default maximum retries for LLM-based agents (per the plan)
MAX_RETRIES = 4

# Reading speed baseline (words per minute) for reading time estimates
WORDS_PER_MINUTE = 200.0

# RAG re-ranking weights (can be overridden by env)
import os


def _getf(name: str, default: float) -> float:
    try:
        return float(os.getenv(name, default))
    except Exception:
        return default


RAG_WEIGHT_VECTOR = _getf("RAG_WEIGHT_VECTOR", 0.6)
RAG_WEIGHT_TRUST = _getf("RAG_WEIGHT_TRUST", 0.25)
RAG_WEIGHT_RECENCY = _getf("RAG_WEIGHT_RECENCY", 0.15)
