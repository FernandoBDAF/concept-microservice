# Monitoring Patterns

> *Migrated and adapted from legacy_project/reference-materials/development/patterns/monitoring-patterns.md*

## Overview

This document outlines the monitoring patterns and best practices for the API Service architecture with direct infrastructure access.

## Core Patterns

### 1. Metrics Collection

#### Service Metrics

```go
var (
    httpRequestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "http_requests_total",
            Help: "Total number of HTTP requests",
        },
        []string{"method", "endpoint", "status"},
    )

    httpRequestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "http_request_duration_seconds",
            Help:    "HTTP request duration in seconds",
            Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
        },
        []string{"method", "endpoint"},
    )
)

// Middleware for collecting metrics
func MetricsMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()
        
        c.Next()
        
        duration := time.Since(start).Seconds()
        status := strconv.Itoa(c.Writer.Status())
        
        httpRequestsTotal.WithLabelValues(c.Request.Method, c.FullPath(), status).Inc()
        httpRequestDuration.WithLabelValues(c.Request.Method, c.FullPath()).Observe(duration)
    }
}
```

#### Cache Metrics (Direct Redis)

```go
var (
    cacheHitsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "cache_hits_total",
            Help: "Total number of cache hits",
        },
        []string{"cache_type"},
    )

    cacheMissesTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "cache_misses_total",
            Help: "Total number of cache misses",
        },
        []string{"cache_type"},
    )

    cacheOperationDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "cache_operation_duration_seconds",
            Help:    "Cache operation duration in seconds",
            Buckets: []float64{.001, .005, .01, .025, .05, .1},
        },
        []string{"operation"},
    )
)

// Instrumented cache client
func (c *Cache) GetProfile(ctx context.Context, id string) (*Profile, error) {
    timer := prometheus.NewTimer(cacheOperationDuration.WithLabelValues("get"))
    defer timer.ObserveDuration()

    profile, err := c.getProfile(ctx, id)
    if err == nil {
        cacheHitsTotal.WithLabelValues("profile").Inc()
    } else if errors.Is(err, ErrCacheMiss) {
        cacheMissesTotal.WithLabelValues("profile").Inc()
    }

    return profile, err
}
```

#### Database Metrics (Direct PostgreSQL)

```go
var (
    dbQueryDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "db_query_duration_seconds",
            Help:    "Database query duration in seconds",
            Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1},
        },
        []string{"query_type"},
    )

    dbConnectionsActive = prometheus.NewGauge(
        prometheus.GaugeOpts{
            Name: "db_connections_active",
            Help: "Number of active database connections",
        },
    )

    dbConnectionsIdle = prometheus.NewGauge(
        prometheus.GaugeOpts{
            Name: "db_connections_idle",
            Help: "Number of idle database connections",
        },
    )
)

// Monitor connection pool
func StartDBMetrics(db *sqlx.DB, interval time.Duration) {
    go func() {
        ticker := time.NewTicker(interval)
        for range ticker.C {
            stats := db.Stats()
            dbConnectionsActive.Set(float64(stats.InUse))
            dbConnectionsIdle.Set(float64(stats.Idle))
        }
    }()
}
```

#### Queue Metrics (Direct RabbitMQ)

```go
var (
    messagesPublishedTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "messages_published_total",
            Help: "Total number of messages published",
        },
        []string{"exchange", "routing_key"},
    )

    publishDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "message_publish_duration_seconds",
            Help:    "Message publish duration in seconds",
            Buckets: []float64{.001, .005, .01, .025, .05, .1},
        },
        []string{"exchange"},
    )
)

// Instrumented publisher
func (p *Publisher) PublishTask(ctx context.Context, task *Task) error {
    timer := prometheus.NewTimer(publishDuration.WithLabelValues(p.exchange))
    defer timer.ObserveDuration()

    err := p.publish(ctx, task)
    if err == nil {
        messagesPublishedTotal.WithLabelValues(p.exchange, task.Type).Inc()
    }

    return err
}
```

### 2. Health Checks

```go
type HealthChecker struct {
    db          *sqlx.DB
    redis       *redis.Client
    rabbitmq    *amqp.Connection
}

func (hc *HealthChecker) CheckHealth(ctx context.Context) HealthStatus {
    status := HealthStatus{
        Status: "healthy",
        Checks: make(map[string]CheckResult),
    }

    // Database health
    dbErr := hc.db.PingContext(ctx)
    status.Checks["database"] = CheckResult{
        Healthy: dbErr == nil,
        Error:   errorString(dbErr),
    }

    // Redis health
    redisErr := hc.redis.Ping(ctx).Err()
    status.Checks["redis"] = CheckResult{
        Healthy: redisErr == nil,
        Error:   errorString(redisErr),
    }

    // RabbitMQ health
    rmqHealthy := hc.rabbitmq != nil && !hc.rabbitmq.IsClosed()
    status.Checks["rabbitmq"] = CheckResult{
        Healthy: rmqHealthy,
    }

    // Update overall status
    for _, check := range status.Checks {
        if !check.Healthy {
            status.Status = "unhealthy"
            break
        }
    }

    return status
}

// Health endpoint
func (h *HealthHandler) Health(c *gin.Context) {
    status := h.checker.CheckHealth(c.Request.Context())
    
    code := http.StatusOK
    if status.Status != "healthy" {
        code = http.StatusServiceUnavailable
    }
    
    c.JSON(code, status)
}
```

### 3. Alerting Rules

```yaml
groups:
  - name: api_service
    rules:
      - alert: HighErrorRate
        expr: |
          sum(rate(http_requests_total{status=~"5.."}[5m])) / 
          sum(rate(http_requests_total[5m])) > 0.05
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "High error rate in API Service"

      - alert: HighLatency
        expr: |
          histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m])) > 0.5
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High latency in API Service"

      - alert: LowCacheHitRate
        expr: |
          sum(rate(cache_hits_total[5m])) / 
          (sum(rate(cache_hits_total[5m])) + sum(rate(cache_misses_total[5m]))) < 0.7
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Low cache hit rate"

      - alert: DBConnectionPoolExhausted
        expr: db_connections_active >= 20
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "Database connection pool nearly exhausted"
```

## Dashboard Panels

### Key Metrics to Display

1. **Request Rate** - `rate(http_requests_total[5m])`
2. **Error Rate** - `rate(http_requests_total{status=~"5.."}[5m])`
3. **Response Time (p95)** - `histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))`
4. **Cache Hit Rate** - `sum(cache_hits_total) / (sum(cache_hits_total) + sum(cache_misses_total))`
5. **DB Connections** - `db_connections_active`

## Best Practices

1. **Use consistent labels** - Same labels across all metrics
2. **Set appropriate buckets** - Match expected latency distribution
3. **Monitor infrastructure directly** - Redis, PostgreSQL, RabbitMQ
4. **Implement health checks** - For all dependencies
5. **Set up alerts** - For critical thresholds

## Cross-References

- [Prometheus Guide](../tools/prometheus.md)
- [Grafana Guide](../tools/grafana.md)
- [Performance Monitoring](../../performance/monitoring.md)

## Notes

- All infrastructure is accessed directly
- Monitor client-side latency, not HTTP service calls
- Track connection pool stats for database and cache
