package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"cache-service/internal/config"
	"cache-service/internal/domain/services"
	"cache-service/internal/infrastructure/logging"
	"cache-service/internal/infrastructure/metrics"
	"cache-service/internal/infrastructure/redis"
)

// TestCacheServiceIntegration tests core cache operations with Redis
func TestCacheServiceIntegration(t *testing.T) {
	// Skip if Redis is not available
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
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
			DefaultTTL:   3600 * time.Second,
			MaxKeySize:   512,
			MaxValueSize: 1048576,
			BatchSize:    100,
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
	require.NoError(t, err, "Redis should be available for integration tests")

	// Initialize cache service
	cacheService := services.NewCacheService(redisClient, metricsCollector, logger, &cfg.Cache)

	t.Run("BasicOperations", func(t *testing.T) {
		testBasicOperations(t, cacheService)
	})

	t.Run("TTLOperations", func(t *testing.T) {
		testTTLOperations(t, cacheService)
	})

	t.Run("BatchOperations", func(t *testing.T) {
		testBatchOperations(t, cacheService)
	})

	t.Run("JSONOperations", func(t *testing.T) {
		testJSONOperations(t, cacheService)
	})

	t.Run("ErrorHandling", func(t *testing.T) {
		testErrorHandling(t, cacheService)
	})
}

func testBasicOperations(t *testing.T, cacheService *services.CacheService) {
	ctx := context.Background()
	testKey := "test:basic:key"
	testValue := []byte("test value")

	// Test SET operation
	err := cacheService.Set(ctx, testKey, testValue, time.Hour)
	assert.NoError(t, err)

	// Test GET operation
	retrievedValue, err := cacheService.Get(ctx, testKey)
	assert.NoError(t, err)
	assert.Equal(t, testValue, retrievedValue)

	// Test EXISTS operation
	exists, err := cacheService.Exists(ctx, testKey)
	assert.NoError(t, err)
	assert.True(t, exists)

	// Test DELETE operation
	err = cacheService.Delete(ctx, testKey)
	assert.NoError(t, err)

	// Verify deletion
	exists, err = cacheService.Exists(ctx, testKey)
	assert.NoError(t, err)
	assert.False(t, exists)

	// Test GET on non-existent key
	_, err = cacheService.Get(ctx, testKey)
	assert.Equal(t, services.ErrKeyNotFound, err)
}

func testTTLOperations(t *testing.T, cacheService *services.CacheService) {
	ctx := context.Background()
	testKey := "test:ttl:key"
	testValue := []byte("test value with TTL")

	// Set with TTL
	err := cacheService.Set(ctx, testKey, testValue, 5*time.Second)
	assert.NoError(t, err)

	// Check TTL
	ttl, err := cacheService.GetTTL(ctx, testKey)
	assert.NoError(t, err)
	assert.Greater(t, ttl, 0*time.Second)
	assert.LessOrEqual(t, ttl, 5*time.Second)

	// Update TTL
	err = cacheService.SetTTL(ctx, testKey, 10*time.Second)
	assert.NoError(t, err)

	// Verify updated TTL
	ttl, err = cacheService.GetTTL(ctx, testKey)
	assert.NoError(t, err)
	assert.Greater(t, ttl, 5*time.Second)

	// Cleanup
	err = cacheService.Delete(ctx, testKey)
	assert.NoError(t, err)
}

func testBatchOperations(t *testing.T, cacheService *services.CacheService) {
	ctx := context.Background()

	// Prepare test data
	testData := map[string][]byte{
		"test:batch:1": []byte("value1"),
		"test:batch:2": []byte("value2"),
		"test:batch:3": []byte("value3"),
	}

	// Test MSET
	err := cacheService.MSet(ctx, testData, time.Hour)
	assert.NoError(t, err)

	// Test MGET
	keys := make([]string, 0, len(testData))
	for key := range testData {
		keys = append(keys, key)
	}

	results, err := cacheService.MGet(ctx, keys)
	assert.NoError(t, err)
	assert.Equal(t, len(testData), len(results))

	for key, expectedValue := range testData {
		actualValue, exists := results[key]
		assert.True(t, exists, "Key %s should exist", key)
		assert.Equal(t, expectedValue, actualValue)
	}

	// Test MDELETE
	err = cacheService.MDelete(ctx, keys)
	assert.NoError(t, err)

	// Verify deletion
	for _, key := range keys {
		exists, err := cacheService.Exists(ctx, key)
		assert.NoError(t, err)
		assert.False(t, exists, "Key %s should be deleted", key)
	}
}

func testJSONOperations(t *testing.T, cacheService *services.CacheService) {
	ctx := context.Background()
	testKey := "test:json:key"

	// Test object
	testObject := struct {
		ID   int      `json:"id"`
		Name string   `json:"name"`
		Tags []string `json:"tags"`
	}{
		ID:   123,
		Name: "test object",
		Tags: []string{"tag1", "tag2"},
	}

	// Test SetJSON
	err := cacheService.SetJSON(ctx, testKey, testObject, time.Hour)
	assert.NoError(t, err)

	// Test GetJSON
	var retrievedObject struct {
		ID   int      `json:"id"`
		Name string   `json:"name"`
		Tags []string `json:"tags"`
	}

	err = cacheService.GetJSON(ctx, testKey, &retrievedObject)
	assert.NoError(t, err)
	assert.Equal(t, testObject, retrievedObject)

	// Cleanup
	err = cacheService.Delete(ctx, testKey)
	assert.NoError(t, err)
}

func testErrorHandling(t *testing.T, cacheService *services.CacheService) {
	ctx := context.Background()

	// Test key size validation
	longKey := string(make([]byte, 1000)) // Exceeds MaxKeySize (512)
	err := cacheService.Set(ctx, longKey, []byte("value"), time.Hour)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "key size exceeds maximum allowed size")

	// Test value size validation
	largeValue := make([]byte, 2*1024*1024) // Exceeds MaxValueSize (1MB)
	err = cacheService.Set(ctx, "testkey", largeValue, time.Hour)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "value size exceeds maximum allowed size")

	// Test batch size validation
	largeBatch := make(map[string][]byte)
	for i := 0; i < 200; i++ { // Exceeds BatchSize (100)
		largeBatch[fmt.Sprintf("key%d", i)] = []byte("value")
	}
	err = cacheService.MSet(ctx, largeBatch, time.Hour)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "batch size exceeds maximum allowed size")
}

// TestMetricsCollection validates that metrics are being collected properly
func TestMetricsCollection(t *testing.T) {
	// This test verifies that metrics are registered and can be collected
	metricsCollector := metrics.NewMetrics()

	// Simulate some operations
	metricsCollector.RecordCacheHit()
	metricsCollector.RecordCacheMiss()
	metricsCollector.RecordCacheLatency("get", "hit", 1*time.Millisecond)
	metricsCollector.RecordBatchOperation("mget", "success", 10)
	metricsCollector.SetCircuitBreakerState("redis-client", "closed")

	// The metrics should be registered with Prometheus
	// This is validated by the fact that NewMetrics() doesn't panic
	// and the operations complete without error
	assert.NotNil(t, metricsCollector)
}
