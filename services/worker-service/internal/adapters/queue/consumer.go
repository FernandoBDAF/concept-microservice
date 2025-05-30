package queue

import (
	"context"

	"github.com/fernandobarroso/common/queue"
	"github.com/fernandobarroso/worker-service/internal/domain"
	"github.com/prometheus/client_golang/prometheus"
)

type Consumer struct {
	*queue.Consumer
	processor domain.Processor
	metrics   *Metrics
}

type Metrics struct {
	consumeLatency prometheus.Histogram
	consumeErrors  prometheus.Counter
	messageAge     prometheus.Histogram
}

func NewConsumer(config *queue.Config, processor domain.Processor) (*Consumer, error) {
	baseConsumer, err := queue.NewConsumer(config)
	if err != nil {
		return nil, err
	}

	metrics := &Metrics{
		consumeLatency: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name: "worker_consume_latency_seconds",
				Help: "Time taken to consume messages",
			},
		),
		consumeErrors: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "worker_consume_errors_total",
				Help: "Total number of consume errors",
			},
		),
		messageAge: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name: "worker_message_age_seconds",
				Help: "Age of messages when consumed",
			},
		),
	}

	return &Consumer{
		Consumer:  baseConsumer,
		processor: processor,
		metrics:   metrics,
	}, nil
}

func (c *Consumer) Start(ctx context.Context) error {
	handler := func(msg *queue.Message) error {
		timer := prometheus.NewTimer(c.metrics.consumeLatency)
		defer timer.ObserveDuration()

		// Convert to domain message
		profileMsg := &domain.ProfileMessage{
			Message: *msg,
		}

		// Process message
		err := c.processor.Process(ctx, profileMsg)
		if err != nil {
			c.metrics.consumeErrors.Inc()
			return err
		}

		return nil
	}

	return c.Consumer.Start(ctx, handler)
}
