# Profile Service Interface Documentation

This document details how the Profile Service connects with other services and its public API endpoints.

## API Endpoints

### Profile Management

#### GET /api/v1/profiles

- Description: Retrieve all profiles
- Authentication: Required
- Response: List of profiles
- Status Codes: 200, 401, 500

#### GET /api/v1/profiles/:id

- Description: Retrieve a specific profile
- Authentication: Required
- Response: Profile details
- Status Codes: 200, 401, 404, 500

#### POST /api/v1/profiles

- Description: Create a new profile
- Authentication: Required
- Request Body: Profile creation data
- Response: Created profile
- Status Codes: 201, 400, 401, 500

#### PUT /api/v1/profiles/:id

- Description: Update an existing profile
- Authentication: Required
- Request Body: Profile update data
- Response: Updated profile
- Status Codes: 200, 400, 401, 404, 500

#### DELETE /api/v1/profiles/:id

- Description: Delete a profile
- Authentication: Required
- Response: Success message
- Status Codes: 200, 401, 404, 500

### Task Management

#### POST /api/v1/profiles/:id/tasks

- Description: Submit a new task for profile processing
- Authentication: Required
- Request Body: Task details
- Response: Task submission confirmation
- Status Codes: 202, 400, 401, 404, 500

### Health and Metrics

#### GET /health

- Description: Service health check
- Authentication: None
- Response: Health status
- Status Codes: 200, 503

#### GET /metrics

- Description: Prometheus metrics
- Authentication: None
- Response: Metrics data
- Status Codes: 200

## External Service Connections

### Queue Service

- Purpose: Task processing and asynchronous operations
- Connection Type: RabbitMQ
- Operations:
  - Publish messages for task processing
  - Subscribe to task status updates
- Configuration:
  - Host: QUEUE_HOST
  - Port: QUEUE_PORT
  - Username: QUEUE_USERNAME
  - Password: QUEUE_PASSWORD
  - Queue Name: profile_tasks

### Storage Service

- Purpose: Profile data persistence
- Connection Type: gRPC
- Operations:
  - Create profile
  - Read profile
  - Update profile
  - Delete profile
  - Search profiles
- Configuration:
  - Host: STORAGE_HOST
  - Port: STORAGE_PORT

### Auth Service

- Purpose: Authentication and authorization
- Connection Type: gRPC
- Operations:
  - Validate tokens
  - Check permissions
- Configuration:
  - Host: AUTH_HOST
  - Port: AUTH_PORT

### Cache Service

- Purpose: Performance optimization
- Connection Type: Redis
- Operations:
  - Cache profile data
  - Cache task status
- Configuration:
  - Host: CACHE_HOST
  - Port: CACHE_PORT
  - Password: CACHE_PASSWORD

## Message Formats

### Task Submission

```json
{
  "type": "profile_update",
  "payload": {
    "user_id": "string",
    "changes": {
      // Dynamic payload based on task type
    }
  }
}
```

### Task Response

```json
{
  "task_id": "string",
  "status": "string",
  "result": {
    // Task-specific result data
  },
  "error": {
    "code": "string",
    "message": "string"
  }
}
```

## Error Handling

### HTTP Status Codes

- 200: Success
- 201: Created
- 202: Accepted
- 400: Bad Request
- 401: Unauthorized
- 404: Not Found
- 500: Internal Server Error
- 503: Service Unavailable

### Error Response Format

```json
{
  "error": {
    "code": "string",
    "message": "string",
    "details": {
      // Additional error details
    }
  }
}
```

## Rate Limiting

- Default: 100 requests per minute per client
- Burst: 200 requests per minute
- Headers:
  - X-RateLimit-Limit
  - X-RateLimit-Remaining
  - X-RateLimit-Reset

## Security

### Authentication

- JWT-based authentication
- Token validation through Auth Service
- Required headers:
  - Authorization: Bearer <token>

### Authorization

- Role-based access control
- Permission validation through Auth Service
- Required roles:
  - profile:read
  - profile:write
  - profile:delete
  - task:submit
