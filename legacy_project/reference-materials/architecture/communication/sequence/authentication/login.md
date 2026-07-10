# Login Flow

This diagram illustrates the sequence of interactions during user login.

## Sequence Diagram

```mermaid
sequenceDiagram
    participant Client
    participant API as API Gateway
    participant Auth as Auth Service
    participant User as User Service
    participant Cache as Cache Service
    participant Events as Event Service

    Client->>API: POST /v1/auth/login
    Note over Client,API: Request includes credentials

    API->>Auth: Authenticate User
    Note over API,Auth: Username and password

    Auth->>User: Validate Credentials
    User-->>Auth: User Validated

    Auth->>Auth: Generate Access Token
    Note over Auth: JWT with user claims

    Auth->>Auth: Generate Refresh Token
    Note over Auth: Long-lived refresh token

    Auth->>Cache: Store Refresh Token
    Note over Auth,Cache: Store with user ID and expiry
    Cache-->>Auth: Token Stored

    Auth->>Events: Publish Login Event
    Note over Auth,Events: Event includes user ID and metadata
    Events-->>Auth: Event Published

    Auth-->>API: Authentication Success
    Note over Auth,API: Access token and refresh token

    API-->>Client: 200 OK
    Note over Client,API: Response includes tokens and user info

    Note over Events: Async Event Processing
    Events->>Cache: Update User Session
    Events->>User: Update Last Login
```

## Description

This sequence diagram shows the complete flow of user login:

1. **Initial Request**

   - Client sends login request with credentials
   - Request includes username and password

2. **Authentication**

   - Auth Service validates credentials with User Service
   - Generates access and refresh tokens

3. **Token Generation**

   - Access token (JWT) with user claims
   - Refresh token for token renewal
   - Tokens stored in cache

4. **Event Publishing**

   - Login event published for tracking
   - Other services can react to login

5. **Response**

   - Success response with tokens
   - Includes user information

6. **Async Processing**
   - Update user session in cache
   - Update last login timestamp

## Error Handling

```mermaid
sequenceDiagram
    participant Client
    participant API as API Gateway
    participant Auth as Auth Service
    participant User as User Service

    Client->>API: POST /v1/auth/login
    Note over Client,API: Invalid credentials

    API->>Auth: Authenticate User
    Auth->>User: Validate Credentials
    User-->>Auth: Invalid Credentials

    Auth-->>API: Authentication Failed
    API-->>Client: 401 Unauthorized

    Note over Client,API: Retry with valid credentials
    Client->>API: POST /v1/auth/login
    Note over Client,API: Valid credentials

    API->>Auth: Authenticate User
    Auth->>User: Validate Credentials
    User-->>Auth: Account Locked

    Auth-->>API: Account Locked
    API-->>Client: 403 Forbidden
```

## Notes

- Access tokens are short-lived (15-60 minutes)
- Refresh tokens are long-lived (days/weeks)
- All tokens are stored securely
- Failed login attempts are tracked
- Account locking after multiple failures
- Events are published with at-least-once delivery
- All sensitive data is encrypted in transit
- Rate limiting is applied to login attempts
- Session tracking for security monitoring
- Audit logging for security events
