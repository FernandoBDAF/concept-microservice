package example

import (
	"context"
	"encoding/json"
	"testing"

	"example.com/worker/internal/common/queue"
)

func TestProcessor_Process_HappyPath(t *testing.T) {
	payload, err := json.Marshal(Payload{Target: "widget-1", Note: "go"})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	msg := &queue.Message{
		ID:      "11111111-1111-1111-1111-111111111111",
		Type:    "example.task",
		Payload: payload,
	}

	p := NewProcessor()

	if err := p.Validate(msg); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
	if err := p.Process(context.Background(), msg); err != nil {
		t.Fatalf("Process() error = %v, want nil", err)
	}
}

func TestProcessor_Process_PoisonMessageErrors(t *testing.T) {
	msg := &queue.Message{
		ID:      "id-2",
		Type:    "example.task",
		Payload: json.RawMessage(`{"note":"no target"}`), // missing target
	}

	p := NewProcessor()
	if err := p.Process(context.Background(), msg); err == nil {
		t.Error("expected Process to fail for a payload missing the required target field")
	}
}

// TestProcessor_FailFirstNAttempts exercises the EXP-40 test hook: with
// FAIL_FIRST_N_ATTEMPTS=2 the simulated work fails on attempts 0 and 1 (which
// the consumer routes through the 5s/30s retry tiers) then succeeds on attempt
// 2, standing in for a recovered dependency without a fault injector.
func TestProcessor_FailFirstNAttempts(t *testing.T) {
	t.Setenv("FAIL_FIRST_N_ATTEMPTS", "2")

	payload, err := json.Marshal(Payload{Target: "widget-flaky"})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	base := queue.Message{ID: "id-flaky", Type: "example.task", Payload: payload}

	p := NewProcessor()

	for attempt := 0; attempt < 2; attempt++ {
		msg := base
		msg.Attempt = attempt
		if err := p.Process(context.Background(), &msg); err == nil {
			t.Errorf("attempt %d: expected transient failure, got nil", attempt)
		}
	}

	recovered := base
	recovered.Attempt = 2
	if err := p.Process(context.Background(), &recovered); err != nil {
		t.Errorf("attempt 2: expected success after recovery, got %v", err)
	}
}

func TestProcessor_FailHookInertWhenUnset(t *testing.T) {
	t.Setenv("FAIL_FIRST_N_ATTEMPTS", "") // explicitly disabled

	payload, err := json.Marshal(Payload{Target: "widget-1"})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	msg := &queue.Message{ID: "id-1", Type: "example.task", Payload: payload}

	p := NewProcessor()
	if err := p.Process(context.Background(), msg); err != nil {
		t.Errorf("hook must be inert when unset, got %v", err)
	}
}
