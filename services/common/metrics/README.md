# Metrics Package

A flexible metrics collection package for Go microservices with Prometheus integration.

## Overview

The metrics package provides a standardized way to collect, aggregate, and report metrics across services. It supports various metric types and integrates seamlessly with Prometheus, making it easy to monitor service performance and health.

## Features

- Multiple metric types (Counter, Gauge, Histogram, Timer)
- Prometheus integration with collectors
- Thread-safe operations using atomic operations
- Custom metric labels
- Metric aggregation
- Performance optimized
- Metric validation
- Default metrics collection
- Service-specific collectors (HTTP, Queue)
- Automatic middleware for HTTP metrics
- Vector metrics support with labels

## Installation

```bash
go get github.com/your-org/common/metrics
```

## Quick Start

```go
package main

import (
    "net/http"
    "github.com/your-org/common/metrics"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
    // Create a new HTTP metrics collector
    collector := metrics.NewHTTPCollector()

    // Create HTTP handler
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        // Record request
        collector.RecordHTTPRequest(r.Method, r.URL.Path, "200")

        // Record request duration
        collector.RecordHTTPRequestDuration(r.Method, r.URL.Path, 0.42)

        w.Write([]byte("Hello, World!"))
    })

    // Expose metrics endpoint
    http.Handle("/metrics", promhttp.HandlerFor(
        prometheus.Gatherers{
            prometheus.DefaultGatherer,
            metrics.DefaultRegistry.Registry,
        },
        promhttp.HandlerOpts{},
    ))

    http.ListenAndServe(":8080", nil)
}
```

## Metric Types

### Counter

```go
// Create a counter
counter := metrics.NewCounter("requests_total", "Total number of requests", nil)

// Increment counter
counter.Inc()

// Add specific value
counter.Add(5)

// Get current value
value := counter.Get()
```

### Gauge

```go
// Create a gauge
gauge := metrics.NewGauge("memory_usage", "Memory usage in bytes", nil)

// Set value
gauge.Set(1024)

// Increment/Decrement
gauge.Inc()
gauge.Dec()

// Add/Subtract
gauge.Add(100)
gauge.Sub(50)

// Get current value
value := gauge.Get()
```

### Histogram

```go
// Create a histogram with custom buckets
buckets := []float64{0.1, 0.5, 1.0, 2.5, 5.0}
histogram := metrics.NewHistogram("response_time", "Response time in seconds", nil, buckets)

// Record observations
histogram.Observe(0.1)
histogram.Observe(0.2)
histogram.Observe(0.3)

// Get statistics
stats := histogram.Get()
count := stats["count"]
sum := stats["sum"]
avg := stats["avg"]
p95 := stats["p95"]
```

### Timer

```go
// Create a timer
timer := metrics.NewTimer("operation_duration", "Operation duration in seconds", nil)

// Time an operation
timer.Start()
// ... perform operation ...
timer.Stop()

// Get statistics
stats := timer.Get()
count := stats["count"]
sum := stats["sum_seconds"]
avg := stats["avg_seconds"]
p95 := stats["p95_seconds"]
```

## Service-Specific Collectors

### HTTP Collector

The package includes a pre-configured HTTP metrics collector for common HTTP metrics:

```go
package main

import (
    "net/http"
    "github.com/your-org/common/metrics"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
    // Create HTTP metrics collector
    collector := metrics.NewHTTPCollector()

    // Create HTTP handler with metrics
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        // Record request
        collector.RecordHTTPRequest(r.Method, r.URL.Path, "200")

        // Record request duration
        collector.RecordHTTPRequestDuration(r.Method, r.URL.Path, 0.42)

        w.Write([]byte("Hello, World!"))
    })

    // Expose metrics endpoint
    http.Handle("/metrics", promhttp.HandlerFor(
        prometheus.Gatherers{
            prometheus.DefaultGatherer,
            metrics.DefaultRegistry.Registry,
        },
        promhttp.HandlerOpts{},
    ))

    http.ListenAndServe(":8080", nil)
}
```

### Queue Collector

The package includes a specialized collector for queue services:

```go
package main

import (
    "github.com/your-org/common/metrics"
    "github.com/gin-gonic/gin"
)

func main() {
    // Create queue metrics collector
    collector := metrics.NewQueueCollector()

    // Record queue metrics
    collector.SetQueueSize("default", 100)
    collector.RecordMessagePublished("default", "email")
    collector.RecordMessageProcessTime("default", "email", 0.5)
    collector.RecordMessageError("default", "timeout")
    collector.RecordQueueLatency("default", 0.1)
    collector.IncrementActiveConnections()
}
```

The Queue Collector provides the following metrics:

- `queue_size`: Current number of messages in the queue
- `queue_messages_published_total`: Total number of messages published
- `queue_message_process_time_seconds`: Time taken to process messages
- `queue_message_errors_total`: Total number of message processing errors
- `queue_latency_seconds`: Queue latency measurements
- `queue_active_connections`: Number of active connections

## HTTP Middleware

The package provides middleware for automatic HTTP metrics collection:

```go
package main

import (
    "github.com/your-org/common/metrics"
    "github.com/gin-gonic/gin"
)

func main() {
    router := gin.Default()
    collector := metrics.NewQueueCollector()

    // Add metrics middleware
    router.Use(func(c *gin.Context) {
        start := time.Now()
        c.Next()
        duration := time.Since(start).Seconds()
        collector.RecordHTTPRequest(c.Request.Method, c.Request.URL.Path, fmt.Sprintf("%d", c.Writer.Status()))
        collector.RecordHTTPRequestDuration(c.Request.Method, c.Request.URL.Path, duration)
    })

    // Your routes here...
}
```

## Best Practices

1. **Metric Naming**

   - Use descriptive names
   - Follow naming conventions (e.g., `queue_messages_published_total`)
   - Use appropriate units in names (e.g., `_seconds`, `_bytes`)
   - Be consistent across services

2. **Labels**

   - Use meaningful labels (e.g., `queue_name`, `message_type`)
   - Keep label cardinality low
   - Document label meanings
   - Validate label values
   - Use consistent label names across services

3. **Performance**

   - Use appropriate metric types for each use case
   - Avoid high-cardinality labels
   - Batch metric updates when possible
   - Monitor metric collection overhead
   - Use vector metrics for labeled data

4. **Service-Specific Metrics**

   - Include service-specific collectors
   - Track business metrics
   - Monitor error rates by type
   - Measure latency and throughput
   - Track resource usage

5. **Maintenance**

   - Document metrics and their purposes
   - Set up alerts for critical metrics
   - Monitor metric growth
   - Clean up unused metrics
   - Version metric names when making breaking changes

6. **Security**
   - Use non-root users in containers
   - Implement proper access controls
   - Sanitize metric labels
   - Monitor for metric abuse

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests
5. Submit a pull request

## License

This package is licensed under the MIT License - see the [LICENSE](../LICENSE) file for details.
