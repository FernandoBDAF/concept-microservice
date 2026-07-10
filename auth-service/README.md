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
| `DATABASE_PASSWORD` | - | Yes | Database password |
| `JWT_SECRET` | - | Yes | JWT signing secret (min 32 chars) |
| `JWT_ACCESS_TOKEN_EXPIRY` | 15m | No | Access token lifetime |
| `JWT_REFRESH_TOKEN_EXPIRY` | 7d | No | Refresh token lifetime |
| `RATE_LIMIT_WINDOW_MS` | 900000 | No | Rate limit window (15 min) |
| `RATE_LIMIT_MAX_REQUESTS` | 100 | No | Max requests per window |
| `ACCOUNT_LOCKOUT_ATTEMPTS` | 5 | No | Failed attempts before lockout |
| `ACCOUNT_LOCKOUT_DURATION_MS` | 1800000 | No | Lockout duration (30 min) |
| `PASSWORD_MIN_LENGTH` | 8 | No | Minimum password length |
| `METRICS_ENABLED` | true | No | Enable Prometheus metrics |
| `METRICS_PREFIX` | auth_service_ | No | Metrics prefix |
| `LOG_LEVEL` | info | No | Log level |
| `LOG_PRETTY` | false | No | Pretty logging |
| `API_SERVICE_URL` | http://api-service:8080 | No | api-service URL |

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
