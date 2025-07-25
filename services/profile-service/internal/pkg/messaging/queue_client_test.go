package messaging

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestQueueMessage_Format tests the updated message format for queue-service compatibility
func TestQueueMessage_Format(t *testing.T) {
	// Create test payload
	payloadData := map[string]interface{}{
		"task_id":    "task-123",
		"profile_id": "profile-456",
		"payload":    map[string]interface{}{"action": "update"},
	}
	payloadBytes, err := json.Marshal(payloadData)
	assert.NoError(t, err, "Payload serialization should not fail")

	// Create message with new format
	msg := &QueueMessage{
		ID:         "msg-123",
		Type:       "profile_update",
		Payload:    json.RawMessage(payloadBytes),
		Timestamp:  time.Now().UTC(),
		Metadata:   map[string]string{"source": "profile-service"},
		RoutingKey: "profile.task",
	}

	// Verify message structure
	assert.NotEmpty(t, msg.ID, "Message ID should not be empty")
	assert.Equal(t, "profile_update", msg.Type, "Message type should match")
	assert.NotNil(t, msg.Payload, "Payload should not be nil")
	assert.False(t, msg.Timestamp.IsZero(), "Timestamp should not be zero")
	assert.NotNil(t, msg.Metadata, "Metadata should not be nil")
	assert.Equal(t, "profile.task", msg.RoutingKey, "Routing key should match")

	// Test JSON serialization compatibility
	jsonBytes, err := json.Marshal(msg)
	assert.NoError(t, err, "Message should serialize to JSON without error")

	// Verify JSON contains expected fields
	var jsonMap map[string]interface{}
	err = json.Unmarshal(jsonBytes, &jsonMap)
	assert.NoError(t, err, "JSON should deserialize without error")

	assert.Contains(t, jsonMap, "id", "JSON should contain id field")
	assert.Contains(t, jsonMap, "type", "JSON should contain type field")
	assert.Contains(t, jsonMap, "payload", "JSON should contain payload field")
	assert.Contains(t, jsonMap, "timestamp", "JSON should contain timestamp field")
	assert.Contains(t, jsonMap, "metadata", "JSON should contain metadata field")
	assert.Contains(t, jsonMap, "routing_key", "JSON should contain routing_key field")

	// Verify old fields are not present
	assert.NotContains(t, jsonMap, "correlation_id", "JSON should not contain deprecated correlation_id field")
	assert.NotContains(t, jsonMap, "priority", "JSON should not contain deprecated priority field")
	assert.NotContains(t, jsonMap, "headers", "JSON should not contain deprecated headers field")
}

// TestQueueMessage_SupportedTaskTypes tests validation for supported task types
func TestQueueMessage_SupportedTaskTypes(t *testing.T) {
	supportedTypes := []string{
		"profile_update",
		"email_notification",
		"image_processing",
	}

	for _, taskType := range supportedTypes {
		t.Run("TaskType_"+taskType, func(t *testing.T) {
			payloadBytes, _ := json.Marshal(map[string]interface{}{"test": "data"})

			msg := &QueueMessage{
				ID:         "test-id",
				Type:       taskType,
				Payload:    json.RawMessage(payloadBytes),
				Timestamp:  time.Now().UTC(),
				Metadata:   make(map[string]string),
				RoutingKey: "test.route",
			}

			// Verify the message can be created and serialized
			jsonBytes, err := json.Marshal(msg)
			assert.NoError(t, err, "Message with type %s should serialize successfully", taskType)
			assert.NotEmpty(t, jsonBytes, "Serialized JSON should not be empty")
		})
	}
}

// TestQueueMessage_TimestampHandling tests the time.Time timestamp handling
func TestQueueMessage_TimestampHandling(t *testing.T) {
	now := time.Now().UTC()
	payloadBytes, _ := json.Marshal(map[string]interface{}{"test": "data"})

	msg := &QueueMessage{
		ID:         "test-id",
		Type:       "profile_update",
		Payload:    json.RawMessage(payloadBytes),
		Timestamp:  now,
		Metadata:   make(map[string]string),
		RoutingKey: "profile.task",
	}

	// Test JSON serialization/deserialization of timestamp
	jsonBytes, err := json.Marshal(msg)
	assert.NoError(t, err, "Message should serialize successfully")

	var deserializedMsg QueueMessage
	err = json.Unmarshal(jsonBytes, &deserializedMsg)
	assert.NoError(t, err, "Message should deserialize successfully")

	// Verify timestamp is preserved correctly (within 1 second tolerance for JSON precision)
	timeDiff := deserializedMsg.Timestamp.Sub(now)
	assert.True(t, timeDiff < time.Second && timeDiff > -time.Second,
		"Deserialized timestamp should match original within 1 second")
}

// TestQueueMessage_PayloadAsRawMessage tests json.RawMessage payload handling
func TestQueueMessage_PayloadAsRawMessage(t *testing.T) {
	originalPayload := map[string]interface{}{
		"task_id":    "task-123",
		"profile_id": "profile-456",
		"action":     "update",
		"nested": map[string]interface{}{
			"field": "value",
		},
	}

	payloadBytes, err := json.Marshal(originalPayload)
	assert.NoError(t, err, "Original payload should serialize successfully")

	msg := &QueueMessage{
		ID:         "test-id",
		Type:       "profile_update",
		Payload:    json.RawMessage(payloadBytes),
		Timestamp:  time.Now().UTC(),
		Metadata:   make(map[string]string),
		RoutingKey: "profile.task",
	}

	// Serialize and deserialize the entire message
	jsonBytes, err := json.Marshal(msg)
	assert.NoError(t, err, "Message should serialize successfully")

	var deserializedMsg QueueMessage
	err = json.Unmarshal(jsonBytes, &deserializedMsg)
	assert.NoError(t, err, "Message should deserialize successfully")

	// Verify payload is preserved as RawMessage
	assert.Equal(t, msg.Payload, deserializedMsg.Payload,
		"Payload should be preserved exactly as json.RawMessage")

	// Verify we can deserialize the payload back to original structure
	var deserializedPayload map[string]interface{}
	err = json.Unmarshal(deserializedMsg.Payload, &deserializedPayload)
	assert.NoError(t, err, "Payload should deserialize to original structure")
	assert.Equal(t, "task-123", deserializedPayload["task_id"], "Payload content should match original")
}
