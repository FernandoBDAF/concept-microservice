# Auth Service - Implementation Plan

**Project:** auth-service  
**Language:** TypeScript/Node.js  
**Status:** 📋 Ready to copy from legacy  
**Session Focus:** Copy and adapt legacy auth-service to new architecture

---

## 1. Overview

This plan covers creating the `auth-service/` folder by copying the production-ready TypeScript authentication service from `legacy_project/services/auth-service/` with minimal modifications.

### What This Service Provides
- JWT-based authentication (access + refresh tokens)
- User management (CRUD, activation, roles)
- Rate limiting and account lockout
- PostgreSQL persistence (users, audit logs)
- Health checks and Prometheus metrics

### Why Minimal Changes
The legacy auth-service is well-tested and production-ready. The core authentication logic should not change - only configuration defaults need updating for the new architecture.

---

## 2. Source and Destination

| Component | Legacy Location | New Location |
|-----------|-----------------|--------------|
| Auth Service | `legacy_project/services/auth-service/` | `auth-service/` |

---

## 3. Implementation Tasks

### Phase 1: Copy Source Code

#### Task 1.1: Copy Core TypeScript Code

**Copy Commands:**
```bash
# From repository root
cp -r legacy_project/services/auth-service/src auth-service/
cp -r legacy_project/services/auth-service/migrations auth-service/
cp legacy_project/services/auth-service/package.json auth-service/
cp legacy_project/services/auth-service/package-lock.json auth-service/
cp legacy_project/services/auth-service/tsconfig.json auth-service/
cp legacy_project/services/auth-service/tsconfig.build.json auth-service/
cp legacy_project/services/auth-service/Dockerfile auth-service/
cp legacy_project/services/auth-service/.dockerignore auth-service/
cp legacy_project/services/auth-service/.nvmrc auth-service/
cp legacy_project/services/auth-service/.gitignore auth-service/
cp legacy_project/services/auth-service/eslint.config.js auth-service/
cp legacy_project/services/auth-service/vitest.config.ts auth-service/
```

#### Task 1.2: Remove Legacy Files Not Needed

The legacy auth-service has both old JavaScript and new TypeScript implementations. Keep only TypeScript:

```bash
# Remove old JavaScript implementation
rm -rf auth-service/src/service/       # Old JS version
rm -rf auth-service/src/routes/        # Old JS version (keep api/routes/)
rm -rf auth-service/src/controllers/   # Old JS version (keep api/controllers/)
rm -rf auth-service/src/models/        # Old JS version
rm -rf auth-service/src/repository/    # Old JS version (keep domain/repositories/)
rm -rf auth-service/src/middleware/    # Old JS version (keep api/middleware/)
rm -f auth-service/src/server.js       # Old JS entry
rm -f auth-service/src/config/config.js # Old JS config
rm -rf auth-service/deployments/       # Will be in central deployment/
```

---

### Phase 2: Update Configuration

#### Task 2.1: Update Environment Defaults

**File:** `auth-service/src/config/env.ts`

Update default hostnames for new architecture:

```typescript
// Database defaults for Kubernetes
DATABASE_HOST: z.string().default('postgres-auth'),
DATABASE_PORT: z.coerce.number().default(5432),
DATABASE_NAME: z.string().default('auth_db'),
DATABASE_USER: z.string().default('auth_user'),

// Add API service URL for potential future integration
API_SERVICE_URL: z.string().url().optional().default('http://api-service:8080'),

// Remove or comment out legacy service URLs if present
// STORAGE_SERVICE_URL: z.string().url().optional(),  // Not needed
// CACHE_SERVICE_URL: z.string().url().optional(),    // Not needed
```

#### Task 2.2: Verify JWT Configuration

Ensure JWT settings are properly configured:

```typescript
JWT_SECRET: z.string().min(32),  // Required, no default for security
JWT_ACCESS_TOKEN_EXPIRY: z.string().default('15m'),
JWT_REFRESH_TOKEN_EXPIRY: z.string().default('7d'),
```

---

### Phase 3: Update Documentation

#### Task 3.1: Create New README.md

**File:** `auth-service/README.md`

```markdown
# Auth Service

JWT-based authentication service for the microservices architecture.

## Features

- User authentication (login, logout, token refresh)
- JWT token management (access + refresh tokens)
- Token validation endpoint for api-service
- User management (CRUD, activation, roles)
- Rate limiting and account lockout
- Audit logging
- Health checks and Prometheus metrics

## Integration with api-service

The api-service validates tokens by calling:

```
POST /v1/auth/token/validate
Authorization: Bearer <token>

Response: { "valid": true, "user": { "id", "email", "role" } }
```

## API Endpoints

### Authentication
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/auth/login` | POST | User login, returns tokens |
| `/v1/auth/token/validate` | POST | Validate JWT (used by api-service) |
| `/v1/auth/token/refresh` | POST | Refresh access token |
| `/v1/auth/logout` | POST | Logout/invalidate token |

### User Management
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/users/me` | GET | Get current user profile |
| `/v1/users` | GET/POST | List/Create users |
| `/v1/users/:id` | GET/PUT/DELETE | User CRUD |

### Health & Monitoring
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check |
| `/ready` | GET | Readiness probe |
| `/live` | GET | Liveness probe |
| `/metrics` | GET | Prometheus metrics |

## Environment Variables

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `NODE_ENV` | development | No | Environment |
| `PORT` | 8080 | No | Server port |
| `DATABASE_HOST` | postgres-auth | No | PostgreSQL host |
| `DATABASE_PORT` | 5432 | No | PostgreSQL port |
| `DATABASE_NAME` | auth_db | No | Database name |
| `DATABASE_USER` | auth_user | No | Database user |
| `DATABASE_PASSWORD` | - | **Yes** | Database password |
| `JWT_SECRET` | - | **Yes** | JWT signing secret (min 32 chars) |
| `JWT_ACCESS_TOKEN_EXPIRY` | 15m | No | Access token lifetime |
| `JWT_REFRESH_TOKEN_EXPIRY` | 7d | No | Refresh token lifetime |

## Local Development

```bash
# Install dependencies
npm install

# Run migrations
npm run migrate

# Start development server
npm run dev

# Run tests
npm test

# Build for production
npm run build
npm start
```

## Docker

```bash
# Build image
docker build -t auth-service:latest .

# Run container
docker run -p 8080:8080 \
  -e DATABASE_HOST=host.docker.internal \
  -e DATABASE_PASSWORD=secret \
  -e JWT_SECRET=your-32-character-secret-here \
  auth-service:latest
```

## Database

Uses PostgreSQL with the following schema:

- `users` - User accounts
- `auth_audit_logs` - Authentication events

Migrations are in `migrations/` directory.
```

---

### Phase 4: Verify and Test

#### Task 4.1: Install Dependencies

```bash
cd auth-service
npm install
```

#### Task 4.2: Verify TypeScript Compilation

```bash
npm run build
```

#### Task 4.3: Run Tests

```bash
npm test
```

#### Task 4.4: Local Docker Test

```bash
# Build Docker image
docker build -t auth-service:latest .

# Test with docker-compose (create if needed)
docker-compose up -d postgres-auth
docker-compose up auth-service
```

---

## 4. File Structure (Final)

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
├── .gitignore
├── eslint.config.js
├── vitest.config.ts
├── IMPLEMENTATION_PLAN.md
└── README.md
```

---

## 5. Files to Remove After Copy

```bash
# Old JavaScript files (if present)
auth-service/src/service/
auth-service/src/routes/          # Old JS routes (keep api/routes/)
auth-service/src/controllers/     # Old JS controllers (keep api/controllers/)
auth-service/src/models/
auth-service/src/repository/
auth-service/src/middleware/      # Old JS middleware (keep api/middleware/)
auth-service/src/server.js
auth-service/src/config/config.js
auth-service/deployments/         # Kubernetes manifests go in deployment/
```

---

## 6. Integration Points

### With api-service
The api-service already has integration code:
- `api-service/internal/infrastructure/auth/client.go` - HTTP client with circuit breaker
- `api-service/internal/api/middleware/auth.go` - Auth middleware

**Token validation endpoint:**
```
POST /v1/auth/token/validate
Authorization: Bearer <token>

Response: { "valid": true, "user": { "id", "email", "role" } }
```

### With PostgreSQL
Uses dedicated PostgreSQL instance (`postgres-auth`) with:
- Database: `auth_db`
- User: `auth_user`

---

## 7. Environment Variables

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `NODE_ENV` | development | No | Environment |
| `PORT` | 8080 | No | Server port |
| `DATABASE_HOST` | postgres-auth | No | PostgreSQL host |
| `DATABASE_PORT` | 5432 | No | PostgreSQL port |
| `DATABASE_NAME` | auth_db | No | Database name |
| `DATABASE_USER` | auth_user | No | Database user |
| `DATABASE_PASSWORD` | - | **Yes** | Database password |
| `JWT_SECRET` | - | **Yes** | JWT signing secret (min 32 chars) |
| `JWT_ACCESS_TOKEN_EXPIRY` | 15m | No | Access token lifetime |
| `JWT_REFRESH_TOKEN_EXPIRY` | 7d | No | Refresh token lifetime |
| `RATE_LIMIT_WINDOW_MS` | 900000 | No | Rate limit window (15 min) |
| `RATE_LIMIT_MAX_REQUESTS` | 100 | No | Max requests per window |
| `LOG_LEVEL` | info | No | Log level |
| `METRICS_ENABLED` | true | No | Enable Prometheus metrics |

---

## 8. Checklist

### Copy Tasks
- [ ] Copy `src/` directory
- [ ] Copy `migrations/` directory
- [ ] Copy `package.json` and `package-lock.json`
- [ ] Copy TypeScript configs (`tsconfig.json`, `tsconfig.build.json`)
- [ ] Copy `Dockerfile` and `.dockerignore`
- [ ] Copy `.nvmrc`, `.gitignore`
- [ ] Copy `eslint.config.js`, `vitest.config.ts`

### Cleanup Tasks
- [ ] Remove old JavaScript files
- [ ] Remove legacy `deployments/` folder

### Modification Tasks
- [ ] Update `src/config/env.ts` with new defaults
- [ ] Create new `README.md`

### Verification Tasks
- [ ] Run `npm install`
- [ ] Run `npm run build` (TypeScript compiles)
- [ ] Run `npm test` (tests pass)
- [ ] Build Docker image
- [ ] Test Docker container locally

---

## 9. Dependencies on Other Components

| Component | Dependency | Notes |
|-----------|------------|-------|
| postgres-auth | Required | Must be deployed first |
| api-service | Downstream | Will call /v1/auth/token/validate |

---

## 10. Success Criteria

- [ ] TypeScript code compiles without errors
- [ ] All existing tests pass
- [ ] Docker image builds successfully
- [ ] Service starts and responds to health check
- [ ] Token validation endpoint works
- [ ] User CRUD operations work
- [ ] Prometheus metrics exposed

---

*Document Version: 1.0*  
*Created: January 2026*  
*Estimated Effort: 1 day*
