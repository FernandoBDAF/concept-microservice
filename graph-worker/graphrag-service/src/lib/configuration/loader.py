"""
Configuration loading with priority: args > env > defaults.
"""

import os
import logging
from typing import Any, Dict, Optional, Type, TypeVar
from dataclasses import fields, is_dataclass, MISSING
from pydantic import BaseModel

logger = logging.getLogger(__name__)

T = TypeVar("T")


def load_config(
    config_class: Type[T],
    args: Optional[Dict[str, Any]] = None,
    env_prefix: str = "",
    defaults: Optional[Dict[str, Any]] = None,
) -> T:
    """Load configuration with priority: args > env > defaults.

    Args:
        config_class: Pydantic model or dataclass to instantiate
        args: Arguments dict (highest priority)
        env_prefix: Prefix for environment variables (e.g., "GRAPHRAG_")
        defaults: Default values (lowest priority)

    Returns:
        Instance of config_class

    Priority (highest to lowest):
        1. args dict
        2. Environment variables (with prefix)
        3. defaults dict
        4. config_class defaults

    Example:
        @dataclass
        class MyConfig:
            db_name: str = "default"
            max_workers: int = 10

        config = load_config(
            MyConfig,
            args={'max_workers': 20},
            env_prefix='MYAPP_',
            defaults={'db_name': 'production'}
        )
        # Priority: args['max_workers']=20 > env > defaults['db_name']='production'
    """
    args = args or {}
    defaults = defaults or {}

    # Collect field names and types
    if is_dataclass(config_class):
        field_info = {f.name: f for f in fields(config_class)}
    elif issubclass(config_class, BaseModel):
        field_info = {name: field for name, field in config_class.model_fields.items()}
    else:
        raise TypeError(
            f"config_class must be a dataclass or Pydantic model, got {type(config_class)}"
        )

    # Build config dict with priority
    config_dict = {}

    for field_name in field_info.keys():
        value = None

        # Priority 1: args dict
        if field_name in args:
            value = args[field_name]
            source = "args"

        # Priority 2: Environment variables
        elif env_prefix:
            env_key = f"{env_prefix}{field_name.upper()}"
            env_value = os.getenv(env_key)

            if env_value is not None:
                # Try to convert to appropriate type
                value = _convert_env_value(env_value, field_info[field_name])
                source = f"env[{env_key}]"

        # Priority 3: defaults dict
        if value is None and field_name in defaults:
            value = defaults[field_name]
            source = "defaults"

        # Set value if found
        if value is not None:
            config_dict[field_name] = value
            logger.debug(
                f"Config {config_class.__name__}.{field_name} = {value} (from {source})"
            )

    # Instantiate config class
    try:
        if is_dataclass(config_class):
            config = config_class(**config_dict)
        else:  # Pydantic
            config = config_class(**config_dict)

        logger.info(f"Loaded configuration: {config_class.__name__}")
        return config

    except Exception as e:
        logger.error(f"Failed to load configuration {config_class.__name__}: {e}")
        raise


def _convert_env_value(env_value: str, field_info: Any) -> Any:
    """Convert environment variable string to appropriate type.

    Args:
        env_value: String value from environment
        field_info: Field information (dataclass field or Pydantic field)

    Returns:
        Converted value
    """
    # Get field type
    if hasattr(field_info, "type"):  # Dataclass
        field_type = field_info.type
    elif hasattr(field_info, "annotation"):  # Pydantic
        field_type = field_info.annotation
    else:
        return env_value  # Can't determine type, return as string

    # Handle Optional types
    if hasattr(field_type, "__origin__"):  # Generic type
        if (
            field_type.__origin__ is type(None)
            or str(field_type.__origin__) == "typing.Union"
        ):
            # Get first non-None type
            args = getattr(field_type, "__args__", ())
            for arg in args:
                if arg is not type(None):
                    field_type = arg
                    break

    # Convert based on type
    try:
        if field_type == bool:
            return env_value.lower() in ("true", "1", "yes", "on")
        elif field_type == int:
            return int(env_value)
        elif field_type == float:
            return float(env_value)
        elif field_type == str:
            return env_value
        else:
            # For other types, try direct conversion
            return field_type(env_value)
    except (ValueError, TypeError):
        logger.warning(
            f"Failed to convert env value '{env_value}' to {field_type}, "
            f"using as string"
        )
        return env_value


class ConfigLoader:
    """Static class for loading configurations."""

    @staticmethod
    def load(
        config_class: Type[T],
        args: Optional[Dict[str, Any]] = None,
        env_prefix: str = "",
        defaults: Optional[Dict[str, Any]] = None,
    ) -> T:
        """Load configuration. See load_config() for details."""
        return load_config(config_class, args, env_prefix, defaults)
