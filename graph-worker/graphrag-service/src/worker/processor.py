import argparse
import asyncio
import logging
import os
import tempfile
from typing import Any, Dict

from minio import Minio

from src.domain.ingestion.pipeline import IngestionPipeline, IngestionPipelineConfig
from src.domain.graphrag.pipeline import GraphRAGPipeline
from src.core.config.graphrag import GraphRAGPipelineConfig

logger = logging.getLogger(__name__)


class DocumentProcessor:
    """Async processor that runs ingestion and GraphRAG pipelines."""

    def __init__(self, config: dict) -> None:
        self.config = config
        self.minio_client = self._init_minio()

    def _init_minio(self) -> Minio:
        minio_cfg = self.config["minio"]
        return Minio(
            minio_cfg["endpoint"],
            access_key=minio_cfg["access_key"],
            secret_key=minio_cfg["secret_key"],
            secure=minio_cfg.get("use_ssl", False),
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
            ingest_config = self._build_ingest_config(local_path, payload)
            graphrag_config = self._build_graphrag_config(user_id, document_id)

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

    async def _download_document(self, bucket: str, path: str) -> str:
        loop = asyncio.get_running_loop()

        def download() -> str:
            suffix = os.path.splitext(path)[1]
            temp_file = tempfile.NamedTemporaryFile(delete=False, suffix=suffix)
            self.minio_client.fget_object(bucket, path, temp_file.name)
            return temp_file.name

        return await loop.run_in_executor(None, download)

    def _build_ingest_config(self, local_path: str, payload: dict) -> IngestionPipelineConfig:
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
        return IngestionPipelineConfig.from_args_env(args, env, db_name)

    def _build_graphrag_config(self, user_id: str, document_id: str) -> GraphRAGPipelineConfig:
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
        return GraphRAGPipelineConfig.from_args_env(args, env, db_name)
