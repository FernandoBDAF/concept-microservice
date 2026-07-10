# Redis Usage Guide

## Overview

Redis is our primary caching and pub/sub solution, providing high-performance data storage and message distribution capabilities. This guide covers our Redis implementation, best practices, and common patterns.

## Key Features Used

### 1. Connection Management

We use a connection pool for efficient Redis connections:

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

### 2. Data Structures

We use various Redis data structures for different use cases:

```go
// String operations
func (c *RedisClient) SetProfile(ctx context.Context, profile *Profile) error {
    data, err := json.Marshal(profile)
    if err != nil {
        return fmt.Errorf("failed to marshal profile: %w", err)
    }

    return c.client.Set(ctx, fmt.Sprintf("profile:%s", profile.ID), data, 24*time.Hour).Err()
}

// Hash operations
func (c *RedisClient) SetProfileHash(ctx context.Context, profile *Profile) error {
    pipe := c.client.Pipeline()
    pipe.HSet(ctx, fmt.Sprintf("profile:%s", profile.ID),
        "name", profile.Name,
        "email", profile.Email,
        "created_at", profile.CreatedAt.Unix(),
    )
    pipe.Expire(ctx, fmt.Sprintf("profile:%s", profile.ID), 24*time.Hour)
    _, err := pipe.Exec(ctx)
    return err
}

// Set operations
func (c *RedisClient) AddUserRoles(ctx context.Context, userID string, roles []string) error {
    return c.client.SAdd(ctx, fmt.Sprintf("user:%s:roles", userID), roles).Err()
}

// Sorted Set operations
func (c *RedisClient) AddUserScore(ctx context.Context, userID string, score float64) error {
    return c.client.ZAdd(ctx, "user:scores", &redis.Z{
        Score:  score,
        Member: userID,
    }).Err()
}
```

### 3. Caching Patterns

We implement various caching patterns:

```go
// Cache-Aside Pattern
func (c *RedisClient) GetProfile(ctx context.Context, id string) (*Profile, error) {
    // Try to get from cache
    data, err := c.client.Get(ctx, fmt.Sprintf("profile:%s", id)).Bytes()
    if err == nil {
        var profile Profile
        if err := json.Unmarshal(data, &profile); err == nil {
            return &profile, nil
        }
    }

    // Cache miss, get from database
    profile, err := c.repository.Get(ctx, id)
    if err != nil {
        return nil, err
    }

    // Update cache
    if err := c.SetProfile(ctx, profile); err != nil {
        c.logger.Warn("failed to update cache", "error", err)
    }

    return profile, nil
}

// Write-Through Pattern
func (c *RedisClient) UpdateProfile(ctx context.Context, profile *Profile) error {
    // Update database
    if err := c.repository.Update(ctx, profile); err != nil {
        return err
    }

    // Update cache
    return c.SetProfile(ctx, profile)
}

// Cache-Aside with TTL
func (c *RedisClient) GetProfileWithTTL(ctx context.Context, id string) (*Profile, error) {
    key := fmt.Sprintf("profile:%s", id)

    // Try to get from cache
    data, err := c.client.Get(ctx, key).Bytes()
    if err == nil {
        var profile Profile
        if err := json.Unmarshal(data, &profile); err == nil {
            return &profile, nil
        }
    }

    // Cache miss, get from database
    profile, err := c.repository.Get(ctx, id)
    if err != nil {
        return nil, err
    }

    // Update cache with TTL
    data, err = json.Marshal(profile)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal profile: %w", err)
    }

    if err := c.client.Set(ctx, key, data, 24*time.Hour).Err(); err != nil {
        c.logger.Warn("failed to update cache", "error", err)
    }

    return profile, nil
}
```

### 4. Pub/Sub Implementation

We use Redis Pub/Sub for event distribution:

```go
// Publisher
type Publisher struct {
    client *redis.Client
    logger *zap.Logger
}

func (p *Publisher) PublishEvent(ctx context.Context, channel string, event interface{}) error {
    data, err := json.Marshal(event)
    if err != nil {
        return fmt.Errorf("failed to marshal event: %w", err)
    }

    return p.client.Publish(ctx, channel, data).Err()
}

// Subscriber
type Subscriber struct {
    client *redis.Client
    logger *zap.Logger
}

func (s *Subscriber) Subscribe(ctx context.Context, channel string, handler func([]byte) error) error {
    pubsub := s.client.Subscribe(ctx, channel)
    defer pubsub.Close()

    ch := pubsub.Channel()
    for msg := range ch {
        if err := handler([]byte(msg.Payload)); err != nil {
            s.logger.Error("failed to handle message",
                zap.String("channel", channel),
                zap.Error(err))
        }
    }

    return nil
}
```

## Best Practices

1. **Connection Management**

   - Use connection pooling
   - Set appropriate timeouts
   - Handle connection errors
   - Monitor connection health

2. **Data Structure Usage**

   - Choose appropriate structures
   - Use atomic operations
   - Implement proper TTLs
   - Handle data serialization

3. **Caching Strategy**

   - Implement proper invalidation
   - Use appropriate TTLs
   - Handle cache misses
   - Monitor cache performance

4. **Pub/Sub Usage**

   - Handle message loss
   - Implement retry logic
   - Monitor message flow
   - Handle subscriber errors

## Common Issues and Solutions

1. **Memory Issues**

   - Problem: High memory usage
   - Solution: Set appropriate TTLs, monitor memory usage

2. **Connection Issues**

   - Problem: Connection failures
   - Solution: Implement retry logic, use connection pooling

3. **Performance Issues**
   - Problem: Slow operations
   - Solution: Use pipelining, optimize data structures

## Examples from Our Project

### Cache Implementation

```go
type Cache struct {
    client *redis.Client
    logger *zap.Logger
}

func (c *Cache) Get(ctx context.Context, key string) ([]byte, error) {
    return c.client.Get(ctx, key).Bytes()
}

func (c *Cache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
    return c.client.Set(ctx, key, value, ttl).Err()
}

func (c *Cache) Delete(ctx context.Context, key string) error {
    return c.client.Del(ctx, key).Err()
}
```

### Event Publisher

```go
type EventPublisher struct {
    client *redis.Client
    logger *zap.Logger
}

func (p *EventPublisher) PublishProfileUpdated(ctx context.Context, profile *Profile) error {
    event := ProfileUpdatedEvent{
        ID:        profile.ID,
        Timestamp: time.Now(),
    }

    data, err := json.Marshal(event)
    if err != nil {
        return fmt.Errorf("failed to marshal event: %w", err)
    }

    return p.client.Publish(ctx, "profile:updated", data).Err()
}
```

## References

- [Redis Documentation](https://redis.io/documentation)
- [Redis Data Types](https://redis.io/topics/data-types)
- [Redis Pub/Sub](https://redis.io/topics/pubsub)
- [Redis Best Practices](https://redis.io/topics/optimization)
