package monitoring

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

// StorageServiceMetrics contains all metrics for the storage service
type StorageServiceMetrics struct {
	// Message processing metrics
	MessagesProcessed     *prometheus.CounterVec
	MessageProcessingTime *prometheus.HistogramVec
	MessagesInFlight      *prometheus.GaugeVec
	MessageErrors         *prometheus.CounterVec

	// Batch processing metrics
	BatchesProcessed    *prometheus.CounterVec
	BatchProcessingTime *prometheus.HistogramVec
	BatchSize           *prometheus.HistogramVec
	BatchOperations     *prometheus.CounterVec

	// DLQ metrics
	DLQMessages       *prometheus.CounterVec
	DLQRetries        *prometheus.CounterVec
	DLQProcessingTime *prometheus.HistogramVec

	// Storage operation metrics
	StorageOperations *prometheus.CounterVec
	StorageLatency    *prometheus.HistogramVec
	StorageErrors     *prometheus.CounterVec

	// Database metrics
	DatabaseConnections *prometheus.GaugeVec
	DatabaseQueries     *prometheus.CounterVec
	DatabaseQueryTime   *prometheus.HistogramVec

	// Queue consumer metrics
	QueueConnections   *prometheus.GaugeVec
	QueueConsumerLag   *prometheus.GaugeVec
	QueueReconnections *prometheus.CounterVec

	// Performance metrics
	ThroughputRate       *prometheus.GaugeVec
	ConcurrentOperations *prometheus.GaugeVec
	ResourceUtilization  *prometheus.GaugeVec

	// Health metrics
	ServiceHealth   *prometheus.GaugeVec
	ComponentHealth *prometheus.GaugeVec

	// Auto-tuning metrics
	AutoTuningAdjustments *prometheus.CounterVec
	OptimalBatchSize      *prometheus.GaugeVec
	LoadFactor            *prometheus.GaugeVec

	mu sync.RWMutex
}

// NewStorageServiceMetrics creates a new metrics instance
func NewStorageServiceMetrics() *StorageServiceMetrics {
	return &StorageServiceMetrics{
		// Message processing metrics
		MessagesProcessed: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "storage_messages_processed_total",
				Help: "Total number of messages processed by the storage service",
			},
			[]string{"operation", "status", "source"},
		),

		MessageProcessingTime: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "storage_message_processing_duration_seconds",
				Help:    "Time taken to process messages",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"operation", "status"},
		),

		MessagesInFlight: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "storage_messages_in_flight",
				Help: "Number of messages currently being processed",
			},
			[]string{"operation"},
		),

		MessageErrors: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "storage_message_errors_total",
				Help: "Total number of message processing errors",
			},
			[]string{"operation", "error_type"},
		),

		// Batch processing metrics
		BatchesProcessed: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "storage_batches_processed_total",
				Help: "Total number of batches processed",
			},
			[]string{"mode", "status"},
		),

		BatchProcessingTime: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "storage_batch_processing_duration_seconds",
				Help:    "Time taken to process batches",
				Buckets: []float64{0.1, 0.5, 1, 5, 10, 30, 60, 120},
			},
			[]string{"mode", "size_range"},
		),

		BatchSize: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "storage_batch_size",
				Help:    "Size of processed batches",
				Buckets: []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000},
			},
			[]string{"mode"},
		),

		BatchOperations: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "storage_batch_operations_total",
				Help: "Total number of operations in batches",
			},
			[]string{"mode", "operation", "status"},
		),

		// DLQ metrics
		DLQMessages: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "storage_dlq_messages_total",
				Help: "Total number of messages sent to DLQ",
			},
			[]string{"reason", "operation"},
		),

		DLQRetries: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "storage_dlq_retries_total",
				Help: "Total number of DLQ retry attempts",
			},
			[]string{"operation", "status"},
		),

		DLQProcessingTime: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "storage_dlq_processing_duration_seconds",
				Help:    "Time taken to process DLQ messages",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"operation"},
		),

		// Storage operation metrics
		StorageOperations: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "storage_operations_total",
				Help: "Total number of storage operations",
			},
			[]string{"operation", "status"},
		),

		StorageLatency: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "storage_operation_duration_seconds",
				Help:    "Time taken for storage operations",
				Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5},
			},
			[]string{"operation"},
		),

		StorageErrors: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "storage_operation_errors_total",
				Help: "Total number of storage operation errors",
			},
			[]string{"operation", "error_type"},
		),

		// Database metrics
		DatabaseConnections: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "storage_database_connections",
				Help: "Number of active database connections",
			},
			[]string{"state"},
		),

		DatabaseQueries: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "storage_database_queries_total",
				Help: "Total number of database queries",
			},
			[]string{"operation", "status"},
		),

		DatabaseQueryTime: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "storage_database_query_duration_seconds",
				Help:    "Time taken for database queries",
				Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1},
			},
			[]string{"operation"},
		),

		// Queue consumer metrics
		QueueConnections: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "storage_queue_connections",
				Help: "Number of active queue connections",
			},
			[]string{"state"},
		),

		QueueConsumerLag: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "storage_queue_consumer_lag",
				Help: "Consumer lag in the message queue",
			},
			[]string{"queue"},
		),

		QueueReconnections: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "storage_queue_reconnections_total",
				Help: "Total number of queue reconnections",
			},
			[]string{"reason"},
		),

		// Performance metrics
		ThroughputRate: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "storage_throughput_rate",
				Help: "Current throughput rate (operations per second)",
			},
			[]string{"operation_type"},
		),

		ConcurrentOperations: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "storage_concurrent_operations",
				Help: "Number of concurrent operations",
			},
			[]string{"operation_type"},
		),

		ResourceUtilization: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "storage_resource_utilization",
				Help: "Resource utilization percentage",
			},
			[]string{"resource_type"},
		),

		// Health metrics
		ServiceHealth: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "storage_service_health",
				Help: "Overall service health status (1=healthy, 0=unhealthy)",
			},
			[]string{"service"},
		),

		ComponentHealth: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "storage_component_health",
				Help: "Individual component health status (1=healthy, 0=unhealthy)",
			},
			[]string{"component"},
		),

		// Auto-tuning metrics
		AutoTuningAdjustments: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "storage_auto_tuning_adjustments_total",
				Help: "Total number of auto-tuning adjustments made",
			},
			[]string{"parameter", "direction"},
		),

		OptimalBatchSize: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "storage_optimal_batch_size",
				Help: "Current optimal batch size determined by auto-tuning",
			},
			[]string{"operation_type"},
		),

		LoadFactor: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "storage_load_factor",
				Help: "Current system load factor (0-1)",
			},
			[]string{"component"},
		),
	}
}

// RecordMessageProcessed records a processed message
func (m *StorageServiceMetrics) RecordMessageProcessed(operation, status, source string, duration time.Duration) {
	m.MessagesProcessed.WithLabelValues(operation, status, source).Inc()
	m.MessageProcessingTime.WithLabelValues(operation, status).Observe(duration.Seconds())
}

// RecordMessageInFlight records messages currently being processed
func (m *StorageServiceMetrics) RecordMessageInFlight(operation string, delta float64) {
	m.MessagesInFlight.WithLabelValues(operation).Add(delta)
}

// RecordMessageError records a message processing error
func (m *StorageServiceMetrics) RecordMessageError(operation, errorType string) {
	m.MessageErrors.WithLabelValues(operation, errorType).Inc()
}

// RecordBatchProcessed records a processed batch
func (m *StorageServiceMetrics) RecordBatchProcessed(mode, status string, size int, duration time.Duration) {
	sizeRange := m.getBatchSizeRange(size)
	m.BatchesProcessed.WithLabelValues(mode, status).Inc()
	m.BatchProcessingTime.WithLabelValues(mode, sizeRange).Observe(duration.Seconds())
	m.BatchSize.WithLabelValues(mode).Observe(float64(size))
}

// RecordBatchOperation records operations within batches
func (m *StorageServiceMetrics) RecordBatchOperation(mode, operation, status string) {
	m.BatchOperations.WithLabelValues(mode, operation, status).Inc()
}

// RecordDLQMessage records a message sent to DLQ
func (m *StorageServiceMetrics) RecordDLQMessage(reason, operation string) {
	m.DLQMessages.WithLabelValues(reason, operation).Inc()
}

// RecordDLQRetry records a DLQ retry attempt
func (m *StorageServiceMetrics) RecordDLQRetry(operation, status string, duration time.Duration) {
	m.DLQRetries.WithLabelValues(operation, status).Inc()
	m.DLQProcessingTime.WithLabelValues(operation).Observe(duration.Seconds())
}

// RecordStorageOperation records a storage operation
func (m *StorageServiceMetrics) RecordStorageOperation(operation, status string, duration time.Duration) {
	m.StorageOperations.WithLabelValues(operation, status).Inc()
	m.StorageLatency.WithLabelValues(operation).Observe(duration.Seconds())
}

// RecordStorageError records a storage operation error
func (m *StorageServiceMetrics) RecordStorageError(operation, errorType string) {
	m.StorageErrors.WithLabelValues(operation, errorType).Inc()
}

// RecordDatabaseConnections records database connection metrics
func (m *StorageServiceMetrics) RecordDatabaseConnections(state string, count float64) {
	m.DatabaseConnections.WithLabelValues(state).Set(count)
}

// RecordDatabaseQuery records a database query
func (m *StorageServiceMetrics) RecordDatabaseQuery(operation, status string, duration time.Duration) {
	m.DatabaseQueries.WithLabelValues(operation, status).Inc()
	m.DatabaseQueryTime.WithLabelValues(operation).Observe(duration.Seconds())
}

// RecordQueueConnections records queue connection metrics
func (m *StorageServiceMetrics) RecordQueueConnections(state string, count float64) {
	m.QueueConnections.WithLabelValues(state).Set(count)
}

// RecordQueueConsumerLag records consumer lag
func (m *StorageServiceMetrics) RecordQueueConsumerLag(queue string, lag float64) {
	m.QueueConsumerLag.WithLabelValues(queue).Set(lag)
}

// RecordQueueReconnection records a queue reconnection
func (m *StorageServiceMetrics) RecordQueueReconnection(reason string) {
	m.QueueReconnections.WithLabelValues(reason).Inc()
}

// RecordThroughput records current throughput rate
func (m *StorageServiceMetrics) RecordThroughput(operationType string, rate float64) {
	m.ThroughputRate.WithLabelValues(operationType).Set(rate)
}

// RecordConcurrentOperations records number of concurrent operations
func (m *StorageServiceMetrics) RecordConcurrentOperations(operationType string, count float64) {
	m.ConcurrentOperations.WithLabelValues(operationType).Set(count)
}

// RecordResourceUtilization records resource utilization
func (m *StorageServiceMetrics) RecordResourceUtilization(resourceType string, utilization float64) {
	m.ResourceUtilization.WithLabelValues(resourceType).Set(utilization)
}

// RecordServiceHealth records overall service health
func (m *StorageServiceMetrics) RecordServiceHealth(service string, healthy bool) {
	value := 0.0
	if healthy {
		value = 1.0
	}
	m.ServiceHealth.WithLabelValues(service).Set(value)
}

// RecordComponentHealth records individual component health
func (m *StorageServiceMetrics) RecordComponentHealth(component string, healthy bool) {
	value := 0.0
	if healthy {
		value = 1.0
	}
	m.ComponentHealth.WithLabelValues(component).Set(value)
}

// RecordAutoTuningAdjustment records an auto-tuning adjustment
func (m *StorageServiceMetrics) RecordAutoTuningAdjustment(parameter, direction string) {
	m.AutoTuningAdjustments.WithLabelValues(parameter, direction).Inc()
}

// RecordOptimalBatchSize records the current optimal batch size
func (m *StorageServiceMetrics) RecordOptimalBatchSize(operationType string, size float64) {
	m.OptimalBatchSize.WithLabelValues(operationType).Set(size)
}

// RecordLoadFactor records the current load factor
func (m *StorageServiceMetrics) RecordLoadFactor(component string, factor float64) {
	m.LoadFactor.WithLabelValues(component).Set(factor)
}

// Helper methods

func (m *StorageServiceMetrics) getBatchSizeRange(size int) string {
	switch {
	case size <= 10:
		return "small"
	case size <= 50:
		return "medium"
	case size <= 100:
		return "large"
	default:
		return "extra_large"
	}
}

// MetricsCollector provides a convenient interface for collecting metrics
type MetricsCollector struct {
	metrics *StorageServiceMetrics
	logger  *zap.Logger

	// Performance tracking
	throughputWindow  time.Duration
	throughputSamples []ThroughputSample
	mu                sync.RWMutex
}

// ThroughputSample represents a throughput measurement sample
type ThroughputSample struct {
	Timestamp  time.Time
	Operations int
	Duration   time.Duration
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector(logger *zap.Logger) *MetricsCollector {
	return &MetricsCollector{
		metrics:           NewStorageServiceMetrics(),
		logger:            logger,
		throughputWindow:  time.Minute,
		throughputSamples: make([]ThroughputSample, 0, 100),
	}
}

// GetMetrics returns the metrics instance
func (mc *MetricsCollector) GetMetrics() *StorageServiceMetrics {
	return mc.metrics
}

// StartPerformanceMonitoring starts continuous performance monitoring
func (mc *MetricsCollector) StartPerformanceMonitoring(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mc.collectPerformanceMetrics()
		}
	}
}

// collectPerformanceMetrics collects and records performance metrics
func (mc *MetricsCollector) collectPerformanceMetrics() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-mc.throughputWindow)

	// Filter recent samples
	var recentSamples []ThroughputSample
	for _, sample := range mc.throughputSamples {
		if sample.Timestamp.After(cutoff) {
			recentSamples = append(recentSamples, sample)
		}
	}
	mc.throughputSamples = recentSamples

	// Calculate throughput
	if len(recentSamples) > 0 {
		totalOps := 0
		for _, sample := range recentSamples {
			totalOps += sample.Operations
		}

		throughput := float64(totalOps) / mc.throughputWindow.Seconds()
		mc.metrics.RecordThroughput("all", throughput)

		mc.logger.Debug("Performance metrics collected",
			zap.Float64("throughput_ops_per_sec", throughput),
			zap.Int("sample_count", len(recentSamples)))
	}
}

// RecordThroughputSample records a throughput sample
func (mc *MetricsCollector) RecordThroughputSample(operations int, duration time.Duration) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	sample := ThroughputSample{
		Timestamp:  time.Now(),
		Operations: operations,
		Duration:   duration,
	}

	mc.throughputSamples = append(mc.throughputSamples, sample)

	// Keep only recent samples to prevent memory growth
	if len(mc.throughputSamples) > 1000 {
		mc.throughputSamples = mc.throughputSamples[100:]
	}
}

// HealthMonitor provides health monitoring capabilities
type HealthMonitor struct {
	metrics *StorageServiceMetrics
	logger  *zap.Logger
	checks  map[string]HealthCheckFunc
	mu      sync.RWMutex
}

// HealthCheckFunc represents a health check function
type HealthCheckFunc func(ctx context.Context) error

// NewHealthMonitor creates a new health monitor
func NewHealthMonitor(metrics *StorageServiceMetrics, logger *zap.Logger) *HealthMonitor {
	return &HealthMonitor{
		metrics: metrics,
		logger:  logger,
		checks:  make(map[string]HealthCheckFunc),
	}
}

// RegisterHealthCheck registers a health check
func (hm *HealthMonitor) RegisterHealthCheck(name string, check HealthCheckFunc) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.checks[name] = check
}

// StartHealthMonitoring starts continuous health monitoring
func (hm *HealthMonitor) StartHealthMonitoring(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			hm.performHealthChecks(ctx)
		}
	}
}

// performHealthChecks performs all registered health checks
func (hm *HealthMonitor) performHealthChecks(ctx context.Context) {
	hm.mu.RLock()
	checks := make(map[string]HealthCheckFunc)
	for name, check := range hm.checks {
		checks[name] = check
	}
	hm.mu.RUnlock()

	overallHealthy := true

	for name, check := range checks {
		checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		err := check(checkCtx)
		cancel()

		healthy := err == nil
		hm.metrics.RecordComponentHealth(name, healthy)

		if !healthy {
			overallHealthy = false
			hm.logger.Warn("Health check failed",
				zap.String("component", name),
				zap.Error(err))
		}
	}

	hm.metrics.RecordServiceHealth("storage-service", overallHealthy)
}
