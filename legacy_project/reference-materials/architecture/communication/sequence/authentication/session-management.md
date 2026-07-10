# Session Management Flow

This diagram illustrates the sequence of interactions during session management.

## Sequence Diagram

```mermaid
sequenceDiagram
    participant Client
    participant API as API Gateway
    participant Auth as Auth Service
    participant Cache as Cache Service
    participant Events as Event Service

    Client->>API: GET /v1/auth/sessions
    Note over Client,API: Request includes access token

    API->>Auth: Validate Token
    Auth-->>API: Token Valid

    API->>Auth: List Active Sessions
    Note over API,Auth: Get user's active sessions

    Auth->>Cache: Get User Sessions
    Note over Auth,Cache: Query by user ID
    Cache-->>Auth: Active Sessions

    Auth-->>API: Sessions List
    API-->>Client: 200 OK
    Note over Client,API: Response includes active sessions

    Note over Client,API: Revoke specific session
    Client->>API: DELETE /v1/auth/sessions/{id}
    Note over Client,API: Request includes session ID

    API->>Auth: Validate Token
    Auth-->>API: Token Valid

    API->>Auth: Revoke Session
    Note over API,Auth: Session ID to revoke

    Auth->>Cache: Remove Session
    Note over Auth,Cache: Remove session data
    Cache-->>Auth: Session Removed

    Auth->>Events: Publish Session Revoked Event
    Note over Auth,Events: Event includes session ID and metadata
    Events-->>Auth: Event Published

    Auth-->>API: Session Revoked
    API-->>Client: 204 No Content

    Note over Events: Async Event Processing
    Events->>Cache: Update Session Metadata
    Events->>Cache: Cleanup Session Data
```

## Description

This sequence diagram shows the complete flow of session management:

1. **List Sessions**

   - Client requests active sessions
   - Auth Service validates token
   - Returns list of active sessions

2. **Revoke Session**

   - Client requests session revocation
   - Auth Service validates token
   - Removes session from cache

3. **Event Publishing**

   - Session revocation event published
   - Other services can react to revocation

4. **Async Processing**
   - Update session metadata
   - Cleanup session data

## Error Handling

```mermaid
sequenceDiagram
    participant Client
    participant API as API Gateway
    participant Auth as Auth Service
    participant Cache as Cache Service

    Client->>API: DELETE /v1/auth/sessions/{id}
    Note over Client,API: Invalid token

    API->>Auth: Validate Token
    Auth-->>API: Token Invalid

    API-->>Client: 401 Unauthorized

    Note over Client,API: Retry with valid token
    Client->>API: DELETE /v1/auth/sessions/{id}
    Note over Client,API: Valid token

    API->>Auth: Validate Token
    Auth-->>API: Token Valid

    API->>Auth: Revoke Session
    Auth->>Cache: Remove Session
    Cache-->>Auth: Session Not Found

    Auth-->>API: Session Not Found
    API-->>Client: 404 Not Found
```

## Notes

- Sessions are tracked per user
- Session metadata includes device info
- Failed revocation attempts are logged
- Events are published with at-least-once delivery
- All sensitive data is encrypted in transit
- Rate limiting is applied to session operations
- Session metadata is tracked for security
- Audit logging for session events
- Automatic cleanup of expired sessions
- Session revocation is immediate
- Multiple sessions per user supported
- Session activity is monitored
