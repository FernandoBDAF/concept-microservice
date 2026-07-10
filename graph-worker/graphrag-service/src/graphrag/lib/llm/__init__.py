"""
LLM Operations Library - Cross-Cutting Concern.

Provides standardized LLM client initialization and call helpers.
Part of the CORE libraries - Tier 2 (foundational implementation).

Usage:
    from src.lib.llm import get_openai_client, call_llm_simple, call_llm_with_structured_output

    # Initialize client
    client = get_openai_client()

    # Simple call
    response = call_llm_simple(
        client,
        system_prompt="You are a helpful assistant.",
        user_prompt="What is Python?",
        model="gpt-4o-mini"
    )

    # Structured output
    from pydantic import BaseModel
    class EntityModel(BaseModel):
        name: str
        type: str

    result = call_llm_with_structured_output(
        client,
        system_prompt="Extract entities.",
        user_prompt="Text: Python is a language.",
        response_model=EntityModel
    )
"""

from src.lib.llm.client import (
    get_openai_client,
    is_openai_available,
)

from src.lib.llm.calls import (
    call_llm,
    call_llm_simple,
    call_llm_with_structured_output,
)

__all__ = [
    # Client
    "get_openai_client",
    "is_openai_available",
    # Calls
    "call_llm",
    "call_llm_simple",
    "call_llm_with_structured_output",
]
