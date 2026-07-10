"""
LLM Client Helpers.

Provides standardized LLM client initialization and configuration.
Part of the CORE libraries - LLM library.
"""

import os
import logging
from typing import Optional
from openai import OpenAI

logger = logging.getLogger(__name__)


def get_openai_client(
    api_key: Optional[str] = None,
    timeout: int = 60,
    max_retries: int = 3,
) -> OpenAI:
    """Get an initialized OpenAI client with standard configuration.

    Args:
        api_key: OpenAI API key (defaults to OPENAI_API_KEY env var)
        timeout: Request timeout in seconds (default: 60)
        max_retries: Maximum number of retries (default: 3)

    Returns:
        Initialized OpenAI client instance

    Raises:
        RuntimeError: If API key is not provided and not found in environment

    Example:
        client = get_openai_client()
        response = client.chat.completions.create(...)
    """
    if api_key is None:
        api_key = os.getenv("OPENAI_API_KEY")

    if not api_key:
        raise RuntimeError(
            "OPENAI_API_KEY is required. Set it in environment or pass as argument."
        )

    client = OpenAI(
        api_key=api_key,
        timeout=timeout,
        max_retries=max_retries,
    )

    logger.debug(f"Initialized OpenAI client (timeout={timeout}, max_retries={max_retries})")

    return client


def is_openai_available() -> bool:
    """Check if OpenAI API key is available.

    Returns:
        True if OPENAI_API_KEY is set, False otherwise

    Example:
        if is_openai_available():
            client = get_openai_client()
        else:
            logger.warning("OpenAI not available")
    """
    return bool(os.getenv("OPENAI_API_KEY"))

