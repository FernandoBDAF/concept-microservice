# Profile Service - Microservices Project

A modern, high-performance profile management system built with Go, featuring direct infrastructure access and asynchronous task processing.

## 🏗️ Architecture Overview

This project uses a **consolidated service architecture** with direct infrastructure access, replacing the previous HTTP-based microservices pattern.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           CURRENT ARCHITECTURE                               │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│                         API Service (Go/Gin)                                 │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │   Profile Handler → Profile Service → Direct Infrastructure Access   │   │
│  │        ↓                   ↓                     ↓                   │   │
│  │   ┌─────────┐    ┌──────────────┐    ┌─────────────┐               │   │
│  │   │  Redis  │    │  PostgreSQL  │    │  RabbitMQ   │               │   │
│  │   │(go-redis)│   │(sqlx+lib/pq) │    │(amqp091-go) │               │   │
│  │   └────┬────┘    └──────┬───────┘    └──────┬──────┘               │   │
│  └────────┼────────────────┼───────────────────┼──────────────────────┘   │
└───────────┼────────────────┼───────────────────┼──────────────────────────┘
            │                │                   │
            ▼                ▼                   ▼
         ┌─────┐        ┌──────────┐       ┌──────────┐
         │Redis│        │PostgreSQL│       │ RabbitMQ │──▶ Workers (Future)
         └─────┘        └──────────┘       └──────────┘

External Dependencies:
┌──────────────┐
│ Auth Service │ ◀── JWT Validation (only HTTP dependency)
│  (Node.js)   │
└──────────────┘
```

### Key Benefits

- ✅ **~10x faster** - Direct connections, no HTTP overhead between services
- ✅ **Single deployment** - One container for the API service
- ✅ **Simpler debugging** - All business logic in one place
- ✅ **Atomic operations** - Proper database transactions
- ✅ **Less code** - No HTTP clients or circuit breakers between internal services

---

## 📁 Project Structure

```
microservices/
│
├── api-service/                    # ✅ Main API Service (Go)
│   ├── cmd/server/                 # Application entry point
│   ├── internal/
│   │   ├── api/                    # HTTP handlers, middleware, router
│   │   ├── domain/                 # Business logic (Profile, Task)
│   │   └── infrastructure/         # Direct clients (Postgres, Redis, RabbitMQ)
│   ├── migrations/                 # Database migrations
│   ├── deployments/kubernetes/     # K8s manifests
│   └── README.md                   # Service documentation
│
├── graph-worker/                   # 🔄 GraphRAG Worker (Planning)
│   ├── PROJECT_OVERVIEW.md         # Quick reference
│   ├── ARCHITECTURE_OPTIONS.md     # Architecture comparison
│   └── IMPLEMENTATION_ROADMAP.md   # Implementation phases
│
├── legacy_project/GraphRAG/        # 📚 GraphRAG Library - External Reference (Python)
│   ├── src/                        # Source code
│   ├── docs/                       # Library documentation
│   └── configs/                    # Configuration files
│
├── documentation/                  # 📖 Active Documentation (NEW)
│   ├── architecture/               # Current system architecture
│   ├── development/                # Development guides
│   ├── deployment/                 # Deployment guides
│   └── api/                        # API specifications
│
├── legacy_project/                 # 📦 Legacy Content (Archived)
│   ├── reference-materials/        # Old documentation (278 files)
│   ├── services/                   # Old microservices code
│   └── k8s/                        # Old K8s configurations
│
└── [Planning Documents]
    ├── CONSOLIDATED_SERVICE_PLAN.md
    ├── GRAPHRAG_AND_CONCURRENCY_PLAN.md
    ├── BRAINSTORM_GRAPH_WORKER_ARCHITECTURE.md
    └── REFACTOR_VERIFICATION_REPORT.md
```

---

## 🚀 Quick Start

### Prerequisites

- Go 1.22+
- Docker & Docker Compose
- Kubernetes cluster (Kind for local development)
- PostgreSQL 15+
- Redis 7+
- RabbitMQ 3.12+

### Local Development

```bash
# Clone the repository
git clone <repository-url>
cd microservices

# Start infrastructure (PostgreSQL, Redis, RabbitMQ)
docker-compose up -d

# Run the API service
cd api-service
make run

# Run tests
make test
```

### API Endpoints

```
# Health & Monitoring
GET  /health                              # Basic health check
GET  /ready                               # Readiness (DB + Redis + RabbitMQ)
GET  /metrics                             # Prometheus metrics

# Profile CRUD (requires authentication)
GET    /api/v1/profiles                   # List profiles (paginated)
POST   /api/v1/profiles                   # Create profile
GET    /api/v1/profiles/:id               # Get profile by ID
PUT    /api/v1/profiles/:id               # Update profile
DELETE /api/v1/profiles/:id               # Delete profile

# Task Submission (publishes to RabbitMQ)
POST   /api/v1/profiles/:id/tasks/email   # Email task → email.send
POST   /api/v1/profiles/:id/tasks/image   # Image task → image.process
POST   /api/v1/profiles/:id/tasks/profile # Profile task → profile.task
```

---

## 📖 Documentation

### Active Documentation

| Document | Description |
|----------|-------------|
| [API Service README](api-service/README.md) | Service overview, endpoints, configuration |
| [Consolidated Service Plan](CONSOLIDATED_SERVICE_PLAN.md) | Architecture design and decisions |
| [GraphRAG Plan](GRAPHRAG_AND_CONCURRENCY_PLAN.md) | GraphRAG integration planning |
| [Graph Worker Overview](graph-worker/PROJECT_OVERVIEW.md) | Worker system architecture |
| [Refactor Verification](REFACTOR_VERIFICATION_REPORT.md) | Migration status and verification |

### Documentation Folder (Coming Soon)

```
documentation/
├── architecture/           System architecture and design
├── development/            Development guides and best practices
├── deployment/             Kubernetes deployment guides
└── api/                    API specifications and examples
```

### Legacy Documentation

Historical documentation from the previous microservices architecture is preserved in:

```
legacy_project/reference-materials/
├── architecture/           Old architecture patterns
├── development/            Old development guides
├── templates/              Documentation templates (still useful)
└── [Navigation docs]       START_HERE.md, QUICK_REFERENCE_GUIDE.md
```

> ⚠️ **Note:** Legacy documentation describes the old HTTP-based microservices pattern. 
> Use for historical reference only. See [Migration Study](legacy_project/reference-materials/LEGACY_CONTENT_MIGRATION_STUDY.md) for details.

---

## 🔧 Technology Stack

### API Service (Go)

| Component | Library | Purpose |
|-----------|---------|---------|
| HTTP Framework | `gin-gonic/gin` | REST API routing |
| PostgreSQL | `jmoiron/sqlx` + `lib/pq` | Database access |
| Redis | `go-redis/redis/v8` | Caching |
| RabbitMQ | `rabbitmq/amqp091-go` | Message publishing |
| Configuration | `spf13/viper` | Config management |
| Logging | `uber-go/zap` | Structured logging |
| Metrics | `prometheus/client_golang` | Prometheus metrics |

### GraphRAG Service (Python)

| Component | Library | Purpose |
|-----------|---------|---------|
| LLM Integration | OpenAI/Anthropic | Entity extraction |
| Graph Database | MongoDB | Knowledge graph storage |
| Message Queue | pika | RabbitMQ consumer |

### Infrastructure

| Component | Technology | Purpose |
|-----------|------------|---------|
| Container Orchestration | Kubernetes | Service deployment |
| Database | PostgreSQL 15 | Profile data storage |
| Cache | Redis 7 | Profile caching |
| Message Broker | RabbitMQ 3.12 | Async task queue |
| Monitoring | Prometheus + Grafana | Observability |

---

## 🏛️ Architecture Evolution

This project evolved from a traditional HTTP-based microservices architecture to a consolidated service model:

### Previous Architecture (Legacy)

```
profile-service → HTTP → cache-service → Redis
                → HTTP → storage-service → PostgreSQL
                → HTTP → queue-service → RabbitMQ
```

**Issues:** Network latency, HTTP overhead, complex debugging, 4+ deployments

### Current Architecture (Consolidated)

```
api-service → Direct → Redis
            → Direct → PostgreSQL
            → Direct → RabbitMQ
```

**Benefits:** ~10x faster, single deployment, simpler debugging, atomic transactions

See [CONSOLIDATED_SERVICE_PLAN.md](CONSOLIDATED_SERVICE_PLAN.md) for detailed comparison.

---

## 🛣️ Roadmap

### Completed ✅

- [x] Project restructuring (legacy → consolidated)
- [x] API service implementation (api-service/)
- [x] Direct infrastructure access (PostgreSQL, Redis, RabbitMQ)
- [x] Documentation migration planning

### In Progress 🔄

- [ ] GraphRAG worker implementation
- [ ] Documentation folder setup
- [ ] Kubernetes deployment refinement

### Planned 📋

- [ ] Auth service integration (Clerk)
- [ ] Operational workers (email, image, profile)
- [ ] Production deployment guides
- [ ] Load testing and performance optimization

---

## 📚 Related Documents

| Document | Purpose |
|----------|---------|
| [CONSOLIDATED_SERVICE_PLAN.md](CONSOLIDATED_SERVICE_PLAN.md) | Architecture design |
| [GRAPHRAG_AND_CONCURRENCY_PLAN.md](GRAPHRAG_AND_CONCURRENCY_PLAN.md) | GraphRAG integration |
| [BRAINSTORM_GRAPH_WORKER_ARCHITECTURE.md](BRAINSTORM_GRAPH_WORKER_ARCHITECTURE.md) | Worker architecture |
| [REFACTOR_VERIFICATION_REPORT.md](REFACTOR_VERIFICATION_REPORT.md) | Migration verification |
| [graph-worker/IMPLEMENTATION_ROADMAP.md](graph-worker/IMPLEMENTATION_ROADMAP.md) | Worker implementation |

---

## 🤝 Contributing

1. Check existing documentation before making changes
2. Follow the established patterns in `api-service/`
3. Update relevant documentation when adding features
4. Test locally before submitting changes

---

## 📄 License

[Add license information]

---

**Last Updated:** January 2026  
**Architecture Version:** Consolidated Service (v1)  
**Status:** Active Development
