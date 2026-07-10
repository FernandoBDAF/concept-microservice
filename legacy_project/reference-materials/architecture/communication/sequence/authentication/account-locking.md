# Account Locking Flow

This diagram illustrates the sequence of interactions during account locking.

## Sequence Diagram

```mermaid
sequenceDiagram
    participant User
    participant API as API Gateway
    participant Auth as Auth Service
    participant Cache as Cache Service
    participant Events as Event Service
    participant Email as Email Service

    User->>API: POST /v1/auth/login
    Note over User,API: Failed login attempt

    API->>Auth: Authenticate User
    Auth->>Auth: Track Failed Attempt
    Note over Auth: Increment failure counter

    Auth->>Cache: Get Failed Attempts
    Note over Auth,Cache: Check attempt count
    Cache-->>Auth: Attempt Count

    Auth->>Auth: Check Lock Threshold
    Note over Auth: Compare with policy

    Auth->>Cache: Lock Account
    Note over Auth,Cache: Set lock status and expiry
    Cache-->>Auth: Account Locked

    Auth->>Events: Publish Account Locked Event
    Note over Auth,Events: Event includes user ID and reason
    Events-->>Auth: Event Published

    Auth->>Email: Send Lock Notification
    Note over Auth,Email: Email with lock details
    Email-->>Auth: Email Sent

    Auth-->>API: Account Locked
    API-->>User: 403 Forbidden

    Note over Events: Async Event Processing
    Events->>Cache: Update Security Metrics
    Events->>Cache: Track Lock Duration

    Note over User,API: Lock period expired
    User->>API: POST /v1/auth/login
    Note over User,API: Valid credentials

    API->>Auth: Authenticate User
    Auth->>Cache: Check Account Status
    Cache-->>Auth: Lock Expired

    Auth->>Cache: Reset Failed Attempts
    Note over Auth,Cache: Clear attempt counter
    Cache-->>Auth: Attempts Reset

    Auth->>Events: Publish Account Unlocked Event
    Note over Auth,Events: Event includes user ID
    Events-->>Auth: Event Published

    Auth-->>API: Authentication Success
    API-->>User: 200 OK
```

## Description

This sequence diagram shows the complete flow of account locking:

1. **Failed Login**

   - Track failed attempts
   - Check lock threshold
   - Lock account if needed

2. **Account Locking**

   - Set lock status
   - Publish lock event
   - Send notification

3. **Lock Management**

   - Track lock duration
   - Update security metrics
   - Monitor lock status

4. **Lock Release**
   - Check lock expiry
   - Reset failed attempts
   - Unlock account

## Error Handling

```mermaid
sequenceDiagram
    participant User
    participant API as API Gateway
    participant Auth as Auth Service
    participant Cache as Cache Service

    User->>API: POST /v1/auth/login
    Note over User,API: Account already locked

    API->>Auth: Authenticate User
    Auth->>Cache: Check Account Status
    Cache-->>Auth: Account Locked

    Auth-->>API: Account Locked
    API-->>User: 403 Forbidden
    Note over User,API: Lock duration remaining

    Note over User,API: Manual unlock request
    User->>API: POST /v1/auth/unlock
    Note over User,API: Unlock token

    API->>Auth: Unlock Account
    Auth->>Cache: Validate Unlock Token
    Cache-->>Auth: Token Invalid

    Auth-->>API: Invalid Token
    API-->>User: 400 Bad Request
```

## Notes

- Progressive lock duration
- Multiple lock triggers
- Lock bypass for admins
- Manual unlock process
- Lock notification system
- Security metrics tracking
- Lock history maintained
- IP-based tracking
- Device fingerprinting
- Lock policy enforcement
- Unlock token system
- Lock duration limits
- Security audit logging
- Lock attempt tracking
- Recovery options available
