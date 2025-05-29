# Auth Service Interface Documentation

## API Overview

The Auth Service provides a RESTful API for managing authentication and authorization. All endpoints are prefixed with `/api/v1/auth`.

## Authentication

All endpoints except login and registration require authentication using a Bearer token in the Authorization header:

```
Authorization: Bearer <token>
```

## Endpoints

### Authentication Operations

#### Login

```http
POST /api/v1/auth/login
```

Authenticates a user and returns a JWT token.

##### Request Body

```json
{
  "email": "user@example.com",
  "password": "securepassword"
}
```

##### Response

```json
{
  "status": "success",
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "expires_at": "2024-02-21T10:00:00Z",
    "user": {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "email": "user@example.com",
      "role": "user",
      "status": "active",
      "created_at": "2024-02-20T10:00:00Z",
      "updated_at": "2024-02-20T10:00:00Z"
    }
  }
}
```

#### Register

```http
POST /api/v1/auth/register
```

Registers a new user.

##### Request Body

```json
{
  "email": "newuser@example.com",
  "password": "securepassword",
  "name": "New User"
}
```

##### Response

```json
{
  "status": "success",
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "email": "newuser@example.com",
    "name": "New User",
    "role": "user",
    "status": "active",
    "created_at": "2024-02-20T10:00:00Z",
    "updated_at": "2024-02-20T10:00:00Z"
  }
}
```

#### Validate Token

```http
POST /api/v1/auth/validate
```

Validates a JWT token.

##### Request Body

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

##### Response

```json
{
  "status": "success",
  "data": {
    "valid": true,
    "user": {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "email": "user@example.com",
      "role": "user",
      "status": "active"
    }
  }
}
```

#### Refresh Token

```http
POST /api/v1/auth/refresh
```

Refreshes an expired JWT token.

##### Request Body

```json
{
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

##### Response

```json
{
  "status": "success",
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "expires_at": "2024-02-21T10:00:00Z"
  }
}
```

### Session Operations

#### Create Session

```http
POST /api/v1/auth/sessions
```

Creates a new session for a user.

##### Request Body

```json
{
  "user_id": "550e8400-e29b-41d4-a716-446655440000",
  "device_info": {
    "device_id": "device123",
    "platform": "web",
    "browser": "chrome"
  }
}
```

##### Response

```json
{
  "status": "success",
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "user_id": "550e8400-e29b-41d4-a716-446655440000",
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "expires_at": "2024-02-21T10:00:00Z",
    "created_at": "2024-02-20T10:00:00Z",
    "updated_at": "2024-02-20T10:00:00Z"
  }
}
```

#### Get Session

```http
GET /api/v1/auth/sessions/{id}
```

Retrieves session information.

##### Response

```json
{
  "status": "success",
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "user_id": "550e8400-e29b-41d4-a716-446655440000",
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "expires_at": "2024-02-21T10:00:00Z",
    "created_at": "2024-02-20T10:00:00Z",
    "updated_at": "2024-02-20T10:00:00Z"
  }
}
```

#### Delete Session

```http
DELETE /api/v1/auth/sessions/{id}
```

Invalidates a session.

##### Response

```json
{
  "status": "success",
  "message": "Session invalidated successfully"
}
```

### Health and Metrics

#### Health Check

```http
GET /health
```

Checks the health status of the service.

##### Response

```json
{
  "status": "healthy",
  "version": "1.0.0",
  "timestamp": "2024-02-20T10:00:00Z"
}
```

#### Metrics

```http
GET /metrics
```

Returns Prometheus metrics.

##### Response

```
# HELP auth_service_requests_total Total number of requests
# TYPE auth_service_requests_total counter
auth_service_requests_total{endpoint="/api/v1/auth/login",method="POST"} 100

# HELP auth_service_request_duration_seconds Request duration in seconds
# TYPE auth_service_request_duration_seconds histogram
auth_service_request_duration_seconds_bucket{endpoint="/api/v1/auth/login",method="POST",le="0.1"} 90
```

## Error Responses

All endpoints may return the following error responses:

### Validation Error

```json
{
  "type": "VALIDATION_ERROR",
  "message": "Invalid request data",
  "details": ["Email is required", "Password must be at least 8 characters"]
}
```

### Authentication Error

```json
{
  "type": "AUTHENTICATION_ERROR",
  "message": "Invalid credentials"
}
```

### Authorization Error

```json
{
  "type": "AUTHORIZATION_ERROR",
  "message": "Insufficient permissions"
}
```

### Not Found Error

```json
{
  "type": "NOT_FOUND_ERROR",
  "message": "Session not found"
}
```

## Rate Limiting

The API implements rate limiting:

- 100 requests per minute per IP
- 1000 requests per hour per IP

Rate limit headers are included in all responses:

```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 99
X-RateLimit-Reset: 1613822400
```

## Service Dependencies

### Internal Services

1. **Profile Service**

   - Purpose: User profile management
   - Operations:
     - User data retrieval
     - Profile updates
     - User status checks

2. **Cache Service**

   - Purpose: Session management
   - Operations:
     - Session storage
     - Token caching
     - Rate limiting

3. **Monitoring Service**
   - Purpose: Metrics and monitoring
   - Operations:
     - Security metrics
     - Performance monitoring
     - Health checks
     - Alerting

### External Services

1. **PostgreSQL**

   - Purpose: User data storage
   - Operations:
     - User records
     - Session data
     - Audit logs

2. **Redis**

   - Purpose: Session management
   - Operations:
     - Session storage
     - Token blacklist
     - Rate limiting

3. **OAuth2 Providers**
   - Purpose: External authentication
   - Operations:
     - Social login
     - Token validation
     - User info retrieval
