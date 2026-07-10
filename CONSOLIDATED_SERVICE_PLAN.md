# Consolidated Go Service - Architecture Planning Document

## Executive Summary

This document outlines the plan to consolidate the **profile-service** API with direct access to **PostgreSQL**, **Redis**, and **RabbitMQ** - eliminating the HTTP calls to cache-service, queue-service, and storage-service. The result is a single, efficient Go service that:

1. **Exposes a Profile CRUD API** (extensible for future endpoints)
2. **Directly connects to infrastructure** (no intermediate HTTP services)
3. **Publishes messages to RabbitMQ** for external workers to consume
4. **Integrates with auth-service** via Kubernetes for authentication

---

## 🔄 Current vs. Target Architecture

### Current Architecture (HTTP-Based Microservices)

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Profile Service (API)                        │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │  CacheClient    │  StorageClient   │  QueueClient           │   │
│  │  (HTTP Client)  │  (HTTP Client)   │  (HTTP Client)         │   │
│  └────────┬────────┴────────┬─────────┴──────────┬─────────────┘   │
└───────────┼─────────────────┼────────────────────┼─────────────────┘
            │ HTTP            │ HTTP               │ HTTP
            ▼                 ▼                    ▼
     ┌─────────────┐   ┌─────────────┐   ┌─────────────┐
     │cache-service│   │storage-svc  │   │queue-service│
     │   (Gin)     │   │ (Gorilla)   │   │   (Gin)     │
     └──────┬──────┘   └──────┬──────┘   └──────┬──────┘
            │                 │                  │
            ▼                 ▼                  ▼
         ┌─────┐         ┌──────────┐      ┌──────────┐
         │Redis│         │PostgreSQL│      │ RabbitMQ │
         └─────┘         └──────────┘      └──────────┘
```

**Problems with current approach:**
- ❌ Network latency on every cache/storage/queue operation
- ❌ HTTP serialization/deserialization overhead
- ❌ Circuit breakers and retries add complexity
- ❌ 4 separate deployments to manage
- ❌ Debugging distributed calls is difficult

### Target Architecture (Direct Infrastructure Access)

```
┌─────────────────────────────────────────────────────────────────────┐
│                   Consolidated Service (API)                        │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │   Profile Handler → Profile Service                         │   │
│  │        ↓                   ↓                                │   │
│  │   ┌─────────┐    ┌──────────────┐    ┌─────────────┐       │   │
│  │   │  Redis  │    │  PostgreSQL  │    │  RabbitMQ   │       │   │
│  │   │ Client  │    │    Client    │    │  Publisher  │       │   │
│  │   │(go-redis)│   │ (sqlx+lib/pq)│    │ (amqp091-go)│       │   │
│  │   └────┬────┘    └──────┬───────┘    └──────┬──────┘       │   │
│  └────────┼────────────────┼───────────────────┼──────────────┘   │
└───────────┼────────────────┼───────────────────┼──────────────────┘
            │                │                   │
            ▼                ▼                   ▼
         ┌─────┐        ┌──────────┐       ┌──────────┐
         │Redis│        │PostgreSQL│       │ RabbitMQ │──▶ Workers
         └─────┘        └──────────┘       └──────────┘   (Future)
```

**Benefits:**
- ✅ **~10x faster** - Direct connections, no HTTP overhead
- ✅ **Single deployment** - One container to manage
- ✅ **Simpler debugging** - All logic in one place
- ✅ **Atomic operations** - Can use DB transactions properly
- ✅ **Less code** - No HTTP clients, circuit breakers between services

---

## 📊 What We're Eliminating

### Current HTTP Clients to Remove

| Client | File | Calls To | Replacement |
|--------|------|----------|-------------|
| `CacheClient` | `profile-service/internal/infrastructure/cache/cache_client.go` | cache-service HTTP | Direct `go-redis` client |
| `StorageClient` | `profile-service/internal/domain/services/storage.go` | storage-service HTTP | Direct `sqlx` + repository |
| `QueueClient` | `profile-service/internal/pkg/messaging/queue_client.go` | queue-service HTTP | Direct `amqp091-go` publisher |

### Services Being Merged

| Service | What We Take | What We Skip |
|---------|--------------|--------------|
| **cache-service** | Redis client logic, cache patterns | HTTP server, gRPC stubs |
| **storage-service** | PostgreSQL repository, migrations | HTTP server, message consumer |
| **queue-service** | RabbitMQ publisher logic, routing config | HTTP server, consumer logic |
| **common** | Logger, metrics, errors, config utilities | Everything (it's already reusable) |
| **profile-service** | API handlers, domain models, business logic | HTTP clients to other services |

---

## 🏗️ Recommended Project Structure

```
api-service/
├── cmd/
│   └── server/
│       └── main.go                     # Application entry point
│
├── internal/
│   │
│   ├── api/                            # HTTP Layer (Gin)
│   │   ├── handlers/
│   │   │   ├── profile.go              # Profile CRUD handlers
│   │   │   ├── task.go                 # Task submission handlers
│   │   │   └── health.go               # Health/readiness checks
│   │   ├── middleware/
│   │   │   ├── auth.go                 # JWT validation (calls auth-service)
│   │   │   ├── logging.go              # Request logging
│   │   │   └── metrics.go              # Prometheus metrics
│   │   └── router.go                   # Route registration
│   │
│   ├── domain/                         # Business Logic Layer
│   │   ├── profile/
│   │   │   ├── model.go                # Profile entity
│   │   │   ├── service.go              # Profile business logic
│   │   │   └── repository.go           # Repository interface
│   │   └── task/
│   │       ├── model.go                # Task/Message models
│   │       └── service.go              # Task orchestration
│   │
│   ├── infrastructure/                 # External Systems (Direct Access)
│   │   │
│   │   ├── postgres/                   # PostgreSQL (Direct)
│   │   │   ├── client.go               # Connection pool management
│   │   │   └── profile_repository.go   # Profile queries (sqlx)
│   │   │
│   │   ├── redis/                      # Redis (Direct)
│   │   │   ├── client.go               # Redis connection (go-redis)
│   │   │   └── cache.go                # Cache operations
│   │   │
│   │   ├── rabbitmq/                   # RabbitMQ (Direct)
│   │   │   ├── client.go               # AMQP connection
│   │   │   └── publisher.go            # Message publishing
│   │   │
│   │   └── auth/                       # Auth Service (HTTP - External)
│   │       └── client.go               # HTTP client to auth-service
│   │
│   ├── config/
│   │   └── config.go                   # Viper-based configuration
│   │
│   └── pkg/                            # Internal shared utilities
│       ├── logger/                     # Zap logger wrapper
│       ├── metrics/                    # Prometheus metrics
│       └── errors/                     # Custom error types
│
├── migrations/                         # SQL migrations
│   ├── 000001_create_profiles.up.sql
│   └── 000001_create_profiles.down.sql
│
├── deployments/
│   └── kubernetes/
│       ├── deployment.yaml
│       ├── service.yaml
│       ├── configmap.yaml
│       └── hpa.yaml
│
├── go.mod
├── go.sum
├── Dockerfile
├── Makefile
└── README.md
```

### Key Design Principles

1. **Clean Architecture** - Dependencies point inward (handlers → services → repositories)
2. **Direct Infrastructure Access** - No HTTP intermediaries to Redis/PostgreSQL/RabbitMQ
3. **Single External HTTP Dependency** - Only auth-service accessed via HTTP (different concern)
4. **Extensible API** - Easy to add new endpoints and domains
5. **Workers are External** - This service publishes; separate workers consume

---

## 🔧 Technical Stack (Finalized)

### Dependencies

| Component | Library | Source | Rationale |
|-----------|---------|--------|-----------|
| **HTTP Framework** | `gin-gonic/gin` | profile-service, cache-service | Best performance, most features |
| **PostgreSQL** | `jmoiron/sqlx` + `lib/pq` | storage-service | Battle-tested, good query builder |
| **Redis** | `go-redis/redis/v8` | cache-service | Full feature set, connection pooling |
| **RabbitMQ** | `rabbitmq/amqp091-go` | queue-service, common | Official Go client |
| **Config** | `spf13/viper` | cache-service | File + env + validation |
| **Logging** | `uber-go/zap` | All services | Structured, fast |
| **Metrics** | `prometheus/client_golang` | All services | Standard for K8s |
| **Resilience** | `sony/gobreaker` | cache-service | For auth-service calls only |
| **UUID** | `google/uuid` | All services | Standard UUID generation |
| **JWT** | `golang-jwt/jwt` | profile-service | Token parsing (validation via auth-service) |

### go.mod

```go
module github.com/fernandobarroso/microservices/api-service

go 1.22

require (
    github.com/gin-gonic/gin v1.10.1
    github.com/jmoiron/sqlx v1.4.0
    github.com/lib/pq v1.10.9
    github.com/go-redis/redis/v8 v8.11.5
    github.com/rabbitmq/amqp091-go v1.10.0
    github.com/spf13/viper v1.17.0
    go.uber.org/zap v1.27.0
    github.com/prometheus/client_golang v1.22.0
    github.com/google/uuid v1.6.0
    github.com/golang-jwt/jwt/v5 v5.2.0
    github.com/sony/gobreaker v1.0.0
)
```

---

## 📡 API Design

### Public Endpoints (Profile CRUD + Tasks)

```
# Health & Monitoring
GET  /health                              # Basic health (no auth)
GET  /ready                               # Readiness: DB + Redis + RabbitMQ
GET  /live                                # Liveness probe
GET  /metrics                             # Prometheus metrics

# Profile CRUD (requires authentication)
GET    /api/v1/profiles                   # List all profiles (paginated)
POST   /api/v1/profiles                   # Create profile
GET    /api/v1/profiles/:id               # Get profile by ID
PUT    /api/v1/profiles/:id               # Update profile
DELETE /api/v1/profiles/:id               # Delete profile

# Task Submission (publishes to RabbitMQ for workers)
POST   /api/v1/profiles/:id/tasks         # Submit generic task
POST   /api/v1/profiles/:id/tasks/email   # Submit email task → email.send
POST   /api/v1/profiles/:id/tasks/image   # Submit image task → image.process
POST   /api/v1/profiles/:id/tasks/profile # Submit profile task → profile.task
```

### Extensibility Pattern

New endpoints follow the same pattern:

```go
// Adding a new domain (e.g., Orders) in the future:
// 1. Create internal/domain/order/{model.go, service.go, repository.go}
// 2. Create internal/infrastructure/postgres/order_repository.go
// 3. Create internal/api/handlers/order.go
// 4. Register routes in router.go

// Example future endpoints:
// GET    /api/v1/orders
// POST   /api/v1/orders
// GET    /api/v1/orders/:id
// POST   /api/v1/orders/:id/tasks/fulfill  → Publishes to RabbitMQ
```

---

## 🔐 Authentication Flow

The API service validates JWT tokens by calling the **auth-service** (the only HTTP dependency):

```
┌────────┐     ┌─────────────────────────┐     ┌──────────────┐
│ Client │────▶│ API Service             │────▶│ Auth Service │
│        │     │                         │     │  (Node.js)   │
│        │     │ 1. Extract JWT from     │     │              │
│        │     │    Authorization header │     │              │
│        │     │ 2. Call auth-service    │────▶│ POST /v1/auth│
│        │     │    to validate          │     │ /token/validate
│        │     │ 3. Get user context     │◀────│              │
│        │     │ 4. Process request      │     │              │
└────────┘     └─────────────────────────┘     └──────────────┘
```

```go
// internal/infrastructure/auth/client.go
type AuthClient struct {
    baseURL        string
    httpClient     *http.Client
    circuitBreaker *gobreaker.CircuitBreaker  // Resilience for external service
}

func (c *AuthClient) ValidateToken(ctx context.Context, token string) (*User, error) {
    // Only HTTP client in the entire service - calls auth-service
    req, _ := http.NewRequestWithContext(ctx, "POST", 
        c.baseURL+"/v1/auth/token/validate", 
        bytes.NewReader([]byte(`{"token": "`+token+`"}`)))
    req.Header.Set("Content-Type", "application/json")
    
    resp, err := c.circuitBreaker.Execute(func() (interface{}, error) {
        return c.httpClient.Do(req)
    })
    // ... handle response
}
```

---

## 🗄️ Database Schema

### Profiles Table (from storage-service)

```sql
-- migrations/000001_create_profiles.up.sql
CREATE TABLE IF NOT EXISTS profiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    email VARCHAR(255) UNIQUE NOT NULL,
    phone VARCHAR(50),
    avatar_url TEXT,
    bio TEXT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_profiles_user_id ON profiles(user_id);
CREATE INDEX idx_profiles_email ON profiles(email);
CREATE INDEX idx_profiles_created_at ON profiles(created_at);
```

---

## 🐰 RabbitMQ Integration

### Publishing Pattern (for Workers)

The API service **publishes** messages to RabbitMQ. External workers **consume** them.

```go
// internal/infrastructure/rabbitmq/publisher.go
type Publisher struct {
    conn    *amqp.Connection
    channel *amqp.Channel
}

func (p *Publisher) PublishTask(ctx context.Context, task *TaskMessage) error {
    body, _ := json.Marshal(task)
    
    return p.channel.PublishWithContext(ctx,
        task.Exchange,      // e.g., "tasks-exchange"
        task.RoutingKey,    // e.g., "email.send", "image.process"
        false,              // mandatory
        false,              // immediate
        amqp.Publishing{
            ContentType:  "application/json",
            DeliveryMode: amqp.Persistent,
            Body:         body,
            Timestamp:    time.Now(),
            MessageId:    task.ID,
        },
    )
}
```

### Routing Keys for Workers

| Task Type | Routing Key | Exchange | Target Worker |
|-----------|-------------|----------|---------------|
| Email notification | `email.send` | `tasks-exchange` | email-worker |
| Image processing | `image.process` | `tasks-exchange` | image-worker |
| Profile task | `profile.task` | `tasks-exchange` | profile-worker |

Workers are **separate deployments** (to be implemented later) that consume from these queues.

---

## 🔴 Redis Caching Strategy

### Cache Patterns

```go
// internal/infrastructure/redis/cache.go
type Cache struct {
    client *redis.Client
    ttl    time.Duration
}

// Profile caching with cache-aside pattern
func (c *Cache) GetProfile(ctx context.Context, id string) (*Profile, error) {
    key := fmt.Sprintf("profile:%s", id)
    data, err := c.client.Get(ctx, key).Bytes()
    if err == redis.Nil {
        return nil, ErrCacheMiss  // Caller should fetch from DB
    }
    // ...unmarshal and return
}

func (c *Cache) SetProfile(ctx context.Context, profile *Profile) error {
    key := fmt.Sprintf("profile:%s", profile.ID)
    data, _ := json.Marshal(profile)
    return c.client.Set(ctx, key, data, c.ttl).Err()
}

func (c *Cache) InvalidateProfile(ctx context.Context, id string) error {
    return c.client.Del(ctx, fmt.Sprintf("profile:%s", id)).Err()
}
```

### Cache Keys

| Pattern | Example | TTL | Purpose |
|---------|---------|-----|---------|
| `profile:{id}` | `profile:123e4567-...` | 15 min | Profile data |
| `profiles:list:{page}` | `profiles:list:1` | 5 min | Paginated lists |

---

## 📊 Service Flow Example

### GET /api/v1/profiles/:id

```
1. Request arrives with JWT token
2. Auth middleware validates token via auth-service (HTTP)
3. Handler calls ProfileService.GetProfile(id)
4. ProfileService checks Redis cache
   - Cache HIT: Return cached profile
   - Cache MISS: Query PostgreSQL, cache result, return
5. Return JSON response
```

### POST /api/v1/profiles/:id/tasks/email

```
1. Request arrives with JWT token + task payload
2. Auth middleware validates token via auth-service (HTTP)
3. Handler validates request body
4. TaskService.SubmitEmailTask()
   - Builds message with routing_key="email.send"
   - Publishes to RabbitMQ
5. Return 202 Accepted with task ID
6. [ASYNC] email-worker consumes message and processes
```

---

## 🚀 Implementation Phases

### Phase 1: Foundation (Days 1-2)
- [ ] Create project structure
- [ ] Set up Viper configuration
- [ ] Initialize Zap logger
- [ ] Set up Prometheus metrics

### Phase 2: Infrastructure Clients (Days 3-4)
- [ ] PostgreSQL client with connection pooling (from storage-service)
- [ ] Redis client with circuit breaker (from cache-service)
- [ ] RabbitMQ publisher (from queue-service)
- [ ] Auth-service HTTP client

### Phase 3: Domain Layer (Days 5-6)
- [ ] Profile model and repository interface
- [ ] Profile service with caching logic
- [ ] Task service with RabbitMQ publishing

### Phase 4: API Layer (Days 7-8)
- [ ] Gin router setup
- [ ] Auth middleware
- [ ] Profile CRUD handlers
- [ ] Task submission handlers
- [ ] Health check endpoints

### Phase 5: Testing & Polish (Days 9-10)
- [ ] Unit tests for services
- [ ] Integration tests
- [ ] Kubernetes manifests
- [ ] Documentation

---

## ✅ Success Criteria

| Criterion | Metric |
|-----------|--------|
| **API Response Time** | p99 < 50ms (cached), < 100ms (uncached) |
| **Availability** | 99.9% uptime |
| **Single Deployment** | 1 Docker image, 1 K8s deployment |
| **Auth Integration** | JWT validation via auth-service works |
| **Task Publishing** | Messages appear in RabbitMQ queues |
| **Backward Compatible** | Existing profile-service API contracts preserved |

---

## 🤔 Decisions Summary

| Question | Decision |
|----------|----------|
| Expose cache API externally? | **No** - Internal only, cache is implementation detail |
| Expose queue API externally? | **No** - Tasks submitted via `/profiles/:id/tasks/*` |
| Include gRPC? | **No** - Start HTTP-only, add if needed |
| Workers in same service? | **No** - Workers are separate deployments |
| Session management? | **Simplified** - JWT-only via auth-service |

---

## 📋 Next Steps

Once you approve this plan:

1. **Confirm the project name** (suggested: `api-service`)
2. **Review the API endpoints** - Any changes needed?
3. **Confirm decisions** - Especially around what's internal vs exposed
4. I'll **create the project skeleton** and begin implementation

---

*Document Version: 2.0*  
*Updated: January 2026*  
*Focus: Consolidated API with direct infrastructure access*

