# Monitoring Tools Guide

## Overview

This guide covers the monitoring tools and practices used in our microservices architecture. We use Prometheus for metrics collection, Grafana for visualization, and AlertManager for alerting, along with additional tools for distributed tracing and log aggregation.

## Core Monitoring Tools

### 1. Prometheus

Configuration for metrics collection:

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
      - targets: ["storage-service:8080"]
    metrics_path: "/metrics"
    scheme: "http"

alerting:
  alertmanagers:
    - static_configs:
        - targets: ["alertmanager:9093"]
```

### 2. Grafana

Dashboard configuration:

```json
{
  "dashboard": {
    "id": null,
    "title": "Profile Service Overview",
    "tags": ["profile-service", "microservices"],
    "timezone": "browser",
    "panels": [
      {
        "title": "Request Rate",
        "type": "graph",
        "datasource": "Prometheus",
        "targets": [
          {
            "expr": "rate(http_requests_total[5m])",
            "legendFormat": "{{method}} {{path}}"
          }
        ]
      },
      {
        "title": "Error Rate",
        "type": "graph",
        "datasource": "Prometheus",
        "targets": [
          {
            "expr": "rate(http_errors_total[5m])",
            "legendFormat": "{{method}} {{path}}"
          }
        ]
      },
      {
        "title": "Response Time",
        "type": "graph",
        "datasource": "Prometheus",
        "targets": [
          {
            "expr": "rate(http_request_duration_seconds_sum[5m]) / rate(http_request_duration_seconds_count[5m])",
            "legendFormat": "{{method}} {{path}}"
          }
        ]
      }
    ]
  }
}
```

### 3. AlertManager

Alert configuration:

```yaml
# alertmanager.yml
global:
  resolve_timeout: 5m

route:
  group_by: ["alertname", "service"]
  group_wait: 30s
  group_interval: 5m
  repeat_interval: 4h
  receiver: "slack-notifications"

receivers:
  - name: "slack-notifications"
    slack_configs:
      - api_url: "https://hooks.slack.com/services/..."
        channel: "#alerts"
        send_resolved: true
```

## Metrics Implementation

### 1. Service Metrics

```go
// Metrics implementation in services
type Metrics struct {
    httpRequestsTotal    *prometheus.CounterVec
    httpRequestDuration  *prometheus.HistogramVec
    httpErrorsTotal      *prometheus.CounterVec
}

func NewMetrics() *Metrics {
    return &Metrics{
        httpRequestsTotal: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "http_requests_total",
                Help: "Total number of HTTP requests",
            },
            []string{"method", "path", "status"},
        ),
        httpRequestDuration: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name:    "http_request_duration_seconds",
                Help:    "HTTP request duration in seconds",
                Buckets: prometheus.DefBuckets,
            },
            []string{"method", "path"},
        ),
        httpErrorsTotal: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "http_errors_total",
                Help: "Total number of HTTP errors",
            },
            []string{"method", "path", "error"},
        ),
    }
}
```

### 2. Middleware for Metrics

```go
// Metrics middleware
func MetricsMiddleware(metrics *Metrics) gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()
        path := c.Request.URL.Path
        method := c.Request.Method

        c.Next()

        duration := time.Since(start).Seconds()
        status := strconv.Itoa(c.Writer.Status())

        metrics.httpRequestsTotal.WithLabelValues(method, path, status).Inc()
        metrics.httpRequestDuration.WithLabelValues(method, path).Observe(duration)

        if c.Writer.Status() >= 400 {
            metrics.httpErrorsTotal.WithLabelValues(method, path, status).Inc()
        }
    }
}
```

## Health Checks

### 1. Service Health

```go
// Health check implementation
func (s *Service) HealthCheck() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        health := struct {
            Status    string            `json:"status"`
            Timestamp time.Time         `json:"timestamp"`
            Details   map[string]string `json:"details"`
        }{
            Status:    "healthy",
            Timestamp: time.Now(),
            Details:   make(map[string]string),
        }

        // Check database connection
        if err := s.db.Ping(); err != nil {
            health.Status = "unhealthy"
            health.Details["database"] = err.Error()
        }

        // Check cache connection
        if err := s.cache.Ping(); err != nil {
            health.Status = "unhealthy"
            health.Details["cache"] = err.Error()
        }

        w.Header().Set("Content-Type", "application/json")
        if health.Status == "unhealthy" {
            w.WriteHeader(http.StatusServiceUnavailable)
        }
        json.NewEncoder(w).Encode(health)
    }
}
```

## Best Practices

1. **Metrics Collection**

   - Use meaningful metric names
   - Add appropriate labels
   - Set reasonable scrape intervals
   - Monitor metric cardinality

2. **Alerting**

   - Set meaningful thresholds
   - Use proper alert grouping
   - Implement alert silencing
   - Monitor alert noise

3. **Dashboard Design**

   - Group related metrics
   - Use appropriate visualizations
   - Add context and documentation
   - Keep dashboards focused

4. **Performance**
   - Optimize query performance
   - Use recording rules
   - Monitor resource usage
   - Implement proper retention

## Common Issues and Solutions

1. **High Cardinality**

   - Problem: Too many unique label combinations
   - Solution: Limit label values, use aggregations

2. **Alert Fatigue**

   - Problem: Too many alerts
   - Solution: Adjust thresholds, group alerts

3. **Resource Usage**
   - Problem: High resource consumption
   - Solution: Optimize queries, adjust scrape intervals

## References

- [Prometheus Documentation](https://prometheus.io/docs/)
- [Grafana Documentation](https://grafana.com/docs/)
- [AlertManager Documentation](https://prometheus.io/docs/alerting/latest/alertmanager/)
- [Monitoring Best Practices](https://prometheus.io/docs/practices/naming/)
