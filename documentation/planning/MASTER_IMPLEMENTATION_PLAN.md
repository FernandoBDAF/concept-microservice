# Master Implementation Plan - Microservices Architecture V2

**Date:** January 30, 2026  
**Status:** Comprehensive Roadmap  
**Purpose:** Central document consolidating all planning and implementation phases

---

## Executive Summary

This master plan consolidates all architectural decisions and implementation phases for the microservices architecture V2. The goal is to create a production-ready, cloud-native system with:

1. **Consolidated API Gateway** (`api-service`) - Go service with direct infrastructure access
2. **Authentication Service** (`auth-service`) - TypeScript/Node.js JWT-based auth
3. **GraphRAG Worker** (`graphrag-service`) - Python AI/knowledge graph processing
4. **Operational Workers** (`operational-workers`) - Go task processors (email, image, profile)
5. **Unified Deployment** (`deployment/`) - Kubernetes manifests and automation
6. **Infrastructure** - PostgreSQL, Redis, RabbitMQ, MinIO, MongoDB Atlas

---

## Table of Contents

1. [Complete System Architecture](#1-complete-system-architecture)
2. [Current Implementation Status](#2-current-implementation-status)
3. [Individual Implementation Plans](#individual-implementation-plans)
4. [Implementation Phases](#3-implementation-phases)
5. [Infrastructure Components](#4-infrastructure-components)
6. [Service Communication Matrix](#5-service-communication-matrix)
7. [Missing Pieces & Gaps](#6-missing-pieces--gaps)
8. [Open Questions & Studies](#7-open-questions--studies)
9. [Risk Assessment](#8-risk-assessment)
10. [Success Criteria](#9-success-criteria)
11. [Timeline & Resource Allocation](#10-timeline--resource-allocation)

---

## Individual Implementation Plans

Each project has its own detailed implementation plan. Use these for individual session work:

| Project | Plan Location | Scope | Dependencies |
|---------|---------------|-------|--------------|
| **api-service** | [`api-service/IMPLEMENTATION_PLAN.md`](api-service/IMPLEMENTATION_PLAN.md) | Document upload, MinIO integration | MinIO, PostgreSQL |
| **auth-service** | [`auth-service/IMPLEMENTATION_PLAN.md`](auth-service/IMPLEMENTATION_PLAN.md) | Copy & adapt from legacy | PostgreSQL (auth) |
| **graphrag-service** | [`graph-worker/graphrag-service/IMPLEMENTATION_PLAN.md`](graph-worker/graphrag-service/IMPLEMENTATION_PLAN.md) | Python worker for document processing | RabbitMQ, MinIO, MongoDB Atlas |
| **operational-workers** | [`graph-worker/operational-workers/IMPLEMENTATION_PLAN.md`](graph-worker/operational-workers/IMPLEMENTATION_PLAN.md) | Go workers (email, image, profile) | RabbitMQ |
| **deployment** | [`deployment/IMPLEMENTATION_PLAN.md`](deployment/IMPLEMENTATION_PLAN.md) | Kubernetes manifests, scripts | All infrastructure |

### Implementation Order

```
1. deployment/        → Create infrastructure manifests first
2. auth-service/      → Copy and adapt (no code dependencies)
3. api-service/       → Add document upload (depends on MinIO)
4. graphrag-service/  → Python worker (depends on RabbitMQ, MinIO, MongoDB)
5. operational-workers/ → Go workers (depends on RabbitMQ)
```

### Cross-Project Concerns (Remain in Master Plan)

The following topics span multiple projects and are documented in this master plan:

- **Service Communication Matrix** (Section 5) - How services interact
- **RabbitMQ Topology** - Exchanges, queues, routing keys
- **Network Policies** - Kubernetes network rules
- **Message Contracts** - Shared message formats
- **Environment Variables** - Cross-service configuration
- **Integration Testing** - End-to-end flows
- **Success Criteria** - System-wide metrics

---

## 1. Complete System Architecture

### 1.1 System Overview Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│                         External Clients                            │
│                    (Web, Mobile, CLI, APIs)                          │
└─────────────────┬───────────────────────────────────────────────────┘
                  │
                  │ HTTPS (Ingress)
                  ▼
         ┌────────────────────┐
         │  NGINX Ingress     │
         │  Controller        │
         └────────┬───────────┘
                  │
        ┌─────────┴────────────────┐
        │                          │
        ▼                          ▼
┌──────────────────┐      ┌──────────────────┐
│  API Service     │─────▶│  Auth Service    │
│  (Go)            │      │  (Node.js)       │
│  Port: 8080      │      │  Port: 8080      │
│                  │      │                  │
│  - Profile CRUD  │      │  - JWT Auth      │
│  - Task Submit   │      │  - User Mgmt     │
│  - Doc Upload    │      │  - Token Valid   │
└────┬─────┬───┬──┘      └────────┬─────────┘
     │     │   │                  │
     │     │   │                  ▼
     │     │   │         ┌──────────────────┐
     │     │   │         │  PostgreSQL      │
     │     │   │         │  (auth-postgres) │
     │     │   │         │  Users, Audit    │
     │     │   │         └──────────────────┘
     │     │   │
     │     │   └────────────────────┐
     │     │                        │
     ▼     ▼                        ▼
┌─────┐ ┌───────┐            ┌──────────┐
│Redis│ │Postgres│            │ RabbitMQ │
│Cache│ │api-db  │            │          │
└─────┘ └────────┘            └────┬─────┘
                                   │
        ┌──────────────────────────┼──────────────────────────┐
        │                          │                          │
        ▼                          ▼                          ▼
┌─────────────────┐      ┌─────────────────┐      ┌─────────────────┐
│  GraphRAG       │      │  Email Worker   │      │  Image Worker   │
│  Service        │      │  (Go)           │      │  (Go)           │
│  (Python)       │      │  Port: 8080     │      │  Port: 8080     │
│  Ports: 8080/81 │      └─────────────────┘      └─────────────────┘
│                 │              │
│  - Doc Process  │              ▼
│  - Entity Extr  │      ┌─────────────────┐
│  - Graph Build  │      │  Profile Worker │
└────┬────┬───────┘      │  (Go)           │
     │    │               │  Port: 8080     │
     │    │               └─────────────────┘
     │    │
     ▼    ▼
┌────────┐ ┌──────────┐
│MongoDB │ │  MinIO   │
│ Atlas  │ │  (K8s)   │
│        │ │          │
│Entities│ │Documents │
│Rels    │ │Storage   │
└────────┘ └──────────┘
```

### 1.2 Service Inventory

| Service | Language | Status | Purpose |
|---------|----------|--------|---------|
| **api-service** | Go | ✅ Implemented | API gateway, profile CRUD, task submission |
| **auth-service** | Node.js | 🔄 To copy | JWT authentication, user management |
| **graphrag-service** | Python | 📋 Planned | Document processing, knowledge graphs |
| **email-worker** | Go | 📋 Planned | Email task processing |
| **image-worker** | Go | 📋 Planned | Image task processing |
| **profile-worker** | Go | 📋 Planned | Profile task processing |

### 1.3 Infrastructure Inventory

| Component | Type | Status | Purpose |
|-----------|------|--------|---------|
| **PostgreSQL (api)** | StatefulSet | 📋 To create | Profiles, documents metadata |
| **PostgreSQL (auth)** | StatefulSet | 📋 To create | Users, audit logs |
| **Redis** | StatefulSet | 📋 To create | Caching layer |
| **RabbitMQ** | StatefulSet | 📋 To create | Message broker |
| **MinIO** | StatefulSet | 📋 To create | Document object storage |
| **MongoDB** | External (Atlas) | ⚙️ To configure | GraphRAG knowledge graphs |
| **Ingress Controller** | Deployment | 📋 To create | NGINX ingress |
| **Metrics Server** | Deployment | 📋 To create | HPA support |

---

## 2. Current Implementation Status

### 2.1 Completed Components ✅

#### api-service (Consolidated Go Service)
- **Status:** ✅ Fully implemented
- **Source:** `api-service/`
- **Features:**
  - Direct PostgreSQL access (profiles)
  - Direct Redis caching
  - Direct RabbitMQ publishing
  - Auth integration via HTTP
  - Profile CRUD API
  - Task submission endpoints
  - Health checks and metrics
- **Missing:** Document upload endpoint, MinIO integration

#### GraphRAG Library (Reference)
- **Status:** ✅ Available for reference
- **Source:** `legacy_project/GraphRAG/` (576 files)
- **Features:**
  - Ingestion pipeline (async)
  - GraphRAG pipeline (async)
  - Entity extraction
  - Relationship detection
  - Community analysis
  - FastAPI query APIs
- **Integration:** Will be copied to graphrag-service with RabbitMQ consumer wrapper

### 2.2 Planned Components 📋

#### auth-service
- **Status:** 📋 Ready to copy from legacy
- **Source:** `legacy_project/services/auth-service/`
- **Features:** JWT auth, user management, audit logs
- **Changes:** Minimal (env defaults, README)
- **Detailed Plan:** [`auth-service/IMPLEMENTATION_PLAN.md`](auth-service/IMPLEMENTATION_PLAN.md)

#### graph-worker
- **Status:** 📋 Architecture planned
- **Source:** Planning documents complete
- **Components:**
  - `graphrag-service/` - Python worker → [`graph-worker/graphrag-service/IMPLEMENTATION_PLAN.md`](graph-worker/graphrag-service/IMPLEMENTATION_PLAN.md)
  - `operational-workers/` - Go workers (email, image, profile) → [`graph-worker/operational-workers/IMPLEMENTATION_PLAN.md`](graph-worker/operational-workers/IMPLEMENTATION_PLAN.md)
  - `shared/` - Contracts and configs

#### deployment
- **Status:** 📋 Structure planned
- **Source:** Based on `legacy_project/k8s/`
- **Components:**
  - Infrastructure manifests
  - Service deployments
  - Automation scripts
- **Detailed Plan:** [`deployment/IMPLEMENTATION_PLAN.md`](deployment/IMPLEMENTATION_PLAN.md)

### 2.3 Reference Materials 📚

#### legacy_project/
- All original microservices (7 services)
- K8s configurations (121 files)
- Reference documentation (278 files)
- Status: ✅ Preserved for reference

---

## 3. Implementation Phases

### Phase 1: Auth Service & Deployment Structure (Week 1)

**Goal:** Create auth-service and deployment/ folder

#### Phase 1.1: Auth Service Creation (Days 1-2)

**Tasks:**
- [ ] Create `auth-service/` directory
- [ ] Copy source code from `legacy_project/services/auth-service/`
- [ ] Update `src/config/env.ts` default hostnames
  ```typescript
  DATABASE_HOST: z.string().default('postgres-auth'),
  API_SERVICE_URL: z.string().url().optional().default('http://api-service:8080'),
  ```
- [ ] Update `README.md` with new architecture context
- [ ] Test locally with Docker Compose
- [ ] Build Docker image

**Files to Copy:**
```bash
# From repository root
cp -r legacy_project/services/auth-service/ auth-service/

# Remove legacy files not needed
rm -rf auth-service/src/service/      # Old JS version
rm -rf auth-service/src/routes/       # Old JS version
rm -rf auth-service/src/controllers/  # Old JS version
rm -rf auth-service/src/models/       # Old JS version
rm -rf auth-service/src/repository/   # Old JS version
rm -rf auth-service/src/server.js     # Old JS entry
rm -rf auth-service/src/config/config.js  # Old JS config
rm -rf auth-service/deployments/      # Will be in central deployment/

# Keep only TypeScript implementation
```

**Deliverables:**
- ✅ `auth-service/` folder with TypeScript code
- ✅ Updated configuration
- ✅ Docker image: `auth-service:latest`

#### Phase 1.2: Deployment Structure Creation (Days 2-4)

**Tasks:**
- [ ] Create `deployment/` directory structure
- [ ] Copy and adapt cluster setup from legacy
- [ ] Create infrastructure manifests (PostgreSQL, Redis, RabbitMQ, MinIO)
- [ ] Create service manifests (auth-service, api-service)
- [ ] Create scripts (deploy, build, test)
- [ ] Test in Kind cluster

**Directory Structure:**
```bash
mkdir -p deployment/{cluster/{infrastructure,validation},services,external,ingress,monitoring,scripts}
mkdir -p deployment/services/{01-infrastructure/{postgres-api,postgres-auth,redis,rabbitmq,minio},02-auth-service,03-api-service}
mkdir -p deployment/external/mongodb-atlas
mkdir -p deployment/scripts/test
```

**Files to Create:**
- Infrastructure: 5 StatefulSets (PostgreSQL×2, Redis, RabbitMQ, MinIO)
- Services: 2 Deployments (auth-service, api-service)
- Networking: Services, NetworkPolicies, Ingress
- Configuration: ConfigMaps, Secrets
- Automation: 8-10 shell scripts

**Deliverables:**
- ✅ Complete `deployment/` folder structure
- ✅ All infrastructure and service manifests
- ✅ Automation scripts
- ✅ Kind cluster tested and validated

**Milestone:** ✨ **Auth-service and api-service running in Kubernetes**

---

### Phase 2: API Service Enhancements (Week 2)

**Goal:** Add document upload capability to api-service

#### Phase 2.1: MinIO Client Integration (Day 5)

**Location:** `api-service/internal/infrastructure/minio/`

**Tasks:**
- [ ] Add MinIO dependencies to `go.mod`
  ```go
  github.com/minio/minio-go/v7 v7.0.66
  github.com/gabriel-vasile/mimetype v1.4.3  // MIME detection
  ```
- [ ] Create `client.go` - MinIO client wrapper
- [ ] Create `uploader.go` - Document upload logic
- [ ] Add configuration in `internal/config/config.go`
  ```go
  type MinIOConfig struct {
      Endpoint        string
      AccessKeyID     string
      SecretAccessKey string
      UseSSL          bool
      BucketName      string
  }
  ```
- [ ] Test local MinIO connection

**Deliverables:**
- ✅ MinIO client integration
- ✅ Document upload/download methods
- ✅ Unit tests

#### Phase 2.2: Document Domain Layer (Days 6-7)

**Location:** `api-service/internal/domain/document/`

**Tasks:**
- [ ] Create `model.go` - Document, Chunk, Entity, Relationship structs
- [ ] Create `repository.go` - Document repository interface
- [ ] Create `service.go` - Document business logic
- [ ] Create `api-service/internal/infrastructure/postgres/document_repository.go`
- [ ] Create database migration `000002_create_documents.up.sql`

**Migration Schema:**
```sql
-- Documents table
CREATE TABLE documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    profile_id UUID NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    user_id UUID NOT NULL,
    filename VARCHAR(255) NOT NULL,
    original_filename VARCHAR(255) NOT NULL,
    file_type VARCHAR(50) NOT NULL,
    file_size BIGINT NOT NULL,
    storage_path TEXT NOT NULL,
    storage_bucket VARCHAR(100) NOT NULL,
    mime_type VARCHAR(100),
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    processing_started_at TIMESTAMP WITH TIME ZONE,
    processing_completed_at TIMESTAMP WITH TIME ZONE,
    error_message TEXT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_documents_profile_id ON documents(profile_id);
CREATE INDEX idx_documents_status ON documents(status);
```

**Deliverables:**
- ✅ Document domain models
- ✅ Document service
- ✅ Database migration
- ✅ Repository implementation

#### Phase 2.3: Document Upload API (Day 8)

**Location:** `api-service/internal/api/handlers/document.go`

**Tasks:**
- [ ] Create `DocumentHandler` with upload endpoint
- [ ] Implement multipart/form-data handling
- [ ] Add file validation (size, type, magic bytes)
- [ ] Integrate MinIO upload
- [ ] Store metadata in PostgreSQL
- [ ] Publish `document.process` message to RabbitMQ
- [ ] Add routes to `router.go`

**New Endpoints:**
```
POST   /api/v1/documents/upload              Upload document
GET    /api/v1/documents/:id                 Get document details
GET    /api/v1/documents/:id/status          Get processing status
GET    /api/v1/documents/:id/download        Download document
DELETE /api/v1/documents/:id                 Delete document
GET    /api/v1/profiles/:id/documents        List profile documents
```

**Deliverables:**
- ✅ Document upload endpoint
- ✅ Document retrieval endpoints
- ✅ Integration tests
- ✅ API documentation

#### Phase 2.4: RabbitMQ Routing Configuration (Day 8)

**Location:** `api-service/internal/domain/task/model.go`

**Tasks:**
- [ ] Add `document.process` routing key
  ```go
  "document.process": {
      Exchange:      "document-tasks",
      Queue:         "document-processing",
      TTL:           12 * time.Hour,
      Prefetch:      1,
      Durable:       true,
      MaxRetries:    3,
      Description:   "Document processing for GraphRAG",
  }
  ```
- [ ] Update RabbitMQ topology in deployment
- [ ] Test message publishing

**Deliverables:**
- ✅ Document routing key configured
- ✅ RabbitMQ tested

**Milestone:** ✨ **Document upload flow complete (api-service → MinIO → RabbitMQ)**

---

### Phase 3: GraphRAG Service (Weeks 3-4)

**Goal:** Python worker for document processing and knowledge graph construction

#### Phase 3.1: Project Setup (Days 9-10)

**Tasks:**
- [ ] Create `graph-worker/graphrag-service/` structure
- [ ] Copy GraphRAG core from `legacy_project/GraphRAG/src/`
  ```bash
  cp -r legacy_project/GraphRAG/src/domain graph-worker/graphrag-service/src/graphrag/
  cp -r legacy_project/GraphRAG/src/core graph-worker/graphrag-service/src/graphrag/
  cp -r legacy_project/GraphRAG/src/infrastructure graph-worker/graphrag-service/src/graphrag/
  cp -r legacy_project/GraphRAG/src/app graph-worker/graphrag-service/src/graphrag/
  cp -r legacy_project/GraphRAG/src/lib graph-worker/graphrag-service/src/graphrag/
  ```
- [ ] Merge `requirements.txt` and add worker dependencies:
  ```
  aio-pika>=9.4.0
  prometheus-client>=0.20.0
  flask>=3.0.0
  ```
- [ ] Test GraphRAG imports

**Deliverables:**
- ✅ GraphRAG core copied
- ✅ Dependencies installed
- ✅ Import verification passed

#### Phase 3.2: RabbitMQ Consumer (Days 11-12)

**Location:** `graphrag-service/src/worker/`

**Tasks:**
- [ ] Create `consumer.py` - Async RabbitMQ consumer (aio-pika)
- [ ] Create `processor.py` - Async document processor
- [ ] Create `base_worker.py` - Worker lifecycle management
- [ ] Implement graceful shutdown

**Key Implementation:**
```python
# Async consumer using aio-pika (matches GraphRAG async pipelines)
class AsyncRabbitMQConsumer:
    async def consume(self, handler):
        queue = await self.connect()
        async with queue.iterator() as queue_iter:
            async for message in queue_iter:
                async with message.process():
                    payload = json.loads(message.body)
                    await handler(payload)  # Async handler for GraphRAG
```

**Deliverables:**
- ✅ Async RabbitMQ consumer
- ✅ Document processor wrapper
- ✅ Worker base class

#### Phase 3.3: Monitoring & Configuration (Day 13)

**Location:** `graphrag-service/src/monitoring/` and `src/config/`

**Tasks:**
- [ ] Create `health.py` - Flask health check server (port 8080)
- [ ] Create `metrics.py` - Prometheus metrics (port 8081)
- [ ] Create `worker_config.py` - Environment configuration
- [ ] Create `cmd/main.py` - Worker entry point

**Deliverables:**
- ✅ Health endpoints (/health, /ready)
- ✅ Prometheus metrics
- ✅ Configuration management

#### Phase 3.4: Containerization (Day 14)

**Tasks:**
- [ ] Create `Dockerfile` (multi-stage Python build)
- [ ] Build Docker image
- [ ] Test image locally
- [ ] Push to registry

**Dockerfile Template:**
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

**Deliverables:**
- ✅ Docker image: `graphrag-service:latest`
- ✅ Image tested locally

#### Phase 3.5: Kubernetes Deployment (Days 15-16)

**Location:** `deployment/services/04-graphrag-service/`

**Tasks:**
- [ ] Create `deployment.yaml` - Deployment manifest
- [ ] Create `service.yaml` - Service for health/metrics
- [ ] Create `configmap.yaml` - Configuration
- [ ] Create `secret.yaml` - MongoDB, OpenAI, MinIO credentials
- [ ] Create `hpa.yaml` - Horizontal Pod Autoscaler
- [ ] Deploy to Kind cluster
- [ ] Test end-to-end: Upload → Queue → Worker → MongoDB

**Resource Allocation:**
```yaml
resources:
  requests:
    cpu: 2000m
    memory: 6Gi
  limits:
    cpu: 4000m
    memory: 10Gi
```

**Deliverables:**
- ✅ GraphRAG service deployed in Kubernetes
- ✅ End-to-end document processing works
- ✅ MongoDB Atlas connected

**Milestone:** ✨ **GraphRAG document processing fully operational**

---

### Phase 4: Operational Workers (Week 5)

**Goal:** Go workers for email, image, and profile task processing

#### Phase 4.1: Common Foundation (Days 17-18)

**Location:** `graph-worker/operational-workers/internal/common/`

**Tasks:**
- [ ] Copy worker common foundation
  ```bash
  cp -r legacy_project/services/worker-service/services/workers/common/* \
    graph-worker/operational-workers/internal/common/
  ```
- [ ] Copy queue package (self-contained)
  ```bash
  cp -r legacy_project/services/common/queue \
    graph-worker/operational-workers/internal/common/queue
  ```
- [ ] Update all import paths
  ```
  OLD: github.com/fernandobarroso/common/queue
  NEW: github.com/fernandobarroso/microservices/operational-workers/internal/common/queue
  ```
- [ ] Initialize Go module
  ```bash
  cd graph-worker/operational-workers
  go mod init github.com/fernandobarroso/microservices/operational-workers
  go mod tidy
  ```

**Deliverables:**
- ✅ Common worker framework copied
- ✅ Queue package integrated
- ✅ Module initialized

#### Phase 4.2: Email Worker (Day 19)

**Tasks:**
- [ ] Copy email worker from legacy
- [ ] Update imports
- [ ] Create `Dockerfile.email`
- [ ] Create Kubernetes manifests
- [ ] Deploy and test

**Deliverables:**
- ✅ Email worker operational
- ✅ Processes `email.send` messages

#### Phase 4.3: Image Worker (Day 20)

**Tasks:**
- [ ] Copy image worker from legacy
- [ ] Update imports
- [ ] Create `Dockerfile.image`
- [ ] Create Kubernetes manifests
- [ ] Deploy and test

**Deliverables:**
- ✅ Image worker operational
- ✅ Processes `image.process` messages

#### Phase 4.4: Profile Worker (Day 21)

**Tasks:**
- [ ] Create profile worker (NEW - based on email/image pattern)
- [ ] Implement profile task processor
- [ ] Create `Dockerfile.profile`
- [ ] Create Kubernetes manifests
- [ ] Deploy and test

**Deliverables:**
- ✅ Profile worker operational
- ✅ Processes `profile.task` messages

**Milestone:** ✨ **All workers deployed and processing tasks**

---

### Phase 5: Integration & Testing (Week 6)

**Goal:** End-to-end testing and system validation

#### Phase 5.1: Infrastructure Validation (Days 22-23)

**Tasks:**
- [ ] Verify all StatefulSets healthy
- [ ] Test database migrations
- [ ] Test RabbitMQ queue topology
- [ ] Test MinIO bucket creation
- [ ] Test MongoDB Atlas connectivity
- [ ] Test Redis caching
- [ ] Run health checks on all services

**Test Script:**
```bash
# deployment/scripts/validate-infrastructure.sh
./deployment/cluster/validation/verify-infrastructure.sh
```

**Deliverables:**
- ✅ All infrastructure components validated
- ✅ Connectivity tests passed

#### Phase 5.2: Service Integration Testing (Days 24-25)

**Tasks:**
- [ ] Test auth flow: Login → Get token → API call
- [ ] Test profile CRUD: Create → Read → Update → Delete
- [ ] Test task submission: API → RabbitMQ → Worker
- [ ] Test document upload: Upload → Store → Queue → Process → Store results
- [ ] Test cross-service communication
- [ ] Monitor metrics and logs

**Integration Test Scenarios:**

**Scenario 1: Profile with Task**
```bash
# 1. Login
TOKEN=$(curl -X POST http://api-service/v1/auth/login \
  -d '{"email":"user@example.com","password":"pass"}' | jq -r .access_token)

# 2. Create profile
PROFILE_ID=$(curl -X POST http://api-service/api/v1/profiles \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"first_name":"John","last_name":"Doe","email":"john@example.com"}' | jq -r .id)

# 3. Submit email task
curl -X POST http://api-service/api/v1/profiles/$PROFILE_ID/tasks/email \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"email_type":"welcome","recipient":"john@example.com"}'

# 4. Verify email worker processed it
kubectl logs -l app=email-worker --tail=20
```

**Scenario 2: Document Upload and Processing**
```bash
# 1. Upload document
DOC_ID=$(curl -X POST http://api-service/api/v1/documents/upload \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@test.pdf" | jq -r .document_id)

# 2. Check processing status
curl http://api-service/api/v1/documents/$DOC_ID/status \
  -H "Authorization: Bearer $TOKEN"

# 3. Monitor graphrag-service logs
kubectl logs -l app=graphrag-service -f

# 4. Verify in MongoDB Atlas
# Check entities and relationships collections
```

**Deliverables:**
- ✅ All integration tests passing
- ✅ End-to-end flows validated
- ✅ Performance benchmarks captured

#### Phase 5.3: Load Testing (Days 26-27)

**Tasks:**
- [ ] Create K6 load test scripts
- [ ] Test profile API (1000 concurrent users)
- [ ] Test document upload (100 concurrent uploads)
- [ ] Test worker throughput (queue depth monitoring)
- [ ] Identify bottlenecks
- [ ] Optimize as needed

**Load Test Targets:**

| Endpoint | Target RPS | P95 Latency | Success Rate |
|----------|-----------|-------------|--------------|
| GET /api/v1/profiles | 1000 | <100ms | >99.5% |
| POST /api/v1/profiles | 100 | <200ms | >99% |
| POST /api/v1/documents/upload | 50 | <1s | >95% |
| POST /api/v1/tasks/* | 500 | <100ms | >99% |

**Deliverables:**
- ✅ Load test results
- ✅ Performance baseline documented
- ✅ Bottlenecks identified and addressed

**Milestone:** ✨ **System validated and performance benchmarked**

---

### Phase 6: Shared Contracts & Documentation (Week 7)

**Goal:** Finalize contracts and comprehensive documentation

#### Phase 6.1: Shared Contracts (Days 28-29)

**Location:** `graph-worker/shared/contracts/`

**Tasks:**
- [ ] Create `MESSAGE_FORMAT.md` - Standard message schema
- [ ] Create `ROUTING_KEYS.md` - All routing keys documented
- [ ] Create `ERROR_CODES.md` - Error code catalog
- [ ] Create `DATA_OWNERSHIP.md` - Data boundaries

**Message Format Standard:**
```json
{
  "id": "uuid-v4",
  "type": "document.process|email.send|image.process|profile.task",
  "timestamp": "2026-01-30T10:30:00Z",
  "correlation_id": "uuid-v4",
  "payload": {
    // Worker-specific fields
  },
  "metadata": {
    "user_id": "string",
    "source": "api-service",
    "priority": 0
  }
}
```

**Data Ownership Matrix:**

| Data Type | Owner | Storage | Access |
|-----------|-------|---------|--------|
| Profiles | api-service | PostgreSQL (api) | CRUD via API |
| Users | auth-service | PostgreSQL (auth) | Auth API |
| Documents metadata | api-service | PostgreSQL (api) | Via API |
| Document files | MinIO | Object storage | S3 API |
| Entities | graphrag-service | MongoDB Atlas | Write by worker |
| Relationships | graphrag-service | MongoDB Atlas | Write by worker |
| Communities | graphrag-service | MongoDB Atlas | Write by worker |

**Deliverables:**
- ✅ Complete contract documentation
- ✅ Data ownership clarified
- ✅ Error codes cataloged

#### Phase 6.2: System Documentation (Days 30-31)

**Tasks:**
- [ ] Create `graph-worker/README.md` - System overview
- [ ] Create `graphrag-service/README.md` - Python service guide
- [ ] Create `operational-workers/README.md` - Go workers guide
- [ ] Update root `README.md` - Complete system
- [ ] Create `deployment/README.md` - Deployment guide
- [ ] Create architecture diagrams
- [ ] Create runbook for operations

**Documentation Structure:**
```
documentation/
├── architecture/
│   ├── system-overview.md          # Complete architecture
│   ├── api-service.md              # Go gateway
│   ├── auth-service.md             # Authentication
│   ├── graph-worker.md             # Worker system
│   └── infrastructure.md           # Databases, queues, storage
├── api/
│   ├── rest-api-reference.md       # Complete API docs
│   ├── authentication.md           # Auth flow
│   └── message-contracts.md        # RabbitMQ messages
├── deployment/
│   ├── kubernetes.md               # K8s deployment
│   ├── configuration.md            # Environment variables
│   ├── monitoring.md               # Observability
│   └── troubleshooting.md          # Common issues
└── development/
    ├── getting-started.md          # Local development
    ├── contributing.md             # Contribution guide
    └── testing.md                  # Test strategy
```

**Deliverables:**
- ✅ Complete system documentation
- ✅ Deployment guides
- ✅ Operational runbook

**Milestone:** ✨ **Complete Phase 1 Implementation (Basic System Operational)**

---

### Phase 7: Advanced Features (Weeks 8-9)

**Goal:** Enhance system with advanced capabilities

#### Phase 7.1: Results Feedback Loop (Days 32-34)

**Tasks:**
- [ ] Add results exchange to RabbitMQ topology
- [ ] Implement result publishing in workers
  ```python
  # graphrag-service publishes completion
  await publisher.publish('results-exchange', {
      'type': 'document.completed',
      'document_id': doc_id,
      'entities_count': 5000,
      'relationships_count': 15000
  })
  ```
- [ ] Add result consumer to api-service
- [ ] Update document status on completion
- [ ] Add webhook notifications (optional)

**Deliverables:**
- ✅ Workers publish results
- ✅ API service consumes results
- ✅ Status updates work

#### Phase 7.2: Query APIs for Knowledge Graph (Days 35-37)

**Decision Point:** Choose GraphRAG service role

**Option A: Proxy via api-service (Recommended)**
- [ ] Add graph query endpoints to api-service
- [ ] Create MongoDB client in api-service
- [ ] Proxy queries to MongoDB
- [ ] Implement caching for queries

**Option B: Direct GraphRAG API exposure**
- [ ] Enable GraphRAG's FastAPI Graph API
- [ ] Expose via Kubernetes service
- [ ] Add to ingress configuration

**Endpoints:**
```
GET  /api/v1/documents/:id/entities
GET  /api/v1/documents/:id/relationships
POST /api/v1/documents/:id/search        # Semantic search
GET  /api/v1/documents/:id/graph         # Graph visualization data
```

**Deliverables:**
- ✅ Graph query capability
- ✅ Semantic search working
- ✅ Graph visualization data available

#### Phase 7.3: Concurrency Optimizations (Days 38-40)

**High-Impact Optimizations:**

1. **Concurrent Document Chunk Processing** (Day 38)
   - Implement worker pool for GraphRAG chunks
   - Process 5 chunks in parallel
   - **Impact:** 5x speedup (15min → 3min)

2. **Parallel Profile List Enrichment** (Day 39)
   - Batch queries for document/entity counts
   - Parallel data fetching
   - **Impact:** 25x speedup (500ms → 20ms)

3. **Worker Message Concurrency** (Day 40)
   - Add configurable concurrency to BaseWorker
   - Email: 10 concurrent, Image: 2 concurrent, Profile: 5 concurrent
   - **Impact:** 2-10x throughput improvement

**Deliverables:**
- ✅ GraphRAG processing 5x faster
- ✅ List endpoints 25x faster
- ✅ Worker throughput 2-10x higher

**Milestone:** ✨ **Phase 2 Complete (Advanced Features & Optimization)**

---

### Phase 8: Production Readiness (Week 10)

**Goal:** Prepare system for production deployment

#### Phase 8.1: Security Hardening (Days 41-42)

**Tasks:**
- [ ] Review and apply network policies
- [ ] Configure RBAC for Kubernetes
- [ ] Enable TLS for ingress
- [ ] Rotate and secure all secrets
- [ ] Enable audit logging
- [ ] Run security scan (Trivy, Grype)

**Deliverables:**
- ✅ Security scan passed
- ✅ Network policies enforced
- ✅ TLS enabled

#### Phase 8.2: Monitoring & Alerting (Days 43-44)

**Tasks:**
- [ ] Deploy Prometheus + Grafana
- [ ] Create service dashboards
- [ ] Configure alerting rules
- [ ] Set up log aggregation
- [ ] Create SLO/SLI definitions
- [ ] Test alert firing

**Key Metrics to Monitor:**
- Service availability (uptime)
- API latency (p50, p95, p99)
- Error rates
- Queue depths
- Worker throughput
- Resource utilization
- Cost metrics (LLM API calls)

**Deliverables:**
- ✅ Monitoring dashboards deployed
- ✅ Alerting configured
- ✅ SLOs defined

#### Phase 8.3: Disaster Recovery (Day 45)

**Tasks:**
- [ ] Configure database backups (PostgreSQL)
- [ ] Configure object storage backups (MinIO)
- [ ] Document recovery procedures
- [ ] Test backup restoration
- [ ] Create DR runbook

**Deliverables:**
- ✅ Backup strategy implemented
- ✅ Recovery procedures documented
- ✅ DR tested

**Milestone:** ✨ **Production Ready**

---

## 4. Infrastructure Components

### 4.1 Databases

#### PostgreSQL (api-postgres)
- **Purpose:** Profiles, documents metadata, entities, relationships
- **Version:** PostgreSQL 15
- **Resources:** 1-2 cores, 2-4Gi memory, 50Gi storage
- **Extensions:** pgvector (for embeddings)
- **Backup:** Daily snapshots, 7-day retention
- **Schema:** Profiles, documents, document_chunks, entities, relationships, embeddings

#### PostgreSQL (auth-postgres)
- **Purpose:** Users, audit logs, sessions
- **Version:** PostgreSQL 15
- **Resources:** 500m-1 core, 1-2Gi memory, 10Gi storage
- **Backup:** Daily snapshots, 30-day retention
- **Schema:** Users, auth_audit_logs

#### MongoDB Atlas
- **Purpose:** GraphRAG knowledge graph (communities, advanced graph structures)
- **Tier:** M10 or higher (dedicated)
- **Region:** Same as Kubernetes cluster
- **Features:** Vector search, auto-scaling
- **Backup:** Continuous via Atlas
- **Estimated Cost:** $57-100/month

### 4.2 Caching & Messaging

#### Redis
- **Purpose:** Caching layer for api-service
- **Version:** Redis 7
- **Resources:** 500m CPU, 512Mi-1Gi memory
- **Persistence:** AOF enabled
- **Eviction:** allkeys-lru
- **Max Memory:** 500MB

#### RabbitMQ
- **Purpose:** Message broker for async task processing
- **Version:** RabbitMQ 3.13
- **Resources:** 1 core, 1-2Gi memory, 5Gi storage
- **Features:** Management plugin, Prometheus plugin
- **Ports:** 5672 (AMQP), 15672 (Management), 15692 (Metrics)

#### MinIO
- **Purpose:** S3-compatible object storage for documents
- **Version:** Latest stable
- **Resources:** 500m-1 core, 1-2Gi memory, 100Gi storage
- **Ports:** 9000 (S3 API), 9001 (Console)
- **Buckets:** documents-raw, documents-processed

### 4.3 Infrastructure Resource Summary

| Component | CPU | Memory | Storage | Cost Estimate |
|-----------|-----|--------|---------|---------------|
| postgres-api | 1-2 cores | 2-4Gi | 50Gi | Included |
| postgres-auth | 500m-1 | 1-2Gi | 10Gi | Included |
| redis | 500m | 512Mi-1Gi | - | Included |
| rabbitmq | 1 core | 1-2Gi | 5Gi | Included |
| minio | 500m-1 | 1-2Gi | 100Gi | Included |
| mongodb-atlas | Managed | Managed | 50Gi | $57-100/mo |
| **Total** | **4-6 cores** | **7-13Gi** | **165Gi** | **~$100/mo** |

---

## 5. Service Communication Matrix

### 5.1 Inter-Service Communication

| From Service | To Service | Protocol | Port | Purpose | Auth Required |
|--------------|------------|----------|------|---------|---------------|
| **Client** | api-service | HTTP | 8080 | All API operations | JWT |
| **Client** | auth-service | HTTP | 8080 | Login, token ops | No (for login) |
| api-service | auth-service | HTTP | 8080 | Token validation | Service call |
| api-service | postgres-api | PostgreSQL | 5432 | Data persistence | Credentials |
| api-service | redis | Redis | 6379 | Caching | No |
| api-service | rabbitmq | AMQP | 5672 | Task publishing | Credentials |
| api-service | minio | S3 API | 9000 | Document storage | Access keys |
| auth-service | postgres-auth | PostgreSQL | 5432 | User data | Credentials |
| graphrag-service | rabbitmq | AMQP | 5672 | Message consuming | Credentials |
| graphrag-service | mongodb-atlas | MongoDB | 27017 | Graph storage | Connection URI |
| graphrag-service | minio | S3 API | 9000 | Document download | Access keys |
| email-worker | rabbitmq | AMQP | 5672 | Message consuming | Credentials |
| image-worker | rabbitmq | AMQP | 5672 | Message consuming | Credentials |
| profile-worker | rabbitmq | AMQP | 5672 | Message consuming | Credentials |

### 5.2 Network Policies

```yaml
# api-service can access:
- auth-service:8080 (HTTP)
- postgres-api:5432 (PostgreSQL)
- redis:6379 (Redis)
- rabbitmq:5672 (AMQP)
- minio:9000 (S3)

# auth-service can access:
- postgres-auth:5432 (PostgreSQL)

# graphrag-service can access:
- rabbitmq:5672 (AMQP)
- mongodb-atlas:27017 (MongoDB - external)
- minio:9000 (S3)

# operational-workers can access:
- rabbitmq:5672 (AMQP)

# Ingress can access:
- api-service:8080 (HTTP)
- auth-service:8080 (HTTP) - optional
```

### 5.3 Message Flows

#### Flow 1: Profile Creation with Email Task
```
Client → api-service: POST /api/v1/profiles
api-service → auth-service: POST /v1/auth/token/validate
api-service → postgres-api: INSERT INTO profiles
api-service → redis: SET profile:{id}
api-service → rabbitmq: PUBLISH email.send
email-worker ← rabbitmq: CONSUME email.send
email-worker → SMTP: Send email
```

#### Flow 2: Document Upload and Processing
```
Client → api-service: POST /api/v1/documents/upload (multipart)
api-service → auth-service: Validate token
api-service → minio: PutObject(document.pdf)
api-service → postgres-api: INSERT INTO documents
api-service → rabbitmq: PUBLISH document.process
graphrag-service ← rabbitmq: CONSUME document.process
graphrag-service → minio: GetObject(document.pdf)
graphrag-service: Process pipelines (async)
graphrag-service → mongodb-atlas: Store entities/relationships
graphrag-service → rabbitmq: PUBLISH document.completed (optional)
api-service ← rabbitmq: CONSUME document.completed (optional)
api-service → postgres-api: UPDATE documents SET status='completed'
```

### 5.4 Service Dependencies Graph

```
┌───────────┐
│  Client   │
└─────┬─────┘
      │
      ▼
┌───────────┐     ┌──────────────┐
│   API     │────▶│    Auth      │
│  Service  │     │   Service    │
└─┬───┬─┬─┬─┘     └──────────────┘
  │   │ │ │
  │   │ │ └────────┐
  │   │ │          │
  │   │ │          ▼
  │   │ │    ┌──────────┐
  │   │ │    │ RabbitMQ │
  │   │ │    └────┬─────┘
  │   │ │         │
  │   │ │    ┌────┼─────┬──────┐
  │   │ │    │    │     │      │
  │   │ │    ▼    ▼     ▼      ▼
  │   │ │  ┌────┐┌───┐┌────┐┌────┐
  │   │ │  │GR  ││Em ││Img ││Pro │
  │   │ │  │AG  ││ail││age ││file│
  │   │ │  └┬───┘└───┘└────┘└────┘
  │   │ │   │
  │   │ │   ▼
  │   │ │  ┌─────────┐
  │   │ │  │MongoDB  │
  │   │ │  │ Atlas   │
  │   │ │  └─────────┘
  │   │ │
  │   │ └───────────┐
  │   │             │
  │   ▼             ▼
  │ ┌─────┐    ┌────────┐
  │ │Redis│    │ MinIO  │
  │ └─────┘    └────────┘
  │
  ▼
┌──────────┐
│Postgres  │
│  (api)   │
└──────────┘

Legend:
GR = GraphRAG Service
AG = (continuation)
```

---

## 6. Missing Pieces & Gaps

### 6.1 Critical Missing Components

#### 6.1.1 Document Upload Endpoint in api-service
**Status:** ❌ Not implemented  
**Priority:** P0 - Critical  
**Effort:** 2 days

**What's Needed:**
- MinIO client integration
- Document handler with multipart upload
- Database migration for documents table
- RabbitMQ message publishing

**Implementation Location:**
- `api-service/internal/infrastructure/minio/client.go`
- `api-service/internal/domain/document/model.go`
- `api-service/internal/api/handlers/document.go`
- `api-service/migrations/000002_create_documents.up.sql`

---

#### 6.1.2 MongoDB Atlas Setup
**Status:** ❌ Not configured  
**Priority:** P0 - Critical  
**Effort:** 1 day

**What's Needed:**
- MongoDB Atlas account creation
- Cluster provisioning (M10 tier recommended)
- Database and collection setup
- Connection string in Kubernetes secret
- Network access configuration (whitelist K8s cluster)

**Configuration Location:**
- `deployment/external/mongodb-atlas/README.md`
- `deployment/external/mongodb-atlas/connection-secret.yaml`

---

#### 6.1.3 MinIO Deployment
**Status:** ❌ Not created  
**Priority:** P0 - Critical  
**Effort:** 1 day

**What's Needed:**
- StatefulSet manifest
- PersistentVolumeClaim (100Gi)
- Service definition
- Secret for credentials
- Bucket initialization job

**Implementation Location:**
- `deployment/services/01-infrastructure/minio/statefulset.yaml`
- `deployment/services/01-infrastructure/minio/service.yaml`
- `deployment/services/01-infrastructure/minio/secret.yaml`
- `deployment/services/01-infrastructure/minio/init-job.yaml`

---

#### 6.1.4 Shared Contracts Documentation
**Status:** ❌ Not created  
**Priority:** P1 - High  
**Effort:** 1 day

**What's Needed:**
- Message format specification
- Routing keys catalog
- Error codes documentation
- Data ownership matrix

**Implementation Location:**
- `graph-worker/shared/contracts/MESSAGE_FORMAT.md`
- `graph-worker/shared/contracts/ROUTING_KEYS.md`
- `graph-worker/shared/contracts/ERROR_CODES.md`
- `graph-worker/shared/contracts/DATA_OWNERSHIP.md`

---

### 6.2 Enhancement Opportunities

#### 6.2.1 Observability Enhancement
**Priority:** P1 - High  
**Effort:** 3 days

**What's Missing:**
- Distributed tracing (Jaeger/Tempo)
- Centralized logging (Loki/ELK)
- Grafana dashboards
- Custom alerting rules

**Benefits:**
- End-to-end request tracing
- Faster debugging
- Better visibility into system behavior

---

#### 6.2.2 Rate Limiting & Quotas
**Priority:** P2 - Medium  
**Effort:** 2 days

**What's Missing:**
- User-level rate limiting
- Document processing quotas
- LLM API cost tracking per user
- Fair usage enforcement

**Benefits:**
- Prevent abuse
- Cost control
- Fair resource allocation

---

#### 6.2.3 Webhook Notifications
**Priority:** P2 - Medium  
**Effort:** 2 days

**What's Missing:**
- Webhook registration API
- Event publishing to webhooks
- Retry logic for webhook failures
- Webhook verification

**Benefits:**
- Real-time notifications for clients
- Better integration with external systems
- Event-driven architecture

---

#### 6.2.4 Multi-Document Batch Processing
**Priority:** P3 - Low  
**Effort:** 3 days

**What's Missing:**
- Batch upload endpoint
- Batch processing optimization
- Progress tracking for batches

**Benefits:**
- Bulk document processing
- Better efficiency for large batches

---

### 6.3 Technical Debt & Improvements

#### 6.3.1 Code Quality
- [ ] Add comprehensive unit tests (target: 80% coverage)
- [ ] Add integration tests for all flows
- [ ] Add end-to-end tests
- [ ] Set up CI/CD pipeline (GitHub Actions)
- [ ] Add linting and formatting (golangci-lint, Black)

#### 6.3.2 Performance
- [ ] Implement connection pooling tuning
- [ ] Add request coalescing for duplicate queries
- [ ] Optimize database queries
- [ ] Add query result caching
- [ ] Implement streaming for large responses

#### 6.3.3 Developer Experience
- [ ] Add development docker-compose.yaml
- [ ] Create local testing scripts
- [ ] Add debugging guides
- [ ] Create contribution guidelines
- [ ] Add code examples and tutorials

---

## 7. Open Questions & Studies

### 7.1 Critical Questions (Must Answer Before Phase 3)

#### Q1: MongoDB Atlas Configuration
**Question:** What Atlas tier should we use?

| Tier | Cost/Month | CPU | RAM | Storage | Recommendation |
|------|------------|-----|-----|---------|----------------|
| M0 (Free) | $0 | Shared | 512MB | 512MB | ❌ Not for production |
| M10 | $57 | 2GB RAM | 10GB | Low volume | ✅ Start here |
| M20 | $118 | 4GB RAM | 20GB | Medium volume | ⚙️ If M10 insufficient |
| M30 | $311 | 8GB RAM | 40GB | High volume | ⚙️ For scaling |

**Decision Needed:** Start with M10 or M20?  
**Recommendation:** M10 for Phase 1, upgrade to M20 if needed

---

#### Q2: GraphRAG Service Role
**Question:** Should graphrag-service expose query APIs or be worker-only?

| Option | Pros | Cons |
|--------|------|------|
| **A: Worker Only** | Simple, focused, no port conflicts | Can't query graph directly |
| **B: Worker + API** | Convenient, uses GraphRAG's APIs | Dual responsibility, complex |
| **C: Separate Query Service** | Clean separation | Another deployment |

**Status:** ⏳ Deferred until Phase 3 complete  
**Recommendation:** Start with Option A (worker-only), add API in Phase 7.2 if needed

---

#### Q3: LLM Provider Choice
**Question:** OpenAI cloud vs local LLM?

| Approach | Cost | Quality | Privacy | Speed |
|----------|------|---------|---------|-------|
| **OpenAI GPT-4** | $0.03/1K tokens | Excellent | Concerns | Fast |
| **OpenAI GPT-3.5** | $0.002/1K tokens | Good | Concerns | Very fast |
| **Local Llama 3** | GPU costs | Good | Excellent | Medium |
| **Local Mistral** | GPU costs | Medium | Excellent | Medium |

**Decision Needed:** Which model for entity/relationship extraction?  
**Recommendation:** GPT-3.5-turbo for cost/performance balance  
**Cost Estimate:** ~$0.01-0.05 per document (assuming 500-2500 tokens per doc)

---

#### Q4: pgvector Extension
**Question:** Should we use pgvector in PostgreSQL for embeddings?

| Option | Pros | Cons |
|--------|------|------|
| **pgvector in PostgreSQL** | Simple, single database | Limited to ~1M vectors, slower search |
| **Dedicated vector DB (Qdrant)** | Optimized, scales well | Another infrastructure component |
| **MongoDB Atlas Vector Search** | Already using Atlas | GraphRAG data split across DBs |

**Decision Needed:** Where to store embeddings?  
**Recommendation:** Start with PostgreSQL + pgvector, migrate to Qdrant if >100K documents

---

### 7.2 Research Studies Required

#### Study 1: GraphRAG Pipeline Performance Baseline
**Priority:** P0 - Before Phase 3  
**Duration:** 2 days  
**Owner:** TBD

**Objectives:**
- Measure actual GraphRAG processing time for various document sizes
- Identify bottlenecks in ingestion and GraphRAG pipelines
- Determine optimal chunk size and overlap
- Test LLM rate limits and API costs
- Validate memory requirements

**Method:**
```bash
# Run GraphRAG CLI on test documents
cd GraphRAG
python -m src.app.cli.run_pipeline --input test_docs/ --output results/

# Measure:
# - Time per document (by size: 5 pages, 50 pages, 500 pages)
# - Memory usage peaks
# - LLM API calls made
# - Costs incurred
# - Entity/relationship counts
```

**Deliverable:** `GRAPHRAG_PERFORMANCE_BASELINE.md`

---

#### Study 2: Worker Concurrency Optimization
**Priority:** P1 - During Phase 7  
**Duration:** 3 days  
**Owner:** TBD

**Objectives:**
- Test worker throughput with varying concurrency levels
- Measure resource usage at different concurrency
- Identify optimal concurrency per worker type
- Test RabbitMQ prefetch count impact
- Validate error handling under load

**Method:**
```bash
# Test email worker with different concurrency
for CONCURRENT in 1 5 10 20; do
  # Configure worker concurrency
  kubectl set env deployment/email-worker WORKER_CONCURRENCY=$CONCURRENT
  
  # Send 1000 test messages
  ./scripts/test/send-test-messages.sh email 1000
  
  # Measure throughput and resource usage
  # Record: messages/sec, CPU usage, memory usage, error rate
done
```

**Deliverable:** `WORKER_CONCURRENCY_OPTIMIZATION.md`

---

#### Study 3: Cost Analysis for GraphRAG
**Priority:** P1 - Before Phase 3  
**Duration:** 1 day  
**Owner:** TBD

**Objectives:**
- Calculate LLM API costs per document
- Compare GPT-4 vs GPT-3.5-turbo vs local LLM
- Estimate monthly costs for different usage levels
- Identify cost optimization strategies

**Method:**
```python
# Process 10 test documents with each model
models = ['gpt-4-turbo', 'gpt-3.5-turbo']

for model in models:
    cost_per_doc = []
    for doc in test_documents:
        tokens_used = process_with_model(doc, model)
        cost = calculate_cost(tokens_used, model)
        cost_per_doc.append(cost)
    
    print(f"{model}: avg ${mean(cost_per_doc)}/doc")
```

**Usage Scenarios:**
- Low: 100 docs/month → Est. $1-5/month
- Medium: 1000 docs/month → Est. $10-50/month
- High: 10,000 docs/month → Est. $100-500/month

**Deliverable:** `GRAPHRAG_COST_ANALYSIS.md`

---

#### Study 4: Database Scaling Strategy
**Priority:** P2 - Before production  
**Duration:** 2 days  
**Owner:** TBD

**Objectives:**
- Estimate data growth (profiles, documents, entities)
- Plan database scaling strategy (vertical vs horizontal)
- Test backup and restore procedures
- Evaluate read replica needs
- Plan for multi-tenancy (if applicable)

**Data Growth Estimates:**

| Data Type | Size per Record | Growth Rate | 1 Year Projection |
|-----------|-----------------|-------------|-------------------|
| Profiles | 2KB | 1000/month | 12K profiles, 24MB |
| Documents | 5MB (avg) | 500/month | 6K docs, 30GB |
| Entities | 500B | 5000/doc | 30M entities, 15GB |
| Relationships | 300B | 15000/doc | 90M rels, 27GB |
| Embeddings | 6KB (1536 dims) | 100/doc | 600K embeddings, 3.6GB |
| **Total** | | | **~76GB** |

**Scaling Recommendations:**
- PostgreSQL: Start with 50GB, plan for 100-200GB in year 1
- MongoDB Atlas: Start with 50GB, auto-scaling enabled
- MinIO: Start with 100GB, expand as needed

**Deliverable:** `DATABASE_SCALING_STRATEGY.md`

---

#### Study 5: Security Threat Model
**Priority:** P1 - Before production  
**Duration:** 2 days  
**Owner:** TBD

**Objectives:**
- Identify security threats and attack vectors
- Document security controls
- Plan security testing (penetration testing)
- Create incident response plan

**Threat Vectors:**
- Unauthorized access to documents
- JWT token theft/replay
- SQL injection in queries
- Malicious file uploads
- API abuse and DDoS
- Secrets exposure in Kubernetes

**Deliverable:** `SECURITY_THREAT_MODEL.md`

---

### 7.3 Optional Studies

#### Study 6: Alternative Architecture for GraphRAG
**Priority:** P3 - Future consideration  
**Duration:** 3 days

**Question:** Should we use existing GraphRAG Python code or rewrite in Go?

**Pros of Python (Current Plan):**
- ✅ Existing implementation (576 files)
- ✅ Rich ML/AI ecosystem
- ✅ Faster to implement (just wrap with consumer)

**Cons:**
- ❌ Larger Docker images (~2GB)
- ❌ Slower startup (~60s)
- ❌ Higher memory usage

**Pros of Go Rewrite:**
- ✅ Consistent with other services
- ✅ Smaller images (~50MB)
- ✅ Fast startup (~2s)
- ✅ Better resource efficiency

**Cons:**
- ❌ Significant development effort (4-6 weeks)
- ❌ Need to port complex ML logic
- ❌ Less mature ML ecosystem

**Recommendation:** Stick with Python (current plan) unless performance becomes critical

---

#### Study 7: Event Sourcing for Audit Trail
**Priority:** P3 - Future enhancement  
**Duration:** 3 days

**Question:** Should we implement event sourcing for complete audit trails?

**Current:** Direct database mutations, audit logs in separate table

**Event Sourcing:** All changes stored as events, state derived from event log

**Benefits:**
- Complete audit trail
- Time travel (view past states)
- Event replay for debugging
- Better analytics

**Trade-offs:**
- More complex
- Higher storage requirements
- Need event store (EventStoreDB, Kafka)

**Recommendation:** Not for Phase 1-2, consider for Phase 3 if audit requirements increase

---

## 8. Risk Assessment

### 8.1 Technical Risks

| Risk | Probability | Impact | Mitigation | Owner |
|------|-------------|--------|------------|-------|
| **GraphRAG processing exceeds memory limits** | Medium | High | Set resource limits, implement streaming, monitor memory | DevOps |
| **LLM API rate limits hit** | High | Medium | Implement exponential backoff, queue depth alerts, consider local LLM | Backend |
| **MongoDB Atlas costs exceed budget** | Medium | Medium | Monitor usage, set budget alerts, optimize queries | Backend |
| **MinIO storage fills up** | Low | High | Implement data retention policies, monitor storage | DevOps |
| **RabbitMQ queue buildup** | Medium | High | Implement HPA on queue depth, increase workers, alerts | DevOps |
| **Database connection pool exhaustion** | Low | High | Tune pool sizes, implement connection limits, monitor | Backend |
| **Network policy misconfiguration** | Medium | Critical | Test all service communication, staged rollout | DevOps |
| **JWT token leakage** | Low | Critical | Short token expiry, token rotation, secure storage guidelines | Security |
| **Malicious file uploads** | Medium | High | File validation, virus scanning, size limits, sandboxing | Security |
| **Service cascade failure** | Low | Critical | Circuit breakers, graceful degradation, health checks | Backend |

### 8.2 Operational Risks

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| **Insufficient documentation** | Medium | Medium | Dedicated documentation phase, runbooks |
| **Complex deployment** | High | Medium | Automation scripts, clear deployment guide |
| **Difficult debugging** | Medium | High | Distributed tracing, centralized logging |
| **Backup failure** | Low | Critical | Test restore procedures, automated backups |
| **Kubernetes cluster issues** | Medium | High | Multi-node cluster, resource monitoring |
| **Secret management complexity** | Medium | Medium | Use external secrets manager (Vault) in future |

### 8.3 Business Risks

| Risk | Probability | Impact | Mitigation |
|------|-------------|--------|------------|
| **GraphRAG feature not used** | Low | High | User research, pilot program, feedback loop |
| **High operational costs** | Medium | High | Cost monitoring, optimization, usage quotas |
| **Slow adoption** | Medium | Medium | Good documentation, examples, support |
| **Competitor moves faster** | Medium | High | Agile iterations, MVP approach |

---

## 9. Success Criteria

### 9.1 Phase 1 Success (Auth & Deployment)

**Completion Criteria:**
- [ ] auth-service deployed and accessible
- [ ] api-service deployed and accessible
- [ ] All infrastructure components running (PostgreSQL, Redis, RabbitMQ)
- [ ] Services can communicate (network policies working)
- [ ] Health checks passing
- [ ] Can create profile and submit task
- [ ] Load test: 100 RPS on profile API

**Performance Targets:**
| Metric | Target |
|--------|--------|
| API Service Availability | >99.5% |
| Auth Service Availability | >99.9% |
| P95 Profile API Latency | <100ms |
| P95 Auth API Latency | <50ms |

---

### 9.2 Phase 2 Success (Document Upload)

**Completion Criteria:**
- [ ] Document upload endpoint working
- [ ] MinIO storing documents
- [ ] PostgreSQL storing document metadata
- [ ] RabbitMQ receiving document.process messages
- [ ] Load test: 50 concurrent uploads

**Performance Targets:**
| Metric | Target |
|--------|--------|
| Document Upload Success Rate | >95% |
| P95 Upload Latency (10MB) | <2s |
| P95 Upload Latency (100MB) | <15s |

---

### 9.3 Phase 3 Success (GraphRAG Worker)

**Completion Criteria:**
- [ ] graphrag-service consuming from RabbitMQ
- [ ] Processing completes successfully
- [ ] Entities stored in MongoDB Atlas
- [ ] Relationships stored in MongoDB Atlas
- [ ] Processing time <45 min for typical document
- [ ] No memory leaks after 24h operation

**Performance Targets:**
| Metric | Target |
|--------|--------|
| Processing Success Rate | >90% |
| Avg Processing Time | <30 min |
| P95 Processing Time | <45 min |
| Memory Usage | <8Gi per worker |
| Entity Extraction Accuracy | >70% (manual review) |

---

### 9.4 Phase 4 Success (Operational Workers)

**Completion Criteria:**
- [ ] All 3 workers (email, image, profile) deployed
- [ ] Workers consuming from respective queues
- [ ] Health checks passing
- [ ] Metrics exposed
- [ ] Load test: 1000 messages processed successfully

**Performance Targets:**
| Metric | Email Worker | Image Worker | Profile Worker |
|--------|--------------|--------------|----------------|
| Throughput | >100 msg/sec | >10 msg/sec | >50 msg/sec |
| P95 Latency | <5s | <20s | <10s |
| Error Rate | <1% | <2% | <1% |

---

### 9.5 Phase 5 Success (Integration)

**Completion Criteria:**
- [ ] End-to-end flows tested and working
- [ ] All services communicating correctly
- [ ] No message loss in queues
- [ ] Monitoring showing all services healthy
- [ ] Load test: Full system under realistic load

**System-Level Targets:**
| Metric | Target |
|--------|--------|
| System Availability | >99.5% |
| End-to-End Success Rate | >95% |
| Mean Time to Recovery | <5 min |
| Incident Response Time | <15 min |

---

## 10. Timeline & Resource Allocation

### 10.1 Overall Timeline

```
┌─────────────────────────────────────────────────────────────────┐
│                     10-Week Implementation                      │
├─────────────────────────────────────────────────────────────────┤
│ Week 1  │ Phase 1: Auth Service & Deployment Structure         │
│ Week 2  │ Phase 2: API Service Document Upload                 │
│ Week 3-4│ Phase 3: GraphRAG Service (Python Worker)            │
│ Week 5  │ Phase 4: Operational Workers (Go)                    │
│ Week 6  │ Phase 5: Integration & Testing                       │
│ Week 7  │ Phase 6: Shared Contracts & Documentation            │
│ Week 8-9│ Phase 7: Advanced Features (Results, Queries, Perf)  │
│ Week 10 │ Phase 8: Production Readiness (Security, Monitoring) │
└─────────────────────────────────────────────────────────────────┘
```

### 10.2 Detailed Phase Breakdown

#### Week 1: Foundation (Phase 1)
| Day | Tasks | Deliverables |
|-----|-------|--------------|
| 1 | Create auth-service folder, copy code | auth-service/ created |
| 2 | Test auth-service locally, build image | Docker image ready |
| 3 | Create deployment/ structure | Folder structure |
| 4 | Create infrastructure manifests | StatefulSets created |
| 5 | Create service manifests, scripts | Complete deployment/ |

#### Week 2: Document Upload (Phase 2)
| Day | Tasks | Deliverables |
|-----|-------|--------------|
| 6 | MinIO client integration | MinIO client working |
| 7 | Document domain layer | Models, service, repo |
| 8 | Document upload API, routing config | Upload endpoint |
| 9 | Testing and bug fixes | Tests passing |
| 10 | Deploy to Kind cluster | E2E upload working |

#### Weeks 3-4: GraphRAG (Phase 3)
| Day | Tasks | Deliverables |
|-----|-------|--------------|
| 11-12 | Copy GraphRAG, setup project | GraphRAG core copied |
| 13-14 | RabbitMQ consumer (async) | Consumer working |
| 15-16 | Document processor wrapper | Processor working |
| 17 | Monitoring and health checks | Health endpoints |
| 18 | Dockerfile and build | Docker image |
| 19-20 | Kubernetes deployment, testing | Deployed, E2E tested |

#### Week 5: Workers (Phase 4)
| Day | Tasks | Deliverables |
|-----|-------|--------------|
| 21 | Copy common foundation, queue package | Foundation ready |
| 22 | Email worker | Email worker deployed |
| 23 | Image worker | Image worker deployed |
| 24 | Profile worker (new) | Profile worker deployed |
| 25 | Integration testing | All workers tested |

#### Week 6: Integration (Phase 5)
| Day | Tasks | Deliverables |
|-----|-------|--------------|
| 26 | Infrastructure validation | All infra healthy |
| 27 | Service integration tests | Integration passing |
| 28-29 | Load testing | Benchmarks captured |
| 30 | Bug fixes and optimizations | Issues resolved |

#### Week 7: Contracts & Docs (Phase 6)
| Day | Tasks | Deliverables |
|-----|-------|--------------|
| 31-32 | Shared contracts documentation | Contracts complete |
| 33-35 | System documentation | Docs complete |

#### Weeks 8-9: Advanced Features (Phase 7)
| Day | Tasks | Deliverables |
|-----|-------|--------------|
| 36-37 | Results feedback loop | Results working |
| 38-39 | Query APIs (decision-based) | Query capability |
| 40-42 | Concurrency optimizations | 5-25x improvements |
| 43-44 | Testing and validation | Optimizations verified |

#### Week 10: Production Ready (Phase 8)
| Day | Tasks | Deliverables |
|-----|-------|--------------|
| 45-46 | Security hardening | Security scan passed |
| 47-48 | Monitoring and alerting | Monitoring deployed |
| 49 | DR planning and testing | DR procedures |
| 50 | Final validation, sign-off | Production ready |

### 10.3 Resource Requirements

**Team Composition:**

| Role | Allocation | Focus |
|------|------------|-------|
| **Backend Engineer (Go)** | 100% | api-service enhancements, operational-workers |
| **Backend Engineer (Python)** | 100% | graphrag-service implementation |
| **Backend Engineer (Node.js)** | 20% | auth-service adaptation |
| **DevOps Engineer** | 80% | Deployment structure, Kubernetes, automation |
| **QA Engineer** | 50% | Testing, validation, load testing |
| **Tech Lead** | 30% | Architecture decisions, code review, guidance |

**Alternative for Smaller Team:**
- 1 Full-stack engineer (Go + Python) can handle Phases 1-4 (6-8 weeks)
- DevOps engineer (part-time or consultant) for deployment
- QA integrated into development (automated testing)

### 10.4 Infrastructure Costs

**Kubernetes Cluster:**
- Development: 3-node Kind cluster (local, $0)
- Staging: 3-node managed K8s (AWS EKS, GKE, AKS: ~$150-300/month)
- Production: 5-node managed K8s with autoscaling (~$500-1000/month)

**External Services:**
- MongoDB Atlas: M10-M20 tier ($57-118/month)
- LLM API (OpenAI): $10-500/month (usage-based)

**Total Estimated Cost:**
- Development: $0/month
- Staging: $200-400/month
- Production: $600-1600/month

---

## 11. Implementation Checklist

### 11.1 Pre-Implementation (Before Week 1)

**Research & Studies:**
- [ ] Complete Study 1: GraphRAG Performance Baseline
- [ ] Complete Study 3: Cost Analysis for GraphRAG
- [ ] Answer Q1: MongoDB Atlas tier decision
- [ ] Answer Q3: LLM provider decision

**Preparation:**
- [ ] Set up MongoDB Atlas account
- [ ] Obtain OpenAI API key
- [ ] Set up container registry
- [ ] Provision Kubernetes cluster (if cloud)
- [ ] Set up monitoring infrastructure

---

### 11.2 Phase Completion Checklists

#### Phase 1 Checklist (Week 1)
- [ ] auth-service code copied and adapted
- [ ] auth-service builds successfully
- [ ] auth-service tested locally
- [ ] deployment/ structure created
- [ ] All infrastructure manifests created
- [ ] All service manifests created
- [ ] Scripts created and tested
- [ ] Deployed to Kind cluster
- [ ] All services reachable
- [ ] Health checks passing
- [ ] Integration test: Create profile via API

#### Phase 2 Checklist (Week 2)
- [ ] MinIO client added to api-service
- [ ] Document domain implemented
- [ ] Database migration created and tested
- [ ] Document upload endpoint working
- [ ] Files stored in MinIO
- [ ] Metadata stored in PostgreSQL
- [ ] Messages published to RabbitMQ
- [ ] Integration test: Upload document
- [ ] Load test: 50 concurrent uploads

#### Phase 3 Checklist (Weeks 3-4)
- [ ] GraphRAG core copied successfully
- [ ] Async RabbitMQ consumer working
- [ ] Document processor wraps GraphRAG pipelines
- [ ] Health and metrics endpoints working
- [ ] Docker image builds
- [ ] Deployed to Kubernetes
- [ ] Consumes messages from queue
- [ ] Processes test document successfully
- [ ] Stores results in MongoDB Atlas
- [ ] Integration test: E2E document processing

#### Phase 4 Checklist (Week 5)
- [ ] Common foundation copied
- [ ] Email worker deployed and working
- [ ] Image worker deployed and working
- [ ] Profile worker created and working
- [ ] All workers consuming from queues
- [ ] Health checks passing
- [ ] Integration test: All worker types

#### Phase 5 Checklist (Week 6)
- [ ] All infrastructure validated
- [ ] All services integrated
- [ ] Load tests completed
- [ ] Performance benchmarks captured
- [ ] No critical bugs
- [ ] Monitoring showing healthy state

#### Phase 6 Checklist (Week 7)
- [ ] Message contracts documented
- [ ] Routing keys catalog complete
- [ ] Data ownership clarified
- [ ] System documentation complete
- [ ] API documentation complete
- [ ] Deployment guide complete
- [ ] Troubleshooting guide created

#### Phase 7 Checklist (Weeks 8-9)
- [ ] Results feedback loop working
- [ ] Query APIs implemented (if decided)
- [ ] Concurrency optimizations applied
- [ ] Performance improvements measured
- [ ] Load tests show improvements

#### Phase 8 Checklist (Week 10)
- [ ] Security scan passed
- [ ] Network policies tested
- [ ] TLS enabled
- [ ] Monitoring dashboards deployed
- [ ] Alerting rules configured
- [ ] Backup and restore tested
- [ ] DR plan documented
- [ ] Production readiness sign-off

---

## 12. Consolidated Reference

### 12.1 Source Documents

This master plan consolidates information from:

| Document | Focus | Status |
|----------|-------|--------|
| `CONSOLIDATED_SERVICE_PLAN.md` | api-service design | ✅ Implemented |
| `BRAINSTORM_GRAPH_WORKER_ARCHITECTURE.md` | Worker architecture | ✅ Updated |
| `GRAPHRAG_AND_CONCURRENCY_PLAN.md` | GraphRAG features, concurrency | 📋 Referenced |
| `PLAN_AUTH_SERVICE_AND_DEPLOYMENT.md` | Auth & deployment | ✅ Incorporated |
| `graph-worker/IMPLEMENTATION_ROADMAP.md` | Worker implementation | ✅ Referenced |
| `graph-worker/PROJECT_OVERVIEW.md` | Quick reference | ✅ Referenced |
| `graph-worker/ARCHITECTURE_OPTIONS.md` | Architecture decisions | ✅ Referenced |

### 12.2 Individual Implementation Plans

Each project now has its own detailed implementation plan:

| Project | Plan | Session Scope |
|---------|------|---------------|
| api-service | `api-service/IMPLEMENTATION_PLAN.md` | Document upload, MinIO integration |
| auth-service | `auth-service/IMPLEMENTATION_PLAN.md` | Copy & adapt from legacy |
| graphrag-service | `graph-worker/graphrag-service/IMPLEMENTATION_PLAN.md` | Python worker implementation |
| operational-workers | `graph-worker/operational-workers/IMPLEMENTATION_PLAN.md` | Go workers implementation |
| deployment | `deployment/IMPLEMENTATION_PLAN.md` | K8s manifests, automation |

### 12.3 Key Architectural Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| **API Gateway** | Consolidated Go service | 10x faster than HTTP microservices |
| **Auth Service** | Keep Node.js/TypeScript | Well-tested, production-ready |
| **GraphRAG Worker** | Python (wrap existing) | Rich ML ecosystem, existing code |
| **Operational Workers** | Go (from legacy) | Lightweight, efficient |
| **Document Storage** | MinIO in Kubernetes | Full control, S3-compatible |
| **Graph Storage** | MongoDB Atlas | Managed, vector search built-in |
| **Message Broker** | RabbitMQ | Reliable, feature-rich |
| **Caching** | Redis | Fast, simple |
| **Python RabbitMQ** | aio-pika (async) | Matches GraphRAG async pipelines |
| **Common Package** | Copy into projects | Self-contained |
| **GraphRAG Role** | Worker-only (Phase 1) | Simple, defer API decision |

### 12.4 Port Assignments

| Service | Container Port | NodePort | Ingress Path |
|---------|---------------|----------|--------------|
| api-service | 8080 | 30080 | /api/v1/* |
| auth-service | 8080 | 30081 | /v1/auth/*, /v1/users/* |
| graphrag-service (health) | 8080 | 30082 | - |
| graphrag-service (metrics) | 8081 | 30083 | - |
| email-worker | 8080 | 30084 | - |
| image-worker | 8080 | 30085 | - |
| profile-worker | 8080 | 30086 | - |
| RabbitMQ (AMQP) | 5672 | - | - |
| RabbitMQ (Management) | 15672 | 30091 | - |
| MinIO (API) | 9000 | 30092 | - |
| MinIO (Console) | 9001 | 30093 | - |

### 12.5 Environment Variables Summary

**Common Variables (All Services):**
```bash
LOG_LEVEL=info
METRICS_ENABLED=true
```

**api-service:**
```bash
SERVER_PORT=8080
POSTGRES_HOST=postgres-api
POSTGRES_PORT=5432
POSTGRES_DATABASE=api_db
POSTGRES_USER=api_user
POSTGRES_PASSWORD=<secret>
REDIS_HOST=redis
REDIS_PORT=6379
RABBITMQ_HOST=rabbitmq
RABBITMQ_PORT=5672
RABBITMQ_USER=guest
RABBITMQ_PASSWORD=<secret>
AUTH_SERVICE_URL=http://auth-service:8080
MINIO_ENDPOINT=minio:9000
MINIO_ACCESS_KEY=<secret>
MINIO_SECRET_KEY=<secret>
```

**auth-service:**
```bash
NODE_ENV=production
PORT=8080
DATABASE_HOST=postgres-auth
DATABASE_PORT=5432
DATABASE_NAME=auth_db
DATABASE_USER=auth_user
DATABASE_PASSWORD=<secret>
JWT_SECRET=<secret>
JWT_ACCESS_TOKEN_EXPIRY=15m
JWT_REFRESH_TOKEN_EXPIRY=7d
```

**graphrag-service:**
```bash
RABBITMQ_HOST=rabbitmq
RABBITMQ_PORT=5672
RABBITMQ_USER=guest
RABBITMQ_PASSWORD=<secret>
MONGODB_URI=<atlas-connection-string>
MONGODB_DATABASE=graphrag
OPENAI_API_KEY=<secret>
MINIO_ENDPOINT=minio:9000
MINIO_ACCESS_KEY=<secret>
MINIO_SECRET_KEY=<secret>
```

**operational-workers:**
```bash
WORKER_TYPE=email|image|profile
RABBITMQ_HOST=rabbitmq
RABBITMQ_PORT=5672
RABBITMQ_USER=guest
RABBITMQ_PASSWORD=<secret>
QUEUE_NAME=email-processing|image-processing|profile-processing
```

---

## 13. Next Actions

### 13.1 Immediate Actions (This Week)

**Before Starting Implementation:**
1. [ ] **Review and approve this master plan** (1 hour)
2. [ ] **Answer critical questions** (Q1-Q4 above) (1 hour)
3. [ ] **Complete Study 1: GraphRAG Performance Baseline** (2 days)
4. [ ] **Complete Study 3: Cost Analysis** (1 day)
5. [ ] **Set up MongoDB Atlas account** (1 hour)
6. [ ] **Obtain OpenAI API key** (30 min)

**Start Implementation Sessions:**

Each project can be implemented in an independent session using its `IMPLEMENTATION_PLAN.md`:

1. [ ] **Session 1:** Execute `deployment/IMPLEMENTATION_PLAN.md` → Infrastructure first
2. [ ] **Session 2:** Execute `auth-service/IMPLEMENTATION_PLAN.md` → Auth service
3. [ ] **Session 3:** Execute `api-service/IMPLEMENTATION_PLAN.md` → Document upload
4. [ ] **Session 4:** Execute `graph-worker/graphrag-service/IMPLEMENTATION_PLAN.md` → Python worker
5. [ ] **Session 5:** Execute `graph-worker/operational-workers/IMPLEMENTATION_PLAN.md` → Go workers

**After All Sessions:** Return to this master plan for:
- Integration testing across all services
- Network policy verification
- End-to-end flow validation
- Performance benchmarking

### 13.2 Weekly Checkpoints

**Week 1 Checkpoint:**
- auth-service deployed in K8s
- api-service deployed in K8s
- Infrastructure components running
- Basic integration working

**Week 2 Checkpoint:**
- Document upload working
- MinIO storing files
- Messages queued for GraphRAG

**Week 4 Checkpoint:**
- graphrag-service processing documents
- MongoDB Atlas storing knowledge graphs
- E2E document flow working

**Week 5 Checkpoint:**
- All workers deployed
- All task types processing
- Full system operational

**Week 6 Checkpoint:**
- Integration tests passing
- Load tests completed
- Performance validated

**Week 7 Checkpoint:**
- Documentation complete
- Contracts defined
- Ready for advanced features

**Week 9 Checkpoint:**
- Advanced features implemented
- Performance optimized
- System enhanced

**Week 10 Checkpoint:**
- Security hardened
- Monitoring operational
- Production ready

---

## 14. Deferred Decisions & Future Considerations

### 14.1 Deferred Decisions

These decisions are intentionally deferred and should be revisited at specified milestones:

#### Decision 1: GraphRAG Service Role ⏳
**When to Decide:** After Phase 3 complete (Week 4)  
**Options:** Worker-only vs Worker+API vs Separate services  
**Current:** Start with worker-only  
**See:** Section 7.1, Q2

#### Decision 2: Vector Database Choice ⏳
**When to Decide:** After 10K documents processed or performance issues  
**Options:** pgvector vs Qdrant vs MongoDB Atlas Vector Search  
**Current:** pgvector in PostgreSQL  
**Trigger:** Search latency >500ms or >100K documents

#### Decision 3: Local LLM Migration ⏳
**When to Decide:** After 1000 documents processed  
**Options:** OpenAI vs Local Llama/Mistral  
**Current:** OpenAI GPT-3.5-turbo  
**Trigger:** Monthly LLM costs >$100 or privacy requirements

### 14.2 Future Enhancements (Post-Phase 8)

**Phase 9: Advanced Analytics**
- Knowledge graph analytics dashboard
- Document insights and trends
- Entity relationship visualization
- Community detection UI

**Phase 10: Multi-Tenancy**
- Tenant isolation
- Tenant-specific quotas
- Per-tenant billing
- Tenant admin UI

**Phase 11: API Versioning**
- API v2 with improved design
- Backward compatibility
- Deprecation strategy

**Phase 12: Global Distribution**
- Multi-region deployment
- Data replication
- Geo-routing
- CDN integration

---

## 15. Appendix

### 15.1 Related Documents

**Original Planning Documents:**
- `CONSOLIDATED_SERVICE_PLAN.md` - api-service architecture
- `BRAINSTORM_GRAPH_WORKER_ARCHITECTURE.md` - Worker architecture deep dive
- `GRAPHRAG_AND_CONCURRENCY_PLAN.md` - GraphRAG features and concurrency
- `PLAN_AUTH_SERVICE_AND_DEPLOYMENT.md` - Auth and deployment details
- `graph-worker/IMPLEMENTATION_ROADMAP.md` - Detailed worker implementation
- `graph-worker/ARCHITECTURE_OPTIONS.md` - Architecture tradeoffs

**Individual Implementation Plans (Use for Sessions):**
- `api-service/IMPLEMENTATION_PLAN.md` - Document upload & MinIO
- `auth-service/IMPLEMENTATION_PLAN.md` - Copy & adapt from legacy
- `graph-worker/graphrag-service/IMPLEMENTATION_PLAN.md` - Python worker
- `graph-worker/operational-workers/IMPLEMENTATION_PLAN.md` - Go workers
- `deployment/IMPLEMENTATION_PLAN.md` - Kubernetes & automation

### 15.2 Key Repositories

| Component | Repository Path | Implementation Plan |
|-----------|----------------|---------------------|
| API Service | `/microservices/api-service/` | [`IMPLEMENTATION_PLAN.md`](api-service/IMPLEMENTATION_PLAN.md) |
| Auth Service | `/microservices/auth-service/` | [`IMPLEMENTATION_PLAN.md`](auth-service/IMPLEMENTATION_PLAN.md) |
| GraphRAG Service | `/microservices/graph-worker/graphrag-service/` | [`IMPLEMENTATION_PLAN.md`](graph-worker/graphrag-service/IMPLEMENTATION_PLAN.md) |
| Operational Workers | `/microservices/graph-worker/operational-workers/` | [`IMPLEMENTATION_PLAN.md`](graph-worker/operational-workers/IMPLEMENTATION_PLAN.md) |
| Deployment | `/microservices/deployment/` | [`IMPLEMENTATION_PLAN.md`](deployment/IMPLEMENTATION_PLAN.md) |
| GraphRAG Library (Ref) | `/microservices/legacy_project/GraphRAG/` | External reference only |
| Legacy Reference | `/microservices/legacy_project/` | Reference only |

### 15.3 Glossary

| Term | Definition |
|------|------------|
| **GraphRAG** | Graph-based Retrieval Augmented Generation - knowledge graph construction from documents |
| **Entity** | Named entity extracted from document (person, organization, concept, etc.) |
| **Relationship** | Connection between two entities |
| **Community** | Cluster of related entities in knowledge graph |
| **Chunk** | Segment of document text (500-1000 tokens) |
| **Embedding** | Vector representation of text for semantic search |
| **Routing Key** | RabbitMQ message routing identifier |
| **Prefetch** | Number of messages worker receives before acknowledging |
| **DLQ** | Dead Letter Queue - for failed messages |
| **HPA** | Horizontal Pod Autoscaler - scales pods based on metrics |

---

## Conclusion

This master plan provides a comprehensive roadmap for implementing the complete microservices architecture V2. It consolidates all planning documents into a single coordination hub with clear phases, milestones, and success criteria.

**Key Takeaways:**

1. **10-week timeline** from start to production-ready
2. **5 individual implementation plans** for independent session work
3. **Clear phases** with distinct milestones
4. **Comprehensive coverage** of all components
5. **Cross-project concerns** documented in this master plan
6. **Success criteria** defined for validation
7. **Flexibility** for deferred decisions at appropriate times

**Implementation Approach:**

- Each project has its own `IMPLEMENTATION_PLAN.md` for focused session work
- This master plan coordinates across projects and tracks overall progress
- Review this master plan after completing each individual plan
- Use this plan for integration testing and validation

**Next Step:** Review this plan, answer open questions, and begin with `deployment/IMPLEMENTATION_PLAN.md`.

---

**Document Version:** 1.1  
**Last Updated:** January 29, 2026  
**Status:** Ready for Implementation  
**Individual Plans:** 5 created (api-service, auth-service, graphrag-service, operational-workers, deployment)  
**Approver:** [TBD]  
**Start Date:** [TBD]
