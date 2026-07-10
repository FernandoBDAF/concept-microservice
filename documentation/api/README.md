# API Documentation

API specifications and reference for the Profile Service.

## Contents

- [API Reference](reference.md) - Complete endpoint documentation
- [Authentication](authentication.md) - JWT authentication flow
- [Error Handling](errors.md) - Error responses and codes
- [Examples](examples.md) - Request/response examples

## Base URL

```
Development: http://localhost:8080
Production:  https://api.example.com
```

## Authentication

All `/api/v1/*` endpoints require JWT authentication:

```bash
curl -H "Authorization: Bearer <token>" \
     http://localhost:8080/api/v1/profiles
```

## Endpoints Overview

### Health & Monitoring

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Basic health check |
| GET | `/ready` | Readiness (DB + Redis + RabbitMQ) |
| GET | `/metrics` | Prometheus metrics |

### Profile CRUD

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/v1/profiles` | List profiles (paginated) |
| POST | `/api/v1/profiles` | Create profile |
| GET | `/api/v1/profiles/:id` | Get profile by ID |
| PUT | `/api/v1/profiles/:id` | Update profile |
| DELETE | `/api/v1/profiles/:id` | Delete profile |

### Task Submission

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/profiles/:id/tasks/email` | Submit email task |
| POST | `/api/v1/profiles/:id/tasks/image` | Submit image task |
| POST | `/api/v1/profiles/:id/tasks/profile` | Submit profile task |

## Response Format

### Success Response

```json
{
  "data": { ... },
  "meta": {
    "page": 1,
    "limit": 20,
    "total": 100
  }
}
```

### Error Response

```json
{
  "error": {
    "code": "NOT_FOUND",
    "message": "Profile not found",
    "details": { ... }
  }
}
```

## See Also

- [api-service/README.md](../../api-service/README.md) - Service implementation
- [CONSOLIDATED_SERVICE_PLAN.md](../../CONSOLIDATED_SERVICE_PLAN.md) - API design decisions
