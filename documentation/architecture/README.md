# Architecture Documentation

System architecture and design decisions for the Profile Service.

## Contents

- [System Overview](overview.md) - High-level architecture
- [API Service](api-service.md) - Consolidated API service design
- [Graph Worker](graph-worker.md) - GraphRAG worker architecture
- [Decisions](decisions.md) - Key architectural decisions

## Current Architecture

The system uses a **consolidated service architecture** with direct infrastructure access:

```
┌─────────────────────────────────────────────────────────────────┐
│                      API Service (Go/Gin)                        │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │  Handlers → Services → Direct Infrastructure Access        │  │
│  └───────────────────────────────────────────────────────────┘  │
│            │                │                    │               │
│            ▼                ▼                    ▼               │
│        ┌───────┐      ┌──────────┐       ┌──────────┐          │
│        │ Redis │      │PostgreSQL│       │ RabbitMQ │          │
│        └───────┘      └──────────┘       └──────────┘          │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
                    ┌──────────────────┐
                    │  Graph Workers   │
                    │  (Python/Go)     │
                    └──────────────────┘
```

## Key Principles

1. **Direct Infrastructure Access** - No HTTP intermediaries
2. **Clean Architecture** - Handlers → Services → Repositories
3. **Single Deployment** - One container for API service
4. **External Workers** - Separate deployments for async processing

## Related Documents

- [CONSOLIDATED_SERVICE_PLAN.md](../../CONSOLIDATED_SERVICE_PLAN.md) - Full architecture plan
- [api-service/README.md](../../api-service/README.md) - Implementation details
