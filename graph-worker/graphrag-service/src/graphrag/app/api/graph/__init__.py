"""
Graph Data API

Provides REST endpoints for querying GraphRAG knowledge graph data.

Usage:
    # Start the unified server
    python -m src.app.api.graph.server --port 8081
    
    # Or use the router directly
    from src.app.api.graph.router import handle_request
    result, status = handle_request("GET", "entities/search", {"q": "algorithm"})
    
    # Or import handlers for direct use
    from src.app.api.graph.handlers import entities, communities, relationships
    results = entities.search(db_name="2025-12", query="machine learning")

Architecture:
    - router.py: Central request routing
    - server.py: HTTP server entry point
    - handlers/: Pure business logic modules
    - constants.py: Shared configuration
    - docs/: API documentation
"""

from .router import handle_request
from .constants import (
    DEFAULT_DB_NAME,
    DEFAULT_LIMIT,
    MAX_LIMIT,
    API_VERSION,
    DEFAULT_PORT,
    EXPORT_FORMATS,
)
from . import handlers

__all__ = [
    # Main entry points
    "handle_request",
    "handlers",
    # Configuration
    "DEFAULT_DB_NAME",
    "DEFAULT_LIMIT",
    "MAX_LIMIT",
    "API_VERSION",
    "DEFAULT_PORT",
    "EXPORT_FORMATS",
]

