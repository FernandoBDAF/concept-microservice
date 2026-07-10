# Development Documentation

Development guides and best practices for the API Service.

## Contents

### Best Practices

- [Overview](best-practices/overview.md) - General development best practices
- [Logging](best-practices/logging-best-practices.md) - Structured logging patterns
- [Error Handling](best-practices/error-handling-best-practices.md) - Error handling strategies
- [Database](best-practices/database-best-practices.md) - PostgreSQL and data access patterns
- [Security](best-practices/security-best-practices.md) - Security coding practices
- [API Design](best-practices/api-design-best-practices.md) - REST API design guidelines

### Tools

- [Docker](tools/docker.md) - Container development and deployment
- [Kubernetes](tools/kubernetes.md) - Kubernetes overview
- [Kubernetes Tools](tools/kubernetes/) - Helm, Kustomize, and deployment tools
- [PostgreSQL](tools/postgresql.md) - Database setup and patterns (sqlx)
- [Redis](tools/redis.md) - Caching setup and patterns (go-redis)
- [Prometheus](tools/prometheus.md) - Metrics collection
- [Grafana](tools/grafana.md) - Dashboards and visualization
- [Testing Frameworks](tools/testing-frameworks.md) - Go testing tools and patterns

## Quick Start

```bash
# Prerequisites
- Go 1.22+
- Docker & Docker Compose
- Make

# Setup
cd api-service
make deps        # Install dependencies
make run         # Start the service
make test        # Run tests
```

## Project Structure

```
api-service/
├── cmd/server/              # Application entry point
├── internal/
│   ├── api/                 # HTTP layer
│   │   ├── handlers/        # Request handlers
│   │   ├── middleware/      # Auth, logging, metrics
│   │   └── router.go        # Route registration
│   ├── domain/              # Business logic
│   │   ├── profile/         # Profile domain
│   │   └── task/            # Task domain
│   ├── infrastructure/      # External systems
│   │   ├── postgres/        # Database access (sqlx)
│   │   ├── redis/           # Cache access (go-redis)
│   │   ├── rabbitmq/        # Message publishing (amqp091-go)
│   │   └── auth/            # Auth service client
│   ├── config/              # Configuration
│   └── pkg/                 # Shared utilities
├── migrations/              # Database migrations
└── deployments/             # Kubernetes manifests
```

### Patterns

- [Patterns Overview](patterns/README.md) - Development patterns for the consolidated architecture
- [Caching Patterns](patterns/caching-patterns.md) - Direct Redis patterns
- [Data Storage Patterns](patterns/data-storage-patterns.md) - Direct PostgreSQL patterns
- [Queuing Patterns](patterns/queuing-patterns.md) - Direct RabbitMQ patterns
- [Long-Running Tasks](patterns/long-running-tasks.md) - Background task patterns
- [Monitoring Patterns](patterns/monitoring-patterns.md) - Prometheus metrics and health checks
- [Security Patterns](patterns/security-patterns.md) - Authentication and authorization

### AI Development

- [AI Development Overview](ai-development/README.md) - Cursor IDE reference materials
- [AI-Assisted Development](ai-development/guides/ai-assisted-development.md) - Best practices for AI-assisted coding
- [Context Configuration](ai-development/context/context-configuration.md) - Managing project context
- [Keyboard Shortcuts](ai-development/guides/keyboard-shortcuts.md) - Productivity shortcuts
- [Configuration Examples](ai-development/configuration/configuration-examples.md) - Cursor settings

## Architecture Context

This documentation applies to the **consolidated service architecture** with direct infrastructure access:

| Component | Library | Description |
|-----------|---------|-------------|
| PostgreSQL | sqlx | Direct database access |
| Redis | go-redis | Direct cache access |
| RabbitMQ | amqp091-go | Direct message publishing |
| Auth | HTTP client | External auth-service |

---

*Last Updated: January 2026*
