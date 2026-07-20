package rabbitmq

import (
	"context"
	"encoding/json"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/fernandobarroso/microservices/api-service/internal/domain/task"
)

// These tests pin the relay-side trace-propagation contract: the pre-outbox
// direct publisher mirrored envelope metadata into AMQP headers, and worker
// consumers continue traces FROM headers (operational-workers
// queue/trace.go extractTraceContext: traceparent/tracestate/baggage).
// headersFromEnvelope is the exact derivation PublishRaw attaches to every
// relay-published message.

func TestHeadersFromEnvelope_RestoresTraceHeadersAndMessageID(t *testing.T) {
	body := []byte(`{
		"id": "env-123",
		"type": "email.send",
		"payload": {},
		"metadata": {
			"source": "api-service",
			"trace_id": "0a0b0c0d0e0f10111213141516171819",
			"traceparent": "00-0a0b0c0d0e0f10111213141516171819-0102030405060708-01",
			"tracestate": "vendor=foo",
			"baggage": "userId=u1"
		}
	}`)

	headers, messageID := headersFromEnvelope(body)
	if messageID != "env-123" {
		t.Errorf("expected MessageId env-123, got %q", messageID)
	}
	if headers == nil {
		t.Fatalf("expected headers to be restored")
	}
	if got := headers["traceparent"]; got != "00-0a0b0c0d0e0f10111213141516171819-0102030405060708-01" {
		t.Errorf("traceparent header mismatch: %v", got)
	}
	if got := headers["tracestate"]; got != "vendor=foo" {
		t.Errorf("tracestate header mismatch: %v", got)
	}
	if got := headers["baggage"]; got != "userId=u1" {
		t.Errorf("baggage header mismatch: %v", got)
	}
}

func TestHeadersFromEnvelope_ToleratesAbsenceAndMalformed(t *testing.T) {
	// No trace metadata: publish proceeds with nil headers, id preserved.
	headers, messageID := headersFromEnvelope([]byte(`{"id":"env-1","metadata":{"source":"api-service"}}`))
	if headers != nil {
		t.Errorf("expected nil headers without trace metadata, got %v", headers)
	}
	if messageID != "env-1" {
		t.Errorf("expected MessageId env-1, got %q", messageID)
	}

	// No metadata object at all.
	headers, messageID = headersFromEnvelope([]byte(`{"id":"env-2"}`))
	if headers != nil || messageID != "env-2" {
		t.Errorf("expected nil headers / env-2, got %v / %q", headers, messageID)
	}

	// Malformed body must not fail the publish path: nil/empty derivation.
	headers, messageID = headersFromEnvelope([]byte(`not json`))
	if headers != nil || messageID != "" {
		t.Errorf("expected nil headers and empty id for malformed body, got %v / %q", headers, messageID)
	}
}

// TestRelayPublishedEnvelope_CarriesInjectedTraceparent closes the loop:
// an envelope built on a traced request (task.BuildEnvelope injects
// traceparent into metadata) and stored in the outbox yields, at relay
// publish time, an AMQP header set the workers' extractTraceContext can
// continue the SAME trace from.
func TestRelayPublishedEnvelope_CarriesInjectedTraceparent(t *testing.T) {
	prev := otel.GetTextMapPropagator()
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	t.Cleanup(func() { otel.SetTextMapPropagator(prev) })

	traceID := trace.TraceID{0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19}
	spanID := trace.SpanID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
		Remote:     true,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)

	msg, err := task.BuildEnvelope(ctx, "email.send", "email.send", map[string]interface{}{}, nil)
	if err != nil {
		t.Fatalf("BuildEnvelope: %v", err)
	}
	stored, err := json.Marshal(msg) // exactly what the outbox stores
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	headers, messageID := headersFromEnvelope(stored)
	if messageID != msg.ID {
		t.Errorf("expected MessageId to equal envelope id %q, got %q", msg.ID, messageID)
	}
	if headers == nil {
		t.Fatalf("expected traceparent header on relay publish, got none")
	}
	tp, ok := headers["traceparent"].(string)
	if !ok || tp == "" {
		t.Fatalf("expected string traceparent header, got %v", headers["traceparent"])
	}
	if len(tp) != 55 || tp[3:35] != traceID.String() {
		t.Errorf("traceparent %q does not carry trace ID %s", tp, traceID.String())
	}

	// And the workers' extraction sees the same trace: simulate
	// extractTraceContext (propagator over string headers).
	carrier := propagation.MapCarrier{}
	for k, v := range headers {
		if s, ok := v.(string); ok {
			carrier[k] = s
		}
	}
	extracted := trace.SpanContextFromContext(
		otel.GetTextMapPropagator().Extract(context.Background(), carrier),
	)
	if !extracted.IsValid() || extracted.TraceID() != traceID {
		t.Errorf("worker-side extraction lost the trace: got %v", extracted.TraceID())
	}
}
