import argparse
import asyncio
import logging
import os
import tempfile
from typing import Any, Dict, Optional

from minio import Minio
from minio.credentials import IamAwsProvider

logger = logging.getLogger(__name__)


def _import_graphrag_pipelines():
    """Lazily import the heavy GraphRAG/ingestion pipeline stack.

    These modules transitively pull in requirements-graphrag.txt (openai,
    pymongo, numpy, pandas, langchain, scikit-learn, ...), which is NOT part
    of the core requirements.txt install. Importing them only here (at call
    time, inside a try/except) means the rest of the service -- cmd/main.py,
    the RabbitMQ consumer, health/metrics server -- imports and runs cleanly
    with just the core dependencies installed. The real pipeline activates
    automatically once requirements-graphrag.txt is installed.
    """
    from src.domain.ingestion.pipeline import IngestionPipeline, IngestionPipelineConfig
    from src.domain.graphrag.pipeline import GraphRAGPipeline
    from src.core.config.graphrag import GraphRAGPipelineConfig

    return IngestionPipeline, IngestionPipelineConfig, GraphRAGPipeline, GraphRAGPipelineConfig


class DocumentProcessor:
    """Async processor that runs ingestion and GraphRAG pipelines."""

    def __init__(self, config: dict) -> None:
        self.config = config
        self.minio_client = self._init_minio()

    def _init_minio(self) -> Minio:
        minio_cfg = self.config["minio"]
        access_key = minio_cfg["access_key"]
        secret_key = minio_cfg["secret_key"]
        secure = minio_cfg.get("use_ssl", False)

        # Credential mode selection:
        #   - static:      both keys set (compose / kind / self-hosted MinIO).
        #   - ambient/IRSA: keys absent -> fall back to the AWS credential chain.
        # A partial config (only one key) is a misconfiguration and fails fast.
        if access_key and secret_key:
            logger.info("MinIO credential mode: static")
            return Minio(
                minio_cfg["endpoint"],
                access_key=access_key,
                secret_key=secret_key,
                secure=secure,
            )
        if access_key or secret_key:
            raise ValueError(
                "MINIO_ACCESS_KEY and MINIO_SECRET_KEY must both be set (static creds) "
                "or both be empty (ambient/IRSA creds)"
            )

        # Ambient credentials. IamAwsProvider resolves EKS IRSA via the
        # web-identity path: it reads AWS_WEB_IDENTITY_TOKEN_FILE + AWS_ROLE_ARN
        # (injected by EKS) and exchanges the projected SA token with STS. It
        # also covers the EC2/ECS metadata providers.
        logger.info("MinIO credential mode: ambient/IRSA")
        return Minio(
            minio_cfg["endpoint"],
            secure=secure,
            credentials=IamAwsProvider(),
        )

    def validate(self, message: dict) -> bool:
        required_fields = ["id", "type", "payload", "timestamp"]
        if not all(field in message for field in required_fields):
            logger.error("Missing required fields", extra={"required": required_fields})
            return False

        payload = message.get("payload", {})
        payload_required = ["document_id", "storage_path", "storage_bucket"]
        if not all(field in payload for field in payload_required):
            logger.error("Missing payload fields", extra={"required": payload_required})
            return False

        return True

    async def process(self, message: dict) -> Dict[str, Any]:
        payload = message["payload"]
        document_id = payload["document_id"]
        storage_path = payload["storage_path"]
        storage_bucket = payload["storage_bucket"]
        user_id = payload.get("user_id")

        logger.info("Processing document", extra={"document_id": document_id})

        local_path = await self._download_document(storage_bucket, storage_path)

        try:
            try:
                (
                    IngestionPipeline,
                    IngestionPipelineConfig,
                    GraphRAGPipeline,
                    GraphRAGPipelineConfig,
                ) = _import_graphrag_pipelines()
            except Exception as exc:  # heavy deps not installed, or a bug in that stack
                logger.warning(
                    "GraphRAG pipeline unavailable; storing stub result",
                    extra={"document_id": document_id, "reason": repr(exc)},
                )
                return self._stub_result(document_id, f"pipeline import failed: {exc}")

            if not self.config.get("openai", {}).get("api_key"):
                logger.warning(
                    "OPENAI_API_KEY not configured; storing stub result",
                    extra={"document_id": document_id},
                )
                return self._stub_result(document_id, "OPENAI_API_KEY not configured")

            ingest_config = self._build_ingest_config(
                local_path, payload, IngestionPipelineConfig
            )
            graphrag_config = self._build_graphrag_config(
                user_id, document_id, GraphRAGPipelineConfig
            )

            logger.info("Starting ingestion", extra={"document_id": document_id})
            ingest_pipeline = IngestionPipeline(ingest_config)
            loop = asyncio.get_running_loop()
            ingest_exit_code = await loop.run_in_executor(
                None, ingest_pipeline.run_full_pipeline
            )

            logger.info("Starting GraphRAG", extra={"document_id": document_id})
            graphrag_pipeline = GraphRAGPipeline(graphrag_config)
            graphrag_exit_code = await loop.run_in_executor(
                None, graphrag_pipeline.run_full_pipeline
            )

            return {
                "status": "completed" if graphrag_exit_code == 0 else "failed",
                "document_id": document_id,
                "ingestion_exit_code": ingest_exit_code,
                "graphrag_exit_code": graphrag_exit_code,
            }
        finally:
            if os.path.exists(local_path):
                os.remove(local_path)

    @staticmethod
    def _stub_result(document_id: str, reason: str) -> Dict[str, Any]:
        """Minimal result used while the heavy GraphRAG pipeline is inactive.

        Lets the worker satisfy consume -> validate -> store minimal result
        -> ack end-to-end using only core dependencies.
        """
        return {"status": "stubbed", "document_id": document_id, "detail": reason}

    async def _download_document(self, bucket: str, path: str) -> str:
        loop = asyncio.get_running_loop()

        def download() -> str:
            suffix = os.path.splitext(path)[1]
            temp_file = tempfile.NamedTemporaryFile(delete=False, suffix=suffix)
            self.minio_client.fget_object(bucket, path, temp_file.name)
            return temp_file.name

        return await loop.run_in_executor(None, download)

    def _build_ingest_config(self, local_path: str, payload: dict, config_cls: Any) -> Any:
        """Build an `IngestionPipelineConfig` (passed in, since it is only
        imported lazily -- see `_import_graphrag_pipelines`)."""
        db_name = self.config["mongodb"]["database"]
        args = argparse.Namespace(
            db_name=db_name,
            concurrency=None,
            verbose=False,
            dry_run=False,
            max=None,
            upsert_existing=False,
            playlist_id=None,
            channel_id=None,
            video_ids=None,
        )
        env = dict(os.environ)
        env.setdefault("DB_NAME", db_name)
        return config_cls.from_args_env(args, env, db_name)

    def _build_graphrag_config(
        self, user_id: Optional[str], document_id: str, config_cls: Any
    ) -> Any:
        """Build a `GraphRAGPipelineConfig` (passed in, since it is only
        imported lazily -- see `_import_graphrag_pipelines`)."""
        db_name = self.config["mongodb"]["database"]
        args = argparse.Namespace(
            db_name=db_name,
            read_db_name=None,
            write_db_name=None,
            verbose=False,
            dry_run=False,
            max=None,
        )
        env = dict(os.environ)
        env.setdefault("DB_NAME", db_name)
        return config_cls.from_args_env(args, env, db_name)
