# BRAINSTORM: Graph Worker Architecture - Python + Go Integration

## Executive Summary

This document explores the architecture for a **graph-worker** system that combines Python-based GraphRAG knowledge graph construction with Go-based task workers, all consuming from RabbitMQ queues populated by the api-service gateway.

---

## 🎯 Project Goals

1. **GraphRAG Worker (Python)**: Process documents through LLM pipelines to build knowledge graphs
2. **Task Workers (Go)**: Handle operational tasks (email, image processing, etc.)
3. **RabbitMQ Integration**: Both consume from api-service via message queues
4. **Crossover Opportunities**: Identify shared patterns and infrastructure
5. **Kubernetes Deployment**: Run both in the same cluster with appropriate resource allocation

---

## 📊 Current State Analysis

### Source Locations (Verified)

```
/microservices/
├── api-service/                              ✅ Consolidated Go API (DONE)
├── legacy_project/GraphRAG/                  ✅ GraphRAG in legacy (external reference)
│   └── src/
│       ├── domain/{ingestion,graphrag}/      ← Pipelines (ASYNC)
│       ├── core/{base,config,models}/
│       ├── infrastructure/{database,llm}/
│       └── app/api/{graph,stages}/
├── legacy_project/
│   └── services/
│       ├── worker-service/                   ✅ Source for operational-workers
│       │   └── services/workers/{common,email-worker,image-worker}/
│       └── common/queue/                     ✅ RabbitMQ consumer
└── graph-worker/                             ✅ New worker projects
```

### GraphRAG Characteristics

| Aspect | Details |
|--------|---------|
| **Language** | Python 3.11+ |
| **Source Location** | `/microservices/legacy_project/GraphRAG/` (external reference) |
| **Primary Function** | Knowledge graph construction from documents |
| **Processing Type** | Long-running (15-45 min per document), CPU/memory intensive |
| **Pipeline API** | **ASYNC** - `await pipeline.run()` |
| **LLM Integration** | OpenAI, AWS Bedrock (100-300 concurrent calls) |
| **Storage** | MongoDB Atlas (entities, relationships, communities) + embeddings |
| **APIs** | Graph API (8081), Stages API (8080) - FastAPI |
| **Frameworks** | Langchain, NetworkX, FastAPI |
| **Concurrency** | Async/await with rate limiting, worker pools |

### Worker-Service Characteristics

| Aspect | Details |
|--------|---------|
| **Language** | Go 1.22+ |
| **Source Location** | `/microservices/legacy_project/services/worker-service/` |
| **Primary Function** | Operational task processing (email, image) |
| **Processing Type** | Short-lived (2-30s per task), I/O or CPU bound |
| **Integrations** | Mock email/image services |
| **Storage** | None (stateless workers) |
| **APIs** | Health checks only (8080) |
| **Frameworks** | Gin, go-redis, prometheus |
| **Concurrency** | Goroutines with configurable prefetch |
| **Common Package** | `legacy_project/services/common/queue/` (to be copied) |

### Crossover Analysis

| Feature | GraphRAG | Go Workers | Crossover Opportunity |
|---------|----------|------------|----------------------|
| **RabbitMQ Consumer** | Not yet implemented | ✅ Fully implemented | **HIGH** - Python needs queue consumer |
| **Message Validation** | N/A | ✅ Per-worker | **MEDIUM** - Python needs similar patterns |
| **Metrics Collection** | ✅ Prometheus | ✅ Prometheus | **HIGH** - Shared metric patterns |
| **Health Checks** | ✅ REST APIs | ✅ REST APIs | **HIGH** - Same pattern works |
| **Error Handling** | ✅ Retry + backoff | ✅ Retry + DLQ | **HIGH** - Align strategies |
| **Configuration** | ✅ Pydantic + env | ✅ Env vars | **MEDIUM** - Different but similar |
| **Graceful Shutdown** | Partial | ✅ Signal handling | **MEDIUM** - Python needs improvement |
| **Rate Limiting** | ✅ Sophisticated (TPM/RPM) | Not needed | **LOW** - GraphRAG-specific |
| **Circuit Breakers** | Not implemented | Not implemented | **LOW** - Both may need for external services |

---

## 🏗️ Architecture Options

### Option 1: Parallel Projects with Shared Infrastructure

```
graph-worker/
├── graphrag-worker/                    # Python project
│   ├── cmd/
│   │   └── main.py                    # Worker entry point
│   ├── src/
│   │   ├── consumer/                  # RabbitMQ consumer (NEW)
│   │   │   ├── client.py
│   │   │   └── handler.py
│   │   ├── graphrag/                  # Copy from reference-materials
│   │   │   ├── domain/
│   │   │   ├── core/
│   │   │   └── infrastructure/
│   │   ├── processors/                # Bridge consumer → GraphRAG
│   │   │   └── document_processor.py
│   │   └── config/
│   │       └── config.py
│   ├── requirements.txt
│   ├── Dockerfile
│   └── deployments/kubernetes/
│
├── task-worker/                       # Go project
│   ├── cmd/
│   │   ├── email-worker/main.go
│   │   ├── image-worker/main.go
│   │   └── profile-worker/main.go
│   ├── internal/
│   │   ├── common/                    # Shared base
│   │   │   ├── worker/
│   │   │   ├── processor/
│   │   │   └── metrics/
│   │   ├── processors/
│   │   │   ├── email/
│   │   │   ├── image/
│   │   │   └── profile/
│   │   └── domain/
│   │       └── message.go
│   ├── go.mod
│   ├── Dockerfile.email
│   ├── Dockerfile.image
│   └── deployments/kubernetes/
│
└── shared/                            # Shared between Python and Go
    ├── docker-compose.yaml           # Local dev environment
    ├── k8s/
    │   └── rabbitmq-config.yaml      # Shared queue definitions
    └── docs/
        ├── MESSAGE_CONTRACT.md        # Shared message format
        └── ARCHITECTURE.md
```

**Pros:**
- ✅ Clear separation: Python for AI/graph, Go for operational tasks
- ✅ Independent deployment and scaling
- ✅ Language-appropriate implementations
- ✅ Easy to understand and maintain

**Cons:**
- ❌ Duplicate RabbitMQ consumer logic (Python vs Go)
- ❌ Need to maintain consistency across languages
- ❌ More complex deployment (2 sets of manifests)

---

### Option 2: Unified Worker Framework with Language-Specific Processors

```
graph-worker/
├── cmd/
│   ├── graphrag/main.py              # Python worker entry
│   ├── email/main.go                 # Go email worker
│   ├── image/main.go                 # Go image worker
│   └── profile/main.go               # Go profile worker
│
├── framework/                         # Shared worker framework
│   ├── rabbitmq/                     # Language-agnostic definitions
│   │   ├── exchanges.yaml
│   │   ├── queues.yaml
│   │   └── routing.yaml
│   ├── metrics/
│   │   ├── prometheus_py.py
│   │   └── prometheus_go.go
│   └── contracts/
│       └── message.proto             # Protobuf message definitions
│
├── python/                           # Python workers
│   ├── src/
│   │   ├── framework/                # Python worker framework
│   │   │   ├── base_worker.py
│   │   │   ├── consumer.py
│   │   │   └── processor.py
│   │   ├── graphrag/                 # GraphRAG implementation
│   │   └── processors/
│   │       └── document_processor.py
│   └── requirements.txt
│
├── go/                               # Go workers
│   ├── internal/
│   │   ├── framework/                # Go worker framework
│   │   │   ├── base_worker.go
│   │   │   ├── consumer.go
│   │   │   └── processor.go
│   │   └── processors/
│   │       ├── email/
│   │       ├── image/
│   │       └── profile/
│   └── go.mod
│
└── deployments/
    └── kubernetes/
        ├── graphrag-worker.yaml
        ├── email-worker.yaml
        ├── image-worker.yaml
        └── profile-worker.yaml
```

**Pros:**
- ✅ Unified framework concepts across languages
- ✅ Single deployment location
- ✅ Shared contracts and configuration
- ✅ Easier to enforce consistency

**Cons:**
- ❌ More complex structure
- ❌ Framework code duplication (Python and Go versions)
- ❌ Harder to navigate

---

### Option 3: Monorepo with Independent Projects (RECOMMENDED)

```
graph-worker/
│
├── graphrag-service/                  # Python: GraphRAG processing
│   ├── cmd/
│   │   └── main.py                   # Worker entry point
│   ├── src/
│   │   ├── worker/                   # RabbitMQ consumer (NEW)
│   │   │   ├── __init__.py
│   │   │   ├── consumer.py          # Pika consumer
│   │   │   ├── processor.py         # GraphRAG processor
│   │   │   └── handler.py           # Message handler
│   │   ├── graphrag/                 # Core GraphRAG (from reference)
│   │   │   ├── app/                  # APIs (keep for debugging)
│   │   │   ├── src/
│   │   │   │   ├── domain/          # Agents, Stages, Pipelines
│   │   │   │   ├── core/            # Base classes, config
│   │   │   │   └── infrastructure/   # MongoDB, LLM providers
│   │   │   └── cli/                  # CLI tools (keep for manual runs)
│   │   ├── config/
│   │   │   └── worker_config.py     # Worker-specific config
│   │   └── monitoring/
│   │       ├── metrics.py            # Prometheus
│   │       └── health.py             # Health check server
│   ├── tests/
│   ├── requirements.txt
│   ├── Dockerfile
│   ├── deployments/
│   │   └── kubernetes/
│   │       ├── deployment.yaml
│   │       ├── service.yaml
│   │       └── configmap.yaml
│   └── README.md
│
├── operational-workers/               # Go: Task processing
│   ├── cmd/
│   │   ├── email-worker/
│   │   │   └── main.go
│   │   ├── image-worker/
│   │   │   └── main.go
│   │   └── profile-worker/
│   │       └── main.go
│   ├── internal/
│   │   ├── common/                   # Shared worker framework
│   │   │   ├── base/
│   │   │   │   ├── worker.go        # BaseWorker
│   │   │   │   └── server.go        # HTTP health server
│   │   │   ├── processor/
│   │   │   │   └── interface.go     # MessageProcessor
│   │   │   └── metrics/
│   │   │       └── metrics.go       # Prometheus metrics
│   │   ├── processors/
│   │   │   ├── email/
│   │   │   │   ├── processor.go
│   │   │   │   └── message.go
│   │   │   ├── image/
│   │   │   │   ├── processor.go
│   │   │   │   └── message.go
│   │   │   └── profile/
│   │   │       ├── processor.go
│   │   │       └── message.go
│   │   └── domain/
│   │       └── message.go           # Common message types
│   ├── go.mod
│   ├── Dockerfile.email
│   ├── Dockerfile.image
│   ├── Dockerfile.profile
│   ├── deployments/
│   │   └── kubernetes/
│   │       ├── email-worker.yaml
│   │       ├── image-worker.yaml
│   │       └── profile-worker.yaml
│   └── README.md
│
├── shared/                            # Shared resources
│   ├── contracts/
│   │   ├── MESSAGE_FORMAT.md        # Standard message format
│   │   └── ROUTING_KEYS.md          # Routing key conventions
│   ├── configs/
│   │   └── rabbitmq/
│   │       ├── exchanges.yaml
│   │       └── queues.yaml
│   └── local-dev/
│       ├── docker-compose.yaml      # All infrastructure
│       └── setup.sh
│
└── README.md                         # Main documentation
```

**Pros:**
- ✅ **Clear separation** - Python AI vs Go operations
- ✅ **Shared documentation** - Message contracts and routing
- ✅ **Independent repos** - Can evolve separately
- ✅ **Language-appropriate** - Each uses best tools
- ✅ **Easy navigation** - Clear project boundaries

**Cons:**
- ❌ Some code duplication (consumer patterns)
- ❌ Need to maintain consistency manually

**Why Recommended:**
- Matches the distinct nature of the two worker types
- GraphRAG is heavy AI processing, task workers are lightweight operational
- Makes it easy to scale and deploy independently
- Clear ownership boundaries for maintenance

---

## 🔍 Deep Dive: GraphRAG Worker (Python)

### Project Name Suggestion: `graphrag-service`

Alternatives: `knowledge-worker`, `graph-processor`, `document-intelligence-worker`

### Core Responsibilities

1. **Document Processing Pipeline**
   - Consume `document.process` messages from RabbitMQ
   - Run full GraphRAG pipeline (ingest → graph construction)
   - Store results in MongoDB
   - Publish completion events back to RabbitMQ

2. **Knowledge Graph Management**
   - Maintain entity and relationship databases
   - Handle incremental updates
   - Provide query interfaces

3. **LLM Orchestration**
   - Manage concurrent LLM calls (100-300 concurrent)
   - Handle rate limiting (950K TPM, 20K RPM)
   - Track costs and usage

### New Components to Add

#### 1. RabbitMQ Consumer (`src/worker/consumer.py`)

> **IMPORTANT:** GraphRAG pipelines are **async** (`await pipeline.run()`), so we use **aio-pika** (async RabbitMQ client) for natural integration.

```python
import aio_pika
import asyncio
import json
import logging
from typing import Callable, Awaitable

logger = logging.getLogger(__name__)

class AsyncRabbitMQConsumer:
    """Async RabbitMQ consumer for GraphRAG worker (using aio-pika)"""
    
    def __init__(self, config: dict):
        self.config = config
        self.connection = None
        self.channel = None
        
    async def connect(self):
        """Establish async connection to RabbitMQ"""
        self.connection = await aio_pika.connect_robust(
            host=self.config['host'],
            port=self.config['port'],
            login=self.config['username'],
            password=self.config['password'],
            virtualhost=self.config.get('vhost', '/')
        )
        
        self.channel = await self.connection.channel()
        await self.channel.set_qos(prefetch_count=1)  # Memory-intensive processing
        
        # Declare exchange
        exchange = await self.channel.declare_exchange(
            'document-tasks',
            aio_pika.ExchangeType.DIRECT,
            durable=True
        )
        
        # Declare queue with DLQ
        queue = await self.channel.declare_queue(
            'document-processing',
            durable=True,
            arguments={
                'x-message-ttl': 43200000,  # 12 hours
                'x-dead-letter-exchange': 'document-tasks.dlx',
            }
        )
        
        # Bind queue to exchange
        await queue.bind(exchange, routing_key='document.process')
        
        return queue
    
    async def consume(self, handler: Callable[[dict], Awaitable[None]]):
        """Start consuming messages with async handler"""
        queue = await self.connect()
        
        async with queue.iterator() as queue_iter:
            async for message in queue_iter:
                async with message.process():
                    try:
                        payload = json.loads(message.body)
                        await handler(payload)  # Async handler
                        logger.info(f"Processed message: {payload.get('id')}")
                    except Exception as e:
                        logger.error(f"Error processing message: {e}")
                        # Message will be nack'd and sent to DLQ
                        raise
    
    async def close(self):
        """Close connection gracefully"""
        if self.connection:
            await self.connection.close()
```

#### 2. Document Processor (`src/worker/processor.py`)

> **IMPORTANT:** GraphRAG pipelines use `async def run()`. The processor must be async.

```python
import asyncio
import logging
from typing import Dict, Any
from src.domain.ingestion.pipeline import IngestionPipeline
from src.domain.graphrag.pipeline import GraphRAGPipeline

logger = logging.getLogger(__name__)

class DocumentProcessor:
    """Async document processor wrapping GraphRAG pipelines"""
    
    def __init__(self, config: dict):
        self.config = config
        
    def validate(self, message: dict) -> bool:
        """Validate message structure"""
        required = ['id', 'type', 'payload', 'timestamp']
        if not all(field in message for field in required):
            return False
        
        payload = message.get('payload', {})
        required_payload = ['document_url', 'document_type']
        return all(field in payload for field in required_payload)
        
    async def process(self, message: dict) -> Dict[str, Any]:
        """
        Process document through GraphRAG pipeline (ASYNC)
        
        Message format:
        {
            "id": "msg-uuid",
            "type": "document.process",
            "payload": {
                "document_url": "s3://bucket/doc.pdf",
                "document_type": "pdf",
                "user_id": "user-123"
            },
            "metadata": {...}
        }
        """
        payload = message['payload']
        document_url = payload['document_url']
        document_type = payload['document_type']
        user_id = payload.get('user_id')
        
        logger.info(f"Processing document: {document_url}")
        
        # 1. Build configs from message payload
        ingest_config = self._build_ingest_config(document_url, document_type)
        graphrag_config = self._build_graphrag_config(user_id)
        
        # 2. Run ingestion pipeline (ASYNC)
        ingest_pipeline = IngestionPipeline(ingest_config)
        ingest_result = await ingest_pipeline.run()
        logger.info(f"Ingestion complete: {ingest_result.chunks_count} chunks")
        
        # 3. Run GraphRAG pipeline (ASYNC)
        graphrag_pipeline = GraphRAGPipeline(graphrag_config)
        graphrag_result = await graphrag_pipeline.run()
        logger.info(f"GraphRAG complete: {graphrag_result.entities_count} entities")
        
        return {
            "status": "completed",
            "chunks_count": ingest_result.chunks_count,
            "entities_count": graphrag_result.entities_count,
            "relationships_count": graphrag_result.relationships_count,
            "communities_count": graphrag_result.communities_count,
            "processing_time_seconds": graphrag_result.duration
        }
    
    def _build_ingest_config(self, url: str, doc_type: str) -> dict:
        """Build IngestionPipelineConfig from message"""
        return {
            'document_url': url,
            'document_type': doc_type,
            'mongodb_uri': self.config['mongodb_uri'],
            # ... other config
        }
    
    def _build_graphrag_config(self, user_id: str) -> dict:
        """Build GraphRAGPipelineConfig from message"""
        return {
            'user_id': user_id,
            'mongodb_uri': self.config['mongodb_uri'],
            'openai_api_key': self.config['openai_api_key'],
            # ... other config
        }
```

#### 3. Health Check Server (`src/monitoring/health.py`)

```python
from flask import Flask, jsonify
import threading

app = Flask(__name__)

@app.route('/health')
def health():
    return jsonify({"status": "ok"})

@app.route('/ready')
def ready():
    # Check MongoDB, LLM provider connectivity
    checks = {
        "mongodb": check_mongodb(),
        "llm_provider": check_llm(),
        "rabbitmq": check_rabbitmq()
    }
    all_ok = all(checks.values())
    status_code = 200 if all_ok else 503
    return jsonify({"status": "ready" if all_ok else "not ready", "checks": checks}), status_code

def start_health_server(port=8080):
    threading.Thread(target=lambda: app.run(host='0.0.0.0', port=port), daemon=True).start()
```

### Dependencies to Add

```
# RabbitMQ (async - required for GraphRAG async pipelines)
aio-pika>=9.4.0

# HTTP server for health checks (Flask is lightweight, or use FastAPI for consistency)
flask>=3.0.0
# OR: fastapi>=0.115.0 + uvicorn>=0.32.0 (matches GraphRAG's existing APIs)

# Metrics
prometheus-client>=0.20.0

# Async utilities
asyncio  # stdlib
```

### Resource Requirements

| Resource | Recommendation | Rationale |
|----------|----------------|-----------|
| **CPU** | 2-4 cores | LLM processing, graph algorithms |
| **Memory** | 4-8 GB | Large language model responses, graph in memory |
| **Replicas** | 1-3 | Limited by LLM rate limits, not CPU |
| **Prefetch** | 1 | One document at a time (memory intensive) |
| **TTL** | 12 hours | Long processing time per document |

---

## 🚀 Deep Dive: Operational Workers (Go)

### Project Name Suggestion: `operational-workers`

Alternatives: `task-workers`, `service-workers`, `event-processors`

### Core Responsibilities

1. **Email Worker** - Send notifications, alerts, welcome emails
2. **Image Worker** - Resize, filter, analyze images
3. **Profile Worker** - Profile-related background tasks

### Architecture Pattern (from worker-service)

```go
// Shared foundation
type BaseWorker struct {
    config    *WorkerConfig
    processor processors.MessageProcessor
    consumer  *queue.Consumer
    server    *HTTPServer
    metrics   *ProcessorMetrics
}

// Worker-specific implementation
type EmailProcessor struct {
    // Email-specific logic
}

func (p *EmailProcessor) Process(ctx context.Context, msg *queue.Message) error {
    // Parse, validate, send email
}
```

### Integration Points

| Component | Purpose | Implementation |
|-----------|---------|----------------|
| **RabbitMQ Consumer** | Message consumption | Copy from `legacy_project/services/common/queue` into `internal/common/queue` |
| **HTTP Server** | Health checks | Gin on port 8080 |
| **Metrics** | Prometheus metrics | `prometheus/client_golang` |
| **Logging** | Structured logs | `uber-go/zap` |

### Resource Requirements

| Worker | CPU | Memory | Replicas | Prefetch |
|--------|-----|--------|----------|----------|
| **Email** | 50m-200m | 64Mi-256Mi | 2-15 | 5 |
| **Image** | 500m-1000m | 512Mi-1Gi | 1-8 | 1 |
| **Profile** | 100m-300m | 128Mi-512Mi | 1-5 | 2 |

---

## 🔄 Message Flow Architecture

### Overall System Flow

```
┌──────────────┐
│   Client     │
└──────┬───────┘
       │ HTTP + JWT
       ▼
┌──────────────────┐
│   API Service    │
│  (Go - Gateway)  │
└──────┬───────────┘
       │ Publishes to RabbitMQ
       ▼
┌──────────────────────────────────────────────────┐
│                   RabbitMQ                       │
│  ┌──────────────┬─────────────┬────────────┐   │
│  │document-tasks│ email-tasks │image-tasks │   │
│  └──────┬───────┴──────┬──────┴─────┬──────┘   │
└─────────┼──────────────┼────────────┼──────────┘
          │              │            │
          ▼              ▼            ▼
  ┌───────────────┐ ┌─────────┐ ┌─────────┐
  │   GraphRAG    │ │  Email  │ │  Image  │
  │    Service    │ │ Worker  │ │ Worker  │
  │   (Python)    │ │  (Go)   │ │  (Go)   │
  └───────┬───────┘ └────┬────┘ └────┬────┘
          │              │            │
          ▼              ▼            ▼
      MongoDB      Email APIs    Image APIs
```

### Routing Keys Design

| Routing Key | Exchange | Queue | Worker | TTL | Priority |
|-------------|----------|-------|--------|-----|----------|
| `document.process` | `document-tasks` | `document-processing` | graphrag-service | 12h | High |
| `email.send` | `email-tasks` | `email-processing` | email-worker | 1h | Medium |
| `image.process` | `image-tasks` | `image-processing` | image-worker | 6h | Low |
| `profile.task` | `tasks-exchange` | `profile-processing` | profile-worker | 24h | Low |

### Message Format Standardization

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "type": "document.process|email.send|image.process|profile.task",
  "timestamp": "2026-01-30T10:30:00Z",
  "correlation_id": "req-456",
  "payload": {
    "worker_specific": "data"
  },
  "metadata": {
    "user_id": "user-123",
    "source": "api-service",
    "priority": 0
  },
  "priority": 0
}
```

---

## 🔀 Crossover Opportunities

### 1. Shared Message Contract (HIGH PRIORITY)

**Create:** `shared/contracts/MESSAGE_FORMAT.md`

Both Python and Go workers should consume the same message format:
- Same field names
- Same timestamp format (RFC3339)
- Same metadata conventions
- Same error response format

**Benefits:**
- Consistency across workers
- Easier debugging
- Can switch languages for a worker type if needed

---

### 2. Shared RabbitMQ Configuration (HIGH PRIORITY)

**Create:** `shared/configs/rabbitmq/topology.yaml`

```yaml
exchanges:
  - name: document-tasks
    type: direct
    durable: true
    
  - name: email-tasks
    type: direct
    durable: true
    
  - name: image-tasks
    type: direct
    durable: true

queues:
  - name: document-processing
    exchange: document-tasks
    routing_key: document.process
    durable: true
    ttl_hours: 12
    prefetch: 1
    dlq:
      ttl_days: 7
      max_retries: 3
      
  - name: email-processing
    exchange: email-tasks
    routing_key: email.send
    durable: true
    ttl_hours: 1
    prefetch: 5
    dlq:
      ttl_days: 1
      max_retries: 5
```

**Benefits:**
- Single source of truth for queue topology
- Can generate setup scripts for both languages
- Documentation reference

---

### 3. Shared Metrics Patterns (MEDIUM PRIORITY)

Both should expose Prometheus metrics with similar naming:

**Standard Metrics:**
```
# Go
worker_messages_processed_total{worker_type="email", status="success"}
worker_processing_duration_seconds{worker_type="email"}
worker_errors_total{worker_type="email", error_type="validation"}

# Python
worker_messages_processed_total{worker_type="graphrag", status="success"}
worker_processing_duration_seconds{worker_type="graphrag"}
worker_errors_total{worker_type="graphrag", error_type="llm"}
```

**Benefits:**
- Unified monitoring dashboards
- Same alerting rules
- Easier to compare performance

---

### 4. Shared Health Check Patterns (MEDIUM PRIORITY)

**Standard Health Check Response:**
```json
{
  "status": "ok|degraded|down",
  "worker_type": "graphrag|email|image|profile",
  "checks": {
    "rabbitmq": "ok|down",
    "database": "ok|down",
    "external_service": "ok|down"
  },
  "uptime_seconds": 1234,
  "messages_processed": 567
}
```

**Benefits:**
- Kubernetes readiness probes work consistently
- Same monitoring scripts
- Standard troubleshooting approach

---

### 5. Python RabbitMQ Consumer Wrapper (HIGH PRIORITY)

**Create:** Python equivalent of Go's BaseWorker

```python
# graphrag-service/src/worker/base_worker.py

from abc import ABC, abstractmethod
import pika
import signal
import threading

class MessageProcessor(ABC):
    @abstractmethod
    def process(self, message: dict) -> dict:
        """Process a message and return result"""
        pass
    
    @abstractmethod
    def validate(self, message: dict) -> bool:
        """Validate message format"""
        pass

class BaseWorker:
    def __init__(self, config, processor: MessageProcessor):
        self.config = config
        self.processor = processor
        self.consumer = RabbitMQConsumer(config)
        self.health_server = HealthCheckServer(port=8080)
        self.metrics = PrometheusMetrics()
        self.shutdown = False
        
    def start(self):
        # Start health check server
        self.health_server.start()
        
        # Setup signal handlers
        signal.signal(signal.SIGTERM, self._handle_shutdown)
        signal.signal(signal.SIGINT, self._handle_shutdown)
        
        # Start consuming
        self.consumer.connect()
        self.consumer.consume(self._handle_message)
        
    def _handle_message(self, message: dict):
        if not self.processor.validate(message):
            self.metrics.record_error("validation")
            return
            
        start = time.time()
        try:
            result = self.processor.process(message)
            duration = time.time() - start
            self.metrics.record_success(duration)
        except Exception as e:
            duration = time.time() - start
            self.metrics.record_error("processing")
            raise
    
    def _handle_shutdown(self, signum, frame):
        print("Shutdown signal received")
        self.shutdown = True
        self.consumer.close()
        self.health_server.stop()
```

**Benefits:**
- Same pattern as Go workers
- Easier for developers to understand both
- Consistent behavior

---

### 6. Go Worker Could Call Python Services (CROSSOVER)

**Scenario:** Image worker needs ML-based image analysis

```go
// operational-workers/internal/processors/image/ml_client.go

type MLClient struct {
    endpoint string
}

func (c *MLClient) AnalyzeImage(imageData []byte) (*AnalysisResult, error) {
    // Call Python ML service
    resp, err := http.Post(
        c.endpoint+"/analyze",
        "application/octet-stream",
        bytes.NewReader(imageData),
    )
    // ...
}
```

This creates a hybrid approach:
- Go worker handles message consumption (efficient)
- Python service handles ML processing (appropriate)

---

## ⚖️ Major Tradeoffs

### Tradeoff 1: Separate Projects vs Single Repo

| Approach | Pros | Cons |
|----------|------|------|
| **Separate** | Language-specific tooling, clear ownership | Harder to share contracts, version drift |
| **Monorepo** | Shared docs, atomic changes | Need polyglot tooling, bigger repo |

**Recommendation:** **Monorepo** (`graph-worker/`) with separate language folders

---

### Tradeoff 2: Python Async vs Sync Consumer

| Approach | Pros | Cons |
|----------|------|------|
| **Sync (pika)** | Simpler, matches Go pattern, easier debugging | Blocks on I/O, less efficient |
| **Async (aio-pika)** | Better for I/O-bound tasks, matches GraphRAG async | More complex, different from Go |

**Recommendation:** **Start with Sync (pika)** - GraphRAG processing is CPU-bound anyway, sync is simpler

---

### Tradeoff 3: GraphRAG APIs - Keep or Remove?

GraphRAG currently has Graph API (8081) and Stages API (8080).

| Approach | Pros | Cons |
|----------|------|------|
| **Keep APIs** | Debugging, manual queries, direct access | Extra ports, security complexity |
| **Remove APIs** | Simpler, queue-only interface | Harder to debug, no manual control |
| **Internal only** | Keep for debugging, not expose externally | Compromise solution |

**Recommendation:** **Keep APIs but mark internal-only** - Valuable for debugging and manual pipeline runs

---

### Tradeoff 4: Where Does Graph Query API Live?

Users need to query the knowledge graph. Where should the query API live?

| Option | Description | Pros | Cons |
|--------|-------------|------|------|
| **A: In graphrag-service** | Python service exposes query API | Natural fit, direct DB access | Worker should process, not serve |
| **B: In api-service** | Go gateway proxies to MongoDB | Gateway pattern, Go handles all HTTP | Go calling MongoDB + Python logic complex |
| **C: Separate graph-api** | New Python service just for queries | Separation of concerns, scales independently | Another deployment |

**Recommendation:** **Option A initially, move to C later** - Keep query API in graphrag-service for now, extract to separate service when scaling needs demand it

---

### Tradeoff 5: Go Workers - Mono-Binary vs Multi-Binary

| Approach | Structure | Pros | Cons |
|----------|-----------|------|------|
| **Multi-binary** | 3 separate main.go files | Independent builds, clearer separation | More Dockerfiles, more manifests |
| **Mono-binary** | Single main.go with worker mode flag | Single build, shared base | Larger binary, all code included |

**Recommendation:** **Multi-binary** - Matches current worker-service pattern, cleaner deployment

---

## 🎯 Proposed Routing Keys and Exchanges

### Exchange Topology

```yaml
exchanges:
  document-tasks:        # GraphRAG processing
    type: direct
    durable: true
    
  email-tasks:           # Email notifications
    type: direct
    durable: true
    
  image-tasks:           # Image processing
    type: direct
    durable: true
    
  tasks-exchange:        # Generic tasks (profile, etc.)
    type: direct
    durable: true
    
  results-exchange:      # Worker results (NEW)
    type: fanout
    durable: true
```

### Queue Configuration

| Queue | Routing Key | TTL | Prefetch | Max Retries | DLQ TTL |
|-------|-------------|-----|----------|-------------|---------|
| `document-processing` | `document.process` | 12h | 1 | 3 | 7d |
| `email-processing` | `email.send` | 1h | 5 | 5 | 1d |
| `image-processing` | `image.process` | 6h | 1 | 2 | 3d |
| `profile-processing` | `profile.task` | 24h | 2 | 3 | 7d |
| `results-inbox` | `*` | 24h | 10 | 3 | 3d |

### NEW: Results Exchange (Feedback Loop)

Workers can publish results back:

```
Worker → RabbitMQ (results-exchange) → API Service consumes
```

Example: GraphRAG completes document processing, publishes completion event:

```json
{
  "id": "result-uuid",
  "type": "document.completed",
  "original_message_id": "original-msg-id",
  "routing_key": "document.process",
  "payload": {
    "entities_count": 5000,
    "relationships_count": 15000,
    "communities_count": 50
  },
  "timestamp": "2026-01-30T12:45:00Z"
}
```

---

## 🔧 Implementation Strategies

### Strategy 1: Parallel Development

**Timeline:**
- Week 1-2: Set up graphrag-service skeleton, add RabbitMQ consumer
- Week 3-4: Integrate GraphRAG pipeline with worker
- Week 5: Set up operational-workers (copy from worker-service)
- Week 6: Testing and deployment

**Team Assignment:**
- Python developer: graphrag-service
- Go developer: operational-workers
- Both: Shared contracts and RabbitMQ config

**Pros:**
- Fastest path to working system
- Parallel work streams
- Early validation of architecture

**Cons:**
- Risk of drift between implementations
- Need strong coordination on contracts

---

### Strategy 2: Sequential Development

**Timeline:**
- Week 1-3: Complete graphrag-service first
- Week 4-5: Build operational-workers
- Week 6: Integration testing

**Pros:**
- Learn from first implementation
- Better consistency
- Lower coordination overhead

**Cons:**
- Longer time to full system
- Can't parallelize work

---

### Strategy 3: Framework-First

**Timeline:**
- Week 1: Define shared contracts and framework
- Week 2: Implement Python worker framework
- Week 3: Implement Go worker framework
- Week 4-5: Implement worker-specific logic
- Week 6: Testing

**Pros:**
- Strong foundation
- High consistency
- Reusable framework

**Cons:**
- Slowest start
- Risk of over-engineering
- Framework may not fit all use cases

**Recommendation:** **Strategy 1 (Parallel)** with strong contract definition upfront

---

## 🐍 Python-Specific Considerations

### Python Worker Challenges

| Challenge | Solution |
|-----------|----------|
| **GIL Limitations** | Use multiprocessing for CPU-bound tasks, asyncio for I/O |
| **Memory Usage** | GraphRAG loads large models - monitor carefully |
| **Deployment Size** | Large Docker images (>2GB) - use multi-stage builds |
| **Startup Time** | 30-60s to load models - adjust readiness probes |
| **Error Handling** | Python exceptions - ensure proper try/catch in consumer |

### Python Dependencies Management

```dockerfile
# Use multi-stage build to reduce image size
FROM python:3.11-slim as builder
WORKDIR /app
COPY requirements.txt .
RUN pip install --user --no-cache-dir -r requirements.txt

FROM python:3.11-slim
WORKDIR /app
COPY --from=builder /root/.local /root/.local
COPY . .
ENV PATH=/root/.local/bin:$PATH
CMD ["python", "cmd/main.py"]
```

### Python Concurrency for GraphRAG

GraphRAG already has sophisticated concurrency:
- Async/await for I/O (LLM calls, DB operations)
- ProcessPoolExecutor for CPU-bound (embeddings)
- Rate limiting with sliding window
- Adaptive concurrency based on rate limit feedback

**Don't change this** - it's already optimized. Just wrap it with RabbitMQ consumer.

---

## 🔐 Security Considerations

### Authentication for Workers

Workers don't need to authenticate users (api-service does that), but they may need:

| Scenario | Solution |
|----------|----------|
| **Calling external APIs** | Service accounts, API keys in secrets |
| **Calling api-service** | Internal service mesh (no auth) or shared secret |
| **Accessing storage** | Database credentials in secrets |
| **Publishing results** | RabbitMQ credentials |

### User Context in Messages

Messages should carry user context:

```json
{
  "metadata": {
    "user_id": "user-123",
    "user_role": "premium",
    "tenant_id": "tenant-456"  // For multi-tenancy
  }
}
```

Workers use this for:
- Audit logging
- Rate limiting per user
- Results association

---

## 📊 Resource Planning

### Cluster Resource Requirements

| Service | Language | Replicas | CPU per Pod | Memory per Pod | Total CPU | Total Memory |
|---------|----------|----------|-------------|----------------|-----------|--------------|
| **api-service** | Go | 3 | 200m | 256Mi | 600m | 768Mi |
| **graphrag-service** | Python | 1-2 | 2000m | 4Gi | 2-4 cores | 4-8Gi |
| **email-worker** | Go | 2-15 | 100m | 128Mi | 200m-1.5 cores | 256Mi-1.9Gi |
| **image-worker** | Go | 1-8 | 500m | 512Mi | 500m-4 cores | 512Mi-4Gi |
| **profile-worker** | Go | 1-5 | 200m | 256Mi | 200m-1 core | 256Mi-1.3Gi |
| **auth-service** | Node.js | 2 | 200m | 256Mi | 400m | 512Mi |
| **Total** | | | | | **4-12 cores** | **6-17Gi** |

**Infrastructure:**
- PostgreSQL: 2 cores, 2Gi
- Redis: 500m, 512Mi
- RabbitMQ: 1 core, 1Gi
- MongoDB (GraphRAG): 2 cores, 4Gi

**Grand Total:** ~10-20 cores, 14-25Gi RAM

---

## 🚨 Critical Design Decisions

### Decision 1: Document Storage ✅ DECIDED

GraphRAG processes documents. Where are raw documents stored?

| Option | Description | Pros | Cons |
|--------|-------------|------|------|
| **MinIO/S3** | Object storage for documents | Industry standard, scalable | Another service to manage |
| **PostgreSQL BLOB** | Store in existing DB | No new service | Not ideal for large files |
| **MongoDB GridFS** | Store in GraphRAG's MongoDB | Unified storage | Couples concerns |

**Decision:** **MinIO deployed in Kubernetes**

**Rationale:** Full control over storage, no external cloud costs, S3-compatible API for easy integration with api-service.

**Implementation:**

```yaml
# graph-worker/shared/infrastructure/minio-statefulset.yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: minio
  namespace: graph-worker
spec:
  serviceName: minio
  replicas: 1
  selector:
    matchLabels:
      app: minio
  template:
    metadata:
      labels:
        app: minio
    spec:
      containers:
      - name: minio
        image: minio/minio:latest
        args: ["server", "/data", "--console-address", ":9001"]
        env:
        - name: MINIO_ROOT_USER
          value: minioadmin
        - name: MINIO_ROOT_PASSWORD
          valueFrom:
            secretKeyRef:
              name: minio-secret
              key: password
        ports:
        - containerPort: 9000  # S3 API
        - containerPort: 9001  # Console
        volumeMounts:
        - name: data
          mountPath: /data
        resources:
          requests:
            cpu: 500m
            memory: 1Gi
          limits:
            cpu: 1000m
            memory: 2Gi
  volumeClaimTemplates:
  - metadata:
      name: data
    spec:
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 100Gi
---
apiVersion: v1
kind: Service
metadata:
  name: minio
  namespace: graph-worker
spec:
  selector:
    app: minio
  ports:
  - port: 9000
    targetPort: 9000
    name: api
  - port: 9001
    targetPort: 9001
    name: console
```

---

### Decision 2: MongoDB Storage ✅ DECIDED

GraphRAG needs MongoDB for entities, relationships, and communities.

| Option | Description | Pros | Cons |
|--------|-------------|------|------|
| **MongoDB Atlas** | Managed cloud service | No ops, vector search built-in, easy | Cost ($), external dependency |
| **Self-hosted K8s** | StatefulSet in cluster | Full control, no cloud cost | Complex ops, backup management |
| **Existing MongoDB** | If already available | No new deployment | May not exist |

**Decision:** **MongoDB Atlas**

**Rationale:** 
- No operational overhead (managed service)
- Built-in vector search capabilities for RAG
- Easy to get started, scales automatically
- Can migrate to self-hosted later if cost becomes a concern

**Configuration:**

```python
# graphrag-service/src/config/worker_config.py
{
    'mongodb_uri': os.getenv('MONGODB_URI',
        'mongodb+srv://<user>:<password>@<cluster>.mongodb.net/graphrag?retryWrites=true&w=majority'
    ),
    'mongodb_database': os.getenv('MONGODB_DATABASE', 'graphrag'),
}
```

**Environment Variables:**
```yaml
# In Kubernetes ConfigMap/Secret
MONGODB_URI: "mongodb+srv://graphrag-user:<password>@cluster0.xxxxx.mongodb.net/graphrag"
MONGODB_DATABASE: "graphrag"
```

---

### Decision 3: Results Notification

When workers complete tasks, how does api-service know?

| Option | Description | Pros | Cons |
|--------|-------------|------|------|
| **Publish to results queue** | Workers publish to `results-exchange` | Async, decoupled | api-service needs consumer |
| **Webhook callback** | Workers call api-service HTTP endpoint | Simple, direct | Requires retry logic, coupling |
| **Polling** | api-service polls status | Simple | Inefficient, polling overhead |
| **No notification** | Client polls api-service | Simplest | Poor UX |

**Recommendation:** **Publish to results queue** - Most decoupled, allows multiple consumers

---

### Decision 4: GraphRAG Service Role ⏳ TO BE DECIDED

> **REMINDER:** This decision should be made after infrastructure and basic worker implementation are complete.

Should graphrag-service be a worker only, or also expose APIs?

| Option | Description | Pros | Cons |
|--------|--------------|------|------|
| **A: Worker Only** | Just consumes from RabbitMQ, no external APIs | Simple, clean separation, no port conflicts | Can't query graph directly |
| **B: Worker + Query API** | Worker + Graph API on port 8081 | Convenient for queries, uses existing GraphRAG APIs | More complex, dual responsibility |
| **C: Separate Query Service** | Worker separate from graph-query-service | Clean separation, independent scaling | Another deployment to manage |

**Option A - Worker Only (Recommended for Phase 1)**

*Rationale:*
- Keeps graphrag-service focused on one job: processing documents
- No port conflicts (8080 for health, 8081 for metrics only)
- Simpler Kubernetes deployment
- Query capability can be added later via api-service proxy or separate service

*Port Allocation:*
```
graphrag-worker: 8080 (health), 8081 (metrics)
```

**Option B - Worker + Query API (Phase 2 consideration)**

*Rationale:*
- Leverages GraphRAG's existing FastAPI Graph API
- Direct access to MongoDB for queries
- Convenient for development and debugging

*Port Allocation:*
```
graphrag-service: 
  - 8080: Health + Stages API (combined FastAPI)
  - 8081: Metrics + Graph API (combined FastAPI)
```

**Option C - Separate Query Service (Phase 3 consideration)**

*Rationale:*
- True separation of concerns
- Query service can scale independently
- Worker never blocked by query load

*Deployment:*
```
graphrag-worker: 8080 (health), 8081 (metrics)
graphrag-query: 8080 (health), 8081 (Graph API)
```

**Current Status:** Deferred until basic implementation complete. **Recommend starting with Option A** for simplicity.

---

### Decision 5: Python Worker Count

GraphRAG is memory/CPU intensive. How many replicas?

| Strategy | Replicas | Rationale |
|----------|----------|-----------|
| **Single replica** | 1 | Simplest, processes documents sequentially |
| **Multiple replicas** | 2-3 | Parallel processing, higher throughput |
| **HPA** | 1-5 | Auto-scale based on queue depth |

**Considerations:**
- LLM rate limits may bottleneck before CPU
- Each replica needs 4-8GB RAM
- MongoDB connection pool limits

**Recommendation:** **Start with 1, add HPA based on queue depth** (scale up if queue > 10 messages)

---

### Decision 5: Shared Code Between Python and Go?

Can we share any code between the two projects?

| What | Shareable? | Approach |
|------|-----------|----------|
| **Message schemas** | No (different languages) | Document in Markdown/YAML |
| **Queue topology** | Yes | YAML config, generate setup scripts |
| **Health check format** | Yes | JSON schema documentation |
| **Metrics names** | Yes | Naming convention document |
| **Error codes** | Yes | Shared error code list |
| **Docker compose** | Yes | Single file for all infrastructure |

**Recommendation:** Share **documentation and contracts**, not code

---

## 📋 Suggested Project Names

### Python Project: `graphrag-service`

**Alternatives:**
- `knowledge-graph-worker` ❌ Too long
- `document-intelligence` ✅ Good, emphasizes capability
- `graph-processor` ❌ Too generic
- `llm-graph-worker` ✅ Descriptive
- `graphrag-service` ✅ Clear, consistent with other services

**Recommendation:** `graphrag-service` - Clear, matches naming convention

---

### Go Project: `operational-workers`

**Alternatives:**
- `task-workers` ✅ Clear and concise
- `event-workers` ❌ Too generic
- `service-workers` ❌ Conflicts with web service workers
- `ops-workers` ❌ Informal abbreviation
- `operational-workers` ✅ Professional, descriptive

**Recommendation:** `operational-workers` - Clearly distinguishes from graphrag-service

---

## 🏛️ Recommended Final Structure

```
graph-worker/
│
├── README.md                          # Main overview
├── ARCHITECTURE.md                    # System architecture
├── docker-compose.yaml                # Local dev environment
│
├── graphrag-service/                  # Python: Knowledge graph construction
│   ├── cmd/
│   │   └── main.py                   # Worker entry point
│   ├── src/
│   │   ├── worker/                   # NEW: RabbitMQ integration
│   │   │   ├── base_worker.py       # Base worker pattern
│   │   │   ├── consumer.py          # Pika consumer
│   │   │   ├── processor.py         # Document processor
│   │   │   └── metrics.py           # Prometheus metrics
│   │   ├── graphrag/                 # COPY: From reference-materials
│   │   │   ├── app/                  # Graph API, Stages API
│   │   │   ├── src/
│   │   │   │   ├── domain/
│   │   │   │   ├── core/
│   │   │   │   └── infrastructure/
│   │   │   └── cli/
│   │   ├── config/
│   │   │   └── worker_config.py
│   │   └── monitoring/
│   │       └── health.py             # Health check server
│   ├── tests/
│   ├── requirements.txt
│   ├── Dockerfile
│   ├── deployments/kubernetes/
│   │   ├── deployment.yaml
│   │   ├── service.yaml
│   │   ├── configmap.yaml
│   │   └── hpa.yaml
│   └── README.md
│
├── operational-workers/               # Go: Task processing
│   ├── cmd/
│   │   ├── email-worker/
│   │   │   └── main.go
│   │   ├── image-worker/
│   │   │   └── main.go
│   │   └── profile-worker/
│   │       └── main.go
│   ├── internal/
│   │   ├── common/                   # COPY: From worker-service
│   │   │   ├── base/
│   │   │   │   ├── worker.go
│   │   │   │   └── server.go
│   │   │   ├── processor/
│   │   │   │   └── interface.go
│   │   │   └── metrics/
│   │   │       └── metrics.go
│   │   ├── processors/
│   │   │   ├── email/
│   │   │   │   ├── processor.go
│   │   │   │   └── message.go
│   │   │   ├── image/
│   │   │   │   ├── processor.go
│   │   │   │   └── message.go
│   │   │   └── profile/
│   │   │       ├── processor.go
│   │   │       └── message.go
│   │   └── domain/
│   │       └── message.go
│   ├── go.mod
│   ├── Dockerfile.email
│   ├── Dockerfile.image
│   ├── Dockerfile.profile
│   ├── deployments/kubernetes/
│   │   ├── email-worker.yaml
│   │   ├── image-worker.yaml
│   │   └── profile-worker.yaml
│   └── README.md
│
└── shared/                            # Shared contracts
    ├── contracts/
    │   ├── MESSAGE_FORMAT.md         # Standard message format
    │   ├── ROUTING_KEYS.md           # Routing conventions
    │   └── ERROR_CODES.md            # Error code list
    ├── configs/
    │   └── rabbitmq/
    │       ├── topology.yaml         # Exchange/queue definitions
    │       └── setup.sh              # Setup script
    ├── monitoring/
    │   ├── dashboards/
    │   │   ├── graphrag-dashboard.json
    │   │   └── workers-dashboard.json
    │   └── alerts/
    │       └── workers.rules
    └── docs/
        ├── DEPLOYMENT.md
        └── TROUBLESHOOTING.md
```

---

## 📦 What to Copy/Move

### From `legacy_project/GraphRAG/` → `graphrag-service/`

> **Source:** `/microservices/legacy_project/GraphRAG/` (external reference)

| Source | Destination | Changes |
|--------|-------------|---------|
| `legacy_project/GraphRAG/src/domain/` | `graphrag-service/src/graphrag/domain/` | Keep as-is |
| `legacy_project/GraphRAG/src/core/` | `graphrag-service/src/graphrag/core/` | Keep as-is |
| `legacy_project/GraphRAG/src/infrastructure/` | `graphrag-service/src/graphrag/infrastructure/` | Keep as-is |
| `legacy_project/GraphRAG/src/app/api/` | `graphrag-service/src/graphrag/app/api/` | Keep for optional API exposure |
| `legacy_project/GraphRAG/src/app/cli/` | `graphrag-service/src/graphrag/app/cli/` | Keep for manual runs |
| `legacy_project/GraphRAG/requirements.txt` | `graphrag-service/requirements.txt` | Add `aio-pika`, `flask`, `prometheus-client` |

**Copy Command:**
```bash
# From repository root
cp -r legacy_project/GraphRAG/src graph-worker/graphrag-service/src/graphrag/
cp legacy_project/GraphRAG/requirements.txt graph-worker/graphrag-service/requirements-graphrag.txt

# Merge requirements and add worker dependencies
cat graph-worker/graphrag-service/requirements-graphrag.txt >> graph-worker/graphrag-service/requirements.txt
echo "aio-pika>=9.4.0" >> graph-worker/graphrag-service/requirements.txt
echo "prometheus-client>=0.20.0" >> graph-worker/graphrag-service/requirements.txt
echo "flask>=3.0.0" >> graph-worker/graphrag-service/requirements.txt
```

**NEW to Create:**
- `src/worker/` - RabbitMQ consumer integration (async with aio-pika)
- `src/monitoring/` - Health check and metrics
- `cmd/main.py` - Worker entry point

---

### From `legacy_project/services/worker-service/` → `operational-workers/`

> **Source:** `/microservices/legacy_project/services/worker-service/`

| Source | Destination | Changes |
|--------|-------------|---------|
| `legacy_project/services/worker-service/services/workers/common/` | `operational-workers/internal/common/` | Keep as-is |
| `legacy_project/services/worker-service/services/workers/email-worker/` | `operational-workers/cmd/email-worker/` + `internal/processors/email/` | Restructure |
| `legacy_project/services/worker-service/services/workers/image-worker/` | `operational-workers/cmd/image-worker/` + `internal/processors/image/` | Restructure |
| Add profile worker | `operational-workers/cmd/profile-worker/` + `internal/processors/profile/` | NEW implementation |

**Also copy common/queue package:**
```bash
# Copy queue package into operational-workers (self-contained approach)
cp -r legacy_project/services/common/queue graph-worker/operational-workers/internal/common/queue
```

**Copy Command:**
```bash
# From repository root
cp -r legacy_project/services/worker-service/services/workers/common \
  graph-worker/operational-workers/internal/

cp -r legacy_project/services/worker-service/services/workers/email-worker/cmd \
  graph-worker/operational-workers/cmd/email-worker

cp -r legacy_project/services/worker-service/services/workers/email-worker/internal \
  graph-worker/operational-workers/internal/processors/email

# Same for image-worker
cp -r legacy_project/services/worker-service/services/workers/image-worker/cmd \
  graph-worker/operational-workers/cmd/image-worker

cp -r legacy_project/services/worker-service/services/workers/image-worker/internal \
  graph-worker/operational-workers/internal/processors/image

# Copy common queue package
cp -r legacy_project/services/common/queue \
  graph-worker/operational-workers/internal/common/queue
```

**Changes Required:**
- Update imports from `github.com/fernandobarroso/common/queue` to `github.com/fernandobarroso/microservices/operational-workers/internal/common/queue`
- Update module path in `go.mod`
- Simplify if possible (remove unused features)

---

## 🎬 Recommended Implementation Order

### Phase 0: Verification & Decisions (Day 0)

**Verification:**
```bash
# Verify GraphRAG structure
cd GraphRAG && python -c "from src.domain.graphrag.pipeline import GraphRAGPipeline; print('✅ OK')"

# Verify api-service
cd api-service && make build && echo "✅ OK"

# Verify worker-service exists
ls legacy_project/services/worker-service/services/workers/
```

**Decisions Confirmed:**
- ✅ MongoDB: Use Atlas (managed)
- ✅ MinIO: Deploy in Kubernetes
- ✅ Common package: Copy into operational-workers
- ✅ RabbitMQ consumer: Use aio-pika (async)
- ⏳ GraphRAG role: Decide after basic implementation (start with worker-only)

### Phase 1: Project Setup (Week 1)
1. Create `graph-worker/` directory structure
2. Set up `shared/` with contracts and RabbitMQ topology
3. Create `docker-compose.yaml` for local development
4. Document message formats and routing keys
5. **NEW:** Deploy MinIO to Kubernetes (or local docker-compose)
6. **NEW:** Set up MongoDB Atlas cluster

### Phase 1.5: API Service Additions (Week 1)
> **Required for document upload flow**

1. Add MinIO client to api-service dependencies
2. Create document upload endpoint: `POST /api/v1/documents/upload`
3. Create document status endpoint: `GET /api/v1/documents/:id/status`
4. Add `document.process` routing key to task/model.go

### Phase 2: GraphRAG Service (Week 2-3)
1. Copy GraphRAG code from `/microservices/legacy_project/GraphRAG/`
2. Implement **async** RabbitMQ consumer wrapper (aio-pika)
3. Create **async** document processor that calls GraphRAG pipelines
4. Add health check server (Flask or FastAPI)
5. Add Prometheus metrics
6. Create Dockerfile and Kubernetes manifests
7. Test with local RabbitMQ

### Phase 3: Operational Workers (Week 4)
1. Copy worker-service common foundation
2. Adapt email, image, profile workers
3. Update imports and module paths
4. Create Dockerfiles for each worker type
5. Create Kubernetes manifests
6. Test with local RabbitMQ

### Phase 4: Integration (Week 5)
1. Deploy all workers to Kubernetes
2. Test end-to-end: api-service → RabbitMQ → workers
3. Load testing and performance validation
4. Monitoring and alerting setup

### Phase 5: Documentation (Week 6)
1. Complete README for each service
2. Architecture diagrams
3. Deployment guides
4. Troubleshooting guides

---

## ❓ Open Questions for Review

### 1. Document Storage Strategy

**Question:** Where should raw documents be stored before GraphRAG processes them?

**Options:**
- A: MinIO/S3 (object storage)
- B: PostgreSQL BLOB
- C: MongoDB GridFS
- D: File system (not production-ready)

**Recommendation:** **A (MinIO/S3)**

---

### 2. Results Feedback

**Question:** Should workers publish completion/results back to api-service?

**Options:**
- A: Yes, via results exchange (api-service consumes)
- B: Yes, via HTTP callback to api-service
- C: No, api-service polls task status
- D: No, client polls api-service

**Recommendation:** **A (Results exchange)** for scalability

---

### 3. GraphRAG Query API

**Question:** Should GraphRAG's query API be exposed externally?

**Options:**
- A: Yes, directly (separate ingress)
- B: Yes, proxied through api-service
- C: No, internal only for debugging
- D: Extract to separate graph-query-service

**Recommendation:** **B (Proxied through api-service)** initially

---

### 4. Worker Deployment Strategy

**Question:** Deploy all workers together or independently?

**Options:**
- A: All in `graph-worker` namespace
- B: Separate namespaces per language (python-workers, go-workers)
- C: Separate namespace per worker type (graphrag, email, image, profile)

**Recommendation:** **A (Single namespace)** for simplicity

---

### 5. Development Workflow

**Question:** How should developers work on both Python and Go?

**Options:**
- A: Separate repos with submodules
- B: Monorepo with clear separation
- C: Separate repos with shared contracts repo

**Recommendation:** **B (Monorepo)** - `graph-worker/` contains both

---

## 🔄 Message Flow Examples

### Example 1: Document Processing (GraphRAG)

```
1. Client uploads document to api-service
   POST /api/v1/documents
   {
     "url": "https://youtube.com/watch?v=xyz",
     "type": "youtube",
     "user_id": "user-123"
   }

2. API service stores reference in PostgreSQL
   
3. API service publishes to RabbitMQ
   Exchange: document-tasks
   Routing Key: document.process
   Message: {
     "id": "msg-uuid",
     "type": "document.process",
     "payload": {
       "document_url": "https://youtube.com/watch?v=xyz",
       "document_type": "youtube",
       "user_id": "user-123"
     },
     "timestamp": "2026-01-30T10:00:00Z"
   }

4. graphrag-service consumes message
   - Fetches YouTube transcript
   - Runs ingestion pipeline (clean, chunk, embed)
   - Runs GraphRAG pipeline (extract, resolve, construct, detect)
   - Stores in MongoDB

5. graphrag-service publishes completion
   Exchange: results-exchange
   Message: {
     "type": "document.completed",
     "original_message_id": "msg-uuid",
     "payload": {
       "entities_count": 5000,
       "relationships_count": 15000,
       "status": "success"
     }
   }

6. api-service consumes result (optional)
   - Updates document status in PostgreSQL
   - Could trigger webhooks to client
```

### Example 2: Email Notification (Go Worker)

```
1. Profile created via api-service
   POST /api/v1/profiles
   
2. API service publishes welcome email task
   Exchange: email-tasks
   Routing Key: email.send
   Message: {
     "id": "msg-uuid",
     "type": "email.send",
     "payload": {
       "email_type": "welcome",
       "recipient": "user@example.com",
       "user_name": "John Doe"
     }
   }

3. email-worker consumes and processes
   - Validates message
   - Renders email template
   - Sends via SMTP/SendGrid
   - Returns success/failure

4. Email-worker acknowledges message
   - Success: ACK
   - Failure: NACK with requeue (up to 5 retries)
```

---

## 🎨 Crossover Opportunity: Unified Worker Framework

### Concept: Language-Agnostic Worker Pattern

Both Python and Go workers follow the same conceptual pattern:

```
┌─────────────────────────────────────────────┐
│           Worker (Python or Go)             │
│                                             │
│  ┌─────────────────────────────────────┐   │
│  │  1. RabbitMQ Consumer               │   │
│  │     - Connect to queue              │   │
│  │     - Consume messages              │   │
│  │     - Manual ACK/NACK               │   │
│  └──────────────┬──────────────────────┘   │
│                 │                           │
│  ┌──────────────▼──────────────────────┐   │
│  │  2. Message Handler                 │   │
│  │     - Validate message              │   │
│  │     - Call processor                │   │
│  │     - Handle errors                 │   │
│  └──────────────┬──────────────────────┘   │
│                 │                           │
│  ┌──────────────▼──────────────────────┐   │
│  │  3. Processor (Worker-specific)     │   │
│  │     - Business logic                │   │
│  │     - External service calls        │   │
│  │     - Result formatting             │   │
│  └──────────────┬──────────────────────┘   │
│                 │                           │
│  ┌──────────────▼──────────────────────┐   │
│  │  4. Side Channels                   │   │
│  │     - HTTP health server (8080)     │   │
│  │     - Prometheus metrics (8080)     │   │
│  │     - Graceful shutdown handler     │   │
│  └─────────────────────────────────────┘   │
└─────────────────────────────────────────────┘
```

**Shared Patterns:**
- Same worker lifecycle (start → consume → shutdown)
- Same message format
- Same health check endpoints
- Same metrics naming conventions
- Same error handling strategy

**Language-Specific Implementation:**
- Python: Pika + Flask + prometheus-client
- Go: amqp091-go + Gin + prometheus/client_golang

---

## 📈 Performance Considerations

### GraphRAG Service

| Metric | Target | Notes |
|--------|--------|-------|
| **Documents per hour** | 1-4 | Depends on document size |
| **Processing time** | 15-45 min | Per document |
| **Memory usage** | 4-8GB | Peaks during embedding |
| **CPU usage** | 80-100% | Sustained during processing |
| **LLM calls per document** | 1000-5000 | Rate limited |
| **Cost per document** | $0.50-$2.00 | OpenAI API costs |

### Operational Workers

| Worker | Messages/sec | Processing Time | Memory | CPU |
|--------|--------------|-----------------|--------|-----|
| **Email** | 100+ | 2-8s | <128Mi | <100m |
| **Image** | 10-20 | 10-25s | <1Gi | <1000m |
| **Profile** | 50+ | 5-15s | <256Mi | <200m |

---

## 🚦 Success Criteria

### Functional Requirements
- ✅ GraphRAG processes documents from RabbitMQ queue
- ✅ Operational workers process tasks from RabbitMQ queues
- ✅ All workers expose health checks and metrics
- ✅ Graceful shutdown on SIGTERM
- ✅ Error messages sent to DLQ after retries

### Performance Requirements
- ✅ GraphRAG: Process 1 document per 30 minutes on average
- ✅ Email worker: Process 100+ messages per second
- ✅ Image worker: Process 10+ messages per second
- ✅ Queue depth < 1000 messages (auto-scale if exceeded)

### Operational Requirements
- ✅ Zero-downtime deployment
- ✅ Kubernetes HPA configured per worker
- ✅ Monitoring dashboards for all workers
- ✅ Alerting on queue depth and error rates

---

## 🎯 Final Recommendations

### Structure
- **Use Option 3**: Monorepo with `graphrag-service` and `operational-workers`
- **Project names**: `graphrag-service` (Python), `operational-workers` (Go)
- **Shared**: Contracts, configs, monitoring in `shared/` directory

### Architecture Decisions
1. ✅ Document storage: MinIO/S3
2. ✅ Results notification: Publish to results exchange
3. ✅ Query API: Proxy via api-service
4. ✅ GraphRAG replicas: Start with 1, HPA based on queue depth
5. ✅ Go workers: Multi-binary approach

### Implementation Priority
1. **High**: Set up project structure and shared contracts
2. **High**: Implement graphrag-service RabbitMQ consumer
3. **High**: Copy and adapt operational-workers
4. **Medium**: Implement results feedback loop
5. **Medium**: Add query API proxy to api-service
6. **Low**: Advanced monitoring and alerting

---

## 📋 Next Steps

1. **Review this brainstorm** - Agree on architecture and names
2. **Decide on open questions** - Document storage, results feedback, etc.
3. **Create project skeleton** - Set up directory structure
4. **Implement in phases** - GraphRAG consumer → operational workers → integration

---

*Document Version: 1.0*  
*Created: January 2026*  
*Purpose: Architecture brainstorming for graph-worker system*
