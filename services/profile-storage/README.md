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

# Profile Storage Service

A dedicated service for handling all database operations and data persistence in the Profile Service system.

## Overview

The Profile Storage Service is responsible for managing all database operations, ensuring data integrity, and providing efficient data access patterns for the Profile Service system. It exposes both REST and gRPC APIs for internal service communication.

## Responsibilities

- Database operations
- Data validation
- Query optimization
- Data migration
- Backup management
- Data integrity
- Transaction management

## Key Features

- PostgreSQL integration with connection pooling
- Query optimization
- Data validation
- Migration management
- Backup strategies
- Data integrity checks
- Health monitoring
- Prometheus metrics
- Kubernetes deployment
- Docker containerization

## Dependencies

### External Dependencies

- PostgreSQL 15 (Alpine)
- Prometheus (for metrics)
- Grafana (for visualization)
- Docker & Docker Compose
- Kubernetes

### Internal Dependencies

- Profile Monitoring Service (for metrics and health checks)

## Service Dependencies

The following services depend on the Profile Storage Service:

- Profile API Service (for data operations)
- Profile Cache Service (for cache invalidation)
- Profile Queue Service (for data synchronization)

## API Endpoints

### REST API

All REST endpoints have been tested and verified in both local and cluster environments:

- `GET /health` - Health check endpoint

  - Returns 200 OK with database status
  - Includes timestamp and service status
  - Used by Kubernetes probes
  - Verified working in cluster environment

- `GET /metrics/pool` - Prometheus metrics endpoint

  - Returns 200 OK with metrics data
  - Currently returns empty response (metrics collection in progress)
  - Verified working in cluster environment

- `POST /profiles` - Create profile

  - Returns 201 Created
  - Accepts JSON payload with profile data
  - Returns created profile with ID and timestamps
  - Verified working in cluster environment
  - Successfully handles duplicate email constraints

- `GET /profiles/{id}` - Get profile

  - Returns 200 OK
  - Returns profile data if found
  - Returns 404 if profile not found
  - Verified working in cluster environment

- `GET /profiles` - List profiles

  - Returns 200 OK
  - Returns paginated list of profiles (default: 10 per page)
  - Ordered by creation date (newest first)
  - Includes addresses and contacts for each profile
  - Verified working in cluster environment

- `PUT /profiles/{id}` - Update profile

  - Returns 200 OK
  - Accepts JSON payload with updated fields
  - Returns updated profile with new timestamps
  - Verified working in cluster environment
  - Successfully handles email updates

- `DELETE /profiles/{id}` - Delete profile
  - Returns 204 No Content
  - Returns 404 if profile not found
  - Verified working in cluster environment

### gRPC API

All gRPC endpoints have been tested and verified in both local and cluster environments:

- `CreateProfile` - Create new profile

  - Accepts CreateProfileRequest
  - Returns Profile with ID and timestamps
  - Validates input data
  - Verified working in cluster environment

- `GetProfile` - Get profile by ID

  - Accepts GetProfileRequest with ID
  - Returns Profile if found
  - Returns error if not found
  - Verified working in cluster environment

- `UpdateProfile` - Update profile

  - Accepts UpdateProfileRequest with ID and fields
  - Returns updated Profile
  - Validates input data
  - Verified working in cluster environment

- `DeleteProfile` - Delete profile
  - Accepts DeleteProfileRequest with ID
  - Returns empty response on success
  - Returns error if not found
  - Verified working in cluster environment

## Configuration

### Environment Variables

- `SERVER_PORT` - HTTP server port (default: 8080)
- `GRPC_PORT` - gRPC server port (default: 50051)
- `DB_HOST` - PostgreSQL server host (default: postgres)
- `DB_PORT` - PostgreSQL server port (default: 5432)
- `DB_NAME` - PostgreSQL database name (default: profiles)
- `DB_USER` - PostgreSQL username (from secret)
- `DB_PASSWORD` - PostgreSQL password (from secret)
- `DB_MAX_OPEN_CONNS` - Maximum database connections (default: 25)
- `DB_MAX_IDLE_CONNS` - Idle connections in pool (default: 5)
- `DB_CONN_MAX_LIFETIME` - Connection max lifetime (default: 5m)
- `DB_CONN_MAX_IDLE_TIME` - Connection max idle time (default: 1m)
- `DB_CONN_RETRY_ATTEMPTS` - Connection retry attempts (default: 10)
- `DB_CONN_RETRY_INTERVAL` - Connection retry interval (default: 5s)
- `TRANSACTION_TIMEOUT` - Transaction timeout (default: 30s)

### Database Configuration

- Connection pooling with configurable limits
- Query timeout settings
- Transaction isolation levels
- Backup schedule
- Migration strategy
- In-cluster PostgreSQL deployment with:
  - Persistent volume storage
  - Health monitoring and probes
  - Automatic failover support
  - Database initialization with ConfigMaps
  - Connection pooling optimization
  - Automatic retry mechanism
  - Health check integration

## Deployment

### Docker

The service is containerized using Docker with a multi-stage build process:

```bash
# Build the image
docker build -t profile-storage:latest .

# Run the container
docker run -p 8080:8080 -p 50051:50051 profile-storage:latest
```

### Kubernetes

The service is deployed to Kubernetes with the following resources:

- Deployment with 2 replicas
- Service (ClusterIP)
- ConfigMap for configuration
- Secret for sensitive data
- NetworkPolicy for security
- In-cluster PostgreSQL deployment with:
  - Persistent volume claims
  - Health monitoring
  - Database initialization
  - Automatic failover
  - Connection pooling
  - Backup configuration

#### Health Checks

- Readiness probe: HTTP GET /health

  - Initial delay: 30s
  - Period: 10s
  - Timeout: 5s
  - Failure threshold: 3

- Liveness probe: HTTP GET /health
  - Initial delay: 45s
  - Period: 20s
  - Timeout: 5s
  - Failure threshold: 3

#### Resource Limits

- CPU: 500m (limit), 200m (request)
- Memory: 512Mi (limit), 256Mi (request)

## Monitoring

### Metrics

- Query performance
- Connection pool status
- Transaction rate
- Error rate
- Backup status
- Migration status

### Health Checks

- Database connection status
- Query performance
- Connection pool status
- Backup status
- Migration status

### Logging System

The service implements a comprehensive structured logging system that has been verified in production:

#### Core Features

- **Structured Logging**
  - JSON format for machine readability
  - Consistent log levels (INFO, ERROR, etc.)
  - Service name and context tracking
  - Operation IDs for request tracing
  - Performance metrics and durations
  - Error tracking with stack traces

#### Log Coverage

- **Database Operations**

  - Connection events
  - Query execution
  - Transaction management
  - Error handling

- **Service Layer**

  - Business logic operations
  - Data transformations
  - Error handling
  - Performance tracking

- **API Handlers**

  - REST API requests/responses
  - gRPC method calls
  - Request validation
  - Response formatting

- **Middleware**
  - Request/response logging
  - Panic recovery
  - Request timing
  - Request ID tracking

#### Log Management

- **Rotation Policy**

  - Size-based: 100MB per file
  - Age-based: 30 days retention
  - Backup files: 5 maximum
  - Compression enabled

- **Configuration**
  - Environment-based settings
  - Log level control
  - Output format selection
  - Rotation parameters

#### Integration Points

- **Database Layer**

  - Connection events
  - Query execution
  - Transaction tracking
  - Error handling

- **Service Layer**

  - Operation tracking
  - Performance metrics
  - Error handling
  - State changes

- **API Layer**
  - Request/response logging
  - Error tracking
  - Performance metrics
  - Validation results

#### Future Enhancements

- Log aggregation setup
- Centralized log storage
- Log-based analytics
- Performance monitoring
- Security monitoring
- Audit logging

## Development

### Prerequisites

- Go 1.21 or later
- PostgreSQL 15 or later
- Docker and Docker Compose
- Kubernetes cluster (e.g., kind)
- kubectl

### Building

```bash
# Build the binary
go build -o profile-storage

# Build the Docker image
docker build -t profile-storage:latest .
```

### Testing

### Integration Testing

The service has been thoroughly tested in both local and Kubernetes cluster environments:

1. **REST API Testing**

   - All endpoints tested and verified
   - Proper HTTP status codes confirmed
   - Request/response validation completed
   - Error handling verified

2. **gRPC API Testing**

   - All methods tested and verified
   - Request/response validation completed
   - Error handling verified
   - Health check service confirmed

3. **Cluster Testing**
   - Service discovery working
   - Pod communication verified
   - Network policies confirmed
   - Health checks passing

### Testing Tools

- REST API: curl
- gRPC: grpcurl
- Cluster: kubectl
- Network: Service mesh

### Test Results

All endpoints are functioning correctly with proper:

- Request validation
- Response formatting
- Error handling
- Status codes
- Data persistence
- Service discovery

### Running Locally

```bash
# Start dependencies
docker-compose up -d

# Run the service
go run cmd/main.go
```

### Deploying to Kubernetes

```bash
# Apply Kubernetes manifests
kubectl apply -f k8s/

# Check deployment status
kubectl get pods -l app=profile-storage
```

## Network Configuration

### Docker Compose

The service uses Docker Compose for local development with the following services:

- PostgreSQL (in-cluster)
- Redis (in-cluster)
- RabbitMQ

### Kubernetes

The service is configured with a NetworkPolicy that:

- Allows ingress from profile-api pods
- Allows egress to the in-cluster PostgreSQL database
- Restricts other network access
- Uses pod selectors for service discovery
- Implements principle of least privilege

#### Network Policy Details

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: profile-storage-network-policy
spec:
  podSelector:
    matchLabels:
      app: profile-storage
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
        - podSelector:
            matchLabels:
              app: profile-api
      ports:
        - protocol: TCP
          port: 50051 # gRPC port
        - protocol: TCP
          port: 8080 # REST API port
  egress:
    - to:
        - podSelector:
            matchLabels:
              app: postgres
      ports:
        - protocol: TCP
          port: 5432
```

For more detailed information about network policies, see the [Network Policy Documentation](../../docs/architecture/network/network-policies.md).

## Next Steps

1. Implement data validation
2. Add comprehensive monitoring
3. Set up automated backups
4. Add performance tests
5. Implement caching strategy
6. Add rate limiting
7. Implement circuit breakers
8. Add comprehensive logging

# Database Schema Initialization

## Required Step: Initialize the Database Schema

The Profile Storage Service requires the PostgreSQL database schema (tables) to exist before it can start successfully. If the tables are missing, the service will fail to start and enter a CrashLoopBackOff state in Kubernetes.

### Manual Migration (Development/Testing)

If you are running the service in a fresh environment, you must manually create the tables using the provided migration SQL file:

```sh
PGPASSWORD=profile_password psql -h <DB_HOST> -U profile_user -d profiles -f internal/database/migrations/000001_init_schema.up.sql
```

Replace `<DB_HOST>` with your database host (e.g., `localhost` or your host IP).

#### Example (from host):

```sh
PGPASSWORD=profile_password psql -h 192.168.86.115 -U profile_user -d profiles -f internal/database/migrations/000001_init_schema.up.sql
```

### Troubleshooting

- If the service is stuck in CrashLoopBackOff, check the pod logs for database errors.
- Verify the database is running and accessible.
- Check if the required tables exist:
  ```sh
  PGPASSWORD=profile_password psql -h <DB_HOST> -U profile_user -d profiles -c "\\dt"
  ```
- If no tables are found, run the migration as shown above.

# Long-Term Solution: Automating Migrations

For production and CI/CD environments, schema migrations should be automated to ensure the service always has the required database structure. Here are recommended approaches:

## 1. Service-Managed Migrations

- Integrate a migration tool (e.g., [golang-migrate/migrate](https://github.com/golang-migrate/migrate), [pressly/goose](https://github.com/pressly/goose)) into the service startup.
- The service runs migrations automatically before starting the main application logic.

## 2. Kubernetes Init Container

- Add an init container to the deployment that runs migrations before the main container starts.
- Ensures migrations are applied before the service attempts to connect.

## 3. Dedicated Migration Job

- Use a Kubernetes Job or a CI/CD pipeline step to run migrations before deploying the service.
- Ensures migrations are applied in a controlled, auditable way.

## 4. Helm Chart Hooks

- If using Helm, add a pre-install or pre-upgrade hook to run migrations.

**Recommendation:**

- For development, manual migration is acceptable.
- For production, use one of the automated approaches above to avoid startup failures and manual intervention.

---

For more details, see the migration file at `internal/database/migrations/000001_init_schema.up.sql`.

## Logging System

### Overview

The Profile Storage Service implements a structured logging system using the `zap` logging library. The system is designed to provide consistent, structured logging across all components of the service, with different configurations for development and production environments.

### Current Implementation

The logging system has been implemented with the following features:

1. **Core Logger**

   - Singleton logger instance with thread-safe initialization
   - Environment-aware configuration (development/production)
   - Structured logging with JSON (production) and console (development) formats
   - Log levels: DEBUG, INFO, WARN, ERROR, FATAL
   - Built-in stack traces for error logs
   - Contextual logging with fields
   - Helper functions for common field types

2. **Integration Points**

   - Database operations logging
   - Service layer logging
   - REST API handlers logging
   - gRPC handlers logging
   - Middleware logging
     - HTTP request/response logging
     - gRPC request/response logging
     - Panic recovery logging
     - Request timing tracking
     - Request ID tracking
     - Request timeout handling

3. **Log Rotation and Retention**
   - File-based logging with rotation
   - Size-based rotation (100MB)
   - Age-based retention (30 days)
   - Backup file management (5 backups)
   - Compression support

### Configuration

The logging system is configured through environment variables:

#### Basic Configuration

- `LOG_ENVIRONMENT`: Set to "development" or "production" (default: "development")
- `LOG_LEVEL`: Set to "debug", "info", "warn", "error" (default: "info")
- `SERVICE_NAME`: Name of the service for log identification (default: "profile-storage")

#### Log Rotation

- `LOG_MAX_SIZE`: Maximum size of log file in MB (default: 100)
- `LOG_MAX_BACKUPS`: Maximum number of backup files (default: 5)
- `LOG_MAX_AGE`: Maximum age of log files in days (default: 30)
- `LOG_COMPRESS`: Enable log compression (default: true)
- `LOG_DIR`: Directory for log files (default: "logs")

### Dependencies

- `go.uber.org/zap`: Core logging library
- `go.uber.org/multierr`: Error handling utilities
- `gopkg.in/natefinch/lumberjack.v2`: Log rotation

### Best Practices

1. **Context and Fields**

   - Always include relevant context in log messages
   - Use structured fields instead of string concatenation
   - Include request context in all logs
   - Use child loggers for request-scoped logging

2. **Error Handling**

   - Include error details when logging errors
   - Use proper error wrapping and context preservation
   - Log stack traces for unexpected errors
   - Use custom error types for specific scenarios

3. **Performance**

   - Use appropriate log levels
   - Track performance metrics in logs
   - Use batch processing for log aggregation
   - Implement proper retry mechanisms

4. **Security**

   - Keep sensitive information out of logs
   - Implement proper log rotation
   - Use secure log shipping
   - Monitor log access

5. **Maintenance**
   - Maintain consistent logging patterns
   - Document logging configuration
   - Monitor log storage usage
   - Regular log cleanup

### Future Improvements

1. **Log Management**

   - Implement log backup strategies
   - Add log archival capabilities
   - Enhance compression options
   - Add log validation

2. **Log Aggregation**

   - Implement log shipping to Elasticsearch
   - Add log indexing capabilities
   - Implement log search functionality
   - Add log analytics

3. **Advanced Features**

   - Implement distributed tracing
   - Add performance metrics collection
   - Enhance error tracking
   - Add audit logging

4. **Monitoring**
   - Add log-based alerts
   - Create performance dashboards
   - Implement error rate monitoring
   - Add usage pattern analysis
