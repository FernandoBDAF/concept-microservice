# Dead Letter Queue Flow

This diagram illustrates the sequence of interactions during dead letter queue processing.

## Sequence Diagram

```mermaid
sequenceDiagram
    participant Producer
    participant Events as Event Service
    participant Queue as Message Queue
    participant DLQ as Dead Letter Queue
    participant Consumer
    participant Monitor as Monitoring Service
    participant Admin

    Producer->>Events: Publish Event
    Note over Producer,Events: Event with metadata

    Events->>Queue: Store Event
    Note over Events,Queue: Primary queue
    Queue-->>Events: Event Stored

    Note over Queue,Consumer: Failed Delivery
    Queue->>Consumer: Deliver Event
    Consumer->>Consumer: Processing Error
    Consumer->>Queue: Negative Acknowledge
    Note over Consumer,Queue: Max retries exceeded

    Queue->>DLQ: Move to DLQ
    Note over Queue,DLQ: Store with failure reason
    DLQ-->>Queue: Event Moved

    Queue->>Monitor: Report DLQ Event
    Note over Queue,Monitor: Alert on DLQ
    Monitor-->>Queue: Alert Acknowledged

    Note over Admin,DLQ: Manual Inspection
    Admin->>DLQ: GET /v1/dlq/events
    Note over Admin,DLQ: List failed events
    DLQ-->>Admin: Failed Events List

    Admin->>DLQ: POST /v1/dlq/retry
    Note over Admin,DLQ: Retry specific event

    DLQ->>Events: Retry Event
    Note over DLQ,Events: Original event context
    Events->>Queue: Republish Event
    Note over Events,Queue: Back to primary queue
    Queue-->>Events: Event Republished

    Note over Queue,Consumer: Retry Processing
    Queue->>Consumer: Deliver Event
    Consumer->>Consumer: Process Event
    Consumer->>Queue: Acknowledge Event
    Queue-->>Consumer: Event Acknowledged

    Note over DLQ: DLQ Management
    DLQ->>DLQ: Cleanup Processed Events
    DLQ->>DLQ: Archive Old Events
    DLQ->>DLQ: Update Metrics
```

## Description

This sequence diagram shows the complete flow of dead letter queue handling:

1. **Event Failure**

   - Event processing fails
   - Max retries exceeded
   - Move to DLQ

2. **DLQ Management**

   - Store failed events
   - Track failure reasons
   - Monitor DLQ size

3. **Manual Intervention**

   - Admin inspects events
   - Retry failed events
   - Monitor retry success

4. **Cleanup**
   - Process successful retries
   - Archive old events
   - Update metrics

## Error Handling

```mermaid
sequenceDiagram
    participant Admin
    participant DLQ as Dead Letter Queue
    participant Events as Event Service
    participant Queue as Message Queue

    Admin->>DLQ: POST /v1/dlq/retry
    Note over Admin,DLQ: Invalid event ID

    DLQ->>DLQ: Validate Event
    DLQ-->>Admin: Event Not Found

    Admin-->>DLQ: 404 Not Found

    Note over Admin,DLQ: Retry with valid ID
    Admin->>DLQ: POST /v1/dlq/retry
    Note over Admin,DLQ: Valid event ID

    DLQ->>Events: Retry Event
    Events->>Queue: Republish Event
    Queue-->>Events: Queue Full

    Events-->>DLQ: Republish Failed
    DLQ-->>Admin: 503 Service Unavailable
```

## Notes

- DLQ monitoring
- Retry policies
- Failure tracking
- Event archiving
- Manual intervention
- Metrics collection
- Alert thresholds
- Cleanup policies
- Event inspection
- Retry limits
- Error categorization
- Recovery procedures
- Performance impact
- Resource management
- Audit logging
