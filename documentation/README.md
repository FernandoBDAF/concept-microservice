# Documentation

Active documentation for the Profile Service API.

## Structure

```
documentation/
├── architecture/           System architecture and design decisions
├── development/            Development guides and best practices
│   ├── best-practices/     Coding standards and patterns
│   └── tools/              Tool-specific documentation
├── performance/            Performance optimization and testing
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

Content from `legacy_project/reference-materials/` is being migrated here.
See [LEGACY_CONTENT_MIGRATION_STUDY.md](../legacy_project/reference-materials/LEGACY_CONTENT_MIGRATION_STUDY.md) for the complete migration plan.

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

**Last Updated:** January 2026
