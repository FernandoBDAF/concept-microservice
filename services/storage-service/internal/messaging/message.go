package messaging

import (
	"encoding/json"
	"time"

	"microservices/services/profile-storage/internal/domain/models"

	"github.com/google/uuid"
)

// Message represents the standardized message format for the ecosystem
type Message struct {
	ID          string          `json:"id"`
	Type        string          `json:"type"`
	RoutingKey  string          `json:"routing_key"`
	Payload     json.RawMessage `json:"payload"`
	Timestamp   time.Time       `json:"timestamp"`
	Source      string          `json:"source"`
	Correlation string          `json:"correlation_id,omitempty"`
	Priority    int             `json:"priority,omitempty"`
	RetryCount  int             `json:"retry_count,omitempty"`
	MaxRetries  int             `json:"max_retries,omitempty"`
}

// BatchStorageTask represents a batch of storage operations
type BatchStorageTask struct {
	BatchID    string               `json:"batch_id"`
	Operations []models.StorageTask `json:"operations"`
	Options    models.BatchOptions  `json:"options,omitempty"`
	Timestamp  time.Time            `json:"timestamp"`
}

// MessageResponse represents the response format for processed messages
type MessageResponse struct {
	MessageID      string                 `json:"message_id"`
	Success        bool                   `json:"success"`
	Error          string                 `json:"error,omitempty"`
	Result         map[string]interface{} `json:"result,omitempty"`
	ProcessedAt    time.Time              `json:"processed_at"`
	ProcessingTime time.Duration          `json:"processing_time"`
}

// NewMessage creates a new message with required fields
func NewMessage(messageType, routingKey string, payload interface{}) (*Message, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	return &Message{
		ID:         uuid.New().String(),
		Type:       messageType,
		RoutingKey: routingKey,
		Payload:    payloadBytes,
		Timestamp:  time.Now(),
		Source:     "storage-service",
		Priority:   0,
		RetryCount: 0,
		MaxRetries: 3,
	}, nil
}

// UnmarshalPayload unmarshals the message payload into the provided interface
func (m *Message) UnmarshalPayload(v interface{}) error {
	return json.Unmarshal(m.Payload, v)
}

// IsExpired checks if the message has exceeded its maximum age
func (m *Message) IsExpired(maxAge time.Duration) bool {
	return time.Since(m.Timestamp) > maxAge
}

// ShouldRetry determines if a message should be retried based on retry count
func (m *Message) ShouldRetry() bool {
	return m.RetryCount < m.MaxRetries
}

// IncrementRetry increments the retry count
func (m *Message) IncrementRetry() {
	m.RetryCount++
}

// NewStorageTask creates a new storage task
func NewStorageTask(operation string, profileID *uuid.UUID, data map[string]interface{}) *models.StorageTask {
	return &models.StorageTask{
		Operation:   operation,
		ProfileID:   profileID,
		Data:        data,
		Options:     make(map[string]interface{}),
		Timestamp:   time.Now(),
		RequestedBy: "system",
	}
}

// NewBatchStorageTask creates a new batch storage task
func NewBatchStorageTask(operations []models.StorageTask, options models.BatchOptions) *BatchStorageTask {
	return &BatchStorageTask{
		BatchID:    uuid.New().String(),
		Operations: operations,
		Options:    options,
		Timestamp:  time.Now(),
	}
}
