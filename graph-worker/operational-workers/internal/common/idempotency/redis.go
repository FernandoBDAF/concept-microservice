package idempotency

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisSetNXAdapter is the thin adapter that satisfies RedisSetNXer with the
// real *redis.Client, keeping the go-redis dependency confined to this file
// (the guard logic in guard.go stays client-agnostic and unit-testable).
type redisSetNXAdapter struct {
	client *redis.Client
}

// NewRedisSetNXer adapts a *redis.Client to the one-method RedisSetNXer the
// guard needs: SetNX(ctx,key,val,ttl).Result().
func NewRedisSetNXer(client *redis.Client) RedisSetNXer {
	return &redisSetNXAdapter{client: client}
}

func (a *redisSetNXAdapter) SetNX(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	return a.client.SetNX(ctx, key, value, ttl).Result()
}

// NewRedisGuardFromAddr builds a production RedisGuard against addr
// (host:port, e.g. "redis:6379"). The client dials lazily, so construction
// does not block on Redis being reachable; a later SETNX failure is surfaced
// to the caller, which fails open (ADR-008.2 — dedupe is a net, not a lock).
func NewRedisGuardFromAddr(addr string) *RedisGuard {
	client := redis.NewClient(&redis.Options{Addr: addr})
	return NewRedisGuard(NewRedisSetNXer(client))
}
