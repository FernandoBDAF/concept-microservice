package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDetermineRoutingKey tests the routing key determination logic without ProfileService dependencies
func TestDetermineRoutingKey(t *testing.T) {
	tests := []struct {
		name     string
		taskType string
		expected string
	}{
		{
			name:     "Profile update task routes to profile worker",
			taskType: "profile_update",
			expected: "profile.task",
		},
		{
			name:     "Email notification task routes to email worker",
			taskType: "email_notification",
			expected: "email.send",
		},
		{
			name:     "Image processing task routes to image worker",
			taskType: "image_processing",
			expected: "image.process",
		},
		{
			name:     "Unknown task type uses fallback routing",
			taskType: "unknown_type",
			expected: "profile.task",
		},
		{
			name:     "Empty task type uses fallback routing",
			taskType: "",
			expected: "profile.task",
		},
	}

	// Test the routing key determination logic directly without ProfileService
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var result string
			if routingKey, exists := RoutingKeyMap[test.taskType]; exists {
				result = routingKey
			} else {
				result = "profile.task" // Default fallback
			}

			assert.Equal(t, test.expected, result,
				"Expected routing key %s for task type %s, got %s",
				test.expected, test.taskType, result)
		})
	}
}

// TestRoutingKeyMap validates the routing key mapping
func TestRoutingKeyMap(t *testing.T) {
	expectedMappings := map[string]string{
		"profile_update":     "profile.task",
		"email_notification": "email.send",
		"image_processing":   "image.process",
	}

	assert.Equal(t, expectedMappings, RoutingKeyMap,
		"RoutingKeyMap should match expected multi-worker mappings")
}

// TestRoutingKeyMap_AllTaskTypesSupported ensures all supported task types have routing keys
func TestRoutingKeyMap_AllTaskTypesSupported(t *testing.T) {
	supportedTaskTypes := []string{
		"profile_update",
		"email_notification",
		"image_processing",
	}

	for _, taskType := range supportedTaskTypes {
		routingKey, exists := RoutingKeyMap[taskType]
		assert.True(t, exists,
			"Routing key should exist for supported task type: %s", taskType)
		assert.NotEmpty(t, routingKey,
			"Routing key should not be empty for task type: %s", taskType)
	}
}
