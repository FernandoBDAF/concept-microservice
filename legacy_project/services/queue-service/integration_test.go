package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/FBDAF/microservices/services/queue-service/internal/domain/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	baseURL = "http://localhost:8080"
	apiPath = "/api/v1/queue"
)

// TestMessage represents the expected message format for worker-service compatibility
type TestMessage struct {
	Type       string            `json:"type"`
	Payload    json.RawMessage   `json:"payload"`
	Metadata   map[string]string `json:"metadata"`
	Priority   int32             `json:"priority,omitempty"`
	RoutingKey string            `json:"routing_key,omitempty"`
}

// TestResponse represents the API response
type TestResponse struct {
	MessageID  string `json:"message_id"`
	Status     string `json:"status"`
	RoutingKey string `json:"routing_key"`
}

// TestMessageStatus represents message status response
type TestMessageStatus struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
}

func TestWorkerServiceCompatibility(t *testing.T) {
	// Test cases for different worker types
	testCases := []struct {
		name       string
		routingKey string
		msgType    string
		payload    map[string]interface{}
		metadata   map[string]string
		priority   int32
	}{
		{
			name:       "Profile Task",
			routingKey: "profile.task",
			msgType:    "profile_update",
			payload: map[string]interface{}{
				"user_id": "12345",
				"changes": map[string]interface{}{
					"name":  "John Doe",
					"email": "john@example.com",
				},
			},
			metadata: map[string]string{
				"source":  "profile-service",
				"version": "1.0",
			},
			priority: 1,
		},
		{
			name:       "Email Task",
			routingKey: "email.send",
			msgType:    "email_send",
			payload: map[string]interface{}{
				"to":       "user@example.com",
				"subject":  "Welcome!",
				"template": "welcome_email",
				"data": map[string]interface{}{
					"user_name": "John Doe",
				},
			},
			metadata: map[string]string{
				"source": "notification-service",
			},
			priority: 2,
		},
		{
			name:       "Image Processing Task",
			routingKey: "image.process",
			msgType:    "image_process",
			payload: map[string]interface{}{
				"image_url":  "https://example.com/image.jpg",
				"operations": []string{"resize", "optimize"},
				"dimensions": map[string]int{
					"width":  800,
					"height": 600,
				},
			},
			metadata: map[string]string{
				"source":    "media-service",
				"user_id":   "12345",
				"operation": "thumbnail_generation",
			},
			priority: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Marshal payload to json.RawMessage for worker-service compatibility
			payloadBytes, err := json.Marshal(tc.payload)
			require.NoError(t, err)

			// Create test message
			testMsg := TestMessage{
				Type:       tc.msgType,
				Payload:    json.RawMessage(payloadBytes),
				Metadata:   tc.metadata,
				Priority:   tc.priority,
				RoutingKey: tc.routingKey,
			}

			// Publish message
			response := publishMessage(t, testMsg)

			// Verify response format
			assert.NotEmpty(t, response.MessageID)
			assert.Equal(t, "accepted", response.Status)
			assert.Equal(t, tc.routingKey, response.RoutingKey)

			// Verify message status
			time.Sleep(100 * time.Millisecond) // Allow processing time
			status := getMessageStatus(t, response.MessageID)
			assert.NotEmpty(t, status.ID)
			assert.Contains(t, []string{"accepted", "published"}, status.Status)
			assert.NotEmpty(t, status.Timestamp)

			fmt.Printf("✅ %s: Message published successfully with routing key %s\n",
				tc.name, tc.routingKey)
		})
	}
}

func TestBackwardCompatibility(t *testing.T) {
	// Test backward compatibility without routing key
	testCases := []struct {
		name     string
		msgType  string
		expected string
	}{
		{
			name:     "Profile Update (Legacy)",
			msgType:  "profile_update",
			expected: "profile.task",
		},
		{
			name:     "Email Send (Legacy)",
			msgType:  "email_send",
			expected: "email.send",
		},
		{
			name:     "Image Process (Legacy)",
			msgType:  "image_process",
			expected: "image.process",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			payload := map[string]interface{}{
				"test": "data",
			}
			payloadBytes, err := json.Marshal(payload)
			require.NoError(t, err)

			// Create message without routing key
			testMsg := TestMessage{
				Type:    tc.msgType,
				Payload: json.RawMessage(payloadBytes),
				Metadata: map[string]string{
					"source": "legacy-system",
				},
			}

			// Publish message
			response := publishMessage(t, testMsg)

			// Verify routing key inference
			assert.Equal(t, tc.expected, response.RoutingKey)

			fmt.Printf("✅ %s: Routing key correctly inferred as %s\n",
				tc.name, response.RoutingKey)
		})
	}
}

func TestRoutingKeyValidation(t *testing.T) {
	testCases := []struct {
		name       string
		routingKey string
		shouldFail bool
	}{
		{
			name:       "Valid Profile Routing Key",
			routingKey: "profile.task",
			shouldFail: false,
		},
		{
			name:       "Valid Email Routing Key",
			routingKey: "email.send",
			shouldFail: false,
		},
		{
			name:       "Valid Image Routing Key",
			routingKey: "image.process",
			shouldFail: false,
		},
		{
			name:       "Invalid Routing Key",
			routingKey: "invalid.key",
			shouldFail: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			payload := map[string]interface{}{"test": "data"}
			payloadBytes, err := json.Marshal(payload)
			require.NoError(t, err)

			testMsg := TestMessage{
				Type:       "test_message",
				Payload:    json.RawMessage(payloadBytes),
				Metadata:   map[string]string{"source": "test"},
				RoutingKey: tc.routingKey,
			}

			if tc.shouldFail {
				// Expect validation error
				publishMessageExpectError(t, testMsg, http.StatusBadRequest)
				fmt.Printf("✅ %s: Validation correctly rejected invalid routing key\n", tc.name)
			} else {
				// Expect success
				response := publishMessage(t, testMsg)
				assert.Equal(t, tc.routingKey, response.RoutingKey)
				fmt.Printf("✅ %s: Valid routing key accepted\n", tc.name)
			}
		})
	}
}

func TestSupportedRoutingKeys(t *testing.T) {
	// Test the routing keys endpoint
	resp, err := http.Get(fmt.Sprintf("%s%s/routing-keys", baseURL, apiPath))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var response map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err)

	// Verify routing keys are present
	routingKeys, ok := response["routing_keys"].([]interface{})
	require.True(t, ok)
	assert.Contains(t, routingKeys, "profile.task")
	assert.Contains(t, routingKeys, "email.send")
	assert.Contains(t, routingKeys, "image.process")

	// Verify configurations are present
	configurations, ok := response["configurations"].(map[string]interface{})
	require.True(t, ok)
	assert.NotEmpty(t, configurations)

	fmt.Printf("✅ Routing keys endpoint working correctly\n")
}

func TestMessageFormatCompatibility(t *testing.T) {
	// Test that our message format matches worker-service expectations
	expectedFormat := model.Message{
		ID:            "test-id",
		Type:          "profile_update",
		Metadata:      map[string]string{"source": "test"},
		Payload:       json.RawMessage(`{"test": "data"}`),
		Priority:      1,
		Timestamp:     time.Now(),
		CorrelationID: "test-correlation",
	}

	// Serialize and deserialize to verify format
	data, err := json.Marshal(expectedFormat)
	require.NoError(t, err)

	var deserialized model.Message
	err = json.Unmarshal(data, &deserialized)
	require.NoError(t, err)

	// Verify critical fields
	assert.Equal(t, expectedFormat.Type, deserialized.Type)
	assert.Equal(t, expectedFormat.Metadata, deserialized.Metadata)
	assert.Equal(t, expectedFormat.Payload, deserialized.Payload)
	assert.Equal(t, expectedFormat.Priority, deserialized.Priority)

	fmt.Printf("✅ Message format compatible with worker-service expectations\n")
}

// Helper functions

func publishMessage(t *testing.T, msg TestMessage) TestResponse {
	data, err := json.Marshal(msg)
	require.NoError(t, err)

	resp, err := http.Post(
		fmt.Sprintf("%s%s/messages", baseURL, apiPath),
		"application/json",
		bytes.NewBuffer(data),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusAccepted, resp.StatusCode)

	var response TestResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err)

	return response
}

func publishMessageExpectError(t *testing.T, msg TestMessage, expectedStatus int) {
	data, err := json.Marshal(msg)
	require.NoError(t, err)

	resp, err := http.Post(
		fmt.Sprintf("%s%s/messages", baseURL, apiPath),
		"application/json",
		bytes.NewBuffer(data),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, expectedStatus, resp.StatusCode)
}

func getMessageStatus(t *testing.T, messageID string) TestMessageStatus {
	resp, err := http.Get(fmt.Sprintf("%s%s/status/%s", baseURL, apiPath, messageID))
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// Message might not be stored yet, return empty status
		return TestMessageStatus{}
	}

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var status TestMessageStatus
	err = json.NewDecoder(resp.Body).Decode(&status)
	require.NoError(t, err)

	return status
}

// TestMain sets up the test environment
func TestMain(m *testing.M) {
	// Check if service is running
	resp, err := http.Get(fmt.Sprintf("%s/health", baseURL))
	if err != nil {
		fmt.Printf("⚠️  Queue service not running at %s. Please start the service first.\n", baseURL)
		fmt.Printf("   Run: go run cmd/main.go\n")
		os.Exit(1)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("⚠️  Queue service health check failed\n")
		os.Exit(1)
	}

	fmt.Printf("🚀 Running integration tests against queue service at %s\n", baseURL)

	// Run tests
	code := m.Run()

	if code == 0 {
		fmt.Printf("✅ All integration tests passed!\n")
	} else {
		fmt.Printf("❌ Some integration tests failed\n")
	}

	os.Exit(code)
}
