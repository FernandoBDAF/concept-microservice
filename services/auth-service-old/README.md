INITIAL CONTEXT FOR LLM - never change the context-----------------------------
-> THIS SECTION IS A GUIDELINE TO THE LLM CONSIDER BEFORE WORKING IN THIS FILE, DO NOT CHANGE THIS

-> GOES OF THE README FILE:

- This file serves as the technical documentation of the service, providing a comprehensive overview of the codebase
- It should document:
  - Service architecture and design decisions
  - Component structure and relationships
  - API endpoints and interfaces
  - Dependencies and integration points
  - Configuration and deployment details
- This is the primary reference for understanding the technical implementation
- This file should be in sync with the `/TRACKER&MANAGER.md` where development progress and tasks are tracked
- While TRACKER&MANAGER.md focuses on "what" and "when", this file focuses on "how" and "why"

-> CONSIDERER BEFORE UPDATING THIS FILE:

- Never add fictional dates, version numbers, or metrics. Only include real, verified information. If information is not available, mark it as "To be determined" or remove the section.
- The changes in this file need to be incremental or to update informations that you confidentilly have knowlegde, they should not be guesses. If there are questions or uncertanty add comments asking for clarification instead.
- Check the `../../microservices/docs` folder for comprehensive project details, including architecture, development guidelines, and integration points. This will help in making informed decisions, haver better context and updates to the development plan. Always compare the implementation of this project with the plan described in the docs and whenever there are inconsistancies, add comments.
- Consider structuring this documentation separating the components and describing each one of them and adding sections to how they interact with each other - because this will be very dinamic and updated during the development process it will make clear what to update after each change
- This documentation is focusing in the current component that is part of a larger project, to have a more sistemic view check `../../microservices/TRACKER&MANAGER` and `../../microservices/README`
- Do not forget to be LLM focus, so because this will be used
- For LLM-specific guidelines and patterns, refer to [LLM Integration Guide](../../docs/llm/README.md)

---

# Auth Service

## Overview

The Auth Service handles user authentication and authorization operations. It provides secure user management and token-based authentication for the microservices architecture.

## Architecture

### Core Components

1. **API Layer**

   - REST API endpoints
   - Request validation
   - Response formatting
   - Error handling

2. **Service Layer**

   - Business logic
   - Data transformation
   - Integration with shared libraries
   - Error handling

3. **Integration Layer**
   - Shared libraries integration
   - API services communication
   - Circuit breaking
   - Retry mechanisms

### Shared Libraries Integration

1. **Logging Library**

   ```go
   // Initialize logger
   logger := logging.NewLogger("auth-service")

   // Usage example
   logger.Info("Processing request",
       logging.WithField("user_id", userID),
       logging.WithField("action", "login"))
   ```

2. **Monitoring Library**

   ```go
   // Initialize collector
   monitor := monitoring.NewCollector("auth-service")

   // Usage example
   monitor.IncRequests("user_login")
   defer monitor.ObserveDuration("user_login")
   ```

## Primary Purpose

The Auth Service provides authentication and authorization functionality for the microservices architecture. This service is responsible for user authentication, JWT token management, OAuth 2.0 integration, and role-based access control.

## Service Architecture

### Core Components

1. **Authentication Module**

   - User registration and login
   - Password hashing and validation
   - Session management
   - Token generation and validation

2. **Authorization Module**

   - Role-based access control (RBAC)
   - Permission management
   - Policy enforcement
   - Access token validation

3. **OAuth 2.0 Module**

   - Authorization code flow
   - Token exchange
   - User info retrieval
   - State validation

4. **Session Management**
   - Redis-based session storage
   - Token management
   - Rate limiting
   - Cache policies

### Dependencies

- Redis: Session storage and token management
- PostgreSQL: User data and role management
- JWT: Token generation and validation
- Prometheus: Metrics collection
- Grafana: Metrics visualization

## API Documentation

### Authentication Endpoints

```bash
# Register a new user
curl -X POST profile-auth/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "securePassword123",
    "role": "user"
  }'

# Login with credentials
curl -X POST profile-auth/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "securePassword123"
  }'

# Refresh token
curl -X POST profile-auth/v1/auth/token/refresh \
  -H "Content-Type: application/json" \
  -d '{
    "refresh_token": "mock_refresh_token"
  }'

# Validate token
curl -X POST profile-auth/v1/auth/token/validate \
  -H "Content-Type: application/json" \
  -d '{
    "token": "mock_access_token"
  }'

# Reset password
curl -X POST profile-auth/v1/auth/password/reset \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com"
  }'
```

### User Endpoints

```bash
# Get current user
curl -X GET profile-auth/v1/users/me \
  -H "Authorization: Bearer mock_access_token"

# Get user by ID
curl -X GET profile-auth/v1/users/{id} \
  -H "Authorization: Bearer mock_access_token"
```

### OAuth Endpoints

```bash
# OAuth authorization
curl -X GET profile-auth/v1/oauth/authorize

# OAuth token
curl -X POST profile-auth/v1/oauth/token \
  -H "Content-Type: application/json" \
  -d '{
    "grant_type": "authorization_code",
    "code": "mock_authorization_code"
  }'

# OAuth user info
curl -X GET profile-auth/v1/oauth/userinfo \
  -H "Authorization: Bearer mock_oauth_access_token"
```

### RBAC Endpoints

```bash
# Get roles
curl -X GET profile-auth/v1/rbac/roles \
  -H "Authorization: Bearer mock_access_token"

# Get permissions
curl -X GET profile-auth/v1/rbac/permissions \
  -H "Authorization: Bearer mock_access_token"
```

## Response Format

All endpoints return responses in the following format:

```json
{
  "status": "success",
  "message": "Operation successful",
  "data": {
    // Response data specific to the endpoint
  }
}
```

## Error Responses

All endpoints may return the following error responses:

```json
// 400 Bad Request
{
  "status": "error",
  "message": "Invalid request body"
}

// 401 Unauthorized
{
  "status": "error",
  "message": "Invalid or expired token"
}

// 403 Forbidden
{
  "status": "error",
  "message": "Insufficient permissions"
}

// 404 Not Found
{
  "status": "error",
  "message": "Resource not found"
}

// 500 Internal Server Error
{
  "status": "error",
  "message": "An unexpected error occurred"
}
```

## Development

### Local Development

1. Start the service:

```bash
make run
```

2. Run tests:

```bash
make test
```

3. Build the service:

```bash
make build
```

### Environment Variables

The service can be configured using the following environment variables:

```bash
# Service Configuration
SERVICE_NAME=profile-auth
SERVICE_PORT=8080
METRICS_PORT=9090

# JWT Configuration
JWT_SECRET=your-secret-key
JWT_EXPIRATION=24h
JWT_REFRESH_EXPIRATION=168h

# Rate Limiting
RATE_LIMIT_REQUESTS=100
RATE_LIMIT_DURATION=1m

# Logging
LOG_LEVEL=info
LOG_FORMAT=json

# Environment
ENVIRONMENT=development
```

## Security Considerations

1. All passwords are hashed using bcrypt
2. JWT tokens are signed with a secure secret
3. Rate limiting is implemented to prevent brute force attacks
4. HTTPS is required in production
5. CORS is configured to allow only trusted origins
6. Input validation is performed on all endpoints
7. Sensitive data is never logged

## Best Practices

1. Always use HTTPS in production
2. Store tokens securely (HttpOnly cookies or secure storage)
3. Implement proper error handling
4. Use appropriate HTTP status codes
5. Follow RESTful conventions
6. Implement proper logging
7. Monitor rate limits and errors

## Key Documentation References

### Architecture

- [Architecture Overview](../../../docs/architecture/README.md)
- [Service Architecture](../../../docs/architecture/services/auth-service.md)
- [Security Architecture](../../../docs/architecture/overview/security.md)

### Development

- [Development Guide](../../../docs/guides/development/guide.md)
- [Testing Guide](../../../docs/guides/development/testing/guide.md)
- [Environment Setup](../../../docs/guides/development/environment/guide.md)

### API

- [API Specification](../../../docs/api/openapi/auth-api.yaml)
- [API Security](../../../docs/api/security.md)
- [API Examples](../../../docs/api/examples/)

### Operations

- [Monitoring Guide](../../../docs/guides/operations/monitoring/guide.md)
- [Logging Guide](../../../docs/guides/operations/logging/guide.md)
- [Troubleshooting Guide](../../../docs/guides/operations/troubleshooting/guide.md)

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
