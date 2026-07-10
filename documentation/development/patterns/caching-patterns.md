# Caching Patterns

> *Migrated and adapted from legacy_project/reference-materials/development/patterns/caching-patterns.md*

## Overview

This document outlines the caching patterns implemented in the API Service using direct Redis access via go-redis.

## Architecture Context

In the consolidated architecture, caching is done through **direct Redis access** using go-redis, not HTTP calls to a cache service.

```go
// Direct Redis client
import "github.com/redis/go-redis/v9"

type Cache struct {
    client *redis.Client
    ttl    time.Duration
}
```

## Cache Strategies

### 1. Cache-Aside Pattern (Recommended)

The primary caching pattern for profile data:

```go
func (s *ProfileService) GetProfile(ctx context.Context, id string) (*Profile, error) {
    // Try cache first
    if profile, err := s.cache.GetProfile(ctx, id); err == nil {
        return profile, nil
    }

    // Cache miss - get from database
    profile, err := s.repository.Get(ctx, id)
    if err != nil {
        return nil, err
    }

    // Update cache asynchronously
    go s.cache.SetProfile(context.Background(), profile)

    return profile, nil
}
```

### 2. Write-Through Pattern

For critical data that must be consistent:

```go
func (s *ProfileService) UpdateProfile(ctx context.Context, profile *Profile) error {
    // Update database first
    if err := s.repository.Update(ctx, profile); err != nil {
        return err
    }

    // Then update cache
    return s.cache.SetProfile(ctx, profile)
}
```

### 3. Cache Invalidation Pattern

For updates and deletes:

```go
func (s *ProfileService) DeleteProfile(ctx context.Context, id string) error {
    // Delete from database
    if err := s.repository.Delete(ctx, id); err != nil {
        return err
    }

    // Invalidate cache
    return s.cache.DeleteProfile(ctx, id)
}
```

## Implementation

### Cache Client

```go
type Cache struct {
    client *redis.Client
    ttl    time.Duration
    logger *zap.Logger
}

func NewCache(client *redis.Client, ttl time.Duration, logger *zap.Logger) *Cache {
    return &Cache{
        client: client,
        ttl:    ttl,
        logger: logger,
    }
}

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

func (c *Cache) SetProfile(ctx context.Context, profile *Profile) error {
    key := fmt.Sprintf("profile:%s", profile.ID)
    
    data, err := json.Marshal(profile)
    if err != nil {
        return fmt.Errorf("cache marshal failed: %w", err)
    }

    return c.client.Set(ctx, key, data, c.ttl).Err()
}

func (c *Cache) DeleteProfile(ctx context.Context, id string) error {
    key := fmt.Sprintf("profile:%s", id)
    return c.client.Del(ctx, key).Err()
}
```

### Batch Operations with Pipelining

```go
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
            continue // Skip misses
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

## Cache Configuration

```go
type CacheConfig struct {
    TTL             time.Duration // Default: 24 hours
    ProfileTTL      time.Duration // Default: 1 hour
    SessionTTL      time.Duration // Default: 15 minutes
}

const (
    DefaultCacheTTL  = 24 * time.Hour
    ProfileCacheTTL  = 1 * time.Hour
    SessionCacheTTL  = 15 * time.Minute
)
```

## Best Practices

1. **Use appropriate TTLs** - Balance freshness vs hit rate
2. **Handle cache misses gracefully** - Always fall back to database
3. **Use pipelining for batch operations** - Reduce round trips
4. **Implement cache warmup** - Pre-populate on startup for hot data
5. **Monitor cache metrics** - Track hit/miss rates

## Cross-References

- [Data Storage Patterns](data-storage-patterns.md)
- [Queuing Patterns](queuing-patterns.md)
- [Caching Best Practices](caching-best-practices.md)
- [Redis Guide](../tools/redis.md)

## Notes

- Always use direct go-redis client, not HTTP
- Implement singleflight to prevent cache stampede
- Use JSON serialization for structured data
