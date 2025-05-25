INITIAL CONTEXT FOR LLM - never change the context-----------------------------
-> THIS SECTION IS A GUIDELINE TO THE LLM CONSIDER BEFORE WORKING IN THIS FILE, DO NOT CHANGE THIS

-> GOES OF THE README FILE:

- This file serves as the technical documentation of the service, providing a comprehensive overview of the codebase
- It should document:
  - Service architecture and design decisions
  - Component structure and relationships
  - API endpoints and interfaces
  - Dependencies and integration points
  - Configuration and deployment details
- This is the primary reference for understanding the technical implementation
- This file should be in sync with the `/TRACKER&MANAGER.md` where development progress and tasks are tracked
- While TRACKER&MANAGER.md focuses on "what" and "when", this file focuses on "how" and "why"

-> CONSIDERER BEFORE UPDATING THIS FILE:

- Never add fictional dates, version numbers, or metrics. Only include real, verified information. If information is not available, mark it as "To be determined" or remove the section.
- The changes in this file need to be incremental or to update informations that you confidentilly have knowlegde, they should not be guesses. If there are questions or uncertanty add comments asking for clarification instead.
- Check the `../../microservices/docs` folder for comprehensive project details, including architecture, development guidelines, and integration points. This will help in making informed decisions, haver better context and updates to the development plan. Always compare the implementation of this project with the plan described in the docs and whenever there are inconsistancies, add comments.
- Consider structuring this documentation separating the components and describing each one of them and adding sections to how they interact with each other - because this will be very dinamic and updated during the development process it will make clear what to update after each change
- This documentation is focusing in the current component that is part of a larger project, to have a more sistemic view check `../../microservices/TRACKER&MANAGER` and `../../microservices/README`
- Do not forget to be LLM focus, so because this will be used
- For LLM-specific guidelines and patterns, refer to [LLM Integration Guide](../../docs/llm/README.md)

---

# Profile API Service

## Overview

The Profile API Service is a microservice responsible for managing user profiles and handling authentication through integration with the Auth Service. It provides a RESTful API for profile management operations while ensuring secure access through token-based authentication.

## Architecture

### Core Components

1. **Session Management**

   ```go
   // File: internal/session/session.go
   type SessionManagerInterface interface {
       CreateSession(userID, password string) (string, error)
       ValidateSession(tokenString string) (*Session, error)
       InvalidateSession(tokenString string) error
       Close() error
   }

   type Session struct {
       UserID    string    `json:"user_id"`
       Role      string    `json:"role"`
       CreatedAt time.Time `json:"created_at"`
       ExpiresAt time.Time `json:"expires_at"`
   }
   ```

   The service uses Redis for session management with the following features:

   - Persistent session storage in Redis (in-cluster)
   - Automatic session expiration after 24 hours
   - Thread-safe operations with context timeouts
   - Integration with Auth Service for token validation
   - Support for session creation, validation, and invalidation
   - Maintains session state with user ID and role
   - Handles session expiration
   - Configurable through environment variables
   - In-cluster deployment with persistent storage
   - Health monitoring and probes
   - Automatic failover support

   Configuration:

   ```bash
   # Redis Configuration
   REDIS_ADDR=redis:6379        # In-cluster Redis service
   REDIS_PASSWORD=              # Redis password (if any)
   REDIS_DB=0                   # Redis database number
   ```

   Usage:

   ```go
   // Create a new session manager
   sessionManager, err := session.NewSessionManager(authClient)
   if err != nil {
       log.Fatalf("Failed to create session manager: %v", err)
   }
   defer sessionManager.Close()

   // Create a new session
   token, err := sessionManager.CreateSession(userID, password)
   if err != nil {
       // Handle error
   }

   // Validate a session
   session, err := sessionManager.ValidateSession(token)
   if err != nil {
       // Handle error
   }
   ```

2. **Auth Service Integration**

   ```go
   // File: internal/services/auth.go
   type AuthServiceClient struct {
       client  *http.Client
       baseURL string
   }

   func NewAuthServiceClient(cfg *config.Config) *AuthServiceClient {
       return &AuthServiceClient{
           client: &http.Client{
               Timeout: time.Second * 5,
           },
           baseURL: cfg.Auth.URL,  // Uses the URL directly from config
       }
   }
   ```

   - HTTP client for Auth Service communication
   - Token generation and validation
   - Error handling with retries
   - Secure communication with timeouts
   - Important: In Kubernetes, the auth service is exposed on port 80, so the URL should be `http://auth-service` without port specification
   - The service will automatically map port 80 to the container's port 8080

3. **Profile Management**

   ```go
   // File: internal/services/profile.go
   type ProfileServiceInterface interface {
       GetProfiles(ctx context.Context) ([]*models.Profile, error)
       GetProfile(ctx context.Context, id string) (*models.Profile, error)
       CreateProfile(ctx context.Context, req *models.ProfileRequest) (*models.Profile, error)
       UpdateProfile(ctx context.Context, id string, req *models.ProfileRequest) (*models.Profile, error)
       DeleteProfile(ctx context.Context, id string) error
   }
   ```

   - CRUD operations for user profiles
   - Integration with Storage Service
   - Input validation and error handling
   - Protected by session middleware
   - Comprehensive logging
   - Error tracking and metrics

4. **Storage Service Integration**

   ```go
   // File: internal/services/storage.go
   type StorageClient struct {
       client  *http.Client
       baseURL string
       config  *config.StorageConfig
       auth    *config.SecurityConfig
   }
   ```

   - HTTP client with connection pooling
   - Retry mechanism with configurable attempts
   - Error handling with custom error types
   - Request ID tracking for debugging
   - Metrics collection for all operations
   - Secure communication with JWT support
   - Automatic request/response logging

5. **Metrics and Monitoring**

   ```go
   // File: internal/metrics/metrics.go
   type StorageMetrics struct {
       // Operation counts
       GetProfileCount     int64
       GetProfilesCount    int64
       CreateProfileCount  int64
       UpdateProfileCount  int64
       DeleteProfileCount  int64
       ErrorCount          int64

       // Latency measurements
       GetProfileLatency     time.Duration
       GetProfilesLatency    time.Duration
       CreateProfileLatency  time.Duration
       UpdateProfileLatency  time.Duration
       DeleteProfileLatency  time.Duration

       // Last operation timestamps
       LastGetProfile     time.Time
       LastGetProfiles    time.Time
       LastCreateProfile  time.Time
       LastUpdateProfile  time.Time
       LastDeleteProfile  time.Time
       LastError          time.Time
   }
   ```

   - Operation counts and latencies
   - Error tracking
   - Last operation timestamps
   - Thread-safe metrics collection
   - Metrics reset capability
   - Exposed via `/metrics` endpoint

### Service Dependencies

1. **Auth Service**

   ```go
   // File: internal/services/auth.go
   func (c *AuthServiceClient) GetToken(ctx context.Context, userID, role string) (string, error) {
       // Implementation details
   }

   func (c *AuthServiceClient) ValidateToken(ctx context.Context, token string) (*ValidateResponse, error) {
       // Implementation details
   }
   ```

   - Token generation and validation
   - User authentication
   - Role-based access control
   - Error handling with retries

2. **Storage Service**

   ```go
   // File: internal/services/storage.go
   type StorageError struct {
       Code    int    `json:"code"`
       Message string `json:"message"`
       Err     error  `json:"-"`
   }
   ```

   - HTTP-based communication
   - Retry mechanism
   - Error handling
   - Metrics collection
   - Connection pooling
   - Request ID tracking

3. **Redis Service**
   ```go
   // File: internal/session/session.go
   type SessionManager struct {
       authClient *services.AuthServiceClient
       redis      *redis.Client
       ctx        context.Context
   }
   ```
   - Session storage with expiration
   - Mock implementation for development
   - Thread-safe operations
   - Automatic cleanup

## Implementation Details

### Project Structure

```
profile-api/
├── cmd/
│   └── main.go           # Application entry point
├── internal/
│   ├── config/          # Configuration management
│   ├── handlers/        # HTTP request handlers
│   ├── middleware/      # HTTP middleware
│   ├── metrics/         # Metrics collection
│   ├── services/        # Business logic and external service clients
│   └── session/         # Session management
└── guidance/           # Documentation and development guidelines
```

### API Endpoints

1. **Authentication**

   ```go
   // File: internal/handlers/auth.go
   func (h *AuthHandler) Authenticate(c *gin.Context) {
       // Implementation details
   }

   func (h *AuthHandler) ValidateToken(c *gin.Context) {
       // Implementation details
   }
   ```

   - `POST /api/v1/auth/token` - Get authentication token
   - `POST /api/v1/auth/validate` - Validate authentication token

2. **Profile Management**

   ```go
   // File: internal/handlers/profile.go
   func (h *ProfileHandler) GetProfiles(c *gin.Context) {
       // Implementation details
   }

   func (h *ProfileHandler) GetProfile(c *gin.Context) {
       // Implementation details
   }
   ```

   - `GET /api/v1/profiles` - List profiles
   - `GET /api/v1/profiles/{id}` - Get profile by ID
   - `POST /api/v1/profiles` - Create profile
   - `PUT /api/v1/profiles/{id}` - Update profile
   - `DELETE /api/v1/profiles/{id}` - Delete profile

3. **Metrics**
   ```go
   // File: internal/handlers/metrics.go
   type MetricsHandler struct {
       metrics *metrics.StorageMetrics
   }
   ```
   - `GET /metrics` - Get current metrics
   - `DELETE /metrics` - Reset metrics

### API Examples

#### Authentication Flow

1. **Get Authentication Token**

   ```bash
   # Request a new authentication token
   curl -X POST http://profile-api/api/v1/auth/token \
     -H "Content-Type: application/json" \
     -d '{
       "user_id": "user1",
       "password": "123456"
     }' | jq '.'

   # Example Response
   {
     "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
     "error": null
   }
   ```

2. **Use Token for Profile Operations**

   ```bash
   # Use the token for profile operations
   curl -X GET http://profile-api/api/v1/profiles \
     -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..." | jq '.'
   ```

Note: The Profile API handles authentication by:

1. Getting tokens from the auth service
2. Storing sessions in Redis
3. Validating tokens with both Redis and the auth service
4. Managing session expiration

#### Profile Management Endpoints

Note: The service is accessible via the service name `profile-api` in the cluster. When accessing from within the cluster, use `http://profile-api` as the base URL. When accessing from outside the cluster, use the appropriate external URL or port-forwarding.

All endpoints have been verified working from within the cluster, with successful communication to the profile-storage service.

1. **List Profiles**

   ```bash
   # Get all profiles (from within cluster)
   curl -X GET http://profile-api/api/v1/profiles \
     -H "Authorization: Bearer mock_access_token" | jq '.'

   # Example Response
   {
     "profiles": [
       {
         "id": "profile123",
         "user_id": "user123",
         "first_name": "John",
         "last_name": "Doe",
         "email": "john.doe@example.com",
         "created_at": "2024-01-01T00:00:00Z",
         "updated_at": "2024-01-01T00:00:00Z"
       }
     ],
     "error": null
   }
   ```

2. **Get Profile by ID**

   ```bash
   # Get a specific profile
   curl -X GET http://profile-api/api/v1/profiles/user1 \
     -H "Authorization: Bearer mock_access_token" | jq '.'

   # Example Response
   {
     "id": "profile123",
     "user_id": "user123",
     "first_name": "John",
     "last_name": "Doe",
     "email": "john.doe@example.com",
     "created_at": "2024-01-01T00:00:00Z",
     "updated_at": "2024-01-01T00:00:00Z",
     "error": null
   }
   ```

3. **Create Profile**

   ```bash
   # Create a new profile
   curl -X POST http://profile-api/api/v1/profiles \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer mock_access_token" \
     -d '{
       "first_name": "John",
       "last_name": "Doe",
       "email": "john.doe2@example.com"
     }' | jq '.'

   # Example Response
   {
     "id": "profile123",
     "user_id": "user123",
     "first_name": "John",
     "last_name": "Doe",
     "email": "john.doe@example.com",
     "created_at": "2024-01-01T00:00:00Z",
     "updated_at": "2024-01-01T00:00:00Z",
     "error": null
   }
   ```

4. **Update Profile**

   ```bash
   # Update an existing profile
   curl -X PUT http://profile-api/api/v1/profiles/profile123 \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer mock_access_token" \
     -d '{
       "first_name": "John",
       "last_name": "Smith",
       "email": "john.smith@example.com"
     }' | jq '.'

   # Example Response
   {
     "id": "profile123",
     "user_id": "user123",
     "first_name": "John",
     "last_name": "Smith",
     "email": "john.smith@example.com",
     "created_at": "2024-01-01T00:00:00Z",
     "updated_at": "2024-01-02T00:00:00Z",
     "error": null
   }
   ```

5. **Delete Profile**

   ```bash
   # Delete a profile
   curl -X DELETE http://profile-api/api/v1/profiles/profile123 \
     -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..." | jq '.'

   # Example Response
   {
     "error": null
   }
   ```

Note:

- The service is accessible via the service name `profile-api` in the cluster
- When accessing from within the cluster, use `http://profile-api` as the base URL
- When accessing from outside the cluster, use the appropriate external URL or port-forwarding
- The token in the examples is a placeholder - use the actual token received from the token endpoint
- All profile endpoints require a valid authentication token
- Error responses will include an error message in the "error" field
- The `jq '.'` command is used to pretty-print JSON responses. You can also use:
  - `jq -C '.'` for colored output
  - `jq -r '.'` for raw output (no quotes around strings)
  - `jq '.field_name'` to extract specific fields

## Configuration

The service uses a centralized configuration system that is loaded at startup and passed to components that need it. The configuration is structured to support different environments and service integrations.

```go
// File: internal/config/config.go
type Config struct {
    Server      ServerConfig
    Auth        AuthConfig
    Redis       RedisConfig
    Environment string
    Storage     StorageConfig
    Cache       CacheConfig
    Queue       QueueConfig
    Security    SecurityConfig
}
```

#### Configuration Usage

1. **Auth Service Client**

   ```go
   // File: internal/services/auth.go
   type AuthServiceClient struct {
       client  *http.Client
       baseURL string
   }

   func NewAuthServiceClient(cfg *config.Config) *AuthServiceClient {
       return &AuthServiceClient{
           client: &http.Client{
               Timeout: time.Second * 5,
           },
           baseURL: cfg.Auth.URL,  // Uses the URL directly from config
       }
   }
   ```

   - Uses auth service URL from configuration
   - Follows principle of least privilege
   - Configures HTTP client with timeouts
   - Relies on auth service for token validation

2. **Profile Service**

   ```go
   // File: internal/services/profile.go
   type ProfileService struct {
       storageClient *StorageClient
   }

   func NewProfileService(cfg *config.Config, storageClient *StorageClient) *ProfileService {
       return &ProfileService{
           storageClient: storageClient,
       }
   }
   ```

   - Uses dependency injection for storage client
   - Receives configuration through constructor
   - Delegates storage configuration to StorageClient

3. **Storage Client**

   ```go
   // File: internal/services/storage.go
   type StorageClient struct {
       client  *http.Client
       baseURL string
       config  *config.StorageConfig
   }

   func NewStorageClient(cfg *config.StorageConfig) *StorageClient {
       return &StorageClient{
           client: &http.Client{
               Timeout: time.Second * 5,
           },
           baseURL: fmt.Sprintf("http://%s:%d", cfg.Host, cfg.Port),
           config:  cfg,
       }
   }
   ```

   - Uses storage-specific configuration
   - Implements retry mechanism using config values
   - Configures connection timeouts and retries

4. **Session Management**

   ```go
   // File: internal/session/session.go
   type SessionManager struct {
       authClient *services.AuthServiceClient
       redis      *redis.Client
       ctx        context.Context
   }

   // File: internal/session/mock_session.go
   type DevSessionManager struct {
       authClient *services.AuthServiceClient
       sessions   map[string]*Session
       mu         sync.RWMutex
   }
   ```

   - Redis configuration for production
   - In-memory storage for development
   - Configurable through environment variables
   - Automatic session cleanup

#### Configuration Loading

The configuration is loaded at application startup using environment variables with sensible defaults:

```go
// File: internal/config/config.go
func LoadConfig() *Config {
    env := getEnv("ENV", "development")
    return &Config{
        Server: ServerConfig{
            Host: getEnv("SERVER_HOST", "0.0.0.0"),
            Port: getEnvAsInt("SERVER_PORT", 8080),
        },
        Auth: AuthConfig{
            Host: getEnv("AUTH_SERVICE_HOST", "localhost"),
            Port: getEnvAsInt("AUTH_SERVICE_PORT", 8081),
        },
        // ... other configurations
    }
}
```

#### Environment Variables

```bash
# Server Configuration
SERVER_HOST=0.0.0.0
SERVER_PORT=8080

# Auth Service Configuration
AUTH_SERVICE_URL=http://auth-service
AUTH_SERVICE_HOST=localhost
AUTH_SERVICE_PORT=8081

# Redis Configuration
REDIS_ADDR=redis:6379          # In-cluster Redis service
REDIS_PASSWORD=
REDIS_DB=0

# Storage Configuration
STORAGE_HOST=localhost
STORAGE_PORT=27017
STORAGE_DATABASE=profile_service
STORAGE_TYPE=memory
STORAGE_MAX_RETRIES=3
STORAGE_RETRY_DELAY_MS=100

# Cache Configuration
CACHE_HOST=localhost
CACHE_PORT=6379
CACHE_ENABLED=false

# Queue Configuration
QUEUE_HOST=localhost
QUEUE_PORT=5672
QUEUE_ENABLED=false

# Security Configuration
SECURITY_ENABLED=true

# Environment
ENV=development
```

### Configuration Best Practices

1. **Dependency Injection**

   - Components receive only the configuration they need
   - Dependencies are injected through constructors
   - Services are decoupled from configuration details

2. **Environment Support**

   - Development mode with mock services
   - Production-ready configuration
   - Easy to switch between environments
   - Kubernetes service discovery using service names
   - Local development using explicit host and port

3. **Security**

   - Sensitive values loaded from environment
   - Default values for development only
   - Security features can be toggled

4. **Service Integration**
   - Each service has its own configuration section
   - Retry mechanisms configurable per service
   - Timeouts and connection settings customizable
   - In Kubernetes, use service names (e.g., `redis:6379`)
   - The Kubernetes service will handle port mapping automatically

## Development

### Prerequisites

- Go 1.21 or later
- Docker (optional)
- Redis (optional, mock available)

### Setup

```bash
# Clone repository
git clone [repository-url]

# Install dependencies
go mod download

# Run with in-memory session storage (development mode)
USE_MOCK_REDIS=true go run cmd/main.go

# Run with Redis (production mode)
USE_MOCK_REDIS=false go run cmd/main.go
```

### Testing

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...
```

## Security

1. **Authentication**

   ```go
   // File: internal/middleware/auth.go
   func SessionMiddleware(sessionManager handlers.SessionManagerInterface) gin.HandlerFunc {
       // Implementation details
   }
   ```

   The service implements a comprehensive authentication system:

   - Token-based authentication through the auth service
   - Session management with Redis for persistence
   - Role-based access control
   - Secure token validation with both Redis and auth service
   - Session expiration handling
   - Automatic session cleanup

   The authentication flow:

   1. User sends credentials to `/api/v1/auth/token`
   2. Service gets token from auth service
   3. Token and session info stored in Redis
   4. Token returned to user
   5. Subsequent requests validated against both Redis and auth service

2. **Data Protection**

   ```go
   // File: internal/middleware/security.go
   func SecurityMiddleware(next http.Handler) http.Handler {
       // Implementation details
   }
   ```

   - HTTPS enforcement
   - Input validation
   - Output sanitization
   - Error handling

3. **Request Tracking**
   ```go
   // File: internal/services/storage.go
   if requestID := ctx.Value("request_id"); requestID != nil {
       req.Header.Set("X-Request-ID", requestID.(string))
   }
   ```
   - Request ID tracking
   - Request/response logging
   - Error tracking
   - Performance monitoring

## Monitoring

1. **Health Checks**

   ```go
   // File: internal/handlers/health.go
   func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
       // Implementation details
   }
   ```

   - `GET /health` endpoint
   - Service status
   - Dependency checks
   - Resource monitoring

2. **Metrics**

   ```go
   // File: internal/metrics/metrics.go
   func RecordGetProfile(duration time.Duration) {
       metrics.mu.Lock()
       defer metrics.mu.Unlock()
       metrics.GetProfileCount++
       metrics.GetProfileLatency = duration
       metrics.LastGetProfile = time.Now()
   }
   ```

   - Operation counts
   - Latency measurements
   - Error tracking
   - Last operation timestamps
   - Thread-safe collection
   - Reset capability

3. **Logging**

   ```go
   // File: internal/logger/logger.go
   type Config struct {
       Level       string
       Environment string
       ServiceName string
   }

   func LogRequest(ctx context.Context, method, path string, status int, duration time.Duration) {
       WithContext(ctx).Info("http request",
           zap.String("method", method),
           zap.String("path", path),
           zap.Int("status", status),
           zap.Duration("duration", duration),
       )
   }
   ```

   - **Structured JSON logging** using zap (fully integrated)
   - **Context-aware logging** with request IDs (used throughout the service)
   - **Log levels** (DEBUG, INFO, ERROR) and filtering
   - **Performance metrics** in logs
   - **Stack traces** for errors
   - **Environment and service context**
   - **Request/response logging**
   - **Error tracking with context**
   - **Operation logging with metrics**
   - **Dependency issues resolved** (zap and multierr)
   - **Consistent usage** across all major service components

   **Configuration:**

   ```bash
   # Logging Configuration
   LOG_LEVEL=info              # Log level (debug, info, warn, error)
   LOG_ENVIRONMENT=production  # Environment (development, production)
   LOG_SERVICE_NAME=profile-api # Service name for log context
   ```

   **Usage:**

   ```go
   // Initialize logger
   loggerCfg := &logger.Config{
       Level:       "info",
       Environment: cfg.Environment,
       ServiceName: "profile-api",
   }
   if err := logger.Initialize(loggerCfg); err != nil {
       log.Fatalf("Failed to initialize logger: %v", err)
   }
   defer logger.Sync()

   // Log with context
   logger.LogInfo(ctx, "Operation completed",
       zap.String("operation", "create_profile"),
       zap.Duration("duration", duration))
   ```

   **Current State:**

   - All logger dependencies are present and up to date
   - Logger is initialized at startup and used in all major flows
   - Logging is consistent, structured, and context-rich
   - Ready for further enhancements (rotation, aggregation, etc.)

## References

- [Development Plan](./DEVELOPMENT_PLAN.md)
- [API Documentation](../docs/api/README.md)
- [Architecture Overview](../docs/architecture/README.md)

## Logging System

### Overview

The service implements a comprehensive logging system using the `zap` logger, designed for high performance and structured logging. The system includes log rotation, log shipping with buffering, and retry mechanisms for reliable log delivery.

### Core Components

1. **Logger Configuration**

   ```go
   // File: internal/logger/logger.go
   type Config struct {
       Level       string
       Environment string
       ServiceName string
       Format      string // "json" or "console"
       LogFile     string // Path to log file for rotation
       Shipping    *ShippingConfig
   }
   ```

   The logger supports:

   - Multiple log levels (DEBUG, INFO, ERROR)
   - JSON and console formats
   - Environment-specific configurations
   - Service context injection
   - Log file rotation
   - Log shipping configuration

2. **Log Rotation**

   ```go
   // File: internal/logger/logger.go
   writeSyncer := zapcore.AddSync(&lumberjack.Logger{
       Filename:   cfg.LogFile,
       MaxSize:    100, // MB
       MaxBackups: 3,
       MaxAge:     28, // days
       Compress:   true,
   })
   ```

   Features:

   - Size-based rotation (100MB per file)
   - Maximum of 3 backup files
   - 28-day retention period
   - Automatic compression
   - Thread-safe operations

3. **Log Shipping**

   ```go
   // File: internal/logger/shipper.go
   type LogShipper struct {
       client      *http.Client
       endpoint    string
       buffer      []map[string]interface{}
       bufferSize  int
       bufferMutex sync.Mutex
       maxRetries  int
       retryDelay  time.Duration
   }
   ```

   Features:

   - Buffered log shipping
   - Configurable buffer size
   - Retry mechanism with backoff
   - Thread-safe operations
   - Automatic batch shipping
   - Error handling and recovery

### Configuration

```bash
# Logging Configuration
LOG_LEVEL=info                    # Log level (debug, info, warn, error)
LOG_FORMAT=json                   # Log format (json or console)
LOG_FILE=app.log                  # Log file path
LOG_SHIPPING_ENABLED=true         # Enable log shipping
LOG_SHIPPING_ENDPOINT=http://log-shipping-service  # Log shipping endpoint
LOG_SHIPPING_BUFFER_SIZE=100      # Number of logs to buffer before shipping
LOG_SHIPPING_MAX_RETRIES=3        # Maximum number of retry attempts
LOG_SHIPPING_RETRY_DELAY_MS=100   # Delay between retries in milliseconds
```

### Usage Examples

1. **Basic Logging**

   ```go
   // Initialize logger
   loggerCfg := &logger.Config{
       Level:       cfg.Logging.Level,
       Environment: cfg.Environment,
       ServiceName: "profile-api",
       Format:      cfg.Logging.Format,
       LogFile:     cfg.Logging.LogFile,
       Shipping: &logger.ShippingConfig{
           Enabled:    cfg.Logging.Shipping.Enabled,
           Endpoint:   cfg.Logging.Shipping.Endpoint,
           BufferSize: cfg.Logging.Shipping.BufferSize,
           MaxRetries: cfg.Logging.Shipping.MaxRetries,
           RetryDelay: cfg.Logging.Shipping.RetryDelay,
       },
   }
   if err := logger.Initialize(loggerCfg); err != nil {
       log.Fatalf("Failed to initialize logger: %v", err)
   }
   defer logger.Sync()
   ```

2. **Context-Aware Logging**

   ```go
   // Log with context
   logger.LogInfo(ctx, "Operation completed",
       zap.String("operation", "create_profile"),
       zap.Duration("duration", duration))

   // Log errors with stack trace
   logger.LogError(ctx, "Failed to process request",
       zap.Error(err),
       zap.String("request_id", requestID))
   ```

3. **Request Logging**

   ```go
   // Log HTTP requests
   logger.LogRequest(ctx, method, path, status, duration)
   ```

### Log Structure

1. **Standard Fields**

   ```json
   {
     "timestamp": "2024-01-01T00:00:00Z",
     "level": "info",
     "service": "profile-api",
     "environment": "production",
     "request_id": "123e4567-e89b-12d3-a456-426614174000",
     "message": "Operation completed",
     "operation": "create_profile",
     "duration": "0.123s"
   }
   ```

2. **Error Logs**

   ```json
   {
     "timestamp": "2024-01-01T00:00:00Z",
     "level": "error",
     "service": "profile-api",
     "environment": "production",
     "request_id": "123e4567-e89b-12d3-a456-426614174000",
     "message": "Failed to process request",
     "error": "connection refused",
     "stacktrace": "..."
   }
   ```

### Performance Considerations

1. **Buffering**

   - Logs are buffered in memory before shipping
   - Configurable buffer size to balance memory usage and shipping frequency
   - Automatic shipping when buffer is full
   - Manual flush available for critical logs

2. **Retry Mechanism**

   - Exponential backoff for retries
   - Configurable retry count and delay
   - Error tracking for failed shipments
   - Automatic recovery after temporary failures

3. **Resource Usage**
   - Memory-efficient buffer management
   - Thread-safe operations
   - Non-blocking log shipping
   - Automatic cleanup of old log files

### Monitoring and Maintenance

1. **Log Shipping Metrics**

   - Number of logs shipped
   - Shipping latency
   - Buffer utilization
   - Retry attempts
   - Failed shipments

2. **Log Rotation Metrics**

   - Current log file size
   - Number of backup files
   - Disk space usage
   - Rotation frequency

3. **Health Checks**
   - Log shipping status
   - Buffer health
   - Disk space monitoring
   - Rotation status

### Best Practices

1. **Log Levels**

   - Use DEBUG for detailed troubleshooting
   - Use INFO for normal operations
   - Use ERROR for exceptional conditions
   - Include relevant context in all logs

2. **Context**

   - Always include request ID
   - Add operation-specific fields
   - Include timing information
   - Add relevant error details

3. **Performance**

   - Use appropriate buffer sizes
   - Configure retry policies
   - Monitor resource usage
   - Regular cleanup of old logs

4. **Security**
   - Sanitize sensitive data
   - Use appropriate log levels
   - Configure proper access controls
   - Monitor log access

### Dependencies

```go
require (
    go.uber.org/zap v1.26.0
    gopkg.in/natefinch/lumberjack.v2 v2.2.1
)
```

### Future Enhancements

1. **Planned Features**

   - Log sampling for high-volume endpoints
   - Advanced log analytics
   - Custom log processors
   - Enhanced monitoring

2. **Integration Points**
   - ELK stack integration
   - Prometheus metrics
   - Grafana dashboards
   - Alerting system
