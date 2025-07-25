# Storage Service Interface Documentation

## Overview

The Storage Service provides data persistence capabilities for the microservices ecosystem, supporting both **synchronous HTTP/gRPC operations** and **asynchronous queue-based operations**. This service integrates with the Profile-Service → Queue-Service → Worker-Service ecosystem to enable reliable, scalable data storage operations.

## Integration Architecture

```
Profile Service → Queue Service → RabbitMQ → Storage Service Consumer
                                     ↓
                            storage.create
                            storage.update
                            storage.delete
                            storage.batch
                                     ↓
                         Storage Service (REST/gRPC + Queue)
```

## Service Interfaces

### 1. REST API Interface

**Base URL**: `http://storage-service:8080/api/v1`

#### Profile Management Endpoints

| Endpoint         | Method | Purpose                       | Authentication |
| ---------------- | ------ | ----------------------------- | -------------- |
| `/profiles`      | GET    | List profiles with pagination | Required       |
| `/profiles`      | POST   | Create new profile            | Required       |
| `/profiles/{id}` | GET    | Get profile by ID             | Required       |
| `/profiles/{id}` | PUT    | Update profile                | Required       |
| `/profiles/{id}` | DELETE | Delete profile                | Required       |

#### Batch Operations Endpoints

| Endpoint          | Method | Purpose                  | Authentication |
| ----------------- | ------ | ------------------------ | -------------- |
| `/profiles/batch` | POST   | Create multiple profiles | Required       |
| `/profiles/batch` | PUT    | Update multiple profiles | Required       |
| `/profiles/batch` | DELETE | Delete multiple profiles | Required       |

#### Health and Monitoring Endpoints

| Endpoint   | Method | Purpose                 | Authentication |
| ---------- | ------ | ----------------------- | -------------- |
| `/health`  | GET    | Service health check    | None           |
| `/ready`   | GET    | Service readiness check | None           |
| `/metrics` | GET    | Prometheus metrics      | None           |

### 2. gRPC Interface

**Service**: `ProfileService`  
**Port**: `50051`

#### gRPC Methods

```protobuf
service ProfileService {
    rpc CreateProfile(CreateProfileRequest) returns (ProfileResponse);
    rpc GetProfile(GetProfileRequest) returns (ProfileResponse);
    rpc UpdateProfile(UpdateProfileRequest) returns (ProfileResponse);
    rpc DeleteProfile(DeleteProfileRequest) returns (DeleteProfileResponse);
    rpc ListProfiles(ListProfilesRequest) returns (ListProfilesResponse);
    rpc BatchCreateProfiles(BatchCreateProfilesRequest) returns (BatchProfilesResponse);
    rpc BatchUpdateProfiles(BatchUpdateProfilesRequest) returns (BatchProfilesResponse);
    rpc BatchDeleteProfiles(BatchDeleteProfilesRequest) returns (BatchProfilesResponse);
}
```

### 3. Queue Consumer Interface

**Queue Configuration**:

- **Queue Name**: `storage-processing`
- **Exchange**: `tasks-exchange`
- **Routing Keys**: `storage.create`, `storage.update`, `storage.delete`, `storage.batch`
- **Message Format**: Standardized ecosystem message format

#### Supported Message Types

##### Single Operation Messages

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "type": "storage.profile.create",
  "payload": {
    "operation": "create",
    "profile_data": {
      "first_name": "John",
      "last_name": "Doe",
      "email": "john.doe@example.com",
      "phone": "+1234567890"
    }
  },
  "timestamp": "2023-12-01T10:30:00Z",
  "metadata": {
    "correlation_id": "req-123",
    "user_id": "user-456",
    "source_service": "profile-service"
  },
  "routing_key": "storage.create"
}
```

##### Batch Operation Messages

```json
{
    "id": "550e8400-e29b-41d4-a716-446655440001",
    "type": "storage.batch.mixed",
    "payload": {
        "batch_id": "batch-789",
        "operations": [
            {
                "operation": "create",
                "profile_data": {...}
            },
            {
                "operation": "update",
                "profile_id": "123",
                "profile_data": {...}
            },
            {
                "operation": "delete",
                "profile_id": "456"
            }
        ]
    },
    "timestamp": "2023-12-01T10:30:00Z",
    "metadata": {
        "correlation_id": "req-124",
        "batch_size": "3",
        "source_service": "worker-service"
    },
    "routing_key": "storage.batch"
}
```

## Message Format Specifications

### Standard Message Structure

```go
type Message struct {
    ID         string            `json:"id"`         // Unique message identifier
    Type       string            `json:"type"`       // Message type (storage.profile.create, etc.)
    Payload    json.RawMessage   `json:"payload"`    // Operation-specific data
    Timestamp  time.Time         `json:"timestamp"`  // Message creation timestamp
    Metadata   map[string]string `json:"metadata"`   // Additional context and headers
    RoutingKey string            `json:"routing_key"` // RabbitMQ routing key
}
```

### Storage Task Payload Structure

```go
type StorageTask struct {
    Operation   string            `json:"operation"`    // create, update, delete
    ProfileID   string            `json:"profile_id,omitempty"` // For update/delete operations
    ProfileData json.RawMessage   `json:"profile_data,omitempty"` // For create/update operations
    Options     map[string]interface{} `json:"options,omitempty"` // Additional options
}
```

### Batch Task Payload Structure

```go
type BatchStorageTask struct {
    BatchID    string             `json:"batch_id"`    // Unique batch identifier
    Operations []StorageOperation `json:"operations"`  // Array of operations
    Options    BatchOptions       `json:"options"`     // Batch-specific options
}

type StorageOperation struct {
    Operation   string          `json:"operation"`    // create, update, delete
    ProfileID   string          `json:"profile_id,omitempty"`
    ProfileData json.RawMessage `json:"profile_data,omitempty"`
}
```

## Request/Response Formats

### REST API Formats

#### Create Profile Request

```json
{
  "first_name": "John",
  "last_name": "Doe",
  "email": "john.doe@example.com",
  "phone": "+1234567890",
  "addresses": [
    {
      "street": "123 Main St",
      "city": "Anytown",
      "state": "CA",
      "country": "USA",
      "postal_code": "12345",
      "is_primary": true
    }
  ],
  "contacts": [
    {
      "type": "email",
      "value": "john.work@company.com",
      "is_primary": false
    }
  ]
}
```

#### Profile Response

```json
{
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "first_name": "John",
    "last_name": "Doe",
    "email": "john.doe@example.com",
    "phone": "+1234567890",
    "addresses": [...],
    "contacts": [...],
    "created_at": "2023-12-01T10:30:00Z",
    "updated_at": "2023-12-01T10:30:00Z"
}
```

#### Batch Request

```json
{
    "operations": [
        {
            "operation": "create",
            "profile_data": {...}
        },
        {
            "operation": "update",
            "profile_id": "123",
            "profile_data": {...}
        }
    ],
    "batch_options": {
        "fail_on_error": false,
        "return_results": true
    }
}
```

#### Batch Response

```json
{
  "batch_id": "batch-789",
  "total_operations": 2,
  "successful_operations": 2,
  "failed_operations": 0,
  "results": [
    {
      "operation": "create",
      "status": "success",
      "profile_id": "new-profile-id",
      "error": null
    },
    {
      "operation": "update",
      "status": "success",
      "profile_id": "123",
      "error": null
    }
  ],
  "processing_time_ms": 150
}
```

## Error Handling

### Standard Error Response Format

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Invalid request data",
    "details": {
      "field": "email",
      "reason": "Email already exists"
    },
    "correlation_id": "req-123",
    "timestamp": "2023-12-01T10:30:00Z"
  }
}
```

### Error Codes

| Code                    | Description                  | HTTP Status |
| ----------------------- | ---------------------------- | ----------- |
| `VALIDATION_ERROR`      | Request validation failed    | 400         |
| `NOT_FOUND`             | Profile not found            | 404         |
| `DUPLICATE_EMAIL`       | Email already exists         | 409         |
| `DATABASE_ERROR`        | Database operation failed    | 500         |
| `TIMEOUT_ERROR`         | Operation timed out          | 504         |
| `BATCH_PARTIAL_FAILURE` | Some batch operations failed | 207         |

### Queue Message Error Handling

For async operations, errors are handled through:

- **Message Rejection**: Invalid messages are rejected and sent to DLQ
- **Retry Logic**: Transient errors trigger retry with exponential backoff
- **Dead Letter Queue**: Failed messages after max retries are sent to `storage-processing-dlq`

## External Service Connections

### Consumed Services

#### Queue Service

- **Connection Type**: RabbitMQ Consumer
- **Queue**: `storage-processing`
- **Message Types**: Storage task messages
- **Acknowledgment**: Manual acknowledgment after successful processing

#### Database

- **Connection Type**: PostgreSQL
- **Usage**: Data persistence for all profile operations
- **Connection Pool**: 100 max connections, 20 idle connections
- **Transaction Support**: Full ACID transaction support

### Published Events

The storage-service can optionally publish completion events:

```json
{
  "id": "completion-event-id",
  "type": "storage.operation.completed",
  "payload": {
    "operation": "create",
    "profile_id": "550e8400-e29b-41d4-a716-446655440000",
    "status": "success",
    "processing_time_ms": 45
  },
  "timestamp": "2023-12-01T10:30:15Z",
  "metadata": {
    "correlation_id": "req-123",
    "original_message_id": "original-msg-id"
  },
  "routing_key": "storage.completed"
}
```

## Performance Characteristics

### Synchronous Operations

- **Average Response Time**: < 100ms
- **99th Percentile**: < 500ms
- **Throughput**: 1000+ requests/second
- **Concurrent Connections**: Up to 1000

### Asynchronous Operations

- **Processing Time**: < 5s per operation
- **Batch Processing**: < 30s for 100 operations
- **Queue Throughput**: 50+ messages/second
- **Queue Prefetch**: 5 messages per consumer

### Resource Limits

- **Request Size**: 1MB maximum
- **Batch Size**: 100 operations maximum
- **Connection Pool**: 100 max, 20 idle
- **Memory Usage**: ~512MB per instance

## Security Considerations

### Authentication

- **JWT Token Validation**: Required for all REST endpoints
- **Service-to-Service**: mTLS for internal communications
- **Queue Security**: AMQP authentication with dedicated user

### Authorization

- **Profile Access**: Users can only access their own profiles
- **Admin Operations**: Batch operations require admin privileges
- **Service Operations**: Queue operations use service-level permissions

### Data Protection

- **Encryption at Rest**: Database encryption enabled
- **Encryption in Transit**: TLS 1.3 for all communications
- **PII Handling**: Proper handling of personally identifiable information
- **Audit Logging**: All operations logged for compliance

## Monitoring and Observability

### Health Checks

- **Liveness**: Service is running and responsive
- **Readiness**: Service can handle requests (DB connected, queue connected)
- **Database Health**: Connection pool status and query performance
- **Queue Health**: Consumer status and message processing

### Metrics

#### REST API Metrics

```
storage_http_requests_total{method, endpoint, status}
storage_http_request_duration_seconds{method, endpoint}
storage_profile_operations_total{operation}
```

#### Queue Processing Metrics

```
storage_queue_messages_processed_total{operation, status}
storage_queue_processing_duration_seconds{operation}
storage_batch_operations_total{batch_size}
storage_dlq_messages_total
```

#### Database Metrics

```
storage_database_connections_active
storage_database_query_duration_seconds{operation}
storage_database_transactions_total{status}
```

### Distributed Tracing

- **Trace Propagation**: OpenTelemetry compatible
- **Span Creation**: For all major operations
- **Context Preservation**: Across sync and async operations

## Integration Testing Scenarios

### Profile Service Integration

1. **Direct HTTP Calls**: Profile service calls storage service REST API
2. **Async Task Submission**: Profile service submits storage tasks via queue service

### Queue Service Integration

1. **Message Consumption**: Storage service consumes messages from queue service
2. **Message Acknowledgment**: Proper ack/nack handling for all scenarios

### Worker Service Integration

1. **Worker Storage Operations**: Workers trigger storage operations as part of task processing
2. **Batch Operations**: Workers submit batch storage requests

### Error Scenarios

1. **Database Failures**: Proper error handling and recovery
2. **Queue Failures**: Message rejection and DLQ handling
3. **Validation Failures**: Proper error responses and logging

## Configuration Examples

### Environment Variables

```bash
# Database Configuration
DATABASE_URL=postgresql://user:pass@postgres:5432/profiles
DATABASE_MAX_CONNECTIONS=100
DATABASE_IDLE_CONNECTIONS=20

# RabbitMQ Configuration
RABBITMQ_URL=amqp://admin:password@rabbitmq:5672/
QUEUE_NAME=storage-processing
PREFETCH_COUNT=5
PROCESSING_TIMEOUT=30s

# Service Configuration
HTTP_PORT=8080
GRPC_PORT=50051
LOG_LEVEL=info
METRICS_PORT=9090
```

### Kubernetes ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: storage-service-config
data:
  HTTP_PORT: "8080"
  GRPC_PORT: "50051"
  QUEUE_NAME: "storage-processing"
  PREFETCH_COUNT: "5"
  PROCESSING_TIMEOUT: "30s"
  LOG_LEVEL: "info"
```

This interface documentation provides a comprehensive guide for integrating with the enhanced storage-service that supports both synchronous and asynchronous operations within the microservices task processing ecosystem.
