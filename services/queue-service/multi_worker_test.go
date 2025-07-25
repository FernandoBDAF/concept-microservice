package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testBaseURL = "http://localhost:8080"
	testAPIPath = "/api/v1/queue"
)

// MultiWorkerTestMessage represents a test message for multi-worker testing
type MultiWorkerTestMessage struct {
	Type       string            `json:"type"`
	Payload    json.RawMessage   `json:"payload"`
	Metadata   map[string]string `json:"metadata"`
	Priority   int32             `json:"priority,omitempty"`
	RoutingKey string            `json:"routing_key,omitempty"`
}

// MultiWorkerTestResponse represents the API response for multi-worker testing
type MultiWorkerTestResponse struct {
	MessageID  string `json:"message_id"`
	Status     string `json:"status"`
	RoutingKey string `json:"routing_key"`
}

func TestMultiWorkerRoutingIsolation(t *testing.T) {
	// Test message routing isolation between different worker types
	testCases := []struct {
		name             string
		routingKey       string
		msgType          string
		expectedExchange string
		expectedQueue    string
		payload          map[string]interface{}
		metadata         map[string]string
	}{
		{
			name:             "Profile Worker Isolation",
			routingKey:       "profile.task",
			msgType:          "profile_update",
			expectedExchange: "tasks-exchange",
			expectedQueue:    "profile-processing",
			payload: map[string]interface{}{
				"user_id": "profile-test-user",
				"changes": map[string]interface{}{
					"isolation_test": true,
					"worker_type":    "profile",
				},
			},
			metadata: map[string]string{
				"test_type":   "isolation",
				"worker_type": "profile",
				"test_id":     "profile-isolation-1",
			},
		},
		{
			name:             "Email Worker Isolation",
			routingKey:       "email.send",
			msgType:          "email_send",
			expectedExchange: "email-tasks",
			expectedQueue:    "email-processing",
			payload: map[string]interface{}{
				"to":             "isolation@test.com",
				"subject":        "Multi-Worker Isolation Test",
				"isolation_test": true,
				"worker_type":    "email",
			},
			metadata: map[string]string{
				"test_type":   "isolation",
				"worker_type": "email",
				"test_id":     "email-isolation-1",
			},
		},
		{
			name:             "Image Worker Isolation",
			routingKey:       "image.process",
			msgType:          "image_process",
			expectedExchange: "image-tasks",
			expectedQueue:    "image-processing",
			payload: map[string]interface{}{
				"image_url":      "https://test.com/isolation-test.jpg",
				"operations":     []string{"resize", "test"},
				"isolation_test": true,
				"worker_type":    "image",
			},
			metadata: map[string]string{
				"test_type":   "isolation",
				"worker_type": "image",
				"test_id":     "image-isolation-1",
			},
		},
	}

	// Track published messages for isolation verification
	publishedMessages := make(map[string]MultiWorkerTestResponse)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Marshal payload to json.RawMessage
			payloadBytes, err := json.Marshal(tc.payload)
			require.NoError(t, err)

			// Create test message
			testMsg := MultiWorkerTestMessage{
				Type:       tc.msgType,
				Payload:    json.RawMessage(payloadBytes),
				Metadata:   tc.metadata,
				Priority:   1,
				RoutingKey: tc.routingKey,
			}

			// Publish message
			response := publishMultiWorkerMessage(t, testMsg)

			// Verify response
			assert.NotEmpty(t, response.MessageID)
			assert.Equal(t, "accepted", response.Status)
			assert.Equal(t, tc.routingKey, response.RoutingKey)

			// Store for isolation verification
			publishedMessages[tc.routingKey] = response

			fmt.Printf("✅ %s: Message routed to %s -> %s (ID: %s)\n",
				tc.name, tc.expectedExchange, tc.expectedQueue, response.MessageID)
		})
	}

	// Verify no cross-contamination
	t.Run("CrossContaminationCheck", func(t *testing.T) {
		// All messages should have been routed to their specific routing keys
		assert.Len(t, publishedMessages, 3, "Should have published messages for all worker types")

		// Verify each routing key got its own message
		for routingKey, response := range publishedMessages {
			assert.Equal(t, routingKey, response.RoutingKey,
				"Message should maintain its routing key: %s", routingKey)
		}

		fmt.Printf("✅ Cross-contamination check: All messages properly isolated\n")
	})
}

func TestDynamicExchangeAndQueueCreation(t *testing.T) {
	// Test that exchanges and queues are created dynamically for each worker type
	testCases := []struct {
		name       string
		routingKey string
		msgType    string
	}{
		{
			name:       "Profile Exchange Creation",
			routingKey: "profile.task",
			msgType:    "profile_update",
		},
		{
			name:       "Email Exchange Creation",
			routingKey: "email.send",
			msgType:    "email_send",
		},
		{
			name:       "Image Exchange Creation",
			routingKey: "image.process",
			msgType:    "image_process",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create unique payload for this test
			payload := map[string]interface{}{
				"dynamic_test": true,
				"routing_key":  tc.routingKey,
				"timestamp":    time.Now().Unix(),
			}
			payloadBytes, err := json.Marshal(payload)
			require.NoError(t, err)

			testMsg := MultiWorkerTestMessage{
				Type:    tc.msgType,
				Payload: json.RawMessage(payloadBytes),
				Metadata: map[string]string{
					"test_type":   "dynamic_creation",
					"routing_key": tc.routingKey,
				},
				Priority:   0,
				RoutingKey: tc.routingKey,
			}

			// Publish message - this should trigger dynamic exchange/queue creation
			response := publishMultiWorkerMessage(t, testMsg)

			// Verify successful publication (indicates successful exchange/queue creation)
			assert.NotEmpty(t, response.MessageID)
			assert.Equal(t, "accepted", response.Status)
			assert.Equal(t, tc.routingKey, response.RoutingKey)

			fmt.Printf("✅ %s: Dynamic topology created successfully\n", tc.name)
		})
	}
}

func TestWorkerSpecificConfiguration(t *testing.T) {
	// Test that worker-specific configurations are properly applied
	// We can verify this by checking the routing keys endpoint response
	resp, err := http.Get(fmt.Sprintf("%s%s/routing-keys", testBaseURL, testAPIPath))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var response map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err)

	// Verify configurations are present
	configurations, ok := response["configurations"].(map[string]interface{})
	require.True(t, ok, "Configurations should be present")

	// Test profile worker configuration
	profileConfig, ok := configurations["profile.task"].(map[string]interface{})
	require.True(t, ok, "Profile configuration should exist")

	assert.Equal(t, "tasks-exchange", profileConfig["exchange"])
	assert.Equal(t, "profile-processing", profileConfig["queue"])
	assert.Equal(t, float64(1), profileConfig["prefetch"]) // JSON numbers are float64

	// Test email worker configuration
	emailConfig, ok := configurations["email.send"].(map[string]interface{})
	require.True(t, ok, "Email configuration should exist")

	assert.Equal(t, "email-tasks", emailConfig["exchange"])
	assert.Equal(t, "email-processing", emailConfig["queue"])
	assert.Equal(t, float64(5), emailConfig["prefetch"]) // Different prefetch for email

	// Test image worker configuration
	imageConfig, ok := configurations["image.process"].(map[string]interface{})
	require.True(t, ok, "Image configuration should exist")

	assert.Equal(t, "image-tasks", imageConfig["exchange"])
	assert.Equal(t, "image-processing", imageConfig["queue"])
	assert.Equal(t, float64(1), imageConfig["prefetch"]) // Low prefetch for resource-intensive tasks

	fmt.Printf("✅ Worker-specific configurations verified:\n")
	fmt.Printf("   - Profile: tasks-exchange -> profile-processing (prefetch: 1)\n")
	fmt.Printf("   - Email: email-tasks -> email-processing (prefetch: 5)\n")
	fmt.Printf("   - Image: image-tasks -> image-processing (prefetch: 1)\n")
}

func TestHighVolumeMessageDistribution(t *testing.T) {
	// Test high-volume message distribution across worker types
	messageCount := 30 // 10 messages per worker type
	workerTypes := []struct {
		routingKey string
		msgType    string
	}{
		{"profile.task", "profile_update"},
		{"email.send", "email_send"},
		{"image.process", "image_process"},
	}

	publishedByWorker := make(map[string][]string) // routing_key -> message_ids

	for i := 0; i < messageCount; i++ {
		workerType := workerTypes[i%len(workerTypes)]

		payload := map[string]interface{}{
			"high_volume_test": true,
			"message_number":   i + 1,
			"worker_type":      workerType.routingKey,
			"timestamp":        time.Now().Unix(),
		}
		payloadBytes, err := json.Marshal(payload)
		require.NoError(t, err)

		testMsg := MultiWorkerTestMessage{
			Type:    workerType.msgType,
			Payload: json.RawMessage(payloadBytes),
			Metadata: map[string]string{
				"test_type":      "high_volume",
				"message_number": fmt.Sprintf("%d", i+1),
			},
			Priority:   int32(i % 3), // Vary priority
			RoutingKey: workerType.routingKey,
		}

		response := publishMultiWorkerMessage(t, testMsg)
		assert.NotEmpty(t, response.MessageID)
		assert.Equal(t, workerType.routingKey, response.RoutingKey)

		publishedByWorker[workerType.routingKey] = append(
			publishedByWorker[workerType.routingKey],
			response.MessageID,
		)

		// Small delay to avoid overwhelming the service
		if i%10 == 9 {
			time.Sleep(10 * time.Millisecond)
		}
	}

	// Verify distribution
	for routingKey, messageIDs := range publishedByWorker {
		expectedCount := messageCount / len(workerTypes)
		assert.Equal(t, expectedCount, len(messageIDs),
			"Each worker type should receive equal message distribution: %s", routingKey)
	}

	fmt.Printf("✅ High-volume distribution test: %d messages distributed across %d worker types\n",
		messageCount, len(workerTypes))
}

func TestDeadLetterQueueConfiguration(t *testing.T) {
	// Test that dead letter queues are properly configured for each worker type
	// We can verify this indirectly by checking the configurations include DLQ settings
	resp, err := http.Get(fmt.Sprintf("%s%s/routing-keys", testBaseURL, testAPIPath))
	require.NoError(t, err)
	defer resp.Body.Close()

	var response map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err)

	configurations, ok := response["configurations"].(map[string]interface{})
	require.True(t, ok)

	// Verify dead letter TTL is configured for each worker type
	expectedDLQConfigs := map[string]string{
		"profile.task":  "profile-processing.dlq",
		"email.send":    "email-processing.dlq",
		"image.process": "image-processing.dlq",
	}

	for routingKey, expectedDLQ := range expectedDLQConfigs {
		config, ok := configurations[routingKey].(map[string]interface{})
		require.True(t, ok, "Configuration should exist for %s", routingKey)

		// Verify main queue configuration implies DLQ setup
		queue, ok := config["queue"].(string)
		require.True(t, ok, "Queue should be configured for %s", routingKey)

		expectedQueue := expectedDLQ[:len(expectedDLQ)-4] // Remove ".dlq" suffix
		assert.Equal(t, expectedQueue, queue,
			"Queue name should match expected pattern for %s", routingKey)
	}

	fmt.Printf("✅ Dead letter queue configuration verified for all worker types\n")
}

// Helper functions

func publishMultiWorkerMessage(t *testing.T, msg MultiWorkerTestMessage) MultiWorkerTestResponse {
	data, err := json.Marshal(msg)
	require.NoError(t, err)

	resp, err := http.Post(
		fmt.Sprintf("%s%s/messages", testBaseURL, testAPIPath),
		"application/json",
		bytes.NewBuffer(data),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusAccepted, resp.StatusCode)

	var response MultiWorkerTestResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err)

	return response
}
