# Caching Best Practices

## Overview

This document outlines the best practices for implementing caching in our microservices architecture. It covers caching strategies, patterns, and implementation details for different types of data and access patterns.

## Caching Strategies

### 1. Cache-Aside Pattern

```go
// Cache-aside implementation
type ProfileCache struct {
    redis    *redis.Client
    db       *sql.DB
    logger   *zap.Logger
}

// Get profile with cache-aside
func (c *ProfileCache) GetProfile(ctx context.Context, id string) (*Profile, error) {
    // Try to get from cache first
    profile, err := c.getFromCache(ctx, id)
    if err == nil {
        return profile, nil
    }

    // Cache miss, get from database
    profile, err = c.getFromDB(ctx, id)
    if err != nil {
        return nil, err
    }

    // Update cache
    if err := c.setCache(ctx, id, profile); err != nil {
        c.logger.Warn("Failed to update cache",
            zap.String("profile_id", id),
            zap.Error(err))
    }

    return profile, nil
}

// Cache operations
func (c *ProfileCache) getFromCache(ctx context.Context, id string) (*Profile, error) {
    data, err := c.redis.Get(ctx, fmt.Sprintf("profile:%s", id)).Bytes()
    if err != nil {
        return nil, err
    }

    var profile Profile
    if err := json.Unmarshal(data, &profile); err != nil {
        return nil, err
    }

    return &profile, nil
}

func (c *ProfileCache) setCache(ctx context.Context, id string, profile *Profile) error {
    data, err := json.Marshal(profile)
    if err != nil {
        return err
    }

    return c.redis.Set(ctx, fmt.Sprintf("profile:%s", id), data, 24*time.Hour).Err()
}
```

### 2. Write-Through Pattern

```go
// Write-through implementation
type ProfileService struct {
    cache *ProfileCache
    db    *sql.DB
}

// Update profile with write-through
func (s *ProfileService) UpdateProfile(ctx context.Context, profile *Profile) error {
    // Update database
    if err := s.updateDB(ctx, profile); err != nil {
        return err
    }

    // Update cache
    if err := s.cache.setCache(ctx, profile.ID, profile); err != nil {
        s.logger.Warn("Failed to update cache",
            zap.String("profile_id", profile.ID),
            zap.Error(err))
    }

    return nil
}
```

## Cache Implementation

### 1. Redis Configuration

```go
// Redis configuration
type RedisConfig struct {
    Host     string
    Port     int
    Password string
    DB       int
    PoolSize int
}

// Redis client setup
func NewRedisClient(config RedisConfig) (*redis.Client, error) {
    client := redis.NewClient(&redis.Options{
        Addr:     fmt.Sprintf("%s:%d", config.Host, config.Port),
        Password: config.Password,
        DB:       config.DB,
        PoolSize: config.PoolSize,
    })

    // Test connection
    if err := client.Ping(context.Background()).Err(); err != nil {
        return nil, fmt.Errorf("failed to connect to Redis: %w", err)
    }

    return client, nil
}
```

### 2. Cache Key Management

```go
// Cache key management
type CacheKey struct {
    prefix string
    id     string
}

func (k CacheKey) String() string {
    return fmt.Sprintf("%s:%s", k.prefix, k.id)
}

// Key patterns
const (
    ProfileKeyPrefix    = "profile"
    PreferencesKeyPrefix = "preferences"
    SessionKeyPrefix    = "session"
)

// Key generation
func NewProfileKey(id string) CacheKey {
    return CacheKey{
        prefix: ProfileKeyPrefix,
        id:     id,
    }
}
```

## Cache Invalidation

### 1. Time-Based Invalidation

```go
// Time-based cache invalidation
func (c *ProfileCache) SetWithTTL(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
    data, err := json.Marshal(value)
    if err != nil {
        return err
    }

    return c.redis.Set(ctx, key, data, ttl).Err()
}

// Different TTLs for different data types
const (
    ProfileCacheTTL     = 24 * time.Hour
    PreferencesCacheTTL = 12 * time.Hour
    SessionCacheTTL     = 1 * time.Hour
)
```

### 2. Event-Based Invalidation

```go
// Event-based cache invalidation
type CacheInvalidator struct {
    redis    *redis.Client
    pubsub   *redis.PubSub
    logger   *zap.Logger
}

// Subscribe to invalidation events
func (i *CacheInvalidator) Start(ctx context.Context) {
    i.pubsub = i.redis.Subscribe(ctx, "cache:invalidate")
    go i.listen(ctx)
}

// Listen for invalidation events
func (i *CacheInvalidator) listen(ctx context.Context) {
    for {
        msg, err := i.pubsub.ReceiveMessage(ctx)
        if err != nil {
            i.logger.Error("Failed to receive message",
                zap.Error(err))
            continue
        }

        // Process invalidation message
        var event InvalidationEvent
        if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
            i.logger.Error("Failed to unmarshal event",
                zap.Error(err))
            continue
        }

        // Delete from cache
        if err := i.redis.Del(ctx, event.Key).Err(); err != nil {
            i.logger.Error("Failed to delete from cache",
                zap.String("key", event.Key),
                zap.Error(err))
        }
    }
}
```

## Cache Monitoring

### 1. Cache Metrics

```go
// Cache metrics
type CacheMetrics struct {
    hits        *prometheus.CounterVec
    misses      *prometheus.CounterVec
    latency     *prometheus.HistogramVec
    size        *prometheus.GaugeVec
}

func NewCacheMetrics() *CacheMetrics {
    return &CacheMetrics{
        hits: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "cache_hits_total",
                Help: "Total number of cache hits",
            },
            []string{"cache_type"},
        ),
        misses: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "cache_misses_total",
                Help: "Total number of cache misses",
            },
            []string{"cache_type"},
        ),
        latency: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name:    "cache_operation_duration_seconds",
                Help:    "Cache operation duration in seconds",
                Buckets: prometheus.DefBuckets,
            },
            []string{"operation", "cache_type"},
        ),
        size: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Name: "cache_size_bytes",
                Help: "Current size of cache in bytes",
            },
            []string{"cache_type"},
        ),
    }
}
```

### 2. Cache Health Checks

```go
// Cache health check
func (c *ProfileCache) HealthCheck(ctx context.Context) error {
    // Check Redis connection
    if err := c.redis.Ping(ctx).Err(); err != nil {
        return fmt.Errorf("Redis health check failed: %w", err)
    }

    // Check cache size
    info, err := c.redis.Info(ctx, "memory").Result()
    if err != nil {
        return fmt.Errorf("Failed to get Redis info: %w", err)
    }

    // Parse and validate memory usage
    if err := c.validateMemoryUsage(info); err != nil {
        return fmt.Errorf("Memory usage validation failed: %w", err)
    }

    return nil
}
```

## Best Practices

1. **Cache Strategy Selection**

   - Choose appropriate caching pattern
   - Consider data consistency requirements
   - Plan for cache invalidation
   - Monitor cache performance

2. **Cache Implementation**

   - Use appropriate cache key patterns
   - Implement proper error handling
   - Set appropriate TTLs
   - Handle cache failures gracefully

3. **Cache Monitoring**

   - Track cache hit/miss rates
   - Monitor cache size
   - Measure cache latency
   - Set up alerts for issues

4. **Cache Security**
   - Secure cache connections
   - Implement access control
   - Encrypt sensitive data
   - Monitor cache access

## Common Issues and Solutions

1. **Cache Invalidation**

   - Problem: Stale data in cache
   - Solution: Implement proper invalidation strategy

2. **Cache Performance**

   - Problem: High cache latency
   - Solution: Optimize cache operations and monitor metrics

3. **Cache Memory Usage**
   - Problem: Excessive memory consumption
   - Solution: Implement size limits and eviction policies

## References

- [Redis Documentation](https://redis.io/documentation)
- [Caching Patterns](https://docs.microsoft.com/en-us/azure/architecture/patterns/cache-aside)
- [Cache Invalidation Strategies](https://redis.io/topics/lru-cache)
- [Cache Monitoring Best Practices](https://redis.io/topics/monitoring)
