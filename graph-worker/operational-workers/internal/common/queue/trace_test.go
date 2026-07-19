package queue

import (
	"context"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// setPropagator installs the W3C propagator globally (as tracing.Init does
// in production) and restores the previous one on cleanup.
func setPropagator(t *testing.T) {
	t.Helper()
	prev := otel.GetTextMapPropagator()
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	t.Cleanup(func() { otel.SetTextMapPropagator(prev) })
}

func TestExtractTraceContext_ParsesTraceparentHeader(t *testing.T) {
	setPropagator(t)

	const (
		wantTraceID = "0a0b0c0d0e0f10111213141516171819"
		wantSpanID  = "0102030405060708"
	)
	headers := amqp.Table{
		"traceparent":    "00-" + wantTraceID + "-" + wantSpanID + "-01",
		"trace_id":       wantTraceID, // envelope metadata copied to headers too
		"source":         "api-service",
		"x-not-a-string": int32(7), // non-string values must be tolerated
	}

	ctx := extractTraceContext(context.Background(), headers)

	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		t.Fatalf("expected a valid remote span context, got %+v", sc)
	}
	if !sc.IsRemote() {
		t.Errorf("expected extracted span context to be marked remote")
	}
	if got := sc.TraceID().String(); got != wantTraceID {
		t.Errorf("expected trace ID %s, got %s", wantTraceID, got)
	}
	if got := sc.SpanID().String(); got != wantSpanID {
		t.Errorf("expected parent span ID %s, got %s", wantSpanID, got)
	}
	if !sc.IsSampled() {
		t.Errorf("expected sampled flag to survive extraction")
	}
}

func TestExtractTraceContext_NoHeaders(t *testing.T) {
	setPropagator(t)

	for _, headers := range []amqp.Table{nil, {}, {"source": "api-service"}} {
		ctx := extractTraceContext(context.Background(), headers)
		if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
			t.Errorf("headers %v: expected no span context, got %+v", headers, sc)
		}
	}
}

// TestHandleDelivery span parentage is covered indirectly: the consumer
// starts its span from the context returned by extractTraceContext, so the
// linkage test here plus the api-service Submit test (traceparent in
// metadata → AMQP headers) pins the end-to-end contract.
