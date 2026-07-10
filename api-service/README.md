# API Service

Consolidated Go API service that provides Profile CRUD and task submission with direct access to PostgreSQL, Redis, and RabbitMQ.

## Features
- Profile CRUD API
- Direct PostgreSQL access (no internal HTTP calls)
- Redis cache-aside for profiles
- RabbitMQ task publishing
- Auth integration via external auth-service

## Endpoints
```
GET    /health
GET    /ready
GET    /metrics

GET    /api/v1/profiles
POST   /api/v1/profiles
GET    /api/v1/profiles/:id
PUT    /api/v1/profiles/:id
DELETE /api/v1/profiles/:id

POST   /api/v1/profiles/:id/tasks
POST   /api/v1/profiles/:id/tasks/email
POST   /api/v1/profiles/:id/tasks/image
POST   /api/v1/profiles/:id/tasks/profile
```

## Configuration
Environment variables (prefix `API_`):
- `API_POSTGRES_DSN`
- `API_REDIS_HOST`
- `API_REDIS_PORT`
- `API_RABBITMQ_PASSWORD`
- `API_AUTH_URL`

## Local Run
```
make run
```

