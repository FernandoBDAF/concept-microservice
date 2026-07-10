package queue

import (
	"encoding/json"
	"time"
)

type Message struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"`
	Payload   json.RawMessage   `json:"payload"`
	Timestamp time.Time         `json:"timestamp"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

type MessageHandler func(msg *Message) error

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
