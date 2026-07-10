# Profile Deletion Flow

This diagram illustrates the sequence of interactions between services during profile deletion.

## Sequence Diagram

```mermaid
sequenceDiagram
    participant Client
    participant API as API Gateway
    participant Auth as Auth Service
    participant Profile as Profile Service
    participant Cache as Cache Service
    participant Storage as Storage Service
    participant Events as Event Service

    Client->>API: DELETE /v1/profiles/{id}
    Note over Client,API: Request includes profile ID and auth token

    API->>Auth: Validate Token
    Auth-->>API: Token Valid

    API->>Profile: Delete Profile
    Note over API,Profile: Profile ID

    Profile->>Storage: Get Current Profile
    Storage-->>Profile: Current Profile Data

    Profile->>Storage: Soft Delete Profile
    Note over Profile,Storage: Mark as deleted with timestamp
    Storage-->>Profile: Profile Deleted

    Profile->>Cache: Invalidate Cache
    Note over Profile,Cache: Invalidate by profile ID
    Cache-->>Profile: Cache Invalidated

    Profile->>Events: Publish Profile Deleted Event
    Note over Profile,Events: Event includes profile ID and metadata
    Events-->>Profile: Event Published

    Profile-->>API: Profile Deleted
    API-->>Client: 204 No Content

    Note over Events: Async Event Processing
    Events->>Cache: Remove from Cache
    Events->>Storage: Update Indexes
    Events->>Storage: Schedule Hard Delete
```

## Description

This sequence diagram shows the complete flow of profile deletion:

1. **Initial Request**

   - Client sends profile deletion request to API Gateway
   - Request includes profile ID and authentication token

2. **Authentication**

   - API Gateway validates the token with Auth Service
   - Proceeds only if token is valid

3. **Profile Deletion**

   - Profile Service retrieves current profile data
   - Performs soft deletion in storage
   - Coordinates with Cache service

4. **Data Storage**

   - Profile is marked as deleted in storage
   - Cache is invalidated

5. **Event Publishing**

   - Profile deletion event is published
   - Other services can react to the event

6. **Response**

   - Success response is sent back to client
   - No content returned (204)

7. **Async Processing**
   - Event Service triggers additional processing
   - Removes from cache
   - Updates storage indexes
   - Schedules hard deletion

## Error Handling

```mermaid
sequenceDiagram
    participant Client
    participant API as API Gateway
    participant Auth as Auth Service
    participant Profile as Profile Service
    participant Storage as Storage Service

    Client->>API: DELETE /v1/profiles/{id}
    Note over Client,API: Invalid or missing token

    API->>Auth: Validate Token
    Auth-->>API: Token Invalid

    API-->>Client: 401 Unauthorized

    Note over Client,API: Retry with valid token
    Client->>API: DELETE /v1/profiles/{id}
    Note over Client,API: Valid token

    API->>Auth: Validate Token
    Auth-->>API: Token Valid

    API->>Profile: Delete Profile
    Profile->>Storage: Get Current Profile
    Storage-->>Profile: Profile Not Found

    Profile-->>API: 404 Not Found
    API-->>Client: 404 Not Found

    Note over Client,API: Retry with existing profile
    Client->>API: DELETE /v1/profiles/{id}
    API->>Profile: Delete Profile
    Profile->>Storage: Soft Delete Profile
    Storage-->>Profile: Deletion Error

    Profile-->>API: 500 Internal Server Error
    API-->>Client: 500 Internal Server Error
```

## Notes

- Soft deletion is used to maintain data history
- Hard deletion is scheduled for later execution
- Cache invalidation is performed immediately
- Events are published with at-least-once delivery guarantee
- All services implement retry mechanisms for transient failures
- Circuit breakers are in place to prevent cascading failures
- Cache operations are performed with best-effort strategy
- Storage operations are performed with strong consistency
- All sensitive data is encrypted in transit and at rest
- Deleted profiles are retained for audit purposes
- Hard deletion is performed after retention period
