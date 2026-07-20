package example

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"example.com/worker/internal/common/queue"
)

var (
	// ErrMissingTarget and ErrInvalidEnvelope are the two structural
	// validation failures. Both are unretryable: BaseWorker wraps a Validate
	// error with queue.ErrUnretryable so the consumer routes it straight to
	// the DLQ instead of cycling it through the retry tiers (ADR-008.1).
	ErrMissingTarget   = errors.New("payload.target is required")
	ErrInvalidEnvelope = errors.New("envelope type must be 'example.task'")
)

// Payload is the typed inner payload of an example.task envelope. This is the
// seam you replace when adapting the template: swap these fields for your
// domain's payload and update Validate accordingly. Keep the shape — parse the
// inner object, enforce structurally-required fields, tolerate unknown fields.
type Payload struct {
	Target string                 `json:"target"`
	Note   string                 `json:"note,omitempty"`
	Params map[string]interface{} `json:"params,omitempty"`
}

// Task is the parsed envelope + typed payload for an example.task message.
type Task struct {
	ID        string
	Type      string
	Timestamp time.Time
	Payload   Payload
	Metadata  map[string]string
}

// NewTask decodes msg.Payload (the envelope's inner "payload" object, already
// extracted by the queue layer) into Payload. It does NOT re-unmarshal the
// whole envelope — msg.Payload is just the inner object.
func NewTask(msg *queue.Message) (*Task, error) {
	var p Payload
	if err := json.Unmarshal(msg.Payload, &p); err != nil {
		return nil, err
	}
	return &Task{
		ID:        msg.ID,
		Type:      msg.Type,
		Timestamp: msg.Timestamp,
		Payload:   p,
		Metadata:  msg.Metadata,
	}, nil
}

// Validate checks the envelope type (when present) and the structurally
// required payload fields. Unknown fields are tolerated (forward compatible);
// only enforce what your processor genuinely needs.
func (t *Task) Validate() error {
	if t.Type != "" && t.Type != "example.task" {
		return ErrInvalidEnvelope
	}
	if strings.TrimSpace(t.Payload.Target) == "" {
		return ErrMissingTarget
	}
	return nil
}
