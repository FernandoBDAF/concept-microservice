# Social Login Flow

This diagram illustrates the sequence of interactions during social login.

## Sequence Diagram

```mermaid
sequenceDiagram
    participant User
    participant Client
    participant API as API Gateway
    participant Auth as Auth Service
    participant Social as Social Provider
    participant Cache as Cache Service
    participant Events as Event Service

    User->>Client: Click Social Login
    Note over User,Client: Select provider (Google, GitHub, etc.)

    Client->>API: GET /v1/auth/social/{provider}/url
    Note over Client,API: Request auth URL

    API->>Auth: Get Auth URL
    Auth->>Social: Generate Auth URL
    Social-->>Auth: Auth URL
    Auth-->>API: Auth URL
    API-->>Client: 200 OK
    Note over Client,API: Return auth URL

    Client->>Social: Redirect to Auth URL
    Note over Client,Social: User authenticates with provider

    Social->>Client: Redirect with Code
    Note over Social,Client: Auth code returned

    Client->>API: POST /v1/auth/social/{provider}/callback
    Note over Client,API: Send auth code

    API->>Auth: Handle Social Callback
    Auth->>Social: Exchange Code for Token
    Social-->>Auth: Access Token

    Auth->>Social: Get User Profile
    Note over Auth,Social: Fetch user info
    Social-->>Auth: User Profile

    Auth->>Auth: Create/Update User
    Note over Auth: Link or create account

    Auth->>Auth: Generate Access Token
    Note over Auth: JWT with user claims

    Auth->>Auth: Generate Refresh Token
    Note over Auth: Long-lived refresh token

    Auth->>Cache: Store Refresh Token
    Note over Auth,Cache: Store with user ID
    Cache-->>Auth: Token Stored

    Auth->>Events: Publish Social Login Event
    Note over Auth,Events: Event includes provider and user ID
    Events-->>Auth: Event Published

    Auth-->>API: Authentication Success
    API-->>Client: 200 OK
    Note over Client,API: Return tokens and user info

    Note over Events: Async Event Processing
    Events->>Cache: Update User Session
    Events->>Cache: Link Social Account
```

## Description

This sequence diagram shows the complete flow of social login:

1. **Initial Request**

   - User initiates social login
   - Get authentication URL
   - Redirect to provider

2. **Provider Authentication**

   - User authenticates with provider
   - Provider returns auth code
   - Exchange code for token

3. **User Management**

   - Get user profile from provider
   - Create or link user account
   - Generate system tokens

4. **Session Management**
   - Store refresh token
   - Publish login event
   - Update user session

## Error Handling

```mermaid
sequenceDiagram
    participant User
    participant Client
    participant API as API Gateway
    participant Auth as Auth Service
    participant Social as Social Provider

    Client->>API: POST /v1/auth/social/{provider}/callback
    Note over Client,API: Invalid auth code

    API->>Auth: Handle Social Callback
    Auth->>Social: Exchange Code for Token
    Social-->>Auth: Invalid Code

    Auth-->>API: Authentication Failed
    API-->>Client: 400 Bad Request

    Note over Client,API: Retry with valid code
    Client->>API: POST /v1/auth/social/{provider}/callback
    Note over Client,API: Valid auth code

    API->>Auth: Handle Social Callback
    Auth->>Social: Exchange Code for Token
    Social-->>Auth: Access Token

    Auth->>Social: Get User Profile
    Social-->>Auth: Profile Unavailable

    Auth-->>API: Profile Error
    API-->>Client: 503 Service Unavailable
```

## Notes

- Multiple provider support
- Account linking capability
- Profile synchronization
- Token management
- Session tracking
- Event publishing
- Error handling
- Rate limiting
- Security measures
- Audit logging
- User data mapping
- Provider metadata
- Account merging
- Profile updates
- Session management
