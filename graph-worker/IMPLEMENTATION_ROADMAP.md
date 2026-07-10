# Implementation Roadmap - Graph Worker System

## 📅 Timeline Overview

```
Day 0: Verification & Decisions
Week 1: Setup, Contracts & Infrastructure
Week 2-3: GraphRAG Service
Week 4: Operational Workers
Week 5: Integration & Testing
Week 6: Documentation & Polish
```

## ✅ Confirmed Decisions

| Decision | Choice |
|----------|--------|
| MongoDB | Atlas (managed) |
| MinIO | Kubernetes deployment |
| Python RabbitMQ | aio-pika (async) |
| Common package | Copy into operational-workers |
| GraphRAG role | Worker-only (Phase 1), decide on API later |

---

## 📁 File-by-File Implementation Plan

### Phase 0: Verification (Day 0)

```bash
# Verify GraphRAG structure
cd /Users/fernandobarroso/Local\ Repo/microservices/legacy_project/GraphRAG
python -c "from src.domain.graphrag.pipeline import GraphRAGPipeline; print('✅ GraphRAG OK')"

# Verify api-service
cd /Users/fernandobarroso/Local\ Repo/microservices/api-service
make build && echo "✅ api-service OK"

# Verify worker-service exists
ls /Users/fernandobarroso/Local\ Repo/microservices/legacy_project/services/worker-service/services/workers/
```

### Phase 1: Project Setup (Days 1-2)

#### Shared Resources

| File | Purpose | Source |
|------|---------|--------|
| `graph-worker/README.md` | Main documentation | New |
| `graph-worker/docker-compose.yaml` | Local dev environment | New |
| `shared/contracts/MESSAGE_FORMAT.md` | Message schema | New |
| `shared/contracts/ROUTING_KEYS.md` | Routing conventions | New |
| `shared/contracts/ERROR_CODES.md` | Error codes | New |
| `shared/configs/rabbitmq/topology.yaml` | Queue definitions | New |
| `shared/infrastructure/minio-statefulset.yaml` | MinIO deployment | New |
| `shared/infrastructure/mongodb-atlas-config.md` | Atlas setup guide | New |

#### GraphRAG Service Structure

```bash
mkdir -p graphrag-service/{cmd,src/{worker,graphrag,config,monitoring},tests,deployments/kubernetes}
```

#### Operational Workers Structure

```bash
mkdir -p operational-workers/{cmd/{email-worker,image-worker,profile-worker},internal/{common/{base,processor,metrics},processors/{email,image,profile},domain},deployments/kubernetes}
```

---

## 🐍 Phase 2: GraphRAG Service (Days 3-10)

### Step 2.1: Copy GraphRAG Core (Day 3)

> **Source:** `/microservices/legacy_project/GraphRAG/` (external reference)

| Source | Destination | Action |
|--------|-------------|--------|
| `legacy_project/GraphRAG/src/domain/` | `graphrag-service/src/graphrag/domain/` | Copy entire directory |
| `legacy_project/GraphRAG/src/core/` | `graphrag-service/src/graphrag/core/` | Copy entire directory |
| `legacy_project/GraphRAG/src/infrastructure/` | `graphrag-service/src/graphrag/infrastructure/` | Copy entire directory |
| `legacy_project/GraphRAG/src/app/` | `graphrag-service/src/graphrag/app/` | Copy entire directory (optional APIs) |
| `legacy_project/GraphRAG/src/lib/` | `graphrag-service/src/graphrag/lib/` | Copy entire directory |
| `legacy_project/GraphRAG/requirements.txt` | `graphrag-service/requirements-graphrag.txt` | Copy, then merge |

**Copy Commands:**
```bash
# From repository root
mkdir -p graph-worker/graphrag-service/src/graphrag

cp -r legacy_project/GraphRAG/src/domain graph-worker/graphrag-service/src/graphrag/
cp -r legacy_project/GraphRAG/src/core graph-worker/graphrag-service/src/graphrag/
cp -r legacy_project/GraphRAG/src/infrastructure graph-worker/graphrag-service/src/graphrag/
cp -r legacy_project/GraphRAG/src/app graph-worker/graphrag-service/src/graphrag/
cp -r legacy_project/GraphRAG/src/lib graph-worker/graphrag-service/src/graphrag/
cp legacy_project/GraphRAG/requirements.txt graph-worker/graphrag-service/requirements-graphrag.txt

# Merge requirements and add worker dependencies
cat graph-worker/graphrag-service/requirements-graphrag.txt > graph-worker/graphrag-service/requirements.txt
echo "" >> graph-worker/graphrag-service/requirements.txt
echo "# Worker dependencies" >> graph-worker/graphrag-service/requirements.txt
echo "aio-pika>=9.4.0" >> graph-worker/graphrag-service/requirements.txt
echo "prometheus-client>=0.20.0" >> graph-worker/graphrag-service/requirements.txt
echo "flask>=3.0.0" >> graph-worker/graphrag-service/requirements.txt
```

**Verification:**
```bash
cd graph-worker/graphrag-service
python -c "from src.graphrag.domain.graphrag.pipeline import GraphRAGPipeline; print('✅ OK')"
```

---

### Step 2.2: Implement RabbitMQ Consumer (Days 4-5)

> **IMPORTANT:** Use **aio-pika** (async) because GraphRAG pipelines are async (`await pipeline.run()`)

#### File: `graphrag-service/src/worker/consumer.py`

**Template:**
```python
import aio_pika
import asyncio
import json
import logging
from typing import Callable, Awaitable, Optional

logger = logging.getLogger(__name__)

class AsyncRabbitMQConsumer:
    """Async RabbitMQ consumer for GraphRAG worker (using aio-pika)"""
    
    def __init__(self, config: dict):
        self.config = config
        self.connection: Optional[aio_pika.RobustConnection] = None
        self.channel: Optional[aio_pika.Channel] = None
        
    async def connect(self) -> aio_pika.Queue:
        """Establish async connection to RabbitMQ"""
        self.connection = await aio_pika.connect_robust(
            host=self.config['host'],
            port=self.config['port'],
            login=self.config['username'],
            password=self.config['password'],
            virtualhost=self.config.get('vhost', '/')
        )
        
        self.channel = await self.connection.channel()
        await self.channel.set_qos(prefetch_count=1)
        
        # Declare exchange and queue
        exchange = await self.channel.declare_exchange(
            'document-tasks', aio_pika.ExchangeType.DIRECT, durable=True
        )
        queue = await self.channel.declare_queue(
            'document-processing', durable=True,
            arguments={'x-message-ttl': 43200000, 'x-dead-letter-exchange': 'document-tasks.dlx'}
        )
        await queue.bind(exchange, routing_key='document.process')
        
        return queue
        
    async def consume(self, handler: Callable[[dict], Awaitable[None]]) -> None:
        """Start consuming messages with async handler"""
        queue = await self.connect()
        
        async with queue.iterator() as queue_iter:
            async for message in queue_iter:
                async with message.process():
                    try:
                        payload = json.loads(message.body)
                        await handler(payload)
                        logger.info(f"Processed: {payload.get('id')}")
                    except Exception as e:
                        logger.error(f"Error: {e}")
                        raise
        
    async def close(self) -> None:
        """Close connection gracefully"""
        if self.connection:
            await self.connection.close()
```

**Lines of Code:** ~80-100

---

#### File: `graphrag-service/src/worker/processor.py`

> **IMPORTANT:** GraphRAG pipelines are async - use `async def` and `await`

**Template:**
```python
import asyncio
import logging
from typing import Dict, Any
from src.graphrag.domain.ingestion.pipeline import IngestionPipeline
from src.graphrag.domain.graphrag.pipeline import GraphRAGPipeline

logger = logging.getLogger(__name__)

class DocumentProcessor:
    """Async processor for documents through GraphRAG pipelines"""
    
    def __init__(self, config: dict):
        self.config = config
        
    def validate(self, message: dict) -> bool:
        """Validate message structure"""
        required = ['id', 'type', 'payload', 'timestamp']
        if not all(field in message for field in required):
            return False
        payload = message.get('payload', {})
        return all(f in payload for f in ['document_url', 'document_type'])
        
    async def process(self, message: dict) -> Dict[str, Any]:
        """Process document message (ASYNC)"""
        payload = message['payload']
        document_url = payload['document_url']
        document_type = payload['document_type']
        user_id = payload.get('user_id')
        
        logger.info(f"Processing: {document_url}")
        
        # Build configs
        ingest_config = self._build_ingest_config(document_url, document_type)
        graphrag_config = self._build_graphrag_config(user_id)
        
        # Run ingestion pipeline (ASYNC)
        ingest_pipeline = IngestionPipeline(ingest_config)
        ingest_result = await ingest_pipeline.run()
        logger.info(f"Ingestion: {ingest_result.chunks_count} chunks")
        
        # Run GraphRAG pipeline (ASYNC)
        graphrag_pipeline = GraphRAGPipeline(graphrag_config)
        graphrag_result = await graphrag_pipeline.run()
        logger.info(f"GraphRAG: {graphrag_result.entities_count} entities")
        
        return {
            'status': 'completed',
            'chunks_count': ingest_result.chunks_count,
            'entities_count': graphrag_result.entities_count,
            'relationships_count': graphrag_result.relationships_count,
            'communities_count': graphrag_result.communities_count
        }
    
    def _build_ingest_config(self, url: str, doc_type: str) -> dict:
        return {'document_url': url, 'document_type': doc_type, **self.config}
    
    def _build_graphrag_config(self, user_id: str) -> dict:
        return {'user_id': user_id, **self.config}
```

**Lines of Code:** ~80-100

---

#### File: `graphrag-service/src/worker/base_worker.py`

**Template:**
```python
import signal
import sys
from src.worker.consumer import RabbitMQConsumer
from src.worker.processor import DocumentProcessor
from src.monitoring.health import start_health_server
from src.monitoring.metrics import PrometheusMetrics

class BaseWorker:
    """Base worker with RabbitMQ consumer, health, and metrics"""
    
    def __init__(self, config: dict):
        self.config = config
        self.consumer = RabbitMQConsumer(config['rabbitmq'])
        self.processor = DocumentProcessor(config['graphrag'])
        self.metrics = PrometheusMetrics()
        self.shutdown_requested = False
        
    def start(self):
        """Start worker"""
        # Setup signal handlers
        signal.signal(signal.SIGTERM, self._handle_shutdown)
        signal.signal(signal.SIGINT, self._handle_shutdown)
        
        # Start health server
        start_health_server(port=8080)
        
        # Connect and consume
        self.consumer.connect()
        self.consumer.consume(self._handle_message)
        
    def _handle_message(self, message: dict):
        """Handle incoming message"""
        if not self.processor.validate(message):
            self.metrics.record_error('validation')
            return
            
        try:
            result = self.processor.process(message)
            self.metrics.record_success()
        except Exception as e:
            self.metrics.record_error('processing')
            raise
            
    def _handle_shutdown(self, signum, frame):
        """Handle graceful shutdown"""
        print("Shutdown signal received")
        self.shutdown_requested = True
        self.consumer.close()
        sys.exit(0)
```

**Lines of Code:** ~100-150

---

### Step 2.3: Add Monitoring (Day 6)

#### File: `graphrag-service/src/monitoring/health.py`

```python
from flask import Flask, jsonify
import threading

app = Flask(__name__)

@app.route('/health')
def health():
    return jsonify({"status": "ok"})

@app.route('/ready')
def ready():
    # Check dependencies
    return jsonify({"status": "ready", "checks": {...}})

def start_health_server(port=8080):
    thread = threading.Thread(
        target=lambda: app.run(host='0.0.0.0', port=port, debug=False),
        daemon=True
    )
    thread.start()
```

**Lines of Code:** ~80-100

---

#### File: `graphrag-service/src/monitoring/metrics.py`

```python
from prometheus_client import Counter, Histogram, start_http_server

class PrometheusMetrics:
    def __init__(self):
        self.processed = Counter(
            'worker_messages_processed_total',
            'Total messages processed',
            ['worker_type', 'status']
        )
        self.duration = Histogram(
            'worker_processing_duration_seconds',
            'Processing duration',
            ['worker_type']
        )
        # Start metrics server on 8081
        start_http_server(8081)
```

**Lines of Code:** ~50-80

---

### Step 2.4: Entry Point (Day 7)

#### File: `graphrag-service/cmd/main.py`

```python
#!/usr/bin/env python3
import os
import sys
from pathlib import Path

# Add src to path
sys.path.insert(0, str(Path(__file__).parent.parent / 'src'))

from worker.base_worker import BaseWorker
from config.worker_config import load_config

def main():
    config = load_config()
    worker = BaseWorker(config)
    
    print(f"Starting GraphRAG worker...")
    print(f"Queue: {config['rabbitmq']['queue']}")
    print(f"Routing Key: {config['rabbitmq']['routing_key']}")
    
    worker.start()

if __name__ == '__main__':
    main()
```

**Lines of Code:** ~30-50

---

### Step 2.5: Configuration (Day 7)

#### File: `graphrag-service/src/config/worker_config.py`

```python
import os
from typing import Dict, Any

def load_config() -> Dict[str, Any]:
    return {
        'rabbitmq': {
            'host': os.getenv('RABBITMQ_HOST', 'localhost'),
            'port': int(os.getenv('RABBITMQ_PORT', '5672')),
            'username': os.getenv('RABBITMQ_USERNAME', 'guest'),
            'password': os.getenv('RABBITMQ_PASSWORD', 'guest'),
            'vhost': os.getenv('RABBITMQ_VHOST', '/'),
            'exchange': 'document-tasks',
            'queue': 'document-processing',
            'routing_key': 'document.process',
            'prefetch_count': 1
        },
        'graphrag': {
            'mongodb_uri': os.getenv('MONGODB_URI'),
            'openai_api_key': os.getenv('OPENAI_API_KEY'),
            # ... other GraphRAG config
        }
    }
```

**Lines of Code:** ~50-100

---

### Step 2.6: Dockerfile (Day 8)

```dockerfile
FROM python:3.11-slim as builder

WORKDIR /app
COPY requirements.txt .
RUN pip install --user --no-cache-dir -r requirements.txt

FROM python:3.11-slim

WORKDIR /app
COPY --from=builder /root/.local /root/.local
COPY . .

ENV PATH=/root/.local/bin:$PATH
ENV PYTHONUNBUFFERED=1

EXPOSE 8080 8081

CMD ["python", "cmd/main.py"]
```

---

### Step 2.7: Kubernetes Deployment (Days 9-10)

#### File: `graphrag-service/deployments/kubernetes/deployment.yaml`

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: graphrag-service
spec:
  replicas: 1
  selector:
    matchLabels:
      app: graphrag-service
  template:
    metadata:
      labels:
        app: graphrag-service
    spec:
      containers:
        - name: graphrag-service
          image: graphrag-service:latest
          resources:
            requests:
              cpu: 2000m
              memory: 4Gi
            limits:
              cpu: 4000m
              memory: 8Gi
          env:
            - name: RABBITMQ_HOST
              value: rabbitmq
            - name: MONGODB_URI
              valueFrom:
                secretKeyRef:
                  name: graphrag-secrets
                  key: mongodb-uri
            - name: OPENAI_API_KEY
              valueFrom:
                secretKeyRef:
                  name: graphrag-secrets
                  key: openai-api-key
          ports:
            - containerPort: 8080  # Health
            - containerPort: 8081  # Metrics/Graph API
          livenessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 60
            periodSeconds: 30
          readinessProbe:
            httpGet:
              path: /ready
              port: 8080
            initialDelaySeconds: 60
            periodSeconds: 30
```

---

## 🚀 Phase 3: Operational Workers (Days 11-15)

> **Source:** `/microservices/legacy_project/services/worker-service/`

### Step 3.1: Copy Common Foundation (Day 11)

| Source | Destination | Changes |
|--------|-------------|---------|
| `legacy_project/services/worker-service/services/workers/common/` | `operational-workers/internal/common/` | Update module paths |
| `legacy_project/services/common/queue/` | `operational-workers/internal/common/queue/` | Copy (self-contained) |

**Copy Commands:**
```bash
# From repository root
mkdir -p graph-worker/operational-workers/internal/common

# Copy worker common foundation
cp -r legacy_project/services/worker-service/services/workers/common/* \
  graph-worker/operational-workers/internal/common/

# Copy queue package (self-contained approach)
cp -r legacy_project/services/common/queue \
  graph-worker/operational-workers/internal/common/queue
```

**Update imports in all Go files:**
```
OLD: github.com/fernandobarroso/common/queue
NEW: github.com/fernandobarroso/microservices/operational-workers/internal/common/queue
```

---

### Step 3.2: Email Worker (Day 12)

| File | Source | Changes |
|------|--------|---------|
| `cmd/email-worker/main.go` | `legacy_project/services/worker-service/services/workers/email-worker/cmd/main.go` | Update imports |
| `internal/processors/email/processor.go` | `legacy_project/services/worker-service/services/workers/email-worker/internal/processors/email_processor.go` | Update imports |
| `internal/processors/email/message.go` | `legacy_project/services/worker-service/services/workers/email-worker/internal/domain/message.go` | Update imports |
| `Dockerfile.email` | `legacy_project/services/worker-service/services/workers/email-worker/Dockerfile` | Minimal changes |
| `deployments/kubernetes/email-worker.yaml` | `legacy_project/services/worker-service/services/workers/email-worker/deployments/kubernetes/deployment.yaml` | Update image name |

**Copy Commands:**
```bash
# Email worker
cp -r legacy_project/services/worker-service/services/workers/email-worker/cmd/* \
  graph-worker/operational-workers/cmd/email-worker/
cp -r legacy_project/services/worker-service/services/workers/email-worker/internal/* \
  graph-worker/operational-workers/internal/processors/email/
```

---

### Step 3.3: Image Worker (Day 13)

| File | Source | Changes |
|------|--------|---------|
| `cmd/image-worker/main.go` | `legacy_project/services/worker-service/services/workers/image-worker/cmd/main.go` | Update imports |
| `internal/processors/image/processor.go` | `legacy_project/services/worker-service/services/workers/image-worker/internal/processors/image_processor.go` | Update imports |
| `internal/processors/image/message.go` | `legacy_project/services/worker-service/services/workers/image-worker/internal/domain/message.go` | Update imports |
| `Dockerfile.image` | `legacy_project/services/worker-service/services/workers/image-worker/Dockerfile` | Minimal changes |
| `deployments/kubernetes/image-worker.yaml` | `legacy_project/services/worker-service/services/workers/image-worker/deployments/kubernetes/deployment.yaml` | Update image name |

**Copy Commands:**
```bash
# Image worker
cp -r legacy_project/services/worker-service/services/workers/image-worker/cmd/* \
  graph-worker/operational-workers/cmd/image-worker/
cp -r legacy_project/services/worker-service/services/workers/image-worker/internal/* \
  graph-worker/operational-workers/internal/processors/image/
```

---

### Step 3.4: Profile Worker (Day 14)

**NEW Worker Type** - Need to create from scratch

#### File: `operational-workers/internal/processors/profile/processor.go`

```go
package profile

import (
    "context"
    "encoding/json"
    "time"
    
    "github.com/fernandobarroso/common/queue"
)

type Processor struct {
    // Profile-specific processing logic
}

func (p *Processor) Process(ctx context.Context, msg *queue.Message) error {
    var profileMsg ProfileMessage
    if err := json.Unmarshal(msg.Payload, &profileMsg); err != nil {
        return err
    }
    
    // Simulate profile processing
    time.Sleep(5 * time.Second)
    
    return nil
}

func (p *Processor) Type() string {
    return "profile"
}
```

---

### Step 3.5: Go Module Setup (Day 15)

#### File: `operational-workers/go.mod`

> **Note:** Common queue package is copied into the project (self-contained), no replace directive needed.

```go
module github.com/fernandobarroso/microservices/operational-workers

go 1.22

require (
    github.com/gin-gonic/gin v1.10.1
    github.com/rabbitmq/amqp091-go v1.10.0
    github.com/prometheus/client_golang v1.22.0
    go.uber.org/zap v1.27.0
    github.com/google/uuid v1.6.0
)

// No replace directive needed - common/queue is copied into internal/common/queue
```

---

## 🔗 Phase 4: Integration Points (Days 16-20)

### Integration 1: API Service → RabbitMQ

**Already done in api-service!**

`api-service/internal/infrastructure/rabbitmq/publisher.go` publishes to:
- `profile.task`
- `email.send`
- `image.process`

**NEW:** Add `document.process` routing key

#### File: `api-service/internal/domain/task/model.go` (UPDATE)

Add to `DefaultRoutingMap`:
```go
"document.process": {
    Exchange:      "document-tasks",
    Queue:         "document-processing",
    TTL:           12 * time.Hour,
    Prefetch:      1,
    Durable:       true,
    AutoDelete:    false,
    Exclusive:     false,
    NoWait:        false,
    DeadLetterTTL: 7 * 24 * time.Hour,
    MaxRetries:    3,
    Description:   "Document processing for GraphRAG",
},
```

---

### Integration 2: Workers → Results Exchange (Optional)

If implementing results feedback:

#### GraphRAG Service: `src/worker/publisher.py`

```python
import pika
import json

class ResultPublisher:
    def publish_result(self, original_msg_id: str, result: dict):
        # Publish to results-exchange
        message = {
            'id': str(uuid.uuid4()),
            'type': 'document.completed',
            'original_message_id': original_msg_id,
            'payload': result,
            'timestamp': datetime.utcnow().isoformat()
        }
        
        self.channel.basic_publish(
            exchange='results-exchange',
            routing_key='',  # Fanout, no routing key needed
            body=json.dumps(message)
        )
```

#### API Service: Add Consumer (Future Enhancement)

Not in initial implementation - can add later when needed.

---

## 🧪 Phase 5: Testing (Days 21-25)

### Local Testing Setup

#### File: `graph-worker/docker-compose.yaml`

```yaml
version: '3.8'

services:
  postgres:
    image: postgres:15
    environment:
      POSTGRES_PASSWORD: postgres
    ports:
      - "5432:5432"
      
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
      
  rabbitmq:
    image: rabbitmq:3.12-management
    environment:
      RABBITMQ_DEFAULT_USER: guest
      RABBITMQ_DEFAULT_PASS: guest
    ports:
      - "5672:5672"
      - "15672:15672"
      
  mongodb:
    image: mongo:7
    environment:
      MONGO_INITDB_ROOT_USERNAME: admin
      MONGO_INITDB_ROOT_PASSWORD: password
    ports:
      - "27017:27017"
```

---

### Integration Tests

#### Test 1: GraphRAG End-to-End

```bash
# 1. Start infrastructure
docker-compose up -d

# 2. Start graphrag-service
cd graphrag-service
python cmd/main.py

# 3. Publish test message
python tests/publish_test_message.py

# 4. Verify processing
# - Check logs
# - Check MongoDB collections
# - Check metrics endpoint
```

---

#### Test 2: Operational Workers

```bash
# Start email worker
cd operational-workers
go run cmd/email-worker/main.go

# Publish test message to email queue
# Verify processing
```

---

#### Test 3: Full System

```bash
# 1. Deploy to local k8s (kind)
kubectl apply -f deployments/

# 2. Send request to api-service
curl -X POST http://api-service/api/v1/profiles/123/tasks/email \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"email_type": "welcome", "recipient": "test@example.com"}'

# 3. Verify message consumed by email-worker
kubectl logs -f deployment/email-worker

# 4. Check metrics
kubectl port-forward svc/email-worker 8080:8080
curl http://localhost:8080/metrics
```

---

## 📊 Migration Complexity Matrix

| Component | Complexity | Effort (Days) | Risk |
|-----------|-----------|---------------|------|
| Copy GraphRAG core | Low | 1 | Low |
| RabbitMQ consumer (Python) | Medium | 2 | Medium |
| Document processor wrapper | Medium | 2 | Medium |
| Health & metrics (Python) | Low | 1 | Low |
| Python Dockerfile | Low | 1 | Low |
| Python K8s manifests | Medium | 2 | Low |
| Copy Go common foundation | Low | 1 | Low |
| Email worker adaptation | Low | 1 | Low |
| Image worker adaptation | Low | 1 | Low |
| Profile worker creation | Medium | 2 | Medium |
| Go Dockerfiles | Low | 1 | Low |
| Go K8s manifests | Low | 1 | Low |
| Integration testing | High | 3 | High |
| Documentation | Medium | 2 | Low |
| **TOTAL** | | **20-25 days** | |

---

## 🎯 Success Metrics

### Development Success
- [ ] GraphRAG service consumes from RabbitMQ
- [ ] GraphRAG processes test document successfully
- [ ] All 3 Go workers consume from respective queues
- [ ] Health checks work for all workers
- [ ] Metrics exposed for all workers
- [ ] Graceful shutdown works

### Deployment Success
- [ ] All workers deploy to Kubernetes
- [ ] HPA configured and working
- [ ] All workers auto-recover from crashes
- [ ] RabbitMQ queues properly configured
- [ ] Monitoring dashboards show all workers

### Performance Success
- [ ] GraphRAG: <30 min average per document
- [ ] Email worker: >100 msg/sec
- [ ] Image worker: >10 msg/sec
- [ ] Queue depth stays <100 messages
- [ ] No message loss (DLQ working)

---

## 📚 Documentation Checklist

- [ ] `graph-worker/README.md` - System overview
- [ ] `graph-worker/ARCHITECTURE.md` - Detailed architecture
- [ ] `graphrag-service/README.md` - Python service guide
- [ ] `operational-workers/README.md` - Go workers guide
- [ ] `shared/contracts/MESSAGE_FORMAT.md` - Message schema
- [ ] `shared/contracts/ROUTING_KEYS.md` - Routing guide
- [ ] `shared/docs/DEPLOYMENT.md` - Deployment guide
- [ ] `shared/docs/TROUBLESHOOTING.md` - Common issues

---

## 🔄 Iterative Approach

### Iteration 1: Minimal Viable Workers (MVP)
- GraphRAG: Just consume messages, run pipeline, log results
- Go workers: Copy existing implementation as-is
- No results feedback loop
- No advanced error handling

### Iteration 2: Production-Ready
- Add results publishing
- Enhanced error handling
- Comprehensive monitoring
- Load testing and optimization

### Iteration 3: Advanced Features
- GraphRAG query API proxy in api-service
- Multi-document batch processing
- Advanced rate limiting
- Cost tracking and budgets

---

## 🚀 Quick Start Command Sequence

```bash
# Create structure
mkdir -p graph-worker/{graphrag-service/src,operational-workers/internal,shared}

# Copy GraphRAG (from legacy_project)
cp -r legacy_project/GraphRAG/src/* graph-worker/graphrag-service/src/graphrag/
cp legacy_project/GraphRAG/requirements.txt graph-worker/graphrag-service/requirements-graphrag.txt

# Add worker dependencies
cat graph-worker/graphrag-service/requirements-graphrag.txt > graph-worker/graphrag-service/requirements.txt
echo "aio-pika>=9.4.0" >> graph-worker/graphrag-service/requirements.txt
echo "prometheus-client>=0.20.0" >> graph-worker/graphrag-service/requirements.txt
echo "flask>=3.0.0" >> graph-worker/graphrag-service/requirements.txt

# Copy workers (from legacy_project)
cp -r legacy_project/services/worker-service/services/workers/common \
  graph-worker/operational-workers/internal/
cp -r legacy_project/services/common/queue \
  graph-worker/operational-workers/internal/common/queue
cp -r legacy_project/services/worker-service/services/workers/email-worker/cmd/* \
  graph-worker/operational-workers/cmd/email-worker/
cp -r legacy_project/services/worker-service/services/workers/image-worker/cmd/* \
  graph-worker/operational-workers/cmd/image-worker/

# Initialize modules
cd graph-worker/graphrag-service && pip install -r requirements.txt
cd ../operational-workers && go mod init github.com/fernandobarroso/microservices/operational-workers && go mod tidy

# Start local dev
cd ../..
docker-compose -f graph-worker/docker-compose.yaml up -d
```

---

## ⏳ Deferred Decision Reminder

**GraphRAG Service Role** - Decide after Phase 2 is complete:
- Option A: Worker only (recommended for Phase 1)
- Option B: Worker + API
- Option C: Separate services

See [BRAINSTORM_GRAPH_WORKER_ARCHITECTURE.md](../BRAINSTORM_GRAPH_WORKER_ARCHITECTURE.md) for rationale.

---

*See [BRAINSTORM_GRAPH_WORKER_ARCHITECTURE.md](../BRAINSTORM_GRAPH_WORKER_ARCHITECTURE.md) for full analysis*
