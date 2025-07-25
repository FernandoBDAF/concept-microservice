# Cache Service

A high-performance, Redis-based caching service designed for the microservices ecosystem, providing profile data caching, task status caching, session management, and comprehensive observability.

## Overview

The Cache Service is a critical performance component that accelerates data access across all ecosystem services. It provides:

- **High-Performance Caching**: Sub-millisecond GET operations with 10,000+ ops/second throughput
- **Multi-Interface Support**: Both REST and gRPC APIs for different use cases
- **Ecosystem Integration**: Specialized caching patterns for profile, task, and session data
- **Production-Ready**: Circuit breakers, comprehensive metrics, health checks, and graceful shutdown
- **Kubernetes Native**: Ready for container deployment with proper resource management

## Key Features

### Performance Targets

- **GET Operations**: < 1ms average, < 5ms 99th percentile
- **SET Operations**: < 2ms average, < 10ms 99th percentile
- **Batch Operations**: < 10ms for 100 items
- **Throughput**: 10,000+ operations/second sustained
- **Availability**: 99.9% uptime with proper failover

### Core Capabilities

- **Basic Cache Operations**: GET, SET, DELETE, EXISTS with TTL management
- **Batch Operations**: MGET, MSET, MDELETE for efficient bulk operations
- **Pattern Operations**: Delete by pattern, key enumeration
- **JSON Support**: Built-in JSON serialization/deserialization
- **Circuit Breaker**: Automatic failover protection for Redis connectivity
- **Comprehensive Metrics**: Prometheus metrics for observability

## Architecture

```
Profile/Storage/Queue Services → Cache Service → Redis Cluster
                ↓                      ↓              ↓
        Cache Requests          REST/gRPC APIs    Persistent Storage
        Batch Operations        Circuit Breakers   Connection Pooling
        Session Management      Health Checks      Monitoring/Metrics
                ↓                      ↓              ↓
        Performance Boost      Service Resilience  Operational Excellence
```

## Quick Start

### Prerequisites

- Go 1.21 or later
- Redis 7+ server
- Docker (optional)
- Kubernetes cluster (for production deployment)

### Local Development

1. **Clone and setup**:

   ```bash
   cd cache-service
   go mod tidy
   ```

2. **Start Redis** (using Docker):

   ```bash
   docker run -d -p 6379:6379 --name redis redis:7-alpine
   ```

3. **Run the service**:

   ```bash
   go run cmd/server/main.go
   ```

4. **Verify service is running**:
   ```bash
   curl http://localhost:8080/health
   ```

### Configuration

The service supports configuration via environment variables:

#### Server Configuration

- `CACHE_SERVER_HTTP_PORT` (default: 8080) - HTTP server port
- `CACHE_SERVER_GRPC_PORT` (default: 9090) - gRPC server port
- `CACHE_SERVER_READ_TIMEOUT` (default: 30s) - Request read timeout
- `CACHE_SERVER_WRITE_TIMEOUT` (default: 30s) - Response write timeout

#### Redis Configuration

- `CACHE_REDIS_HOST` (default: localhost) - Redis host
- `CACHE_REDIS_PORT` (default: 6379) - Redis port
- `CACHE_REDIS_PASSWORD` - Redis password (optional)
- `CACHE_REDIS_DATABASE` (default: 0) - Redis database number
- `CACHE_REDIS_POOL_SIZE` (default: 100) - Connection pool size

#### Cache Configuration

- `CACHE_CACHE_DEFAULT_TTL` (default: 3600s) - Default TTL for cache entries
- `CACHE_CACHE_PROFILE_TTL` (default: 1800s) - TTL for profile data
- `CACHE_CACHE_TASK_TTL` (default: 300s) - TTL for task status
- `CACHE_CACHE_SESSION_TTL` (default: 86400s) - TTL for session data

#### Logging and Metrics

- `CACHE_LOGGING_LEVEL` (default: info) - Log level (debug, info, warn, error)
- `CACHE_LOGGING_FORMAT` (default: json) - Log format (json, console)
- `CACHE_METRICS_ENABLED` (default: true) - Enable Prometheus metrics
- `CACHE_METRICS_PORT` (default: 8081) - Metrics server port

## API Documentation

### Health Endpoints

#### Health Check

```bash
GET /health
```

Returns service health status and Redis connectivity.

#### Readiness Check

```bash
GET /ready
```

Returns service readiness status.

### Cache Operations

#### Get Cache Entry

```bash
GET /api/v1/cache/{key}
```

Retrieves a value from cache. Returns 404 if key not found.

#### Set Cache Entry

```bash
POST /api/v1/cache/{key}?ttl=3600s
Content-Type: application/octet-stream

[binary data]
```

Stores a value in cache with optional TTL.

#### Delete Cache Entry

```bash
DELETE /api/v1/cache/{key}
```

Removes a key from cache.

#### Check Key Existence

```bash
GET /api/v1/cache/{key}/exists
```

Returns whether a key exists in cache.

#### Get/Set TTL

```bash
GET /api/v1/cache/{key}/ttl
PUT /api/v1/cache/{key}/ttl
Content-Type: application/json

{"ttl": "3600s"}
```

### Batch Operations

```bash
POST /api/v1/cache/batch/get     # Batch get (planned)
POST /api/v1/cache/batch/set     # Batch set (planned)
DELETE /api/v1/cache/batch       # Batch delete (planned)
```

### Statistics

```bash
GET /api/v1/stats    # Cache statistics
GET /api/v1/status   # Service status
```

### Metrics Endpoint

```bash
GET /metrics         # Prometheus metrics (port 8081)
```

## Docker Deployment

### Build Image

```bash
docker build -t cache-service:1.0.0 .
```

### Run Container

```bash
docker run -d \
  --name cache-service \
  -p 8080:8080 \
  -p 9090:9090 \
  -p 8081:8081 \
  -e CACHE_REDIS_HOST=redis-host \
  cache-service:1.0.0
```

## Kubernetes Deployment

### Deploy to Kubernetes

```bash
kubectl apply -f deployments/k8s/
```

### Scale Deployment

```bash
kubectl scale deployment cache-service --replicas=5
```

### View Logs

```bash
kubectl logs -f deployment/cache-service
```

## Monitoring and Observability

### Prometheus Metrics

The service exposes comprehensive metrics on `/metrics`:

- **Cache Operations**: Hit/miss ratios, operation latencies
- **Batch Operations**: Batch sizes, success/failure rates
- **Redis Metrics**: Connection pool status, error rates
- **Circuit Breaker**: State changes, trip events
- **HTTP/gRPC**: Request rates, response times, status codes

### Structured Logging

All operations are logged with structured JSON format including:

- Request tracing
- Performance metrics
- Error details
- Circuit breaker events

### Health Monitoring

- **Liveness Probe**: `/health` - Checks Redis connectivity
- **Readiness Probe**: `/ready` - Service readiness status
- **Circuit Breaker**: Automatic failover protection

## Integration Patterns

### Profile Service Integration

```go
// Expected configuration format
type CacheConfig struct {
    Host     string `json:"host"`
    Port     int    `json:"port"`
    Password string `json:"password"`
    Database int    `json:"database"`
    Enabled  bool   `json:"enabled"`
}
```

### Cache-Aside Pattern

```bash
# 1. Try to get from cache
GET /api/v1/cache/profile:user123

# 2. If miss, get from database and cache
POST /api/v1/cache/profile:user123?ttl=1800s
```

### Session Management

```bash
# Store session
POST /api/v1/cache/session:abc123?ttl=86400s

# Validate session
GET /api/v1/cache/session:abc123

# Invalidate session
DELETE /api/v1/cache/session:abc123
```

## Performance Tuning

### Redis Configuration

- Use Redis persistence (AOF/RDB) for data safety
- Configure appropriate memory limits and eviction policies
- Use Redis Cluster for horizontal scaling

### Connection Pooling

- Default pool size: 100 connections
- Adjust based on load: `CACHE_REDIS_POOL_SIZE`
- Monitor connection usage via metrics

### Circuit Breaker

- Automatic protection against Redis failures
- Configurable trip thresholds and recovery timeouts
- Metrics available for monitoring breaker state

## Development

### Project Structure

```
cache-service/
├── cmd/server/           # Service entry point
├── internal/
│   ├── config/          # Configuration management
│   ├── domain/          # Business logic and models
│   ├── infrastructure/  # Redis, logging, metrics
│   └── interfaces/      # HTTP handlers (planned)
├── api/                 # API definitions
├── deployments/k8s/     # Kubernetes manifests
├── Dockerfile           # Container build
└── go.mod              # Go module
```

### Testing

```bash
# Run unit tests
go test ./...

# Run with coverage
go test -cover ./...

# Integration tests (requires Redis)
go test -tags=integration ./...
```

### Building

```bash
# Local build
go build -o cache-service ./cmd/server

# Docker build
docker build -t cache-service:latest .
```

## Ecosystem Integration

### Services Using Cache Service

- **Profile Service**: User profile caching with email lookup
- **Queue Service**: Task status and queue metrics caching
- **Worker Service**: Worker status and job progress caching
- **Storage Service**: Metadata and access pattern caching

### Expected Integration Patterns

- Profile data cached with 30-minute TTL
- Task status cached with 5-minute TTL
- Session data cached with 24-hour TTL
- Queue metrics cached with 1-minute TTL

## Troubleshooting

### Common Issues

**Connection Refused**

- Check Redis server is running
- Verify `CACHE_REDIS_HOST` and `CACHE_REDIS_PORT`
- Check network connectivity

**High Memory Usage**

- Monitor Redis memory usage
- Configure appropriate TTLs
- Use memory-efficient data structures

**Circuit Breaker Tripping**

- Check Redis health and performance
- Monitor error rates in metrics
- Adjust circuit breaker thresholds if needed

### Monitoring Queries

```promql
# Cache hit ratio
cache_hits_total / (cache_hits_total + cache_misses_total)

# Average response time
rate(cache_operation_duration_seconds_sum[5m]) / rate(cache_operation_duration_seconds_count[5m])

# Error rate
rate(cache_errors_total[5m])
```

## Contributing

1. Follow Go best practices and style guidelines
2. Add tests for new functionality
3. Update documentation for API changes
4. Ensure metrics are added for new operations
5. Test with Redis failover scenarios

## License

Internal microservices project - All rights reserved.
