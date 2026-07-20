// Package idempotency implements the shared consumer dedupe guard
// (ADR-008.2): first delivery of an envelope id wins, redeliveries within
// the TTL window are skipped. Workers call Guard.Begin before processing;
// a false return means "already handled (or in flight) — ack and move on".
//
// SKELETON (phase v4 handoff): InMemoryGuard is complete and unit-testable;
// RedisGuard is the production implementation and needs the go-redis client
// wired in. See documentation/phases/v4-HANDOFF.md §A4 for the exact steps.
package idempotency

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"
)

// DefaultTTL bounds the dedupe window. RabbitMQ redelivery happens within
// seconds/minutes; 24h covers operator-driven replays too (ADR-008.2).
const DefaultTTL = 24 * time.Hour

// KeyFor namespaces envelope ids per queue so the same message id consumed
// by two different queues (fanout future) doesn't collide.
func KeyFor(queue, envelopeID string) string {
	return "idem:" + queue + ":" + envelopeID
}

// KeyForAttempt scopes KeyFor by the retry attempt (x-death count). This lets
// the guard dedupe a genuine duplicate delivery (same attempt redelivered —
// e.g. a crash between process and ack, where x-death is unchanged) while
// still admitting an intentional retry (ADR-008.1 republish increments the
// attempt), which must be reprocessed rather than silently deduped.
// Key shape: idem:<queue>:<envelopeID>:<attempt>. The Python graphrag consumer
// mirrors this exact shape.
func KeyForAttempt(queue, envelopeID string, attempt int) string {
	return KeyFor(queue, envelopeID) + ":" + strconv.Itoa(attempt)
}

// Guard is the seam BaseWorker will call around processing.
//
//	ok, err := guard.Begin(ctx, key, ttl)
//	if err != nil  -> treat as retryable infra failure (do NOT drop the message)
//	if !ok         -> duplicate: ack without processing, count a metric
type Guard interface {
	Begin(ctx context.Context, key string, ttl time.Duration) (bool, error)
}

// ErrNotImplemented marks skeleton paths (v4 handoff work).
var ErrNotImplemented = errors.New("idempotency: not implemented (v4 handoff)")

// ── In-memory implementation (tests, single-replica dev) ────────────────────

type InMemoryGuard struct {
	mu   sync.Mutex
	seen map[string]time.Time // key -> expiry
}

func NewInMemoryGuard() *InMemoryGuard {
	return &InMemoryGuard{seen: make(map[string]time.Time)}
}

func (g *InMemoryGuard) Begin(_ context.Context, key string, ttl time.Duration) (bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := time.Now()
	if exp, dup := g.seen[key]; dup && now.Before(exp) {
		return false, nil
	}
	g.seen[key] = now.Add(ttl)
	return true, nil
}

// ── Redis implementation (production; TODO v4) ──────────────────────────────

// RedisSetNXer is the one-method slice of go-redis this package needs, kept
// as an interface so the module doesn't take the dependency until wired:
// *redis.Client satisfies it via a thin adapter (HANDOFF §A4 shows it).
type RedisSetNXer interface {
	SetNX(ctx context.Context, key string, value any, ttl time.Duration) (bool, error)
}

type RedisGuard struct {
	client RedisSetNXer
}

func NewRedisGuard(client RedisSetNXer) *RedisGuard { return &RedisGuard{client: client} }

// Begin is a straight SETNX with TTL: true == we own this envelope.
// TODO(v4): wire go-redis in BaseWorker (env REDIS_ADDR, fail-open vs
// fail-closed decision is documented in HANDOFF §A4 — the lab chooses
// fail-open with a loud metric, because dedupe is a safety net, not a lock).
func (g *RedisGuard) Begin(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	if g.client == nil {
		return false, ErrNotImplemented
	}
	return g.client.SetNX(ctx, key, 1, ttl)
}
