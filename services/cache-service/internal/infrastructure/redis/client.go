package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/sony/gobreaker"
	"go.uber.org/zap"

	"cache-service/internal/config"
	"cache-service/internal/domain/models"
)

// Client wraps Redis client with circuit breaker and additional functionality
type Client struct {
	client *redis.Client
	cb     *gobreaker.CircuitBreaker
	logger *zap.Logger
	config *config.RedisConfig
}

// NewClient creates a new Redis client with circuit breaker
func NewClient(cfg *config.RedisConfig, logger *zap.Logger) (*Client, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("Redis is disabled")
	}

	// Configure Redis client
	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.GetRedisAddr(),
		Password:     cfg.Password,
		DB:           cfg.Database,
		MaxRetries:   cfg.MaxRetries,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	client := &Client{
		client: rdb,
		logger: logger,
		config: cfg,
	}

	return client, nil
}

// InitializeCircuitBreaker initializes circuit breaker with configuration
func (c *Client) InitializeCircuitBreaker(cbConfig *config.CircuitBrConfig) {
	cbSettings := gobreaker.Settings{
		Name:        "redis-client",
		MaxRequests: cbConfig.MaxRequests,
		Interval:    cbConfig.Interval,
		Timeout:     cbConfig.Timeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= cbConfig.ReadyToTrip
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			c.logger.Info("Circuit breaker state changed",
				zap.String("name", name),
				zap.String("from", from.String()),
				zap.String("to", to.String()),
			)
		},
	}

	c.cb = gobreaker.NewCircuitBreaker(cbSettings)

	c.logger.Info("Circuit breaker initialized",
		zap.String("name", cbSettings.Name),
		zap.Uint32("max_requests", cbSettings.MaxRequests),
		zap.Duration("interval", cbSettings.Interval),
		zap.Duration("timeout", cbSettings.Timeout),
		zap.Uint32("ready_to_trip", cbConfig.ReadyToTrip),
	)
}

// Get retrieves a value from Redis with circuit breaker protection
func (c *Client) Get(ctx context.Context, key string) ([]byte, error) {
	result, err := c.cb.Execute(func() (interface{}, error) {
		return c.client.Get(ctx, key).Bytes()
	})

	if err != nil {
		if err == redis.Nil {
			return nil, ErrKeyNotFound
		}
		c.logger.Error("Redis GET failed", zap.String("key", key), zap.Error(err))
		return nil, fmt.Errorf("failed to get key %s: %w", key, err)
	}

	return result.([]byte), nil
}

// Set stores a value in Redis with TTL and circuit breaker protection
func (c *Client) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	_, err := c.cb.Execute(func() (interface{}, error) {
		return nil, c.client.Set(ctx, key, value, ttl).Err()
	})

	if err != nil {
		c.logger.Error("Redis SET failed",
			zap.String("key", key),
			zap.Duration("ttl", ttl),
			zap.Error(err))
		return fmt.Errorf("failed to set key %s: %w", key, err)
	}

	return nil
}

// Delete removes a key from Redis with circuit breaker protection
func (c *Client) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}

	_, err := c.cb.Execute(func() (interface{}, error) {
		return nil, c.client.Del(ctx, keys...).Err()
	})

	if err != nil {
		c.logger.Error("Redis DELETE failed", zap.Strings("keys", keys), zap.Error(err))
		return fmt.Errorf("failed to delete keys: %w", err)
	}

	return nil
}

// Exists checks if a key exists in Redis
func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	result, err := c.cb.Execute(func() (interface{}, error) {
		return c.client.Exists(ctx, key).Result()
	})

	if err != nil {
		c.logger.Error("Redis EXISTS failed", zap.String("key", key), zap.Error(err))
		return false, fmt.Errorf("failed to check key existence %s: %w", key, err)
	}

	return result.(int64) > 0, nil
}

// MGet retrieves multiple values from Redis
func (c *Client) MGet(ctx context.Context, keys []string) (map[string][]byte, error) {
	if len(keys) == 0 {
		return make(map[string][]byte), nil
	}

	result, err := c.cb.Execute(func() (interface{}, error) {
		return c.client.MGet(ctx, keys...).Result()
	})

	if err != nil {
		c.logger.Error("Redis MGET failed", zap.Strings("keys", keys), zap.Error(err))
		return nil, fmt.Errorf("failed to get multiple keys: %w", err)
	}

	values := result.([]interface{})
	resultMap := make(map[string][]byte)

	for i, key := range keys {
		if i < len(values) && values[i] != nil {
			if str, ok := values[i].(string); ok {
				resultMap[key] = []byte(str)
			}
		}
	}

	return resultMap, nil
}

// MSet sets multiple key-value pairs in Redis
func (c *Client) MSet(ctx context.Context, items map[string][]byte, ttl time.Duration) error {
	if len(items) == 0 {
		return nil
	}

	// Convert to slice for Redis MSet
	pairs := make([]interface{}, 0, len(items)*2)
	for key, value := range items {
		pairs = append(pairs, key, value)
	}

	_, err := c.cb.Execute(func() (interface{}, error) {
		pipe := c.client.TxPipeline()
		pipe.MSet(ctx, pairs...)

		// Set TTL for each key if specified
		if ttl > 0 {
			for key := range items {
				pipe.Expire(ctx, key, ttl)
			}
		}

		_, err := pipe.Exec(ctx)
		return nil, err
	})

	if err != nil {
		c.logger.Error("Redis MSET failed",
			zap.Int("count", len(items)),
			zap.Duration("ttl", ttl),
			zap.Error(err))
		return fmt.Errorf("failed to set multiple keys: %w", err)
	}

	return nil
}

// GetTTL returns the remaining TTL for a key
func (c *Client) GetTTL(ctx context.Context, key string) (time.Duration, error) {
	result, err := c.cb.Execute(func() (interface{}, error) {
		return c.client.TTL(ctx, key).Result()
	})

	if err != nil {
		c.logger.Error("Redis TTL failed", zap.String("key", key), zap.Error(err))
		return 0, fmt.Errorf("failed to get TTL for key %s: %w", key, err)
	}

	return result.(time.Duration), nil
}

// SetTTL sets the TTL for an existing key
func (c *Client) SetTTL(ctx context.Context, key string, ttl time.Duration) error {
	_, err := c.cb.Execute(func() (interface{}, error) {
		return nil, c.client.Expire(ctx, key, ttl).Err()
	})

	if err != nil {
		c.logger.Error("Redis EXPIRE failed",
			zap.String("key", key),
			zap.Duration("ttl", ttl),
			zap.Error(err))
		return fmt.Errorf("failed to set TTL for key %s: %w", key, err)
	}

	return nil
}

// DeleteByPattern deletes keys matching a pattern
func (c *Client) DeleteByPattern(ctx context.Context, pattern string) (int64, error) {
	var deleted int64

	_, err := c.cb.Execute(func() (interface{}, error) {
		iter := c.client.Scan(ctx, 0, pattern, 0).Iterator()
		keys := make([]string, 0)

		for iter.Next(ctx) {
			keys = append(keys, iter.Val())
			// Delete in batches of 100
			if len(keys) >= 100 {
				result := c.client.Del(ctx, keys...)
				if result.Err() != nil {
					return nil, result.Err()
				}
				deleted += result.Val()
				keys = keys[:0]
			}
		}

		// Delete remaining keys
		if len(keys) > 0 {
			result := c.client.Del(ctx, keys...)
			if result.Err() != nil {
				return nil, result.Err()
			}
			deleted += result.Val()
		}

		return deleted, iter.Err()
	})

	if err != nil {
		c.logger.Error("Redis delete by pattern failed",
			zap.String("pattern", pattern),
			zap.Error(err))
		return 0, fmt.Errorf("failed to delete keys by pattern %s: %w", pattern, err)
	}

	return deleted, nil
}

// GetStats returns Redis statistics
func (c *Client) GetStats(ctx context.Context) (*models.CacheStats, error) {
	_, err := c.cb.Execute(func() (interface{}, error) {
		info := c.client.Info(ctx, "stats", "memory", "keyspace")
		return info.Result()
	})

	if err != nil {
		c.logger.Error("Redis INFO failed", zap.Error(err))
		return nil, fmt.Errorf("failed to get Redis stats: %w", err)
	}

	// Parse Redis INFO output (simplified)
	stats := &models.CacheStats{
		LastUpdated: time.Now(),
		// Note: Full parsing of Redis INFO would be more complex
		// This is a simplified version for the implementation
	}

	return stats, nil
}

// Ping checks Redis connectivity
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.cb.Execute(func() (interface{}, error) {
		return nil, c.client.Ping(ctx).Err()
	})

	if err != nil {
		return fmt.Errorf("Redis ping failed: %w", err)
	}

	return nil
}

// Close closes the Redis connection
func (c *Client) Close() error {
	return c.client.Close()
}

// GetCircuitBreakerStatus returns circuit breaker status
func (c *Client) GetCircuitBreakerStatus() *models.CircuitBreakerStatus {
	counts := c.cb.Counts()
	return &models.CircuitBreakerStatus{
		Name:     c.cb.Name(),
		State:    c.cb.State().String(),
		Failures: counts.ConsecutiveFailures,
		Requests: counts.Requests,
	}
}

// Custom errors
var (
	ErrKeyNotFound = fmt.Errorf("key not found")
	ErrCircuitOpen = fmt.Errorf("circuit breaker is open")
)
