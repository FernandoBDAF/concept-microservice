# Implementation Request

## Task Context

Task: Implement Queue-Based Task Processing System
Priority: High
Effort: Medium
Status: In Progress
Dependencies: None

## Documentation References

1. README.md (Queue Service)

   - Section: API Documentation
   - Purpose: Understand queue service endpoints
   - Impact: Guide implementation of queue interactions

2. README.md (Profile Service)

   - Section: API Documentation
   - Purpose: Understand profile service structure
   - Impact: Guide implementation of new endpoints

3. README.md (Worker Service)
   - Section: Service Architecture
   - Purpose: Understand worker service requirements
   - Impact: Guide implementation of worker logic

## Requirements

1. Queue Service Review (Completed)

   - [x] Review existing queue service implementation
   - [x] Identify required endpoints for task submission
   - [x] Document message format and requirements

2. Profile Service Implementation (Completed)

   - [x] Create new endpoint for task submission
   - [x] Implement queue service client
   - [x] Add task submission logic
   - [x] Add proper error handling
   - [x] Add request validation

3. Worker Service Implementation (Completed)

   - [x] Create main.go file
   - [x] Implement queue consumer
   - [x] Add random number generation (1-100)
   - [x] Add 10-second delay
   - [x] Implement response message handling
   - [x] Add proper error handling
   - [x] Add graceful shutdown
   - [x] Add reconnection logic
   - [x] Implement structured logging

## Communication Flow

### New Architecture

```
Profile Service -> Queue Service -> RabbitMQ
Worker Service -> Queue Service -> RabbitMQ
```

### Service Responsibilities

1. Queue Service

   - Provides HTTP endpoints for message operations
   - Manages RabbitMQ connection internally
   - Handles message persistence and delivery
   - Provides message status tracking
   - Endpoints:
     - POST /api/v1/queue/messages - Publish messages
     - GET /api/v1/queue/messages - Consume messages
     - POST /api/v1/queue/messages/{id}/ack - Acknowledge messages
     - POST /api/v1/queue/messages/{id}/reject - Reject messages
     - GET /api/v1/queue/status/{id} - Get message status

2. Profile Service

   - Uses HTTP client to communicate with queue-service
   - No direct RabbitMQ connection
   - Configuration:
     - QUEUE_SERVICE_URL: http://queue-service:80
     - QUEUE_TIMEOUT_MS: 5000
     - QUEUE_RETRIES: 3
     - QUEUE_NAME: profile_queue

3. Worker Service
   - Uses HTTP client to communicate with queue-service
   - No direct RabbitMQ connection
   - Implements polling for message consumption
   - Configuration:
     - QUEUE_SERVICE_URL: http://queue-service:80
     - QUEUE_TIMEOUT_MS: 5000
     - QUEUE_RETRIES: 3
     - QUEUE_NAME: profile_tasks

## Implementation Details

### Queue Service Implementation

1. Available Endpoints:

   - POST /api/v1/queue/messages - Publish new messages
   - GET /api/v1/queue/messages - Consume messages
   - POST /api/v1/queue/messages/{id}/ack - Acknowledge messages
   - POST /api/v1/queue/messages/{id}/reject - Reject messages
   - GET /api/v1/queue/status/{id} - Get message status
   - GET /health - Health check
   - GET /metrics - Prometheus metrics

2. Message Format:

```json
{
  "type": "profile_update",
  "payload": {
    "user_id": "string",
    "changes": {
      // Dynamic payload based on message type
    }
  }
}
```

### Profile Service Implementation

1. Queue Client Implementation:

   - HTTP-based client for queue-service communication
   - Retry mechanism for failed requests
   - Error handling and logging
   - Configuration through environment variables

2. Task Submission Endpoint:
   - POST /api/v1/profiles/:id/tasks
   - Validates task request
   - Checks profile existence
   - Publishes message to queue-service
   - Returns task ID and status

### Worker Service Implementation

1. Queue Client Implementation:

   - HTTP-based client for queue-service communication
   - Polling mechanism for message consumption
   - Message acknowledgment and rejection
   - Error handling and retry logic

2. Message Processing:
   - Polls queue-service for new messages
   - Processes messages with random delay
   - Acknowledges successful processing
   - Rejects failed messages with requeue option

## Verification Requirements

### Queue Service

- [x] Endpoints are properly documented
- [x] Message format is clear
- [x] Integration requirements are complete
- [x] RabbitMQ connection is working
- [x] Message persistence is working

### Profile Service

- [x] Endpoint is working
- [x] Queue client is working
- [x] Error handling is working
- [x] Validation is working
- [x] Logging is working
- [x] Configuration is correct

### Worker Service

- [x] Consumer is working
- [x] Random number generation is working
- [x] Delay is working
- [x] Response handling is working
- [x] Error handling is working
- [x] Logging is working
- [x] Configuration is correct

## Deployment Requirements

1. Kubernetes Configuration:

   - Queue Service:
     - Service name: queue-service
     - Port: 80
     - RabbitMQ connection details
   - Profile Service:
     - Service name: profile-service
     - Queue service URL: http://queue-service:80
   - Worker Service:
     - Service name: worker-service
     - Queue service URL: http://queue-service:80

2. Environment Variables:
   - Queue Service:
     - RABBITMQ_NODES
     - RABBITMQ_USERNAME
     - RABBITMQ_PASSWORD
   - Profile Service:
     - QUEUE_SERVICE_URL
     - QUEUE_TIMEOUT_MS
     - QUEUE_RETRIES
     - QUEUE_NAME
   - Worker Service:
     - QUEUE_SERVICE_URL
     - QUEUE_TIMEOUT_MS
     - QUEUE_RETRIES
     - QUEUE_NAME

## Success Criteria

1. All services are properly integrated
2. Messages are processed reliably
3. System handles errors gracefully
4. Performance meets requirements
5. Monitoring and observability in place
6. Documentation is complete and accurate

## Next Steps

1. Testing and Validation

   - [ ] Unit tests for all services
   - [ ] Integration tests
   - [ ] End-to-end tests
   - [ ] Load testing
   - [ ] Error handling tests

2. Monitoring and Observability

   - [x] Configure logging aggregation
   - [ ] Add Prometheus metrics
   - [ ] Set up alerting rules
   - [ ] Create dashboards

3. Deployment

   - [ ] Create Kubernetes manifests
   - [ ] Configure resource limits
   - [ ] Set up health checks
   - [ ] Configure autoscaling

4. Documentation
   - [ ] Update service READMEs
   - [ ] Document deployment process
   - [ ] Add troubleshooting guide
   - [ ] Create runbook

## Dependencies

### External Dependencies

- [x] RabbitMQ server
- [x] Protocol Buffers
- [x] AMQP client
- [ ] Monitoring tools
- [x] Logging system (zap)

### Internal Dependencies

- [x] Profile Storage service
- [x] Auth service
- [ ] Monitoring service
- [x] Logging system (common/logging package)

## Logging Implementation Details

### Structured Logging Features

- [x] Service name identification
- [x] Message ID tracking
- [x] Error context preservation
- [x] Operation timing information
- [x] Log levels (Info, Debug, Error, Fatal)
- [x] Structured fields for better analysis

### Log Categories

- [x] Service lifecycle (startup/shutdown)
- [x] Message processing lifecycle
- [x] Error handling and recovery
- [x] Reconnection attempts
- [x] Task completion status

### Log Fields

- [x] message_id: For message tracking
- [x] message_type: For message categorization
- [x] error: For error context
- [x] queue: For queue identification
- [x] attempt: For retry tracking
- [x] max_attempts: For retry limits
- [x] delay_seconds: For timing information
- [x] response: For task results

## Error Handling Implementation Details

### Error Types

- [x] Queue Connection Errors

  - Type: queue_connection_error
  - Description: Failed to connect to queue service
  - Context: Connection attempts, reconnection logic

- [x] Message Processing Errors

  - Type: message_processing_error
  - Description: Failed to process message
  - Context: Message processing, response generation

- [x] Message Acknowledgment Errors

  - Type: message_acknowledgment_error
  - Description: Failed to acknowledge message
  - Context: Message acknowledgment after processing

- [x] Message Rejection Errors
  - Type: message_rejection_error
  - Description: Failed to reject message
  - Context: Message rejection on processing failure

### Error Handling Features

- [x] Custom error types for different scenarios
- [x] Error wrapping for context preservation
- [x] Structured error logging
- [x] Error recovery mechanisms
- [x] Graceful error handling in message processing
- [x] Error context in logs

### Error Recovery Strategies

- [x] Automatic reconnection for queue connection failures
- [x] Message requeuing on processing failures
- [x] Graceful shutdown handling
- [x] Context cancellation handling
- [x] Error propagation with context

1. What the queue-service expects (from queue-service-api-test.py):
   Endpoint:
   POST /api/v1/queue/messages
   (with JSON body containing fields: id, type, timestamp, payload, priority, headers, etc.)
   Message structure:
   id (string, required)
   type (string, required, e.g. "profile_update")
   timestamp (string, required)
   payload (object, required)
   priority (int, required)
   headers (object, should include at least correlation_id)
   Service URL:
   http://queue-service:80 (inside the cluster)
2. What to check in profile-api configuration:
   a. Service Discovery
   Is the profile-api using the correct internal DNS name for the queue-service?
   It should be http://queue-service or http://queue-service:80.
   b. Endpoint Path
   Is the profile-api POSTing to /api/v1/queue/messages on the queue-service?
   c. Message Format
   Is the message sent by profile-api matching the structure expected by the queue-service?
   (See above for required fields and types.)
   d. Headers
   Is the profile-api including all required headers, especially correlation_id?
   e. Error Handling
   If the queue-service returns an error, does the profile-api log it or surface it in its own logs?
   f. Environment Variables / Config
   Check the profile-api deployment YAML or config files for:
   QUEUE_SERVICE_URL or similar variable
   Any queue-related configuration (host, port, endpoint, etc.)
3. How to check this in your codebase:
   Look in the profile-api source code (likely in internal/, cmd/, or pkg/):
   Search for any HTTP client code that references queue-service.
   Look for environment variable usage like QUEUE_SERVICE_URL.
   Check for any code that builds the message payload for queue-service.
   Check the deployment YAML (in k8s manifests):
   Look for environment variables passed to the profile-api container.
   Confirm the service name and port.
4. If you want to debug live:
   Add debug logging in the profile-api where it sends messages to the queue-service.
   Temporarily change the queue-service endpoint in the profile-api to a test endpoint and see if requests arrive.
   Use port-forwarding to manually test the queue-service endpoint from within the cluster.
5. If you want me to review the code/config for you:
   Please specify:
   The file(s) where the queue integration is implemented in profile-api (or let me know if you want me to search for it).
   The deployment YAML or config file for profile-api.
   Summary:
   The most likely issues are:
   Wrong service name or port in the profile-api config
   Wrong endpoint path
   Message format mismatch
   Missing required headers
