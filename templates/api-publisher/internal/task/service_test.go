package task

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

// mockEnqueuer captures what Submit stores in the outbox: the serialized
// envelope IS what the relay later publishes verbatim.
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

	payload := map[string]interface{}{"resource_id": "abc-123"}
	metadata := map[string]string{"source": "api-publisher"}

	taskID, err := svc.Submit(context.Background(), ExampleTaskRoutingKey, ExampleTaskRoutingKey, payload, metadata)
	if err != nil {
		t.Fatalf("Submit returned error: %v", err)
	}
	if taskID == "" {
		t.Fatalf("expected non-empty task ID")
	}
	if enq.calls != 1 {
		t.Fatalf("expected exactly one enqueue call, got %d", enq.calls)
	}
	if enq.lastRoutingKey != ExampleTaskRoutingKey {
		t.Errorf("expected routing key %q, got %q", ExampleTaskRoutingKey, enq.lastRoutingKey)
	}

	msg := enq.lastMessage(t)
	if msg.ID != taskID {
		t.Errorf("expected envelope ID to match returned task ID")
	}
	if msg.Type != ExampleTaskRoutingKey {
		t.Errorf("expected envelope type %q, got %q", ExampleTaskRoutingKey, msg.Type)
	}
	if msg.Timestamp.IsZero() {
		t.Errorf("expected envelope timestamp to be set")
	}
	if msg.Metadata["source"] != "api-publisher" {
		t.Errorf("expected metadata source api-publisher, got %q", msg.Metadata["source"])
	}
	if msg.Metadata["trace_id"] == "" {
		t.Errorf("expected a trace_id to be set")
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(msg.Payload, &decoded); err != nil {
		t.Fatalf("failed to decode enqueued payload: %v", err)
	}
	if decoded["resource_id"] != "abc-123" {
		t.Errorf("expected payload to round-trip resource_id, got %+v", decoded)
	}
}

func TestService_SubmitExample_TypedHelper(t *testing.T) {
	enq := &mockEnqueuer{}
	svc := NewService(enq)

	taskID, err := svc.SubmitExample(context.Background(), ExamplePayload{ResourceID: "r1", Note: "hi"}, nil)
	if err != nil {
		t.Fatalf("SubmitExample returned error: %v", err)
	}
	if taskID == "" || enq.lastRoutingKey != ExampleTaskRoutingKey {
		t.Fatalf("typed helper did not submit on the example routing key: id=%q rk=%q", taskID, enq.lastRoutingKey)
	}
	var payload ExamplePayload
	if err := json.Unmarshal(enq.lastMessage(t).Payload, &payload); err != nil {
		t.Fatalf("payload did not round-trip: %v", err)
	}
	if payload.ResourceID != "r1" || payload.Note != "hi" {
		t.Errorf("typed payload mismatch: %+v", payload)
	}
}

func TestService_Submit_UnknownRoutingKeyFailsFast(t *testing.T) {
	enq := &mockEnqueuer{}
	svc := NewService(enq)

	// No fallback — an unknown key is a bug, not a parking lot.
	_, err := svc.Submit(context.Background(), "mystery.task", "mystery.task", map[string]interface{}{}, nil)
	if !errors.Is(err, ErrUnknownRoutingKey) {
		t.Fatalf("expected ErrUnknownRoutingKey, got %v", err)
	}
	if enq.calls != 0 {
		t.Errorf("expected no enqueue for unknown routing key, got %d calls", enq.calls)
	}
}

func TestService_Submit_PropagatesEnqueueError(t *testing.T) {
	wantErr := errors.New("outbox unavailable")
	enq := &mockEnqueuer{err: wantErr}
	svc := NewService(enq)

	taskID, err := svc.Submit(context.Background(), ExampleTaskRoutingKey, ExampleTaskRoutingKey, map[string]interface{}{}, nil)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected enqueue error to propagate, got %v", err)
	}
	if taskID != "" {
		t.Errorf("expected empty task ID on failure, got %q", taskID)
	}
}
