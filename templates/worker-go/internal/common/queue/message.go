package queue

import (
	"context"
	"encoding/json"
	"time"
)

type Message struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"`
	Payload   json.RawMessage   `json:"payload"`
	Timestamp time.Time         `json:"timestamp"`
	Metadata  map[string]string `json:"metadata,omitempty"`

	// Attempt is the retry-cycle count derived from the delivery's x-death
	// header (0 on first delivery), set by the consumer before dispatch. It is
	// transport metadata, not part of the frozen envelope contract, so it is
	// never (de)serialised — the tag keeps MarshalJSON/UnmarshalJSON clean.
	// Consumers use it to scope the idempotency key per attempt (so retries
	// aren't deduped) and processors use it for attempt-aware test hooks.
	Attempt int `json:"-"`
}

// MessageHandler processes one delivered message. The context carries the
// consumer span extracted from the delivery's trace headers; it is derived
// from context.Background() (not the shutdown context) so an in-flight
// message finishes even when shutdown has been signaled.
type MessageHandler func(ctx context.Context, msg *Message) error

func NewMessage(id, msgType string, payload interface{}) (*Message, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	return &Message{
		ID:        id,
		Type:      msgType,
		Payload:   payloadBytes,
		Timestamp: time.Now(),
		Metadata:  make(map[string]string),
	}, nil
}

func (m *Message) UnmarshalPayload(v interface{}) error {
	return json.Unmarshal(m.Payload, v)
}

func (m *Message) AddMetadata(key, value string) {
	if m.Metadata == nil {
		m.Metadata = make(map[string]string)
	}
	m.Metadata[key] = value
}

func (m *Message) GetMetadata(key string) string {
	if m.Metadata == nil {
		return ""
	}
	return m.Metadata[key]
}

func (m *Message) MarshalJSON() ([]byte, error) {
	type Alias Message
	return json.Marshal(&struct {
		*Alias
		Timestamp time.Time `json:"timestamp"`
	}{
		Alias:     (*Alias)(m),
		Timestamp: m.Timestamp,
	})
}
