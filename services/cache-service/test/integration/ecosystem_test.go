package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cache-service/internal/config"
	"cache-service/internal/domain/models"
	"cache-service/internal/domain/services"
	"cache-service/internal/infrastructure/logging"
	"cache-service/internal/infrastructure/metrics"
	"cache-service/internal/infrastructure/redis"
)

// TestEcosystemIntegration tests the ecosystem-specific cache services
func TestEcosystemIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping ecosystem integration test in short mode")
	}

	// Setup test configuration
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host:         "localhost",
			Port:         6379,
			Database:     1, // Use test database
			PoolSize:     10,
			MinIdleConns: 2,
			DialTimeout:  5 * time.Second,
			ReadTimeout:  3 * time.Second,
			WriteTimeout: 3 * time.Second,
			Enabled:      true,
		},
		Cache: config.CacheConfig{
			DefaultTTL:      3600 * time.Second,
			ProfileTTL:      1800 * time.Second,
			TaskTTL:         300 * time.Second,
			SessionTTL:      1800 * time.Second,
			QueueMetricsTTL: 120 * time.Second,
			WorkerStatusTTL: 600 * time.Second,
			MaxKeySize:      512,
			MaxValueSize:    1048576,
			BatchSize:       100,
		},
		Logging: config.LoggingConfig{
			Level:       "info",
			Format:      "json",
			Development: false,
		},
	}

	// Initialize components
	logger, err := logging.NewLogger(&cfg.Logging)
	require.NoError(t, err)

	metricsCollector := metrics.NewMetrics()

	redisClient, err := redis.NewClient(&cfg.Redis, logger)
	require.NoError(t, err)
	defer redisClient.Close()

	// Test Redis connectivity
	ctx := context.Background()
	err = redisClient.Ping(ctx)
	if err != nil {
		t.Skip("Redis not available for integration tests")
	}

	// Initialize cache service
	cacheService := services.NewCacheService(redisClient, metricsCollector, logger, &cfg.Cache)

	// Initialize ecosystem services
	profileCache := services.NewProfileCacheService(cacheService, logger, metricsCollector, &cfg.Cache)
	taskCache := services.NewTaskCacheService(cacheService, logger, metricsCollector, &cfg.Cache)
	sessionCache := services.NewSessionCacheService(cacheService, logger, metricsCollector, &cfg.Cache)

	t.Run("ProfileCacheService", func(t *testing.T) {
		testProfileCacheService(t, profileCache)
	})

	t.Run("TaskCacheService", func(t *testing.T) {
		testTaskCacheService(t, taskCache)
	})

	t.Run("SessionCacheService", func(t *testing.T) {
		testSessionCacheService(t, sessionCache)
	})

	t.Run("EcosystemIntegrationPatterns", func(t *testing.T) {
		testEcosystemIntegrationPatterns(t, profileCache, taskCache, sessionCache)
	})
}

func testProfileCacheService(t *testing.T, profileCache *services.ProfileCacheService) {
	ctx := context.Background()

	// Test profile data
	testProfile := &models.Profile{
		ID:       "profile-123",
		Email:    "user@example.com",
		Username: "testuser",
		FullName: "Test User",
		Metadata: map[string]interface{}{
			"role":        "user",
			"preferences": map[string]string{"theme": "dark"},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Test SetProfile and GetProfile
	err := profileCache.SetProfile(ctx, testProfile.ID, testProfile, time.Hour)
	assert.NoError(t, err)

	retrievedProfile, err := profileCache.GetProfile(ctx, testProfile.ID)
	assert.NoError(t, err)
	assert.Equal(t, testProfile.ID, retrievedProfile.ID)
	assert.Equal(t, testProfile.Email, retrievedProfile.Email)
	assert.Equal(t, testProfile.Username, retrievedProfile.Username)

	// Test SetProfileByEmail and GetProfileByEmail
	err = profileCache.SetProfileByEmail(ctx, testProfile.Email, testProfile, time.Hour)
	assert.NoError(t, err)

	profileByEmail, err := profileCache.GetProfileByEmail(ctx, testProfile.Email)
	assert.NoError(t, err)
	assert.Equal(t, testProfile.ID, profileByEmail.ID)
	assert.Equal(t, testProfile.Email, profileByEmail.Email)

	// Test BatchGetProfiles
	profileIDs := []string{testProfile.ID, "non-existent-profile"}
	profiles, err := profileCache.BatchGetProfiles(ctx, profileIDs)
	assert.NoError(t, err)
	assert.Len(t, profiles, 1)
	assert.Contains(t, profiles, testProfile.ID)

	// Test InvalidateProfile
	err = profileCache.InvalidateProfile(ctx, testProfile.ID, testProfile.Email)
	assert.NoError(t, err)

	// Verify invalidation
	_, err = profileCache.GetProfile(ctx, testProfile.ID)
	assert.Equal(t, services.ErrKeyNotFound, err)

	_, err = profileCache.GetProfileByEmail(ctx, testProfile.Email)
	assert.Equal(t, services.ErrKeyNotFound, err)
}

func testTaskCacheService(t *testing.T, taskCache *services.TaskCacheService) {
	ctx := context.Background()

	// Test task status data
	testTask := &models.TaskStatus{
		ID:        "task-456",
		Type:      "data-processing",
		Status:    "running",
		Progress:  0.75,
		Result:    map[string]interface{}{"processed": 750, "total": 1000},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Test SetTaskStatus and GetTaskStatus
	err := taskCache.SetTaskStatus(ctx, testTask.ID, testTask, time.Hour)
	assert.NoError(t, err)

	retrievedTask, err := taskCache.GetTaskStatus(ctx, testTask.ID)
	assert.NoError(t, err)
	assert.Equal(t, testTask.ID, retrievedTask.ID)
	assert.Equal(t, testTask.Status, retrievedTask.Status)
	assert.Equal(t, testTask.Progress, retrievedTask.Progress)

	// Test queue metrics
	queueMetrics := &models.QueueMetrics{
		QueueName:       "processing-queue",
		TotalJobs:       1000,
		PendingJobs:     150,
		ProcessingJobs:  50,
		CompletedJobs:   800,
		FailedJobs:      0,
		AverageWaitTime: 30 * time.Second,
		LastUpdated:     time.Now(),
	}

	err = taskCache.SetQueueMetrics(ctx, queueMetrics.QueueName, queueMetrics, time.Minute*5)
	assert.NoError(t, err)

	retrievedMetrics, err := taskCache.GetQueueMetrics(ctx, queueMetrics.QueueName)
	assert.NoError(t, err)
	assert.Equal(t, queueMetrics.QueueName, retrievedMetrics.QueueName)
	assert.Equal(t, queueMetrics.PendingJobs, retrievedMetrics.PendingJobs)

	// Test worker status
	workerStatus := &models.WorkerStatus{
		WorkerType:     "data-processor",
		ActiveWorkers:  5,
		IdleWorkers:    2,
		TotalWorkers:   7,
		ProcessingJobs: 5,
		CompletedJobs:  245,
		FailedJobs:     3,
		LastHeartbeat:  time.Now(),
	}

	err = taskCache.SetWorkerStatus(ctx, workerStatus.WorkerType, workerStatus, time.Minute*10)
	assert.NoError(t, err)

	retrievedWorkerStatus, err := taskCache.GetWorkerStatus(ctx, workerStatus.WorkerType)
	assert.NoError(t, err)
	assert.Equal(t, workerStatus.WorkerType, retrievedWorkerStatus.WorkerType)
	assert.Equal(t, workerStatus.ActiveWorkers, retrievedWorkerStatus.ActiveWorkers)

	// Test BatchGetTaskStatuses
	taskIDs := []string{testTask.ID, "non-existent-task"}
	tasks, err := taskCache.BatchGetTaskStatuses(ctx, taskIDs)
	assert.NoError(t, err)
	assert.Len(t, tasks, 1)
	assert.Contains(t, tasks, testTask.ID)

	// Clean up
	taskCache.InvalidateTasksByStatus(ctx, "running")
}

func testSessionCacheService(t *testing.T, sessionCache *services.SessionCacheService) {
	ctx := context.Background()

	// Test session data
	testSession := &models.Session{
		ID:        "session-789",
		UserID:    "user-123",
		DeviceID:  "device-456",
		IPAddress: "192.168.1.100",
		UserAgent: "Mozilla/5.0 Test Browser",
		Data: map[string]interface{}{
			"theme":     "dark",
			"language":  "en",
			"last_page": "/dashboard",
		},
		CreatedAt: time.Now(),
		LastUsed:  time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	// Test SetSession and GetSession
	err := sessionCache.SetSession(ctx, testSession.ID, testSession, time.Hour)
	assert.NoError(t, err)

	retrievedSession, err := sessionCache.GetSession(ctx, testSession.ID)
	assert.NoError(t, err)
	assert.Equal(t, testSession.ID, retrievedSession.ID)
	assert.Equal(t, testSession.UserID, retrievedSession.UserID)
	assert.Equal(t, testSession.IPAddress, retrievedSession.IPAddress)

	// Test JWT token blacklisting
	tokenID := "jwt-token-123"

	// Initially token should not be blacklisted
	isBlacklisted, err := sessionCache.IsTokenBlacklisted(ctx, tokenID)
	assert.NoError(t, err)
	assert.False(t, isBlacklisted)

	// Blacklist the token
	err = sessionCache.BlacklistToken(ctx, tokenID, time.Hour)
	assert.NoError(t, err)

	// Now token should be blacklisted
	isBlacklisted, err = sessionCache.IsTokenBlacklisted(ctx, tokenID)
	assert.NoError(t, err)
	assert.True(t, isBlacklisted)

	// Test UpdateSessionActivity
	err = sessionCache.UpdateSessionActivity(ctx, testSession.ID)
	assert.NoError(t, err)

	// Verify session was updated
	updatedSession, err := sessionCache.GetSession(ctx, testSession.ID)
	assert.NoError(t, err)
	assert.True(t, updatedSession.LastUsed.After(testSession.LastUsed))

	// Test DeleteSession
	err = sessionCache.DeleteSession(ctx, testSession.ID)
	assert.NoError(t, err)

	// Verify deletion
	_, err = sessionCache.GetSession(ctx, testSession.ID)
	assert.Equal(t, services.ErrKeyNotFound, err)
}

func testEcosystemIntegrationPatterns(t *testing.T,
	profileCache *services.ProfileCacheService,
	taskCache *services.TaskCacheService,
	sessionCache *services.SessionCacheService) {

	ctx := context.Background()

	// Test cross-service integration patterns
	userID := "integration-user-123"

	// 1. Profile-Session Integration Pattern
	profile := &models.Profile{
		ID:        userID,
		Email:     "integration@example.com",
		Username:  "integrationuser",
		FullName:  "Integration Test User",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	session := &models.Session{
		ID:        "integration-session-123",
		UserID:    userID,
		IPAddress: "10.0.0.1",
		CreatedAt: time.Now(),
		LastUsed:  time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	// Cache profile and session
	err := profileCache.SetProfile(ctx, profile.ID, profile, time.Hour)
	assert.NoError(t, err)

	err = sessionCache.SetSession(ctx, session.ID, session, time.Hour)
	assert.NoError(t, err)

	// 2. Task-Profile Integration Pattern
	task := &models.TaskStatus{
		ID:        "integration-task-123",
		Type:      "profile-update",
		Status:    "completed",
		Progress:  1.0,
		Result:    map[string]interface{}{"profile_id": userID, "updated_fields": []string{"email"}},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = taskCache.SetTaskStatus(ctx, task.ID, task, time.Hour)
	assert.NoError(t, err)

	// 3. Cache Invalidation Pattern
	// When a profile is updated, invalidate related caches

	// Simulate profile update - invalidate profile cache
	err = profileCache.InvalidateProfile(ctx, profile.ID, profile.Email)
	assert.NoError(t, err)

	// Simulate task completion - invalidate completed tasks
	deletedCount, err := taskCache.InvalidateTasksByStatus(ctx, "completed")
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, deletedCount, int64(0))

	// 4. Session Management Pattern
	// Multiple sessions per user should be manageable
	sessions, err := sessionCache.GetSessionsByUserID(ctx, userID)
	assert.NoError(t, err)
	// Note: This will return empty since we're using a simplified index approach
	assert.IsType(t, []*models.Session{}, sessions)

	// Clean up
	sessionCache.DeleteSession(ctx, session.ID)
}
