# Documentation

Active documentation for the microservices platform. Cross-service contracts
live at the repo root in [CONTRACTS.md](../CONTRACTS.md).

## Start here

- **[phases/](phases/)** — the development methodology: one implementation brief per milestone (start any working session here)
- **[decisions/](decisions/)** — ADR-001..010: every architectural/tooling decision with its why (2026-07-10 Q&A)
- **[PRD.md](PRD.md)** — project vision, resequenced roadmap (v1–v8), success metrics
- **[../EXPERIMENTS.md](../EXPERIMENTS.md)** — the guided experiment catalog; write-ups go to [experiments/](experiments/)
- **[review/CONCEPTUAL_REVIEW.md](review/CONCEPTUAL_REVIEW.md)** — architecture-level issues & tensions
- **[review/INFERRED_INTENT.md](review/INFERRED_INTENT.md)** — reconstructed goals across the project's three eras
- **[refactor/2026-07-full-refactor.md](refactor/2026-07-full-refactor.md)** — what the 2026-07 refactor did and verified

## Structure

```
documentation/
├── PRD.md                  Product vision, roadmap, success metrics
├── phases/                 One implementation brief per milestone (v1–v8)
├── decisions/              ADRs — decisions with their why
├── review/                 Conceptual review + inferred intent
├── refactor/               Refactor records (2026-07)
├── architecture/           System architecture and design decisions
├── development/            Development guides and best practices
│   ├── best-practices/     Coding standards and patterns
│   └── tools/              Tool-specific documentation
├── deployment/             Local (compose) + Kubernetes deployment docs
├── performance/            Performance optimization and testing
├── planning/               Historical planning documents (pre-refactor)
├── templates/              Documentation templates
│   ├── api/                API documentation templates
│   ├── architecture/       Architecture templates
│   ├── development/        Development templates
│   ├── operations/         Operations templates
│   ├── security/           Security templates
│   ├── testing/            Testing templates
│   └── maintenance/        Maintenance templates
├── deployment/             Kubernetes deployment and operations
└── api/                    API specifications and examples
```

## Quick Links

### Architecture
- [Architecture Overview](architecture/README.md)

### Development
- [Development Overview](development/README.md)
- **Best Practices:**
  - [Logging](development/best-practices/logging-best-practices.md)
  - [Error Handling](development/best-practices/error-handling-best-practices.md)
  - [Database](development/best-practices/database-best-practices.md)
  - [Security](development/best-practices/security-best-practices.md)
  - [API Design](development/best-practices/api-design-best-practices.md)
- **Tools:**
  - [Docker](development/tools/docker.md)
  - [Kubernetes](development/tools/kubernetes.md)
  - [PostgreSQL](development/tools/postgresql.md)
  - [Redis](development/tools/redis.md)
  - [Prometheus](development/tools/prometheus.md)
  - [Testing Frameworks](development/tools/testing-frameworks.md)

### Performance
- [Performance Overview](performance/README.md)
- [Load Testing Strategy](performance/load-testing-strategy.md)
- [Benchmarking](performance/benchmarking.md)
- [Monitoring](performance/monitoring.md)
- [Optimization](performance/optimization.md)

### Deployment
- [Deployment Overview](deployment/README.md)

### API
- [API Overview](api/README.md)

### Planning (historical)
- [Master Implementation Plan](planning/MASTER_IMPLEMENTATION_PLAN.md)
- [Consolidated Service Plan](planning/CONSOLIDATED_SERVICE_PLAN.md)
- [GraphRAG & Concurrency Plan](planning/GRAPHRAG_AND_CONCURRENCY_PLAN.md)
- [Auth Service & Deployment Plan](planning/PLAN_AUTH_SERVICE_AND_DEPLOYMENT.md)
- [Graph Worker Brainstorm](planning/BRAINSTORM_GRAPH_WORKER_ARCHITECTURE.md)
- [2026-01 Refactor Verification](planning/REFACTOR_VERIFICATION_REPORT.md)

> These predate the 2026-07 refactor; where they disagree with the code,
> root README, or CONTRACTS.md, the latter win.

### Templates
- [Templates Overview](templates/README.md)
- [LLM-Friendly Template](templates/LLM_FRIENDLY_TEMPLATE.md)
- [README Template](templates/README_TEMPLATE.md)
- [Architecture Templates](templates/architecture/)
- [API Templates](templates/api/)
- [Operations Templates](templates/operations/)
- [Testing Templates](templates/testing/)

## Content Status

| Section | Status | Files |
|---------|--------|-------|
| Best Practices | ✅ Migrated | 6 files |
| Tools Documentation | ✅ Migrated | 10 files |
| Performance Docs | ✅ Migrated | 5 files |
| Templates | ✅ Migrated | 38 files |
| Patterns | ✅ Migrated | 7 files |
| AI/Cursor Guides | ✅ Migrated | 10 files |

## Migration from Legacy

Content from era-1's `reference-materials/` was migrated here; the source
tree (and `reference-materials/LEGACY_CONTENT_MIGRATION_STUDY.md`) lives on
branch `archive/era-1` (ADR-010.4).

### Completed Migrations

- ✅ Best practices documentation (6 files)
- ✅ Tool documentation (10 files)
- ✅ Performance documentation (5 files)
- ✅ Templates (38 files)
- ✅ Pattern documentation (7 files with adaptations)
- ✅ AI/Cursor guides (10 files)

### What Stays in Legacy

- ❌ Old service documentation (cache-service, queue-service, storage-service)
- ❌ HTTP service-to-service patterns
- ❌ Old deployment configurations
- ❌ Historical context documents

---

**Last Updated:** July 2026
