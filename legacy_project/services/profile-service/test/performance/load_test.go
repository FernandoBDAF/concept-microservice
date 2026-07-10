package performance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fernandobarroso/microservices/services/profile-service/internal/api/handlers"
	"github.com/fernandobarroso/microservices/services/profile-service/internal/config"
	"github.com/fernandobarroso/microservices/services/profile-service/internal/domain/models"
	"github.com/fernandobarroso/microservices/services/profile-service/internal/domain/services"
	"github.com/fernandobarroso/microservices/services/profile-service/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// ✅ Phase 4 Task 4.2: Performance Testing & Optimization
// Tests the performance targets specified in TRACKER.md

// Performance targets from Phase 4 requirements:
// - API Response Time: < 50ms for task submission acceptance
// - Message Publishing: < 100ms for queue-service communication
// - Error Rate: < 1% for all task submissions
// - Throughput: Support 1000+ tasks/second submission rate

// PerformanceMetrics tracks comprehensive performance metrics
type PerformanceMetrics struct {
	mu                   sync.RWMutex
	totalRequests        int64
	successfulRequests   int64
	failedRequests       int64
	totalResponseTime    time.Duration
	minResponseTime      time.Duration
	maxResponseTime      time.Duration
	responseTimeBuckets  map[string]int64 // For percentile calculations
	requestsPerSecond    float64
	startTime            time.Time
	endTime              time.Time
	concurrentUsers      int
	taskTypeDistribution map[string]int64
}

func NewPerformanceMetrics() *PerformanceMetrics {
	return &PerformanceMetrics{
		responseTimeBuckets:  make(map[string]int64),
		taskTypeDistribution: make(map[string]int64),
		minResponseTime:      time.Hour, // Initialize to high value
		startTime:            time.Now(),
	}
}

func (pm *PerformanceMetrics) RecordRequest(responseTime time.Duration, success bool, taskType string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.totalRequests++
	pm.totalResponseTime += responseTime
	pm.taskTypeDistribution[taskType]++

	if success {
		pm.successfulRequests++
	} else {
		pm.failedRequests++
	}

	// Track min/max response times
	if responseTime < pm.minResponseTime {
		pm.minResponseTime = responseTime
	}
	if responseTime > pm.maxResponseTime {
		pm.maxResponseTime = responseTime
	}

	// Bucket response times for percentile calculation
	bucket := pm.getResponseTimeBucket(responseTime)
	pm.responseTimeBuckets[bucket]++
}

func (pm *PerformanceMetrics) getResponseTimeBucket(responseTime time.Duration) string {
	ms := responseTime.Milliseconds()
	switch {
	case ms < 10:
		return "0-10ms"
	case ms < 25:
		return "10-25ms"
	case ms < 50:
		return "25-50ms"
	case ms < 100:
		return "50-100ms"
	case ms < 200:
		return "100-200ms"
	case ms < 500:
		return "200-500ms"
	default:
		return "500ms+"
	}
}

func (pm *PerformanceMetrics) FinalizeTiming() {
	pm.endTime = time.Now()
	duration := pm.endTime.Sub(pm.startTime).Seconds()
	if duration > 0 {
		pm.requestsPerSecond = float64(pm.totalRequests) / duration
	}
}

func (pm *PerformanceMetrics) GetSummary() map[string]interface{} {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	avgResponseTime := time.Duration(0)
	if pm.totalRequests > 0 {
		avgResponseTime = pm.totalResponseTime / time.Duration(pm.totalRequests)
	}

	errorRate := 0.0
	if pm.totalRequests > 0 {
		errorRate = float64(pm.failedRequests) / float64(pm.totalRequests) * 100
	}

	successRate := 100.0 - errorRate

	return map[string]interface{}{
		"total_requests":      pm.totalRequests,
		"successful_requests": pm.successfulRequests,
		"failed_requests":     pm.failedRequests,
		"success_rate":        fmt.Sprintf("%.2f%%", successRate),
		"error_rate":          fmt.Sprintf("%.2f%%", errorRate),
		"avg_response_time":   avgResponseTime.String(),
		"min_response_time":   pm.minResponseTime.String(),
		"max_response_time":   pm.maxResponseTime.String(),
		"requests_per_second": fmt.Sprintf("%.2f", pm.requestsPerSecond),
		"concurrent_users":    pm.concurrentUsers,
		"test_duration":       pm.endTime.Sub(pm.startTime).String(),
		"response_buckets":    pm.responseTimeBuckets,
		"task_distribution":   pm.taskTypeDistribution,
	}
}

// MockPerformanceQueueService optimized for performance testing
type MockPerformanceQueueService struct {
	server                *httptest.Server
	receivedCount         int64
	responseDelay         time.Duration
	successRate           float64
	maxConcurrentRequests int64
	currentRequests       int64
}

func NewMockPerformanceQueueService() *MockPerformanceQueueService {
	mock := &MockPerformanceQueueService{
		successRate:           100.0, // Default 100% success
		maxConcurrentRequests: 1000,
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Track concurrent requests
		current := atomic.AddInt64(&mock.currentRequests, 1)
		defer atomic.AddInt64(&mock.currentRequests, -1)

		// Simulate processing delay
		if mock.responseDelay > 0 {
			time.Sleep(mock.responseDelay)
		}

		// Simulate failure rate
		atomic.AddInt64(&mock.receivedCount, 1)
		if current > mock.maxConcurrentRequests {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}

		// Simulate random failures based on success rate
		if mock.successRate < 100.0 {
			requestCount := atomic.LoadInt64(&mock.receivedCount)
			failureThreshold := (100.0 - mock.successRate) / 100.0
			if float64(requestCount%100)/100.0 < failureThreshold {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}

		w.WriteHeader(http.StatusOK)
		response := map[string]interface{}{
			"message": "OK",
			"id":      fmt.Sprintf("msg-%d", atomic.LoadInt64(&mock.receivedCount)),
		}
		json.NewEncoder(w).Encode(response)
	})

	mock.server = httptest.NewServer(handler)
	return mock
}

func (m *MockPerformanceQueueService) Close() {
	m.server.Close()
}

func (m *MockPerformanceQueueService) URL() string {
	return m.server.URL
}

func (m *MockPerformanceQueueService) SetResponseDelay(delay time.Duration) {
	m.responseDelay = delay
}

func (m *MockPerformanceQueueService) SetSuccessRate(rate float64) {
	m.successRate = rate
}

func (m *MockPerformanceQueueService) GetReceivedCount() int64 {
	return atomic.LoadInt64(&m.receivedCount)
}

func (m *MockPerformanceQueueService) Reset() {
	atomic.StoreInt64(&m.receivedCount, 0)
	atomic.StoreInt64(&m.currentRequests, 0)
}

// setupPerformanceTestEnvironment creates optimized test environment for performance testing
func setupPerformanceTestEnvironment(t *testing.T) (*gin.Engine, *MockPerformanceQueueService) {
	// ✅ Initialize logger for tests
	logger.Initialize(&logger.Config{
		Level:       "error", // Minimal logging for performance
		Environment: "test",
		ServiceName: "profile-service-perf-test",
		Format:      "console",
	})

	// Create high-performance mock queue service
	mockQueue := NewMockPerformanceQueueService()

	// Create optimized test configuration
	cfg := &config.Config{
		Queue: config.QueueConfig{
			URL:            mockQueue.URL(),
			Timeout:        2 * time.Second, // Reduced timeout for performance
			Retries:        1,               // Minimal retries for performance
			MaxRequestSize: 1024 * 1024,
			CircuitBreaker: config.CircuitBreakerConfig{
				Enabled:               true,
				FailureThreshold:      10,
				RecoveryTimeout:       5 * time.Second,
				MaxConcurrentRequests: 1000,
			},
			RoutingKeys: map[string]string{
				"profile_update":     "profile.task",
				"email_notification": "email.send",
				"image_processing":   "image.process",
			},
		},
	}

	// Create services
	storageClient := &services.StorageClient{}
	profileService := services.NewProfileService(cfg, storageClient, nil, nil, logger.Logger)
	taskHandler := handlers.NewTaskHandler(profileService)

	// Setup optimized Gin router
	gin.SetMode(gin.ReleaseMode) // Release mode for better performance
	router := gin.New()

	// Minimal middleware for performance testing
	router.Use(gin.Recovery())

	// Add task routes
	v1 := router.Group("/api/v1")
	profiles := v1.Group("/profiles")
	profiles.POST("/:id/tasks", taskHandler.SubmitTask)
	profiles.POST("/:id/tasks/email", taskHandler.SubmitEmailTask)
	profiles.POST("/:id/tasks/image", taskHandler.SubmitImageTask)
	profiles.POST("/:id/tasks/profile", taskHandler.SubmitProfileTask)
	profiles.GET("/:id/tasks/stats", taskHandler.GetTaskTypeStats)

	return router, mockQueue
}

// setupBenchmarkEnvironment creates optimized test environment for benchmarks
func setupBenchmarkEnvironment() (*gin.Engine, *MockPerformanceQueueService) {
	// ✅ Initialize logger for benchmarks
	logger.Initialize(&logger.Config{
		Level:       "error", // Minimal logging for performance
		Environment: "test",
		ServiceName: "profile-service-benchmark",
		Format:      "console",
	})

	// Create high-performance mock queue service
	mockQueue := NewMockPerformanceQueueService()

	// Create optimized test configuration
	cfg := &config.Config{
		Queue: config.QueueConfig{
			URL:            mockQueue.URL(),
			Timeout:        2 * time.Second, // Reduced timeout for performance
			Retries:        1,               // Minimal retries for performance
			MaxRequestSize: 1024 * 1024,
			CircuitBreaker: config.CircuitBreakerConfig{
				Enabled:               true,
				FailureThreshold:      10,
				RecoveryTimeout:       5 * time.Second,
				MaxConcurrentRequests: 1000,
			},
			RoutingKeys: map[string]string{
				"profile_update":     "profile.task",
				"email_notification": "email.send",
				"image_processing":   "image.process",
			},
		},
	}

	// Create services
	storageClient := &services.StorageClient{}
	profileService := services.NewProfileService(cfg, storageClient, nil, nil, logger.Logger)
	taskHandler := handlers.NewTaskHandler(profileService)

	// Setup optimized Gin router
	gin.SetMode(gin.ReleaseMode) // Release mode for better performance
	router := gin.New()

	// Minimal middleware for performance testing
	router.Use(gin.Recovery())

	// Add task routes
	v1 := router.Group("/api/v1")
	profiles := v1.Group("/profiles")
	profiles.POST("/:id/tasks", taskHandler.SubmitTask)
	profiles.POST("/:id/tasks/email", taskHandler.SubmitEmailTask)
	profiles.POST("/:id/tasks/image", taskHandler.SubmitImageTask)
	profiles.POST("/:id/tasks/profile", taskHandler.SubmitProfileTask)
	profiles.GET("/:id/tasks/stats", taskHandler.GetTaskTypeStats)

	return router, mockQueue
}

// TestAPIResponseTimeTarget tests that API response time is < 50ms
func TestAPIResponseTimeTarget(t *testing.T) {
	router, mockQueue := setupPerformanceTestEnvironment(t)
	defer mockQueue.Close()

	metrics := NewPerformanceMetrics()

	// Test different task types for response time
	testCases := []struct {
		name     string
		endpoint string
		payload  interface{}
	}{
		{
			name:     "ProfileTask",
			endpoint: "/api/v1/profiles/profile-123/tasks/profile",
			payload: models.ProfileTaskPayload{
				UserID: "user-123",
				Action: "update",
			},
		},
		{
			name:     "EmailTask",
			endpoint: "/api/v1/profiles/profile-123/tasks/email",
			payload: models.EmailTaskPayload{
				To:       "user@example.com",
				Template: "test",
			},
		},
		{
			name:     "ImageTask",
			endpoint: "/api/v1/profiles/profile-123/tasks/image",
			payload: models.ImageTaskPayload{
				ImageURL:  "https://example.com/test.jpg",
				Operation: "resize",
			},
		},
	}

	// Test response time for each task type
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Warm up
			for i := 0; i < 5; i++ {
				body, _ := json.Marshal(tc.payload)
				req, _ := http.NewRequest("POST", tc.endpoint, bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")

				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)
			}

			// Measure response times
			numSamples := 100
			responseTimes := make([]time.Duration, numSamples)

			for i := 0; i < numSamples; i++ {
				startTime := time.Now()

				body, _ := json.Marshal(tc.payload)
				req, _ := http.NewRequest("POST", tc.endpoint, bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")

				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				responseTime := time.Since(startTime)
				responseTimes[i] = responseTime

				success := w.Code == http.StatusAccepted
				metrics.RecordRequest(responseTime, success, tc.name)

				assert.Equal(t, http.StatusAccepted, w.Code, "Request should succeed")
			}

			// Calculate statistics
			var totalTime time.Duration
			minTime := responseTimes[0]
			maxTime := responseTimes[0]

			for _, rt := range responseTimes {
				totalTime += rt
				if rt < minTime {
					minTime = rt
				}
				if rt > maxTime {
					maxTime = rt
				}
			}

			avgTime := totalTime / time.Duration(numSamples)

			// Verify performance targets
			assert.Less(t, avgTime.Milliseconds(), int64(50),
				"Average response time should be < 50ms, got %v", avgTime)

			// 95th percentile should also be under 50ms for good user experience
			var under50ms int
			for _, rt := range responseTimes {
				if rt.Milliseconds() < 50 {
					under50ms++
				}
			}

			percentileUnder50ms := float64(under50ms) / float64(numSamples) * 100
			assert.Greater(t, percentileUnder50ms, 95.0,
				"At least 95%% of requests should be under 50ms, got %.2f%%", percentileUnder50ms)

			t.Logf("✅ %s Response Time Performance:", tc.name)
			t.Logf("   Average: %v", avgTime)
			t.Logf("   Min: %v", minTime)
			t.Logf("   Max: %v", maxTime)
			t.Logf("   Under 50ms: %.2f%%", percentileUnder50ms)
		})
	}

	metrics.FinalizeTiming()
	summary := metrics.GetSummary()
	t.Logf("✅ Overall API Response Time Test Summary: %+v", summary)
}

// TestHighThroughputLoad tests support for 1000+ tasks/second
func TestHighThroughputLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping throughput test in short mode")
	}

	router, mockQueue := setupPerformanceTestEnvironment(t)
	defer mockQueue.Close()

	metrics := NewPerformanceMetrics()
	metrics.concurrentUsers = 100

	// Test parameters
	targetRPS := 1000.0 // Target: 1000+ requests per second
	testDuration := 10 * time.Second
	concurrentUsers := 100
	requestsPerUser := int(targetRPS * testDuration.Seconds() / float64(concurrentUsers))

	t.Logf("Starting high throughput test:")
	t.Logf("  Target RPS: %.0f", targetRPS)
	t.Logf("  Test Duration: %v", testDuration)
	t.Logf("  Concurrent Users: %d", concurrentUsers)
	t.Logf("  Requests per User: %d", requestsPerUser)

	var wg sync.WaitGroup
	startTime := time.Now()

	// Launch concurrent users
	for userID := 0; userID < concurrentUsers; userID++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Each user submits different types of tasks
			taskTypes := []struct {
				endpoint string
				payload  interface{}
				taskType string
			}{
				{
					endpoint: "/api/v1/profiles/profile-123/tasks/profile",
					payload: models.ProfileTaskPayload{
						UserID: fmt.Sprintf("user-%d", id),
						Action: "update",
					},
					taskType: "profile_update",
				},
				{
					endpoint: "/api/v1/profiles/profile-123/tasks/email",
					payload: models.EmailTaskPayload{
						To:       fmt.Sprintf("user%d@example.com", id),
						Template: "test",
					},
					taskType: "email_notification",
				},
				{
					endpoint: "/api/v1/profiles/profile-123/tasks/image",
					payload: models.ImageTaskPayload{
						ImageURL:  fmt.Sprintf("https://example.com/user%d.jpg", id),
						Operation: "resize",
					},
					taskType: "image_processing",
				},
			}

			for requestNum := 0; requestNum < requestsPerUser; requestNum++ {
				taskType := taskTypes[requestNum%len(taskTypes)]

				requestStart := time.Now()

				body, _ := json.Marshal(taskType.payload)
				req, _ := http.NewRequest("POST", taskType.endpoint, bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")

				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				responseTime := time.Since(requestStart)
				success := w.Code == http.StatusAccepted

				metrics.RecordRequest(responseTime, success, taskType.taskType)

				// Rate limiting to maintain target RPS
				if requestNum < requestsPerUser-1 {
					expectedInterval := testDuration / time.Duration(requestsPerUser)
					elapsed := time.Since(startTime) - time.Duration(requestNum)*expectedInterval
					if elapsed < expectedInterval {
						time.Sleep(expectedInterval - elapsed)
					}
				}
			}
		}(userID)
	}

	// Wait for all users to complete
	wg.Wait()
	metrics.FinalizeTiming()

	// Analyze results
	summary := metrics.GetSummary()
	actualRPS := metrics.requestsPerSecond
	errorRate := float64(metrics.failedRequests) / float64(metrics.totalRequests) * 100

	// Performance assertions
	assert.Greater(t, actualRPS, targetRPS,
		"Should achieve target throughput of %.0f RPS, got %.2f RPS", targetRPS, actualRPS)

	assert.Less(t, errorRate, 1.0,
		"Error rate should be < 1%%, got %.2f%%", errorRate)

	// Verify queue service received all messages
	receivedCount := mockQueue.GetReceivedCount()
	expectedCount := int64(concurrentUsers * requestsPerUser)

	// Allow for some variance due to test timing
	assert.InDelta(t, expectedCount, receivedCount, float64(expectedCount)*0.05,
		"Queue service should receive approximately %d messages, got %d", expectedCount, receivedCount)

	t.Logf("✅ High Throughput Load Test Results:")
	t.Logf("   Achieved RPS: %.2f", actualRPS)
	t.Logf("   Error Rate: %.2f%%", errorRate)
	t.Logf("   Messages Received: %d/%d", receivedCount, expectedCount)
	t.Logf("   Summary: %+v", summary)
}

// TestQueueServiceCommunicationPerformance tests queue service communication < 100ms
func TestQueueServiceCommunicationPerformance(t *testing.T) {
	router, mockQueue := setupPerformanceTestEnvironment(t)
	defer mockQueue.Close()

	// Test with different queue service response delays
	delayScenarios := []struct {
		name     string
		delay    time.Duration
		expected bool
	}{
		{"FastQueue", 10 * time.Millisecond, true},
		{"MediumQueue", 50 * time.Millisecond, true},
		{"SlowQueue", 120 * time.Millisecond, false}, // Should exceed 100ms target
	}

	for _, scenario := range delayScenarios {
		t.Run(scenario.name, func(t *testing.T) {
			mockQueue.Reset()
			mockQueue.SetResponseDelay(scenario.delay)

			metrics := NewPerformanceMetrics()
			numRequests := 50

			for i := 0; i < numRequests; i++ {
				payload := models.ProfileTaskPayload{
					UserID: fmt.Sprintf("user-%d", i),
					Action: "update",
				}

				startTime := time.Now()

				body, _ := json.Marshal(payload)
				req, _ := http.NewRequest("POST", "/api/v1/profiles/profile-123/tasks/profile", bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")

				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				responseTime := time.Since(startTime)
				success := w.Code == http.StatusAccepted

				metrics.RecordRequest(responseTime, success, "profile_update")

				if scenario.expected {
					assert.Equal(t, http.StatusAccepted, w.Code)
					// Response time should include queue communication time
					assert.Less(t, responseTime.Milliseconds(), int64(100),
						"Total response time (including queue communication) should be < 100ms")
				}
			}

			metrics.FinalizeTiming()
			summary := metrics.GetSummary()

			t.Logf("✅ Queue Communication Performance (%s):", scenario.name)
			t.Logf("   Queue Delay: %v", scenario.delay)
			t.Logf("   Average Response Time: %s", summary["avg_response_time"])
			t.Logf("   Success Rate: %s", summary["success_rate"])
		})
	}
}

// TestErrorRateUnderLoad tests that error rate stays < 1% under load
func TestErrorRateUnderLoad(t *testing.T) {
	router, mockQueue := setupPerformanceTestEnvironment(t)
	defer mockQueue.Close()

	// Configure mock to simulate 99.5% success rate (0.5% errors)
	mockQueue.SetSuccessRate(99.5)

	metrics := NewPerformanceMetrics()

	// Load test parameters
	concurrentUsers := 50
	requestsPerUser := 100

	var wg sync.WaitGroup

	for userID := 0; userID < concurrentUsers; userID++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for requestNum := 0; requestNum < requestsPerUser; requestNum++ {
				payload := models.ProfileTaskPayload{
					UserID: fmt.Sprintf("user-%d-%d", id, requestNum),
					Action: "update",
				}

				startTime := time.Now()

				body, _ := json.Marshal(payload)
				req, _ := http.NewRequest("POST", "/api/v1/profiles/profile-123/tasks/profile", bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")

				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)

				responseTime := time.Since(startTime)
				success := w.Code == http.StatusAccepted

				metrics.RecordRequest(responseTime, success, "profile_update")
			}
		}(userID)
	}

	wg.Wait()
	metrics.FinalizeTiming()

	// Calculate error rate
	totalRequests := metrics.totalRequests
	failedRequests := metrics.failedRequests
	errorRate := float64(failedRequests) / float64(totalRequests) * 100

	// Verify error rate is under 1%
	assert.Less(t, errorRate, 1.0,
		"Error rate should be < 1%%, got %.2f%% (%d failures out of %d requests)",
		errorRate, failedRequests, totalRequests)

	summary := metrics.GetSummary()
	t.Logf("✅ Error Rate Under Load Test:")
	t.Logf("   Total Requests: %d", totalRequests)
	t.Logf("   Failed Requests: %d", failedRequests)
	t.Logf("   Error Rate: %.2f%%", errorRate)
	t.Logf("   Summary: %+v", summary)
}

// BenchmarkTaskSubmission benchmarks task submission performance
func BenchmarkTaskSubmission(b *testing.B) {
	router, mockQueue := setupBenchmarkEnvironment()
	defer mockQueue.Close()

	// Benchmark different task types
	benchmarks := []struct {
		name     string
		endpoint string
		payload  interface{}
	}{
		{
			name:     "ProfileTask",
			endpoint: "/api/v1/profiles/profile-123/tasks/profile",
			payload: models.ProfileTaskPayload{
				UserID: "user-123",
				Action: "update",
			},
		},
		{
			name:     "EmailTask",
			endpoint: "/api/v1/profiles/profile-123/tasks/email",
			payload: models.EmailTaskPayload{
				To:       "user@example.com",
				Template: "test",
			},
		},
		{
			name:     "ImageTask",
			endpoint: "/api/v1/profiles/profile-123/tasks/image",
			payload: models.ImageTaskPayload{
				ImageURL:  "https://example.com/test.jpg",
				Operation: "resize",
			},
		},
	}

	for _, benchmark := range benchmarks {
		b.Run(benchmark.name, func(b *testing.B) {
			body, _ := json.Marshal(benchmark.payload)

			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					req, _ := http.NewRequest("POST", benchmark.endpoint, bytes.NewBuffer(body))
					req.Header.Set("Content-Type", "application/json")

					w := httptest.NewRecorder()
					router.ServeHTTP(w, req)

					if w.Code != http.StatusAccepted {
						b.Errorf("Expected status %d, got %d", http.StatusAccepted, w.Code)
					}
				}
			})
		})
	}
}

// BenchmarkRoutingKeyDetermination benchmarks routing key determination overhead
func BenchmarkRoutingKeyDetermination(b *testing.B) {
	// Test routing key determination performance
	taskTypes := []string{"profile_update", "email_notification", "image_processing", "unknown_type"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		taskType := taskTypes[i%len(taskTypes)]

		// Simulate routing key determination (copy of the logic from services)
		routingKeyMap := map[string]string{
			"profile_update":     "profile.task",
			"email_notification": "email.send",
			"image_processing":   "image.process",
		}

		var routingKey string
		if key, exists := routingKeyMap[taskType]; exists {
			routingKey = key
		} else {
			routingKey = "profile.task" // Default fallback
		}

		// Prevent compiler optimization
		_ = routingKey
	}
}

// BenchmarkMessageSerialization benchmarks message serialization performance
func BenchmarkMessageSerialization(b *testing.B) {
	payloads := []interface{}{
		models.ProfileTaskPayload{
			UserID: "user-123",
			Action: "update",
			Data: map[string]interface{}{
				"name":  "John Doe",
				"email": "john@example.com",
			},
		},
		models.EmailTaskPayload{
			To:       "user@example.com",
			Template: "welcome",
			Subject:  "Welcome!",
			Data: map[string]interface{}{
				"username":    "johndoe",
				"signup_date": "2024-01-15",
			},
		},
		models.ImageTaskPayload{
			ImageURL:     "https://example.com/image.jpg",
			Operation:    "resize",
			OutputFormat: "webp",
			Options: map[string]interface{}{
				"width":   400,
				"height":  400,
				"quality": 85,
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		payload := payloads[i%len(payloads)]

		// Benchmark JSON serialization
		_, err := json.Marshal(payload)
		if err != nil {
			b.Fatal(err)
		}
	}
}
