"""
Graph Data API Constants

Shared constants for the Graph Data API.
Part of the centralized API architecture.
"""

import os

# Default database (can be overridden by query param or env var)
DEFAULT_DB_NAME = os.getenv("MONGODB_DB") or os.getenv("DB_NAME") or "2025-12"

# Collection names (from business/services/graphrag/indexes.py)
COLL_ENTITIES = "entities"
COLL_RELATIONS = "relations"
COLL_COMMUNITIES = "communities"
COLL_CHUNKS = "video_chunks"

# Pagination defaults
DEFAULT_LIMIT = 50
MAX_LIMIT = 500
DEFAULT_OFFSET = 0

# API version
API_VERSION = "1.0.0"

# Default server port
DEFAULT_PORT = 8081

# Export formats
EXPORT_FORMATS = ["json", "csv", "graphml", "gexf"]

# Entity types (common ones)
ENTITY_TYPES = [
    "PERSON",
    "ORGANIZATION", 
    "CONCEPT",
    "LOCATION",
    "EVENT",
    "TECHNIQUE",
    "ALGORITHM",
    "OTHER",
]

# Ego network defaults
DEFAULT_MAX_HOPS = 2
DEFAULT_MAX_NODES = 100

# Path finding defaults
DEFAULT_PATH_MAX_HOPS = 5
DEFAULT_PATH_MAX_PATHS = 3

# Timeline defaults
DEFAULT_MAX_CONTEXTS = 3

# Query execution defaults
QUERY_MODES = ["local", "global", "hybrid"]
DEFAULT_QUERY_MODE = "global"
DEFAULT_QUERY_MODEL = "gpt-4o-mini"
DEFAULT_QUERY_TEMPERATURE = 0.3
DEFAULT_QUERY_TOP_K = 10

