INITIAL CONTEXT FOR LLM - never change the context-----------------------------
-> THIS SECTION IS A GUIDELINE TO THE LLM CONSIDER BEFORE WORKING IN THIS FILE, DO NOT CHANGE THIS

-> GOES OF THE README FILE:

- This is a high level documentation file, many of the subfolders have their all documentations files, so you should keep track of all the documentation under you.
- This file serves as the technical documentation of the service, providing a comprehensive overview of the codebase
- It should document:
  - Service architecture and design decisions focusing in the cross interaction between services having a more sistemic view
  - Component structure and relationships
  - Dependencies and integration points
  - Configuration and deployment details
- This is the primary reference for understanding the technical implementation
- This file should be in sync with the `/TRACKER&MANAGER.md` where development progress and tasks are tracked
- While TRACKER&MANAGER.md focuses on "what" and "when", this file focuses on "how" and "why"

-> CONSIDERER BEFORE UPDATING THIS FILE:

- Never add fictional dates, version numbers, or metrics. Only include real, verified information. If information is not available, mark it as "To be determined" or remove the section.
- The changes in this file need to be incremental or to update informations that you confidentilly have knowlegde, they should not be guesses. If there are questions or uncertanty add comments asking for clarification instead.
- Check the `microservices/docs` folder for comprehensive project details, including architecture, development guidelines, and integration points. This will help in making informed decisions, haver better context and updates to the development plan. Always compare the implementation of this project with the plan described in the docs and whenever there are inconsistancies, add comments.
- Consider structuring this documentation separating the services and referencing their documentation yet summarizing them. Else add sections to organize different aspects of the cross interaction, dependencies and decisions. Because this will be very dinamic and updated during the development process it will make clear what to update after each change
- Do not forget to be LLM focus, so because this will be used
- For LLM-specific guidelines and patterns, refer to [LLM Integration Guide](../../docs/llm/README.md)

---

# Profile Service Microservices Architecture

## System Overview

The Profile Service Microservices architecture is a distributed system designed to handle user profile management, authentication, and related operations. The system is built with scalability, maintainability, and operational efficiency in mind.

## Service Architecture

### Core Services

1. **Profile API Service** (`/services/profile-api`)

   - Primary entry point for client applications
   - Handles request routing and validation
   - Manages authentication and authorization
   - Integrates with other services for data operations
   - Status: In Progress
   - Key Features:
     - REST API endpoints
     - Authentication middleware
     - Session management with Redis
     - Health monitoring
     - Error handling
     - Structured logging with Zap logger
     - Prometheus metrics integration

2. **Auth Service** (`/services/auth`)

   - Handles user authentication and authorization
   - Manages JWT tokens and sessions
   - Implements OAuth 2.0 / OpenID Connect
   - Provides role-based access control
   - Status: Migration in Progress
   - Key Features:
     - User authentication
     - Token management
     - Session handling
     - Role management
     - Clerk integration (in progress)
     - Backward compatibility layer

3. **Profile Storage Service** (`/services/profile-storage`)
   - Manages data persistence and database operations
   - Ensures data integrity and consistency
   - Provides efficient data access patterns
   - Status: In Progress
   - Key Features:
     - gRPC API for internal communication
     - PostgreSQL integration with connection pooling
     - Health monitoring with Prometheus metrics
     - Kubernetes deployment with ConfigMaps and Secrets
     - Docker containerization with multi-stage builds
     - Structured logging with Zap logger

### Supporting Services

4. **Profile Cache Service** (`/services/profile-cache`)

   - Provides distributed caching
   - Manages cache invalidation
   - Optimizes data access performance
   - Status: Planned
   - Key Features:
     - Redis integration
     - Cache policies
     - Invalidation strategies

5. **Profile Queue Service** (`/services/profile-queue`)

   - Handles asynchronous message processing
   - Manages event-driven communication
   - Ensures message persistence
   - Status: Planned
   - Key Features:
     - Message queuing
     - Event handling
     - Queue management

6. **Profile Worker Service** (`/services/profile-worker`)

   - Processes background jobs
   - Handles scheduled tasks
   - Manages job monitoring
   - Status: Planned
   - Key Features:
     - Job processing
     - Task scheduling
     - Error handling

7. **Profile Monitoring Service** (`/services/profile-monitoring`)
   - Collects system metrics
   - Manages health checks
   - Handles alerting
   - Status: Planned
   - Key Features:
     - Metrics collection
     - Health monitoring
     - Alert management

## Service Interactions

### Communication Patterns

1. **Synchronous Communication**

   - REST APIs for external clients
   - gRPC for internal service communication
   - Health check endpoints

2. **Asynchronous Communication**
   - Message queues for event handling
   - Event-driven patterns
   - Background job processing

### Data Flow

1. **Profile Management Flow**

   ```
   Client → Profile API → Auth Service
                    ↓
              Profile Storage
                    ↓
              PostgreSQL
   ```

2. **Authentication Flow**

   ```
   Client → Auth Service → Get Token
                    ↓
              Profile API
                    ↓
              Redis (Session Management)
                    ↓
              Token Validation (Auth Service)
   ```

   Key points:

   - Authentication is handled by the Auth Service
   - Profile API uses Redis for session management
   - Token validation is delegated to Auth Service
   - No direct JWT handling in Profile API

## Cross-Cutting Concerns

### Security

1. **Authentication**

   - JWT token validation
   - OAuth 2.0 integration
   - Session management
   - Clerk integration (in progress)

2. **Authorization**
   - Role-based access control
   - Permission management
   - Service-to-service authentication

### Monitoring

1. **Health Checks**

   - Service health monitoring
   - Database connectivity
   - Cache status

2. **Metrics**

   - Performance metrics
   - Error rates
   - Resource utilization

3. **Logging**
   - Structured logging with Zap
   - Log aggregation
   - Log levels and formatting
   - Request/response logging
   - Error tracking

### Error Handling

1. **Error Patterns**

   - Standardized error responses
   - Error propagation
   - Error tracking

2. **Recovery Strategies**
   - Circuit breakers
   - Retry mechanisms
   - Fallback patterns

## Development Status

### Current Phase: Alpha Testing

1. **Completed Features**

   - Basic service structures
   - Docker configurations
   - Health check endpoints
   - Authentication middleware
   - Session management
   - PostgreSQL integration (in-cluster)
   - Redis integration (in-cluster)
   - Kubernetes deployment
   - Network policies
   - ConfigMaps and Secrets
   - Structured logging implementation
   - Prometheus metrics integration
   - Service discovery and communication
   - All endpoints verified working in cluster
   - Successful integration between profile-api and profile-storage
   - Complete mock implementation of Auth Service
   - Both gRPC and REST APIs in Profile Storage Service
   - Graceful shutdown implementation
   - Proper error handling and logging across services
   - In-cluster database deployments (PostgreSQL and Redis)
   - Updated network policies for in-cluster communication
   - Successful service-to-database communication

2. **In Progress**

   - Auth service migration to Clerk
   - Redis session management implementation
   - Real implementation of Auth Service (currently mocked)
   - Logging system enhancements
   - Performance optimization
   - Metrics collection improvements

3. **Pending**
   - Cache service implementation (directory structure only)
   - Queue service implementation (directory structure only)
   - Worker service implementation (directory structure only)
   - Monitoring service implementation (directory structure only)
   - Advanced monitoring features
   - Log aggregation setup
   - Performance testing
   - Load testing
   - Clerk integration
   - Token translation service
   - Session management adapter

## Infrastructure

### Deployment

1. **Kubernetes**

   - Service deployments
   - Resource management
   - Health monitoring
   - Service mesh
   - Network policies
   - ConfigMaps and Secrets
   - In-cluster PostgreSQL and Redis

2. **Docker**
   - Container images
   - Docker Compose
   - Development environment
   - Multi-stage builds

### Network Security

1. **Network Policies**

   - Service-to-service communication control
   - External resource access management
   - Namespace isolation
   - Port management
   - [Network Policy Documentation](docs/architecture/network/network-policies.md)

2. **Security Best Practices**
   - Principle of least privilege
   - Namespace-based isolation
   - External access restrictions
   - Regular policy reviews

### Dependencies

1. **Databases**

   - PostgreSQL for data storage (in-cluster)
   - Redis for caching (in-cluster)
   - RabbitMQ for messaging

2. **Monitoring**
   - Prometheus for metrics
   - Grafana for visualization
   - ELK stack for logging

## Documentation

### Key References

1. **Architecture**

   - [Architecture Overview](docs/architecture/README.md)
   - [Service Architecture](docs/architecture/services/service-architecture.md)
   - [Security Architecture](docs/architecture/overview/security.md)

2. **Development**

   - [Development Guide](docs/guides/development/guide.md)
   - [Testing Guide](docs/guides/development/testing/guide.md)
   - [Environment Setup](docs/guides/development/environment/guide.md)

3. **API**

   - [API Specification](docs/api/openapi/profile-api.yaml)
   - [API Security](docs/api/security.md)
   - [API Examples](docs/api/examples/)

4. **Operations**
   - [Monitoring Guide](docs/guides/operations/monitoring/guide.md)
   - [Logging Guide](docs/guides/operations/logging/guide.md)
   - [Troubleshooting Guide](docs/guides/operations/troubleshooting/guide.md)

## Next Steps

1. Complete service integration
2. Implement monitoring
3. Add comprehensive testing
4. Update documentation
5. Prepare for beta testing

## Notes

- Track all decisions
- Update documentation
- Maintain progress
- Document challenges
- Record lessons learned
- Track improvements

## Tasks History

- Initial setup
- Created development plan
- Set up documentation structure
- Updated service integration documentation
- Implemented mock authentication endpoints
- Added mock JWT functionality
- Implemented mock OAuth 2.0 endpoints
- Added mock RBAC endpoints
- Implemented Redis-based session management
- Validated authentication endpoints
- Set up PostgreSQL with Docker Compose
- Configured Kubernetes deployment
- Implemented network policies
- Added ConfigMaps and Secrets
- Resolved database connectivity issues

## Infrastructure Integration

### Redis Integration

- Token storage (in-cluster)
- Session management
- Rate limiting
- Cache policies
- In-cluster deployment with persistent storage
- Health monitoring and probes
- Automatic failover support

### PostgreSQL Integration

- User data storage (in-cluster)
- Role management
- Permission storage
- Audit logging
- Connection pooling
- Health monitoring
- In-cluster deployment with persistent storage
- Automatic failover support
- Database initialization with ConfigMaps
- [External Database Connectivity](docs/architecture/database/connectivity.md)

### Monitoring Integration

- Prometheus metrics
- Grafana dashboards
- Health checks
- Log aggregation
- Database metrics collection
- Cache performance monitoring

## Load Testing

We use k6 for load testing our microservices. k6 was chosen for its:

- Native integration with Grafana
- Support for both REST and gRPC
- Kubernetes compatibility
- Real-time metrics collection
- Developer-friendly JavaScript-based scripts

### Implementation Details

1. **Kubernetes Integration**

   - k6 runs as a Kubernetes Job
   - Test scripts stored in ConfigMaps
   - Results available in pod logs
   - Easy integration with existing monitoring

2. **Test Scenarios**

   - Basic Load Test: 20 concurrent users
   - Ramp-up period: 30 seconds
   - Hold period: 2 minutes
   - Ramp-down period: 30 seconds
   - Tests both REST and gRPC endpoints
   - Current test coverage:
     - Profile listing
     - Profile creation
     - Profile retrieval
     - Profile updates
     - Profile deletion

3. **Metrics Collection**

   - Response times
   - Request rates
   - Error rates
   - Virtual user counts
   - Iteration counts
   - Custom metrics for each operation type

4. **Test Coverage**
   - Profile API endpoints
   - Profile Storage endpoints
   - Authentication flows (pending implementation)
   - Error scenarios
   - Performance baselines

### Running Tests

1. **Basic Test**

   ```bash
   kubectl apply -f k8s/k6/k6-job.yaml
   ```

2. **View Results**

   ```bash
   kubectl logs -n microservice -l job-name=k6-load-test
   ```

3. **Cleanup**
   ```bash
   kubectl delete job k6-load-test -n microservice
   ```

### Test Results

Initial test results show:

- Successful ramp-up to 20 concurrent users
- Stable request handling
- Consistent response times
- Error threshold exceeded (less than 1% error rate not met)
- Authentication testing pending implementation
- Service communication verified

### Areas for Improvement

1. **Authentication Testing**

   - Implement token-based authentication in tests
   - Add authentication flow testing
   - Test token validation
   - Test unauthorized access scenarios

2. **Test Coverage**

   - Add more comprehensive test scenarios
   - Implement edge case testing
   - Add rate limiting tests
   - Test concurrent authenticated requests

3. **Monitoring**
   - Add authentication-specific metrics
   - Implement token validation timing metrics
   - Set up authentication failure alerts
   - Enhance error tracking

For detailed information about our load testing strategy, see [Load Testing Documentation](docs/load-testing/README.md).
