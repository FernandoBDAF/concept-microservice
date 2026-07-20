package task

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

func mustUUID(t *testing.T, s string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(s)
	if err != nil {
		t.Fatalf("bad uuid fixture %q: %v", s, err)
	}
	return id
}

type mockDocUpdater struct {
	calls  int
	lastID uuid.UUID
	status string
	errMsg *string
	err    error
}

func (m *mockDocUpdater) ApplyTaskResult(ctx context.Context, id uuid.UUID, status string, errorMsg *string) error {
	m.calls++
	m.lastID = id
	m.status = status
	m.errMsg = errorMsg
	return m.err
}

func resultEnvelope(t *testing.T, payload ResultPayload) *Message {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return &Message{
		ID:        uuid.New().String(),
		Type:      TypeTaskResult,
		Timestamp: time.Now().UTC(),
		Payload:   body,
		Metadata:  map[string]string{"source": "graphrag-service"},
	}
}

func TestResultHandler_CompletedDocumentResult(t *testing.T) {
	docID := mustUUID(t, "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
	docs := &mockDocUpdater{}
	h := NewResultHandler(docs, zap.NewNop())

	msg := resultEnvelope(t, ResultPayload{
		TaskID:     "task-1",
		TaskType:   "document.process",
		Status:     ResultCompleted,
		EnvelopeID: "env-1",
		DocumentID: docID.String(),
	})

	if err := h.Handle(context.Background(), msg); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if docs.calls != 1 {
		t.Fatalf("expected one ApplyTaskResult call, got %d", docs.calls)
	}
	if docs.lastID != docID {
		t.Errorf("expected document id %s, got %s", docID, docs.lastID)
	}
	if docs.status != ResultCompleted {
		t.Errorf("expected status completed, got %q", docs.status)
	}
	if docs.errMsg != nil {
		t.Errorf("expected nil error message for completed result")
	}
}

func TestResultHandler_FailedDocumentResultCarriesError(t *testing.T) {
	docID := mustUUID(t, "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
	docs := &mockDocUpdater{}
	h := NewResultHandler(docs, zap.NewNop())

	msg := resultEnvelope(t, ResultPayload{
		TaskID:     "task-2",
		TaskType:   "document.process",
		Status:     ResultFailed,
		Error:      "extraction blew up",
		EnvelopeID: "env-2",
		DocumentID: docID.String(),
	})

	if err := h.Handle(context.Background(), msg); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if docs.status != ResultFailed {
		t.Errorf("expected status failed, got %q", docs.status)
	}
	if docs.errMsg == nil || *docs.errMsg != "extraction blew up" {
		t.Errorf("expected error message to be forwarded, got %v", docs.errMsg)
	}
}

func TestResultHandler_NonDocumentResultIsAckedWithoutUpdate(t *testing.T) {
	docs := &mockDocUpdater{}
	h := NewResultHandler(docs, zap.NewNop())

	msg := resultEnvelope(t, ResultPayload{
		TaskID:     "task-3",
		TaskType:   "email.send",
		Status:     ResultCompleted,
		EnvelopeID: "env-3",
	})

	if err := h.Handle(context.Background(), msg); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if docs.calls != 0 {
		t.Errorf("expected no document update for non-document result")
	}
}

func TestResultHandler_PoisonIsDroppedNotErrored(t *testing.T) {
	docs := &mockDocUpdater{}
	h := NewResultHandler(docs, zap.NewNop())

	cases := []*Message{
		// wrong envelope type
		{ID: "x", Type: "document.process", Payload: json.RawMessage(`{}`)},
		// unparseable payload
		{ID: "y", Type: TypeTaskResult, Payload: json.RawMessage(`{"task_id":42}`)},
		// unknown status
		resultEnvelope(t, ResultPayload{TaskID: "t", Status: "midway", DocumentID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"}),
		// invalid document id
		resultEnvelope(t, ResultPayload{TaskID: "t", Status: ResultCompleted, DocumentID: "not-a-uuid"}),
	}
	for i, msg := range cases {
		if err := h.Handle(context.Background(), msg); err != nil {
			t.Errorf("case %d: poison must be dropped (nil error, ack), got %v", i, err)
		}
	}
	if docs.calls != 0 {
		t.Errorf("expected no document updates from poison, got %d", docs.calls)
	}
}

func TestResultHandler_TransientErrorPropagatesForRedelivery(t *testing.T) {
	wantErr := errors.New("db down")
	docs := &mockDocUpdater{err: wantErr}
	h := NewResultHandler(docs, zap.NewNop())

	msg := resultEnvelope(t, ResultPayload{
		TaskID:     "task-4",
		TaskType:   "document.process",
		Status:     ResultCompleted,
		DocumentID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
	})

	if err := h.Handle(context.Background(), msg); !errors.Is(err, wantErr) {
		t.Fatalf("expected transient error to propagate (nack+requeue), got %v", err)
	}
}

func TestResultHandler_NilUpdaterDropsDocumentResults(t *testing.T) {
	h := NewResultHandler(nil, zap.NewNop())
	msg := resultEnvelope(t, ResultPayload{
		TaskID:     "task-5",
		Status:     ResultCompleted,
		DocumentID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
	})
	if err := h.Handle(context.Background(), msg); err != nil {
		t.Fatalf("expected nil error when document service is disabled, got %v", err)
	}
}
