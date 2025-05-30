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

## Prerequisites

- Go 1.21+
- RabbitMQ 3.12+
- Docker (optional)

## Configuration

The service can be configured using environment variables:

| Variable        | Description                            | Default        |
| --------------- | -------------------------------------- | -------------- |
| SERVICE_PORT    | HTTP server port                       | 8080           |
| SERVICE_ENV     | Environment (development/production)   | development    |
| RABBITMQ_NODES  | Comma-separated list of RabbitMQ nodes | localhost:5672 |
| LOG_LEVEL       | Logging level                          | info           |
| LOG_FORMAT      | Log format (json/text)                 | json           |
| METRICS_ENABLED | Enable Prometheus metrics              | true           |
| METRICS_PORT    | Prometheus metrics port                | 9090           |

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

## Error Handling

The service implements comprehensive error handling:

- Input validation
- Message format validation
- RabbitMQ connection errors
- Message processing errors
- Graceful degradation

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

3. **Performance Issues**
   - Monitor message throughput
   - Check resource usage
   - Verify queue sizes

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

### Best Practices

1. **Configuration**

   - Use secrets management
   - Encrypt sensitive data
   - Regular key rotation

2. **Network**

   - Use internal networks
   - Implement firewalls
   - Monitor access

3. **Monitoring**
   - Track security events
   - Monitor access patterns
   - Alert on anomalies

## License

MIT
