# Metrics Package

A flexible metrics collection package for Go microservices with support for multiple backends and metric types.

## Overview

The metrics package provides a standardized way to collect, aggregate, and report metrics across services. It supports various metric types, backends, and aggregation methods, making it easy to monitor service performance and health.

## Features

- Multiple metric types (Counter, Gauge, Histogram, Timer)
- Prometheus integration
- Custom metric labels
- Metric aggregation
- Performance optimized
- Multiple backends support
- Metric validation
- Default metrics collection

## Installation

```bash
go get github.com/your-org/common/metrics
```

## Quick Start

```go
package main

import (
    "github.com/your-org/common/metrics"
)

func main() {
    // Create a new metrics collector
    collector := metrics.NewCollector()

    // Create metrics
    requestCounter := collector.Counter("http_requests_total", "Total number of HTTP requests")
    responseTime := collector.Histogram("http_response_time_seconds", "HTTP response time in seconds")
    activeUsers := collector.Gauge("active_users", "Number of active users")

    // Record metrics
    requestCounter.Inc()
    responseTime.Observe(0.42)
    activeUsers.Set(100)
}
```

## Metric Types

### Counter

```go
// Create a counter
counter := collector.Counter("requests_total", "Total number of requests")

// Increment counter
counter.Inc()

// Add specific value
counter.Add(5)

// Get current value
value := counter.Value()
```

### Gauge

```go
// Create a gauge
gauge := collector.Gauge("memory_usage", "Memory usage in bytes")

// Set value
gauge.Set(1024)

// Increment/Decrement
gauge.Inc()
gauge.Dec()

// Add/Subtract
gauge.Add(100)
gauge.Sub(50)
```

### Histogram

```go
// Create a histogram
histogram := collector.Histogram("response_time", "Response time in seconds")

// Record observations
histogram.Observe(0.1)
histogram.Observe(0.2)
histogram.Observe(0.3)

// Get statistics
count := histogram.Count()
sum := histogram.Sum()
avg := histogram.Mean()
```

### Timer

```go
// Create a timer
timer := collector.Timer("operation_duration", "Operation duration in seconds")

// Time an operation
timer.Time(func() {
    // Operation to time
    time.Sleep(100 * time.Millisecond)
})

// Get statistics
count := timer.Count()
sum := timer.Sum()
avg := timer.Mean()
```

## Prometheus Integration

```go
package main

import (
    "net/http"
    "github.com/your-org/common/metrics"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
    // Create collector with Prometheus backend
    collector := metrics.NewCollector(metrics.WithPrometheus())

    // Create metrics
    requestCounter := collector.Counter("http_requests_total", "Total number of HTTP requests")

    // Expose metrics endpoint
    http.Handle("/metrics", promhttp.Handler())
    http.ListenAndServe(":8080", nil)
}
```

## Best Practices

1. **Metric Naming**

   - Use descriptive names
   - Follow naming conventions
   - Use appropriate units
   - Be consistent

2. **Labels**

   - Use meaningful labels
   - Keep label cardinality low
   - Document label meanings
   - Validate label values

3. **Performance**

   - Use appropriate metric types
   - Avoid high-cardinality labels
   - Batch metric updates
   - Monitor metric collection overhead

4. **Maintenance**
   - Document metrics
   - Set up alerts
   - Monitor metric growth
   - Clean up unused metrics

## Examples

### HTTP Server

```go
package main

import (
    "net/http"
    "github.com/your-org/common/metrics"
)

func main() {
    collector := metrics.NewCollector()

    // Create metrics
    requestCounter := collector.Counter("http_requests_total", "Total number of HTTP requests")
    responseTime := collector.Histogram("http_response_time_seconds", "HTTP response time in seconds")

    // Create HTTP handler
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        requestCounter.Inc()

        timer := collector.Timer("request_duration", "Request duration in seconds")
        timer.Time(func() {
            // Handle request
            w.Write([]byte("Hello, World!"))
        })
    })

    http.ListenAndServe(":8080", nil)
}
```

### Database Operations

```go
package main

import (
    "database/sql"
    "github.com/your-org/common/metrics"
)

func main() {
    collector := metrics.NewCollector()

    // Create metrics
    queryCounter := collector.Counter("db_queries_total", "Total number of database queries")
    queryTime := collector.Histogram("db_query_time_seconds", "Database query time in seconds")

    // Execute query
    func executeQuery(db *sql.DB, query string) error {
        queryCounter.Inc()

        timer := collector.Timer("query_duration", "Query duration in seconds")
        return timer.Time(func() error {
            _, err := db.Exec(query)
            return err
        })
    }
}
```

### Custom Metrics

```go
package main

import (
    "github.com/your-org/common/metrics"
)

func main() {
    collector := metrics.NewCollector()

    // Create custom metric
    customMetric := collector.Counter("custom_metric", "Custom metric description",
        metrics.WithLabels("label1", "label2"),
        metrics.WithHelp("Detailed help message"),
    )

    // Record metric with labels
    customMetric.WithLabels("value1", "value2").Inc()
}
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests
5. Submit a pull request

## License

This package is licensed under the MIT License - see the [LICENSE](../LICENSE) file for details.
