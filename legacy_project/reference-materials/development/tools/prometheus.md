# Prometheus Usage Guide

## Overview

Prometheus is an open-source monitoring and alerting system. In our microservices architecture, we use Prometheus to collect metrics from our services, monitor their health, and trigger alerts when necessary.

## Key Features Used

### 1. Service Monitoring

We use Prometheus to monitor our services with custom metrics:

```yaml
# prometheus.yml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: "profile-service"
    static_configs:
      - targets: ["profile-service:8080"]
    metrics_path: "/metrics"
    scheme: "http"

  - job_name: "storage-service"
    static_configs:
      - targets: ["storage-service:8081"]
    metrics_path: "/metrics"
    scheme: "http"
```

### 2. Custom Metrics

We define custom metrics in our services:

```go
// Custom metrics definition
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
            Buckets: prometheus.DefBuckets,
        },
        []string{"method", "endpoint"},
    )
)

func init() {
    prometheus.MustRegister(httpRequestsTotal)
    prometheus.MustRegister(httpRequestDuration)
}
```

### 3. Alert Rules

We define alert rules for monitoring:

```yaml
# alert.rules
groups:
  - name: service_alerts
    rules:
      - alert: HighErrorRate
        expr: rate(http_requests_total{status=~"5.."}[5m]) > 0.1
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: High error rate detected
          description: "Service {{ $labels.service }} has high error rate"

      - alert: HighLatency
        expr: histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m])) > 1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: High latency detected
          description: "Service {{ $labels.service }} has high latency"
```

## Best Practices

1. **Metric Naming**

   - Use consistent naming conventions
   - Include units in metric names
   - Use appropriate metric types
   - Document metric purposes

2. **Labeling**

   - Use meaningful label names
   - Keep label cardinality low
   - Use consistent label values
   - Document label purposes

3. **Alerting**

   - Set appropriate thresholds
   - Use meaningful alert names
   - Include clear descriptions
   - Configure proper routing

4. **Performance**
   - Optimize scrape intervals
   - Use appropriate retention
   - Monitor Prometheus itself
   - Use recording rules

## Common Issues and Solutions

1. **High Cardinality**

   - Problem: Too many unique label combinations
   - Solution: Review and optimize label usage

2. **Memory Usage**

   - Problem: Prometheus using too much memory
   - Solution: Adjust retention and scrape intervals

3. **Scrape Failures**
   - Problem: Targets not being scraped
   - Solution: Check network and target configuration

## Examples from Our Project

### Profile Service Metrics

```go
// Profile service metrics
var (
    profileOperationsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "profile_operations_total",
            Help: "Total number of profile operations",
        },
        []string{"operation", "status"},
    )

    profileOperationDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "profile_operation_duration_seconds",
            Help:    "Profile operation duration in seconds",
            Buckets: prometheus.DefBuckets,
        },
        []string{"operation"},
    )
)

// Usage in handlers
func (s *ProfileService) GetProfile(ctx context.Context, id string) (*Profile, error) {
    timer := prometheus.NewTimer(profileOperationDuration.WithLabelValues("get"))
    defer timer.ObserveDuration()

    profile, err := s.repository.Get(ctx, id)
    if err != nil {
        profileOperationsTotal.WithLabelValues("get", "error").Inc()
        return nil, err
    }

    profileOperationsTotal.WithLabelValues("get", "success").Inc()
    return profile, nil
}
```

### Storage Service Metrics

```go
// Storage service metrics
var (
    storageOperationsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "storage_operations_total",
            Help: "Total number of storage operations",
        },
        []string{"operation", "status"},
    )

    storageOperationDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "storage_operation_duration_seconds",
            Help:    "Storage operation duration in seconds",
            Buckets: prometheus.DefBuckets,
        },
        []string{"operation"},
    )
)

// Usage in handlers
func (s *StorageService) StoreFile(ctx context.Context, file *File) error {
    timer := prometheus.NewTimer(storageOperationDuration.WithLabelValues("store"))
    defer timer.ObserveDuration()

    err := s.repository.Store(ctx, file)
    if err != nil {
        storageOperationsTotal.WithLabelValues("store", "error").Inc()
        return err
    }

    storageOperationsTotal.WithLabelValues("store", "success").Inc()
    return nil
}
```

## References

- [Prometheus Official Documentation](https://prometheus.io/docs/)
- [Prometheus Best Practices](https://prometheus.io/docs/practices/naming/)
- [Prometheus Alerting](https://prometheus.io/docs/alerting/latest/overview/)
