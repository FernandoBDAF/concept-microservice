package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"

	"cache-service/internal/config"
	"cache-service/internal/domain/models"
	"cache-service/internal/infrastructure/metrics"
	"cache-service/internal/infrastructure/redis"
)

// CacheService implements core cache operations
type CacheService struct {
	redis   *redis.Client
	metrics *metrics.Metrics
	logger  *zap.Logger
	config  *config.CacheConfig
}

// NewCacheService creates a new cache service
func NewCacheService(
	redisClient *redis.Client,
	metrics *metrics.Metrics,
	logger *zap.Logger,
	config *config.CacheConfig,
) *CacheService {
	return &CacheService{
		redis:   redisClient,
		metrics: metrics,
		logger:  logger,
		config:  config,
	}
}

// Get retrieves a value from cache
func (c *CacheService) Get(ctx context.Context, key string) ([]byte, error) {
	start := time.Now()

	// Validate key size
	if len(key) > c.config.MaxKeySize {
		c.metrics.RecordCacheError()
		return nil, fmt.Errorf("key size exceeds maximum allowed size")
	}

	value, err := c.redis.Get(ctx, key)
	duration := time.Since(start)

	if err != nil {
		if err == redis.ErrKeyNotFound {
			c.metrics.RecordCacheMiss()
			c.metrics.RecordCacheLatency("get", "miss", duration)
			return nil, ErrKeyNotFound
		}

		c.metrics.RecordCacheError()
		c.metrics.RecordCacheLatency("get", "error", duration)
		c.logger.Error("Cache GET failed",
			zap.String("key", key),
			zap.Error(err))
		return nil, err
	}

	c.metrics.RecordCacheHit()
	c.metrics.RecordCacheLatency("get", "hit", duration)

	return value, nil
}

// Set stores a value in cache with TTL
func (c *CacheService) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	start := time.Now()

	// Validate key and value sizes
	if len(key) > c.config.MaxKeySize {
		c.metrics.RecordCacheError()
		return fmt.Errorf("key size exceeds maximum allowed size")
	}

	if len(value) > c.config.MaxValueSize {
		c.metrics.RecordCacheError()
		return fmt.Errorf("value size exceeds maximum allowed size")
	}

	// Use default TTL if not specified
	if ttl <= 0 {
		ttl = c.config.DefaultTTL
	}

	err := c.redis.Set(ctx, key, value, ttl)
	duration := time.Since(start)

	if err != nil {
		c.metrics.RecordCacheError()
		c.metrics.RecordCacheLatency("set", "error", duration)
		c.logger.Error("Cache SET failed",
			zap.String("key", key),
			zap.Duration("ttl", ttl),
			zap.Error(err))
		return err
	}

	c.metrics.RecordCacheLatency("set", "success", duration)

	return nil
}

// Delete removes a key from cache
func (c *CacheService) Delete(ctx context.Context, key string) error {
	start := time.Now()

	err := c.redis.Delete(ctx, key)
	duration := time.Since(start)

	if err != nil {
		c.metrics.RecordCacheError()
		c.metrics.RecordCacheLatency("delete", "error", duration)
		c.logger.Error("Cache DELETE failed",
			zap.String("key", key),
			zap.Error(err))
		return err
	}

	c.metrics.RecordCacheLatency("delete", "success", duration)

	return nil
}

// Exists checks if a key exists in cache
func (c *CacheService) Exists(ctx context.Context, key string) (bool, error) {
	start := time.Now()

	exists, err := c.redis.Exists(ctx, key)
	duration := time.Since(start)

	if err != nil {
		c.metrics.RecordCacheError()
		c.metrics.RecordCacheLatency("exists", "error", duration)
		c.logger.Error("Cache EXISTS failed",
			zap.String("key", key),
			zap.Error(err))
		return false, err
	}

	c.metrics.RecordCacheLatency("exists", "success", duration)

	return exists, nil
}

// MGet retrieves multiple values from cache
func (c *CacheService) MGet(ctx context.Context, keys []string) (map[string][]byte, error) {
	start := time.Now()

	if len(keys) == 0 {
		return make(map[string][]byte), nil
	}

	if len(keys) > c.config.BatchSize {
		c.metrics.RecordCacheError()
		return nil, fmt.Errorf("batch size exceeds maximum allowed size")
	}

	// Validate key sizes
	for _, key := range keys {
		if len(key) > c.config.MaxKeySize {
			c.metrics.RecordCacheError()
			return nil, fmt.Errorf("key size exceeds maximum allowed size")
		}
	}

	values, err := c.redis.MGet(ctx, keys)
	duration := time.Since(start)

	if err != nil {
		c.metrics.RecordBatchOperation("mget", "error", len(keys))
		c.metrics.RecordCacheLatency("mget", "error", duration)
		c.logger.Error("Cache MGET failed",
			zap.Int("key_count", len(keys)),
			zap.Error(err))
		return nil, err
	}

	// Count hits and misses
	hits := len(values)
	misses := len(keys) - hits

	for i := 0; i < hits; i++ {
		c.metrics.RecordCacheHit()
	}
	for i := 0; i < misses; i++ {
		c.metrics.RecordCacheMiss()
	}

	c.metrics.RecordBatchOperation("mget", "success", len(keys))
	c.metrics.RecordCacheLatency("mget", "success", duration)

	return values, nil
}

// MSet sets multiple key-value pairs in cache
func (c *CacheService) MSet(ctx context.Context, items map[string][]byte, ttl time.Duration) error {
	start := time.Now()

	if len(items) == 0 {
		return nil
	}

	if len(items) > c.config.BatchSize {
		c.metrics.RecordCacheError()
		return fmt.Errorf("batch size exceeds maximum allowed size")
	}

	// Validate key and value sizes
	for key, value := range items {
		if len(key) > c.config.MaxKeySize {
			c.metrics.RecordCacheError()
			return fmt.Errorf("key size exceeds maximum allowed size")
		}
		if len(value) > c.config.MaxValueSize {
			c.metrics.RecordCacheError()
			return fmt.Errorf("value size exceeds maximum allowed size")
		}
	}

	// Use default TTL if not specified
	if ttl <= 0 {
		ttl = c.config.DefaultTTL
	}

	err := c.redis.MSet(ctx, items, ttl)
	duration := time.Since(start)

	if err != nil {
		c.metrics.RecordBatchOperation("mset", "error", len(items))
		c.metrics.RecordCacheLatency("mset", "error", duration)
		c.logger.Error("Cache MSET failed",
			zap.Int("item_count", len(items)),
			zap.Duration("ttl", ttl),
			zap.Error(err))
		return err
	}

	c.metrics.RecordBatchOperation("mset", "success", len(items))
	c.metrics.RecordCacheLatency("mset", "success", duration)

	return nil
}

// MDelete removes multiple keys from cache
func (c *CacheService) MDelete(ctx context.Context, keys []string) error {
	start := time.Now()

	if len(keys) == 0 {
		return nil
	}

	if len(keys) > c.config.BatchSize {
		c.metrics.RecordCacheError()
		return fmt.Errorf("batch size exceeds maximum allowed size")
	}

	err := c.redis.Delete(ctx, keys...)
	duration := time.Since(start)

	if err != nil {
		c.metrics.RecordBatchOperation("mdelete", "error", len(keys))
		c.metrics.RecordCacheLatency("mdelete", "error", duration)
		c.logger.Error("Cache MDELETE failed",
			zap.Int("key_count", len(keys)),
			zap.Error(err))
		return err
	}

	c.metrics.RecordBatchOperation("mdelete", "success", len(keys))
	c.metrics.RecordCacheLatency("mdelete", "success", duration)

	return nil
}

// GetTTL returns the remaining TTL for a key
func (c *CacheService) GetTTL(ctx context.Context, key string) (time.Duration, error) {
	start := time.Now()

	ttl, err := c.redis.GetTTL(ctx, key)
	duration := time.Since(start)

	if err != nil {
		c.metrics.RecordCacheError()
		c.metrics.RecordCacheLatency("get_ttl", "error", duration)
		c.logger.Error("Cache GET_TTL failed",
			zap.String("key", key),
			zap.Error(err))
		return 0, err
	}

	c.metrics.RecordCacheLatency("get_ttl", "success", duration)

	return ttl, nil
}

// SetTTL sets the TTL for an existing key
func (c *CacheService) SetTTL(ctx context.Context, key string, ttl time.Duration) error {
	start := time.Now()

	err := c.redis.SetTTL(ctx, key, ttl)
	duration := time.Since(start)

	if err != nil {
		c.metrics.RecordCacheError()
		c.metrics.RecordCacheLatency("set_ttl", "error", duration)
		c.logger.Error("Cache SET_TTL failed",
			zap.String("key", key),
			zap.Duration("ttl", ttl),
			zap.Error(err))
		return err
	}

	c.metrics.RecordCacheLatency("set_ttl", "success", duration)

	return nil
}

// DeleteByPattern deletes keys matching a pattern
func (c *CacheService) DeleteByPattern(ctx context.Context, pattern string) (int64, error) {
	start := time.Now()

	deleted, err := c.redis.DeleteByPattern(ctx, pattern)
	duration := time.Since(start)

	if err != nil {
		c.metrics.RecordCacheError()
		c.metrics.RecordCacheLatency("delete_pattern", "error", duration)
		c.logger.Error("Cache DELETE_PATTERN failed",
			zap.String("pattern", pattern),
			zap.Error(err))
		return 0, err
	}

	c.metrics.RecordCacheLatency("delete_pattern", "success", duration)
	c.logger.Info("Keys deleted by pattern",
		zap.String("pattern", pattern),
		zap.Int64("count", deleted))

	return deleted, nil
}

// GetStats returns cache statistics
func (c *CacheService) GetStats(ctx context.Context) (*models.CacheStats, error) {
	return c.redis.GetStats(ctx)
}

// Ping checks cache connectivity
func (c *CacheService) Ping(ctx context.Context) error {
	return c.redis.Ping(ctx)
}

// SetJSON stores a JSON-serializable object in cache
func (c *CacheService) SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		c.metrics.RecordCacheError()
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return c.Set(ctx, key, data, ttl)
}

// GetJSON retrieves and unmarshals a JSON object from cache
func (c *CacheService) GetJSON(ctx context.Context, key string, dest interface{}) error {
	data, err := c.Get(ctx, key)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, dest); err != nil {
		c.metrics.RecordCacheError()
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return nil
}

// Custom errors
var (
	ErrKeyNotFound = fmt.Errorf("key not found")
)
