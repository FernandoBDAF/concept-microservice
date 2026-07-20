package queue

import (
	"context"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// extractTraceContext returns a context carrying the remote span context
// propagated in the delivery headers (W3C "traceparent"/"tracestate" and
// "baggage", injected by the api-service publisher via envelope metadata →
// AMQP headers). Non-string header values (amqp.Table is map[string]any)
// are ignored. If no valid trace context is present, the returned context
// equals ctx.
func extractTraceContext(ctx context.Context, headers amqp.Table) context.Context {
	if len(headers) == 0 {
		return ctx
	}
	carrier := propagation.MapCarrier{}
	for k, v := range headers {
		if s, ok := v.(string); ok {
			carrier[k] = s
		}
	}
	return otel.GetTextMapPropagator().Extract(ctx, carrier)
}
