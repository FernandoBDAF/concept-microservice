# Token Refresh Flow

This diagram illustrates the sequence of interactions during token refresh.

## Sequence Diagram

```mermaid
sequenceDiagram
    participant Client
    participant API as API Gateway
    participant Auth as Auth Service
    participant Cache as Cache Service
    participant Events as Event Service

    Client->>API: POST /v1/auth/refresh
    Note over Client,API: Request includes refresh token

    API->>Auth: Refresh Token
    Note over API,Auth: Refresh token validation

    Auth->>Cache: Validate Refresh Token
    Note over Auth,Cache: Check token existence and expiry
    Cache-->>Auth: Token Valid

    Auth->>Auth: Generate New Access Token
    Note over Auth: JWT with user claims

    Auth->>Auth: Generate New Refresh Token
    Note over Auth: Long-lived refresh token

    Auth->>Cache: Invalidate Old Refresh Token
    Note over Auth,Cache: Remove old token
    Cache-->>Auth: Token Invalidated

    Auth->>Cache: Store New Refresh Token
    Note over Auth,Cache: Store with user ID and expiry
    Cache-->>Auth: Token Stored

    Auth->>Events: Publish Token Refresh Event
    Note over Auth,Events: Event includes user ID and metadata
    Events-->>Auth: Event Published

    Auth-->>API: Refresh Success
    Note over Auth,API: New access token and refresh token

    API-->>Client: 200 OK
    Note over Client,API: Response includes new tokens

    Note over Events: Async Event Processing
    Events->>Cache: Update Token Metadata
    Events->>Cache: Cleanup Expired Tokens
```

## Description

This sequence diagram shows the complete flow of token refresh:

1. **Initial Request**

   - Client sends refresh request with refresh token
   - Request includes current refresh token

2. **Token Validation**

   - Auth Service validates refresh token
   - Checks token existence and expiry

3. **Token Generation**

   - New access token generated
   - New refresh token generated
   - Old refresh token invalidated

4. **Token Storage**

   - New refresh token stored in cache
   - Old refresh token removed
   - Token metadata updated

5. **Event Publishing**

   - Token refresh event published
   - Other services can react to refresh

6. **Response**

   - Success response with new tokens
   - Includes new access and refresh tokens

7. **Async Processing**
   - Update token metadata
   - Cleanup expired tokens

## Error Handling

```mermaid
sequenceDiagram
    participant Client
    participant API as API Gateway
    participant Auth as Auth Service
    participant Cache as Cache Service

    Client->>API: POST /v1/auth/refresh
    Note over Client,API: Invalid refresh token

    API->>Auth: Refresh Token
    Auth->>Cache: Validate Refresh Token
    Cache-->>Auth: Token Invalid

    Auth-->>API: Refresh Failed
    API-->>Client: 401 Unauthorized

    Note over Client,API: Retry with valid token
    Client->>API: POST /v1/auth/refresh
    Note over Client,API: Valid refresh token

    API->>Auth: Refresh Token
    Auth->>Cache: Validate Refresh Token
    Cache-->>Auth: Token Expired

    Auth-->>API: Token Expired
    API-->>Client: 401 Unauthorized
```

## Notes

- Refresh tokens are rotated on each refresh
- Old refresh tokens are immediately invalidated
- Token validation includes expiry check
- Failed refresh attempts are logged
- Events are published with at-least-once delivery
- All sensitive data is encrypted in transit
- Rate limiting is applied to refresh attempts
- Token metadata is tracked for security
- Audit logging for refresh events
- Automatic cleanup of expired tokens
