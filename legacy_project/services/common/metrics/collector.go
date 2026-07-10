package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// HTTPCollector is a wrapper around prometheus metrics for HTTP endpoints
type HTTPCollector struct {
	httpRequestsTotal   *prometheus.CounterVec
	httpRequestDuration *prometheus.HistogramVec
	activeUsers         prometheus.Gauge
}

// NewHTTPCollector creates a new HTTP metrics collector
func NewHTTPCollector() *HTTPCollector {
	collector := &HTTPCollector{
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

	// Register metrics with the default registry using adapters
	DefaultRegistry.RegisterCounter(NewPrometheusCounterVecAdapter(collector.httpRequestsTotal, []string{"method", "path", "status"}))
	DefaultRegistry.RegisterHistogram(NewPrometheusHistogramVecAdapter(collector.httpRequestDuration, []string{"method", "path"}))
	DefaultRegistry.RegisterGauge(NewPrometheusGaugeAdapter(collector.activeUsers))

	return collector
}

// RecordHTTPRequest records an HTTP request
func (c *HTTPCollector) RecordHTTPRequest(method, path, status string) {
	c.httpRequestsTotal.WithLabelValues(method, path, status).Inc()
}

// RecordHTTPRequestDuration records the duration of an HTTP request
func (c *HTTPCollector) RecordHTTPRequestDuration(method, path string, duration float64) {
	c.httpRequestDuration.WithLabelValues(method, path).Observe(duration)
}

// SetActiveUsers sets the number of active users
func (c *HTTPCollector) SetActiveUsers(count float64) {
	c.activeUsers.Set(count)
}

// IncrementActiveUsers increments the number of active users
func (c *HTTPCollector) IncrementActiveUsers() {
	c.activeUsers.Inc()
}

// DecrementActiveUsers decrements the number of active users
func (c *HTTPCollector) DecrementActiveUsers() {
	c.activeUsers.Dec()
}

// QueueCollector is a wrapper around prometheus metrics for queue operations
type QueueCollector struct {
	// HTTP metrics
	HTTPRequestsTotal   *prometheus.CounterVec
	HTTPRequestDuration *prometheus.HistogramVec

	// Queue metrics
	QueueSize          *prometheus.GaugeVec
	MessagePublishRate *prometheus.CounterVec
	MessageProcessTime *prometheus.HistogramVec
	MessageErrorRate   *prometheus.CounterVec
	QueueLatency       *prometheus.HistogramVec
	ActiveConnections  prometheus.Gauge
}

// NewQueueCollector creates a new queue metrics collector
func NewQueueCollector() *QueueCollector {
	collector := &QueueCollector{
		// HTTP metrics
		HTTPRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "queue_http_requests_total",
				Help: "Total number of HTTP requests to the queue service",
			},
			[]string{"method", "path", "status"},
		),
		HTTPRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "queue_http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path"},
		),

		// Queue metrics
		QueueSize: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "queue_size",
				Help: "Current number of messages in the queue",
			},
			[]string{"queue_name"},
		),
		MessagePublishRate: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "queue_messages_published_total",
				Help: "Total number of messages published to the queue",
			},
			[]string{"queue_name", "message_type"},
		),
		MessageProcessTime: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "queue_message_process_time_seconds",
				Help:    "Time taken to process a message",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"queue_name", "message_type"},
		),
		MessageErrorRate: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "queue_message_errors_total",
				Help: "Total number of message processing errors",
			},
			[]string{"queue_name", "error_type"},
		),
		QueueLatency: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "queue_latency_seconds",
				Help:    "Queue latency in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"queue_name"},
		),
		ActiveConnections: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "queue_active_connections",
				Help: "Number of active connections to the queue",
			},
		),
	}

	// Register metrics with the default registry using adapters
	DefaultRegistry.RegisterCounter(NewPrometheusCounterVecAdapter(collector.HTTPRequestsTotal, []string{"method", "path", "status"}))
	DefaultRegistry.RegisterHistogram(NewPrometheusHistogramVecAdapter(collector.HTTPRequestDuration, []string{"method", "path"}))
	DefaultRegistry.RegisterGauge(NewPrometheusGaugeVecAdapter(collector.QueueSize, []string{"queue_name"}))
	DefaultRegistry.RegisterCounter(NewPrometheusCounterVecAdapter(collector.MessagePublishRate, []string{"queue_name", "message_type"}))
	DefaultRegistry.RegisterHistogram(NewPrometheusHistogramVecAdapter(collector.MessageProcessTime, []string{"queue_name", "message_type"}))
	DefaultRegistry.RegisterCounter(NewPrometheusCounterVecAdapter(collector.MessageErrorRate, []string{"queue_name", "error_type"}))
	DefaultRegistry.RegisterHistogram(NewPrometheusHistogramVecAdapter(collector.QueueLatency, []string{"queue_name"}))
	DefaultRegistry.RegisterGauge(NewPrometheusGaugeAdapter(collector.ActiveConnections))

	return collector
}

// HTTP Metrics
func (c *QueueCollector) RecordHTTPRequest(method, path, status string) {
	c.HTTPRequestsTotal.WithLabelValues(method, path, status).Inc()
}

func (c *QueueCollector) RecordHTTPRequestDuration(method, path string, duration float64) {
	c.HTTPRequestDuration.WithLabelValues(method, path).Observe(duration)
}

// Queue Metrics
func (c *QueueCollector) SetQueueSize(queueName string, size float64) {
	c.QueueSize.WithLabelValues(queueName).Set(size)
}

func (c *QueueCollector) IncrementQueueSize(queueName string) {
	c.QueueSize.WithLabelValues(queueName).Inc()
}

func (c *QueueCollector) DecrementQueueSize(queueName string) {
	c.QueueSize.WithLabelValues(queueName).Dec()
}

func (c *QueueCollector) RecordMessagePublished(queueName, messageType string) {
	c.MessagePublishRate.WithLabelValues(queueName, messageType).Inc()
}

func (c *QueueCollector) RecordMessageProcessTime(queueName, messageType string, duration float64) {
	c.MessageProcessTime.WithLabelValues(queueName, messageType).Observe(duration)
}

func (c *QueueCollector) RecordMessageError(queueName, errorType string) {
	c.MessageErrorRate.WithLabelValues(queueName, errorType).Inc()
}

func (c *QueueCollector) RecordQueueLatency(queueName string, latency float64) {
	c.QueueLatency.WithLabelValues(queueName).Observe(latency)
}

func (c *QueueCollector) SetActiveConnections(count float64) {
	c.ActiveConnections.Set(count)
}

func (c *QueueCollector) IncrementActiveConnections() {
	c.ActiveConnections.Inc()
}

func (c *QueueCollector) DecrementActiveConnections() {
	c.ActiveConnections.Dec()
}
