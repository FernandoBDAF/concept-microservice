"""
GraphRAG Configuration.

This module provides centralized configuration management for GraphRAG
pipeline stages and production environments.
"""

import os
from typing import Dict, Any, Optional, List
from dataclasses import dataclass, field
from src.core.models.config import BaseStageConfig


@dataclass
class GraphRAGEnvironmentConfig:
    """Environment-specific configuration for GraphRAG."""

    # Environment identification
    environment: str = "development"  # development, staging, production

    # MongoDB settings
    mongodb_uri: str = field(
        default_factory=lambda: os.getenv("MONGODB_URI", "mongodb://localhost:27017")
    )
    database_name: str = field(default_factory=lambda: os.getenv("DB_NAME", "mongo_hack"))

    # OpenAI settings
    openai_api_key: str = field(default_factory=lambda: os.getenv("OPENAI_API_KEY", ""))
    openai_model: str = field(default_factory=lambda: os.getenv("GRAPHRAG_MODEL", "gpt-4o-mini"))
    openai_temperature: float = 0.1
    openai_max_tokens: int = 4000

    # GraphRAG pipeline settings
    enable_graphrag: bool = field(
        default_factory=lambda: os.getenv("GRAPHRAG_ENABLED", "true").lower() == "true"
    )
    max_concurrent_extractions: int = field(
        default_factory=lambda: int(os.getenv("GRAPHRAG_EXTRACTION_CONCURRENCY", "300"))
    )
    max_concurrent_resolutions: int = field(
        default_factory=lambda: int(os.getenv("GRAPHRAG_RESOLUTION_CONCURRENCY", "300"))
    )
    max_cluster_size: int = field(
        default_factory=lambda: int(os.getenv("GRAPHRAG_MAX_CLUSTER_SIZE", "10"))
    )
    entity_resolution_threshold: float = field(
        default_factory=lambda: float(os.getenv("GRAPHRAG_ENTITY_RESOLUTION_THRESHOLD", "0.85"))
    )

    # Performance settings
    operation_timeout_ms: int = 30000
    memory_limit_mb: int = 8192
    cpu_limit_percent: int = 80

    # Monitoring settings
    enable_monitoring: bool = True
    metrics_retention_days: int = 30
    log_level: str = "INFO"

    # Caching settings
    enable_caching: bool = True
    cache_ttl_hours: int = 2
    max_cache_size: int = 1000

    # Error handling
    max_retry_attempts: int = 3
    retry_delay_ms: int = 1000
    circuit_breaker_threshold: int = 5

    def __post_init__(self):
        """Post-initialization validation and setup."""
        # Validate required settings
        if not self.openai_api_key:
            raise ValueError("OpenAI API key is required")

        if not self.mongodb_uri:
            raise ValueError("MongoDB URI is required")

        # Environment-specific overrides
        if self.environment == "production":
            self._apply_production_settings()
        elif self.environment == "staging":
            self._apply_staging_settings()
        else:
            self._apply_development_settings()

    def _apply_production_settings(self):
        """Apply production-specific settings."""
        self.log_level = "WARNING"
        self.enable_monitoring = True
        self.enable_caching = True
        self.max_concurrent_extractions = 20
        self.max_concurrent_resolutions = 15
        self.operation_timeout_ms = 60000
        self.memory_limit_mb = 16384
        self.cpu_limit_percent = 70
        self.max_retry_attempts = 5
        self.circuit_breaker_threshold = 3

    def _apply_staging_settings(self):
        """Apply staging-specific settings."""
        self.log_level = "INFO"
        self.enable_monitoring = True
        self.enable_caching = True
        self.max_concurrent_extractions = 10
        self.max_concurrent_resolutions = 8
        self.operation_timeout_ms = 45000
        self.memory_limit_mb = 8192
        self.cpu_limit_percent = 80
        self.max_retry_attempts = 3
        self.circuit_breaker_threshold = 5

    def _apply_development_settings(self):
        """Apply development-specific settings."""
        self.log_level = "DEBUG"
        self.enable_monitoring = False
        self.enable_caching = False
        self.max_concurrent_extractions = 5
        self.max_concurrent_resolutions = 3
        self.operation_timeout_ms = 30000
        self.memory_limit_mb = 4096
        self.cpu_limit_percent = 90
        self.max_retry_attempts = 2
        self.circuit_breaker_threshold = 10

    def to_production_config(self) -> Dict[str, Any]:
        """
        Convert to ProductionConfig for GraphRAG production manager.

        ⚠️  NOTE: This method is for future use when graphrag_production.py is implemented.
        Currently returns a dict representation of production config.

        This method is only used by documentation/examples/monitor_graphrag.py,
        which is non-functional until graphrag_production.py is implemented.
        """
        # Stub implementation - returns dict instead of ProductionConfig object
        # TODO: Implement proper ProductionConfig when graphrag_production.py is created
        return {
            "max_concurrent_operations": self.max_concurrent_extractions,
            "operation_timeout_ms": self.operation_timeout_ms,
            "memory_limit_mb": self.memory_limit_mb,
            "cpu_limit_percent": self.cpu_limit_percent,
            "enable_performance_monitoring": self.enable_monitoring,
            "metrics_retention_days": self.metrics_retention_days,
            "cache_config": {
                "enable_entity_cache": self.enable_caching,
                "enable_community_cache": self.enable_caching,
                "enable_query_cache": self.enable_caching,
                "entity_cache_ttl": self.cache_ttl_hours * 3600,
                "community_cache_ttl": self.cache_ttl_hours * 3600,
                "query_cache_ttl": self.cache_ttl_hours * 1800,
                "max_cache_size": self.max_cache_size,
                "cache_cleanup_interval": 300,
            },
            "max_retry_attempts": self.max_retry_attempts,
            "retry_delay_ms": self.retry_delay_ms,
            "circuit_breaker_threshold": self.circuit_breaker_threshold,
        }

    def get_graphrag_stage_configs(self) -> Dict[str, Any]:
        """Get configuration for GraphRAG pipeline stages."""
        return {
            "graph_extraction": {
                "max": None,
                "llm": True,
                "verbose": self.log_level == "DEBUG",
                "concurrency": self.max_concurrent_extractions,
                "model_name": self.openai_model,
                "temperature": self.openai_temperature,
                "max_tokens": self.openai_max_tokens,
                "timeout_ms": self.operation_timeout_ms,
            },
            "entity_resolution": {
                "max": None,
                "llm": True,
                "verbose": self.log_level == "DEBUG",
                "concurrency": self.max_concurrent_resolutions,
                "model_name": self.openai_model,
                "temperature": self.openai_temperature,
                "max_tokens": self.openai_max_tokens,
                "similarity_threshold": self.entity_resolution_threshold,
                "timeout_ms": self.operation_timeout_ms,
            },
            "graph_construction": {
                "max": None,
                "llm": True,
                "verbose": self.log_level == "DEBUG",
                "concurrency": self.max_concurrent_resolutions,
                "model_name": self.openai_model,
                "temperature": self.openai_temperature,
                "max_tokens": self.openai_max_tokens,
                "min_relationship_confidence": 0.6,
                "timeout_ms": self.operation_timeout_ms,
            },
            "community_detection": {
                "max": None,
                "llm": True,
                "verbose": self.log_level == "DEBUG",
                "concurrency": 5,  # Lower concurrency for community detection
                "model_name": self.openai_model,
                "temperature": self.openai_temperature,
                "max_tokens": self.openai_max_tokens,
                "max_cluster_size": self.max_cluster_size,
                "resolution_parameter": 1.0,
                "timeout_ms": self.operation_timeout_ms,
            },
        }

    def get_mongodb_settings(self) -> Dict[str, Any]:
        """Get MongoDB-specific settings."""
        return {
            "uri": self.mongodb_uri,
            "database": self.database_name,
            "connection_timeout_ms": 10000,
            "server_selection_timeout_ms": 5000,
            "max_pool_size": 50 if self.environment == "production" else 20,
            "min_pool_size": 5 if self.environment == "production" else 2,
            "max_idle_time_ms": 300000,  # 5 minutes
            "retry_writes": True,
            "retry_reads": True,
        }

    def get_openai_settings(self) -> Dict[str, Any]:
        """Get OpenAI-specific settings."""
        return {
            "api_key": self.openai_api_key,
            "model": self.openai_model,
            "temperature": self.openai_temperature,
            "max_tokens": self.openai_max_tokens,
            "timeout": self.operation_timeout_ms / 1000,  # Convert to seconds
            "max_retries": self.max_retry_attempts,
            "retry_delay": self.retry_delay_ms / 1000,  # Convert to seconds
            "rate_limit_requests_per_minute": (60 if self.environment == "production" else 20),
            "rate_limit_tokens_per_minute": (150000 if self.environment == "production" else 40000),
        }

    def validate_configuration(self) -> Dict[str, Any]:
        """Validate the configuration and return validation results."""
        validation_results = {
            "valid": True,
            "errors": [],
            "warnings": [],
            "recommendations": [],
        }

        # Check required environment variables
        required_env_vars = ["MONGODB_URI", "OPENAI_API_KEY"]
        for var in required_env_vars:
            if not os.getenv(var):
                validation_results["errors"].append(f"Missing required environment variable: {var}")
                validation_results["valid"] = False

        # Check MongoDB connection
        try:
            from pymongo import MongoClient

            client = MongoClient(self.mongodb_uri, serverSelectionTimeoutMS=5000)
            client.server_info()
            client.close()
        except Exception as e:
            validation_results["errors"].append(f"MongoDB connection failed: {e}")
            validation_results["valid"] = False

        # Check OpenAI API key format
        if self.openai_api_key and not self.openai_api_key.startswith("sk-"):
            validation_results["warnings"].append("OpenAI API key format may be incorrect")

        # Performance recommendations
        if self.environment == "production":
            if self.max_concurrent_extractions < 15:
                validation_results["recommendations"].append(
                    "Consider increasing max_concurrent_extractions for production"
                )

            if not self.enable_monitoring:
                validation_results["recommendations"].append(
                    "Enable monitoring for production environment"
                )

            if not self.enable_caching:
                validation_results["recommendations"].append(
                    "Enable caching for production environment"
                )

        # Resource recommendations
        if self.memory_limit_mb < 4096:
            validation_results["recommendations"].append(
                "Consider increasing memory limit for better performance"
            )

        if self.operation_timeout_ms < 30000:
            validation_results["recommendations"].append(
                "Consider increasing operation timeout for complex operations"
            )

        return validation_results

    def get_environment_summary(self) -> Dict[str, Any]:
        """Get a summary of the current environment configuration."""
        return {
            "environment": self.environment,
            "graphrag_enabled": self.enable_graphrag,
            "monitoring_enabled": self.enable_monitoring,
            "caching_enabled": self.enable_caching,
            "concurrent_extractions": self.max_concurrent_extractions,
            "concurrent_resolutions": self.max_concurrent_resolutions,
            "operation_timeout_ms": self.operation_timeout_ms,
            "memory_limit_mb": self.memory_limit_mb,
            "cpu_limit_percent": self.cpu_limit_percent,
            "log_level": self.log_level,
            "openai_model": self.openai_model,
            "database": self.database_name,
            "mongodb_uri": (
                self.mongodb_uri[:20] + "..." if len(self.mongodb_uri) > 20 else self.mongodb_uri
            ),
        }


# GraphRAG Stage Configurations
@dataclass
class GraphExtractionConfig(BaseStageConfig):
    """Configuration for graph extraction stage."""

    # LLM settings
    model_name: str = "gpt-4o-mini"
    temperature: float = 0.1
    max_tokens: Optional[int] = None
    llm_retries: int = 3
    llm_backoff_s: float = 1.0

    # Extraction settings
    max_entities_per_chunk: int = 20
    max_relationships_per_chunk: int = 30
    min_entity_confidence: float = 0.3
    min_relationship_confidence: float = 0.3

    # Processing settings
    batch_size: int = 50
    extraction_timeout: int = 30  # seconds per chunk

    # Entity types to extract
    entity_types: List[str] = None

    def __post_init__(self):
        if self.entity_types is None:
            self.entity_types = [
                "PERSON",
                "ORGANIZATION",
                "TECHNOLOGY",
                "CONCEPT",
                "LOCATION",
                "EVENT",
                "OTHER",
            ]

    @classmethod
    def from_args_env(cls, args, env, default_db):
        from src.core.config.paths import COLL_CHUNKS

        # Get base config with default collections
        base = BaseStageConfig.from_args_env(
            args,
            env,
            default_db,
            default_read_coll=COLL_CHUNKS,
            default_write_coll=COLL_CHUNKS,
        )

        # Get stage-specific settings from args first, then env (UI values take precedence)
        model_name = getattr(args, "model_name", None) or env.get("GRAPHRAG_MODEL") or env.get("OPENAI_MODEL") or "gpt-4o-mini"
        temperature = float(getattr(args, "temperature", None) or env.get("GRAPHRAG_TEMPERATURE", "0.1"))
        max_tokens_arg = getattr(args, "max_tokens", None)
        max_tokens = int(max_tokens_arg) if max_tokens_arg else (int(env.get("GRAPHRAG_MAX_TOKENS")) if env.get("GRAPHRAG_MAX_TOKENS") else None)
        llm_retries = int(getattr(args, "llm_retries", None) or env.get("GRAPHRAG_LLM_RETRIES", "3"))
        llm_backoff_s = float(getattr(args, "llm_backoff_s", None) or env.get("GRAPHRAG_LLM_BACKOFF_S", "1.0"))
        max_entities_per_chunk = int(getattr(args, "max_entities_per_chunk", None) or env.get("GRAPHRAG_MAX_ENTITIES_PER_CHUNK", "20"))
        max_relationships_per_chunk = int(getattr(args, "max_relationships_per_chunk", None) or env.get("GRAPHRAG_MAX_RELATIONSHIPS_PER_CHUNK", "30"))
        min_entity_confidence = float(getattr(args, "min_entity_confidence", None) or env.get("GRAPHRAG_MIN_ENTITY_CONFIDENCE", "0.3"))
        min_relationship_confidence = float(getattr(args, "min_relationship_confidence", None) or env.get("GRAPHRAG_MIN_RELATIONSHIP_CONFIDENCE", "0.3"))
        batch_size = int(getattr(args, "batch_size", None) or env.get("GRAPHRAG_BATCH_SIZE", "50"))
        extraction_timeout = int(getattr(args, "extraction_timeout", None) or env.get("GRAPHRAG_EXTRACTION_TIMEOUT", "30"))

        return cls(
            **vars(base),
            model_name=model_name,
            temperature=temperature,
            max_tokens=max_tokens,
            llm_retries=llm_retries,
            llm_backoff_s=llm_backoff_s,
            max_entities_per_chunk=max_entities_per_chunk,
            max_relationships_per_chunk=max_relationships_per_chunk,
            min_entity_confidence=min_entity_confidence,
            min_relationship_confidence=min_relationship_confidence,
            batch_size=batch_size,
            extraction_timeout=extraction_timeout,
        )


@dataclass
class EntityResolutionConfig(BaseStageConfig):
    """Configuration for entity resolution stage."""

    # LLM settings
    model_name: str = "gpt-4o-mini"
    temperature: float = 0.1
    max_tokens: Optional[int] = None
    llm_retries: int = 3
    llm_backoff_s: float = 1.0

    # Resolution settings
    similarity_threshold: float = 0.85
    max_aliases_per_entity: int = 10
    min_source_count: int = 1

    # Token budget management (optional, disabled by default)
    # Set to None to disable token budget (preserves quality)
    # Set to a number (e.g., 6000) to limit input tokens per entity
    # Note: With cheap models (gpt-4o-mini), token limits are usually not critical
    max_input_tokens_per_entity: Optional[int] = None

    # Processing settings
    batch_size: int = 100
    resolution_timeout: int = 60  # seconds per batch

    # Resolution strategies
    use_fuzzy_matching: bool = True
    use_embedding_similarity: bool = True
    use_context_similarity: bool = True
    use_relationship_clustering: bool = True

    @classmethod
    def from_args_env(cls, args, env, default_db):
        from src.core.config.paths import COLL_CHUNKS

        # Get base config with default collections
        base = BaseStageConfig.from_args_env(
            args,
            env,
            default_db,
            default_read_coll=COLL_CHUNKS,
            default_write_coll=COLL_CHUNKS,
        )

        # Get stage-specific settings from args first, then env (UI values take precedence)
        model_name = getattr(args, "model_name", None) or env.get("GRAPHRAG_MODEL") or env.get("OPENAI_MODEL") or "gpt-4o-mini"
        temperature = float(getattr(args, "temperature", None) or env.get("GRAPHRAG_TEMPERATURE", "0.1"))
        max_tokens_arg = getattr(args, "max_tokens", None)
        max_tokens = int(max_tokens_arg) if max_tokens_arg else (int(env.get("GRAPHRAG_MAX_TOKENS")) if env.get("GRAPHRAG_MAX_TOKENS") else None)
        llm_retries = int(getattr(args, "llm_retries", None) or env.get("GRAPHRAG_LLM_RETRIES", "3"))
        llm_backoff_s = float(getattr(args, "llm_backoff_s", None) or env.get("GRAPHRAG_LLM_BACKOFF_S", "1.0"))
        similarity_threshold = float(getattr(args, "similarity_threshold", None) or env.get("GRAPHRAG_ENTITY_RESOLUTION_THRESHOLD", "0.85"))
        max_aliases_per_entity = int(getattr(args, "max_aliases_per_entity", None) or env.get("GRAPHRAG_MAX_ALIASES_PER_ENTITY", "10"))
        min_source_count = int(getattr(args, "min_source_count", None) or env.get("GRAPHRAG_MIN_SOURCE_COUNT", "1"))
        batch_size = int(getattr(args, "batch_size", None) or env.get("GRAPHRAG_RESOLUTION_BATCH_SIZE", "100"))
        resolution_timeout = int(getattr(args, "resolution_timeout", None) or env.get("GRAPHRAG_RESOLUTION_TIMEOUT", "60"))
        
        # Boolean settings - check args first
        use_fuzzy_matching_arg = getattr(args, "use_fuzzy_matching", None)
        use_fuzzy_matching = use_fuzzy_matching_arg if use_fuzzy_matching_arg is not None else env.get("GRAPHRAG_USE_FUZZY_MATCHING", "true").lower() == "true"
        
        use_embedding_similarity_arg = getattr(args, "use_embedding_similarity", None)
        use_embedding_similarity = use_embedding_similarity_arg if use_embedding_similarity_arg is not None else env.get("GRAPHRAG_USE_EMBEDDING_SIMILARITY", "true").lower() == "true"
        
        use_context_similarity_arg = getattr(args, "use_context_similarity", None)
        use_context_similarity = use_context_similarity_arg if use_context_similarity_arg is not None else env.get("GRAPHRAG_USE_CONTEXT_SIMILARITY", "true").lower() == "true"
        
        use_relationship_clustering_arg = getattr(args, "use_relationship_clustering", None)
        use_relationship_clustering = use_relationship_clustering_arg if use_relationship_clustering_arg is not None else env.get("GRAPHRAG_USE_RELATIONSHIP_CLUSTERING", "true").lower() == "true"

        return cls(
            **vars(base),
            model_name=model_name,
            temperature=temperature,
            max_tokens=max_tokens,
            llm_retries=llm_retries,
            llm_backoff_s=llm_backoff_s,
            similarity_threshold=similarity_threshold,
            max_aliases_per_entity=max_aliases_per_entity,
            min_source_count=min_source_count,
            batch_size=batch_size,
            resolution_timeout=resolution_timeout,
            use_fuzzy_matching=use_fuzzy_matching,
            use_embedding_similarity=use_embedding_similarity,
            use_context_similarity=use_context_similarity,
            use_relationship_clustering=use_relationship_clustering,
        )


@dataclass
class GraphConstructionConfig(BaseStageConfig):
    """Configuration for graph construction stage."""

    # Processing settings
    batch_size: int = 200
    max_relationships_per_entity: int = 100

    # Graph metrics
    calculate_centrality: bool = True
    calculate_degree: bool = True
    calculate_clustering: bool = False

    # Relationship validation
    validate_entity_existence: bool = True
    min_relationship_confidence: float = 0.3
    max_relationship_distance: int = 3  # max hops for relationship validation

    @classmethod
    def from_args_env(cls, args, env, default_db):
        from src.core.config.paths import COLL_CHUNKS

        # Get base config with default collections
        base = BaseStageConfig.from_args_env(
            args,
            env,
            default_db,
            default_read_coll=COLL_CHUNKS,
            default_write_coll=COLL_CHUNKS,
        )

        # Get stage-specific settings from env
        batch_size = int(env.get("GRAPHRAG_CONSTRUCTION_BATCH_SIZE", "200"))
        max_relationships_per_entity = int(env.get("GRAPHRAG_MAX_RELATIONSHIPS_PER_ENTITY", "100"))
        calculate_centrality = env.get("GRAPHRAG_CALCULATE_CENTRALITY", "true").lower() == "true"
        calculate_degree = env.get("GRAPHRAG_CALCULATE_DEGREE", "true").lower() == "true"
        calculate_clustering = env.get("GRAPHRAG_CALCULATE_CLUSTERING", "false").lower() == "true"
        validate_entity_existence = (
            env.get("GRAPHRAG_VALIDATE_ENTITY_EXISTENCE", "true").lower() == "true"
        )
        min_relationship_confidence = float(env.get("GRAPHRAG_MIN_RELATIONSHIP_CONFIDENCE", "0.3"))
        max_relationship_distance = int(env.get("GRAPHRAG_MAX_RELATIONSHIP_DISTANCE", "3"))

        return cls(
            **vars(base),
            batch_size=batch_size,
            max_relationships_per_entity=max_relationships_per_entity,
            calculate_centrality=calculate_centrality,
            calculate_degree=calculate_degree,
            calculate_clustering=calculate_clustering,
            validate_entity_existence=validate_entity_existence,
            min_relationship_confidence=min_relationship_confidence,
            max_relationship_distance=max_relationship_distance,
        )


@dataclass
class CommunityDetectionConfig(BaseStageConfig):
    """Configuration for community detection stage."""

    # LLM settings for summarization
    model_name: str = "gpt-4o-mini"  # Use gpt-4o-mini for small communities (fast, cost-effective)
    temperature: float = 0.2
    max_tokens: Optional[int] = None
    llm_retries: int = 3
    llm_backoff_s: float = 1.0

    # Community detection settings
    algorithm: str = "louvain"  # Algorithm: "louvain" (default) or "hierarchical_leiden"
    max_cluster_size: int = 50  # Soft limit for community size (Louvain ignores this)
    min_cluster_size: int = 2  # Used for post-filtering single-node communities
    resolution_parameter: float = 1.0  # Louvain resolution (0.5-2.0, default 1.0)
    max_iterations: int = 100

    # Hierarchical settings (for hierarchical_leiden only)
    max_levels: int = 3
    level_size_threshold: int = 5

    # Summarization settings
    max_summary_length: int = 2000
    min_summary_length: int = 100
    summarization_timeout: int = 120  # seconds per community

    # Quality thresholds
    min_coherence_score: float = 0.6
    min_entity_count: int = 2

    @classmethod
    def from_args_env(cls, args, env, default_db):
        from src.core.config.paths import COLL_CHUNKS

        # Get base config with default collections
        base = BaseStageConfig.from_args_env(
            args,
            env,
            default_db,
            default_read_coll=COLL_CHUNKS,
            default_write_coll=COLL_CHUNKS,
        )

        # Get stage-specific settings from args first, then env (UI values take precedence)
        model_name = getattr(args, "model_name", None) or env.get("GRAPHRAG_MODEL") or env.get("OPENAI_MODEL") or "gpt-4o-mini"
        temperature = float(getattr(args, "temperature", None) or env.get("GRAPHRAG_COMMUNITY_TEMPERATURE", "0.2"))
        max_tokens_arg = getattr(args, "max_tokens", None)
        max_tokens = int(max_tokens_arg) if max_tokens_arg else (int(env.get("GRAPHRAG_MAX_TOKENS")) if env.get("GRAPHRAG_MAX_TOKENS") else None)
        llm_retries = int(getattr(args, "llm_retries", None) or env.get("GRAPHRAG_LLM_RETRIES", "3"))
        llm_backoff_s = float(getattr(args, "llm_backoff_s", None) or env.get("GRAPHRAG_LLM_BACKOFF_S", "1.0"))
        algorithm = getattr(args, "algorithm", None) or env.get("GRAPHRAG_COMMUNITY_ALGORITHM", "louvain")
        max_cluster_size = int(getattr(args, "max_cluster_size", None) or env.get("GRAPHRAG_MAX_CLUSTER_SIZE", "50"))
        min_cluster_size = int(getattr(args, "min_cluster_size", None) or env.get("GRAPHRAG_MIN_CLUSTER_SIZE", "2"))
        resolution_parameter = float(getattr(args, "resolution_parameter", None) or env.get("GRAPHRAG_RESOLUTION_PARAMETER", "1.0"))
        max_iterations = int(getattr(args, "max_iterations", None) or env.get("GRAPHRAG_MAX_ITERATIONS", "100"))
        max_levels = int(getattr(args, "max_levels", None) or env.get("GRAPHRAG_MAX_LEVELS", "3"))
        level_size_threshold = int(getattr(args, "level_size_threshold", None) or env.get("GRAPHRAG_LEVEL_SIZE_THRESHOLD", "5"))
        max_summary_length = int(getattr(args, "max_summary_length", None) or env.get("GRAPHRAG_MAX_SUMMARY_LENGTH", "2000"))
        min_summary_length = int(getattr(args, "min_summary_length", None) or env.get("GRAPHRAG_MIN_SUMMARY_LENGTH", "100"))
        summarization_timeout = int(getattr(args, "summarization_timeout", None) or env.get("GRAPHRAG_SUMMARIZATION_TIMEOUT", "120"))
        min_coherence_score = float(getattr(args, "min_coherence_score", None) or env.get("GRAPHRAG_MIN_COHERENCE_SCORE", "0.6"))
        min_entity_count = int(getattr(args, "min_entity_count", None) or env.get("GRAPHRAG_MIN_ENTITY_COUNT", "2"))

        # Get concurrency from args or env (default to 300 like other stages)
        # Only override if base doesn't have it set (base already gets it from args/env)
        if base.concurrency is None:
            base.concurrency = int(env.get("GRAPHRAG_COMMUNITY_CONCURRENCY", "300"))

        return cls(
            **vars(base),
            model_name=model_name,
            temperature=temperature,
            max_tokens=max_tokens,
            llm_retries=llm_retries,
            llm_backoff_s=llm_backoff_s,
            algorithm=algorithm,
            max_cluster_size=max_cluster_size,
            min_cluster_size=min_cluster_size,
            resolution_parameter=resolution_parameter,
            max_iterations=max_iterations,
            max_levels=max_levels,
            level_size_threshold=level_size_threshold,
            max_summary_length=max_summary_length,
            min_summary_length=min_summary_length,
            summarization_timeout=summarization_timeout,
            min_coherence_score=min_coherence_score,
            min_entity_count=min_entity_count,
        )


@dataclass
class GraphRAGQueryConfig:
    """Configuration for GraphRAG query processing."""

    # Query processing
    max_query_entities: int = 10
    max_related_entities: int = 50
    max_community_context: int = 5

    # Graph traversal
    max_traversal_depth: int = 2
    max_traversal_width: int = 20

    # Retrieval settings
    entity_search_limit: int = 20
    relationship_search_limit: int = 50
    community_search_limit: int = 10

    # Generation settings
    model_name: str = "gpt-4o-mini"
    temperature: float = 0.3
    max_tokens: Optional[int] = None

    # Timeouts
    query_timeout: int = 30  # seconds
    retrieval_timeout: int = 10  # seconds
    generation_timeout: int = 20  # seconds


@dataclass
class GraphRAGPipelineConfig:
    """
    Configuration for the complete GraphRAG pipeline.

    EXPERIMENT SUPPORT (2024-11-04):
    - experiment_id: Optional identifier for tracking experiments
    - Enables running multiple configurations in parallel
    - Used for comparative analysis and A/B testing
    """

    # Experiment tracking
    experiment_id: Optional[str] = None  # For tracking experiment runs

    # Pipeline settings
    enable_incremental: bool = True
    max_processing_time: int = 7200  # 2 hours in seconds
    checkpoint_interval: int = 100  # chunks

    # Achievement 0.1: Stage Selection & Partial Runs
    selected_stages: Optional[str] = (
        None  # Stage selection string (e.g., "extraction,resolution" or "1-3")
    )

    # Achievement 0.2: Resume from Failure
    resume_from_failure: bool = False  # If True, resume from last failure (skip completed stages)

    # Stage configurations
    extraction_config: GraphExtractionConfig = None
    resolution_config: EntityResolutionConfig = None
    construction_config: GraphConstructionConfig = None
    detection_config: CommunityDetectionConfig = None

    # Error handling
    max_retries: int = 3
    retry_delay: float = 5.0  # seconds
    continue_on_error: bool = True

    # Logging
    log_level: str = "INFO"
    log_file: Optional[str] = None

    def __post_init__(self):
        if self.extraction_config is None:
            self.extraction_config = GraphExtractionConfig()
        if self.resolution_config is None:
            self.resolution_config = EntityResolutionConfig()
        if self.construction_config is None:
            self.construction_config = GraphConstructionConfig()
        if self.detection_config is None:
            self.detection_config = CommunityDetectionConfig()

    @classmethod
    def from_args_env(cls, args, env, default_db):
        """
        Create pipeline config from command line arguments and environment.

        Args:
            args: Parsed command line arguments
            env: Environment variables dict
            default_db: Default database name

        Returns:
            GraphRAGPipelineConfig instance
        """
        # Create stage configs using their from_args_env methods
        extraction_config = GraphExtractionConfig.from_args_env(args, env, default_db)
        resolution_config = EntityResolutionConfig.from_args_env(args, env, default_db)
        construction_config = GraphConstructionConfig.from_args_env(args, env, default_db)
        detection_config = CommunityDetectionConfig.from_args_env(args, env, default_db)

        # Get experiment ID from env (for tracking)
        experiment_id = env.get("EXPERIMENT_ID")

        # Get pipeline-level settings from env
        enable_incremental = env.get("GRAPHRAG_ENABLE_INCREMENTAL", "true").lower() == "true"
        max_processing_time = int(env.get("GRAPHRAG_MAX_PROCESSING_TIME", "7200"))
        checkpoint_interval = int(env.get("GRAPHRAG_CHECKPOINT_INTERVAL", "100"))
        max_retries = int(env.get("GRAPHRAG_MAX_RETRIES", "3"))
        retry_delay = float(env.get("GRAPHRAG_RETRY_DELAY", "5.0"))
        continue_on_error = env.get("GRAPHRAG_CONTINUE_ON_ERROR", "true").lower() == "true"
        log_level = env.get("GRAPHRAG_LOG_LEVEL", "INFO")
        log_file = env.get("GRAPHRAG_LOG_FILE")

        return cls(
            experiment_id=experiment_id,
            enable_incremental=enable_incremental,
            max_processing_time=max_processing_time,
            checkpoint_interval=checkpoint_interval,
            extraction_config=extraction_config,
            resolution_config=resolution_config,
            construction_config=construction_config,
            detection_config=detection_config,
            max_retries=max_retries,
            retry_delay=retry_delay,
            continue_on_error=continue_on_error,
            log_level=log_level,
            log_file=log_file,
        )


def load_config_from_env(
    environment: Optional[str] = None,
) -> GraphRAGEnvironmentConfig:
    """
    Load GraphRAG configuration from environment variables.

    Args:
        environment: Override environment detection

    Returns:
        Configured GraphRAGEnvironmentConfig instance
    """
    # Detect environment if not provided
    if environment is None:
        environment = os.getenv("GRAPHRAG_ENVIRONMENT", "development")

    return GraphRAGEnvironmentConfig(environment=environment)


def load_config_from_file(config_file: str) -> GraphRAGEnvironmentConfig:
    """
    Load GraphRAG configuration from a JSON file.

    Args:
        config_file: Path to configuration file

    Returns:
        Configured GraphRAGEnvironmentConfig instance
    """
    import json

    with open(config_file, "r") as f:
        config_data = json.load(f)

    return GraphRAGEnvironmentConfig(**config_data)


def create_production_config() -> GraphRAGEnvironmentConfig:
    """Create a production-ready configuration."""
    return GraphRAGEnvironmentConfig(environment="production")


def create_staging_config() -> GraphRAGEnvironmentConfig:
    """Create a staging configuration."""
    return GraphRAGEnvironmentConfig(environment="staging")


def create_development_config() -> GraphRAGEnvironmentConfig:
    """Create a development configuration."""
    return GraphRAGEnvironmentConfig(environment="development")


# TODO: Future enhancement - environment-specific config overrides
# For now, use environment variables to configure stages:
# - GRAPHRAG_MODEL, GRAPHRAG_TEMPERATURE for model settings
# - GRAPHRAG_EXTRACTION_CONCURRENCY for extraction concurrency
# - GRAPHRAG_ENTITY_RESOLUTION_THRESHOLD for resolution threshold
# - etc.
#
# Future: Implement GraphRAGEnvironmentConfig to apply environment-specific
# presets (production/staging/development) that override stage configs
