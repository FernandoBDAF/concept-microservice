package metrics

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all Prometheus metrics for the cache service
type Metrics struct {
	// Cache operation counters
	CacheHits   prometheus.Counter
	CacheMisses prometheus.Counter
	CacheErrors prometheus.Counter

	// Cache operation histograms
	CacheLatency prometheus.HistogramVec

	// Batch operation metrics
	BatchOperations prometheus.CounterVec
	BatchSize       prometheus.HistogramVec

	// Redis connection metrics
	RedisConnections prometheus.Gauge
	RedisErrors      prometheus.CounterVec

	// Circuit breaker metrics
	CircuitBreakerState prometheus.GaugeVec
	CircuitBreakerTrips prometheus.CounterVec

	// Profile cache specific metrics
	ProfileCacheOps prometheus.CounterVec

	// Task cache specific metrics
	TaskCacheOps prometheus.CounterVec

	// Session cache specific metrics
	SessionCacheOps prometheus.CounterVec

	// Cache invalidation metrics
	InvalidationOps prometheus.CounterVec

	// HTTP request metrics
	HTTPRequests prometheus.CounterVec
	HTTPDuration prometheus.HistogramVec

	// gRPC request metrics
	GRPCRequests prometheus.CounterVec
	GRPCDuration prometheus.HistogramVec
}

// NewMetrics creates and registers Prometheus metrics
func NewMetrics() *Metrics {
	return &Metrics{
		CacheHits: promauto.NewCounter(prometheus.CounterOpts{
			Name: "cache_hits_total",
			Help: "Total number of cache hits",
		}),

		CacheMisses: promauto.NewCounter(prometheus.CounterOpts{
			Name: "cache_misses_total",
			Help: "Total number of cache misses",
		}),

		CacheErrors: promauto.NewCounter(prometheus.CounterOpts{
			Name: "cache_errors_total",
			Help: "Total number of cache errors",
		}),

		CacheLatency: *promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "cache_operation_duration_seconds",
			Help:    "Histogram of cache operation latencies",
			Buckets: prometheus.ExponentialBuckets(0.0001, 2, 15), // 0.1ms to ~3s
		}, []string{"operation", "status"}),

		BatchOperations: *promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "cache_batch_operations_total",
			Help: "Total number of batch operations",
		}, []string{"operation", "status"}),

		BatchSize: *promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "cache_batch_size",
			Help:    "Histogram of batch operation sizes",
			Buckets: prometheus.ExponentialBuckets(1, 2, 10), // 1 to 512 items
		}, []string{"operation"}),

		RedisConnections: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "redis_connections_active",
			Help: "Number of active Redis connections",
		}),

		RedisErrors: *promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "redis_errors_total",
			Help: "Total number of Redis errors",
		}, []string{"type"}),

		CircuitBreakerState: *promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "circuit_breaker_state",
			Help: "Circuit breaker state (0=closed, 1=half-open, 2=open)",
		}, []string{"name"}),

		CircuitBreakerTrips: *promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "circuit_breaker_trips_total",
			Help: "Total number of circuit breaker trips",
		}, []string{"name"}),

		ProfileCacheOps: *promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "profile_cache_operations_total",
			Help: "Total number of profile cache operations",
		}, []string{"operation", "status"}),

		TaskCacheOps: *promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "task_cache_operations_total",
			Help: "Total number of task cache operations",
		}, []string{"operation", "status"}),

		SessionCacheOps: *promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "session_cache_operations_total",
			Help: "Total number of session cache operations",
		}, []string{"operation", "status"}),

		InvalidationOps: *promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "cache_invalidation_operations_total",
			Help: "Total number of cache invalidation operations",
		}, []string{"pattern", "result"}),

		HTTPRequests: *promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		}, []string{"method", "endpoint", "status_code"}),

		HTTPDuration: *promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Histogram of HTTP request latencies",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "endpoint"}),

		GRPCRequests: *promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "grpc_requests_total",
			Help: "Total number of gRPC requests",
		}, []string{"method", "status_code"}),

		GRPCDuration: *promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "grpc_request_duration_seconds",
			Help:    "Histogram of gRPC request latencies",
			Buckets: prometheus.DefBuckets,
		}, []string{"method"}),
	}
}

// RecordCacheHit records a cache hit
func (m *Metrics) RecordCacheHit() {
	m.CacheHits.Inc()
}

// RecordCacheMiss records a cache miss
func (m *Metrics) RecordCacheMiss() {
	m.CacheMisses.Inc()
}

// RecordCacheError records a cache error
func (m *Metrics) RecordCacheError() {
	m.CacheErrors.Inc()
}

// RecordCacheLatency records cache operation latency
func (m *Metrics) RecordCacheLatency(operation, status string, duration time.Duration) {
	m.CacheLatency.WithLabelValues(operation, status).Observe(duration.Seconds())
}

// RecordBatchOperation records a batch operation
func (m *Metrics) RecordBatchOperation(operation, status string, size int) {
	m.BatchOperations.WithLabelValues(operation, status).Inc()
	m.BatchSize.WithLabelValues(operation).Observe(float64(size))
}

// SetRedisConnections sets the number of active Redis connections
func (m *Metrics) SetRedisConnections(count int) {
	m.RedisConnections.Set(float64(count))
}

// RecordRedisError records a Redis error
func (m *Metrics) RecordRedisError(errorType string) {
	m.RedisErrors.WithLabelValues(errorType).Inc()
}

// SetCircuitBreakerState sets circuit breaker state
func (m *Metrics) SetCircuitBreakerState(name, state string) {
	var stateValue float64
	switch state {
	case "closed":
		stateValue = 0
	case "half-open":
		stateValue = 1
	case "open":
		stateValue = 2
	}
	m.CircuitBreakerState.WithLabelValues(name).Set(stateValue)
}

// RecordCircuitBreakerTrip records a circuit breaker trip
func (m *Metrics) RecordCircuitBreakerTrip(name string) {
	m.CircuitBreakerTrips.WithLabelValues(name).Inc()
}

// RecordProfileCacheOp records a profile cache operation
func (m *Metrics) RecordProfileCacheOp(operation, status string) {
	m.ProfileCacheOps.WithLabelValues(operation, status).Inc()
}

// RecordTaskCacheOp records a task cache operation
func (m *Metrics) RecordTaskCacheOp(operation, status string) {
	m.TaskCacheOps.WithLabelValues(operation, status).Inc()
}

// RecordSessionCacheOp records a session cache operation
func (m *Metrics) RecordSessionCacheOp(operation, status string) {
	m.SessionCacheOps.WithLabelValues(operation, status).Inc()
}

// RecordHTTPRequest records an HTTP request
func (m *Metrics) RecordHTTPRequest(method, endpoint string, statusCode int, duration time.Duration) {
	m.HTTPRequests.WithLabelValues(method, endpoint, strconv.Itoa(statusCode)).Inc()
	m.HTTPDuration.WithLabelValues(method, endpoint).Observe(duration.Seconds())
}

// RecordGRPCRequest records a gRPC request
func (m *Metrics) RecordGRPCRequest(method string, statusCode int, duration time.Duration) {
	m.GRPCRequests.WithLabelValues(method, strconv.Itoa(statusCode)).Inc()
	m.GRPCDuration.WithLabelValues(method).Observe(duration.Seconds())
}

// RecordInvalidationOp records cache invalidation operations
func (m *Metrics) RecordInvalidationOp(pattern, result string) {
	m.InvalidationOps.WithLabelValues(pattern, result).Inc()
}
