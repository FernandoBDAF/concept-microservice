package email

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/fernandobarroso/microservices/operational-workers/internal/common/queue"
)

func TestEmailProcessor_Process_HappyPath(t *testing.T) {
	payload, err := json.Marshal(EmailPayload{
		EmailType:  "welcome",
		Recipient:  "user@example.com",
		TemplateID: "welcome-template",
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	msg := &queue.Message{
		ID:      "11111111-1111-1111-1111-111111111111",
		Type:    "email.send",
		Payload: payload,
	}

	p := NewEmailProcessor()

	if err := p.Validate(msg); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}

	if err := p.Process(context.Background(), msg); err != nil {
		t.Fatalf("Process() error = %v, want nil", err)
	}
}

func TestEmailProcessor_Process_PoisonMessageErrors(t *testing.T) {
	msg := &queue.Message{
		ID:      "id-2",
		Type:    "email.send",
		Payload: json.RawMessage(`{"email_type":"welcome"}`), // missing recipient
	}

	p := NewEmailProcessor()

	if err := p.Process(context.Background(), msg); err == nil {
		t.Error("expected Process to fail for a payload missing the required recipient field")
	}
}

// TestEmailProcessor_FailFirstNAttempts exercises the EXP-40 test hook: with
// FAIL_FIRST_N_ATTEMPTS=2 the simulated send fails on attempts 0 and 1 (which
// the consumer routes through the 5s/30s retry tiers) then succeeds on attempt
// 2, standing in for a recovered dependency without Chaos Mesh.
func TestEmailProcessor_FailFirstNAttempts(t *testing.T) {
	t.Setenv("FAIL_FIRST_N_ATTEMPTS", "2")

	payload, err := json.Marshal(EmailPayload{EmailType: "welcome", Recipient: "user@example.com"})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	base := queue.Message{ID: "id-flaky", Type: "email.send", Payload: payload}

	p := NewEmailProcessor()

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

func TestEmailProcessor_FailHookInertWhenUnset(t *testing.T) {
	t.Setenv("FAIL_FIRST_N_ATTEMPTS", "") // explicitly disabled

	payload, err := json.Marshal(EmailPayload{EmailType: "welcome", Recipient: "user@example.com"})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	msg := &queue.Message{ID: "id-1", Type: "email.send", Payload: payload}

	p := NewEmailProcessor()
	if err := p.Process(context.Background(), msg); err != nil {
		t.Errorf("hook must be inert when unset, got %v", err)
	}
}
