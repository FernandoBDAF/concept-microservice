# GraphRAG Service - Implementation Plan

**Project:** graphrag-service  
**Language:** Python  
**Status:** ✅ Core worker implemented and verified (compileall + entrypoint import + offline
smoke test, core deps only) as of 2026-07-07. ⚠️ GraphRAG/ingestion pipeline itself
(`src/domain/graphrag`, `src/domain/ingestion`) is present but lazily-loaded and
**not yet run end-to-end** (no live MongoDB/OpenAI in the verification environment) --
see `README.md` for current status, env vars, and known gaps.  
**Session Focus:** This document is the original Day 1-6 build plan, kept for historical
reference. The plan below (directory layout, phases, file checklist) mostly matches what
exists on disk; where the two differ, the actual code and `README.md` are authoritative.

---

## 1. Overview

The graphrag-service is a Python worker that:
- Consumes document processing messages from RabbitMQ
- Downloads documents from MinIO
- Runs GraphRAG pipelines (ingestion + knowledge graph construction)
- Stores results in MongoDB Atlas

### Why Python
- GraphRAG library is Python-based (576 files)
- Rich ML/AI ecosystem (LangChain, OpenAI, etc.)
- Async support (`async def`) matches GraphRAG pipeline design
- Faster implementation by wrapping existing code

### Key Decision: Worker-Only (Phase 1)
Start as a pure message consumer without API endpoints. Decision on adding query APIs deferred until after basic processing works.

---

## 2. Source References

| Component | Source Location | Notes |
|-----------|-----------------|-------|
| GraphRAG Core | `legacy_project/GraphRAG/` | External reference project |
| Worker Pattern | `legacy_project/services/worker-service/` | Go worker (for pattern reference) |

---

## 3. Implementation Tasks

### Phase 1: Project Setup (Day 1)

#### Task 1.1: Create Directory Structure

```bash
mkdir -p graph-worker/graphrag-service/{cmd,src/{worker,graphrag,config,monitoring},tests,deployments/kubernetes}
```

**Final structure:**
```
graphrag-service/
├── cmd/
│   └── main.py                   # Entry point
├── src/
│   ├── graphrag/                 # Copied from legacy_project/GraphRAG/src/
│   │   ├── domain/
│   │   ├── core/
│   │   ├── infrastructure/
│   │   ├── app/
│   │   └── lib/
│   ├── worker/
│   │   ├── __init__.py
│   │   ├── consumer.py          # Async RabbitMQ consumer
│   │   ├── processor.py         # Document processor
│   │   └── base_worker.py       # Worker lifecycle
│   ├── config/
│   │   ├── __init__.py
│   │   └── worker_config.py     # Environment configuration
│   └── monitoring/
│       ├── __init__.py
│       ├── health.py            # Flask health server
│       └── metrics.py           # Prometheus metrics
├── tests/
│   └── ...
├── deployments/
│   └── kubernetes/
│       ├── deployment.yaml
│       ├── service.yaml
│       ├── configmap.yaml
│       └── secret.yaml
├── requirements.txt
├── requirements-graphrag.txt
├── Dockerfile
├── IMPLEMENTATION_PLAN.md
└── README.md
```

#### Task 1.2: Copy GraphRAG Core

**Source:** `legacy_project/GraphRAG/src/`

```bash
# From repository root
mkdir -p graph-worker/graphrag-service/src/graphrag

cp -r legacy_project/GraphRAG/src/domain graph-worker/graphrag-service/src/graphrag/
cp -r legacy_project/GraphRAG/src/core graph-worker/graphrag-service/src/graphrag/
cp -r legacy_project/GraphRAG/src/infrastructure graph-worker/graphrag-service/src/graphrag/
cp -r legacy_project/GraphRAG/src/app graph-worker/graphrag-service/src/graphrag/
cp -r legacy_project/GraphRAG/src/lib graph-worker/graphrag-service/src/graphrag/
cp legacy_project/GraphRAG/requirements.txt graph-worker/graphrag-service/requirements-graphrag.txt
```

#### Task 1.3: Create Requirements File

**File:** `requirements.txt`

```txt
# Include GraphRAG dependencies
-r requirements-graphrag.txt

# Worker dependencies
aio-pika>=9.4.0              # Async RabbitMQ client
prometheus-client>=0.20.0    # Prometheus metrics
flask>=3.0.0                 # Health check server

# MinIO client
minio>=7.2.0

# Additional utilities
python-json-logger>=2.0.0
pydantic>=2.0.0
pydantic-settings>=2.0.0
```

#### Task 1.4: Verify GraphRAG Imports

```bash
cd graph-worker/graphrag-service
python -c "from src.graphrag.domain.graphrag.pipeline import GraphRAGPipeline; print('✅ GraphRAG OK')"
python -c "from src.graphrag.domain.ingestion.pipeline import IngestionPipeline; print('✅ Ingestion OK')"
```

---

### Phase 2: RabbitMQ Consumer (Days 2-3)

#### Task 2.1: Create Async Consumer

**File:** `src/worker/consumer.py`

> **CRITICAL:** Use `aio-pika` because GraphRAG pipelines are async (`async def run()`)

```python
import aio_pika
import asyncio
import json
import logging
from typing import Callable, Awaitable, Optional

logger = logging.getLogger(__name__)


class AsyncRabbitMQConsumer:
    """Async RabbitMQ consumer for GraphRAG worker using aio-pika."""

    def __init__(self, config: dict):
        self.config = config
        self.connection: Optional[aio_pika.RobustConnection] = None
        self.channel: Optional[aio_pika.Channel] = None
        self._shutdown = False

    async def connect(self) -> aio_pika.Queue:
        """Establish async connection to RabbitMQ."""
        self.connection = await aio_pika.connect_robust(
            host=self.config['host'],
            port=self.config['port'],
            login=self.config['username'],
            password=self.config['password'],
            virtualhost=self.config.get('vhost', '/')
        )

        self.channel = await self.connection.channel()
        await self.channel.set_qos(prefetch_count=self.config.get('prefetch_count', 1))

        # Declare exchange
        exchange = await self.channel.declare_exchange(
            self.config.get('exchange', 'document-tasks'),
            aio_pika.ExchangeType.DIRECT,
            durable=True
        )

        # Declare queue with DLQ
        queue = await self.channel.declare_queue(
            self.config.get('queue', 'document-processing'),
            durable=True,
            arguments={
                'x-message-ttl': 43200000,  # 12 hours
                'x-dead-letter-exchange': f"{self.config.get('exchange', 'document-tasks')}.dlx"
            }
        )

        # Bind queue to exchange
        await queue.bind(
            exchange,
            routing_key=self.config.get('routing_key', 'document.process')
        )

        logger.info(f"Connected to RabbitMQ, consuming from {queue.name}")
        return queue

    async def consume(self, handler: Callable[[dict], Awaitable[None]]) -> None:
        """Start consuming messages with async handler."""
        queue = await self.connect()

        async with queue.iterator() as queue_iter:
            async for message in queue_iter:
                if self._shutdown:
                    break

                async with message.process():
                    try:
                        payload = json.loads(message.body.decode())
                        logger.info(f"Received message: {payload.get('id')}")
                        await handler(payload)
                        logger.info(f"Processed message: {payload.get('id')}")
                    except json.JSONDecodeError as e:
                        logger.error(f"Invalid JSON: {e}")
                        # Don't requeue - message is malformed
                    except Exception as e:
                        logger.error(f"Processing error: {e}")
                        raise  # Will be requeued by aio-pika

    async def close(self) -> None:
        """Close connection gracefully."""
        self._shutdown = True
        if self.connection:
            await self.connection.close()
            logger.info("RabbitMQ connection closed")
```

#### Task 2.2: Create Document Processor

**File:** `src/worker/processor.py`

> **CRITICAL:** GraphRAG pipelines are async - use `async def` and `await`

```python
import asyncio
import logging
from typing import Dict, Any, Optional
from minio import Minio
import tempfile
import os

# Import GraphRAG pipelines
from src.graphrag.domain.ingestion.pipeline import IngestionPipeline
from src.graphrag.domain.graphrag.pipeline import GraphRAGPipeline

logger = logging.getLogger(__name__)


class DocumentProcessor:
    """Async processor for documents through GraphRAG pipelines."""

    def __init__(self, config: dict):
        self.config = config
        self.minio_client = self._init_minio()

    def _init_minio(self) -> Minio:
        """Initialize MinIO client."""
        return Minio(
            self.config['minio']['endpoint'],
            access_key=self.config['minio']['access_key'],
            secret_key=self.config['minio']['secret_key'],
            secure=self.config['minio'].get('use_ssl', False)
        )

    def validate(self, message: dict) -> bool:
        """Validate message structure."""
        required = ['id', 'type', 'payload', 'timestamp']
        if not all(field in message for field in required):
            logger.error(f"Missing required fields: {required}")
            return False

        payload = message.get('payload', {})
        payload_required = ['document_id', 'storage_path', 'storage_bucket']
        if not all(f in payload for f in payload_required):
            logger.error(f"Missing payload fields: {payload_required}")
            return False

        return True

    async def process(self, message: dict) -> Dict[str, Any]:
        """Process document message (ASYNC)."""
        payload = message['payload']
        document_id = payload['document_id']
        storage_path = payload['storage_path']
        storage_bucket = payload['storage_bucket']
        user_id = payload.get('user_id')

        logger.info(f"Processing document: {document_id}")

        # Download document from MinIO
        local_path = await self._download_document(storage_bucket, storage_path)

        try:
            # Build configs
            ingest_config = self._build_ingest_config(local_path, payload)
            graphrag_config = self._build_graphrag_config(user_id, document_id)

            # Run ingestion pipeline (ASYNC)
            logger.info(f"Starting ingestion for {document_id}")
            ingest_pipeline = IngestionPipeline(ingest_config)
            ingest_result = await ingest_pipeline.run()
            logger.info(f"Ingestion complete: {ingest_result.chunks_count} chunks")

            # Run GraphRAG pipeline (ASYNC)
            logger.info(f"Starting GraphRAG for {document_id}")
            graphrag_pipeline = GraphRAGPipeline(graphrag_config)
            graphrag_result = await graphrag_pipeline.run()
            logger.info(f"GraphRAG complete: {graphrag_result.entities_count} entities")

            return {
                'status': 'completed',
                'document_id': document_id,
                'chunks_count': ingest_result.chunks_count,
                'entities_count': graphrag_result.entities_count,
                'relationships_count': graphrag_result.relationships_count,
                'communities_count': getattr(graphrag_result, 'communities_count', 0)
            }

        finally:
            # Cleanup temporary file
            if os.path.exists(local_path):
                os.remove(local_path)

    async def _download_document(self, bucket: str, path: str) -> str:
        """Download document from MinIO to temp file."""
        # Run sync MinIO operation in thread pool
        loop = asyncio.get_event_loop()

        def download():
            temp_file = tempfile.NamedTemporaryFile(delete=False, suffix=os.path.splitext(path)[1])
            self.minio_client.fget_object(bucket, path, temp_file.name)
            return temp_file.name

        return await loop.run_in_executor(None, download)

    def _build_ingest_config(self, local_path: str, payload: dict) -> dict:
        """Build ingestion pipeline configuration."""
        return {
            'document_path': local_path,
            'document_type': payload.get('file_type', 'pdf'),
            'chunk_size': self.config.get('chunk_size', 1000),
            'chunk_overlap': self.config.get('chunk_overlap', 200),
            **self.config.get('ingestion', {})
        }

    def _build_graphrag_config(self, user_id: str, document_id: str) -> dict:
        """Build GraphRAG pipeline configuration."""
        return {
            'user_id': user_id,
            'document_id': document_id,
            'mongodb_uri': self.config['mongodb']['uri'],
            'mongodb_database': self.config['mongodb']['database'],
            'openai_api_key': self.config['openai']['api_key'],
            **self.config.get('graphrag', {})
        }
```

#### Task 2.3: Create Base Worker

**File:** `src/worker/base_worker.py`

```python
import signal
import asyncio
import logging
from typing import Optional

from src.worker.consumer import AsyncRabbitMQConsumer
from src.worker.processor import DocumentProcessor
from src.monitoring.health import start_health_server
from src.monitoring.metrics import PrometheusMetrics

logger = logging.getLogger(__name__)


class BaseWorker:
    """Base worker with async RabbitMQ consumer, health, and metrics."""

    def __init__(self, config: dict):
        self.config = config
        self.consumer = AsyncRabbitMQConsumer(config['rabbitmq'])
        self.processor = DocumentProcessor(config)
        self.metrics = PrometheusMetrics()
        self._shutdown_event = asyncio.Event()

    async def start(self) -> None:
        """Start worker (async)."""
        # Setup signal handlers
        loop = asyncio.get_event_loop()
        for sig in (signal.SIGTERM, signal.SIGINT):
            loop.add_signal_handler(sig, self._handle_shutdown)

        # Start health server (runs in separate thread)
        start_health_server(
            port=self.config.get('health_port', 8080),
            metrics_port=self.config.get('metrics_port', 8081)
        )

        logger.info("Worker starting...")

        # Consume messages
        try:
            await self.consumer.consume(self._handle_message)
        except asyncio.CancelledError:
            logger.info("Worker cancelled")
        finally:
            await self.consumer.close()

    async def _handle_message(self, message: dict) -> None:
        """Handle incoming message."""
        message_id = message.get('id', 'unknown')

        # Validate message
        if not self.processor.validate(message):
            self.metrics.record_error('validation')
            logger.error(f"Invalid message: {message_id}")
            return

        # Process message
        with self.metrics.track_duration():
            try:
                result = await self.processor.process(message)
                self.metrics.record_success()
                logger.info(f"Message processed: {message_id} - {result}")
            except Exception as e:
                self.metrics.record_error('processing')
                logger.error(f"Processing failed: {message_id} - {e}")
                raise  # Requeue message

    def _handle_shutdown(self) -> None:
        """Handle graceful shutdown."""
        logger.info("Shutdown signal received")
        self._shutdown_event.set()
        # Cancel the main task
        for task in asyncio.all_tasks():
            if task is not asyncio.current_task():
                task.cancel()
```

---

### Phase 3: Monitoring (Day 4)

#### Task 3.1: Create Health Server

**File:** `src/monitoring/health.py`

```python
from flask import Flask, jsonify
import threading
import logging

logger = logging.getLogger(__name__)

app = Flask(__name__)

# Health state (set by worker)
_ready = False
_healthy = True


def set_ready(ready: bool) -> None:
    global _ready
    _ready = ready


def set_healthy(healthy: bool) -> None:
    global _healthy
    _healthy = healthy


@app.route('/health')
def health():
    """Liveness probe."""
    if _healthy:
        return jsonify({'status': 'ok'}), 200
    return jsonify({'status': 'unhealthy'}), 503


@app.route('/ready')
def ready():
    """Readiness probe."""
    if _ready:
        return jsonify({'status': 'ready'}), 200
    return jsonify({'status': 'not ready'}), 503


@app.route('/live')
def live():
    """Alias for /health."""
    return health()


def start_health_server(port: int = 8080, metrics_port: int = 8081) -> None:
    """Start health server in background thread."""
    def run():
        # Disable Flask logs
        import logging as flask_logging
        flask_logging.getLogger('werkzeug').setLevel(flask_logging.ERROR)
        app.run(host='0.0.0.0', port=port, debug=False, use_reloader=False)

    thread = threading.Thread(target=run, daemon=True)
    thread.start()
    logger.info(f"Health server started on port {port}")

    # Start metrics server
    from prometheus_client import start_http_server
    start_http_server(metrics_port)
    logger.info(f"Metrics server started on port {metrics_port}")
```

#### Task 3.2: Create Prometheus Metrics

**File:** `src/monitoring/metrics.py`

```python
from prometheus_client import Counter, Histogram, Gauge
from contextlib import contextmanager
import time


class PrometheusMetrics:
    """Prometheus metrics for GraphRAG worker."""

    def __init__(self):
        # Counters
        self.messages_processed = Counter(
            'graphrag_messages_processed_total',
            'Total messages processed',
            ['status']
        )

        self.documents_processed = Counter(
            'graphrag_documents_processed_total',
            'Total documents processed'
        )

        # Histograms
        self.processing_duration = Histogram(
            'graphrag_processing_duration_seconds',
            'Document processing duration',
            buckets=[60, 120, 300, 600, 900, 1200, 1800, 3600]  # Up to 1 hour
        )

        # Gauges
        self.current_processing = Gauge(
            'graphrag_currently_processing',
            'Number of documents currently being processed'
        )

        self.last_success_timestamp = Gauge(
            'graphrag_last_success_timestamp',
            'Timestamp of last successful processing'
        )

    def record_success(self) -> None:
        """Record successful processing."""
        self.messages_processed.labels(status='success').inc()
        self.documents_processed.inc()
        self.last_success_timestamp.set_to_current_time()

    def record_error(self, error_type: str) -> None:
        """Record processing error."""
        self.messages_processed.labels(status=f'error_{error_type}').inc()

    @contextmanager
    def track_duration(self):
        """Context manager to track processing duration."""
        self.current_processing.inc()
        start = time.time()
        try:
            yield
        finally:
            duration = time.time() - start
            self.processing_duration.observe(duration)
            self.current_processing.dec()
```

---

### Phase 4: Configuration (Day 4)

#### Task 4.1: Create Configuration Module

**File:** `src/config/worker_config.py`

```python
import os
from typing import Dict, Any
from pydantic_settings import BaseSettings
from pydantic import Field


class Settings(BaseSettings):
    """Worker configuration from environment variables."""

    # RabbitMQ
    rabbitmq_host: str = Field(default='rabbitmq', alias='RABBITMQ_HOST')
    rabbitmq_port: int = Field(default=5672, alias='RABBITMQ_PORT')
    rabbitmq_user: str = Field(default='guest', alias='RABBITMQ_USER')
    rabbitmq_password: str = Field(default='guest', alias='RABBITMQ_PASSWORD')
    rabbitmq_vhost: str = Field(default='/', alias='RABBITMQ_VHOST')

    # MongoDB
    mongodb_uri: str = Field(..., alias='MONGODB_URI')
    mongodb_database: str = Field(default='graphrag', alias='MONGODB_DATABASE')

    # OpenAI
    openai_api_key: str = Field(..., alias='OPENAI_API_KEY')

    # MinIO
    minio_endpoint: str = Field(default='minio:9000', alias='MINIO_ENDPOINT')
    minio_access_key: str = Field(..., alias='MINIO_ACCESS_KEY')
    minio_secret_key: str = Field(..., alias='MINIO_SECRET_KEY')
    minio_use_ssl: bool = Field(default=False, alias='MINIO_USE_SSL')

    # Worker
    health_port: int = Field(default=8080, alias='HEALTH_PORT')
    metrics_port: int = Field(default=8081, alias='METRICS_PORT')
    log_level: str = Field(default='INFO', alias='LOG_LEVEL')

    class Config:
        env_file = '.env'
        case_sensitive = False


def load_config() -> Dict[str, Any]:
    """Load configuration from environment."""
    settings = Settings()

    return {
        'rabbitmq': {
            'host': settings.rabbitmq_host,
            'port': settings.rabbitmq_port,
            'username': settings.rabbitmq_user,
            'password': settings.rabbitmq_password,
            'vhost': settings.rabbitmq_vhost,
            'exchange': 'document-tasks',
            'queue': 'document-processing',
            'routing_key': 'document.process',
            'prefetch_count': 1
        },
        'mongodb': {
            'uri': settings.mongodb_uri,
            'database': settings.mongodb_database
        },
        'openai': {
            'api_key': settings.openai_api_key
        },
        'minio': {
            'endpoint': settings.minio_endpoint,
            'access_key': settings.minio_access_key,
            'secret_key': settings.minio_secret_key,
            'use_ssl': settings.minio_use_ssl
        },
        'health_port': settings.health_port,
        'metrics_port': settings.metrics_port,
        'log_level': settings.log_level
    }
```

---

### Phase 5: Entry Point (Day 5)

#### Task 5.1: Create Main Entry Point

**File:** `cmd/main.py`

```python
#!/usr/bin/env python3
"""GraphRAG Worker - Document processing service."""

import asyncio
import logging
import sys
from pathlib import Path

# Add src to path
sys.path.insert(0, str(Path(__file__).parent.parent / 'src'))

from config.worker_config import load_config
from worker.base_worker import BaseWorker
from monitoring.health import set_ready, set_healthy


def setup_logging(level: str) -> None:
    """Configure logging."""
    logging.basicConfig(
        level=getattr(logging, level.upper()),
        format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
        handlers=[logging.StreamHandler()]
    )


async def main() -> None:
    """Main entry point."""
    # Load configuration
    config = load_config()

    # Setup logging
    setup_logging(config.get('log_level', 'INFO'))

    logger = logging.getLogger(__name__)
    logger.info("Starting GraphRAG Worker...")
    logger.info(f"RabbitMQ: {config['rabbitmq']['host']}:{config['rabbitmq']['port']}")
    logger.info(f"Queue: {config['rabbitmq']['queue']}")
    logger.info(f"MongoDB: {config['mongodb']['database']}")

    # Create and start worker
    worker = BaseWorker(config)

    # Mark as ready after initialization
    set_ready(True)
    set_healthy(True)

    try:
        await worker.start()
    except KeyboardInterrupt:
        logger.info("Interrupted")
    except Exception as e:
        logger.error(f"Worker error: {e}")
        set_healthy(False)
        raise
    finally:
        set_ready(False)
        logger.info("Worker stopped")


if __name__ == '__main__':
    asyncio.run(main())
```

---

### Phase 6: Dockerfile (Day 5)

**File:** `Dockerfile`

```dockerfile
# Multi-stage build for Python GraphRAG worker

FROM python:3.11-slim as builder

WORKDIR /app

# Install build dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

# Copy requirements and install
COPY requirements.txt requirements-graphrag.txt ./
RUN pip install --user --no-cache-dir -r requirements.txt

# Production stage
FROM python:3.11-slim

WORKDIR /app

# Copy installed packages from builder
COPY --from=builder /root/.local /root/.local

# Copy application code
COPY cmd/ ./cmd/
COPY src/ ./src/

# Set environment
ENV PATH=/root/.local/bin:$PATH
ENV PYTHONUNBUFFERED=1
ENV PYTHONDONTWRITEBYTECODE=1

# Health and metrics ports
EXPOSE 8080 8081

# Run worker
CMD ["python", "cmd/main.py"]
```

---

### Phase 7: Kubernetes Deployment (Day 6)

#### Task 7.1: Deployment Manifest

**File:** `deployments/kubernetes/deployment.yaml`

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: graphrag-service
  labels:
    app: graphrag-service
spec:
  replicas: 1  # Memory-intensive, scale carefully
  selector:
    matchLabels:
      app: graphrag-service
  template:
    metadata:
      labels:
        app: graphrag-service
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8081"
        prometheus.io/path: "/metrics"
    spec:
      containers:
        - name: graphrag-service
          image: graphrag-service:latest
          imagePullPolicy: IfNotPresent
          ports:
            - containerPort: 8080
              name: health
            - containerPort: 8081
              name: metrics
          env:
            - name: RABBITMQ_HOST
              value: "rabbitmq"
            - name: RABBITMQ_PORT
              value: "5672"
            - name: RABBITMQ_USER
              valueFrom:
                secretKeyRef:
                  name: rabbitmq-secret
                  key: RABBITMQ_USER
            - name: RABBITMQ_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: rabbitmq-secret
                  key: RABBITMQ_PASSWORD
            - name: MONGODB_URI
              valueFrom:
                secretKeyRef:
                  name: mongodb-atlas-secret
                  key: MONGODB_URI
            - name: MONGODB_DATABASE
              value: "graphrag"
            - name: OPENAI_API_KEY
              valueFrom:
                secretKeyRef:
                  name: graphrag-secret
                  key: OPENAI_API_KEY
            - name: MINIO_ENDPOINT
              value: "minio:9000"
            - name: MINIO_ACCESS_KEY
              valueFrom:
                secretKeyRef:
                  name: minio-secret
                  key: MINIO_ACCESS_KEY
            - name: MINIO_SECRET_KEY
              valueFrom:
                secretKeyRef:
                  name: minio-secret
                  key: MINIO_SECRET_KEY
            - name: LOG_LEVEL
              value: "INFO"
          resources:
            requests:
              cpu: 2000m
              memory: 6Gi
            limits:
              cpu: 4000m
              memory: 10Gi
          livenessProbe:
            httpGet:
              path: /health
              port: health
            initialDelaySeconds: 60
            periodSeconds: 30
            timeoutSeconds: 10
          readinessProbe:
            httpGet:
              path: /ready
              port: health
            initialDelaySeconds: 60
            periodSeconds: 30
            timeoutSeconds: 10
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
```

#### Task 7.2: Service Manifest

**File:** `deployments/kubernetes/service.yaml`

```yaml
apiVersion: v1
kind: Service
metadata:
  name: graphrag-service
  labels:
    app: graphrag-service
spec:
  type: ClusterIP
  ports:
    - name: health
      port: 8080
      targetPort: health
    - name: metrics
      port: 8081
      targetPort: metrics
  selector:
    app: graphrag-service
```

---

## 4. Environment Variables

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `RABBITMQ_HOST` | rabbitmq | No | RabbitMQ host |
| `RABBITMQ_PORT` | 5672 | No | RabbitMQ port |
| `RABBITMQ_USER` | guest | No | RabbitMQ user |
| `RABBITMQ_PASSWORD` | - | **Yes** | RabbitMQ password |
| `MONGODB_URI` | - | **Yes** | MongoDB Atlas connection string |
| `MONGODB_DATABASE` | graphrag | No | MongoDB database name |
| `OPENAI_API_KEY` | - | **Yes** | OpenAI API key |
| `MINIO_ENDPOINT` | minio:9000 | No | MinIO endpoint |
| `MINIO_ACCESS_KEY` | - | **Yes** | MinIO access key |
| `MINIO_SECRET_KEY` | - | **Yes** | MinIO secret key |
| `HEALTH_PORT` | 8080 | No | Health check port |
| `METRICS_PORT` | 8081 | No | Prometheus metrics port |
| `LOG_LEVEL` | INFO | No | Log level |

---

## 5. File Checklist

### New Files to Create
- [ ] `cmd/main.py`
- [ ] `src/worker/__init__.py`
- [ ] `src/worker/consumer.py`
- [ ] `src/worker/processor.py`
- [ ] `src/worker/base_worker.py`
- [ ] `src/config/__init__.py`
- [ ] `src/config/worker_config.py`
- [ ] `src/monitoring/__init__.py`
- [ ] `src/monitoring/health.py`
- [ ] `src/monitoring/metrics.py`
- [ ] `requirements.txt`
- [ ] `Dockerfile`
- [ ] `deployments/kubernetes/deployment.yaml`
- [ ] `deployments/kubernetes/service.yaml`
- [ ] `deployments/kubernetes/configmap.yaml`
- [ ] `deployments/kubernetes/secret.yaml`
- [ ] `README.md`

### Files to Copy
- [ ] `legacy_project/GraphRAG/src/domain/` → `src/graphrag/domain/`
- [ ] `legacy_project/GraphRAG/src/core/` → `src/graphrag/core/`
- [ ] `legacy_project/GraphRAG/src/infrastructure/` → `src/graphrag/infrastructure/`
- [ ] `legacy_project/GraphRAG/src/app/` → `src/graphrag/app/`
- [ ] `legacy_project/GraphRAG/src/lib/` → `src/graphrag/lib/`
- [ ] `legacy_project/GraphRAG/requirements.txt` → `requirements-graphrag.txt`

---

## 6. Testing Checklist

### Unit Tests
- [ ] Consumer connection and message handling
- [ ] Processor validation logic
- [ ] Configuration loading

### Integration Tests
- [ ] Consume message from RabbitMQ
- [ ] Download document from MinIO
- [ ] Process through ingestion pipeline
- [ ] Store results in MongoDB

### Manual Tests
```bash
# Start local infrastructure
docker-compose up -d rabbitmq mongodb minio

# Run worker
cd graphrag-service
python cmd/main.py

# Publish test message
python tests/publish_test_message.py

# Verify:
# - Logs show processing
# - MongoDB has entities/relationships
# - Metrics endpoint shows counts
```

---

## 7. Dependencies on Other Components

| Component | Dependency | Notes |
|-----------|------------|-------|
| RabbitMQ | Required | Must be deployed first |
| MinIO | Required | Document storage |
| MongoDB Atlas | Required | Knowledge graph storage |
| api-service | Upstream | Publishes document.process messages |

---

## 8. Success Criteria

- [ ] Worker starts and connects to RabbitMQ
- [ ] Health check returns 200
- [ ] Metrics endpoint exposes Prometheus metrics
- [ ] Message consumed from queue
- [ ] Document downloaded from MinIO
- [ ] GraphRAG pipelines execute successfully
- [ ] Results stored in MongoDB Atlas
- [ ] Processing time < 30 min for typical document
- [ ] Memory usage stays under 8Gi

---

## 9. Deferred Decision Reminder

**GraphRAG Service Role** - Decide after basic processing works:
- **Option A:** Worker only (current plan)
- **Option B:** Worker + Query API
- **Option C:** Separate worker and query services

See `MASTER_IMPLEMENTATION_PLAN.md` Section 7.1, Q2 for rationale.

---

*Document Version: 1.0*  
*Created: January 2026*  
*Estimated Effort: 6-8 days*
