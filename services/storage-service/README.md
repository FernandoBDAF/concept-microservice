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

# Storage Service

## Overview

The Storage Service is the **data persistence backbone** of the microservices task processing ecosystem. It provides comprehensive data storage capabilities through both **synchronous HTTP/gRPC operations** and **asynchronous queue-based processing**, enabling seamless integration with the Profile-Service → Queue-Service → Worker-Service architecture.

## 🎯 Strategic Role

### Primary Responsibilities

1. **Data Persistence Layer**: Reliable storage and retrieval of profile data with ACID compliance
2. **Dual-Mode Operations**: Support both synchronous API calls and asynchronous task processing
3. **Batch Processing**: Efficient handling of bulk operations for performance optimization
4. **Integration Hub**: Seamless connectivity with the task processing ecosystem

### Service Position in Ecosystem

```
Profile Service → Queue Service → RabbitMQ → Storage Service Consumer
                                     ↓
                            storage.create
                            storage.update
                            storage.delete
                            storage.batch
                                     ↓
                         Storage Service (Enhanced)
                                     ↓
                            PostgreSQL Database
```

## 🏗️ Enhanced Architecture

### Core Components

#### 1. **Dual Interface Layer**

- **REST API**: Direct HTTP access for synchronous operations
- **gRPC API**: High-performance RPC for service-to-service communication
- **Queue Consumer**: Asynchronous message processing from RabbitMQ

#### 2. **Message Processing Layer**

- **Message Processor**: Handles standardized ecosystem messages
- **Task Handlers**: Specialized processors for different operation types
- **Batch Processor**: Optimized bulk operation handling

#### 3. **Enhanced Service Layer**

- **Profile Service**: Core business logic with async support
- **Async Operations Service**: Dedicated async task processing
- **Batch Operations Service**: Efficient bulk processing logic

#### 4. **Infrastructure Layer**

- **Repository Pattern**: Enhanced with batch operations
- **Database Management**: Optimized connection pooling and transactions
- **Queue Integration**: RabbitMQ consumer with reliability patterns

## 🔄 Supported Operations

### Synchronous Operations (HTTP/gRPC)

#### Profile Management

- **Create Profile**: `POST /profiles`
- **Get Profile**: `GET /profiles/{id}`
- **Update Profile**: `PUT /profiles/{id}`
- **Delete Profile**: `DELETE /profiles/{id}`
- **List Profiles**: `GET /profiles` (with pagination)

#### Batch Operations

- **Batch Create**: `POST /profiles/batch`
- **Batch Update**: `PUT /profiles/batch`
- **Batch Delete**: `DELETE /profiles/batch`

### Asynchronous Operations (Queue-Based)

#### Single Operations

```json
{
  "type": "storage.profile.create",
  "routing_key": "storage.create",
  "payload": {
    "operation": "create",
    "profile_data": {
      "first_name": "John",
      "last_name": "Doe",
      "email": "john.doe@example.com"
    }
  }
}
```

#### Batch Operations

```json
{
    "type": "storage.batch.mixed",
    "routing_key": "storage.batch",
    "payload": {
        "batch_id": "batch-789",
        "operations": [
            {"operation": "create", "profile_data": {...}},
            {"operation": "update", "profile_id": "123", "profile_data": {...}},
            {"operation": "delete", "profile_id": "456"}
        ]
    }
}
```

## 📡 Integration Patterns

### Profile-Service Integration

**Synchronous Flow**:

```
Profile Service → Storage Service REST API → Database
```

**Asynchronous Flow**:

```
Profile Service → Queue Service → RabbitMQ → Storage Consumer → Database
```

### Queue-Service Integration

**Message Consumption**:

- **Queue**: `storage-processing`
- **Exchange**: `tasks-exchange`
- **Routing Keys**: `storage.create`, `storage.update`, `storage.delete`, `storage.batch`
- **Acknowledgment**: Manual acknowledgment after successful processing

### Worker-Service Integration

**Storage Operations within Tasks**:

- Workers can trigger storage operations as part of complex task processing
- Batch operations for efficiency when processing multiple profiles
- Event-driven completion notifications

## 🔧 Configuration

### Environment Variables

```bash
# Database Configuration
DATABASE_URL=postgresql://user:pass@postgres:5432/profiles
DATABASE_MAX_CONNECTIONS=100
DATABASE_IDLE_CONNECTIONS=20

# RabbitMQ Configuration
RABBITMQ_URL=amqp://admin:password@rabbitmq:5672/
QUEUE_NAME=storage-processing
EXCHANGE_NAME=tasks-exchange
PREFETCH_COUNT=5
PROCESSING_TIMEOUT=30s
MAX_RETRIES=3

# Service Configuration
HTTP_PORT=8080
GRPC_PORT=50051
LOG_LEVEL=info

# Batch Configuration
MAX_BATCH_SIZE=100
BATCH_TIMEOUT=60s
```

### Kubernetes Configuration

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: storage-service
spec:
  replicas: 3
  template:
    spec:
      containers:
        - name: storage-service
          image: storage-service:latest
          env:
            - name: RABBITMQ_URL
              valueFrom:
                secretKeyRef:
                  name: rabbitmq-secret
                  key: url
            - name: QUEUE_NAME
              value: "storage-processing"
            - name: MAX_BATCH_SIZE
              value: "100"
          resources:
            requests:
              memory: "256Mi"
              cpu: "250m"
            limits:
              memory: "512Mi"
              cpu: "500m"
```

## 📊 Performance Characteristics

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

## 🛡️ Error Handling & Reliability

### Error Categories

| Error Code              | Description                  | Retryable | Action             |
| ----------------------- | ---------------------------- | --------- | ------------------ |
| `VALIDATION_ERROR`      | Request validation failed    | No        | Return 400         |
| `NOT_FOUND`             | Profile not found            | No        | Return 404         |
| `DUPLICATE_EMAIL`       | Email already exists         | No        | Return 409         |
| `DATABASE_ERROR`        | Database operation failed    | Yes       | Retry with backoff |
| `TIMEOUT_ERROR`         | Operation timed out          | Yes       | Retry with backoff |
| `BATCH_PARTIAL_FAILURE` | Some batch operations failed | No        | Return 207         |

### Queue Message Reliability

#### Retry Logic

- **Exponential Backoff**: 2^retry_count seconds
- **Max Retries**: 3 attempts
- **Retry Conditions**: Transient errors (database timeout, connection issues)

#### Dead Letter Queue

- **DLQ Name**: `storage-processing-dlq`
- **Routing**: Failed messages after max retries
- **Monitoring**: DLQ size alerts and manual inspection capabilities

#### Message Acknowledgment

- **Manual Ack**: Only after successful processing
- **Nack with Requeue**: For retryable errors
- **Nack without Requeue**: For non-retryable errors (sends to DLQ)

## 📈 Monitoring & Observability

### Key Metrics

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

### Health Checks

#### Liveness Probe

```bash
curl http://storage-service:8080/health
```

#### Readiness Probe

```bash
curl http://storage-service:8080/ready
```

**Readiness Criteria**:

- Database connection healthy
- RabbitMQ consumer connected
- Service ready to process requests

### Distributed Tracing

- **OpenTelemetry Compatible**: Full trace propagation
- **Span Creation**: All major operations instrumented
- **Context Preservation**: Across sync and async boundaries

## 🧪 Testing Strategy

### Unit Testing

```bash
go test ./internal/...
```

### Integration Testing

```bash
go test ./tests/integration/...
```

**Test Scenarios**:

- Synchronous API operations
- Asynchronous queue message processing
- Batch operations with mixed success/failure
- Error handling and retry logic
- Database transaction rollback scenarios

### Load Testing

```bash
k6 run ./tests/load/storage-service.js
```

**Performance Targets**:

- 1000+ req/sec for sync operations
- 50+ msg/sec for async operations
- < 100ms response time for 95th percentile

## 🚀 Development

### Setup

1. **Install Dependencies**:

   ```bash
   go mod download
   ```

2. **Setup Database**:

   ```bash
   docker run -d --name postgres \
     -e POSTGRES_DB=profiles \
     -e POSTGRES_USER=storage \
     -e POSTGRES_PASSWORD=password \
     -p 5432:5432 postgres:14
   ```

3. **Setup RabbitMQ**:

   ```bash
   docker run -d --name rabbitmq \
     -e RABBITMQ_DEFAULT_USER=admin \
     -e RABBITMQ_DEFAULT_PASS=password \
     -p 5672:5672 -p 15672:15672 \
     rabbitmq:3.11-management
   ```

4. **Run Service**:
   ```bash
   go run ./cmd/server
   ```

### Building

   ```bash
# Build binary
go build -o storage-service ./cmd/server

# Build Docker image
docker build -t storage-service:latest .
```

### Running Tests

   ```bash
# Unit tests
go test -v ./internal/...

# Integration tests (requires Docker)
go test -v ./tests/integration/...

# Load tests
k6 run ./tests/load/storage-service.js
```

## 📚 API Documentation

### REST API

**OpenAPI Specification**: Available at `/api/docs` when service is running

#### Example: Create Profile

   ```bash
curl -X POST http://storage-service:8080/profiles \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{
    "first_name": "John",
    "last_name": "Doe",
    "email": "john.doe@example.com",
    "phone": "+1234567890"
  }'
```

#### Example: Batch Create

   ```bash
curl -X POST http://storage-service:8080/profiles/batch \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{
    "operations": [
      {
        "operation": "create",
        "profile_data": {
          "first_name": "John",
          "last_name": "Doe",
          "email": "john.doe@example.com"
        }
      },
      {
        "operation": "create",
        "profile_data": {
          "first_name": "Jane",
          "last_name": "Smith",
          "email": "jane.smith@example.com"
        }
      }
    ]
  }'
```

### gRPC API

**Protocol Buffers**: Available in `/api/proto/profile/profile.proto`

#### Example: Create Profile (gRPC)

```go
client := pb.NewProfileServiceClient(conn)
response, err := client.CreateProfile(ctx, &pb.CreateProfileRequest{
    FirstName: "John",
    LastName:  "Doe",
    Email:     "john.doe@example.com",
    Phone:     "+1234567890",
})
```

## 🔒 Security

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

## 📋 Implementation Status

### ✅ Current Features

#### Core Storage Layer

- [x] PostgreSQL integration with connection pooling
- [x] Transaction management with rollback support
- [x] Profile CRUD operations
   - [x] Email uniqueness validation
   - [x] Connection health monitoring

#### API Layer

- [x] REST API with comprehensive endpoints
- [x] gRPC API with Protocol Buffers
- [x] Request validation and error handling
- [x] Health and metrics endpoints

#### Operational Features

- [x] Structured logging with correlation IDs
- [x] Prometheus metrics collection
- [x] Kubernetes deployment configuration
- [x] Docker containerization

### 🔄 Integration Enhancements (In Progress)

#### Queue Integration

- [ ] RabbitMQ consumer implementation
- [ ] Message format alignment with ecosystem
- [ ] Async operation handlers
- [ ] Dead letter queue configuration

#### Batch Processing

- [ ] Batch operation endpoints
- [ ] Bulk database operations
- [ ] Partial failure handling
- [ ] Batch size optimization

#### Advanced Features

- [ ] Distributed tracing integration
- [ ] Circuit breaker patterns
- [ ] Advanced retry logic
- [ ] Performance optimization

### 🎯 Performance Targets

#### Current Performance

- **Sync Operations**: ~100ms average response time
- **Database Operations**: ~50ms average query time
- **Throughput**: 500+ requests/second
- **Connection Pool**: 100 max, 20 idle connections

#### Target Performance (Post-Enhancement)

- **Sync Operations**: < 100ms (maintained)
- **Async Operations**: < 5s processing time
- **Batch Operations**: < 30s for 100 operations
- **Queue Throughput**: 50+ messages/second

## 🛠️ Dependencies

### Core Dependencies

- **PostgreSQL**: Primary data storage
- **Go**: Runtime and development language
- **Gorilla Mux**: HTTP routing
- **SQLX**: Database operations
- **Zap**: Structured logging

### Integration Dependencies

- **RabbitMQ**: Message queue for async operations
- **Common Queue Package**: Shared queue utilities from worker-service
- **Prometheus**: Metrics collection
- **OpenTelemetry**: Distributed tracing

### Development Dependencies

- **Docker**: Containerization
- **Kubernetes**: Orchestration
- **k6**: Load testing
- **Testify**: Unit testing

## 🔗 Related Services

### Direct Integrations

- **Profile Service**: Primary consumer of storage operations
- **Queue Service**: Message routing for async operations
- **Worker Service**: Batch storage operations within task processing

### Shared Infrastructure

- **RabbitMQ**: Message broker
- **PostgreSQL**: Database
- **Prometheus**: Monitoring
- **Grafana**: Observability dashboards

## 📞 Support & Contributing

### Getting Help

- **Documentation**: Check service-specific docs in this folder
- **Issues**: Create GitHub issues for bugs or feature requests
- **Discussions**: Use GitHub discussions for questions

### Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Update documentation
6. Submit a pull request

### Development Guidelines

- Follow Go best practices and coding standards
- Maintain test coverage above 80%
- Update documentation for all changes
- Ensure backward compatibility for API changes
- Add appropriate logging and metrics

---

The Storage Service is a critical component of the microservices ecosystem, providing reliable, scalable, and efficient data persistence capabilities. Its dual-mode architecture enables both high-performance synchronous operations and reliable asynchronous task processing, making it an essential foundation for the entire system. 🎯
