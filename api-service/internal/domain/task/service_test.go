package task

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type mockPublisher struct {
	lastRoutingKey string
	lastMessage    *Message
	err            error
	calls          int
}

func (m *mockPublisher) PublishWithRoutingKey(routingKey string, msg *Message) error {
	m.calls++
	m.lastRoutingKey = routingKey
	m.lastMessage = msg
	return m.err
}

func TestService_Submit_BuildsEnvelopeAndPublishes(t *testing.T) {
	pub := &mockPublisher{}
	svc := NewService(pub)

	payload := map[string]interface{}{"profile_id": "abc-123"}
	metadata := map[string]string{"source": "api-service"}

	taskID, err := svc.Submit(context.Background(), "profile.task", "profile.task", payload, metadata)
	if err != nil {
		t.Fatalf("Submit returned error: %v", err)
	}
	if taskID == "" {
		t.Fatalf("expected non-empty task ID")
	}
	if pub.calls != 1 {
		t.Fatalf("expected exactly one publish call, got %d", pub.calls)
	}
	if pub.lastRoutingKey != "profile.task" {
		t.Errorf("expected routing key 'profile.task', got %q", pub.lastRoutingKey)
	}
	if pub.lastMessage.ID != taskID {
		t.Errorf("expected message ID to match returned task ID")
	}
	if pub.lastMessage.Type != "profile.task" {
		t.Errorf("expected message type 'profile.task', got %q", pub.lastMessage.Type)
	}
	if pub.lastMessage.Timestamp.IsZero() {
		t.Errorf("expected message timestamp to be set")
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(pub.lastMessage.Payload, &decoded); err != nil {
		t.Fatalf("failed to decode published payload: %v", err)
	}
	if decoded["profile_id"] != "abc-123" {
		t.Errorf("expected payload to round-trip profile_id, got %+v", decoded)
	}
}

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

func TestService_Submit_WithActiveSpan_SetsTraceIDAndTraceparent(t *testing.T) {
	setPropagator(t)

	traceID := trace.TraceID{0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19}
	spanID := trace.SpanID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
		Remote:     true,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)

	pub := &mockPublisher{}
	svc := NewService(pub)

	// Caller-supplied trace_id must be overridden by the real trace ID
	// when a span is active.
	metadata := map[string]string{"trace_id": "caller-supplied"}
	if _, err := svc.Submit(ctx, "email.send", "email.send", map[string]interface{}{}, metadata); err != nil {
		t.Fatalf("Submit returned error: %v", err)
	}

	got := pub.lastMessage.Metadata
	if got["trace_id"] != traceID.String() {
		t.Errorf("expected metadata trace_id %q, got %q", traceID.String(), got["trace_id"])
	}
	if len(got["trace_id"]) != 32 {
		t.Errorf("expected 32-char hex trace_id, got %q", got["trace_id"])
	}
	tp, ok := got["traceparent"]
	if !ok {
		t.Fatalf("expected metadata to contain traceparent, got %+v", got)
	}
	// traceparent format: 00-<32 hex trace id>-<16 hex span id>-<2 hex flags>
	if len(tp) != 55 || tp[3:35] != traceID.String() {
		t.Errorf("expected traceparent carrying trace ID %s, got %q", traceID.String(), tp)
	}
	if got["source"] != "api-service" {
		t.Errorf("expected metadata source api-service, got %q", got["source"])
	}
}

func TestService_Submit_WithoutSpan_KeepsLegacyTraceID(t *testing.T) {
	setPropagator(t)

	pub := &mockPublisher{}
	svc := NewService(pub)

	// Caller-supplied trace_id is honored when there is no active span.
	metadata := map[string]string{"trace_id": "caller-supplied"}
	if _, err := svc.Submit(context.Background(), "email.send", "email.send", map[string]interface{}{}, metadata); err != nil {
		t.Fatalf("Submit returned error: %v", err)
	}
	if got := pub.lastMessage.Metadata["trace_id"]; got != "caller-supplied" {
		t.Errorf("expected caller-supplied trace_id to be honored, got %q", got)
	}
	if _, ok := pub.lastMessage.Metadata["traceparent"]; ok {
		t.Errorf("expected no traceparent without an active span")
	}

	// Without a caller-supplied trace_id, it falls back to the correlation ID.
	pub2 := &mockPublisher{}
	svc2 := NewService(pub2)
	if _, err := svc2.Submit(context.Background(), "email.send", "email.send", map[string]interface{}{}, nil); err != nil {
		t.Fatalf("Submit returned error: %v", err)
	}
	md := pub2.lastMessage.Metadata
	if md["trace_id"] == "" {
		t.Errorf("expected fallback trace_id to be set")
	}
	if md["trace_id"] != pub2.lastMessage.CorrelationID {
		t.Errorf("expected fallback trace_id %q to equal correlation ID %q", md["trace_id"], pub2.lastMessage.CorrelationID)
	}
}

func TestService_Submit_PropagatesPublishError(t *testing.T) {
	wantErr := errors.New("broker unavailable")
	pub := &mockPublisher{err: wantErr}
	svc := NewService(pub)

	taskID, err := svc.Submit(context.Background(), "email.send", "email.send", map[string]interface{}{}, nil)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected publish error to propagate, got %v", err)
	}
	if taskID != "" {
		t.Errorf("expected empty task ID on failure, got %q", taskID)
	}
}
