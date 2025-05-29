# Auth Service

## Overview

The Auth Service is a critical component of our microservices architecture, responsible for managing authentication, authorization, and user session management. It provides secure access control and identity verification for all other services in the system.

### Role in the System

The Auth Service interacts with several other services in our microservices ecosystem:

1. **Internal Services**

   - Profile Service: User identity verification
   - Storage Service: Access control for stored data
   - Cache Service: Session management
   - Monitoring Service: Security metrics and alerts

2. **External Services**
   - PostgreSQL: User and session data storage
   - Redis: Session caching and rate limiting
   - JWT: Token management
   - OAuth2 providers: External authentication

### Main Functionalities

1. **Authentication**

   - User authentication
   - OAuth2 integration
   - JWT token management
   - Session handling

2. **Authorization**

   - Role-based access control
   - Permission management
   - Policy enforcement
   - Resource access control

3. **Security**

   - Password hashing
   - Token encryption
   - Rate limiting
   - Security logging

4. **Monitoring and Health**
   - Security metrics
   - Audit logging
   - Health checks
   - Performance monitoring

## Project Structure

```
auth-service/
├── cmd/
│   └── auth-service/
│       └── main.go
├── internal/
│   ├── domain/
│   │   ├── models/
│   │   ├── services/
│   │   ├── security/
│   │   └── interfaces/
│   ├── infrastructure/
│   │   ├── database/
│   │   ├── repository/
│   │   └── cache/
│   ├── pkg/
│   │   ├── config/
│   │   ├── logger/
│   │   └── utils/
│   └── server/
│       ├── http/
│       └── grpc/
├── pkg/
│   └── client/
├── api/
│   ├── proto/
│   └── openapi/
├── deployments/
│   ├── docker/
│   └── k8s/
├── docs/
│   ├── api/
│   └── architecture/
├── scripts/
├── test/
│   ├── integration/
│   └── e2e/
├── Dockerfile
├── go.mod
├── go.sum
├── README.md
├── CONTEXT.md
├── INTERFACE.md
└── TRACKER.md
```

## Quick Start

### Prerequisites

- Go 1.21 or later
- Docker and Docker Compose
- Kubernetes cluster (for production)
- PostgreSQL 14 or later
- Redis 6 or later

### Setup

1. **Clone the Repository**

   ```bash
   git clone https://github.com/your-org/microservices.git
   cd microservices/services/auth-service
   ```

2. **Install Dependencies**

   ```bash
   go mod download
   ```

3. **Configuration**
   Create a `config.yaml` file:

   ```yaml
   service:
     name: auth-service
     version: 1.0.0
     port: 8080

   database:
     host: localhost
     port: 5432
     name: auth
     user: postgres
     password: your-password

   redis:
     host: localhost
     port: 6379
     password: your-password

   jwt:
     secret: your-secret-key
     expiration: 24h
   ```

4. **Run Locally**

   ```bash
   go run cmd/auth-service/main.go
   ```

5. **Run with Docker**
   ```bash
   docker-compose up -d
   ```

### Development

1. **Common Tasks**

   ```bash
   # Run tests
   go test ./...

   # Build service
   go build -o auth-service ./cmd/auth-service

   # Run linter
   golangci-lint run
   ```

2. **Testing**

   ```bash
   # Unit tests
   go test -v ./internal/...

   # Integration tests
   go test -v ./test/integration/...

   # Load tests
   k6 run ./test/load/auth-service.js
   ```

## Documentation

For more detailed information, refer to:

- [CONTEXT.md](./CONTEXT.md): Technical architecture and design decisions
- [INTERFACE.md](./INTERFACE.md): API endpoints and service interactions
- [TRACKER.md](./TRACKER.md): Development progress and planned features

## Contributing

1. Fork the repository
2. Create a feature branch
3. Commit your changes
4. Push to the branch
5. Create a pull request

## License

MIT License

## Clerk Migration Plan

### Overview

The Auth Service is transitioning to use Clerk as the external authentication provider. This migration aims to enhance security, reduce maintenance overhead, and provide additional authentication features while maintaining backward compatibility with existing systems.

### New Architecture

1. **Clerk Integration Layer**

   - Clerk client integration for authentication flows
   - Token translation service for JWT compatibility
   - Session management adapter for Redis integration
   - User data synchronization with webhooks
   - Backward compatibility layer for existing clients

2. **Authentication Flow**

   - Clerk-based user authentication and registration
   - Token translation between Clerk and internal JWT
   - Session management with Redis integration
   - Role-based access control (RBAC) integration
   - Multi-factor authentication support

3. **Authorization Module**
   - Role-based access control (RBAC)
   - Permission management with Clerk roles
   - Policy enforcement through middleware
   - Access token validation and translation
   - Session-based authorization

### Integration Components

1. **Token Translation Service**

   - Clerk session token to JWT translation
   - Token validation and refresh handling
   - Token revocation and blacklisting
   - Session synchronization

2. **Session Management**

   - Redis session store integration
   - Session validation and refresh
   - Session cleanup and expiration
   - Device tracking and management

3. **User Data Synchronization**

   - Webhook handlers for user events
   - Data consistency checks
   - Profile synchronization
   - Role and permission mapping

### Security Implementation

1. **Authentication Security**

   - Multi-factor authentication support
   - Device tracking and management
   - Session security controls
   - Rate limiting and DDoS protection

2. **Token Security**

   - Secure token translation
   - Token validation and verification
   - Token revocation handling
   - Session token security

3. **Data Security**
   - Encrypted data transmission
   - Secure storage practices
   - Access control implementation
   - Audit logging and monitoring

### Monitoring and Observability

1. **Metrics Collection**

   - Authentication success/failure rates
   - Token validation metrics
   - Session management metrics
   - API performance metrics
   - Error rates and types

2. **Logging**

   - Structured logging for all operations
   - Security event logging
   - Audit trail for sensitive operations
   - Error tracking and monitoring
   - Performance logging

### New Dependencies

- Clerk SDK for Go
- Clerk Frontend API
- Clerk Backend API
- Existing dependencies maintained for backward compatibility:
  - Redis for session management
  - PostgreSQL for user data
  - JWT for token handling
  - Prometheus for metrics

### New Environment Variables

```bash
# Clerk Configuration
CLERK_SECRET_KEY=your-clerk-secret-key
CLERK_FRONTEND_API=your-clerk-frontend-api
CLERK_BACKEND_API=your-clerk-backend-api
CLERK_JWT_VERIFICATION_KEY=your-jwt-verification-key

# Token Translation
TOKEN_TRANSLATION_ENABLED=true
TOKEN_CACHE_TTL=3600
TOKEN_REFRESH_THRESHOLD=300

# Session Management
SESSION_SYNC_ENABLED=true
SESSION_CACHE_TTL=86400
SESSION_CLEANUP_INTERVAL=3600

# Existing configuration maintained
SERVICE_NAME=profile-auth
SERVICE_PORT=8080
METRICS_PORT=9090
JWT_SECRET=your-secret-key
JWT_EXPIRATION=24h
JWT_REFRESH_EXPIRATION=168h
```

### New API Endpoints

```bash
# Clerk Authentication Endpoints
curl -X POST profile-auth/v1/clerk/auth/sign-up \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "securePassword123"
  }'

curl -X POST profile-auth/v1/clerk/auth/sign-in \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "securePassword123"
  }'

# Token Translation Endpoint
curl -X POST profile-auth/v1/clerk/token/translate \
  -H "Content-Type: application/json" \
  -d '{
    "clerk_token": "clerk_session_token"
  }'

# Session Management Endpoints
curl -X GET profile-auth/v1/clerk/sessions \
  -H "Authorization: Bearer clerk_session_token"

curl -X DELETE profile-auth/v1/clerk/sessions/{session_id} \
  -H "Authorization: Bearer clerk_session_token"
```

### Migration Strategy

1. **Phase 1: Preparation**

   - Clerk account setup and configuration
   - Environment and SDK setup
   - Development and testing environment preparation
   - Documentation updates

2. **Phase 2: Core Integration**

   - Clerk client implementation
   - Token translation service
   - Session management adapter
   - API endpoint modifications
   - Backward compatibility layer

3. **Phase 3: Data Migration**

   - User data migration scripts
   - Session migration handling
   - Token translation implementation
   - Data verification and validation
   - Rollback procedures

4. **Phase 4: Testing**

   - Integration testing with Clerk
   - Performance testing and optimization
   - Security testing and validation
   - Backward compatibility testing
   - Load testing and scalability

5. **Phase 5: Deployment**
   - Staged rollout planning
   - Monitoring and alerting setup
   - Rollback procedures
   - Performance monitoring
   - User communication

### Security Considerations

1. **Clerk Security Features**

   - Multi-factor authentication
   - Session management
   - Device tracking
   - Security monitoring
   - Rate limiting

2. **Token Security**

   - Secure token translation
   - Token validation
   - Session management
   - Token revocation
   - Blacklisting

3. **Data Security**
   - Secure user data migration
   - Data encryption
   - Access control
   - Audit logging
   - Compliance requirements

### Best Practices

1. **Clerk Integration**

   - Follow Clerk's security guidelines
   - Implement proper error handling
   - Use recommended SDK features
   - Monitor Clerk's status
   - Regular security reviews

2. **Migration**

   - Maintain backward compatibility
   - Implement proper logging
   - Monitor performance
   - Track migration progress
   - Regular testing

3. **Security**
   - Regular security audits
   - Monitor access patterns
   - Implement rate limiting
   - Track security events
   - Update security policies

### Key Documentation References

#### Clerk Documentation

- [Clerk API Reference](https://clerk.com/docs)
- [Clerk SDK Documentation](https://clerk.com/docs/sdks)
- [Clerk Security Guide](https://clerk.com/docs/security)

#### Migration Guides

- [Clerk Migration Guide](../../../docs/migration/clerk/guide.md)
- [Token Translation Guide](../../../docs/migration/clerk/token-translation.md)
- [Data Migration Guide](../../../docs/migration/clerk/data-migration.md)
