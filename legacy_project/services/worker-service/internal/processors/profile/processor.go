package profile

import (
	"context"
	"time"

	"github.com/fernandobarroso/worker-service/internal/domain"
	"github.com/prometheus/client_golang/prometheus"
)

type Processor struct {
	metrics *Metrics
}

type Metrics struct {
	processingTime    prometheus.Histogram
	processingErrors  prometheus.Counter
	processingSuccess prometheus.Counter
}

func NewProcessor() *Processor {
	return &Processor{
		metrics: &Metrics{
			processingTime: prometheus.NewHistogram(
				prometheus.HistogramOpts{
					Name: "profile_processing_time_seconds",
					Help: "Time taken to process profile messages",
				},
			),
			processingErrors: prometheus.NewCounter(
				prometheus.CounterOpts{
					Name: "profile_processing_errors_total",
					Help: "Total number of profile processing errors",
				},
			),
			processingSuccess: prometheus.NewCounter(
				prometheus.CounterOpts{
					Name: "profile_processing_success_total",
					Help: "Total number of successful profile processing",
				},
			),
		},
	}
}

func (p *Processor) Process(ctx context.Context, msg *domain.ProfileMessage) error {
	timer := prometheus.NewTimer(p.metrics.processingTime)
	defer timer.ObserveDuration()

	// Validate message
	if err := p.Validate(msg); err != nil {
		p.metrics.processingErrors.Inc()
		return err
	}

	// Process based on action
	switch msg.Action {
	case "update":
		return p.handleUpdate(ctx, msg)
	case "delete":
		return p.handleDelete(ctx, msg)
	default:
		p.metrics.processingErrors.Inc()
		return domain.ErrInvalidAction
	}
}

func (p *Processor) Validate(msg *domain.ProfileMessage) error {
	return msg.Validate()
}

func (p *Processor) Type() string {
	return "profile"
}

func (p *Processor) handleUpdate(ctx context.Context, msg *domain.ProfileMessage) error {
	// TODO: Implement profile update logic
	// For now, just simulate processing
	time.Sleep(10 * time.Second)
	p.metrics.processingSuccess.Inc()
	return nil
}

func (p *Processor) handleDelete(ctx context.Context, msg *domain.ProfileMessage) error {
	// TODO: Implement profile delete logic
	// For now, just simulate processing
	time.Sleep(10 * time.Second)
	p.metrics.processingSuccess.Inc()
	return nil
}
