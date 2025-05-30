package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// MessageType represents the type of message being processed
type MessageType string

const (
	// MessageTypeProfileUpdate represents a profile update message
	MessageTypeProfileUpdate MessageType = "profile_update"
	// MessageTypeCacheInvalidation represents a cache invalidation message
	MessageTypeCacheInvalidation MessageType = "cache_invalidation"
	// MessageTypeBackgroundJob represents a background job message
	MessageTypeBackgroundJob MessageType = "background_job"
)

// Message represents a message in the queue
type Message struct {
	ID            string            `json:"id" validate:"required"`
	Type          MessageType       `json:"type" validate:"required,oneof=profile_update cache_invalidation background_job"`
	Timestamp     time.Time         `json:"timestamp" validate:"required"`
	CorrelationID string            `json:"correlation_id"`
	Payload       interface{}       `json:"payload" validate:"required"`
	Headers       map[string]string `json:"headers"`
	Priority      int32             `json:"priority" validate:"min=0,max=9"`
}

// NewMessage creates a new message with default values
func NewMessage(msgType MessageType, payload interface{}) *Message {
	return &Message{
		ID:            uuid.New().String(),
		Type:          msgType,
		Timestamp:     time.Now().UTC(),
		CorrelationID: uuid.New().String(),
		Payload:       payload,
		Headers:       make(map[string]string),
		Priority:      0,
	}
}

// MarshalJSON implements the json.Marshaler interface
func (m *Message) MarshalJSON() ([]byte, error) {
	type Alias Message
	return json.Marshal(&struct {
		*Alias
		Timestamp string `json:"timestamp"`
	}{
		Alias:     (*Alias)(m),
		Timestamp: m.Timestamp.Format(time.RFC3339),
	})
}

// UnmarshalJSON implements the json.Unmarshaler interface
func (m *Message) UnmarshalJSON(data []byte) error {
	type Alias Message
	aux := &struct {
		*Alias
		Timestamp string `json:"timestamp"`
	}{
		Alias: (*Alias)(m),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	t, err := time.Parse(time.RFC3339, aux.Timestamp)
	if err != nil {
		return err
	}

	m.Timestamp = t
	return nil
}

// ProfileUpdate represents a profile update message payload
type ProfileUpdate struct {
	UserID    string                 `json:"user_id" validate:"required"`
	Changes   map[string]interface{} `json:"changes" validate:"required"`
	Timestamp time.Time              `json:"timestamp" validate:"required"`
}

// CacheInvalidation represents a cache invalidation message payload
type CacheInvalidation struct {
	Keys     []string `json:"keys" validate:"required"`
	Pattern  string   `json:"pattern"`
	AllCache bool     `json:"all_cache"`
}

// BackgroundJob represents a background job message payload
type BackgroundJob struct {
	JobType    string                 `json:"job_type" validate:"required"`
	Parameters map[string]interface{} `json:"parameters"`
	Priority   int32                  `json:"priority" validate:"min=0,max=9"`
}
