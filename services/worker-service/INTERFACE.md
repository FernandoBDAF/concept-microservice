# Worker Service Interface Documentation

## Service Interface Overview

The Worker Service operates as a **message consumer** in the microservices architecture, implementing asynchronous task processing through RabbitMQ message queues. It provides operational HTTP endpoints for health monitoring while consuming messages from queues populated by the Queue Service.

## External Service Connections

### 1. RabbitMQ Message Broker

**Connection Type**: Direct AMQP Consumer
**Purpose**: Primary message consumption interface

#### Connection Configuration

```go
// Environment-based connection
URL: amqp://{RABBITMQ_USER}:{RABBITMQ_PASSWORD}@{RABBITMQ_HOST}:{RABBITMQ_PORT}/
```

#### Queue Specifications

| Property           | Value                | Purpose                             |
| ------------------ | -------------------- | ----------------------------------- |
| **Exchange**       | `profile-tasks`      | Direct exchange for task routing    |
| **Queue**          | `profile-processing` | Main processing queue               |
| **Routing Key**    | `profile.task`       | Message routing identifier          |
| **Durability**     | `true`               | Persist messages to disk            |
| **Auto-Delete**    | `false`              | Queue survives consumer disconnects |
| **Prefetch Count** | `1`                  | Process one message at a time       |

#### Dead Letter Queue Configuration

| Property         | Value                    | Purpose                 |
| ---------------- | ------------------------ | ----------------------- |
| **DLX Exchange** | `profile-tasks.dlx`      | Dead letter exchange    |
| **DLQ Queue**    | `profile-processing.dlq` | Failed message storage  |
| **Message TTL**  | `24 hours`               | Message expiration time |

### 2. Queue Service (Indirect)

**Connection Type**: Indirect via RabbitMQ
**Purpose**: Receives messages published by Queue Service

#### Message Flow

```
Queue Service → RabbitMQ Exchange → Worker Service Queue → Worker Service
```

The Worker Service does not directly communicate with the Queue Service but consumes messages that the Queue Service publishes to RabbitMQ.

## Message Consumption Interface

### Message Format

The Worker Service consumes messages conforming to the common queue message structure:

```json
{
  "id": "string",
  "type": "string",
  "payload": "json_object",
  "timestamp": "RFC3339_timestamp",
  "metadata": {
    "key": "value"
  }
}
```

### Domain Message Structure

Messages are transformed into domain-specific `ProfileMessage` format:

```go
type ProfileMessage struct {
    queue.Message                    // Embedded common message
    ProfileID string    `json:"profile_id"`  // Target profile identifier
    Action    string    `json:"action"`      // Operation type (update/delete)
    Data      any       `json:"data"`        // Action-specific payload
    CreatedAt time.Time `json:"created_at"`  // Message creation timestamp
}
```

### Supported Message Types

| Message Type     | Action   | Processing Logic                       |
| ---------------- | -------- | -------------------------------------- |
| `profile_update` | `update` | Simulated profile update (10s delay)   |
| `profile_delete` | `delete` | Simulated profile deletion (10s delay) |

### Message Processing Contract

#### Validation Rules

- `ProfileID` must be non-empty string
- `Action` must be either "update" or "delete"
- `Data` field must be present (not null)

#### Processing Outcomes

- **Success**: Message acknowledged, metrics incremented
- **Validation Failure**: Message rejected (not requeued)
- **Processing Error**: Message rejected with requeue
- **Unknown Action**: Message rejected (not requeued)

## HTTP Operational Interface

### Health Check Endpoint

**Endpoint**: `GET /health`
**Port**: `8080`
**Purpose**: Kubernetes readiness/liveness probes

#### Response Format

**Healthy State** (200 OK):

```json
{
  "status": "ok",
  "ready": true
}
```

**Unhealthy State** (503 Service Unavailable):

```json
{
  "status": "unhealthy",
  "ready": false
}
```

#### Health Determination Logic

- **Ready**: Consumer is connected and processing messages
- **Not Ready**: Consumer connection failed or service shutting down

### Metrics Endpoint

**Endpoint**: `GET /metrics`
**Port**: `8080` (same as health)
**Format**: Prometheus metrics format

#### Available Metrics

##### Consumer Metrics

```prometheus
# Message consumption latency
worker_consume_latency_seconds{} histogram

# Total consumption errors
worker_consume_errors_total{} counter

# Age of messages when consumed
worker_message_age_seconds{} histogram
```

##### Processor Metrics

```prometheus
# Processing execution time
profile_processing_time_seconds{} histogram

# Processing error count
profile_processing_errors_total{} counter

# Successful processing count
profile_processing_success_total{} counter
```

## Service Dependencies

### Required Services

| Service           | Type           | Purpose          | Failure Impact         |
| ----------------- | -------------- | ---------------- | ---------------------- |
| **RabbitMQ**      | Message Broker | Message delivery | Service cannot start   |
| **Queue Service** | Publisher      | Message source   | No messages to process |

### Optional Dependencies

| Service            | Type          | Purpose            | Failure Impact      |
| ------------------ | ------------- | ------------------ | ------------------- |
| **Prometheus**     | Monitoring    | Metrics collection | Metrics unavailable |
| **Log Aggregator** | Observability | Log collection     | Logs only local     |

## Integration Patterns

### Consumer Pattern Implementation

```go
// Consumer lifecycle management
consumer.Start(ctx, handler) // Start consuming with context
consumer.Close()             // Graceful shutdown
```

### Message Acknowledgment Pattern

```go
// Processing success
delivery.Ack(false) // Acknowledge message

// Processing failure (requeue)
delivery.Nack(false, true) // Reject with requeue

// Validation failure (no requeue)
delivery.Nack(false, false) // Reject without requeue
```

### Error Handling Pattern

```go
// Automatic reconnection on connection loss
if connection.IsClosed() {
    reconnect() // Automatic retry with backoff
}

// Message processing error handling
if err := processor.Process(msg); err != nil {
    logError(err)
    incrementErrorMetrics()
    return err // Triggers message requeue
}
```

## Scaling and Load Balancing

### Horizontal Scaling

- **Pattern**: Multiple consumer instances
- **Load Distribution**: RabbitMQ round-robin delivery
- **Coordination**: No inter-instance communication required
- **State**: Stateless processing enables easy scaling

### Prefetch Configuration

```go
// Controls message delivery rate
PrefetchCount: 1  // Process one message at a time
```

**Tradeoffs**:

- **Low Prefetch**: Better load distribution, slower throughput
- **High Prefetch**: Higher throughput, uneven load distribution

## Connection Management

### Connection Resilience

#### Automatic Reconnection

- Connection monitoring via AMQP connection events
- Exponential backoff retry strategy
- Graceful degradation during outages

#### Connection Configuration

```go
amqp.Config{
    Heartbeat: 10 * time.Second,  // Connection keepalive
    Locale:    "en_US",           // AMQP locale
}
```

### Channel Management

- Single channel per consumer instance
- Channel recreation on connection recovery
- QoS settings applied per channel

## Security Considerations

### Authentication

- **Method**: AMQP username/password authentication
- **Credentials**: Environment variables (`RABBITMQ_USER`, `RABBITMQ_PASSWORD`)
- **Transport**: Plain AMQP (consider AMQPS for production)

### Authorization

- **Queue Access**: Consumer permissions on designated queues
- **Exchange Access**: Binding permissions for message routing
- **Management**: No administrative permissions required

### Network Security

- **Internal Communication**: Cluster-internal RabbitMQ access
- **External Exposure**: Health endpoint only (port 8080)
- **Container Security**: Non-root user execution

## Monitoring and Observability

### Log Integration

#### Structured Logging Fields

```json
{
  "level": "info|error|debug",
  "timestamp": "RFC3339",
  "message": "description",
  "queue": "profile-processing",
  "message_id": "uuid",
  "processing_time": "duration",
  "error": "error_details"
}
```

#### Key Log Events

- Consumer startup/shutdown
- Message processing start/completion
- Connection events (connect/disconnect/reconnect)
- Error conditions and recovery

### Metrics Integration

#### Prometheus Configuration

```yaml
# Scrape configuration
- job_name: "worker-service"
  static_configs:
    - targets: ["worker-service:8080"]
  metrics_path: "/metrics"
  scrape_interval: 15s
```

#### Alert Conditions

- High error rate: `rate(profile_processing_errors_total[5m]) > 0.1`
- Processing delays: `histogram_quantile(0.95, profile_processing_time_seconds) > 30`
- Consumer disconnections: `up{job="worker-service"} == 0`

## Development and Testing Interfaces

### Local Development

#### Mock Queue Setup

```bash
# RabbitMQ via Docker
docker run -d --name rabbitmq \
  -p 5672:5672 -p 15672:15672 \
  rabbitmq:3-management
```

#### Environment Configuration

```bash
export RABBITMQ_USER=guest
export RABBITMQ_PASSWORD=guest
export RABBITMQ_HOST=localhost
export RABBITMQ_PORT=5672
```

### Testing Interfaces

#### Message Injection

```bash
# Direct queue message publishing for testing
rabbitmqadmin publish exchange=profile-tasks \
  routing_key=profile.task \
  payload='{"profile_id":"test","action":"update","data":{}}'
```

#### Health Check Testing

```bash
# Verify service health
curl http://localhost:8080/health

# Check metrics
curl http://localhost:8080/metrics
```

## Interface Evolution and Versioning

### Message Format Compatibility

#### Current Version Support

- Common queue message format v1
- ProfileMessage domain model v1

#### Future Considerations

- Message schema versioning strategy
- Backward compatibility requirements
- Migration path for format changes

### API Stability

#### Stable Interfaces

- Health check endpoint contract
- Basic metrics format
- Message acknowledgment patterns

#### Evolving Interfaces

- Processor implementations (business logic)
- Metrics collection (additional metrics)
- Configuration options (new parameters)

## Troubleshooting Guide

### Common Connection Issues

#### RabbitMQ Connection Failures

```
Error: "connection refused"
Check: RabbitMQ service availability, network connectivity
Solution: Verify RABBITMQ_HOST and port configuration
```

#### Authentication Failures

```
Error: "access refused"
Check: Username/password credentials
Solution: Verify RABBITMQ_USER and RABBITMQ_PASSWORD
```

### Message Processing Issues

#### Messages Not Being Consumed

```
Check: Queue existence, binding configuration
Debug: RabbitMQ management UI queue status
Solution: Verify exchange/queue/binding setup
```

#### High Error Rates

```
Check: Message format validation, processor logic
Debug: Application logs for error details
Solution: Review message structure and processing code
```

### Performance Issues

#### Slow Processing

```
Symptom: High processing_time_seconds metrics
Cause: 10-second simulation delay in processor
Solution: Optimize business logic implementation
```

#### Memory Issues

```
Symptom: Container OOM kills
Cause: Message accumulation, connection leaks
Solution: Monitor prefetch settings, connection cleanup
```
