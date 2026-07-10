import os
import uuid
import json
import logging
from datetime import datetime
from abc import ABC, abstractmethod
from typing import Any, Dict, Optional

from pydantic import BaseModel, Field
import openai

try:
    import boto3
except Exception:  # pragma: no cover - bedrock is optional
    boto3 = None  # type: ignore[assignment]

from src.lib.error_handling.decorators import handle_errors
from src.lib.error_handling.context import agent_context
from src.lib.error_handling.exceptions import format_exception_message
from src.lib.logging import log_exception
from src.lib.metrics import Counter, Histogram, MetricRegistry
from src.lib.retry import retry_llm_call
from typing import Tuple

logger = logging.getLogger(__name__)

# Initialize agent metrics (shared across all agents)
_agent_llm_calls = Counter(
    "agent_llm_calls", "Number of LLM calls", labels=["agent", "model"]
)
_agent_llm_errors = Counter(
    "agent_llm_errors", "Number of LLM errors", labels=["agent", "model"]
)
_agent_llm_duration = Histogram(
    "agent_llm_duration_seconds", "LLM call duration", labels=["agent", "model"]
)
_agent_tokens_used = Counter(
    "agent_tokens_used", "Total tokens used", labels=["agent", "model", "token_type"]
)
_agent_llm_cost = Counter(
    "agent_llm_cost_usd", "Estimated LLM cost in USD", labels=["agent", "model"]
)

# Register metrics
_registry = MetricRegistry.get_instance()
_registry.register(_agent_llm_calls)
_registry.register(_agent_llm_errors)
_registry.register(_agent_llm_duration)
_registry.register(_agent_tokens_used)
_registry.register(_agent_llm_cost)


class BaseAgentConfig(BaseModel):
    """
    Strongly typed configuration object for Agents with validation.
    """

    model_name: Optional[str] = Field(default=None, description="Name of the LLM model")
    temperature: float = Field(
        default=0, ge=0, le=1, description="Sampling temperature"
    )
    max_tokens: int = Field(
        default=8000, gt=0, description="Maximum tokens to generate"
    )
    log_level: str = Field(default="INFO", description="Logging verbosity")
    output_dir: Optional[str] = Field(
        default=None, description="Directory for saving outputs"
    )
    input_dir: Optional[str] = Field(
        default=None, description="Directory for reading inputs"
    )
    extra: Dict = Field(
        default_factory=dict, description="Custom agent-specific config"
    )


class BaseAgent(ABC):
    def __init__(
        self,
        name: str,
        config: Optional[BaseAgentConfig] = None,
        prompt_id: Optional[str] = None
    ):
        self.id = str(uuid.uuid4())
        self.name = name
        self.config = config
        self.prompt_id = prompt_id  # For dynamic prompt selection
        self._prompt_registry = None  # Lazy-loaded

        if config is None:
            self.config = BaseAgentConfig()

        # Select backend: Bedrock if configured, otherwise OpenAI
        bedrock_model = os.getenv("BEDROCK_MODEL_ID")
        if bedrock_model:
            if boto3 is None:
                raise RuntimeError(
                    "boto3 is required for Bedrock. Install boto3 or unset BEDROCK_MODEL_ID."
                )
            # Guard against empty AWS_PROFILE values that break boto3 profile resolution.
            if os.getenv("AWS_PROFILE") is not None and os.getenv("AWS_PROFILE").strip() == "":
                os.environ.pop("AWS_PROFILE", None)
            region = os.getenv("AWS_REGION") or os.getenv("AWS_DEFAULT_REGION")
            if not region:
                raise RuntimeError("AWS_REGION is required for Bedrock.")
            session_kwargs = {}
            if os.getenv("AWS_ACCESS_KEY_ID") and os.getenv("AWS_SECRET_ACCESS_KEY"):
                session_kwargs.update(
                    aws_access_key_id=os.getenv("AWS_ACCESS_KEY_ID"),
                    aws_secret_access_key=os.getenv("AWS_SECRET_ACCESS_KEY"),
                )
                if os.getenv("AWS_SESSION_TOKEN"):
                    session_kwargs["aws_session_token"] = os.getenv("AWS_SESSION_TOKEN")
            bedrock_client = boto3.client("bedrock-runtime", region_name=region, **session_kwargs)
            self.config.model_name = bedrock_model
            self._llm_backend = "bedrock"
            self._bedrock_client = bedrock_client
        else:
            # Load model: explicit > env var > safe default
            self.config.model_name = (
                self.config.model_name
                or os.getenv("OPENAI_DEFAULT_MODEL")
                or "gpt-5-nano"
            )

            api_key = os.getenv("OPENAI_API_KEY")
            if not api_key:
                raise RuntimeError(
                    "OPENAI_API_KEY is required for LLM agents. Set it or run without --llm."
                )
            self._llm_backend = "openai"
            # Use a sane default timeout to avoid hanging calls
            self.model = openai.OpenAI(api_key=api_key, timeout=60)

        self.timestamp = datetime.utcnow().isoformat()

    def run(self, prompt: str) -> str:
        """Default: call model with prompt."""
        result = self.call_model(prompt)
        return result

    # ---- Prompt Management ----
    def get_prompts(self, **kwargs) -> Tuple[str, str]:
        """
        Get prompts for this agent with fallback chain.
        
        Lookup order:
        1. Try registry with specific prompt_id (if provided)
        2. Try registry default for this agent type
        3. Fall back to hardcoded build_prompts()
        
        Args:
            **kwargs: Variables to pass to prompt templates
            
        Returns:
            Tuple of (system_prompt, user_prompt)
        """
        # Lazy-load the prompt registry
        if self._prompt_registry is None:
            try:
                from src.lib.prompts import get_prompt_registry
                self._prompt_registry = get_prompt_registry()
            except Exception as e:
                logger.debug(f"Could not load prompt registry: {e}")
                self._prompt_registry = None
        
        # Try registry lookup
        if self._prompt_registry is not None:
            agent_type = self.__class__.__name__
            registry_prompts = self._prompt_registry.get_prompt(
                agent_type,
                self.prompt_id
            )
            
            if registry_prompts:
                system_prompt, user_template = registry_prompts
                # Render the user template with provided kwargs
                user_prompt = self._render_template(user_template, **kwargs)
                logger.debug(
                    f"[{self.name}] Using registry prompt: "
                    f"prompt_id={self.prompt_id or 'default'}"
                )
                return system_prompt, user_prompt
        
        # Fall back to hardcoded prompts
        logger.debug(f"[{self.name}] Using hardcoded prompts (fallback)")
        return self.build_prompts(**kwargs)
    
    def build_prompts(self, **kwargs) -> Tuple[str, str]:
        """
        Build hardcoded prompts for this agent.
        
        Subclasses should override this method to provide default prompts.
        This serves as the fallback when registry prompts are not available.
        
        Args:
            **kwargs: Variables for prompt construction
            
        Returns:
            Tuple of (system_prompt, user_prompt)
        """
        # Default implementation - subclasses should override
        return (
            "You are a helpful assistant.",
            str(kwargs.get("input", ""))
        )
    
    def _render_template(self, template: str, **kwargs) -> str:
        """
        Render a prompt template with provided variables.
        
        Supports Python format string syntax: {variable_name}
        
        Args:
            template: Template string with {placeholders}
            **kwargs: Variables to substitute
            
        Returns:
            Rendered template string
        """
        try:
            # Use safe formatting that handles missing keys gracefully
            import re
            
            def replace_placeholder(match):
                key = match.group(1)
                if key in kwargs:
                    return str(kwargs[key])
                # Keep original placeholder if not found
                logger.warning(f"Template variable '{key}' not provided")
                return match.group(0)
            
            # Match {variable_name} patterns
            pattern = r'\{(\w+)\}'
            rendered = re.sub(pattern, replace_placeholder, template)
            return rendered
            
        except Exception as e:
            logger.warning(f"Template rendering failed: {e}, using raw template")
            return template

    @retry_llm_call(max_attempts=3)
    def call_model(self, system_prompt: str, prompt: str, **kwargs) -> str:
        """Unified model call with automatic retry, metrics, and error tracking."""
        from src.lib.metrics import Timer

        model_name = self.config.model_name
        agent_labels = {"agent": self.name, "model": model_name}
        _agent_llm_calls.inc(labels=agent_labels)

        self._log_event(
            {
                "type": "model_call:start",
                "model": self.config.model_name,
                "temperature": kwargs.get("temperature", self.config.temperature),
                "max_tokens": kwargs.get("max_tokens", self.config.max_tokens),
                "system_chars": len(system_prompt or ""),
                "user_chars": len(prompt or ""),
            }
        )

        temperature = kwargs.get("temperature", self.config.temperature)
        max_completion_tokens = kwargs.get("max_tokens", self.config.max_tokens)
        timeout = kwargs.get("timeout", 60)

        # Bedrock path
        if getattr(self, "_llm_backend", "") == "bedrock":
            body = {
                # Anthropic Bedrock format: system is a plain string, messages use typed content.
                "system": system_prompt,
                "messages": [
                    {"role": "user", "content": [{"type": "text", "text": prompt}]}
                ],
                "max_tokens": max_completion_tokens,
                "temperature": temperature,
                "anthropic_version": "bedrock-2023-05-31",
            }
            from time import monotonic

            with Timer() as timer:
                try:
                    response = self._bedrock_client.invoke_model(
                        modelId=self.config.model_name,
                        contentType="application/json",
                        accept="application/json",
                        body=json.dumps(body),
                    )
                    payload = json.loads(response["body"].read())
                    content = payload.get("content") or []
                    text = "".join(
                        item.get("text", "") for item in content if isinstance(item, dict)
                    )
                    out = text.strip()

                    _agent_llm_duration.observe(timer.elapsed(), labels=agent_labels)

                    self._log_event(
                        {
                            "type": "model_call:done",
                            "model": self.config.model_name,
                            "ok": True,
                            "output_preview": out[:120],
                            "usage": None,
                        }
                    )
                    return out
                except Exception as e:
                    _agent_llm_errors.inc(labels=agent_labels)
                    _agent_llm_duration.observe(timer.elapsed(), labels=agent_labels)
                    self._log_event(
                        {
                            "type": "model_call:done",
                            "model": self.config.model_name,
                            "ok": False,
                            "error": format_exception_message(e),
                        }
                    )
                    raise

        # OpenAI path
        if hasattr(getattr(self, "model", None), "chat"):
            with Timer() as timer:
                try:
                    response = self.model.chat.completions.create(
                        model=self.config.model_name,
                        messages=[
                            {"role": "system", "content": system_prompt},
                            {"role": "user", "content": prompt},
                        ],
                        max_completion_tokens=max_completion_tokens,
                        timeout=timeout,
                        **kwargs,
                    )
                    out = response.choices[0].message.content.strip()
                    usage = getattr(response, "usage", None)

                    # Track successful LLM call duration
                    _agent_llm_duration.observe(timer.elapsed(), labels=agent_labels)

                    # Track token usage and cost
                    if usage:
                        from src.lib.metrics.cost_models import estimate_llm_cost

                        prompt_tokens = getattr(usage, "prompt_tokens", 0) or 0
                        completion_tokens = getattr(usage, "completion_tokens", 0) or 0
                        total_tokens = prompt_tokens + completion_tokens

                        # Track tokens by type (prompt/completion) and total
                        _agent_tokens_used.inc(
                            amount=prompt_tokens,
                            labels={**agent_labels, "token_type": "prompt"},
                        )
                        _agent_tokens_used.inc(
                            amount=completion_tokens,
                            labels={**agent_labels, "token_type": "completion"},
                        )
                        _agent_tokens_used.inc(
                            amount=total_tokens,
                            labels={**agent_labels, "token_type": "total"},
                        )

                        # Estimate cost using robust cost model
                        cost = estimate_llm_cost(
                            model_name, prompt_tokens, completion_tokens
                        )
                        _agent_llm_cost.inc(amount=cost, labels=agent_labels)

                    self._log_event(
                        {
                            "type": "model_call:done",
                            "model": self.config.model_name,
                            "ok": True,
                            "output_preview": out[:120],
                            "usage": {
                                "prompt_tokens": (
                                    getattr(usage, "prompt_tokens", None)
                                    if usage
                                    else None
                                ),
                                "completion_tokens": (
                                    getattr(usage, "completion_tokens", None)
                                    if usage
                                    else None
                                ),
                                "total_tokens": (
                                    getattr(usage, "total_tokens", None)
                                    if usage
                                    else None
                                ),
                            },
                        }
                    )
                    return out
                except Exception as e:
                    # Track LLM error
                    _agent_llm_errors.inc(labels=agent_labels)
                    _agent_llm_duration.observe(timer.elapsed(), labels=agent_labels)

                    # Enhanced error logging using library helper
                    error_formatted = format_exception_message(e)
                    self._log_event(
                        {
                            "type": "model_call:error",
                            "model": self.config.model_name,
                            "error": error_formatted,
                        }
                    )
                    # Log with full traceback
                    log_exception(logger, f"[{self.name}] LLM call failed", e)
                    return ""

        elif callable(self.model):
            return self.model(prompt, **kwargs)

        elif isinstance(self.model, str):
            raise RuntimeError(
                f"Model '{self.model}' is just a string. "
                "Provide a proper client instance or wrap it."
            )

        else:
            raise RuntimeError(f"Unsupported model type for {self.name}")

    # ---- Logging ----
    def log(self, prompt: str, output: str):
        """Log agent interaction (called explicitly, not automatically)."""
        log_entry = {
            "agent_id": self.id,
            "agent_name": self.name,
            "timestamp": self.timestamp,
            "prompt_preview": prompt[:100],
            "output_preview": output[:100],
        }
        # Only log if explicitly called (not auto-logged)
        logger.debug(
            f"[{self.name}] Agent interaction: {json.dumps(log_entry, ensure_ascii=False)}"
        )

    def _log_event(self, event: Dict[str, Any]):
        """Internal event logging - uses DEBUG level to avoid terminal flooding."""
        payload = {"agent": self.name, "ts": self.timestamp}
        payload.update(event)

        # Use DEBUG level for detailed events (model_call:start/done)
        # Only log errors at INFO/WARNING level
        event_type = event.get("type", "unknown")
        if "error" in event_type.lower() or event_type == "model_call:error":
            logger.warning(
                f"[{self.name}] {event_type}: {payload.get('error', 'unknown error')}"
            )
        else:
            # Model call start/done, retry attempts - all at DEBUG level
            logger.debug(
                f"[{self.name}] {event_type}: {json.dumps(payload, ensure_ascii=False)}"
            )

    # ---- Generic retry-with-feedback executor ----
    def execute_with_retries(self, max_retries: int, step_fn):
        """
        Generic retry loop that standardizes feedback accumulation and approval flow.

        step_fn(feedback: Optional[str]) -> dict with keys:
          - status: "APPROVE" | "RETURN_FOR_IMPROVEMENT"
          - reasons: list[str]
          - data: any (present when approved)

        Returns the last step_fn result (approved or not).
        """
        feedback = None
        last_result = {"status": "RETURN_FOR_IMPROVEMENT", "reasons": ["not started"]}
        for i in range(1, max_retries + 1):
            self._log_event({"type": "retry:attempt", "attempt": i})
            result = step_fn(feedback)
            last_result = result
            if result.get("status") == "APPROVE":
                self._log_event({"type": "retry:approved", "attempts": i})
                return result
            reasons = result.get("reasons", [])
            feedback = "Reviewer feedback:\n" + "\n".join(f"- {r}" for r in reasons)
            self._log_event(
                {"type": "retry:feedback", "attempt": i, "reasons": reasons}
            )
        return last_result


# Note: do not re-export via relative imports; import BaseAgent directly from this module.
