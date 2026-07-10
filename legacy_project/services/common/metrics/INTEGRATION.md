# Metrics Package Integration Guide

This guide explains how to integrate the metrics package into your services and best practices for using it effectively.

## Table of Contents

1. [Getting Started](#getting-started)
2. [Service Integration](#service-integration)
3. [Common Use Cases](#common-use-cases)
4. [Best Practices](#best-practices)
5. [Troubleshooting](#troubleshooting)

## Getting Started

### Prerequisites

- Go 1.21 or later
- Prometheus client library
- Basic understanding of metrics and monitoring

### Installation

Add the metrics package to your service:

```bash
go get github.com/FBDAF/microservices/services/common/metrics
```

## Service Integration

### 1. Initialize Metrics

Create a metrics registry in your service's main function:

```go
package main

import (
    "github.com/FBDAF/microservices/services/common/metrics"
)

func main() {
    // Create metrics registry
    registry := metrics.NewRegistry()

    // Initialize service metrics
    initServiceMetrics(registry)

    // Start HTTP server with metrics endpoint
    startServer(registry)
}
```

### 2. Define Service Metrics

Create a dedicated package for your service's metrics:

```go
// metrics/service.go
package metrics

import (
    "github.com/FBDAF/microservices/services/common/metrics"
)

var (
    // HTTP metrics
    RequestCounter   *metrics.Counter
    RequestDuration  *metrics.Timer
    ResponseSize     *metrics.Histogram
    ActiveRequests   *metrics.Gauge

    // Business metrics
    ProcessedItems   *metrics.Counter
    ProcessingTime   *metrics.Timer
    QueueSize        *metrics.Gauge
    ErrorCounter     *metrics.Counter
)

func InitServiceMetrics(registry *metrics.Registry) {
    // HTTP metrics
    RequestCounter = metrics.NewCounter(
        "http_requests_total",
        "Total number of HTTP requests",
        []string{"method", "path", "status"},
    )
    registry.RegisterCounter(RequestCounter)

    RequestDuration = metrics.NewTimer(
        "http_request_duration_seconds",
        "HTTP request duration in seconds",
        []string{"method", "path"},
    )
    registry.RegisterTimer(RequestDuration)

    ResponseSize = metrics.NewHistogram(
        "http_response_size_bytes",
        "HTTP response size in bytes",
        []string{"method", "path"},
        []float64{100, 1000, 10000, 100000},
    )
    registry.RegisterHistogram(ResponseSize)

    ActiveRequests = metrics.NewGauge(
        "http_active_requests",
        "Number of active HTTP requests",
        []string{"method", "path"},
    )
    registry.RegisterGauge(ActiveRequests)

    // Business metrics
    ProcessedItems = metrics.NewCounter(
        "processed_items_total",
        "Total number of processed items",
        []string{"type", "status"},
    )
    registry.RegisterCounter(ProcessedItems)

    ProcessingTime = metrics.NewTimer(
        "processing_duration_seconds",
        "Item processing duration in seconds",
        []string{"type"},
    )
    registry.RegisterTimer(ProcessingTime)

    QueueSize = metrics.NewGauge(
        "queue_size",
        "Current queue size",
        []string{"type"},
    )
    registry.RegisterGauge(QueueSize)

    ErrorCounter = metrics.NewCounter(
        "errors_total",
        "Total number of errors",
        []string{"type", "code"},
    )
    registry.RegisterCounter(ErrorCounter)
}
```

### 3. Use Metrics in Handlers

```go
// handlers/http.go
package handlers

import (
    "net/http"
    "time"

    "your-service/metrics"
)

func HandleRequest(w http.ResponseWriter, r *http.Request) {
    // Record request
    metrics.RequestCounter.Inc()

    // Start timer
    timer := metrics.RequestDuration.Start()
    defer timer.Stop()

    // Track active requests
    metrics.ActiveRequests.Inc()
    defer metrics.ActiveRequests.Dec()

    // Process request
    // ...

    // Record response size
    metrics.ResponseSize.Observe(float64(len(response)))
}
```

### 4. Expose Metrics Endpoint

```go
// server/server.go
package server

import (
    "net/http"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

func StartServer(registry *metrics.Registry) {
    // Register collectors with Prometheus
    for _, collector := range registry.Collectors() {
        prometheus.MustRegister(collector)
    }

    // Expose metrics endpoint
    http.Handle("/metrics", promhttp.Handler())

    // Start server
    http.ListenAndServe(":8080", nil)
}
```

## Common Use Cases

### 1. HTTP Service Metrics

```go
func HTTPMetricsMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Record request
        metrics.RequestCounter.Inc()

        // Start timer
        timer := metrics.RequestDuration.Start()
        defer timer.Stop()

        // Track active requests
        metrics.ActiveRequests.Inc()
        defer metrics.ActiveRequests.Dec()

        // Create response writer to capture size
        rw := &responseWriter{ResponseWriter: w}
        next.ServeHTTP(rw, r)

        // Record response size
        metrics.ResponseSize.Observe(float64(rw.size))
    })
}
```

### 2. Background Job Metrics

```go
func ProcessJob(job *Job) {
    // Record processing start
    metrics.ProcessedItems.Inc()
    metrics.QueueSize.Dec()

    // Start timer
    timer := metrics.ProcessingTime.Start()
    defer timer.Stop()

    // Process job
    err := process(job)
    if err != nil {
        metrics.ErrorCounter.Inc()
    }
}
```

### 3. Database Metrics

```go
// metrics/database.go
package metrics

import (
    "github.com/FBDAF/microservices/services/common/metrics"
)

var (
    // Database metrics
    DBConnections    *metrics.Gauge
    DBQueryDuration  *metrics.Timer
    DBQueryErrors    *metrics.Counter
    DBQueryLatency   *metrics.Histogram
)

func InitDatabaseMetrics(registry *metrics.Registry) {
    DBConnections = metrics.NewGauge(
        "db_connections",
        "Number of active database connections",
        []string{"database", "type"},
    )
    registry.RegisterGauge(DBConnections)

    DBQueryDuration = metrics.NewTimer(
        "db_query_duration_seconds",
        "Database query duration in seconds",
        []string{"database", "query_type"},
    )
    registry.RegisterTimer(DBQueryDuration)

    DBQueryErrors = metrics.NewCounter(
        "db_query_errors_total",
        "Total number of database query errors",
        []string{"database", "error_type"},
    )
    registry.RegisterCounter(DBQueryErrors)

    DBQueryLatency = metrics.NewHistogram(
        "db_query_latency_seconds",
        "Database query latency in seconds",
        []string{"database", "query_type"},
        []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0},
    )
    registry.RegisterHistogram(DBQueryLatency)
}
```

### 4. Cache Metrics

```go
// metrics/cache.go
package metrics

import (
    "github.com/FBDAF/microservices/services/common/metrics"
)

var (
    // Cache metrics
    CacheHits       *metrics.Counter
    CacheMisses     *metrics.Counter
    CacheSize       *metrics.Gauge
    CacheLatency    *metrics.Timer
)

func InitCacheMetrics(registry *metrics.Registry) {
    CacheHits = metrics.NewCounter(
        "cache_hits_total",
        "Total number of cache hits",
        []string{"cache", "type"},
    )
    registry.RegisterCounter(CacheHits)

    CacheMisses = metrics.NewCounter(
        "cache_misses_total",
        "Total number of cache misses",
        []string{"cache", "type"},
    )
    registry.RegisterCounter(CacheMisses)

    CacheSize = metrics.NewGauge(
        "cache_size_bytes",
        "Current cache size in bytes",
        []string{"cache", "type"},
    )
    registry.RegisterGauge(CacheSize)

    CacheLatency = metrics.NewTimer(
        "cache_operation_duration_seconds",
        "Cache operation duration in seconds",
        []string{"cache", "operation"},
    )
    registry.RegisterTimer(CacheLatency)
}
```

## Best Practices

1. **Metric Naming**

   - Use descriptive names that indicate what is being measured
   - Follow Prometheus naming conventions (snake_case)
   - Include units in the name (e.g., `_seconds`, `_bytes`)
   - Use consistent naming patterns across related metrics

2. **Labels**

   - Use labels to add dimensions to your metrics
   - Keep the number of label combinations reasonable
   - Use consistent label names across related metrics
   - Avoid high-cardinality labels (e.g., user IDs, request IDs)

3. **Help Text**

   - Provide clear and concise descriptions
   - Include units and expected ranges
   - Document any special conditions or edge cases
   - Keep help text consistent across related metrics

4. **Performance**

   - Use atomic operations for counters and gauges
   - Use RWMutex for histograms and timers
   - Consider the impact of high-cardinality labels
   - Batch metric updates when possible

5. **Prometheus Integration**

   - Register metrics with a registry
   - Expose metrics on a dedicated endpoint
   - Use appropriate metric types for Prometheus
   - Configure appropriate scrape intervals

## Troubleshooting

### Common Issues

1. **Missing Metrics**

   - Check if metrics are properly registered
   - Verify metric names and labels
   - Ensure metrics are being updated
   - Check Prometheus configuration

2. **High Cardinality**

   - Review label usage
   - Consider aggregating metrics
   - Use appropriate bucket sizes
   - Monitor metric cardinality

3. **Performance Issues**

   - Check metric update frequency
   - Review label cardinality
   - Monitor memory usage
   - Profile metric operations

4. **Prometheus Integration**

   - Verify endpoint configuration
   - Check scrape settings
   - Monitor scrape duration
   - Review metric format

### Debugging Tips

1. **Enable Debug Logging**

   ```go
   import "github.com/sirupsen/logrus"

   logrus.SetLevel(logrus.DebugLevel)
   ```

2. **Check Metric Values**

   ```go
   // Print metric value
   fmt.Printf("Counter value: %d\n", counter.Get())

   // Print histogram statistics
   stats := histogram.Get()
   fmt.Printf("Histogram stats: %+v\n", stats)
   ```

3. **Verify Prometheus Configuration**

   ```yaml
   scrape_configs:
     - job_name: "your-service"
       scrape_interval: 15s
       static_configs:
         - targets: ["localhost:8080"]
   ```

4. **Monitor Resource Usage**

   ```go
   // Track memory usage
   var m runtime.MemStats
   runtime.ReadMemStats(&m)
   fmt.Printf("Memory usage: %d MB\n", m.Alloc/1024/1024)
   ```

## Additional Resources

- [Prometheus Documentation](https://prometheus.io/docs/)
- [Prometheus Best Practices](https://prometheus.io/docs/practices/naming/)
- [Go Metrics Best Practices](https://pkg.go.dev/github.com/prometheus/client_golang/prometheus)
- [Monitoring Distributed Systems](https://sre.google/sre-book/monitoring-distributed-systems/)
