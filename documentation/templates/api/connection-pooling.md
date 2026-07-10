# Connection Pooling Guide

## Overview

Connection pooling is a critical component for optimizing database performance and resource utilization in the Profile Storage Service. This guide outlines the implementation details, best practices, and configuration options for connection pooling.

## Implementation Details

### Current Implementation

The Profile Storage Service uses `sqlx` with PostgreSQL for connection pooling, configured through the following parameters:

```go
// Connection pool configuration
MaxOpenConns    int           // Maximum number of open connections
MaxIdleConns    int           // Maximum number of idle connections
ConnMaxLifetime time.Duration // Maximum lifetime of a connection
ConnMaxIdleTime time.Duration // Maximum idle time of a connection
```

### Configuration Parameters

1. **MaxOpenConns**

   - Default: 25
   - Purpose: Limits the total number of open connections
   - Impact: Prevents connection exhaustion
   - Consideration: Should be based on expected concurrent requests

2. **MaxIdleConns**

   - Default: 5
   - Purpose: Maintains a pool of idle connections
   - Impact: Reduces connection creation overhead
   - Consideration: Should be lower than MaxOpenConns

3. **ConnMaxLifetime**

   - Default: 5 minutes
   - Purpose: Prevents stale connections
   - Impact: Ensures fresh connections
   - Consideration: Should be less than database connection timeout

4. **ConnMaxIdleTime**
   - Default: 1 minute
   - Purpose: Removes idle connections
   - Impact: Frees up resources
   - Consideration: Should be based on traffic patterns

### Environment Variables

```bash
DB_MAX_OPEN_CONNS=25
DB_MAX_IDLE_CONNS=5
DB_CONN_MAX_LIFETIME=5m
DB_CONN_MAX_IDLE_TIME=1m
```

## Best Practices

### 1. Sizing the Pool

- **MaxOpenConns**: Should be based on:

  - Expected concurrent requests
  - Available system resources
  - Database capacity
  - Formula: `(CPU cores * 2) + effective disk spindles`

- **MaxIdleConns**: Should be based on:
  - Average idle time between requests
  - Memory constraints
  - Formula: `MaxOpenConns * 0.25`

### 2. Connection Lifecycle

- Monitor connection usage
- Implement proper error handling
- Use context for timeouts
- Implement retry logic

### 3. Health Checks

- Regular pool health monitoring
- Connection validation
- Automatic pool recovery
- Metrics collection

## Monitoring

### Key Metrics

1. **Pool Status**

   - Open connections
   - Idle connections
   - Waiting requests
   - Connection errors

2. **Performance Metrics**
   - Connection acquisition time
   - Connection reuse rate
   - Connection errors
   - Pool saturation

### Health Checks

```go
// Example health check implementation
func (s *HealthServer) Check(ctx context.Context, req *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
    // Check database connection
    if err := s.db.PingContext(ctx); err != nil {
        return &grpc_health_v1.HealthCheckResponse{
            Status: grpc_health_v1.HealthCheckResponse_NOT_SERVING,
        }, nil
    }

    return &grpc_health_v1.HealthCheckResponse{
        Status: grpc_health_v1.HealthCheckResponse_SERVING,
    }, nil
}
```

## Error Handling

### Common Scenarios

1. **Connection Timeout**

   - Implement retry logic
   - Use exponential backoff
   - Log connection attempts

2. **Pool Exhaustion**

   - Monitor pool size
   - Implement circuit breaker
   - Add request queuing

3. **Connection Errors**
   - Proper error wrapping
   - Detailed error logging
   - Error categorization

## Implementation Example

```go
// Database connection setup with connection pooling
func setupDatabase(cfg *config.Config) (*sqlx.DB, error) {
    db, err := sqlx.Connect("postgres", cfg.GetDSN())
    if err != nil {
        return nil, fmt.Errorf("failed to connect to database: %v", err)
    }

    // Configure connection pool
    db.SetMaxOpenConns(cfg.MaxOpenConns)
    db.SetMaxIdleConns(cfg.MaxIdleConns)
    db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
    db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

    return db, nil
}
```

## Troubleshooting

### Common Issues

1. **Connection Leaks**

   - Symptoms: Growing number of connections
   - Solution: Implement proper connection closing
   - Prevention: Use defer statements

2. **Pool Exhaustion**

   - Symptoms: Connection timeouts
   - Solution: Adjust pool size
   - Prevention: Monitor pool metrics

3. **Stale Connections**
   - Symptoms: Connection errors
   - Solution: Adjust connection lifetime
   - Prevention: Regular health checks

## Next Steps

1. Implement connection retry logic
2. Add detailed metrics collection
3. Implement circuit breaker pattern
4. Add connection pool monitoring
5. Optimize pool configuration

## References

- [sqlx Documentation](https://github.com/jmoiron/sqlx)
- [PostgreSQL Connection Settings](https://www.postgresql.org/docs/current/runtime-config-connection.html)
- [Go Database Best Practices](https://golang.org/doc/database/best-practices)
