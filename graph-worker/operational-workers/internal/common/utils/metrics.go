package utils

import (
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

// NewProcessorMetrics creates a new set of processor metrics
func NewProcessorMetrics(workerType string) *ProcessorMetrics {
	metrics := &ProcessorMetrics{
		ProcessingTime: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:    fmt.Sprintf("%s_processing_time_seconds", workerType),
				Help:    fmt.Sprintf("Time taken to process messages for %s worker", workerType),
				Buckets: prometheus.DefBuckets, // 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10
			},
		),
		ProcessingErrors: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: fmt.Sprintf("%s_processing_errors_total", workerType),
				Help: fmt.Sprintf("Total number of processing errors for %s worker", workerType),
			},
		),
		ProcessingSuccess: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: fmt.Sprintf("%s_processing_success_total", workerType),
				Help: fmt.Sprintf("Total number of successful processing for %s worker", workerType),
			},
		),
		MessagesInFlight: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: fmt.Sprintf("%s_messages_in_flight", workerType),
				Help: fmt.Sprintf("Number of messages currently being processed by %s worker", workerType),
			},
		),
	}

	// Register metrics
	prometheus.MustRegister(
		metrics.ProcessingTime,
		metrics.ProcessingErrors,
		metrics.ProcessingSuccess,
		metrics.MessagesInFlight,
	)

	return metrics
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
