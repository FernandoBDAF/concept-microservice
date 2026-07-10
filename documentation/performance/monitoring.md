# Performance Monitoring

> *Migrated from legacy_project/reference-materials/performance/monitoring.md*

## Overview

This document outlines the monitoring strategy for the API Service architecture, focusing on real-time performance metrics, alerts, and dashboards.

## Monitoring Categories

### Service Monitoring

- Request rates
- Response times
- Error rates
- Resource usage
- Health checks

### Infrastructure Monitoring

- CPU utilization
- Memory usage
- Disk I/O
- Network traffic
- Container metrics

### Business Metrics

- Active users
- Transaction volume
- Success rates
- Feature usage
- User experience

## Prometheus Metrics

### Request Metrics

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
```

### Cache Metrics

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
```

### Database Metrics

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
```

## Monitoring Tools

- Prometheus - Metrics collection
- Grafana - Visualization
- Loki - Log aggregation
- AlertManager - Alert routing

## Alerting Strategy

### Critical Alerts (P0)

```yaml
- alert: HighErrorRate
  expr: rate(http_requests_total{status=~"5.."}[5m]) / rate(http_requests_total[5m]) > 0.05
  for: 2m
  labels:
    severity: critical
  annotations:
    summary: "High error rate detected"
    description: "Error rate is {{ $value | humanizePercentage }}"

- alert: ServiceDown
  expr: up{job="api-service"} == 0
  for: 1m
  labels:
    severity: critical
  annotations:
    summary: "API Service is down"
```

### Warning Alerts (P1)

```yaml
- alert: HighLatency
  expr: histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m])) > 0.5
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "High latency detected"
    description: "p95 latency is {{ $value | humanizeDuration }}"

- alert: LowCacheHitRate
  expr: sum(rate(cache_hits_total[5m])) / (sum(rate(cache_hits_total[5m])) + sum(rate(cache_misses_total[5m]))) < 0.7
  for: 10m
  labels:
    severity: warning
  annotations:
    summary: "Low cache hit rate"
    description: "Cache hit rate is {{ $value | humanizePercentage }}"
```

### Info Alerts (P2)

```yaml
- alert: HighMemoryUsage
  expr: container_memory_usage_bytes / container_spec_memory_limit_bytes > 0.8
  for: 10m
  labels:
    severity: info
  annotations:
    summary: "High memory usage"
    description: "Memory usage is {{ $value | humanizePercentage }}"
```

## Dashboards

### Service Overview Dashboard

- Request rate (RPS)
- Error rate (%)
- Response time (p50, p95, p99)
- Concurrent connections
- Health status

### Cache Performance Dashboard

- Hit/miss ratio
- Cache latency
- Memory usage
- Eviction rate
- Key count

### Database Performance Dashboard

- Query latency
- Connection pool status
- Active queries
- Lock contention
- Replication lag

## Cross-References

- [Load Testing Strategy](load-testing-strategy.md)
- [Performance Benchmarking](benchmarking.md)
- [Performance Optimization](optimization.md)
- [Prometheus Guide](../development/tools/prometheus.md)
- [Grafana Guide](../development/tools/grafana.md)

## Notes

- Real-time monitoring
- Historical data retention
- Metric aggregation
- Alert noise reduction
- Dashboard maintenance
