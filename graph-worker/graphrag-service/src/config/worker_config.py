from typing import Any, Dict

from pydantic import Field
from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    # RabbitMQ
    rabbitmq_host: str = Field(default="rabbitmq", alias="RABBITMQ_HOST")
    rabbitmq_port: int = Field(default=5672, alias="RABBITMQ_PORT")
    rabbitmq_user: str = Field(default="guest", alias="RABBITMQ_USER")
    rabbitmq_password: str = Field(default="guest", alias="RABBITMQ_PASSWORD")
    rabbitmq_vhost: str = Field(default="/", alias="RABBITMQ_VHOST")

    # MongoDB
    mongodb_uri: str = Field(..., alias="MONGODB_URI")
    mongodb_database: str = Field(default="graphrag", alias="MONGODB_DATABASE")

    # OpenAI
    openai_api_key: str = Field(..., alias="OPENAI_API_KEY")

    # MinIO
    minio_endpoint: str = Field(default="minio:9000", alias="MINIO_ENDPOINT")
    minio_access_key: str = Field(..., alias="MINIO_ACCESS_KEY")
    minio_secret_key: str = Field(..., alias="MINIO_SECRET_KEY")
    minio_use_ssl: bool = Field(default=False, alias="MINIO_USE_SSL")

    # Worker
    health_port: int = Field(default=8080, alias="HEALTH_PORT")
    metrics_port: int = Field(default=8081, alias="METRICS_PORT")
    log_level: str = Field(default="INFO", alias="LOG_LEVEL")

    class Config:
        env_file = ".env"
        case_sensitive = False


def load_config() -> Dict[str, Any]:
    settings = Settings()

    return {
        "rabbitmq": {
            "host": settings.rabbitmq_host,
            "port": settings.rabbitmq_port,
            "username": settings.rabbitmq_user,
            "password": settings.rabbitmq_password,
            "vhost": settings.rabbitmq_vhost,
            "exchange": "document-tasks",
            "queue": "document-processing",
            "routing_key": "document.process",
            "prefetch_count": 1,
        },
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
