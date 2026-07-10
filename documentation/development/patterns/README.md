# Development Patterns

> *Migrated and adapted from legacy_project/reference-materials/development/patterns/*

Development patterns for the API Service architecture with direct infrastructure access.

## Contents

### Data Patterns
- [Caching Patterns](caching-patterns.md) - Direct Redis caching with go-redis
- [Data Storage Patterns](data-storage-patterns.md) - Direct PostgreSQL with sqlx
- [Caching Best Practices](caching-best-practices.md) - Comprehensive caching guide

### Messaging Patterns
- [Queuing Patterns](queuing-patterns.md) - Direct RabbitMQ with amqp091-go
- [Long-Running Tasks](long-running-tasks.md) - Background task processing

### Observability Patterns
- [Monitoring Patterns](monitoring-patterns.md) - Prometheus metrics and health checks

### Security Patterns
- [Security Patterns](security-patterns.md) - Authentication and authorization

## Architecture Context

These patterns have been adapted for the **consolidated service architecture** with direct infrastructure access:

| Component | Library | Old Approach | New Approach |
|-----------|---------|--------------|--------------|
| Cache | go-redis | HTTP to cache-service | Direct Redis access |
| Database | sqlx | HTTP to storage-service | Direct PostgreSQL access |
| Queue | amqp091-go | HTTP to queue-service | Direct RabbitMQ publish |

## Quick Reference

### Direct Redis Access
```go
// OLD: HTTP client to cache-service (deprecated)
// cacheClient.Get(ctx, "profile:"+id)

// NEW: Direct go-redis access
result, err := redisClient.Get(ctx, "profile:"+id).Result()
```

### Direct PostgreSQL Access
```go
// OLD: HTTP client to storage-service (deprecated)
// storageClient.GetProfile(ctx, id)

// NEW: Direct sqlx access
var profile Profile
err := db.GetContext(ctx, &profile, query, id)
```

### Direct RabbitMQ Publish
```go
// OLD: HTTP client to queue-service (deprecated)
// queueClient.Publish(ctx, msg)

// NEW: Direct amqp091-go publish
err := channel.PublishWithContext(ctx, exchange, routingKey, false, false, msg)
```

---

*Last Updated: January 2026*
