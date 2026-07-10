# Quick Reference Guide - Reference Materials

## 📊 Documentation Structure Overview

```
reference-materials/
│
├── 🏗️  architecture/          Architecture & design decisions
│   ├── overview/              System architecture, design decisions
│   ├── services/              Service-specific architecture
│   ├── patterns/              Design patterns (33 documents)
│   ├── communication/         Service communication patterns
│   ├── data/                  Data architecture
│   ├── security/              Security architecture
│   ├── network/               Network architecture
│   └── database/              Database architecture
│
├── 💻 development/            Development practices & tools
│   ├── patterns/              Implementation patterns (7 documents)
│   ├── best-practices/        Coding guidelines (7 documents)
│   ├── tools/                 Tool-specific guides (15+ documents)
│   └── testing/               Testing strategies
│
├── 📈 performance/            Performance & optimization
│   ├── load-testing-strategy.md
│   ├── optimization.md
│   ├── benchmarking.md
│   └── monitoring.md
│
├── 🔒 security/               Security guides & patterns
│   ├── guide.md
│   └── README.md
│
├── 📚 templates/              Documentation templates
│   ├── api/                   API documentation templates
│   ├── operations/            Deployment & ops templates
│   ├── architecture/          Architecture templates
│   ├── testing/               Testing templates
│   ├── development/           Development templates
│   ├── maintenance/           Maintenance templates
│   └── security/              Security templates
│
├── 🤖 cursor/                 AI-assisted development
│   ├── guides/                Cursor IDE guides
│   ├── patterns/              AI development patterns
│   └── context/               Context management
│
└── 📖 Main Files
    ├── START_HERE.md          👈 Start here!
    ├── README.md              Overview & structure
    ├── CROSS_REFERENCE_INDEX.md   Complete cross-references
    ├── TRACKER&MANAGER.md     Development tracking
    └── DOCUMENTATION_REVIEW_AND_REFACTOR_PLAN.md   Refactoring plan
```

---

## 🎯 Common Tasks - Where to Look

### Architecture Questions

| What do you need? | Where to look |
|-------------------|---------------|
| System overview | `architecture/overview/system-architecture.md` |
| Service architecture | `architecture/services/README.md` |
| Design decisions | `architecture/overview/design-decisions.md` |
| Architecture patterns | `architecture/patterns/` |
| Data patterns | `architecture/data/` |
| Security architecture | `architecture/security/` |

### Development Questions

| What do you need? | Where to look |
|-------------------|---------------|
| API design | `development/api-design-best-practices.md` |
| Error handling | `development/error-handling-best-practices.md` |
| Database access | `development/database-best-practices.md` |
| Caching patterns | `development/caching-best-practices.md` |
| Security practices | `development/security-best-practices.md` |
| Worker services | `development/patterns/worker-service-patterns.md` |
| Logging | `development/logging-best-practices.md` |

### Deployment Questions

| What do you need? | Where to look |
|-------------------|---------------|
| Deployment guide | `templates/operations/deployment-guide.md` |
| Kubernetes setup | `templates/operations/kubernetes-setup.md` |
| Helm usage | `development/tools/kubernetes/helm.md` |
| Kustomize usage | `development/tools/kubernetes/kustomize.md` |
| Scaling strategies | `templates/operations/scaling-guide.md` |
| Environment setup | `templates/operations/environment-setup.md` |

### Tools Documentation

| Tool | Where to find docs |
|------|-------------------|
| Kubernetes | `development/tools/kubernetes.md`, `development/tools/kubernetes/` |
| Docker | `development/tools/docker.md` |
| PostgreSQL | `development/tools/postgresql.md` |
| Redis | `development/tools/redis.md` |
| RabbitMQ | Via queuing patterns (direct AMQP usage) |
| Prometheus | `development/tools/prometheus.md` |
| Grafana | `development/tools/grafana.md` |
| gRPC | `development/tools/grpc.md` |
| Jaeger | `development/tools/jaeger.md` |

### Performance & Optimization

| What do you need? | Where to look |
|-------------------|---------------|
| Optimization guide | `performance/optimization.md` |
| Load testing | `performance/load-testing-strategy.md` |
| Benchmarking | `performance/benchmarking.md` |
| Monitoring | `performance/monitoring.md` |
| Performance guide | `performance/performance-guide.md` |

---

## 🔍 Key Concepts & Where to Find Them

### Consolidated Service Architecture

**What it is:** Single Go service with direct infrastructure access (no HTTP wrappers)

**Key documents:**
- Architecture overview: `architecture/overview/system-architecture.md`
- Service design: `architecture/services/README.md`
- Direct client patterns: `architecture/communication/` (to be created)
- Implementation: See `CONSOLIDATED_SERVICE_PLAN.md` (project root)

### Worker Services

**What they are:** Background job processors consuming from RabbitMQ

**Key documents:**
- Worker patterns: `development/patterns/worker-service-patterns.md`
- Long-running tasks: `development/patterns/long-running-tasks.md`
- Queuing patterns: `development/patterns/queuing-patterns.md`
- Monitoring: `development/patterns/monitoring-patterns.md`

### Direct Infrastructure Access

**What it is:** Services connect directly to PostgreSQL/Redis/RabbitMQ (no HTTP intermediaries)

**Key documents:**
- Database patterns: `development/database-best-practices.md`
- Caching patterns: `development/caching-best-practices.md`
- Connection pooling: `development/connection-pooling.md`
- PostgreSQL tool: `development/tools/postgresql.md`
- Redis tool: `development/tools/redis.md`

### Kubernetes Deployment

**What we use:** Helm for templating + Kustomize for environment overlays

**Key documents:**
- Helm guide: `development/tools/kubernetes/helm.md`
- Kustomize guide: `development/tools/kubernetes/kustomize.md`
- Comparison: `development/tools/kubernetes/comparison.md`
- Service evolution: `development/tools/kubernetes/service-evolution.md`
- Deployment guide: `templates/operations/deployment-guide.md`

---

## 📋 Document Status Legend

Throughout the documentation, you'll see these status indicators:

- ✅ **Complete & Current** - Aligned with consolidated service architecture
- ⚠️ **Needs Update** - Contains outdated information
- ❌ **Archived** - Legacy content (old microservices architecture)
- ✨ **New/Planned** - To be created
- ⟳ **In Progress** - Currently being updated
- 📦 **Legacy** - Historical reference only

---

## 🗺️ Navigation Tips

### For Human Developers

1. **Start with START_HERE.md** - Choose your intent-based path
2. **Use README files** - Each directory has an overview README
3. **Follow cross-references** - Links connect related topics
4. **Check status** - Verify document is current vs. archived
5. **Use templates** - Start new docs with provided templates

### For LLMs

1. **Check metadata blocks** - Every document has structured metadata
2. **Read "INITIAL CONTEXT FOR LLM"** - Context at the top of documents
3. **Use CROSS_REFERENCE_INDEX.md** - Complete navigation map
4. **Follow semantic relationships** - Explicit concept relationships
5. **Verify architecture version** - Consolidated-v1 vs microservices-v1

---

## 🚨 Important Notes

### Current Architecture (Consolidated Service)

**What it means:**
- Single Go service (`api-service`) with direct infrastructure access
- No HTTP wrappers for cache/queue/storage
- Direct connections: PostgreSQL (sqlx), Redis (go-redis), RabbitMQ (amqp091-go)
- Only external HTTP call: auth-service for JWT validation

**Impact on documentation:**
- Many old docs reference cache-service, queue-service, storage-service (now removed)
- Look for "consolidated service" or "direct client" documentation
- Check document status to ensure you're reading current architecture

### Legacy Architecture (Archived)

**What it was:**
- Multiple microservices with HTTP service-to-service communication
- Separate cache-service, queue-service, storage-service
- HTTP client libraries for inter-service calls

**Where to find it:**
- Documents marked "archived" or "legacy"
- Old service documentation in `architecture/services/legacy/` (to be created)
- Historical reference only - not current implementation

---

## 🔗 Essential Links

### Start Here
- [START_HERE.md](START_HERE.md) - Intent-based navigation
- [README.md](README.md) - Overview of reference materials

### Key Indexes
- [CROSS_REFERENCE_INDEX.md](CROSS_REFERENCE_INDEX.md) - Complete cross-references
- [TRACKER&MANAGER.md](TRACKER&MANAGER.md) - Development progress

### Planning Documents
- [DOCUMENTATION_REVIEW_AND_REFACTOR_PLAN.md](DOCUMENTATION_REVIEW_AND_REFACTOR_PLAN.md) - Refactoring plan
- `CONSOLIDATED_SERVICE_PLAN.md` (project root) - Architecture plan

### By Category
- [Architecture](architecture/README.md)
- [Development](development/README.md)
- [Performance](performance/README.md)
- [Security](security/README.md)
- [Templates](templates/README.md)

---

## 💡 Tips for Effective Use

1. **Bookmark key documents** - Keep frequently used docs handy
2. **Use search** - Most IDEs have project-wide search
3. **Follow patterns** - Look for similar documents as examples
4. **Check dates** - Recent updates indicate current information
5. **Contribute back** - Update docs as you learn new things

---

**Last Updated:** 2026-01-29  
**Architecture Version:** Consolidated Service (v1)  
**Maintained by:** Development Team
