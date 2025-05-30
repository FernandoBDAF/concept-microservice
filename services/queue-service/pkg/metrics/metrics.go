package metrics

import (
	"github.com/FBDAF/microservices/services/common/metrics"
)

var defaultBuckets = []float64{0.1, 0.5, 1, 2, 5, 10}

// QueueMetrics holds all queue-related metrics
type QueueMetrics struct {
	MessagesTotal      metrics.Counter
	ProcessingDuration metrics.Histogram
	Size               metrics.Gauge
	ErrorsTotal        metrics.Counter
	RetriesTotal       metrics.Counter
	Latency            metrics.Histogram
	Consumers          metrics.Gauge
}

// NewQueueMetrics creates a new QueueMetrics instance
func NewQueueMetrics() *QueueMetrics {
	return &QueueMetrics{
		MessagesTotal: metrics.NewCounter(
			"queue_messages_total",
			"Total number of messages processed",
			[]string{"queue", "type"},
		),
		ProcessingDuration: metrics.NewHistogram(
			"queue_processing_duration_seconds",
			"Message processing duration in seconds",
			[]string{"queue", "type"},
			defaultBuckets,
		),
		Size: metrics.NewGauge(
			"queue_size",
			"Current queue size",
			[]string{"queue"},
		),
		ErrorsTotal: metrics.NewCounter(
			"queue_errors_total",
			"Total number of errors",
			[]string{"queue", "type", "error"},
		),
		RetriesTotal: metrics.NewCounter(
			"queue_retries_total",
			"Total number of retries",
			[]string{"queue", "type"},
		),
		Latency: metrics.NewHistogram(
			"queue_latency_seconds",
			"Message processing latency in seconds",
			[]string{"queue", "type"},
			defaultBuckets,
		),
		Consumers: metrics.NewGauge(
			"queue_consumers",
			"Number of active consumers",
			[]string{"queue"},
		),
	}
}

// Registry is the global metrics registry
var Registry = metrics.NewRegistry()

// DefaultMetrics is the default queue metrics instance
var DefaultMetrics = NewQueueMetrics()

func init() {
	// Register all metrics
	Registry.RegisterCounter(DefaultMetrics.MessagesTotal)
	Registry.RegisterHistogram(DefaultMetrics.ProcessingDuration)
	Registry.RegisterGauge(DefaultMetrics.Size)
	Registry.RegisterCounter(DefaultMetrics.ErrorsTotal)
	Registry.RegisterCounter(DefaultMetrics.RetriesTotal)
	Registry.RegisterHistogram(DefaultMetrics.Latency)
	Registry.RegisterGauge(DefaultMetrics.Consumers)
}
