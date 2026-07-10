# Redis Usage Guide

> *Migrated from legacy_project/reference-materials/development/tools/redis.md*

## Overview

Redis is our primary caching solution, providing high-performance data storage capabilities. This guide covers our Redis implementation using go-redis for direct access, best practices, and common patterns.

## Key Features Used

### 1. Connection Management with go-redis

We use go-redis for direct Redis access:

```go
// Redis client configuration
type RedisConfig struct {
    Addr     string
    Password string
    DB       int
    PoolSize int
}

func NewRedisClient(cfg *RedisConfig) (*redis.Client, error) {
    client := redis.NewClient(&redis.Options{
        Addr:         cfg.Addr,
        Password:     cfg.Password,
        DB:           cfg.DB,
        PoolSize:     cfg.PoolSize,
        MinIdleConns: 10,
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

### 2. Profile Cache Implementation

Direct cache operations for profiles:

```go
// Cache service with go-redis
type Cache struct {
    client *redis.Client
    ttl    time.Duration
}

func NewCache(client *redis.Client, ttl time.Duration) *Cache {
    return &Cache{client: client, ttl: ttl}
}

// Get profile from cache
func (c *Cache) GetProfile(ctx context.Context, id string) (*Profile, error) {
    key := fmt.Sprintf("profile:%s", id)
    data, err := c.client.Get(ctx, key).Bytes()
    if err != nil {
        if errors.Is(err, redis.Nil) {
            return nil, ErrCacheMiss
        }
        return nil, fmt.Errorf("cache get failed: %w", err)
    }

    var profile Profile
    if err := json.Unmarshal(data, &profile); err != nil {
        return nil, fmt.Errorf("cache unmarshal failed: %w", err)
    }

    return &profile, nil
}

// Set profile in cache
func (c *Cache) SetProfile(ctx context.Context, profile *Profile) error {
    key := fmt.Sprintf("profile:%s", profile.ID)
    data, err := json.Marshal(profile)
    if err != nil {
        return fmt.Errorf("cache marshal failed: %w", err)
    }

    return c.client.Set(ctx, key, data, c.ttl).Err()
}

// Delete profile from cache
func (c *Cache) DeleteProfile(ctx context.Context, id string) error {
    key := fmt.Sprintf("profile:%s", id)
    return c.client.Del(ctx, key).Err()
}
```

### 3. Cache-Aside Pattern

The recommended caching pattern for our service:

```go
// Service with cache-aside pattern
type ProfileService struct {
    repository *ProfileRepository
    cache      *Cache
    logger     *zap.Logger
}

func (s *ProfileService) GetProfile(ctx context.Context, id string) (*Profile, error) {
    // Try cache first
    profile, err := s.cache.GetProfile(ctx, id)
    if err == nil {
        return profile, nil
    }
    if !errors.Is(err, ErrCacheMiss) {
        s.logger.Warn("cache error", zap.Error(err))
    }

    // Get from database
    profile, err = s.repository.Get(ctx, id)
    if err != nil {
        return nil, err
    }

    // Update cache (fire and forget)
    go func() {
        if err := s.cache.SetProfile(context.Background(), profile); err != nil {
            s.logger.Warn("failed to update cache", zap.Error(err))
        }
    }()

    return profile, nil
}

// Write-through on updates
func (s *ProfileService) UpdateProfile(ctx context.Context, profile *Profile) error {
    // Update database first
    if err := s.repository.Update(ctx, profile); err != nil {
        return err
    }

    // Invalidate cache
    if err := s.cache.DeleteProfile(ctx, profile.ID); err != nil {
        s.logger.Warn("failed to invalidate cache", zap.Error(err))
    }

    return nil
}
```

### 4. Pipelining for Batch Operations

```go
// Batch get profiles
func (c *Cache) GetProfiles(ctx context.Context, ids []string) (map[string]*Profile, error) {
    pipe := c.client.Pipeline()
    cmds := make(map[string]*redis.StringCmd)

    for _, id := range ids {
        key := fmt.Sprintf("profile:%s", id)
        cmds[id] = pipe.Get(ctx, key)
    }

    _, err := pipe.Exec(ctx)
    if err != nil && !errors.Is(err, redis.Nil) {
        return nil, err
    }

    profiles := make(map[string]*Profile)
    for id, cmd := range cmds {
        data, err := cmd.Bytes()
        if err != nil {
            continue // Skip cache misses
        }

        var profile Profile
        if err := json.Unmarshal(data, &profile); err != nil {
            continue
        }
        profiles[id] = &profile
    }

    return profiles, nil
}
```

## Best Practices

1. **Connection Management**

   - Use connection pooling
   - Set appropriate timeouts
   - Handle connection errors
   - Monitor connection health

2. **Cache Strategy**

   - Use cache-aside pattern
   - Set appropriate TTLs
   - Handle cache misses gracefully
   - Monitor cache hit rates

3. **Data Serialization**

   - Use efficient serialization (JSON or msgpack)
   - Handle serialization errors
   - Keep cached data minimal
   - Version cache keys when schema changes

4. **Performance**

   - Use pipelining for batch operations
   - Monitor memory usage
   - Set appropriate pool sizes
   - Use appropriate data structures

## Common Issues and Solutions

1. **Memory Issues**

   - Problem: High memory usage
   - Solution: Set appropriate TTLs, monitor memory usage

2. **Connection Issues**

   - Problem: Connection failures
   - Solution: Implement retry logic, use connection pooling

3. **Cache Stampede**
   - Problem: Many requests hit database on cache miss
   - Solution: Use singleflight pattern or probabilistic early expiration

## Cross-References

- [PostgreSQL Guide](postgresql.md)
- [Performance Optimization](../../performance/optimization.md)

## References

- [go-redis Documentation](https://redis.uptrace.dev/)
- [Redis Documentation](https://redis.io/documentation)
- [Redis Best Practices](https://redis.io/topics/optimization)
