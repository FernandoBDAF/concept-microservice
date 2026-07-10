"""
OpenAI Client Adapter.

This module provides a centralized OpenAI client with configuration management.
Part of the DEPENDENCIES layer - abstracts external LLM dependency.
"""

import os
from typing import Optional
from openai import OpenAI


class OpenAIClient:
    """Singleton OpenAI client wrapper."""

    _instance: Optional[OpenAI] = None

    @classmethod
    def get_instance(cls, api_key: Optional[str] = None, timeout: int = 60) -> OpenAI:
        """Get OpenAI client instance (singleton).

        Args:
            api_key: OpenAI API key. If None, reads from OPENAI_API_KEY env var.
            timeout: Request timeout in seconds (default: 60)

        Returns:
            OpenAI client instance

        Raises:
            RuntimeError: If OPENAI_API_KEY is not set
        """
        if cls._instance is None:
            key = api_key or os.getenv("OPENAI_API_KEY")
            if not key:
                raise RuntimeError(
                    "OPENAI_API_KEY environment variable is required. "
                    "Set it in your .env file or environment."
                )
            cls._instance = OpenAI(api_key=key, timeout=timeout)
        return cls._instance

    @classmethod
    def reset_instance(cls) -> None:
        """Reset singleton instance (useful for testing)."""
        cls._instance = None


# Backward compatibility wrapper
def get_openai_client(api_key: Optional[str] = None, timeout: int = 60) -> OpenAI:
    """Get OpenAI client instance.

    This function maintains backward compatibility with existing code.
    New code should use OpenAIClient.get_instance() directly.

    Args:
        api_key: OpenAI API key (optional)
        timeout: Request timeout (default: 60)

    Returns:
        OpenAI client instance
    """
    return OpenAIClient.get_instance(api_key, timeout)
