# Queue Service

A microservice that provides reliable message queuing capabilities using RabbitMQ as the message broker. This service is part of a larger microservices architecture and handles asynchronous message processing between services.

## Architecture

The service follows clean architecture principles with the following layers:

- Domain Layer: Core business logic and models
- Application Layer: Use cases and service implementations
- Infrastructure Layer: External adapters (RabbitMQ, HTTP)

### Key Components

- **Message Broker**: RabbitMQ for reliable message delivery
- **HTTP API**: RESTful endpoints for message operations
- **Metrics**: Prometheus integration for monitoring
- **Health Checks**: Endpoint for service health monitoring
- **Configuration**: Environment-based configuration management

## Features

- Message publishing and consumption
- Multiple message types support:
  - Profile updates
  - Cache invalidation
  - Background jobs
- Message status tracking
- Prometheus metrics
- Health checks
- Graceful shutdown
- RabbitMQ cluster support
- Message persistence
- Priority queues
- Dead letter queues
- Message TTL (Time-To-Live)

## Prerequisites

- Go 1.21+
- RabbitMQ 3.12+
- Docker (optional)

## Configuration

The service can be configured using environment variables:

| Variable             | Description                            | Default        |
| -------------------- | -------------------------------------- | -------------- |
| SERVICE_PORT         | HTTP server port                       | 8080           |
| SERVICE_ENV          | Environment (development/production)   | development    |
| RABBITMQ_NODES       | Comma-separated list of RabbitMQ nodes | localhost:5672 |
| RABBITMQ_USERNAME    | RabbitMQ username                      | guest          |
| RABBITMQ_PASSWORD    | RabbitMQ password                      | guest          |
| RABBITMQ_VHOST       | RabbitMQ virtual host                  | /              |
| RABBITMQ_MESSAGE_TTL | Message time-to-live duration          | 24h            |
| LOG_LEVEL            | Logging level                          | info           |
| LOG_FORMAT           | Log format (json/text)                 | json           |
| METRICS_ENABLED      | Enable Prometheus metrics              | true           |
| METRICS_PORT         | Prometheus metrics port                | 9090           |

### Message Persistence

Messages are persisted to disk by default, ensuring they survive RabbitMQ restarts and node failures. This is achieved through:

- Durable queues
- Persistent message delivery mode
- Message acknowledgments
- Dead letter queues for failed messages

### Dead Letter Queues

The service automatically creates dead letter queues (DLQ) for each queue. Failed messages are moved to the DLQ when:

- Message processing fails
- Message is rejected
- Message TTL expires

To inspect messages in the DLQ:

```bash
# List all queues including DLQs
rabbitmqctl list_queues name messages_ready messages_unacknowledged

# Get messages from a DLQ
rabbitmqctl get_queue queue_name.dlq
```

### Message TTL

Messages have a configurable time-to-live (TTL). When a message expires:

1. It is automatically moved to the DLQ
2. The original message is removed from the queue
3. The message can be inspected in the DLQ

Configure TTL using the `RABBITMQ_MESSAGE_TTL` environment variable:

```bash
# Set TTL to 1 hour
export RABBITMQ_MESSAGE_TTL="1h"

# Set TTL to 30 minutes
export RABBITMQ_MESSAGE_TTL="30m"

# Set TTL to 7 days
export RABBITMQ_MESSAGE_TTL="168h"
```

## API Endpoints

### Publish Message

```http
POST /api/v1/queue/messages
Content-Type: application/json

{
  "type": "profile_update",
  "payload": {
    "user_id": "123",
    "changes": {
      "name": "John Doe"
    }
  },
  "headers": {
    "correlation_id": "123e4567-e89b-12d3-a456-426614174000",
    "priority": 1
  }
}
```

### Get Message Status

```http
GET /api/v1/queue/status/{messageId}
```

### Health Check

```http
GET /health
```

### Metrics

```http
GET /metrics
```

## API Documentation

### OpenAPI Specification

```yaml
openapi: 3.0.0
info:
  title: Queue Service API
  version: 1.0.0
paths:
  /api/v1/queue/messages:
    post:
      summary: Publish a new message
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/QueueMessage"
      responses:
        "202":
          description: Message accepted
        "400":
          description: Invalid message format
        "500":
          description: Internal server error
  /api/v1/queue/status/{messageId}:
    get:
      summary: Get message status
      parameters:
        - name: messageId
          in: path
          required: true
          schema:
            type: string
      responses:
        "200":
          description: Message status
        "404":
          description: Message not found
        "500":
          description: Internal server error
components:
  schemas:
    QueueMessage:
      type: object
      required:
        - type
        - payload
      properties:
        type:
          type: string
          enum: [profile_update, cache_invalidation, background_job]
        payload:
          type: object
        headers:
          type: object
          properties:
            correlation_id:
              type: string
              format: uuid
            priority:
              type: integer
              minimum: 0
              maximum: 9
```

## Development

1. Clone the repository
2. Install dependencies:
   ```bash
   go mod download
   ```
3. Run the service:
   ```bash
   go run cmd/main.go
   ```

## Docker

Build the Docker image:

```bash
docker build -t queue-service .
```

Run the container:

```bash
docker run -p 8080:8080 -p 9090:9090 queue-service
```

## Testing

Run the tests:

```bash
go test ./...
```

## Monitoring

The service exposes Prometheus metrics at `/metrics`. Key metrics include:

- Message publish rate
- Message consumption rate
- Message processing latency
- Error rates
- Queue sizes
- DLQ sizes
- Message TTL statistics

## Error Handling

The service implements comprehensive error handling:

- Input validation
- Message format validation
- RabbitMQ connection errors
- Message processing errors
- Graceful degradation
- Dead letter queue handling
- Message TTL management

## Security

- Environment-based configuration
- No sensitive data in messages
- Input validation
- Rate limiting (TODO)
- Authentication (TODO)

## Deployment Guide

### Prerequisites

- Docker 20.10+
- Kubernetes 1.21+ (optional)
- RabbitMQ 3.12+

### Environment Setup

1. Set required environment variables:

   ```bash
   export SERVICE_PORT=8080
   export SERVICE_ENV=production
   export RABBITMQ_NODES=rabbitmq-1:5672,rabbitmq-2:5672
   export RABBITMQ_USERNAME=queue_user
   export RABBITMQ_PASSWORD=secure_password
   export RABBITMQ_VHOST=/queue
   export RABBITMQ_MESSAGE_TTL=24h
   export LOG_LEVEL=info
   export LOG_FORMAT=json
   export METRICS_ENABLED=true
   export METRICS_PORT=9090
   ```

2. Build and run with Docker:

   ```bash
   docker build -t queue-service .
   docker run -p 8080:8080 -p 9090:9090 queue-service
   ```

3. Kubernetes deployment (optional):
   ```bash
   kubectl apply -f k8s/
   ```

### Monitoring Setup

1. Configure Prometheus:

   ```yaml
   scrape_configs:
     - job_name: "queue-service"
       static_configs:
         - targets: ["queue-service:9090"]
   ```

2. Set up Grafana dashboards:
   - Message throughput
   - Error rates
   - Processing latency
   - Queue sizes
   - DLQ sizes
   - Message TTL statistics

## Troubleshooting Guide

### Common Issues

1. **RabbitMQ Connection Issues**

   - Check RabbitMQ cluster status
   - Verify connection credentials
   - Check network connectivity

2. **Message Processing Errors**

   - Check message format
   - Verify queue configuration
   - Check consumer status
   - Inspect DLQ for failed messages

3. **Performance Issues**

   - Monitor message throughput
   - Check resource usage
   - Verify queue sizes
   - Monitor DLQ sizes

4. **Message TTL Issues**
   - Verify TTL configuration
   - Check DLQ for expired messages
   - Monitor message age metrics

### Logging

- Application logs: `docker logs queue-service`
- RabbitMQ logs: `docker logs rabbitmq`
- Metrics: `curl localhost:9090/metrics`

## Security Considerations

### Authentication

- JWT-based authentication
- Service-to-service mTLS
- API key authentication (optional)

### Authorization

- Role-based access control
- Resource-level permissions
- Rate limiting

### Data Security

- Message encryption
- Secure connections (TLS)
- Audit logging
- Message persistence security

### Best Practices

1. **Configuration**

   - Use secrets management
   - Encrypt sensitive data
   - Regular key rotation
   - Configure appropriate TTL values

2. **Network**

   - Use internal networks
   - Implement firewalls
   - Monitor access

3. **Monitoring**
   - Track security events
   - Monitor access patterns
   - Alert on anomalies
   - Monitor DLQ sizes

## License

MIT
