INITIAL CONTEXT FOR LLM - never change the context-----------------------------
-> THIS SECTION IS A GUIDELINE TO THE LLM CONSIDER BEFORE WORKING IN THIS FILE, DO NOT CHANGE THIS

-> GOES OF THE README FILE:

- This file serves as the technical documentation of the deployment configuration of the profile microservices, providing a comprehensive overview of the codebase and the detailed information about the enviroment in the kubernet cluster where the microservices will be running,.
- It should document:
  - The different resources implementations and configurations
  - Give a general overview of the cluster
  - Deep description of the network
  - Details about namespaces
  - Overview of labels and annotations
  - Deep description obv volumes
  - Organize Configuration and secrets
  - Resource usase insights
  - Feel free to identify other points of interest that are not listed here
- It should also suggest improvements and tools
- This is the primary reference for understanding the technical implementation
- This file should be in sync with the `/TRACKER&MANAGER.md` where development progress and tasks are tracked
- While TRACKER&MANAGER.md focuses on "what" and "when", this file focuses on "how" and "why"

-> CONSIDERER BEFORE UPDATING THIS FILE:

- Never add fictional dates, version numbers, or metrics. Only include real, verified information. If information is not available, mark it as "To be determined" or remove the section.
- The changes in this file need to be incremental or to update informations that you confidentilly have knowlegde, they should not be guesses. If there are questions or uncertanty add comments asking for clarification instead.
- Check the `../reference-materials` folder for comprehensive project details, including architecture, development guidelines, and integration points. This will help in making informed decisions, haver better context and updates to the development plan. Always compare the implementation of this project with the plan described in the docs and whenever there are inconsistancies, add comments.
- Consider structuring this documentation separating the different topics and aspects - because this will be very dinamic and updated during the development process it will make clear what to update after each change
- This documentation is focusing on the kubernetes deployment of the profile microservices, to have a more sistemic view check `../TRACKER&MANAGER` and `../../README` or go in depth at each component at the folder `../services`
- Do not forget to be LLM focus, so because this will be used
- For LLM-specific guidelines and patterns, refer to [LLM Integration Guide](../../docs/llm/README.md)

---

# Kubernetes Manifests

This directory contains all Kubernetes manifests for the microservices project. The manifests are organized by service and purpose.

## Cluster Overview

### Namespace Structure

- All services currently run in the `default` namespace
- No explicit namespace isolation implemented yet
- TODO: Consider implementing namespace separation for better resource isolation

### Resource Organization

- Services are organized by functionality:
  - Core services (profile-api, profile-storage, auth)
  - Supporting services (postgres, redis)
  - Utility services (debug, k6)

## Network Architecture

### Service Communication

- All services use ClusterIP type, limiting access to cluster-internal
- Service discovery through DNS names (e.g., `redis:6379`, `postgres:5432`)
- No explicit service mesh implementation yet
- Free pod-to-pod and pod-to-service communication for testing purposes

### Current Network Setup

- **Service Types**: All services use ClusterIP, providing internal cluster access only
- **DNS Resolution**: Kubernetes DNS (kube-dns) enabled for service discovery
- **Pod Networking**: Using default CNI plugin (no custom network policies)
- **Service Ports**:
  - Profile API: 80 (HTTP)
  - Profile Storage: 8080 (HTTP), 50051 (gRPC)
  - Auth Service: 80 (HTTP), 9090 (gRPC)
  - PostgreSQL: 5432
  - Redis: 6379

### Network Security

- Network policies temporarily removed for testing
- No TLS for service-to-service communication
- No explicit namespace isolation
- TODO: Implement proper network policies, TLS, and namespace isolation in production

### Network Access Patterns

- **Internal Communication**:
  - Profile API → Profile Storage (gRPC)
  - Profile API → Auth Service (HTTP/gRPC)
  - Profile API → Redis (TCP)
  - Profile Storage → PostgreSQL (TCP)
  - Auth Service → Redis (TCP)
- **External Access**:
  - No external access configured
  - All services are cluster-internal only
  - TODO: Configure external access for production

### Network Configuration Details

- **DNS Policy**: Default (ClusterFirst)
- **Network Mode**: Default pod networking
- **Service Discovery**: Kubernetes DNS
- **Load Balancing**: kube-proxy (userspace mode)
- **Network Plugins**: Default CNI

## Network Troubleshooting Guide

### Basic Connectivity Checks

1. **Pod-to-Pod Communication**

   ```bash
   # Test pod-to-pod communication
   kubectl exec -it <source-pod> -- curl <target-service>:<port>

   # Example: Test Profile API to Profile Storage
   kubectl exec -it <profile-api-pod> -- curl profile-storage:8080/health
   ```

2. **DNS Resolution**

   ```bash
   # Test DNS resolution from a pod
   kubectl exec -it <pod-name> -- nslookup <service-name>

   # Example: Test Redis DNS
   kubectl exec -it <profile-api-pod> -- nslookup redis
   ```

3. **Service Endpoint Verification**

   ```bash
   # Check service endpoints
   kubectl get endpoints <service-name>

   # Example: Check Profile Storage endpoints
   kubectl get endpoints profile-storage
   ```

### Common Issues and Solutions

1. **Service Unreachable**

   - Verify service exists: `kubectl get svc <service-name>`
   - Check service endpoints: `kubectl get endpoints <service-name>`
   - Verify pod labels match service selectors
   - Check pod readiness probes

2. **DNS Resolution Issues**

   - Verify kube-dns is running: `kubectl get pods -n kube-system -l k8s-app=kube-dns`
   - Check DNS configuration: `kubectl get configmap -n kube-system coredns -o yaml`
   - Test DNS from a pod: `kubectl exec -it <pod-name> -- nslookup <service-name>`

3. **Connection Timeouts**

   - Check pod logs: `kubectl logs <pod-name>`
   - Verify service ports match container ports
   - Check for resource constraints
   - Verify network plugin status

4. **Debug Pod Usage**

   ```bash
   # Deploy debug pod
   kubectl apply -f k8s/debug/debug-pod.yaml

   # Test connectivity from debug pod
   kubectl exec -it debug -- curl <service-name>:<port>
   ```

## Planned Network Security Implementation

### Phase 1: Basic Network Policies

1. **Service Isolation**

   - Implement namespace-based isolation
   - Define pod-to-pod communication rules
   - Configure service-to-service access patterns

2. **Network Policy Rules**
   ```yaml
   # Example structure for future implementation
   apiVersion: networking.k8s.io/v1
   kind: NetworkPolicy
   metadata:
     name: service-isolation
   spec:
     podSelector:
       matchLabels:
         app: <service-name>
     policyTypes:
       - Ingress
       - Egress
     ingress:
       - from:
           - podSelector:
               matchLabels:
                 app: <allowed-service>
     egress:
       - to:
           - podSelector:
               matchLabels:
                 app: <target-service>
   ```

### Phase 2: Advanced Security

1. **TLS Implementation**

   - Service-to-service TLS
   - Certificate management
   - mTLS for sensitive communications

2. **Service Mesh Integration**

   - Traffic management
   - Security policies
   - Observability

3. **Network Monitoring**
   - Traffic analysis
   - Security auditing
   - Anomaly detection

### Phase 3: Production Hardening

1. **External Access**

   - Ingress controller setup
   - Load balancer configuration
   - External access policies

2. **Security Compliance**
   - Network segmentation
   - Access control policies
   - Audit logging

## Additional Documentation Points

### Service Dependencies

- Document service communication patterns
- List required ports and protocols
- Define service health check endpoints

### Network Performance

- Document expected latency between services
- Define bandwidth requirements
- List performance monitoring metrics

### Disaster Recovery

- Document network recovery procedures
- Define failover scenarios
- List backup and restore procedures

### Development Guidelines

- Document network testing procedures
- Define local development setup
- List debugging tools and procedures

### Security Best Practices

- Document security requirements
- Define access control policies
- List security monitoring tools

### Monitoring and Alerting

- Document network monitoring setup
- Define alert thresholds
- List monitoring tools and dashboards

### Deployment Procedures

- Document network policy deployment
- Define service deployment order
- List deployment verification steps

### Maintenance Procedures

- Document network maintenance tasks
- Define update procedures
- List troubleshooting procedures

## Resource Configuration

### Labels and Annotations

Common labels across services:

- `app`: Service name (e.g., profile-api, profile-storage)
- `tier`: Service tier (api, storage, auth)
- `environment`: Deployment environment

### Resource Limits

Current implementation:

```
Profile API:
- CPU: Request 200m, Limit 500m
- Memory: Request 256Mi, Limit 512Mi

Profile Storage:
- CPU: Request 200m, Limit 500m
- Memory: Request 256Mi, Limit 512Mi

Auth Service:
- CPU: Request 200m, Limit 500m
- Memory: Request 256Mi, Limit 512Mi
```

## Storage and Volumes

### Persistent Storage

- PostgreSQL: PersistentVolumeClaim for data storage
- Redis: EmptyDir for temporary storage
- K6: PersistentVolumeClaim for test results

### Volume Mounts

- ConfigMaps mounted as volumes for configuration
- Secrets mounted as volumes for sensitive data
- Health check endpoints at `/health`

## Configuration Management

### ConfigMaps

- Service-specific configurations
- Environment variables
- Non-sensitive settings

### Secrets

- Database credentials
- JWT secrets
- Service authentication tokens

## Monitoring and Health

### Health Checks

- Liveness probes: HTTP GET /health
- Readiness probes: HTTP GET /health
- Startup probes: Not implemented yet

### Resource Usage

- Basic resource limits implemented
- No horizontal pod autoscaling
- No pod disruption budgets

## Recent Changes

- Removed NetworkPolicy from deployment.yaml
- Fixed profile-storage pod connectivity issues
- Verified successful database connections
- All services now running with proper health checks
- Confirmed pod-to-pod communication working

- Reorganized manifests in k8s folder
- Added comprehensive README.md
- Created TRACKER&MANAGER.md
- Removed network policies for testing
- Updated service configurations

## Suggested Improvements

### High Priority

1. Implement proper namespace isolation
2. Add TLS for service-to-service communication
3. Set up monitoring and alerting
4. Implement proper secret management
5. Reintroduce network policies with proper security rules

### Medium Priority

1. Add service mesh for advanced traffic management
2. Implement horizontal pod autoscaling
3. Add pod disruption budgets
4. Configure pod topology spread

### Low Priority

1. Implement canary deployments
2. Add chaos testing
3. Set up disaster recovery procedures
4. Implement advanced monitoring

### Redis Configuration Enhancements

The following Redis configuration improvements are suggested for future implementation:

#### 1. Memory Management

```yaml
# Memory Management
maxmemory 256mb
maxmemory-policy allkeys-lru
maxmemory-samples 5
```

#### 2. Persistence Configuration

```yaml
# Persistence
save 900 1
save 300 10
save 60 10000
stop-writes-on-bgsave-error yes
rdbcompression yes
rdbchecksum yes
dbfilename dump.rdb
dir /data
```

#### 3. Security Settings

```yaml
# Security
protected-mode yes
tcp-keepalive 300
```

#### 4. Performance Tuning

```yaml
# Performance
tcp-backlog 511
timeout 0
tcp-keepalive 300
daemonize no
supervised no
pidfile /var/run/redis_6379.pid
loglevel notice
logfile ""
```

#### 5. Snapshotting Configuration

```yaml
# Snapshotting
stop-writes-on-bgsave-error yes
rdbcompression yes
rdbchecksum yes
dbfilename dump.rdb
dir /data
```

#### 6. Append Only Mode

```yaml
# Append Only Mode
appendonly yes
appendfilename "appendonly.aof"
appendfsync everysec
no-appendfsync-on-rewrite no
auto-aof-rewrite-percentage 100
auto-aof-rewrite-min-size 64mb
aof-load-truncated yes
aof-use-rdb-preamble yes
```

### Implementation Notes

1. **When to Implement**:

   - Consider implementing these changes when:
     - Moving to production environment
     - Scaling up the application
     - Implementing data persistence requirements
     - Adding security requirements

2. **Benefits**:

   - Better memory management
   - Improved data persistence
   - Enhanced security
   - Better performance tuning
   - More reliable snapshotting
   - Improved data durability with AOF

3. **Considerations**:

   - These settings should be tuned based on:
     - Available system resources
     - Data persistence requirements
     - Performance requirements
     - Security requirements
     - Monitoring capabilities

4. **Implementation Steps**:

   1. Test changes in development environment
   2. Monitor Redis performance
   3. Adjust settings based on metrics
   4. Document final configuration
   5. Implement in staging
   6. Deploy to production

5. **Monitoring Requirements**:
   - Memory usage
   - Persistence operations
   - Performance metrics
   - Error rates
   - Connection statistics

## Tools and Integrations

### Current Tools

- K6 for load testing
- Debug pod for troubleshooting
- Basic health checks

### Planned Integrations

- Prometheus for metrics
- Grafana for visualization
- Service mesh for traffic management
- Advanced logging solution

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
     - Service replication (2 replicas)
     - Proper error handling for invalid IDs
     - UUID v4 for profile IDs
     - ISO 8601 timestamp format

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
     - Service replication (2 replicas)
     - Mock token implementation for testing
     - Token validation endpoints

3. **Profile Storage Service** (`/services/profile-storage`)
   - Manages data persistence and database operations
   - Ensures data integrity and consistency
   - Provides efficient data access patterns
   - Status: In Progress
   - Key Features:
     - gRPC API for internal communication
     - REST API implementation
     - PostgreSQL integration with connection pooling
     - Health monitoring with Prometheus metrics
     - Kubernetes deployment with ConfigMaps and Secrets
     - Docker containerization with multi-stage builds
     - Structured logging with Zap logger
     - Service replication (2 replicas)
     - Proper error handling
     - Transaction management

#### API Examples

#### Authentication Flow

1. **Get Authentication Token**

   ```bash
   # Request a new authentication token
   curl -X POST http://profile-api/api/v1/auth/token \
     -H "Content-Type: application/json" \
     -d '{"user_id": "user1", "password": "123456"}'

   # Example Response
   {
     "token": "mock_access_token",
     "error": null
   }
   ```

2. **Use Token for Profile Operations**

   ```bash
   # Use the token for profile operations
   curl -X GET http://profile-api/api/v1/profiles \
     -H "Authorization: Bearer mock_access_token"
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
     -H "Authorization: Bearer mock_access_token"

   # Example Response
   [
     {
       "id": "89afa111-ab61-4ec8-a197-bb91a203a81b",
       "first_name": "John",
       "last_name": "Smith",
       "email": "john.smith@example.com",
       "created_at": "2025-05-25T05:51:05.849323Z",
       "updated_at": "2025-05-25T05:52:06.778368Z"
     }
   ]
   ```

2. **Get Profile by ID**

   ```bash
   # Get a specific profile
   curl -X GET http://profile-api/api/v1/profiles/89afa111-ab61-4ec8-a197-bb91a203a81b \
     -H "Authorization: Bearer mock_access_token"

   # Example Response
   {
     "profile": {
       "id": "89afa111-ab61-4ec8-a197-bb91a203a81b",
       "first_name": "John",
       "last_name": "Smith",
       "email": "john.smith@example.com",
       "created_at": "2025-05-25T05:51:05.849323Z",
       "updated_at": "2025-05-25T05:52:06.778368Z",
       "get_from": "storage"
     }
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
     }'

   # Example Response
   {
     "profile": {
       "id": "89afa111-ab61-4ec8-a197-bb91a203a81b",
       "first_name": "John",
       "last_name": "Doe",
       "email": "john.doe2@example.com",
       "created_at": "2025-05-25T05:51:05.849323Z",
       "updated_at": "2025-05-25T05:51:05.849323Z"
     }
   }
   ```

4. **Update Profile**

   ```bash
   # Update an existing profile
   curl -X PUT http://profile-api/api/v1/profiles/89afa111-ab61-4ec8-a197-bb91a203a81b \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer mock_access_token" \
     -d '{
       "first_name": "John",
       "last_name": "Smith",
       "email": "john.smith@example.com"
     }'

   # Example Response
   {
     "profile": {
       "id": "89afa111-ab61-4ec8-a197-bb91a203a81b",
       "first_name": "John",
       "last_name": "Smith",
       "email": "john.smith@example.com",
       "created_at": "2025-05-25T05:51:05.849323Z",
       "updated_at": "2025-05-25T05:52:06.778368Z"
     }
   }
   ```

5. **Delete Profile**

   ```bash
   # Delete a profile
   curl -X DELETE http://profile-api/api/v1/profiles/89afa111-ab61-4ec8-a197-bb91a203a81b \
     -H "Authorization: Bearer mock_access_token"

   # Example Response
   {}
   ```

6. **Error Handling**

   ```bash
   # Invalid Profile ID
   curl -X GET http://profile-api/api/v1/profiles/invalid-id \
     -H "Authorization: Bearer mock_access_token"

   # Example Error Response
   {
     "error": "Failed to get existing profile invalid-id: unexpected status code 400: Invalid profile ID"
   }
   ```

Note:

- The service is accessible via the service name `profile-api` in the cluster
- When accessing from within the cluster, use `http://profile-api` as the base URL
- When accessing from outside the cluster, use the appropriate external URL or port-forwarding
- The token in the examples is a mock token for testing - in production, use the actual token received from the token endpoint
- All profile endpoints require a valid authentication token
- Error responses will include an error message in the "error" field
- The service uses UUID v4 for profile IDs
- All timestamps are in ISO 8601 format with UTC timezone
- All services are running with 2 replicas for high availability
- Health checks are responding with good latency (< 1ms in most cases)
- Service communication is working properly between all components

## Kustomize Implementation

### Overview

Kustomize is being implemented to manage environment-specific configurations and provide a more maintainable deployment structure. This implementation follows Kubernetes-native practices and enables GitOps workflows.

### Directory Structure

```
k8s/
├── base/
│   ├── profile-api/
│   │   ├── deployment.yaml
│   │   ├── service.yaml
│   │   ├── configmap.yaml
│   │   └── kustomization.yaml
│   ├── profile-storage/
│   │   ├── deployment.yaml
│   │   ├── service.yaml
│   │   ├── configmap.yaml
│   │   └── kustomization.yaml
│   ├── auth/
│   │   ├── deployment.yaml
│   │   ├── service.yaml
│   │   ├── configmap.yaml
│   │   └── kustomization.yaml
│   └── kustomization.yaml
├── overlays/
│   ├── development/
│   │   ├── profile-api/
│   │   │   ├── kustomization.yaml
│   │   │   └── patch-deployment.yaml
│   │   ├── profile-storage/
│   │   │   ├── kustomization.yaml
│   │   │   └── patch-deployment.yaml
│   │   ├── auth/
│   │   │   ├── kustomization.yaml
│   │   │   └── patch-deployment.yaml
│   │   └── kustomization.yaml
│   └── production/
│       ├── profile-api/
│       │   ├── kustomization.yaml
│       │   └── patch-deployment.yaml
│       ├── profile-storage/
│       │   ├── kustomization.yaml
│       │   └── patch-deployment.yaml
│       ├── auth/
│       │   ├── kustomization.yaml
│       │   └── patch-deployment.yaml
│       └── kustomization.yaml
```

### Base Configuration

The base configuration contains the common Kubernetes resources shared across all environments:

```yaml
# base/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - profile-api
  - profile-storage
  - auth

commonLabels:
  app.kubernetes.io/part-of: profile-service
  app.kubernetes.io/managed-by: kustomize

commonAnnotations:
  description: "Profile Service Base Configuration"
```

### Environment-Specific Configurations

#### Development Environment

```yaml
# overlays/development/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - ../../base

namespace: profile-dev

commonLabels:
  environment: development

patches:
  - path: profile-api/patch-deployment.yaml
  - path: profile-storage/patch-deployment.yaml
  - path: auth/patch-deployment.yaml

configMapGenerator:
  - name: profile-service-config
    literals:
      - ENVIRONMENT=development
      - LOG_LEVEL=debug
```

#### Production Environment

```yaml
# overlays/production/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - ../../base

namespace: profile-prod

commonLabels:
  environment: production

patches:
  - path: profile-api/patch-deployment.yaml
  - path: profile-storage/patch-deployment.yaml
  - path: auth/patch-deployment.yaml

configMapGenerator:
  - name: profile-service-config
    literals:
      - ENVIRONMENT=production
      - LOG_LEVEL=info
```

### Resource Patching

Example of a deployment patch for development environment:

```yaml
# overlays/development/profile-api/patch-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: profile-api
spec:
  replicas: 1
  template:
    spec:
      containers:
        - name: profile-api
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: 200m
              memory: 256Mi
```

### Deployment Process

1. **Build Configuration**

   ```bash
   # Development
   kustomize build overlays/development

   # Production
   kustomize build overlays/production
   ```

2. **Apply Configuration**

   ```bash
   # Development
   kustomize build overlays/development | kubectl apply -f -

   # Production
   kustomize build overlays/production | kubectl apply -f -
   ```

### Benefits

1. **Environment Management**

   - Clear separation of environments
   - Consistent base configuration
   - Environment-specific customizations
   - Simplified deployment process

2. **Configuration Management**

   - Declarative configuration
   - Version control friendly
   - Easy to review changes
   - Reduced configuration drift

3. **Resource Organization**
   - Logical grouping of resources
   - Clear dependency structure
   - Easy to maintain and update
   - Better resource isolation

### Integration with Existing Setup

- Compatible with current service architecture
- Maintains existing resource configurations
- Preserves current networking setup
- Supports existing monitoring configuration

### Next Steps

1. **Implementation**

   - [ ] Create base configurations
   - [ ] Set up development overlay
   - [ ] Set up production overlay
   - [ ] Test configurations

2. **Documentation**

   - [ ] Document deployment procedures
   - [ ] Create troubleshooting guide
   - [ ] Add configuration examples
   - [ ] Update CI/CD pipelines

3. **Validation**
   - [ ] Test all environments
   - [ ] Verify resource configurations
   - [ ] Validate security settings
   - [ ] Check monitoring integration

### Service Evolution Guidelines

#### Adding New Services

1. **Base Configuration**

   ```yaml
   # base/<new-service>/kustomization.yaml
   apiVersion: kustomize.config.k8s.io/v1beta1
   kind: Kustomization

   resources:
     - deployment.yaml
     - service.yaml
     - configmap.yaml

   commonLabels:
     app.kubernetes.io/name: <new-service>
     app.kubernetes.io/part-of: profile-service
   ```

2. **Environment Overlays**

   ```yaml
   # overlays/<environment>/<new-service>/kustomization.yaml
   apiVersion: kustomize.config.k8s.io/v1beta1
   kind: Kustomization

   resources:
     - ../../../base/<new-service>

   patches:
     - path: patch-deployment.yaml
   ```

3. **Integration Steps**
   - Add service to base kustomization.yaml
   - Create environment-specific overlays
   - Configure service dependencies
   - Update network policies
   - Add monitoring configuration

#### Evolving Existing Services

1. **Configuration Updates**

   - Update base configuration
   - Modify environment-specific patches
   - Update resource requirements
   - Adjust health checks
   - Update monitoring configuration

2. **Dependency Management**

   - Update service dependencies
   - Modify network policies
   - Adjust resource limits
   - Update service mesh configuration

3. **Version Control**
   - Maintain configuration history
   - Document changes
   - Update documentation
   - Track configuration versions

### Service Integration Patterns

#### 1. Core Services (Profile API, Storage, Auth)

```yaml
# base/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - profile-api
  - profile-storage
  - auth

commonLabels:
  tier: core
  app.kubernetes.io/part-of: profile-service
```

#### 2. Supporting Services (Cache, Queue, Worker)

```yaml
# base/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - profile-cache
  - profile-queue
  - profile-worker

commonLabels:
  tier: supporting
  app.kubernetes.io/part-of: profile-service
```

#### 3. Monitoring Services

```yaml
# base/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - profile-monitoring

commonLabels:
  tier: monitoring
  app.kubernetes.io/part-of: profile-service
```

### Service Evolution Process

1. **Planning Phase**

   - Define service requirements
   - Identify dependencies
   - Plan resource allocation
   - Design monitoring strategy

2. **Implementation Phase**

   - Create base configuration
   - Set up environment overlays
   - Configure service integration
   - Implement monitoring

3. **Testing Phase**

   - Test service deployment
   - Verify integration
   - Validate monitoring
   - Check resource usage

4. **Deployment Phase**
   - Deploy to development
   - Test in staging
   - Deploy to production
   - Monitor performance

### Best Practices

1. **Configuration Management**

   - Use consistent naming
   - Follow directory structure
   - Maintain documentation
   - Version control all changes

2. **Resource Management**

   - Set appropriate limits
   - Configure health checks
   - Implement monitoring
   - Plan for scaling

3. **Security**

   - Implement network policies
   - Configure RBAC
   - Manage secrets
   - Regular security audits

4. **Monitoring**
   - Set up metrics collection
   - Configure alerts
   - Monitor resource usage
   - Track service health

### Service Categories

1. **Core Services**

   - Profile API
   - Profile Storage
   - Auth Service
   - High availability required
   - Strict monitoring

2. **Supporting Services**

   - Profile Cache
   - Profile Queue
   - Profile Worker
   - Flexible scaling
   - Basic monitoring

3. **Monitoring Services**
   - Profile Monitoring
   - Logging
   - Metrics
   - High reliability
   - Comprehensive monitoring

### Integration Guidelines

1. **Service Communication**

   - Define communication patterns
   - Configure service mesh
   - Set up load balancing
   - Implement circuit breakers

2. **Data Management**

   - Configure data persistence
   - Set up backups
   - Implement caching
   - Manage data lifecycle

3. **Security Integration**

   - Configure authentication
   - Set up authorization
   - Implement encryption
   - Manage secrets

4. **Monitoring Integration**
   - Set up metrics
   - Configure logging
   - Implement tracing
   - Set up alerts

## Helm Integration Considerations

While we are currently using Kustomize for our deployment management, we maintain a Helm implementation plan for potential future use. Here are the scenarios where transitioning to Helm might be beneficial:

### When to Consider Using Helm

1. **Complex Application Management**

   - When managing multiple interdependent services
   - When dealing with complex dependencies between components
   - When versioning and release management become critical
   - When sharing configurations across multiple projects

2. **Package Management Needs**

   - When distributing services as reusable packages
   - When managing multiple versions of the same service
   - When sharing configurations with other teams
   - When using chart repositories for common components

3. **Advanced Templating Requirements**
   - When dynamic configuration generation is needed
   - When complex conditional resource creation is required
   - When variable substitution becomes complex
   - When template reuse across services is needed

### Integration with Kustomize

We can use both tools together in a hybrid approach:

1. **Helm for Base Deployment**

   ```bash
   # Deploy base application with Helm
   helm upgrade --install profile-service ./helm \
     -f helm/values-prod.yaml \
     --namespace profile-prod
   ```

2. **Kustomize for Environment-Specific Changes**
   ```bash
   # Apply environment-specific changes with Kustomize
   kustomize build overlays/production | kubectl apply -f -
   ```

### Migration Considerations

When considering a migration to Helm:

1. **Current Benefits of Kustomize**

   - Native Kubernetes integration
   - Simple learning curve
   - GitOps-friendly
   - No template language complexity
   - Direct resource patching

2. **Potential Benefits of Helm**

   - Better package management
   - Version control for deployments
   - Template-based configuration
   - Dependency management
   - Release management

3. **Migration Triggers**
   - Growing service complexity
   - Need for package distribution
   - Complex dependency management
   - Advanced templating requirements
   - Release management needs

### Implementation Plan

Our Helm implementation plan is documented in `HELM-IMPLEMENTATION.md` and includes:

1. **Base Chart Setup**

   - Chart structure
   - Value management
   - Template development
   - Environment configuration

2. **Service Templates**

   - Common templates
   - Service-specific templates
   - Resource definitions
   - Configuration management

3. **Environment Configuration**

   - Development setup
   - Staging setup
   - Production setup
   - Value overrides

4. **Testing and Validation**
   - Template validation
   - Configuration testing
   - Deployment testing
   - Integration testing

### Best Practices for Both Tools

1. **Configuration Management**

   - Keep base configurations minimal
   - Use environment-specific overlays
   - Document all customizations
   - Follow naming conventions

2. **Resource Organization**

   - Group related resources
   - Use consistent naming
   - Document dependencies
   - Maintain clear structure

3. **Security**

   - Never store secrets in version control
   - Use external secret management
   - Implement proper RBAC
   - Follow security best practices

4. **Monitoring**
   - Set up proper health checks
   - Configure resource monitoring
   - Implement logging
   - Set up alerts
