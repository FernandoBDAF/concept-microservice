# Queue Service Implementation Request

## Task Context

**Task**: Implement critical architectural upgrade for queue-service to align with RabbitMQ best practices and support multi-worker architecture

**Priority**: CRITICAL (BLOCKING)
**Effort**: 4-week implementation (6 phases, 15 tasks)
**Status**: Ready for execution
**Dependencies**: None - all analysis and planning complete

**Critical Issue**: The current queue-service has **fundamental architectural misalignments** with RabbitMQ best practices and **cannot integrate** with the worker-service implementation. This upgrade is **BLOCKING** for multi-worker architecture implementation.

## Documentation References

### 1. QUEUE_SERVICE_ANALYSIS.md

- **Section**: Complete technical analysis
- **Purpose**: Provides detailed assessment of current architectural gaps and integration issues
- **Impact**: Defines the critical problems that must be solved and provides technical rationale for all changes
- **Key Finding**: Current implementation has message format incompatibility, exchange strategy misalignment, and connection management anti-patterns

### 2. TRACKER.md

- **Section**: Implementation plan with 6 phases and 15 detailed tasks
- **Purpose**: Provides step-by-step implementation roadmap with timelines, priorities, and acceptance criteria
- **Impact**: Guides implementation sequence and ensures all critical fixes are addressed
- **Key Phases**: Message format alignment, exchange strategy overhaul, publisher confirms, multi-worker support, testing, and documentation

### 3. README.md

- **Section**: Post-upgrade architecture and features
- **Purpose**: Documents target architecture, API endpoints, and integration patterns
- **Impact**: Defines what the service should look like after upgrade completion
- **Key Features**: Multi-worker support, RabbitMQ best practices, publisher confirms, enhanced monitoring

### 4. INTERFACE.md

- **Section**: Enhanced HTTP API endpoints and multi-worker message queue interfaces
- **Purpose**: Specifies exact API changes, routing key support, and integration patterns
- **Impact**: Defines precise interface contracts for client services and worker integration
- **Key Changes**: Routing key support, enhanced message format, publisher confirm tracking

### 5. CONTEXT.md

- **Section**: Technical implementation details and architecture patterns
- **Purpose**: Provides detailed technical context for implementation decisions
- **Impact**: Guides code structure, design patterns, and RabbitMQ integration approach
- **Key Patterns**: Clean architecture, publisher confirms, connection management, routing strategy

### 6. MIGRATION.md

- **Section**: Step-by-step migration guide with testing procedures
- **Purpose**: Provides detailed migration phases, testing procedures, and rollback strategies
- **Impact**: Ensures safe implementation with backward compatibility and zero message loss
- **Key Phases**: Message format alignment, exchange strategy overhaul, publisher confirms, integration testing

### 7. CURSOR.md

- **Section**: Guidelines for working with documentation and implementation patterns
- **Purpose**: Provides best practices for documentation-driven development
- **Impact**: Ensures consistent implementation approach and proper documentation updates

## Requirements

### Critical Integration Fixes (BLOCKING)

1. **Message Format Alignment**

   - Change `Headers map[string]string` to `Metadata map[string]string`
   - Change `Payload interface{}` to `Payload json.RawMessage`
   - Remove `MessageType` enum, use `string` for `Type` field
   - Update JSON marshaling/unmarshaling methods
   - Maintain backward compatibility in HTTP API layer

2. **Exchange Strategy Overhaul**

   - Replace per-queue exchanges (`queueName.exchange`) with single exchange approach
   - Implement routing key-based message distribution
   - Support multiple worker types: `profile.task`, `email.send`, `image.process`
   - Update queue binding logic to use semantic routing keys
   - Remove complex per-queue exchange setup

3. **API Layer Routing Key Support**
   - Add `routing_key` field to publish message API
   - Support routing key specification in request body
   - Validate routing key format (`worker_type.action`)
   - Update API documentation and examples
   - Maintain backward compatibility with default routing

### Architecture Improvements (HIGH PRIORITY)

4. **Connection Management Simplification**

   - Implement one long-lived connection per service pattern
   - Use single channel for publishing with proper reuse
   - Simplify reconnection logic following best practices
   - Remove complex monitoring goroutines
   - Add proper connection state management

5. **Publisher Confirms Implementation**

   - Enable publisher confirms on channel
   - Implement confirm handling in publish method
   - Add timeout for confirm acknowledgments
   - Update metrics to track confirm success/failure
   - Handle confirm failures with retry logic

6. **Multi-Worker Architecture Support**
   - Support multiple exchanges for different worker types
   - Dynamic exchange declaration based on routing key
   - Proper queue binding for each worker type
   - Worker-specific queue configuration (TTL, prefetch, DLQ)

## Constraints

- **Must maintain backward compatibility** during transition period
- **Must follow RabbitMQ best practices** as outlined in rabbit+go+kind.md patterns
- **Must align with worker-service expectations** for message format and routing
- **Must implement zero-downtime deployment** strategy
- **Must include comprehensive error handling** and recovery mechanisms
- **Must maintain clean architecture principles** with proper separation of concerns
- **Must include comprehensive logging** for debugging and monitoring
- **Must update all documentation** to reflect changes

## Expected Output

### Code Changes Required

1. **Domain Layer Updates**

   - `internal/domain/model/message.go` - Updated message structure
   - `internal/domain/service/queue.go` - Enhanced publisher service with routing keys
   - New routing strategy implementation

2. **Infrastructure Layer Updates**

   - `internal/adapters/rabbitmq/rabbitmq.go` - Simplified RabbitMQ client with best practices
   - `internal/adapters/http/handler.go` - Enhanced HTTP handlers with routing key support
   - Connection management improvements

3. **Configuration Updates**

   - Environment variable updates for multi-worker support
   - Routing key configuration mapping
   - Worker-specific queue configurations

4. **API Enhancements**
   - New request/response formats with routing key support
   - Enhanced error handling and validation
   - Backward compatibility layer

### Architecture Alignment

- **Single Exchange Pattern**: Use `tasks-exchange` with routing keys instead of per-queue exchanges
- **Publisher Confirms**: Reliable message delivery with acknowledgment handling
- **Connection Best Practices**: Long-lived connection with single channel for publishing
- **Multi-Worker Support**: Dynamic routing to profile, email, and image workers
- **Enhanced Monitoring**: Per-worker-type metrics and routing key distribution

## Documentation Updates Required

### 1. Code Documentation

- **Files**: All modified Go files
- **Changes**: Add comprehensive comments explaining new patterns and routing logic
- **Reason**: Ensure maintainability and knowledge transfer

### 2. API Documentation

- **Files**: Update examples in README.md and INTERFACE.md
- **Changes**: Include routing key examples and new request/response formats
- **Reason**: Guide client service integration

### 3. Implementation Status

- **Files**: TRACKER.md
- **Changes**: Update task status as implementation progresses
- **Reason**: Track implementation progress and maintain project visibility

### 4. Architecture Documentation

- **Files**: CONTEXT.md
- **Changes**: Update technical implementation details to reflect actual code structure
- **Reason**: Maintain accurate technical documentation

## Verification Requirements

### Functional Verification

- [ ] **Message Compatibility**: Worker-service can consume messages published by queue-service
- [ ] **Routing Key Support**: All three routing keys (profile.task, email.send, image.process) work correctly
- [ ] **Backward Compatibility**: Existing API continues working with default routing key assignment
- [ ] **Publisher Confirms**: 99%+ publisher confirm success rate in testing
- [ ] **Exchange Strategy**: Single exchange with proper routing key bindings

### Technical Verification

- [ ] **RabbitMQ Best Practices**: Implementation follows patterns from rabbit+go+kind.md
- [ ] **Clean Architecture**: Proper separation of concerns maintained
- [ ] **Error Handling**: Comprehensive error handling for all failure scenarios
- [ ] **Connection Management**: Single long-lived connection with proper reconnection logic
- [ ] **Performance**: Equal or better throughput compared to current implementation

### Integration Verification

- [ ] **API Compatibility**: All endpoints respond correctly with new message format
- [ ] **Metrics Collection**: Enhanced metrics collecting data correctly
- [ ] **Health Checks**: Enhanced health checks reporting RabbitMQ connection status
- [ ] **Dead Letter Queues**: Failed messages properly handled in worker-specific DLQs
- [ ] **Configuration**: All environment variables and configuration options working

## Implementation Phases

### Phase 1: Critical Integration Fixes [CRITICAL - 4 hours]

**Tasks**: 1.1 Message Format Alignment, 1.2 Exchange Strategy Overhaul, 1.3 API Routing Key Support
**Goal**: Fix blocking integration issues with worker-service
**Success Criteria**: Worker-service can consume queue-service messages

### Phase 2: Connection Management & Publisher Confirms [HIGH - 3 hours]

**Tasks**: 2.1 Connection Pattern Simplification, 2.2 Publisher Confirms Implementation
**Goal**: Implement RabbitMQ best practices for reliability
**Success Criteria**: Publisher confirms working, simplified connection management

### Phase 3: Multi-Worker Architecture Support [HIGH - 4 hours]

**Tasks**: 3.1 Dynamic Exchange Configuration, 3.2 Worker-Specific Queue Configuration
**Goal**: Enable support for email and image workers
**Success Criteria**: All three worker types supported with proper routing

### Phase 4: Testing & Validation [MEDIUM - 4 hours]

**Tasks**: 4.1 Integration Testing, 4.2 Multi-Worker Preparation Testing
**Goal**: Validate end-to-end functionality
**Success Criteria**: Comprehensive testing passing, ready for deployment

### Phase 5: Documentation & Deployment Preparation [MEDIUM - 3 hours]

**Tasks**: 5.1 Documentation Updates, 5.2 Kubernetes Deployment Updates
**Goal**: Complete implementation with proper documentation
**Success Criteria**: All documentation updated, deployment manifests ready

### Phase 6: Performance & Monitoring [LOW - 2 hours]

**Tasks**: 6.1 Metrics Enhancement, 6.2 Performance Optimization
**Goal**: Optimize and monitor new architecture
**Success Criteria**: Enhanced metrics, performance optimizations applied

## Current Architecture Issues (Critical Problems to Solve)

### 1. Message Format Incompatibility

```go
// CURRENT (Broken)
type Message struct {
    Type    MessageType       `json:"type"`        // Enum - worker expects string
    Headers map[string]string `json:"headers"`     // Worker expects "metadata"
    Payload interface{}       `json:"payload"`     // Worker expects json.RawMessage
}

// REQUIRED (Compatible)
type Message struct {
    Type     string            `json:"type"`       // String
    Metadata map[string]string `json:"metadata"`   // Renamed from Headers
    Payload  json.RawMessage   `json:"payload"`    // RawMessage instead of interface{}
}
```

### 2. Exchange Strategy Anti-Pattern

```go
// CURRENT (Wrong - Creates multiple exchanges)
exchangeName := queueName + ".exchange"
err = channel.Publish(exchangeName, queueName, ...)

// REQUIRED (Best Practice - Single exchange with routing keys)
err = channel.Publish("tasks-exchange", "profile.task", ...)
```

### 3. Connection Management Problems

- Creates new channel for every operation (inefficient)
- Complex reconnection logic with potential race conditions
- No proper channel pooling or reuse
- Monitoring goroutines may create memory leaks

## Target Architecture (Implementation Goal)

### RabbitMQ Integration Pattern

```
Profile Service → Queue Service HTTP API → RabbitMQ Exchange → Worker Queues
                                              ↓
                    ┌─────────────────────────┼─────────────────────────┐
                    ↓                         ↓                         ↓
            profile.task                 email.send              image.process
                    ↓                         ↓                         ↓
        profile-processing            email-processing         image-processing
                    ↓                         ↓                         ↓
           Profile Worker              Email Worker             Image Worker
```

### Routing Key Mapping

```go
var RoutingMap = map[string]RoutingConfig{
    "profile.task": {
        Exchange: "tasks-exchange",
        Queue:    "profile-processing",
        TTL:      24 * time.Hour,
        Prefetch: 1,
    },
    "email.send": {
        Exchange: "email-tasks",
        Queue:    "email-processing",
        TTL:      1 * time.Hour,
        Prefetch: 5,
    },
    "image.process": {
        Exchange: "image-tasks",
        Queue:    "image-processing",
        TTL:      6 * time.Hour,
        Prefetch: 1,
    },
}
```

## Success Criteria Summary

### Critical Success Factors

- **Worker-Service Integration**: Messages published by queue-service successfully consumed by worker-service
- **Multi-Worker Ready**: Infrastructure prepared for email and image workers
- **RabbitMQ Best Practices**: Implementation aligned with industry standards
- **Zero Message Loss**: No messages lost during implementation and testing
- **Backward Compatibility**: Existing clients continue working during transition

### Performance Targets

- **API Response Time**: < 100ms (message acceptance)
- **Publisher Confirm**: < 500ms (RabbitMQ confirmation)
- **Throughput**: Equal or better than current implementation
- **Error Rate**: < 1% for all operations

## Implementation Guidelines

### 1. Follow Documentation-Driven Development

- Reference TRACKER.md for task sequence and acceptance criteria
- Use MIGRATION.md for step-by-step implementation guidance
- Update documentation as implementation progresses
- Verify against INTERFACE.md for API compliance

### 2. Maintain Clean Architecture

- Keep domain logic independent of infrastructure
- Use dependency injection for testability
- Implement proper error handling at each layer
- Follow single responsibility principle

### 3. Implement Comprehensive Testing

- Unit tests for all new functionality
- Integration tests for RabbitMQ interaction
- API tests for all endpoints
- Load testing for performance validation

### 4. Ensure Observability

- Add comprehensive logging for debugging
- Implement metrics for monitoring
- Include health checks for operational visibility
- Provide clear error messages for troubleshooting

## Risk Mitigation

### High-Risk Areas

1. **Message Format Changes**: Risk of breaking existing integrations
   - **Mitigation**: Implement backward compatibility layer
2. **Exchange Strategy Changes**: Risk of message routing failures
   - **Mitigation**: Comprehensive testing of all routing keys
3. **Connection Management**: Risk of connection instability
   - **Mitigation**: Follow proven patterns from rabbit+go+kind.md

### Rollback Strategy

- Maintain current implementation alongside new implementation
- Implement feature flags for gradual rollout
- Comprehensive monitoring during deployment
- Quick rollback procedures documented in MIGRATION.md

---

**Implementation Status**: 🔄 **Ready for Execution**

This comprehensive upgrade will transform the queue-service from a broken, incompatible implementation to a robust, scalable message publisher that follows RabbitMQ best practices and enables the full multi-worker architecture. All analysis, planning, and documentation is complete - ready for immediate implementation.
