# Graph Worker System - Quick Reference

## 🏗️ Proposed Structure

```
graph-worker/
├── graphrag-service/         # Python - AI/Knowledge Graph
├── operational-workers/      # Go - Email/Image/Profile tasks  
└── shared/                   # Contracts & Config
```

## 📂 Source Locations

| Component | Location |
|-----------|----------|
| GraphRAG Source | `/microservices/legacy_project/GraphRAG/` (external reference) |
| Worker Service Source | `/microservices/legacy_project/services/worker-service/` |
| Common Queue Package | `/microservices/legacy_project/services/common/queue/` |

---

## 🐍 GraphRAG Service (Python)

**Purpose:** Process documents into knowledge graphs using LLMs

**Source:** Copy from `/microservices/legacy_project/GraphRAG/src/`

**Routing Key:** `document.process`  
**Queue:** `document-processing`  
**Processing Time:** 15-45 minutes per document (estimated)  
**Resources:** 2-4 CPU, 6-10GB RAM  
**Replicas:** 1-3 (HPA on queue depth)

**What it does:**
1. Consumes document URLs from RabbitMQ (async with aio-pika)
2. Downloads content from MinIO (PDF, etc.) or URLs (YouTube)
3. Runs LLM extraction (entities, relationships) via async pipelines
4. Builds knowledge graph
5. Stores in MongoDB Atlas
6. Publishes completion event (optional)

**New Components to Add:**
- RabbitMQ consumer (**aio-pika** - async, required for GraphRAG async pipelines)
- Worker entry point (`cmd/main.py`)
- Health check server (Flask or FastAPI)
- Prometheus metrics

---

## 🚀 Operational Workers (Go)

**Purpose:** Handle lightweight operational tasks

### Email Worker
- **Routing Key:** `email.send`
- **Processing:** 2-8s per message
- **Resources:** 50m CPU, 64-256Mi RAM
- **Replicas:** 2-15 (burst scaling)
- **Prefetch:** 5

### Image Worker
- **Routing Key:** `image.process`
- **Processing:** 10-25s per message
- **Resources:** 500m-1000m CPU, 512Mi-1Gi RAM
- **Replicas:** 1-8
- **Prefetch:** 1

### Profile Worker
- **Routing Key:** `profile.task`
- **Processing:** 5-15s per message
- **Resources:** 100-300m CPU, 128-512Mi RAM
- **Replicas:** 1-5
- **Prefetch:** 2

---

## 🔄 Message Flow

```
Client Request
    ↓
API Service (Go)
    ↓
RabbitMQ Publish
    ↓
    ├─→ document.process → GraphRAG Service (Python)
    ├─→ email.send → Email Worker (Go)
    ├─→ image.process → Image Worker (Go)
    └─→ profile.task → Profile Worker (Go)
```

---

## ✅ Key Decisions

| Decision | Choice | Status |
|----------|--------|--------|
| **Structure** | Monorepo with separate projects | ✅ Confirmed |
| **Python project name** | `graphrag-service` | ✅ Confirmed |
| **Go project name** | `operational-workers` | ✅ Confirmed |
| **Document storage** | MinIO deployed in Kubernetes | ✅ Decided |
| **MongoDB** | MongoDB Atlas (managed) | ✅ Decided |
| **RabbitMQ consumer (Python)** | aio-pika (async) | ✅ Decided |
| **Common package (Go)** | Copy into operational-workers | ✅ Decided |
| **GraphRAG role** | Worker-only vs Worker+API | ⏳ Decide later |
| **Results feedback** | Publish to results-exchange | Recommended |
| **Shared code** | Contracts/docs only, not code | ✅ Confirmed |

---

## 📊 Resource Summary

| Component | CPU | Memory | Storage | Notes |
|-----------|-----|--------|---------|-------|
| graphrag-service | 2-4 cores | 6-10Gi | N/A | Higher memory for LLM responses |
| operational-workers | 1-2 cores | 1-3Gi | N/A | |
| MongoDB Atlas | Managed | Managed | 50Gi+ | Managed service |
| MinIO | 500m-1 core | 1-2Gi | 100Gi | In-cluster storage |
| **Total Workers** | **3-6 cores** | **8-15Gi** | **100Gi** | |

---

## ⏳ Deferred Decision: GraphRAG Service Role

> **Decide after basic implementation is complete**

| Option | Description | Recommendation |
|--------|-------------|----------------|
| **A: Worker Only** | Just consumes from RabbitMQ | **Start here** (Phase 1) |
| **B: Worker + API** | Worker + Graph API on port 8081 | Consider for Phase 2 |
| **C: Separate Services** | Worker and Query as separate deployments | Phase 3 if needed |

See [BRAINSTORM_GRAPH_WORKER_ARCHITECTURE.md](../BRAINSTORM_GRAPH_WORKER_ARCHITECTURE.md) for detailed rationale.

---

See [BRAINSTORM_GRAPH_WORKER_ARCHITECTURE.md](../BRAINSTORM_GRAPH_WORKER_ARCHITECTURE.md) for full analysis.
