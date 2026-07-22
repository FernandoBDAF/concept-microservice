package example

import (
	"encoding/json"
	"testing"

	"example.com/worker/internal/common/queue"
)

func TestNewTask_HappyPath(t *testing.T) {
	envelope := []byte(`{
		"id": "11111111-1111-1111-1111-111111111111",
		"type": "example.task",
		"timestamp": "2026-01-30T12:34:56Z",
		"payload": {
			"target": "widget-42",
			"note": "hello",
			"params": {"count": 3}
		}
	}`)

	var msg queue.Message
	if err := json.Unmarshal(envelope, &msg); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}

	task, err := NewTask(&msg)
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	if task.Payload.Target != "widget-42" {
		t.Errorf("Target = %q, want widget-42", task.Payload.Target)
	}
	if task.Payload.Params["count"] != float64(3) {
		t.Errorf("Params[count] = %v, want 3", task.Payload.Params["count"])
	}
	if err := task.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestTask_Validate_MissingTarget(t *testing.T) {
	task := &Task{Type: "example.task", Payload: Payload{Note: "no target"}}
	if err := task.Validate(); err == nil {
		t.Error("expected error for missing target")
	}
}

func TestTask_Validate_WrongEnvelopeType(t *testing.T) {
	task := &Task{Type: "other.task", Payload: Payload{Target: "x"}}
	if err := task.Validate(); err == nil {
		t.Error("expected error for mismatched envelope type")
	}
}

func TestTask_Validate_EmptyEnvelopeTypeTolerated(t *testing.T) {
	// An absent envelope type is tolerated (the payload target is what's
	// structurally required).
	task := &Task{Payload: Payload{Target: "x"}}
	if err := task.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil for empty envelope type", err)
	}
}
