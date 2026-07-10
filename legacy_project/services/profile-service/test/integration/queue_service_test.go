package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fernandobarroso/microservices/services/profile-service/internal/api/handlers"
	"github.com/fernandobarroso/microservices/services/profile-service/internal/config"
	"github.com/fernandobarroso/microservices/services/profile-service/internal/domain/models"
	"github.com/fernandobarroso/microservices/services/profile-service/internal/domain/services"
	"github.com/fernandobarroso/microservices/services/profile-service/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ✅ Phase 4: End-to-End Integration Testing
// Tests the complete flow from API → Queue Service → Worker routing

// MockQueueService simulates the queue service for integration testing
type MockQueueService struct {
	server           *httptest.Server
	receivedMessages []ReceivedMessage
	responseCode     int
	responseDelay    time.Duration
}

type ReceivedMessage struct {
	ID         string            `json:"id"`
	Type       string            `json:"type"`
	Payload    json.RawMessage   `json:"payload"`
	Timestamp  time.Time         `json:"timestamp"`
	Metadata   map[string]string `json:"metadata"`
	RoutingKey string            `json:"routing_key"`
	Headers    map[string]string `json:"headers"`
}

func NewMockQueueService() *MockQueueService {
	mock := &MockQueueService{
		receivedMessages: make([]ReceivedMessage, 0),
		responseCode:     200,
		responseDelay:    0,
	}

	// Create HTTP server that mimics queue service
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate processing delay
		if mock.responseDelay > 0 {
			time.Sleep(mock.responseDelay)
		}

		// Parse incoming message
		var msg ReceivedMessage
		if err := json.NewDecoder(r.Body).Decode(&msg); err == nil {
			// Capture headers
			msg.Headers = make(map[string]string)
			for key, values := range r.Header {
				if len(values) > 0 {
					msg.Headers[key] = values[0]
				}
			}
			mock.receivedMessages = append(mock.receivedMessages, msg)
		}

		// Send response
		w.WriteHeader(mock.responseCode)
		if mock.responseCode == 200 {
			response := map[string]interface{}{
				"message":     "Message received successfully",
				"id":          msg.ID,
				"routing_key": msg.RoutingKey,
			}
			json.NewEncoder(w).Encode(response)
		} else {
			errorResponse := map[string]interface{}{
				"error": "Queue service error",
				"code":  mock.responseCode,
			}
			json.NewEncoder(w).Encode(errorResponse)
		}
	})

	mock.server = httptest.NewServer(handler)
	return mock
}

func (m *MockQueueService) Close() {
	m.server.Close()
}

func (m *MockQueueService) URL() string {
	return m.server.URL
}

func (m *MockQueueService) SetResponseCode(code int) {
	m.responseCode = code
}

func (m *MockQueueService) SetResponseDelay(delay time.Duration) {
	m.responseDelay = delay
}

func (m *MockQueueService) GetReceivedMessages() []ReceivedMessage {
	return m.receivedMessages
}

func (m *MockQueueService) Reset() {
	m.receivedMessages = make([]ReceivedMessage, 0)
	m.responseCode = 200
	m.responseDelay = 0
}

// setupTestEnvironment creates a test environment with mock queue service
func setupTestEnvironment(t *testing.T) (*gin.Engine, *MockQueueService, *handlers.TaskHandler) {
	// ✅ Initialize logger for tests
	err := logger.Initialize(&logger.Config{
		Level:       "info",
		Environment: "test",
		ServiceName: "profile-service-test",
		Format:      "console",
	})
	require.NoError(t, err, "Failed to initialize logger")

	// Create mock queue service
	mockQueue := NewMockQueueService()

	// Create test configuration
	cfg := &config.Config{
		Queue: config.QueueConfig{
			URL:            mockQueue.URL(),
			Timeout:        5 * time.Second,
			Retries:        3,
			MaxRequestSize: 1024 * 1024,
			CircuitBreaker: config.CircuitBreakerConfig{
				Enabled:               true,
				FailureThreshold:      5,
				RecoveryTimeout:       30 * time.Second,
				MaxConcurrentRequests: 100,
			},
			RoutingKeys: map[string]string{
				"profile_update":     "profile.task",
				"email_notification": "email.send",
				"image_processing":   "image.process",
			},
			TaskTimeouts: map[string]time.Duration{
				"profile_update":     30 * time.Second,
				"email_notification": 60 * time.Second,
				"image_processing":   300 * time.Second,
			},
		},
	}

	// Create mock storage client
	storageClient := &services.StorageClient{}

	// Create profile service with mock dependencies
	profileService := services.NewProfileService(cfg, storageClient, nil, nil, logger.Logger)

	// Create task handler
	taskHandler := handlers.NewTaskHandler(profileService)

	// Setup Gin router
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Add task routes
	v1 := router.Group("/api/v1")
	profiles := v1.Group("/profiles")
	profiles.POST("/:id/tasks", taskHandler.SubmitTask)
	profiles.POST("/:id/tasks/email", taskHandler.SubmitEmailTask)
	profiles.POST("/:id/tasks/image", taskHandler.SubmitImageTask)
	profiles.POST("/:id/tasks/profile", taskHandler.SubmitProfileTask)
	profiles.GET("/:id/tasks/stats", taskHandler.GetTaskTypeStats)

	return router, mockQueue, taskHandler
}

// TestProfileTaskEndToEnd tests the complete profile task flow
func TestProfileTaskEndToEnd(t *testing.T) {
	router, mockQueue, _ := setupTestEnvironment(t)
	defer mockQueue.Close()

	// Test payload
	payload := map[string]interface{}{
		"type": "profile_update",
		"payload": map[string]interface{}{
			"user_id": "user-123",
			"action":  "update",
			"data": map[string]interface{}{
				"name": "John Doe",
			},
		},
	}

	// Make request
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/api/v1/profiles/profile-123/tasks", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify API response
	assert.Equal(t, http.StatusAccepted, w.Code)

	var response handlers.APISuccessResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "v1", response.Version)
	assert.NotNil(t, response.Metadata)
	assert.Equal(t, "profile.task", response.Metadata["routing_key"])
	assert.Equal(t, "profile-worker", response.Metadata["worker_type"])

	// Verify queue service received the message
	messages := mockQueue.GetReceivedMessages()
	require.Len(t, messages, 1)

	msg := messages[0]
	assert.Equal(t, "profile_update", msg.Type)
	assert.Equal(t, "profile.task", msg.RoutingKey)
	assert.NotEmpty(t, msg.ID)
	assert.False(t, msg.Timestamp.IsZero())

	// Verify message metadata
	assert.Equal(t, "profile-service", msg.Metadata["client"])
	assert.Equal(t, "profile-123", msg.Metadata["profile_id"])

	// Verify headers
	assert.Equal(t, "application/json", msg.Headers["Content-Type"])
	assert.Equal(t, "profile-service", msg.Headers["X-Client"])
	assert.Equal(t, "profile.task", msg.Headers["X-Routing-Key"])

	t.Logf("✅ Profile task end-to-end test completed successfully")
}

// TestEmailTaskEndToEnd tests the complete email task flow
func TestEmailTaskEndToEnd(t *testing.T) {
	router, mockQueue, _ := setupTestEnvironment(t)
	defer mockQueue.Close()

	// Test email payload
	payload := models.EmailTaskPayload{
		To:       "user@example.com",
		Template: "welcome",
		Subject:  "Welcome to our service!",
		Priority: 1,
		Data: map[string]interface{}{
			"username":    "john_doe",
			"signup_date": "2024-01-15",
		},
	}

	// Make request to specialized email endpoint
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/api/v1/profiles/profile-123/tasks/email", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify API response
	assert.Equal(t, http.StatusAccepted, w.Code)

	var response handlers.APISuccessResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "v1", response.Version)
	assert.Equal(t, "email-worker", response.Metadata["worker_type"])

	// Extract email response data
	// ✅ FIX: Handle JSON unmarshaling - Data is map[string]interface{} after unmarshaling
	dataMap, ok := response.Data.(map[string]interface{})
	require.True(t, ok, "Response data should be a map")

	// Convert map to EmailTaskResponse for validation
	emailData := &models.EmailTaskResponse{
		TaskID:     dataMap["task_id"].(string),
		Type:       dataMap["type"].(string),
		EmailTo:    dataMap["email_to"].(string),
		Template:   dataMap["template"].(string),
		RoutingKey: dataMap["routing_key"].(string),
	}

	assert.Equal(t, "email_notification", emailData.Type)
	assert.Equal(t, "user@example.com", emailData.EmailTo)
	assert.Equal(t, "welcome", emailData.Template)
	assert.Equal(t, "email.send", emailData.RoutingKey)

	// Verify queue service received the message
	messages := mockQueue.GetReceivedMessages()
	require.Len(t, messages, 1)

	msg := messages[0]
	assert.Equal(t, "email_notification", msg.Type)
	assert.Equal(t, "email.send", msg.RoutingKey)

	// Verify email payload content
	var payloadData map[string]interface{}
	err = json.Unmarshal(msg.Payload, &payloadData)
	require.NoError(t, err)

	payload_inner, exists := payloadData["payload"]
	require.True(t, exists)

	payloadMap, ok := payload_inner.(map[string]interface{})
	require.True(t, ok)

	assert.Equal(t, "user@example.com", payloadMap["to"])
	assert.Equal(t, "welcome", payloadMap["template"])
	assert.Equal(t, "Welcome to our service!", payloadMap["subject"])
	assert.Equal(t, float64(1), payloadMap["priority"]) // JSON unmarshaling converts to float64

	t.Logf("✅ Email task end-to-end test completed successfully")
}

// TestImageTaskEndToEnd tests the complete image processing task flow
func TestImageTaskEndToEnd(t *testing.T) {
	router, mockQueue, _ := setupTestEnvironment(t)
	defer mockQueue.Close()

	// Test image payload
	payload := models.ImageTaskPayload{
		ImageURL:     "https://example.com/user-avatar.jpg",
		Operation:    "resize",
		OutputFormat: "webp",
		Options: map[string]interface{}{
			"width":   400,
			"height":  400,
			"quality": 85,
		},
	}

	// Make request to specialized image endpoint
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/api/v1/profiles/profile-123/tasks/image", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify API response
	assert.Equal(t, http.StatusAccepted, w.Code)

	var response handlers.APISuccessResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "v1", response.Version)
	assert.Equal(t, "image-worker", response.Metadata["worker_type"])

	// Extract image response data
	// ✅ FIX: Handle JSON unmarshaling - Data is map[string]interface{} after unmarshaling
	dataMap, ok := response.Data.(map[string]interface{})
	require.True(t, ok, "Response data should be a map")

	// Convert map to ImageTaskResponse for validation
	imageData := &models.ImageTaskResponse{
		TaskID:     dataMap["task_id"].(string),
		Type:       dataMap["type"].(string),
		ImageURL:   dataMap["image_url"].(string),
		Operation:  dataMap["operation"].(string),
		RoutingKey: dataMap["routing_key"].(string),
	}

	assert.Equal(t, "image_processing", imageData.Type)
	assert.Equal(t, "https://example.com/user-avatar.jpg", imageData.ImageURL)
	assert.Equal(t, "resize", imageData.Operation)
	assert.Equal(t, "image.process", imageData.RoutingKey)

	// Verify queue service received the message
	messages := mockQueue.GetReceivedMessages()
	require.Len(t, messages, 1)

	msg := messages[0]
	assert.Equal(t, "image_processing", msg.Type)
	assert.Equal(t, "image.process", msg.RoutingKey)

	// Verify image payload content
	var payloadData map[string]interface{}
	err = json.Unmarshal(msg.Payload, &payloadData)
	require.NoError(t, err)

	payload_inner, exists := payloadData["payload"]
	require.True(t, exists)

	payloadMap, ok := payload_inner.(map[string]interface{})
	require.True(t, ok)

	assert.Equal(t, "https://example.com/user-avatar.jpg", payloadMap["image_url"])
	assert.Equal(t, "resize", payloadMap["operation"])
	assert.Equal(t, "webp", payloadMap["output_format"])

	t.Logf("✅ Image task end-to-end test completed successfully")
}

// TestRoutingKeyDistributionAccuracy tests that routing keys are correctly distributed
func TestRoutingKeyDistributionAccuracy(t *testing.T) {
	router, mockQueue, _ := setupTestEnvironment(t)
	defer mockQueue.Close()

	// Test cases for different task types
	testCases := []struct {
		taskType        string
		endpoint        string
		expectedRouting string
		expectedWorker  string
		payload         interface{}
	}{
		{
			taskType:        "profile_update",
			endpoint:        "/api/v1/profiles/test-123/tasks",
			expectedRouting: "profile.task",
			expectedWorker:  "profile-worker",
			payload: map[string]interface{}{
				"type": "profile_update",
				"payload": map[string]interface{}{
					"user_id": "user-123",
					"action":  "update",
				},
			},
		},
		{
			taskType:        "email_notification",
			endpoint:        "/api/v1/profiles/test-123/tasks",
			expectedRouting: "email.send",
			expectedWorker:  "email-worker",
			payload: map[string]interface{}{
				"type": "email_notification",
				"payload": map[string]interface{}{
					"to":       "test@example.com",
					"template": "test",
				},
			},
		},
		{
			taskType:        "image_processing",
			endpoint:        "/api/v1/profiles/test-123/tasks",
			expectedRouting: "image.process",
			expectedWorker:  "image-worker",
			payload: map[string]interface{}{
				"type": "image_processing",
				"payload": map[string]interface{}{
					"image_url": "https://example.com/test.jpg",
					"operation": "resize",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("Routing_%s", tc.taskType), func(t *testing.T) {
			mockQueue.Reset()

			// Make request
			body, _ := json.Marshal(tc.payload)
			req, _ := http.NewRequest("POST", tc.endpoint, bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Verify response
			assert.Equal(t, http.StatusAccepted, w.Code)

			// Verify routing key distribution
			messages := mockQueue.GetReceivedMessages()
			require.Len(t, messages, 1)

			msg := messages[0]
			assert.Equal(t, tc.taskType, msg.Type)
			assert.Equal(t, tc.expectedRouting, msg.RoutingKey)
			assert.Equal(t, tc.expectedRouting, msg.Headers["X-Routing-Key"])

			t.Logf("✅ Routing verified: %s → %s → %s", tc.taskType, tc.expectedRouting, tc.expectedWorker)
		})
	}
}

// TestMessageFormatCompatibility verifies message format compatibility end-to-end
func TestMessageFormatCompatibility(t *testing.T) {
	router, mockQueue, _ := setupTestEnvironment(t)
	defer mockQueue.Close()

	// Test with comprehensive payload
	payload := map[string]interface{}{
		"type": "profile_update",
		"payload": map[string]interface{}{
			"user_id": "user-123",
			"action":  "update",
			"data": map[string]interface{}{
				"name":  "John Doe",
				"email": "john@example.com",
				"nested": map[string]interface{}{
					"field": "value",
					"array": []string{"item1", "item2"},
				},
			},
		},
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/api/v1/profiles/profile-123/tasks", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)

	// Verify complete message format
	messages := mockQueue.GetReceivedMessages()
	require.Len(t, messages, 1)

	msg := messages[0]

	// Verify required fields are present
	assert.NotEmpty(t, msg.ID, "Message ID should be generated")
	assert.Equal(t, "profile_update", msg.Type)
	assert.False(t, msg.Timestamp.IsZero(), "Timestamp should be set")
	assert.NotNil(t, msg.Metadata, "Metadata should be present")
	assert.Equal(t, "profile.task", msg.RoutingKey)

	// Verify metadata content
	assert.Equal(t, "profile-service", msg.Metadata["client"])
	assert.Equal(t, "profile-123", msg.Metadata["profile_id"])
	assert.NotEmpty(t, msg.Metadata["attempt"])
	assert.NotEmpty(t, msg.Metadata["max_attempts"])

	// Verify payload is json.RawMessage and can be unmarshaled
	var payloadData map[string]interface{}
	err := json.Unmarshal(msg.Payload, &payloadData)
	require.NoError(t, err, "Payload should be valid JSON")

	// Verify payload structure
	assert.NotEmpty(t, payloadData["task_id"])
	assert.Equal(t, "profile-123", payloadData["profile_id"])
	assert.Equal(t, "profile_update", payloadData["task_type"])
	assert.NotNil(t, payloadData["payload"])

	t.Logf("✅ Message format compatibility verified end-to-end")
}

// TestErrorHandlingScenarios tests various error scenarios
func TestErrorHandlingScenarios(t *testing.T) {
	router, mockQueue, _ := setupTestEnvironment(t)
	defer mockQueue.Close()

	t.Run("InvalidTaskType", func(t *testing.T) {
		payload := map[string]interface{}{
			"type": "invalid_task_type",
			"payload": map[string]interface{}{
				"data": "test",
			},
		}

		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", "/api/v1/profiles/profile-123/tasks", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

		var errorResponse handlers.APIErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &errorResponse)
		require.NoError(t, err)

		assert.Equal(t, "VALIDATION_ERROR", errorResponse.Code)
		assert.Equal(t, "v1", errorResponse.Version)
		assert.Equal(t, "invalid_task_type", errorResponse.TaskType)
		assert.NotNil(t, errorResponse.Details)

		// Verify no message was sent to queue
		messages := mockQueue.GetReceivedMessages()
		assert.Len(t, messages, 0)
	})

	t.Run("QueueServiceUnavailable", func(t *testing.T) {
		// Set queue service to return 503
		mockQueue.SetResponseCode(503)

		payload := map[string]interface{}{
			"type": "profile_update",
			"payload": map[string]interface{}{
				"user_id": "user-123",
				"action":  "update",
			},
		}

		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", "/api/v1/profiles/profile-123/tasks", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should still receive request but get error response
		assert.Equal(t, http.StatusServiceUnavailable, w.Code)

		var errorResponse handlers.APIErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &errorResponse)
		require.NoError(t, err)

		assert.Contains(t, errorResponse.Code, "SERVICE_UNAVAILABLE")
		assert.Equal(t, "v1", errorResponse.Version)

		// Reset for other tests
		mockQueue.SetResponseCode(200)
	})

	t.Run("InvalidJSONPayload", func(t *testing.T) {
		// Send invalid JSON
		req, _ := http.NewRequest("POST", "/api/v1/profiles/profile-123/tasks", bytes.NewBufferString("{invalid json"))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var errorResponse handlers.APIErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &errorResponse)
		require.NoError(t, err)

		assert.Equal(t, "INVALID_JSON", errorResponse.Code)
		assert.Equal(t, "v1", errorResponse.Version)
	})

	t.Logf("✅ Error handling scenarios tested successfully")
}

// TestHighVolumeTaskSubmission tests handling of multiple concurrent requests
func TestHighVolumeTaskSubmission(t *testing.T) {
	router, mockQueue, _ := setupTestEnvironment(t)
	defer mockQueue.Close()

	// Number of concurrent requests to test
	numRequests := 50
	done := make(chan bool, numRequests)

	// Submit multiple requests concurrently
	for i := 0; i < numRequests; i++ {
		go func(requestID int) {
			payload := map[string]interface{}{
				"type": "profile_update",
				"payload": map[string]interface{}{
					"user_id": fmt.Sprintf("user-%d", requestID),
					"action":  "update",
				},
			}

			body, _ := json.Marshal(payload)
			req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/profiles/profile-%d/tasks", requestID), bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Verify successful response
			if w.Code != http.StatusAccepted {
				t.Errorf("Request %d failed with status %d", requestID, w.Code)
			}

			done <- true
		}(i)
	}

	// Wait for all requests to complete
	for i := 0; i < numRequests; i++ {
		<-done
	}

	// Verify all messages were received
	messages := mockQueue.GetReceivedMessages()
	assert.Len(t, messages, numRequests, "All messages should be received by queue service")

	// Verify message uniqueness
	messageIDs := make(map[string]bool)
	for _, msg := range messages {
		assert.False(t, messageIDs[msg.ID], "Message IDs should be unique")
		messageIDs[msg.ID] = true
		assert.Equal(t, "profile_update", msg.Type)
		assert.Equal(t, "profile.task", msg.RoutingKey)
	}

	t.Logf("✅ High volume task submission test completed: %d requests processed", numRequests)
}
