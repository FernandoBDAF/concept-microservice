# Cache Service Interface Specification

This document defines the external interfaces and integration patterns for the Cache Service, including REST API endpoints, gRPC services, and ecosystem integration contracts.

## Service Interfaces

### REST API (HTTP)

The Cache Service exposes a REST API on port 8080 for general-purpose caching operations and integration with web applications.

#### Base URL

```
http://cache-service:8080
```

#### Health and Status Endpoints

**Health Check**

```http
GET /health
```

- **Purpose**: Service health monitoring for load balancers and orchestrators
- **Response**: 200 OK if healthy, 503 Service Unavailable if unhealthy
- **Checks**: Redis connectivity, service status
- **Example Response**:

```json
{
  "status": "healthy",
  "timestamp": "2024-01-01T12:00:00Z",
  "version": "1.0.0"
}
```

**Readiness Check**

```http
GET /ready
```

- **Purpose**: Kubernetes readiness probe
- **Response**: 200 OK if ready, 503 if not ready
- **Example Response**:

```json
{
  "status": "ready",
  "timestamp": "2024-01-01T12:00:00Z"
}
```

#### Core Cache Operations

**Get Cache Entry**

```http
GET /api/v1/cache/{key}
```

- **Purpose**: Retrieve a value from cache
- **Parameters**:
  - `key` (path): Cache key (max 512 chars)
- **Response**:
  - 200 OK: Binary data (application/octet-stream)
  - 404 Not Found: Key doesn't exist
  - 500 Internal Server Error: Cache error
- **Example**:

```bash
curl http://cache-service:8080/api/v1/cache/user:123
```

**Set Cache Entry**

```http
POST /api/v1/cache/{key}[?ttl=duration]
Content-Type: application/octet-stream
```

- **Purpose**: Store a value in cache
- **Parameters**:
  - `key` (path): Cache key (max 512 chars)
  - `ttl` (query, optional): Time-to-live duration (e.g., "3600s", "1h")
- **Body**: Binary data (max 1MB)
- **Response**:
  - 200 OK: Successfully stored
  - 400 Bad Request: Invalid key/value size or TTL format
  - 500 Internal Server Error: Cache error
- **Example**:

```bash
curl -X POST -d "value data" \
  "http://cache-service:8080/api/v1/cache/user:123?ttl=1800s"
```

**Delete Cache Entry**

```http
DELETE /api/v1/cache/{key}
```

- **Purpose**: Remove a key from cache
- **Parameters**:
  - `key` (path): Cache key to delete
- **Response**:
  - 200 OK: Successfully deleted
  - 500 Internal Server Error: Cache error
- **Example**:

```bash
curl -X DELETE http://cache-service:8080/api/v1/cache/user:123
```

**Check Key Existence**

```http
GET /api/v1/cache/{key}/exists
```

- **Purpose**: Check if a key exists without retrieving its value
- **Parameters**:
  - `key` (path): Cache key to check
- **Response**:

```json
{
  "exists": true
}
```

#### TTL Management

**Get TTL**

```http
GET /api/v1/cache/{key}/ttl
```

- **Purpose**: Get remaining time-to-live for a key
- **Response**:

```json
{
  "ttl": "1h23m45s"
}
```

**Set TTL**

```http
PUT /api/v1/cache/{key}/ttl
Content-Type: application/json
```

- **Purpose**: Update TTL for an existing key
- **Body**:

```json
{
  "ttl": "3600s"
}
```

- **Response**:

```json
{
  "status": "success"
}
```

#### Batch Operations (Planned - Phase 2)

**Batch Get**

```http
POST /api/v1/cache/batch/get
Content-Type: application/json
```

- **Purpose**: Retrieve multiple keys in a single request
- **Body**:

```json
{
  "keys": ["user:123", "user:456", "profile:789"]
}
```

- **Response**:

```json
{
  "values": {
    "user:123": "base64-encoded-data",
    "user:456": "base64-encoded-data"
  },
  "missing": ["profile:789"]
}
```

**Batch Set**

```http
POST /api/v1/cache/batch/set
Content-Type: application/json
```

- **Purpose**: Store multiple key-value pairs
- **Body**:

```json
{
  "items": [
    { "key": "user:123", "value": "base64-data", "ttl": "1800s" },
    { "key": "user:456", "value": "base64-data", "ttl": "3600s" }
  ]
}
```

**Batch Delete**

```http
DELETE /api/v1/cache/batch
Content-Type: application/json
```

- **Purpose**: Delete multiple keys
- **Body**:

```json
{
  "keys": ["user:123", "user:456", "profile:789"]
}
```

#### Statistics and Monitoring

**Cache Statistics**

```http
GET /api/v1/stats
```

- **Purpose**: Get cache performance statistics
- **Response**:

```json
{
  "hits": 12345,
  "misses": 678,
  "evictions": 12,
  "total_keys": 5432,
  "used_memory": 134217728,
  "hit_ratio": 0.948,
  "last_updated": "2024-01-01T12:00:00Z"
}
```

**Service Status**

```http
GET /api/v1/status
```

- **Purpose**: Get service status information
- **Response**:

```json
{
  "service": "cache-service",
  "version": "1.0.0",
  "status": "running",
  "timestamp": "2024-01-01T12:00:00Z"
}
```

### gRPC Interface (High-Performance)

The Cache Service exposes a gRPC interface on port 9090 for high-performance operations, particularly suited for service-to-service communication.

#### Service Definition (Planned - Phase 1 Completion)

```protobuf
syntax = "proto3";

package cache.v1;

service CacheService {
  // Basic operations
  rpc Get(GetRequest) returns (GetResponse);
  rpc Set(SetRequest) returns (SetResponse);
  rpc Delete(DeleteRequest) returns (DeleteResponse);
  rpc Exists(ExistsRequest) returns (ExistsResponse);

  // Batch operations
  rpc BatchGet(BatchGetRequest) returns (BatchGetResponse);
  rpc BatchSet(BatchSetRequest) returns (BatchSetResponse);
  rpc BatchDelete(BatchDeleteRequest) returns (BatchDeleteResponse);

  // TTL management
  rpc GetTTL(GetTTLRequest) returns (GetTTLResponse);
  rpc SetTTL(SetTTLRequest) returns (SetTTLResponse);

  // Statistics
  rpc GetStats(GetStatsRequest) returns (GetStatsResponse);
}
```

#### Performance Characteristics

- **Latency**: ~30% lower than REST for small operations
- **Throughput**: ~50% higher than REST for batch operations
- **Connection**: Persistent HTTP/2 connections with multiplexing
- **Serialization**: Protocol Buffers for efficient data transfer

### Metrics Interface

**Prometheus Metrics**

```http
GET /metrics
```

- **Port**: 8081 (separate metrics server)
- **Format**: Prometheus exposition format
- **Metrics Include**:
  - Cache operation counters and histograms
  - Redis connection pool metrics
  - Circuit breaker status
  - HTTP/gRPC request metrics
  - Batch operation statistics

## Ecosystem Integration Contracts

### Profile Service Integration

The Cache Service provides specialized caching for the Profile Service with expected configuration and key patterns.

**Configuration Contract**

```go
type CacheConfig struct {
    Host     string `json:"host"`     // cache-service
    Port     int    `json:"port"`     // 8080
    Password string `json:"password"` // optional
    Database int    `json:"database"` // 0
    Enabled  bool   `json:"enabled"`  // true
}
```

**Key Patterns**

- Profile by ID: `profile:{userID}`
- Profile by email: `profile:email:{email}`
- Profile metadata: `profile:meta:{userID}`

**TTL Strategy**

- Profile data: 30 minutes (1800s)
- Email lookups: 15 minutes (900s)
- Metadata: 1 hour (3600s)

**Integration Example**

```bash
# Cache user profile
POST /api/v1/cache/profile:user123?ttl=1800s

# Lookup by email
GET /api/v1/cache/profile:email:user@example.com

# Invalidate on profile update
DELETE /api/v1/cache/profile:user123
DELETE /api/v1/cache/profile:email:user@example.com
```

### Queue Service Integration

Caching for task status and queue metrics.

**Key Patterns**

- Task status: `task:{taskID}`
- Queue metrics: `queue:metrics:{queueName}`
- Worker status: `worker:status:{workerType}`

**TTL Strategy**

- Task status: 5 minutes (300s)
- Queue metrics: 1 minute (60s)
- Worker status: 2 minutes (120s)

### Session Management Integration

Session and authentication token caching.

**Key Patterns**

- User sessions: `session:{sessionID}`
- JWT blacklist: `jwt:blacklist:{tokenID}`
- Device sessions: `device:session:{deviceID}`

**TTL Strategy**

- User sessions: 24 hours (86400s)
- JWT blacklist: Token expiry time
- Device sessions: 7 days (604800s)

**Integration Example**

```bash
# Store session
POST /api/v1/cache/session:abc123?ttl=86400s

# Validate session
GET /api/v1/cache/session:abc123

# Blacklist JWT
POST /api/v1/cache/jwt:blacklist:token123?ttl=3600s
```

### Storage Service Integration

Metadata and access pattern caching.

**Key Patterns**

- File metadata: `file:meta:{fileID}`
- Directory listings: `dir:list:{dirPath}`
- Access permissions: `access:{userID}:{resourceID}`

**TTL Strategy**

- File metadata: 15 minutes (900s)
- Directory listings: 5 minutes (300s)
- Access permissions: 10 minutes (600s)

## Client Libraries and SDKs

### Go Client (Planned)

```go
import "cache-service/pkg/client"

client := client.New(&client.Config{
    Host: "cache-service",
    Port: 8080,
})

// Basic operations
value, err := client.Get(ctx, "key")
err = client.Set(ctx, "key", value, time.Hour)
```

### HTTP Client Examples

**curl**

```bash
# Get
curl http://cache-service:8080/api/v1/cache/mykey

# Set with TTL
curl -X POST -d "value" \
  "http://cache-service:8080/api/v1/cache/mykey?ttl=3600s"

# Delete
curl -X DELETE http://cache-service:8080/api/v1/cache/mykey
```

**Python**

```python
import requests

base_url = "http://cache-service:8080"

# Get
response = requests.get(f"{base_url}/api/v1/cache/mykey")
if response.status_code == 200:
    value = response.content

# Set
requests.post(
    f"{base_url}/api/v1/cache/mykey?ttl=3600s",
    data=b"my value"
)
```

## Service Discovery and Load Balancing

### Kubernetes Service

```yaml
apiVersion: v1
kind: Service
metadata:
  name: cache-service
spec:
  selector:
    app: cache-service
  ports:
    - port: 8080
      name: http
    - port: 9090
      name: grpc
```

### Service Mesh Integration

- **Istio**: Automatic traffic management and observability
- **Consul**: Service discovery and health checking
- **Linkerd**: Load balancing and circuit breaking

## Error Handling and Circuit Breaker

### Error Response Format

```json
{
  "error": "error description",
  "code": "ERROR_CODE",
  "timestamp": "2024-01-01T12:00:00Z"
}
```

### Circuit Breaker Patterns

- **Closed**: Normal operation
- **Open**: Fast-fail when Redis is unavailable
- **Half-Open**: Testing Redis recovery

### Retry Strategies

- **Exponential Backoff**: For temporary failures
- **Circuit Breaker**: For persistent failures
- **Client Timeout**: 5-second default timeout

## Performance and SLA

### Response Time SLA

- **GET Operations**: < 1ms P50, < 5ms P99
- **SET Operations**: < 2ms P50, < 10ms P99
- **Batch Operations**: < 10ms for 100 items
- **Health Checks**: < 100ms

### Throughput Targets

- **Single Operations**: 10,000+ ops/second
- **Batch Operations**: 50,000+ items/second
- **Concurrent Connections**: 1,000+ simultaneous

### Availability SLA

- **Uptime**: 99.9% (8.76 hours downtime/year)
- **Recovery Time**: < 30 seconds for Redis failover
- **Data Consistency**: Eventual consistency with Redis persistence
