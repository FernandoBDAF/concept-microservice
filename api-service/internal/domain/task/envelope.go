package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// ErrUnknownRoutingKey is returned for routing keys outside the four
// contract task types (ADR-008.6): there is no default-tasks fallback, an
// unknown key is a publisher bug and fails fast. Handlers map it to 400.
var ErrUnknownRoutingKey = errors.New("unknown routing key: not one of the contract task types (ADR-008.6)")

// BuildEnvelope constructs the frozen-shape task envelope
// (graph-worker/shared/contracts/MESSAGE_FORMAT.md). It is the SINGLE
// construction point for every publish path — Service.Submit and the
// document-upload outbox path both go through it, so the envelope a task
// endpoint produces is byte-shape identical to the one the upload
// transaction stores (ADR-008.3).
//
// Every envelope carries metadata.source="api-service" and a trace_id.
// When ctx carries an active span (the normal HTTP path, via otelgin), a
// child producer span is created for the enqueue and the envelope's
// trace_id is the real W3C trace ID (ADR-003.2); the span context is also
// injected into metadata ("traceparent" key) so workers can continue the
// trace from the envelope. On non-traced paths, legacy behavior is kept: a
// caller-supplied trace_id is honored, otherwise the correlation ID is
// used.
//
// With the transactional outbox the broker publish happens later in the
// relay; the producer span therefore marks the enqueue moment (where the
// message entered the system), which is the honest boundary.
func BuildEnvelope(ctx context.Context, routingKey, msgType string, payload interface{}, metadata map[string]string) (*Message, error) {
	if !IsContractTaskType(routingKey) {
		return nil, fmt.Errorf("%w: %q", ErrUnknownRoutingKey, routingKey)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	correlationID := uuid.New().String()

	if metadata == nil {
		metadata = map[string]string{}
	}
	metadata["source"] = "api-service"

	if trace.SpanContextFromContext(ctx).IsValid() {
		spanCtx, span := otel.Tracer("api-service/task").Start(ctx,
			"publish "+routingKey,
			trace.WithSpanKind(trace.SpanKindProducer),
			trace.WithAttributes(
				attribute.String("messaging.system", "rabbitmq"),
				attribute.String("messaging.operation", "publish"),
				attribute.String("messaging.rabbitmq.destination.routing_key", routingKey),
			),
		)
		otel.GetTextMapPropagator().Inject(spanCtx, propagation.MapCarrier(metadata))
		metadata["trace_id"] = span.SpanContext().TraceID().String()
		defer span.End()

		msg := newMessage(msgType, correlationID, body, metadata)
		span.SetAttributes(attribute.String("messaging.message.id", msg.ID))
		return msg, nil
	}

	if _, ok := metadata["trace_id"]; !ok {
		metadata["trace_id"] = correlationID
	}
	return newMessage(msgType, correlationID, body, metadata), nil
}

func newMessage(msgType, correlationID string, body json.RawMessage, metadata map[string]string) *Message {
	return &Message{
		ID:            uuid.New().String(),
		Type:          msgType,
		Timestamp:     time.Now().UTC(),
		CorrelationID: correlationID,
		Payload:       body,
		Metadata:      metadata,
		Priority:      0,
	}
}
