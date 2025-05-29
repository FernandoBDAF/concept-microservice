# Storage Service Interface Documentation

## API Overview

The Storage Service provides a RESTful API for managing data storage operations. All endpoints are prefixed with `/api/v1/storage`.

## Authentication

All endpoints require authentication using a Bearer token in the Authorization header:

```
Authorization: Bearer <token>
```

## Endpoints

### File Operations

#### Upload File

```http
POST /api/v1/storage/files
```

Uploads a new file to storage.

##### Request

```http
Content-Type: multipart/form-data

file: <file>
type: string
path: string
metadata: {
    "content_type": "string",
    "attributes": {}
}
```

##### Response

```json
{
  "status": "success",
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "type": "file",
    "path": "/uploads/example.pdf",
    "size": 1024,
    "created_at": "2024-02-20T10:00:00Z",
    "updated_at": "2024-02-20T10:00:00Z",
    "metadata": {
      "content_type": "application/pdf",
      "version": 1,
      "attributes": {
        "owner": "user123",
        "tags": ["document", "pdf"]
      }
    }
  }
}
```

#### Download File

```http
GET /api/v1/storage/files/{id}
```

Downloads a file from storage.

##### Response

```http
Content-Type: <file_content_type>
Content-Disposition: attachment; filename="<filename>"

<file_content>
```

#### Delete File

```http
DELETE /api/v1/storage/files/{id}
```

Deletes a file from storage.

##### Response

```json
{
  "status": "success",
  "message": "File deleted successfully"
}
```

### Object Storage Operations

#### Create Object

```http
POST /api/v1/storage/objects
```

Creates a new object in storage.

##### Request Body

```json
{
  "type": "object",
  "path": "/objects/example",
  "data": {
    "key": "value"
  },
  "metadata": {
    "content_type": "application/json",
    "attributes": {}
  }
}
```

##### Response

```json
{
  "status": "success",
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "type": "object",
    "path": "/objects/example",
    "size": 1024,
    "created_at": "2024-02-20T10:00:00Z",
    "updated_at": "2024-02-20T10:00:00Z",
    "metadata": {
      "content_type": "application/json",
      "version": 1,
      "attributes": {}
    }
  }
}
```

#### Get Object

```http
GET /api/v1/storage/objects/{id}
```

Retrieves an object from storage.

##### Response

```json
{
  "status": "success",
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "type": "object",
    "path": "/objects/example",
    "data": {
      "key": "value"
    },
    "created_at": "2024-02-20T10:00:00Z",
    "updated_at": "2024-02-20T10:00:00Z",
    "metadata": {
      "content_type": "application/json",
      "version": 1,
      "attributes": {}
    }
  }
}
```

#### Update Object

```http
PUT /api/v1/storage/objects/{id}
```

Updates an existing object in storage.

##### Request Body

```json
{
  "data": {
    "key": "updated_value"
  },
  "metadata": {
    "attributes": {
      "updated": true
    }
  }
}
```

##### Response

```json
{
  "status": "success",
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "type": "object",
    "path": "/objects/example",
    "data": {
      "key": "updated_value"
    },
    "created_at": "2024-02-20T10:00:00Z",
    "updated_at": "2024-02-20T10:00:00Z",
    "metadata": {
      "content_type": "application/json",
      "version": 2,
      "attributes": {
        "updated": true
      }
    }
  }
}
```

### Batch Operations

#### Batch Upload

```http
POST /api/v1/storage/batch/upload
```

Uploads multiple files in a single request.

##### Request

```http
Content-Type: multipart/form-data

files[]: <file1>
files[]: <file2>
type: string
path: string
metadata: {
    "content_type": "string",
    "attributes": {}
}
```

##### Response

```json
{
  "status": "success",
  "data": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "type": "file",
      "path": "/uploads/file1.pdf",
      "size": 1024,
      "created_at": "2024-02-20T10:00:00Z",
      "updated_at": "2024-02-20T10:00:00Z",
      "metadata": {
        "content_type": "application/pdf",
        "version": 1,
        "attributes": {}
      }
    },
    {
      "id": "550e8400-e29b-41d4-a716-446655440001",
      "type": "file",
      "path": "/uploads/file2.pdf",
      "size": 2048,
      "created_at": "2024-02-20T10:00:00Z",
      "updated_at": "2024-02-20T10:00:00Z",
      "metadata": {
        "content_type": "application/pdf",
        "version": 1,
        "attributes": {}
      }
    }
  ]
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
# HELP storage_service_requests_total Total number of requests
# TYPE storage_service_requests_total counter
storage_service_requests_total{endpoint="/api/v1/storage/files",method="POST"} 100

# HELP storage_service_request_duration_seconds Request duration in seconds
# TYPE storage_service_request_duration_seconds histogram
storage_service_request_duration_seconds_bucket{endpoint="/api/v1/storage/files",method="POST",le="0.1"} 90
```

## Error Responses

All endpoints may return the following error responses:

### Validation Error

```json
{
  "type": "VALIDATION_ERROR",
  "message": "Invalid request data",
  "details": ["File size exceeds limit", "Invalid file type"]
}
```

### Not Found Error

```json
{
  "type": "NOT_FOUND_ERROR",
  "message": "File not found"
}
```

### Storage Full Error

```json
{
  "type": "STORAGE_FULL_ERROR",
  "message": "Storage quota exceeded"
}
```

### Storage Unavailable Error

```json
{
  "type": "STORAGE_UNAVAILABLE_ERROR",
  "message": "Storage service temporarily unavailable"
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

1. **Auth Service**

   - Purpose: Authentication and authorization
   - Operations:
     - Token validation
     - Permission checking
     - User context

2. **Cache Service**

   - Purpose: Performance optimization
   - Operations:
     - Data caching
     - Cache invalidation
     - Cache warming

3. **Monitoring Service**
   - Purpose: Metrics and monitoring
   - Operations:
     - Metrics collection
     - Performance monitoring
     - Health checks
     - Alerting

### External Services

1. **PostgreSQL**

   - Purpose: Metadata storage
   - Operations:
     - File metadata
     - Object metadata
     - Access logs

2. **MinIO**

   - Purpose: Object storage
   - Operations:
     - File storage
     - Object storage
     - Data backup

3. **Redis**
   - Purpose: Caching
   - Operations:
     - Cache storage
     - Rate limiting
     - Session management
