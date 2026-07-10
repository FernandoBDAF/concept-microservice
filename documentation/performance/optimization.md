# Performance Optimization

> *Migrated from legacy_project/reference-materials/performance/optimization.md*

## Overview

This document outlines the optimization strategy for the API Service architecture, focusing on improving performance through various techniques and best practices with direct infrastructure access.

## Optimization Areas

### Code Optimization

- Algorithm efficiency
- Memory management
- Concurrency patterns
- Resource pooling
- Caching strategies

### Database Optimization (Direct PostgreSQL via sqlx)

```go
// Connection pool optimization
func NewOptimizedDB(cfg *Config) (*sqlx.DB, error) {
    db, err := sqlx.Open("postgres", cfg.DSN)
    if err != nil {
        return nil, err
    }

    // Optimize connection pool
    db.SetMaxOpenConns(cfg.MaxOpenConns)        // Default: 25
    db.SetMaxIdleConns(cfg.MaxIdleConns)        // Default: 10
    db.SetConnMaxLifetime(30 * time.Minute)
    db.SetConnMaxIdleTime(5 * time.Minute)

    return db, nil
}

// Batch queries for efficiency
func (r *ProfileRepository) GetByIDs(ctx context.Context, ids []string) ([]*Profile, error) {
    query, args, err := sqlx.In(`
        SELECT id, first_name, last_name, email, status, created_at, updated_at
        FROM profiles
        WHERE id IN (?) AND deleted_at IS NULL
    `, ids)
    if err != nil {
        return nil, err
    }
    
    query = r.db.Rebind(query)
    var profiles []*Profile
    err = r.db.SelectContext(ctx, &profiles, query, args...)
    return profiles, err
}
```

### Cache Optimization (Direct Redis via go-redis)

```go
// Optimized cache configuration
func NewOptimizedCache(cfg *Config) *Cache {
    client := redis.NewClient(&redis.Options{
        Addr:         cfg.Addr,
        PoolSize:     cfg.PoolSize,      // Default: 10 * runtime.GOMAXPROCS
        MinIdleConns: cfg.MinIdleConns,  // Default: 10
        MaxRetries:   3,
        DialTimeout:  5 * time.Second,
        ReadTimeout:  3 * time.Second,
        WriteTimeout: 3 * time.Second,
    })

    return &Cache{
        client: client,
        ttl:    cfg.TTL,
    }
}

// Pipelining for batch operations
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
            continue
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

### Infrastructure Optimization

- Resource allocation
- Load balancing
- Network tuning
- Container optimization
- Scaling strategies

## Optimization Process

1. Identify bottlenecks
2. Measure baseline
3. Implement changes
4. Test improvements
5. Monitor results
6. Document changes

## Performance Patterns

### Cache-Aside with Warm-up

```go
func (s *ProfileService) WarmupCache(ctx context.Context) error {
    // Get frequently accessed profiles
    profiles, err := s.repository.GetFrequentlyAccessed(ctx, 1000)
    if err != nil {
        return err
    }

    // Warm up cache in batches
    for i := 0; i < len(profiles); i += 100 {
        end := i + 100
        if end > len(profiles) {
            end = len(profiles)
        }

        batch := profiles[i:end]
        pipe := s.cache.Pipeline()
        for _, profile := range batch {
            key := fmt.Sprintf("profile:%s", profile.ID)
            data, _ := json.Marshal(profile)
            pipe.Set(ctx, key, data, s.cacheTTL)
        }
        pipe.Exec(ctx)
    }

    return nil
}
```

### Singleflight for Cache Stampede Prevention

```go
import "golang.org/x/sync/singleflight"

type ProfileService struct {
    repository *ProfileRepository
    cache      *Cache
    sflight    singleflight.Group
}

func (s *ProfileService) GetProfile(ctx context.Context, id string) (*Profile, error) {
    // Use singleflight to prevent cache stampede
    key := fmt.Sprintf("profile:%s", id)
    
    result, err, _ := s.sflight.Do(key, func() (interface{}, error) {
        // Try cache first
        if profile, err := s.cache.GetProfile(ctx, id); err == nil {
            return profile, nil
        }

        // Get from database
        profile, err := s.repository.Get(ctx, id)
        if err != nil {
            return nil, err
        }

        // Update cache
        s.cache.SetProfile(ctx, profile)
        return profile, nil
    })

    if err != nil {
        return nil, err
    }
    return result.(*Profile), nil
}
```

## Best Practices

- Regular profiling
- Performance testing
- Code reviews
- Monitoring
- Documentation

## Success Metrics

| Metric | Target | Current |
|--------|--------|---------|
| p95 Response Time (cached) | < 50ms | - |
| p95 Response Time (uncached) | < 100ms | - |
| Cache Hit Rate | > 80% | - |
| Error Rate | < 0.1% | - |
| CPU Utilization | < 70% | - |
| Memory Usage | < 80% | - |

## Cross-References

- [Load Testing Strategy](load-testing-strategy.md)
- [Performance Benchmarking](benchmarking.md)
- [Performance Monitoring](monitoring.md)
- [Redis Guide](../development/tools/redis.md)
- [PostgreSQL Guide](../development/tools/postgresql.md)

## Notes

- Continuous optimization
- Measure before/after
- Document all changes
- Consider trade-offs
- Regular reviews
