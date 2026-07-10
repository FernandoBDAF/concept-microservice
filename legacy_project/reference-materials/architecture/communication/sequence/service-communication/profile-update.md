# Profile Update Flow

This diagram illustrates the sequence of interactions between services during profile updates.

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

    Client->>API: PUT /v1/profiles/{id}
    Note over Client,API: Request includes updated profile data and auth token

    API->>Auth: Validate Token
    Auth-->>API: Token Valid

    API->>Profile: Update Profile
    Note over API,Profile: Profile ID and update data

    Profile->>Storage: Get Current Profile
    Storage-->>Profile: Current Profile Data

    Profile->>Storage: Update Profile Data
    Note over Profile,Storage: Optimistic locking with version
    Storage-->>Profile: Profile Updated

    Profile->>Cache: Invalidate Cache
    Note over Profile,Cache: Invalidate by profile ID
    Cache-->>Profile: Cache Invalidated

    Profile->>Cache: Update Cache
    Note over Profile,Cache: Cache with TTL
    Cache-->>Profile: Cache Updated

    Profile->>Events: Publish Profile Updated Event
    Note over Profile,Events: Event includes profile ID, changes, and metadata
    Events-->>Profile: Event Published

    Profile-->>API: Profile Updated
    API-->>Client: 200 OK
    Note over Client,API: Response includes updated profile data

    Note over Events: Async Event Processing
    Events->>Cache: Update Cache
    Events->>Storage: Update Indexes
```

## Description

This sequence diagram shows the complete flow of profile updates:

1. **Initial Request**

   - Client sends profile update request to API Gateway
   - Request includes profile ID, update data, and authentication token

2. **Authentication**

   - API Gateway validates the token with Auth Service
   - Proceeds only if token is valid

3. **Profile Update**

   - Profile Service retrieves current profile data
   - Applies updates with optimistic locking
   - Coordinates with Storage and Cache services

4. **Data Storage**

   - Profile data is updated in persistent storage
   - Cache is invalidated and updated

5. **Event Publishing**

   - Profile update event is published
   - Other services can react to the event

6. **Response**

   - Success response is sent back to client
   - Includes updated profile data

7. **Async Processing**
   - Event Service triggers additional processing
   - Updates cache and storage indexes

## Error Handling

```mermaid
sequenceDiagram
    participant Client
    participant API as API Gateway
    participant Auth as Auth Service
    participant Profile as Profile Service
    participant Storage as Storage Service

    Client->>API: PUT /v1/profiles/{id}
    Note over Client,API: Invalid or missing token

    API->>Auth: Validate Token
    Auth-->>API: Token Invalid

    API-->>Client: 401 Unauthorized

    Note over Client,API: Retry with valid token
    Client->>API: PUT /v1/profiles/{id}
    Note over Client,API: Valid token

    API->>Auth: Validate Token
    Auth-->>API: Token Valid

    API->>Profile: Update Profile
    Profile->>Storage: Get Current Profile
    Storage-->>Profile: Profile Not Found

    Profile-->>API: 404 Not Found
    API-->>Client: 404 Not Found

    Note over Client,API: Retry with existing profile
    Client->>API: PUT /v1/profiles/{id}
    API->>Profile: Update Profile
    Profile->>Storage: Update Profile Data
    Storage-->>Profile: Version Conflict

    Profile-->>API: 409 Conflict
    API-->>Client: 409 Conflict
```

## Notes

- Optimistic locking is used to prevent concurrent updates
- Cache invalidation is performed before update to prevent stale data
- Events include both old and new profile data for change tracking
- All services implement retry mechanisms for transient failures
- Circuit breakers are in place to prevent cascading failures
- Events are published with at-least-once delivery guarantee
- Cache operations are performed with best-effort strategy
- Storage operations are performed with strong consistency
- All sensitive data is encrypted in transit and at rest
