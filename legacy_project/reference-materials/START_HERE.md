# 🚀 Start Here - Legacy Reference Materials Navigation

---
> ⚠️ **LEGACY DOCUMENTATION**  
> This folder documents the **old HTTP-based microservices architecture**.
> The project has been refactored. See `/api-service/` for current architecture.
---

Welcome to the Profile Service legacy reference materials! This guide helps you navigate the historical documentation for the original microservices architecture.

---

## 📍 Choose Your Path

### 🏗️ I want to understand the architecture

**Current Architecture (Consolidated Service):**
- [System Architecture Overview](architecture/overview/system-architecture.md) - Start here for architecture overview
- [Consolidated Service Design](architecture/services/README.md) - Single-service architecture details
- [Design Decisions](architecture/overview/design-decisions.md) - Why we chose this architecture

**Architecture Components:**
- [API Layer](architecture/patterns/) - HTTP API patterns
- [Domain Layer](development/patterns/) - Business logic patterns
- [Infrastructure Layer](architecture/communication/direct-client-patterns.md) - Direct database/cache/queue access

**Coming from Old Architecture?**
- [Architecture Evolution](architecture/overview/future-roadmap.md) - How we got here
- [Migration Guide](DOCUMENTATION_REVIEW_AND_REFACTOR_PLAN.md#4-proposed-refactoring-strategy) - Migrating from microservices

---

### 💻 I want to develop features

**Getting Started:**
- [Development Overview](development/README.md) - Development guide
- [Best Practices](development/best-practices.md) - Coding standards
- [Development Workflow](templates/development/workflow-process-template.md) - How we work

**Patterns & Practices:**
- [API Design](development/api-design-best-practices.md) - REST API patterns
- [Error Handling](development/error-handling-best-practices.md) - Error patterns
- [Database Access](development/database-best-practices.md) - PostgreSQL/sqlx patterns
- [Caching](development/caching-best-practices.md) - Redis/go-redis patterns
- [Security](development/security-best-practices.md) - Security patterns

**Worker Services:**
- [Worker Patterns](development/patterns/worker-service-patterns.md) - Background job patterns
- [Queuing](development/patterns/queuing-patterns.md) - RabbitMQ patterns
- [Long-Running Tasks](development/patterns/long-running-tasks.md) - Async processing

**Tools & Technologies:**
- [Kubernetes](development/tools/kubernetes/) - K8s deployment tools (Helm, Kustomize)
- [Docker](development/tools/docker.md) - Container development
- [PostgreSQL](development/tools/postgresql.md) - Database tools
- [Redis](development/tools/redis.md) - Cache tools
- [gRPC](development/tools/grpc.md) - gRPC patterns

---

### 🚀 I want to deploy the system

**Deployment Guides:**
- [Deployment Overview](templates/operations/deployment-guide.md) - Start here for deployment
- [Kubernetes Setup](templates/operations/kubernetes-setup.md) - K8s cluster setup
- [Environment Setup](templates/operations/environment-setup.md) - Environment configuration
- [Production Deployment](templates/operations/production-deployment.md) - Production checklist

**Deployment Tools:**
- [Helm Guide](development/tools/kubernetes/helm.md) - Helm charts
- [Kustomize Guide](development/tools/kubernetes/kustomize.md) - Kustomize overlays
- [Tool Comparison](development/tools/kubernetes/comparison.md) - When to use which tool

**Scaling & Operations:**
- [Scaling Guide](templates/operations/scaling-guide.md) - Scaling strategies
- [Monitoring Setup](templates/operations/monitoring-guide-template.md) - Observability
- [Troubleshooting](templates/maintenance/troubleshooting-template.md) - Common issues

---

### 📈 I want to optimize performance

**Performance Guides:**
- [Performance Overview](performance/README.md) - Performance guide
- [Optimization Techniques](performance/optimization.md) - Optimization patterns
- [Load Testing](performance/load-testing-strategy.md) - Load testing strategy
- [Benchmarking](performance/benchmarking.md) - Benchmarking guide

**Monitoring & Analysis:**
- [Performance Monitoring](performance/monitoring.md) - Metrics and dashboards
- [Prometheus](development/tools/prometheus.md) - Metrics collection
- [Grafana](development/tools/grafana.md) - Visualization

---

### 🔒 I want to secure the system

**Security Documentation:**
- [Security Overview](security/README.md) - Security guide
- [Security Guide](security/guide.md) - Comprehensive security guide
- [Security Best Practices](development/security-best-practices.md) - Coding security

**Security Patterns:**
- [Authentication Patterns](architecture/security/) - Auth patterns
- [API Security](templates/api/api-security.md) - API security
- [Security Monitoring](templates/security/security-monitoring-template.md) - Security observability

---

### 📚 I want templates and examples

**Templates:**
- [Template Library](templates/README.md) - All templates
- [API Documentation Template](templates/api/api-documentation.md) - API docs
- [Architecture Template](templates/architecture/architecture-template.md) - Architecture docs
- [Testing Template](templates/testing/testing-template.md) - Test docs

**Best Practices:**
- [Logging Best Practices](templates/maintenance/logging-best-practices.md) - Logging guidelines
- [LLM Guidelines](templates/development/llm-guidelines.md) - AI-assisted development

---

### 🔄 I'm migrating from the old architecture

**Migration Resources:**
- [Architecture Evolution](architecture/overview/future-roadmap.md) - How architecture evolved
- [Documentation Review & Refactor Plan](DOCUMENTATION_REVIEW_AND_REFACTOR_PLAN.md) - Complete migration analysis
- [Service Comparison](DOCUMENTATION_REVIEW_AND_REFACTOR_PLAN.md#11-project-evolution-analysis) - Old vs new architecture

**What Changed:**
- ❌ **Removed:** cache-service, queue-service, storage-service (HTTP wrappers)
- ✅ **Added:** Direct connections to PostgreSQL, Redis, RabbitMQ
- ✅ **Simplified:** Single deployment, atomic transactions, reduced latency
- ✅ **Performance:** ~10x faster due to eliminated HTTP overhead

---

## 🗺️ Navigation Aids

### Quick Reference Indexes

- **[Cross-Reference Index](CROSS_REFERENCE_INDEX.md)** - Complete document cross-references
- **[Tracker & Manager](TRACKER&MANAGER.md)** - Development progress tracking
- **[Main README](README.md)** - Overview of reference materials

### Documentation by Type

- **Architecture:** [architecture/README.md](architecture/README.md)
- **Development:** [development/README.md](development/README.md)
- **Performance:** [performance/README.md](performance/README.md)
- **Security:** [security/README.md](security/README.md)
- **Templates:** [templates/README.md](templates/README.md)

### For LLM Consumption

All documentation includes:
- **Structured metadata** - YAML frontmatter with categories, tags, relationships
- **Context blocks** - "INITIAL CONTEXT FOR LLM" sections
- **Cross-references** - Explicit links to related content
- **Examples** - Code examples and patterns

---

## 🤖 For AI Assistants (LLMs)

If you're an LLM reading this:

**Current Architecture Context:**
- Project uses **consolidated service architecture** (single Go service)
- Direct infrastructure access (PostgreSQL via sqlx, Redis via go-redis, RabbitMQ via amqp091-go)
- No HTTP wrappers for infrastructure (cache/queue/storage services removed)
- Only external HTTP dependency: auth-service for JWT validation

**Legacy Architecture (Archived):**
- Old microservices architecture with HTTP service-to-service communication
- Separate cache-service, queue-service, storage-service (now consolidated)
- Documentation marked as "legacy" or "archived" refers to this old architecture

**When Referencing Documentation:**
1. Check document status (current vs. archived)
2. Verify architecture version (consolidated-v1 vs. microservices-v1)
3. Use CROSS_REFERENCE_INDEX.md for navigation
4. Follow semantic relationships for related content

**Key Documentation Areas:**
- Architecture patterns: `architecture/patterns/`
- Development patterns: `development/patterns/`
- Best practices: `development/best-practices/`
- Deployment: `templates/operations/`

---

## 📞 Need Help?

- **Can't find what you need?** Check the [Cross-Reference Index](CROSS_REFERENCE_INDEX.md)
- **Documentation unclear?** Review the [Documentation Review Plan](DOCUMENTATION_REVIEW_AND_REFACTOR_PLAN.md)
- **Reporting issues?** Update the [Tracker & Manager](TRACKER&MANAGER.md)

---

**Last Updated:** 2026-01-29  
**Architecture Version:** Consolidated Service (v1)  
**Status:** Active Development
