package queue

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/fernandobarroso/microservices/operational-workers/internal/common/utils"
)

var (
	// Publisher Metrics
	messagesPublished = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "queue_messages_published_total",
			Help: "Total number of messages published",
		},
		[]string{"exchange", "routing_key"},
	)

	publishErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "queue_publish_errors_total",
			Help: "Total number of publish errors",
		},
		[]string{"exchange", "routing_key", "error"},
	)

	// Consumer Metrics
	messagesConsumed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "queue_messages_consumed_total",
			Help: "Total number of messages consumed",
		},
		[]string{"queue"},
	)

	consumeErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "queue_consume_errors_total",
			Help: "Total number of consume errors",
		},
		[]string{"queue", "error"},
	)

	processingTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "queue_message_processing_seconds",
			Help: "Time taken to process messages",
		},
		[]string{"queue"},
	)
)

func init() {
	prometheus.MustRegister(
		messagesPublished,
		publishErrors,
		messagesConsumed,
		consumeErrors,
		processingTime,
	)
}

// Publisher metrics helpers
func incrementMessagesPublished(exchange, routingKey string) {
	messagesPublished.WithLabelValues(exchange, routingKey).Inc()
}

func incrementPublishErrors(exchange, routingKey, err string) {
	publishErrors.WithLabelValues(exchange, routingKey, err).Inc()
}

// Consumer metrics helpers
func incrementMessagesConsumed(queue string) {
	messagesConsumed.WithLabelValues(queue).Inc()
}

func incrementConsumeErrors(queue, err string) {
	consumeErrors.WithLabelValues(queue, err).Inc()
}

func observeProcessingTime(queue string, duration float64) {
	processingTime.WithLabelValues(queue).Observe(duration)
}

// ── Per-worker retry / DLQ metrics (ADR-008.1) ──────────────────────────────
//
// Named <type>_retries_total{tier} and <type>_dlq_total to follow the
// per-worker naming convention in utils/metrics.go (e.g. email_processing_*).
// Registered lazily per worker type and deduped via utils.RegisterOrExisting
// so multiple workers/tests in one process don't panic on re-registration.

var (
	retryMetricsMu sync.Mutex
	retriesByType  = map[string]*prometheus.CounterVec{}
	dlqByType      = map[string]prometheus.Counter{}
)

func metricType(workerType string) string {
	if workerType == "" {
		return "worker"
	}
	return workerType
}

func incrementRetries(workerType, tier string) {
	t := metricType(workerType)
	retryMetricsMu.Lock()
	cv, ok := retriesByType[t]
	if !ok {
		cv = utils.RegisterOrExisting(prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: t + "_retries_total",
				Help: "Total messages republished to a retry wait-queue by the " + t + " worker",
			},
			[]string{"tier"},
		))
		retriesByType[t] = cv
	}
	retryMetricsMu.Unlock()
	cv.WithLabelValues(tier).Inc()
}

func incrementDLQ(workerType string) {
	t := metricType(workerType)
	retryMetricsMu.Lock()
	c, ok := dlqByType[t]
	if !ok {
		c = utils.RegisterOrExisting(prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: t + "_dlq_total",
				Help: "Total messages dead-lettered (exhausted retries or unretryable) by the " + t + " worker",
			},
		))
		dlqByType[t] = c
	}
	retryMetricsMu.Unlock()
	c.Inc()
}

// ── Per-worker task.result metrics (ADR-008.3) ──────────────────────────────

var (
	resultMetricsMu        sync.Mutex
	resultsPublishedByType = map[string]*prometheus.CounterVec{}
	resultErrorsByType     = map[string]prometheus.Counter{}
)

func incrementResultsPublished(workerType, status string) {
	t := metricType(workerType)
	resultMetricsMu.Lock()
	cv, ok := resultsPublishedByType[t]
	if !ok {
		cv = utils.RegisterOrExisting(prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: t + "_results_published_total",
				Help: "Total task.result messages published by the " + t + " worker",
			},
			[]string{"status"},
		))
		resultsPublishedByType[t] = cv
	}
	resultMetricsMu.Unlock()
	cv.WithLabelValues(status).Inc()
}

func incrementResultPublishErrors(workerType string) {
	t := metricType(workerType)
	resultMetricsMu.Lock()
	c, ok := resultErrorsByType[t]
	if !ok {
		c = utils.RegisterOrExisting(prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: t + "_result_publish_errors_total",
				Help: "Total task.result publish failures (best-effort; work-message routing unaffected) for the " + t + " worker",
			},
		))
		resultErrorsByType[t] = c
	}
	resultMetricsMu.Unlock()
	c.Inc()
}
