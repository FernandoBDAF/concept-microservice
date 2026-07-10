# Event Replay Flow

This diagram illustrates the sequence of interactions during event replay processing.

## Sequence Diagram

```mermaid
sequenceDiagram
    participant Admin
    participant API as API Gateway
    participant Events as Event Service
    participant Queue as Message Queue
    participant Consumer
    participant Cache as Cache Service
    participant DB as Database

    Admin->>API: POST /v1/events/replay
    Note over Admin,API: Replay request with filters

    API->>Events: Initiate Replay
    Note over API,Events: Time range and filters

    Events->>Events: Validate Replay Request
    Note over Events: Check permissions and limits

    Events->>DB: Query Events
    Note over Events,DB: Get events by criteria
    DB-->>Events: Event List

    Events->>Events: Prepare Replay Batch
    Note over Events: Group events by type

    Events->>Queue: Publish Replay Events
    Note over Events,Queue: Events with replay flag
    Queue-->>Events: Events Published

    Events-->>API: Replay Initiated
    API-->>Admin: 202 Accepted
    Note over Admin,API: Replay ID returned

    Note over Queue,Consumer: Replay Processing
    Queue->>Consumer: Deliver Replay Events
    Note over Queue,Consumer: Process with replay context

    Consumer->>Consumer: Process Event
    Note over Consumer: Business logic with replay mode

    Consumer->>Cache: Update Cache
    Note over Consumer,Cache: Conditional updates
    Cache-->>Consumer: Cache Updated

    Consumer->>DB: Persist Changes
    Note over Consumer,DB: Store with replay metadata
    DB-->>Consumer: Changes Persisted

    Consumer->>Queue: Acknowledge Event
    Note over Consumer,Queue: Confirm processing
    Queue-->>Consumer: Event Acknowledged

    Note over Events: Replay Monitoring
    Events->>Events: Track Replay Progress
    Events->>Events: Monitor Processing Time
    Events->>Events: Check Error Rates
```

## Description

This sequence diagram shows the complete flow of event replay:

1. **Replay Initiation**

   - Admin requests replay
   - Validate replay parameters
   - Query relevant events

2. **Event Publishing**

   - Prepare events for replay
   - Publish with replay context
   - Track replay progress

3. **Replay Processing**

   - Process events in replay mode
   - Handle conditional updates
   - Track processing status

4. **Monitoring**
   - Track replay progress
   - Monitor processing time
   - Check error rates

## Error Handling

```mermaid
sequenceDiagram
    participant Admin
    participant API as API Gateway
    participant Events as Event Service
    participant Queue as Message Queue
    participant Consumer

    Admin->>API: POST /v1/events/replay
    Note over Admin,API: Invalid time range

    API->>Events: Initiate Replay
    Events->>Events: Validate Replay Request
    Events-->>API: Invalid Parameters

    API-->>Admin: 400 Bad Request

    Note over Admin,API: Retry with valid range
    Admin->>API: POST /v1/events/replay
    Note over Admin,API: Valid parameters

    API->>Events: Initiate Replay
    Events->>Queue: Publish Replay Events
    Queue-->>Events: Queue Full

    Events-->>API: Queue Capacity Exceeded
    API-->>Admin: 503 Service Unavailable

    Note over Queue,Consumer: Replay Processing Error
    Queue->>Consumer: Deliver Replay Event
    Consumer->>Consumer: Processing Error

    Consumer->>Queue: Negative Acknowledge
    Note over Consumer,Queue: Retry with backoff
    Queue-->>Consumer: Event Requeued
```

## Notes

- Replay with filters
- Time range selection
- Batch processing
- Progress tracking
- Error handling
- Rate limiting
- Resource management
- Audit logging
- Replay metadata
- Conditional updates
- Conflict resolution
- State management
- Performance monitoring
- Recovery options
- Replay validation
