"""
System Database Services

Handles initialization, indexing, and seeding for the system_data database.
This includes constant collections like raw_videos, agent_prompts, and observability data.
"""

from .initialization import (
    ensure_system_database_initialized,
    ensure_system_indexes,
    ensure_default_prompts,
)

__all__ = [
    "ensure_system_database_initialized",
    "ensure_system_indexes", 
    "ensure_default_prompts",
]

