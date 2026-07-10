"""
Configuration Library - Cross-Cutting Concern.

Provides centralized configuration loading with priority: args > env > defaults.
Part of the CORE libraries - Tier 2.

Usage:
    from src.lib.configuration import ConfigLoader, load_config
    from dataclasses import dataclass

    @dataclass
    class MyConfig:
        db_name: str = "default"
        max_workers: int = 10
        temperature: float = 0.1

    # Method 1: Using function
    config = load_config(
        MyConfig,
        args={'max_workers': 20},
        env_prefix='MYAPP_',  # Reads MYAPP_DB_NAME, MYAPP_MAX_WORKERS, etc.
        defaults={'db_name': 'production'}
    )

    # Method 2: Using ConfigLoader class
    config = ConfigLoader.load(
        MyConfig,
        args=cli_args,
        env_prefix='GRAPHRAG_',
        defaults={'temperature': 0.2}
    )

Priority (highest to lowest):
    1. args dict (e.g., from CLI arguments)
    2. Environment variables (with prefix)
    3. defaults dict
    4. config_class field defaults
"""

from src.lib.configuration.loader import ConfigLoader, load_config

__all__ = [
    "ConfigLoader",
    "load_config",
]
