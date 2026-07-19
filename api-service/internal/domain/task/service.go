package task

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type Publisher interface {
	PublishWithRoutingKey(routingKey string, msg *Message) error
}

type Service struct {
	publisher Publisher
}

func NewService(publisher Publisher) *Service {
	return &Service{publisher: publisher}
}

func (s *Service) Submit(ctx context.Context, routingKey, msgType string, payload interface{}, metadata map[string]string) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	correlationID := uuid.New().String()

	// Every published message carries metadata.source="api-service" and a
	// trace_id, per the pinned envelope shape.
	if metadata == nil {
		metadata = map[string]string{}
	}
	metadata["source"] = "api-service"

	// When the caller's context carries an active span (the normal HTTP
	// path, via otelgin), this publish becomes a child producer span and
	// the envelope's trace_id is the real W3C trace ID (ADR-003.2). The
	// span context is also injected into metadata ("traceparent" key),
	// which the publisher copies into AMQP headers so workers can continue
	// the trace. On non-traced paths, legacy behavior is kept: a
	// caller-supplied trace_id is honored, otherwise the correlation ID is
	// used.
	var span trace.Span
	if trace.SpanContextFromContext(ctx).IsValid() {
		ctx, span = otel.Tracer("api-service/task").Start(ctx,
			"publish "+routingKey,
			trace.WithSpanKind(trace.SpanKindProducer),
			trace.WithAttributes(
				attribute.String("messaging.system", "rabbitmq"),
				attribute.String("messaging.operation", "publish"),
				attribute.String("messaging.rabbitmq.destination.routing_key", routingKey),
			),
		)
		defer span.End()

		otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(metadata))
		metadata["trace_id"] = span.SpanContext().TraceID().String()
	} else if _, ok := metadata["trace_id"]; !ok {
		metadata["trace_id"] = correlationID
	}

	msg := &Message{
		ID:            uuid.New().String(),
		Type:          msgType,
		Timestamp:     time.Now().UTC(),
		CorrelationID: correlationID,
		Payload:       body,
		Metadata:      metadata,
		Priority:      0,
	}

	if span != nil {
		span.SetAttributes(attribute.String("messaging.message.id", msg.ID))
	}

	if err := s.publisher.PublishWithRoutingKey(routingKey, msg); err != nil {
		if span != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "publish failed")
		}
		return "", err
	}

	return msg.ID, nil
}
