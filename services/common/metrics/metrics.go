package metrics

import (
	"sync"
)

// MetricType represents the type of metric
type MetricType string

const (
	CounterType   MetricType = "counter"
	GaugeType     MetricType = "gauge"
	HistogramType MetricType = "histogram"
	TimerType     MetricType = "timer"
)

// Metric represents the base interface for all metrics
type Metric interface {
	// Type returns the type of the metric
	Type() MetricType
	// Name returns the name of the metric
	Name() string
	// Help returns the help text for the metric
	Help() string
	// Labels returns the labels associated with the metric
	Labels() []string
}

// Counter represents a monotonically increasing counter
type Counter interface {
	Metric
	// Inc increments the counter by 1
	Inc()
	// Add increments the counter by the given value
	Add(value float64)
	// Get returns the current counter value
	Get() float64
}

// Gauge represents a metric that can go up and down
type Gauge interface {
	Metric
	// Set sets the gauge to the given value
	Set(value float64)
	// Inc increments the gauge by 1
	Inc()
	// Dec decrements the gauge by 1
	Dec()
	// Add adds the given value to the gauge
	Add(value float64)
	// Sub subtracts the given value from the gauge
	Sub(value float64)
	// Get returns the current gauge value
	Get() float64
}

// Histogram represents a metric that tracks the distribution of values
type Histogram interface {
	Metric
	// Observe records a value in the histogram
	Observe(value float64)
	// Get returns the current histogram statistics
	Get() map[string]float64
}

// Timer represents a metric that tracks the duration of operations
type Timer interface {
	Metric
	// Start starts the timer
	Start() Timer
	// Stop stops the timer and records the duration
	Stop()
	// Get returns the current timer statistics
	Get() map[string]float64
}

// baseMetric provides common functionality for all metrics
type baseMetric struct {
	mu     sync.RWMutex
	name   string
	help   string
	labels []string
	mType  MetricType
}

// Type returns the type of the metric
func (m *baseMetric) Type() MetricType {
	return m.mType
}

// Name returns the name of the metric
func (m *baseMetric) Name() string {
	return m.name
}

// Help returns the help text for the metric
func (m *baseMetric) Help() string {
	return m.help
}

// Labels returns the labels associated with the metric
func (m *baseMetric) Labels() []string {
	return m.labels
}
