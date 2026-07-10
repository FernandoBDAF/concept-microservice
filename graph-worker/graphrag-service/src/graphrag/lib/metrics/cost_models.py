"""
LLM Cost Models.

Provides cost estimation for different LLM models.
Part of the CORE libraries - metrics library.
"""

from typing import Dict, Tuple


# LLM pricing (USD per 1M tokens)
# Format: model_name -> (input_price, output_price)
LLM_PRICING: Dict[str, Tuple[float, float]] = {
    # OpenAI models
    "gpt-4o-mini": (0.150, 0.600),
    "gpt-4o": (2.50, 10.00),
    "gpt-4": (30.00, 60.00),
    "gpt-3.5-turbo": (0.50, 1.50),
    "gpt-3.5-turbo-16k": (3.00, 4.00),
    # TODO: Add other providers when used
    # "claude-3-opus": (15.00, 75.00),
    # "claude-3-sonnet": (3.00, 15.00),
    # "claude-3-haiku": (0.25, 1.25),
}


def estimate_llm_cost(
    model_name: str, prompt_tokens: int, completion_tokens: int
) -> float:
    """Estimate LLM cost in USD.

    Args:
        model_name: Model name (e.g., "gpt-4o-mini")
        prompt_tokens: Number of input tokens
        completion_tokens: Number of output tokens

    Returns:
        Estimated cost in USD

    Example:
        cost = estimate_llm_cost("gpt-4o-mini", 1000, 500)
        # Returns: (1000/1M * 0.150) + (500/1M * 0.600) = 0.00045
    """
    # Find pricing (exact match or partial match)
    pricing = None

    # Try exact match first
    if model_name in LLM_PRICING:
        pricing = LLM_PRICING[model_name]
    else:
        # Try partial match (e.g., "gpt-4o-mini-2024" matches "gpt-4o-mini")
        for model_key, model_pricing in LLM_PRICING.items():
            if model_key in model_name:
                pricing = model_pricing
                break

    # If model not found, use conservative estimate (gpt-4o pricing)
    if pricing is None:
        pricing = (2.50, 10.00)  # Default to gpt-4o pricing

    input_price, output_price = pricing

    # Calculate cost
    input_cost = (prompt_tokens / 1_000_000) * input_price
    output_cost = (completion_tokens / 1_000_000) * output_price

    return input_cost + output_cost


def add_model_pricing(model_name: str, input_price: float, output_price: float) -> None:
    """Add custom model pricing.

    Args:
        model_name: Model identifier
        input_price: Price per 1M input tokens (USD)
        output_price: Price per 1M output tokens (USD)

    Example:
        add_model_pricing("custom-model", 1.00, 2.00)
    """
    LLM_PRICING[model_name] = (input_price, output_price)
