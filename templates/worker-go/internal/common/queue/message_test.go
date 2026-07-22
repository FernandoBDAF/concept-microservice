package queue

import (
	"encoding/json"
	"testing"
)

func TestMessage_UnmarshalEnvelope_RequiredFieldsOnly(t *testing.T) {
	raw := []byte(`{
		"id": "11111111-1111-1111-1111-111111111111",
		"type": "email.send",
		"timestamp": "2026-01-30T12:34:56Z",
		"payload": {"recipient": "user@example.com", "email_type": "welcome"}
	}`)

	var msg Message
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatalf("unmarshal envelope without metadata: %v", err)
	}

	if msg.ID != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("ID = %q", msg.ID)
	}
	if msg.Type != "email.send" {
		t.Errorf("Type = %q", msg.Type)
	}
	if msg.Metadata != nil {
		t.Errorf("Metadata should be nil when omitted (optional per contract), got %#v", msg.Metadata)
	}

	var payload struct {
		Recipient string `json:"recipient"`
		EmailType string `json:"email_type"`
	}
	if err := msg.UnmarshalPayload(&payload); err != nil {
		t.Fatalf("UnmarshalPayload: %v", err)
	}
	if payload.Recipient != "user@example.com" || payload.EmailType != "welcome" {
		t.Errorf("payload = %+v", payload)
	}
}

func TestMessage_UnmarshalEnvelope_ToleratesUnknownFields(t *testing.T) {
	raw := []byte(`{
		"id": "id-1",
		"type": "profile.task",
		"timestamp": "2026-01-30T12:34:56Z",
		"payload": {"profile_id": "p-1", "task_type": "sync"},
		"metadata": {"source": "api-service", "trace_id": "abc"},
		"correlation_id": "corr-1",
		"priority": 0,
		"some_future_field": {"nested": true}
	}`)

	var msg Message
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatalf("unmarshal envelope with unknown top-level fields: %v", err)
	}

	if msg.Metadata["source"] != "api-service" || msg.Metadata["trace_id"] != "abc" {
		t.Errorf("Metadata = %#v", msg.Metadata)
	}
	if msg.ID != "id-1" || msg.Type != "profile.task" {
		t.Errorf("envelope fields not parsed correctly: %+v", msg)
	}
}

func TestMessage_UnmarshalPayload_IsInnerObjectOnly(t *testing.T) {
	// Regression guard: the queue layer must hand processors just the
	// inner "payload" object, not the whole envelope re-wrapped.
	raw := []byte(`{"id":"id-1","type":"image.process","timestamp":"2026-01-30T12:34:56Z","payload":{"operation":"resize","source_url":"s3://b/p.png","target_path":"out.png"}}`)

	var msg Message
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	var payload map[string]interface{}
	if err := msg.UnmarshalPayload(&payload); err != nil {
		t.Fatalf("UnmarshalPayload: %v", err)
	}

	if _, hasEnvelopeField := payload["type"]; hasEnvelopeField {
		t.Errorf("payload should not contain envelope-level fields like 'type', got %#v", payload)
	}
	if payload["operation"] != "resize" {
		t.Errorf("payload[operation] = %v, want resize", payload["operation"])
	}
}
