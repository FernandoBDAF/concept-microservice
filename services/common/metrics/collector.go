package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Collector is a wrapper around prometheus metrics
type Collector struct {
	httpRequestsTotal   *prometheus.CounterVec
	httpRequestDuration *prometheus.HistogramVec
	activeUsers         prometheus.Gauge
}

// New creates a new metrics collector
func New() *Collector {
	return &Collector{
		httpRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "path", "status"},
		),
		httpRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path"},
		),
		activeUsers: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "active_users",
				Help: "Number of active users",
			},
		),
	}
}

// RecordHTTPRequest records an HTTP request
func (c *Collector) RecordHTTPRequest(method, path, status string) {
	c.httpRequestsTotal.WithLabelValues(method, path, status).Inc()
}

// RecordHTTPRequestDuration records the duration of an HTTP request
func (c *Collector) RecordHTTPRequestDuration(method, path string, duration float64) {
	c.httpRequestDuration.WithLabelValues(method, path).Observe(duration)
}

// SetActiveUsers sets the number of active users
func (c *Collector) SetActiveUsers(count float64) {
	c.activeUsers.Set(count)
}

// IncrementActiveUsers increments the number of active users
func (c *Collector) IncrementActiveUsers() {
	c.activeUsers.Inc()
}

// DecrementActiveUsers decrements the number of active users
func (c *Collector) DecrementActiveUsers() {
	c.activeUsers.Dec()
}
