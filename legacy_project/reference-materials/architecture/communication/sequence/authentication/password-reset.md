# Password Reset Flow

This diagram illustrates the sequence of interactions during password reset.

## Sequence Diagram

```mermaid
sequenceDiagram
    participant User
    participant API as API Gateway
    participant Auth as Auth Service
    participant Email as Email Service
    participant Cache as Cache Service
    participant Events as Event Service

    User->>API: POST /v1/auth/password/reset-request
    Note over User,API: Request includes email

    API->>Auth: Request Password Reset
    Note over API,Auth: Email validation

    Auth->>Auth: Generate Reset Token
    Note over Auth: Time-limited token

    Auth->>Cache: Store Reset Token
    Note over Auth,Cache: Store with expiry
    Cache-->>Auth: Token Stored

    Auth->>Email: Send Reset Email
    Note over Auth,Email: Email with reset link
    Email-->>Auth: Email Sent

    Auth->>Events: Publish Reset Request Event
    Note over Auth,Events: Event includes user ID
    Events-->>Auth: Event Published

    Auth-->>API: Reset Request Processed
    API-->>User: 200 OK

    Note over User,API: User clicks reset link
    User->>API: POST /v1/auth/password/reset
    Note over User,API: New password and token

    API->>Auth: Reset Password
    Note over API,Auth: Validate token and password

    Auth->>Cache: Validate Reset Token
    Note over Auth,Cache: Check token validity
    Cache-->>Auth: Token Valid

    Auth->>Auth: Hash New Password
    Note over Auth: Secure password hashing

    Auth->>Cache: Invalidate Reset Token
    Note over Auth,Cache: Remove used token
    Cache-->>Auth: Token Invalidated

    Auth->>Events: Publish Password Reset Event
    Note over Auth,Events: Event includes user ID
    Events-->>Auth: Event Published

    Auth-->>API: Password Reset Complete
    API-->>User: 200 OK

    Note over Events: Async Event Processing
    Events->>Email: Send Confirmation Email
    Events->>Cache: Invalidate User Sessions
```

## Description

This sequence diagram shows the complete flow of password reset:

1. **Reset Request**

   - User requests password reset
   - Reset token generated
   - Reset email sent

2. **Token Storage**

   - Token stored in cache
   - Time-limited validity
   - Secure storage

3. **Password Reset**

   - Token validation
   - Password hashing
   - Token invalidation

4. **Event Processing**
   - Reset events published
   - Confirmation email sent
   - Sessions invalidated

## Error Handling

```mermaid
sequenceDiagram
    participant User
    participant API as API Gateway
    participant Auth as Auth Service
    participant Cache as Cache Service

    User->>API: POST /v1/auth/password/reset
    Note over User,API: Invalid token

    API->>Auth: Reset Password
    Auth->>Cache: Validate Reset Token
    Cache-->>Auth: Token Invalid

    Auth-->>API: Invalid Token
    API-->>User: 400 Bad Request

    Note over User,API: Retry with valid token
    User->>API: POST /v1/auth/password/reset
    Note over User,API: Valid token, weak password

    API->>Auth: Reset Password
    Auth->>Auth: Validate Password Strength
    Auth-->>API: Password Too Weak

    API-->>User: 400 Bad Request
    Note over User,API: Password requirements not met
```

## Notes

- Reset tokens are time-limited
- Tokens are single-use only
- Password strength requirements enforced
- Rate limiting on reset requests
- Email verification required
- Session invalidation on reset
- Audit logging of reset attempts
- Secure password hashing
- Reset link expiration
- Multiple reset prevention
- Email delivery tracking
- Reset attempt tracking
- Security notifications
- Account recovery options
- Password history check
