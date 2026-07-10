"""
LLM Call Helpers.

Provides standardized LLM call patterns with retry and error handling.
Part of the CORE libraries - LLM library.
"""

import logging
from typing import Any, Dict, List, Optional, Type, TypeVar
from openai import OpenAI
from pydantic import BaseModel

from src.lib.retry.decorators import retry_llm_call

logger = logging.getLogger(__name__)

T = TypeVar("T", bound=BaseModel)


def call_llm(
    client: OpenAI,
    messages: List[Dict[str, str]],
    model: str = "gpt-4o-mini",
    temperature: float = 0.1,
    max_tokens: Optional[int] = None,
    response_format: Optional[Type[BaseModel]] = None,
    max_attempts: int = 3,
) -> Any:
    """Make an LLM call with automatic retry and error handling.

    Args:
        client: OpenAI client instance
        messages: List of message dicts with 'role' and 'content'
        model: Model name (default: "gpt-4o-mini")
        temperature: Temperature for generation (default: 0.1)
        max_tokens: Maximum tokens for response
        response_format: Optional Pydantic model for structured output
        max_attempts: Maximum retry attempts (default: 3)

    Returns:
        Response content (string or parsed Pydantic model)

    Example:
        client = get_openai_client()
        response = call_llm(
            client,
            messages=[
                {"role": "system", "content": "You are a helpful assistant."},
                {"role": "user", "content": "Extract entities from: Python is a language."}
            ],
            model="gpt-4o-mini",
            temperature=0.1
        )
    """
    @retry_llm_call(max_attempts=max_attempts)
    def _call():
        if response_format:
            # Use structured output (beta API)
            response = client.beta.chat.completions.parse(
                model=model,
                messages=messages,
                response_format=response_format,
                temperature=temperature,
                max_tokens=max_tokens,
            )
            return response.choices[0].message.parsed
        else:
            # Standard chat completion
            response = client.chat.completions.create(
                model=model,
                messages=messages,
                temperature=temperature,
                max_tokens=max_tokens,
            )
            content = response.choices[0].message.content
            return content.strip() if content else ""

    return _call()


def call_llm_with_structured_output(
    client: OpenAI,
    system_prompt: str,
    user_prompt: str,
    response_model: Type[T],
    model: str = "gpt-4o-mini",
    temperature: float = 0.1,
    max_tokens: Optional[int] = None,
    max_attempts: int = 3,
) -> T:
    """Make an LLM call with structured output (Pydantic model).

    Args:
        client: OpenAI client instance
        system_prompt: System prompt
        user_prompt: User prompt
        response_model: Pydantic model class for structured output
        model: Model name (default: "gpt-4o-mini")
        temperature: Temperature for generation (default: 0.1)
        max_tokens: Maximum tokens for response
        max_attempts: Maximum retry attempts (default: 3)

    Returns:
        Parsed Pydantic model instance

    Example:
        from pydantic import BaseModel

        class EntityModel(BaseModel):
            name: str
            type: str

        client = get_openai_client()
        result = call_llm_with_structured_output(
            client,
            system_prompt="Extract entities from text.",
            user_prompt="Text: Python is a programming language.",
            response_model=EntityModel
        )
    """
    messages = [
        {"role": "system", "content": system_prompt},
        {"role": "user", "content": user_prompt},
    ]

    return call_llm(
        client=client,
        messages=messages,
        model=model,
        temperature=temperature,
        max_tokens=max_tokens,
        response_format=response_model,
        max_attempts=max_attempts,
    )


def call_llm_simple(
    client: OpenAI,
    system_prompt: str,
    user_prompt: str,
    model: str = "gpt-4o-mini",
    temperature: float = 0.1,
    max_tokens: Optional[int] = None,
    max_attempts: int = 3,
) -> str:
    """Make a simple LLM call with system and user prompts.

    Args:
        client: OpenAI client instance
        system_prompt: System prompt
        user_prompt: User prompt
        model: Model name (default: "gpt-4o-mini")
        temperature: Temperature for generation (default: 0.1)
        max_tokens: Maximum tokens for response
        max_attempts: Maximum retry attempts (default: 3)

    Returns:
        Response text

    Example:
        client = get_openai_client()
        answer = call_llm_simple(
            client,
            system_prompt="You are a helpful assistant.",
            user_prompt="What is Python?",
            model="gpt-4o-mini"
        )
    """
    messages = [
        {"role": "system", "content": system_prompt},
        {"role": "user", "content": user_prompt},
    ]

    return call_llm(
        client=client,
        messages=messages,
        model=model,
        temperature=temperature,
        max_tokens=max_tokens,
        max_attempts=max_attempts,
    )

