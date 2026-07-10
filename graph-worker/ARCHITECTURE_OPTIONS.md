# Architecture Options - Quick Comparison

## 🏗️ Three Structure Options

### Option 1: Parallel Projects with Shared Infrastructure

```
graph-worker/
├── graphrag-worker/          (Python)
├── task-worker/              (Go)
└── shared/                   (Both)
```

| Aspect | Rating | Notes |
|--------|--------|-------|
| Clarity | ⭐⭐⭐⭐⭐ | Very clear separation |
| Maintainability | ⭐⭐⭐⭐ | Easy to understand |
| Code Reuse | ⭐⭐ | Limited sharing |
| Deployment | ⭐⭐⭐ | Two separate deployments |

---

### Option 2: Unified Framework

```
graph-worker/
├── framework/                (Shared)
├── python/                   (Python workers)
└── go/                       (Go workers)
```

| Aspect | Rating | Notes |
|--------|--------|-------|
| Clarity | ⭐⭐⭐ | Framework abstraction |
| Maintainability | ⭐⭐⭐ | Need to maintain framework |
| Code Reuse | ⭐⭐⭐⭐ | High conceptual reuse |
| Deployment | ⭐⭐⭐⭐ | Unified approach |

---

### Option 3: Monorepo with Independent Projects ✅ RECOMMENDED

```
graph-worker/
├── graphrag-service/         (Python)
├── operational-workers/      (Go)
└── shared/                   (Contracts)
```

| Aspect | Rating | Notes |
|--------|--------|-------|
| Clarity | ⭐⭐⭐⭐⭐ | Clear, professional structure |
| Maintainability | ⭐⭐⭐⭐⭐ | Easy to navigate and modify |
| Code Reuse | ⭐⭐⭐ | Shared docs and contracts |
| Deployment | ⭐⭐⭐⭐ | Independent but coordinated |

---

## 🐍 Python vs 🚀 Go Comparison

| Aspect | Python (GraphRAG) | Go (Operational) |
|--------|-------------------|------------------|
| **Use Case** | Knowledge graph construction | Task processing |
| **Complexity** | High (AI/ML) | Low-Medium (CRUD) |
| **Processing Time** | 15-45 min | 2-30 sec |
| **CPU Usage** | Heavy | Light-Medium |
| **Memory Usage** | 4-8GB | 64Mi-1Gi |
| **Concurrency Model** | Async/await + multiprocessing | Goroutines |
| **External Dependencies** | OpenAI, MongoDB | Mock services |
| **Deployment Size** | ~2GB image | ~20MB image |
| **Startup Time** | 30-60s | 1-2s |
| **Scaling Strategy** | Vertical (bigger pods) | Horizontal (more pods) |

---

## 📊 Cost-Benefit Analysis

### GraphRAG Service

**Costs:**
- Large Docker images (~2GB)
- High memory requirements (4-8GB per pod)
- LLM API costs ($0.50-$2 per document)
- MongoDB storage costs
- Slow startup times

**Benefits:**
- Powerful knowledge graph capabilities
- Multi-hop reasoning
- Community detection
- Hybrid retrieval

**Verdict:** High value for AI/knowledge features, worth the cost

---

### Operational Workers

**Costs:**
- Need to maintain Go codebase
- Multiple deployments (3 worker types)
- Coordination with message contracts

**Benefits:**
- Very lightweight (~20MB images)
- Fast startup (<2s)
- Efficient resource usage
- Easy to scale horizontally

**Verdict:** Essential for operational tasks, low overhead

---

## 🎯 Recommended Tech Stack

### GraphRAG Service (Python)

```python
# Core
python = "3.11+"

# RabbitMQ (async - required for GraphRAG async pipelines)
aio-pika = "9.4.0"          # Async consumer ✅ DECIDED

# GraphRAG dependencies (existing - from /microservices/legacy_project/GraphRAG/)
openai = "1.42.0"
langchain = "0.2.14"
pymongo = "4.7.0"           # For MongoDB Atlas
networkx = "3.0"
# ... (50+ more dependencies from GraphRAG/requirements.txt)

# NEW dependencies for worker
flask = "3.0.0"             # Health server (or fastapi for consistency)
prometheus-client = "0.20.0" # Metrics
```

### Operational Workers (Go)

```go
// Core
go 1.22

// RabbitMQ
github.com/rabbitmq/amqp091-go v1.10.0

// HTTP
github.com/gin-gonic/gin v1.10.1

// Metrics
github.com/prometheus/client_golang v1.22.0

// Logging
go.uber.org/zap v1.27.0

// Utils
github.com/google/uuid v1.6.0
```

---

## 🔑 Critical Success Factors

### For GraphRAG Service
1. ✅ Successfully integrate RabbitMQ consumer
2. ✅ Maintain GraphRAG pipeline performance
3. ✅ Handle rate limiting properly
4. ✅ Graceful shutdown (important - processing is long)
5. ✅ Memory management (avoid OOM)

### For Operational Workers
1. ✅ Copy worker-service common foundation correctly
2. ✅ Maintain message processing speed
3. ✅ Proper error handling and retries
4. ✅ Independent scaling per worker type

### For Integration
1. ✅ Consistent message format across all workers
2. ✅ api-service → RabbitMQ → workers flow works
3. ✅ Monitoring covers all components
4. ✅ Can deploy all workers in Kubernetes

---

## ⚠️ Risk Assessment

| Risk | Impact | Likelihood | Mitigation |
|------|--------|------------|------------|
| GraphRAG memory overflow | High | Medium | Resource limits, monitoring, OOMKilled restarts |
| Python worker slower than expected | Medium | Low | Already optimized, use existing implementation |
| Message format inconsistencies | High | Medium | Strict schema validation, shared contracts doc |
| Queue depth explosion | High | Low | HPA on queue depth, monitoring alerts |
| Go worker complexity creep | Low | Medium | Stick to simple operational tasks |
| LLM rate limit exceeded | Medium | Medium | Already handled in GraphRAG, ensure config correct |

---

## 🗳️ Decision Matrix

### Confirmed Decisions ✅

| Decision | Choice | Rationale |
|----------|--------|-----------|
| **Project Names** | `graphrag-service` + `operational-workers` | Clear, professional naming |
| **Python Consumer** | **aio-pika** (async) | GraphRAG pipelines are async (`await pipeline.run()`) |
| **MongoDB** | **MongoDB Atlas** (managed) | No ops overhead, vector search built-in |
| **Document Storage** | **MinIO in Kubernetes** | Full control, S3-compatible, no cloud costs |
| **Common Package** | Copy into operational-workers | Self-contained project |
| **Results Feedback** | Via RabbitMQ results-exchange | Recommended (implement in Phase 2) |

### Deferred Decision ⏳

| Decision | Options | Status |
|----------|---------|--------|
| **GraphRAG Service Role** | A: Worker only, B: Worker+API, C: Separate services | **Decide after Phase 1** |

**Recommendation:** Start with **Option A (Worker Only)** for simplicity. Add API capabilities later if needed.

**Rationale for each option:**

| Option | Pros | Cons | When to Choose |
|--------|------|------|----------------|
| **A: Worker Only** | Simple, clean, no port conflicts | Can't query graph directly | Phase 1 - focus on processing |
| **B: Worker + API** | Convenient queries, uses existing GraphRAG APIs | More complex, dual responsibility | Phase 2 - if direct queries needed |
| **C: Separate Services** | Clean separation, independent scaling | Another deployment | Phase 3 - if high query load |

---

*See [BRAINSTORM_GRAPH_WORKER_ARCHITECTURE.md](../BRAINSTORM_GRAPH_WORKER_ARCHITECTURE.md) for detailed analysis*
