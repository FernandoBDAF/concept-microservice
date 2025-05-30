package queue

import (
	"github.com/prometheus/client_golang/prometheus"
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
