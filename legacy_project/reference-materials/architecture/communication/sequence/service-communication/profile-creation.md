# Profile Creation Flow

This diagram illustrates the sequence of interactions between services during profile creation.

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

    Client->>API: POST /v1/profiles
    Note over Client,API: Request includes profile data and auth token

    API->>Auth: Validate Token
    Auth-->>API: Token Valid

    API->>Profile: Create Profile
    Note over API,Profile: Profile data and metadata

    Profile->>Storage: Store Profile Data
    Storage-->>Profile: Profile Stored

    Profile->>Cache: Cache Profile
    Note over Profile,Cache: Cache with TTL
    Cache-->>Profile: Profile Cached

    Profile->>Events: Publish Profile Created Event
    Note over Profile,Events: Event includes profile ID and metadata
    Events-->>Profile: Event Published

    Profile-->>API: Profile Created
    API-->>Client: 201 Created
    Note over Client,API: Response includes profile ID and metadata

    Note over Events: Async Event Processing
    Events->>Cache: Update Cache
    Events->>Storage: Update Indexes
```

## Description

This sequence diagram shows the complete flow of profile creation:

1. **Initial Request**

   - Client sends profile creation request to API Gateway
   - Request includes profile data and authentication token

2. **Authentication**

   - API Gateway validates the token with Auth Service
   - Proceeds only if token is valid

3. **Profile Creation**

   - Profile Service handles the creation request
   - Coordinates with Storage and Cache services

4. **Data Storage**

   - Profile data is stored in persistent storage
   - Profile is cached for quick access

5. **Event Publishing**

   - Profile creation event is published
   - Other services can react to the event

6. **Response**

   - Success response is sent back to client
   - Includes profile ID and metadata

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

    Client->>API: POST /v1/profiles
    Note over Client,API: Invalid or missing token

    API->>Auth: Validate Token
    Auth-->>API: Token Invalid

    API-->>Client: 401 Unauthorized

    Note over Client,API: Retry with valid token
    Client->>API: POST /v1/profiles
    Note over Client,API: Valid token

    API->>Auth: Validate Token
    Auth-->>API: Token Valid

    API->>Profile: Create Profile
    Profile->>Storage: Store Profile Data
    Storage-->>Profile: Storage Error

    Profile-->>API: 500 Internal Server Error
    API-->>Client: 500 Internal Server Error
```

## Notes

- All services implement retry mechanisms for transient failures
- Circuit breakers are in place to prevent cascading failures
- Events are published with at-least-once delivery guarantee
- Cache operations are performed with best-effort strategy
- Storage operations are performed with strong consistency
- All sensitive data is encrypted in transit and at rest
