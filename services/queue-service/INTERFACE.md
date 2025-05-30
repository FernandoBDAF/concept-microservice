# Queue Service Interface

This document describes how the Queue Service connects with other services in the microservices architecture, including its public API endpoints and message queue interfaces.

## Service Overview

The Queue Service acts as a central message broker for asynchronous communication between services. It provides reliable message delivery, persistence, and dead letter queue handling.

## HTTP API Endpoints

### 1. Message Publishing

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

**Consumers:**

- Profile Service: Publishes profile updates
- Cache Service: Publishes cache invalidation events
- Background Job Service: Publishes job status updates

### 2. Message Status

```http
GET /api/v1/queue/status/{messageId}
```

**Consumers:**

- All services that need to track message processing status

### 3. Health Check

```http
GET /health
```

**Consumers:**

- Kubernetes health probes
- Service mesh health checks
- Monitoring systems

### 4. Metrics

```http
GET /metrics
```

**Consumers:**

- Prometheus
- Monitoring dashboards

## Message Queues

### 1. Profile Updates Queue

**Queue Name:** `profile_updates`
**TTL:** 24 hours
**DLQ:** `profile_updates.dlq`

**Publishers:**

- Profile Service: User profile changes
- Auth Service: User authentication events

**Consumers:**

- Cache Service: Updates user cache
- Notification Service: Sends profile update notifications

### 2. Cache Invalidation Queue

**Queue Name:** `cache_invalidation`
**TTL:** 1 hour
**DLQ:** `cache_invalidation.dlq`

**Publishers:**

- All services that modify cached data

**Consumers:**

- Cache Service: Invalidates cached entries

### 3. Background Jobs Queue

**Queue Name:** `background_jobs`
**TTL:** 7 days
**DLQ:** `background_jobs.dlq`

**Publishers:**

- Background Job Service: Job status updates
- Email Service: Email sending jobs
- Report Service: Report generation jobs

**Consumers:**

- Background Job Service: Processes jobs
- Notification Service: Sends job status notifications

## Message Types

### 1. Profile Updates

```json
{
  "type": "profile_update",
  "payload": {
    "user_id": "string",
    "changes": {
      "field": "value"
    }
  }
}
```

### 2. Cache Invalidation

```json
{
  "type": "cache_invalidation",
  "payload": {
    "cache_key": "string",
    "reason": "string"
  }
}
```

### 3. Background Jobs

```json
{
  "type": "background_job",
  "payload": {
    "job_id": "string",
    "status": "string",
    "result": "object"
  }
}
```

## Configuration

### Environment Variables

| Variable             | Description                            | Default |
| -------------------- | -------------------------------------- | ------- |
| SERVICE_PORT         | HTTP server port                       | 8080    |
| RABBITMQ_NODES       | Comma-separated list of RabbitMQ nodes | -       |
| RABBITMQ_USERNAME    | RabbitMQ username                      | guest   |
| RABBITMQ_PASSWORD    | RabbitMQ password                      | guest   |
| RABBITMQ_VHOST       | RabbitMQ virtual host                  | /       |
| RABBITMQ_MESSAGE_TTL | Message time-to-live duration          | 24h     |

## Security

### Authentication

- JWT-based authentication (TODO)
- Service-to-service mTLS (TODO)
- API key authentication (TODO)

### Authorization

- Role-based access control (TODO)
- Resource-level permissions (TODO)
- Rate limiting (TODO)

## Error Handling

### HTTP Errors

| Status Code | Description            |
| ----------- | ---------------------- |
| 400         | Invalid request format |
| 401         | Unauthorized           |
| 403         | Forbidden              |
| 404         | Resource not found     |
| 429         | Too many requests      |
| 500         | Internal server error  |

### Queue Errors

- Failed messages are moved to DLQ
- Messages are retried up to 3 times
- Messages expire after TTL
- DLQ messages are retained for 24 hours

## Monitoring

### Metrics

- Message publish rate
- Message consumption rate
- Message processing latency
- Error rates
- Queue sizes
- DLQ sizes
- Message TTL statistics

### Health Checks

- RabbitMQ connection status
- Queue health
- Service health
- Resource usage

## Dependencies

### External Services

- RabbitMQ Cluster
- Prometheus
- Service Mesh (optional)

### Internal Services

- Profile Service
- Cache Service
- Background Job Service
- Notification Service
- Auth Service
- Email Service
- Report Service
