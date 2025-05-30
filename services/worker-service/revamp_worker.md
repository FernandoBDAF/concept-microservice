# Worker Service Revamp Implementation Plan

## Overview

This document outlines the detailed implementation plan for revamping the worker-service to use the new common queue package. The service will act as a consumer, processing messages from RabbitMQ that were published by the queue-service.

## Current Implementation Analysis

The current implementation has several key components that we need to consider in our revamp:

1. **RabbitMQ Client**:

   - [x] Using common queue package for connection management
   - [x] Dead letter queue support
   - [x] Message acknowledgment handling
   - [x] Reconnection logic

2. **Message Processing**:

   - [x] Profile message structure
   - [x] Action-based processing (update/delete)
   - [x] 10-second processing delay simulation
   - [x] Metrics collection

3. **Server**:

   - [x] HTTP server for health checks
   - [x] Ready state management
   - [x] Graceful shutdown

4. **Configuration**:
   - [x] Environment-based configuration
   - [x] RabbitMQ connection settings
   - [x] Default values for local development

## Package Structure

```
services/worker-service/
├── cmd/
│   └── main.go                 # Main entry point [COMPLETED]
├── internal/
│   ├── adapters/
│   │   └── queue/
│   │       ├── consumer.go         # Service-specific consumer [COMPLETED]
│   │       └── middleware.go       # Consumer middleware [PENDING]
│   ├── domain/
│   │   ├── message.go             # Service-specific message model [COMPLETED]
│   │   ├── processor.go           # Message processor interface [COMPLETED]
│   │   └── types.go               # Domain types [COMPLETED]
│   ├── processors/
│   │   └── profile/
│   │       ├── processor.go       # Profile message processor [COMPLETED]
│   │       └── validator.go       # Message validation [COMPLETED]
│   └── server/
│       └── server.go              # Worker server setup [COMPLETED]
├── pkg/
│   └── middleware/
│       └── logging.go             # Logging middleware [PENDING]
└── k8s/
    └── base/
        ├── deployment.yaml        # Updated deployment [PENDING]
        └── secrets.yaml           # RabbitMQ credentials [PENDING]
```

## Implementation Phases

### Phase 0: Preparation and Migration Planning [COMPLETED]

- [x] Document current message formats and processing logic
- [x] Identify all current dependencies and their versions
- [x] Create migration strategy for existing messages
- [x] Set up development environment with new dependencies
- [x] Create test data for migration validation

### Phase 1: Core Package Setup [COMPLETED]

- [x] Create directory structure
- [x] Set up Go module with new dependencies
- [x] Add common queue package dependency
- [x] Create basic configuration
- [x] Set up logging infrastructure
- [x] Configure metrics collection

### Phase 2: Domain Implementation [COMPLETED]

- [x] Implement message model
  - [x] Define message structure
  - [x] Add validation rules
  - [x] Implement serialization
- [x] Create processor interface
  - [x] Define processing contract
  - [x] Add error handling
  - [x] Implement metrics interface
- [x] Add validation
  - [x] Create validation rules
  - [x] Implement validation logic
  - [x] Add custom validators
- [x] Define domain types
  - [x] Create type definitions
  - [x] Add type conversions
  - [x] Implement type safety

### Phase 3: Processor Implementation [COMPLETED]

- [x] Create profile processor
  - [x] Implement processing logic
  - [x] Add error handling
  - [x] Implement retry mechanism
- [x] Add metrics
  - [x] Define metric types
  - [x] Implement metric collection
  - [x] Add metric aggregation
- [x] Implement validation
  - [x] Add input validation
  - [x] Implement business rules
  - [x] Add validation metrics
- [x] Add error handling
  - [x] Define error types
  - [x] Implement error recovery
  - [x] Add error reporting

### Phase 4: Consumer Implementation [COMPLETED]

- [x] Create service-specific consumer
  - [x] Implement connection management
  - [x] Add message handling
  - [x] Implement error recovery
- [x] Add metrics
  - [x] Define consumer metrics
  - [x] Implement metric collection
  - [x] Add performance tracking
- [x] Implement message handling
  - [x] Add message processing
  - [x] Implement acknowledgment
  - [x] Add dead letter handling
- [x] Add retry logic
  - [x] Implement retry mechanism
  - [x] Add backoff strategy
  - [x] Implement circuit breaker

### Phase 5: Server Setup [COMPLETED]

- [x] Create server package
  - [x] Implement HTTP server
  - [x] Add health checks
  - [x] Implement metrics endpoint
- [x] Implement graceful shutdown
  - [x] Add shutdown hooks
  - [x] Implement cleanup
  - [x] Add timeout handling
- [x] Add configuration
  - [x] Implement config loading
  - [x] Add validation
  - [x] Implement hot reload
- [x] Set up logging
  - [x] Configure log levels
  - [x] Add structured logging
  - [x] Implement log rotation

### Phase 6: Testing [PENDING]

- [ ] Write unit tests
  - [ ] Test message processing
  - [ ] Test error handling
  - [ ] Test validation
- [ ] Add integration tests
  - [ ] Test queue integration
  - [ ] Test message flow
  - [ ] Test error scenarios
- [ ] Implement error scenarios
  - [ ] Test connection failures
  - [ ] Test message rejection
  - [ ] Test retry logic
- [ ] Add performance tests
  - [ ] Test message throughput
  - [ ] Test resource usage
  - [ ] Test scalability

### Phase 7: Kubernetes Deployment [PENDING]

- [ ] Create Kubernetes manifests
  - [ ] Deployment configuration
    - [ ] Resource limits and requests
    - [ ] Health check configuration
    - [ ] Readiness probe
    - [ ] Liveness probe
  - [ ] Service configuration
    - [ ] Port mapping
    - [ ] Service type
  - [ ] ConfigMap for environment variables
    - [ ] RABBITMQ_URL
    - [ ] Log level
    - [ ] Metrics configuration
  - [ ] Secrets for sensitive data
    - [ ] RabbitMQ credentials
    - [ ] Service tokens
- [ ] Implement deployment strategy
  - [ ] Create rollback plan
  - [ ] Add health checks
  - [ ] Implement monitoring
- [ ] Document changes
  - [ ] Update API documentation
  - [ ] Add deployment guide
  - [ ] Create troubleshooting guide

## Environment Variables

The service requires the following environment variables:

1. RabbitMQ Configuration:

   - `RABBITMQ_URL`: AMQP URL for RabbitMQ connection
     - Format: `amqp://username:password@host:port/vhost`
     - Default: `amqp://guest:guest@localhost:5672/`

2. Service Configuration:
   - HTTP server runs on port 8080
   - Health check endpoint: `/health`
   - Metrics endpoint: `/metrics`

## Current Status

- Core implementation is complete
- Basic functionality is working
- Need to implement tests
- Need to create Kubernetes manifests
- Need to update deployment documentation

## Next Steps

1. Implement unit tests
2. Add integration tests
3. Create Kubernetes manifests
4. Update deployment documentation
5. Set up monitoring and alerting
