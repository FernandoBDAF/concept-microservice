# Profile Service Interface Documentation

This document details the Profile Service's public API endpoints, message formats, and integration patterns with the multi-worker architecture ecosystem.

## Service Role & Integration

The **Profile Service** serves as the **primary entry point and orchestrator** for the microservices task processing ecosystem. It coordinates with the upgraded queue-service to route tasks to specialized workers based on task types.

### Integration Architecture

```
Client Applications → Profile Service → Queue Service → RabbitMQ → Multi-Workers
                                           ↓
                         ┌─────────────────┼─────────────────┐
                         ↓                 ↓                 ↓
                 profile.task         email.send      image.process
                         ↓                 ↓                 ↓
              Profile Processing   Email Processing   Image Processing
                         ↓                 ↓                 ↓
                Profile Worker      Email Worker       Image Worker
```

## API Endpoints

### Profile Management Endpoints

#### GET /api/v1/profiles

**Description**: Retrieve all profiles with pagination and search capabilities  
**Authentication**: Required (Bearer token)  
**Method**: GET

**Query Parameters**:

- `page` (optional): Page number (default: 1)
- `limit` (optional): Items per page (default: 20, max: 100)
- `search` (optional): Search term for profile filtering

**Request Headers**:

```
Authorization: Bearer <jwt_token>
Content-Type: application/json
```

**Response Format**:

```json
{
  "profiles": [
    {
      "id": "uuid",
      "email": "user@example.com",
      "name": "John Doe",
      "created_at": "2024-01-01T00:00:00Z",
      "updated_at": "2024-01-01T00:00:00Z"
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 20,
    "total": 100,
    "total_pages": 5
  }
}
```

**Status Codes**: 200 (Success), 401 (Unauthorized), 500 (Internal Error)

#### GET /api/v1/profiles/:id

**Description**: Retrieve a specific profile by ID  
**Authentication**: Required (Bearer token)  
**Method**: GET

**Path Parameters**:

- `id`: Profile UUID

**Response Format**:

```json
{
  "profile": {
    "id": "uuid",
    "email": "user@example.com",
    "name": "John Doe",
    "phone": "+1234567890",
    "avatar_url": "https://example.com/avatar.jpg",
    "preferences": {
      "language": "en",
      "timezone": "UTC"
    },
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z"
  }
}
```

**Status Codes**: 200 (Success), 401 (Unauthorized), 404 (Not Found), 500 (Internal Error)

#### POST /api/v1/profiles

**Description**: Create a new user profile  
**Authentication**: Required (Bearer token)  
**Method**: POST

**Request Body**:

```json
{
  "email": "user@example.com",
  "name": "John Doe",
  "phone": "+1234567890",
  "avatar_url": "https://example.com/avatar.jpg",
  "preferences": {
    "language": "en",
    "timezone": "UTC"
  }
}
```

**Response Format**:

```json
{
  "profile": {
    "id": "generated-uuid",
    "email": "user@example.com",
    "name": "John Doe",
    "phone": "+1234567890",
    "avatar_url": "https://example.com/avatar.jpg",
    "preferences": {
      "language": "en",
      "timezone": "UTC"
    },
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z"
  }
}
```

**Status Codes**: 201 (Created), 400 (Bad Request), 401 (Unauthorized), 500 (Internal Error)

#### PUT /api/v1/profiles/:id

**Description**: Update an existing profile  
**Authentication**: Required (Bearer token)  
**Method**: PUT

**Path Parameters**:

- `id`: Profile UUID

**Request Body**: Same as POST (partial updates supported)

**Response Format**: Same as GET profile response

**Status Codes**: 200 (Success), 400 (Bad Request), 401 (Unauthorized), 404 (Not Found), 500 (Internal Error)

#### DELETE /api/v1/profiles/:id

**Description**: Delete a profile  
**Authentication**: Required (Bearer token)  
**Method**: DELETE

**Path Parameters**:

- `id`: Profile UUID

**Response Format**:

```json
{
  "message": "Profile deleted successfully",
  "deleted_id": "uuid"
}
```

**Status Codes**: 200 (Success), 401 (Unauthorized), 404 (Not Found), 500 (Internal Error)

### Multi-Worker Task Management

#### POST /api/v1/profiles/:id/tasks

**Description**: Submit a task for asynchronous processing by specialized workers  
**Authentication**: Required (Bearer token)  
**Method**: POST

**Path Parameters**:

- `id`: Profile UUID

**Supported Task Types**:

- `profile_update` - Routes to Profile Worker (`profile.task`)
- `email_notification` - Routes to Email Worker (`email.send`)
- `image_processing` - Routes to Image Worker (`image.process`)

**Request Body Format**:

```json
{
  "type": "profile_update|email_notification|image_processing",
  "payload": {
    // Task-specific payload (see examples below)
  }
}
```

**Response Format**:

```json
{
  "task": {
    "id": "task-uuid",
    "profile_id": "profile-uuid",
    "type": "task_type",
    "status": "submitted",
    "routing_key": "worker.type",
    "created_at": "2024-01-01T00:00:00Z",
    "estimated_completion": "2024-01-01T00:05:00Z"
  }
}
```

**Status Codes**: 202 (Accepted), 400 (Bad Request), 401 (Unauthorized), 404 (Profile Not Found), 500 (Internal Error)

### Task Type Specifications

#### 1. Profile Processing Tasks

**Task Type**: `profile_update`  
**Routing Key**: `profile.task`  
**Worker**: Profile Worker  
**Use Cases**: Profile updates, data synchronization, profile deletion tasks

**Request Example**:

```json
{
  "type": "profile_update",
  "payload": {
    "user_id": "123",
    "action": "update|delete|sync",
    "changes": {
      "email": "new@example.com",
      "name": "Updated Name",
      "preferences": {
        "language": "es"
      }
    },
    "reason": "user_request",
    "priority": "normal"
  }
}
```

#### 2. Email Notification Tasks

**Task Type**: `email_notification`  
**Routing Key**: `email.send`  
**Worker**: Email Worker  
**Use Cases**: Welcome emails, notifications, alerts, password resets

**Request Example**:

```json
{
  "type": "email_notification",
  "payload": {
    "to": "user@example.com",
    "template": "welcome|profile_updated|password_reset|notification",
    "variables": {
      "user_name": "John Doe",
      "activation_link": "https://app.example.com/activate/token123",
      "expires_at": "2024-01-02T00:00:00Z"
    },
    "priority": "high|normal|low",
    "send_after": "2024-01-01T00:00:00Z"
  }
}
```

#### 3. Image Processing Tasks

**Task Type**: `image_processing`  
**Routing Key**: `image.process`  
**Worker**: Image Worker  
**Use Cases**: Avatar processing, image optimization, format conversion

**Request Example**:

```json
{
  "type": "image_processing",
  "payload": {
    "image_url": "https://example.com/profile/image.jpg",
    "operations": ["resize", "compress", "convert"],
    "output_format": "webp|jpeg|png",
    "dimensions": {
      "width": 800,
      "height": 600
    },
    "quality": 85,
    "destination": "s3://bucket/processed/",
    "callback_url": "https://api.example.com/webhooks/image-processed"
  }
}
```

### Health and Monitoring Endpoints

#### GET /health

**Description**: Service health check with dependency status  
**Authentication**: None  
**Method**: GET

**Response Format**:

```json
{
  "status": "healthy|degraded|unhealthy",
  "timestamp": "2024-01-01T00:00:00Z",
  "dependencies": {
    "queue_service": {
      "status": "healthy|unhealthy",
      "response_time": "45ms",
      "last_check": "2024-01-01T00:00:00Z"
    },
    "storage_service": {
      "status": "healthy|unhealthy",
      "response_time": "12ms",
      "last_check": "2024-01-01T00:00:00Z"
    },
    "auth_service": {
      "status": "healthy|unhealthy",
      "response_time": "8ms",
      "last_check": "2024-01-01T00:00:00Z"
    },
    "cache_service": {
      "status": "healthy|unhealthy",
      "response_time": "3ms",
      "last_check": "2024-01-01T00:00:00Z"
    }
  }
}
```

**Status Codes**: 200 (Healthy), 503 (Unhealthy)

#### GET /metrics

**Description**: Prometheus metrics endpoint  
**Authentication**: None  
**Method**: GET

**Response Format**: Prometheus metrics format

**Key Metrics**:

```
# Task submission metrics
profile_tasks_submitted_total{type="profile_update|email_notification|image_processing"}
profile_tasks_routing_key_distribution{routing_key="profile.task|email.send|image.process"}

# Queue service communication metrics
profile_queue_service_requests_total{status="success|error"}
profile_queue_service_request_duration_seconds

# API endpoint metrics
profile_api_requests_total{method="GET|POST|PUT|DELETE", endpoint="/profiles|/tasks"}
profile_api_request_duration_seconds{method="GET|POST|PUT|DELETE", endpoint="/profiles|/tasks"}

# Profile management metrics
profile_operations_total{operation="create|read|update|delete"}
profile_operations_duration_seconds{operation="create|read|update|delete"}
```

**Status Codes**: 200 (Success)

## Message Format Specifications

### Queue-Service Integration Message Format

The Profile Service uses a standardized message format compatible with the upgraded queue-service:

```go
type QueueMessage struct {
    ID         string            `json:"id"`         // Unique message identifier
    Type       string            `json:"type"`       // Task type for routing
    Payload    json.RawMessage   `json:"payload"`    // Task-specific data (JSON)
    Timestamp  time.Time         `json:"timestamp"`  // Message creation time
    Metadata   map[string]string `json:"metadata"`   // Additional context
    RoutingKey string            `json:"routing_key"` // Worker routing key
}
```

### Routing Key Determination

The service automatically determines routing keys based on task types:

```go
var RoutingKeyMap = map[string]string{
    "profile_update":     "profile.task",
    "email_notification": "email.send",
    "image_processing":   "image.process",
}
```

### Message Metadata

Standard metadata fields included in all messages:

```json
{
  "metadata": {
    "profile_id": "uuid",
    "user_id": "uuid",
    "source": "profile-service",
    "version": "1.0",
    "trace_id": "trace-uuid",
    "correlation_id": "correlation-uuid"
  }
}
```

## External Service Connections

### Queue Service Integration

**Purpose**: Task routing and asynchronous processing coordination  
**Connection Type**: HTTP API  
**Communication Pattern**: Request/Response with task submission

**Configuration**:

- **URL**: `QUEUE_SERVICE_URL` (e.g., `http://queue-service:8080`)
- **Timeout**: `QUEUE_SERVICE_TIMEOUT` (default: 30s)
- **Retries**: `QUEUE_SERVICE_RETRIES` (default: 3)

**Operations**:

- **POST** `/api/v1/queue/messages` - Publish task messages
- **GET** `/api/v1/queue/routing-keys` - Get available routing keys
- **GET** `/health` - Health check

**Message Publishing Request**:

```json
{
  "id": "message-uuid",
  "type": "profile_update",
  "payload": "{\"user_id\":\"123\",\"action\":\"update\"}",
  "timestamp": "2024-01-01T00:00:00Z",
  "metadata": {
    "profile_id": "profile-uuid",
    "source": "profile-service"
  },
  "routing_key": "profile.task"
}
```

### Storage Service Integration

**Purpose**: Profile data persistence  
**Connection Type**: gRPC  
**Communication Pattern**: Synchronous CRUD operations

**Configuration**:

- **Host**: `STORAGE_SERVICE_HOST`
- **Port**: `STORAGE_SERVICE_PORT`
- **Timeout**: `STORAGE_SERVICE_TIMEOUT`

**Operations**:

- `CreateProfile(ProfileData) → ProfileID`
- `GetProfile(ProfileID) → ProfileData`
- `UpdateProfile(ProfileID, ProfileData) → ProfileData`
- `DeleteProfile(ProfileID) → Success`
- `ListProfiles(Pagination) → ProfileList`

### Auth Service Integration

**Purpose**: Authentication and authorization  
**Connection Type**: gRPC  
**Communication Pattern**: Token validation and permission checks

**Configuration**:

- **Host**: `AUTH_SERVICE_HOST`
- **Port**: `AUTH_SERVICE_PORT`
- **Timeout**: `AUTH_SERVICE_TIMEOUT`

**Operations**:

- `ValidateToken(Token) → UserInfo`
- `CheckPermission(UserID, Resource, Action) → Allowed`
- `GetUserRoles(UserID) → Roles`

### Cache Service Integration

**Purpose**: Performance optimization and session management  
**Connection Type**: Redis Protocol  
**Communication Pattern**: Key-value operations

**Configuration**:

- **Host**: `CACHE_SERVICE_HOST`
- **Port**: `CACHE_SERVICE_PORT`
- **Password**: `CACHE_SERVICE_PASSWORD`
- **Database**: `CACHE_SERVICE_DB`

**Operations**:

- Profile caching with TTL
- Session data storage
- Task status caching
- Rate limiting counters

## Error Handling

### HTTP Status Codes

| Code | Description           | Use Cases                                     |
| ---- | --------------------- | --------------------------------------------- |
| 200  | OK                    | Successful GET, PUT, DELETE operations        |
| 201  | Created               | Successful POST operations (profile creation) |
| 202  | Accepted              | Successful task submission                    |
| 400  | Bad Request           | Invalid request body, validation errors       |
| 401  | Unauthorized          | Missing or invalid authentication token       |
| 403  | Forbidden             | Insufficient permissions                      |
| 404  | Not Found             | Profile or resource not found                 |
| 409  | Conflict              | Duplicate email or constraint violation       |
| 429  | Too Many Requests     | Rate limit exceeded                           |
| 500  | Internal Server Error | Unexpected server errors                      |
| 503  | Service Unavailable   | Dependencies unavailable                      |

### Error Response Format

```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable error description",
    "details": {
      "field": "specific_field_with_error",
      "supported_values": ["list", "of", "valid", "options"],
      "routing_key": "attempted_routing_key",
      "task_type": "attempted_task_type"
    },
    "trace_id": "trace-uuid",
    "timestamp": "2024-01-01T00:00:00Z"
  }
}
```

### Common Error Codes

#### Profile Management Errors

- `PROFILE_NOT_FOUND` - Profile with specified ID doesn't exist
- `PROFILE_ALREADY_EXISTS` - Profile with email already exists
- `INVALID_PROFILE_DATA` - Profile data validation failed
- `PROFILE_UPDATE_FAILED` - Profile update operation failed

#### Task Submission Errors

- `INVALID_TASK_TYPE` - Unsupported task type provided
- `INVALID_TASK_PAYLOAD` - Task payload validation failed
- `QUEUE_SERVICE_UNAVAILABLE` - Cannot connect to queue service
- `ROUTING_KEY_INVALID` - Invalid routing key for task type
- `TASK_SUBMISSION_FAILED` - Failed to submit task to queue

#### Authentication & Authorization Errors

- `MISSING_TOKEN` - Authorization header missing
- `INVALID_TOKEN` - JWT token invalid or expired
- `INSUFFICIENT_PERMISSIONS` - User lacks required permissions
- `AUTH_SERVICE_UNAVAILABLE` - Cannot validate token

## Rate Limiting

### Default Limits

- **Profile Operations**: 100 requests per minute per user
- **Task Submissions**: 50 requests per minute per user
- **Health Checks**: 1000 requests per minute (no user limit)

### Rate Limit Headers

```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1640995200
X-RateLimit-Window: 60
```

### Rate Limit Error Response

```json
{
  "error": {
    "code": "RATE_LIMIT_EXCEEDED",
    "message": "Too many requests. Please try again later.",
    "details": {
      "limit": 100,
      "window": 60,
      "reset_at": "2024-01-01T00:01:00Z"
    }
  }
}
```

## Security

### Authentication

**Method**: JWT Bearer Token  
**Header**: `Authorization: Bearer <token>`  
**Token Source**: Auth Service  
**Validation**: On every request

### Authorization

**Model**: Role-Based Access Control (RBAC)  
**Permissions Required**:

| Operation                | Required Permission |
| ------------------------ | ------------------- |
| GET /profiles            | `profile:read`      |
| POST /profiles           | `profile:write`     |
| PUT /profiles/:id        | `profile:write`     |
| DELETE /profiles/:id     | `profile:delete`    |
| POST /profiles/:id/tasks | `task:submit`       |

### Data Protection

- **Input Validation**: All request data validated
- **SQL Injection Protection**: Parameterized queries
- **XSS Protection**: Input sanitization
- **HTTPS Only**: All communication encrypted
- **Token Encryption**: JWT tokens properly signed

## Performance Characteristics

### Response Time Targets

| Endpoint                 | Target Response Time |
| ------------------------ | -------------------- |
| GET /profiles            | < 100ms              |
| GET /profiles/:id        | < 50ms               |
| POST /profiles           | < 150ms              |
| PUT /profiles/:id        | < 100ms              |
| DELETE /profiles/:id     | < 75ms               |
| POST /profiles/:id/tasks | < 50ms               |
| GET /health              | < 10ms               |
| GET /metrics             | < 20ms               |

### Throughput Targets

- **Profile CRUD Operations**: 500+ requests/second
- **Task Submissions**: 1000+ requests/second
- **Concurrent Connections**: 1000+

### Caching Strategy

- **Profile Data**: 5-minute TTL
- **User Sessions**: 30-minute TTL
- **Task Status**: 1-minute TTL
- **Rate Limiting**: 1-minute windows

## Integration Testing

### End-to-End Test Scenarios

#### Profile Task Flow Test

```bash
# 1. Create profile
curl -X POST http://profile-service:8080/api/v1/profiles \
  -H "Authorization: Bearer ${TOKEN}" \
  -d '{"email":"test@example.com","name":"Test User"}'

# 2. Submit profile update task
curl -X POST http://profile-service:8080/api/v1/profiles/${PROFILE_ID}/tasks \
  -H "Authorization: Bearer ${TOKEN}" \
  -d '{"type":"profile_update","payload":{"action":"update","changes":{"name":"Updated Name"}}}'

# 3. Verify task submission in queue-service logs
# Expected: routing_key="profile.task", type="profile_update"
```

#### Email Task Flow Test

```bash
# Submit email notification task
curl -X POST http://profile-service:8080/api/v1/profiles/${PROFILE_ID}/tasks \
  -H "Authorization: Bearer ${TOKEN}" \
  -d '{
    "type": "email_notification",
    "payload": {
      "to": "user@example.com",
      "template": "welcome",
      "variables": {"user_name": "John Doe"}
    }
  }'

# Expected: routing_key="email.send", type="email_notification"
```

#### Image Processing Task Flow Test

```bash
# Submit image processing task
curl -X POST http://profile-service:8080/api/v1/profiles/${PROFILE_ID}/tasks \
  -H "Authorization: Bearer ${TOKEN}" \
  -d '{
    "type": "image_processing",
    "payload": {
      "image_url": "https://example.com/avatar.jpg",
      "operations": ["resize", "compress"],
      "dimensions": {"width": 200, "height": 200}
    }
  }'

# Expected: routing_key="image.process", type="image_processing"
```

## Monitoring and Observability

### Health Check Integration

The service provides comprehensive health checks that validate:

- **Self Health**: Service operational status
- **Queue Service**: HTTP connectivity and response time
- **Storage Service**: gRPC connectivity and response time
- **Auth Service**: gRPC connectivity and response time
- **Cache Service**: Redis connectivity and response time

### Metrics Collection

Comprehensive metrics are exposed for monitoring:

- **Request Metrics**: Rate, latency, status codes
- **Task Metrics**: Submission rate, routing key distribution
- **Dependency Metrics**: Response times, availability
- **Business Metrics**: Profile operations, user activity

### Structured Logging

All operations include structured logging with:

- **Trace ID**: Request tracing across services
- **User Context**: User ID, profile ID when available
- **Operation Context**: Endpoint, method, parameters
- **Performance Data**: Response times, payload sizes
- **Error Context**: Error codes, stack traces, recovery actions

## Future Enhancements

### Planned API Extensions

1. **Batch Operations**: Bulk profile operations
2. **Profile Search**: Advanced search and filtering
3. **Task Status Tracking**: Real-time task progress
4. **Webhook Support**: Task completion notifications
5. **File Upload**: Direct avatar upload handling

### Integration Improvements

1. **GraphQL Support**: Alternative API interface
2. **WebSocket Integration**: Real-time updates
3. **Event Streaming**: Profile change events
4. **Advanced Caching**: Multi-level cache strategy
5. **Circuit Breakers**: Enhanced resilience patterns

This interface documentation provides comprehensive coverage of the Profile Service's integration patterns, API endpoints, and multi-worker task orchestration capabilities, ensuring seamless integration with the upgraded queue-service and worker architecture.
