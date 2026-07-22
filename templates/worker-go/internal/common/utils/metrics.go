package utils

import (
	"errors"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

// ProcessorMetrics provides a standard set of metrics for message processors
type ProcessorMetrics struct {
	ProcessingTime    prometheus.Histogram
	ProcessingErrors  prometheus.Counter
	ProcessingSuccess prometheus.Counter
	MessagesInFlight  prometheus.Gauge
}

// NewProcessorMetrics creates a new set of processor metrics. Registration
// is idempotent per workerType: constructing a processor more than once in
// the same process (e.g. from multiple tests in one package, or a future
// caller) reuses the already-registered collectors instead of panicking
// with prometheus.MustRegister's "duplicate metrics collector registration".
func NewProcessorMetrics(workerType string) *ProcessorMetrics {
	return &ProcessorMetrics{
		ProcessingTime: RegisterOrExisting(prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:    fmt.Sprintf("%s_processing_time_seconds", workerType),
				Help:    fmt.Sprintf("Time taken to process messages for %s worker", workerType),
				Buckets: prometheus.DefBuckets, // 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10
			},
		)),
		ProcessingErrors: RegisterOrExisting(prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: fmt.Sprintf("%s_processing_errors_total", workerType),
				Help: fmt.Sprintf("Total number of processing errors for %s worker", workerType),
			},
		)),
		ProcessingSuccess: RegisterOrExisting(prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: fmt.Sprintf("%s_processing_success_total", workerType),
				Help: fmt.Sprintf("Total number of successful processing for %s worker", workerType),
			},
		)),
		MessagesInFlight: RegisterOrExisting(prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: fmt.Sprintf("%s_messages_in_flight", workerType),
				Help: fmt.Sprintf("Number of messages currently being processed by %s worker", workerType),
			},
		)),
	}
}

// RegisterOrExisting registers c with the default Prometheus registry, or
// returns the already-registered collector of the same type/name if it was
// registered before. This makes metric construction safe to call more than
// once per process (tests, hypothetical multi-instantiation) without the
// panic that prometheus.MustRegister would raise.
func RegisterOrExisting[T prometheus.Collector](c T) T {
	if err := prometheus.Register(c); err != nil {
		var are prometheus.AlreadyRegisteredError
		if errors.As(err, &are) {
			if existing, ok := are.ExistingCollector.(T); ok {
				return existing
			}
		}
		panic(err)
	}
	return c
}

// RecordProcessingStart records the start of message processing
func (m *ProcessorMetrics) RecordProcessingStart() {
	m.MessagesInFlight.Inc()
}

// RecordProcessingSuccess records successful message processing
func (m *ProcessorMetrics) RecordProcessingSuccess() {
	m.ProcessingSuccess.Inc()
	m.MessagesInFlight.Dec()
}

// RecordProcessingError records failed message processing
func (m *ProcessorMetrics) RecordProcessingError() {
	m.ProcessingErrors.Inc()
	m.MessagesInFlight.Dec()
}

// StartTimer returns a timer for recording processing duration
func (m *ProcessorMetrics) StartTimer() *prometheus.Timer {
	return prometheus.NewTimer(m.ProcessingTime)
}
