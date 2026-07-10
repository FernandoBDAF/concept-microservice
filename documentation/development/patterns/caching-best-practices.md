# Caching Best Practices

> *Migrated and adapted from legacy_project/reference-materials/development/caching-best-practices.md*

## Overview

This document outlines the best practices for implementing caching in our architecture using direct Redis access via go-redis.

## Architecture Context

In the consolidated architecture, caching is done through **direct Redis access** using go-redis.

## Caching Strategies

### 1. Cache-Aside Pattern (Recommended)

```go
type ProfileService struct {
    repository *ProfileRepository
    cache      *Cache
    logger     *zap.Logger
}

// Get profile with cache-aside
func (s *ProfileService) GetProfile(ctx context.Context, id string) (*Profile, error) {
    // Try to get from cache first
    profile, err := s.cache.GetProfile(ctx, id)
    if err == nil {
        return profile, nil
    }

    // Cache miss, get from database
    profile, err = s.repository.Get(ctx, id)
    if err != nil {
        return nil, err
    }

    // Update cache asynchronously
    go func() {
        if err := s.cache.SetProfile(context.Background(), profile); err != nil {
            s.logger.Warn("Failed to update cache",
                zap.String("profile_id", id),
                zap.Error(err))
        }
    }()

    return profile, nil
}
```

### 2. Write-Through Pattern

```go
// Update profile with write-through
func (s *ProfileService) UpdateProfile(ctx context.Context, profile *Profile) error {
    // Update database
    if err := s.repository.Update(ctx, profile); err != nil {
        return err
    }

    // Update cache
    if err := s.cache.SetProfile(ctx, profile); err != nil {
        s.logger.Warn("Failed to update cache",
            zap.String("profile_id", profile.ID),
            zap.Error(err))
    }

    return nil
}
```

## Cache Implementation

### 1. Redis Client Configuration

```go
func NewRedisClient(cfg *RedisConfig) (*redis.Client, error) {
    client := redis.NewClient(&redis.Options{
        Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
        Password:     cfg.Password,
        DB:           cfg.DB,
        PoolSize:     cfg.PoolSize,      // Default: 10 * GOMAXPROCS
        MinIdleConns: cfg.MinIdleConns,  // Default: 10
        MaxRetries:   3,
        DialTimeout:  5 * time.Second,
        ReadTimeout:  3 * time.Second,
        WriteTimeout: 3 * time.Second,
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
// Key patterns
const (
    ProfileKeyPrefix    = "profile"
    PreferencesKeyPrefix = "preferences"
    SessionKeyPrefix    = "session"
)

// Key generation
func ProfileKey(id string) string {
    return fmt.Sprintf("%s:%s", ProfileKeyPrefix, id)
}

func PreferencesKey(profileID string) string {
    return fmt.Sprintf("%s:%s", PreferencesKeyPrefix, profileID)
}
```

### 3. Serialization

```go
type Cache struct {
    client *redis.Client
    ttl    time.Duration
}

func (c *Cache) SetProfile(ctx context.Context, profile *Profile) error {
    data, err := json.Marshal(profile)
    if err != nil {
        return fmt.Errorf("failed to marshal profile: %w", err)
    }

    return c.client.Set(ctx, ProfileKey(profile.ID), data, c.ttl).Err()
}

func (c *Cache) GetProfile(ctx context.Context, id string) (*Profile, error) {
    data, err := c.client.Get(ctx, ProfileKey(id)).Bytes()
    if err != nil {
        if errors.Is(err, redis.Nil) {
            return nil, ErrCacheMiss
        }
        return nil, err
    }

    var profile Profile
    if err := json.Unmarshal(data, &profile); err != nil {
        return nil, fmt.Errorf("failed to unmarshal profile: %w", err)
    }

    return &profile, nil
}
```

## Cache Invalidation

### 1. Time-Based Invalidation

```go
// Different TTLs for different data types
const (
    ProfileCacheTTL     = 1 * time.Hour
    PreferencesCacheTTL = 24 * time.Hour
    SessionCacheTTL     = 15 * time.Minute
)

func (c *Cache) SetWithTTL(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
    data, err := json.Marshal(value)
    if err != nil {
        return err
    }
    return c.client.Set(ctx, key, data, ttl).Err()
}
```

### 2. Manual Invalidation

```go
// Invalidate on update
func (s *ProfileService) UpdateProfile(ctx context.Context, profile *Profile) error {
    if err := s.repository.Update(ctx, profile); err != nil {
        return err
    }

    // Invalidate cache
    if err := s.cache.Delete(ctx, ProfileKey(profile.ID)); err != nil {
        s.logger.Warn("Failed to invalidate cache", zap.Error(err))
    }

    return nil
}

// Invalidate on delete
func (s *ProfileService) DeleteProfile(ctx context.Context, id string) error {
    if err := s.repository.Delete(ctx, id); err != nil {
        return err
    }

    // Invalidate cache
    return s.cache.Delete(ctx, ProfileKey(id))
}
```

## Cache Monitoring

### 1. Metrics

```go
var (
    cacheHitsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "cache_hits_total",
            Help: "Total number of cache hits",
        },
        []string{"cache_type"},
    )

    cacheMissesTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "cache_misses_total",
            Help: "Total number of cache misses",
        },
        []string{"cache_type"},
    )

    cacheOperationDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "cache_operation_duration_seconds",
            Help:    "Cache operation duration in seconds",
            Buckets: []float64{.001, .005, .01, .025, .05, .1},
        },
        []string{"operation", "cache_type"},
    )
)
```

### 2. Health Checks

```go
func (c *Cache) HealthCheck(ctx context.Context) error {
    // Check Redis connection
    if err := c.client.Ping(ctx).Err(); err != nil {
        return fmt.Errorf("Redis health check failed: %w", err)
    }

    return nil
}
```

## Best Practices Summary

1. **Cache Strategy Selection**
   - Use cache-aside for read-heavy workloads
   - Use write-through for consistency-critical data
   - Set appropriate TTLs based on data freshness requirements

2. **Cache Implementation**
   - Use consistent key patterns
   - Handle cache failures gracefully
   - Implement proper error handling
   - Use pipelining for batch operations

3. **Cache Monitoring**
   - Track hit/miss rates
   - Monitor cache latency
   - Set up alerts for anomalies
   - Monitor Redis memory usage

4. **Cache Security**
   - Use authentication
   - Encrypt sensitive data
   - Implement proper access control

## Common Issues and Solutions

1. **Cache Stampede**
   - Solution: Use singleflight pattern

2. **Stale Data**
   - Solution: Set appropriate TTLs, invalidate on updates

3. **Memory Pressure**
   - Solution: Monitor memory, set maxmemory policy

## Cross-References

- [Caching Patterns](caching-patterns.md)
- [Redis Guide](../tools/redis.md)
- [Performance Optimization](../../performance/optimization.md)

## References

- [go-redis Documentation](https://redis.uptrace.dev/)
- [Redis Best Practices](https://redis.io/topics/optimization)
