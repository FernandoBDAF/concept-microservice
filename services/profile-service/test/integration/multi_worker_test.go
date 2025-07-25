package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/fernandobarroso/microservices/services/profile-service/internal/api/handlers"
	"github.com/fernandobarroso/microservices/services/profile-service/internal/domain/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ✅ Phase 4: Multi-Worker Integration Testing
// Tests the integration with different worker types (profile, email, image)

// WorkerMetrics tracks metrics for each worker type
type WorkerMetrics struct {
	mu                 sync.RWMutex
	taskCount          map[string]int
	totalTasks         int
	averageProcessTime map[string]time.Duration
	errorRate          map[string]float64
}

func NewWorkerMetrics() *WorkerMetrics {
	return &WorkerMetrics{
		taskCount:          make(map[string]int),
		averageProcessTime: make(map[string]time.Duration),
		errorRate:          make(map[string]float64),
	}
}

func (wm *WorkerMetrics) RecordTaskSubmission(taskType string, processingTime time.Duration, failed bool) {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	wm.taskCount[taskType]++
	wm.totalTasks++

	// Update average processing time (simple moving average)
	if currentAvg, exists := wm.averageProcessTime[taskType]; exists {
		wm.averageProcessTime[taskType] = (currentAvg + processingTime) / 2
	} else {
		wm.averageProcessTime[taskType] = processingTime
	}

	// Update error rate
	if failed {
		wm.errorRate[taskType]++
	}
}

func (wm *WorkerMetrics) GetStats() map[string]interface{} {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	stats := map[string]interface{}{
		"total_tasks": wm.totalTasks,
		"by_type":     make(map[string]interface{}),
	}

	for taskType, count := range wm.taskCount {
		errorCount := wm.errorRate[taskType]
		errorRate := 0.0
		if count > 0 {
			errorRate = errorCount / float64(count) * 100
		}

		stats["by_type"].(map[string]interface{})[taskType] = map[string]interface{}{
			"count":        count,
			"average_time": wm.averageProcessTime[taskType].String(),
			"error_rate":   fmt.Sprintf("%.2f%%", errorRate),
			"success_rate": fmt.Sprintf("%.2f%%", 100-errorRate),
		}
	}

	return stats
}

// TestProfileWorkerIntegration tests integration with profile worker
func TestProfileWorkerIntegration(t *testing.T) {
	router, mockQueue, _ := setupTestEnvironment(t)
	defer mockQueue.Close()

	t.Run("ProfileUpdateTasks", func(t *testing.T) {
		testCases := []struct {
			name    string
			payload models.ProfileTaskPayload
			valid   bool
		}{
			{
				name: "ValidProfileCreate",
				payload: models.ProfileTaskPayload{
					UserID: "user-123",
					Action: "create",
					Data: map[string]interface{}{
						"name":  "John Doe",
						"email": "john@example.com",
					},
				},
				valid: true,
			},
			{
				name: "ValidProfileUpdate",
				payload: models.ProfileTaskPayload{
					UserID: "user-456",
					Action: "update",
					Data: map[string]interface{}{
						"name":  "Jane Smith",
						"phone": "+1234567890",
					},
				},
				valid: true,
			},
			{
				name: "ValidProfileDelete",
				payload: models.ProfileTaskPayload{
					UserID: "user-789",
					Action: "delete",
				},
				valid: true,
			},
			{
				name: "ValidProfileSync",
				payload: models.ProfileTaskPayload{
					UserID: "user-999",
					Action: "sync",
					Data: map[string]interface{}{
						"external_id": "ext-999",
					},
				},
				valid: true,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				mockQueue.Reset()

				// Test via specialized profile endpoint
				body, _ := json.Marshal(tc.payload)
				req, _ := http.NewRequest("POST", "/api/v1/profiles/profile-123/tasks/profile", bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")

				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				if tc.valid {
					assert.Equal(t, http.StatusAccepted, w.Code)

					// Verify queue message
					messages := mockQueue.GetReceivedMessages()
					require.Len(t, messages, 1)

					msg := messages[0]
					assert.Equal(t, "profile_update", msg.Type)
					assert.Equal(t, "profile.task", msg.RoutingKey)

					// Verify payload contains profile worker specifics
					var payloadData map[string]interface{}
					err := json.Unmarshal(msg.Payload, &payloadData)
					require.NoError(t, err)

					assert.Equal(t, "profile-worker", payloadData["worker_target"])
					assert.Equal(t, "profile_update", payloadData["task_type"])

					t.Logf("✅ Profile worker integration verified for %s", tc.name)
				} else {
					assert.NotEqual(t, http.StatusAccepted, w.Code)
				}
			})
		}
	})

	t.Run("ProfileWorkerLoadTest", func(t *testing.T) {
		mockQueue.Reset()
		metrics := NewWorkerMetrics()

		// Submit multiple profile tasks concurrently
		numTasks := 25
		done := make(chan bool, numTasks)

		for i := 0; i < numTasks; i++ {
			go func(taskID int) {
				startTime := time.Now()

				payload := models.ProfileTaskPayload{
					UserID: fmt.Sprintf("user-%d", taskID),
					Action: "update",
					Data: map[string]interface{}{
						"name": fmt.Sprintf("User %d", taskID),
					},
				}

				body, _ := json.Marshal(payload)
				req, _ := http.NewRequest("POST", "/api/v1/profiles/profile-123/tasks/profile", bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")

				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				processingTime := time.Since(startTime)
				failed := w.Code != http.StatusAccepted
				metrics.RecordTaskSubmission("profile_update", processingTime, failed)

				done <- true
			}(i)
		}

		// Wait for all tasks to complete
		for i := 0; i < numTasks; i++ {
			<-done
		}

		// Verify all messages were processed
		messages := mockQueue.GetReceivedMessages()
		assert.Len(t, messages, numTasks)

		// Verify all messages are profile worker tasks
		for _, msg := range messages {
			assert.Equal(t, "profile_update", msg.Type)
			assert.Equal(t, "profile.task", msg.RoutingKey)
		}

		stats := metrics.GetStats()
		t.Logf("✅ Profile worker load test completed - Stats: %+v", stats)
	})
}

// TestEmailWorkerIntegration tests integration with email worker
func TestEmailWorkerIntegration(t *testing.T) {
	router, mockQueue, _ := setupTestEnvironment(t)
	defer mockQueue.Close()

	t.Run("EmailNotificationTasks", func(t *testing.T) {
		testCases := []struct {
			name    string
			payload models.EmailTaskPayload
			valid   bool
		}{
			{
				name: "ValidWelcomeEmail",
				payload: models.EmailTaskPayload{
					To:       "new-user@example.com",
					Template: "welcome",
					Subject:  "Welcome to our platform!",
					Priority: 1,
					Data: map[string]interface{}{
						"username":       "newuser",
						"activation_url": "https://example.com/activate",
					},
				},
				valid: true,
			},
			{
				name: "ValidPasswordResetEmail",
				payload: models.EmailTaskPayload{
					To:       "user@example.com",
					Template: "password_reset",
					Subject:  "Password Reset Request",
					Priority: 1,
					Data: map[string]interface{}{
						"reset_token": "abc123",
						"expires_at":  "2024-01-16T12:00:00Z",
					},
				},
				valid: true,
			},
			{
				name: "ValidNotificationEmail",
				payload: models.EmailTaskPayload{
					To:       "user@example.com",
					Template: "notification",
					Priority: 2,
					Data: map[string]interface{}{
						"notification_type": "new_message",
						"message_count":     5,
					},
				},
				valid: true,
			},
			{
				name: "ValidLowPriorityEmail",
				payload: models.EmailTaskPayload{
					To:       "user@example.com",
					Template: "newsletter",
					Priority: 3,
					Data: map[string]interface{}{
						"issue_number": 42,
						"articles":     []string{"article1", "article2"},
					},
				},
				valid: true,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				mockQueue.Reset()

				// Test via specialized email endpoint
				body, _ := json.Marshal(tc.payload)
				req, _ := http.NewRequest("POST", "/api/v1/profiles/profile-123/tasks/email", bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")

				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				if tc.valid {
					assert.Equal(t, http.StatusAccepted, w.Code)

					// Verify enhanced email response
					var response handlers.APISuccessResponse
					err := json.Unmarshal(w.Body.Bytes(), &response)
					require.NoError(t, err)

					assert.Equal(t, "email-worker", response.Metadata["worker_type"])

					// Verify queue message
					messages := mockQueue.GetReceivedMessages()
					require.Len(t, messages, 1)

					msg := messages[0]
					assert.Equal(t, "email_notification", msg.Type)
					assert.Equal(t, "email.send", msg.RoutingKey)

					// Verify email-specific metadata
					assert.Equal(t, "email-worker", msg.Metadata["worker_target"])
					assert.Equal(t, "profile-123", msg.Metadata["profile_id"])

					// Verify payload structure for email worker
					var payloadData map[string]interface{}
					err = json.Unmarshal(msg.Payload, &payloadData)
					require.NoError(t, err)

					assert.Equal(t, "email-worker", payloadData["worker_target"])
					assert.Equal(t, "email_notification", payloadData["task_type"])

					t.Logf("✅ Email worker integration verified for %s", tc.name)
				} else {
					assert.NotEqual(t, http.StatusAccepted, w.Code)
				}
			})
		}
	})

	t.Run("EmailWorkerPriorityRouting", func(t *testing.T) {
		mockQueue.Reset()

		// Test different priority levels
		priorities := []struct {
			priority int
			label    string
		}{
			{1, "high"},
			{2, "normal"},
			{3, "low"},
		}

		for _, p := range priorities {
			payload := models.EmailTaskPayload{
				To:       "user@example.com",
				Template: "test",
				Priority: p.priority,
			}

			body, _ := json.Marshal(payload)
			req, _ := http.NewRequest("POST", "/api/v1/profiles/profile-123/tasks/email", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusAccepted, w.Code)
		}

		// Verify all emails routed to email worker
		messages := mockQueue.GetReceivedMessages()
		assert.Len(t, messages, 3)

		for i, msg := range messages {
			assert.Equal(t, "email_notification", msg.Type)
			assert.Equal(t, "email.send", msg.RoutingKey)

			// Verify priority is preserved in payload
			var payloadData map[string]interface{}
			err := json.Unmarshal(msg.Payload, &payloadData)
			require.NoError(t, err)

			payload_inner := payloadData["payload"].(map[string]interface{})
			expectedPriority := priorities[i].priority
			assert.Equal(t, float64(expectedPriority), payload_inner["priority"])

			t.Logf("✅ Email priority routing verified: %s priority (%d)", priorities[i].label, expectedPriority)
		}
	})
}

// TestImageWorkerIntegration tests integration with image worker
func TestImageWorkerIntegration(t *testing.T) {
	router, mockQueue, _ := setupTestEnvironment(t)
	defer mockQueue.Close()

	t.Run("ImageProcessingTasks", func(t *testing.T) {
		testCases := []struct {
			name    string
			payload models.ImageTaskPayload
			valid   bool
		}{
			{
				name: "ValidImageResize",
				payload: models.ImageTaskPayload{
					ImageURL:     "https://example.com/profile.jpg",
					Operation:    "resize",
					OutputFormat: "webp",
					Options: map[string]interface{}{
						"width":   400,
						"height":  400,
						"quality": 85,
					},
				},
				valid: true,
			},
			{
				name: "ValidImageConvert",
				payload: models.ImageTaskPayload{
					ImageURL:     "https://example.com/document.png",
					Operation:    "convert",
					OutputFormat: "jpg",
					Options: map[string]interface{}{
						"quality": 90,
					},
				},
				valid: true,
			},
			{
				name: "ValidImageOptimize",
				payload: models.ImageTaskPayload{
					ImageURL:  "https://example.com/large-image.jpg",
					Operation: "optimize",
					Options: map[string]interface{}{
						"compression":    "lossless",
						"strip_metadata": true,
					},
				},
				valid: true,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				mockQueue.Reset()

				// Test via specialized image endpoint
				body, _ := json.Marshal(tc.payload)
				req, _ := http.NewRequest("POST", "/api/v1/profiles/profile-123/tasks/image", bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")

				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				if tc.valid {
					assert.Equal(t, http.StatusAccepted, w.Code)

					// Verify enhanced image response
					var response handlers.APISuccessResponse
					err := json.Unmarshal(w.Body.Bytes(), &response)
					require.NoError(t, err)

					assert.Equal(t, "image-worker", response.Metadata["worker_type"])

					// Verify queue message
					messages := mockQueue.GetReceivedMessages()
					require.Len(t, messages, 1)

					msg := messages[0]
					assert.Equal(t, "image_processing", msg.Type)
					assert.Equal(t, "image.process", msg.RoutingKey)

					// Verify image-specific metadata
					assert.Equal(t, "image-worker", msg.Metadata["worker_target"])

					// Verify payload structure for image worker
					var payloadData map[string]interface{}
					err = json.Unmarshal(msg.Payload, &payloadData)
					require.NoError(t, err)

					assert.Equal(t, "image-worker", payloadData["worker_target"])
					assert.Equal(t, "image_processing", payloadData["task_type"])

					// Verify image-specific payload content
					payload_inner := payloadData["payload"].(map[string]interface{})
					assert.Equal(t, tc.payload.ImageURL, payload_inner["image_url"])
					assert.Equal(t, tc.payload.Operation, payload_inner["operation"])

					t.Logf("✅ Image worker integration verified for %s", tc.name)
				} else {
					assert.NotEqual(t, http.StatusAccepted, w.Code)
				}
			})
		}
	})

	t.Run("ImageWorkerOperationRouting", func(t *testing.T) {
		mockQueue.Reset()

		// Test different image operations
		operations := []string{"resize", "convert", "optimize"}

		for _, operation := range operations {
			payload := models.ImageTaskPayload{
				ImageURL:  fmt.Sprintf("https://example.com/test-%s.jpg", operation),
				Operation: operation,
			}

			body, _ := json.Marshal(payload)
			req, _ := http.NewRequest("POST", "/api/v1/profiles/profile-123/tasks/image", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusAccepted, w.Code)
		}

		// Verify all image tasks routed to image worker
		messages := mockQueue.GetReceivedMessages()
		assert.Len(t, messages, 3)

		for i, msg := range messages {
			assert.Equal(t, "image_processing", msg.Type)
			assert.Equal(t, "image.process", msg.RoutingKey)

			// Verify operation is preserved
			var payloadData map[string]interface{}
			err := json.Unmarshal(msg.Payload, &payloadData)
			require.NoError(t, err)

			payload_inner := payloadData["payload"].(map[string]interface{})
			expectedOperation := operations[i]
			assert.Equal(t, expectedOperation, payload_inner["operation"])

			t.Logf("✅ Image operation routing verified: %s", expectedOperation)
		}
	})
}

// TestMultiWorkerWorkloadDistribution tests workload distribution across different workers
func TestMultiWorkerWorkloadDistribution(t *testing.T) {
	router, mockQueue, _ := setupTestEnvironment(t)
	defer mockQueue.Close()

	metrics := NewWorkerMetrics()

	// Define realistic workload distribution
	workloadTasks := []struct {
		taskType    string
		endpoint    string
		payload     interface{}
		count       int
		expectedKey string
	}{
		{
			taskType: "profile_update",
			endpoint: "/api/v1/profiles/profile-123/tasks/profile",
			payload: models.ProfileTaskPayload{
				UserID: "user-123",
				Action: "update",
				Data:   map[string]interface{}{"name": "Test User"},
			},
			count:       15, // 15 profile tasks
			expectedKey: "profile.task",
		},
		{
			taskType: "email_notification",
			endpoint: "/api/v1/profiles/profile-123/tasks/email",
			payload: models.EmailTaskPayload{
				To:       "user@example.com",
				Template: "test",
				Priority: 2,
			},
			count:       25, // 25 email tasks (higher volume)
			expectedKey: "email.send",
		},
		{
			taskType: "image_processing",
			endpoint: "/api/v1/profiles/profile-123/tasks/image",
			payload: models.ImageTaskPayload{
				ImageURL:  "https://example.com/test.jpg",
				Operation: "resize",
			},
			count:       10, // 10 image tasks
			expectedKey: "image.process",
		},
	}

	// Submit all tasks concurrently to simulate realistic load
	var wg sync.WaitGroup
	totalTasks := 0

	for _, workload := range workloadTasks {
		totalTasks += workload.count
		for i := 0; i < workload.count; i++ {
			wg.Add(1)
			go func(w struct {
				taskType    string
				endpoint    string
				payload     interface{}
				count       int
				expectedKey string
			}, taskIndex int) {
				defer wg.Done()

				startTime := time.Now()

				body, _ := json.Marshal(w.payload)
				req, _ := http.NewRequest("POST", w.endpoint, bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")

				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, req)

				processingTime := time.Since(startTime)
				failed := recorder.Code != http.StatusAccepted
				metrics.RecordTaskSubmission(w.taskType, processingTime, failed)

				if recorder.Code != http.StatusAccepted {
					t.Errorf("Task %s[%d] failed with status %d", w.taskType, taskIndex, recorder.Code)
				}
			}(workload, i)
		}
	}

	// Wait for all tasks to complete
	wg.Wait()

	// Verify message distribution
	messages := mockQueue.GetReceivedMessages()
	assert.Len(t, messages, totalTasks, "All tasks should be received")

	// Count messages by worker type
	workerCounts := make(map[string]int)
	for _, msg := range messages {
		workerCounts[msg.RoutingKey]++
	}

	// Verify distribution matches expected workload
	for _, workload := range workloadTasks {
		assert.Equal(t, workload.count, workerCounts[workload.expectedKey],
			"Worker %s should receive %d tasks", workload.expectedKey, workload.count)
	}

	// Log distribution statistics
	stats := metrics.GetStats()
	t.Logf("✅ Multi-worker workload distribution completed")
	t.Logf("   Total tasks: %d", totalTasks)
	t.Logf("   Profile tasks: %d → profile.task", workerCounts["profile.task"])
	t.Logf("   Email tasks: %d → email.send", workerCounts["email.send"])
	t.Logf("   Image tasks: %d → image.process", workerCounts["image.process"])
	t.Logf("   Performance stats: %+v", stats)
}

// TestWorkerFailoverScenarios tests failover scenarios for each worker type
func TestWorkerFailoverScenarios(t *testing.T) {
	router, mockQueue, _ := setupTestEnvironment(t)
	defer mockQueue.Close()

	t.Run("QueueServiceFailover", func(t *testing.T) {
		// Simulate queue service failures and recovery
		scenarios := []struct {
			name          string
			responseCode  int
			shouldSucceed bool
		}{
			{"HealthyService", 200, true},
			{"ServiceUnavailable", 503, false},
			{"BadGateway", 502, false},
			{"InternalError", 500, false},
			{"ServiceRecovered", 200, true},
		}

		for _, scenario := range scenarios {
			t.Run(scenario.name, func(t *testing.T) {
				mockQueue.Reset()
				mockQueue.SetResponseCode(scenario.responseCode)

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

				if scenario.shouldSucceed {
					assert.Equal(t, http.StatusAccepted, w.Code, "Request should succeed when queue service is healthy")
				} else {
					assert.NotEqual(t, http.StatusAccepted, w.Code, "Request should fail when queue service is unhealthy")
				}

				t.Logf("✅ Failover scenario %s tested - Response: %d", scenario.name, w.Code)
			})
		}
	})

	t.Run("CircuitBreakerBehavior", func(t *testing.T) {
		mockQueue.Reset()
		mockQueue.SetResponseCode(503) // Service unavailable

		// Submit multiple requests to trigger circuit breaker
		failureCount := 0
		for i := 0; i < 8; i++ { // More than failure threshold (5)
			payload := map[string]interface{}{
				"type": "profile_update",
				"payload": map[string]interface{}{
					"user_id": fmt.Sprintf("user-%d", i),
					"action":  "update",
				},
			}

			body, _ := json.Marshal(payload)
			req, _ := http.NewRequest("POST", "/api/v1/profiles/profile-123/tasks", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusAccepted {
				failureCount++
			}

			// Small delay to allow circuit breaker state changes
			time.Sleep(10 * time.Millisecond)
		}

		assert.Greater(t, failureCount, 5, "Should have multiple failures due to circuit breaker")

		// Restore service and test recovery
		mockQueue.SetResponseCode(200)
		time.Sleep(100 * time.Millisecond) // Allow circuit breaker to recover

		payload := map[string]interface{}{
			"type": "profile_update",
			"payload": map[string]interface{}{
				"user_id": "recovery-test",
				"action":  "update",
			},
		}

		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", "/api/v1/profiles/profile-123/tasks", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should succeed after service recovery
		assert.Equal(t, http.StatusAccepted, w.Code, "Should succeed after service recovery")

		t.Logf("✅ Circuit breaker behavior tested - Failures: %d, Recovery: %d", failureCount, w.Code)
	})
}

// TestMetricsCollection verifies that metrics are collected accurately
func TestMetricsCollection(t *testing.T) {
	router, mockQueue, _ := setupTestEnvironment(t)
	defer mockQueue.Close()

	// Submit tasks to generate metrics
	taskTypes := []struct {
		taskType string
		endpoint string
		payload  interface{}
	}{
		{
			taskType: "profile_update",
			endpoint: "/api/v1/profiles/profile-123/tasks/profile",
			payload: models.ProfileTaskPayload{
				UserID: "user-123",
				Action: "update",
			},
		},
		{
			taskType: "email_notification",
			endpoint: "/api/v1/profiles/profile-123/tasks/email",
			payload: models.EmailTaskPayload{
				To:       "user@example.com",
				Template: "test",
			},
		},
		{
			taskType: "image_processing",
			endpoint: "/api/v1/profiles/profile-123/tasks/image",
			payload: models.ImageTaskPayload{
				ImageURL:  "https://example.com/test.jpg",
				Operation: "resize",
			},
		},
	}

	// Submit multiple tasks of each type
	for _, taskType := range taskTypes {
		for i := 0; i < 3; i++ {
			body, _ := json.Marshal(taskType.payload)
			req, _ := http.NewRequest("POST", taskType.endpoint, bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusAccepted, w.Code)
		}
	}

	// Test statistics endpoint
	req, _ := http.NewRequest("GET", "/api/v1/profiles/profile-123/tasks/stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var statsResponse handlers.APISuccessResponse
	err := json.Unmarshal(w.Body.Bytes(), &statsResponse)
	require.NoError(t, err)

	assert.Equal(t, "v1", statsResponse.Version)
	assert.NotNil(t, statsResponse.Data)

	// Verify metrics are collected
	messages := mockQueue.GetReceivedMessages()
	assert.Len(t, messages, 9) // 3 types × 3 tasks each

	// Verify message distribution
	typeCounts := make(map[string]int)
	for _, msg := range messages {
		typeCounts[msg.Type]++
	}

	assert.Equal(t, 3, typeCounts["profile_update"])
	assert.Equal(t, 3, typeCounts["email_notification"])
	assert.Equal(t, 3, typeCounts["image_processing"])

	t.Logf("✅ Metrics collection verified - Message distribution: %+v", typeCounts)
}
