# Plan: Auth Service & Deployment Structure

## Executive Summary

This document plans the creation of:
1. **`auth-service/`** - New auth service at repository root (based on legacy TypeScript code)
2. **`deployment/`** - New Kubernetes deployment folder (based on legacy k8s structure)

Both will be adapted to work with the new architecture components:
- `api-service` (Go - consolidated API gateway)
- `graph-worker/` (Python + Go workers)
- MongoDB Atlas, MinIO (Kubernetes), RabbitMQ, Redis, PostgreSQL

---

## 📂 Source References

| Component | Legacy Location | New Location |
|-----------|-----------------|--------------|
| Auth Service | `legacy_project/services/auth-service/` | `auth-service/` |
| K8s Deployment | `legacy_project/k8s/` | `deployment/` |

---

## Part 1: Auth Service (`auth-service/`)

### 1.1 Overview

The legacy auth-service is a **TypeScript/Node.js** service with:
- Express web server
- JWT-based authentication (access + refresh tokens)
- PostgreSQL database (users, audit logs)
- Rate limiting, account lockout
- Health checks, Prometheus metrics

**What changes for the new architecture:**
- Integration with `api-service` (Go) for token validation
- No changes to core authentication logic (well-tested, secure)
- Environment variables updated for new service names
- Kubernetes manifests moved to central `deployment/` folder

### 1.2 What to Copy (As-Is)

These components are production-ready and don't need modification:

| Source | Destination | Notes |
|--------|-------------|-------|
| `src/domain/` | `auth-service/src/domain/` | Core business logic |
| `src/infrastructure/` | `auth-service/src/infrastructure/` | Database, logging |
| `src/schemas/` | `auth-service/src/schemas/` | Zod validation |
| `src/types/` | `auth-service/src/types/` | TypeScript types |
| `src/utils/` | `auth-service/src/utils/` | Error classes |
| `src/api/middleware/` | `auth-service/src/api/middleware/` | Express middleware |
| `src/api/controllers/` | `auth-service/src/api/controllers/` | Request handlers |
| `src/api/routes/` | `auth-service/src/api/routes/` | Route definitions |
| `src/api/docs/` | `auth-service/src/api/docs/` | OpenAPI docs |
| `src/config/` | `auth-service/src/config/` | Configuration |
| `src/app.ts` | `auth-service/src/app.ts` | Express app setup |
| `src/server.ts` | `auth-service/src/server.ts` | Entry point |
| `migrations/` | `auth-service/migrations/` | Database migrations |
| `package.json` | `auth-service/package.json` | Dependencies |
| `tsconfig.json` | `auth-service/tsconfig.json` | TypeScript config |
| `Dockerfile` | `auth-service/Dockerfile` | Docker build |

### 1.3 What to Modify

#### 1.3.1 Environment Variables (`src/config/env.ts`)

Update service URLs for new architecture:

```typescript
// OLD (legacy inter-service HTTP calls - not actively used)
STORAGE_SERVICE_URL: z.string().url().optional(),
CACHE_SERVICE_URL: z.string().url().optional(),

// NEW (for potential future integration)
API_SERVICE_URL: z.string().url().optional().default('http://api-service:8080'),
```

**Note:** The legacy auth-service already uses direct PostgreSQL access, so no HTTP client changes needed.

#### 1.3.2 Service Discovery Configuration

Update default hostnames:

```typescript
// Database (same pattern, just confirm naming)
DATABASE_HOST: z.string().default('auth-postgres'),  // Keep dedicated DB
DATABASE_NAME: z.string().default('auth_db'),
DATABASE_USER: z.string().default('auth_user'),
```

#### 1.3.3 README.md

Update documentation to reflect:
- New architecture context
- Integration points with api-service
- Updated deployment instructions

### 1.4 Integration Points with api-service

The `api-service` needs to validate JWT tokens with auth-service. This is **already implemented** in:
- `api-service/internal/infrastructure/auth/client.go` - HTTP client with circuit breaker
- `api-service/internal/api/middleware/auth.go` - Auth middleware

**Auth-service endpoint used by api-service:**
```
POST /v1/auth/token/validate
Authorization: Bearer <token>
OR
Body: { "token": "<token>" }

Response: { "valid": true, "user": { "id", "email", "role" } }
```

No changes needed to auth-service - the endpoint already exists.

### 1.5 File Structure (New auth-service/)

```
auth-service/
├── src/
│   ├── api/
│   │   ├── controllers/
│   │   │   ├── AuthController.ts
│   │   │   ├── HealthController.ts
│   │   │   └── UserController.ts
│   │   ├── docs/
│   │   │   └── openapi.yaml
│   │   ├── middleware/
│   │   │   ├── auth.middleware.ts
│   │   │   ├── error.middleware.ts
│   │   │   ├── rateLimit.middleware.ts
│   │   │   ├── requestId.middleware.ts
│   │   │   └── validation.middleware.ts
│   │   └── routes/
│   │       ├── auth.routes.ts
│   │       ├── health.routes.ts
│   │       ├── user.routes.ts
│   │       └── index.ts
│   ├── config/
│   │   ├── env.ts
│   │   └── index.ts
│   ├── domain/
│   │   ├── entities/
│   │   │   └── User.ts
│   │   ├── repositories/
│   │   │   ├── IUserRepository.ts
│   │   │   └── UserRepository.ts
│   │   └── services/
│   │       ├── AuthService.ts
│   │       ├── TokenService.ts
│   │       └── UserService.ts
│   ├── infrastructure/
│   │   ├── database/
│   │   │   ├── connection.ts
│   │   │   └── migrations.ts
│   │   └── logging/
│   │       └── logger.ts
│   ├── schemas/
│   │   └── *.ts
│   ├── types/
│   │   └── *.ts
│   ├── utils/
│   │   └── errors.ts
│   ├── app.ts
│   └── server.ts
├── migrations/
│   └── 001_create_users_table.sql
├── tests/
│   └── *.test.ts
├── package.json
├── package-lock.json
├── tsconfig.json
├── tsconfig.build.json
├── Dockerfile
├── .dockerignore
├── .nvmrc
├── eslint.config.js
├── vitest.config.ts
└── README.md
```

### 1.6 API Endpoints (No Changes)

The auth-service exposes these endpoints (unchanged):

**Authentication:**
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/auth/login` | POST | User login, returns tokens |
| `/v1/auth/token/validate` | POST | Validate JWT (used by api-service) |
| `/v1/auth/token/refresh` | POST | Refresh access token |
| `/v1/auth/logout` | POST | Logout/invalidate token |

**User Management (Admin):**
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/users/me` | GET | Get current user profile |
| `/v1/users` | GET/POST | List/Create users |
| `/v1/users/:id` | GET/PUT/DELETE | User CRUD |
| `/v1/users/:id/activate` | PATCH | Activate user |
| `/v1/users/:id/deactivate` | PATCH | Deactivate user |
| `/v1/users/:id/role` | PATCH | Change user role |

**Health & Monitoring:**
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check |
| `/ready` | GET | Readiness probe |
| `/live` | GET | Liveness probe |
| `/metrics` | GET | Prometheus metrics |
| `/api-docs` | GET | Swagger UI (dev) |

---

## Part 2: Deployment Structure (`deployment/`)

### 2.1 Overview

Create a new `deployment/` folder at repository root with:
- Kubernetes manifests for all services
- Infrastructure components
- Scripts for automation
- Kind cluster configuration

### 2.2 New Architecture Services

| Service | Language | Type | Notes |
|---------|----------|------|-------|
| `api-service` | Go | Deployment | New consolidated API gateway |
| `auth-service` | Node.js | Deployment | Authentication (from legacy) |
| `graphrag-service` | Python | Deployment | Knowledge graph worker |
| `email-worker` | Go | Deployment | Email task worker |
| `image-worker` | Go | Deployment | Image task worker |
| `profile-worker` | Go | Deployment | Profile task worker |

### 2.3 Infrastructure Components

| Component | Type | Notes |
|-----------|------|-------|
| PostgreSQL (api) | StatefulSet | Profiles, documents metadata |
| PostgreSQL (auth) | StatefulSet | Users, audit logs |
| Redis | StatefulSet | Caching for api-service |
| RabbitMQ | StatefulSet | Message broker |
| MinIO | StatefulSet | Document storage (S3-compatible) |
| MongoDB | External | Atlas (managed) - for GraphRAG |

### 2.4 File Structure (New deployment/)

```
deployment/
│
├── README.md                              # Deployment documentation
│
├── cluster/                               # Cluster setup
│   ├── kind-config.yaml                  # Kind cluster config
│   ├── kind-multinode.yaml               # Multi-node Kind config
│   ├── setup-cluster.sh                  # Cluster setup automation
│   │
│   ├── infrastructure/                   # Core infrastructure
│   │   ├── ingress-nginx.yaml           # NGINX Ingress Controller
│   │   ├── metrics-server.yaml          # Metrics Server
│   │   ├── network-policies.yaml        # Global network policies
│   │   └── storage-class.yaml           # Storage provisioner
│   │
│   └── validation/                       # Validation scripts
│       ├── test-cluster.sh
│       └── verify-infrastructure.sh
│
├── services/                              # Service deployments
│   │
│   ├── 01-infrastructure/                # Infrastructure services
│   │   ├── postgres-api/                 # PostgreSQL for api-service
│   │   │   ├── statefulset.yaml
│   │   │   ├── service.yaml
│   │   │   ├── configmap.yaml
│   │   │   └── secret.yaml
│   │   │
│   │   ├── postgres-auth/                # PostgreSQL for auth-service
│   │   │   ├── statefulset.yaml
│   │   │   ├── service.yaml
│   │   │   ├── configmap.yaml
│   │   │   └── secret.yaml
│   │   │
│   │   ├── redis/                        # Redis for caching
│   │   │   ├── statefulset.yaml
│   │   │   ├── service.yaml
│   │   │   ├── configmap.yaml
│   │   │   └── secret.yaml
│   │   │
│   │   ├── rabbitmq/                     # RabbitMQ message broker
│   │   │   ├── statefulset.yaml
│   │   │   ├── service.yaml
│   │   │   ├── configmap.yaml
│   │   │   └── secret.yaml
│   │   │
│   │   └── minio/                        # MinIO object storage
│   │       ├── statefulset.yaml
│   │       ├── service.yaml
│   │       ├── configmap.yaml
│   │       └── secret.yaml
│   │
│   ├── 02-auth-service/                  # Auth service (Node.js)
│   │   ├── deployment.yaml
│   │   ├── service.yaml
│   │   ├── configmap.yaml
│   │   ├── secret.yaml
│   │   ├── network-policy.yaml
│   │   └── hpa.yaml
│   │
│   ├── 03-api-service/                   # API service (Go)
│   │   ├── deployment.yaml
│   │   ├── service.yaml
│   │   ├── configmap.yaml
│   │   ├── secret.yaml
│   │   ├── network-policy.yaml
│   │   └── hpa.yaml
│   │
│   ├── 04-graphrag-service/              # GraphRAG worker (Python)
│   │   ├── deployment.yaml
│   │   ├── service.yaml
│   │   ├── configmap.yaml
│   │   ├── secret.yaml
│   │   ├── network-policy.yaml
│   │   └── hpa.yaml
│   │
│   └── 05-operational-workers/           # Go workers
│       ├── email-worker/
│       │   ├── deployment.yaml
│       │   ├── service.yaml
│       │   ├── configmap.yaml
│       │   └── secret.yaml
│       ├── image-worker/
│       │   ├── deployment.yaml
│       │   ├── service.yaml
│       │   ├── configmap.yaml
│       │   └── secret.yaml
│       └── profile-worker/
│           ├── deployment.yaml
│           ├── service.yaml
│           ├── configmap.yaml
│           └── secret.yaml
│
├── external/                              # External service configs
│   └── mongodb-atlas/
│       ├── README.md                     # Atlas setup instructions
│       └── connection-secret.yaml        # Connection string secret
│
├── ingress/                               # Ingress configuration
│   ├── ingress.yaml                      # Main ingress rules
│   └── tls-secret.yaml                   # TLS certificates (dev)
│
├── monitoring/                            # Monitoring setup
│   ├── prometheus/
│   │   └── service-monitors.yaml
│   └── grafana/
│       └── dashboards/
│
└── scripts/                               # Automation scripts
    ├── common-functions.sh               # Shared utilities
    ├── deploy-all.sh                     # Deploy all services
    ├── build-all-images.sh               # Build Docker images
    ├── load-images-to-kind.sh            # Load images to Kind
    ├── setup-infrastructure.sh           # Setup infra only
    ├── teardown.sh                       # Clean teardown
    └── test/
        ├── test-auth.sh
        ├── test-api.sh
        └── integration-tests.sh
```

### 2.5 Service Port Assignments

| Service | Container Port | NodePort | Notes |
|---------|---------------|----------|-------|
| api-service | 8080 | 30080 | Main gateway |
| auth-service | 8080 | 30081 | Authentication |
| graphrag-service | 8080/8081 | 30082/30083 | Health/Metrics |
| email-worker | 8080 | 30084 | Health only |
| image-worker | 8080 | 30085 | Health only |
| profile-worker | 8080 | 30086 | Health only |
| PostgreSQL (api) | 5432 | - | Internal only |
| PostgreSQL (auth) | 5432 | - | Internal only |
| Redis | 6379 | - | Internal only |
| RabbitMQ | 5672/15672 | 30090/30091 | AMQP/Management |
| MinIO | 9000/9001 | 30092/30093 | API/Console |

### 2.6 Key Kubernetes Manifests

#### 2.6.1 api-service Deployment (New)

```yaml
# deployment/services/03-api-service/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api-service
  labels:
    app: api-service
spec:
  replicas: 2
  selector:
    matchLabels:
      app: api-service
  template:
    metadata:
      labels:
        app: api-service
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8080"
        prometheus.io/path: "/metrics"
    spec:
      containers:
      - name: api-service
        image: api-service:latest
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: 8080
          name: http
        env:
        - name: SERVER_PORT
          value: "8080"
        - name: POSTGRES_HOST
          value: "postgres-api"
        - name: POSTGRES_PORT
          value: "5432"
        - name: POSTGRES_DATABASE
          valueFrom:
            configMapKeyRef:
              name: api-service-config
              key: POSTGRES_DATABASE
        - name: POSTGRES_USER
          valueFrom:
            secretKeyRef:
              name: api-service-secret
              key: POSTGRES_USER
        - name: POSTGRES_PASSWORD
          valueFrom:
            secretKeyRef:
              name: api-service-secret
              key: POSTGRES_PASSWORD
        - name: REDIS_HOST
          value: "redis"
        - name: REDIS_PORT
          value: "6379"
        - name: RABBITMQ_HOST
          value: "rabbitmq"
        - name: RABBITMQ_PORT
          value: "5672"
        - name: RABBITMQ_USER
          valueFrom:
            secretKeyRef:
              name: rabbitmq-secret
              key: RABBITMQ_USER
        - name: RABBITMQ_PASSWORD
          valueFrom:
            secretKeyRef:
              name: rabbitmq-secret
              key: RABBITMQ_PASSWORD
        - name: AUTH_SERVICE_URL
          value: "http://auth-service:8080"
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 512Mi
        livenessProbe:
          httpGet:
            path: /health
            port: http
          initialDelaySeconds: 10
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: http
          initialDelaySeconds: 5
          periodSeconds: 5
      securityContext:
        runAsNonRoot: true
        runAsUser: 65534
```

#### 2.6.2 auth-service Deployment (Adapted from Legacy)

```yaml
# deployment/services/02-auth-service/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: auth-service
  labels:
    app: auth-service
spec:
  replicas: 2
  selector:
    matchLabels:
      app: auth-service
  template:
    metadata:
      labels:
        app: auth-service
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8080"
        prometheus.io/path: "/metrics"
    spec:
      containers:
      - name: auth-service
        image: auth-service:latest
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: 8080
          name: http
        env:
        - name: NODE_ENV
          value: "production"
        - name: PORT
          value: "8080"
        - name: DATABASE_HOST
          value: "postgres-auth"
        - name: DATABASE_PORT
          value: "5432"
        - name: DATABASE_NAME
          value: "auth_db"
        - name: DATABASE_USER
          valueFrom:
            secretKeyRef:
              name: auth-service-secret
              key: DATABASE_USER
        - name: DATABASE_PASSWORD
          valueFrom:
            secretKeyRef:
              name: auth-service-secret
              key: DATABASE_PASSWORD
        - name: JWT_SECRET
          valueFrom:
            secretKeyRef:
              name: auth-service-secret
              key: JWT_SECRET
        - name: JWT_ACCESS_TOKEN_EXPIRY
          value: "15m"
        - name: JWT_REFRESH_TOKEN_EXPIRY
          value: "7d"
        - name: RATE_LIMIT_WINDOW_MS
          value: "900000"
        - name: RATE_LIMIT_MAX_REQUESTS
          value: "100"
        - name: LOG_LEVEL
          value: "info"
        - name: METRICS_ENABLED
          value: "true"
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 300m
            memory: 384Mi
        livenessProbe:
          httpGet:
            path: /live
            port: http
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: http
          initialDelaySeconds: 10
          periodSeconds: 5
        startupProbe:
          httpGet:
            path: /health
            port: http
          initialDelaySeconds: 5
          periodSeconds: 5
          failureThreshold: 15
      securityContext:
        runAsNonRoot: true
        runAsUser: 1001
```

#### 2.6.3 graphrag-service Deployment (New)

```yaml
# deployment/services/04-graphrag-service/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: graphrag-service
  labels:
    app: graphrag-service
spec:
  replicas: 1  # Memory-intensive, scale carefully
  selector:
    matchLabels:
      app: graphrag-service
  template:
    metadata:
      labels:
        app: graphrag-service
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8081"
        prometheus.io/path: "/metrics"
    spec:
      containers:
      - name: graphrag-service
        image: graphrag-service:latest
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: 8080
          name: health
        - containerPort: 8081
          name: metrics
        env:
        - name: RABBITMQ_HOST
          value: "rabbitmq"
        - name: RABBITMQ_PORT
          value: "5672"
        - name: RABBITMQ_USER
          valueFrom:
            secretKeyRef:
              name: rabbitmq-secret
              key: RABBITMQ_USER
        - name: RABBITMQ_PASSWORD
          valueFrom:
            secretKeyRef:
              name: rabbitmq-secret
              key: RABBITMQ_PASSWORD
        - name: MONGODB_URI
          valueFrom:
            secretKeyRef:
              name: mongodb-atlas-secret
              key: MONGODB_URI
        - name: MONGODB_DATABASE
          value: "graphrag"
        - name: OPENAI_API_KEY
          valueFrom:
            secretKeyRef:
              name: graphrag-secret
              key: OPENAI_API_KEY
        - name: MINIO_ENDPOINT
          value: "minio:9000"
        - name: MINIO_ACCESS_KEY
          valueFrom:
            secretKeyRef:
              name: minio-secret
              key: MINIO_ACCESS_KEY
        - name: MINIO_SECRET_KEY
          valueFrom:
            secretKeyRef:
              name: minio-secret
              key: MINIO_SECRET_KEY
        resources:
          requests:
            cpu: 2000m
            memory: 6Gi
          limits:
            cpu: 4000m
            memory: 10Gi
        livenessProbe:
          httpGet:
            path: /health
            port: health
          initialDelaySeconds: 60
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /ready
            port: health
          initialDelaySeconds: 60
          periodSeconds: 30
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
```

### 2.7 What to Copy from Legacy k8s

| Legacy File | New Location | Changes |
|-------------|--------------|---------|
| `cluster/kind-config.yaml` | `deployment/cluster/kind-config.yaml` | Update port mappings |
| `cluster/infrastructure/*` | `deployment/cluster/infrastructure/*` | Keep as-is |
| `cluster/setup-cluster.sh` | `deployment/cluster/setup-cluster.sh` | Update for new services |
| `scripts/common-functions.sh` | `deployment/scripts/common-functions.sh` | Keep as-is |
| `deployment/03-auth-service/*` | `deployment/services/02-auth-service/*` | Adapt env vars |

### 2.8 What to Create New

| File | Purpose |
|------|---------|
| `services/01-infrastructure/minio/*` | MinIO object storage |
| `services/03-api-service/*` | Go API gateway |
| `services/04-graphrag-service/*` | Python GraphRAG worker |
| `services/05-operational-workers/*` | Go task workers |
| `external/mongodb-atlas/*` | MongoDB Atlas configuration |
| `scripts/deploy-all.sh` | Orchestrated deployment |

---

## Part 3: Implementation Order

### Phase 1: Auth Service (Day 1-2)

1. Create `auth-service/` directory
2. Copy TypeScript code from legacy
3. Update `src/config/env.ts` with new defaults
4. Update `README.md` with new context
5. Test locally with Docker Compose

### Phase 2: Deployment Structure (Day 2-3)

1. Create `deployment/` directory structure
2. Copy cluster setup from legacy k8s
3. Create infrastructure manifests (PostgreSQL, Redis, RabbitMQ, MinIO)
4. Create auth-service manifests
5. Create api-service manifests

### Phase 3: Worker Deployments (Day 3-4)

1. Create graphrag-service manifests
2. Create operational-workers manifests
3. Create MongoDB Atlas configuration docs

### Phase 4: Scripts & Automation (Day 4-5)

1. Adapt deployment scripts
2. Create test scripts
3. Update Kind configuration
4. Create docker-compose.yaml for local dev

### Phase 5: Validation (Day 5)

1. Test full deployment in Kind cluster
2. Verify service communication
3. Test auth flow end-to-end
4. Document any issues/fixes

---

## Part 4: Environment Variables Summary

### auth-service Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `NODE_ENV` | development | Environment |
| `PORT` | 8080 | Server port |
| `DATABASE_HOST` | postgres-auth | PostgreSQL host |
| `DATABASE_PORT` | 5432 | PostgreSQL port |
| `DATABASE_NAME` | auth_db | Database name |
| `DATABASE_USER` | auth_user | Database user |
| `DATABASE_PASSWORD` | (required) | Database password |
| `JWT_SECRET` | (required) | JWT signing secret (min 32 chars) |
| `JWT_ACCESS_TOKEN_EXPIRY` | 15m | Access token lifetime |
| `JWT_REFRESH_TOKEN_EXPIRY` | 7d | Refresh token lifetime |
| `RATE_LIMIT_WINDOW_MS` | 900000 | Rate limit window (15 min) |
| `RATE_LIMIT_MAX_REQUESTS` | 100 | Max requests per window |
| `LOG_LEVEL` | info | Log level |
| `METRICS_ENABLED` | true | Enable Prometheus metrics |

### api-service Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVER_PORT` | 8080 | Server port |
| `POSTGRES_HOST` | postgres-api | PostgreSQL host |
| `POSTGRES_PORT` | 5432 | PostgreSQL port |
| `POSTGRES_DATABASE` | api_db | Database name |
| `POSTGRES_USER` | api_user | Database user |
| `POSTGRES_PASSWORD` | (required) | Database password |
| `REDIS_HOST` | redis | Redis host |
| `REDIS_PORT` | 6379 | Redis port |
| `RABBITMQ_HOST` | rabbitmq | RabbitMQ host |
| `RABBITMQ_PORT` | 5672 | RabbitMQ port |
| `RABBITMQ_USER` | guest | RabbitMQ user |
| `RABBITMQ_PASSWORD` | (required) | RabbitMQ password |
| `AUTH_SERVICE_URL` | http://auth-service:8080 | Auth service URL |

### graphrag-service Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `RABBITMQ_HOST` | rabbitmq | RabbitMQ host |
| `RABBITMQ_PORT` | 5672 | RabbitMQ port |
| `RABBITMQ_USER` | guest | RabbitMQ user |
| `RABBITMQ_PASSWORD` | (required) | RabbitMQ password |
| `MONGODB_URI` | (required) | MongoDB Atlas connection string |
| `MONGODB_DATABASE` | graphrag | MongoDB database name |
| `OPENAI_API_KEY` | (required) | OpenAI API key |
| `MINIO_ENDPOINT` | minio:9000 | MinIO endpoint |
| `MINIO_ACCESS_KEY` | (required) | MinIO access key |
| `MINIO_SECRET_KEY` | (required) | MinIO secret key |

---

## Part 5: Network Policies

### Service Communication Matrix

| From | To | Allowed |
|------|-----|---------|
| api-service | auth-service | ✅ HTTP (token validation) |
| api-service | postgres-api | ✅ PostgreSQL |
| api-service | redis | ✅ Redis |
| api-service | rabbitmq | ✅ AMQP (publish) |
| api-service | minio | ✅ S3 API (document upload) |
| auth-service | postgres-auth | ✅ PostgreSQL |
| graphrag-service | rabbitmq | ✅ AMQP (consume) |
| graphrag-service | mongodb-atlas | ✅ MongoDB (external) |
| graphrag-service | minio | ✅ S3 API (document download) |
| operational-workers | rabbitmq | ✅ AMQP (consume) |
| ingress | api-service | ✅ HTTP (gateway) |
| ingress | auth-service | ❌ (via api-service only) |

---

## Summary

### New Folders to Create

1. **`auth-service/`** - TypeScript authentication service
   - Copy from `legacy_project/services/auth-service/`
   - Minimal changes (env defaults, README)
   - Production-ready code

2. **`deployment/`** - Kubernetes deployment structure
   - Based on `legacy_project/k8s/`
   - Reorganized for new architecture
   - New manifests for api-service, graphrag-service, workers

### Key Decisions Confirmed

| Decision | Choice |
|----------|--------|
| Auth service language | TypeScript/Node.js (keep legacy) |
| Auth database | Dedicated PostgreSQL (keep isolated) |
| Deployment structure | Numbered folders for ordering |
| Infrastructure | StatefulSets for databases |
| External MongoDB | Atlas (managed) |
| Document storage | MinIO in Kubernetes |

### Ready to Implement

After review, we can:
1. Create `auth-service/` by copying and adapting legacy code
2. Create `deployment/` structure with all manifests
3. Test in Kind cluster

---

*Document Version: 1.0*
*Created: January 2026*
*Status: Ready for Review*
