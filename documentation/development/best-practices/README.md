# Best Practices

Development best practices and coding guidelines for the Profile Service.

## Contents

- [Overview](overview.md) - General best practices summary
- [Logging](logging-best-practices.md) - Structured logging patterns
- [Error Handling](error-handling-best-practices.md) - Error handling strategies
- [Database](database-best-practices.md) - PostgreSQL and data access patterns
- [Security](security-best-practices.md) - Security coding practices
- [API Design](api-design-best-practices.md) - REST API design guidelines

## Quick Reference

These best practices apply to the consolidated service architecture with direct infrastructure access:

- **Database:** Direct PostgreSQL access via sqlx
- **Cache:** Direct Redis access via go-redis
- **Queue:** Direct RabbitMQ publishing via amqp091-go
- **Auth:** HTTP client to auth-service (only external HTTP dependency)

---

*Migrated from legacy_project/reference-materials/development/*
