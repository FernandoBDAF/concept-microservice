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

// mockEnqueuer captures what Submit stores in the outbox (ADR-008.3: the
// serialized envelope IS what the relay later publishes verbatim).
type mockEnqueuer struct {
	lastRoutingKey string
	lastEnvelope   []byte
	err            error
	calls          int
}

func (m *mockEnqueuer) Enqueue(ctx context.Context, routingKey string, envelope []byte) error {
	m.calls++
	m.lastRoutingKey = routingKey
	m.lastEnvelope = envelope
	return m.err
}

func (m *mockEnqueuer) lastMessage(t *testing.T) *Message {
	t.Helper()
	var msg Message
	if err := json.Unmarshal(m.lastEnvelope, &msg); err != nil {
		t.Fatalf("failed to decode enqueued envelope: %v", err)
	}
	return &msg
}

func TestService_Submit_BuildsEnvelopeAndEnqueues(t *testing.T) {
	enq := &mockEnqueuer{}
	svc := NewService(enq)

	payload := map[string]interface{}{"profile_id": "abc-123"}
	metadata := map[string]string{"source": "api-service"}

	taskID, err := svc.Submit(context.Background(), "profile.task", "profile.task", payload, metadata)
	if err != nil {
		t.Fatalf("Submit returned error: %v", err)
	}
	if taskID == "" {
		t.Fatalf("expected non-empty task ID")
	}
	if enq.calls != 1 {
		t.Fatalf("expected exactly one enqueue call, got %d", enq.calls)
	}
	if enq.lastRoutingKey != "profile.task" {
		t.Errorf("expected routing key 'profile.task', got %q", enq.lastRoutingKey)
	}

	msg := enq.lastMessage(t)
	if msg.ID != taskID {
		t.Errorf("expected envelope ID to match returned task ID")
	}
	if msg.Type != "profile.task" {
		t.Errorf("expected envelope type 'profile.task', got %q", msg.Type)
	}
	if msg.Timestamp.IsZero() {
		t.Errorf("expected envelope timestamp to be set")
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(msg.Payload, &decoded); err != nil {
		t.Fatalf("failed to decode enqueued payload: %v", err)
	}
	if decoded["profile_id"] != "abc-123" {
		t.Errorf("expected payload to round-trip profile_id, got %+v", decoded)
	}
}

func TestService_Submit_UnknownRoutingKeyFailsFast(t *testing.T) {
	enq := &mockEnqueuer{}
	svc := NewService(enq)

	// ADR-008.6: no default-tasks fallback — unknown keys are a bug.
	_, err := svc.Submit(context.Background(), "mystery.task", "mystery.task", map[string]interface{}{}, nil)
	if !errors.Is(err, ErrUnknownRoutingKey) {
		t.Fatalf("expected ErrUnknownRoutingKey, got %v", err)
	}
	if enq.calls != 0 {
		t.Errorf("expected no enqueue for unknown routing key, got %d calls", enq.calls)
	}
}

// TestEnvelopeEquivalence_SubmitVsDocumentPath proves the outbox migration
// kept ONE envelope construction point: the envelope Submit enqueues and
// the envelope the document-upload path stores are shape-identical for the
// same inputs (same fields set, same payload, same metadata semantics) —
// both flow through BuildEnvelope.
func TestEnvelopeEquivalence_SubmitVsDocumentPath(t *testing.T) {
	payload := map[string]interface{}{
		"document_id":    "d1",
		"profile_id":     "p1",
		"user_id":        "u1",
		"storage_path":   "p1/2026/07/19/d1.pdf",
		"storage_bucket": "documents-raw",
		"file_type":      "pdf",
	}
	metadata := map[string]string{"source": "api-service", "document_id": "d1", "user_id": "u1"}

	// Path 1: Service.Submit (task endpoints).
	enq := &mockEnqueuer{}
	if _, err := NewService(enq).Submit(context.Background(), "document.process", "document.process", payload, copyMap(metadata)); err != nil {
		t.Fatalf("Submit failed: %v", err)
	}
	submitMsg := enq.lastMessage(t)

	// Path 2: the helper the document-upload outbox path serializes.
	helperMsg, err := BuildEnvelope(context.Background(), "document.process", "document.process", payload, copyMap(metadata))
	if err != nil {
		t.Fatalf("BuildEnvelope failed: %v", err)
	}

	if submitMsg.Type != helperMsg.Type {
		t.Errorf("type mismatch: %q vs %q", submitMsg.Type, helperMsg.Type)
	}
	if string(submitMsg.Payload) != string(helperMsg.Payload) {
		t.Errorf("payload mismatch:\n%s\nvs\n%s", submitMsg.Payload, helperMsg.Payload)
	}
	if submitMsg.Priority != helperMsg.Priority {
		t.Errorf("priority mismatch")
	}
	// Metadata: identical key sets; identical values except trace_id (a
	// fresh correlation-derived value per envelope on non-traced paths).
	if len(submitMsg.Metadata) != len(helperMsg.Metadata) {
		t.Fatalf("metadata key sets differ: %v vs %v", submitMsg.Metadata, helperMsg.Metadata)
	}
	for k, v := range submitMsg.Metadata {
		hv, ok := helperMsg.Metadata[k]
		if !ok {
			t.Errorf("metadata key %q missing from helper envelope", k)
			continue
		}
		if k == "trace_id" {
			if v == "" || hv == "" {
				t.Errorf("expected non-empty trace_id on both paths")
			}
			continue
		}
		if v != hv {
			t.Errorf("metadata[%q] mismatch: %q vs %q", k, v, hv)
		}
	}
	// Per-envelope identity fields are fresh on both paths.
	if submitMsg.ID == helperMsg.ID {
		t.Errorf("expected distinct envelope IDs per build")
	}
	if submitMsg.ID == "" || helperMsg.ID == "" || submitMsg.CorrelationID == "" || helperMsg.CorrelationID == "" {
		t.Errorf("expected non-empty id/correlation_id on both paths")
	}
	if submitMsg.Timestamp.IsZero() || helperMsg.Timestamp.IsZero() {
		t.Errorf("expected timestamps on both paths")
	}
}

func copyMap(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
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

	enq := &mockEnqueuer{}
	svc := NewService(enq)

	// Caller-supplied trace_id must be overridden by the real trace ID
	// when a span is active.
	metadata := map[string]string{"trace_id": "caller-supplied"}
	if _, err := svc.Submit(ctx, "email.send", "email.send", map[string]interface{}{}, metadata); err != nil {
		t.Fatalf("Submit returned error: %v", err)
	}

	got := enq.lastMessage(t).Metadata
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

	enq := &mockEnqueuer{}
	svc := NewService(enq)

	// Caller-supplied trace_id is honored when there is no active span.
	metadata := map[string]string{"trace_id": "caller-supplied"}
	if _, err := svc.Submit(context.Background(), "email.send", "email.send", map[string]interface{}{}, metadata); err != nil {
		t.Fatalf("Submit returned error: %v", err)
	}
	msg := enq.lastMessage(t)
	if got := msg.Metadata["trace_id"]; got != "caller-supplied" {
		t.Errorf("expected caller-supplied trace_id to be honored, got %q", got)
	}
	if _, ok := msg.Metadata["traceparent"]; ok {
		t.Errorf("expected no traceparent without an active span")
	}

	// Without a caller-supplied trace_id, it falls back to the correlation ID.
	enq2 := &mockEnqueuer{}
	svc2 := NewService(enq2)
	if _, err := svc2.Submit(context.Background(), "email.send", "email.send", map[string]interface{}{}, nil); err != nil {
		t.Fatalf("Submit returned error: %v", err)
	}
	msg2 := enq2.lastMessage(t)
	if msg2.Metadata["trace_id"] == "" {
		t.Errorf("expected fallback trace_id to be set")
	}
	if msg2.Metadata["trace_id"] != msg2.CorrelationID {
		t.Errorf("expected fallback trace_id %q to equal correlation ID %q", msg2.Metadata["trace_id"], msg2.CorrelationID)
	}
}

func TestService_Submit_PropagatesEnqueueError(t *testing.T) {
	wantErr := errors.New("outbox unavailable")
	enq := &mockEnqueuer{err: wantErr}
	svc := NewService(enq)

	taskID, err := svc.Submit(context.Background(), "email.send", "email.send", map[string]interface{}{}, nil)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected enqueue error to propagate, got %v", err)
	}
	if taskID != "" {
		t.Errorf("expected empty task ID on failure, got %q", taskID)
	}
}

func TestDocumentTaskBuilder_BuildsContractEnvelope(t *testing.T) {
	b := NewDocumentTaskBuilder()

	docID := mustUUID(t, "6ba7b810-9dad-11d1-80b4-00c04fd430c8")
	profileID := mustUUID(t, "6ba7b811-9dad-11d1-80b4-00c04fd430c8")
	userID := mustUUID(t, "6ba7b812-9dad-11d1-80b4-00c04fd430c8")

	taskID, rk, envelope, err := b.BuildDocumentTask(context.Background(), docID, profileID, userID, "path/doc.pdf", "documents-raw", "pdf")
	if err != nil {
		t.Fatalf("BuildDocumentTask failed: %v", err)
	}
	if rk != "document.process" {
		t.Errorf("expected routing key document.process, got %q", rk)
	}

	var msg Message
	if err := json.Unmarshal(envelope, &msg); err != nil {
		t.Fatalf("envelope is not a valid Message: %v", err)
	}
	if msg.ID != taskID {
		t.Errorf("returned taskID must be the envelope id")
	}
	if msg.Type != "document.process" {
		t.Errorf("expected type document.process, got %q", msg.Type)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		t.Fatalf("bad payload: %v", err)
	}
	// graphrag-service contract fields (MESSAGE_FORMAT.md).
	for _, field := range []string{"document_id", "storage_path", "storage_bucket", "file_type", "user_id", "profile_id"} {
		if _, ok := payload[field]; !ok {
			t.Errorf("payload missing contract field %q", field)
		}
	}
	if msg.Metadata["source"] != "api-service" {
		t.Errorf("expected metadata source api-service")
	}
}
