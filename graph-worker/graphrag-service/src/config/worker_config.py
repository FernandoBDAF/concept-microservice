"""Worker configuration, loaded from environment variables via pydantic-settings.

Env vars are pinned by graph-worker/shared/contracts (CONTRACTS.md section 4):
RABBITMQ_URL, MONGODB_URI, MINIO_ENDPOINT, MINIO_ACCESS_KEY, MINIO_SECRET_KEY,
HEALTH_PORT. Everything else below is an existing "extra" var kept for
backwards compatibility, each with a sane default so the service is
runnable out of the box against the docker-compose local-dev stack.
"""

from typing import Any, Dict, Optional

from pydantic import Field
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", case_sensitive=False, extra="ignore")

    # RabbitMQ. RABBITMQ_URL (contract-pinned) takes precedence when set; the
    # host/port/user/password/vhost fields are the pre-existing fallback used
    # to build a connection URL when RABBITMQ_URL is not provided.
    rabbitmq_url: Optional[str] = Field(default=None, alias="RABBITMQ_URL")
    rabbitmq_host: str = Field(default="rabbitmq", alias="RABBITMQ_HOST")
    rabbitmq_port: int = Field(default=5672, alias="RABBITMQ_PORT")
    rabbitmq_user: str = Field(default="guest", alias="RABBITMQ_USER")
    rabbitmq_password: str = Field(default="guest", alias="RABBITMQ_PASSWORD")
    rabbitmq_vhost: str = Field(default="/", alias="RABBITMQ_VHOST")

    # MongoDB. Default matches the documented local-dev compose credentials.
    mongodb_uri: str = Field(default="mongodb://admin:password@mongodb:27017", alias="MONGODB_URI")
    mongodb_database: str = Field(default="graphrag", alias="MONGODB_DATABASE")

    # OpenAI. Optional: without it the heavy GraphRAG pipeline stays in stub
    # mode (see src/worker/processor.py) even if requirements-graphrag.txt is
    # installed.
    openai_api_key: str = Field(default="", alias="OPENAI_API_KEY")

    # MinIO / S3. The local-dev compose stack and the k8s base overlay set
    # MINIO_ACCESS_KEY / MINIO_SECRET_KEY explicitly (minioadmin). The AWS
    # overlay deletes them so pods authenticate via IRSA; the keys default to
    # empty here so an unset env selects the ambient AWS credential chain in
    # processor._init_minio (see there). Both-set -> static, both-empty ->
    # ambient/IRSA; a partial config is rejected below.
    minio_endpoint: str = Field(default="minio:9000", alias="MINIO_ENDPOINT")
    minio_access_key: str = Field(default="", alias="MINIO_ACCESS_KEY")
    minio_secret_key: str = Field(default="", alias="MINIO_SECRET_KEY")
    minio_use_ssl: bool = Field(default=False, alias="MINIO_USE_SSL")

    # Redis (idempotency guard, ADR-008.2). host:port, e.g. redis:6379.
    # Empty -> in-process dedupe fallback (single-replica only) + warning.
    redis_addr: str = Field(default="", alias="REDIS_ADDR")

    # Worker
    health_port: int = Field(default=8080, alias="HEALTH_PORT")
    metrics_port: int = Field(default=8081, alias="METRICS_PORT")
    log_level: str = Field(default="INFO", alias="LOG_LEVEL")


def load_config() -> Dict[str, Any]:
    settings = Settings()

    return {
        "rabbitmq": {
            "url": settings.rabbitmq_url,
            "host": settings.rabbitmq_host,
            "port": settings.rabbitmq_port,
            "username": settings.rabbitmq_user,
            "password": settings.rabbitmq_password,
            "vhost": settings.rabbitmq_vhost,
            "exchange": "document-tasks",
            "queue": "document-processing",
            "routing_key": "document.process",
            "prefetch_count": 1,
            # Retry/DLX exchanges follow the `<exchange>.retry` / `<exchange>.dlx`
            # convention in deploy/rabbitmq/definitions.json; task-results is the
            # shared completion channel (ADR-008.3). All are passive-declared.
            "results_exchange": "task-results",
            "results_routing_key": "task.result",
        },
        "redis_addr": settings.redis_addr,
        "mongodb": {
            "uri": settings.mongodb_uri,
            "database": settings.mongodb_database,
        },
        "openai": {
            "api_key": settings.openai_api_key,
        },
        "minio": {
            "endpoint": settings.minio_endpoint,
            "access_key": settings.minio_access_key,
            "secret_key": settings.minio_secret_key,
            "use_ssl": settings.minio_use_ssl,
        },
        "health_port": settings.health_port,
        "metrics_port": settings.metrics_port,
        "log_level": settings.log_level,
    }
